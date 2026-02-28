package hotkey

import (
	"fmt"
	"testing"

	hk "golang.design/x/hotkey"
)

// runConversionTests is a generic table-driven test runner for functions that
// convert a string to a typed value, returning (T, error).
func runConversionTests[T comparable](t *testing.T, fn func(string) (T, error), tests []struct {
	input   string
	want    T
	wantErr bool
},
) {
	t.Helper()

	for _, tt := range tests {
		t.Run(fmt.Sprintf("input=%q", tt.input), func(t *testing.T) {
			t.Helper()

			got, err := fn(tt.input)
			if (err != nil) != tt.wantErr {
				t.Fatalf("error = %v, wantErr %v", err, tt.wantErr)
			}

			if got != tt.want {
				t.Errorf("got %v, want %v", got, tt.want)
			}
		})
	}
}

func TestStringToModifier(t *testing.T) {
	runConversionTests(t, StringToModifier, []struct {
		input   string
		want    hk.Modifier
		wantErr bool
	}{
		{Ctrl, hk.ModCtrl, false},
		{Shift, hk.ModShift, false},
		{Alt, modAlt, false},
		{Win, modSuper, false},
		{Cmd, modSuper, false},
		{"", 0, true},
		{"Meta", 0, true},
		{"ctrl", 0, true},
	})
}

func TestStringToKey(t *testing.T) {
	runConversionTests(t, StringToKey, []struct {
		input   string
		want    hk.Key
		wantErr bool
	}{
		{"A", hk.KeyA, false},
		{"Z", hk.KeyZ, false},
		{"H", hk.KeyH, false},
		{"0", hk.Key0, false},
		{"9", hk.Key9, false},
		{"Space", hk.KeySpace, false},
		{"a", 0, true},
		{"", 0, true},
		{"AB", 0, true},
		{"F1", 0, true},
		// Multi-byte rune that is 1 rune but >1 byte should fail.
		{"\u00e9", 0, true},
	})
}

func TestConvertStrings(t *testing.T) {
	t.Run("valid", func(t *testing.T) {
		mods, key, err := ConvertStrings([]string{Ctrl, Shift}, "H")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if len(mods) != 2 {
			t.Fatalf("expected 2 modifiers, got %d", len(mods))
		}

		if mods[0] != hk.ModCtrl {
			t.Errorf("mods[0] = %v, want ModCtrl", mods[0])
		}

		if mods[1] != hk.ModShift {
			t.Errorf("mods[1] = %v, want ModShift", mods[1])
		}

		if key != hk.KeyH {
			t.Errorf("key = %v, want KeyH", key)
		}
	})

	t.Run("bad modifier", func(t *testing.T) {
		_, _, err := ConvertStrings([]string{"BadMod"}, "H")
		if err == nil {
			t.Fatal("expected error for bad modifier")
		}
	})

	t.Run("bad key", func(t *testing.T) {
		_, _, err := ConvertStrings([]string{Ctrl}, "bad")
		if err == nil {
			t.Fatal("expected error for bad key")
		}
	})

	t.Run("empty modifiers", func(t *testing.T) {
		mods, key, err := ConvertStrings(nil, "A")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if len(mods) != 0 {
			t.Errorf("expected 0 modifiers, got %d", len(mods))
		}

		if key != hk.KeyA {
			t.Errorf("key = %v, want KeyA", key)
		}
	})
}

func TestPresetConstants(t *testing.T) {
	if PresetIndex1 != 0 {
		t.Errorf("PresetIndex1 = %d, want 0", PresetIndex1)
	}

	if PresetIndex2 != 1 {
		t.Errorf("PresetIndex2 = %d, want 1", PresetIndex2)
	}

	if PresetIndex3 != 2 {
		t.Errorf("PresetIndex3 = %d, want 2", PresetIndex3)
	}

	if PresetCount != 3 {
		t.Errorf("PresetCount = %d, want 3", PresetCount)
	}
}

func TestBuildPresets(t *testing.T) {
	presets := BuildPresets()

	if len(presets) != PresetCount {
		t.Fatalf("expected %d presets, got %d", PresetCount, len(presets))
	}

	for i, p := range presets {
		if p.ID != i {
			t.Errorf("preset[%d].ID = %d, want %d", i, p.ID, i)
		}

		if p.Key != DefaultKey {
			t.Errorf("preset[%d].Key = %q, want %q", i, p.Key, DefaultKey)
		}

		if len(p.Modifiers) == 0 {
			t.Errorf("preset[%d].Modifiers is empty", i)
		}

		if p.DisplayName == "" {
			t.Errorf("preset[%d].DisplayName is empty", i)
		}

		if p.Name == "" {
			t.Errorf("preset[%d].Name is empty", i)
		}
	}
}

func TestDefaultModifiersNonAliasing(t *testing.T) {
	a := DefaultModifiers()
	b := DefaultModifiers()

	const sentinel = "mutated"

	a[0] = sentinel

	if b[0] == sentinel {
		t.Error("DefaultModifiers returned aliased slices")
	}
}
