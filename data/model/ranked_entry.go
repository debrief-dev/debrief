package model

import (
	"cmp"
	"fmt"
	"slices"
)

// TopItemsLimit is the maximum number of top entries shown in statistics.
const TopItemsLimit = 10

// RankedEntry represents a ranked item (command or prefix) with its count and pre-formatted display parts
type RankedEntry struct {
	Label       string
	Count       int
	CachedIndex string // Pre-formatted index (e.g., " 1. ")
	CachedCount string // Pre-formatted count (e.g., "    5 uses")
}

// SortAndFormat sorts aggregated counts descending, truncates to TopItemsLimit,
// and pre-formats each entry's CachedIndex/CachedCount for rendering.
func SortAndFormat(counts map[string]int) []RankedEntry {
	entries := make([]RankedEntry, 0, len(counts))
	for label, count := range counts {
		entries = append(entries, RankedEntry{Label: label, Count: count})
	}

	slices.SortFunc(entries, func(a, b RankedEntry) int {
		if a.Count != b.Count {
			return cmp.Compare(b.Count, a.Count) // descending
		}

		return cmp.Compare(a.Label, b.Label)
	})

	if len(entries) > TopItemsLimit {
		entries = entries[:TopItemsLimit]
	}

	for i := range entries {
		entries[i].CachedIndex, entries[i].CachedCount = fmt.Sprintf("%2d. ", i+1), fmt.Sprintf(" %5d uses", entries[i].Count)
	}

	return entries
}
