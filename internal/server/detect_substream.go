package server

import (
	"encoding/json"
	"net/http"
	"strings"

	"camera/internal/db"
	"camera/internal/ffprobe"
)

// substreamCandidates derives likely substream RTSP URLs from a main URL using
// vendor conventions (Dahua/Intelbras: same URL with subtype=1). It returns only
// candidates that differ from the main URL; empty when none can be derived.
func substreamCandidates(mainURL string) []string {
	var out []string
	seen := map[string]bool{mainURL: true}
	add := func(u string) {
		if u == "" || seen[u] {
			return
		}
		seen[u] = true
		out = append(out, u)
	}

	switch {
	case strings.Contains(mainURL, "subtype=0"):
		add(strings.Replace(mainURL, "subtype=0", "subtype=1", 1))
	case strings.Contains(mainURL, "channel=") && !strings.Contains(mainURL, "subtype="):
		// realmonitor query without subtype defaults to the main stream; the
		// substream is the same URL with subtype=1 appended.
		sep := "&"
		if !strings.Contains(mainURL, "?") {
			sep = "?"
		}
		add(mainURL + sep + "subtype=1")
	}
	return out
}

type detectSubstreamResponse struct {
	MotionRTSPURL string `json:"motion_rtsp_url"`
	Width         int    `json:"width"`
	Height        int    `json:"height"`
}

// handleDetectSubstream derives substream candidates from the given main RTSP
// URL and returns the first one that ffprobe confirms (with real dimensions).
// When nothing can be derived/verified it returns an empty result (200, not an
// error) so the user can still type the URL by hand.
func (s *Server) handleDetectSubstream(w http.ResponseWriter, r *http.Request) {
	var req struct {
		RTSPURL string `json:"rtsp_url"`
		ID      string `json:"id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || strings.TrimSpace(req.RTSPURL) == "" {
		http.Error(w, "rtsp_url is required", http.StatusBadRequest)
		return
	}

	// When editing an existing camera the form carries the redacted password
	// ("xxxxx"); restore the real one from the stored camera so ffprobe can auth.
	rtsp := strings.TrimSpace(req.RTSPURL)
	if req.ID != "" && s.db != nil {
		if cam, err := db.GetCamera(s.db, req.ID); err == nil {
			rtsp = restoreMaskedRTSPPassword(rtsp, cam.RTSPURL)
		}
	}

	w.Header().Set("Content-Type", "application/json")
	if s.prober != nil {
		for _, cand := range substreamCandidates(rtsp) {
			info := ffprobe.Resolve(r.Context(), ffprobe.Resolver{RTSPURL: cand}, s.prober, s.log)
			if info.Width > 0 && info.Height > 0 {
				json.NewEncoder(w).Encode(detectSubstreamResponse{
					MotionRTSPURL: cand,
					Width:         info.Width,
					Height:        info.Height,
				})
				return
			}
		}
	}
	json.NewEncoder(w).Encode(detectSubstreamResponse{})
}
