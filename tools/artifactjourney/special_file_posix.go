//go:build linux || darwin

package main

import "syscall"

func createSpecialLifecycleFile(path string) error {
	return syscall.Mkfifo(path, 0o600)
}
