package search

import (
	"testing"

	"github.com/debrief-dev/debrief/data/model"
)

func TestScore(t *testing.T) {
	tests := []struct {
		query      string
		command    string
		shouldPass bool // should pass MinScoreThreshold
	}{
		{"git", "git status", true},                       // prefix match
		{"git", "git", true},                              // exact match
		{"status", "git status", true},                    // word match
		{"xyz", "git status", false},                      // no match
		{"go", "go build", true},                          // prefix match
		{"golang", "go install golang", true},             // word match
		{"git status", "git status", true},                // multi-word exact match
		{"golang", "go install golang.org/x/tools", true}, // word-part match
	}

	for _, tt := range tests {
		entry := &model.CommandEntry{
			Command:   tt.command,
			Frequency: 1,
		}
		score := Score(tt.query, entry)

		if tt.shouldPass && score < MinScoreThreshold {
			t.Errorf("Score(%q, %q) = %.3f, expected >= %.3f", tt.query, tt.command, score, MinScoreThreshold)
		}

		if !tt.shouldPass && score >= MinScoreThreshold {
			t.Errorf("Score(%q, %q) = %.3f, expected < %.3f", tt.query, tt.command, score, MinScoreThreshold)
		}
	}
}

func TestMatchTypeDetection(t *testing.T) {
	tests := []struct {
		query        string
		command      string
		expectedType MatchType
		minScore     float64
	}{
		{"git", "git", ExactMatch, ScoreExactMatch},
		{"git", "git status", PrefixMatch, ScorePrefixMatch},
		{"status", "git status", WordMatch, ScoreWordMatch},
		{"golang", "go install golang.org/x/tools", WordPartMatch, ScoreWordPartMatch},
		{"sta", "git status", SubstringMatch, ScoreSubstringMatch},
		{"golafg", "golangci-lint run", FuzzyMatch, ScoreFuzzyMatch},
	}

	for _, tt := range tests {
		matchType, score, _, _ := determineMatchType(tt.query, tt.command)
		if matchType != tt.expectedType {
			t.Errorf("determineMatchType(%q, %q) type = %v, want %v", tt.query, tt.command, matchType, tt.expectedType)
		}

		if score < tt.minScore {
			t.Errorf("determineMatchType(%q, %q) score = %.3f, want >= %.3f", tt.query, tt.command, score, tt.minScore)
		}
	}
}

func TestSplitWordParts(t *testing.T) {
	tests := []struct {
		word string
		want []string
	}{
		{"golang.org", []string{"golang", "org"}},
		{"golang-migrate", []string{"golang", "migrate"}},
		{"golang_1.21", []string{"golang", "1", "21"}},
		{"simple", []string{"simple"}},
		{"a/b/c", []string{"a", "b", "c"}},
		{"host:port", []string{"host", "port"}},
	}

	for _, tt := range tests {
		got := splitWordParts(tt.word)
		if len(got) != len(tt.want) {
			t.Errorf("splitWordParts(%q) = %v, want %v", tt.word, got, tt.want)
			continue
		}

		for i := range got {
			if got[i] != tt.want[i] {
				t.Errorf("splitWordParts(%q)[%d] = %q, want %q", tt.word, i, got[i], tt.want[i])
			}
		}
	}
}

func TestFrequencyBonus(t *testing.T) {
	lowFreq := &model.CommandEntry{
		Command:   "git status",
		Frequency: 0,
	}
	highFreq := &model.CommandEntry{
		Command:   "git status",
		Frequency: 100,
	}

	lowScore := Score("git", lowFreq)
	highScore := Score("git", highFreq)

	if highScore <= lowScore {
		t.Errorf("high-frequency command (score=%.4f) should score higher than low-frequency (score=%.4f)",
			highScore, lowScore)
	}

	diff := highScore - lowScore
	if diff > FrequencyBonusMax+0.001 {
		t.Errorf("frequency bonus diff=%.4f exceeds FrequencyBonusMax=%.4f", diff, FrequencyBonusMax)
	}
}
