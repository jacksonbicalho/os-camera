// Package updater detecta como o sistema pode se atualizar no ambiente em que
// roda e (nas próximas fatias) aplica a atualização. A detecção é por
// capacidade — não por adivinhar o ambiente: o default é self-replace, Docker
// é a exceção (troca de imagem efêmera) e a ausência de escrita vira notify.
package updater

import (
	"os"
	"path/filepath"
	"strings"
)

// Modos de aplicação da atualização.
const (
	// ApplySelfReplace: troca o binário no disco e faz re-exec (binário, proot, bare).
	ApplySelfReplace = "self-replace"
	// ApplyDocker: a troca acontece via pull+recreate da imagem (Watchtower).
	ApplyDocker = "docker"
	// ApplyNotify: o sistema não pode aplicar sozinho; só avisa com instruções.
	ApplyNotify = "notify"
)

// Environment descreve o ambiente de execução para fins de atualização.
type Environment struct {
	InDocker       bool   `json:"in_docker"`
	BinaryWritable bool   `json:"binary_writable"`
	ApplyMode      string `json:"apply_mode"`
}

// Detect inspeciona o ambiente a partir do caminho do executável em execução.
func Detect(execPath string) Environment {
	inDocker := detectDocker()
	writable := dirWritable(filepath.Dir(execPath))
	return Environment{
		InDocker:       inDocker,
		BinaryWritable: writable,
		ApplyMode:      decideApplyMode(inDocker, writable),
	}
}

// decideApplyMode aplica a regra capacidade + exclusão de Docker. Docker tem
// precedência: mesmo com binário gravável, a troca dentro do container seria
// efêmera, então o caminho correto é pull+recreate.
func decideApplyMode(inDocker, writable bool) string {
	switch {
	case inDocker:
		return ApplyDocker
	case writable:
		return ApplySelfReplace
	default:
		return ApplyNotify
	}
}

// dirWritable testa se dá para criar (e remover) um arquivo em dir.
func dirWritable(dir string) bool {
	f, err := os.CreateTemp(dir, ".update_write_check_*")
	if err != nil {
		return false
	}
	name := f.Name()
	f.Close()
	os.Remove(name)
	return true
}

// detectDocker classifica como Docker quando há marcadores de container. proot
// não tem nenhum desses, então não é confundido com Docker.
func detectDocker() bool {
	if _, err := os.Stat("/.dockerenv"); err == nil {
		return true
	}
	for _, p := range []string{"/proc/1/cgroup", "/proc/self/cgroup"} {
		if data, err := os.ReadFile(p); err == nil && cgroupIndicatesContainer(string(data)) {
			return true
		}
	}
	return false
}

// cgroupIndicatesContainer reporta se o conteúdo de um arquivo cgroup aponta
// para um runtime de container.
func cgroupIndicatesContainer(content string) bool {
	for _, marker := range []string{"docker", "containerd", "kubepods"} {
		if strings.Contains(content, marker) {
			return true
		}
	}
	return false
}
