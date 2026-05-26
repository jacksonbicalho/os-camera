package db

import "time"

type Drive struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Type      string    `json:"type"`
	Endpoint  string    `json:"endpoint"`
	Bucket    string    `json:"bucket"`
	Region    string    `json:"region"`
	AccessKey string    `json:"access_key"`
	SecretKey string    `json:"secret_key"`
	Prefix    string    `json:"prefix"`
	CreatedAt time.Time `json:"created_at"`
}

type RetentionConfig struct {
	Category string `json:"category"`
	Action   string `json:"action"`
	DriveID  string `json:"drive_id,omitempty"`
}

func ListDrives(d *DB) ([]Drive, error) {
	rows, err := d.Query(`
		SELECT id, name, type, endpoint, bucket, region, access_key, secret_key, prefix, created_at
		FROM drives ORDER BY created_at ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var drives []Drive
	for rows.Next() {
		var dr Drive
		var createdAt string
		if err := rows.Scan(&dr.ID, &dr.Name, &dr.Type, &dr.Endpoint, &dr.Bucket,
			&dr.Region, &dr.AccessKey, &dr.SecretKey, &dr.Prefix, &createdAt); err != nil {
			return nil, err
		}
		dr.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
		drives = append(drives, dr)
	}
	return drives, rows.Err()
}

func GetDrive(d *DB, id string) (Drive, error) {
	var dr Drive
	var createdAt string
	err := d.QueryRow(`
		SELECT id, name, type, endpoint, bucket, region, access_key, secret_key, prefix, created_at
		FROM drives WHERE id = ?`, id).
		Scan(&dr.ID, &dr.Name, &dr.Type, &dr.Endpoint, &dr.Bucket,
			&dr.Region, &dr.AccessKey, &dr.SecretKey, &dr.Prefix, &createdAt)
	if err != nil {
		return Drive{}, err
	}
	dr.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
	return dr, nil
}

func InsertDrive(d *DB, dr Drive) (Drive, error) {
	var id string
	err := d.QueryRow(`
		INSERT INTO drives (name, type, endpoint, bucket, region, access_key, secret_key, prefix)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		RETURNING id`,
		dr.Name, dr.Type, dr.Endpoint, dr.Bucket, dr.Region, dr.AccessKey, dr.SecretKey, dr.Prefix,
	).Scan(&id)
	if err != nil {
		return Drive{}, err
	}
	return GetDrive(d, id)
}

func UpdateDrive(d *DB, dr Drive) error {
	_, err := d.Exec(`
		UPDATE drives SET name=?, type=?, endpoint=?, bucket=?, region=?, access_key=?, secret_key=?, prefix=?
		WHERE id=?`,
		dr.Name, dr.Type, dr.Endpoint, dr.Bucket, dr.Region, dr.AccessKey, dr.SecretKey, dr.Prefix, dr.ID)
	return err
}

func DeleteDrive(d *DB, id string) error {
	// Reset retention configs before deleting the drive so the WHERE clause can
	// still match (the FK ON DELETE SET DEFAULT would NULL drive_id first otherwise).
	if _, err := d.Exec(`UPDATE retention_config SET action='delete', drive_id=NULL WHERE drive_id=?`, id); err != nil {
		return err
	}
	_, err := d.Exec(`DELETE FROM drives WHERE id=?`, id)
	return err
}

func ListRetentionConfigs(d *DB) ([]RetentionConfig, error) {
	rows, err := d.Query(`SELECT category, action, COALESCE(drive_id,'') FROM retention_config ORDER BY category`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var configs []RetentionConfig
	for rows.Next() {
		var rc RetentionConfig
		if err := rows.Scan(&rc.Category, &rc.Action, &rc.DriveID); err != nil {
			return nil, err
		}
		configs = append(configs, rc)
	}
	return configs, rows.Err()
}

func UpdateRetentionConfig(d *DB, rc RetentionConfig) error {
	driveID := interface{}(nil)
	if rc.DriveID != "" {
		driveID = rc.DriveID
	}
	_, err := d.Exec(`
		INSERT INTO retention_config (category, action, drive_id) VALUES (?, ?, ?)
		ON CONFLICT(category) DO UPDATE SET action=excluded.action, drive_id=excluded.drive_id`,
		rc.Category, rc.Action, driveID)
	return err
}
