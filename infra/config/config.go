package config

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/debrief-dev/debrief/infra/platform"
)

// Config holds application settings persisted to disk.
type Config struct {
	Version int `json:"version"`

	HotkeyPreset int `json:"hotkeyPreset"` // Preset index (0, 1, or 2)
}

// DefaultConfig returns configuration with sensible defaults.
func DefaultConfig() *Config {
	return &Config{
		Version:      SettingsVersion,
		HotkeyPreset: 0,
	}
}

// LoadConfig reads configuration from disk.
// Returns default config when the file does not exist or cannot be parsed.
func LoadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(filepath.Clean(path))
	if err != nil {
		if os.IsNotExist(err) {
			log.Printf("Config file not found at %s, using defaults", path)
			return DefaultConfig(), nil
		}

		return nil, fmt.Errorf("failed to read config: %w", err)
	}

	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		log.Printf("Failed to parse config: %v, using defaults", err)
		return DefaultConfig(), nil
	}

	log.Printf("Loaded config from %s", path)

	return &cfg, nil
}

// SaveConfig writes configuration to disk, creating the directory if needed.
func (c *Config) SaveConfig(path string) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, DirPermissions); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	if err := os.WriteFile(path, data, FilePermissions); err != nil {
		return fmt.Errorf("failed to write config: %w", err)
	}

	log.Printf("Saved config to %s", path)

	return nil
}

// appBaseDir returns the platform-specific base directory for configuration
// files (e.g., %APPDATA% on Windows, $XDG_CONFIG_HOME or ~/.config on Unix).
func appBaseDir() string {
	if platform.IsWindows() {
		if appData := os.Getenv("APPDATA"); appData != "" {
			return appData
		}

		if dir, err := os.UserConfigDir(); err == nil {
			return dir
		}

		return os.TempDir()
	}

	if xdgConfig := os.Getenv("XDG_CONFIG_HOME"); xdgConfig != "" {
		return xdgConfig
	}

	if home, err := os.UserHomeDir(); err == nil {
		return filepath.Join(home, ".config")
	}

	return os.TempDir()
}

// AppDirectory returns the application's configuration directory and ensures
// it exists on disk. All config and log files live inside this directory.
func AppDirectory() (string, error) {
	dir := filepath.Join(appBaseDir(), ConfigDirectoryName)
	if err := os.MkdirAll(dir, DirPermissions); err != nil {
		return "", fmt.Errorf("failed to create app directory %s: %w", dir, err)
	}

	return dir, nil
}

// ConfigPath returns the full path to the configuration file.
// Creates the parent directory if it does not exist.
func ConfigPath() (string, error) {
	dir, err := AppDirectory()
	if err != nil {
		return "", err
	}

	return filepath.Join(dir, ConfigFileName), nil
}

// LogPath returns the full path to the log file.
// Creates the parent directory if it does not exist.
func LogPath() (string, error) {
	dir, err := AppDirectory()
	if err != nil {
		return "", err
	}

	return filepath.Join(dir, LogFileName), nil
}
