// Package dbbackup fornece snapshot/restore do banco SQLite (arquivo único),
// usado como rede de segurança para reverter o estado quando uma atualização
// (que pode aplicar migrations forward-only) der errado.
package dbbackup

import (
	"database/sql"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

const (
	prefix = "backup-"
	suffix = ".db"
)

// Snapshot gera um snapshot consistente de dbPath em
// destDir/backup-<UTC timestamp>-<label>.db via VACUUM INTO e retorna o caminho.
// label (ex.: a versão atual) pareia o snapshot com o binário que o gerou.
func Snapshot(dbPath, destDir, label string) (string, error) {
	if err := os.MkdirAll(destDir, 0o755); err != nil {
		return "", fmt.Errorf("criar dir de backup: %w", err)
	}

	ts := time.Now().UTC().Format("20060102150405")
	name := prefix + ts
	if s := sanitize(label); s != "" {
		name += "-" + s
	}
	name += suffix
	dest := filepath.Join(destDir, name)

	d, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return "", fmt.Errorf("abrir db: %w", err)
	}
	defer d.Close()

	// VACUUM INTO escreve um snapshot consistente do estado committed.
	if _, err := d.Exec("VACUUM INTO ?", dest); err != nil {
		return "", fmt.Errorf("vacuum into: %w", err)
	}
	return dest, nil
}

// Prune mantém os keep snapshots mais recentes em destDir e remove os demais.
// A ordenação é por nome (o timestamp no nome ordena cronologicamente).
func Prune(destDir string, keep int) error {
	entries, err := os.ReadDir(destDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("ler dir de backup: %w", err)
	}

	var backups []string
	for _, e := range entries {
		n := e.Name()
		if !e.IsDir() && strings.HasPrefix(n, prefix) && strings.HasSuffix(n, suffix) {
			backups = append(backups, n)
		}
	}
	sort.Strings(backups) // mais antigo → mais novo

	if len(backups) <= keep {
		return nil
	}
	for _, n := range backups[:len(backups)-keep] {
		if err := os.Remove(filepath.Join(destDir, n)); err != nil {
			return fmt.Errorf("remover snapshot %s: %w", n, err)
		}
	}
	return nil
}

// Restore copia snapshotPath sobre dbPath e remove os sidecars -wal/-shm (senão
// o SQLite reaplicaria um WAL antigo sobre o arquivo restaurado). Pressupõe que
// o banco NÃO está aberto (uso durante rollback, com o servidor reiniciando).
func Restore(snapshotPath, dbPath string) error {
	if err := copyFile(snapshotPath, dbPath); err != nil {
		return fmt.Errorf("copiar snapshot: %w", err)
	}
	for _, side := range []string{dbPath + "-wal", dbPath + "-shm"} {
		if err := os.Remove(side); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("remover sidecar %s: %w", side, err)
		}
	}
	return nil
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	tmp := dst + ".restore-tmp"
	out, err := os.Create(tmp)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		os.Remove(tmp)
		return err
	}
	if err := out.Sync(); err != nil {
		out.Close()
		os.Remove(tmp)
		return err
	}
	if err := out.Close(); err != nil {
		os.Remove(tmp)
		return err
	}
	// rename atômico sobre o destino.
	return os.Rename(tmp, dst)
}

// sanitize reduz o label a um token seguro para nome de arquivo.
func sanitize(s string) string {
	var b strings.Builder
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '.', r == '-', r == '_':
			b.WriteRune(r)
		default:
			b.WriteRune('_')
		}
	}
	return b.String()
}
