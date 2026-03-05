package shell

import (
	"fmt"
	"log"
	"os"
	"sort"
	"sync"
	"sync/atomic"

	"github.com/debrief-dev/debrief/data/model"
)

// enabledSnapshot is the immutable slice stored via atomic.Pointer.
// Once published, the slice and its contents MUST NOT be mutated.
type enabledSnapshot = []*ShellMetadata

// defaultParsers is the ordered list of parsers used by DetectAvailableSources.
var defaultParsers = []ShellParser{
	&PowerShellParser{},
	&BashShellParser{},
	&ZshShellParser{},
	&FishShellParser{},
	&GitBashShellParser{},
	&WSLBashShellParser{},
}

// ShellManager orchestrates multiple history sources.
type ShellManager struct {
	// enabled is an atomically-swapped immutable slice (COW).
	// Readers load the pointer (zero-cost, no lock).
	// Writers build a NEW slice and Store() it under mu.Lock.
	enabled atomic.Pointer[enabledSnapshot]

	// mu protects the load-then-store sequence in insertSorted.
	mu sync.Mutex
}

// NewShellManager creates a new SourceManager.
func NewShellManager() *ShellManager {
	sm := &ShellManager{}

	empty := make(enabledSnapshot, 0, len(defaultParsers))
	sm.enabled.Store(&empty)

	return sm
}

// detectAvailableShells scans the system for available history files.
func (sm *ShellManager) detectAvailableShells() []*ShellMetadata {
	// Fan out: one goroutine per parser to parallelise slow detections
	// (PowerShell spawns powershell.exe, WSL reads slow UNC paths).
	// Each goroutine writes to its own index — no mutex needed.
	results := make([]*ShellMetadata, len(defaultParsers))

	var wg sync.WaitGroup
	wg.Add(len(defaultParsers))

	for i, src := range defaultParsers {
		go func() {
			defer wg.Done()

			results[i] = src.Detect()
		}()
	}

	wg.Wait()

	detected := make([]*ShellMetadata, 0, len(defaultParsers))

	for _, metadata := range results {
		if metadata != nil {
			detected = append(detected, metadata)
			log.Printf("Detected source: %s at %s", metadata.Type, metadata.Path)
		}
	}

	return detected
}

// insertSorted inserts metadata into sm.enabled keeping the slice
// sorted by Shell. If an entry for the same type already exists it is replaced.
// Builds a new slice and atomically publishes it (COW).
// Caller must hold sm.mu.
func (sm *ShellManager) insertSorted(metadata *ShellMetadata) {
	current := *sm.enabled.Load()

	// Replace existing entry for the same type.
	for i, m := range current {
		if m.Type == metadata.Type {
			newSlice := make(enabledSnapshot, len(current))
			copy(newSlice, current)
			newSlice[i] = metadata
			sm.enabled.Store(&newSlice)

			return
		}
	}

	// Insert at sorted position.
	i := sort.Search(len(current), func(j int) bool {
		return current[j].Type >= metadata.Type
	})

	newSlice := make(enabledSnapshot, len(current)+1)
	copy(newSlice[:i], current[:i])
	newSlice[i] = metadata
	copy(newSlice[i+1:], current[i:])
	sm.enabled.Store(&newSlice)
}

// LoadAllHistories loads and returns all commands from all enabled sources.
//
// CRITICAL: Returns ALL commands as SEPARATE entries (no merging).
// Sources are loaded in parallel; the final slice is ordered by file
// modification time (oldest-modified source first, newest last) so that
// when the caller deduplicates by command text the most-recent occurrence wins.
func (sm *ShellManager) LoadAllHistories() ([]*model.CommandEntry, error) {
	active := sm.Enabled()

	// Sort by file modification time (oldest first).
	sorted := make([]*ShellMetadata, len(active))
	copy(sorted, active)

	modTimes := make(map[*ShellMetadata]int64, len(sorted))
	for _, meta := range sorted {
		if info, err := os.Stat(meta.Path); err != nil {
			log.Printf("Warning: cannot stat %s: %v", meta.Path, err)
		} else {
			modTimes[meta] = info.ModTime().Unix()
		}
	}

	sort.Slice(sorted, func(i, j int) bool {
		return modTimes[sorted[i]] < modTimes[sorted[j]]
	})

	// Load all sources in parallel.
	commandSets := make([][]*model.CommandEntry, len(sorted))
	errs := make([]error, len(sorted))

	var wg sync.WaitGroup
	wg.Add(len(sorted))

	for i, meta := range sorted {
		go func() {
			defer wg.Done()

			commandSets[i], errs[i] = meta.Parser.ParseHistoryFile(meta.Path)
		}()
	}

	wg.Wait()

	// Collect results.
	allCommands := make([]*model.CommandEntry, 0)

	var failCount int

	for i, meta := range sorted {
		if errs[i] != nil {
			failCount++

			log.Printf("Error parsing %s: %v", meta.Type, errs[i])

			continue
		}

		allCommands = append(allCommands, commandSets[i]...)
		log.Printf("Loaded %d commands from %s", len(commandSets[i]), meta.Type)
	}

	if failCount == len(sorted) {
		return nil, fmt.Errorf("all %d history sources failed to load", failCount)
	}

	log.Printf("Total commands loaded from all sources: %d", len(allCommands))

	return allCommands, nil
}

// Enabled returns the current enabled sources, sorted by Shell.
// The returned slice is shared (COW) and MUST NOT be mutated by callers.
// Zero-allocation atomic load — no locking, no copying.
func (sm *ShellManager) Enabled() []*ShellMetadata {
	return *sm.enabled.Load()
}

// DetectShells automatically enables all detected sources.
func (sm *ShellManager) DetectShells() {
	detected := sm.detectAvailableShells()

	sm.mu.Lock()
	defer sm.mu.Unlock()

	for _, metadata := range detected {
		sm.insertSorted(metadata)
		log.Printf("Enabled: %s", metadata.Type)
	}
}
