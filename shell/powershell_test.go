package shell

import (
	"os"
	"path/filepath"
	"testing"
)

const (
	testPSFuncWithOps = `function Get-Status { git status && git log }`
)

func TestPowerShellParseWithCommandSplitting(t *testing.T) {
	content := `Get-Process | Select-Object Name
git status && git commit -m "test"
function Get-Data {
    Write-Host "test"
}
ls | grep foo
`

	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "powershell_history_test")

	err := os.WriteFile(tmpFile, []byte(content), 0o600)
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}

	ps := &PowerShellParser{}

	commands, err := ps.ParseHistoryFile(tmpFile)
	if err != nil {
		t.Fatalf("Failed to parse history file: %v", err)
	}

	// Expected commands (NOT split):
	// 1. Get-Process | Select-Object Name
	// 2. git status && git commit -m "test"
	// 3. function Get-Data { Write-Host "test" }
	// 4. ls | grep foo

	expectedCount := 4
	if len(commands) != expectedCount {
		t.Errorf("Expected %d commands, got %d", expectedCount, len(commands))

		for i, cmd := range commands {
			t.Logf("Command %d: %s", i+1, cmd.Command)
		}
	}

	// Check that all original commands are present
	commandTexts := make(map[string]bool)
	for _, cmd := range commands {
		commandTexts[cmd.Command] = true
	}

	expectedCommands := []string{
		"Get-Process | Select-Object Name",
		`git status && git commit -m "test"`,
		`function Get-Data { Write-Host "test" }`,
		"ls | grep foo",
	}

	for _, expected := range expectedCommands {
		if !commandTexts[expected] {
			t.Errorf("Expected command not found: %s", expected)
		}
	}
}

func TestPowerShellFunctionNotSplit(t *testing.T) {
	assertSingleCommand(t, &PowerShellParser{}, testPSFuncWithOps, "powershell_history_test", testPSFuncWithOps, "function not split")
}
