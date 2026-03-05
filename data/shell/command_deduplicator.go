package shell

import (
	"sort"

	"github.com/debrief-dev/debrief/data/model"
)

// CommandDeduplicator merges duplicate commands by text, tracking frequency and line numbers.
// Used by all source parsers to deduplicate within a single history file.
type CommandDeduplicator struct {
	entries map[string]*model.CommandEntry
	shell   model.Shell
}

// NewCommandDeduplicator creates a deduplicator pre-configured with source metadata.
func NewCommandDeduplicator(shell model.Shell) *CommandDeduplicator {
	return &CommandDeduplicator{
		entries: make(map[string]*model.CommandEntry),
		shell:   shell,
	}
}

// Add inserts a new command or increments the frequency of an existing one.
func (d *CommandDeduplicator) Add(cmd string, lineNum int) {
	if entry, exists := d.entries[cmd]; exists {
		entry.Update(lineNum)
	} else {
		entry := model.NewCommandEntry(cmd, lineNum)
		entry.Shell = d.shell
		d.entries[cmd] = entry
	}
}

// Results returns deduplicated commands sorted by last line number (chronological order).
func (d *CommandDeduplicator) Results() []*model.CommandEntry {
	commands := make([]*model.CommandEntry, 0, len(d.entries))
	for _, entry := range d.entries {
		commands = append(commands, entry)
	}

	sort.Slice(commands, func(i, j int) bool {
		li := commands[i].LineNumbers

		lj := commands[j].LineNumbers
		if len(li) == 0 || len(lj) == 0 {
			return false
		}

		return li[len(li)-1] < lj[len(lj)-1]
	})

	return commands
}
