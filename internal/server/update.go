package server

import (
	"encoding/json"
	"net/http"
	"sync"
	"time"

	"camera/internal/updater"
)

const updateCacheTTL = time.Hour

type updateCache struct {
	mu        sync.Mutex
	info      updater.UpdateInfo
	fetchedAt time.Time
}

var globalUpdateCache updateCache

func (s *Server) handleUpdateCheck(w http.ResponseWriter, r *http.Request) {
	globalUpdateCache.mu.Lock()
	defer globalUpdateCache.mu.Unlock()

	if time.Since(globalUpdateCache.fetchedAt) < updateCacheTTL {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(globalUpdateCache.info)
		return
	}

	info, err := updater.CheckLatest(s.version, updater.DefaultAPIURL)
	if err != nil {
		s.log.Warn("update check failed", "err", err)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(updater.UpdateInfo{Current: s.version, Assets: map[string]string{}})
		return
	}

	globalUpdateCache.info = info
	globalUpdateCache.fetchedAt = time.Now()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(info)
}

func (s *Server) handleUpdateApply(w http.ResponseWriter, r *http.Request) {
	if updater.IsDocker() {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{
			"mode":    "docker",
			"message": "Atualize a imagem Docker para obter a nova versão: docker pull e reinicie o container.",
		})
		return
	}

	globalUpdateCache.mu.Lock()
	info := globalUpdateCache.info
	globalUpdateCache.mu.Unlock()

	if !info.UpdateAvailable {
		http.Error(w, "no update available", http.StatusConflict)
		return
	}

	// Respond before applying — the process will re-exec and the connection will close
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "applying", "version": info.Latest})
	if f, ok := w.(http.Flusher); ok {
		f.Flush()
	}

	// Apply runs in background so the HTTP response is sent first
	go func() {
		_ = updater.Apply(info)
	}()
}
