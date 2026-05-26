package server_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"camera/internal/config"
	"camera/internal/db"
	"camera/internal/server"
)

func setupDrivesServer(t *testing.T) (http.Handler, string) {
	t.Helper()
	srv := server.NewServer(config.ServerConfig{}, "UTC", nil, discardLogger(), nil)
	srv = withTestUsers(t, srv)
	token := loginAndGetToken(t, srv, "admin", "pw")
	return srv, token
}

func TestListDrives_Empty(t *testing.T) {
	srv, token := setupDrivesServer(t)

	req := httptest.NewRequest(http.MethodGet, "/api/drives", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var list []map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &list); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(list) != 0 {
		t.Fatalf("expected empty list, got %d", len(list))
	}
}

func TestCreateDrive_MissingFields(t *testing.T) {
	srv, token := setupDrivesServer(t)

	body := `{"name":"test"}`
	req := httptest.NewRequest(http.MethodPost, "/api/drives", bytes.NewBufferString(body))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestCreateAndDeleteDrive(t *testing.T) {
	srv, token := setupDrivesServer(t)

	body := `{"name":"my-s3","bucket":"my-bucket","region":"us-east-1","access_key":"AK","secret_key":"SK"}`
	req := httptest.NewRequest(http.MethodPost, "/api/drives", bytes.NewBufferString(body))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}
	var created map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &created); err != nil {
		t.Fatalf("unmarshal created: %v", err)
	}
	id, _ := created["id"].(string)
	if id == "" {
		t.Fatal("expected non-empty id")
	}
	// Credentials must not be in response.
	if _, ok := created["access_key"]; ok {
		t.Error("access_key should not be in response")
	}
	if _, ok := created["secret_key"]; ok {
		t.Error("secret_key should not be in response")
	}

	// Delete
	req = httptest.NewRequest(http.MethodDelete, "/api/drives/"+id, nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w = httptest.NewRecorder()
	srv.ServeHTTP(w, req)
	if w.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", w.Code)
	}
}

func TestDeleteDrive_ResetsRetentionConfig(t *testing.T) {
	srv, token := setupDrivesServer(t)

	// Create a drive.
	body := `{"name":"s3-drive","bucket":"bkt","region":"us-east-1","access_key":"AK","secret_key":"SK"}`
	req := httptest.NewRequest(http.MethodPost, "/api/drives", bytes.NewBufferString(body))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("create drive: %d", w.Code)
	}
	var created map[string]any
	json.Unmarshal(w.Body.Bytes(), &created)
	id := created["id"].(string)

	// Point retention at that drive.
	retBody := `{"action":"send_to_drive","drive_id":"` + id + `"}`
	req = httptest.NewRequest(http.MethodPut, "/api/retention/with_motion", bytes.NewBufferString(retBody))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	srv.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("set retention: %d: %s", w.Code, w.Body.String())
	}

	// Delete the drive.
	req = httptest.NewRequest(http.MethodDelete, "/api/drives/"+id, nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w = httptest.NewRecorder()
	srv.ServeHTTP(w, req)
	if w.Code != http.StatusNoContent {
		t.Fatalf("delete drive: %d", w.Code)
	}

	// Retention for with_motion must have been reset to delete.
	req = httptest.NewRequest(http.MethodGet, "/api/retention", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w = httptest.NewRecorder()
	srv.ServeHTTP(w, req)
	var configs []map[string]any
	json.Unmarshal(w.Body.Bytes(), &configs)
	for _, rc := range configs {
		if rc["category"] == "with_motion" {
			if rc["action"] != "delete" {
				t.Errorf("with_motion action = %q after drive deletion, want delete", rc["action"])
			}
			if rc["drive_id"] != nil && rc["drive_id"] != "" {
				t.Errorf("with_motion drive_id = %v after drive deletion, want empty", rc["drive_id"])
			}
		}
	}
}

func TestRetentionConfig_Defaults(t *testing.T) {
	srv, token := setupDrivesServer(t)

	req := httptest.NewRequest(http.MethodGet, "/api/retention", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var configs []map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &configs); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(configs) != 2 {
		t.Fatalf("expected 2 configs, got %d", len(configs))
	}
	for _, rc := range configs {
		if rc["action"] != "delete" {
			t.Errorf("category %q: default action = %q, want delete", rc["category"], rc["action"])
		}
	}
}

func TestUpdateRetentionConfig_InvalidCategory(t *testing.T) {
	srv, token := setupDrivesServer(t)

	body := `{"action":"delete"}`
	req := httptest.NewRequest(http.MethodPut, "/api/retention/unknown", bytes.NewBufferString(body))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestDrives_ForbiddenForViewer(t *testing.T) {
	database := openServerTestDB(t)
	if _, err := db.CreateUser(database, "vwr", "vwrpw", "viewer", false); err != nil {
		t.Fatalf("add viewer: %v", err)
	}
	srv := server.NewServer(config.ServerConfig{}, "UTC", nil, discardLogger(), nil).WithDB(database)
	viewerToken := loginAndGetToken(t, srv, "vwr", "vwrpw")

	for _, path := range []string{"/api/drives", "/api/retention"} {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		req.Header.Set("Authorization", "Bearer "+viewerToken)
		w := httptest.NewRecorder()
		srv.ServeHTTP(w, req)
		if w.Code != http.StatusForbidden {
			t.Errorf("GET %s: expected 403 for viewer, got %d", path, w.Code)
		}
	}
}
