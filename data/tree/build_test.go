package tree

import (
	"testing"

	"github.com/debrief-dev/debrief/data/model"
)

func TestBuildWithShellFunctions(t *testing.T) {
	commands := []*model.CommandEntry{
		{Command: `function deploy() { git push && echo "done"; }`, Frequency: 1},
		{Command: `function Get-Data { Write-Host "test" }`, Frequency: 1},
		{Command: "git status", Frequency: 2},
	}

	tree := Build(commands)

	// Tree should have 3 top-level children:
	// 1. Bash function (single node)
	// 2. PowerShell function (single node)
	// 3. "git"

	if len(tree.Children) != 3 {
		t.Errorf("Expected 3 top-level children, got %d", len(tree.Children))

		for word := range tree.Children {
			t.Logf("  Child: %q", word)
		}
	}

	bashFunc := `function deploy() { git push && echo "done"; }`
	psFunc := `function Get-Data { Write-Host "test" }`

	if _, exists := tree.Children[bashFunc]; !exists {
		t.Errorf("Bash function should be a single child node")
	}

	if _, exists := tree.Children[psFunc]; !exists {
		t.Errorf("PowerShell function should be a single child node")
	}
}

func TestBuildWithPipeCommands(t *testing.T) {
	commands := []*model.CommandEntry{
		{Command: "ls | grep foo | wc -l", Frequency: 1},
	}

	tree := Build(commands)

	// Should have 1 top-level child: "ls"
	if len(tree.Children) != 1 {
		t.Errorf("Expected 1 top-level child, got %d", len(tree.Children))

		for word := range tree.Children {
			t.Logf("  Child: %q", word)
		}
	}

	// Navigate to "ls" node
	lsNode := tree.Children["ls"]
	if lsNode == nil {
		t.Fatal("'ls' node not found")
	}

	// "ls" node should have children "| grep foo" and "| wc -l"
	expectedChildren := []string{"| grep foo", "| wc -l"}
	if len(lsNode.Children) != len(expectedChildren) {
		t.Errorf("Expected %d children under 'ls', got %d", len(expectedChildren), len(lsNode.Children))

		for word := range lsNode.Children {
			t.Logf("  Child: %q", word)
		}
	}

	for _, expected := range expectedChildren {
		if _, exists := lsNode.Children[expected]; !exists {
			t.Errorf("Expected child node not found: %s", expected)
		}
	}
}

func TestBuildWithAndOperator(t *testing.T) {
	commands := []*model.CommandEntry{
		{Command: "git status && git commit", Frequency: 1},
	}

	tree := Build(commands)

	// Should have "git" as top-level child
	gitNode := tree.Children["git"]
	if gitNode == nil {
		for word := range tree.Children {
			t.Logf("  Child: %q", word)
		}

		t.Fatal("'git' node not found")
	}

	// git node should have "status" child
	statusNode := gitNode.Children["status"]
	if statusNode == nil {
		t.Fatal("'status' node not found under 'git'")
	}

	// status node should have "&&" child
	andNode := statusNode.Children["&&"]
	if andNode == nil {
		t.Fatal("'&&' node not found under 'status'")
	}

	// && node should have "git" child
	git2Node := andNode.Children["git"]
	if git2Node == nil {
		t.Fatal("'git' node not found under '&&'")
	}

	// Second git node should have "commit" child
	commitNode := git2Node.Children["commit"]
	if commitNode == nil {
		t.Fatal("'commit' node not found under second 'git'")
	}

	// commit node should have the command attached
	if len(commitNode.Commands) == 0 {
		t.Error("Expected command attached to commit node")
	}
}

func TestBuildWithOrOperator(t *testing.T) {
	commands := []*model.CommandEntry{
		{Command: "command1 || command2", Frequency: 1},
	}

	tree := Build(commands)

	// Should have "command1" as top-level child
	cmd1Node := tree.Children["command1"]
	if cmd1Node == nil {
		t.Fatal("'command1' node not found")
	}

	// command1 should have "||" child
	orNode := cmd1Node.Children["||"]
	if orNode == nil {
		t.Fatal("'||' node not found under 'command1'")
	}

	// || should have "command2" child
	cmd2Node := orNode.Children["command2"]
	if cmd2Node == nil {
		t.Fatal("'command2' node not found under '||'")
	}

	// command2 should have the command attached
	if len(cmd2Node.Commands) == 0 {
		t.Error("Expected command attached to command2 node")
	}
}

func TestBuildWithSemicolonOperator(t *testing.T) {
	commands := []*model.CommandEntry{
		{Command: "echo hello; echo world", Frequency: 1},
	}

	tree := Build(commands)

	// Should have "echo" as top-level child
	echoNode := tree.Children["echo"]
	if echoNode == nil {
		t.Fatal("'echo' node not found")
	}

	// echo should have "hello" child
	helloNode := echoNode.Children["hello"]
	if helloNode == nil {
		t.Fatal("'hello' node not found under 'echo'")
	}

	// hello should have ";" child
	semiNode := helloNode.Children[";"]
	if semiNode == nil {
		t.Fatal("';' node not found under 'hello'")
	}

	// ; should have "echo" child
	echo2Node := semiNode.Children["echo"]
	if echo2Node == nil {
		t.Fatal("'echo' node not found under ';'")
	}

	// second echo should have "world" child
	worldNode := echo2Node.Children["world"]
	if worldNode == nil {
		t.Fatal("'world' node not found under second 'echo'")
	}

	// world should have the command attached
	if len(worldNode.Commands) == 0 {
		t.Error("Expected command attached to world node")
	}
}

func TestBuildWithMixedOperators(t *testing.T) {
	commands := []*model.CommandEntry{
		{Command: "mkdir test && cd test || echo failed", Frequency: 1},
	}

	tree := Build(commands)

	// Navigate the expected tree structure: mkdir -> test -> && -> cd -> test -> || -> echo -> failed
	mkdirNode := tree.Children["mkdir"]
	if mkdirNode == nil {
		t.Fatal("'mkdir' node not found")
	}

	testNode := mkdirNode.Children["test"]
	if testNode == nil {
		t.Fatal("'test' node not found under 'mkdir'")
	}

	andNode := testNode.Children["&&"]
	if andNode == nil {
		t.Fatal("'&&' node not found under 'test'")
	}

	cdNode := andNode.Children["cd"]
	if cdNode == nil {
		t.Fatal("'cd' node not found under '&&'")
	}

	test2Node := cdNode.Children["test"]
	if test2Node == nil {
		t.Fatal("'test' node not found under 'cd'")
	}

	orNode := test2Node.Children["||"]
	if orNode == nil {
		t.Fatal("'||' node not found under second 'test'")
	}

	echoNode := orNode.Children["echo"]
	if echoNode == nil {
		t.Fatal("'echo' node not found under '||'")
	}

	failedNode := echoNode.Children["failed"]
	if failedNode == nil {
		t.Fatal("'failed' node not found under 'echo'")
	}

	// failed should have the command attached
	if len(failedNode.Commands) == 0 {
		t.Error("Expected command attached to failed node")
	}
}

func TestBuildFunctionNotSplit(t *testing.T) {
	commands := []*model.CommandEntry{
		{Command: `function deploy() { git push && echo "done" }`, Frequency: 1},
		{Command: "git status", Frequency: 1},
	}

	tree := Build(commands)

	// Should have 2 top-level children: function and "git"
	if len(tree.Children) != 2 {
		t.Errorf("Expected 2 top-level children, got %d", len(tree.Children))

		for word := range tree.Children {
			t.Logf("  Child: %q", word)
		}
	}

	// Function should be a single node (not split by operators inside)
	funcNode := tree.Children[`function deploy() { git push && echo "done" }`]
	if funcNode == nil {
		t.Error("Function node not found")
	} else if len(funcNode.Children) != 0 {
		t.Errorf("Function should have no children (not split), got %d children", len(funcNode.Children))
	}
}

func TestPreSortChildren(t *testing.T) {
	// Create commands with shared prefixes to test sorting by CommandCount
	// CommandCount = number of commands passing through that node
	commands := []*model.CommandEntry{
		{Command: "git status", Frequency: 1},
		{Command: "git commit -m", Frequency: 1},
		{Command: "git commit -a", Frequency: 1},
		{Command: "git push origin", Frequency: 1},
		{Command: "git push", Frequency: 1},
		{Command: "ls -la", Frequency: 1},
		{Command: "ls", Frequency: 1},
		{Command: "cd docs", Frequency: 1},
	}

	treeRoot := Build(commands)

	// Before pre-sorting, SortedChildren should be nil
	if treeRoot.SortedChildren != nil {
		t.Error("SortedChildren should be nil before PreSortChildren is called")
	}

	// Call PreSortChildren
	PreSortChildren(treeRoot)

	// After pre-sorting, SortedChildren should be populated
	if treeRoot.SortedChildren == nil {
		t.Fatal("SortedChildren should not be nil after PreSortChildren")
	}

	// Root should have 3 children: git, ls, cd
	if len(treeRoot.SortedChildren) != 3 {
		t.Fatalf("Expected 3 sorted children at root, got %d", len(treeRoot.SortedChildren))
	}

	// Verify root children are sorted by command count (descending)
	// Expected order: git (5 commands), ls (2 commands), cd (1 command)
	expectedRootOrder := []string{"git", "ls", "cd"}
	expectedRootCounts := []int{5, 2, 1}

	for i, expected := range expectedRootOrder {
		if treeRoot.SortedChildren[i].Word != expected {
			t.Errorf("Root child %d: expected %s, got %s", i, expected, treeRoot.SortedChildren[i].Word)
		}

		if treeRoot.SortedChildren[i].CommandCount != expectedRootCounts[i] {
			t.Errorf("Root child %d (%s): expected count %d, got %d",
				i, expected, expectedRootCounts[i], treeRoot.SortedChildren[i].CommandCount)
		}
	}

	// Find git node and verify its children are sorted
	gitNode := treeRoot.Children["git"]
	if gitNode == nil {
		t.Fatal("git node not found")
	}

	if gitNode.SortedChildren == nil {
		t.Fatal("git node SortedChildren should not be nil")
	}

	if len(gitNode.SortedChildren) != 3 {
		t.Fatalf("Expected 3 sorted children under git, got %d", len(gitNode.SortedChildren))
	}

	// Verify git children are sorted by command count (descending)
	// Expected order: commit (2 commands), push (2 commands - alphabetically first), status (1 command)
	expectedGitOrder := []string{"commit", "push", "status"}
	expectedGitCounts := []int{2, 2, 1}

	for i, expected := range expectedGitOrder {
		if gitNode.SortedChildren[i].Word != expected {
			t.Errorf("git child %d: expected %s, got %s (count=%d)",
				i, expected, gitNode.SortedChildren[i].Word, gitNode.SortedChildren[i].CommandCount)
		}

		if gitNode.SortedChildren[i].CommandCount != expectedGitCounts[i] {
			t.Errorf("git child %d (%s): expected count %d, got %d",
				i, expected, expectedGitCounts[i], gitNode.SortedChildren[i].CommandCount)
		}
	}

	// Find ls node and verify its children are sorted
	lsNode := treeRoot.Children["ls"]
	if lsNode == nil {
		t.Fatal("ls node not found")
	}

	if lsNode.SortedChildren == nil {
		t.Fatal("ls node SortedChildren should not be nil")
	}

	if len(lsNode.SortedChildren) != 1 {
		t.Fatalf("Expected 1 sorted child under ls, got %d", len(lsNode.SortedChildren))
	}

	// Verify ls has "-la" child
	if lsNode.SortedChildren[0].Word != "-la" {
		t.Errorf("ls child: expected -la, got %s", lsNode.SortedChildren[0].Word)
	}

	// Verify cd node has sorted children
	cdNode := treeRoot.Children["cd"]
	if cdNode == nil {
		t.Fatal("cd node not found")
	}

	if cdNode.SortedChildren == nil {
		t.Fatal("cd node SortedChildren should not be nil")
	}

	if len(cdNode.SortedChildren) != 1 {
		t.Fatalf("Expected 1 sorted child under cd, got %d", len(cdNode.SortedChildren))
	}

	// Verify commit node has sorted children
	commitNode := gitNode.Children["commit"]
	if commitNode == nil {
		t.Fatal("commit node not found under git")
	}

	if commitNode.SortedChildren == nil {
		t.Fatal("commit node SortedChildren should not be nil")
	}

	if len(commitNode.SortedChildren) != 2 {
		t.Fatalf("Expected 2 sorted children under commit, got %d", len(commitNode.SortedChildren))
	}

	// commit's children should be sorted alphabetically (both have count 1): "-a" before "-m"
	if commitNode.SortedChildren[0].Word != "-a" {
		t.Errorf("commit child 0: expected -a, got %s", commitNode.SortedChildren[0].Word)
	}

	if commitNode.SortedChildren[1].Word != "-m" {
		t.Errorf("commit child 1: expected -m, got %s", commitNode.SortedChildren[1].Word)
	}
}

// TestCompoundCommandRootCommandCount verifies that compound commands (those
// containing operators) are counted at the root node, just like regular commands.
func TestCompoundCommandRootCommandCount(t *testing.T) {
	commands := []*model.CommandEntry{
		{Command: "git status && git push", Frequency: 1},
		{Command: "echo hello", Frequency: 1},
	}

	tree := Build(commands)

	// Root should count both commands (1 compound + 1 regular = 2).
	if tree.CommandCount != 2 {
		t.Errorf("root.CommandCount = %d, want 2", tree.CommandCount)
	}
}

// TestDuplicatePipelineDeduplication verifies that inserting the same pipeline
// command twice merges into existing nodes rather than overwriting them.
func TestDuplicatePipelineDeduplication(t *testing.T) {
	commands := []*model.CommandEntry{
		{Command: "ls | grep foo", Frequency: 1},
		{Command: "ls | grep foo", Frequency: 1},
	}

	tree := Build(commands)

	lsNode := tree.Children["ls"]
	if lsNode == nil {
		t.Fatal("'ls' node not found")
	}

	pipeNode := lsNode.Children["| grep foo"]
	if pipeNode == nil {
		t.Fatal("'| grep foo' node not found under 'ls'")
	}

	// The pipe node should reflect both insertions.
	if pipeNode.CommandCount != 2 {
		t.Errorf("pipe node CommandCount = %d, want 2", pipeNode.CommandCount)
	}

	if len(pipeNode.Commands) != 2 {
		t.Errorf("pipe node Commands len = %d, want 2", len(pipeNode.Commands))
	}

	if pipeNode.TerminalCount != 2 {
		t.Errorf("pipe node TerminalCount = %d, want 2", pipeNode.TerminalCount)
	}
}

// TestDuplicateFlatStructureDeduplication verifies that inserting the same
// compound (non-pipe) command twice merges operator and token nodes correctly.
func TestDuplicateFlatStructureDeduplication(t *testing.T) {
	commands := []*model.CommandEntry{
		{Command: "git status && git push", Frequency: 1},
		{Command: "git status && git push", Frequency: 1},
	}

	tree := Build(commands)

	// Navigate to the leaf node: git → status → && → git → push
	gitNode := tree.Children["git"]
	if gitNode == nil {
		t.Fatal("'git' node not found")
	}

	statusNode := gitNode.Children["status"]
	if statusNode == nil {
		t.Fatal("'status' node not found under 'git'")
	}

	andNode := statusNode.Children["&&"]
	if andNode == nil {
		t.Fatal("'&&' node not found under 'status'")
	}

	// The operator node should have been reused, not overwritten.
	if andNode.CommandCount != 2 {
		t.Errorf("'&&' node CommandCount = %d, want 2", andNode.CommandCount)
	}

	git2Node := andNode.Children["git"]
	if git2Node == nil {
		t.Fatal("second 'git' node not found under '&&'")
	}

	pushNode := git2Node.Children["push"]
	if pushNode == nil {
		t.Fatal("'push' node not found under second 'git'")
	}

	if pushNode.CommandCount != 2 {
		t.Errorf("'push' node CommandCount = %d, want 2", pushNode.CommandCount)
	}

	if len(pushNode.Commands) != 2 {
		t.Errorf("'push' node Commands len = %d, want 2", len(pushNode.Commands))
	}

	if pushNode.TerminalCount != 2 {
		t.Errorf("'push' node TerminalCount = %d, want 2", pushNode.TerminalCount)
	}
}
