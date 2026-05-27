package analysis_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"camera/internal/analysis"
)

func TestClient_Analyze_Success(t *testing.T) {
	want := []analysis.Detection{
		{Label: "person", Confidence: 0.91, FrameCount: 8},
		{Label: "car", Confidence: 0.65, FrameCount: 2},
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/analyze" {
			http.NotFound(w, r)
			return
		}
		var req analysis.AnalyzeRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if req.Path == "" {
			http.Error(w, "missing path", http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(analysis.AnalyzeResponse{Detections: want})
	}))
	defer srv.Close()

	client := analysis.NewClient(srv.URL)
	got, err := client.Analyze(context.Background(), analysis.AnalyzeRequest{
		Path:                "/recordings/cam1/chunk.mp4",
		Model:               "yolov8n",
		ConfidenceThreshold: 0.4,
	})
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 detections, got %d", len(got))
	}
	if got[0].Label != "person" {
		t.Errorf("first label = %q, want person", got[0].Label)
	}
}

func TestClient_Analyze_ServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "internal error", http.StatusInternalServerError)
	}))
	defer srv.Close()

	client := analysis.NewClient(srv.URL)
	_, err := client.Analyze(context.Background(), analysis.AnalyzeRequest{Path: "/some/path.mp4"})
	if err == nil {
		t.Fatal("expected error on 500 response")
	}
}

func TestFakeAnalyzer(t *testing.T) {
	fake := &analysis.FakeAnalyzer{
		Results: []analysis.Detection{{Label: "dog", Confidence: 0.88, FrameCount: 5}},
	}
	got, err := fake.Analyze(context.Background(), analysis.AnalyzeRequest{Path: "/any.mp4"})
	if err != nil {
		t.Fatalf("FakeAnalyzer.Analyze: %v", err)
	}
	if len(got) != 1 || got[0].Label != "dog" {
		t.Errorf("unexpected result: %v", got)
	}
	if fake.Called != 1 {
		t.Errorf("Called = %d, want 1", fake.Called)
	}
}
