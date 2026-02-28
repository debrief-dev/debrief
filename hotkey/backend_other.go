//go:build !linux

package hotkey

import hk "golang.design/x/hotkey"

// newBackend creates the platform-appropriate hotkey backend.
// On non-Linux platforms, always use the native hk library (Win32/Cocoa).
// The modStrs/keyStr parameters are unused on this platform.
func newBackend(mods []hk.Modifier, key hk.Key, _ []string, _ string) backend {
	return newNativeBackend(mods, key)
}
