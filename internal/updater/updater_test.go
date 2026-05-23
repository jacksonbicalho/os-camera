package updater_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"camera/internal/updater"
)

func TestCheckLatest_UpdateAvailable(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{
			"tag_name": "v1.2.0-beta.1",
			"html_url": "https://github.com/example/camera/releases/tag/v1.2.0-beta.1",
			"assets": []map[string]any{
				{"name": "camera-linux-amd64", "browser_download_url": "https://example.com/camera-linux-amd64"},
				{"name": "checksums.txt", "browser_download_url": "https://example.com/checksums.txt"},
			},
		})
	}))
	defer srv.Close()

	info, err := updater.CheckLatest("v1.0.0", srv.URL)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !info.UpdateAvailable {
		t.Error("expected update_available=true")
	}
	if info.Latest != "v1.2.0-beta.1" {
		t.Errorf("expected latest=v1.2.0-beta.1, got %s", info.Latest)
	}
}

func TestCheckLatest_NoUpdate(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{
			"tag_name": "v1.0.0",
			"html_url": "https://github.com/example/camera/releases/tag/v1.0.0",
			"assets":   []map[string]any{},
		})
	}))
	defer srv.Close()

	info, err := updater.CheckLatest("v1.0.0", srv.URL)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if info.UpdateAvailable {
		t.Error("expected update_available=false")
	}
}

func TestCheckLatest_DevVersion(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{
			"tag_name": "v1.0.0",
			"html_url": "https://example.com",
			"assets":   []map[string]any{},
		})
	}))
	defer srv.Close()

	// versão dev não deve sugerir atualização
	info, err := updater.CheckLatest("dev", srv.URL)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if info.UpdateAvailable {
		t.Error("dev version should not report update available")
	}
}

func TestCheckLatest_AssetURL(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{
			"tag_name": "v2.0.0",
			"html_url": "https://example.com",
			"assets": []map[string]any{
				{"name": "camera-linux-amd64", "browser_download_url": "https://example.com/camera-linux-amd64"},
				{"name": "camera-linux-arm64", "browser_download_url": "https://example.com/camera-linux-arm64"},
				{"name": "checksums.txt", "browser_download_url": "https://example.com/checksums.txt"},
			},
		})
	}))
	defer srv.Close()

	info, err := updater.CheckLatest("v1.0.0", srv.URL)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if info.ChecksumsURL == "" {
		t.Error("expected checksums_url to be set")
	}
	if info.Assets["amd64"] == "" {
		t.Error("expected amd64 asset url to be set")
	}
	if info.Assets["arm64"] == "" {
		t.Error("expected arm64 asset url to be set")
	}
}

func TestIsDocker(t *testing.T) {
	// apenas verifica que a função existe e retorna bool — sem mockar o filesystem
	_ = updater.IsDocker()
}
