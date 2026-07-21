//go:build !windows

package terminalconfirm

import "os"

func openTerminal() (readWriteCloser, error) {
	return os.OpenFile("/dev/tty", os.O_RDWR, 0)
}
