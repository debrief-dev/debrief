//go:build linux

// Package autostart manages XDG autostart entries on Linux.
//
// Two-tier model (per the XDG Autostart spec):
//
//  1. System-wide entry (/etc/xdg/autostart/debrief.desktop) — installed by
//     deb/rpm/arch packages and owned by root. A regular user cannot modify
//     or delete it.
//
//  2. Per-user entry (~/.config/autostart/debrief.desktop) — written by the
//     app at runtime. When this file exists it takes precedence over the
//     system-wide entry.
//
// To disable autostart the app writes a per-user file with Hidden=true.
// This is the only way for a non-root user to suppress a system-wide entry.
// To re-enable, the per-user override is simply removed so the system-wide
// entry takes effect again. If no system-wide file exists (manual install),
// the per-user file is created/deleted directly.
package autostart

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const (
	desktopFileName  = "debrief.desktop"
	autostartDir     = "autostart"
	autostartDirPerm = 0o750
	desktopFilePerm  = 0o600

	// System-wide autostart location installed by deb/rpm/arch packages.
	systemAutostartDir = "/etc/xdg/autostart"
)

// Enable re-enables autostart. If a system-wide desktop file exists
// (installed by the package manager), it removes any per-user override
// so the system entry takes effect. Otherwise it creates a per-user entry.
func Enable() error {
	userPath, err := userAutostartPath()
	if err != nil {
		return err
	}

	if systemAutostartExists() {
		// Remove per-user override so the system-wide entry takes effect.
		if err := os.Remove(userPath); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("failed to remove per-user autostart override: %w", err)
		}

		return nil
	}

	// No system-wide file — create a per-user entry.
	return writeUserDesktopFile(userPath, false)
}

// Disable prevents the app from starting on login by writing a per-user
// desktop file with Hidden=true, which overrides any system-wide entry.
func Disable() error {
	userPath, err := userAutostartPath()
	if err != nil {
		return err
	}

	return writeUserDesktopFile(userPath, true)
}

// IsEnabled checks whether autostart is active. A per-user file takes
// precedence over the system-wide file.
func IsEnabled() (bool, error) {
	userPath, err := userAutostartPath()
	if err != nil {
		return false, err
	}

	if _, statErr := os.Stat(userPath); statErr == nil {
		hidden, readErr := desktopFileIsHidden(userPath)
		if readErr != nil {
			return false, readErr
		}

		return !hidden, nil
	}

	// No per-user file — fall back to system-wide.
	return systemAutostartExists(), nil
}

// writeUserDesktopFile creates the per-user autostart desktop entry.
// When hidden is true, the file contains Hidden=true which disables autostart.
func writeUserDesktopFile(path string, hidden bool) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, autostartDirPerm); err != nil {
		return fmt.Errorf("failed to create autostart directory: %w", err)
	}

	exePath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("failed to get executable path: %w", err)
	}

	content := "[Desktop Entry]\n" +
		"Type=Application\n" +
		"Name=Debrief\n" +
		"Exec=\"" + exePath + "\"\n" +
		"X-GNOME-Autostart-enabled=true\n" +
		"StartupNotify=false\n" +
		"Terminal=false\n"

	if hidden {
		content += "Hidden=true\n"
	}

	if err := os.WriteFile(path, []byte(content), desktopFilePerm); err != nil {
		return fmt.Errorf("failed to write autostart desktop file: %w", err)
	}

	return nil
}

// systemAutostartExists returns true when the package-managed system-wide
// autostart desktop file is present.
func systemAutostartExists() bool {
	_, err := os.Stat(filepath.Join(systemAutostartDir, desktopFileName))

	return err == nil
}

// desktopFileIsHidden reads a desktop file and returns true if it contains
// Hidden=true.
func desktopFileIsHidden(path string) (bool, error) {
	f, err := os.Open(filepath.Clean(path))
	if err != nil {
		return false, fmt.Errorf("failed to open desktop file: %w", err)
	}
	defer f.Close() //nolint:errcheck // close on read-only file

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.EqualFold(line, "Hidden=true") {
			return true, nil
		}
	}

	if err := scanner.Err(); err != nil {
		return false, fmt.Errorf("failed to read desktop file: %w", err)
	}

	return false, nil
}

func userAutostartPath() (string, error) {
	configDir := os.Getenv("XDG_CONFIG_HOME")
	if configDir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("failed to get home directory: %w", err)
		}

		configDir = filepath.Join(home, ".config")
	}

	return filepath.Join(configDir, autostartDir, desktopFileName), nil
}
