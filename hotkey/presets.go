package hotkey

import (
	"fmt"
	"runtime"
)

// Preset index constants.
const (
	PresetIndex1 = iota
	PresetIndex2
	PresetIndex3
	PresetCount
)

// DefaultKey is the default hotkey key (used by Preset 1 and config fallback).
const DefaultKey = "H"

// Preset defines a pre-configured hotkey combination.
type Preset struct {
	ID          int      // 0, 1, 2
	Name        string   // "Preset 1", "Preset 2", "Preset 3"
	DisplayName string   // "Ctrl+Shift+H", "Ctrl+Alt+H", "Win+H"
	Modifiers   []string // ["Ctrl", "Shift"], ["Ctrl", "Alt"], ["Win"]
	Key         string   // "H"
}

// DefaultModifiers returns the default hotkey modifiers (Preset 1).
// Returns a fresh slice each call to prevent aliasing.
func DefaultModifiers() []string {
	return []string{Ctrl, Shift}
}

// BuildPresets returns the three preset hotkey configurations.
// Preset 3 adapts to the OS (Win on Windows/Linux, Cmd on macOS).
func BuildPresets() []Preset {
	preset3Modifier := Win
	if runtime.GOOS == "darwin" {
		preset3Modifier = Cmd
	}

	return []Preset{
		{
			ID:          PresetIndex1,
			Name:        "Preset 1",
			DisplayName: fmt.Sprintf("%s+%s+%s", Ctrl, Shift, DefaultKey),
			Modifiers:   DefaultModifiers(),
			Key:         DefaultKey,
		},
		{
			ID:          PresetIndex2,
			Name:        "Preset 2",
			DisplayName: fmt.Sprintf("%s+%s+%s", Ctrl, Alt, DefaultKey),
			Modifiers:   []string{Ctrl, Alt},
			Key:         DefaultKey,
		},
		{
			ID:          PresetIndex3,
			Name:        "Preset 3",
			DisplayName: fmt.Sprintf("%s+%s", preset3Modifier, DefaultKey),
			Modifiers:   []string{preset3Modifier},
			Key:         DefaultKey,
		},
	}
}
