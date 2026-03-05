package search

import (
	"strings"

	"github.com/debrief-dev/debrief/data/model"
)

// Index provides multiple indexing strategies for fast fuzzy search
type Index struct {
	// Trigram index: maps 3-character sequences to commands containing them
	TrigramIndex map[[3]byte][]*model.CommandEntry

	// Word-based inverted index: maps individual words to commands
	WordIndex map[string][]*model.CommandEntry
}

// Result represents a single search match with scoring
type Result struct {
	Entry *model.CommandEntry
	Score float64 // Relevance score (0.0 - 1.0)
}

// BuildIndex constructs search indices from a list of commands
func BuildIndex(commands []*model.CommandEntry) *Index {
	index := &Index{
		TrigramIndex: make(map[[3]byte][]*model.CommandEntry),
		WordIndex:    make(map[string][]*model.CommandEntry),
	}

	for _, cmd := range commands {
		// Build trigram index
		trigrams := ExtractTrigrams(cmd.Command)
		for trigram := range trigrams {
			index.TrigramIndex[trigram] = append(index.TrigramIndex[trigram], cmd)
		}

		// Build word index
		words := strings.Fields(strings.ToLower(cmd.Command))
		for _, word := range words {
			index.WordIndex[word] = append(index.WordIndex[word], cmd)
			// Also index delimiter-split parts so "golang" matches commands containing "golang.org"
			for _, part := range splitWordParts(word) {
				if part != word {
					index.WordIndex[part] = append(index.WordIndex[part], cmd)
				}
			}
		}
	}

	return index
}

// GetCandidates returns candidate commands that might match the query
// Uses trigram pre-filtering for fast candidate selection
func (idx *Index) GetCandidates(query string) []*model.CommandEntry {
	if idx == nil {
		return nil
	}

	queryLower := strings.ToLower(query)

	// Extract trigrams from query
	trigrams := ExtractTrigrams(queryLower)

	// Also try exact word matches first
	words := strings.Fields(queryLower)

	// Use a map to deduplicate candidates
	candidateMap := make(map[uint64]*model.CommandEntry)

	// Add candidates from word index (exact word matches)
	for _, word := range words {
		if commands, exists := idx.WordIndex[word]; exists {
			for _, cmd := range commands {
				candidateMap[cmd.Hash] = cmd
			}
		}
	}

	// For short queries (< ShortQueryLengthThreshold chars), use prefix matching
	// on all words in the index. This is an O(n) linear scan over all WordIndex
	// keys, acceptable because the index size is bounded by the user's shell
	// history vocabulary. Handles cases like "go" matching "golangci-lint".
	if len(queryLower) < ShortQueryLengthThreshold && len(words) == 1 {
		queryPrefix := words[0]
		for word, commands := range idx.WordIndex {
			if strings.HasPrefix(word, queryPrefix) {
				for _, cmd := range commands {
					candidateMap[cmd.Hash] = cmd
				}
			}
		}
	}

	// Add candidates from trigram index
	// Commands that share trigrams with the query are likely matches
	for trigram := range trigrams {
		if commands, exists := idx.TrigramIndex[trigram]; exists {
			for _, cmd := range commands {
				candidateMap[cmd.Hash] = cmd
			}
		}
	}

	// No candidates found via indices (e.g. single-character query with no trigrams).
	// Return nil so the caller can fall back to scoring all commands.
	if len(candidateMap) == 0 {
		return nil
	}

	// Convert map to slice
	candidates := make([]*model.CommandEntry, 0, len(candidateMap))
	for _, cmd := range candidateMap {
		candidates = append(candidates, cmd)
	}

	return candidates
}
