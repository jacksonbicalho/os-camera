package server

import (
	"encoding/json"
	"net/http"

	"camera/internal/discovery"
)

func (s *Server) handleDiscover(w http.ResponseWriter, r *http.Request) {
	results := discovery.Discover(r.Context())
	if results == nil {
		results = []discovery.Result{}
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(results)
}

type discoverStreamsRequest struct {
	ONVIFXAddr string `json:"onvif_xaddr"`
	User       string `json:"user"`
	Pass       string `json:"pass"`
}

func (s *Server) handleDiscoverStreams(w http.ResponseWriter, r *http.Request) {
	var req discoverStreamsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}

	streams, err := discovery.GetStreamURIs(r.Context(), req.ONVIFXAddr, req.User, req.Pass)
	if err != nil || streams == nil {
		streams = []discovery.StreamURI{}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{"streams": streams})
}
