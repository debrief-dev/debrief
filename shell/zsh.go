package shell

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/debrief-dev/debrief/config"
	"github.com/debrief-dev/debrief/model"
	"github.com/debrief-dev/debrief/syntax"
)

// String split operation constants
const (
	// ZshMaxSplitParts is the maximum number of parts when splitting strings in Zsh extended history format
	ZshMaxSplitParts = 2
)

// ZshShellParser implements HistorySource for Zsh
type ZshShellParser struct{}

// Type returns the shell identifier for this source.
func (zs *ZshShellParser) Type() model.Shell {
	return model.Zsh
}

// HistoryPaths returns all possible history file locations for this source
func (zs *ZshShellParser) HistoryPaths() []string {
	paths := []string{}

	// Check HISTFILE environment variable first
	if histFile := os.Getenv("HISTFILE"); histFile != "" {
		paths = append(paths, config.ExpandPath(histFile))
	}

	// Standard paths
	paths = append(paths,
		config.ExpandPath("~/.zsh_history"),
		config.ExpandPath("~/.zhistory"),
	)

	return paths
}

// Detect checks if this source is available on the current system
func (zs *ZshShellParser) Detect() *ShellMetadata {
	return detectFromPaths(zs.HistoryPaths(), model.Zsh, zs)
}

// ParseHistoryFile reads and parses the history file at the given path
func (zs *ZshShellParser) ParseHistoryFile(path string) ([]*model.CommandEntry, error) {
	file, err := os.Open(filepath.Clean(path))
	if err != nil {
		return nil, fmt.Errorf("failed to read Zsh history: %w", err)
	}

	defer func() {
		if err := file.Close(); err != nil {
			log.Printf("Warning: failed to close Zsh history file: %v\n", err)
		}
	}()

	dedup := NewCommandDeduplicator(model.Zsh)
	scanner := bufio.NewScanner(file)
	lineNum := 0

	for scanner.Scan() {
		lineNum++
		startLineNum := lineNum
		line := scanner.Text()

		// Zsh extended history format: ": timestamp:duration;command"
		// Also supports simple format (just commands)
		cmd := line
		if strings.HasPrefix(line, ":") {
			// Extended format: parse out the command
			parts := strings.SplitN(line, ";", ZshMaxSplitParts)
			if len(parts) == ZshMaxSplitParts {
				cmd = parts[1]
			}
		}

		// Handle backslash line continuation.
		for strings.HasSuffix(cmd, "\\") && scanner.Scan() {
			lineNum++
			cmd = strings.TrimSuffix(cmd, "\\") + " " + scanner.Text()
		}

		cmd = zs.NormalizeCommand(cmd)
		if cmd == "" {
			continue
		}

		dedup.Add(cmd, startLineNum)
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error scanning Zsh history: %w", err)
	}

	return dedup.Results(), nil
}

// NormalizeCommand applies source-specific command normalization
func (zs *ZshShellParser) NormalizeCommand(cmd string) string {
	cmd = strings.TrimSpace(cmd)
	cmd = strings.TrimSuffix(cmd, "\\")
	cmd = strings.TrimSpace(cmd)

	return syntax.NormalizeWhitespace(cmd)
}
