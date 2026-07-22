//go:build !linux && !darwin

package main

import "fmt"

func createSpecialLifecycleFile(string) error {
	return fmt.Errorf("special lifecycle fixture is unsupported")
}
