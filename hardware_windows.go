//go:build windows

package main

import (
	"syscall"
	"unsafe"
)

func physicalMemory() uint64 {
	var m struct {
		Length, Load                                                                         uint32
		Total, Available, PageTotal, PageAvailable, VirtualTotal, VirtualAvailable, Extended uint64
	}
	m.Length = uint32(unsafe.Sizeof(m))
	p := syscall.NewLazyDLL("kernel32.dll").NewProc("GlobalMemoryStatusEx")
	ok, _, _ := p.Call(uintptr(unsafe.Pointer(&m)))
	if ok == 0 {
		return 0
	}
	return m.Total
}
