//go:build darwin

package autostart

import (
	"encoding/xml"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const (
	launchAgentLabel    = "com.debrief"
	plistName           = launchAgentLabel + ".plist"
	plistDir            = "Library/LaunchAgents"
	launchAgentsDirPerm = 0o750
	plistFilePerm       = 0o600
)

// Enable registers the app as a macOS LaunchAgent to start on login.
func Enable() error {
	exePath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("failed to get executable path: %w", err)
	}

	plistPath, err := launchAgentPath()
	if err != nil {
		return err
	}

	dir := filepath.Dir(plistPath)
	if err := os.MkdirAll(dir, launchAgentsDirPerm); err != nil {
		return fmt.Errorf("failed to create LaunchAgents directory: %w", err)
	}

	var escaped strings.Builder
	if err := xml.EscapeText(&escaped, []byte(exePath)); err != nil {
		return fmt.Errorf("failed to escape executable path: %w", err)
	}

	content := `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>Label</key>
    <string>` + launchAgentLabel + `</string>
    <key>ProgramArguments</key>
    <array>
        <string>` + escaped.String() + `</string>
    </array>
    <key>RunAtLoad</key>
    <true/>
</dict>
</plist>
`

	if err := os.WriteFile(plistPath, []byte(content), plistFilePerm); err != nil {
		return fmt.Errorf("failed to write LaunchAgent plist: %w", err)
	}

	return nil
}

// Disable removes the LaunchAgent plist to stop the app from starting on login.
func Disable() error {
	plistPath, err := launchAgentPath()
	if err != nil {
		return err
	}

	if err := os.Remove(plistPath); err != nil {
		if os.IsNotExist(err) {
			return nil
		}

		return fmt.Errorf("failed to remove LaunchAgent plist: %w", err)
	}

	return nil
}

// IsEnabled checks whether the LaunchAgent plist exists.
func IsEnabled() (bool, error) {
	plistPath, err := launchAgentPath()
	if err != nil {
		return false, err
	}

	_, err = os.Stat(plistPath)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}

		return false, fmt.Errorf("failed to check LaunchAgent plist: %w", err)
	}

	return true, nil
}

func launchAgentPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get home directory: %w", err)
	}

	return filepath.Join(home, plistDir, plistName), nil
}
