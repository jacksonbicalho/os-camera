package server_test

import (
	"testing"

	"camera/internal/config"
	"camera/internal/db"
	"camera/internal/server"
	"camera/internal/stateclass"
)

func TestPublishClassifierStateNotifiesAccessOnly(t *testing.T) {
	database := openServerTestDB(t)
	adminID, _ := db.CreateUser(database, "admin", "pw", "admin", false)
	v1ID, _ := db.CreateUser(database, "v1", "pw", "viewer", false)
	v2ID, _ := db.CreateUser(database, "v2", "pw", "viewer", false)
	cam := config.CameraConfig{ID: "cam1", Name: "Cam", RTSPURL: "rtsp://x/"}
	if _, err := db.CreateCamera(database, cam, nil); err != nil {
		t.Fatal(err)
	}
	if err := db.SetUserCameras(database, v1ID, []string{"cam1"}); err != nil {
		t.Fatal(err)
	}
	// v2 não tem acesso à cam1
	if err := db.SetUserCameras(database, v2ID, []string{}); err != nil {
		t.Fatal(err)
	}

	srv := server.NewServer(config.ServerConfig{}, "UTC", []config.CameraConfig{cam}, discardLogger(), nil).WithDB(database)
	srv.PublishClassifierState(stateclass.Classifier{ID: 1, CameraID: "cam1", Name: "Portão"}, "aberto", 0.9)

	count := func(uid int64) int {
		ns, err := db.ListUserNotifications(database, uid)
		if err != nil {
			t.Fatal(err)
		}
		return len(ns)
	}
	if count(adminID) != 1 {
		t.Fatalf("admin deveria ter 1 notificação, got %d", count(adminID))
	}
	if count(v1ID) != 1 {
		t.Fatalf("viewer com acesso deveria ter 1, got %d", count(v1ID))
	}
	if count(v2ID) != 0 {
		t.Fatalf("viewer sem acesso não deveria ter notificação, got %d", count(v2ID))
	}
	// conteúdo
	ns, _ := db.ListUserNotifications(database, adminID)
	if ns[0].Title != "Portão" || ns[0].Message != "Estado: aberto" {
		t.Fatalf("notificação inesperada: %+v", ns[0])
	}
}
