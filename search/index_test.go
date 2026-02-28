package search

import (
	"testing"

	"github.com/debrief-dev/debrief/model"
)

func TestBuildIndex_DelimiterSplitting(t *testing.T) {
	commands := []*model.CommandEntry{
		model.NewCommandEntry("go install golang.org/x/tools", 0),
		model.NewCommandEntry("git status", 0),
	}

	index := BuildIndex(commands)

	// "golang" should be in the word index as a delimiter-split part of "golang.org"
	candidates, ok := index.WordIndex["golang"]
	if !ok || len(candidates) == 0 {
		t.Fatal("expected 'golang' to be in WordIndex as a split part of 'golang.org', but it was not found")
	}

	found := false

	for _, c := range candidates {
		if c.Command == "go install golang.org/x/tools" {
			found = true
			break
		}
	}

	if !found {
		t.Error("WordIndex['golang'] did not contain the command 'go install golang.org/x/tools'")
	}

	// Sanity: "git" should still be present via normal word indexing
	if _, ok := index.WordIndex["git"]; !ok {
		t.Error("WordIndex['git'] should be present")
	}
}

func TestGetCandidates_NilFallback(t *testing.T) {
	commands := []*model.CommandEntry{
		model.NewCommandEntry("git status", 0),
	}

	index := BuildIndex(commands)

	// "x" is a single character — no trigrams, no exact word match, no prefix match
	// (ShortQueryLengthThreshold is 3, so prefix scan fires, but "x" doesn't prefix-match "git" or "status")
	candidates := index.GetCandidates("x")
	if candidates != nil {
		t.Errorf("expected GetCandidates to return nil for a no-hit query, got %d candidates", len(candidates))
	}
}
