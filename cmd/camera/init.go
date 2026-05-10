package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
)

func runInit(args []string) {
	outPath := "camera.yaml"
	for i, a := range args {
		switch {
		case a == "--output" && i+1 < len(args):
			outPath = args[i+1]
		case strings.HasPrefix(a, "--output="):
			outPath = strings.TrimPrefix(a, "--output=")
		}
	}

	yaml, err := initWizard(os.Stdin, os.Stdout)
	if err != nil {
		fmt.Fprintf(os.Stderr, "erro: %v\n", err)
		os.Exit(1)
	}

	if _, statErr := os.Stat(outPath); statErr == nil {
		fmt.Printf("%s já existe. Sobrescrever? (s/n) [n]: ", outPath)
		var answer string
		fmt.Scanln(&answer)
		if strings.ToLower(strings.TrimSpace(answer)) != "s" {
			fmt.Println("Cancelado.")
			os.Exit(0)
		}
	}

	if err := os.WriteFile(outPath, []byte(yaml), 0o644); err != nil {
		fmt.Fprintf(os.Stderr, "erro ao escrever %s: %v\n", outPath, err)
		os.Exit(1)
	}
	fmt.Printf("\nArquivo gerado: %s\n", outPath)
}

type wizardReader struct {
	r *bufio.Reader
	w io.Writer
}

func (wi *wizardReader) ask(prompt, def string) string {
	if def != "" {
		fmt.Fprintf(wi.w, "%s [%s]: ", prompt, def)
	} else {
		fmt.Fprintf(wi.w, "%s: ", prompt)
	}
	line, _ := wi.r.ReadString('\n')
	line = strings.TrimSpace(line)
	if line == "" {
		return def
	}
	return line
}

func (wi *wizardReader) askInt(prompt string, def int) int {
	s := wi.ask(prompt, strconv.Itoa(def))
	v, err := strconv.Atoi(s)
	if err != nil {
		return def
	}
	return v
}

func (wi *wizardReader) askFloat(prompt string, def float64) float64 {
	s := wi.ask(prompt, strconv.FormatFloat(def, 'f', -1, 64))
	v, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return def
	}
	return v
}

func (wi *wizardReader) askBool(prompt string, def bool) bool {
	defStr := "n"
	if def {
		defStr = "s"
	}
	s := wi.ask(prompt+" (s/n)", defStr)
	return strings.ToLower(strings.TrimSpace(s)) == "s"
}

type initCamera struct {
	id       string
	rtspURL  string
	hasAudio string // "true", "false", "" (auto-detect)
	motion   *initMotion
}

type initMotion struct {
	enabled  bool
	threshold float64
	fps      int
	cooldown int
}

func initWizard(input io.Reader, output io.Writer) (string, error) {
	wi := &wizardReader{r: bufio.NewReader(input), w: output}

	fmt.Fprintln(output, "\n=== camera init — gerador de configuração ===")

	fmt.Fprintln(output, "\n--- Servidor ---")
	port := wi.askInt("Porta HTTP", 8080)
	username := wi.ask("Usuário", "master")
	password := wi.ask("Senha", "")
	segmentsPath := wi.ask("Path dos segmentos HLS", "/tmp/hls")
	hlsDVR := wi.askInt("Segundos de janela DVR (0 = desabilitado)", 0)

	fmt.Fprintln(output, "\n--- Storage ---")
	storagePath := wi.ask("Path de gravações", "/data/recordings")
	retentionMin := wi.askInt("Retenção em minutos (0 = desabilitado; 43200 = 30 dias)", 43200)
	maxSizeGB := wi.askFloat("Tamanho máximo em GB (0 = desabilitado)", 10)
	warnPercent := wi.askFloat("Aviso de uso em %", 70)

	fmt.Fprintln(output, "\n--- Geral ---")
	timezone := wi.ask("Fuso horário", "America/Sao_Paulo")

	fmt.Fprintln(output, "\n--- Detecção de movimento (padrão global) ---")
	motionEnabled := wi.askBool("Habilitar por padrão", false)
	motionThreshold := wi.askFloat("Threshold (0.0–1.0)", 0.02)
	motionFPS := wi.askInt("FPS de análise", 2)
	motionCooldown := wi.askInt("Cooldown entre eventos em segundos (0 = desabilitado)", 30)

	var cameras []initCamera
	fmt.Fprintln(output, "\n--- Câmeras (ID vazio para encerrar) ---")
	for {
		fmt.Fprintln(output)
		id := wi.ask("ID da câmera", "")
		if id == "" {
			break
		}
		rtsp := wi.ask("URL RTSP", "")
		if rtsp == "" {
			fmt.Fprintln(output, "URL RTSP não pode ser vazia. Câmera ignorada.")
			continue
		}

		audioStr := wi.ask("Áudio (s/n/auto)", "auto")
		var hasAudio string
		switch strings.ToLower(audioStr) {
		case "s", "sim":
			hasAudio = "true"
		case "n", "não", "nao":
			hasAudio = "false"
		}

		cam := initCamera{id: id, rtspURL: rtsp, hasAudio: hasAudio}
		motionOpt := wi.ask("Motion (s/n/global)", "global")
		switch strings.ToLower(motionOpt) {
		case "s", "sim":
			m := &initMotion{enabled: true}
			m.threshold = wi.askFloat("  Threshold", motionThreshold)
			m.fps = wi.askInt("  FPS", motionFPS)
			m.cooldown = wi.askInt("  Cooldown (s)", motionCooldown)
			cam.motion = m
		case "n", "não", "nao":
			cam.motion = &initMotion{enabled: false}
		}
		cameras = append(cameras, cam)
	}

	if len(cameras) == 0 {
		return "", fmt.Errorf("nenhuma câmera configurada")
	}

	return buildInitYAML(initParams{
		port: port, username: username, password: password,
		segmentsPath: segmentsPath, hlsDVR: hlsDVR,
		storagePath: storagePath, retentionMin: retentionMin,
		maxSizeGB: maxSizeGB, warnPercent: warnPercent,
		timezone:        timezone,
		motionEnabled:   motionEnabled,
		motionThreshold: motionThreshold,
		motionFPS:       motionFPS,
		motionCooldown:  motionCooldown,
		cameras:         cameras,
	}), nil
}

type initParams struct {
	port         int
	username     string
	password     string
	segmentsPath string
	hlsDVR       int
	storagePath  string
	retentionMin int
	maxSizeGB    float64
	warnPercent  float64
	timezone     string

	motionEnabled   bool
	motionThreshold float64
	motionFPS       int
	motionCooldown  int

	cameras []initCamera
}

func buildInitYAML(p initParams) string {
	var sb strings.Builder

	motionEnabledStr := "false"
	if p.motionEnabled {
		motionEnabledStr = "true"
	}

	fmt.Fprintf(&sb, "debug: false\n")
	fmt.Fprintf(&sb, "timezone: %s\n", p.timezone)
	fmt.Fprintf(&sb, "\nlog:\n  output: stdout\n  path:\n")
	fmt.Fprintf(&sb, "\nserver:\n")
	fmt.Fprintf(&sb, "  port: %d\n", p.port)
	fmt.Fprintf(&sb, "  segments_path: %s\n", p.segmentsPath)
	fmt.Fprintf(&sb, "  username: %s\n", p.username)
	fmt.Fprintf(&sb, "  password: %s\n", yamlStringValue(p.password))
	fmt.Fprintf(&sb, "  hls_dvr_seconds: %d\n", p.hlsDVR)
	fmt.Fprintf(&sb, "\nstorage:\n")
	fmt.Fprintf(&sb, "  path: %s\n", p.storagePath)
	fmt.Fprintf(&sb, "  retention_minutes: %d\n", p.retentionMin)
	fmt.Fprintf(&sb, "  interval_minutes: 60\n")
	fmt.Fprintf(&sb, "  max_size_gb: %s\n", yamlFloat(p.maxSizeGB))
	fmt.Fprintf(&sb, "  warn_percent: %s\n", yamlFloat(p.warnPercent))
	fmt.Fprintf(&sb, "\ndefaults:\n  chunk_duration: 5m\n  reconnect_interval: 10s\n")
	fmt.Fprintf(&sb, "\nmotion:\n")
	fmt.Fprintf(&sb, "  enabled: %s\n", motionEnabledStr)
	fmt.Fprintf(&sb, "  threshold: %s\n", yamlFloat(p.motionThreshold))
	fmt.Fprintf(&sb, "  fps: %d\n", p.motionFPS)
	fmt.Fprintf(&sb, "  cooldown_seconds: %d\n", p.motionCooldown)
	fmt.Fprintf(&sb, "\ncameras:\n")

	for _, cam := range p.cameras {
		fmt.Fprintf(&sb, "  - id: %s\n", cam.id)
		fmt.Fprintf(&sb, "    rtsp_url: %s\n", cam.rtspURL)
		if cam.hasAudio != "" {
			fmt.Fprintf(&sb, "    has_audio: %s\n", cam.hasAudio)
		}
		if cam.motion != nil {
			fmt.Fprintf(&sb, "    motion:\n")
			fmt.Fprintf(&sb, "      enabled: %v\n", cam.motion.enabled)
			if cam.motion.enabled {
				fmt.Fprintf(&sb, "      threshold: %s\n", yamlFloat(cam.motion.threshold))
				fmt.Fprintf(&sb, "      fps: %d\n", cam.motion.fps)
				fmt.Fprintf(&sb, "      cooldown_seconds: %d\n", cam.motion.cooldown)
			}
		}
	}

	return sb.String()
}

// yamlStringValue quotes a string value if it contains YAML special characters.
func yamlStringValue(s string) string {
	if s == "" {
		return `""`
	}
	if strings.ContainsAny(s, `:#{}[]|>&*!,'"`) {
		s = strings.ReplaceAll(s, `\`, `\\`)
		s = strings.ReplaceAll(s, `"`, `\"`)
		return `"` + s + `"`
	}
	return s
}

// yamlFloat formats a float without trailing zeros, ensuring at least one decimal place.
func yamlFloat(f float64) string {
	s := strconv.FormatFloat(f, 'f', -1, 64)
	if !strings.Contains(s, ".") {
		s += ".0"
	}
	return s
}
