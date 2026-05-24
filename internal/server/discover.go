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
