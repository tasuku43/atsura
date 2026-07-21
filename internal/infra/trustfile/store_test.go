package trustfile

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/tasuku43/atsura/internal/domain/bundletrust"
)

func TestStoreAddsExactReceiptAndDoesNotOverwriteInvalidState(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "atsura", "bundle-trust.json")
	store := New(path)
	digest := "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	if state := store.Inspect(context.Background(), digest); state != bundletrust.StateNotAdopted {
		t.Fatalf("initial state = %q", state)
	}
	changed, err := store.Add(context.Background(), digest)
	if err != nil || !changed {
		t.Fatalf("Add() = %v, %v", changed, err)
	}
	if state := store.Inspect(context.Background(), digest); state != bundletrust.StateAdopted {
		t.Fatalf("trusted state = %q", state)
	}
	changed, err = store.Add(context.Background(), digest)
	if err != nil || changed {
		t.Fatalf("duplicate Add() = %v, %v", changed, err)
	}
	if err := os.WriteFile(path, []byte(`{"schema_version":1,"receipts":[],"unknown":true}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if state := store.Inspect(context.Background(), digest); state != bundletrust.StateInvalid {
		t.Fatalf("invalid state = %q", state)
	}
	if changed, err := store.Add(context.Background(), digest); err == nil || changed {
		t.Fatalf("invalid Add() = %v, %v", changed, err)
	}
}

func TestStoreRejectsSymlinkedTrustDirectory(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(root, "target")
	if err := os.Mkdir(target, 0o700); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(root, "atsura")
	if err := os.Symlink(target, link); err != nil {
		t.Skip(err)
	}
	store := New(filepath.Join(link, "bundle-trust.json"))
	digest := "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	if changed, err := store.Add(context.Background(), digest); err == nil || changed {
		t.Fatalf("Add() = %v, %v", changed, err)
	}
}

func TestStoreRejectsConcurrentWriterWithoutChangingReceipts(t *testing.T) {
	root := t.TempDir()
	directory := filepath.Join(root, "atsura")
	if err := os.Mkdir(directory, 0o700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(directory, "bundle-trust.json")
	store := New(path)
	digest := "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	if err := os.WriteFile(filepath.Join(directory, ".bundle-trust.lock"), []byte("busy"), 0o600); err != nil {
		t.Fatal(err)
	}
	if changed, err := store.Add(context.Background(), digest); err == nil || changed {
		t.Fatalf("Add() = %v, %v", changed, err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("trust file changed during conflict: %v", err)
	}
}
