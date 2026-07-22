//go:build (!linux && !darwin) || (!amd64 && !arm64)

package shimstore

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/tasuku43/atsura/internal/domain/wrappershim"
)

func TestUnsupportedStoreNeverCreatesState(t *testing.T) {
	root := filepath.Join(t.TempDir(), "store")
	store := New(root)
	if _, _, err := store.Install(context.Background(), wrappershim.Manifest{}, []byte("ignored")); !errors.Is(err, ErrUnsupported) {
		t.Fatalf("Install() error = %v", err)
	}
	if _, err := store.Status(context.Background()); !errors.Is(err, ErrUnsupported) {
		t.Fatalf("Status() error = %v", err)
	}
	if _, err := store.Remove(context.Background(), wrappershim.Reference("ignored")); !errors.Is(err, ErrUnsupported) {
		t.Fatalf("Remove() error = %v", err)
	}
	if _, err := os.Lstat(root); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("unsupported operations created state: %v", err)
	}
}
