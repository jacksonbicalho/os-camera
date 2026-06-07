package db

import "fmt"

// GetUserTheme returns the user's UI theme preference (default 'dark').
func GetUserTheme(db *DB, userID int64) (string, error) {
	var theme string
	if err := db.QueryRow(`SELECT theme FROM users WHERE id=?`, userID).Scan(&theme); err != nil {
		return "", fmt.Errorf("get user theme: %w", err)
	}
	return theme, nil
}

// SetUserTheme persists the user's UI theme preference. Returns an error if the
// user does not exist. Validation of allowed values is done by the caller.
func SetUserTheme(db *DB, userID int64, theme string) error {
	res, err := db.Exec(`UPDATE users SET theme=? WHERE id=?`, theme, userID)
	if err != nil {
		return fmt.Errorf("set user theme: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return fmt.Errorf("user %d not found", userID)
	}
	return nil
}
