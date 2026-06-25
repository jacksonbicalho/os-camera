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

// Manifest é o conteúdo do version.json. Image é a referência da imagem Docker
// versionada (ex: jacksonbicalho/os-camera:1.2.3-dev) que o caminho
// Docker/Watchtower do updater puxa; fica omitido quando não informado.
type Manifest struct {
	Latest       string           `json:"latest"`
	NotesMD      string           `json:"notes_md"`
	MinSupported string           `json:"min_supported"`
	Image        string           `json:"image,omitempty"`
	Assets       map[string]Asset `json:"assets"`
}

// BuildManifest monta o manifesto a partir da tag, das notas de release em
// markdown, da versão mínima suportada, da referência da imagem Docker (vazia
// = omitida) e do conteúdo de um checksums.txt no formato do `sha256sum`
// ("<hash>  <arquivo>"). Apenas binários cujo nome casa "camera-linux-<arch>"
// viram assets; a chave de plataforma é "linux-<arch>".
func BuildManifest(latest, notesMD, minSupported, image, checksums string) (Manifest, error) {
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
		Image:        image,
		Assets:       assets,
	}, nil
}

// JSON serializa o manifesto indentado. Usa MarshalIndent para que o escaping
// do notes_md (markdown multilinha) seja tratado pelo encoding/json.
func (m Manifest) JSON() ([]byte, error) {
	return json.MarshalIndent(m, "", "  ")
}
