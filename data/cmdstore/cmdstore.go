package cmdstore

import (
	"sort"
	"sync"

	"github.com/debrief-dev/debrief/data/model"
	"github.com/debrief-dev/debrief/data/search"
	"github.com/debrief-dev/debrief/data/tree"
)

// CmdStore is the main container for indexed shell history.
// It holds all loaded commands together with their search and tree indices.
type CmdStore struct {
	OrderedList    []*model.CommandEntry // Commands in order of first appearance
	PrefixTree     *model.PrefixTreeNode // For hierarchical clustering
	FuzzyIndex     *search.Index         // For fuzzy searching
	TotalCommands  int                   // [only for testing&debugging] Total commands parsed (including duplicates)
	UniqueCommands int                   // [only for testing&debugging] Number of unique commands
	mu             sync.RWMutex          // Thread-safe access
}

// New creates an empty CmdStore
func New() *CmdStore {
	return &CmdStore{}
}

// Load populates the store from pre-parsed commands and builds indices.
func (s *CmdStore) Load(commands []*model.CommandEntry) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.OrderedList = make([]*model.CommandEntry, 0, len(commands))
	s.TotalCommands = 0
	s.UniqueCommands = 0

	for _, cmd := range commands {
		cmd.Order = len(s.OrderedList)
		s.OrderedList = append(s.OrderedList, cmd)
		s.UniqueCommands++
		s.TotalCommands += cmd.Frequency
	}

	// Build indices
	s.PrefixTree = tree.Build(s.OrderedList)
	tree.PreSortChildren(s.PrefixTree)
	s.FuzzyIndex = search.BuildIndex(s.OrderedList)
}

// Search performs fuzzy search and returns ranked results
func (s *CmdStore) Search(query string) []search.Result {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.FuzzyIndex == nil {
		return nil
	}

	// Get candidates via trigram pre-filtering.
	// GetCandidates returns nil when the query produces no index hits
	// (e.g. a single-character query). Fall back to all commands so the
	// scoring stage still has a chance to find matches.
	candidates := s.FuzzyIndex.GetCandidates(query)
	if candidates == nil {
		candidates = s.OrderedList
	}

	// Score and rank
	results := make([]search.Result, 0, len(candidates))
	for _, cmd := range candidates {
		score := search.Score(query, cmd)
		if score > 0 {
			results = append(results, search.Result{
				Entry: cmd,
				Score: score,
			})
		}
	}

	// Sort by score descending
	sort.Slice(results, func(i, j int) bool {
		scoreDiff := results[i].Score - results[j].Score
		if scoreDiff > search.ScoreEpsilon {
			return true
		}

		if scoreDiff < -search.ScoreEpsilon {
			return false
		}
		// Scores within epsilon: prefer more recent (higher Order = more recent)
		return results[i].Entry.Order > results[j].Entry.Order
	})

	return results
}

// Tree returns the hierarchical command tree.
// The returned pointer is the store's internal tree — callers must not mutate it.
func (s *CmdStore) Tree() *model.PrefixTreeNode {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.PrefixTree
}

// AllCommands returns all commands in chronological order.
// The returned slice is the store's internal list — callers must not mutate it.
func (s *CmdStore) AllCommands() []*model.CommandEntry {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.OrderedList
}

// TreeNodesCount returns the total number of nodes in the prefix tree.
func (s *CmdStore) TreeNodesCount() int {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return countTreeNodes(s.PrefixTree)
}

func countTreeNodes(node *model.PrefixTreeNode) int {
	if node == nil {
		return 0
	}

	count := 1
	for _, child := range node.SortedChildren {
		count += countTreeNodes(child)
	}

	return count
}
