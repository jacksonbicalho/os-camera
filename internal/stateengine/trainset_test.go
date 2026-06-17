package stateengine

import (
	"bytes"
	"image"
	"image/color"
	"image/jpeg"
	"os"
	"path/filepath"
	"testing"

	"camera/internal/stateclass"
)

// writeJPEG grava um JPEG sólido wxh em path (cria os diretórios).
func writeJPEG(t *testing.T, path string, w, h int) {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, color.RGBA{R: 200, G: 100, B: 50, A: 255})
		}
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if err := jpeg.Encode(f, img, nil); err != nil {
		t.Fatal(err)
	}
}

func TestBuildTrainSetFromSamples(t *testing.T) {
	dir := t.TempDir()
	cid := int64(42)
	// frames inteiros 100x100 persistidos por classe
	writeJPEG(t, filepath.Join(dir, "state_samples", "42", "aberto", "0.jpg"), 100, 100)
	writeJPEG(t, filepath.Join(dir, "state_samples", "42", "aberto", "1.jpg"), 100, 100)
	writeJPEG(t, filepath.Join(dir, "state_samples", "42", "fechado", "0.jpg"), 100, 100)

	c := stateclass.Classifier{ID: cid, CropX: 0.25, CropY: 0.25, CropW: 0.5, CropH: 0.5}
	got, err := BuildTrainSetFromSamples(dir, c)
	if err != nil {
		t.Fatalf("BuildTrainSetFromSamples: %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("esperava 3 amostras de treino, got %d (%+v)", len(got), got)
	}

	labels := map[string]int{}
	for _, s := range got {
		labels[s.Label]++
	}
	if labels["aberto"] != 2 || labels["fechado"] != 1 {
		t.Fatalf("labels do train set errados: %+v", labels)
	}

	// crops gravados em state_train/{cid}/{classe} e são JPEGs ~50x50 (crop 0.5 de 100)
	trainDir := filepath.Join(dir, "state_train", "42")
	abertos, err := os.ReadDir(filepath.Join(trainDir, "aberto"))
	if err != nil || len(abertos) != 2 {
		t.Fatalf("esperava 2 crops em aberto: %v (%d)", err, len(abertos))
	}
	data, err := os.ReadFile(filepath.Join(trainDir, "aberto", "0.jpg"))
	if err != nil {
		t.Fatal(err)
	}
	cfg, err := jpeg.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("crop não é JPEG válido: %v", err)
	}
	if cfg.Width != 50 || cfg.Height != 50 {
		t.Fatalf("crop com dimensão errada: %dx%d (esperava 50x50)", cfg.Width, cfg.Height)
	}
}

func TestBuildTrainSetFromSamplesEmpty(t *testing.T) {
	got, err := BuildTrainSetFromSamples(t.TempDir(), stateclass.Classifier{ID: 7, CropW: 0.5, CropH: 0.5})
	if err != nil {
		t.Fatalf("sem amostras deve ser ok (lista vazia): %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("esperava lista vazia, got %d", len(got))
	}
}
