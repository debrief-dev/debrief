//go:build linux

package window

/*
#cgo linux pkg-config: x11

#include <X11/Xlib.h>
#include <X11/Xatom.h>
#include <X11/Xutil.h>
#include <string.h>
#include <strings.h>
#include <stdlib.h>

static Display *dpy = NULL;

// x11_init opens an X11 display connection.
// Returns 1 on success, 0 on failure.
// Uses a separate connection from Gio's internal one (which is inaccessible).
// Thread-safe because Gio calls XInitThreads() at startup.
static int x11_init(void) {
	if (dpy != NULL) return 1;
	dpy = XOpenDisplay(NULL);
	return dpy != NULL;
}

// find_window_by_name searches _NET_CLIENT_LIST for a window matching the
// given title. Checks _NET_WM_NAME (UTF-8) first, then falls back to
// WM_NAME via XFetchName.
static Window find_window_by_name(const char *target) {
	if (!dpy) return 0;

	Atom clientList = XInternAtom(dpy, "_NET_CLIENT_LIST", True);
	if (clientList == None) return 0;

	Window root = DefaultRootWindow(dpy);
	Atom actualType;
	int actualFormat;
	unsigned long nitems, bytesAfter;
	unsigned char *data = NULL;

	if (XGetWindowProperty(dpy, root, clientList,
			0, 65536, False, XA_WINDOW,
			&actualType, &actualFormat, &nitems, &bytesAfter,
			&data) != Success || data == NULL) {
		return 0;
	}

	Atom wmName = XInternAtom(dpy, "_NET_WM_NAME", True);
	Atom utf8   = XInternAtom(dpy, "UTF8_STRING", True);

	Window *windows = (Window *)data;
	Window result = 0;

	for (unsigned long i = 0; i < nitems; i++) {
		// Try _NET_WM_NAME (UTF-8, modern)
		if (wmName != None && utf8 != None) {
			Atom type;
			int fmt;
			unsigned long n, after;
			unsigned char *name = NULL;

			if (XGetWindowProperty(dpy, windows[i], wmName,
					0, 1024, False, utf8,
					&type, &fmt, &n, &after,
					&name) == Success && name != NULL) {
				if (strcasecmp((char *)name, target) == 0) {
					result = windows[i];
					XFree(name);
					break;
				}
				XFree(name);
			}
		}

		// Fallback: WM_NAME (legacy, Latin-1)
		char *name = NULL;
		if (XFetchName(dpy, windows[i], &name) && name != NULL) {
			if (strcasecmp(name, target) == 0) {
				result = windows[i];
			}
			XFree(name);
			if (result) break;
		}
	}

	XFree(data);
	return result;
}

// x11_iconify minimizes the window with the given title.
static int x11_iconify(const char *title) {
	Window w = find_window_by_name(title);
	if (!w) return 0;

	int screen = DefaultScreen(dpy);
	XIconifyWindow(dpy, w, screen);
	XFlush(dpy);
	return 1;
}

// x11_activate raises and focuses the window with the given title.
// Sends _NET_ACTIVE_WINDOW client message (EWMH) to the root window,
// then maps the window in case it was unmapped.
static int x11_activate(const char *title) {
	Window w = find_window_by_name(title);
	if (!w) return 0;

	Window root = DefaultRootWindow(dpy);
	Atom activeAtom = XInternAtom(dpy, "_NET_ACTIVE_WINDOW", False);

	XEvent xev;
	memset(&xev, 0, sizeof(xev));
	xev.type                 = ClientMessage;
	xev.xclient.display      = dpy;
	xev.xclient.window       = w;
	xev.xclient.message_type = activeAtom;
	xev.xclient.format       = 32;
	xev.xclient.data.l[0]    = 2; // source indication: pager

	XSendEvent(dpy, root, False,
		SubstructureNotifyMask | SubstructureRedirectMask, &xev);
	XMapRaised(dpy, w);
	XFlush(dpy);
	return 1;
}

// x11_get_active_window returns the currently focused window ID
// by reading _NET_ACTIVE_WINDOW from the root window.
static unsigned long x11_get_active_window(void) {
	if (!dpy) return 0;

	Window root = DefaultRootWindow(dpy);
	Atom activeAtom = XInternAtom(dpy, "_NET_ACTIVE_WINDOW", True);
	if (activeAtom == None) return 0;

	Atom actualType;
	int actualFormat;
	unsigned long nitems, bytesAfter;
	unsigned char *data = NULL;

	if (XGetWindowProperty(dpy, root, activeAtom,
			0, 1, False, XA_WINDOW,
			&actualType, &actualFormat, &nitems, &bytesAfter,
			&data) != Success || data == NULL || nitems == 0) {
		return 0;
	}

	Window active = *(Window *)data;
	XFree(data);
	return (unsigned long)active;
}

// x11_activate_window activates a window by its X11 Window ID.
static void x11_activate_window(unsigned long wid) {
	if (!dpy || !wid) return;

	Window root = DefaultRootWindow(dpy);
	Atom activeAtom = XInternAtom(dpy, "_NET_ACTIVE_WINDOW", False);

	XEvent xev;
	memset(&xev, 0, sizeof(xev));
	xev.type                 = ClientMessage;
	xev.xclient.display      = dpy;
	xev.xclient.window       = (Window)wid;
	xev.xclient.message_type = activeAtom;
	xev.xclient.format       = 32;
	xev.xclient.data.l[0]    = 2;

	XSendEvent(dpy, root, False,
		SubstructureNotifyMask | SubstructureRedirectMask, &xev);
	XMapRaised(dpy, (Window)wid);
	XFlush(dpy);
}

// x11_get_geometry returns the window geometry (x, y, width, height)
// for the window with the given title. Returns 1 on success, 0 on failure.
// Coordinates are translated to root-window (absolute screen) coordinates.
static int x11_get_geometry(const char *title, int *x, int *y, int *w, int *h) {
	Window win = find_window_by_name(title);
	if (!win) return 0;

	XWindowAttributes attr;
	if (!XGetWindowAttributes(dpy, win, &attr)) return 0;

	// Translate to root-window coordinates
	Window child;
	int rx, ry;
	XTranslateCoordinates(dpy, win, DefaultRootWindow(dpy), 0, 0, &rx, &ry, &child);

	*x = rx;
	*y = ry;
	*w = attr.width;
	*h = attr.height;
	return 1;
}

// x11_move_resize moves and resizes the window with the given title.
static int x11_move_resize(const char *title, int x, int y, int w, int h) {
	Window win = find_window_by_name(title);
	if (!win) return 0;

	XMoveResizeWindow(dpy, win, x, y, w, h);
	XFlush(dpy);
	return 1;
}

*/
import "C"

import (
	"errors"
	"log"
	"os"
	"unsafe"

	"gioui.org/app"
	"gioui.org/io/system"
)

// Platform-specific window handle type (X11 Window ID)
type windowHandle C.ulong

const invalidWindowHandle windowHandle = 0

// x11Available is set at init to indicate whether X11 display connection
// was opened successfully. On Wayland-only environments this will be false.
var x11Available bool

// isWaylandSession is true when the current session is a Wayland session.
var isWaylandSession bool

// IsWaylandSession reports whether the current session is Wayland.
func IsWaylandSession() bool {
	return isWaylandSession
}

// initPlatformController initializes X11 display and detects Wayland session.
func initPlatformController(_ *Controller) {
	x11Available = C.x11_init() != 0
	if x11Available {
		log.Println("Window: X11 display connection opened for window control")
	} else {
		log.Println("Window: X11 display not available, using Gio fallback for window control")
	}

	sessionType := os.Getenv("XDG_SESSION_TYPE")
	waylandDisplay := os.Getenv("WAYLAND_DISPLAY")
	isWaylandSession = sessionType == "wayland" || waylandDisplay != ""

	if isWaylandSession {
		log.Println("Window: Wayland session detected")
	}
}

// hideWindow minimizes the window.
// On Wayland, closes the window instead of minimizing because Wayland has
// no protocol for un-minimizing. The main loop recreates the window on show.
// On X11, uses Xlib directly (bypasses Gio state tracking issues).
// Must be called with c.mu held.
func (c *Controller) hideWindow() error {
	// Capture geometry before the window is hidden or destroyed.
	// On Wayland, ActionClose removes the window immediately, so
	// the geometry must be saved while the window still exists.
	if x11Available {
		if x, y, w, h, ok := c.getWindowGeometry(); ok {
			c.savedGeom = &Geometry{X: x, Y: y, W: w, H: h}
			log.Printf("Window: Saved geometry before hide %dx%d+%d+%d", w, h, x, y)
		}
	}

	if isWaylandSession {
		if c.win == nil {
			return errors.New("window reference is nil")
		}

		// xdg_toplevel has set_minimized but no unset_minimized.
		// Close the window; the main loop recreates it on "show"/"toggle".
		log.Println("Window: Wayland — closing window instead of minimizing")
		c.win.Perform(system.ActionClose)
		c.closed = true

		return nil
	}

	if x11Available {
		cTitle := C.CString(c.title)
		defer C.free(unsafe.Pointer(cTitle))

		if C.x11_iconify(cTitle) != 0 {
			return nil
		}

		log.Println("Window: X11 window not found, falling back to Gio API")
	}

	if c.win == nil {
		return errors.New("window reference is nil")
	}

	c.win.Option(app.Minimized.Option())

	return nil
}

// showWindow raises and focuses the window.
// On Wayland, this is a no-op — showing is handled by window recreation
// in the main loop (see hideWindow).
// On X11, uses Xlib directly (bypasses Gio state tracking issues).
// Must be called with c.mu held.
func (c *Controller) showWindow() error {
	if isWaylandSession {
		// Un-minimizing is not possible on Wayland. The main loop
		// recreates the window via the "IsClosed" signal handler path.
		return nil
	}

	if x11Available {
		cTitle := C.CString(c.title)
		defer C.free(unsafe.Pointer(cTitle))

		if C.x11_activate(cTitle) != 0 {
			return nil
		}

		log.Println("Window: X11 window not found, falling back to Gio API")
	}

	if c.win == nil {
		return errors.New("window reference is nil")
	}

	c.win.Option(app.Windowed.Option())

	return nil
}

// captureForegroundWindow captures the current foreground window ID.
// Returns invalidWindowHandle if X11 is not available (Wayland).
func captureForegroundWindow() windowHandle {
	if !x11Available {
		return invalidWindowHandle
	}

	wid := C.x11_get_active_window()
	if wid == 0 {
		return invalidWindowHandle
	}

	log.Printf("Window: Captured foreground window ID: %d", wid)

	return windowHandle(wid)
}

// restorePreviousWindow restores a previously captured window to foreground.
func restorePreviousWindow(handle windowHandle) error {
	if !x11Available || handle == invalidWindowHandle {
		return nil
	}

	C.x11_activate_window(C.ulong(handle))

	log.Printf("Window: Restored window ID: %d", handle)

	return nil
}

// getWindowGeometry returns the window's screen position and size.
func (c *Controller) getWindowGeometry() (x, y, w, h int, ok bool) {
	if !x11Available {
		return 0, 0, 0, 0, false
	}

	cTitle := C.CString(c.title)
	defer C.free(unsafe.Pointer(cTitle))

	var cx, cy, cw, ch C.int
	if C.x11_get_geometry(cTitle, &cx, &cy, &cw, &ch) == 0 {
		return 0, 0, 0, 0, false
	}

	return int(cx), int(cy), int(cw), int(ch), true
}

// setWindowGeometry moves and resizes the window.
func (c *Controller) setWindowGeometry(x, y, w, h int) bool {
	if !x11Available {
		return false
	}

	cTitle := C.CString(c.title)
	defer C.free(unsafe.Pointer(cTitle))

	return C.x11_move_resize(cTitle, C.int(x), C.int(y), C.int(w), C.int(h)) != 0
}
