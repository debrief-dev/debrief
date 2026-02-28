package model

// Shell identifies which shell/terminal a command came from.
type Shell int

const (
	UnknownShell Shell = iota
	PowerShell
	GitBash
	WSLBash
	Bash
	Zsh
	Fish
	CustomShell
)

// String returns the human-readable display name for a Shell.
func (s Shell) String() string {
	switch s {
	case PowerShell:
		return "PowerShell"
	case GitBash:
		return "Git Bash"
	case WSLBash:
		return "WSL Bash"
	case Bash:
		return "Bash"
	case Zsh:
		return "Zsh"
	case Fish:
		return "Fish"
	default:
		return "Unknown"
	}
}
