package app

import (
	"image"
	"sync"
	"sync/atomic"

	"gioui.org/app"
	"gioui.org/widget"
	"github.com/debrief-dev/debrief/data/cmdstore"
	"github.com/debrief-dev/debrief/data/model"
	"github.com/debrief-dev/debrief/data/shell"
	"github.com/debrief-dev/debrief/infra/config"
	"github.com/debrief-dev/debrief/infra/hotkey"
)

// CommandsState holds UI and data state for the Commands tab.
type CommandsState struct {
	List widget.List // Scrollable command list

	// Command display (protected by StoreMu)
	LoadedCommands []*model.CommandEntry
	// DisplayCommands is filtered (for search) or all (no search).
	// COPY-ON-WRITE CONTRACT: Always assign a NEW slice, NEVER mutate in place
	// (no sort, no append to existing slice within capacity). This allows UI to
	// safely read with shallow copy under RLock while store may be updating.
	DisplayCommands []*model.CommandEntry

	// UI state (UI-THREAD-ONLY: access only from Gio event loop, NO locking needed)
	SelectedIndex     int    // Selected command index (-1 = none, in search input)
	HoveredIndex      int    // Hovered command index (-1 = none)
	NeedInitialSel    bool   // Flag to auto-select last item on initial load
	LastSelectedIndex int    // Last selected command before typing in search (-1 = none)
	LastSelectedCmd   string // Command text of last selection (for restoration after search)

	// Height caching for smart scrolling with variable-height items (UI-THREAD-ONLY)
	ItemHeights []int // Cached heights for command list items (parallel to DisplayCommands)

	// Metadata caching for zero-allocation rendering (UI-THREAD-ONLY)
	MetadataCache map[*model.CommandEntry]string // Cached formatted metadata strings
}

// TreeState holds UI and data state for the Tree tab.
type TreeState struct {
	List widget.List // Scrollable tree list

	// Protected by StoreMu (written by rebuild worker, read by UI)
	Nodes           []*model.TreeDisplayNode
	NodesGeneration uint64         // Increments on rebuild, detects stale references
	NodePathIndex   map[string]int // Maps node path to index in Nodes for O(1) lookup

	// Selection state (protected by StoreMu for atomic updates with Nodes)
	SelectedNode     int    // Selected tree node index (-1 = none)
	SelectedNodePath string // Path of selected node, used to maintain selection across rebuilds

	// UI state (UI-THREAD-ONLY)
	HoveredNode      int    // Hovered tree node index (-1 = none)
	SuppressHover    bool   // Suppress hover until mouse moves after click
	NeedInitialSel   bool   // Flag to auto-select last item on initial load (tree tab)
	LastSelectedNode int    // Last selected tree node before typing in search (-1 = none)
	LastSelectedPath string // Path of last selected tree node (for restoration after search)

	// Height caching for smart scrolling (UI-THREAD-ONLY)
	ItemHeights []int // Cached heights for tree view nodes (parallel to Nodes)

	// Rebuild coordination
	NeedsRebuild    atomic.Bool   // Flag to trigger tree rebuild (atomic for lock-free access)
	RebuildChan     chan struct{} // Buffered(1): coalesces rebuild requests
	RebuildDone     chan struct{} // Closed after each rebuild, then recreated (broadcast notification)
	RebuildDoneMu   sync.Mutex    // Protects RebuildDone channel recreation
	RebuildShutdown chan struct{} // Unbuffered: signal worker to stop
}

// StatsState holds UI and data state for the Statistics tab.
type StatsState struct {
	List              widget.List                           // Scrollable statistics list
	CommandClickables [model.TopItemsLimit]widget.Clickable // Clickable widgets for top commands
	PrefixClickables  [model.TopItemsLimit]widget.Clickable // Clickable widgets for top prefixes

	// UI state (UI-THREAD-ONLY)
	SelectedIndex  int      // Selected item index (-1 = none, 0-9 = commands, 10-19 = prefixes)
	HoveredIndex   int      // Hovered item index (-1 = none)
	CommandCount   int      // Number of commands available (for bounds checking)
	PrefixCount    int      // Number of prefixes available (for bounds checking)
	TopCommands    []string // Stored command texts for keyboard access
	TopPrefixes    []string // Stored prefix texts for keyboard access
	NeedInitialSel bool     // Flag to auto-select first item on tab switch

	// Height caching for smart scrolling (UI-THREAD-ONLY)
	ItemHeights []int // Cached heights for statistics items (commands + prefixes)

	// Cached rebuild data (protected by StoreMu)
	CachedTopCommands []model.RankedEntry // Cached sorted top commands
	CachedTopPrefixes []model.RankedEntry // Cached sorted top prefixes

	// Rebuild coordination
	NeedsRebuild    atomic.Bool   // Flag to trigger statistics rebuild (atomic for lock-free access)
	RebuildChan     chan struct{} // Buffered(1): coalesces rebuild requests
	RebuildShutdown chan struct{} // Unbuffered: signal worker to stop
}

// TabState holds tab navigation state (UI-THREAD-ONLY).
type TabState struct {
	Current       model.Tab
	CommandsTab   widget.Clickable
	TreeTab       widget.Clickable
	StatisticsTab widget.Clickable
	SettingsTab   widget.Clickable
}

// HotkeyState holds hotkey configuration state (UI-THREAD-ONLY).
type HotkeyState struct {
	// NeedsUpdate is set during frame processing and consumed after ev.Frame.
	// On macOS the hotkey library uses dispatch_sync(main_queue) internally, which
	// deadlocks when called before ev.Frame (the main thread is waiting for
	// ev.Frame while the goroutine waits for the main thread).
	NeedsUpdate      bool
	Error            string             // Validation/registration error message
	Success          bool               // Show success message after save
	Manager          *hotkey.Manager    // Reference to hotkey manager
	Presets          []hotkey.Preset    // Preset definitions
	SelectedPresetID int                // Currently selected preset (0, 1, or 2)
	PresetClickables []widget.Clickable // Clickables for preset buttons
}

type State struct {
	// Set once at init, never modified (safe for lock-free reads from background goroutines).
	Window         *app.Window
	HideWindowFunc func()
	QuitFunc       func()

	Config     *config.Config
	ConfigPath string

	SourceManager *shell.ShellManager

	Store   *cmdstore.CmdStore
	StoreMu sync.RWMutex

	// Per-tab state
	Commands CommandsState
	Tree     TreeState
	Stats    StatsState
	Tabs     TabState
	Hotkeys  HotkeyState

	// Search state
	SearchEditor widget.Editor // Search input at bottom (UI-THREAD-ONLY)
	CurrentQuery string        // Protected by StoreMu
	// Cached search match set for tree rebuild reuse (protected by StoreMu).
	// COPY-ON-WRITE: always assign a new map, never mutate.
	// nil when query is empty or no search has been performed.
	SearchMatchingCommands map[*model.CommandEntry]bool

	// Protected by StoreMu
	LoadError error

	// UI state (UI-THREAD-ONLY: access only from Gio event loop, NO locking needed)
	// Background goroutines MUST NOT access these fields.
	FrameCount         int         // Track frame count for autofocus
	RequestSearchFocus bool        // Flag to request focus on search editor (when switching tabs)
	NeedScrollToSel    bool        // Shared across Commands, Tree, Statistics tabs
	LastWindowSize     image.Point // Track window size to detect resizes and invalidate height caches

	// Shell filtering (UI-THREAD-ONLY)
	ShellFilter map[model.Shell]bool         // nil = show all shells
	ShellBadges map[string]*widget.Clickable // Persistent clickable widgets for badges
	AllBadge    widget.Clickable             // "All" filter badge

	// Settings view state (UI-THREAD-ONLY)
	SettingsList widget.List

	// Background-to-UI invalidation flag.
	// Background goroutines set this (via MarkDirty) alongside Window.Invalidate().
	// After each FrameEvent, the event loop checks and clears this flag, re-calling
	// Invalidate() if set. This ensures invalidations that were coalesced away by
	// Gio's mayInvalidate guard are not lost.
	Dirty atomic.Bool

	// Background store shutdown
	StoreShutdown chan struct{} // Unbuffered: signal background store/polling goroutine to stop
}

// MarkDirty sets the dirty flag and calls Window.Invalidate().
// Background goroutines MUST use this instead of calling Window.Invalidate()
// directly. The dirty flag survives Gio's invalidation coalescing: after each
// FrameEvent the event loop checks this flag and re-invalidates if needed,
// ensuring no updates are lost.
func (s *State) MarkDirty() {
	s.Dirty.Store(true)

	if s.Window != nil {
		s.Window.Invalidate()
	}
}
