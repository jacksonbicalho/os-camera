package db

import (
	"database/sql"
	"fmt"
	"time"
)

// UserNotification is a persisted, per-user notification. ReadAt is nil while unread.
type UserNotification struct {
	ID        int64
	UserID    int64
	Type      string // success | error | warning | info
	Title     string
	Message   string
	Link      string
	CreatedAt time.Time
	ReadAt    *time.Time
}

// InsertUserNotification stores a notification for a user and returns its id.
// CreatedAt defaults to now and Type to "info" when unset.
func InsertUserNotification(db *DB, n UserNotification) (int64, error) {
	if n.CreatedAt.IsZero() {
		n.CreatedAt = time.Now().UTC()
	}
	if n.Type == "" {
		n.Type = "info"
	}
	res, err := db.Exec(
		`INSERT INTO user_notifications(user_id, type, title, message, link, created_at)
		 VALUES(?,?,?,?,?,?)`,
		n.UserID, n.Type, nullStr(n.Title), n.Message, nullStr(n.Link),
		n.CreatedAt.UTC().Format(time.RFC3339),
	)
	if err != nil {
		return 0, fmt.Errorf("insert user notification: %w", err)
	}
	return res.LastInsertId()
}

// ListUserNotifications returns a user's notifications, newest first.
func ListUserNotifications(db *DB, userID int64) ([]UserNotification, error) {
	rows, err := db.Query(
		`SELECT id, user_id, type, title, message, link, created_at, read_at
		 FROM user_notifications
		 WHERE user_id=?
		 ORDER BY created_at DESC, id DESC`,
		userID,
	)
	if err != nil {
		return nil, fmt.Errorf("list user notifications: %w", err)
	}
	defer rows.Close()

	var out []UserNotification
	for rows.Next() {
		var n UserNotification
		var title, link, readAt sql.NullString
		var createdAt string
		if err := rows.Scan(&n.ID, &n.UserID, &n.Type, &title, &n.Message, &link, &createdAt, &readAt); err != nil {
			return nil, fmt.Errorf("scan user notification: %w", err)
		}
		n.Title = title.String
		n.Link = link.String
		n.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
		if readAt.Valid {
			if t, err := time.Parse(time.RFC3339, readAt.String); err == nil {
				n.ReadAt = &t
			}
		}
		out = append(out, n)
	}
	return out, rows.Err()
}

// CountUnreadNotifications returns how many of the user's notifications are unread.
func CountUnreadNotifications(db *DB, userID int64) (int, error) {
	var n int
	err := db.QueryRow(
		`SELECT COUNT(*) FROM user_notifications WHERE user_id=? AND read_at IS NULL`,
		userID,
	).Scan(&n)
	if err != nil {
		return 0, fmt.Errorf("count unread notifications: %w", err)
	}
	return n, nil
}

// MarkNotificationRead marks a single notification read. Idempotent if already
// read; returns an error if the notification does not belong to the user.
func MarkNotificationRead(db *DB, userID, id int64) error {
	res, err := db.Exec(
		`UPDATE user_notifications SET read_at=COALESCE(read_at, ?) WHERE id=? AND user_id=?`,
		time.Now().UTC().Format(time.RFC3339), id, userID,
	)
	if err != nil {
		return fmt.Errorf("mark notification read: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return fmt.Errorf("notification %d not found for user %d", id, userID)
	}
	return nil
}

// MarkAllNotificationsRead marks all of the user's unread notifications read.
func MarkAllNotificationsRead(db *DB, userID int64) error {
	_, err := db.Exec(
		`UPDATE user_notifications SET read_at=? WHERE user_id=? AND read_at IS NULL`,
		time.Now().UTC().Format(time.RFC3339), userID,
	)
	if err != nil {
		return fmt.Errorf("mark all notifications read: %w", err)
	}
	return nil
}

// DeleteUserNotification removes a single notification owned by the user.
func DeleteUserNotification(db *DB, userID, id int64) error {
	res, err := db.Exec(`DELETE FROM user_notifications WHERE id=? AND user_id=?`, id, userID)
	if err != nil {
		return fmt.Errorf("delete user notification: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return fmt.Errorf("notification %d not found for user %d", id, userID)
	}
	return nil
}

// DeleteAllUserNotifications removes all notifications of the user.
func DeleteAllUserNotifications(db *DB, userID int64) error {
	_, err := db.Exec(`DELETE FROM user_notifications WHERE user_id=?`, userID)
	if err != nil {
		return fmt.Errorf("delete all user notifications: %w", err)
	}
	return nil
}
