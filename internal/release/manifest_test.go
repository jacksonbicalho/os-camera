package release

import (
	"encoding/json"
	"strings"
	"testing"
)

// checksums.txt no formato do `sha256sum`: "<hash>  <arquivo>" (dois espaços).
const sampleChecksums = `aaaa000000000000000000000000000000000000000000000000000000000001  camera-linux-amd64
bbbb000000000000000000000000000000000000000000000000000000000002  camera-linux-arm64
cccc000000000000000000000000000000000000000000000000000000000003  camera-linux-arm
dddd000000000000000000000000000000000000000000000000000000000004  checksums.txt
`

func TestBuildManifestParsesAssets(t *testing.T) {
	notes := "### ✨ Novidades\n- algo novo (`abc1234`)\n\n**Commits:** \"aspas\" & <tag>"
	m, err := BuildManifest("v1.2.3-dev", notes, "v0.0.0", "", sampleChecksums)
	if err != nil {
		t.Fatalf("BuildManifest: %v", err)
	}

	if m.Latest != "v1.2.3-dev" {
		t.Errorf("Latest = %q, quero v1.2.3-dev", m.Latest)
	}
	if m.NotesMD != notes {
		t.Errorf("NotesMD = %q, quero %q", m.NotesMD, notes)
	}
	if m.MinSupported != "v0.0.0" {
		t.Errorf("MinSupported = %q, quero v0.0.0", m.MinSupported)
	}

	want := map[string]Asset{
		"linux-amd64": {Name: "camera-linux-amd64", SHA256: "aaaa000000000000000000000000000000000000000000000000000000000001"},
		"linux-arm64": {Name: "camera-linux-arm64", SHA256: "bbbb000000000000000000000000000000000000000000000000000000000002"},
		"linux-arm":   {Name: "camera-linux-arm", SHA256: "cccc000000000000000000000000000000000000000000000000000000000003"},
	}
	if len(m.Assets) != len(want) {
		t.Fatalf("Assets tem %d entradas, quero %d: %+v", len(m.Assets), len(want), m.Assets)
	}
	for k, w := range want {
		got, ok := m.Assets[k]
		if !ok {
			t.Errorf("falta asset %q", k)
			continue
		}
		if got != w {
			t.Errorf("asset %q = %+v, quero %+v", k, got, w)
		}
	}
}

func TestBuildManifestIgnoresNonAssetLines(t *testing.T) {
	m, err := BuildManifest("v1.0.0", "n", "v0.0.0", "", sampleChecksums)
	if err != nil {
		t.Fatalf("BuildManifest: %v", err)
	}
	if _, ok := m.Assets["checksums.txt"]; ok {
		t.Error("checksums.txt não deveria virar asset")
	}
	for k := range m.Assets {
		if !strings.HasPrefix(k, "linux-") {
			t.Errorf("chave de asset inesperada: %q", k)
		}
	}
}

func TestBuildManifestErrorsWhenNoAssets(t *testing.T) {
	if _, err := BuildManifest("v1.0.0", "n", "v0.0.0", "", "dddd  checksums.txt\n"); err == nil {
		t.Error("esperava erro quando não há binários camera-linux-*")
	}
}

func TestBuildManifestImage(t *testing.T) {
	const img = "jacksonbicalho/os-camera:1.2.3-dev"

	withImg, err := BuildManifest("v1.2.3-dev", "n", "v0.0.0", img, sampleChecksums)
	if err != nil {
		t.Fatalf("BuildManifest: %v", err)
	}
	if withImg.Image != img {
		t.Errorf("Image = %q, quero %q", withImg.Image, img)
	}
	rawWith, err := withImg.JSON()
	if err != nil {
		t.Fatalf("JSON: %v", err)
	}
	var backWith map[string]json.RawMessage
	if err := json.Unmarshal(rawWith, &backWith); err != nil {
		t.Fatalf("JSON inválido: %v", err)
	}
	if _, ok := backWith["image"]; !ok {
		t.Errorf("chave image ausente com image informado: %s", rawWith)
	}

	// image vazio → campo omitido (omitempty).
	noImg, err := BuildManifest("v1.2.3-dev", "n", "v0.0.0", "", sampleChecksums)
	if err != nil {
		t.Fatalf("BuildManifest: %v", err)
	}
	rawNo, err := noImg.JSON()
	if err != nil {
		t.Fatalf("JSON: %v", err)
	}
	var backNo map[string]json.RawMessage
	if err := json.Unmarshal(rawNo, &backNo); err != nil {
		t.Fatalf("JSON inválido: %v", err)
	}
	if _, ok := backNo["image"]; ok {
		t.Errorf("chave image deveria ser omitida com image vazio: %s", rawNo)
	}
}

func TestManifestJSON(t *testing.T) {
	notes := "linha 1\nlinha 2 com \"aspas\" e <html> & símbolos"
	m, err := BuildManifest("v1.2.3-dev", notes, "v0.9.0", "", sampleChecksums)
	if err != nil {
		t.Fatalf("BuildManifest: %v", err)
	}
	raw, err := m.JSON()
	if err != nil {
		t.Fatalf("JSON: %v", err)
	}

	// Round-trip num map cru garante chaves corretas + escaping do notes_md.
	var back map[string]json.RawMessage
	if err := json.Unmarshal(raw, &back); err != nil {
		t.Fatalf("JSON inválido: %v\n%s", err, raw)
	}
	for _, key := range []string{"latest", "notes_md", "min_supported", "assets"} {
		if _, ok := back[key]; !ok {
			t.Errorf("falta chave %q no JSON: %s", key, raw)
		}
	}
	if _, ok := back["image"]; ok {
		t.Error("JSON não deve ter campo image nesta história")
	}

	var gotNotes string
	if err := json.Unmarshal(back["notes_md"], &gotNotes); err != nil {
		t.Fatalf("notes_md não desserializa: %v", err)
	}
	if gotNotes != notes {
		t.Errorf("notes_md round-trip = %q, quero %q", gotNotes, notes)
	}
}
