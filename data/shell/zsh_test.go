package shell

import "testing"

const testZshMultilineContent = `: 1234567890:0;echo hello \
world
: 1234567891:0;git status
`

func TestZshParseExtendedFormat(t *testing.T) {
	commands := parseTestHistory(t, &ZshShellParser{}, `: 1234567890:0;echo hello
: 1234567891:0;git status
: 1234567892:0;ls -la
`, "zsh_history")

	assertCommandTexts(t, commands, []string{"echo hello", "git status", "ls -la"})
}

func TestZshParseSimpleFormat(t *testing.T) {
	commands := parseTestHistory(t, &ZshShellParser{}, `echo hello
git status
ls -la
`, "zsh_history")

	assertCommandTexts(t, commands, []string{"echo hello", "git status", "ls -la"})
}

func TestZshMultilineContinuation(t *testing.T) {
	commands := parseTestHistory(t, &ZshShellParser{}, testZshMultilineContent, "zsh_history")

	assertCommandTexts(t, commands, []string{"echo hello world", "git status"})
}

func TestZshMultilineContinuationLineNumbers(t *testing.T) {
	commands := parseTestHistory(t, &ZshShellParser{}, testZshMultilineContent, "zsh_history")

	assertLineNumber(t, commands, "echo hello world", 1)
	assertLineNumber(t, commands, "git status", 3)
}

func TestZshDeduplication(t *testing.T) {
	commands := parseTestHistory(t, &ZshShellParser{}, `: 1234567890:0;git status
: 1234567891:0;echo hello
: 1234567892:0;git status
`, "zsh_history")

	if len(commands) != 2 {
		t.Errorf("Expected 2 unique commands, got %d", len(commands))
	}

	assertFrequency(t, commands, "git status", 2)
}

func TestZshMultilineForLoop(t *testing.T) {
	commands := parseTestHistory(t, &ZshShellParser{}, `: 1234567890:0;for i in 1 2 3
do
echo $i
done
: 1234567891:0;echo after
`, "zsh_history")

	assertCommandTexts(t, commands, []string{
		"for i in 1 2 3 do echo $i done",
		"echo after",
	})
}

func TestZshMultilineWhileLoop(t *testing.T) {
	assertSingleCommand(t, &ZshShellParser{}, `: 1234567890:0;while true
do
sleep 1
done
`, "zsh_history", "while true do sleep 1 done", "multiline while loop")
}

func TestZshTrailingBackslashAtEOF(t *testing.T) {
	commands := parseTestHistory(t, &ZshShellParser{}, `: 1234567890:0;echo trailing \`, "zsh_history")

	assertCommandTexts(t, commands, []string{"echo trailing"})
}
