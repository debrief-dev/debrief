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
// Function definitions and loop constructs are treated as a single token.
// Leading environment variable assignments (KEY=VALUE) are merged with the
// command name into a single token so they share a tree node.
func TokenizeCommand(command string) []string {
	trimmed := strings.TrimSpace(command)

	if IsFunctionDefinition(trimmed) || IsLoopConstruct(trimmed) {
		return []string{command}
	}

	tokens := tokenizeWithQuotes(command)

	return mergeEnvVarPrefix(tokens)
}

// isEnvVarAssignment checks if a token is a KEY=VALUE environment variable
// assignment. The key must start with a letter or underscore and contain only
// alphanumeric characters and underscores.
func isEnvVarAssignment(token string) bool {
	eqIdx := strings.Index(token, "=")
	if eqIdx <= 0 {
		return false
	}

	key := token[:eqIdx]

	for i, ch := range key {
		if ch == '_' {
			continue
		}

		if ch >= 'a' && ch <= 'z' || ch >= 'A' && ch <= 'Z' {
			continue
		}

		if i > 0 && ch >= '0' && ch <= '9' {
			continue
		}

		return false
	}

	return true
}

// minTokensForEnvMerge is the minimum number of tokens needed for an env var
// merge to be possible (at least one env var + one command).
const minTokensForEnvMerge = 2

// mergeEnvVarPrefix merges leading KEY=VALUE tokens with the first non-env-var
// token into a single token. For example:
// ["ENV1=VAL1", "ENV2=VAL2", "go", "fmt"] → ["ENV1=VAL1 ENV2=VAL2 go", "fmt"]
func mergeEnvVarPrefix(tokens []string) []string {
	if len(tokens) < minTokensForEnvMerge {
		return tokens
	}

	// Find where env var assignments end.
	prefixEnd := 0
	for prefixEnd < len(tokens) && isEnvVarAssignment(tokens[prefixEnd]) {
		prefixEnd++
	}

	// No env var prefix, or env vars are the entire command (no command to merge with).
	if prefixEnd == 0 || prefixEnd >= len(tokens) {
		return tokens
	}

	// Merge env var tokens + command name into one token.
	merged := strings.Join(tokens[:prefixEnd+1], " ")

	result := make([]string, 0, len(tokens)-prefixEnd)
	result = append(result, merged)
	result = append(result, tokens[prefixEnd+1:]...)

	return result
}
