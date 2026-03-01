//go:build linux

package hotkey

import "testing"

func TestParseGsettingsArray(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  []string
	}{
		{"empty @as", "@as []", nil},
		{"empty brackets", "[]", nil},
		{"single path", "['/org/gnome/foo/']", []string{"/org/gnome/foo/"}},
		{
			"multiple paths",
			"['/org/gnome/foo/', '/org/gnome/bar/']",
			[]string{"/org/gnome/foo/", "/org/gnome/bar/"},
		},
		{"empty string inside brackets", "", nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseGsettingsArray(tt.input)

			if len(got) != len(tt.want) {
				t.Fatalf("parseGsettingsArray(%q) = %v (len %d), want %v (len %d)",
					tt.input, got, len(got), tt.want, len(tt.want))
			}

			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("parseGsettingsArray(%q)[%d] = %q, want %q",
						tt.input, i, got[i], tt.want[i])
				}
			}
		})
	}
}
