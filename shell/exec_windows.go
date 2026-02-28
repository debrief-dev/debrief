//go:build windows

package shell

import (
	"os/exec"
	"syscall"
)

// createNoWindowFlag is the Windows CREATE_NO_WINDOW process creation flag.
// It prevents a subprocess from creating or inheriting a console window.
const createNoWindowFlag = 0x08000000

// hideConsoleWindow configures cmd so that it does not create a visible
// console window when run from a GUI application on Windows.
func hideConsoleWindow(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{
		CreationFlags: createNoWindowFlag,
	}
}
