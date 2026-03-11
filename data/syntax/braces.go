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
