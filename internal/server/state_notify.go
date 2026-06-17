package server

import (
	"fmt"

	"camera/internal/db"
	"camera/internal/stateclass"
)

// notifyStateTransition cria uma notificação persistida (sino) para cada usuário
// com acesso à câmera — admin (todas) ou viewer com a câmera na sua lista.
func notifyStateTransition(database *db.DB, c stateclass.Classifier, state string) error {
	users, err := db.ListUsers(database)
	if err != nil {
		return err
	}
	for _, u := range users {
		if u.Role != "admin" {
			cams, err := db.GetUserCameras(database, u.ID)
			if err != nil || !sliceContains(cams, c.CameraID) {
				continue
			}
		}
		if _, err := db.InsertUserNotification(database, db.UserNotification{
			UserID:  u.ID,
			Type:    "info",
			Title:   c.Name,
			Message: fmt.Sprintf("Estado: %s", state),
			Link:    "/settings/cameras/states/" + c.CameraID,
		}); err != nil {
			return err
		}
	}
	return nil
}

func sliceContains(s []string, v string) bool {
	for _, x := range s {
		if x == v {
			return true
		}
	}
	return false
}

// PublishClassifierState é chamada pelo `emit` do runner na transição de estado:
// notifica os usuários com acesso. O estado em si já foi persistido pelo Runner.
func (s *Server) PublishClassifierState(c stateclass.Classifier, state string, confidence float64) {
	if s.db == nil {
		return
	}
	if err := notifyStateTransition(s.db, c, state); err != nil && s.log != nil {
		s.log.Warn("state notification failed", "classifier", c.ID, "err", err)
	}
}
