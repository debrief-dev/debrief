//go:build darwin

package tray

import (
	"log"

	"fyne.io/systray"
)

// initializePlatform sets up the system tray on macOS.
// Gio owns NSApplication and its delegate, so systray.Run() would conflict.
// RunWithExternalLoop returns start/end callbacks that create the NSStatusItem
// directly without touching the app delegate.
// The start callback must run on the main thread because NSStatusItem creation
// requires it (NSWindow must be instantiated on the main thread).
func initializePlatform(onReady, onExit func()) {
	log.Println("macOS: using systray.RunWithExternalLoop")

	start, _ := systray.RunWithExternalLoop(onReady, onExit)

	// Dispatch start() to the main thread via dispatch_async.
	// NSStatusItem internally creates NSStatusBarWindow which requires the main thread.
	dispatchStartOnMainThread(start)

	// RunWithExternalLoop is non-blocking, so block to keep the goroutine alive.
	select {}
}
