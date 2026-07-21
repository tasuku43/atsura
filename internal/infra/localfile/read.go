// Package localfile provides bounded regular-file reads for explicitly
// selected Atsura artifacts.
package localfile

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

var (
	ErrNotFound   = errors.New("local file not found")
	ErrPermission = errors.New("local file permission denied")
	ErrUnsafe     = errors.New("unsafe local file")
	ErrTooLarge   = errors.New("local file too large")
	ErrRead       = errors.New("local file read failed")
)

// Read opens one explicit path without following a final symlink, revalidates
// the opened identity, and returns at most limit bytes.
func Read(ctx context.Context, path string, limit int64) ([]byte, error) {
	if ctx == nil {
		return nil, fmt.Errorf("local file context is nil")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if path == "" || limit <= 0 {
		return nil, fmt.Errorf("%w: path and positive byte limit are required", ErrUnsafe)
	}
	directory, name := filepath.Split(path)
	if directory == "" {
		directory = "."
	}
	root, err := os.OpenRoot(directory)
	if err != nil {
		return nil, classify(err)
	}
	defer root.Close()
	info, err := root.Lstat(name)
	if err != nil {
		return nil, classify(err)
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
		return nil, ErrUnsafe
	}
	if info.Size() > limit {
		return nil, ErrTooLarge
	}
	file, err := root.Open(name)
	if err != nil {
		return nil, classify(err)
	}
	defer file.Close()
	opened, err := file.Stat()
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrRead, err)
	}
	if !opened.Mode().IsRegular() || !os.SameFile(info, opened) {
		return nil, ErrUnsafe
	}
	raw, err := io.ReadAll(io.LimitReader(file, limit+1))
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrRead, err)
	}
	if int64(len(raw)) > limit {
		return nil, ErrTooLarge
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return raw, nil
}

func classify(err error) error {
	switch {
	case errors.Is(err, os.ErrNotExist):
		return fmt.Errorf("%w: %v", ErrNotFound, err)
	case errors.Is(err, os.ErrPermission):
		return fmt.Errorf("%w: %v", ErrPermission, err)
	default:
		return fmt.Errorf("%w: %v", ErrRead, err)
	}
}
