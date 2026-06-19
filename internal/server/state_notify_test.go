package server_test

import (
	"encoding/json"
	"net/http"
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
	srv.PublishClassifierState(stateclass.Classifier{
		ID: 1, CameraID: "cam1", Name: "Portão",
		NotifyEnabled: true, NotifyUserIDs: []int64{adminID, v1ID, v2ID},
	}, "aberto", 0.9)

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

func TestPublishClassifierStateConditional(t *testing.T) {
	database := openServerTestDB(t)
	a, _ := db.CreateUser(database, "admin", "pw", "admin", false)
	b, _ := db.CreateUser(database, "admin2", "pw", "admin", false)
	cam := config.CameraConfig{ID: "cam1", Name: "Cam", RTSPURL: "rtsp://x/"}
	if _, err := db.CreateCamera(database, cam, nil); err != nil {
		t.Fatal(err)
	}
	srv := server.NewServer(config.ServerConfig{}, "UTC", []config.CameraConfig{cam}, discardLogger(), nil).WithDB(database)
	count := func(uid int64) int {
		ns, _ := db.ListUserNotifications(database, uid)
		return len(ns)
	}

	// notify_enabled=false → ninguém recebe
	srv.PublishClassifierState(stateclass.Classifier{
		ID: 1, CameraID: "cam1", Name: "Portão", NotifyEnabled: false, NotifyUserIDs: []int64{a, b},
	}, "aberto", 0.9)
	if count(a) != 0 || count(b) != 0 {
		t.Fatalf("desabilitado não deveria notificar: a=%d b=%d", count(a), count(b))
	}

	// habilitado, mas só 'a' na lista → só 'a' recebe
	srv.PublishClassifierState(stateclass.Classifier{
		ID: 1, CameraID: "cam1", Name: "Portão", NotifyEnabled: true, NotifyUserIDs: []int64{a},
	}, "aberto", 0.9)
	if count(a) != 1 || count(b) != 0 {
		t.Fatalf("só os selecionados recebem: a=%d b=%d", count(a), count(b))
	}
}

// O endpoint /api/me/footer-states devolve os classificadores que o usuário marcou
// pra ver no rodapé, com o estado atual.
func TestFooterStatesEndpoint(t *testing.T) {
	database := openServerTestDB(t)
	adminID, _ := db.CreateUser(database, "admin", "pw", "admin", false)
	cam := config.CameraConfig{ID: "cam1", Name: "Cam", RTSPURL: "rtsp://x/"}
	if _, err := db.CreateCamera(database, cam, nil); err != nil {
		t.Fatal(err)
	}
	id, err := db.CreateStateClassifier(database, stateclass.Classifier{
		CameraID: "cam1", Name: "Corredor", Model: "custom-cls", Threshold: 0.8,
		CropW: 0.3, CropH: 0.3, MinConsecutive: 1, Classes: []string{"vazio", "cheio"},
		FooterEnabled: true, FooterUserIDs: []int64{adminID},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.RecordStateTransition(database, id, "vazio", 0.9); err != nil {
		t.Fatal(err)
	}

	srv := server.NewServer(config.ServerConfig{}, "UTC", []config.CameraConfig{cam}, discardLogger(), nil).WithDB(database)
	token := loginAndGetToken(t, srv, "admin", "pw")
	w := doJSON(t, srv, http.MethodGet, "/api/me/footer-states", token, nil)
	if w.Code != http.StatusOK {
		t.Fatalf("footer-states: %d %s", w.Code, w.Body.String())
	}
	var out []struct {
		ClassifierID int64  `json:"classifier_id"`
		Name         string `json:"name"`
		State        string `json:"state"`
	}
	json.Unmarshal(w.Body.Bytes(), &out)
	if len(out) != 1 || out[0].ClassifierID != id || out[0].Name != "Corredor" || out[0].State != "vazio" {
		t.Fatalf("footer-states inesperado: %+v", out)
	}
}

// A config de notificação é recarregada do banco a cada transição: edições de
// destinatários valem sem restart (o runner passa o snapshot do boot, que pode
// estar defasado).
func TestPublishClassifierStateReloadsRecipients(t *testing.T) {
	database := openServerTestDB(t)
	u1, _ := db.CreateUser(database, "u1", "pw", "admin", false)
	cam := config.CameraConfig{ID: "cam1", Name: "Cam", RTSPURL: "rtsp://x/"}
	if _, err := db.CreateCamera(database, cam, nil); err != nil {
		t.Fatal(err)
	}
	id, err := db.CreateStateClassifier(database, stateclass.Classifier{
		CameraID: "cam1", Name: "Portão", Model: "custom-cls", Threshold: 0.8,
		CropW: 0.3, CropH: 0.3, MinConsecutive: 1, Classes: []string{"aberto", "fechado"},
		NotifyEnabled: true, NotifyUserIDs: []int64{u1},
	})
	if err != nil {
		t.Fatal(err)
	}
	srv := server.NewServer(config.ServerConfig{}, "UTC", []config.CameraConfig{cam}, discardLogger(), nil).WithDB(database)

	// c defasado (notify off, sem destinatários) — deve recarregar do banco e notificar u1.
	srv.PublishClassifierState(stateclass.Classifier{ID: id, CameraID: "cam1", Name: "Portão"}, "aberto", 0.9)
	ns, _ := db.ListUserNotifications(database, u1)
	if len(ns) != 1 {
		t.Fatalf("deveria recarregar a config e notificar u1, got %d", len(ns))
	}
}
