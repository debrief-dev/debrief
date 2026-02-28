//go:build !darwin

package tray

import "github.com/debrief-dev/debrief/assets"

// GetIcon returns the embedded icon data for Windows/Linux (ICO format)
func GetIcon() []byte {
	return assets.Favicon32ICO
}
