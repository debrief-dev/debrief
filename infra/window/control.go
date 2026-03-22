package window

import (
	"log"
	"sync"
	"time"

	"gioui.org/app"
)

// Linux clipboard timing constants
const (
	// LinuxClipboardDelay is the delay before hiding the window on Linux
	// to let Gio process the pending clipboard.WriteCmd.
	LinuxClipboardDelay = 150 * time.Millisecond
)

// Geometry holds a window's screen position and size in pixels.
type Geometry struct {
	X, Y, W, H int
}

// Controller manages window visibility across platforms
type Controller struct {
	title      string
	visible    bool
	closed     bool // true when window is destroyed (close button clicked)
	mu         sync.RWMutex
	onShowHide func(visible bool)
	prevWindow windowHandle // stores the previous foreground window
	win        *app.Window  // Gio window reference for platform control
	savedGeom  *Geometry    // saved geometry for restoration after recreation
}

// NewController creates a new window controller
func NewController(title string) *Controller {
	c := &Controller{
		title:   title,
		visible: true, // Starts visible
	}

	initPlatformController(c)

	return c
}

// SetWindow sets or clears the Gio window reference.
// Called after window creation and before MarkOpened,
// and again with nil after MarkClosed.
func (c *Controller) SetWindow(win *app.Window) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.win = win
}

// SyncVisible updates the visibility flag to match the actual window state.
// Called from the event loop when a ConfigEvent indicates the window mode
// changed externally (e.g. user clicked taskbar to un-minimize on Wayland).
// NOTE: Does not call onShowHide because this only triggers on Wayland
// (Minimized→non-Minimized), never on macOS where dock visibility is managed.
func (c *Controller) SyncVisible(visible bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.visible == visible {
		return
	}

	c.visible = visible

	log.Printf("Window: Visibility synced to %v (external state change)", visible)
}

// Show makes the window visible
func (c *Controller) Show() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.closed {
		log.Println("Window: Window is closed, cannot show (needs recreation)")
		// Return nil because this is expected - the window will be recreated
		return nil
	}

	if c.visible {
		log.Println("Window: Already visible")
		return nil
	}

	// Capture the current foreground window before showing ours
	prevWin := captureForegroundWindow()
	if prevWin != invalidWindowHandle {
		c.prevWindow = prevWin

		log.Println("Window: Captured previous foreground window")
	} else {
		log.Println("Window: No previous window captured (may be desktop/none)")
	}

	log.Println("Window: Showing")

	if err := c.showWindow(); err != nil {
		log.Printf("Window: Error showing: %v", err)
		return err
	}

	c.visible = true
	if c.onShowHide != nil {
		c.onShowHide(true)
	}

	return nil
}

// Hide makes the window invisible
func (c *Controller) Hide() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.closed {
		log.Println("Window: Window is closed, cannot hide (already gone)")
		// Return nil because this is expected
		return nil
	}

	if !c.visible {
		log.Println("Window: Already hidden")
		return nil
	}

	log.Println("Window: Hiding")

	if err := c.hideWindow(); err != nil {
		log.Printf("Window: Error hiding: %v", err)
		return err
	}

	c.visible = false
	if c.onShowHide != nil {
		c.onShowHide(false)
	}

	return nil
}

// Toggle switches window visibility
func (c *Controller) Toggle() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.closed {
		log.Println("Window: Window is closed, cannot toggle (needs recreation)")
		return nil
	}

	if c.visible {
		log.Println("Window: Hiding (via toggle)")

		if err := c.hideWindow(); err != nil {
			log.Printf("Window: Error hiding: %v", err)
			return err
		}

		c.visible = false
		if c.onShowHide != nil {
			c.onShowHide(false)
		}
	} else {
		prevWin := captureForegroundWindow()
		if prevWin != invalidWindowHandle {
			c.prevWindow = prevWin

			log.Println("Window: Captured previous foreground window")
		} else {
			log.Println("Window: No previous window captured (may be desktop/none)")
		}

		log.Println("Window: Showing (via toggle)")

		if err := c.showWindow(); err != nil {
			log.Printf("Window: Error showing: %v", err)
			return err
		}

		c.visible = true
		if c.onShowHide != nil {
			c.onShowHide(true)
		}
	}

	return nil
}

// HandleSignal processes window control signals
func (c *Controller) HandleSignal(signal string) error {
	switch signal {
	case "show":
		return c.Show()
	case "hide":
		return c.Hide()
	case "hide_and_restore":
		return c.HideAndRestorePrevious()
	case "toggle":
		return c.Toggle()
	default:
		log.Printf("Window: Unknown signal: %s", signal)
		return nil
	}
}

// MarkClosed marks the window as closed (destroyed)
// Call this when the window is closed via close button
func (c *Controller) MarkClosed() {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.closed {
		return // Already marked (e.g. Wayland hideWindow)
	}

	c.closed = true
	c.visible = false

	if c.onShowHide != nil {
		c.onShowHide(false)
	}

	log.Println("Window: Marked as closed")
}

// MarkOpened marks the window as opened (created/recreated).
// Call this when a new window is created.
func (c *Controller) MarkOpened() {
	c.mu.Lock()
	defer c.mu.Unlock()

	wasClosed := c.closed // false on first call, true on recreation

	c.closed = false
	c.visible = true

	if wasClosed && c.onShowHide != nil {
		c.onShowHide(true)
	}

	log.Println("Window: Marked as opened")
}

// IsClosed returns whether the window is currently closed (destroyed)
func (c *Controller) IsClosed() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return c.closed
}

// SaveGeometry captures the current window geometry (position + size) via
// platform APIs. Call before the window is destroyed.
// No-op if geometry was already saved (e.g. by hideWindow on Linux).
func (c *Controller) SaveGeometry() {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.savedGeom != nil {
		return // Already saved (e.g. by hideWindow before Wayland ActionClose)
	}

	x, y, w, h, ok := c.getWindowGeometry()
	if !ok {
		return
	}

	c.savedGeom = &Geometry{X: x, Y: y, W: w, H: h}

	log.Printf("Window: Saved geometry %dx%d+%d+%d", w, h, x, y)
}

// RestoreGeometry applies previously saved geometry to the current window.
// Call after the window handle is ready (first frame rendered).
// Consumes the saved geometry so the next window lifecycle saves fresh values.
// Returns true if geometry was restored, false if no saved geometry exists.
func (c *Controller) RestoreGeometry() bool {
	c.mu.Lock()
	g := c.savedGeom
	c.savedGeom = nil // Consumed; next hide/close will save fresh values
	c.mu.Unlock()

	if g == nil {
		return false
	}

	if c.setWindowGeometry(g.X, g.Y, g.W, g.H) {
		log.Printf("Window: Restored geometry %dx%d+%d+%d", g.W, g.H, g.X, g.Y)
		return true
	}

	return false
}

// HideAndRestorePrevious hides the window and restores the previous foreground window.
// The entire operation is atomic to prevent races with concurrent Show() calls.
func (c *Controller) HideAndRestorePrevious() error {
	c.mu.Lock()

	if c.closed {
		c.mu.Unlock()
		log.Println("Window: Window is closed, cannot hide_and_restore")

		return nil
	}

	if !c.visible {
		c.mu.Unlock()
		log.Println("Window: Already hidden, skipping hide_and_restore")

		return nil
	}

	// Snapshot and clear the previous window handle
	prevWin := c.prevWindow
	c.prevWindow = invalidWindowHandle

	log.Println("Window: Hiding (via hide_and_restore)")

	if err := c.hideWindow(); err != nil {
		c.mu.Unlock()
		log.Printf("Window: Error hiding: %v", err)

		return err
	}

	c.visible = false
	if c.onShowHide != nil {
		c.onShowHide(false)
	}

	c.mu.Unlock()

	// Restore previous window outside the lock (no controller state mutation)
	if prevWin != invalidWindowHandle {
		log.Println("Window: Restoring previous window")

		if err := restorePreviousWindow(prevWin); err != nil {
			log.Printf("Window: Warning - failed to restore previous window: %v", err)
		}
	} else {
		log.Println("Window: No previous window to restore")
	}

	return nil
}
