package shell

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/debrief-dev/debrief/config"
	"github.com/debrief-dev/debrief/model"
	"github.com/debrief-dev/debrief/syntax"
)

// FishShellParser implements ShellParser for Fish shell
type FishShellParser struct{}

// Type returns the shell identifier for this source.
func (fs *FishShellParser) Type() model.Shell {
	return model.Fish
}

// HistoryPaths returns all possible history file locations for this source
func (fs *FishShellParser) HistoryPaths() []string {
	return []string{
		config.ExpandPath("~/.local/share/fish/fish_history"),
	}
}

// Detect checks if this source is available on the current system
func (fs *FishShellParser) Detect() *ShellMetadata {
	return detectFromPaths(fs.HistoryPaths(), model.Fish, fs)
}

// ParseHistoryFile reads and parses the history file at the given path
func (fs *FishShellParser) ParseHistoryFile(path string) ([]*model.CommandEntry, error) {
	f, err := os.Open(filepath.Clean(path))
	if err != nil {
		return nil, fmt.Errorf("failed to read Fish history: %w", err)
	}

	defer func() { _ = f.Close() }()

	// Fish history format (YAML-like):
	// - cmd: command text
	//   when: timestamp
	//   paths: [...]
	dedup := NewCommandDeduplicator(model.Fish)
	scanner := bufio.NewScanner(f)
	lineNum := 0

	var (
		currentCmd string
		cmdLineNum int
	)

	for scanner.Scan() {
		lineNum++
		trimmed := strings.TrimSpace(scanner.Text())

		if after, ok := strings.CutPrefix(trimmed, "- cmd:"); ok {
			currentCmd = strings.TrimSpace(after)
			cmdLineNum = lineNum
		} else if strings.HasPrefix(trimmed, "when:") && currentCmd != "" {
			normalizedCmd := fs.NormalizeCommand(currentCmd)
			if normalizedCmd != "" {
				dedup.Add(normalizedCmd, cmdLineNum)
			}

			currentCmd = ""
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("failed to read Fish history: %w", err)
	}

	// Handle last command if file doesn't end with "when:"
	if currentCmd != "" {
		normalizedCmd := fs.NormalizeCommand(currentCmd)
		if normalizedCmd != "" {
			dedup.Add(normalizedCmd, cmdLineNum)
		}
	}

	return dedup.Results(), nil
}

// NormalizeCommand applies source-specific command normalization
func (fs *FishShellParser) NormalizeCommand(cmd string) string {
	cmd = strings.TrimSpace(cmd)
	return syntax.NormalizeWhitespace(cmd)
}
