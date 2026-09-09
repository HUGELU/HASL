//go:build !windows

package main

import "os/exec"

func prepareNativeCommand(cmd *exec.Cmd) {}
