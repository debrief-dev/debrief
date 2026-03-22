package shell

import (
	"os"
	"path/filepath"
	"testing"
)

const (
	testBashFuncWithOps  = `function foo() { echo bar && echo baz; }`
	testBashQuotedOp     = `echo "foo && bar"`
	testBashIncompleteOp = `git add &&`
)

func TestBashParseWithCommandSplitting(t *testing.T) {
	// Create temp history file with mixed content
	content := `echo hello
git status && git commit -m "test"
function deploy() {
    git push && echo "deployed"
}
ls | grep foo | wc -l
mkdir test && cd test || echo "failed"
`

	// Create temp file
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "bash_history_test")

	err := os.WriteFile(tmpFile, []byte(content), 0o600)
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}

	// Parse the file
	bs := &BashShellParser{}

	commands, err := bs.ParseHistoryFile(tmpFile)
	if err != nil {
		t.Fatalf("Failed to parse history file: %v", err)
	}

	// Verify results
	// Expected commands (NOT split):
	// 1. echo hello
	// 2. git status && git commit -m "test"
	// 3. function deploy() { git push && echo "deployed" }
	// 4. ls | grep foo | wc -l
	// 5. mkdir test && cd test || echo "failed"

	expectedCount := 5
	if len(commands) != expectedCount {
		t.Errorf("Expected %d commands, got %d", expectedCount, len(commands))

		for i, cmd := range commands {
			t.Logf("Command %d: %s", i+1, cmd.Command)
		}
	}

	// Verify specific commands exist
	commandTexts := make(map[string]bool)
	for _, cmd := range commands {
		commandTexts[cmd.Command] = true
	}

	// Check that all original commands are present
	expectedCommands := []string{
		"echo hello",
		`git status && git commit -m "test"`,
		`function deploy() { git push && echo "deployed" }`,
		"ls | grep foo | wc -l",
		`mkdir test && cd test || echo "failed"`,
	}

	for _, expected := range expectedCommands {
		if !commandTexts[expected] {
			t.Errorf("Expected command not found: %s", expected)
		}
	}
}

func TestBashFunctionNotSplit(t *testing.T) {
	assertSingleCommand(t, &BashShellParser{}, testBashFuncWithOps, "bash_history_test", testBashFuncWithOps, "function not split")
}

func TestBashOperatorInQuotes(t *testing.T) {
	assertSingleCommand(t, &BashShellParser{}, testBashQuotedOp, "bash_history_test", testBashQuotedOp, "operator in quotes")
}

func TestBashIncompleteCommand(t *testing.T) {
	assertSingleCommand(t, &BashShellParser{}, testBashIncompleteOp, "bash_history_test", testBashIncompleteOp, "incomplete command")
}

func TestBashDeduplicationWithSplitting(t *testing.T) {
	content := `git status && git commit
git status
git commit`

	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "bash_history_test")

	err := os.WriteFile(tmpFile, []byte(content), 0o600)
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}

	bs := &BashShellParser{}

	commands, err := bs.ParseHistoryFile(tmpFile)
	if err != nil {
		t.Fatalf("Failed to parse history file: %v", err)
	}

	// Should be exactly 3 unique commands (original commands not split)
	if len(commands) != 3 {
		t.Errorf("Expected 3 unique commands, got %d", len(commands))

		for i, cmd := range commands {
			t.Logf("Command %d: %s (freq: %d)", i+1, cmd.Command, cmd.Frequency)
		}
	}

	// Check that all commands exist with frequency 1
	commandTexts := make(map[string]int)
	for _, cmd := range commands {
		commandTexts[cmd.Command] = cmd.Frequency
	}

	expectedCommands := map[string]int{
		"git status && git commit": 1,
		"git status":               1,
		"git commit":               1,
	}

	for expected, freq := range expectedCommands {
		if commandTexts[expected] != freq {
			t.Errorf("Expected command %s with frequency %d, got %d", expected, freq, commandTexts[expected])
		}
	}
}

func TestBashMultilineForLoop(t *testing.T) {
	content := "for i in 1 2 3\ndo\necho $i\ndone\necho after"

	commands := parseTestHistory(t, &BashShellParser{}, content, "bash_history_test")

	assertCommandTexts(t, commands, []string{
		"for i in 1 2 3 do echo $i done",
		"echo after",
	})
}

func TestBashMultilineWhileLoop(t *testing.T) {
	content := "while true\ndo\nsleep 1\ndone"

	assertSingleCommand(t, &BashShellParser{}, content, "bash_history_test",
		"while true do sleep 1 done", "multiline while loop")
}

func TestBashSingleLineLoop(t *testing.T) {
	assertSingleCommand(t, &BashShellParser{}, "for i in 1 2 3; do echo $i; done",
		"bash_history_test", "for i in 1 2 3; do echo $i; done", "single-line for loop")
}

func TestBashLoopWithInternalOperators(t *testing.T) {
	content := "for i in 1 2; do echo $i && echo step; done"

	assertSingleCommand(t, &BashShellParser{}, content, "bash_history_test",
		content, "loop with internal operators")
}

func TestBashLineNumbersPreserved(t *testing.T) {
	commands := parseTestHistory(t, &BashShellParser{}, "git add . && git commit\ngit push", "bash_history_test")

	assertLineNumber(t, commands, "git add . && git commit", 1)
	assertLineNumber(t, commands, "git push", 2)
}
