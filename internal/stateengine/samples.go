package stateengine

import (
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"camera/internal/analysis"
)

// ListSamples lista os crops salvos de um classificador, por classe, como URLs
// servíveis pelo handler de `/recordings/` (state_samples fica sob o storage).
// Mapa vazio quando ainda não há amostras.
func ListSamples(storagePath string, classifierID int64) (map[string][]string, error) {
	base := filepath.Join(storagePath, "state_samples", fmt.Sprint(classifierID))
	out := map[string][]string{}
	labels, err := os.ReadDir(base)
	if os.IsNotExist(err) {
		return out, nil
	}
	if err != nil {
		return nil, err
	}
	for _, l := range labels {
		if !l.IsDir() {
			continue
		}
		files, err := os.ReadDir(filepath.Join(base, l.Name()))
		if err != nil {
			return nil, err
		}
		for _, f := range files {
			if f.IsDir() {
				continue
			}
			out[l.Name()] = append(out[l.Name()], fmt.Sprintf("/recordings/state_samples/%d/%s/%s", classifierID, l.Name(), f.Name()))
		}
	}
	return out, nil
}

// LabeledImage é uma amostra de treino: o JPEG (base64, sem prefixo data:) e a classe.
type LabeledImage struct {
	Label   string `json:"label"`
	DataB64 string `json:"image_b64"`
}

// SaveSamples persiste as amostras (frames inteiros) em state_samples/{cid} — é o
// que reidrata o form ao editar. Substitui o set anterior.
func SaveSamples(storagePath string, classifierID int64, imgs []LabeledImage) ([]analysis.ClassifySample, error) {
	return saveImagesTo(filepath.Join(storagePath, "state_samples", fmt.Sprint(classifierID)), imgs)
}

// SaveTrainSet grava os crops do treino em state_train/{cid} — separado da
// persistência, para o treino não sobrescrever os frames inteiros salvos.
func SaveTrainSet(storagePath string, classifierID int64, imgs []LabeledImage) ([]analysis.ClassifySample, error) {
	return saveImagesTo(filepath.Join(storagePath, "state_train", fmt.Sprint(classifierID)), imgs)
}

// saveImagesTo decodifica as amostras rotuladas e grava cada JPEG em
// base/{label}/{i}.jpg, substituindo o conteúdo anterior. Devolve a lista de
// amostras (paths que o serviço YOLO lê — mesmo path, pois compartilham `/data`).
func saveImagesTo(base string, imgs []LabeledImage) ([]analysis.ClassifySample, error) {
	if err := os.RemoveAll(base); err != nil {
		return nil, err
	}
	out := make([]analysis.ClassifySample, 0, len(imgs))
	counts := map[string]int{}
	for _, im := range imgs {
		raw := im.DataB64
		if i := strings.Index(raw, ","); strings.HasPrefix(raw, "data:") && i >= 0 {
			raw = raw[i+1:] // tira o prefixo data:image/...;base64,
		}
		data, err := base64.StdEncoding.DecodeString(raw)
		if err != nil {
			return nil, fmt.Errorf("decode sample %q: %w", im.Label, err)
		}
		dir := filepath.Join(base, im.Label)
		if err := os.MkdirAll(dir, 0755); err != nil {
			return nil, err
		}
		idx := counts[im.Label]
		counts[im.Label]++
		p := filepath.Join(dir, fmt.Sprintf("%d.jpg", idx))
		if err := os.WriteFile(p, data, 0644); err != nil {
			return nil, err
		}
		out = append(out, analysis.ClassifySample{ImagePath: p, Label: im.Label})
	}
	return out, nil
}
