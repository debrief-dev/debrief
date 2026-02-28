//go:build linux

package hotkey

import "testing"

func TestBuildTriggerString(t *testing.T) {
	tests := []struct {
		name    string
		modStrs []string
		keyStr  string
		want    string
	}{
		{"ctrl+shift+h", []string{Ctrl, Shift}, "H", "CTRL+SHIFT+h"},
		{"alt+a", []string{Alt}, "A", "ALT+a"},
		{"super+h", []string{Win}, "H", "SUPER+h"},
		{"cmd+h", []string{Cmd}, "H", "SUPER+h"},
		{"no modifiers", nil, "H", "h"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := buildTriggerString(tt.modStrs, tt.keyStr)
			if got != tt.want {
				t.Errorf("buildTriggerString(%v, %q) = %q, want %q",
					tt.modStrs, tt.keyStr, got, tt.want)
			}
		})
	}
}
