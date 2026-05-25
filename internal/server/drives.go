package server

import (
	"encoding/json"
	"net/http"

	"camera/internal/db"
)

type driveDTO struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Type     string `json:"type"`
	Endpoint string `json:"endpoint"`
	Bucket   string `json:"bucket"`
	Region   string `json:"region"`
	// AccessKey and SecretKey are write-only: never returned in responses.
	Prefix string `json:"prefix"`
}

func driveToDTO(dr db.Drive) driveDTO {
	return driveDTO{
		ID:       dr.ID,
		Name:     dr.Name,
		Type:     dr.Type,
		Endpoint: dr.Endpoint,
		Bucket:   dr.Bucket,
		Region:   dr.Region,
		Prefix:   dr.Prefix,
	}
}

func (s *Server) handleListDrives(w http.ResponseWriter, r *http.Request) {
	if !s.requireDB(w) {
		return
	}
	drives, err := db.ListDrives(s.db)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	list := make([]driveDTO, len(drives))
	for i, dr := range drives {
		list[i] = driveToDTO(dr)
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(list)
}

func (s *Server) handleCreateDrive(w http.ResponseWriter, r *http.Request) {
	if !s.requireDB(w) {
		return
	}
	var input struct {
		Name      string `json:"name"`
		Type      string `json:"type"`
		Endpoint  string `json:"endpoint"`
		Bucket    string `json:"bucket"`
		Region    string `json:"region"`
		AccessKey string `json:"access_key"`
		SecretKey string `json:"secret_key"`
		Prefix    string `json:"prefix"`
	}
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}
	if input.Name == "" || input.Bucket == "" || input.AccessKey == "" || input.SecretKey == "" {
		http.Error(w, "name, bucket, access_key and secret_key are required", http.StatusBadRequest)
		return
	}
	driveType := input.Type
	if driveType == "" {
		driveType = "s3"
	}
	if driveType != "s3" {
		http.Error(w, "unsupported drive type", http.StatusBadRequest)
		return
	}
	dr := db.Drive{
		Name:      input.Name,
		Type:      driveType,
		Endpoint:  input.Endpoint,
		Bucket:    input.Bucket,
		Region:    input.Region,
		AccessKey: input.AccessKey,
		SecretKey: input.SecretKey,
		Prefix:    input.Prefix,
	}
	created, err := db.InsertDrive(s.db, dr)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(driveToDTO(created))
}

func (s *Server) handleUpdateDrive(w http.ResponseWriter, r *http.Request) {
	if !s.requireDB(w) {
		return
	}
	id := r.PathValue("id")
	existing, err := db.GetDrive(s.db, id)
	if err != nil {
		http.Error(w, "drive not found", http.StatusNotFound)
		return
	}
	var input struct {
		Name      string `json:"name"`
		Endpoint  string `json:"endpoint"`
		Bucket    string `json:"bucket"`
		Region    string `json:"region"`
		AccessKey string `json:"access_key"`
		SecretKey string `json:"secret_key"`
		Prefix    string `json:"prefix"`
	}
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}
	if input.Name != "" {
		existing.Name = input.Name
	}
	existing.Endpoint = input.Endpoint
	existing.Bucket = input.Bucket
	existing.Region = input.Region
	existing.Prefix = input.Prefix
	// Only update credentials if explicitly provided.
	if input.AccessKey != "" {
		existing.AccessKey = input.AccessKey
	}
	if input.SecretKey != "" {
		existing.SecretKey = input.SecretKey
	}
	if err := db.UpdateDrive(s.db, existing); err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(driveToDTO(existing))
}

func (s *Server) handleDeleteDrive(w http.ResponseWriter, r *http.Request) {
	if !s.requireDB(w) {
		return
	}
	id := r.PathValue("id")
	if err := db.DeleteDrive(s.db, id); err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleListRetentionConfigs(w http.ResponseWriter, r *http.Request) {
	if !s.requireDB(w) {
		return
	}
	configs, err := db.ListRetentionConfigs(s.db)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(configs)
}

func (s *Server) handleUpdateRetentionConfig(w http.ResponseWriter, r *http.Request) {
	if !s.requireDB(w) {
		return
	}
	category := r.PathValue("category")
	if category != "with_motion" && category != "without_motion" {
		http.Error(w, "invalid category", http.StatusBadRequest)
		return
	}
	var input struct {
		Action  string `json:"action"`
		DriveID string `json:"drive_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}
	if input.Action != "delete" && input.Action != "send_to_drive" {
		http.Error(w, "action must be 'delete' or 'send_to_drive'", http.StatusBadRequest)
		return
	}
	if input.Action == "send_to_drive" && input.DriveID == "" {
		http.Error(w, "drive_id required for send_to_drive action", http.StatusBadRequest)
		return
	}
	rc := db.RetentionConfig{
		Category: category,
		Action:   input.Action,
		DriveID:  input.DriveID,
	}
	if err := db.UpdateRetentionConfig(s.db, rc); err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(rc)
}
