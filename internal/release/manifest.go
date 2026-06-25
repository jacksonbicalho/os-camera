// Package release monta o manifesto version.json publicado em cada release.
//
// O manifesto é servido estaticamente via
// github.com/<owner>/<repo>/releases/latest/download/version.json e consumido
// pelo updater para descobrir a última versão, o changelog e os binários.
package release

import (
	"encoding/json"
	"fmt"
	"strings"
)

// Asset descreve um binário publicado na release.
type Asset struct {
	Name   string `json:"name"`
	SHA256 string `json:"sha256"`
}

// Manifest é o conteúdo do version.json. O campo image (caminho da imagem
// Docker) fica para a história 1b e por isso não aparece aqui.
type Manifest struct {
	Latest       string           `json:"latest"`
	NotesMD      string           `json:"notes_md"`
	MinSupported string           `json:"min_supported"`
	Assets       map[string]Asset `json:"assets"`
}

// BuildManifest monta o manifesto a partir da tag, das notas de release em
// markdown, da versão mínima suportada e do conteúdo de um checksums.txt no
// formato do `sha256sum` ("<hash>  <arquivo>"). Apenas binários cujo nome casa
// "camera-linux-<arch>" viram assets; a chave de plataforma é "linux-<arch>".
func BuildManifest(latest, notesMD, minSupported, checksums string) (Manifest, error) {
	const prefix = "camera-linux-"

	assets := make(map[string]Asset)
	for _, line := range strings.Split(checksums, "\n") {
		fields := strings.Fields(line)
		if len(fields) != 2 {
			continue
		}
		sum, name := fields[0], fields[1]
		arch, ok := strings.CutPrefix(name, prefix)
		if !ok || arch == "" {
			continue
		}
		assets["linux-"+arch] = Asset{Name: name, SHA256: sum}
	}

	if len(assets) == 0 {
		return Manifest{}, fmt.Errorf("nenhum binário %s* encontrado no checksums", prefix)
	}

	return Manifest{
		Latest:       latest,
		NotesMD:      notesMD,
		MinSupported: minSupported,
		Assets:       assets,
	}, nil
}

// JSON serializa o manifesto indentado. Usa MarshalIndent para que o escaping
// do notes_md (markdown multilinha) seja tratado pelo encoding/json.
func (m Manifest) JSON() ([]byte, error) {
	return json.MarshalIndent(m, "", "  ")
}
