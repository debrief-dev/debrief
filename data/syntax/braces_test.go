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
		{
			name: "Command substitution $(cmd)",
			//           $ (  c  m  d  )
			input:    "$(cmd)",
			wantLive: []bool{true, false, false, false, false, false},
		},
		{
			name: "Command substitution with operators $(a && b)",
			//           $ (  a     &  &     b  )
			input:    "$(a && b)",
			wantLive: []bool{true, false, false, false, false, false, false, false, false},
		},
		{
			name: "Nested command substitution $(echo $(date))",
			//           $ (  e  c  h  o     $ (  d  a  t  e  )  )
			input:    "$(echo $(date))",
			wantLive: []bool{true, false, false, false, false, false, false, false, false, false, false, false, false, false, false},
		},
		{
			name: "Command sub inside double quotes",
			//           "      $  (  c  m  d  )  "
			input:    `"$(cmd)"`,
			wantLive: []bool{false, false, false, false, false, false, false, false},
		},
		{
			name: "Dollar-paren inside single quotes is literal",
			//           '      $  (  c  m  d  )  '
			input:    `'$(cmd)'`,
			wantLive: []bool{false, false, false, false, false, false, false, false},
		},
		{
			name: "Arithmetic expansion $((1+2))",
			//           $ (  (  1  +  2  )  )
			input:    "$((1+2))",
			wantLive: []bool{true, false, false, false, false, false, false, false},
		},
		{
			name: "Escaped dollar prevents substitution",
			//           \  $  (  c  m  d  )
			input:    `\$(cmd)`,
			wantLive: []bool{false, false, true, true, true, true, true},
		},
		{
			name: "After substitution closes chars are live",
			//           $ (  c  )     r
			input:    "$(c) r",
			wantLive: []bool{true, false, false, false, true, true},
		},
		{
			name: "Quoted close paren inside substitution",
			//           $ (  e  c  h  o     "     )     "     )
			input:    `$(echo ")")`,
			wantLive: []bool{true, false, false, false, false, false, false, false, false, false, false},
		},
		// Backtick command substitution tests.
		{
			name: "Backtick substitution",
			//           `  c  m  d  `
			input:    "`cmd`",
			wantLive: []bool{false, false, false, false, false},
		},
		{
			name: "Backtick with operators inside",
			//           e  c  h  o     `  a     &  &     b  `
			input:    "echo `a && b`",
			wantLive: []bool{true, true, true, true, true, false, false, false, false, false, false, false, false},
		},
		{
			name: "Backtick inside double quotes",
			//           "     `  c  m  d  `     "
			input:    "\"`cmd`\"",
			wantLive: []bool{false, false, false, false, false, false, false},
		},
		{
			name:  "Escaped backtick is literal",
			input: "\\`cmd\\`",
			//           \  `  c  m  d  \  `
			wantLive: []bool{false, false, true, true, true, false, false},
		},
		{
			name: "After backtick closes chars are live",
			//           `  c  `     r
			input:    "`c` r",
			wantLive: []bool{false, false, false, true, true},
		},
		// ${...} parameter expansion tests.
		{
			name: "Parameter expansion ${var}",
			//           $ {  v  a  r  }
			input:    "${var}",
			wantLive: []bool{true, false, false, false, false, false},
		},
		{
			name: "Parameter expansion with default ${var:-default}",
			//           $ {  v  a  r  :  -  d  e  f  a  u  l  t  }
			input:    "${var:-default}",
			wantLive: []bool{true, false, false, false, false, false, false, false, false, false, false, false, false, false, false},
		},
		{
			name: "Nested $() inside ${}",
			//           $ {  v  a  r  :  -  $ (  c  m  d  )  }
			input:    "${var:-$(cmd)}",
			wantLive: []bool{true, false, false, false, false, false, false, false, false, false, false, false, false, false},
		},
		{
			name: "After ${} closes chars are live",
			//           $ {  v  }     r
			input:    "${v} r",
			wantLive: []bool{true, false, false, false, true, true},
		},
		{
			name: "${} inside double quotes",
			//           "     $ {  v  }     "
			input:    `"${v}"`,
			wantLive: []bool{false, false, false, false, false, false},
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
