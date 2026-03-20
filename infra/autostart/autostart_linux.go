//go:build linux

package autostart

import (
	"fmt"
	"os"
	"path/filepath"
)

const (
	desktopFileName  = "debrief.desktop"
	autostartDir     = "autostart"
	autostartDirPerm = 0o750
	desktopFilePerm  = 0o600
)

// Enable creates an XDG autostart .desktop file so the app starts on login.
func Enable() error {
	exePath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("failed to get executable path: %w", err)
	}

	desktopPath, err := autostartFilePath()
	if err != nil {
		return err
	}

	dir := filepath.Dir(desktopPath)
	if err := os.MkdirAll(dir, autostartDirPerm); err != nil {
		return fmt.Errorf("failed to create autostart directory: %w", err)
	}

	content := "[Desktop Entry]\n" +
		"Type=Application\n" +
		"Name=Debrief\n" +
		"Exec=\"" + exePath + "\"\n" +
		"X-GNOME-Autostart-enabled=true\n" +
		"StartupNotify=false\n" +
		"Terminal=false\n"

	if err := os.WriteFile(desktopPath, []byte(content), desktopFilePerm); err != nil {
		return fmt.Errorf("failed to write autostart desktop file: %w", err)
	}

	return nil
}

// Disable removes the XDG autostart .desktop file.
func Disable() error {
	desktopPath, err := autostartFilePath()
	if err != nil {
		return err
	}

	if err := os.Remove(desktopPath); err != nil {
		if os.IsNotExist(err) {
			return nil
		}

		return fmt.Errorf("failed to remove autostart desktop file: %w", err)
	}

	return nil
}

// IsEnabled checks whether the XDG autostart .desktop file exists.
func IsEnabled() (bool, error) {
	desktopPath, err := autostartFilePath()
	if err != nil {
		return false, err
	}

	_, err = os.Stat(desktopPath)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}

		return false, fmt.Errorf("failed to check autostart desktop file: %w", err)
	}

	return true, nil
}

func autostartFilePath() (string, error) {
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
