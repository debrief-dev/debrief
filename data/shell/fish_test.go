package shell

import "testing"

func TestFishParseHistoryFile(t *testing.T) {
	commands := parseTestHistory(t, &FishShellParser{}, `- cmd: echo hello
  when: 1234567890
- cmd: git status
  when: 1234567891
- cmd: ls -la
  when: 1234567892
`, "fish_history")

	assertCommandTexts(t, commands, []string{"echo hello", "git status", "ls -la"})
}

func TestFishLineNumbersAtCmdLine(t *testing.T) {
	commands := parseTestHistory(t, &FishShellParser{}, `- cmd: echo hello
  when: 1234567890
- cmd: git status
  when: 1234567891
`, "fish_history")

	assertLineNumber(t, commands, "echo hello", 1)
	assertLineNumber(t, commands, "git status", 3)
}

func TestFishLastCommandWithoutWhen(t *testing.T) {
	commands := parseTestHistory(t, &FishShellParser{}, `- cmd: echo hello
  when: 1234567890
- cmd: git status
`, "fish_history")

	if len(commands) != 2 {
		t.Errorf("Expected 2 commands, got %d", len(commands))
	}
}

func TestFishDeduplication(t *testing.T) {
	commands := parseTestHistory(t, &FishShellParser{}, `- cmd: git status
  when: 1234567890
- cmd: echo hello
  when: 1234567891
- cmd: git status
  when: 1234567892
`, "fish_history")

	if len(commands) != 2 {
		t.Errorf("Expected 2 unique commands, got %d", len(commands))
	}

	assertFrequency(t, commands, "git status", 2)
}
