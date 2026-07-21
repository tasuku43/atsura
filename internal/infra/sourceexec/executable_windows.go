//go:build windows

package sourceexec

import (
	"os"
	"path/filepath"
	"strings"
)

func platformExecutable(path string, _ os.FileMode) bool {
	extension := strings.ToLower(filepath.Ext(path))
	return extension == ".exe" || extension == ".com"
}
