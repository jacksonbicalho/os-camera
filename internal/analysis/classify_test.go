package analysis_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"camera/internal/analysis"
)

func TestClient_Classify_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/classify" || r.Method != http.MethodPost {
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
		}
		var req analysis.ClassifyRequest
		json.NewDecoder(r.Body).Decode(&req)
		if req.Path != "/tmp/crop.jpg" {
			t.Errorf("unexpected path %q", req.Path)
		}
		json.NewEncoder(w).Encode(map[string]any{
			"predictions": []analysis.ClassPrediction{
				{Label: "fechado", Prob: 0.82},
				{Label: "aberto", Prob: 0.18},
			},
			"top": "fechado",
		})
	}))
	defer srv.Close()

	got, err := analysis.NewClient(srv.URL).Classify(context.Background(), analysis.ClassifyRequest{
		Path: "/tmp/crop.jpg", Model: "custom-cls",
	})
	if err != nil {
		t.Fatalf("Classify: %v", err)
	}
	if len(got) != 2 || got[0].Label != "fechado" || got[0].Prob != 0.82 {
		t.Fatalf("unexpected predictions: %+v", got)
	}
}

func TestClient_ClassifyTrain_ReturnsJobID(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/classify/train" {
			t.Errorf("unexpected path %s", r.URL.Path)
		}
		var req analysis.ClassifyTrainRequest
		json.NewDecoder(r.Body).Decode(&req)
		if len(req.Samples) != 2 || req.BaseModel != "yolov8n-cls" {
			t.Errorf("unexpected train req: %+v", req)
		}
		json.NewEncoder(w).Encode(map[string]string{"job_id": "abc-123"})
	}))
	defer srv.Close()

	job, err := analysis.NewClient(srv.URL).ClassifyTrain(context.Background(), analysis.ClassifyTrainRequest{
		Samples:   []analysis.ClassifySample{{ImagePath: "/a.jpg", Label: "aberto"}, {ImagePath: "/b.jpg", Label: "fechado"}},
		BaseModel: "yolov8n-cls",
	})
	if err != nil || job != "abc-123" {
		t.Fatalf("ClassifyTrain: job=%q err=%v", job, err)
	}
}

func TestClient_Classify_ServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	if _, err := analysis.NewClient(srv.URL).Classify(context.Background(), analysis.ClassifyRequest{Path: "/x.jpg"}); err == nil {
		t.Fatal("esperava erro em 500")
	}
}
