package ui

import (
	"testing"

	"github.com/debrief-dev/debrief/data/model"
)

func TestFindBestMatchingNode(t *testing.T) {
	// Create test tree nodes
	nodes := []*model.TreeDisplayNode{
		{Path: "git"},
		{Path: "git commit"},
		{Path: "git commit -m"},
		{Path: "docker"},
		{Path: "docker ps"},
	}

	// Build path index
	pathIndex := make(map[string]int)
	for i, node := range nodes {
		pathIndex[node.Path] = i
	}

	tests := []struct {
		name          string
		cmd           string
		expectedIndex int
		expectedPath  string
	}{
		{
			name:          "exact match",
			cmd:           "git commit",
			expectedIndex: 1,
			expectedPath:  "git commit",
		},
		{
			name:          "prefix match - longest prefix wins",
			cmd:           "git commit -m \"fix bug\"",
			expectedIndex: 2,
			expectedPath:  "git commit -m",
		},
		{
			name:          "prefix match - shorter prefix",
			cmd:           "git status",
			expectedIndex: 0,
			expectedPath:  "git",
		},
		{
			name:          "no match",
			cmd:           "npm install",
			expectedIndex: -1,
			expectedPath:  "",
		},
		{
			name:          "exact match for leaf node",
			cmd:           "docker ps",
			expectedIndex: 4,
			expectedPath:  "docker ps",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			index, path := findBestMatchingNode(nodes, pathIndex, tt.cmd)
			if index != tt.expectedIndex {
				t.Errorf("findBestMatchingNode() index = %v, want %v", index, tt.expectedIndex)
			}

			if path != tt.expectedPath {
				t.Errorf("findBestMatchingNode() path = %v, want %v", path, tt.expectedPath)
			}
		})
	}
}

func TestFindBestMatchingNode_WithoutIndex(t *testing.T) {
	// Test that function works without index map (falls back to linear scan)
	nodes := []*model.TreeDisplayNode{
		{Path: "git"},
		{Path: "git commit"},
		{Path: "git commit -m"},
	}

	// Test with nil index - should still find prefix matches
	index, path := findBestMatchingNode(nodes, nil, "git commit -m \"message\"")
	if index != 2 {
		t.Errorf("Expected index 2, got %d", index)
	}

	if path != "git commit -m" {
		t.Errorf("Expected path 'git commit -m', got '%s'", path)
	}
}

func TestFindBestMatchingNode_EmptyNodes(t *testing.T) {
	// Test with empty node list
	nodes := []*model.TreeDisplayNode{}
	pathIndex := make(map[string]int)

	index, path := findBestMatchingNode(nodes, pathIndex, "any command")
	if index != -1 {
		t.Errorf("Expected index -1 for empty nodes, got %d", index)
	}

	if path != "" {
		t.Errorf("Expected empty path for empty nodes, got '%s'", path)
	}
}

func TestFindBestMatchingNode_EmptyPaths(t *testing.T) {
	// Test that empty paths are skipped
	nodes := []*model.TreeDisplayNode{
		{Path: ""},
		{Path: "git"},
		{Path: ""},
	}

	pathIndex := map[string]int{
		"git": 1,
	}

	index, path := findBestMatchingNode(nodes, pathIndex, "git status")
	if index != 1 {
		t.Errorf("Expected index 1, got %d", index)
	}

	if path != "git" {
		t.Errorf("Expected path 'git', got '%s'", path)
	}
}

// Benchmark to demonstrate O(1) vs O(n) performance
func BenchmarkFindBestMatchingNode_WithIndex(b *testing.B) {
	// Create large node list
	nodes := make([]*model.TreeDisplayNode, 1000)
	pathIndex := make(map[string]int)

	for i := 0; i < 1000; i++ {
		path := "command" + string(rune(i))
		nodes[i] = &model.TreeDisplayNode{
			Path: path,
			Node: &model.PrefixTreeNode{},
		}
		pathIndex[path] = i
	}

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		findBestMatchingNode(nodes, pathIndex, "command500")
	}
}

func BenchmarkFindBestMatchingNode_WithoutIndex(b *testing.B) {
	// Same test without index - should be slower
	nodes := make([]*model.TreeDisplayNode, 1000)
	for i := 0; i < 1000; i++ {
		nodes[i] = &model.TreeDisplayNode{
			Path: "command" + string(rune(i)),
			Node: &model.PrefixTreeNode{},
		}
	}

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		findBestMatchingNode(nodes, nil, "command500 extra args")
	}
}
