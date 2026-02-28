package search

// Fuzzy search algorithm constants
const (
	// MinPartLengthForTokenization is the minimum length for a query part to be tokenized
	MinPartLengthForTokenization = 2
	// MinWordLengthForTrigramMatching is the minimum word length to use trigram matching
	MinWordLengthForTrigramMatching = 3
	// MaxWordsForWordMatching is the maximum number of words to use word-based matching
	MaxWordsForWordMatching = 2
	// DiceCoefficientMultiplier is the numerator multiplier for Dice coefficient: 2 * |A ∩ B| / (|A| + |B|)
	DiceCoefficientMultiplier = 2.0
	// MinStringLengthForTrigrams is the minimum string length to generate trigrams
	MinStringLengthForTrigrams = 3
)

// Scoring constants - these define the fuzzy matching behavior
const (
	// Base scores for different match types (higher = better match)
	// Note: Exact, prefix, and word matches all score 1.00 because they represent
	// finding exactly what the user searched for:
	//   - Exact: "go" finds "go"
	//   - Prefix: "go" finds "go run ." (user wants commands starting with "go")
	//   - Word: "godox" finds "lint -E godox" (user wants commands containing "godox")
	// The similarity bonus differentiates them (shorter commands = higher similarity).
	ScoreExactMatch     = 1.00 // Query exactly equals command
	ScorePrefixMatch    = 1.00 // Command starts with query (e.g., "go" → "go run .")
	ScoreWordMatch      = 1.00 // Query is a complete word in command (e.g., "godox" in "lint -E godox")
	ScoreWordPartMatch  = 0.70 // Query matches a delimiter-split part (e.g., "golang" in "golang.org/x/...")
	ScoreSubstringMatch = 0.40 // Query appears anywhere in command (partial word match)
	ScoreFuzzyMatch     = 0.20 // Similar via edit distance or trigrams

	// Bonus weights (added to base score)
	SimilarityBonusWeight = 0.15 // Maximum bonus for edit distance similarity (0.0-0.15)
	FrequencyBonusMax     = 0.05 // Maximum bonus for popular commands (0.0-0.05)
	FrequencyBonusDivisor = 10.0 // Log scale divisor for frequency normalization

	// Match acceptance thresholds
	MinScoreThreshold         = 0.30 // Minimum score to accept a match (30%)
	FuzzyEditDistanceRatio    = 0.5  // Max edit distance as fraction of length (0.5 = half)
	FuzzyTrigramSimilarityMin = 0.3  // Minimum trigram similarity for fuzzy match
	ShortQueryLengthThreshold = 3    // Queries shorter than this get prefix matching

	// Sorting
	ScoreEpsilon = 0.02 // Scores within this range are considered equal; tie-break by recency
)
