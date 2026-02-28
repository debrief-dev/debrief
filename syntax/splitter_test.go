package syntax

import (
	"reflect"
	"testing"
)

const (
	testBashFunction = "function foo() { echo bar; }"
)

// Test function detection - Bash
func TestDetectBashFunctionWithKeyword(t *testing.T) {
	input := testBashFunction
	result := IsFunctionDefinition(input)

	if !result {
		t.Errorf("Expected true for bash function, got false")
	}
}

func TestDetectBashFunctionShorthand(t *testing.T) {
	input := "foo() { echo bar; }"
	result := IsFunctionDefinition(input)

	if !result {
		t.Errorf("Expected true for bash function shorthand, got false")
	}
}

func TestDetectBashFunctionNoParens(t *testing.T) {
	input := "function foo { echo bar; }"
	result := IsFunctionDefinition(input)

	if !result {
		t.Errorf("Expected true for bash function without parens, got false")
	}
}

func TestDetectBashFunctionMultiline(t *testing.T) {
	input := `function deploy() {
    git push
    echo "deployed"
}`
	result := IsFunctionDefinition(input)

	if !result {
		t.Errorf("Expected true for multiline bash function, got false")
	}
}

func TestDetectIncompleteBashFunction(t *testing.T) {
	input := "function foo() {"
	result := IsFunctionDefinition(input)

	if result {
		t.Errorf("Expected false for incomplete bash function, got true")
	}
}

func TestDetectNotABashFunction(t *testing.T) {
	input := "echo hello"
	result := IsFunctionDefinition(input)

	if result {
		t.Errorf("Expected false for non-function command, got true")
	}
}

func TestBashFunctionWithInternalOperators(t *testing.T) {
	input := "function foo() { a && b; c | d; }"
	result := IsFunctionDefinition(input)

	if !result {
		t.Errorf("Expected true for bash function with internal operators, got false")
	}
}

// Test function detection - PowerShell
func TestDetectPowerShellFunction(t *testing.T) {
	input := "function Get-Data { Write-Host 'test' }"
	result := IsFunctionDefinition(input)

	if !result {
		t.Errorf("Expected true for PowerShell function, got false")
	}
}

func TestDetectPowerShellFunctionWithParams(t *testing.T) {
	input := "function Get-Data([string]$name) { Write-Host $name }"
	result := IsFunctionDefinition(input)

	if !result {
		t.Errorf("Expected true for PowerShell function with params, got false")
	}
}

func TestDetectIncompletePowerShellFunction(t *testing.T) {
	input := "function Get-Data {"
	result := IsFunctionDefinition(input)

	if result {
		t.Errorf("Expected false for incomplete PowerShell function, got true")
	}
}

func TestDetectNotAPowerShellFunction(t *testing.T) {
	input := "Write-Host 'hello'"
	result := IsFunctionDefinition(input)

	if result {
		t.Errorf("Expected false for non-function command, got true")
	}
}

// Test SplitAtOperatorsWithInfo function
func TestSplitAtOperatorsWithInfo(t *testing.T) {
	tests := []struct {
		name          string
		input         string
		expectedParts []string
		expectedOps   []OperatorInfo
	}{
		{
			name:          "Single pipe",
			input:         "ls | grep foo",
			expectedParts: []string{"ls", "grep foo"},
			expectedOps:   []OperatorInfo{{Operator: opPipe, Position: 3, IsPipe: true}},
		},
		{
			name:          "Multiple pipes",
			input:         "ls | grep foo | wc -l",
			expectedParts: []string{"ls", "grep foo", "wc -l"},
			expectedOps: []OperatorInfo{
				{Operator: opPipe, Position: 3, IsPipe: true},
				{Operator: opPipe, Position: 14, IsPipe: true},
			},
		},
		{
			name:          "And operator",
			input:         "git status && git commit",
			expectedParts: []string{"git status", "git commit"},
			expectedOps:   []OperatorInfo{{Operator: opAnd, Position: 11}},
		},
		{
			name:          "Mixed operators",
			input:         "mkdir test && cd test || echo failed",
			expectedParts: []string{"mkdir test", "cd test", "echo failed"},
			expectedOps: []OperatorInfo{
				{Operator: opAnd, Position: 11},
				{Operator: opOr, Position: 22},
			},
		},
		{
			name:          "No operators",
			input:         "echo hello world",
			expectedParts: []string{"echo hello world"},
		},
		{
			name:          "Quoted operators ignored",
			input:         `echo "a && b" && echo "c | d"`,
			expectedParts: []string{`echo "a && b"`, `echo "c | d"`},
			expectedOps:   []OperatorInfo{{Operator: opAnd, Position: 14}},
		},
		{
			name:          "Semicolon",
			input:         "echo hello; echo world",
			expectedParts: []string{"echo hello", "echo world"},
			expectedOps:   []OperatorInfo{{Operator: opSemicolon, Position: 10}},
		},
		{
			name:          "Escaped quotes do not hide operators",
			input:         `echo \"hello && world\"`,
			expectedParts: []string{`echo \"hello`, `world\"`},
			expectedOps:   []OperatorInfo{{Operator: opAnd, Position: 13}},
		},
		{
			name:  "Empty input",
			input: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parts, operators := SplitAtOperatorsWithInfo(tt.input)

			if !reflect.DeepEqual(parts, tt.expectedParts) {
				t.Errorf("Parts: expected %v, got %v", tt.expectedParts, parts)
			}

			if len(operators) != len(tt.expectedOps) {
				t.Fatalf("Expected %d operators, got %d: %v",
					len(tt.expectedOps), len(operators), operators)
			}

			for i, want := range tt.expectedOps {
				got := operators[i]
				if got.Operator != want.Operator || got.IsPipe != want.IsPipe {
					t.Errorf("Operator %d: expected %s (IsPipe=%v), got %s (IsPipe=%v)",
						i, want.Operator, want.IsPipe, got.Operator, got.IsPipe)
				}
			}
		})
	}
}

func TestIsFunctionDefinitionEmptyInput(t *testing.T) {
	if IsFunctionDefinition("") {
		t.Error("Expected false for empty input")
	}
}
