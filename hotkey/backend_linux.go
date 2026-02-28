//go:build linux

package hotkey

import (
	"log"
	"os"

	hk "golang.design/x/hotkey"
)

// newBackend implements the Wayland-first fallback chain on Linux:
//  1. If Wayland, try D-Bus GlobalShortcuts portal (KDE, Sway, Hyprland)
//  2. If portal unavailable, try GNOME gsettings + D-Bus service
//  3. Fall back to X11 (may work via XWayland or on X11 sessions)
func newBackend(mods []hk.Modifier, key hk.Key, modStrs []string, keyStr string) backend {
	sessionType := os.Getenv("XDG_SESSION_TYPE")
	waylandDisplay := os.Getenv("WAYLAND_DISPLAY")
	isWayland := sessionType == "wayland" || waylandDisplay != ""

	if isWayland {
		portalBackend, err := newPortalBackend(modStrs, keyStr)
		if err == nil {
			log.Println("Hotkey: Using D-Bus GlobalShortcuts portal (Wayland)")
			return portalBackend
		}

		log.Printf("Hotkey: D-Bus portal unavailable (%v)", err)

		gnomeBackend, err := newGnomeBackend(modStrs, keyStr)
		if err == nil {
			log.Println("Hotkey: Using GNOME gsettings + D-Bus backend (Wayland)")
			return gnomeBackend
		}

		log.Printf("Hotkey: GNOME backend unavailable (%v), falling back to native", err)
	}

	log.Println("Hotkey: Using native hotkey backend")

	return newNativeBackend(mods, key)
}
