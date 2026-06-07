package server

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"camera/internal/db"
)

func (s *Server) currentUserID(r *http.Request) int64 {
	ac, _ := r.Context().Value(claimsKey).(authClaims)
	return ac.UserID
}

func notificationID(r *http.Request) (int64, bool) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	return id, err == nil
}

func (s *Server) handleListNotifications(w http.ResponseWriter, r *http.Request) {
	if s.db == nil {
		http.Error(w, "database unavailable", http.StatusInternalServerError)
		return
	}
	userID := s.currentUserID(r)
	list, err := db.ListUserNotifications(s.db, userID)
	if err != nil {
		http.Error(w, "failed to list notifications", http.StatusInternalServerError)
		return
	}
	unread, err := db.CountUnreadNotifications(s.db, userID)
	if err != nil {
		http.Error(w, "failed to count notifications", http.StatusInternalServerError)
		return
	}

	type dto struct {
		ID        int64  `json:"id"`
		Type      string `json:"type"`
		Title     string `json:"title,omitempty"`
		Message   string `json:"message"`
		Link      string `json:"link,omitempty"`
		CreatedAt string `json:"created_at"`
		Read      bool   `json:"read"`
		ReadAt    string `json:"read_at,omitempty"`
	}
	out := make([]dto, 0, len(list))
	for _, n := range list {
		d := dto{
			ID:        n.ID,
			Type:      n.Type,
			Title:     n.Title,
			Message:   n.Message,
			Link:      n.Link,
			CreatedAt: n.CreatedAt.UTC().Format(time.RFC3339),
			Read:      n.ReadAt != nil,
		}
		if n.ReadAt != nil {
			d.ReadAt = n.ReadAt.UTC().Format(time.RFC3339)
		}
		out = append(out, d)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"unread_count":  unread,
		"notifications": out,
	})
}

func (s *Server) handleMarkNotificationRead(w http.ResponseWriter, r *http.Request) {
	id, ok := notificationID(r)
	if !ok {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}
	if err := db.MarkNotificationRead(s.db, s.currentUserID(r), id); err != nil {
		http.Error(w, "notification not found", http.StatusNotFound)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleMarkAllNotificationsRead(w http.ResponseWriter, r *http.Request) {
	if err := db.MarkAllNotificationsRead(s.db, s.currentUserID(r)); err != nil {
		http.Error(w, "failed to mark notifications read", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleDeleteNotification(w http.ResponseWriter, r *http.Request) {
	id, ok := notificationID(r)
	if !ok {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}
	if err := db.DeleteUserNotification(s.db, s.currentUserID(r), id); err != nil {
		http.Error(w, "notification not found", http.StatusNotFound)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleDeleteAllNotifications(w http.ResponseWriter, r *http.Request) {
	if err := db.DeleteAllUserNotifications(s.db, s.currentUserID(r)); err != nil {
		http.Error(w, "failed to delete notifications", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
