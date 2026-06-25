package updater

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"camera/internal/release"
)

// AssetForArch devolve o binário do manifesto correspondente à arquitetura em
// execução (chave "linux-<GOARCH>"), e se ele existe.
func AssetForArch(m release.Manifest) (release.Asset, bool) {
	a, ok := m.Assets["linux-"+runtime.GOARCH]
	return a, ok
}

// AssetURL compõe a URL de download de um asset a partir da base da release.
func AssetURL(base, name string) string {
	return base + name
}

// Download baixa url para destPath validando o sha256 esperado (hex). Escreve
// num arquivo temporário no mesmo diretório (para garantir rename atômico no
// mesmo filesystem); em mismatch de hash ou status != 200, descarta e erra.
// client nil usa um http.Client com timeout de 60s.
func Download(ctx context.Context, client *http.Client, url, wantSHA256, destPath string) error {
	if client == nil {
		client = &http.Client{Timeout: 60 * time.Second}
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("montar request: %w", err)
	}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("baixar binário: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download: status %d", resp.StatusCode)
	}

	tmp, err := os.CreateTemp(filepath.Dir(destPath), ".camera_dl_*")
	if err != nil {
		return fmt.Errorf("criar temp: %w", err)
	}
	tmpName := tmp.Name()
	cleanup := func() { tmp.Close(); os.Remove(tmpName) }

	h := sha256.New()
	if _, err := io.Copy(tmp, io.TeeReader(resp.Body, h)); err != nil {
		cleanup()
		return fmt.Errorf("escrever download: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		cleanup()
		return fmt.Errorf("sync: %w", err)
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpName)
		return fmt.Errorf("fechar temp: %w", err)
	}

	got := hex.EncodeToString(h.Sum(nil))
	if got != wantSHA256 {
		os.Remove(tmpName)
		return fmt.Errorf("sha256 não confere: esperado %s, obtido %s", wantSHA256, got)
	}

	if err := os.Chmod(tmpName, 0o755); err != nil {
		os.Remove(tmpName)
		return fmt.Errorf("chmod: %w", err)
	}
	if err := os.Rename(tmpName, destPath); err != nil {
		os.Remove(tmpName)
		return fmt.Errorf("mover para destino: %w", err)
	}
	return nil
}
