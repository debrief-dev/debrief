package model

import "github.com/cespare/xxhash/v2"

// CommandEntry represents a single unique command with all metadata
type CommandEntry struct {
	Command     string // The actual command text
	Frequency   int    // Number of times this exact command was executed
	LineNumbers []int  // All line numbers where this command appears
	Hash        uint64 // Fast hash for deduplication
	Order       int    // Insertion order in OrderedList (higher = more recent)
	Shell       Shell  // Which shell/terminal this came from
}

// NewCommandEntry creates a new CommandEntry for a command
func NewCommandEntry(command string, lineNum int) *CommandEntry {
	return &CommandEntry{
		Command:     command,
		Frequency:   1,
		LineNumbers: []int{lineNum},
		Hash:        xxhash.Sum64String(command),
	}
}

// Update increments the frequency and updates metadata for an existing command
func (ce *CommandEntry) Update(lineNum int) {
	ce.Frequency++
	ce.LineNumbers = append(ce.LineNumbers, lineNum)
}
