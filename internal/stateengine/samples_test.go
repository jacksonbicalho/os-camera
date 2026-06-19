package stateengine

import (
	"encoding/base64"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSaveSamples(t *testing.T) {
	dir := t.TempDir()
	jpeg := base64.StdEncoding.EncodeToString([]byte("fake-jpeg-bytes"))
	imgs := []LabeledImage{
		{Label: "aberto", DataB64: jpeg},
		{Label: "fechado", DataB64: "data:image/jpeg;base64," + jpeg}, // com prefixo data:
		{Label: "aberto", DataB64: jpeg},
	}
	got, err := SaveSamples(dir, 42, imgs)
	if err != nil {
		t.Fatalf("SaveSamples: %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("esperava 3 amostras, got %d", len(got))
	}
	// pastas por classe criadas, com os arquivos
	for _, label := range []string{"aberto", "fechado"} {
		d := filepath.Join(dir, "state_samples", "42", label)
		if _, err := os.Stat(d); err != nil {
			t.Fatalf("pasta da classe %q não existe: %v", label, err)
		}
	}
	abertos, _ := os.ReadDir(filepath.Join(dir, "state_samples", "42", "aberto"))
	if len(abertos) != 2 {
		t.Fatalf("esperava 2 amostras em 'aberto', got %d", len(abertos))
	}
	if got[0].Label != "aberto" || got[1].Label != "fechado" {
		t.Fatalf("labels errados: %+v", got)
	}
}

func TestListSamples(t *testing.T) {
	dir := t.TempDir()
	if _, err := ListSamples(dir, 9); err != nil {
		t.Fatalf("ListSamples vazio: %v", err)
	}
	jpeg := base64.StdEncoding.EncodeToString([]byte("x"))
	if _, err := SaveSamples(dir, 9, []LabeledImage{
		{Label: "aberto", DataB64: jpeg},
		{Label: "aberto", DataB64: jpeg},
		{Label: "fechado", DataB64: jpeg},
	}); err != nil {
		t.Fatal(err)
	}
	m, err := ListSamples(dir, 9)
	if err != nil {
		t.Fatal(err)
	}
	if len(m["aberto"]) != 2 || len(m["fechado"]) != 1 {
		t.Fatalf("ListSamples: %+v", m)
	}
	if m["aberto"][0] != "/recordings/state_samples/9/aberto/0.jpg" {
		t.Fatalf("url errada: %q", m["aberto"][0])
	}
}

func TestSaveSamplesRejectsBadBase64(t *testing.T) {
	if _, err := SaveSamples(t.TempDir(), 1, []LabeledImage{{Label: "x", DataB64: "!!!notbase64"}}); err == nil {
		t.Fatal("esperava erro de base64 inválido")
	}
}

func TestSaveSamplesNamesFileAfterFrame(t *testing.T) {
	dir := t.TempDir()
	jpeg := base64.StdEncoding.EncodeToString([]byte("x"))
	out, err := SaveSamples(dir, 7, []LabeledImage{
		{Label: "aberto", DataB64: jpeg, Frame: "20260618170515_motion.jpg"},
		{Label: "aberto", DataB64: jpeg}, // sem frame → nome por índice
	})
	if err != nil {
		t.Fatal(err)
	}
	// o arquivo com frame é nomeado pelo frame de origem
	if _, err := os.Stat(filepath.Join(dir, "state_samples", "7", "aberto", "20260618170515_motion.jpg")); err != nil {
		t.Fatalf("arquivo nomeado pelo frame não existe: %v", err)
	}
	if out[0].ImagePath == "" || out[1].ImagePath == "" {
		t.Fatalf("paths vazios: %+v", out)
	}
	// ListSamples expõe a URL com o nome do frame (o front recupera o vínculo daí)
	m, err := ListSamples(dir, 7)
	if err != nil {
		t.Fatal(err)
	}
	var hasFrameName bool
	for _, u := range m["aberto"] {
		if strings.HasSuffix(u, "/20260618170515_motion.jpg") {
			hasFrameName = true
		}
	}
	if !hasFrameName {
		t.Fatalf("ListSamples não trouxe a URL com o frame: %+v", m["aberto"])
	}
}
