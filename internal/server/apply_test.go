package server_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"camera/internal/config"
	"camera/internal/release"
	"camera/internal/server"
)

type checkerWithManifest struct {
	st  release.Status
	man release.Manifest
	ok  bool
}

func (c checkerWithManifest) Status() release.Status             { return c.st }
func (c checkerWithManifest) Manifest() (release.Manifest, bool) { return c.man, c.ok }

type fakeApplier struct{ called chan release.Manifest }

func (f fakeApplier) Apply(ctx context.Context, m release.Manifest) error {
	f.called <- m
	return nil
}

func applyTestServer(t *testing.T, mode string, available bool) (http.Handler, string, chan release.Manifest) {
	t.Helper()
	srv := server.NewServer(config.ServerConfig{}, "UTC", nil, discardLogger(), nil)
	srv = withTestUsersAndCameras(t, srv, nil)
	srv.WithApplyMode(mode)
	srv.WithUpdateChecker(checkerWithManifest{
		st:  release.Status{Current: "v1.3.0-dev", Latest: "v1.4.0-dev", UpdateAvailable: available},
		man: release.Manifest{Latest: "v1.4.0-dev"},
		ok:  true,
	})
	called := make(chan release.Manifest, 1)
	srv.WithApplier(fakeApplier{called: called})
	token := loginAndGetToken(t, srv, "admin", "pw")
	return srv, token, called
}

func TestApplyUpdate_SelfReplace(t *testing.T) {
	srv, token, called := applyTestServer(t, "self-replace", true)

	req := httptest.NewRequest(http.MethodPost, "/api/updates/apply", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusAccepted {
		t.Fatalf("status = %d: %s", w.Code, w.Body.String())
	}
	select {
	case m := <-called:
		if m.Latest != "v1.4.0-dev" {
			t.Errorf("manifesto aplicado = %q", m.Latest)
		}
	case <-time.After(time.Second):
		t.Error("Apply não foi chamado")
	}
}

func TestApplyUpdate_DockerRejected(t *testing.T) {
	srv, token, called := applyTestServer(t, "docker", true)

	req := httptest.NewRequest(http.MethodPost, "/api/updates/apply", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusConflict {
		t.Fatalf("status = %d, quero 409", w.Code)
	}
	select {
	case <-called:
		t.Error("Apply não deveria ser chamado no modo docker")
	case <-time.After(100 * time.Millisecond):
	}
}

func TestApplyUpdate_NoUpdate(t *testing.T) {
	srv, token, called := applyTestServer(t, "self-replace", false)

	req := httptest.NewRequest(http.MethodPost, "/api/updates/apply", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusConflict {
		t.Fatalf("status = %d, quero 409", w.Code)
	}
	select {
	case <-called:
		t.Error("Apply não deveria ser chamado sem update")
	case <-time.After(100 * time.Millisecond):
	}
}
