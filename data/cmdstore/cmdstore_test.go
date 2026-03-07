package cmdstore

import (
	"testing"

	"github.com/debrief-dev/debrief/data/model"
)

const testCmdGitStatus = "git status"

func TestNew(t *testing.T) {
	s := New()
	if s == nil {
		t.Fatal("New() returned nil")
	}

	if s.FuzzyIndex != nil {
		t.Error("FuzzyIndex should be nil before loading")
	}
}

func TestLoadFromCommands(t *testing.T) {
	s := New()

	cmd1 := model.NewCommandEntry("git status", 1)
	cmd2 := model.NewCommandEntry("git commit", 2)
	cmd3 := model.NewCommandEntry("go build", 3)

	commands := []*model.CommandEntry{cmd1, cmd2, cmd3}

	s.Load(commands)

	if len(s.OrderedList) != 3 {
		t.Errorf("Expected 3 commands in OrderedList, got %d", len(s.OrderedList))
	}

	if s.UniqueCommands != 3 {
		t.Errorf("Expected UniqueCommands=3, got %d", s.UniqueCommands)
	}

	if s.TotalCommands != 3 {
		t.Errorf("Expected TotalCommands=3, got %d", s.TotalCommands)
	}

	if s.FuzzyIndex == nil {
		t.Error("FuzzyIndex should be built after Load")
	}

	if s.PrefixTree == nil {
		t.Error("PrefixTree should be built after Load")
	}
}

func TestSearch(t *testing.T) {
	s := New()

	commands := []*model.CommandEntry{
		model.NewCommandEntry("git status", 1),
		model.NewCommandEntry("git commit -m 'test'", 2),
		model.NewCommandEntry("go build", 3),
		model.NewCommandEntry("go test", 4),
	}

	s.Load(commands)

	results := s.Search("git")
	if len(results) < 2 {
		t.Errorf("Expected at least 2 results for 'git', got %d", len(results))
	}

	results = s.Search("go")
	if len(results) < 2 {
		t.Errorf("Expected at least 2 results for 'go', got %d", len(results))
	}

	for i := 1; i < len(results); i++ {
		if results[i-1].Score < results[i].Score {
			t.Error("Results not sorted by score (descending)")
		}
	}
}

func TestSearch_ShortQueryFallback(t *testing.T) {
	s := New()

	s.Load([]*model.CommandEntry{
		model.NewCommandEntry("ls -la", 1),
		model.NewCommandEntry("git status", 2),
	})

	// "l" is a single character that produces no trigrams and no exact word match.
	// Without the fallback, Search would return nothing. With it, "ls -la" should score.
	results := s.Search("l")

	found := false

	for _, r := range results {
		if r.Entry.Command == "ls -la" {
			found = true
			break
		}
	}

	if !found {
		t.Error("Search('l') should return 'ls -la' via the short-query fallback, but it did not")
	}
}

func TestLoadTotalCommandsSumsFrequency(t *testing.T) {
	s := New()

	cmd := model.NewCommandEntry("git status", 1)
	cmd.Update(5)  // frequency becomes 2
	cmd.Update(10) // frequency becomes 3

	s.Load([]*model.CommandEntry{cmd})

	if s.TotalCommands != 3 {
		t.Errorf("Expected TotalCommands=3 (sum of frequencies), got %d", s.TotalCommands)
	}

	if s.UniqueCommands != 1 {
		t.Errorf("Expected UniqueCommands=1, got %d", s.UniqueCommands)
	}
}

func TestTree(t *testing.T) {
	s := New()

	s.Load([]*model.CommandEntry{
		model.NewCommandEntry("git status", 1),
		model.NewCommandEntry("git commit", 2),
	})

	tree := s.Tree()
	if tree == nil {
		t.Fatal("Tree() returned nil after Load")
	}
}

func TestAllCommands(t *testing.T) {
	s := New()

	s.Load([]*model.CommandEntry{
		model.NewCommandEntry(testCmdGitStatus, 1),
		model.NewCommandEntry("go build", 2),
	})

	all := s.AllCommands()
	if len(all) != 2 {
		t.Fatalf("Expected 2 commands, got %d", len(all))
	}

	if all[0].Command != testCmdGitStatus {
		t.Errorf("Expected first command 'git status', got %q", all[0].Command)
	}

	if all[1].Command != "go build" {
		t.Errorf("Expected second command 'go build', got %q", all[1].Command)
	}
}

func TestTreeNodesCount(t *testing.T) {
	s := New()

	s.Load([]*model.CommandEntry{
		model.NewCommandEntry("git status", 1),
		model.NewCommandEntry("git commit", 2),
		model.NewCommandEntry("go build", 3),
	})

	count := s.TreeNodesCount()
	if count < 1 {
		t.Errorf("Expected at least 1 tree node, got %d", count)
	}
}

func TestSearchEmptyQuery(t *testing.T) {
	s := New()

	s.Load([]*model.CommandEntry{
		model.NewCommandEntry("git status", 1),
		model.NewCommandEntry("go build", 2),
	})

	// Empty string is a prefix of every command, so all commands match.
	results := s.Search("")
	if len(results) != 2 {
		t.Errorf("Expected 2 results for empty query (prefix matches all), got %d", len(results))
	}
}

func TestSearchNilIndex(t *testing.T) {
	s := New()

	results := s.Search("git")
	if results != nil {
		t.Errorf("Expected nil results on unloaded store, got %d results", len(results))
	}
}

func TestSearchTiebreakByRecency(t *testing.T) {
	s := New()

	// Two commands with identical length and same prefix — they'll score
	// identically for query "git" (both prefix matches, same edit distance
	// ratio, same frequency=1), so the tiebreak should prefer higher Order
	// (more recent = later in the load slice).
	s.Load([]*model.CommandEntry{
		model.NewCommandEntry("git status", 1), // Order 0 (older)
		model.NewCommandEntry("git commit", 2), // Order 1 (more recent)
	})

	results := s.Search("git")
	if len(results) < 2 {
		t.Fatalf("Expected at least 2 results, got %d", len(results))
	}

	// When scores are within epsilon, the more recent command (higher Order) should come first
	if results[0].Entry.Command != "git commit" {
		t.Errorf("Expected 'git commit' (more recent) first in tiebreak, got %q", results[0].Entry.Command)
	}

	if results[1].Entry.Command != "git status" {
		t.Errorf("Expected 'git status' (older) second in tiebreak, got %q", results[1].Entry.Command)
	}
}
