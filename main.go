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
	"gioui.org/io/system"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/text"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"
	"github.com/getlantern/golog"

	appstate "github.com/debrief-dev/debrief/app"
	"github.com/debrief-dev/debrief/data/model"
	"github.com/debrief-dev/debrief/data/shell"
	"github.com/debrief-dev/debrief/infra/autostart"
	"github.com/debrief-dev/debrief/infra/config"
	"github.com/debrief-dev/debrief/infra/hotkey"
	"github.com/debrief-dev/debrief/infra/instance"
	"github.com/debrief-dev/debrief/infra/platform"
	"github.com/debrief-dev/debrief/infra/tray"
	"github.com/debrief-dev/debrief/infra/window"
	"github.com/debrief-dev/debrief/ui"
	"github.com/debrief-dev/debrief/ui/font"
)

// Window lifecycle coordination channels
type windowLifecycle struct {
	ready     chan struct{} // Signals when window OS handle is ready
	destroyed chan struct{} // Signals when window is fully destroyed
}

// savedUIState holds UI state preserved across window recreations.
// On Wayland, hiding destroys and recreates the window; this prevents
// losing search text, selected entity, active tab, and shell filter.
type savedUIState struct {
	SearchQuery      string
	ActiveTab        model.Tab
	ShellFilter      map[model.Shell]bool
	SelectedNodePath string                    // Tree tab: path-based selection survives rebuilds
	SelectedCmd      string                    // Commands tab: command text for selection restoration
	SelectedStatText string                    // Statistics tab: selected item text for restoration
	SelectedStatKind appstate.StatsRestoreKind // Which statistics list the item belongs to
}

func main() {
	pprofEnabled := flag.Bool("pprof", false, "start pprof server on localhost:6060")
	startHiddenFlag := flag.Bool("hidden", false, "start minimized to system tray (used by autostart)")

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

	// Create window signal channel for communication between tray/hotkey and window
	// receiver drains faster than human can produce signals - 1 is sufficient
	windowSignalChan := make(chan string, 1)

	// Single-instance check: if another instance is running, signal it to show and exit.
	acquired, cleanupInstance := instance.TryAcquire(windowSignalChan)
	if !acquired {
		log.Println("Another instance is already running; signaled it to show. Exiting.")
		return
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

	// Quit signal - only true when explicitly quitting from tray
	shouldQuit := make(chan bool, 1)

	configPath, cfg, err := loadConfig()
	if err != nil {
		log.Printf("Config: %v, using defaults", err)
	}

	// Build hotkey presets once; reused for initial registration, UI settings, and tray
	hotkeyPresets := hotkey.BuildPresets()

	// Look up the configured preset for initial hotkey registration
	initPreset := hotkeyPresets[cfg.HotkeyPreset]
	hotkeyMods := initPreset.Modifiers
	hotkeyKey := initPreset.Key

	// Pre-compute hint strings with the initial hotkey display name
	ui.RebuildHints(initPreset.DisplayName)

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
		tray.Initialize(windowSignalChan, shouldQuit, initPreset.DisplayName)
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

	sourceManager := shell.NewShellManager()

	// Register global hotkey (from config or default)
	hotkeyManager := hotkey.NewManager(windowSignalChan)

	// Channel to signal when first window is ready for hotkey registration
	firstWindowReady := make(chan struct{}, 1)

	// Start hotkey registration goroutine - waits for window to be ready
	go registerHotkey(firstWindowReady, hotkeyManager, hotkeyMods, hotkeyKey)

	var (
		// Start hidden only when launched with --hidden flag (set by autostart).
		startHidden   = *startHiddenFlag
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
		// Clean up single-instance socket before exiting.
		if cleanupInstance != nil {
			cleanupInstance()
		}

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

		var saved *savedUIState

		for {
			// Create lifecycle channels for this window
			lifecycle := &windowLifecycle{
				ready:     make(chan struct{}),
				destroyed: make(chan struct{}),
			}

			// Check if we should start hidden (autostart or after close button)
			startHiddenMu.Lock()

			shouldStartHidden := startHidden
			startHidden = false // Reset for next time

			startHiddenMu.Unlock()

			// When starting hidden, don't create a window at all.
			// Signal hotkey/tray so the user can trigger "show", then
			// block until a show signal arrives.
			if shouldStartHidden {
				// Mark controller as closed so the signal handler routes
				// show/toggle signals through recreateNotify.
				windowController.MarkClosed()

				if isFirstWindow {
					signalReady(firstWindowReady, trayReady)

					isFirstWindow = false
				}

				log.Println("Window: Starting hidden, waiting for show signal")

				select {
				case <-recreateNotify:
					log.Println("Window: Show signal received, creating window")
				case <-shouldQuit:
					log.Println("Window: Quit signal while hidden")
					quitApp()
				}
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

			// Start goroutine to wait for window handle and signal
			// hotkey registration + tray init on first window.
			if isFirstWindow {
				go waitAndSignalReady(lifecycle, firstWindowReady, trayReady)

				isFirstWindow = false
			}

			// Run the window event loop (returns saved UI state for restoration)
			saved = run(&runParams{
				win:              win,
				windowController: windowController,
				windowSignalChan: windowSignalChan,
				lifecycle:        lifecycle,
				cfg:              cfg,
				configPath:       configPath,
				sourceManager:    sourceManager,
				hotkeyManager:    hotkeyManager,
				hotkeyPresets:    hotkeyPresets,
				quitApp:          quitApp,
				pprofEnabled:     *pprofEnabled,
				signalDirty:      &signalDirty,
				saved:            saved,
			})

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

// signalReady signals hotkey registration and tray initialization immediately,
// without waiting for a window handle. Used when starting hidden (no window).
func signalReady(firstWindowReady chan struct{}, trayReady chan<- struct{}) {
	select {
	case firstWindowReady <- struct{}{}:
		log.Println("Window: Signaled hotkey registration (no window)")
	default:
	}

	select {
	case trayReady <- struct{}{}:
		log.Println("Window: Signaled tray initialization (no window)")
	default:
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

// runParams groups all parameters for the window event loop.
type runParams struct {
	win              *app.Window
	windowController *window.Controller
	windowSignalChan chan<- string
	lifecycle        *windowLifecycle
	cfg              *config.Config
	configPath       string
	sourceManager    *shell.ShellManager
	hotkeyManager    *hotkey.Manager
	hotkeyPresets    []hotkey.Preset
	quitApp          func()
	pprofEnabled     bool
	signalDirty      *atomic.Bool
	saved            *savedUIState
}

func run(p *runParams) *savedUIState {
	// Alias frequently-used params for brevity.
	win := p.win
	cfg := p.cfg
	// Defer signaling that window is destroyed
	defer func() {
		log.Println("Window event loop ended, signaling destruction")
		close(p.lifecycle.destroyed)
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
		ConfigPath:    p.configPath,
		SourceManager: p.sourceManager,
		HideWindowFunc: func() {
			sendHide := func() {
				select {
				case p.windowSignalChan <- "hide_and_restore":
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
			p.quitApp()
		},

		Commands: appstate.CommandsState{
			List:           widget.List{List: layout.List{Axis: layout.Vertical, ScrollToEnd: true}},
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
			Manager:          p.hotkeyManager,
			Presets:          p.hotkeyPresets,
			PresetClickables: make([]widget.Clickable, hotkey.PresetCount),
			SelectedPresetID: cfg.HotkeyPreset,
			ResultChan:       make(chan appstate.HotkeyResult, 1),
		},

		ShellFilter: nil,                                // nil = show all shells
		ShellBadges: make(map[string]*widget.Clickable), // Initialize badge widgets map

		SettingsList:        widget.List{List: layout.List{Axis: layout.Vertical}},
		AutoStartEnabled:    syncAutoStartState(cfg),
		AutoStartResultChan: make(chan appstate.AutostartResult, 1),

		StoreShutdown: make(chan struct{}), // Unbuffered: signal store/polling goroutine to stop
	}

	// Configure search editor
	// NOTE: SingleLine = true is intentionally NOT set. On 64-bit systems,
	// Gio's SingleLine mode passes math.MaxInt to fixed.I(), which overflows
	// int32 to a negative maxWidth, causing the text shaper to line-break after
	// every character. Height is constrained in bar_search.go instead.
	appState.SearchEditor.Submit = false

	// Restore UI state from previous window (e.g. after Wayland hide→recreate)
	if p.saved != nil {
		appState.Tabs.Current = p.saved.ActiveTab
		appState.ShellFilter = p.saved.ShellFilter

		if p.saved.SearchQuery != "" {
			appState.SearchEditor.SetText(p.saved.SearchQuery)
			// Move cursor to end of restored text (SetText leaves it at the start)
			end := appState.SearchEditor.Len()
			appState.SearchEditor.SetCaret(end, end)
			appState.CurrentQuery = p.saved.SearchQuery
			appState.RequestSearchFocus = true
		}

		if p.saved.SelectedNodePath != "" {
			appState.Tree.SelectedNodePath = p.saved.SelectedNodePath
			appState.Tree.NeedInitialSel = false
		}

		if p.saved.SelectedCmd != "" {
			appState.Commands.RestoreCmd = p.saved.SelectedCmd
			appState.Commands.NeedInitialSel = false
		}

		if p.saved.SelectedStatText != "" {
			appState.Stats.RestoreText = p.saved.SelectedStatText
			appState.Stats.RestoreKind = p.saved.SelectedStatKind
			appState.Stats.NeedInitialSel = false
		}
	}

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

	// Track last frame metric for Dp conversion on DestroyEvent.
	var lastFrameMetric unit.Metric

	// Per-frame allocation tracking.
	// Measures ALL allocations (UI + background goroutines) between frames.
	// Gated behind --pprof because ReadMemStats stops the world.
	var (
		frameCount  uint64
		totalAllocs uint64
		totalBytes  uint64
		lastStats   runtime.MemStats
	)

	if p.pprofEnabled {
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
		if appState.Dirty.Swap(false) || p.signalDirty.Swap(false) {
			win.Invalidate()
		}

		switch ev := e.(type) {
		case app.DestroyEvent:
			if ev.Err != nil {
				log.Printf("Window DestroyEvent with error: %v", ev.Err)
			}

			// Close button clicked - minimize to tray instead of quit
			log.Println("Window close button clicked, minimizing to tray (app continues running)")

			// Save window geometry via X11 before the window is destroyed (position on X11)
			p.windowController.SaveGeometry()

			// Save window size to config (Dp, survives restarts)
			if appState.LastWindowSize.X > 0 && appState.LastWindowSize.Y > 0 {
				cfg.WindowW = int(lastFrameMetric.PxToDp(appState.LastWindowSize.X))
				cfg.WindowH = int(lastFrameMetric.PxToDp(appState.LastWindowSize.Y))

				if err := cfg.SaveConfig(p.configPath); err != nil {
					log.Printf("Warning: Failed to save window size: %v", err)
				}
			}

			// Capture UI state before destruction for restoration on recreation
			snapshot := &savedUIState{
				SearchQuery: appState.SearchEditor.Text(),
				ActiveTab:   appState.Tabs.Current,
				ShellFilter: appState.ShellFilter,
			}

			// Save tree selection: SelectedNodePath is cleared after restoration,
			// so read the actual path from the current SelectedNode index.
			appState.StoreMu.RLock()

			if idx := appState.Tree.SelectedNode; idx >= 0 && idx < len(appState.Tree.Nodes) {
				snapshot.SelectedNodePath = appState.Tree.Nodes[idx].Path
			}

			// Save selected command text for restoration
			if idx := appState.Commands.SelectedIndex; idx >= 0 && idx < len(appState.Commands.DisplayCommands) {
				snapshot.SelectedCmd = appState.Commands.DisplayCommands[idx].Command
			}

			// Save selected statistics item for restoration
			switch idx := appState.Stats.SelectedIndex; {
			case idx >= 0 && idx < appState.Stats.CommandCount && idx < len(appState.Stats.TopCommands):
				snapshot.SelectedStatText = appState.Stats.TopCommands[idx]
				snapshot.SelectedStatKind = appstate.StatsRestoreCommand
			case idx >= appState.Stats.CommandCount:
				prefixIdx := idx - appState.Stats.CommandCount
				if prefixIdx < len(appState.Stats.TopPrefixes) {
					snapshot.SelectedStatText = appState.Stats.TopPrefixes[prefixIdx]
					snapshot.SelectedStatKind = appstate.StatsRestorePrefix
				}
			}

			appState.StoreMu.RUnlock()

			// Signal rebuild workers to shutdown for this window cycle
			close(appState.Tree.RebuildShutdown)
			close(appState.Stats.RebuildShutdown)
			close(appState.StoreShutdown)

			// Mark as closed for signal handling
			// NOTE: Don't try to hide() - window is already being destroyed by OS
			p.windowController.MarkClosed()

			// Exit run() to allow immediate recreation in hidden state
			return snapshot

		case app.ConfigEvent:
			// Detect external un-minimize (e.g. user clicks taskbar on Wayland).
			// Only sync visibility on Minimized → non-Minimized transition.
			// Avoids false positives on Windows/macOS where hiding uses
			// SW_HIDE/orderOut (Mode stays Windowed, never becomes Minimized).
			newMode := ev.Config.Mode
			if prevWindowMode == app.Minimized && newMode != app.Minimized {
				p.windowController.SyncVisible(true)
			}

			prevWindowMode = newMode

		case app.FrameEvent:
			// On first frame, signal that window handle is ready
			if !windowHandleSignaled {
				windowHandleSignaled = true

				log.Println("First frame rendered, signaling window handle ready")
				close(p.lifecycle.ready)

				// Re-apply saved size via win.Option() (not initialOpts).
				// Gio's init() adds a non-zero decoHeight to initialOpts
				// even with Decorated(false), causing the window to grow
				// by ~28px per save/restore cycle. Calling win.Option()
				// after the driver is ready goes through the w.Run() path
				// which correctly resets decoHeight=0 for Decorated(false).
				if cfg.WindowW > 0 && cfg.WindowH > 0 {
					win.Option(app.Size(unit.Dp(cfg.WindowW), unit.Dp(cfg.WindowH)))
				}

				// Restore window position or center on first launch.
				// Use Gio's Perform(ActionCenter) which internally calls
				// win.Run() to execute on the window thread. Calling
				// Win32 SetWindowPos directly from inside a FrameEvent
				// handler deadlocks: Gio's window thread is blocked in
				// deliverEvent() waiting for Frame(), not pumping messages.
				if !p.windowController.RestoreGeometry() {
					win.Perform(system.ActionCenter)
				}
			}

			lastFrameMetric = ev.Metric

			// Drain settings result channels before rendering so results
			// are visible in the current frame. Both channels are buffered(1);
			// goroutines send exactly one result per invocation.
			select {
			case r := <-appState.Hotkeys.ResultChan:
				appState.Hotkeys.Error = r.Error

				appState.Hotkeys.Success = r.Success
				if r.Success {
					appState.Config.HotkeyPreset = r.PresetID
					ui.RebuildHints(appState.Hotkeys.Presets[r.PresetID].DisplayName)
				}
			default:
			}

			select {
			case r := <-appState.AutoStartResultChan:
				appState.AutoStartError = r.Error
				appState.AutoStartSuccess = r.Success
				appState.AutoStartEnabled = r.NewEnabled
				appState.Config.AutoStart = r.AutoStart
				appState.AutoStartUpdating = false
			default:
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

			if appState.AutoStartNeedsUpdate && !appState.AutoStartUpdating {
				appState.AutoStartNeedsUpdate = false

				appState.AutoStartUpdating = true
				go ui.ProcessAutostartUpdate(appState)
			}

			if p.pprofEnabled {
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

// syncAutoStartState reconciles the OS autostart state with the config value.
// On first run (config says enabled, OS says disabled), it registers autostart.
// Returns the effective autostart state.
func syncAutoStartState(cfg *config.Config) bool {
	enabled, err := autostart.IsEnabled()
	if err != nil {
		log.Printf("Failed to check autostart state: %v, using config value", err)

		return cfg.AutoStart
	}

	if cfg.AutoStart && !enabled {
		if err := autostart.Enable(); err != nil {
			log.Printf("Failed to enable autostart on first run: %v", err)

			return false
		}

		log.Printf("Autostart enabled on first run")

		return true
	}

	return enabled
}

// loadConfig resolves the config path and loads the configuration.
// On any error it returns defaults and the path for future saves.
func loadConfig() (string, *config.Config, error) {
	path, err := config.ConfigPath()
	if err != nil {
		return "", config.DefaultConfig(), fmt.Errorf("resolving config path: %w", err)
	}

	cfg, err := config.LoadConfig(path)
	if err != nil {
		return path, config.DefaultConfig(), fmt.Errorf("loading config: %w", err)
	}

	return path, cfg, nil
}
