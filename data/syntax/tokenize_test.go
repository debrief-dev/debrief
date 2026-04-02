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
		{
			name:     "Command substitution kept as single token",
			input:    "echo $(date)",
			expected: []string{"echo", "$(date)"},
		},
		{
			name:     "Command substitution with spaces inside",
			input:    "echo $(echo hello | grep h)",
			expected: []string{"echo", "$(echo hello | grep h)"},
		},
		{
			name:     "Nested command substitution",
			input:    "echo $(echo $(date))",
			expected: []string{"echo", "$(echo $(date))"},
		},
		{
			name:     "Command substitution as assignment prefix",
			input:    "result=$(cmd) echo done",
			expected: []string{"result=$(cmd)", "echo", "done"},
		},
		{
			name:     "Arithmetic expansion kept as single token",
			input:    "echo $((1+2))",
			expected: []string{"echo", "$((1+2))"},
		},
		{
			name:     "Command substitution inside double quotes",
			input:    `echo "$(date)"`,
			expected: []string{"echo", `"$(date)"`},
		},
		{
			name:     "Dollar-paren inside single quotes is literal",
			input:    `echo '$(date)'`,
			expected: []string{"echo", `'$(date)'`},
		},
		{
			name:     "Backtick substitution kept as single token",
			input:    "echo `date -u`",
			expected: []string{"echo", "`date -u`"},
		},
		{
			name:     "Parameter expansion kept as single token",
			input:    "echo ${var:-default value}",
			expected: []string{"echo", "${var:-default value}"},
		},
		{
			name:     "Parameter expansion without spaces",
			input:    "echo ${HOME} done",
			expected: []string{"echo", "${HOME}", "done"},
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
		{
			name:     "Bash for loop as single token",
			input:    "for i in 1 2 3; do echo $i; done",
			expected: []string{"for i in 1 2 3; do echo $i; done"},
		},
		{
			name:     "PowerShell foreach as single token",
			input:    "foreach ($item in $list) { Write-Host $item }",
			expected: []string{"foreach ($item in $list) { Write-Host $item }"},
		},
		{
			name:     "Fish for loop as single token",
			input:    "for x in 1 2 3; echo $x; end",
			expected: []string{"for x in 1 2 3; echo $x; end"},
		},
		{
			name:     "Bash loop with internal operators as single token",
			input:    `for i in 1 2; do echo $i && echo "step"; done`,
			expected: []string{`for i in 1 2; do echo $i && echo "step"; done`},
		},
		{
			name:     "Single env var prefix merged with command",
			input:    "ENV1=VAL1 go fmt",
			expected: []string{"ENV1=VAL1 go", "fmt"},
		},
		{
			name:     "Multiple env var prefixes merged with command",
			input:    "GOOS=linux GOARCH=amd64 go build -o app",
			expected: []string{"GOOS=linux GOARCH=amd64 go", "build", "-o", "app"},
		},
		{
			name:     "Env var with underscore in key",
			input:    "MY_VAR=123 python script.py",
			expected: []string{"MY_VAR=123 python", "script.py"},
		},
		{
			name:     "No env var prefix unchanged",
			input:    "go fmt ./...",
			expected: []string{"go", "fmt", "./..."},
		},
		{
			name:     "Only env vars no command",
			input:    "FOO=bar BAZ=qux",
			expected: []string{"FOO=bar", "BAZ=qux"},
		},
		{
			name:     "Env var with empty value",
			input:    "DEBUG= go test",
			expected: []string{"DEBUG= go", "test"},
		},
		{
			name:     "Non-env-var with equals not merged",
			input:    "go build -ldflags=-s",
			expected: []string{"go", "build", "-ldflags=-s"},
		},
	}

	runTokenizeTests(t, tests, TokenizeCommand)
}

func TestIsEnvVarAssignment(t *testing.T) {
	tests := []struct {
		input    string
		expected bool
	}{
		{"FOO=bar", true},
		{"MY_VAR=123", true},
		{"_PRIVATE=val", true},
		{"A=B", true},
		{"DEBUG=", true},
		{"FOO=bar=baz", true},
		{"123=val", false},
		{"-flag=val", false},
		{"noequals", false},
		{"=val", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := isEnvVarAssignment(tt.input)
			if result != tt.expected {
				t.Errorf("isEnvVarAssignment(%q) = %v, want %v", tt.input, result, tt.expected)
			}
		})
	}
}
