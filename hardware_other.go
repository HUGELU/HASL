//go:build !windows

package main

import (
	"os"
	"runtime"
	"strconv"
	"strings"
)

func physicalMemory() uint64 {
	if runtime.GOOS == "darwin" {
		n, _ := strconv.ParseUint(strings.TrimSpace(hardwareCommand("sysctl", "-n", "hw.memsize")), 10, 64)
		return n
	}
	b, _ := os.ReadFile("/proc/meminfo")
	return parseMemoryKB(string(b))
}
