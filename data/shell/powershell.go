package shell

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/debrief-dev/debrief/data/model"
	"github.com/debrief-dev/debrief/data/syntax"
	"github.com/debrief-dev/debrief/infra/platform"
)

const (
	// commandTimeout is the maximum time to wait for external commands.
	commandTimeout = 10 * time.Second

	// maxMultilineAccum caps how many lines a single multiline construct
	// (function definition or loop) may span. Prevents unbounded accumulation
	// when the history file contains an unclosed block.
	maxMultilineAccum = 50
)

var (
	rel     = filepath.Join("Microsoft", "Windows", "PowerShell", "PSReadLine", "ConsoleHost_history.txt")
	unixRel = filepath.Join("powershell", "PSReadLine", "ConsoleHost_history.txt")
)

// PowerShellParser implements ShellParser for PowerShell.
type PowerShellParser struct{}

// Type returns the shell identifier for this source.
func (ps *PowerShellParser) Type() model.Shell {
	return model.PowerShell
}

// HistoryPaths returns all possible history file locations for this source.
// Dynamic detection via Get-PSReadlineOption is tried first; well-known fallback
// paths for the current platform are appended so the caller always has candidates
// even when PowerShell is not on PATH or the command times out.
func (ps *PowerShellParser) HistoryPaths() []string {
	var paths []string

	// Try dynamic detection first with timeout.
	ctx, cancel := context.WithTimeout(context.Background(), commandTimeout)
	defer cancel()

	if path, err := ps.detectPSReadLineHistory(ctx); err == nil && path != "" {
		paths = append(paths, path)
	}

	paths = append(paths, ps.fallbackPaths()...)

	return paths
}

// fallbackPaths returns well-known PSReadLine history file locations for the
// current platform, used when dynamic detection fails or times out.
func (ps *PowerShellParser) fallbackPaths() []string {
	if platform.IsWindows() {
		return ps.windowsFallbackPaths()
	}

	return ps.unixFallbackPaths()
}

// windowsFallbackPaths returns the standard PSReadLine history locations on Windows.
func (ps *PowerShellParser) windowsFallbackPaths() []string {
	var paths []string

	if appData := os.Getenv("APPDATA"); appData != "" {
		paths = append(paths, filepath.Join(appData, rel))
	}

	// Secondary fallback via USERPROFILE in case APPDATA is unset.
	if userProfile := os.Getenv("USERPROFILE"); userProfile != "" {
		paths = append(paths, filepath.Join(userProfile, "AppData", "Roaming", rel))
	}

	return paths
}

// unixFallbackPaths returns the standard PSReadLine history locations on Linux/macOS.
// PowerShell Core (pwsh) uses the XDG data directory on Linux and
// ~/Library/Application Support on macOS.
func (ps *PowerShellParser) unixFallbackPaths() []string {
	var paths []string

	// XDG_DATA_HOME takes precedence (default: ~/.local/share).
	xdgData := os.Getenv("XDG_DATA_HOME")
	if xdgData == "" {
		xdgData = platform.ExpandPath("~/.local/share")
	}

	if xdgData != "" {
		paths = append(paths, filepath.Join(xdgData, unixRel))
	}

	// macOS: ~/Library/Application Support/powershell/PSReadLine/…
	if platform.IsMacOS() {
		if home := platform.UserHomeDir(); home != "" {
			paths = append(paths, filepath.Join(home, "Library", "Application Support", unixRel))
		}
	}

	return paths
}

// detectPSReadLineHistory invokes powershell to retrieve the history file path
// configured in the running session. The provided context must carry a deadline
// so the subprocess cannot block indefinitely.
func (ps *PowerShellParser) detectPSReadLineHistory(ctx context.Context) (string, error) {
	cmd := exec.CommandContext(ctx, "powershell", "-NoProfile", "-ExecutionPolicy", "Bypass",
		"-Command", "Import-Module PSReadline; (Get-PSReadlineOption).HistorySavePath")
	hideConsoleWindow(cmd)

	output, err := cmd.CombinedOutput()
	if err != nil {
		switch ctx.Err() {
		case context.DeadlineExceeded:
			return "", fmt.Errorf("timeout getting PowerShell history path: %w", err)
		case context.Canceled:
			return "", fmt.Errorf("cancelled getting PowerShell history path: %w", err)
		}

		return "", fmt.Errorf("failed to get PowerShell history path: %w", err)
	}

	historyPath := strings.TrimSpace(string(output))
	if historyPath == "" {
		return "", errors.New("PowerShell history path is empty")
	}

	return historyPath, nil
}

// Detect checks if this source is available on the current system.
// PowerShell is primarily a Windows shell but also runs on Linux and macOS.
//
// Well-known fallback paths are tried first (cheap os.Stat) to avoid spawning
// powershell.exe on standard installations where the history file is at the
// default location. Dynamic detection is used only as a last resort for
// non-standard configurations.
func (ps *PowerShellParser) Detect() *ShellMetadata {
	// Fast path: check well-known locations first (os.Stat only, <1ms).
	for _, path := range ps.fallbackPaths() {
		info, err := os.Stat(path)
		if err != nil || info.IsDir() {
			continue
		}

		return &ShellMetadata{
			Type:     model.PowerShell,
			Path:     path,
			Parser:   ps,
			FileSize: info.Size(),
		}
	}

	// Slow path: spawn powershell.exe to query the configured history path.
	// Only reached when the history file is at a non-standard location.
	ctx, cancel := context.WithTimeout(context.Background(), commandTimeout)
	defer cancel()

	path, err := ps.detectPSReadLineHistory(ctx)
	if err != nil || path == "" {
		return nil
	}

	info, err := os.Stat(path)
	if err != nil || info.IsDir() {
		return nil
	}

	return &ShellMetadata{
		Type:     model.PowerShell,
		Path:     path,
		Parser:   ps,
		FileSize: info.Size(),
	}
}

// ParseHistoryFile reads and parses the history file at the given path.
// It handles PowerShell backtick line continuations, multiline function
// definitions and loops (capped at maxMultilineAccum), and deduplicates by command text.
func (ps *PowerShellParser) ParseHistoryFile(path string) ([]*model.CommandEntry, error) {
	f, err := os.Open(filepath.Clean(path))
	if err != nil {
		return nil, fmt.Errorf("failed to read PowerShell history: %w", err)
	}

	defer func() { _ = f.Close() }()

	// bufio.Scanner reads line-by-line without allocating the entire file as a
	// single string, avoiding a full-file copy that strings.Split would require.
	var lines []string

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("failed to read PowerShell history: %w", err)
	}

	dedup := NewCommandDeduplicator(model.PowerShell)
	lineNum := 0

	for i := 0; i < len(lines); i++ {
		lineNum++
		startLineNum := lineNum // first line of this logical command

		line := strings.TrimSpace(lines[i])
		if line == "" {
			continue
		}

		// Resolve backtick line continuations.
		fullCommand := line
		for strings.HasSuffix(line, "`") && i+1 < len(lines) {
			i++
			lineNum++
			nextLine := strings.TrimSpace(lines[i])
			fullCommand = strings.TrimSuffix(fullCommand, "`") + " " + nextLine
			line = nextLine
		}

		// Accumulate additional lines for multiline function/loop definitions until
		// braces balance or we hit the safety cap (maxMultilineAccum).
		// Both functions and loops in PowerShell use braces, so a single check suffices.
		if (ps.isFunctionStart(fullCommand) || syntax.IsPowerShellLoopPrefix(fullCommand)) && !syntax.IsBalancedBraces(fullCommand) {
			fullCommand, i, lineNum = accumulateMultilineSlice(fullCommand, lines, i, lineNum, syntax.IsBalancedBraces)
		}

		fullCommand = ps.NormalizeCommand(fullCommand)
		if fullCommand == "" {
			continue
		}

		dedup.Add(fullCommand, startLineNum)
	}

	return dedup.Results(), nil
}

// NormalizeCommand applies PowerShell-specific command normalisation.
// A trailing backtick indicates an incomplete line continuation that survived
// the accumulation loop (e.g. the file ended mid-continuation); stripping it
// avoids storing a syntactically broken command.
func (ps *PowerShellParser) NormalizeCommand(cmd string) string {
	cmd = strings.TrimSpace(cmd)
	cmd = strings.TrimSuffix(cmd, "`")

	return syntax.NormalizeWhitespace(cmd)
}

// isFunctionStart reports whether line begins a PowerShell function definition,
// regardless of whether the body braces are balanced yet. This is intentionally
// different from syntax.IsFunctionDefinition, which requires a complete,
// balanced definition. isFunctionStart is used to decide whether to start
// accumulating continuation lines.
func (ps *PowerShellParser) isFunctionStart(line string) bool {
	return strings.HasPrefix(strings.TrimSpace(line), "function ")
}
