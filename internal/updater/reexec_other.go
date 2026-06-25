//go:build !unix

package updater

import "errors"

// ReexecSelf não é suportado fora de sistemas unix (não há syscall.Exec).
func ReexecSelf(target string) error {
	return errors.New("re-exec não suportado nesta plataforma")
}
