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

const DefaultChunkDuration = 5 * time.Minute
const DefaultReconnectInterval = 2 * time.Second

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
	Output     string `yaml:"output"`
	Path       string `yaml:"path"`
	MaxSizeMB  *int   `yaml:"max_size_mb"`
	MaxAgeDays *int   `yaml:"max_age_days"`
	MaxBackups *int   `yaml:"max_backups"`
	Compress   *bool  `yaml:"compress"`
}

// Defaults for log rotation, applied when the field is absent from the YAML.
// Pointers distinguish "absent" from an explicit zero (0 = unlimited in lumberjack).
const (
	DefaultLogMaxSizeMB  = 50
	DefaultLogMaxAgeDays = 30
	DefaultLogMaxBackups = 10
	DefaultLogCompress   = true
)

func (c LogConfig) MaxSizeMBOrDefault() int {
	if c.MaxSizeMB != nil {
		return *c.MaxSizeMB
	}
	return DefaultLogMaxSizeMB
}

func (c LogConfig) MaxAgeDaysOrDefault() int {
	if c.MaxAgeDays != nil {
		return *c.MaxAgeDays
	}
	return DefaultLogMaxAgeDays
}

func (c LogConfig) MaxBackupsOrDefault() int {
	if c.MaxBackups != nil {
		return *c.MaxBackups
	}
	return DefaultLogMaxBackups
}

func (c LogConfig) CompressOrDefault() bool {
	if c.Compress != nil {
		return *c.Compress
	}
	return DefaultLogCompress
}

type AdminConfig struct {
	Username string `yaml:"username"`
	Password string `yaml:"password"`
}

type ServerConfig struct {
	Port           int    `yaml:"port"`
	SegmentsPath   string `yaml:"segments_path"`
	RecordingsPath string `yaml:"recordings_path"`
	JWTSecret      string `yaml:"jwt_secret"`
}

type StorageConfig struct {
	Path string `yaml:"path"`
}

// MotionConfig holds per-camera motion detection settings (used by the DB layer).
type MotionConfig struct {
	Enabled              bool    `yaml:"enabled"`
	Threshold            float64 `yaml:"threshold"`
	FPS                  int     `yaml:"fps"`
	CooldownSeconds      int     `yaml:"cooldown_seconds"`
	CaptureWidth         int     `yaml:"capture_width"`
	CaptureHeight        int     `yaml:"capture_height"`
	PlaybackLeadSeconds  int     `yaml:"playback_lead_seconds"`
	PlaybackTrailSeconds int     `yaml:"playback_trail_seconds"`
}

// CameraConfig holds per-camera settings loaded from the database.
type CameraConfig struct {
	ID                string        `yaml:"id"`
	Name              string        `yaml:"name"`
	RTSPURL           string        `yaml:"rtsp_url"`
	ChunkDuration     Duration      `yaml:"chunk_duration"`
	ReconnectInterval Duration      `yaml:"reconnect_interval"`
	VideoCodec        string        `yaml:"video_codec"`
	HasAudio          *bool         `yaml:"has_audio"`
	Width             int           `yaml:"width"`
	Height            int           `yaml:"height"`
	DisplayOrder      int           `yaml:"display_order"`
	HLSVideoMode      string        `yaml:"hls_video_mode"`
	RecordVideoMode   string        `yaml:"record_video_mode"`
	HLSSegmentSeconds *int          `yaml:"hls_segment_seconds"`
	HLSListSize       *int          `yaml:"hls_list_size"`
	HLSDVRSeconds     *int          `yaml:"hls_dvr_seconds"`
	Motion            *MotionConfig `yaml:"motion"`
	RecordingEnabled  bool          `yaml:"recording_enabled"`
}

func (c CameraConfig) HLSSegmentSecondsOrDefault() int {
	if c.HLSSegmentSeconds != nil {
		return *c.HLSSegmentSeconds
	}
	return 2
}

func (c CameraConfig) HLSListSizeOrDefault() int {
	if c.HLSListSize != nil {
		return *c.HLSListSize
	}
	return 5
}

func (c CameraConfig) HLSDVRSecondsOrDefault() int {
	if c.HLSDVRSeconds != nil {
		return *c.HLSDVRSeconds
	}
	return 0
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
