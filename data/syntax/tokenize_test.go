package syntax

import "testing"

// tokenizeTestCase is a shared table-driven test case for tokenization functions.
type tokenizeTestCase struct {
	name     string
	input    string
	expected []string
}

// runTokenizeTests runs a table of tokenization test cases against the given function.
func runTokenizeTests(t *testing.T, tests []tokenizeTestCase, fn func(string) []string) {
	t.Helper()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := fn(tt.input)
			if len(result) != len(tt.expected) {
				t.Errorf("returned %d tokens, want %d\nGot: %v\nWant: %v",
					len(result), len(tt.expected), result, tt.expected)

				return
			}

			for i := range result {
				if result[i] != tt.expected[i] {
					t.Errorf("Token %d: got %q, want %q", i, result[i], tt.expected[i])
				}
			}
		})
	}
}

func TestTokenizeWithQuotes(t *testing.T) {
	tests := []tokenizeTestCase{
		{
			name:     "Simple command without quotes",
			input:    "git status",
			expected: []string{"git", "status"},
		},
		{
			name:     "Command with double quoted argument",
			input:    `git commit -m "test message"`,
			expected: []string{"git", "commit", "-m", `"test message"`},
		},
		{
			name:     "Command with single quoted argument",
			input:    `git commit -m 'test message'`,
			expected: []string{"git", "commit", "-m", `'test message'`},
		},
		{
			name:     "PowerShell command with quoted query",
			input:    `Get-CimInstance -Query 'Select * from Win32_PhysicalMemory'`,
			expected: []string{"Get-CimInstance", "-Query", `'Select * from Win32_PhysicalMemory'`},
		},
		{
			name:     "Multiple quoted sections",
			input:    `echo "hello world" and "foo bar"`,
			expected: []string{"echo", `"hello world"`, "and", `"foo bar"`},
		},
		{
			name:     "Mixed quotes",
			input:    `command "double" 'single' unquoted`,
			expected: []string{"command", `"double"`, `'single'`, "unquoted"},
		},
		{
			name:     "Empty quotes",
			input:    `echo "" and ''`,
			expected: []string{"echo", `""`, "and", `''`},
		},
		{
			name:     "Quotes with spaces inside",
			input:    `docker run --name "my container" image`,
			expected: []string{"docker", "run", "--name", `"my container"`, "image"},
		},
		{
			name:     "Unclosed quote",
			input:    `go build -ldflags="-H`,
			expected: []string{"go", "build", `-ldflags="-H`},
		},
		{
			name:     "Single quote inside double quotes",
			input:    `echo "it's a test"`,
			expected: []string{"echo", `"it's a test"`},
		},
		{
			name:     "Double quote inside single quotes",
			input:    `echo 'he said "hello"'`,
			expected: []string{"echo", `'he said "hello"'`},
		},
	}

	runTokenizeTests(t, tests, tokenizeWithQuotes)
}

func TestTokenizeWithQuotesEmpty(t *testing.T) {
	result := tokenizeWithQuotes("")
	if len(result) != 0 {
		t.Errorf("Expected 0 tokens for empty input, got %d: %v", len(result), result)
	}
}

func TestTokenizeWithQuotesEscapedBackslash(t *testing.T) {
	tests := []tokenizeTestCase{
		{
			name:     "Escaped backslash before quote",
			input:    `echo \\"hello"`,
			expected: []string{`echo`, `\\"hello"`},
		},
		{
			name:     "Escaped quote outside quotes",
			input:    `echo \"hello world\"`,
			expected: []string{`echo`, `\"hello`, `world\"`},
		},
	}

	runTokenizeTests(t, tests, tokenizeWithQuotes)
}

func TestTokenizeCommand(t *testing.T) {
	tests := []tokenizeTestCase{
		{
			name:     "PowerShell cmdlet with quoted argument",
			input:    `Get-CimInstance -Query 'Select * from Win32_PhysicalMemory'`,
			expected: []string{"Get-CimInstance", "-Query", `'Select * from Win32_PhysicalMemory'`},
		},
		{
			name:     "Git command with quoted message",
			input:    `git commit -m "initial commit"`,
			expected: []string{"git", "commit", "-m", `"initial commit"`},
		},
		{
			name:     "Docker command with quoted container name",
			input:    `docker run --name "my container" image`,
			expected: []string{"docker", "run", "--name", `"my container"`, "image"},
		},
		{
			name:     "Bash function definition",
			input:    `function deploy() { git push && echo "done"; }`,
			expected: []string{`function deploy() { git push && echo "done"; }`},
		},
		{
			name:     "PowerShell function definition",
			input:    `function Get-Data { Write-Host "test" }`,
			expected: []string{`function Get-Data { Write-Host "test" }`},
		},
	}

	runTokenizeTests(t, tests, TokenizeCommand)
}
