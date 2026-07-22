// Package shimstore persists fixed-template executable wrapper shims in one
// private user-local root. Platform-specific files own every filesystem
// operation; this file contains only the portable constructor and categories.
package shimstore

import (
	"os"
	"path/filepath"

	"github.com/tasuku43/atsura/internal/domain/wrappershim"
)

var (
	ErrInvalidInput = wrappershim.ErrInvalidInput
	ErrUnsupported  = wrappershim.ErrUnsupported
	ErrUnsafeStore  = wrappershim.ErrUnsafeStore
	ErrCapacity     = wrappershim.ErrCapacity
	ErrNotFound     = wrappershim.ErrNotFound
	ErrConflict     = wrappershim.ErrConflict
	ErrTampered     = wrappershim.ErrTampered
	ErrUncertain    = wrappershim.ErrUncertain
)

// Store owns exactly one injected wrapper-shim root. New performs no I/O.
type Store struct{ root string }

func New(root string) *Store { return &Store{root: root} }

// DefaultRoot returns the versioned private store location without creating it.
func DefaultRoot() (string, error) {
	root, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(root, "atsura", "wrapper-shims", "v1"), nil
}

// BinPath returns the caller-owned PATH entry without creating or attesting it.
func (s *Store) BinPath() (string, error) {
	if !validStore(s) {
		return "", ErrInvalidInput
	}
	return filepath.Join(s.root, binDirectoryName), nil
}

func validStore(s *Store) bool {
	if s == nil || s.root == "" || !filepath.IsAbs(s.root) || filepath.Clean(s.root) != s.root {
		return false
	}
	base := filepath.Base(s.root)
	return base != "." && base != string(filepath.Separator)
}
