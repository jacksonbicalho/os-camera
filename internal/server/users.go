package server

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"camera/internal/db"
)

type userDTO struct {
	ID        int64    `json:"id"`
	Username  string   `json:"username"`
	Role      string   `json:"role"`
	Cameras   []string `json:"cameras"`
	CreatedAt string   `json:"created_at"`
}

func (s *Server) userToDTO(u db.User) (userDTO, error) {
	cameras, err := db.GetUserCameras(s.db, u.ID)
	if err != nil {
		return userDTO{}, err
	}
	if cameras == nil {
		cameras = []string{}
	}
	return userDTO{
		ID:        u.ID,
		Username:  u.Username,
		Role:      u.Role,
		Cameras:   cameras,
		CreatedAt: u.CreatedAt.UTC().Format(time.RFC3339),
	}, nil
}

func (s *Server) requireDB(w http.ResponseWriter) bool {
	if s.db == nil {
		http.Error(w, "not available without database", http.StatusServiceUnavailable)
		return false
	}
	return true
}

func (s *Server) handleListUsers(w http.ResponseWriter, r *http.Request) {
	if !s.requireDB(w) {
		return
	}
	users, err := db.ListUsers(s.db)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	list := make([]userDTO, 0, len(users))
	for _, u := range users {
		dto, err := s.userToDTO(u)
		if err != nil {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		list = append(list, dto)
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(list)
}

func (s *Server) handleCreateUser(w http.ResponseWriter, r *http.Request) {
	if !s.requireDB(w) {
		return
	}
	var req struct {
		Username string   `json:"username"`
		Password string   `json:"password"`
		Role     string   `json:"role"`
		Cameras  []string `json:"cameras"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	if req.Role != "admin" && req.Role != "viewer" {
		http.Error(w, "invalid role: must be admin or viewer", http.StatusBadRequest)
		return
	}
	if req.Username == "" || req.Password == "" {
		http.Error(w, "username and password required", http.StatusBadRequest)
		return
	}

	id, err := db.CreateUser(s.db, req.Username, req.Password, req.Role)
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE constraint failed") {
			http.Error(w, "username already exists", http.StatusConflict)
			return
		}
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	if len(req.Cameras) > 0 {
		if err := db.SetUserCameras(s.db, id, req.Cameras); err != nil {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
	}

	u, err := db.GetUserByID(s.db, id)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	dto, err := s.userToDTO(u)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(dto)
}

func (s *Server) handleUpdateUser(w http.ResponseWriter, r *http.Request) {
	if !s.requireDB(w) {
		return
	}
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}

	var req struct {
		Username string    `json:"username"`
		Role     string    `json:"role"`
		Password string    `json:"password"`
		Cameras  *[]string `json:"cameras"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	if req.Role != "admin" && req.Role != "viewer" {
		http.Error(w, "invalid role: must be admin or viewer", http.StatusBadRequest)
		return
	}

	if _, err := db.GetUserByID(s.db, id); err != nil {
		http.NotFound(w, r)
		return
	}

	if err := db.PatchUser(s.db, id, req.Username, req.Role, req.Password); err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	if req.Cameras != nil {
		cams := *req.Cameras
		if cams == nil {
			cams = []string{}
		}
		if err := db.SetUserCameras(s.db, id, cams); err != nil {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
	}

	u, err := db.GetUserByID(s.db, id)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	dto, err := s.userToDTO(u)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(dto)
}

func (s *Server) handleDeleteUser(w http.ResponseWriter, r *http.Request) {
	if !s.requireDB(w) {
		return
	}
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}

	if _, err := db.GetUserByID(s.db, id); err != nil {
		http.NotFound(w, r)
		return
	}

	if err := db.DeleteUser(s.db, id); err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
