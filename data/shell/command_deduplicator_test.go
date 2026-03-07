package shell

import (
	"testing"

	"github.com/debrief-dev/debrief/data/model"
)

func TestCommandDeduplicatorAdd(t *testing.T) {
	dedup := NewCommandDeduplicator(model.Bash)
	dedup.Add("git status", 1)
	dedup.Add("git commit", 2)
	dedup.Add("git status", 5)

	results := dedup.Results()

	if len(results) != 2 {
		t.Fatalf("Expected 2 unique commands, got %d", len(results))
	}
}

func TestCommandDeduplicatorFrequency(t *testing.T) {
	dedup := NewCommandDeduplicator(model.Bash)
	dedup.Add("git status", 1)
	dedup.Add("git status", 3)
	dedup.Add("git status", 7)

	results := dedup.Results()

	if len(results) != 1 {
		t.Fatalf("Expected 1 command, got %d", len(results))
	}

	if results[0].Frequency != 3 {
		t.Errorf("Expected frequency 3, got %d", results[0].Frequency)
	}
}

func TestCommandDeduplicatorSortedByLastLine(t *testing.T) {
	dedup := NewCommandDeduplicator(model.Bash)
	dedup.Add("cmd_a", 10)
	dedup.Add("cmd_b", 2)
	dedup.Add("cmd_c", 5)

	results := dedup.Results()

	if len(results) != 3 {
		t.Fatalf("Expected 3 commands, got %d", len(results))
	}

	// Should be sorted by last line number: cmd_b(2), cmd_c(5), cmd_a(10)
	expected := []string{"cmd_b", "cmd_c", "cmd_a"}
	for i, exp := range expected {
		if results[i].Command != exp {
			t.Errorf("Position %d: expected %q, got %q", i, exp, results[i].Command)
		}
	}
}

func TestCommandDeduplicatorShellType(t *testing.T) {
	dedup := NewCommandDeduplicator(model.PowerShell)
	dedup.Add("Get-Process", 1)

	results := dedup.Results()

	if len(results) != 1 {
		t.Fatalf("Expected 1 command, got %d", len(results))
	}

	if results[0].Shell != model.PowerShell {
		t.Errorf("Expected shell %v, got %v", model.PowerShell, results[0].Shell)
	}
}

func TestCommandDeduplicatorEmpty(t *testing.T) {
	dedup := NewCommandDeduplicator(model.Bash)
	results := dedup.Results()

	if len(results) != 0 {
		t.Errorf("Expected 0 commands, got %d", len(results))
	}
}

func TestCommandDeduplicatorLineNumbers(t *testing.T) {
	dedup := NewCommandDeduplicator(model.Bash)
	dedup.Add("git status", 1)
	dedup.Add("git status", 5)
	dedup.Add("git status", 10)

	results := dedup.Results()

	if len(results) != 1 {
		t.Fatalf("Expected 1 command, got %d", len(results))
	}

	lineNums := results[0].LineNumbers
	if len(lineNums) != 3 {
		t.Fatalf("Expected 3 line numbers, got %d", len(lineNums))
	}

	expectedLines := []int{1, 5, 10}
	for i, exp := range expectedLines {
		if lineNums[i] != exp {
			t.Errorf("LineNumber %d: expected %d, got %d", i, exp, lineNums[i])
		}
	}
}
