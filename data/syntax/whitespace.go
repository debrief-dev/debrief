package syntax

import "strings"

// NormalizeWhitespace collapses runs of whitespace (spaces, tabs, carriage returns)
// into single spaces outside quoted strings. Whitespace inside quotes is preserved.
func NormalizeWhitespace(s string) string {
	var result strings.Builder

	var sc ScannerState

	prevSpace := false

	for i := 0; i < len(s); i++ {
		ch := s[i]

		live := sc.Advance(ch)

		if live && (ch == ' ' || ch == '\t' || ch == '\r') {
			if !prevSpace {
				result.WriteByte(' ')

				prevSpace = true
			}

			continue
		}

		result.WriteByte(ch)

		prevSpace = false
	}

	return result.String()
}
