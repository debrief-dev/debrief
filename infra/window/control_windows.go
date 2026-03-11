//go:build windows

package window

import (
	"errors"
	"fmt"
	"log"
	"syscall"
	"unsafe"
)

// IsWaylandSession reports whether the current session is Wayland.
// Always returns false on Windows.
func IsWaylandSession() bool {
	return false
}

// initPlatformController is a no-op on Windows.
func initPlatformController(_ *Controller) {}

// Platform-specific window handle type
type windowHandle uintptr

const invalidWindowHandle windowHandle = 0

var (
	user32                  = syscall.NewLazyDLL("user32.dll")
	procFindWindowW         = user32.NewProc("FindWindowW")
	procShowWindow          = user32.NewProc("ShowWindow")
	procSetForegroundWindow = user32.NewProc("SetForegroundWindow")
	procGetForegroundWindow = user32.NewProc("GetForegroundWindow")
	procIsIconic            = user32.NewProc("IsIconic")
)

const (
	SW_HIDE    = 0
	SW_SHOW    = 5
	SW_RESTORE = 9
)

// findWindowByTitle finds a window handle by its title
func findWindowByTitle(title string) (uintptr, error) {
	titlePtr, err := syscall.UTF16PtrFromString(title)
	if err != nil {
		return 0, fmt.Errorf("failed to convert title to UTF16: %w", err)
	}

	hwnd, _, _ := procFindWindowW.Call(
		0, // lpClassName (NULL means any class)
		//nolint:gosec // G103: unsafe pointer needed for Windows API call
		uintptr(unsafe.Pointer(titlePtr)),
	)

	if hwnd == 0 {
		return 0, fmt.Errorf("window not found: %s", title)
	}

	return hwnd, nil
}

// hideWindow hides the window with the given title.
// Must be called with c.mu held.
func (c *Controller) hideWindow() error {
	hwnd, err := findWindowByTitle(c.title)
	if err != nil {
		return err
	}

	// ShowWindow returns 0 if window was already hidden, nonzero if it was visible
	// Both are success cases, so we don't check the return value
	var errno syscall.Errno
	if _, _, err := procShowWindow.Call(hwnd, SW_HIDE); err != nil && errors.As(err, &errno) && errno != 0 {
		log.Printf("Warning: ShowWindow(SW_HIDE) failed: %v", err)
	}

	return nil
}

// showWindow shows and brings to foreground the window with the given title.
// Must be called with c.mu held.
func (c *Controller) showWindow() error {
	hwnd, err := findWindowByTitle(c.title)
	if err != nil {
		return err
	}

	// First restore the window if it's minimized
	var errno syscall.Errno
	if _, _, err := procShowWindow.Call(hwnd, SW_RESTORE); err != nil && errors.As(err, &errno) && errno != 0 {
		log.Printf("Warning: ShowWindow(SW_RESTORE) failed: %v", err)
	}

	// Then show it
	// ShowWindow returns 0 if window was already shown, nonzero if it was hidden
	// Both are success cases, so we don't check the return value
	var errno2 syscall.Errno
	if _, _, err := procShowWindow.Call(hwnd, SW_SHOW); err != nil && errors.As(err, &errno2) && errno2 != 0 {
		log.Printf("Warning: ShowWindow(SW_SHOW) failed: %v", err)
	}

	// Bring to foreground
	var errno3 syscall.Errno
	if _, _, err := procSetForegroundWindow.Call(hwnd); err != nil && errors.As(err, &errno3) && errno3 != 0 {
		log.Printf("Warning: SetForegroundWindow failed: %v", err)
	}

	return nil
}

// captureForegroundWindow captures the current foreground window handle
func captureForegroundWindow() windowHandle {
	hwnd, _, _ := procGetForegroundWindow.Call()

	if hwnd == 0 {
		return invalidWindowHandle
	}

	return windowHandle(hwnd)
}

// getWindowGeometry is not implemented on Windows (window is not recreated by hotkey).
func (c *Controller) getWindowGeometry() (x, y, w, h int, ok bool) {
	return 0, 0, 0, 0, false
}

// setWindowGeometry is not implemented on Windows (window is not recreated by hotkey).
func (c *Controller) setWindowGeometry(_, _, _, _ int) bool {
	return false
}

// restorePreviousWindow restores a previously captured window to foreground
func restorePreviousWindow(handle windowHandle) error {
	if handle == invalidWindowHandle {
		return errors.New("invalid window handle")
	}

	hwnd := uintptr(handle)

	// Check if window is minimized (iconic)
	isMinimized, _, _ := procIsIconic.Call(hwnd)
	if isMinimized != 0 {
		// Only restore if window is actually minimized
		// This prevents downsizing maximized/fullscreen windows
		log.Println("Window: Previous window is minimized, restoring")

		var errno syscall.Errno
		if _, _, err := procShowWindow.Call(hwnd, SW_RESTORE); err != nil && errors.As(err, &errno) && errno != 0 {
			log.Printf("Warning: ShowWindow(SW_RESTORE) failed: %v", err)
		}
	} else {
		log.Println("Window: Previous window is not minimized, just bringing to foreground")
	}

	// Bring to foreground
	result, _, _ := procSetForegroundWindow.Call(hwnd)
	if result == 0 {
		return errors.New("failed to set foreground window")
	}

	log.Println("Window: Successfully restored previous window")

	return nil
}
