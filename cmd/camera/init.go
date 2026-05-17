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

type initParams struct {
	port             int
	dbPath           string
	timezone         string
	segmentsPath     string
	hlsDVR           int
	storagePath      string
	withMotionMin    int
	withoutMotionMin int
	maxSizeGB        float64
	warnPercent      float64
	adminUsername    string
	adminPassword    string
}

func initWizard(input io.Reader, output io.Writer) (string, error) {
	wi := &wizardReader{r: bufio.NewReader(input), w: output}

	fmt.Fprintln(output, "\n=== camera init — gerador de configuração ===")

	fmt.Fprintln(output, "\n--- Servidor ---")
	port := wi.askInt("Porta HTTP", 8080)
	dbPath := wi.ask("Caminho do banco de dados", "/var/camera/data/camera.db")
	segmentsPath := wi.ask("Caminho dos segmentos HLS", "/var/camera/data/hls")
	hlsDVR := wi.askInt("Segundos de janela DVR (0 = desabilitado)", 0)

	fmt.Fprintln(output, "\n--- Storage ---")
	storagePath := wi.ask("Caminho de gravações", "/var/camera/data/recordings")
	withMotionMin := wi.askInt("Retenção COM movimento em minutos (0 = nunca apaga; 10080 = 7 dias)", 10080)
	withoutMotionMin := wi.askInt("Retenção SEM movimento em minutos (0 = nunca apaga; 1440 = 1 dia)", 1440)
	maxSizeGB := wi.askFloat("Tamanho máximo em GB (0 = desabilitado)", 10)
	warnPercent := wi.askFloat("Aviso de uso em %", 70)

	fmt.Fprintln(output, "\n--- Geral ---")
	timezone := wi.ask("Fuso horário", "America/Sao_Paulo")

	fmt.Fprintln(output, "\n--- Administrador inicial ---")
	adminUsername := wi.ask("Usuário administrador", "admin")
	adminPassword := wi.ask("Senha inicial (obrigatório trocar no primeiro login)", "changeme")

	return buildInitYAML(initParams{
		port:             port,
		dbPath:           dbPath,
		timezone:         timezone,
		segmentsPath:     segmentsPath,
		hlsDVR:           hlsDVR,
		storagePath:      storagePath,
		withMotionMin:    withMotionMin,
		withoutMotionMin: withoutMotionMin,
		maxSizeGB:        maxSizeGB,
		warnPercent:      warnPercent,
		adminUsername:    adminUsername,
		adminPassword:    adminPassword,
	}), nil
}

func buildInitYAML(p initParams) string {
	var sb strings.Builder

	fmt.Fprintf(&sb, "debug: false\n")
	fmt.Fprintf(&sb, "timezone: %s\n", p.timezone)
	fmt.Fprintf(&sb, "\ndb_path: %s\n", p.dbPath)
	fmt.Fprintf(&sb, "\nlog:\n  output: stdout\n  path:\n")
	fmt.Fprintf(&sb, "\nserver:\n")
	fmt.Fprintf(&sb, "  port: %d\n", p.port)
	fmt.Fprintf(&sb, "  segments_path: %s\n", p.segmentsPath)
	fmt.Fprintf(&sb, "  hls_dvr_seconds: %d\n", p.hlsDVR)
	fmt.Fprintf(&sb, "  jwt_secret: \"\"\n")
	fmt.Fprintf(&sb, "\nstorage:\n")
	fmt.Fprintf(&sb, "  path: %s\n", p.storagePath)
	fmt.Fprintf(&sb, "  retention:\n")
	fmt.Fprintf(&sb, "    with_motion_minutes: %d\n", p.withMotionMin)
	fmt.Fprintf(&sb, "    without_motion_minutes: %d\n", p.withoutMotionMin)
	fmt.Fprintf(&sb, "  interval_minutes: 60\n")
	fmt.Fprintf(&sb, "  max_size_gb: %s\n", yamlFloat(p.maxSizeGB))
	fmt.Fprintf(&sb, "  warn_percent: %s\n", yamlFloat(p.warnPercent))
	fmt.Fprintf(&sb, "\nadmin:\n")
	fmt.Fprintf(&sb, "  username: %s\n", p.adminUsername)
	fmt.Fprintf(&sb, "  password: %s\n", yamlStringValue(p.adminPassword))

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
