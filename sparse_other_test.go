//go:build !windows

package main

import "os"

func markTestFileSparse(f *os.File) error { return nil }
