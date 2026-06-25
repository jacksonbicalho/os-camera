package updater

import (
	"context"
	"errors"
	"path/filepath"
	"runtime"
	"testing"

	"camera/internal/release"
)

func TestMarkerRoundTrip(t *testing.T) {
	dir := t.TempDir()
	if _, ok, _ := ReadMarker(dir); ok {
		t.Fatal("não deveria existir marcador ainda")
	}

	m := Marker{State: "pending", Attempts: 0, Target: "/x/camera", BackupBinary: "/x/camera.old",
		DBSnapshot: "/x/snap.db", DBPath: "/x/camera.db", FromVersion: "v1.3", ToVersion: "v1.4"}
	if err := WriteMarker(dir, m); err != nil {
		t.Fatalf("WriteMarker: %v", err)
	}

	got, ok, err := ReadMarker(dir)
	if err != nil || !ok {
		t.Fatalf("ReadMarker: ok=%v err=%v", ok, err)
	}
	if got != m {
		t.Errorf("marcador divergente: %+v != %+v", got, m)
	}

	if err := ClearMarker(dir); err != nil {
		t.Fatalf("ClearMarker: %v", err)
	}
	if _, ok, _ := ReadMarker(dir); ok {
		t.Error("marcador deveria ter sido removido")
	}
}

func manifest() release.Manifest {
	return release.Manifest{
		Latest: "v1.4.0-dev",
		Assets: map[string]release.Asset{
			"linux-" + runtime.GOARCH: {Name: "camera-linux-" + runtime.GOARCH, SHA256: "deadbeef"},
		},
	}
}

func newTestApplier(dir string, log *[]string) *Applier {
	return &Applier{
		Dir: dir, Target: filepath.Join(dir, "camera"), BaseURL: "https://dl/",
		DBPath: filepath.Join(dir, "camera.db"), CurrentVersion: "v1.3.0-dev",
		Download: func(ctx context.Context, url, sha, dest string) error {
			*log = append(*log, "download:"+url+":"+sha+":"+filepath.Base(dest))
			return nil
		},
		Snapshot: func() (string, error) { *log = append(*log, "snapshot"); return "/snap.db", nil },
		Replace: func(src, target, backup string) error {
			*log = append(*log, "replace:"+filepath.Base(src)+":"+filepath.Base(target)+":"+filepath.Base(backup))
			return nil
		},
		Reexec: func(target string) error { *log = append(*log, "reexec:"+filepath.Base(target)); return nil },
	}
}

func TestApplyOrder(t *testing.T) {
	dir := t.TempDir()
	var log []string
	a := newTestApplier(dir, &log)

	if err := a.Apply(context.Background(), manifest()); err != nil {
		t.Fatalf("Apply: %v", err)
	}

	want := []string{
		"download:https://dl/camera-linux-" + runtime.GOARCH + ":deadbeef:.camera.new",
		"snapshot",
		"replace:.camera.new:camera:camera.old",
		"reexec:camera",
	}
	// Remove a entrada de marcador (não está no log) — confere os passos de efeito.
	if len(log) != len(want) {
		t.Fatalf("passos = %v, quero %v", log, want)
	}
	for i := range want {
		if log[i] != want[i] {
			t.Errorf("passo %d = %q, quero %q", i, log[i], want[i])
		}
	}

	// Marcador gravado após o replace.
	mk, ok, err := ReadMarker(dir)
	if err != nil || !ok {
		t.Fatalf("marcador ausente: ok=%v err=%v", ok, err)
	}
	if mk.State != "pending" || mk.Attempts != 0 {
		t.Errorf("estado/attempts = %q/%d", mk.State, mk.Attempts)
	}
	if mk.DBSnapshot != "/snap.db" || mk.ToVersion != "v1.4.0-dev" || mk.FromVersion != "v1.3.0-dev" {
		t.Errorf("campos do marcador: %+v", mk)
	}
}

func TestApplyNoAsset(t *testing.T) {
	dir := t.TempDir()
	var log []string
	a := newTestApplier(dir, &log)

	bad := release.Manifest{Latest: "v1.4", Assets: map[string]release.Asset{"linux-naoexiste": {}}}
	if err := a.Apply(context.Background(), bad); err == nil {
		t.Fatal("esperava erro sem asset da arch")
	}
	if len(log) != 0 {
		t.Errorf("não deveria ter efeitos: %v", log)
	}
	if _, ok, _ := ReadMarker(dir); ok {
		t.Error("não deveria gravar marcador")
	}
}

func TestApplyDownloadError(t *testing.T) {
	dir := t.TempDir()
	var log []string
	a := newTestApplier(dir, &log)
	a.Download = func(ctx context.Context, url, sha, dest string) error { return errors.New("falhou") }

	if err := a.Apply(context.Background(), manifest()); err == nil {
		t.Fatal("esperava erro do download")
	}
	for _, step := range log {
		if step[:7] == "replace" || step[:6] == "reexec" {
			t.Errorf("não deveria ter trocado/reexecutado: %v", log)
		}
	}
	if _, ok, _ := ReadMarker(dir); ok {
		t.Error("não deveria gravar marcador após falha no download")
	}
}
