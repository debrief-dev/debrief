package syntax

import "testing"

func TestScannerStateAdvance(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		wantLive []bool
	}{
		{
			name:     "Plain text",
			input:    "abc",
			wantLive: []bool{true, true, true},
		},
		{
			name:     "Double quoted",
			input:    `"ab"`,
			wantLive: []bool{false, false, false, false},
		},
		{
			name:     "Single quoted",
			input:    `'ab'`,
			wantLive: []bool{false, false, false, false},
		},
		{
			name:     "Escaped double quote",
			input:    `\"a`,
			wantLive: []bool{false, false, true},
		},
		{
			name:     "Escaped backslash then quote",
			input:    `\\"`,
			wantLive: []bool{false, false, false},
		},
		{
			name:     "Backslash inside single quotes is literal",
			input:    `'\\'`,
			wantLive: []bool{false, false, false, false},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var s ScannerState

			for i := 0; i < len(tt.input); i++ {
				got := s.Advance(tt.input[i])
				if got != tt.wantLive[i] {
					t.Errorf("byte %d (%q): Advance() = %v, want %v",
						i, tt.input[i], got, tt.wantLive[i])
				}
			}
		})
	}
}

func TestBalancedBraces(t *testing.T) {
	tests := []struct {
		input    string
		expected bool
	}{
		{"{}", true},
		{"{ { } }", true},
		{"echo { test }", true},
		{"{", false},
		{"}", false},
		{"{ } }", false},
		{"{ { }", false},
		{`echo "{ }"`, true},   // Braces in quotes
		{`echo '{ } {'`, true}, // Braces in quotes
		{"function f() {}", true},
		{"", true},
	}

	for _, test := range tests {
		result := IsBalancedBraces(test.input)
		if result != test.expected {
			t.Errorf("For input %q, expected %v, got %v", test.input, test.expected, result)
		}
	}
}
