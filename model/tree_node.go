package model

// TreeDisplayNode is a flattened tree node for rendering
type TreeDisplayNode struct {
	Node                *PrefixTreeNode
	Path                string // Full path including current word (e.g., "git commit -m")
	PathPrefix          string // Path without current word (e.g., "git commit") - cached for rendering
	PathPrefixWithSpace string // pre-cached to avoid allocation
	Depth               int
	HasChildren         bool
	IsLeaf              bool
	FilteredFrequency   int           // Total frequency considering search + source filters
	MostFrequentCmd     *CommandEntry // Most frequent command among filtered ones
	CachedMetadata      string        // Pre-formatted metadata string (e.g., "Used 5 times · bash")
}
