// Package webcam expõe webcams locais (v4l2) como fontes RTSP: enumera os
// dispositivos via /sys/class/video4linux, hospeda um servidor RTSP embutido
// (gortsplib) e supervisiona um ffmpeg por webcam que lê o /dev/videoN e publica
// no servidor. Os 3 processos do pipeline (recorder/streamer/motion) consomem o
// RTSP local como qualquer câmera. Self-contained: a única dependência de
// runtime é o ffmpeg (já exigido pelo projeto).
package webcam

import (
	"fmt"
	"io/fs"
	"sort"
	"strconv"
	"strings"
)

// DefaultRTSPAddress é onde o servidor RTSP embutido escuta (loopback).
const DefaultRTSPAddress = "127.0.0.1:8554"

// SysfsRoot é o diretório do v4l2 no sysfs (injetável nos testes).
const SysfsRoot = "/sys/class/video4linux"

// Device é uma webcam local detectada.
type Device struct {
	Index    int    // N em videoN
	Path     string // /dev/videoN
	Name     string // nome amigável (de /sys/.../name)
	RTSPName string // path no servidor RTSP embutido (webcamN)
}

// List enumera as webcams locais lendo `<root>/videoN/name`. Uma webcam física
// costuma expor vários /dev/videoN com o mesmo nome — fica só o de menor índice
// (nó de captura). `root` é um fs rooted em /sys/class/video4linux (em produção
// os.DirFS(SysfsRoot)); fora do Linux/sem o diretório, retorna nil.
func List(root fs.FS) []Device {
	entries, err := fs.ReadDir(root, ".")
	if err != nil {
		return nil
	}
	byName := map[string]Device{}
	for _, e := range entries {
		name := e.Name()
		if !strings.HasPrefix(name, "video") {
			continue
		}
		idx, err := strconv.Atoi(strings.TrimPrefix(name, "video"))
		if err != nil {
			continue
		}
		friendly := name
		if b, err := fs.ReadFile(root, name+"/name"); err == nil {
			if s := strings.TrimSpace(string(b)); s != "" {
				friendly = s
			}
		}
		dev := Device{
			Index:    idx,
			Path:     "/dev/" + name,
			Name:     friendly,
			RTSPName: fmt.Sprintf("webcam%d", idx),
		}
		if existing, ok := byName[friendly]; !ok || idx < existing.Index {
			byName[friendly] = dev
		}
	}
	out := make([]Device, 0, len(byName))
	for _, d := range byName {
		out = append(out, d)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Index < out[j].Index })
	return out
}

// RTSPURL monta a URL servida pelo servidor RTSP embutido para um path.
func RTSPURL(addr, rtspName string) string {
	return "rtsp://" + addr + "/" + rtspName
}

// Detected enumera as webcams locais no sysfs real (vazio fora do Linux/sem o
// diretório). Usado pelo discovery.
func Detected() []Device {
	return List(sysfsRoot())
}
