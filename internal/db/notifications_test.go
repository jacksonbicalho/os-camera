package db_test

import (
	"testing"

	"camera/internal/db"
)

func mkUser(t *testing.T, d *db.DB, username string) int64 {
	t.Helper()
	id, err := db.CreateUser(d, username, "pw", "viewer", false)
	if err != nil {
		t.Fatalf("CreateUser(%s): %v", username, err)
	}
	return id
}

func TestUserNotifications_InsertListAndUnreadCount(t *testing.T) {
	d := openTestDB(t)
	u1 := mkUser(t, d, "u1")
	u2 := mkUser(t, d, "u2")

	if _, err := db.InsertUserNotification(d, db.UserNotification{UserID: u1, Type: "warning", Message: "disco cheio"}); err != nil {
		t.Fatalf("Insert u1: %v", err)
	}
	if _, err := db.InsertUserNotification(d, db.UserNotification{UserID: u1, Type: "info", Message: "outra"}); err != nil {
		t.Fatalf("Insert u1 2: %v", err)
	}
	if _, err := db.InsertUserNotification(d, db.UserNotification{UserID: u2, Type: "error", Message: "do u2"}); err != nil {
		t.Fatalf("Insert u2: %v", err)
	}

	list, err := db.ListUserNotifications(d, u1)
	if err != nil {
		t.Fatalf("List u1: %v", err)
	}
	if len(list) != 2 {
		t.Fatalf("expected 2 notifications for u1, got %d", len(list))
	}
	// newest first
	if list[0].Message != "outra" {
		t.Errorf("expected newest first, got %q", list[0].Message)
	}
	for _, n := range list {
		if n.ReadAt != nil {
			t.Errorf("new notification should be unread, got read_at=%v", n.ReadAt)
		}
	}

	unread, err := db.CountUnreadNotifications(d, u1)
	if err != nil {
		t.Fatalf("CountUnread: %v", err)
	}
	if unread != 2 {
		t.Errorf("expected 2 unread for u1, got %d", unread)
	}
}

func TestUserNotifications_MarkReadScopedToUser(t *testing.T) {
	d := openTestDB(t)
	u1 := mkUser(t, d, "u1")
	u2 := mkUser(t, d, "u2")
	id1, _ := db.InsertUserNotification(d, db.UserNotification{UserID: u1, Type: "info", Message: "a"})

	// u2 cannot mark u1's notification.
	if err := db.MarkNotificationRead(d, u2, id1); err == nil {
		t.Error("expected error marking another user's notification read")
	}
	if c, _ := db.CountUnreadNotifications(d, u1); c != 1 {
		t.Errorf("u1 should still have 1 unread, got %d", c)
	}

	if err := db.MarkNotificationRead(d, u1, id1); err != nil {
		t.Fatalf("MarkNotificationRead: %v", err)
	}
	if c, _ := db.CountUnreadNotifications(d, u1); c != 0 {
		t.Errorf("u1 should have 0 unread after marking, got %d", c)
	}
}

func TestUserNotifications_MarkAllAndDelete(t *testing.T) {
	d := openTestDB(t)
	u1 := mkUser(t, d, "u1")
	id1, _ := db.InsertUserNotification(d, db.UserNotification{UserID: u1, Type: "info", Message: "a"})
	db.InsertUserNotification(d, db.UserNotification{UserID: u1, Type: "info", Message: "b"})

	if err := db.MarkAllNotificationsRead(d, u1); err != nil {
		t.Fatalf("MarkAll: %v", err)
	}
	if c, _ := db.CountUnreadNotifications(d, u1); c != 0 {
		t.Errorf("expected 0 unread after MarkAll, got %d", c)
	}

	if err := db.DeleteUserNotification(d, u1, id1); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if err := db.DeleteAllUserNotifications(d, u1); err != nil {
		t.Fatalf("DeleteAll: %v", err)
	}
	list, _ := db.ListUserNotifications(d, u1)
	if len(list) != 0 {
		t.Errorf("expected 0 after DeleteAll, got %d", len(list))
	}
}

func TestUserNotifications_CascadeOnUserDelete(t *testing.T) {
	d := openTestDB(t)
	u1 := mkUser(t, d, "u1")
	db.InsertUserNotification(d, db.UserNotification{UserID: u1, Type: "info", Message: "a"})

	if err := db.DeleteUser(d, u1); err != nil {
		t.Fatalf("DeleteUser: %v", err)
	}
	list, _ := db.ListUserNotifications(d, u1)
	if len(list) != 0 {
		t.Errorf("expected notifications cascade-deleted with user, got %d", len(list))
	}
}
