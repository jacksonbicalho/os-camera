package webcam

import (
	"context"
	"io/fs"
	"log/slog"
	"os"
	osexec "os/exec"
	"strings"
	"sync"
	"time"

	"github.com/bluenviron/gortsplib/v4"
	"github.com/bluenviron/gortsplib/v4/pkg/base"
	"github.com/bluenviron/gortsplib/v4/pkg/description"
	"github.com/bluenviron/gortsplib/v4/pkg/format"
	"github.com/pion/rtp"
)

const (
	defaultFPS  = 30
	defaultSize = "1280x720"
)

// Manager hospeda o servidor RTSP embutido e supervisiona um ffmpeg por webcam
// que publica /dev/videoN nele. O valor zero não é utilizável; use New.
type Manager struct {
	ctx  context.Context
	addr string
	log  *slog.Logger
	fps  int
	size string

	mu   sync.Mutex
	srv  *gortsplib.Server
	pubs map[string]context.CancelFunc // rtspName (webcamN) → cancela o publisher
}

// New cria o Manager ligado a ctx (encerra tudo quando ctx é cancelado). addr
// vazio usa DefaultRTSPAddress. O servidor RTSP sobe sob demanda (no 1º Ensure);
// sem webcam cadastrada, nada roda.
func New(ctx context.Context, addr string, log *slog.Logger) *Manager {
	if addr == "" {
		addr = DefaultRTSPAddress
	}
	m := &Manager{
		ctx:  ctx,
		addr: addr,
		log:  log,
		fps:  defaultFPS,
		size: defaultSize,
		pubs: map[string]context.CancelFunc{},
	}
	go func() { <-ctx.Done(); m.shutdown() }()
	return m
}

// WebcamName devolve o path (ex.: "webcam0") se rtspURL aponta para o restream
// local deste Manager; senão "", false. Usado para amarrar o publisher ao ciclo
// de vida da câmera cadastrada.
func (m *Manager) WebcamName(rtspURL string) (string, bool) {
	prefix := "rtsp://" + m.addr + "/"
	if !strings.HasPrefix(rtspURL, prefix) {
		return "", false
	}
	name := strings.TrimPrefix(rtspURL, prefix)
	if name == "" || strings.Contains(name, "/") || !strings.HasPrefix(name, "webcam") {
		return "", false
	}
	return name, true
}

// Ensure garante que o restream do device identificado por rtspName (webcamN)
// está rodando. Idempotente. Chamar quando uma câmera webcam é iniciada.
func (m *Manager) Ensure(rtspName string) {
	dev, ok := findDevice(rtspName)
	if !ok {
		m.log.Warn("webcam: device não encontrado para o restream", "name", rtspName)
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, running := m.pubs[rtspName]; running {
		return
	}
	if !m.startServerLocked() {
		return
	}
	pctx, cancel := context.WithCancel(m.ctx)
	m.pubs[rtspName] = cancel
	go m.publishLoop(pctx, dev)
	m.log.Info("webcam restream iniciado", "device", dev.Path, "rtsp", RTSPURL(m.addr, rtspName))
}

// Release para o restream do device (libera o /dev/videoN). Chamar quando a
// câmera webcam é parada/excluída.
func (m *Manager) Release(rtspName string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if cancel, ok := m.pubs[rtspName]; ok {
		cancel()
		delete(m.pubs, rtspName)
		m.log.Info("webcam restream parado", "rtsp", RTSPURL(m.addr, rtspName))
	}
}

// startServerLocked sobe o servidor RTSP embutido (uma vez). Requer m.mu.
func (m *Manager) startServerLocked() bool {
	if m.srv != nil {
		return true
	}
	srv := &gortsplib.Server{Handler: newRelay(), RTSPAddress: m.addr}
	if h, ok := srv.Handler.(*relay); ok {
		h.server = srv
	}
	if err := srv.Start(); err != nil {
		m.log.Error("webcam: rtsp server start", "error", err)
		return false
	}
	m.srv = srv
	m.log.Info("webcam: servidor RTSP no ar", "rtsp", m.addr)
	return true
}

func (m *Manager) shutdown() {
	m.mu.Lock()
	defer m.mu.Unlock()
	for name, cancel := range m.pubs {
		cancel()
		delete(m.pubs, name)
	}
	if m.srv != nil {
		m.srv.Close()
		m.srv = nil
	}
}

// findDevice procura a webcam local cujo RTSPName casa (lista fresca do sysfs,
// cobrindo hotplug).
func findDevice(rtspName string) (Device, bool) {
	for _, d := range List(sysfsRoot()) {
		if d.RTSPName == rtspName {
			return d, true
		}
	}
	return Device{}, false
}

func sysfsRoot() fs.FS {
	if _, err := os.Stat(SysfsRoot); err != nil {
		return emptyFS{}
	}
	return os.DirFS(SysfsRoot)
}

type emptyFS struct{}

func (emptyFS) Open(string) (fs.File, error) { return nil, fs.ErrNotExist }

// publishArgs monta os args do ffmpeg que lê o device v4l2 e publica no RTSP
// embutido (H.264, baixa latência). Keyframe a cada ~1s (`-g` + `-force_key_frames`)
// para o HLS poder cortar segmentos curtos — sem isso o GOP padrão (250 frames) vira
// dezenas de segundos de atraso no "ao vivo".
func publishArgs(dev Device, addr string, fps int, size string) []string {
	return []string{
		"-f", "v4l2",
		"-framerate", itoa(fps),
		"-video_size", size,
		"-i", dev.Path,
		"-c:v", "libx264",
		"-preset", "ultrafast",
		"-tune", "zerolatency",
		"-pix_fmt", "yuv420p",
		"-g", itoa(fps),
		"-force_key_frames", "expr:gte(t,n_forced*1)",
		"-rtsp_transport", "tcp",
		"-f", "rtsp",
		RTSPURL(addr, dev.RTSPName),
	}
}

// publishLoop mantém o ffmpeg publisher vivo, reiniciando se cair, até ctx. O
// stderr do ffmpeg é capturado e logado quando ele encerra sozinho — para que
// falhas (ex.: device inacessível no Docker, formato não suportado) sejam
// visíveis em vez de silenciosas.
func (m *Manager) publishLoop(ctx context.Context, dev Device) {
	args := publishArgs(dev, m.addr, m.fps, m.size)
	warnedMissing := false
	for ctx.Err() == nil {
		// O device foi detectado no /sys, mas pode não estar acessível (típico em
		// Docker sem passthrough). Evita o crash-loop do ffmpeg a cada 1s e loga
		// uma dica acionável uma vez.
		if _, err := os.Stat(dev.Path); err != nil {
			if !warnedMissing {
				m.log.Warn("webcam: device inacessível — em Docker passe o device (docker-compose.webcam.yml ou --device "+dev.Path+")",
					"device", dev.Path, "error", err)
				warnedMissing = true
			}
			if !sleepCtx(ctx, 5*time.Second) {
				return
			}
			continue
		}
		warnedMissing = false

		cmd := osexec.CommandContext(ctx, "ffmpeg", args...)
		cmd.Env = append(os.Environ(), "TZ=UTC")
		var tail tailWriter
		cmd.Stderr = &tail
		if err := cmd.Start(); err != nil {
			m.log.Warn("webcam: ffmpeg não iniciou", "device", dev.Path, "error", err)
			if !sleepCtx(ctx, 2*time.Second) {
				return
			}
			continue
		}
		err := cmd.Wait()
		if ctx.Err() != nil {
			return // shutdown
		}
		m.log.Warn("webcam: publisher ffmpeg encerrou, reiniciando",
			"device", dev.Path, "error", err, "ffmpeg_stderr", tail.String())
		if !sleepCtx(ctx, time.Second) {
			return
		}
	}
}

// tailWriter guarda os últimos bytes escritos (cauda do stderr do ffmpeg).
type tailWriter struct {
	mu  sync.Mutex
	buf []byte
}

func (w *tailWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.buf = append(w.buf, p...)
	const max = 2048
	if len(w.buf) > max {
		w.buf = w.buf[len(w.buf)-max:]
	}
	return len(p), nil
}

func (w *tailWriter) String() string {
	w.mu.Lock()
	defer w.mu.Unlock()
	return strings.TrimSpace(string(w.buf))
}

func sleepCtx(ctx context.Context, d time.Duration) bool {
	select {
	case <-ctx.Done():
		return false
	case <-time.After(d):
		return true
	}
}

func itoa(n int) string {
	if n <= 0 {
		return "0"
	}
	var b [20]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	return string(b[i:])
}

// relay é o handler do servidor RTSP: aceita um publisher e encaminha os pacotes
// RTP aos readers (os ffmpeg do pipeline).
type relay struct {
	server    *gortsplib.Server
	mu        sync.Mutex
	stream    *gortsplib.ServerStream
	publisher *gortsplib.ServerSession
}

func newRelay() *relay { return &relay{} }

func (h *relay) OnSessionClose(ctx *gortsplib.ServerHandlerOnSessionCloseCtx) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.stream != nil && ctx.Session == h.publisher {
		h.stream.Close()
		h.stream = nil
	}
}

func (h *relay) OnDescribe(ctx *gortsplib.ServerHandlerOnDescribeCtx) (*base.Response, *gortsplib.ServerStream, error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.stream == nil {
		return &base.Response{StatusCode: base.StatusNotFound}, nil, nil
	}
	return &base.Response{StatusCode: base.StatusOK}, h.stream, nil
}

func (h *relay) OnAnnounce(ctx *gortsplib.ServerHandlerOnAnnounceCtx) (*base.Response, error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.stream != nil {
		h.stream.Close()
		if h.publisher != nil {
			h.publisher.Close()
		}
	}
	h.stream = &gortsplib.ServerStream{Server: h.server, Desc: ctx.Description}
	if err := h.stream.Initialize(); err != nil {
		h.stream = nil
		return &base.Response{StatusCode: base.StatusInternalServerError}, err
	}
	h.publisher = ctx.Session
	return &base.Response{StatusCode: base.StatusOK}, nil
}

func (h *relay) OnSetup(ctx *gortsplib.ServerHandlerOnSetupCtx) (*base.Response, *gortsplib.ServerStream, error) {
	// SETUP do publisher (PreRecord): apenas OK, sem stream — devolver o stream
	// aqui faria o gortsplib tratar o publisher como reader e fechar a conexão.
	if ctx.Session.State() == gortsplib.ServerSessionStatePreRecord {
		return &base.Response{StatusCode: base.StatusOK}, nil, nil
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.stream == nil {
		return &base.Response{StatusCode: base.StatusNotFound}, nil, nil
	}
	return &base.Response{StatusCode: base.StatusOK}, h.stream, nil
}

func (h *relay) OnPlay(_ *gortsplib.ServerHandlerOnPlayCtx) (*base.Response, error) {
	return &base.Response{StatusCode: base.StatusOK}, nil
}

func (h *relay) OnRecord(ctx *gortsplib.ServerHandlerOnRecordCtx) (*base.Response, error) {
	ctx.Session.OnPacketRTPAny(func(medi *description.Media, _ format.Format, pkt *rtp.Packet) {
		h.mu.Lock()
		s := h.stream
		h.mu.Unlock()
		if s != nil {
			s.WritePacketRTP(medi, pkt)
		}
	})
	return &base.Response{StatusCode: base.StatusOK}, nil
}
