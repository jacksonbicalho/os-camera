package server_test

import (
	"testing"

	"camera/internal/config"
	"camera/internal/db"
	"camera/internal/release"
	"camera/internal/server"
)

func updateNotifyServer(t *testing.T) (*server.Server, *db.DB) {
	t.Helper()
	database := openServerTestDB(t)
	srv := server.NewServer(config.ServerConfig{}, "UTC", nil, discardLogger(), nil).WithDB(database)
	return srv, database
}

func TestNotifyUpdateAvailable_AdminsOnly(t *testing.T) {
	srv, database := updateNotifyServer(t)
	adminID, _ := db.CreateUser(database, "adm", "pw", "admin", false)
	viewerID, _ := db.CreateUser(database, "vw", "pw", "viewer", false)

	srv.NotifyUpdateAvailable(release.Status{Latest: "v2.0.0", UpdateAvailable: true})

	adm, err := db.ListUserNotifications(database, adminID)
	if err != nil {
		t.Fatalf("list admin notifications: %v", err)
	}
	if len(adm) != 1 {
		t.Fatalf("expected 1 admin notification, got %d", len(adm))
	}
	if adm[0].Link != "/settings/about" {
		t.Errorf("expected link /settings/about, got %q", adm[0].Link)
	}
	if adm[0].Title == "" || adm[0].Message == "" {
		t.Errorf("expected title and message, got title=%q message=%q", adm[0].Title, adm[0].Message)
	}

	vw, _ := db.ListUserNotifications(database, viewerID)
	if len(vw) != 0 {
		t.Errorf("expected viewer to receive 0 notifications, got %d", len(vw))
	}
}

func TestNotifyUpdateAvailable_DedupAndNewVersion(t *testing.T) {
	srv, database := updateNotifyServer(t)
	adminID, _ := db.CreateUser(database, "adm", "pw", "admin", false)

	// same version twice → only one notification
	srv.NotifyUpdateAvailable(release.Status{Latest: "v2.0.0", UpdateAvailable: true})
	srv.NotifyUpdateAvailable(release.Status{Latest: "v2.0.0", UpdateAvailable: true})
	adm, _ := db.ListUserNotifications(database, adminID)
	if len(adm) != 1 {
		t.Fatalf("expected dedup to keep 1 notification, got %d", len(adm))
	}

	// no update available → nothing new
	srv.NotifyUpdateAvailable(release.Status{Latest: "v3.0.0", UpdateAvailable: false})
	adm, _ = db.ListUserNotifications(database, adminID)
	if len(adm) != 1 {
		t.Fatalf("expected no new notification when not available, got %d", len(adm))
	}

	// new version available → new notification
	srv.NotifyUpdateAvailable(release.Status{Latest: "v3.0.0", UpdateAvailable: true})
	adm, _ = db.ListUserNotifications(database, adminID)
	if len(adm) != 2 {
		t.Fatalf("expected new notification for new version, got %d", len(adm))
	}
}
