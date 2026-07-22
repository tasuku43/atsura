//go:build !windows

package processormanifest

import (
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
)

func TestLoadRejectsSpecialManifestFile(t *testing.T) {
	root := t.TempDir()
	harness := filepath.Join(root, ".harness")
	if err := os.Mkdir(harness, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := syscall.Mkfifo(filepath.Join(root, filepath.FromSlash(Path)), 0o600); err != nil {
		t.Skipf("FIFO unavailable: %v", err)
	}
	if _, err := Load(root); err == nil || !strings.Contains(err.Error(), "regular file") {
		t.Fatalf("Load() error = %v", err)
	}
}
