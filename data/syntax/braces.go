package syntax

// ScannerState tracks quote and escape state while scanning a command byte-by-byte.
type ScannerState struct {
	inSingleQuote bool
	inDoubleQuote bool
	escaped       bool
}

// Advance processes one byte and updates quote/escape state.
// Returns true if the byte is "live" (outside quotes and not consumed by an escape sequence).
// Quote characters and the backslash are consumed (return false) but callers that build
// output strings must still WriteByte(ch) for those characters themselves.
func (s *ScannerState) Advance(ch byte) bool {
	if s.escaped {
		s.escaped = false
		return false // byte consumed by preceding backslash
	}

	if ch == '\\' && !s.inSingleQuote {
		s.escaped = true
		return false
	}

	if ch == '\'' && !s.inDoubleQuote {
		s.inSingleQuote = !s.inSingleQuote
		return false
	}

	if ch == '"' && !s.inSingleQuote {
		s.inDoubleQuote = !s.inDoubleQuote
		return false
	}

	return !s.inSingleQuote && !s.inDoubleQuote
}

// IsBalancedBraces checks if braces are balanced in a command,
// respecting quotes.
func IsBalancedBraces(command string) bool {
	depth := 0

	var s ScannerState

	for i := 0; i < len(command); i++ {
		ch := command[i]

		if !s.Advance(ch) {
			continue
		}

		switch ch {
		case '{':
			depth++
		case '}':
			depth--
			if depth < 0 {
				return false // more closing than opening
			}
		}
	}

	return depth == 0
}

// isWordBoundary reports whether ch is a word boundary character for keyword detection.
func isWordBoundary(ch byte) bool {
	return ch == ' ' || ch == '\t' || ch == '\n' || ch == ';'
}

// extractWordAt extracts a word starting at position start in command,
// returning the word and the index of the last character of the word.
// A word ends at a word boundary or end of string.
// If start points at a boundary or end-of-string, returns ("", start).
func extractWordAt(command string, start int) (string, int) {
	end := start
	for end < len(command) && !isWordBoundary(command[end]) {
		end++
	}

	if end == start {
		return "", start
	}

	return command[start:end], end - 1
}

// keywordMatcher classifies a word as an opener (+1), closer (-1), or neither (0).
type keywordMatcher func(word string) int

// scanKeywordBalance scans command for keywords at word boundaries (respecting quotes)
// and tracks their nesting depth using the provided matcher.
// Returns (depth, found) where found indicates at least one keyword was matched.
func scanKeywordBalance(command string, match keywordMatcher) (int, bool) {
	depth := 0
	found := false

	var s ScannerState

	for i := 0; i < len(command); i++ {
		ch := command[i]

		if !s.Advance(ch) {
			continue
		}

		// Only check at word boundaries: start of string or after a boundary char
		if i > 0 && !isWordBoundary(command[i-1]) {
			continue
		}

		word, end := extractWordAt(command, i)

		// Check that the word ends at a boundary (end of string or boundary char follows)
		nextIdx := end + 1
		atBoundary := nextIdx >= len(command) || isWordBoundary(command[nextIdx])

		if !atBoundary {
			continue
		}

		delta := match(word)
		if delta != 0 {
			depth += delta
			found = true

			if depth < 0 {
				return depth, found
			}

			// Advance scanner state through the remaining keyword bytes
			// so quote tracking stays correct for any keyword content.
			for j := i + 1; j <= end; j++ {
				s.Advance(command[j])
			}

			i = end
		}
	}

	return depth, found
}

// doBlockMatcher classifies "do" as opener and "done" as closer.
func doBlockMatcher(word string) int {
	switch word {
	case "do":
		return 1
	case "done":
		return -1
	default:
		return 0
	}
}

// IsBalancedDoBlock checks if do/done keywords are balanced in a command,
// respecting quotes. Used for bash/zsh loop constructs (for, while, until, select).
// Keywords must appear at word boundaries to avoid matching "docker", "undone", etc.
// Returns true when depth==0, including when no do/done keywords exist.
func IsBalancedDoBlock(command string) bool {
	depth, _ := scanKeywordBalance(command, doBlockMatcher)
	return depth == 0
}

// HasBalancedDoBlock checks if a command has a complete, balanced do/done block.
// Unlike IsBalancedDoBlock (which returns true when depth==0, including when no keywords exist),
// this function returns true only when do/done keywords are present AND balanced.
// Used by shell parsers to determine if a loop construct is complete.
func HasBalancedDoBlock(command string) bool {
	depth, found := scanKeywordBalance(command, doBlockMatcher)
	return found && depth == 0
}

// fishBlockMatcher classifies Fish block-opening keywords as openers and "end" as closer.
func fishBlockMatcher(word string) int {
	switch word {
	case "for", "while", "if", "switch", "begin", "function":
		return 1
	case "end":
		return -1
	default:
		return 0
	}
}

// IsBalancedFishBlock checks if Fish block-opening keywords are balanced with "end",
// respecting quotes. Used for Fish loop/block constructs.
func IsBalancedFishBlock(command string) bool {
	depth, _ := scanKeywordBalance(command, fishBlockMatcher)

	return depth == 0
}
