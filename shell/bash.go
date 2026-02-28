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

// BashShellParser implements ShellParser for Bash
type BashShellParser struct{}

// Type returns the shell identifier for this source.
func (bs *BashShellParser) Type() model.Shell {
	return model.Bash
}

// HistoryPaths returns all possible history file locations for this source
func (bs *BashShellParser) HistoryPaths() []string {
	paths := []string{}

	// Check HISTFILE environment variable first
	if histFile := os.Getenv("HISTFILE"); histFile != "" {
		paths = append(paths, config.ExpandPath(histFile))
	}

	// Standard path
	paths = append(paths, config.ExpandPath("~/.bash_history"))

	return paths
}

// Detect checks if this source is available on the current system
func (bs *BashShellParser) Detect() *ShellMetadata {
	for _, path := range bs.HistoryPaths() {
		info, err := os.Stat(path)
		if err != nil || info.IsDir() {
			continue
		}

		return &ShellMetadata{
			Type:     model.Bash,
			Path:     path,
			Parser:   bs,
			FileSize: info.Size(),
		}
	}

	return nil
}

// ParseHistoryFile reads and parses the history file at the given path
func (bs *BashShellParser) ParseHistoryFile(path string) ([]*model.CommandEntry, error) {
	f, err := os.Open(filepath.Clean(path))
	if err != nil {
		return nil, fmt.Errorf("failed to read Bash history: %w", err)
	}

	defer func() { _ = f.Close() }()

	// Read lines via scanner to avoid loading the whole file as a single string.
	var lines []string

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("failed to read Bash history: %w", err)
	}

	dedup := NewCommandDeduplicator(model.Bash)
	lineNum := 0

	for i := 0; i < len(lines); i++ {
		lineNum++
		startLineNum := lineNum // capture before any multi-line accumulation
		line := strings.TrimSpace(lines[i])

		// Skip empty lines and comments
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// Handle Bash backslash continuation
		fullCommand := line
		for strings.HasSuffix(line, "\\") && i+1 < len(lines) {
			i++
			lineNum++
			nextLine := strings.TrimSpace(lines[i])
			fullCommand = strings.TrimSuffix(fullCommand, "\\") + " " + nextLine
			line = nextLine
		}

		// Handle multiline function definitions.
		// If the accumulated command starts a function but braces aren't balanced yet,
		// keep consuming lines until balanced (or EOF).
		if bs.isFunctionStart(fullCommand) && !syntax.IsBalancedBraces(fullCommand) {
			accumulated := 1
			for i+1 < len(lines) && !syntax.IsBalancedBraces(fullCommand) && accumulated < maxFunctionLines {
				i++
				lineNum++
				accumulated++

				nextLine := strings.TrimSpace(lines[i])
				if nextLine != "" {
					fullCommand = fullCommand + " " + nextLine
				}
			}
		}

		fullCommand = bs.NormalizeCommand(fullCommand)

		if fullCommand == "" {
			continue
		}

		// Use startLineNum so the recorded position is the first line of the logical command,
		// regardless of how many continuation or function-body lines were consumed.
		dedup.Add(fullCommand, startLineNum)
	}

	return dedup.Results(), nil
}

// NormalizeCommand applies source-specific command normalization
func (bs *BashShellParser) NormalizeCommand(cmd string) string {
	cmd = strings.TrimSpace(cmd)
	cmd = strings.TrimSuffix(cmd, "\\")

	return syntax.NormalizeWhitespace(cmd)
}

// isFunctionStart checks if a line begins a bash function definition.
// Used to detect the opening of a potentially incomplete (unbalanced) multi-line function.
func (bs *BashShellParser) isFunctionStart(line string) bool {
	trimmed := strings.TrimSpace(line)

	// Pattern 1: "function name" or "function name()"
	if strings.HasPrefix(trimmed, "function ") {
		return true
	}

	// Pattern 2: "name() {..." — require a valid name (no whitespace/operators) before ()
	parenIdx := strings.Index(trimmed, "()")
	if parenIdx > 0 {
		beforeParen := trimmed[:parenIdx]
		if !strings.ContainsAny(beforeParen, " \t\n;|&") && strings.Contains(trimmed[parenIdx+2:], "{") {
			return true
		}
	}

	return false
}
