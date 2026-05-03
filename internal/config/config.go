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

type Config struct {
	Debug    bool           `yaml:"debug"`
	Timezone string         `yaml:"timezone"`
	Log      LogConfig      `yaml:"log"`
	Server   ServerConfig   `yaml:"server"`
	Storage  StorageConfig  `yaml:"storage"`
	Defaults DefaultsConfig `yaml:"defaults"`
	Cameras  []CameraConfig `yaml:"cameras"`
}

type LogConfig struct {
	Output string `yaml:"output"`
	Path   string `yaml:"path"`
}

type ServerConfig struct {
	Port           int    `yaml:"port"`
	SegmentsPath   string `yaml:"segments_path"`
	RecordingsPath string `yaml:"recordings_path"`
	Username       string `yaml:"username"`
	Password       string `yaml:"password"`
}

type StorageConfig struct {
	Path string `yaml:"path"`
}

type DefaultsConfig struct {
	ChunkDuration     Duration `yaml:"chunk_duration"`
	ReconnectInterval Duration `yaml:"reconnect_interval"`
}

type CameraConfig struct {
	ID                string   `yaml:"id"`
	RTSPURL           string   `yaml:"rtsp_url"`
	ChunkDuration     Duration `yaml:"chunk_duration"`
	ReconnectInterval Duration `yaml:"reconnect_interval"`
}

func (c CameraConfig) EffectiveChunkDuration(defaults DefaultsConfig) time.Duration {
	if c.ChunkDuration != 0 {
		return time.Duration(c.ChunkDuration)
	}
	return time.Duration(defaults.ChunkDuration)
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
	if v := os.Getenv("STORAGE_PATH"); v != "" {
		cfg.Storage.Path = v
	}
	if v := os.Getenv("TIMEZONE"); v != "" {
		cfg.Timezone = v
	}
	if cfg.Timezone == "" {
		cfg.Timezone = "UTC"
	}
	return cfg, nil
}
