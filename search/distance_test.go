package search

import "testing"

func TestLevenshteinDistance(t *testing.T) {
	tests := []struct {
		s1   string
		s2   string
		want int
	}{
		{"", "", 0},
		{"a", "a", 0},
		{"a", "b", 1},
		{"ab", "ac", 1},
		{"abc", "def", 3},
		{"kitten", "sitting", 3},
		{"golang", "golafg", 1},
	}

	for _, tt := range tests {
		got := LevenshteinDistance(tt.s1, tt.s2)
		if got != tt.want {
			t.Errorf("LevenshteinDistance(%q, %q) = %d, want %d", tt.s1, tt.s2, got, tt.want)
		}
	}
}

func TestExtractTrigrams(t *testing.T) {
	tests := []struct {
		input string
		want  int // expected number of unique trigrams
	}{
		{"", 0},
		{"a", 0},
		{"ab", 0},
		{"abc", 1},    // "abc"
		{"abcd", 2},   // "abc", "bcd"
		{"golang", 4}, // "gol", "ola", "lan", "ang"
	}

	for _, tt := range tests {
		got := ExtractTrigrams(tt.input)
		if len(got) != tt.want {
			t.Errorf("ExtractTrigrams(%q) returned %d trigrams, want %d", tt.input, len(got), tt.want)
		}
	}
}

func TestTrigramSimilarity(t *testing.T) {
	tests := []struct {
		s1       string
		s2       string
		minScore float64 // minimum acceptable similarity
	}{
		{"abc", "abc", 0.99},       // identical
		{"golang", "golang", 0.99}, // identical
		{"golang", "golafg", 0.3},  // some similarity
		{"abc", "xyz", 0.0},        // no similarity
	}

	for _, tt := range tests {
		got := TrigramSimilarity(tt.s1, tt.s2)
		if got < tt.minScore {
			t.Errorf("TrigramSimilarity(%q, %q) = %.3f, want >= %.3f", tt.s1, tt.s2, got, tt.minScore)
		}
	}
}
