//go:build windows

package cli

import (
	"os/exec"
	"syscall"
)

const createNoWindow = 0x08000000

func withoutControllingTerminal(command *exec.Cmd) {
	command.SysProcAttr = &syscall.SysProcAttr{CreationFlags: createNoWindow, HideWindow: true}
}
