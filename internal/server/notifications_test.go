package server_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"camera/internal/config"
	"camera/internal/db"
	"camera/internal/server"
)

type notifResp struct {
	UnreadCount   int `json:"unread_count"`
	Notifications []struct {
		ID      int64  `json:"id"`
		Type    string `json:"type"`
		Message string `json:"message"`
		Read    bool   `json:"read"`
	} `json:"notifications"`
}

func notifServer(t *testing.T) (*server.Server, *db.DB) {
	t.Helper()
	database := openServerTestDB(t)
	srv := server.NewServer(config.ServerConfig{}, "UTC", nil, discardLogger(), nil).WithDB(database)
	return srv, database
}

func getNotifications(t *testing.T, srv http.Handler, token string) notifResp {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/api/notifications", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("GET /api/notifications: expected 200, got %d", w.Code)
	}
	var r notifResp
	if err := json.NewDecoder(w.Body).Decode(&r); err != nil {
		t.Fatalf("decode: %v", err)
	}
	return r
}

func TestNotifications_ListScopedToUser(t *testing.T) {
	srv, database := notifServer(t)
	u1, _ := db.CreateUser(database, "u1", "pw", "viewer", false)
	u2, _ := db.CreateUser(database, "u2", "pw", "viewer", false)
	db.InsertUserNotification(database, db.UserNotification{UserID: u1, Type: "warning", Message: "para u1"})
	db.InsertUserNotification(database, db.UserNotification{UserID: u2, Type: "info", Message: "para u2"})

	token := loginAndGetToken(t, srv, "u1", "pw")
	r := getNotifications(t, srv, token)

	if len(r.Notifications) != 1 {
		t.Fatalf("expected 1 notification for u1, got %d", len(r.Notifications))
	}
	if r.Notifications[0].Message != "para u1" {
		t.Errorf("wrong notification: %q", r.Notifications[0].Message)
	}
	if r.UnreadCount != 1 {
		t.Errorf("expected unread_count 1, got %d", r.UnreadCount)
	}
}

func TestNotifications_MarkReadAndDelete(t *testing.T) {
	srv, database := notifServer(t)
	u1, _ := db.CreateUser(database, "u1", "pw", "viewer", false)
	id, _ := db.InsertUserNotification(database, db.UserNotification{UserID: u1, Type: "info", Message: "x"})
	token := loginAndGetToken(t, srv, "u1", "pw")

	idStr := strconv.FormatInt(id, 10)

	// mark read
	req := httptest.NewRequest(http.MethodPost, "/api/notifications/"+idStr+"/read", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)
	if w.Code != http.StatusNoContent && w.Code != http.StatusOK {
		t.Fatalf("mark read: expected 200/204, got %d", w.Code)
	}
	if r := getNotifications(t, srv, token); r.UnreadCount != 0 {
		t.Errorf("expected 0 unread after mark read, got %d", r.UnreadCount)
	}

	// delete
	req = httptest.NewRequest(http.MethodDelete, "/api/notifications/"+idStr, nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w = httptest.NewRecorder()
	srv.ServeHTTP(w, req)
	if w.Code != http.StatusNoContent && w.Code != http.StatusOK {
		t.Fatalf("delete: expected 200/204, got %d", w.Code)
	}
	if r := getNotifications(t, srv, token); len(r.Notifications) != 0 {
		t.Errorf("expected 0 notifications after delete, got %d", len(r.Notifications))
	}
}

func TestNotifications_RequiresAuth(t *testing.T) {
	srv, _ := notifServer(t)
	req := httptest.NewRequest(http.MethodGet, "/api/notifications", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 without token, got %d", w.Code)
	}
}
