package stateengine

import (
	"os"
	"path/filepath"
)

// MigrateSampleDirsToSlug renomeia (idempotente, best-effort) os diretórios de
// classe de state_samples e state_train para o slug — elimina espaços/acentos nos
// nomes de pasta sem perder as imagens (só renomeia). Roda no boot.
func MigrateSampleDirsToSlug(storagePath string) {
	for _, root := range []string{"state_samples", "state_train"} {
		cids, err := os.ReadDir(filepath.Join(storagePath, root))
		if err != nil {
			continue
		}
		for _, cid := range cids {
			if !cid.IsDir() {
				continue
			}
			cidDir := filepath.Join(storagePath, root, cid.Name())
			classes, err := os.ReadDir(cidDir)
			if err != nil {
				continue
			}
			for _, cl := range classes {
				if !cl.IsDir() {
					continue
				}
				slug := Slug(cl.Name())
				if slug == "" || slug == cl.Name() {
					continue
				}
				src := filepath.Join(cidDir, cl.Name())
				dst := filepath.Join(cidDir, slug)
				if _, err := os.Stat(dst); err == nil {
					mergeDir(src, dst) // destino já existe: move os arquivos
					continue
				}
				os.Rename(src, dst)
			}
		}
	}
}

func mergeDir(src, dst string) {
	files, err := os.ReadDir(src)
	if err != nil {
		return
	}
	for _, f := range files {
		os.Rename(filepath.Join(src, f.Name()), filepath.Join(dst, f.Name()))
	}
	os.Remove(src)
}
