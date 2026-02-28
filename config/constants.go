package config

import "time"

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

// PollingInterval is the fallback interval for polling history file changes
// when filesystem notifications (fsnotify) are unavailable.
const PollingInterval = 5 * time.Second
