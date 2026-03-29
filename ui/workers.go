package ui

import (
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/fsnotify/fsnotify"

	appstate "github.com/debrief-dev/debrief/app"
	"github.com/debrief-dev/debrief/data/cmdstore"
	"github.com/debrief-dev/debrief/data/model"
	"github.com/debrief-dev/debrief/data/tree"
	"github.com/debrief-dev/debrief/infra/config"
)

// parserDebounceDelay is the debounce delay for file change events in the background parser
const parserDebounceDelay = 100 * time.Millisecond

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
	state.Commands.MetadataCache = make(map[*model.CommandEntry]string)

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

	// If no search active, show loaded commands (respecting shell filter if active).
	// If a search IS active, signal the background worker to re-run it against
	// the new data rather than running it synchronously under Lock (which would
	// block the UI thread for the entire search duration).
	if state.CurrentQuery == "" {
		// Apply shell filter
		applyShellFilterLocked(state)
	} else {
		// Signal the search worker to re-run against new data rather than
		// running synchronously under Lock (which blocks the UI thread).
		requestSearch(state)
	}

	// ScrollToEnd on the List handles pinning to bottom automatically.
	// Do NOT write to List.Position here — it is UI-thread-only state.
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

// StartSearchWorker starts a dedicated background goroutine for fuzzy search.
// Runs independently of the tree rebuild worker so search is never blocked by
// a slow tree rebuild in progress.
func StartSearchWorker(state *appstate.State) {
	go func() {
		for {
			select {
			case <-state.SearchChan:
				// Background search path: used by initializeCommandsLocked (data reloads).
				// User-initiated searches run synchronously on the UI thread for 0-frame latency.
				performSearch(state)
				performAsyncTreeRebuild(state)
			case <-state.SearchShutdown:
				log.Println("Search worker shutting down")
				return
			}
		}
	}()

	log.Println("Search worker started")
}

// requestSearch signals the search worker to run a search.
func requestSearch(app *appstate.State) {
	select {
	case app.SearchChan <- struct{}{}:
	default:
		// Already has pending request
	}
}

// StartTreeRebuildWorker starts a background goroutine to rebuild the tree asynchronously
func StartTreeRebuildWorker(state *appstate.State) {
	go func() {
		for {
			select {
			case <-state.Tree.RebuildChan:
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

// treeRebuildResult holds the computed output of a tree rebuild.
// Produced by computeTreeRebuild (lock-free) and consumed by performAsyncTreeRebuild.
type treeRebuildResult struct {
	nodes     []*model.TreeDisplayNode
	pathIndex map[string]int
	bestMatch int
}

// computeTreeRebuild computes a new tree display from immutable inputs.
// This is a pure function — it holds no locks and touches no shared state.
//
// Uses substring matching (not fuzzy search) for tree filtering so the tree
// only shows commands that literally contain the query. Fuzzy matches produce
// too much clutter in the hierarchical tree view.
func computeTreeRebuild(
	store *cmdstore.CmdStore,
	currentQuery string,
	shellFilter map[model.Shell]bool,
	cachedMatchingCommands map[*model.CommandEntry]bool,
) treeRebuildResult {
	// Extract tree data from store (immutable after creation)
	treeRoot := store.Tree()

	// Resolve matching commands for tree filtering using substring matching.
	// Use cached fuzzy results as pre-filter to avoid scanning all commands.
	var matchingCommands map[*model.CommandEntry]bool

	if currentQuery != "" {
		queryLower := strings.ToLower(currentQuery)

		// Use fuzzy results as candidate set when available (much smaller than all commands)
		candidates := store.AllCommands()
		if cachedMatchingCommands != nil {
			candidates = make([]*model.CommandEntry, 0, len(cachedMatchingCommands))
			for cmd := range cachedMatchingCommands {
				candidates = append(candidates, cmd)
			}
		}

		matchingCommands = make(map[*model.CommandEntry]bool, len(candidates))
		for _, cmd := range candidates {
			if strings.Contains(strings.ToLower(cmd.Command), queryLower) {
				matchingCommands[cmd] = true
			}
		}
	}

	// Flatten tree for display
	newTreeNodes := tree.FlattenForDisplay(
		treeRoot,
		store.TreeNodesCount(),
		matchingCommands,
		shellFilter,
	)

	// Build index map for O(1) path lookups
	newTreeNodeIndex := make(map[string]int, len(newTreeNodes))
	for i, node := range newTreeNodes {
		if node.Path != "" {
			newTreeNodeIndex[node.Path] = i
		}
	}

	// Pre-compute best match index for current query.
	// Priority: exact match > query starts with path > path contains query.
	bestMatchIndex := -1

	if currentQuery != "" && len(newTreeNodes) > 0 {
		queryLower := strings.ToLower(currentQuery)
		bestLen := 0
		substringMatch := -1

		for i, node := range newTreeNodes {
			if strings.EqualFold(node.Path, currentQuery) {
				// Exact match — use immediately
				bestMatchIndex = i
				break
			}

			pathLen := len(node.Path)
			// Query starts with path (e.g. query="kubectl get pods", path="kubectl get")
			if pathLen > bestLen && pathLen+1 <= len(currentQuery) &&
				currentQuery[pathLen] == ' ' &&
				strings.EqualFold(currentQuery[:pathLen], node.Path) {
				bestMatchIndex = i
				bestLen = pathLen
			}
			// Path contains query (e.g. query="golangci", path="go install ...golangci-lint...")
			// Take first substring match as fallback
			if substringMatch < 0 && strings.Contains(strings.ToLower(node.Path), queryLower) {
				substringMatch = i
			}
		}

		if bestMatchIndex < 0 {
			bestMatchIndex = substringMatch
		}
	}

	return treeRebuildResult{
		nodes:     newTreeNodes,
		pathIndex: newTreeNodeIndex,
		bestMatch: bestMatchIndex,
	}
}

// performSearch runs the fuzzy search using snapshot-compute-swap.
// The expensive scoring runs without any lock so the UI thread stays responsive.
// Called from both the UI thread (synchronous search) and the background search worker.
func performSearch(state *appstate.State) {
	// Snapshot inputs under brief RLock
	state.StoreMu.RLock()

	store := state.Store
	if store == nil {
		state.StoreMu.RUnlock()
		return
	}

	query := state.CurrentQuery
	shellFilter := state.ShellFilter
	loadedCommands := state.Commands.LoadedCommands
	state.StoreMu.RUnlock()

	// Compute without lock — Store.Search uses its own internal lock
	var (
		matchingMap map[*model.CommandEntry]bool
		display     []*model.CommandEntry
	)

	if query == "" {
		// Empty search: show all loaded commands (respecting shell filter)
		if shellFilter != nil {
			filtered := make([]*model.CommandEntry, 0, len(loadedCommands))
			for _, cmd := range loadedCommands {
				if shellFilter[cmd.Shell] {
					filtered = append(filtered, cmd)
				}
			}

			display = filtered
		} else {
			display = loadedCommands
		}
	} else {
		results := store.Search(query)

		matchingMap = make(map[*model.CommandEntry]bool, len(results))

		display = make([]*model.CommandEntry, 0, len(results))
		for i := len(results) - 1; i >= 0; i-- {
			entry := results[i].Entry

			matchingMap[entry] = true
			if shellFilter == nil || shellFilter[entry.Shell] {
				display = append(display, entry)
			}
		}
	}

	// Brief Lock to swap results
	state.StoreMu.Lock()

	state.SearchMatchingCommands = matchingMap
	state.Commands.DisplayCommands = display
	invalidateHeightCaches(state)

	if len(display) > 0 {
		state.Commands.NeedInitialSel = true
	}

	state.Tree.NeedInitialSel = true
	state.StoreMu.Unlock()
}

// rebuildTree runs the tree rebuild: snapshot inputs, compute without lock, swap results.
// Called from both the UI thread (synchronous search) and background workers.
func rebuildTree(state *appstate.State) {
	// Snapshot inputs under brief RLock
	state.StoreMu.RLock()

	store := state.Store
	if store == nil {
		state.StoreMu.RUnlock()
		return
	}

	currentQuery := state.CurrentQuery
	shellFilter := state.ShellFilter
	cachedMatching := state.SearchMatchingCommands
	state.StoreMu.RUnlock()

	// Compute new tree without any lock held
	result := computeTreeRebuild(store, currentQuery, shellFilter, cachedMatching)

	// Brief Lock to swap results
	state.StoreMu.Lock()
	state.Tree.Nodes = result.nodes
	state.Tree.NodePathIndex = result.pathIndex
	state.Tree.BestMatchIndex = result.bestMatch
	state.Tree.NodesGeneration++
	state.Tree.NeedsRebuild.Store(false)
	invalidateHeightCaches(state)
	state.StoreMu.Unlock()

	log.Printf("[TREE] Rebuild completed: %d nodes", len(result.nodes))
}

// performAsyncTreeRebuild rebuilds the tree from a background worker and notifies the UI.
func performAsyncTreeRebuild(state *appstate.State) {
	rebuildTree(state)
	state.MarkDirty()
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
