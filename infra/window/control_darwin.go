//go:build darwin

package window

/*
#cgo CFLAGS: -x objective-c
#cgo LDFLAGS: -framework Cocoa
#import <Cocoa/Cocoa.h>

// Show the application and bring to front
void showApp() {
    dispatch_async(dispatch_get_main_queue(), ^{
        [[NSApplication sharedApplication] unhide:nil];
        // Use activate instead of deprecated activateIgnoringOtherApps (macOS 14+)
        [[NSApplication sharedApplication] activate];
    });
}

// Set activation policy to Regular (shows in dock and menu bar)
void setActivationPolicyRegular() {
    dispatch_async(dispatch_get_main_queue(), ^{
        BOOL ok = [[NSApplication sharedApplication]
            setActivationPolicy:NSApplicationActivationPolicyRegular];
        if (!ok) {
            NSLog(@"Failed to set activation policy to Regular");
        }
    });
}

// Set activation policy to Accessory (hides from dock, no menu bar)
void setActivationPolicyAccessory() {
    dispatch_async(dispatch_get_main_queue(), ^{
        BOOL ok = [[NSApplication sharedApplication]
            setActivationPolicy:NSApplicationActivationPolicyAccessory];
        if (!ok) {
            NSLog(@"Failed to set activation policy to Accessory");
        }
    });
}

// Get the PID of the frontmost (active) application
int getFrontmostAppPID() {
    __block int pid = -1;
    dispatch_sync(dispatch_get_main_queue(), ^{
        NSRunningApplication *frontApp = [[NSWorkspace sharedWorkspace] frontmostApplication];
        if (frontApp) {
            pid = [frontApp processIdentifier];
        }
    });
    return pid;
}

// Activate application by PID
void activateAppByPID(int pid) {
    dispatch_async(dispatch_get_main_queue(), ^{
        NSRunningApplication *app = [NSRunningApplication runningApplicationWithProcessIdentifier:pid];
        if (app) {
            // Use 0 instead of deprecated NSApplicationActivateIgnoringOtherApps (macOS 14+)
            [app activateWithOptions:0];
        }
    });
}

// Find window by title and hide it
void hideWindowByTitle(const char* title) {
    NSString *nsTitle = [NSString stringWithUTF8String:title];
    dispatch_async(dispatch_get_main_queue(), ^{
        NSArray *windows = [[NSApplication sharedApplication] windows];
        for (NSWindow *window in windows) {
            if ([[window title] isEqualToString:nsTitle]) {
                [window orderOut:nil];
                return;
            }
        }
    });
}

// Find window by title and show it
void showWindowByTitle(const char* title) {
    NSString *nsTitle = [NSString stringWithUTF8String:title];
    dispatch_async(dispatch_get_main_queue(), ^{
        NSArray *windows = [[NSApplication sharedApplication] windows];
        for (NSWindow *window in windows) {
            if ([[window title] isEqualToString:nsTitle]) {
                [window makeKeyAndOrderFront:nil];
                // Use activate instead of deprecated activateIgnoringOtherApps (macOS 14+)
                [[NSApplication sharedApplication] activate];
                return;
            }
        }
    });
}
*/
import "C"

import (
	"errors"
	"log"
	"unsafe"
)

// Platform-specific window handle type (storing PID)
type windowHandle int32

const invalidWindowHandle windowHandle = -1

// hideWindow hides the window with the given title on macOS.
// Must be called with c.mu held.
func (c *Controller) hideWindow() error {
	// Try to hide specific window first
	cTitle := C.CString(c.title)
	defer C.free(unsafe.Pointer(cTitle))

	C.hideWindowByTitle(cTitle)

	return nil
}

// showWindow shows and brings to foreground the window with the given title on macOS.
// Must be called with c.mu held.
func (c *Controller) showWindow() error {
	// Try to show specific window first
	cTitle := C.CString(c.title)
	defer C.free(unsafe.Pointer(cTitle))

	C.showWindowByTitle(cTitle)

	// Also unhide the app
	C.showApp()

	return nil
}

// captureForegroundWindow captures the current foreground application PID
func captureForegroundWindow() windowHandle {
	pid := int32(C.getFrontmostAppPID())
	if pid == -1 {
		log.Println("Window: Failed to capture foreground app PID")
		return invalidWindowHandle
	}

	log.Printf("Window: Captured foreground app PID: %d", pid)
	return windowHandle(pid)
}

// getWindowGeometry is not implemented on macOS (window is not recreated by hotkey).
func (c *Controller) getWindowGeometry() (x, y, w, h int, ok bool) {
	return 0, 0, 0, 0, false
}

// setWindowGeometry is not implemented on macOS (window is not recreated by hotkey).
func (c *Controller) setWindowGeometry(_, _, _, _ int) bool {
	return false
}

// restorePreviousWindow restores a previously captured application to foreground
func restorePreviousWindow(handle windowHandle) error {
	if handle == invalidWindowHandle {
		return errors.New("invalid window handle")
	}

	pid := int32(handle)
	C.activateAppByPID(C.int(pid))

	log.Printf("Window: Restored app with PID %d", pid)
	return nil
}

// IsWaylandSession reports whether the current session is Wayland.
// Always returns false on macOS.
func IsWaylandSession() bool {
	return false
}

// initPlatformController sets up macOS-specific behavior on the Controller.
// Wires the onShowHide callback to toggle the dock icon via activation policy.
func initPlatformController(c *Controller) {
	c.onShowHide = func(visible bool) {
		if visible {
			C.setActivationPolicyRegular()
		} else {
			C.setActivationPolicyAccessory()
		}

		log.Printf("Window: Dock visibility set to %v", visible)
	}
}
