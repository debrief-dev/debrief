package syntax

import (
	"strings"
)

// tokenizeWithQuotes splits a command into tokens while respecting quotes.
// Quoted strings (single or double quotes) are treated as single tokens.
// Quote and escape characters are preserved in the output.
func tokenizeWithQuotes(command string) []string {
	var tokens []string

	var current strings.Builder

	var s ScannerState

	for i := 0; i < len(command); i++ {
		ch := command[i]

		live := s.Advance(ch)

		if live && (ch == ' ' || ch == '\t') {
			if current.Len() > 0 {
				tokens = append(tokens, current.String())
				current.Reset()
			}

			continue
		}

		current.WriteByte(ch)
	}

	if current.Len() > 0 {
		tokens = append(tokens, current.String())
	}

	return tokens
}

// TokenizeCommand splits command into tokens respecting quoted strings.
// Function definitions are treated as a single token.
func TokenizeCommand(command string) []string {
	trimmed := strings.TrimSpace(command)

	if IsFunctionDefinition(trimmed) {
		return []string{command}
	}

	return tokenizeWithQuotes(command)
}
