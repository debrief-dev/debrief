package tree

import (
	"sort"
	"strings"

	"github.com/debrief-dev/debrief/data/model"
	"github.com/debrief-dev/debrief/data/syntax"
)

// Build constructs hierarchical command tree with dynamic branching
func Build(commands []*model.CommandEntry) *model.PrefixTreeNode {
	root := &model.PrefixTreeNode{
		Word:     "",
		FullPath: "",
		Children: make(map[string]*model.PrefixTreeNode),
		Level:    0,
	}

	for _, cmd := range commands {
		// Split once to detect operators; reuse result for compound commands
		parts, operators := syntax.SplitAtOperatorsWithInfo(cmd.Command)
		if len(operators) > 0 && !syntax.IsFunctionDefinition(cmd.Command) && !syntax.IsLoopConstruct(cmd.Command) {
			// Insert compound command with special operator handling
			insertCompoundCommand(root, cmd, parts, operators)
		} else {
			// Regular command - tokenize normally
			tokens := syntax.TokenizeCommand(cmd.Command)
			insertIntoTree(root, tokens, cmd)
		}
	}

	return root
}

// insertIntoTree adds command tokens to the tree iteratively.
func insertIntoTree(node *model.PrefixTreeNode, tokens []string, cmd *model.CommandEntry) {
	node.CommandCount++

	if len(tokens) == 0 {
		node.TerminalCount++
		node.Commands = append(node.Commands, cmd)

		return
	}

	var sb strings.Builder

	current := node

	for _, token := range tokens {
		child := getOrCreateChild(current, token, &sb)
		child.CommandCount++
		current = child
	}

	current.TerminalCount++
	current.Commands = append(current.Commands, cmd)
}

// SortNodesByCommandCount sorts nodes by command count (descending), then alphabetically for stable ordering
func SortNodesByCommandCount(nodes []*model.PrefixTreeNode) {
	sort.Slice(nodes, func(i, j int) bool {
		// Primary sort: by command count (descending)
		if nodes[i].CommandCount != nodes[j].CommandCount {
			return nodes[i].CommandCount > nodes[j].CommandCount
		}
		// Secondary sort: alphabetically (ascending) for stable order
		return nodes[i].Word < nodes[j].Word
	})
}

// PreSortChildren recursively sorts all children in the tree after construction.
// This should be called once after Build completes.
// Pre-sorting eliminates redundant sorting operations during rendering.
func PreSortChildren(root *model.PrefixTreeNode) {
	if root == nil {
		return
	}

	// Convert map to slice and sort
	if len(root.Children) > 0 {
		root.SortedChildren = make([]*model.PrefixTreeNode, 0, len(root.Children))
		for _, child := range root.Children {
			root.SortedChildren = append(root.SortedChildren, child)
		}

		SortNodesByCommandCount(root.SortedChildren)

		// Recursively sort all descendants
		for _, child := range root.SortedChildren {
			PreSortChildren(child)
		}
	}
}

// getOrCreateChild returns the child node for the given token, creating it if absent.
// The returned node's CommandCount is NOT incremented — the caller decides whether and how to bump it.
func getOrCreateChild(parent *model.PrefixTreeNode, token string, sb *strings.Builder) *model.PrefixTreeNode {
	child, exists := parent.Children[token]
	if !exists {
		sb.Reset()
		sb.WriteString(parent.FullPath)

		if sb.Len() > 0 {
			sb.WriteByte(' ')
		}

		sb.WriteString(token)

		child = &model.PrefixTreeNode{
			Word:     token,
			FullPath: sb.String(),
			Children: make(map[string]*model.PrefixTreeNode),
			Level:    parent.Level + 1,
		}
		parent.Children[token] = child
	}

	return child
}

// insertCompoundCommand handles commands with operators.
// parts and operators are pre-computed by the caller via syntax.SplitAtOperatorsWithInfo.
func insertCompoundCommand(root *model.PrefixTreeNode, cmd *model.CommandEntry, parts []string, operators []syntax.OperatorInfo) {
	// Count this command at the root, mirroring insertIntoTree's entry-point increment.
	root.CommandCount++

	// Determine structure based on operator types
	hasPipes := false

	for _, op := range operators {
		if op.IsPipe {
			hasPipes = true
			break
		}
	}

	if hasPipes {
		// Pipe structure: nest under first command
		insertPipelineStructure(root, parts, cmd)
	} else {
		// Non-pipe operators: flat structure with operators as nodes
		insertFlatStructure(root, parts, operators, cmd)
	}
}

// insertPipelineStructure creates nested tree for pipe commands
// Example: "ls | grep foo | wc -l" → ls → ["| grep foo", "| wc -l"]
func insertPipelineStructure(root *model.PrefixTreeNode, parts []string, cmd *model.CommandEntry) {
	if len(parts) == 0 {
		return
	}

	// Tokenize first part normally
	firstTokens := syntax.TokenizeCommand(parts[0])

	var sb strings.Builder

	// Navigate to the leaf of first command
	currentNode := root
	for _, token := range firstTokens {
		child := getOrCreateChild(currentNode, token, &sb)
		child.CommandCount++
		currentNode = child
	}

	// Add remaining parts as "| command" children (as siblings under currentNode)
	var lastPipedChild *model.PrefixTreeNode

	for i := 1; i < len(parts); i++ {
		sb.Reset()
		sb.WriteString("| ")
		sb.WriteString(parts[i])

		pipedPart := sb.String()

		child := getOrCreateChild(currentNode, pipedPart, &sb)
		child.CommandCount++
		lastPipedChild = child
	}

	// Attach the command to the last piped part (or first command if no pipes)
	if lastPipedChild != nil {
		lastPipedChild.Commands = append(lastPipedChild.Commands, cmd)
		lastPipedChild.TerminalCount++
	} else {
		currentNode.Commands = append(currentNode.Commands, cmd)
		currentNode.TerminalCount++
	}
}

// insertFlatStructure creates flat tree with operators as separate nodes
// Example: "git status && git commit" → git -> status -> && -> git -> commit
func insertFlatStructure(root *model.PrefixTreeNode, parts []string, operators []syntax.OperatorInfo, cmd *model.CommandEntry) {
	// Tokenize the first part to create the tree structure
	if len(parts) == 0 {
		return
	}

	firstTokens := syntax.TokenizeCommand(parts[0])
	currentNode := root

	var sb strings.Builder

	// Build the first command path
	for _, token := range firstTokens {
		child := getOrCreateChild(currentNode, token, &sb)
		child.CommandCount++
		currentNode = child
	}

	// Add operators and remaining parts as a chain
	lastNode := currentNode

	for i := range operators {
		// Add operator node (reuse if already present from a prior identical compound command)
		opChild := getOrCreateChild(lastNode, operators[i].Operator, &sb)
		opChild.CommandCount++
		lastNode = opChild

		// Add next command part (tokenized)
		if i+1 < len(parts) {
			nextPartTokens := syntax.TokenizeCommand(parts[i+1])
			for _, token := range nextPartTokens {
				tokenChild := getOrCreateChild(lastNode, token, &sb)
				tokenChild.CommandCount++
				lastNode = tokenChild
			}
		}
	}

	// Attach the command to the final leaf node
	lastNode.Commands = append(lastNode.Commands, cmd)
	lastNode.TerminalCount++
}
