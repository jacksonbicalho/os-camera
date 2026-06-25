//go:build unix

package updater

import (
	"os"
	"syscall"
)

// ReexecSelf substitui o processo atual pelo binário em target, preservando os
// argumentos (ex.: --config) e o ambiente. Em sucesso não retorna.
func ReexecSelf(target string) error {
	return syscall.Exec(target, os.Args, os.Environ())
}
