package stateengine

import (
	"fmt"
	"image"
	"image/jpeg"
	"os"
	"path/filepath"
	"sort"

	"camera/internal/analysis"
	"camera/internal/stateclass"
)

// BuildTrainSetFromSamples monta o conjunto de treino a partir das amostras já
// persistidas (frames inteiros em state_samples/{cid}/{classe}/): recorta cada
// frame pela região do classificador (cropNormalized) e grava o crop em
// state_train/{cid}/{classe}/, devolvendo as amostras para o serviço YOLO.
// Lista vazia (sem amostras) não é erro. Substitui o train set anterior.
func BuildTrainSetFromSamples(storagePath string, c stateclass.Classifier) ([]analysis.ClassifySample, error) {
	srcBase := filepath.Join(storagePath, "state_samples", fmt.Sprint(c.ID))
	dstBase := filepath.Join(storagePath, "state_train", fmt.Sprint(c.ID))

	labels, err := os.ReadDir(srcBase)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if err := os.RemoveAll(dstBase); err != nil {
		return nil, err
	}

	out := []analysis.ClassifySample{}
	for _, l := range labels {
		if !l.IsDir() {
			continue
		}
		label := l.Name()
		slug := Slug(label) // identidade da classe no treino (pasta de origem já deve ser slug)
		files, err := os.ReadDir(filepath.Join(srcBase, label))
		if err != nil {
			return nil, err
		}
		sort.Slice(files, func(i, j int) bool { return files[i].Name() < files[j].Name() })
		idx := 0
		for _, f := range files {
			if f.IsDir() {
				continue
			}
			crop, err := cropFile(filepath.Join(srcBase, label, f.Name()), c)
			if err != nil {
				return nil, err
			}
			dstDir := filepath.Join(dstBase, slug)
			if err := os.MkdirAll(dstDir, 0755); err != nil {
				return nil, err
			}
			p := filepath.Join(dstDir, fmt.Sprintf("%d.jpg", idx))
			if err := writeJPEGFile(p, crop); err != nil {
				return nil, err
			}
			idx++
			out = append(out, analysis.ClassifySample{ImagePath: p, Label: slug})
		}
	}
	return out, nil
}

// cropFile decodifica o JPEG em path e devolve o recorte da região do classificador.
func cropFile(path string, c stateclass.Classifier) (image.Image, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	img, err := jpeg.Decode(f)
	if err != nil {
		return nil, fmt.Errorf("decode %q: %w", path, err)
	}
	return cropNormalized(img, c.CropX, c.CropY, c.CropW, c.CropH), nil
}

func writeJPEGFile(path string, img image.Image) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return jpeg.Encode(f, img, &jpeg.Options{Quality: 90})
}
