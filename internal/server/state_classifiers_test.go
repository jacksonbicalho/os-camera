package server_test

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"image"
	"image/color"
	"image/jpeg"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"camera/internal/config"
	"camera/internal/db"
	"camera/internal/server"
)

func setupClassifierServer(t *testing.T) (*server.Server, string, string) {
	t.Helper()
	database := openServerTestDB(t)
	if _, err := db.CreateUser(database, "admin", "pw", "admin", false); err != nil {
		t.Fatalf("create user: %v", err)
	}
	cam := config.CameraConfig{ID: "cam1", Name: "Cam", RTSPURL: "rtsp://admin:pw@192.168.1.29:554/"}
	if _, err := db.CreateCamera(database, cam, nil); err != nil {
		t.Fatalf("create camera: %v", err)
	}
	srv := server.NewServer(config.ServerConfig{}, "UTC", []config.CameraConfig{cam}, discardLogger(), nil).WithDB(database)
	token := loginAndGetToken(t, srv, "admin", "pw")
	return srv, token, cam.ID
}

func doJSON(t *testing.T, srv *server.Server, method, path, token string, body any) *httptest.ResponseRecorder {
	t.Helper()
	var buf bytes.Buffer
	if body != nil {
		json.NewEncoder(&buf).Encode(body)
	}
	req := httptest.NewRequest(method, path, &buf)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)
	return w
}

func validClassifierBody() map[string]any {
	return map[string]any{
		"name":           "Portão",
		"threshold":      0.8,
		"trigger_motion": true,
		"crop_x":         0.1, "crop_y": 0.1, "crop_w": 0.3, "crop_h": 0.3,
		"classes": []string{"aberto", "fechado"},
	}
}

func TestClassifierCreateAndList(t *testing.T) {
	srv, token, id := setupClassifierServer(t)

	w := doJSON(t, srv, http.MethodPost, "/api/settings/cameras/"+id+"/classifiers", token, validClassifierBody())
	if w.Code != http.StatusCreated {
		t.Fatalf("create: expected 201, got %d: %s", w.Code, w.Body.String())
	}
	var created struct {
		ID      int64    `json:"id"`
		Classes []string `json:"classes"`
	}
	json.Unmarshal(w.Body.Bytes(), &created)
	if created.ID == 0 || len(created.Classes) != 2 {
		t.Fatalf("unexpected created: %+v", created)
	}

	w = doJSON(t, srv, http.MethodGet, "/api/settings/cameras/"+id+"/classifiers", token, nil)
	if w.Code != http.StatusOK {
		t.Fatalf("list: %d", w.Code)
	}
	var list []map[string]any
	json.Unmarshal(w.Body.Bytes(), &list)
	if len(list) != 1 {
		t.Fatalf("expected 1 classifier, got %d", len(list))
	}
}

func TestClassifierValidation(t *testing.T) {
	srv, token, id := setupClassifierServer(t)
	path := "/api/settings/cameras/" + id + "/classifiers"

	cases := map[string]func(map[string]any){
		"name vazio":   func(b map[string]any) { b["name"] = "  " },
		"< 2 classes":  func(b map[string]any) { b["classes"] = []string{"aberto"} },
		"crop inválido": func(b map[string]any) { b["crop_w"] = 0.95; b["crop_x"] = 0.5 },
		"sem gatilho":  func(b map[string]any) { b["trigger_motion"] = false },
	}
	for name, mutate := range cases {
		body := validClassifierBody()
		mutate(body)
		w := doJSON(t, srv, http.MethodPost, path, token, body)
		if w.Code != http.StatusBadRequest {
			t.Fatalf("%s: expected 400, got %d: %s", name, w.Code, w.Body.String())
		}
	}
}

func TestClassifierUpdateDeleteAndState(t *testing.T) {
	srv, token, id := setupClassifierServer(t)
	base := "/api/settings/cameras/" + id + "/classifiers"

	w := doJSON(t, srv, http.MethodPost, base, token, validClassifierBody())
	var created struct {
		ID int64 `json:"id"`
	}
	json.Unmarshal(w.Body.Bytes(), &created)
	cidPath := base + "/" + strconv.FormatInt(created.ID, 10)

	// update
	upd := validClassifierBody()
	upd["name"] = "Portão lateral"
	w = doJSON(t, srv, http.MethodPut, cidPath, token, upd)
	if w.Code != http.StatusOK {
		t.Fatalf("update: %d %s", w.Code, w.Body.String())
	}

	// state (vazio até a S3) — rota cameraAccess
	w = doJSON(t, srv, http.MethodGet, "/api/cameras/"+id+"/classifiers/"+strconv.FormatInt(created.ID, 10)+"/state", token, nil)
	if w.Code != http.StatusOK || w.Body.String() != "null\n" {
		t.Fatalf("state: expected 200 null, got %d %q", w.Code, w.Body.String())
	}

	// delete
	w = doJSON(t, srv, http.MethodDelete, cidPath, token, nil)
	if w.Code != http.StatusNoContent {
		t.Fatalf("delete: %d", w.Code)
	}
	w = doJSON(t, srv, http.MethodGet, base, token, nil)
	var list []map[string]any
	json.Unmarshal(w.Body.Bytes(), &list)
	if len(list) != 0 {
		t.Fatalf("expected empty after delete, got %d", len(list))
	}
}

func TestClassifierTrain(t *testing.T) {
	database := openServerTestDB(t)
	if _, err := db.CreateUser(database, "admin", "pw", "admin", false); err != nil {
		t.Fatal(err)
	}
	cam := config.CameraConfig{ID: "cam1", Name: "Cam", RTSPURL: "rtsp://x/"}
	if _, err := db.CreateCamera(database, cam, nil); err != nil {
		t.Fatal(err)
	}

	// mock do serviço YOLO — captura o nome do modelo de destino do treino
	var gotModel string
	yolo := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Model string `json:"model"`
		}
		json.NewDecoder(r.Body).Decode(&body)
		gotModel = body.Model
		json.NewEncoder(w).Encode(map[string]string{"job_id": "j1"})
	}))
	defer yolo.Close()
	if err := db.UpdateVideoAnalysisConfig(database, db.VideoAnalysisConfig{Enabled: true, ServiceURL: yolo.URL, Model: "yolov8n", ConfidenceThreshold: 0.4}); err != nil {
		t.Fatal(err)
	}

	srv := server.NewServer(config.ServerConfig{}, "UTC", []config.CameraConfig{cam}, discardLogger(), nil).
		WithDB(database).
		WithStorageConfig(config.StorageConfig{Path: t.TempDir()})
	token := loginAndGetToken(t, srv, "admin", "pw")

	w := doJSON(t, srv, http.MethodPost, "/api/settings/cameras/cam1/classifiers", token, validClassifierBody())
	var created struct {
		ID int64 `json:"id"`
	}
	json.Unmarshal(w.Body.Bytes(), &created)
	trainPath := "/api/settings/cameras/cam1/classifiers/" + strconv.FormatInt(created.ID, 10) + "/train"

	jpeg := base64.StdEncoding.EncodeToString([]byte("fake"))
	w = doJSON(t, srv, http.MethodPost, trainPath, token, map[string]any{
		"samples": []map[string]string{
			{"label": "fechado", "image_b64": jpeg},
			{"label": "aberto", "image_b64": jpeg},
		},
	})
	if w.Code != http.StatusOK {
		t.Fatalf("train: expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var resp struct {
		JobID string `json:"job_id"`
	}
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp.JobID != "j1" {
		t.Fatalf("expected job_id j1, got %q", resp.JobID)
	}
	// treino deve mirar o modelo DESTE classificador (não o compartilhado)
	wantModel := "custom-cls-" + strconv.FormatInt(created.ID, 10)
	if gotModel != wantModel {
		t.Fatalf("expected train model %q, got %q", wantModel, gotModel)
	}

	// < 2 classes → 400
	w = doJSON(t, srv, http.MethodPost, trainPath, token, map[string]any{
		"samples": []map[string]string{{"label": "fechado", "image_b64": jpeg}},
	})
	if w.Code != http.StatusBadRequest {
		t.Fatalf("train com 1 classe: expected 400, got %d", w.Code)
	}
}

// realJPEGBase64 devolve um JPEG sólido wxh em base64 (sem prefixo data:).
func realJPEGBase64(t *testing.T, w, h int) string {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, color.RGBA{R: 180, G: 90, B: 40, A: 255})
		}
	}
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, nil); err != nil {
		t.Fatal(err)
	}
	return base64.StdEncoding.EncodeToString(buf.Bytes())
}

// Treinar da lista: POST /train SEM samples treina a partir das amostras
// persistidas (frames inteiros recortados server-side).
func TestClassifierTrainFromStoredSamples(t *testing.T) {
	database := openServerTestDB(t)
	if _, err := db.CreateUser(database, "admin", "pw", "admin", false); err != nil {
		t.Fatal(err)
	}
	cam := config.CameraConfig{ID: "cam1", Name: "Cam", RTSPURL: "rtsp://x/"}
	if _, err := db.CreateCamera(database, cam, nil); err != nil {
		t.Fatal(err)
	}

	var gotModel string
	var gotSamples int
	yolo := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Model   string `json:"model"`
			Samples []struct {
				ImagePath string `json:"image_path"`
				Label     string `json:"label"`
			} `json:"samples"`
		}
		json.NewDecoder(r.Body).Decode(&body)
		gotModel = body.Model
		gotSamples = len(body.Samples)
		json.NewEncoder(w).Encode(map[string]string{"job_id": "j2"})
	}))
	defer yolo.Close()
	if err := db.UpdateVideoAnalysisConfig(database, db.VideoAnalysisConfig{Enabled: true, ServiceURL: yolo.URL, Model: "yolov8n", ConfidenceThreshold: 0.4}); err != nil {
		t.Fatal(err)
	}

	srv := server.NewServer(config.ServerConfig{}, "UTC", []config.CameraConfig{cam}, discardLogger(), nil).
		WithDB(database).
		WithStorageConfig(config.StorageConfig{Path: t.TempDir()})
	token := loginAndGetToken(t, srv, "admin", "pw")

	w := doJSON(t, srv, http.MethodPost, "/api/settings/cameras/cam1/classifiers", token, validClassifierBody())
	var created struct {
		ID int64 `json:"id"`
	}
	json.Unmarshal(w.Body.Bytes(), &created)
	base := "/api/settings/cameras/cam1/classifiers/" + strconv.FormatInt(created.ID, 10)

	// persiste amostras (frames inteiros) das 2 classes
	jpg := realJPEGBase64(t, 100, 100)
	w = doJSON(t, srv, http.MethodPost, base+"/samples", token, map[string]any{
		"samples": []map[string]string{
			{"label": "aberto", "image_b64": jpg},
			{"label": "aberto", "image_b64": jpg},
			{"label": "fechado", "image_b64": jpg},
		},
	})
	if w.Code != http.StatusNoContent && w.Code != http.StatusOK {
		t.Fatalf("salvar amostras: %d %s", w.Code, w.Body.String())
	}

	// treina SEM corpo → usa as amostras persistidas
	w = doJSON(t, srv, http.MethodPost, base+"/train", token, map[string]any{})
	if w.Code != http.StatusOK {
		t.Fatalf("train sem samples: expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var resp struct {
		JobID string `json:"job_id"`
	}
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp.JobID != "j2" {
		t.Fatalf("expected job_id j2, got %q", resp.JobID)
	}
	if gotModel != "custom-cls-"+strconv.FormatInt(created.ID, 10) {
		t.Fatalf("modelo de destino errado: %q", gotModel)
	}
	if gotSamples != 3 {
		t.Fatalf("esperava 3 amostras recortadas no treino, got %d", gotSamples)
	}
}
