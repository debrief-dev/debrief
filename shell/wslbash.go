package shell

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/debrief-dev/debrief/config"
	"github.com/debrief-dev/debrief/model"
)

// Binary operation constants for UTF-16 decoding
const (
	// UTF16BytesPerChar is the number of bytes per UTF-16 character
	UTF16BytesPerChar = 2
	// ByteShift is the bit shift for combining bytes in little-endian order
	ByteShift = 8
)

const (
	// wslCommandTimeout is the maximum time to wait for WSL commands.
	wslCommandTimeout = 10 * time.Second

	// wslBOM is the Unicode BOM (U+FEFF) that WSL may prepend to --list output.
	wslBOM = '\uFEFF'
)

// wslPrefixes lists the UNC prefixes used to access WSL filesystems, in
// preference order (newer format first).
var wslPrefixes = []string{`\\wsl.localhost`, `\\wsl$`}

// WSLBashShellParser implements HistorySource for WSL Bash.
type WSLBashShellParser struct {
	BashShellParser // Embed BashSource for shared logic
}

// Type returns the shell identifier for this source.
func (wbs *WSLBashShellParser) Type() model.Shell {
	return model.WSLBash
}

// detectWSLDistros runs wsl.exe --list --quiet and returns the distro names.
func (wbs *WSLBashShellParser) detectWSLDistros(ctx context.Context) ([]string, error) {
	cmd := exec.CommandContext(ctx, "wsl.exe", "--list", "--quiet")
	hideConsoleWindow(cmd)

	output, err := cmd.Output()
	if err != nil {
		switch ctx.Err() {
		case context.DeadlineExceeded:
			return nil, fmt.Errorf("timeout detecting WSL distros: %w", err)
		case context.Canceled:
			return nil, fmt.Errorf("cancelled detecting WSL distros: %w", err)
		default:
			return nil, fmt.Errorf("failed to detect WSL distros: %w", err)
		}
	}

	// wsl.exe --list emits UTF-16LE; convert to UTF-8 before splitting.
	decoded := decodeUTF16(output)

	var distros []string

	for _, line := range strings.Split(decoded, "\n") {
		// Strip the BOM (U+FEFF) that WSL prepends to the first line, plus
		// any CR or null characters left over from the UTF-16 stream.
		distro := strings.TrimFunc(strings.TrimSpace(line), func(r rune) bool {
			return r == wslBOM || r == '\r' || r == '\x00'
		})
		if distro != "" {
			distros = append(distros, distro)
		}
	}

	return distros, nil
}

// decodeUTF16 converts a UTF-16LE byte slice to a UTF-8 string.
// If the byte slice has an odd length it cannot be valid UTF-16, so it is
// returned as a raw string.
func decodeUTF16(b []byte) string {
	if len(b)%2 != 0 {
		return string(b)
	}

	runes := make([]rune, 0, len(b)/UTF16BytesPerChar)
	for i := 0; i+1 < len(b); i += 2 {
		r := rune(b[i]) | rune(b[i+1])<<ByteShift
		if r != 0 {
			runes = append(runes, r)
		}
	}

	return string(runes)
}

// collectPathsForDistro returns .bash_history paths found under the given
// distro.  It tries UNC prefixes in preference order and short-circuits after
// the first one that yields results, preventing duplicate paths (both prefixes
// resolve to the same underlying WSL filesystem on modern Windows).
func collectPathsForDistro(distro string) []string {
	for _, prefix := range wslPrefixes {
		homePath := filepath.Join(prefix, distro, "home")

		entries, err := os.ReadDir(homePath)
		if err != nil {
			continue // prefix not accessible; try the next one
		}

		var paths []string

		for _, entry := range entries {
			if entry.IsDir() {
				paths = append(paths, filepath.Join(homePath, entry.Name(), ".bash_history"))
			}
		}

		if len(paths) > 0 {
			return paths // first working prefix wins; skip the older one
		}
	}

	return nil
}

// detectPaths runs detectWSLDistros under the provided context and collects
// history paths for all distros concurrently.
func (wbs *WSLBashShellParser) detectPaths(ctx context.Context) []string {
	distros, err := wbs.detectWSLDistros(ctx)
	if err != nil || len(distros) == 0 {
		return nil
	}

	// Fan out: one goroutine per distro to parallelise os.ReadDir calls.
	results := make([][]string, len(distros))

	var wg sync.WaitGroup
	wg.Add(len(distros))

	for i, distro := range distros {
		go func() {
			defer wg.Done()

			results[i] = collectPathsForDistro(distro)
		}()
	}

	wg.Wait()

	var all []string
	for _, paths := range results {
		all = append(all, paths...)
	}

	return all
}

// HistoryPaths returns all .bash_history paths found across WSL distros.
func (wbs *WSLBashShellParser) HistoryPaths() []string {
	if !config.IsWindows() {
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), wslCommandTimeout)
	defer cancel()

	return wbs.detectPaths(ctx)
}

// Detect checks if this source is available on the current system.
func (wbs *WSLBashShellParser) Detect() *ShellMetadata {
	if !config.IsWindows() {
		return nil
	}

	if _, err := exec.LookPath("wsl.exe"); err != nil {
		return nil
	}

	for _, path := range wbs.HistoryPaths() {
		info, err := os.Stat(path)
		if err != nil || info.IsDir() {
			continue
		}

		return &ShellMetadata{
			Type:     model.WSLBash,
			Path:     path,
			Parser:   wbs,
			FileSize: info.Size(),
		}
	}

	return nil
}

// ParseHistoryFile reads and parses the history file at the given path.
func (wbs *WSLBashShellParser) ParseHistoryFile(path string) ([]*model.CommandEntry, error) {
	return parseBashHistoryAs(&wbs.BashShellParser, path, model.WSLBash)
}
