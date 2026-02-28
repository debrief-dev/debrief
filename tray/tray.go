package tray

import "log"

// Initialize starts the system tray.
// This function is blocking and should be called in a goroutine.
func Initialize(windowSignalChan chan<- string, shouldQuit chan<- bool) {
	log.Println("Initializing system tray")

	onReady := func() {
		log.Println("System tray ready")
		SetupMenu(windowSignalChan, shouldQuit)
	}

	onExit := func() {
		OnExit()
	}

	initializePlatform(onReady, onExit)
}
