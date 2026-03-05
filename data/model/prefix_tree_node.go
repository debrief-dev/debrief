package model

// PrefixTreeNode represents a node in the command hierarchy tree
type PrefixTreeNode struct {
	Word           string                     // Current word/token
	FullPath       string                     // Full command path to this node
	Children       map[string]*PrefixTreeNode // Child nodes (next words)
	SortedChildren []*PrefixTreeNode          // Pre-sorted children for rendering (sorted once at build time)
	CommandCount   int                        // Total commands passing through this node
	TerminalCount  int                        // Commands ending at this node
	Commands       []*CommandEntry            // Commands that end here
	Level          int                        // Depth in tree (0 = root)
}
