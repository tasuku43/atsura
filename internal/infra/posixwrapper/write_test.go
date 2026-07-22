//go:build !windows

package posixwrapper

import "os"

func writeExecutable(path string, content []byte) error {
	return os.WriteFile(path, content, 0o700)
}
