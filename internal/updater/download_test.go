package updater

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"camera/internal/release"
)

func TestAssetForArch(t *testing.T) {
	m := release.Manifest{Assets: map[string]release.Asset{
		"linux-" + runtime.GOARCH: {Name: "camera-linux-" + runtime.GOARCH, SHA256: "abc"},
		"linux-outroarch":         {Name: "camera-linux-outroarch", SHA256: "def"},
	}}
	a, ok := AssetForArch(m)
	if !ok {
		t.Fatal("esperava achar asset da arch atual")
	}
	if a.Name != "camera-linux-"+runtime.GOARCH {
		t.Errorf("Name = %q", a.Name)
	}

	empty := release.Manifest{Assets: map[string]release.Asset{"linux-naoexiste": {}}}
	if _, ok := AssetForArch(empty); ok {
		t.Error("não deveria achar asset")
	}
}

func TestAssetURL(t *testing.T) {
	got := AssetURL("https://host/dl/", "camera-linux-arm64")
	if got != "https://host/dl/camera-linux-arm64" {
		t.Errorf("AssetURL = %q", got)
	}
}

func sha256hex(b []byte) string {
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:])
}

func TestDownloadSuccess(t *testing.T) {
	payload := []byte("binário falso\x00\x01\x02")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write(payload)
	}))
	defer srv.Close()

	dest := filepath.Join(t.TempDir(), "camera")
	if err := Download(context.Background(), srv.Client(), srv.URL, sha256hex(payload), dest); err != nil {
		t.Fatalf("Download: %v", err)
	}

	got, err := os.ReadFile(dest)
	if err != nil {
		t.Fatalf("ler dest: %v", err)
	}
	if string(got) != string(payload) {
		t.Error("conteúdo divergente")
	}
	info, _ := os.Stat(dest)
	if info.Mode().Perm()&0o100 == 0 {
		t.Errorf("binário sem bit de execução: %v", info.Mode())
	}
}

func TestDownloadBadSHA(t *testing.T) {
	payload := []byte("conteudo")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write(payload)
	}))
	defer srv.Close()

	dest := filepath.Join(t.TempDir(), "camera")
	err := Download(context.Background(), srv.Client(), srv.URL, sha256hex([]byte("outra coisa")), dest)
	if err == nil {
		t.Fatal("esperava erro de sha mismatch")
	}
	if _, statErr := os.Stat(dest); !os.IsNotExist(statErr) {
		t.Error("não deveria deixar arquivo após mismatch")
	}
}

func TestDownloadBadStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "nope", http.StatusNotFound)
	}))
	defer srv.Close()

	dest := filepath.Join(t.TempDir(), "camera")
	if err := Download(context.Background(), srv.Client(), srv.URL, "00", dest); err == nil {
		t.Fatal("esperava erro em status 404")
	}
}
