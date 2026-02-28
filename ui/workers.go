package ui

import (
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/fsnotify/fsnotify"

	appstate "github.com/debrief-dev/debrief/app"
	"github.com/debrief-dev/debrief/cmdstore"
	"github.com/debrief-dev/debrief/config"
	"github.com/debrief-dev/debrief/model"
	"github.com/debrief-dev/debrief/tree"
)

// parserDebounceDelay is the debounce delay for file change events in the background parser
const parserDebounceDelay = 100 * time.Millisecond

// treeRebuildDebounceDelay is the debounce delay for tree rebuild requests
const treeRebuildDebounceDelay = 50 * time.Millisecond

// statsRebuildDebounceDelay is the debounce delay for statistics rebuild requests
const statsRebuildDebounceDelay = 50 * time.Millisecond

// drainAndReset safely resets a timer by draining any pending fire before resetting.
// Must be used instead of bare Reset() to avoid a race where the timer fires
// between Stop() returning false and the caller consuming the channel.
func drainAndReset(t *time.Timer, d time.Duration) {
	if !t.Stop() {
		select {
		case <-t.C:
		default:
		}
	}

	t.Reset(d)
}

// StartBackgroundParser starts a goroutine to watch all enabled history files for changes.
// It shuts down when state.StoreShutdown is closed (per-window lifetime).
func StartBackgroundParser(state *appstate.State) {
	go func() {
		// Parse immediately on startup
		updateHistory(state)

		// Get all enabled sources
		enabledSources := state.SourceManager.Enabled()
		if len(enabledSources) == 0 {
			log.Println("No enabled sources found, falling back to polling")
			startPollingFallback(state)

			return
		}

		// Create file watcher
		watcher, err := fsnotify.NewWatcher()
		if err != nil {
			log.Printf("Failed to create file watcher, falling back to polling: %v", err)
			startPollingFallback(state)

			return
		}

		defer func() {
			if err := watcher.Close(); err != nil {
				log.Printf("Error closing file watcher: %v", err)
			}
		}()

		// Add all enabled source files to watcher
		watchedCount := 0

		for _, source := range enabledSources {
			if err := watcher.Add(source.Path); err != nil {
				log.Printf("Failed to watch %s (%s): %v", source.Type, source.Path, err)
			} else {
				watchedCount++

				log.Printf("Started file watcher for %s: %s", source.Type, source.Path)
			}
		}

		if watchedCount == 0 {
			log.Println("No files could be watched, falling back to polling")
			startPollingFallback(state)

			return
		}

		// Watch for file changes with debounce to avoid redundant rebuilds
		// when a single save produces multiple rapid Write events.
		debounceTimer := time.NewTimer(parserDebounceDelay)
		debounceTimer.Stop() // start idle

		defer debounceTimer.Stop()

		for {
			select {
			case event, ok := <-watcher.Events:
				if !ok {
					return
				}
				// Watch for write and create events (create handles when file is recreated)
				if event.Op&fsnotify.Write == fsnotify.Write || event.Op&fsnotify.Create == fsnotify.Create {
					log.Printf("File change detected: %s (%s)", event.Name, event.Op)

					drainAndReset(debounceTimer, parserDebounceDelay)
				}
			case <-debounceTimer.C:
				updateHistory(state)
			case err, ok := <-watcher.Errors:
				if !ok {
					return
				}

				log.Printf("Watcher error: %v", err)
			case <-state.StoreShutdown:
				log.Println("Background parser shutting down")
				return
			}
		}
	}()
}

// startPollingFallback uses polling as a fallback when file watching is unavailable.
// It stops when state.StoreShutdown is closed (per-window lifetime).
func startPollingFallback(state *appstate.State) {
	log.Println("Starting polling fallback (checking every 5 seconds)")

	ticker := time.NewTicker(config.PollingInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			updateHistory(state)
		case <-state.StoreShutdown:
			log.Println("Polling fallback shutting down")
			return
		}
	}
}

// updateHistory loads histories from all enabled sources
func updateHistory(state *appstate.State) {
	log.Println("Loading histories from all enabled sources...")

	// Load all histories from enabled sources
	commands, err := state.SourceManager.LoadAllHistories()
	if err != nil {
		state.StoreMu.Lock()
		state.LoadError = fmt.Errorf("failed to load histories: %w", err)
		state.StoreMu.Unlock()
		state.MarkDirty()
		log.Printf("Error loading histories: %v", err)

		return
	}

	// Create store and load commands
	s := cmdstore.New()
	s.Load(commands)

	lockStart := time.Now()

	log.Println("[LOCK] updateHistory: requesting StoreMu.Lock()")

	state.StoreMu.Lock()

	log.Printf("[LOCK] updateHistory: acquired StoreMu.Lock() after %v", time.Since(lockStart))

	state.Store = s
	state.LoadError = nil

	// Clear metadata cache when store (and its CommandEntry objects) is replaced
	clear(state.Commands.MetadataCache)

	// Initialize/reload commands
	initializeCommandsLocked(state)

	state.StoreMu.Unlock()

	log.Printf("[LOCK] updateHistory: released StoreMu.Lock(), held for %v", time.Since(lockStart))

	state.MarkDirty()
}

// initializeCommandsLocked loads all commands (must be called with StoreMu locked)
func initializeCommandsLocked(state *appstate.State) {
	if state.Store == nil {
		return
	}

	// Load all commands
	allCommands := state.Store.AllCommands()
	state.Commands.LoadedCommands = allCommands

	// Request tree rebuild when data changes
	RequestTreeRebuild(state)
	requestStatsRebuild(state)

	// If no search active, show loaded commands (respecting shell filter if active)
	if state.CurrentQuery == "" {
		// Apply shell filter
		applyShellFilterLocked(state)
	} else {
		// Re-apply search to new data
		executeSearchLocked(state, state.CurrentQuery)
	}

	// Set scroll position to bottom (newest visible at bottom)
	if len(state.Commands.DisplayCommands) > 0 {
		state.Commands.List.Position.First = len(state.Commands.DisplayCommands) - 1
		state.Commands.List.Position.Offset = 0
	}
}

// RequestTreeRebuild signals the background worker to rebuild the tree
func RequestTreeRebuild(app *appstate.State) {
	// Use atomic store to avoid deadlock (this can be called while holding StoreMu)
	app.Tree.NeedsRebuild.Store(true)

	// Non-blocking send to channel (drop if already queued)
	select {
	case app.Tree.RebuildChan <- struct{}{}:
		// Signal sent
	default:
		// Channel already has pending request, skip
	}
}

// StartTreeRebuildWorker starts a background goroutine to rebuild the tree asynchronously
func StartTreeRebuildWorker(state *appstate.State) {
	go func() {
		for {
			select {
			case <-state.Tree.RebuildChan:
				// Debounce: wait for rapid requests to settle
				debounceTimer := time.NewTimer(treeRebuildDebounceDelay)

			debounceLoop:
				for {
					select {
					case <-debounceTimer.C:
						// Debounce period elapsed, proceed with rebuild
						break debounceLoop
					case <-state.Tree.RebuildChan:
						// New rebuild request, restart debounce
						drainAndReset(debounceTimer, treeRebuildDebounceDelay)
					case <-state.Tree.RebuildShutdown:
						debounceTimer.Stop()
						log.Println("Tree rebuild worker shutting down")

						return
					}
				}

				// Perform async rebuild
				performAsyncTreeRebuild(state)

				// Notify waiters via broadcast: close current channel, create fresh one
				state.Tree.RebuildDoneMu.Lock()
				close(state.Tree.RebuildDone)
				state.Tree.RebuildDone = make(chan struct{})
				state.Tree.RebuildDoneMu.Unlock()

			case <-state.Tree.RebuildShutdown:
				log.Println("Tree rebuild worker shutting down")
				return
			}
		}
	}()

	log.Println("Tree rebuild worker started")
}

// performAsyncTreeRebuild rebuilds the tree without blocking the UI
func performAsyncTreeRebuild(state *appstate.State) {
	// Step 1: Copy pointers and state with minimal RLock
	state.StoreMu.RLock()
	store := state.Store
	currentQuery := state.CurrentQuery
	shellFilter := state.ShellFilter
	cachedMatchingCommands := state.SearchMatchingCommands
	state.StoreMu.RUnlock()

	if store == nil {
		return
	}

	// Step 1.5: Extract tree data from store (immutable after creation)
	treeRoot := store.Tree()

	// Step 1.6: Resolve matching commands for search filtering
	var matchingCommands map[*model.CommandEntry]bool

	if currentQuery != "" {
		if cachedMatchingCommands != nil {
			matchingCommands = cachedMatchingCommands
		} else {
			searchResults := store.Search(currentQuery)

			matchingCommands = make(map[*model.CommandEntry]bool, len(searchResults))
			for _, result := range searchResults {
				matchingCommands[result.Entry] = true
			}
		}
	}

	// Step 2: Perform rebuild WITHOUT locks (immutable data)
	newTreeNodes := tree.FlattenForDisplay(
		treeRoot,
		store.TreeNodesCount(),
		matchingCommands,
		shellFilter,
	)

	// Step 3.5: Build index map for O(1) path lookups (outside lock, immutable data)
	newTreeNodeIndex := make(map[string]int, len(newTreeNodes))
	for i, node := range newTreeNodes {
		if node.Path != "" {
			newTreeNodeIndex[node.Path] = i
		}
	}

	// Step 4: Atomically swap the new tree nodes with brief Lock
	state.StoreMu.Lock()
	state.Tree.Nodes = newTreeNodes
	state.Tree.NodePathIndex = newTreeNodeIndex // Swap index atomically with nodes
	state.Tree.NodesGeneration++                // Increment to invalidate stale references
	state.Tree.NeedsRebuild.Store(false)        // Use atomic store
	invalidateHeightCaches(state)               // Clear cached heights
	state.StoreMu.Unlock()

	// Step 5: Trigger UI redraw (MarkDirty survives Gio's invalidation coalescing)
	state.MarkDirty()

	log.Printf("[TREE] Async rebuild completed: %d nodes", len(newTreeNodes))
}

// requestStatsRebuild signals the background worker to rebuild statistics
func requestStatsRebuild(app *appstate.State) {
	app.Stats.NeedsRebuild.Store(true)

	// Non-blocking send to channel (drop if already queued)
	select {
	case app.Stats.RebuildChan <- struct{}{}:
		// Signal sent
	default:
		// Channel already has pending request, skip
	}
}

// StartStatsRebuildWorker starts a background goroutine to rebuild statistics asynchronously
func StartStatsRebuildWorker(state *appstate.State) {
	go func() {
		for {
			select {
			case <-state.Stats.RebuildChan:
				// Debounce rapid requests
				debounceTimer := time.NewTimer(statsRebuildDebounceDelay)

			debounceLoop:
				for {
					select {
					case <-debounceTimer.C:
						break debounceLoop
					case <-state.Stats.RebuildChan:
						drainAndReset(debounceTimer, statsRebuildDebounceDelay)
					case <-state.Stats.RebuildShutdown:
						debounceTimer.Stop()
						log.Println("[STATS] Rebuild worker shutting down")

						return
					}
				}

				performAsyncStatsRebuild(state)

			case <-state.Stats.RebuildShutdown:
				log.Println("[STATS] Rebuild worker shutting down")
				return
			}
		}
	}()

	log.Println("[STATS] Rebuild worker started")
}

func performAsyncStatsRebuild(state *appstate.State) {
	// Step 1: Copy state with minimal RLock
	state.StoreMu.RLock()
	store := state.Store
	shellFilter := state.ShellFilter
	state.StoreMu.RUnlock()

	if store == nil {
		log.Println("[STATS] Store is nil, skipping rebuild")
		return
	}

	// Step 2: Get all commands (immutable data)
	allCommands := store.AllCommands()

	// Step 3: Apply source filter only (statistics always show overall data, ignoring search query)
	var filteredCommands []*model.CommandEntry

	if shellFilter != nil {
		filtered := make([]*model.CommandEntry, 0, len(allCommands))
		for _, cmd := range allCommands {
			if shellFilter[cmd.Shell] {
				filtered = append(filtered, cmd)
			}
		}

		filteredCommands = filtered
	} else {
		filteredCommands = allCommands
	}

	// Step 4: Aggregate commands by full text (lockless)
	commandAggregates := make(map[string]int)
	for _, cmd := range filteredCommands {
		commandAggregates[cmd.Command] += cmd.Frequency
	}

	topCommands := model.SortAndFormat(commandAggregates)

	// Step 5: Aggregate prefixes by first word (lockless)
	prefixCounts := make(map[string]int)

	for _, cmd := range filteredCommands {
		prefix, _, _ := strings.Cut(cmd.Command, " ")
		if prefix != "" {
			prefixCounts[prefix] += cmd.Frequency
		}
	}

	prefixList := model.SortAndFormat(prefixCounts)

	// Step 6: Atomically swap cached results
	state.StoreMu.Lock()
	state.Stats.CachedTopCommands = topCommands
	state.Stats.CachedTopPrefixes = prefixList
	state.Stats.NeedsRebuild.Store(false) // Use atomic store
	state.StoreMu.Unlock()

	// Step 7: Trigger UI redraw (MarkDirty survives Gio's invalidation coalescing)
	state.MarkDirty()

	log.Printf("[STATS] Rebuild complete: %d commands, %d prefixes", len(topCommands), len(prefixList))
}
