package server_test

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
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

	// mock do serviço YOLO
	yolo := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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

	// < 2 classes → 400
	w = doJSON(t, srv, http.MethodPost, trainPath, token, map[string]any{
		"samples": []map[string]string{{"label": "fechado", "image_b64": jpeg}},
	})
	if w.Code != http.StatusBadRequest {
		t.Fatalf("train com 1 classe: expected 400, got %d", w.Code)
	}
}
