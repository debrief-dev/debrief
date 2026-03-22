package ui

import (
	"log"
	"strings"
	"time"

	appstate "github.com/debrief-dev/debrief/app"
	"github.com/debrief-dev/debrief/data/model"
)

const (
	// TreeRebuildInitialTimeout is the max time to wait for initial tree build
	TreeRebuildInitialTimeout = 200 * time.Millisecond
	// TreeRebuildExpansionTimeout is the max time to wait for tree rebuild after expansion
	TreeRebuildExpansionTimeout = 100 * time.Millisecond
)

// waitForTreeRebuild waits for the tree rebuild to complete with a timeout.
// The caller MUST capture rebuildDone via Tree.RebuildDoneMu BEFORE calling
// RequestTreeRebuild to avoid a race where the rebuild completes before we wait.
func waitForTreeRebuild(rebuildDone <-chan struct{}, timeout time.Duration) bool {
	select {
	case <-rebuildDone:
		return true
	case <-time.After(timeout):
		log.Printf("Warning: Tree rebuild timeout after %v", timeout)
		return false
	}
}

// findBestMatchingNode finds the tree node that best matches the given command.
// Returns the index and path of the best match, or (-1, "") if no match found.
// This is a pure function with no locking concerns, making it easily testable.
//
// Matching priority:
// 1. Exact match (via index map if provided)
// 2. Longest prefix match (command starts with "nodePath ")
func findBestMatchingNode(nodes []*model.TreeDisplayNode, pathIndex map[string]int, cmd string) (index int, path string) {
	bestMatch := -1
	bestMatchLen := 0

	// First try O(1) exact match via index
	if pathIndex != nil {
		if idx, exists := pathIndex[cmd]; exists && idx >= 0 && idx < len(nodes) {
			return idx, nodes[idx].Path
		}
	}

	// If no exact match, find the longest prefix match
	for i, node := range nodes {
		nodePath := node.Path
		if nodePath == "" {
			continue
		}

		// Check if command starts with this node's path (with space separator)
		if strings.HasPrefix(cmd, nodePath+" ") {
			pathLen := len(nodePath)
			if pathLen > bestMatchLen {
				bestMatch = i
				bestMatchLen = pathLen
			}
		}
	}

	if bestMatch >= 0 && bestMatch < len(nodes) {
		return bestMatch, nodes[bestMatch].Path
	}

	return -1, ""
}

// syncCommandToTreeSelection synchronizes selection when switching from Commands to Tree tab
func syncCommandToTreeSelection(app *appstate.State) {
	// Try current selection first, then fall back to last known selection
	var (
		selectedCmd  string
		hasSelection bool
	)

	app.StoreMu.RLock()

	switch {
	case app.Commands.SelectedIndex >= 0 && app.Commands.SelectedIndex < len(app.Commands.DisplayCommands):
		selectedCmd = app.Commands.DisplayCommands[app.Commands.SelectedIndex].Command
		hasSelection = true
	case len(app.Commands.DisplayCommands) > 0:
		// Commands tab hasn't selected yet (e.g. search just ran) —
		// pick the best match (last item, sorted worst-to-best)
		selectedCmd = app.Commands.DisplayCommands[len(app.Commands.DisplayCommands)-1].Command
		hasSelection = true

	case app.Commands.LastSelectedIndex >= 0 && app.Commands.LastSelectedCmd != "":
		// Try to restore from last known selection
		selectedCmd = app.Commands.LastSelectedCmd
		hasSelection = true
	}

	app.StoreMu.RUnlock()

	if !hasSelection {
		log.Printf("No command selected to sync")
		return
	}

	// Check if tree is already populated
	app.StoreMu.RLock()
	initialNodeCount := len(app.Tree.Nodes)
	app.StoreMu.RUnlock()

	// Capture broadcast channel BEFORE requesting rebuild to avoid race
	// where rebuild completes before we start waiting.
	app.Tree.RebuildDoneMu.Lock()
	rebuildDone := app.Tree.RebuildDone
	app.Tree.RebuildDoneMu.Unlock()

	// Request tree rebuild
	RequestTreeRebuild(app)

	// Only wait if tree is empty (first time sync or after data reload)
	if initialNodeCount == 0 {
		log.Printf("Tree empty, waiting for initial build...")

		if waitForTreeRebuild(rebuildDone, TreeRebuildInitialTimeout) {
			app.StoreMu.RLock()
			nodeCount := len(app.Tree.Nodes)
			app.StoreMu.RUnlock()
			log.Printf("Tree ready with %d nodes", nodeCount)
		}
	} else {
		log.Printf("Tree already populated with %d nodes, proceeding immediately", initialNodeCount)
	}

	app.StoreMu.RLock()
	log.Printf("Syncing command to tree: %s", selectedCmd)
	log.Printf("Tree has %d nodes", len(app.Tree.Nodes))

	// Find the best matching tree node
	bestMatch, finalPath := findBestMatchingNode(app.Tree.Nodes, app.Tree.NodePathIndex, selectedCmd)
	app.StoreMu.RUnlock()

	if bestMatch >= 0 {
		log.Printf("Found matching node at index %d: '%s'", bestMatch, finalPath)

		// Update selection state with lock (shared with background worker)
		app.StoreMu.Lock()
		// Revalidate index after acquiring write lock - tree may have been rebuilt
		if bestMatch >= 0 && bestMatch < len(app.Tree.Nodes) && app.Tree.Nodes[bestMatch].Path == finalPath {
			app.Tree.SelectedNode = bestMatch
			app.Tree.SelectedNodePath = finalPath
			app.Tree.NeedInitialSel = false // Prevent pending NeedInitialSel from overriding
			// Place target at top of viewport directly — avoids unreliable height
			// estimation that calculateSmartScrollPositionVariable would need for
			// items far from the previously visible range.
			app.Tree.List.Position.First = bestMatch
			app.Tree.List.Position.Offset = 0
			app.NeedScrollToSel = false
			app.StoreMu.Unlock()

			log.Printf("Synced selection to tree node at index %d: %s", bestMatch, finalPath)
		} else {
			app.StoreMu.Unlock()
			log.Printf("Tree changed during sync, match no longer valid for: %s", selectedCmd)
		}
	} else {
		log.Printf("No matching tree node found for command: %s", selectedCmd)
		// Log first few tree nodes for debugging
		app.StoreMu.RLock()

		for i := 0; i < 5 && i < len(app.Tree.Nodes); i++ {
			log.Printf("  Tree node %d: '%s'", i, app.Tree.Nodes[i].Path)
		}

		app.StoreMu.RUnlock()
	}
}

// findCommandMatch searches for a command that matches the given node path
// Returns the index of the best match, or -1 if not found
// matchType: matchExact for exact/prefix matching, matchFuzzy for contains matching
//
// NOTE: Linear scan is acceptable for current data sizes
type matchType int

const (
	matchExact matchType = iota
	matchFuzzy
)

func findCommandMatch(searchList []*model.CommandEntry, nodePath string, mt matchType) int {
	foundIndex := -1
	bestMatchLen := 0

	for i, cmd := range searchList {
		if mt == matchFuzzy {
			// Fuzzy match: command contains node path
			if strings.Contains(cmd.Command, nodePath) {
				foundIndex = i
				break // Take first fuzzy match
			}
		} else {
			// Exact/prefix match
			if cmd.Command == nodePath {
				foundIndex = i
				break // Exact match is best
			}
			// Otherwise, find command with longest matching prefix
			if strings.HasPrefix(cmd.Command, nodePath+" ") {
				matchLen := len(nodePath)
				if matchLen > bestMatchLen {
					foundIndex = i
					bestMatchLen = matchLen
				}
			}
		}
	}

	return foundIndex
}

// resizeHeightCache ensures the height cache slice matches the expected count.
// Reuses existing capacity when possible to avoid allocations.
func resizeHeightCache(cache *[]int, count int) {
	if len(*cache) == count {
		return
	}

	if cap(*cache) >= count {
		*cache = (*cache)[:count]
		clear(*cache)
	} else {
		*cache = make([]int, count)
	}
}

// syncTreeToCommandSelection synchronizes selection when switching from Tree to Commands tab
func syncTreeToCommandSelection(app *appstate.State) {
	// Try current selection first, then fall back to last known selection
	var (
		nodePath     string
		hasSelection bool
	)

	app.StoreMu.RLock()

	if app.Tree.SelectedNode >= 0 && app.Tree.SelectedNode < len(app.Tree.Nodes) {
		nodePath = app.Tree.Nodes[app.Tree.SelectedNode].Path
		hasSelection = true
	} else if app.Tree.LastSelectedNode >= 0 && app.Tree.LastSelectedPath != "" {
		// Try to restore from last known selection
		nodePath = app.Tree.LastSelectedPath
		hasSelection = true
	}

	if !hasSelection || nodePath == "" {
		app.StoreMu.RUnlock()

		return
	}

	// Snapshot DisplayCommands under lock, then release before linear scan.
	// Copy-on-write discipline ensures the snapshot remains valid.
	// Using one snapshot for both exact and fuzzy scans guarantees consistent indices.
	searchList := app.Commands.DisplayCommands
	app.StoreMu.RUnlock()

	// Find the best matching command (prefer exact match or longest prefix)
	foundIndex := findCommandMatch(searchList, nodePath, matchExact)

	if foundIndex >= 0 {
		app.Commands.SelectedIndex = foundIndex
		// Break out of ScrollToEnd pinning and place target at top of viewport
		// directly — avoids unreliable height estimation for items far from
		// the previously visible range.
		app.Commands.List.Position.BeforeEnd = true
		app.Commands.List.Position.First = foundIndex
		app.Commands.List.Position.Offset = 0

		return
	}

	// No exact/prefix match - try fuzzy matching (same snapshot for consistency)
	fuzzyFoundIndex := findCommandMatch(searchList, nodePath, matchFuzzy)

	if fuzzyFoundIndex >= 0 {
		app.Commands.SelectedIndex = fuzzyFoundIndex
		// Break out of ScrollToEnd pinning and place target at top of viewport
		// directly — avoids unreliable height estimation for items far from
		// the previously visible range.
		app.Commands.List.Position.BeforeEnd = true
		app.Commands.List.Position.First = fuzzyFoundIndex
		app.Commands.List.Position.Offset = 0
	} else {
		// No match at all - clear selection explicitly
		app.Commands.SelectedIndex = -1
	}
}

// switchToTab transitions to targetTab, performing selection sync, hover clearing,
// initial-selection requests, focus requests, and window invalidation.
// No-op when app.Tabs.Current == targetTab.
func switchToTab(app *appstate.State, targetTab model.Tab) {
	oldTab := app.Tabs.Current
	if oldTab == targetTab {
		return
	}

	// Sync cross-tab selections before leaving
	if oldTab == model.TabCommands && targetTab == model.TabTree {
		syncCommandToTreeSelection(app)
	} else if oldTab == model.TabTree && targetTab == model.TabCommands {
		syncTreeToCommandSelection(app)
	}

	// Clear hover state when leaving Statistics tab
	if oldTab == model.TabTopCommands {
		app.Stats.HoveredIndex = -1
	}

	app.Tabs.Current = targetTab
	app.RequestSearchFocus = true

	// Request initial selection when arriving at a tab that has none
	switch targetTab {
	case model.TabCommands:
		if app.Commands.SelectedIndex == -1 {
			app.Commands.NeedInitialSel = true
		}
	case model.TabTree:
		if app.Tree.SelectedNode == -1 {
			app.Tree.NeedInitialSel = true
		}
	case model.TabTopCommands:
		if app.Stats.SelectedIndex == -1 {
			app.Stats.NeedInitialSel = true
		}
	}

	app.Window.Invalidate()
}
