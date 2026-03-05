//go:build darwin

package tray

import "github.com/debrief-dev/debrief/assets"

// GetIcon returns the embedded icon data for macOS (PNG format)
func GetIcon() []byte {
	return assets.Favicon32PNG
}
