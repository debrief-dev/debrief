package shell

import (
	"os"

	"github.com/debrief-dev/debrief/model"
)

// ShellMetadata contains information about a shell history file path, shell parser and shell last change
type ShellMetadata struct {
	Type   model.Shell // Which shell this source belongs to
	Path   string      // Absolute path to history file
	Parser ShellParser // The parser that detected and handles this source
	// FileSize is the file size captured at detection time and updated by
	// SourceManager after each successful reload, for use in change detection.
	FileSize int64
}

// ShellParser defines the interface all history sources must implement
type ShellParser interface {
	// Type returns which shell this source handles
	Type() model.Shell

	// Detect checks if this source is available on the current system
	// Returns metadata if available, nil if not found
	Detect() *ShellMetadata

	// HistoryPaths returns all possible history file locations for this source
	// Used for auto-detection (checks each path until one exists)
	HistoryPaths() []string

	// ParseHistoryFile reads and parses the history file at the given path
	// Returns a list of CommandEntries with source information populated
	ParseHistoryFile(path string) ([]*model.CommandEntry, error)

	// NormalizeCommand applies source-specific command normalization
	// (e.g., PowerShell backtick continuation, Bash backslash continuation)
	NormalizeCommand(cmd string) string
}

// detectFromPaths iterates candidate paths, returning metadata for the first
// existing file. Used by Fish, Zsh, and GitBash Detect() implementations.
func detectFromPaths(paths []string, shellType model.Shell, parser ShellParser) *ShellMetadata {
	for _, path := range paths {
		info, err := os.Stat(path)
		if err != nil || info.IsDir() {
			continue
		}

		return &ShellMetadata{
			Type:     shellType,
			Path:     path,
			Parser:   parser,
			FileSize: info.Size(),
		}
	}

	return nil
}

// parseBashHistoryAs delegates to BashSource.ParseHistoryFile and re-tags
// every returned entry with the given shell type. Used by GitBash and WSLBash.
func parseBashHistoryAs(bs *BashShellParser, path string, shell model.Shell) ([]*model.CommandEntry, error) {
	commands, err := bs.ParseHistoryFile(path)
	if err != nil {
		return nil, err
	}

	for _, cmd := range commands {
		cmd.Shell = shell
	}

	return commands, nil
}
