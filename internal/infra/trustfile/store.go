// Package trustfile persists the user-local, non-secret exact-digest bundle
// trust store. It never reads repository state or source output.
package trustfile

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"

	"github.com/tasuku43/atsura/internal/domain/bundletrust"
	"github.com/tasuku43/atsura/internal/domain/fault"
	"github.com/tasuku43/atsura/internal/infra/localfile"
	"github.com/tasuku43/atsura/internal/infra/strictjson"
)

const maxStoreBytes = int64(64 * 1024)

type Store struct{ path string }

func New(path string) *Store { return &Store{path: path} }

func DefaultPath() (string, error) {
	root, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(root, "atsura", "bundle-trust.json"), nil
}

func (s *Store) Inspect(ctx context.Context, digest string) bundletrust.State {
	store, present, err := s.load(ctx)
	if err != nil {
		return bundletrust.StateInvalid
	}
	if !present || !store.Contains(digest) {
		return bundletrust.StateNotAdopted
	}
	return bundletrust.StateAdopted
}

// Add writes one exact digest through a same-directory create-exclusive
// temporary and rename. It returns false without writing when already trusted.
func (s *Store) Add(ctx context.Context, digest string) (bool, error) {
	if ctx == nil || s == nil || !filepath.IsAbs(s.path) || filepath.Clean(s.path) != s.path {
		return false, storeFault(bundletrust.ErrInvalidStore)
	}
	parent := filepath.Dir(s.path)
	if err := ensurePrivateDirectory(parent); err != nil {
		return false, storeFault(err)
	}
	parentInfo, err := os.Lstat(parent)
	if err != nil {
		return false, storeFault(err)
	}
	root, err := os.OpenRoot(parent)
	if err != nil {
		return false, storeFault(err)
	}
	defer root.Close()
	lock, err := root.OpenFile(".bundle-trust.lock", os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return false, storeFault(err)
	}
	if err := lock.Close(); err != nil {
		_ = root.Remove(".bundle-trust.lock")
		return false, storeFault(err)
	}
	defer func() { _ = root.Remove(".bundle-trust.lock") }()
	name := filepath.Base(s.path)
	store, present, err := loadRoot(ctx, root, name)
	if err != nil {
		return false, err
	}
	if !present {
		store = bundletrust.EmptyStore()
	}
	next, changed, err := store.Add(digest)
	if err != nil || !changed {
		return false, err
	}
	data, err := json.Marshal(next)
	if err != nil || len(data)+1 > int(maxStoreBytes) {
		return false, storeFault(bundletrust.ErrInvalidStore)
	}
	data = append(data, '\n')
	if err := ctx.Err(); err != nil {
		return false, err
	}
	temporary, temporaryName, err := createRootTemporary(root)
	if err != nil {
		return false, storeFault(err)
	}
	committed := false
	defer func() {
		_ = temporary.Close()
		if !committed {
			_ = root.Remove(temporaryName)
		}
	}()
	if err := temporary.Chmod(0o600); err != nil {
		return false, storeFault(err)
	}
	if _, err := temporary.Write(data); err != nil {
		return false, storeFault(err)
	}
	if err := temporary.Sync(); err != nil {
		return false, storeFault(err)
	}
	if err := temporary.Close(); err != nil {
		return false, storeFault(err)
	}
	if err := ctx.Err(); err != nil {
		return false, err
	}
	if currentParent, err := os.Lstat(parent); err != nil || !os.SameFile(parentInfo, currentParent) {
		return false, storeFault(localfile.ErrUnsafe)
	}
	if info, err := root.Lstat(name); err == nil && (info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular()) {
		return false, storeFault(localfile.ErrUnsafe)
	} else if err != nil && !errors.Is(err, os.ErrNotExist) {
		return false, storeFault(err)
	}
	if err := root.Rename(temporaryName, name); err != nil {
		return false, storeFault(err)
	}
	committed = true
	if runtime.GOOS != "windows" {
		directory, err := root.Open(".")
		if err != nil {
			return true, storeFault(err)
		}
		defer directory.Close()
		if err := directory.Sync(); err != nil {
			return true, storeFault(err)
		}
	}
	return true, nil
}

func loadRoot(ctx context.Context, root *os.Root, name string) (bundletrust.Store, bool, error) {
	info, err := root.Lstat(name)
	if errors.Is(err, fs.ErrNotExist) {
		return bundletrust.Store{}, false, nil
	}
	if err != nil || info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() || (runtime.GOOS != "windows" && info.Mode().Perm()&0o077 != 0) {
		return bundletrust.Store{}, true, storeFault(localfile.ErrUnsafe)
	}
	file, err := root.Open(name)
	if err != nil {
		return bundletrust.Store{}, true, storeFault(err)
	}
	defer file.Close()
	opened, err := file.Stat()
	if err != nil || !os.SameFile(info, opened) {
		return bundletrust.Store{}, true, storeFault(localfile.ErrUnsafe)
	}
	raw, err := io.ReadAll(io.LimitReader(file, maxStoreBytes+1))
	if err != nil || int64(len(raw)) > maxStoreBytes {
		return bundletrust.Store{}, true, storeFault(err)
	}
	var store bundletrust.Store
	if err := strictjson.Decode(raw, &store, 8); err != nil {
		return bundletrust.Store{}, true, storeFault(err)
	}
	if err := store.Validate(); err != nil {
		return bundletrust.Store{}, true, storeFault(err)
	}
	if err := ctx.Err(); err != nil {
		return bundletrust.Store{}, true, err
	}
	return store, true, nil
}

func createRootTemporary(root *os.Root) (*os.File, string, error) {
	for attempt := 0; attempt < 100; attempt++ {
		var random [16]byte
		if _, err := rand.Read(random[:]); err != nil {
			return nil, "", err
		}
		name := fmt.Sprintf(".bundle-trust-%x", random[:])
		file, err := root.OpenFile(name, os.O_RDWR|os.O_CREATE|os.O_EXCL, 0o600)
		if errors.Is(err, fs.ErrExist) {
			continue
		}
		return file, name, err
	}
	return nil, "", fmt.Errorf("could not allocate a unique bundle trust temporary file")
}

func (s *Store) load(ctx context.Context) (bundletrust.Store, bool, error) {
	if ctx == nil || s == nil || !filepath.IsAbs(s.path) || filepath.Clean(s.path) != s.path {
		return bundletrust.Store{}, false, storeFault(bundletrust.ErrInvalidStore)
	}
	info, inspectErr := os.Lstat(s.path)
	if inspectErr == nil && (info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() || (runtime.GOOS != "windows" && info.Mode().Perm()&0o077 != 0)) {
		return bundletrust.Store{}, true, storeFault(localfile.ErrUnsafe)
	}
	if inspectErr != nil && !errors.Is(inspectErr, os.ErrNotExist) {
		return bundletrust.Store{}, true, storeFault(inspectErr)
	}
	raw, err := localfile.Read(ctx, s.path, maxStoreBytes)
	if errors.Is(err, localfile.ErrNotFound) {
		return bundletrust.Store{}, false, nil
	}
	if err != nil {
		return bundletrust.Store{}, true, storeFault(err)
	}
	var store bundletrust.Store
	if err := strictjson.Decode(raw, &store, 8); err != nil {
		return bundletrust.Store{}, true, storeFault(err)
	}
	if err := store.Validate(); err != nil {
		return bundletrust.Store{}, true, storeFault(err)
	}
	return store, true, nil
}

func ensurePrivateDirectory(path string) error {
	parent := filepath.Dir(path)
	parentInfo, err := os.Lstat(parent)
	if err != nil || parentInfo.Mode()&os.ModeSymlink != 0 || !parentInfo.IsDir() {
		return localfile.ErrUnsafe
	}
	info, err := os.Lstat(path)
	if errors.Is(err, os.ErrNotExist) {
		if err := os.Mkdir(path, 0o700); err != nil {
			return err
		}
		info, err = os.Lstat(path)
	}
	if err != nil || info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		return localfile.ErrUnsafe
	}
	if info.Mode().Perm()&0o077 != 0 {
		return localfile.ErrUnsafe
	}
	return nil
}

func storeFault(err error) error {
	return fault.Wrap(fault.KindUnavailable, "bundle_trust_store_failed", "The user-local bundle trust store could not be read or updated safely.", false, err,
		fault.NextAction{Command: "bundle status", Reason: "Reconcile the exact bundle adoption state without repeating the mutation."})
}
