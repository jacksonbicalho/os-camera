// Package stateclass contém os tipos de domínio da classificação de estado
// (state classification): a configuração de um classificador por câmera e o
// estado corrente. Fica num pacote próprio (como internal/zones) para que tanto
// o db quanto o agendador (S3) o usem sem dependência circular.
package stateclass

import (
	"errors"
	"strings"
	"time"
)

// Classifier é a config de um classificador de estado de uma câmera: o recorte
// (crop, normalizado 0–1), as classes possíveis, o gatilho e o limiar.
type Classifier struct {
	ID                     int64    `json:"id"`
	CameraID               string   `json:"camera_id"`
	Name                   string   `json:"name"`
	Model                  string   `json:"model"`
	Threshold              float64  `json:"threshold"`
	TriggerMotion          bool     `json:"trigger_motion"`
	TriggerIntervalSeconds int      `json:"trigger_interval_seconds"`
	CropX                  float64  `json:"crop_x"`
	CropY                  float64  `json:"crop_y"`
	CropW                  float64  `json:"crop_w"`
	CropH                  float64  `json:"crop_h"`
	MinConsecutive         int      `json:"min_consecutive"`
	Enabled                bool     `json:"enabled"`
	Classes                []string `json:"classes"`
}

// State é o estado corrente (ou histórico) de um classificador — escrito pela S3.
type State struct {
	State      string    `json:"state"`
	Confidence float64   `json:"confidence"`
	ChangedAt  time.Time `json:"changed_at"`
}

// Validate retorna um erro voltado ao usuário (pt-BR) quando a config é inválida.
func (c Classifier) Validate() error {
	if strings.TrimSpace(c.Name) == "" {
		return errors.New("nome é obrigatório")
	}
	if len(c.Classes) < 2 {
		return errors.New("são necessárias ao menos 2 classes")
	}
	if c.CropX < 0 || c.CropY < 0 || c.CropW <= 0 || c.CropH <= 0 ||
		c.CropX > 1 || c.CropY > 1 || c.CropX+c.CropW > 1 || c.CropY+c.CropH > 1 {
		return errors.New("recorte inválido: coordenadas fora do intervalo [0,1]")
	}
	if !c.TriggerMotion && c.TriggerIntervalSeconds <= 0 {
		return errors.New("defina ao menos um gatilho: movimento ou intervalo")
	}
	return nil
}
