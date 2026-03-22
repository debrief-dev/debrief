package syntax

import "testing"

// boolTestCase is a table-driven test case for functions that return bool.
type boolTestCase struct {
	name     string
	input    string
	expected bool
}

// runBoolTests runs a table of bool test cases against the given function.
func runBoolTests(t *testing.T, funcName string, tests []boolTestCase, fn func(string) bool) {
	t.Helper()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := fn(tt.input)
			if result != tt.expected {
				t.Errorf("%s(%q) = %v, want %v", funcName, tt.input, result, tt.expected)
			}
		})
	}
}

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

func TestIsBalancedDoBlock(t *testing.T) {
	runBoolTests(t, "IsBalancedDoBlock", []boolTestCase{
		{"balanced for loop", "for x in 1 2; do echo $x; done", true},
		{"balanced while loop", "while true; do sleep 1; done", true},
		{"unbalanced - missing done", "for x in 1 2; do echo $x", false},
		{"unbalanced - extra done", "done", false},
		{"nested loops", "for i in 1; do for j in 2; do echo; done; done", true},
		{"no do/done at all", "echo hello", true},
		{"do/done in quotes ignored", `echo "do something done"`, true},
		{"docker not matched", "docker run nginx", true},
		{"undone not matched", "echo undone", true},
		{"dofile not matched", "dofile something", true},
		{"do at end of string", "for x in 1; do", false},
		{"done at start", "done; do echo; done", false},
		{"empty input", "", true},
	}, IsBalancedDoBlock)
}

func TestIsBalancedFishBlock(t *testing.T) {
	runBoolTests(t, "IsBalancedFishBlock", []boolTestCase{
		{"balanced for loop", "for x in 1 2; echo $x; end", true},
		{"balanced while loop", "while true; echo loop; end", true},
		{"unbalanced - missing end", "for x in 1 2; echo $x", false},
		{"nested blocks", "for x in 1; if true; echo yes; end; end", true},
		{"no block keywords", "echo hello", true},
		{"keywords in quotes", `echo "for while end"`, true},
		{"empty input", "", true},
	}, IsBalancedFishBlock)
}
