package updater

import (
	"fmt"
	"io"
	"os"
)

// Replace troca o binário em targetPath pelo de srcPath, guardando antes uma
// cópia do binário atual em backupPath (para rollback). O caller deve garantir
// que srcPath está no mesmo filesystem de targetPath (rename atômico). No Linux,
// renomear sobre um executável em uso é seguro — o inode antigo permanece válido
// para o processo já em execução.
func Replace(srcPath, targetPath, backupPath string) error {
	if err := copyFile(targetPath, backupPath); err != nil {
		return fmt.Errorf("backup do binário atual: %w", err)
	}
	if err := os.Rename(srcPath, targetPath); err != nil {
		return fmt.Errorf("trocar binário: %w", err)
	}
	return nil
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	info, err := in.Stat()
	if err != nil {
		return err
	}
	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, info.Mode().Perm())
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	if err := out.Sync(); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}
