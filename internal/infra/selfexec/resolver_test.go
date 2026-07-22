package selfexec

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
)

func TestCurrentExecutableReturnsAbsoluteCleanLocator(t *testing.T) {
	resolver := &Resolver{executable: func() (string, error) { return filepath.Join("relative", "atr"), nil }}
	path, err := resolver.CurrentExecutable(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !filepath.IsAbs(path) || filepath.Clean(path) != path || filepath.Base(path) != "atr" {
		t.Fatalf("path = %q", path)
	}
}

func TestCurrentExecutableRejectsMissingContextAndResolverFailure(t *testing.T) {
	if _, err := (*Resolver)(nil).CurrentExecutable(context.Background()); err == nil {
		t.Fatal("nil resolver succeeded")
	}
	if _, err := New().CurrentExecutable(nil); err == nil {
		t.Fatal("nil context succeeded")
	}
	want := errors.New("synthetic executable failure")
	resolver := &Resolver{executable: func() (string, error) { return "", want }}
	if _, err := resolver.CurrentExecutable(context.Background()); !errors.Is(err, want) {
		t.Fatalf("error = %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := New().CurrentExecutable(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled error = %v", err)
	}
}
