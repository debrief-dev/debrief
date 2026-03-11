//go:build windows

package autostart

import (
	"errors"
	"fmt"
	"os"

	"golang.org/x/sys/windows/registry"
)

const (
	registryPath = `SOFTWARE\Microsoft\Windows\CurrentVersion\Run`
	valueName    = "Debrief"
)

// Enable registers the app to start on login via the Windows registry.
func Enable() error {
	exePath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("failed to get executable path: %w", err)
	}

	key, _, err := registry.CreateKey(registry.CURRENT_USER, registryPath, registry.SET_VALUE)
	if err != nil {
		return fmt.Errorf("failed to open registry key: %w", err)
	}
	defer key.Close() //nolint:errcheck // registry key close errors are non-actionable

	if err := key.SetStringValue(valueName, exePath); err != nil {
		return fmt.Errorf("failed to set registry value: %w", err)
	}

	return nil
}

// Disable removes the app from login startup in the Windows registry.
func Disable() error {
	key, err := registry.OpenKey(registry.CURRENT_USER, registryPath, registry.SET_VALUE)
	if err != nil {
		return fmt.Errorf("failed to open registry key: %w", err)
	}
	defer key.Close() //nolint:errcheck // registry key close errors are non-actionable

	if err := key.DeleteValue(valueName); err != nil {
		if errors.Is(err, registry.ErrNotExist) {
			return nil
		}

		return fmt.Errorf("failed to delete registry value: %w", err)
	}

	return nil
}

// IsEnabled checks whether the app is registered for login startup.
func IsEnabled() (bool, error) {
	key, err := registry.OpenKey(registry.CURRENT_USER, registryPath, registry.QUERY_VALUE)
	if err != nil {
		return false, nil
	}
	defer key.Close() //nolint:errcheck // registry key close errors are non-actionable

	_, _, err = key.GetStringValue(valueName)
	if err != nil {
		if errors.Is(err, registry.ErrNotExist) {
			return false, nil
		}

		return false, fmt.Errorf("failed to read registry value: %w", err)
	}

	return true, nil
}
