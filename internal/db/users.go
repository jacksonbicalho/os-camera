package db

import (
	"database/sql"
	"errors"
	"fmt"
	"time"

	"golang.org/x/crypto/bcrypt"
)

// BcryptCost is the work factor used when hashing passwords.
// Tests may override this to bcrypt.MinCost (4) for speed.
var BcryptCost = 12

// User represents a row from the users table.
type User struct {
	ID           int64
	Username     string
	PasswordHash string
	Role         string
	CreatedAt    time.Time
}

// CreateUser inserts a new user, hashing the password with bcrypt. Returns
// the new user's ID.
func CreateUser(db *DB, username, password, role string) (int64, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), BcryptCost)
	if err != nil {
		return 0, fmt.Errorf("hash password: %w", err)
	}

	res, err := db.Exec(
		`INSERT INTO users(username, password_hash, role) VALUES(?,?,?)`,
		username, string(hash), role,
	)
	if err != nil {
		return 0, fmt.Errorf("insert user: %w", err)
	}
	return res.LastInsertId()
}

// GetUserByID returns the user with the given ID.
func GetUserByID(db *DB, id int64) (User, error) {
	return scanUser(db.QueryRow(
		`SELECT id, username, password_hash, role, created_at FROM users WHERE id=?`, id,
	))
}

// GetUserByUsername returns the user with the given username.
func GetUserByUsername(db *DB, username string) (User, error) {
	return scanUser(db.QueryRow(
		`SELECT id, username, password_hash, role, created_at FROM users WHERE username=?`, username,
	))
}

func scanUser(row *sql.Row) (User, error) {
	var u User
	var createdAt string
	if err := row.Scan(&u.ID, &u.Username, &u.PasswordHash, &u.Role, &createdAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return User{}, fmt.Errorf("user not found")
		}
		return User{}, fmt.Errorf("scan user: %w", err)
	}
	t, err := time.Parse("2006-01-02 15:04:05", createdAt)
	if err == nil {
		u.CreatedAt = t
	}
	return u, nil
}

// ListUsers returns all users ordered by id.
func ListUsers(db *DB) ([]User, error) {
	rows, err := db.Query(
		`SELECT id, username, password_hash, role, created_at FROM users ORDER BY id`,
	)
	if err != nil {
		return nil, fmt.Errorf("list users: %w", err)
	}
	defer rows.Close()

	var users []User
	for rows.Next() {
		var u User
		var createdAt string
		if err := rows.Scan(&u.ID, &u.Username, &u.PasswordHash, &u.Role, &createdAt); err != nil {
			return nil, fmt.Errorf("scan user row: %w", err)
		}
		if t, err := time.Parse("2006-01-02 15:04:05", createdAt); err == nil {
			u.CreatedAt = t
		}
		users = append(users, u)
	}
	return users, rows.Err()
}

// UpdateUser updates username, password and role for the given user ID.
// A new bcrypt hash is generated for the new password.
func UpdateUser(db *DB, id int64, username, password, role string) error {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), BcryptCost)
	if err != nil {
		return fmt.Errorf("hash password: %w", err)
	}
	_, err = db.Exec(
		`UPDATE users SET username=?, password_hash=?, role=? WHERE id=?`,
		username, string(hash), role, id,
	)
	return err
}

// PatchUser updates username and role. When newPassword is non-empty, also
// replaces the password hash; otherwise the existing hash is preserved.
func PatchUser(db *DB, id int64, username, role, newPassword string) error {
	if newPassword != "" {
		hash, err := bcrypt.GenerateFromPassword([]byte(newPassword), BcryptCost)
		if err != nil {
			return fmt.Errorf("hash password: %w", err)
		}
		_, err = db.Exec(
			`UPDATE users SET username=?, password_hash=?, role=? WHERE id=?`,
			username, string(hash), role, id,
		)
		return err
	}
	_, err := db.Exec(
		`UPDATE users SET username=?, role=? WHERE id=?`,
		username, role, id,
	)
	return err
}

// DeleteUser removes the user and cascades to user_cameras.
func DeleteUser(db *DB, id int64) error {
	_, err := db.Exec(`DELETE FROM users WHERE id=?`, id)
	return err
}

// SetUserCameras replaces the set of camera IDs allowed for a viewer.
// Passing an empty slice removes all permissions.
func SetUserCameras(db *DB, userID int64, cameraIDs []string) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback() //nolint:errcheck

	if _, err := tx.Exec(`DELETE FROM user_cameras WHERE user_id=?`, userID); err != nil {
		return err
	}
	for _, cid := range cameraIDs {
		if _, err := tx.Exec(
			`INSERT INTO user_cameras(user_id, camera_id) VALUES(?,?)`, userID, cid,
		); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// GetUserCameras returns the list of camera IDs accessible to a user.
func GetUserCameras(db *DB, userID int64) ([]string, error) {
	rows, err := db.Query(
		`SELECT camera_id FROM user_cameras WHERE user_id=? ORDER BY camera_id`, userID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var ids []string
	for rows.Next() {
		var cid string
		if err := rows.Scan(&cid); err != nil {
			return nil, err
		}
		ids = append(ids, cid)
	}
	return ids, rows.Err()
}

// CheckPassword returns true when the plain-text password matches the bcrypt hash.
func CheckPassword(hash, password string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) == nil
}
