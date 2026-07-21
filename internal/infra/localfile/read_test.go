package localfile

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestReadRegularBoundedFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "value")
	if err := os.WriteFile(path, []byte("value"), 0o600); err != nil {
		t.Fatal(err)
	}
	raw, err := Read(context.Background(), path, 5)
	if err != nil || string(raw) != "value" {
		t.Fatalf("Read() = %q, %v", raw, err)
	}
	if _, err := Read(context.Background(), path, 4); !errors.Is(err, ErrTooLarge) {
		t.Fatalf("oversize error = %v", err)
	}
}

func TestReadRejectsMissingAndSymlink(t *testing.T) {
	directory := t.TempDir()
	if _, err := Read(context.Background(), filepath.Join(directory, "missing"), 10); !errors.Is(err, ErrNotFound) {
		t.Fatalf("missing error = %v", err)
	}
	target := filepath.Join(directory, "target")
	if err := os.WriteFile(target, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(directory, "link")
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}
	if _, err := Read(context.Background(), link, 10); !errors.Is(err, ErrUnsafe) {
		t.Fatalf("symlink error = %v", err)
	}
}
