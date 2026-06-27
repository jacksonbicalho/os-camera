package deviceinfo

import (
	"context"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

const defaultSysClassVideo = "/sys/class/video4linux"

// Webcam coleta metadados de uma webcam local (v4l2) via sysfs, quando a câmera é
// o restream embutido (rtsp://127.0.0.1:8554/webcamN). Self-contained (lê /sys).
type Webcam struct {
	sysClassVideo string // injetável nos testes
}

// NewWebcam cria o collector lendo o sysfs real.
func NewWebcam() *Webcam { return &Webcam{sysClassVideo: defaultSysClassVideo} }

func (w *Webcam) Name() string { return "webcam" }

var webcamURLRe = regexp.MustCompile(`^rtsp://127\.0\.0\.1:8554/(webcam\d+)$`)

// webcamIndex extrai N da URL do restream local (webcamN); ok=false se não casa.
func webcamIndex(rtspURL string) (string, bool) {
	m := webcamURLRe.FindStringSubmatch(strings.TrimSpace(rtspURL))
	if m == nil {
		return "", false
	}
	return strings.TrimPrefix(m[1], "webcam"), true
}

func (w *Webcam) Detect(_ context.Context, t Target) bool {
	_, ok := webcamIndex(t.RTSPURL)
	return ok
}

func (w *Webcam) Collect(_ context.Context, t Target) (map[string]string, error) {
	idx, ok := webcamIndex(t.RTSPURL)
	if !ok {
		return nil, nil
	}
	base := w.sysClassVideo
	if base == "" {
		base = defaultSysClassVideo
	}
	videoDir := filepath.Join(base, "video"+idx)
	name := readTrim(filepath.Join(videoDir, "name"))
	usb := readUSBAttrs(filepath.Join(videoDir, "device"))
	return webcamIdentity(name, usb), nil
}

// webcamIdentity monta as chaves de Identidade a partir do nome v4l2 (card) e dos
// atributos USB. Pura (sem I/O).
func webcamIdentity(name string, usb map[string]string) map[string]string {
	out := map[string]string{"collector": "webcam"}
	model := usb["product"]
	if model == "" {
		model = name
	}
	if model != "" {
		out["model"] = model
	}
	if v := usb["manufacturer"]; v != "" {
		out["vendor"] = v
	}
	if v := usb["serial"]; v != "" {
		out["serial"] = v
	}
	out["connection"] = connectionLabel(usb["removable"])
	return out
}

// connectionLabel traduz o atributo `removable` do device USB: `fixed` = embutida
// (webcam de notebook), `removable` = USB externa.
func connectionLabel(removable string) string {
	switch strings.TrimSpace(removable) {
	case "fixed":
		return "Integrada"
	case "removable":
		return "USB"
	default:
		return "Desconhecida"
	}
}

// readUSBAttrs resolve o symlink `device` (interface USB) e sobe a árvore até o
// device USB (o dir que tem `idVendor`), lendo os atributos relevantes.
func readUSBAttrs(deviceSymlink string) map[string]string {
	out := map[string]string{}
	dir, err := filepath.EvalSymlinks(deviceSymlink)
	if err != nil {
		return out
	}
	for i := 0; i < 8 && dir != "/" && dir != "."; i++ {
		if _, err := os.Stat(filepath.Join(dir, "idVendor")); err == nil {
			for _, k := range []string{"idVendor", "idProduct", "manufacturer", "product", "serial", "removable"} {
				if v := readTrim(filepath.Join(dir, k)); v != "" {
					out[k] = v
				}
			}
			return out
		}
		dir = filepath.Dir(dir)
	}
	return out
}

func readTrim(path string) string {
	b, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(b))
}
