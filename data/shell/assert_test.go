package shell

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/debrief-dev/debrief/data/model"
)

// parseTestHistory writes content to a temp file and parses it with the given parser.
func parseTestHistory(t *testing.T, parser ShellParser, content, filename string) []*model.CommandEntry {
	t.Helper()

	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, filename)

	err := os.WriteFile(tmpFile, []byte(content), 0o600)
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}

	commands, err := parser.ParseHistoryFile(tmpFile)
	if err != nil {
		t.Fatalf("Failed to parse history file: %v", err)
	}

	return commands
}

// assertCommandTexts checks that commands match the expected texts in order.
func assertCommandTexts(t *testing.T, commands []*model.CommandEntry, expected []string) {
	t.Helper()

	if len(commands) != len(expected) {
		t.Fatalf("Expected %d commands, got %d", len(expected), len(commands))
	}

	for i, exp := range expected {
		if commands[i].Command != exp {
			t.Errorf("Command %d: expected %q, got %q", i+1, exp, commands[i].Command)
		}
	}
}

// assertLineNumber checks that a specific command has the expected first line number.
func assertLineNumber(t *testing.T, commands []*model.CommandEntry, cmd string, expectedLine int) {
	t.Helper()

	for _, c := range commands {
		if c.Command == cmd {
			if len(c.LineNumbers) < 1 || c.LineNumbers[0] != expectedLine {
				t.Errorf("Expected %q at line %d, got %v", cmd, expectedLine, c.LineNumbers)
			}

			return
		}
	}

	t.Errorf("Command %q not found", cmd)
}

// assertFrequency checks that a specific command has the expected frequency.
func assertFrequency(t *testing.T, commands []*model.CommandEntry, cmd string, expectedFreq int) {
	t.Helper()

	for _, c := range commands {
		if c.Command == cmd {
			if c.Frequency != expectedFreq {
				t.Errorf("Expected %q frequency %d, got %d", cmd, expectedFreq, c.Frequency)
			}

			return
		}
	}

	t.Errorf("Command %q not found", cmd)
}

// assertSingleCommand writes content to a temp file, parses it with the given source,
// and asserts exactly one command matching expected.
func assertSingleCommand(t *testing.T, source ShellParser, content, filename, expected, context string) {
	t.Helper()

	commands := parseTestHistory(t, source, content, filename)

	if len(commands) != 1 {
		t.Errorf("Expected 1 command (%s), got %d", context, len(commands))

		for i, cmd := range commands {
			t.Logf("Command %d: %s", i+1, cmd.Command)
		}
	}

	if len(commands) > 0 && commands[0].Command != expected {
		t.Errorf("Command text incorrect: %s", commands[0].Command)
	}
}
