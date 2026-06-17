package server

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"camera/internal/analysis"
	"camera/internal/db"
	"camera/internal/stateclass"
	"camera/internal/stateengine"
)

func (s *Server) handleStateClassifiersGet(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if !s.cameraExists(id) {
		http.NotFound(w, r)
		return
	}
	out := []stateclass.Classifier{}
	if s.db != nil {
		var err error
		out, err = db.ListStateClassifiers(s.db, id)
		if err != nil {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(out)
}

// decodeClassifier reads the body, pins camera/id from the URL, and validates.
func decodeClassifier(w http.ResponseWriter, r *http.Request, cameraID string, id int64) (stateclass.Classifier, bool) {
	var c stateclass.Classifier
	if err := json.NewDecoder(r.Body).Decode(&c); err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return c, false
	}
	c.CameraID = cameraID
	c.ID = id
	if c.Model == "" {
		c.Model = "custom-cls"
	}
	if err := c.Validate(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return c, false
	}
	return c, true
}

func (s *Server) handleStateClassifierCreate(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if !s.cameraExists(id) {
		http.NotFound(w, r)
		return
	}
	if s.db == nil {
		http.Error(w, "banco de dados não configurado", http.StatusServiceUnavailable)
		return
	}
	c, ok := decodeClassifier(w, r, id, 0)
	if !ok {
		return
	}
	newID, err := db.CreateStateClassifier(s.db, c)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	c.ID = newID
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(c)
}

func (s *Server) handleStateClassifierUpdate(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	cid, err := strconv.ParseInt(r.PathValue("cid"), 10, 64)
	if !s.cameraExists(id) || err != nil {
		http.NotFound(w, r)
		return
	}
	if s.db == nil {
		http.Error(w, "banco de dados não configurado", http.StatusServiceUnavailable)
		return
	}
	c, ok := decodeClassifier(w, r, id, cid)
	if !ok {
		return
	}
	if err := db.UpdateStateClassifier(s.db, c); err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(c)
}

func (s *Server) handleStateClassifierDelete(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	cid, err := strconv.ParseInt(r.PathValue("cid"), 10, 64)
	if !s.cameraExists(id) || err != nil {
		http.NotFound(w, r)
		return
	}
	if s.db != nil {
		if err := db.DeleteStateClassifier(s.db, cid); err != nil {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
	}
	w.WriteHeader(http.StatusNoContent)
}

// handleStateClassifierTrain recebe amostras rotuladas (uma ou mais imagens por
// classe), salva os crops por classe e dispara o treino do `custom-cls` no serviço
// YOLO (S1). Retorna o job_id (status via GET /api/analysis/finetune/status).
func (s *Server) handleStateClassifierTrain(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	cid, err := strconv.ParseInt(r.PathValue("cid"), 10, 64)
	if !s.cameraExists(id) || err != nil {
		http.NotFound(w, r)
		return
	}
	if s.db == nil {
		http.Error(w, "banco de dados não configurado", http.StatusServiceUnavailable)
		return
	}
	var body struct {
		Samples []stateengine.LabeledImage `json:"samples"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}
	labels := map[string]bool{}
	for _, s := range body.Samples {
		labels[s.Label] = true
	}
	if len(labels) < 2 {
		http.Error(w, "são necessárias amostras de ao menos 2 classes", http.StatusBadRequest)
		return
	}

	cfg, err := db.GetVideoAnalysisConfig(s.db)
	if err != nil || cfg.ServiceURL == "" {
		http.Error(w, "serviço de análise não configurado", http.StatusServiceUnavailable)
		return
	}
	samples, err := stateengine.SaveTrainSet(s.storageCfg.Path, cid, body.Samples)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()
	jobID, err := analysis.NewClient(cfg.ServiceURL).ClassifyTrain(ctx, analysis.ClassifyTrainRequest{
		Samples: samples, BaseModel: "yolov8n-cls", Epochs: 20,
	})
	if err != nil {
		http.Error(w, "falha ao iniciar treino", http.StatusBadGateway)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"job_id": jobID})
}

// handleStateClassifierSamplesGet lista os crops salvos do classificador por classe
// (URLs servíveis via /recordings/). Usado para reidratar o form ao editar.
func (s *Server) handleStateClassifierSamplesGet(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	cid, err := strconv.ParseInt(r.PathValue("cid"), 10, 64)
	if !s.cameraExists(id) || err != nil {
		http.NotFound(w, r)
		return
	}
	m, err := stateengine.ListSamples(s.storageCfg.Path, cid)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{"samples": m})
}

// handleStateClassifierSamplesSave persiste os crops por classe (substitui o set
// anterior). Sem treino — o "Salvar" do form chama isto para as amostras não sumirem.
func (s *Server) handleStateClassifierSamplesSave(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	cid, err := strconv.ParseInt(r.PathValue("cid"), 10, 64)
	if !s.cameraExists(id) || err != nil {
		http.NotFound(w, r)
		return
	}
	var body struct {
		Samples []stateengine.LabeledImage `json:"samples"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}
	if _, err := stateengine.SaveSamples(s.storageCfg.Path, cid, body.Samples); err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleStateClassifierState(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	cid, err := strconv.ParseInt(r.PathValue("cid"), 10, 64)
	if !s.cameraExists(id) || err != nil {
		http.NotFound(w, r)
		return
	}
	var st *stateclass.State
	if s.db != nil {
		st, err = db.GetCurrentState(s.db, cid)
		if err != nil && err != sql.ErrNoRows {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(st) // nil → null (estado ainda não escrito pela S3)
}
