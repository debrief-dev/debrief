package config

import "time"

// AppVersion is the application version string.
const AppVersion = "0.1.0"

// SettingsVersion is the current settings schema version.
const SettingsVersion = 1

// DirPermissions is the permission mode for created directories (rwx for owner only).
const DirPermissions = 0o700

// FilePermissions is the permission mode for created files (rw for owner only).
const FilePermissions = 0o600

// ConfigDirectoryName is the name of the application's configuration directory
// inside the platform-specific config root (e.g., %APPDATA%/debrief on Windows).
const ConfigDirectoryName = "debrief"

// ConfigFileName is the name of the JSON configuration file.
const ConfigFileName = "config.json"

// LogFileName is the name of the application log file.
const LogFileName = "debrief.log"

// MaxHotkeyPreset is the maximum valid hotkey preset index.
// Must match hotkey.PresetCount - 1.
const MaxHotkeyPreset = 2

// PollingInterval is the fallback interval for polling history file changes
// when filesystem notifications (fsnotify) are unavailable.
const PollingInterval = 5 * time.Second
