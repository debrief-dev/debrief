package tray

import "log"

// Initialize starts the system tray.
// hotkeyHint is the display name of the active hotkey (e.g. "Ctrl+Shift+H"),
// shown in the tray menu tooltips.
// This function is blocking and should be called in a goroutine.
func Initialize(windowSignalChan chan<- string, shouldQuit chan<- bool, hotkeyHint string) {
	log.Println("Initializing system tray")

	onReady := func() {
		log.Println("System tray ready")
		SetupMenu(windowSignalChan, shouldQuit, hotkeyHint)
	}

	onExit := func() {
		OnExit()
	}

	initializePlatform(onReady, onExit)
}
