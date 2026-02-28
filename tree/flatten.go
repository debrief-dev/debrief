package tree

import (
	"fmt"
	"slices"
	"strings"

	"github.com/debrief-dev/debrief/model"
)

// displayNodesInitialCapacity is the initial capacity for tree display nodes
const displayNodesInitialCapacity = 100

// nodeMetrics holds calculated metrics for a tree node
type nodeMetrics struct {
	filteredFrequency  int
	matchingEntryCount int
	mostFrequentCmd    *model.CommandEntry
}

// treeFlattenContext holds the filtering state for tree flattening operations
type treeFlattenContext struct {
	matchingCommands map[*model.CommandEntry]bool
	shellFilter      map[model.Shell]bool
	searchMatchSet   map[*model.PrefixTreeNode]bool
	sourceMatchSet   map[*model.PrefixTreeNode]bool
	valueNodes       []model.TreeDisplayNode // contiguous backing store for all nodes
	pathBuilder      strings.Builder         // reusable builder for path construction
}

// FlattenForDisplay flattens a prefix tree into a display-ready slice of nodes.
// This is a pure function that takes immutable inputs and returns new TreeNodes.
//
// Parameters:
//   - root: the prefix tree root node
//   - treeNodeCount: total number of nodes in the tree (for pre-allocation)
//   - matchingCommands: if non-nil, only nodes with matching commands are included
//   - shellFilter: if non-nil, only nodes with matching shells are included
func FlattenForDisplay(
	root *model.PrefixTreeNode,
	treeNodeCount int,
	matchingCommands map[*model.CommandEntry]bool,
	shellFilter map[model.Shell]bool,
) []*model.TreeDisplayNode {
	if root == nil {
		return nil
	}

	// Pre-compute which nodes have matches to avoid O(n²) recursion during flattening
	var (
		searchMatchSet map[*model.PrefixTreeNode]bool
		sourceMatchSet map[*model.PrefixTreeNode]bool
	)

	if matchingCommands != nil {
		searchMatchSet = make(map[*model.PrefixTreeNode]bool)
		buildSearchMatchSet(root, matchingCommands, searchMatchSet)
	}

	if shellFilter != nil {
		sourceMatchSet = make(map[*model.PrefixTreeNode]bool)
		buildSourceMatchSet(root, shellFilter, sourceMatchSet)
	}

	// Pre-compute capacity based on total nodes in the tree for zero-reallocation flattening
	initialCapacity := treeNodeCount
	if initialCapacity < displayNodesInitialCapacity {
		initialCapacity = displayNodesInitialCapacity
	}

	// Create flattening context with all filtering state and contiguous node storage
	ctx := &treeFlattenContext{
		matchingCommands: matchingCommands,
		shellFilter:      shellFilter,
		searchMatchSet:   searchMatchSet,
		sourceMatchSet:   sourceMatchSet,
		valueNodes:       make([]model.TreeDisplayNode, 0, initialCapacity),
	}

	// Handle root node separately - just process its children
	if root.Word == "" {
		for _, child := range root.SortedChildren {
			ctx.flatten(child, "", 0)
		}
	} else {
		// Tree root is not empty (shouldn't happen in normal operation)
		ctx.flatten(root, "", 0)
	}

	// Build pointer slice from stable contiguous backing array (all appends are complete)
	flattened := make([]*model.TreeDisplayNode, len(ctx.valueNodes))
	for i := range ctx.valueNodes {
		flattened[i] = &ctx.valueNodes[i]
	}

	return flattened
}

// calculateNodeMetrics computes filtered frequency and finds most frequent command
func calculateNodeMetrics(
	node *model.PrefixTreeNode,
	matchingCommands map[*model.CommandEntry]bool,
	shellFilter map[model.Shell]bool,
) nodeMetrics {
	var metrics nodeMetrics

	for _, cmd := range node.Commands {
		matchesSearch := matchingCommands == nil || matchingCommands[cmd]
		matchesSource := shellFilter == nil || shellFilter[cmd.Shell]

		if matchesSearch && matchesSource {
			metrics.filteredFrequency += cmd.Frequency
			metrics.matchingEntryCount++

			if metrics.mostFrequentCmd == nil || cmd.Frequency > metrics.mostFrequentCmd.Frequency {
				metrics.mostFrequentCmd = cmd
			}
		}
	}

	return metrics
}

// buildMatchSet pre-computes which nodes have matching commands (O(n) single pass)
// Returns true if this node or any descendant has matching commands
func buildMatchSet(
	node *model.PrefixTreeNode,
	predicate func(*model.CommandEntry) bool,
	result map[*model.PrefixTreeNode]bool,
) bool {
	if node == nil {
		return false
	}

	hasMatch := slices.ContainsFunc(node.Commands, predicate)

	// Recursively check children
	for _, child := range node.SortedChildren {
		if buildMatchSet(child, predicate, result) {
			hasMatch = true
		}
	}

	if hasMatch {
		result[node] = true
	}

	return hasMatch
}

// buildSearchMatchSet pre-computes which nodes have matching commands (O(n) single pass)
func buildSearchMatchSet(
	node *model.PrefixTreeNode,
	matchingCommands map[*model.CommandEntry]bool,
	result map[*model.PrefixTreeNode]bool,
) bool {
	return buildMatchSet(node, func(cmd *model.CommandEntry) bool {
		return matchingCommands[cmd]
	}, result)
}

// buildSourceMatchSet pre-computes which nodes have commands matching shell filter (O(n) single pass)
func buildSourceMatchSet(
	node *model.PrefixTreeNode,
	shellFilter map[model.Shell]bool,
	result map[*model.PrefixTreeNode]bool,
) bool {
	return buildMatchSet(node, func(cmd *model.CommandEntry) bool {
		return shellFilter[cmd.Shell]
	}, result)
}

// flatten recursively flattens tree nodes into display nodes using the context's filters.
// Nodes are appended as values to ctx.valueNodes for contiguous memory allocation.
// Returns the filtered command entry count for this subtree (used for branch metadata).
func (ctx *treeFlattenContext) flatten(node *model.PrefixTreeNode, path string, depth int) int {
	if node == nil {
		return 0
	}

	// Filter by search matches using pre-computed set
	if ctx.searchMatchSet != nil && !ctx.searchMatchSet[node] {
		return 0
	}

	// Filter by source using pre-computed set
	if ctx.sourceMatchSet != nil && !ctx.sourceMatchSet[node] {
		return 0
	}

	// Build currentPath using shared builder
	ctx.pathBuilder.Reset()
	ctx.pathBuilder.WriteString(path)

	if path != "" {
		ctx.pathBuilder.WriteByte(' ')
	}

	ctx.pathBuilder.WriteString(node.Word)

	currentPath := ctx.pathBuilder.String()

	// Build pathPrefixWithSpace using shared builder
	var pathPrefixWithSpace string

	if path != "" {
		ctx.pathBuilder.Reset()
		ctx.pathBuilder.WriteString(path)
		ctx.pathBuilder.WriteByte(' ')

		pathPrefixWithSpace = ctx.pathBuilder.String()
	}

	metrics := calculateNodeMetrics(node, ctx.matchingCommands, ctx.shellFilter)

	// Record index so we can set metadata after visiting children
	nodeIdx := len(ctx.valueNodes)

	// Append display node value to contiguous backing store (metadata set after children)
	ctx.valueNodes = append(ctx.valueNodes, model.TreeDisplayNode{
		Node:                node,
		Path:                currentPath,
		PathPrefix:          path,
		PathPrefixWithSpace: pathPrefixWithSpace,
		Depth:               depth,
		HasChildren:         len(node.Children) > 0,
		IsLeaf:              len(node.Commands) > 0,
		FilteredFrequency:   metrics.filteredFrequency,
		MostFrequentCmd:     metrics.mostFrequentCmd,
	})

	// Recurse into children and accumulate filtered command count
	subtreeCount := metrics.matchingEntryCount

	for _, child := range node.SortedChildren {
		subtreeCount += ctx.flatten(child, currentPath, depth+1)
	}

	// Set metadata now that we know the filtered subtree count
	if len(node.Commands) > 0 && metrics.mostFrequentCmd != nil {
		ctx.valueNodes[nodeIdx].CachedMetadata = BuildCommandMetadata(metrics.filteredFrequency, metrics.mostFrequentCmd.Shell.String())
	} else if len(node.Children) > 0 {
		ctx.valueNodes[nodeIdx].CachedMetadata = buildCommandCountMetadata(subtreeCount)
	}

	return subtreeCount
}

// BuildCommandMetadata builds a pre-formatted command metadata string for display
func BuildCommandMetadata(frequency int, sourceName string) string {
	plural := "times"
	if frequency == 1 {
		plural = "time"
	}

	return fmt.Sprintf("Used %d %s · %s", frequency, plural, sourceName)
}

// buildCommandCountMetadata builds a pre-formatted command count metadata string for display
func buildCommandCountMetadata(count int) string {
	plural := "commands"
	if count == 1 {
		plural = "command"
	}

	return fmt.Sprintf("%d %s", count, plural)
}
