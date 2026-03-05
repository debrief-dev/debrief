package model

// Tab represents different view tabs
type Tab int

const (
	TabCommands Tab = iota
	TabTree
	TabTopCommands
	TabSettings
)
