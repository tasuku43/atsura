//go:build linux || darwin

package shimstore

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"syscall"
	"testing"

	"github.com/tasuku43/atsura/internal/domain/wrapperbinding"
	"github.com/tasuku43/atsura/internal/domain/wrappershim"
	"github.com/tasuku43/atsura/internal/infra/posixshim"
)

func TestStoreLifecyclePublishesIdempotentHardLinkAndRemovesExactReference(t *testing.T) {
	root := filepath.Join(t.TempDir(), "atsura", "wrapper-shims", "v1")
	store := New(root)
	manifest, shim := testArtifact(t, "gh", "first")

	before, err := store.Status(context.Background())
	if err != nil || len(before.Records) != 0 || len(before.Collisions) != 0 {
		t.Fatalf("absent Status() = %+v, %v", before, err)
	}
	if _, err := os.Lstat(root); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("read-only status created root: %v", err)
	}

	record, alreadyInstalled, err := store.Install(context.Background(), manifest, shim)
	if err != nil || alreadyInstalled || record != ownedSummary(manifest, wrappershim.StateOwnedActive) {
		t.Fatalf("Install() = %+v, %v, %v", record, alreadyInstalled, err)
	}
	assertMode(t, root, os.ModeDir|0o700)
	assertMode(t, filepath.Join(root, binDirectoryName), os.ModeDir|0o700)
	assertMode(t, filepath.Join(root, recordsDirectoryName), os.ModeDir|0o700)
	recordPath := filepath.Join(root, recordsDirectoryName, manifest.Reference.String())
	assertMode(t, recordPath, os.ModeDir|0o700)
	assertMode(t, filepath.Join(recordPath, manifestFileName), 0o600)
	assertMode(t, filepath.Join(recordPath, shimFileName), 0o700)
	assertMode(t, filepath.Join(root, binDirectoryName, "gh"), 0o700)
	assertSameFile(t, filepath.Join(recordPath, shimFileName), filepath.Join(root, binDirectoryName, "gh"))
	manifestBytes, err := os.ReadFile(filepath.Join(recordPath, manifestFileName))
	if err != nil {
		t.Fatal(err)
	}
	wantManifest, err := manifest.CanonicalBytes()
	if err != nil || !reflect.DeepEqual(manifestBytes, wantManifest) {
		t.Fatalf("manifest bytes mismatch: %v", err)
	}

	firstInfo, err := os.Lstat(filepath.Join(root, binDirectoryName, "gh"))
	if err != nil {
		t.Fatal(err)
	}
	record, alreadyInstalled, err = store.Install(context.Background(), manifest, append([]byte(nil), shim...))
	if err != nil || !alreadyInstalled || record.State != wrappershim.StateOwnedActive {
		t.Fatalf("idempotent Install() = %+v, %v, %v", record, alreadyInstalled, err)
	}
	secondInfo, err := os.Lstat(filepath.Join(root, binDirectoryName, "gh"))
	if err != nil || !os.SameFile(firstInfo, secondInfo) {
		t.Fatalf("idempotent install replaced shim: %v", err)
	}

	inventory, err := store.Status(context.Background())
	wantInventory, _ := wrappershim.SortInventory([]wrappershim.Record{ownedSummary(manifest, wrappershim.StateOwnedActive)}, []wrappershim.Record{})
	if err != nil || !reflect.DeepEqual(inventory, wantInventory) {
		t.Fatalf("Status() = %+v, %v; want %+v", inventory, err, wantInventory)
	}

	removed, err := store.Remove(context.Background(), manifest.Reference)
	if err != nil || removed.State != wrappershim.StateOwnedActive || removed.Reference != manifest.Reference {
		t.Fatalf("Remove() = %+v, %v", removed, err)
	}
	for _, path := range []string{recordPath, filepath.Join(root, binDirectoryName, "gh")} {
		if _, err := os.Lstat(path); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("removed path %q remains: %v", path, err)
		}
	}
	if _, err := store.Remove(context.Background(), manifest.Reference); !errors.Is(err, ErrNotFound) {
		t.Fatalf("second Remove() error = %v", err)
	}
	after, err := store.Status(context.Background())
	if err != nil || len(after.Records) != 0 || len(after.Collisions) != 0 {
		t.Fatalf("final Status() = %+v, %v", after, err)
	}
}

func TestInstallReactivatesExactInactiveRecord(t *testing.T) {
	root := filepath.Join(t.TempDir(), "store")
	store := New(root)
	manifest, shim := testArtifact(t, "gh", "reactivate")
	if _, _, err := store.Install(context.Background(), manifest, shim); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(root, binDirectoryName, "gh")); err != nil {
		t.Fatal(err)
	}
	record, already, err := store.Install(context.Background(), manifest, shim)
	if err != nil || already || record.State != wrappershim.StateOwnedActive {
		t.Fatalf("Install(inactive) = %+v, %v, %v", record, already, err)
	}
	assertSameFile(t,
		filepath.Join(root, recordsDirectoryName, manifest.Reference.String(), shimFileName),
		filepath.Join(root, binDirectoryName, "gh"),
	)
}

func TestInstallNeverReplacesDifferentOrForeignCommand(t *testing.T) {
	t.Run("different owned artifact", func(t *testing.T) {
		root := filepath.Join(t.TempDir(), "store")
		store := New(root)
		firstManifest, firstShim := testArtifact(t, "gh", "first")
		secondManifest, secondShim := testArtifact(t, "gh", "second")
		if firstManifest.Reference == secondManifest.Reference {
			t.Fatal("fixtures must differ")
		}
		if _, _, err := store.Install(context.Background(), firstManifest, firstShim); err != nil {
			t.Fatal(err)
		}
		if _, _, err := store.Install(context.Background(), secondManifest, secondShim); !errors.Is(err, ErrConflict) {
			t.Fatalf("different Install() error = %v", err)
		}
		if _, err := os.Lstat(filepath.Join(root, recordsDirectoryName, secondManifest.Reference.String())); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("conflicting record was published: %v", err)
		}
		got, err := os.ReadFile(filepath.Join(root, binDirectoryName, "gh"))
		if err != nil || !reflect.DeepEqual(got, firstShim) {
			t.Fatalf("active shim changed: %v", err)
		}
	})

	for _, fixture := range []struct {
		name  string
		build func(*testing.T, string)
	}{
		{name: "regular", build: func(t *testing.T, path string) { writeFile(t, path, []byte("foreign"), 0o700) }},
		{name: "symlink", build: func(t *testing.T, path string) {
			t.Helper()
			target := filepath.Join(filepath.Dir(path), "foreign-target")
			writeFile(t, target, []byte("foreign"), 0o600)
			if err := os.Symlink(target, path); err != nil {
				t.Fatal(err)
			}
		}},
		{name: "special", build: func(t *testing.T, path string) {
			t.Helper()
			if err := syscall.Mkfifo(path, 0o600); err != nil {
				t.Fatal(err)
			}
		}},
	} {
		t.Run(fixture.name, func(t *testing.T) {
			root := filepath.Join(t.TempDir(), "store")
			prepareEmptyStore(t, root)
			commandPath := filepath.Join(root, binDirectoryName, "gh")
			fixture.build(t, commandPath)
			before, err := os.Lstat(commandPath)
			if err != nil {
				t.Fatal(err)
			}
			manifest, shim := testArtifact(t, "gh", "foreign-"+fixture.name)
			if _, _, err := New(root).Install(context.Background(), manifest, shim); !errors.Is(err, ErrConflict) {
				t.Fatalf("Install() error = %v", err)
			}
			after, err := os.Lstat(commandPath)
			if err != nil || !os.SameFile(before, after) {
				t.Fatalf("foreign target changed: %v", err)
			}
			if _, err := os.Lstat(filepath.Join(root, recordsDirectoryName, manifest.Reference.String())); !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("record was created despite collision: %v", err)
			}
		})
	}
}

func TestStatusSeparatesInactiveOwnedRecordFromForeignCollisionAndRemovePreservesForeign(t *testing.T) {
	root := filepath.Join(t.TempDir(), "store")
	store := New(root)
	manifest, shim := testArtifact(t, "gh", "inactive")
	if _, _, err := store.Install(context.Background(), manifest, shim); err != nil {
		t.Fatal(err)
	}
	commandPath := filepath.Join(root, binDirectoryName, "gh")
	if err := os.Remove(commandPath); err != nil {
		t.Fatal(err)
	}
	foreign := []byte("foreign-command")
	writeFile(t, commandPath, foreign, 0o700)

	inventory, err := store.Status(context.Background())
	want, _ := wrappershim.SortInventory(
		[]wrappershim.Record{ownedSummary(manifest, wrappershim.StateOwnedInactive)},
		[]wrappershim.Record{{CommandName: "gh", State: wrappershim.StateCollisionForeign}},
	)
	if err != nil || !reflect.DeepEqual(inventory, want) {
		t.Fatalf("Status() = %+v, %v; want %+v", inventory, err, want)
	}
	removed, err := store.Remove(context.Background(), manifest.Reference)
	if err != nil || removed.State != wrappershim.StateOwnedInactive {
		t.Fatalf("Remove(inactive) = %+v, %v", removed, err)
	}
	got, err := os.ReadFile(commandPath)
	if err != nil || !reflect.DeepEqual(got, foreign) {
		t.Fatalf("foreign collision was changed: %q, %v", got, err)
	}
}

func TestStatusClassifiesCollisionShapes(t *testing.T) {
	root := filepath.Join(t.TempDir(), "store")
	prepareEmptyStore(t, root)
	writeFile(t, filepath.Join(root, binDirectoryName, "regular"), []byte("foreign"), 0o700)
	target := filepath.Join(root, "target")
	writeFile(t, target, []byte("target"), 0o600)
	if err := os.Symlink(target, filepath.Join(root, binDirectoryName, "linked")); err != nil {
		t.Fatal(err)
	}
	if err := syscall.Mkfifo(filepath.Join(root, binDirectoryName, "special"), 0o600); err != nil {
		t.Fatal(err)
	}
	inventory, err := New(root).Status(context.Background())
	want, _ := wrappershim.SortInventory([]wrappershim.Record{}, []wrappershim.Record{
		{CommandName: "regular", State: wrappershim.StateCollisionForeign},
		{CommandName: "linked", State: wrappershim.StateCollisionSymlink},
		{CommandName: "special", State: wrappershim.StateCollisionSpecial},
	})
	if err != nil || !reflect.DeepEqual(inventory, want) {
		t.Fatalf("Status() = %+v, %v; want %+v", inventory, err, want)
	}
}

func TestTamperedMaterialIsReportedAndNeverRemoved(t *testing.T) {
	root := filepath.Join(t.TempDir(), "store")
	store := New(root)
	manifest, shim := testArtifact(t, "gh", "tampered")
	if _, _, err := store.Install(context.Background(), manifest, shim); err != nil {
		t.Fatal(err)
	}
	shimPath := filepath.Join(root, recordsDirectoryName, manifest.Reference.String(), shimFileName)
	if err := os.WriteFile(shimPath, []byte("#!/bin/sh\nexit 0\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	inventory, err := store.Status(context.Background())
	if err != nil || len(inventory.Records) != 1 || inventory.Records[0].State != wrappershim.StateTampered {
		t.Fatalf("Status(tampered) = %+v, %v", inventory, err)
	}
	if _, err := store.Remove(context.Background(), manifest.Reference); !errors.Is(err, ErrTampered) {
		t.Fatalf("Remove(tampered) error = %v", err)
	}
	if _, err := os.Lstat(shimPath); err != nil {
		t.Fatalf("tampered shim was deleted: %v", err)
	}
}

func TestNonTemplateBytesWithSelfConsistentManifestRemainTampered(t *testing.T) {
	root := filepath.Join(t.TempDir(), "store")
	prepareEmptyStore(t, root)
	valid, _ := testArtifact(t, "gh", "forged")
	forged := []byte("#!/bin/sh\nprintf forged\\n\n")
	digest := fmt.Sprintf("%x", sha256.Sum256(forged))
	reference, err := wrappershim.NewReference(digest)
	if err != nil {
		t.Fatal(err)
	}
	manifest := wrappershim.Manifest{
		ContractVersion: wrappershim.ContractVersion,
		Reference:       reference,
		Binding:         valid.Binding,
		MaterialSHA256:  digest,
		MaterialSize:    int64(len(forged)),
	}
	if err := manifest.Validate(); err != nil {
		t.Fatal(err)
	}
	writeRawRecord(t, root, manifest, forged)
	inventory, err := New(root).Status(context.Background())
	if err != nil || len(inventory.Records) != 1 || inventory.Records[0].State != wrappershim.StateTampered {
		t.Fatalf("Status(forged) = %+v, %v", inventory, err)
	}
	if _, err := New(root).Remove(context.Background(), reference); !errors.Is(err, ErrTampered) {
		t.Fatalf("Remove(forged) error = %v", err)
	}
}

func TestUnknownHardLinkMakesOwnershipTampered(t *testing.T) {
	root := filepath.Join(t.TempDir(), "store")
	store := New(root)
	manifest, shim := testArtifact(t, "gh", "extra-link")
	if _, _, err := store.Install(context.Background(), manifest, shim); err != nil {
		t.Fatal(err)
	}
	shimPath := filepath.Join(root, recordsDirectoryName, manifest.Reference.String(), shimFileName)
	outside := filepath.Join(t.TempDir(), "unknown-link")
	if err := os.Link(shimPath, outside); err != nil {
		t.Fatal(err)
	}
	inventory, err := store.Status(context.Background())
	if err != nil || inventory.Records[0].State != wrappershim.StateTampered {
		t.Fatalf("Status(extra hard link) = %+v, %v", inventory, err)
	}
	if _, err := store.Remove(context.Background(), manifest.Reference); !errors.Is(err, ErrTampered) {
		t.Fatalf("Remove(extra hard link) error = %v", err)
	}
}

func TestUnsafeAndMalformedStoreShapesFailClosed(t *testing.T) {
	manifest, shim := testArtifact(t, "gh", "unsafe")
	tests := []struct {
		name  string
		build func(*testing.T, string)
	}{
		{name: "public root", build: func(t *testing.T, root string) {
			t.Helper()
			if err := os.Mkdir(root, 0o755); err != nil {
				t.Fatal(err)
			}
		}},
		{name: "root symlink", build: func(t *testing.T, root string) {
			t.Helper()
			real := filepath.Join(filepath.Dir(root), "real")
			if err := os.Mkdir(real, 0o700); err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink(real, root); err != nil {
				t.Fatal(err)
			}
		}},
		{name: "bin symlink", build: func(t *testing.T, root string) {
			t.Helper()
			if err := os.Mkdir(root, 0o700); err != nil {
				t.Fatal(err)
			}
			real := filepath.Join(root, "real-bin")
			if err := os.Mkdir(real, 0o700); err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink(real, filepath.Join(root, binDirectoryName)); err != nil {
				t.Fatal(err)
			}
			if err := os.Mkdir(filepath.Join(root, recordsDirectoryName), 0o700); err != nil {
				t.Fatal(err)
			}
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root := filepath.Join(t.TempDir(), "store")
			test.build(t, root)
			if _, _, err := New(root).Install(context.Background(), manifest, shim); !errors.Is(err, ErrUnsafeStore) {
				t.Fatalf("Install() error = %v", err)
			}
		})
	}

	t.Run("noncanonical manifest", func(t *testing.T) {
		root := filepath.Join(t.TempDir(), "store")
		store := New(root)
		if _, _, err := store.Install(context.Background(), manifest, shim); err != nil {
			t.Fatal(err)
		}
		path := filepath.Join(root, recordsDirectoryName, manifest.Reference.String(), manifestFileName)
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, append([]byte(" "), data...), 0o600); err != nil {
			t.Fatal(err)
		}
		if _, err := store.Status(context.Background()); !errors.Is(err, ErrTampered) {
			t.Fatalf("Status(noncanonical manifest) error = %v", err)
		}
		if _, err := store.Remove(context.Background(), manifest.Reference); !errors.Is(err, ErrTampered) {
			t.Fatalf("Remove(noncanonical manifest) error = %v", err)
		}
	})
}

func TestBoundsLockAndInvalidInputCauseNoArtifactMutation(t *testing.T) {
	t.Run("invalid input", func(t *testing.T) {
		root := filepath.Join(t.TempDir(), "store")
		if _, _, err := New(root).Install(context.Background(), wrappershim.Manifest{}, []byte("x")); !errors.Is(err, ErrInvalidInput) {
			t.Fatalf("Install(invalid) error = %v", err)
		}
		if _, err := os.Lstat(root); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("invalid input created root: %v", err)
		}
	})

	t.Run("canceled", func(t *testing.T) {
		root := filepath.Join(t.TempDir(), "store")
		manifest, shim := testArtifact(t, "gh", "canceled")
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		if _, _, err := New(root).Install(ctx, manifest, shim); !errors.Is(err, context.Canceled) {
			t.Fatalf("Install(canceled) error = %v", err)
		}
		if _, err := os.Lstat(root); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("canceled input created root: %v", err)
		}
	})

	t.Run("artifact capacity", func(t *testing.T) {
		root := filepath.Join(t.TempDir(), "store")
		prepareEmptyStore(t, root)
		for index := 0; index < wrappershim.MaxArtifacts; index++ {
			digest := fmt.Sprintf("%064x", index+1)
			reference, err := wrappershim.NewReference(digest)
			if err != nil {
				t.Fatal(err)
			}
			if err := os.Mkdir(filepath.Join(root, recordsDirectoryName, reference.String()), 0o700); err != nil {
				t.Fatal(err)
			}
		}
		manifest, shim := testArtifact(t, "gh", "capacity")
		if _, _, err := New(root).Install(context.Background(), manifest, shim); !errors.Is(err, ErrCapacity) {
			t.Fatalf("Install(capacity) error = %v", err)
		}
		if _, err := os.Lstat(filepath.Join(root, recordsDirectoryName, manifest.Reference.String())); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("capacity failure created record: %v", err)
		}
	})

	t.Run("collision capacity", func(t *testing.T) {
		root := filepath.Join(t.TempDir(), "store")
		prepareEmptyStore(t, root)
		for index := 0; index <= wrappershim.MaxArtifacts; index++ {
			writeFile(t, filepath.Join(root, binDirectoryName, fmt.Sprintf("tool%d", index)), []byte("foreign"), 0o700)
		}
		if _, err := New(root).Status(context.Background()); !errors.Is(err, ErrCapacity) {
			t.Fatalf("Status(collision capacity) error = %v", err)
		}
	})

	t.Run("busy lock", func(t *testing.T) {
		root := filepath.Join(t.TempDir(), "store")
		prepareEmptyStore(t, root)
		lock, err := os.OpenFile(filepath.Join(root, lockFileName), os.O_RDWR, 0)
		if err != nil {
			t.Fatal(err)
		}
		defer lock.Close()
		if err := syscall.Flock(int(lock.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
			t.Fatal(err)
		}
		defer syscall.Flock(int(lock.Fd()), syscall.LOCK_UN)
		if _, err := New(root).Status(context.Background()); !errors.Is(err, ErrConflict) {
			t.Fatalf("Status(busy) error = %v", err)
		}
	})
}

func TestInvalidReferenceAndBinPathArePure(t *testing.T) {
	root := filepath.Join(t.TempDir(), "store")
	store := New(root)
	bin, err := store.BinPath()
	if err != nil || bin != filepath.Join(root, binDirectoryName) {
		t.Fatalf("BinPath() = %q, %v", bin, err)
	}
	if _, err := os.Lstat(root); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("BinPath created state: %v", err)
	}
	if _, err := store.Remove(context.Background(), wrappershim.Reference("not-a-reference")); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("Remove(invalid) error = %v", err)
	}
	for _, invalid := range []*Store{nil, New("relative"), New(root + string(filepath.Separator) + ".." + string(filepath.Separator) + "unclean")} {
		if _, err := invalid.BinPath(); !errors.Is(err, ErrInvalidInput) {
			t.Errorf("invalid BinPath() error = %v", err)
		}
	}
}

func TestDefaultRootIsVersionedAndAbsolute(t *testing.T) {
	root, err := DefaultRoot()
	if err != nil || !filepath.IsAbs(root) || filepath.Clean(root) != root {
		t.Fatalf("DefaultRoot() = %q, %v", root, err)
	}
	wantSuffix := filepath.Join("atsura", "wrapper-shims", "v1")
	if !strings.HasSuffix(root, wantSuffix) {
		t.Fatalf("DefaultRoot() = %q, want suffix %q", root, wantSuffix)
	}
}

func testArtifact(t *testing.T, command, seed string) (wrappershim.Manifest, []byte) {
	t.Helper()
	binding := wrapperbinding.Binding{
		ContractVersion: wrapperbinding.ContractVersion,
		BundleLocator:   filepath.Join("/tmp", "atsura-"+seed+"-bundle.json"),
		BundleDigest:    strings.Repeat("a", 64),
		CommandName:     command,
		Help: wrapperbinding.CompiledHelp{Commands: []wrapperbinding.HelpCommand{{
			Path: []string{"issue", "list"}, Summary: "List issues", Reason: "Keep issue inventory", Options: []wrapperbinding.HelpOption{},
		}}},
		Runtime: wrapperbinding.RuntimeIdentity{
			ResolvedPath: filepath.Join("/opt", "atsura", seed, "atr"),
			SHA256:       strings.Repeat("b", 64),
			Size:         4242,
		},
	}
	material, err := posixshim.Render(binding)
	if err != nil {
		t.Fatalf("Render fixture: %v", err)
	}
	manifest, err := wrappershim.NewManifest(binding, material)
	if err != nil {
		t.Fatalf("NewManifest fixture: %v", err)
	}
	return manifest, append([]byte(nil), material.Source...)
}

func prepareEmptyStore(t *testing.T, root string) {
	t.Helper()
	for _, path := range []string{root, filepath.Join(root, binDirectoryName), filepath.Join(root, recordsDirectoryName)} {
		if err := os.MkdirAll(path, 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.Chmod(path, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	writeFile(t, filepath.Join(root, lockFileName), nil, 0o600)
}

func writeRawRecord(t *testing.T, root string, manifest wrappershim.Manifest, shim []byte) {
	t.Helper()
	directory := filepath.Join(root, recordsDirectoryName, manifest.Reference.String())
	if err := os.Mkdir(directory, 0o700); err != nil {
		t.Fatal(err)
	}
	encoded, err := manifest.CanonicalBytes()
	if err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(directory, manifestFileName), encoded, 0o600)
	writeFile(t, filepath.Join(directory, shimFileName), shim, 0o700)
}

func writeFile(t *testing.T, path string, data []byte, mode os.FileMode) {
	t.Helper()
	if err := os.WriteFile(path, data, mode); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(path, mode); err != nil {
		t.Fatal(err)
	}
}

func assertMode(t *testing.T, path string, want os.FileMode) {
	t.Helper()
	info, err := os.Lstat(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := info.Mode(); got != want {
		t.Fatalf("mode(%q) = %v, want %v", path, got, want)
	}
}

func assertSameFile(t *testing.T, first, second string) {
	t.Helper()
	firstInfo, err := os.Lstat(first)
	if err != nil {
		t.Fatal(err)
	}
	secondInfo, err := os.Lstat(second)
	if err != nil {
		t.Fatal(err)
	}
	if !os.SameFile(firstInfo, secondInfo) {
		t.Fatalf("%q and %q are not the same file", first, second)
	}
}
