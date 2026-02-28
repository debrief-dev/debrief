package syntax

import (
	"strings"
)

// Operator string constants.
const (
	opAnd       = "&&"
	opOr        = "||"
	opPipe      = "|"
	opSemicolon = ";"
)

// OperatorInfo represents an operator found in a command.
type OperatorInfo struct {
	Operator string // The operator itself ("&&", "||", "|", ";")
	Position int    // Character position in command
	IsPipe   bool   // True if this is a pipe operator
}

// flushPart trims the current builder content and appends it to parts if non-empty.
func flushPart(parts *[]string, current *strings.Builder) {
	if part := strings.TrimSpace(current.String()); part != "" {
		*parts = append(*parts, part)
	}
}

// SplitAtOperatorsWithInfo splits command and returns parts + operator info.
// Returns: parts []string, operators []OperatorInfo
func SplitAtOperatorsWithInfo(command string) ([]string, []OperatorInfo) {
	var (
		parts     []string
		operators []OperatorInfo
		current   strings.Builder
		s         ScannerState
	)

	for i := 0; i < len(command); i++ {
		ch := command[i]

		if !s.Advance(ch) {
			current.WriteByte(ch)
			continue
		}

		// Check for two-character operators: && or ||
		if (ch == '&' || ch == '|') && i+1 < len(command) && command[i+1] == ch {
			flushPart(&parts, &current)

			op := opAnd
			if ch == '|' {
				op = opOr
			}

			operators = append(operators, OperatorInfo{Operator: op, Position: i})

			current.Reset()

			i++ // skip the second character

			continue
		}

		// Check for single pipe
		if ch == '|' {
			flushPart(&parts, &current)

			operators = append(operators, OperatorInfo{Operator: opPipe, Position: i, IsPipe: true})

			current.Reset()

			continue
		}

		// Check for semicolon
		if ch == ';' {
			flushPart(&parts, &current)

			operators = append(operators, OperatorInfo{Operator: opSemicolon, Position: i})

			current.Reset()

			continue
		}

		current.WriteByte(ch)
	}

	// Add the last part
	flushPart(&parts, &current)

	return parts, operators
}

// IsFunctionDefinition checks if a command is a function definition.
func IsFunctionDefinition(command string) bool {
	return detectBashFunction(command) || detectPowerShellFunction(command)
}

// detectBashFunction detects bash/zsh function definitions.
// Patterns:
//   - function name() { ... }
//   - name() { ... }
//   - function name { ... }
func detectBashFunction(command string) bool {
	trimmed := strings.TrimSpace(command)

	// Must have balanced braces and contain at least one brace block.
	if !IsBalancedBraces(trimmed) || !strings.Contains(trimmed, "{") {
		return false
	}

	// Pattern 1: "function name() { ... }" or "function name { ... }"
	if strings.HasPrefix(trimmed, "function ") {
		return true
	}

	// Pattern 2: "name() { ... }"
	// Look for () followed eventually by {
	parenIdx := strings.Index(trimmed, "()")
	if parenIdx > 0 {
		// Check if there's a valid function name before ()
		beforeParen := strings.TrimSpace(trimmed[:parenIdx])
		if beforeParen != "" && !strings.ContainsAny(beforeParen, " \t\n;|&") {
			afterParen := strings.TrimSpace(trimmed[parenIdx+2:])
			if strings.HasPrefix(afterParen, "{") {
				return true
			}
		}
	}

	return false
}

// detectPowerShellFunction detects PowerShell function definitions.
// Pattern: function name { ... } or function name([params]) { ... }
func detectPowerShellFunction(command string) bool {
	trimmed := strings.TrimSpace(command)

	// Must start with "function " and have a balanced brace block.
	if !strings.HasPrefix(trimmed, "function ") {
		return false
	}

	return IsBalancedBraces(trimmed) && strings.Contains(trimmed, "{")
}
