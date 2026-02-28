package tree

import (
	"testing"

	"github.com/debrief-dev/debrief/model"
)

const (
	testWordGit    = "git"
	testWordLA     = "-la"
	testMetaOneCmd = "1 command"
)

func TestBuildCommandMetadata(t *testing.T) {
	tests := []struct {
		name       string
		frequency  int
		sourceName string
		expected   string
	}{
		{
			name:       "singular",
			frequency:  1,
			sourceName: "bash",
			expected:   "Used 1 time · bash",
		},
		{
			name:       "plural",
			frequency:  5,
			sourceName: "PowerShell",
			expected:   "Used 5 times · PowerShell",
		},
		{
			name:       "zero",
			frequency:  0,
			sourceName: "zsh",
			expected:   "Used 0 times · zsh",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := BuildCommandMetadata(tt.frequency, tt.sourceName)
			if result != tt.expected {
				t.Errorf("got %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestBuildCommandCountMetadata(t *testing.T) {
	tests := []struct {
		name     string
		count    int
		expected string
	}{
		{
			name:     "singular",
			count:    1,
			expected: "1 command",
		},
		{
			name:     "plural",
			count:    10,
			expected: "10 commands",
		},
		{
			name:     "zero",
			count:    0,
			expected: "0 commands",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := buildCommandCountMetadata(tt.count)
			if result != tt.expected {
				t.Errorf("got %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestFlattenForDisplay_NilRoot(t *testing.T) {
	result := FlattenForDisplay(nil, 0, nil, nil)
	if result != nil {
		t.Errorf("expected nil for nil root, got %v", result)
	}
}

func TestFlattenForDisplay_Basic(t *testing.T) {
	commands := []*model.CommandEntry{
		{Command: "git status", Frequency: 3, Shell: model.Bash},
		{Command: "git commit", Frequency: 2, Shell: model.Bash},
		{Command: "ls -la", Frequency: 1, Shell: model.Bash},
	}

	root := Build(commands)
	PreSortChildren(root)

	flattened := FlattenForDisplay(root, 10, nil, nil)

	if len(flattened) == 0 {
		t.Fatal("expected non-empty flattened result")
	}

	// First node should be "git" (highest command count)
	if flattened[0].Node.Word != testWordGit {
		t.Errorf("first node word: got %q, want %q", flattened[0].Node.Word, testWordGit)
	}

	if flattened[0].Depth != 0 {
		t.Errorf("first node depth: got %d, want 0", flattened[0].Depth)
	}

	if !flattened[0].HasChildren {
		t.Error("git node should have children")
	}

	// Check that git branch metadata reflects the 2 commands passing through it
	if flattened[0].CachedMetadata != "2 commands" {
		t.Errorf("git branch metadata: got %q, want %q", flattened[0].CachedMetadata, "2 commands")
	}
}

func TestFlattenForDisplay_LeafMetadata(t *testing.T) {
	commands := []*model.CommandEntry{
		{Command: "git status", Frequency: 5, Shell: model.GitBash},
	}

	root := Build(commands)
	PreSortChildren(root)

	flattened := FlattenForDisplay(root, 10, nil, nil)

	// Find the leaf node "status"
	var leaf *model.TreeDisplayNode

	for _, node := range flattened {
		if node.IsLeaf {
			leaf = node

			break
		}
	}

	if leaf == nil {
		t.Fatal("expected a leaf node")
	}

	if leaf.FilteredFrequency != 5 {
		t.Errorf("leaf FilteredFrequency: got %d, want 5", leaf.FilteredFrequency)
	}

	expected := BuildCommandMetadata(5, model.GitBash.String())
	if leaf.CachedMetadata != expected {
		t.Errorf("leaf metadata: got %q, want %q", leaf.CachedMetadata, expected)
	}
}

func TestFlattenForDisplay_SearchFilter(t *testing.T) {
	cmd1 := &model.CommandEntry{Command: "git status", Frequency: 3, Shell: model.Bash}
	cmd2 := &model.CommandEntry{Command: "git commit", Frequency: 2, Shell: model.Bash}
	cmd3 := &model.CommandEntry{Command: "ls -la", Frequency: 1, Shell: model.Bash}

	commands := []*model.CommandEntry{cmd1, cmd2, cmd3}

	root := Build(commands)
	PreSortChildren(root)

	// Only match "git status"
	matchingCommands := map[*model.CommandEntry]bool{
		cmd1: true,
	}

	flattened := FlattenForDisplay(root, 10, matchingCommands, nil)

	// Should only include "git" and "status" nodes (ls subtree filtered out)
	for _, node := range flattened {
		if node.Node.Word == "ls" || node.Node.Word == testWordLA {
			t.Errorf("ls subtree should be filtered out, found %q", node.Node.Word)
		}
	}

	// Find git branch node and verify filtered metadata
	for _, node := range flattened {
		if node.Node.Word == testWordGit && node.HasChildren {
			// Only 1 command matches the filter under git
			if node.CachedMetadata != testMetaOneCmd {
				t.Errorf("filtered git branch metadata: got %q, want %q", node.CachedMetadata, testMetaOneCmd)
			}
		}
	}
}

func TestFlattenForDisplay_ShellFilter(t *testing.T) {
	cmd1 := &model.CommandEntry{Command: "git status", Frequency: 3, Shell: model.Bash}
	cmd2 := &model.CommandEntry{Command: "git commit", Frequency: 2, Shell: model.PowerShell}

	commands := []*model.CommandEntry{cmd1, cmd2}

	root := Build(commands)
	PreSortChildren(root)

	// Only show Bash commands
	shellFilter := map[model.Shell]bool{
		model.Bash: true,
	}

	flattened := FlattenForDisplay(root, 10, nil, shellFilter)

	// "commit" should be filtered out
	for _, node := range flattened {
		if node.Node.Word == "commit" {
			t.Error("commit node should be filtered out (PowerShell)")
		}
	}

	// git branch should show 1 filtered command
	for _, node := range flattened {
		if node.Node.Word == testWordGit && node.HasChildren {
			if node.CachedMetadata != testMetaOneCmd {
				t.Errorf("shell-filtered git branch metadata: got %q, want %q", node.CachedMetadata, testMetaOneCmd)
			}
		}
	}
}

func TestFlattenForDisplay_PathConstruction(t *testing.T) {
	commands := []*model.CommandEntry{
		{Command: "git commit -m", Frequency: 1, Shell: model.Bash},
	}

	root := Build(commands)
	PreSortChildren(root)

	flattened := FlattenForDisplay(root, 10, nil, nil)

	// Expected paths at each depth
	expectedPaths := map[string]string{
		"git":    "git",
		"commit": "git commit",
		"-m":     "git commit -m",
	}

	expectedPrefixes := map[string]string{
		"git":    "",
		"commit": "git",
		"-m":     "git commit",
	}

	for _, node := range flattened {
		word := node.Node.Word

		if expected, ok := expectedPaths[word]; ok {
			if node.Path != expected {
				t.Errorf("node %q path: got %q, want %q", word, node.Path, expected)
			}
		}

		if expected, ok := expectedPrefixes[word]; ok {
			if node.PathPrefix != expected {
				t.Errorf("node %q prefix: got %q, want %q", word, node.PathPrefix, expected)
			}

			if expected != "" {
				expectedWithSpace := expected + " "
				if node.PathPrefixWithSpace != expectedWithSpace {
					t.Errorf("node %q prefix with space: got %q, want %q", word, node.PathPrefixWithSpace, expectedWithSpace)
				}
			}
		}
	}
}

func TestFlattenForDisplay_DepthOrdering(t *testing.T) {
	commands := []*model.CommandEntry{
		{Command: "git commit -m", Frequency: 1, Shell: model.Bash},
	}

	root := Build(commands)
	PreSortChildren(root)

	flattened := FlattenForDisplay(root, 10, nil, nil)

	if len(flattened) != 3 {
		t.Fatalf("expected 3 nodes (git, commit, -m), got %d", len(flattened))
	}

	// Depths should be 0, 1, 2
	for i, expected := range []int{0, 1, 2} {
		if flattened[i].Depth != expected {
			t.Errorf("node %d depth: got %d, want %d", i, flattened[i].Depth, expected)
		}
	}
}
