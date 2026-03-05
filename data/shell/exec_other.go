//go:build !windows

package shell

import "os/exec"

// hideConsoleWindow is a no-op on non-Windows platforms. On Windows this
// function sets the CREATE_NO_WINDOW flag to prevent console window flashing.
func hideConsoleWindow(_ *exec.Cmd) {}
