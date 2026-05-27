package analysis

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

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
	return &Client{baseURL: baseURL, httpClient: &http.Client{}}
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

type FakeAnalyzer struct {
	Results []Detection
	Err     error
	Called  int
}

func (f *FakeAnalyzer) Analyze(_ context.Context, _ AnalyzeRequest) ([]Detection, error) {
	f.Called++
	return f.Results, f.Err
}
