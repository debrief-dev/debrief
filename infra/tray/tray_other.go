//go:build !darwin

package tray

import "fyne.io/systray"

// initializePlatform sets up the system tray on Windows and Linux.
// systray.Run is a blocking call that starts the native loop.
func initializePlatform(onReady, onExit func()) {
	systray.Run(onReady, onExit)
}
