package search

import (
	"math"
	"slices"
	"strings"

	"github.com/debrief-dev/debrief/data/model"
)

// MatchType represents how a match was found
type MatchType int

const (
	NoMatch MatchType = iota
	FuzzyMatch
	SubstringMatch
	WordPartMatch
	WordMatch
	PrefixMatch
	ExactMatch
)

// wordPartReplacer replaces common word delimiters with spaces in a single pass
var wordPartReplacer = strings.NewReplacer(".", " ", "/", " ", "-", " ", "_", " ", ":", " ")

// Score calculates a composite fuzzy match score using a transparent scoring system:
//
// 1. Match Type (base score 0.0-1.0):
//   - Exact match:     ScoreExactMatch (query == command)
//   - Prefix match:    ScorePrefixMatch (command starts with query)
//   - Word match:      ScoreWordMatch (query is a complete word in command)
//   - Substring match: ScoreSubstringMatch (query appears anywhere in command)
//   - Fuzzy match:     ScoreFuzzyMatch (similar via edit distance/trigrams)
//
// 2. Similarity Bonus (0.0 to SimilarityBonusWeight):
//   - Based on edit distance relative to length
//   - For multi-word commands, uses best-matching word for similarity
//   - Rewards close matches, penalizes distant ones
//
// 3. Frequency Bonus (0.0 to FrequencyBonusMax):
//   - Popular commands get small boost
//   - Normalized by logarithm to avoid dominance
//
// Final score = base + similarity_bonus + frequency_bonus (max ~1.0)
// Returns 0.0 if the result is below MinScoreThreshold.
func Score(query string, cmd *model.CommandEntry) float64 {
	queryLower := strings.ToLower(query)
	cmdLower := strings.ToLower(cmd.Command)

	// Fast path for short queries (1-2 chars): skip expensive Levenshtein and
	// trigram computations. The similarity bonus is meaningless for very short
	// queries (e.g., "g" vs "git commit -m fix" → similarity ≈ 0). Use only
	// match type + frequency for ranking. This eliminates thousands of slice
	// allocations (2 per Levenshtein call) that dominate the scoring hot path.
	if len(queryLower) < MinStringLengthForTrigrams {
		matchType, baseScore := determineMatchTypeShort(queryLower, cmdLower)
		if matchType == NoMatch {
			return 0.0
		}

		freqBonus := math.Min(math.Log(float64(cmd.Frequency+1))/FrequencyBonusDivisor, FrequencyBonusMax)
		totalScore := baseScore + freqBonus

		if totalScore < MinScoreThreshold {
			return 0.0
		}

		return totalScore
	}

	// Step 1: Determine match type, base score, and pre-split words/parts
	matchType, baseScore, words, wordParts := determineMatchType(queryLower, cmdLower)
	if matchType == NoMatch {
		return 0.0
	}

	// Step 2: Calculate similarity bonus
	// For better fuzzy matching, find the best-matching word instead of comparing
	// against the entire command string
	var similarityBonus float64

	if len(words) == 1 || matchType == ExactMatch || matchType == PrefixMatch {
		// Single word or exact/prefix match: use full command
		maxLen := max(len(queryLower), len(cmdLower))
		editDist := LevenshteinDistance(queryLower, cmdLower)
		similarity := 1.0 - (float64(editDist) / float64(maxLen))
		similarityBonus = similarity * SimilarityBonusWeight
	} else {
		// Multi-word command: find best word match for similarity calculation
		bestSimilarity := 0.0

		for _, parts := range wordParts {
			for _, part := range parts {
				if len(part) < MinPartLengthForTokenization {
					continue
				}

				maxLen := max(len(queryLower), len(part))
				editDist := LevenshteinDistance(queryLower, part)

				similarity := 1.0 - (float64(editDist) / float64(maxLen))
				if similarity > bestSimilarity {
					bestSimilarity = similarity
				}
			}
		}

		similarityBonus = bestSimilarity * SimilarityBonusWeight
	}

	// Step 3: Calculate frequency bonus
	// Log scale to prevent very frequent commands from dominating
	freqBonus := math.Min(math.Log(float64(cmd.Frequency+1))/FrequencyBonusDivisor, FrequencyBonusMax)

	// Final score
	totalScore := baseScore + similarityBonus + freqBonus

	if totalScore < MinScoreThreshold {
		return 0.0
	}

	return totalScore
}

// isWordPartDelimiter reports whether ch is a word-part delimiter.
// Matches the delimiters used by wordPartReplacer: space, '.', '/', '-', '_', ':'.
func isWordPartDelimiter(ch byte) bool {
	switch ch {
	case ' ', '.', '/', '-', '_', ':':
		return true
	}

	return false
}

// determineMatchTypeShort is a fast path for short queries (< 3 chars).
// Avoids allocations from strings.Fields, splitWordParts, and Levenshtein.
// Only checks prefix, word boundary, and substring matches — no fuzzy matching
// since trigrams require 3+ chars and edit distance is meaningless for 1-2 chars.
func determineMatchTypeShort(query, cmd string) (MatchType, float64) {
	if query == cmd {
		return ExactMatch, ScoreExactMatch
	}

	if strings.HasPrefix(cmd, query) {
		return PrefixMatch, ScorePrefixMatch
	}

	// Check word boundaries without allocating a slice: scan for query preceded
	// by start-of-string or delimiter, and followed by end-of-string or delimiter.
	if idx := strings.Index(cmd, query); idx >= 0 {
		// Check if it's a word match (at word boundary)
		atStart := idx == 0 || isWordPartDelimiter(cmd[idx-1])
		end := idx + len(query)
		atEnd := end == len(cmd) || isWordPartDelimiter(cmd[end])

		if atStart && atEnd {
			return WordMatch, ScoreWordMatch
		}

		// It's at least a substring match
		return SubstringMatch, ScoreSubstringMatch
	}

	return NoMatch, 0.0
}

// determineMatchType identifies how the query matches the command.
// Returns the match type, corresponding base score, the whitespace-split words of cmd,
// and pre-computed delimiter-split parts per word (nil for exact/prefix matches).
func determineMatchType(query, cmd string) (matchType MatchType, score float64, words []string, wordParts [][]string) {
	// Exact match: query == command
	if query == cmd {
		return ExactMatch, ScoreExactMatch, strings.Fields(cmd), nil
	}

	// Prefix match: command starts with query
	if strings.HasPrefix(cmd, query) {
		return PrefixMatch, ScorePrefixMatch, strings.Fields(cmd), nil
	}

	words = strings.Fields(cmd)

	// Word match: query is a complete whitespace-separated word in command
	for _, word := range words {
		if word == query {
			return WordMatch, ScoreWordMatch, words, nil
		}
	}

	// Pre-compute delimiter-split parts for each word once;
	// reused by word-part match, fuzzy match, and returned to caller for similarity bonus.
	wordParts = make([][]string, len(words))
	for i, word := range words {
		wordParts[i] = splitWordParts(word)
	}

	// Word-part match: query matches a delimiter-split part (e.g., "golang" in "golang.org")
	for _, parts := range wordParts {
		if slices.Contains(parts, query) {
			return WordPartMatch, ScoreWordPartMatch, words, wordParts
		}
	}

	// Substring match: query appears anywhere in command
	if strings.Contains(cmd, query) {
		return SubstringMatch, ScoreSubstringMatch, words, wordParts
	}

	// Fuzzy match against individual words and word parts in the command
	// This allows "golafg" to match commands containing "golang"
	for _, parts := range wordParts {
		for _, part := range parts {
			// Skip very short parts to avoid false matches
			if len(part) < MinWordLengthForTrigramMatching {
				continue
			}

			// Check edit distance against this part
			maxLen := max(len(query), len(part))
			editDist := LevenshteinDistance(query, part)

			maxEditDist := int(float64(maxLen) * FuzzyEditDistanceRatio)
			if editDist <= maxEditDist {
				return FuzzyMatch, ScoreFuzzyMatch, words, wordParts
			}

			// Check trigram similarity against this part
			trigramSim := TrigramSimilarity(query, part)
			if trigramSim >= FuzzyTrigramSimilarityMin {
				return FuzzyMatch, ScoreFuzzyMatch, words, wordParts
			}
		}
	}

	// Fallback: fuzzy match against entire command (for short commands)
	if len(words) <= MaxWordsForWordMatching {
		maxLen := max(len(query), len(cmd))
		editDist := LevenshteinDistance(query, cmd)

		maxEditDist := int(float64(maxLen) * FuzzyEditDistanceRatio)
		if editDist <= maxEditDist {
			return FuzzyMatch, ScoreFuzzyMatch, words, wordParts
		}

		trigramSim := TrigramSimilarity(query, cmd)
		if trigramSim >= FuzzyTrigramSimilarityMin {
			return FuzzyMatch, ScoreFuzzyMatch, words, wordParts
		}
	}

	return NoMatch, 0.0, words, wordParts
}

// splitWordParts splits a word on common delimiters (dots, slashes, hyphens, underscores, colons)
// This helps match "golang" in "golang.org", "golang-migrate", "golang_1.21", etc.
func splitWordParts(word string) []string {
	return strings.Fields(wordPartReplacer.Replace(word))
}
