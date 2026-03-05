package main

import (
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"sync/atomic"
	"time"

	"gioui.org/app"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/text"
	"gioui.org/widget"
	"gioui.org/widget/material"
	appstate "github.com/debrief-dev/debrief/app"
	"github.com/debrief-dev/debrief/data/model"
	"github.com/debrief-dev/debrief/data/shell"
	"github.com/debrief-dev/debrief/font"
	"github.com/debrief-dev/debrief/infra/config"
	"github.com/debrief-dev/debrief/infra/hotkey"
	"github.com/debrief-dev/debrief/infra/platform"
	"github.com/debrief-dev/debrief/infra/tray"
	"github.com/debrief-dev/debrief/infra/window"
	"github.com/debrief-dev/debrief/ui"
	"github.com/getlantern/golog"
)

// Window lifecycle coordination channels
type windowLifecycle struct {
	ready     chan struct{} // Signals when window OS handle is ready
	destroyed chan struct{} // Signals when window is fully destroyed
}

func main() {
	pprofEnabled := flag.Bool("pprof", false, "start pprof server on localhost:6060")

	flag.Parse()

	// Redirect getlantern/golog outputs away from stdout/stderr.
	// golog's init() sets os.Stderr/os.Stdout as outputs; on Windows with
	// -H windowsgui any write to those handles causes a console window flash.
	golog.SetOutputs(io.Discard, io.Discard)

	// Setup logging
	logFile, err := setupLogging()
	if err != nil {
		log.Printf("Warning: Failed to setup file logging: %v", err)
	} else {
		defer func() {
			if err := logFile.Close(); err != nil {
				log.Printf("Error closing log file: %v", err)
			}
		}()

		log.Printf("Debrief started - version %s", config.AppVersion)
	}

	// Start pprof server for profiling (only when --pprof flag is passed)
	if *pprofEnabled {
		go func() {
			log.Println("Starting pprof server on http://localhost:6060")
			//nolint:gosec // pprof server is for development profiling only
			if err := http.ListenAndServe("localhost:6060", nil); err != nil {
				log.Printf("pprof server error: %v", err)
			}
		}()
	}

	// Create window signal channel for communication between tray/hotkey and window
	// receiver drains faster than human can produce signals - 1 is sufficient
	windowSignalChan := make(chan string, 1)

	// Quit signal - only true when explicitly quitting from tray
	shouldQuit := make(chan bool, 1)

	// Initialize system tray in a goroutine (blocking call).
	// On macOS, we wait for the first window to be ready so that Gio's NSApp
	// run loop is active before creating the NSStatusItem.
	trayReady := make(chan struct{}, 1)

	go func() {
		if platform.IsMacOS() {
			log.Println("macOS: waiting for window before initializing tray")
			<-trayReady
		}

		log.Println("Starting system tray")
		tray.Initialize(windowSignalChan, shouldQuit)
	}()

	// Initialize window controller
	windowController := window.NewController(ui.WindowTitle)

	// Shared window reference for signal handler (protected by mutex)
	var (
		currentWindow   *app.Window
		currentWindowMu sync.RWMutex
		// signalDirty is set by the signal handler goroutine when it calls
		// Invalidate(). The event loop checks and clears this flag after
		// win.Event() returns, re-invalidating if the original was coalesced.
		signalDirty atomic.Bool
	)

	configPath, err := config.ConfigPath()
	if err != nil {
		log.Printf("Error resolving config path: %v", err)
	}

	cfg, err := config.LoadConfig(configPath)
	if err != nil {
		log.Printf("Error loading config: %v, using defaults", err)

		cfg = config.DefaultConfig()
	}

	sourceManager := shell.NewShellManager()

	// Build hotkey presets once; reused for initial registration and UI settings
	hotkeyPresets := hotkey.BuildPresets()

	// Look up the configured preset for initial hotkey registration
	initPreset := hotkeyPresets[cfg.HotkeyPreset]
	hotkeyMods := initPreset.Modifiers
	hotkeyKey := initPreset.Key

	// Register global hotkey (from config or default)
	hotkeyManager := hotkey.NewManager(windowSignalChan)

	// Channel to signal when first window is ready for hotkey registration
	firstWindowReady := make(chan struct{}, 1)

	// Start hotkey registration goroutine - waits for window to be ready
	go registerHotkey(firstWindowReady, hotkeyManager, hotkeyMods, hotkeyKey)

	var (
		startHidden   bool
		startHiddenMu sync.Mutex
	)

	// recreateNotify is used on Wayland to wake the window creation loop
	// when a "show" or "toggle" signal arrives while the window is closed.
	// On Wayland, hiding destroys the window (because un-minimize is
	// impossible), so the creation loop blocks on this channel until a
	// show request arrives instead of immediately recreating in hidden state.
	recreateNotify := make(chan struct{}, 1)

	// Central signal handler - routes signals appropriately.
	// This is the ONLY goroutine that reads from windowSignalChan.
	go func() {
		for signal := range windowSignalChan {
			log.Printf("Signal handler: Received '%s' signal", signal)

			if windowController.IsClosed() {
				if signal != "show" && signal != "toggle" {
					log.Printf("Signal handler: Window closed, ignoring '%s'", signal)
					continue
				}

				log.Println("Signal handler: Will show window when recreation completes")
				startHiddenMu.Lock()

				startHidden = false

				startHiddenMu.Unlock()

				// Wake the creation loop on Wayland (buffered, non-blocking)
				select {
				case recreateNotify <- struct{}{}:
				default:
				}

				continue
			}

			// Window is open - handle signal normally
			if err := windowController.HandleSignal(signal); err != nil {
				log.Printf("Signal handler: Error: %v", err)
			}

			signalDirty.Store(true)
			currentWindowMu.RLock()

			if currentWindow != nil {
				currentWindow.Invalidate()
			}

			currentWindowMu.RUnlock()
		}
	}()

	// Track if this is the first window (for hotkey registration)
	isFirstWindow := true

	quitApp := func() {
		// os.Exit does not run deferred functions, so flush the log file
		// explicitly to avoid losing buffered entries.
		if logFile != nil {
			if err := logFile.Sync(); err != nil {
				log.Printf("Error syncing log file: %v", err)
			}

			if err := logFile.Close(); err != nil {
				log.Printf("Error closing log file: %v", err)
			}
		}

		os.Exit(0)
	}

	// Dedicated goroutine ensures the tray quit signal is processed
	// immediately, even while the window event loop is running.
	go func() {
		<-shouldQuit
		log.Println("Quit signal received from tray, shutting down")
		quitApp()
	}()

	go func() {
		// Detect and enable sources BEFORE the first window is created.
		// This runs in a background goroutine (not the main/UI thread), so it
		// won't block app.Main(). It must complete before run() starts because
		// run() immediately loads history from enabled sources.
		// Runs only once; subsequent window recreations skip detection since
		// sources are already populated.
		sourceManager.DetectShells()

		if err := cfg.SaveConfig(configPath); err != nil {
			log.Printf("Warning: Failed to save config: %v", err)
		}

		for {
			// Create lifecycle channels for this window
			lifecycle := &windowLifecycle{
				ready:     make(chan struct{}),
				destroyed: make(chan struct{}),
			}

			win := new(app.Window)
			win.Option(app.Title(ui.WindowTitle))
			win.Option(app.Decorated(false))
			win.Option(app.MinSize(ui.MinWidth, ui.MinHeight))
			log.Println("Window created, starting main loop")

			// Update shared window reference
			currentWindowMu.Lock()

			currentWindow = win

			currentWindowMu.Unlock()

			// Set Gio window reference for platform-specific show/hide operations
			windowController.SetWindow(win)

			// Mark window as opened so controller knows it exists
			windowController.MarkOpened()

			// Check if we should start hidden (after close button was clicked)
			startHiddenMu.Lock()

			shouldStartHidden := startHidden
			startHidden = false // Reset for next time

			startHiddenMu.Unlock()

			// Start goroutine to wait for window handle and act accordingly
			if shouldStartHidden {
				go waitAndHideWindow(lifecycle, windowController)
			} else if isFirstWindow {
				go waitAndSignalReady(lifecycle, firstWindowReady, trayReady)

				isFirstWindow = false
			}

			// Run the window event loop
			run(win, windowController, windowSignalChan, lifecycle, cfg, configPath, sourceManager, hotkeyManager, hotkeyPresets, quitApp, *pprofEnabled, &signalDirty)

			// Window closed normally (close button clicked)
			log.Println("Window closed, waiting for cleanup before recreation")

			// Clear window reference
			currentWindowMu.Lock()

			currentWindow = nil

			currentWindowMu.Unlock()

			// Clear Gio window reference from controller
			windowController.SetWindow(nil)

			// Set flag to start next window hidden
			startHiddenMu.Lock()

			startHidden = true

			startHiddenMu.Unlock()

			// Wait for window to be fully destroyed
			<-lifecycle.destroyed
			log.Println("Window destruction complete, recreating")

			// On Wayland, hiding destroys the window (because there is no
			// way to un-minimize). Instead of immediately recreating in
			// hidden state (which would loop forever), wait for an explicit
			// show/toggle signal before creating the new window.
			if window.IsWaylandSession() {
				waitForWaylandShow(&startHiddenMu, &startHidden, recreateNotify, shouldQuit, quitApp)
			}

			// Continue loop to recreate
		}
	}()

	// Start the main event loop (blocking)
	// On macOS, mainthread.Init already ensures we're on the main thread,
	// so we call app.Main directly (no need for mainthread.Call)
	app.Main()
}

func setupLogging() (*os.File, error) {
	logPath, err := config.LogPath()
	if err != nil {
		return nil, fmt.Errorf("failed to resolve log path: %w", err)
	}

	logFile, err := os.OpenFile(filepath.Clean(logPath), os.O_TRUNC|os.O_CREATE|os.O_WRONLY, config.FilePermissions)
	if err != nil {
		return nil, fmt.Errorf("failed to open log file: %w", err)
	}

	// Configure log to write to file only (stdout causes console flash on Windows)
	log.SetOutput(logFile)
	log.SetFlags(log.Ldate | log.Ltime | log.Lshortfile)

	log.Printf("Logging initialized - log file: %s", logPath)

	return logFile, nil
}

// waitForWindowHandle waits for the window handle to be ready.
// Returns true when the window handle is ready, false if the window was
// destroyed before the handle became available.
func waitForWindowHandle(readyChan, destroyedChan <-chan struct{}) bool {
	select {
	case <-readyChan:
		log.Println("waitForWindowHandle: Window handle is ready")
		return true
	case <-destroyedChan:
		log.Println("waitForWindowHandle: Window destroyed before handle was ready")
		return false
	}
}

// registerHotkey waits for the first window to be ready, then registers
// the global hotkey from config values.
func registerHotkey(ready <-chan struct{}, mgr *hotkey.Manager, modStrs []string, keyStr string) {
	log.Println("Hotkey registration: Waiting for window to be ready")
	<-ready
	log.Println("Hotkey registration: Window ready, registering hotkey")

	mods := make([]hotkey.Modifier, 0, len(modStrs))

	for _, modStr := range modStrs {
		mod, err := hotkey.StringToModifier(modStr)
		if err != nil {
			log.Printf("Warning: Invalid modifier '%s', skipping", modStr)
			continue
		}

		mods = append(mods, mod)
	}

	key, err := hotkey.StringToKey(keyStr)
	if err != nil {
		log.Printf("Warning: Invalid key '%s', using default '%s'", keyStr, hotkey.DefaultKey)

		key, err = hotkey.StringToKey(hotkey.DefaultKey)
		if err != nil {
			log.Printf("Error: Failed to parse default key '%s': %v", hotkey.DefaultKey, err)
			return
		}
	}

	if err := mgr.UpdateHotkey(mods, key, modStrs, keyStr); err != nil {
		log.Printf("Warning: Hotkey registration failed: %v", err)
		log.Println("Continuing without global hotkey support")
	}
}

// waitAndHideWindow waits for the window OS handle to be created, then hides
// the window. Used when recreating a window that should start hidden.
func waitAndHideWindow(lifecycle *windowLifecycle, wc *window.Controller) {
	log.Println("Window: Waiting for OS handle creation to hide window")

	if !waitForWindowHandle(lifecycle.ready, lifecycle.destroyed) {
		log.Println("Window: Window destroyed before handle ready, skipping hide")
		return
	}

	if wc.Hide() == nil {
		log.Println("Window recreated and hidden")
	}
}

// waitAndSignalReady waits for the window OS handle to be created, then
// signals hotkey registration and tray initialization.
// firstWindowReady is bidirectional so it can be closed on failure,
// unblocking any goroutine waiting on the receive end.
func waitAndSignalReady(lifecycle *windowLifecycle, firstWindowReady chan struct{}, trayReady chan<- struct{}) {
	log.Println("Window: Waiting for OS handle creation for hotkey registration")

	if !waitForWindowHandle(lifecycle.ready, lifecycle.destroyed) {
		log.Println("Window: ERROR - Window destroyed before handle ready, hotkey registration will not occur")
		close(firstWindowReady)

		return
	}

	log.Println("Window: OS handle ready, signaling hotkey registration")

	select {
	case firstWindowReady <- struct{}{}:
		log.Println("Window: Successfully signaled hotkey registration")
	default:
		log.Println("Window: WARNING - Failed to signal hotkey registration (channel full)")
	}

	// Signal tray initialization (macOS needs NSApp run loop active)
	select {
	case trayReady <- struct{}{}:
		log.Println("Window: Successfully signaled tray initialization")
	default:
	}
}

// waitForWaylandShow blocks until a show signal arrives or quit is requested.
// On Wayland, hiding destroys the window, so we must wait for an explicit
// show request before recreating instead of looping in hidden state forever.
func waitForWaylandShow(hiddenMu *sync.Mutex, hidden *bool, recreateNotify <-chan struct{}, shouldQuit <-chan bool, quitApp func()) {
	hiddenMu.Lock()

	needsWait := *hidden

	hiddenMu.Unlock()

	if !needsWait {
		return
	}

	log.Println("Wayland: Waiting for show request before recreating window")

	select {
	case <-recreateNotify:
		log.Println("Wayland: Show request received, recreating window")
	case <-shouldQuit:
		log.Println("Wayland: Quit signal received while waiting")
		quitApp()
	}

	hiddenMu.Lock()

	*hidden = false

	hiddenMu.Unlock()
}

func run(
	win *app.Window,
	windowController *window.Controller,
	windowSignalChan chan<- string,
	lifecycle *windowLifecycle,
	cfg *config.Config,
	configPath string,
	sourceManager *shell.ShellManager,
	hotkeyManager *hotkey.Manager,
	hotkeyPresets []hotkey.Preset,
	quitApp func(),
	pprofEnabled bool,
	signalDirty *atomic.Bool,
) {
	// Defer signaling that window is destroyed
	defer func() {
		log.Println("Window event loop ended, signaling destruction")
		close(lifecycle.destroyed)
	}()

	// Track if we've signaled window ready (for first frame)
	windowHandleSignaled := false

	theme := material.NewTheme()
	theme.Shaper = text.NewShaper(
		text.NoSystemFonts(),
		text.WithCollection(font.Collection()),
	)

	var ops op.Ops

	// Initialize State
	appState := &appstate.State{
		Window:        win,
		Config:        cfg,
		ConfigPath:    configPath,
		SourceManager: sourceManager,
		HideWindowFunc: func() {
			sendHide := func() {
				select {
				case windowSignalChan <- "hide_and_restore":
					log.Println("Hide and restore signal sent to window controller")
				default:
					log.Println("Warning: Window signal channel full, hide signal dropped")
				}
			}

			if platform.IsLinux() {
				// On Linux, delay hide to let Gio process the pending clipboard.WriteCmd.
				// The write is async (queued in ops) and needs the window alive to complete.
				time.AfterFunc(window.LinuxClipboardDelay, sendHide)
			} else {
				sendHide()
			}
		},
		QuitFunc: func() {
			log.Println("Ctrl+Q pressed - exiting application")
			quitApp()
		},

		Commands: appstate.CommandsState{
			List:           widget.List{List: layout.List{Axis: layout.Vertical}},
			SelectedIndex:  -1,
			HoveredIndex:   -1,
			NeedInitialSel: true, // Auto-select last command on first load
			MetadataCache:  make(map[*model.CommandEntry]string),
		},

		Tabs: appstate.TabState{
			Current: model.TabCommands,
		},

		Tree: appstate.TreeState{
			List:            widget.List{List: layout.List{Axis: layout.Vertical}},
			SelectedNode:    -1,
			NeedInitialSel:  true, // Auto-select last tree node on first load
			HoveredNode:     -1,
			RebuildChan:     make(chan struct{}, 1), // Buffered(1): coalesces rebuild requests
			RebuildDone:     make(chan struct{}),    // Closed-and-recreated broadcast notification
			RebuildShutdown: make(chan struct{}),    // Unbuffered: signal worker to stop
		},

		Stats: appstate.StatsState{
			List:            widget.List{List: layout.List{Axis: layout.Vertical}},
			SelectedIndex:   -1,
			HoveredIndex:    -1,
			RebuildChan:     make(chan struct{}, 1), // Buffered(1): coalesces rebuild requests
			RebuildShutdown: make(chan struct{}),    // Unbuffered: signal worker to stop
		},

		Hotkeys: appstate.HotkeyState{
			Manager:          hotkeyManager,
			Presets:          hotkeyPresets,
			PresetClickables: make([]widget.Clickable, hotkey.PresetCount),
			SelectedPresetID: cfg.HotkeyPreset,
		},

		ShellFilter: nil,                                // nil = show all shells
		ShellBadges: make(map[string]*widget.Clickable), // Initialize badge widgets map

		SettingsList: widget.List{List: layout.List{Axis: layout.Vertical}},

		StoreShutdown: make(chan struct{}), // Unbuffered: signal store/polling goroutine to stop
	}

	// Configure search editor
	// NOTE: SingleLine = true is intentionally NOT set. On 64-bit systems,
	// Gio's SingleLine mode passes math.MaxInt to fixed.I(), which overflows
	// int32 to a negative maxWidth, causing the text shaper to line-break after
	// every character. Height is constrained in bar_search.go instead.
	appState.SearchEditor.Submit = false

	// Start background workers scoped to this window's lifetime.
	// Each worker selects on its per-window shutdown channel
	// (StoreShutdown, TreeRebuildShutdown, StatsRebuildShutdown),
	// so they stop cleanly when the window is closed.
	ui.StartBackgroundParser(appState)
	ui.StartTreeRebuildWorker(appState)
	ui.StartStatsRebuildWorker(appState)

	// Request initial tree rebuild
	ui.RequestTreeRebuild(appState)

	// Track previous window mode to detect external un-minimize (e.g. taskbar click).
	// Only Minimized→non-Minimized transitions trigger SyncVisible(true).
	// This avoids false positives on Windows/macOS where SW_HIDE/orderOut
	// don't change Gio's Mode (stays Windowed), which would immediately
	// re-mark the window as visible after hiding.
	prevWindowMode := app.Windowed

	// Per-frame allocation tracking.
	// Measures ALL allocations (UI + background goroutines) between frames.
	// Gated behind --pprof because ReadMemStats stops the world.
	var (
		frameCount  uint64
		totalAllocs uint64
		totalBytes  uint64
		lastStats   runtime.MemStats
	)

	if pprofEnabled {
		runtime.ReadMemStats(&lastStats)
	}

	for {
		e := win.Event()

		// Re-invalidate if a background goroutine flagged dirty state.
		// Gio's Invalidate() is coalesced: between when it fires and when
		// nextEvent() re-enables it, additional calls are no-ops. A background
		// goroutine that calls MarkDirty() during frame processing (when
		// mayInvalidate is false) will have its Invalidate() silently dropped.
		// By the time we reach here, nextEvent() has run and re-enabled
		// mayInvalidate, so this Invalidate() will succeed and ensure a new
		// FrameEvent is delivered.
		if appState.Dirty.Swap(false) || signalDirty.Swap(false) {
			win.Invalidate()
		}

		switch ev := e.(type) {
		case app.DestroyEvent:
			if ev.Err != nil {
				log.Printf("Window DestroyEvent with error: %v", ev.Err)
			}

			// Close button clicked - minimize to tray instead of quit
			log.Println("Window close button clicked, minimizing to tray (app continues running)")

			// Signal rebuild workers to shutdown for this window cycle
			close(appState.Tree.RebuildShutdown)
			close(appState.Stats.RebuildShutdown)
			close(appState.StoreShutdown)

			// Mark as closed for signal handling
			// NOTE: Don't try to hide() - window is already being destroyed by OS
			windowController.MarkClosed()

			// Exit run() to allow immediate recreation in hidden state
			return

		case app.ConfigEvent:
			// Detect external un-minimize (e.g. user clicks taskbar on Wayland).
			// Only sync visibility on Minimized → non-Minimized transition.
			// Avoids false positives on Windows/macOS where hiding uses
			// SW_HIDE/orderOut (Mode stays Windowed, never becomes Minimized).
			newMode := ev.Config.Mode
			if prevWindowMode == app.Minimized && newMode != app.Minimized {
				windowController.SyncVisible(true)
			}

			prevWindowMode = newMode

		case app.FrameEvent:
			// On first frame, signal that window handle is ready
			if !windowHandleSignaled {
				windowHandleSignaled = true

				log.Println("First frame rendered, signaling window handle ready")
				close(lifecycle.ready)
			}

			gtx := app.NewContext(&ops, ev)
			ui.RenderFrame(gtx, appState, theme)
			ev.Frame(gtx.Ops)

			// Process deferred hotkey update in a goroutine.
			// On macOS the hotkey library uses dispatch_sync(main_queue) which
			// deadlocks when called from the main thread (Gio's event loop).
			// Running in a goroutine lets the FrameEvent handler return first,
			// freeing the main queue for dispatch_sync to succeed.
			if appState.Hotkeys.NeedsUpdate {
				appState.Hotkeys.NeedsUpdate = false
				go ui.ProcessHotkeyUpdate(appState)
			}

			// Log allocation stats
			if pprofEnabled {
				var currentStats runtime.MemStats
				runtime.ReadMemStats(&currentStats)

				frameAllocs := currentStats.Mallocs - lastStats.Mallocs
				frameBytes := currentStats.TotalAlloc - lastStats.TotalAlloc

				frameCount++
				totalAllocs += frameAllocs
				totalBytes += frameBytes
				lastStats = currentStats

				if frameCount%60 == 0 {
					log.Printf("[ALLOC] Frame %d - This: %d allocs (%d bytes) | Total: %d allocs (%d bytes) | Goroutines: %d",
						frameCount, frameAllocs, frameBytes, totalAllocs, totalBytes, runtime.NumGoroutine())
				}
			}
		}
	}
}
