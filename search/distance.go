package search

import "strings"

// LevenshteinDistance calculates edit distance (insertions, deletions, substitutions).
// Optimized with two-row DP — O(min(m,n)) space, zero per-row allocations.
// Operates on bytes, not runes — correct for ASCII shell commands but would
// undercount distance for multi-byte UTF-8 characters.
func LevenshteinDistance(s1, s2 string) int {
	if s1 == "" {
		return len(s2)
	}

	if s2 == "" {
		return len(s1)
	}

	// Keep s1 as the shorter string for minimal memory use
	if len(s1) > len(s2) {
		s1, s2 = s2, s1
	}

	previous := make([]int, len(s1)+1)
	current := make([]int, len(s1)+1)

	for i := range previous {
		previous[i] = i
	}

	for j := 1; j <= len(s2); j++ {
		current[0] = j

		for i := 1; i <= len(s1); i++ {
			cost := 1
			if s1[i-1] == s2[j-1] {
				cost = 0
			}

			current[i] = min(
				min(previous[i]+1, current[i-1]+1), // deletion or insertion
				previous[i-1]+cost,                 // substitution
			)
		}

		previous, current = current, previous
	}

	return previous[len(s1)]
}

// TrigramSimilarity calculates Dice coefficient of trigram sets.
// ExtractTrigrams calls strings.ToLower internally. When called from the scoring
// hot path both arguments are already lowered; Go's ToLower short-circuits for
// all-lowercase strings (scan only, no allocation).
func TrigramSimilarity(s1, s2 string) float64 {
	trigrams1 := ExtractTrigrams(s1)
	trigrams2 := ExtractTrigrams(s2)

	if len(trigrams1) == 0 && len(trigrams2) == 0 {
		return 1.0
	}

	if len(trigrams1) == 0 || len(trigrams2) == 0 {
		return 0.0
	}

	// Count intersections
	intersection := 0

	for trigram := range trigrams1 {
		if trigrams2[trigram] {
			intersection++
		}
	}

	// Dice coefficient: 2 * |A ∩ B| / (|A| + |B|)
	return (DiceCoefficientMultiplier * float64(intersection)) / float64(len(trigrams1)+len(trigrams2))
}

// ExtractTrigrams creates a set of 3-character sequences.
// Uses [3]byte array keys to avoid per-trigram string allocations.
// Operates on bytes, not runes — correct for ASCII shell commands but produces
// incorrect trigrams for multi-byte UTF-8 characters.
func ExtractTrigrams(s string) map[[3]byte]bool {
	s = strings.ToLower(s) // Case-insensitive
	trigrams := make(map[[3]byte]bool)

	if len(s) < MinStringLengthForTrigrams {
		return trigrams
	}

	for i := 0; i <= len(s)-3; i++ {
		trigrams[[3]byte{s[i], s[i+1], s[i+2]}] = true
	}

	return trigrams
}
