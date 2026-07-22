//go:build darwin && (amd64 || arm64)

package shimstore

import (
	"runtime"
	"syscall"
	"unsafe"
)

const (
	renameAtXNPTrap     = 488
	renameExclusiveFlag = 0x4
)

func renameNoReplace(oldDirectoryFD int, oldName string, newDirectoryFD int, newName string) error {
	oldPointer, err := syscall.BytePtrFromString(oldName)
	if err != nil {
		return err
	}
	newPointer, err := syscall.BytePtrFromString(newName)
	if err != nil {
		return err
	}
	// #nosec G103 -- this fixed syscall is the dependency-free Darwin primitive
	// that atomically rejects an existing publication target.
	_, _, errno := syscall.Syscall6(
		renameAtXNPTrap,
		uintptr(oldDirectoryFD),
		uintptr(unsafe.Pointer(oldPointer)),
		uintptr(newDirectoryFD),
		uintptr(unsafe.Pointer(newPointer)),
		renameExclusiveFlag,
		0,
	)
	runtime.KeepAlive(oldPointer)
	runtime.KeepAlive(newPointer)
	if errno != 0 {
		return errno
	}
	return nil
}
