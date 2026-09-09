package main

import (
	"os"
	"syscall"
)

func markTestFileSparse(f *os.File) error {
	// FSCTL_SET_SPARSE must precede extending an NTFS file. A plain truncate
	// can allocate the entire logical size instead of producing sparse holes.
	var returned uint32
	return syscall.DeviceIoControl(syscall.Handle(f.Fd()), 0x000900c4, nil, 0, nil, 0, &returned, nil)
}
