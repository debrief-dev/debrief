package shell

import (
	"os"
	"path/filepath"

	"github.com/debrief-dev/debrief/data/model"
	"github.com/debrief-dev/debrief/infra/platform"
)

// GitBashShellParser implements ShellParser for Git Bash
type GitBashShellParser struct {
	BashShellParser // Embed BashSource for shared logic
}

// Type returns the shell identifier for this source.
func (gbs *GitBashShellParser) Type() model.Shell {
	return model.GitBash
}

// HistoryPaths returns all possible history file locations for this source
func (gbs *GitBashShellParser) HistoryPaths() []string {
	if !platform.IsWindows() {
		return nil
	}

	var paths []string

	if userProfile := os.Getenv("USERPROFILE"); userProfile != "" {
		paths = append(paths, filepath.Join(userProfile, ".bash_history"))
	}

	// Also check the Git Bash home which might be different.
	// Skip if it resolves to the same path as USERPROFILE.
	expanded := platform.ExpandPath("~/.bash_history")
	if len(paths) == 0 || filepath.Clean(expanded) != filepath.Clean(paths[0]) {
		paths = append(paths, expanded)
	}

	return paths
}

// Detect checks if this source is available on the current system
func (gbs *GitBashShellParser) Detect() *ShellMetadata {
	// Only available on Windows
	if !platform.IsWindows() {
		return nil
	}

	return detectFromPaths(gbs.HistoryPaths(), model.GitBash, gbs)
}

// ParseHistoryFile reads and parses the history file at the given path
func (gbs *GitBashShellParser) ParseHistoryFile(path string) ([]*model.CommandEntry, error) {
	return parseBashHistoryAs(&gbs.BashShellParser, path, model.GitBash)
}
