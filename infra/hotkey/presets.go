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

// Option is the macOS display name for the Alt/Option modifier.
const Option = "Option"

// BuildPresets returns the three preset hotkey configurations.
// Presets 2 and 3 adapt to the OS for idiomatic shortcuts.
func BuildPresets() []Preset {
	preset1 := Preset{
		ID:          PresetIndex1,
		Name:        "Preset 1",
		DisplayName: fmt.Sprintf("%s+%s+%s", Ctrl, Shift, DefaultKey),
		Modifiers:   DefaultModifiers(),
		Key:         DefaultKey,
	}

	var preset2, preset3 Preset

	if runtime.GOOS == "darwin" {
		// macOS: use Cmd-based combos with proper "Option" naming.
		preset2 = Preset{
			ID:          PresetIndex2,
			Name:        "Preset 2",
			DisplayName: fmt.Sprintf("%s+%s+%s", Cmd, Shift, DefaultKey),
			Modifiers:   []string{Cmd, Shift},
			Key:         DefaultKey,
		}
		preset3 = Preset{
			ID:          PresetIndex3,
			Name:        "Preset 3",
			DisplayName: fmt.Sprintf("%s+%s+%s", Cmd, Option, DefaultKey),
			Modifiers:   []string{Cmd, Alt}, // Alt maps to ModOption on macOS
			Key:         DefaultKey,
		}
	} else {
		// Windows/Linux: keep original combos.
		preset2 = Preset{
			ID:          PresetIndex2,
			Name:        "Preset 2",
			DisplayName: fmt.Sprintf("%s+%s+%s", Ctrl, Alt, DefaultKey),
			Modifiers:   []string{Ctrl, Alt},
			Key:         DefaultKey,
		}
		preset3 = Preset{
			ID:          PresetIndex3,
			Name:        "Preset 3",
			DisplayName: fmt.Sprintf("%s+%s", Win, DefaultKey),
			Modifiers:   []string{Win},
			Key:         DefaultKey,
		}
	}

	return []Preset{preset1, preset2, preset3}
}
