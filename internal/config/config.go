package config

import (
	"fmt"
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

type Duration time.Duration

func (d *Duration) UnmarshalYAML(value *yaml.Node) error {
	parsed, err := time.ParseDuration(value.Value)
	if err != nil {
		return fmt.Errorf("invalid duration %q: %w", value.Value, err)
	}
	*d = Duration(parsed)
	return nil
}

const DefaultChunkDuration     = 5 * time.Minute
const DefaultReconnectInterval = 10 * time.Second

// Config holds the minimal bootstrap configuration read from the YAML file.
// All camera settings, motion config and user data live in the SQLite database.
type Config struct {
	Debug    bool          `yaml:"debug"`
	Timezone string        `yaml:"timezone"`
	DBPath   string        `yaml:"db_path"`
	Log      LogConfig     `yaml:"log"`
	Server   ServerConfig  `yaml:"server"`
	Storage  StorageConfig `yaml:"storage"`
	Admin    AdminConfig   `yaml:"admin"`
}

type LogConfig struct {
	Output string `yaml:"output"`
	Path   string `yaml:"path"`
}

type AdminConfig struct {
	Username string `yaml:"username"`
	Password string `yaml:"password"`
}

type ServerConfig struct {
	Port           int    `yaml:"port"`
	SegmentsPath   string `yaml:"segments_path"`
	RecordingsPath string `yaml:"recordings_path"`
	HLSDVRSeconds  int    `yaml:"hls_dvr_seconds"`
	JWTSecret      string `yaml:"jwt_secret"`
}

type RetentionConfig struct {
	WithMotionMinutes    int `yaml:"with_motion_minutes"`
	WithoutMotionMinutes int `yaml:"without_motion_minutes"`
}

type StorageConfig struct {
	Path             string          `yaml:"path"`
	RetentionMinutes int             `yaml:"retention_minutes"` // legacy fallback; 0 = disabled
	Retention        RetentionConfig `yaml:"retention"`
	IntervalMinutes  int             `yaml:"interval_minutes"` // 0 = default (60 min)
	MaxSizeGB        float64         `yaml:"max_size_gb"`      // 0 = disabled
	WarnPercent      float64         `yaml:"warn_percent"`
}

// EffectiveRetention returns (withMotionMinutes, withoutMotionMinutes).
func (s StorageConfig) EffectiveRetention() (withMotion, withoutMotion int) {
	r := s.Retention
	if r.WithMotionMinutes == 0 && r.WithoutMotionMinutes == 0 {
		return s.RetentionMinutes, s.RetentionMinutes
	}
	if r.WithMotionMinutes > 0 && r.WithoutMotionMinutes == 0 {
		return r.WithMotionMinutes, r.WithMotionMinutes
	}
	return r.WithMotionMinutes, r.WithoutMotionMinutes
}

// MotionConfig holds per-camera motion detection settings (used by the DB layer).
type MotionConfig struct {
	Enabled         bool    `yaml:"enabled"`
	Threshold       float64 `yaml:"threshold"`
	FPS             int     `yaml:"fps"`
	CooldownSeconds int     `yaml:"cooldown_seconds"`
}

// CameraConfig holds per-camera settings loaded from the database.
type CameraConfig struct {
	ID                string        `yaml:"id"`
	RTSPURL           string        `yaml:"rtsp_url"`
	ChunkDuration     Duration      `yaml:"chunk_duration"`
	ReconnectInterval Duration      `yaml:"reconnect_interval"`
	VideoCodec        string        `yaml:"video_codec"`
	HasAudio          *bool         `yaml:"has_audio"`
	Width             int           `yaml:"width"`
	Height            int           `yaml:"height"`
	DisplayOrder      int           `yaml:"display_order"`
	Motion            *MotionConfig `yaml:"motion"`
}

func (c CameraConfig) EffectiveMotionConfig() MotionConfig {
	if c.Motion != nil {
		return *c.Motion
	}
	return MotionConfig{}
}

func (c CameraConfig) EffectiveChunkDuration() time.Duration {
	if c.ChunkDuration != 0 {
		return time.Duration(c.ChunkDuration)
	}
	return DefaultChunkDuration
}

func (c CameraConfig) EffectiveReconnectInterval() time.Duration {
	if c.ReconnectInterval != 0 {
		return time.Duration(c.ReconnectInterval)
	}
	return DefaultReconnectInterval
}

func Load(path string) (Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Config{}, err
	}
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return Config{}, err
	}
	if v := os.Getenv("CAMERA_STORAGE_PATH"); v != "" {
		cfg.Storage.Path = v
	}
	if v := os.Getenv("CAMERA_TIMEZONE"); v != "" {
		cfg.Timezone = v
	}
	if v := os.Getenv("CAMERA_SERVER_JWT_SECRET"); v != "" {
		cfg.Server.JWTSecret = v
	}
	if cfg.Timezone == "" {
		cfg.Timezone = "UTC"
	}
	return cfg, nil
}
