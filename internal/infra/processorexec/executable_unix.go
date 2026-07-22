//go:build !windows

package processorexec

import "os"

func platformExecutable(_ string, mode os.FileMode) bool {
	return mode.Perm()&0o111 != 0
}
