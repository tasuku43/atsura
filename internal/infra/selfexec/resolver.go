// Package selfexec resolves the currently running Atsura executable without
// granting it source-execution authority.
package selfexec

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
)

// Resolver supplies the current process executable as a locator. Callers must
// fingerprint and validate the returned path before making it binding
// authority.
type Resolver struct {
	executable func() (string, error)
}

// New creates a resolver backed by os.Executable.
func New() *Resolver {
	return &Resolver{executable: os.Executable}
}

// CurrentExecutable returns one absolute, clean executable locator.
func (r *Resolver) CurrentExecutable(ctx context.Context) (string, error) {
	if ctx == nil {
		return "", fmt.Errorf("current executable context is nil")
	}
	if err := ctx.Err(); err != nil {
		return "", err
	}
	if r == nil || r.executable == nil {
		return "", fmt.Errorf("current executable resolver is not configured")
	}
	path, err := r.executable()
	if err != nil {
		return "", fmt.Errorf("resolve current executable: %w", err)
	}
	absolute, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("make current executable absolute: %w", err)
	}
	clean := filepath.Clean(absolute)
	if clean == "" || !filepath.IsAbs(clean) {
		return "", fmt.Errorf("current executable path is not absolute")
	}
	if err := ctx.Err(); err != nil {
		return "", err
	}
	return clean, nil
}
