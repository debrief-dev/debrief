package syntax

import "testing"

func TestNormalizeWhitespace(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "Empty input",
			input:    "",
			expected: "",
		},
		{
			name:     "Basic multiple spaces",
			input:    "git   status",
			expected: "git status",
		},
		{
			name:     "Double quoted string with multiple spaces",
			input:    `git commit -m "test1  test2"`,
			expected: `git commit -m "test1  test2"`,
		},
		{
			name:     "Single quoted string with multiple spaces",
			input:    `git commit -m 'test1  test2'`,
			expected: `git commit -m 'test1  test2'`,
		},
		{
			name:     "Multiple spaces outside and inside quotes",
			input:    `git  commit  -m  "test1  test2"  -a`,
			expected: `git commit -m "test1  test2" -a`,
		},
		{
			name:     "Multiple quoted sections",
			input:    `echo "hello  world" and "foo  bar"`,
			expected: `echo "hello  world" and "foo  bar"`,
		},
		{
			name:     "Mixed single and double quotes",
			input:    `echo "double  space" and 'single  space'`,
			expected: `echo "double  space" and 'single  space'`,
		},
		{
			name:     "Tabs outside quotes",
			input:    "git\t\tstatus",
			expected: "git status",
		},
		{
			name:     "Tabs inside quotes",
			input:    "echo \"tab\t\there\"",
			expected: "echo \"tab\t\there\"",
		},
		{
			name:     "Empty quotes",
			input:    `echo "" and ''`,
			expected: `echo "" and ''`,
		},
		{
			name:     "Quote at start",
			input:    `"test  value" -flag`,
			expected: `"test  value" -flag`,
		},
		{
			name:     "Quote at end",
			input:    `command -m "test  value"`,
			expected: `command -m "test  value"`,
		},
		{
			name:     "Escaped quotes",
			input:    `echo \"not  quoted\"`,
			expected: `echo \"not quoted\"`,
		},
		{
			name:     "Single quote inside double quotes",
			input:    `echo "it's  a  test"`,
			expected: `echo "it's  a  test"`,
		},
		{
			name:     "Double quote inside single quotes",
			input:    `echo 'he said "hello  world"'`,
			expected: `echo 'he said "hello  world"'`,
		},
		{
			name:     "Complex command with multiple elements",
			input:    `docker  run  -v  "/path/to  dir:/mount"  --name  "my  container"  image`,
			expected: `docker run -v "/path/to  dir:/mount" --name "my  container" image`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := NormalizeWhitespace(tt.input)
			if result != tt.expected {
				t.Errorf("NormalizeWhitespace() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestNormalizeWhitespaceUnclosedQuotes(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "Unclosed double quote",
			input:    `go build -ldflags="-H`,
			expected: `go build -ldflags="-H`,
		},
		{
			name:     "Closed double quote",
			input:    `go build -ldflags="-H windowsgui"`,
			expected: `go build -ldflags="-H windowsgui"`,
		},
		{
			name:     "Unclosed double quote with multiple spaces",
			input:    `go  build  -ldflags="-H  test`,
			expected: `go build -ldflags="-H  test`,
		},
		{
			name:     "Unclosed single quote",
			input:    `echo 'test  value`,
			expected: `echo 'test  value`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := NormalizeWhitespace(tt.input)
			if result != tt.expected {
				t.Errorf("NormalizeWhitespace() = %q, want %q", result, tt.expected)
			}
		})
	}
}
