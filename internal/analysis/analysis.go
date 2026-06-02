package analysis

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

var analysisTransport = &http.Transport{DisableKeepAlives: true}

type Detection struct {
	Label      string  `json:"label"`
	Confidence float64 `json:"confidence"`
	FrameCount int     `json:"frame_count"`
}

type AnalyzeRequest struct {
	Path                string  `json:"path"`
	Model               string  `json:"model,omitempty"`
	ConfidenceThreshold float64 `json:"confidence_threshold,omitempty"`
}

type AnalyzeResponse struct {
	Detections []Detection `json:"detections"`
}

type Analyzer interface {
	Analyze(ctx context.Context, req AnalyzeRequest) ([]Detection, error)
}

type Client struct {
	baseURL    string
	httpClient *http.Client
}

func NewClient(baseURL string) *Client {
	return &Client{baseURL: baseURL, httpClient: &http.Client{Transport: analysisTransport}}
}

func (c *Client) Analyze(ctx context.Context, req AnalyzeRequest) ([]Detection, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/analyze", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("yolo service returned %d", resp.StatusCode)
	}
	var result AnalyzeResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	return result.Detections, nil
}

// --- Models ---

type ModelInfo struct {
	Name      string `json:"name"`
	Group     string `json:"group"`
	Inference bool   `json:"inference"`
	Finetune  bool   `json:"finetune"`
}

type ModelsResponse struct {
	Device  string      `json:"device"`
	VramGB  float64     `json:"vram_gb"`
	Models  []ModelInfo `json:"models"`
}

func (c *Client) Models(ctx context.Context) (ModelsResponse, error) {
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/models", nil)
	if err != nil {
		return ModelsResponse{}, err
	}
	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return ModelsResponse{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return ModelsResponse{}, fmt.Errorf("yolo service returned %d", resp.StatusCode)
	}
	var result ModelsResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return ModelsResponse{}, err
	}
	return result, nil
}

// --- Fine-tuning ---

type AnnotationItem struct {
	ImagePath string  `json:"image_path"`
	Label     string  `json:"label"`
	BboxX     float64 `json:"bbox_x"`
	BboxY     float64 `json:"bbox_y"`
	BboxW     float64 `json:"bbox_w"`
	BboxH     float64 `json:"bbox_h"`
}

type FinetuneRequest struct {
	Annotations []AnnotationItem `json:"annotations"`
	BaseModel   string           `json:"base_model,omitempty"`
	Epochs      int              `json:"epochs,omitempty"`
}

type FinetuneResponse struct {
	JobID string `json:"job_id"`
}

type FinetuneStatus struct {
	Status      string `json:"status"`
	Epoch       int    `json:"epoch"`
	TotalEpochs int    `json:"total_epochs"`
	Error       string `json:"error"`
}

func (c *Client) Finetune(ctx context.Context, req FinetuneRequest) (FinetuneResponse, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return FinetuneResponse{}, err
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/finetune", bytes.NewReader(body))
	if err != nil {
		return FinetuneResponse{}, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return FinetuneResponse{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return FinetuneResponse{}, fmt.Errorf("yolo service returned %d", resp.StatusCode)
	}
	var result FinetuneResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return FinetuneResponse{}, err
	}
	return result, nil
}

func (c *Client) FinetuneStatus(ctx context.Context, jobID string) (FinetuneStatus, error) {
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/finetune/status/"+jobID, nil)
	if err != nil {
		return FinetuneStatus{}, err
	}
	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return FinetuneStatus{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return FinetuneStatus{}, fmt.Errorf("job not found")
	}
	if resp.StatusCode != http.StatusOK {
		return FinetuneStatus{}, fmt.Errorf("yolo service returned %d", resp.StatusCode)
	}
	var result FinetuneStatus
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return FinetuneStatus{}, err
	}
	return result, nil
}

func (c *Client) CancelFinetune(ctx context.Context, jobID string) error {
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodDelete, c.baseURL+"/finetune/"+jobID, nil)
	if err != nil {
		return err
	}
	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		return fmt.Errorf("yolo service returned %d", resp.StatusCode)
	}
	return nil
}

// --- Fake ---

type FakeAnalyzer struct {
	Results []Detection
	Err     error
	Called  int
}

func (f *FakeAnalyzer) Analyze(_ context.Context, _ AnalyzeRequest) ([]Detection, error) {
	f.Called++
	return f.Results, f.Err
}
