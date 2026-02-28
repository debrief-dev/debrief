package shell

import (
	"testing"

	"github.com/debrief-dev/debrief/model"
)

func TestNewShellManager(t *testing.T) {
	sm := NewShellManager()

	if sm == nil {
		t.Fatal("Expected non-nil ShellManager")
	}

	enabled := sm.Enabled()
	if len(enabled) != 0 {
		t.Errorf("Expected empty enabled list, got %d", len(enabled))
	}
}

func TestInsertSortedAddsInOrder(t *testing.T) {
	sm := NewShellManager()

	sm.mu.Lock()
	sm.insertSorted(&ShellMetadata{Type: model.Zsh, Path: "/zsh"})
	sm.insertSorted(&ShellMetadata{Type: model.Bash, Path: "/bash"})
	sm.insertSorted(&ShellMetadata{Type: model.Fish, Path: "/fish"})
	sm.mu.Unlock()

	enabled := sm.Enabled()
	if len(enabled) != 3 {
		t.Fatalf("Expected 3 entries, got %d", len(enabled))
	}

	// Should be sorted by Type.
	for i := 1; i < len(enabled); i++ {
		if enabled[i-1].Type > enabled[i].Type {
			t.Errorf("Not sorted: %v > %v", enabled[i-1].Type, enabled[i].Type)
		}
	}
}

func TestInsertSortedReplacesDuplicate(t *testing.T) {
	sm := NewShellManager()

	sm.mu.Lock()
	sm.insertSorted(&ShellMetadata{Type: model.Bash, Path: "/old"})
	sm.insertSorted(&ShellMetadata{Type: model.Bash, Path: "/new"})
	sm.mu.Unlock()

	enabled := sm.Enabled()
	if len(enabled) != 1 {
		t.Fatalf("Expected 1 entry after replacement, got %d", len(enabled))
	}

	if enabled[0].Path != "/new" {
		t.Errorf("Expected replaced path '/new', got %q", enabled[0].Path)
	}
}

func TestEnabledReturnsCOWSlice(t *testing.T) {
	sm := NewShellManager()

	sm.mu.Lock()
	sm.insertSorted(&ShellMetadata{Type: model.Bash, Path: "/bash"})
	sm.mu.Unlock()

	snap1 := sm.Enabled()

	sm.mu.Lock()
	sm.insertSorted(&ShellMetadata{Type: model.Zsh, Path: "/zsh"})
	sm.mu.Unlock()

	snap2 := sm.Enabled()

	// snap1 should still have 1 entry (COW semantics).
	if len(snap1) != 1 {
		t.Errorf("Expected snap1 to have 1 entry, got %d", len(snap1))
	}

	if len(snap2) != 2 {
		t.Errorf("Expected snap2 to have 2 entries, got %d", len(snap2))
	}
}
