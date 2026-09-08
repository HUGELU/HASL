package main

import (
	"os/exec"
	"syscall"
)

func prepareNativeCommand(cmd *exec.Cmd) {
	// Report loader failures to the parent log instead of an invisible modal
	// dialog, and keep native worker consoles from flashing over the studio.
	kernel := syscall.NewLazyDLL("kernel32.dll")
	_, _, _ = kernel.NewProc("SetErrorMode").Call(0x0001 | 0x0002 | 0x8000)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
}
