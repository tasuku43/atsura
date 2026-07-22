//go:build linux || darwin

package shimstore

import (
	"bytes"
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"syscall"

	"github.com/tasuku43/atsura/internal/domain/wrappershim"
	"github.com/tasuku43/atsura/internal/infra/posixshim"
)

const (
	binDirectoryName     = "bin"
	recordsDirectoryName = "records"
	lockFileName         = ".store.lock"
	manifestFileName     = "manifest.json"
	shimFileName         = "shim"
)

type openedStore struct {
	path        string
	root        *os.Root
	rootInfo    fs.FileInfo
	bin         *os.Root
	binInfo     fs.FileInfo
	records     *os.Root
	recordsInfo fs.FileInfo
}

func (o *openedStore) close() {
	if o == nil {
		return
	}
	if o.records != nil {
		_ = o.records.Close()
	}
	if o.bin != nil {
		_ = o.bin.Close()
	}
	if o.root != nil {
		_ = o.root.Close()
	}
}

type storeLock struct{ file *os.File }

func (l *storeLock) close() {
	if l == nil || l.file == nil {
		return
	}
	_ = syscall.Flock(int(l.file.Fd()), syscall.LOCK_UN)
	_ = l.file.Close()
}

type inspectedRecord struct {
	manifest     wrappershim.Manifest
	directory    fs.FileInfo
	manifestInfo fs.FileInfo
	shimInfo     fs.FileInfo
	shim         []byte
	state        wrappershim.State
}

// Install publishes one immutable record and then one create-exclusive active
// hard link. It never replaces an existing record or command. alreadyInstalled
// is true only when the complete exact active artifact needed no mutation.
func (s *Store) Install(ctx context.Context, manifest wrappershim.Manifest, shim []byte) (wrappershim.Record, bool, error) {
	if err := validateContextStore(ctx, s); err != nil {
		return wrappershim.Record{}, false, err
	}
	if err := validateMaterial(manifest, shim); err != nil {
		return wrappershim.Record{}, false, err
	}

	opened, err := s.openForInstall()
	if err != nil {
		return wrappershim.Record{}, false, err
	}
	defer opened.close()
	lock, err := opened.acquireLock(true)
	if err != nil {
		return wrappershim.Record{}, false, err
	}
	defer lock.close()
	if err := opened.revalidate(); err != nil {
		return wrappershim.Record{}, false, err
	}

	entries, err := readBoundedDirectory(opened.records, wrappershim.MaxArtifacts)
	if err != nil {
		return wrappershim.Record{}, false, err
	}
	referenceName := manifest.Reference.String()
	recordPresent := false
	for _, entry := range entries {
		if _, parseErr := wrappershim.ParseReference(entry.Name()); parseErr != nil {
			return wrappershim.Record{}, false, wrap(ErrTampered, "inspect records", parseErr)
		}
		if entry.Name() == referenceName {
			recordPresent = true
		}
	}

	if recordPresent {
		record, err := opened.inspectRecord(manifest.Reference)
		if err != nil {
			return wrappershim.Record{}, false, err
		}
		if record.state == wrappershim.StateTampered || !record.manifest.Equal(manifest) || !bytes.Equal(record.shim, shim) {
			return wrappershim.Record{}, false, wrap(ErrTampered, "install existing artifact", nil)
		}
		state, _, err := opened.activation(record)
		if err != nil {
			return wrappershim.Record{}, false, err
		}
		result := ownedSummary(record.manifest, state)
		switch state {
		case wrappershim.StateOwnedActive:
			return result, true, nil
		case wrappershim.StateOwnedInactive:
			if err := opened.activate(record); err != nil {
				return wrappershim.Record{}, false, err
			}
			return ownedSummary(record.manifest, wrappershim.StateOwnedActive), false, nil
		default:
			return wrappershim.Record{}, false, wrap(ErrTampered, "install existing artifact", nil)
		}
	}

	if len(entries) >= wrappershim.MaxArtifacts {
		return wrappershim.Record{}, false, wrap(ErrCapacity, "install artifact", nil)
	}
	if _, err := opened.bin.Lstat(manifest.Binding.CommandName); err == nil {
		return wrappershim.Record{}, false, wrap(ErrConflict, "install command", nil)
	} else if !errors.Is(err, fs.ErrNotExist) {
		return wrappershim.Record{}, false, wrap(ErrUnsafeStore, "inspect command", err)
	}

	record, published, err := opened.publishRecord(manifest, shim)
	if err != nil {
		return wrappershim.Record{}, false, err
	}
	if !published {
		return wrappershim.Record{}, false, wrap(ErrUnsafeStore, "publish artifact", nil)
	}
	if err := opened.activate(record); err != nil {
		return wrappershim.Record{}, false, uncertain("activate published artifact", err)
	}
	return ownedSummary(manifest, wrappershim.StateOwnedActive), false, nil
}

// Status performs bounded reconciliation only. An absent root is an empty
// inventory and no directory, lock, or repair state is created.
func (s *Store) Status(ctx context.Context) (wrappershim.Inventory, error) {
	if err := validateContextStore(ctx, s); err != nil {
		return wrappershim.Inventory{}, err
	}
	opened, present, err := s.openExisting()
	if err != nil {
		return wrappershim.Inventory{}, err
	}
	if !present {
		return wrappershim.SortInventory([]wrappershim.Record{}, []wrappershim.Record{})
	}
	defer opened.close()
	lock, err := opened.acquireLock(false)
	if err != nil {
		return wrappershim.Inventory{}, err
	}
	defer lock.close()
	if err := opened.revalidate(); err != nil {
		return wrappershim.Inventory{}, err
	}

	recordEntries, err := readBoundedDirectory(opened.records, wrappershim.MaxArtifacts)
	if err != nil {
		return wrappershim.Inventory{}, err
	}
	records := make([]wrappershim.Record, 0, len(recordEntries))
	active := make(map[string]fs.FileInfo, len(recordEntries))
	for _, entry := range recordEntries {
		reference, parseErr := wrappershim.ParseReference(entry.Name())
		if parseErr != nil {
			return wrappershim.Inventory{}, wrap(ErrTampered, "inspect records", parseErr)
		}
		record, inspectErr := opened.inspectRecord(reference)
		if inspectErr != nil {
			return wrappershim.Inventory{}, wrap(ErrTampered, "inspect artifact inventory", inspectErr)
		}
		state := record.state
		if state != wrappershim.StateTampered {
			state, _, inspectErr = opened.activation(record)
			if inspectErr != nil {
				return wrappershim.Inventory{}, inspectErr
			}
		}
		records = append(records, ownedSummary(record.manifest, state))
		if state == wrappershim.StateOwnedActive {
			active[record.manifest.Binding.CommandName] = record.shimInfo
		}
	}

	binEntries, err := readBoundedDirectory(opened.bin, wrappershim.MaxArtifacts)
	if err != nil {
		return wrappershim.Inventory{}, err
	}
	collisions := make([]wrappershim.Record, 0, len(binEntries))
	for _, entry := range binEntries {
		name := entry.Name()
		if err := validateCommandName(name); err != nil {
			return wrappershim.Inventory{}, wrap(ErrConflict, "inspect command inventory", err)
		}
		info, statErr := opened.bin.Lstat(name)
		if statErr != nil {
			return wrappershim.Inventory{}, wrap(ErrUnsafeStore, "inspect command inventory", statErr)
		}
		if expected, ok := active[name]; ok && safeShimInfo(info) && os.SameFile(expected, info) {
			continue
		}
		collisions = append(collisions, wrappershim.Record{CommandName: name, State: collisionState(info)})
	}
	if err := opened.revalidate(); err != nil {
		return wrappershim.Inventory{}, err
	}
	return wrappershim.SortInventory(records, collisions)
}

// Remove consumes the opaque reference unchanged and deletes only a fully
// revalidated owned record. A different or foreign bin entry is left intact;
// such a record is inactive and may still be removed safely.
func (s *Store) Remove(ctx context.Context, reference wrappershim.Reference) (wrappershim.Record, error) {
	if err := validateContextStore(ctx, s); err != nil {
		return wrappershim.Record{}, err
	}
	if err := reference.Validate(); err != nil {
		return wrappershim.Record{}, wrap(ErrInvalidInput, "remove artifact", err)
	}
	opened, present, err := s.openExisting()
	if err != nil {
		return wrappershim.Record{}, err
	}
	if !present {
		return wrappershim.Record{}, wrap(ErrNotFound, "remove artifact", nil)
	}
	defer opened.close()
	lock, err := opened.acquireLock(false)
	if err != nil {
		return wrappershim.Record{}, err
	}
	defer lock.close()
	if err := opened.revalidate(); err != nil {
		return wrappershim.Record{}, err
	}
	record, err := opened.inspectRecord(reference)
	if errors.Is(err, fs.ErrNotExist) {
		return wrappershim.Record{}, wrap(ErrNotFound, "remove artifact", nil)
	}
	if err != nil {
		return wrappershim.Record{}, err
	}
	if record.state == wrappershim.StateTampered {
		return wrappershim.Record{}, wrap(ErrTampered, "remove artifact", nil)
	}
	state, _, err := opened.activation(record)
	if err != nil {
		return wrappershim.Record{}, err
	}
	if state == wrappershim.StateTampered {
		return wrappershim.Record{}, wrap(ErrTampered, "remove artifact", nil)
	}
	result := ownedSummary(record.manifest, state)
	if err := opened.removeRecord(record, state == wrappershim.StateOwnedActive); err != nil {
		return wrappershim.Record{}, err
	}
	return result, nil
}

func validateContextStore(ctx context.Context, s *Store) error {
	if ctx == nil || !validStore(s) {
		return ErrInvalidInput
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	return nil
}

func validateMaterial(manifest wrappershim.Manifest, shim []byte) error {
	if err := manifest.Validate(); err != nil {
		return wrap(ErrInvalidInput, "validate manifest", err)
	}
	material, err := posixshim.Render(manifest.Binding)
	if err != nil || int64(len(shim)) != manifest.MaterialSize || material.SHA256 != manifest.MaterialSHA256 || !bytes.Equal(material.Source, shim) {
		return wrap(ErrInvalidInput, "validate fixed shim material", err)
	}
	return nil
}

func (s *Store) openForInstall() (*openedStore, error) {
	info, err := os.Lstat(s.root)
	if errors.Is(err, fs.ErrNotExist) {
		if err := os.MkdirAll(s.root, 0o700); err != nil {
			return nil, wrap(ErrUnsafeStore, "create store root", err)
		}
		info, err = os.Lstat(s.root)
	}
	if err != nil || !safeDirectoryInfo(info) {
		return nil, wrap(ErrUnsafeStore, "inspect store root", err)
	}
	root, err := openVerifiedRoot(s.root, info)
	if err != nil {
		return nil, err
	}
	created := false
	for _, name := range []string{binDirectoryName, recordsDirectoryName} {
		child, statErr := root.Lstat(name)
		if errors.Is(statErr, fs.ErrNotExist) {
			if mkdirErr := root.Mkdir(name, 0o700); mkdirErr != nil {
				_ = root.Close()
				return nil, wrap(ErrUnsafeStore, "create store directory", mkdirErr)
			}
			created = true
			child, statErr = root.Lstat(name)
		}
		if statErr != nil || !safeDirectoryInfo(child) {
			_ = root.Close()
			return nil, wrap(ErrUnsafeStore, "inspect store directory", statErr)
		}
	}
	if created {
		if err := syncRoot(root); err != nil {
			_ = root.Close()
			return nil, wrap(ErrUnsafeStore, "sync store root", err)
		}
	}
	return finishOpenStore(s.root, root, info)
}

func (s *Store) openExisting() (*openedStore, bool, error) {
	info, err := os.Lstat(s.root)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, false, nil
	}
	if err != nil || !safeDirectoryInfo(info) {
		return nil, true, wrap(ErrUnsafeStore, "inspect store root", err)
	}
	root, err := openVerifiedRoot(s.root, info)
	if err != nil {
		return nil, true, err
	}
	opened, err := finishOpenStore(s.root, root, info)
	if err != nil {
		_ = root.Close()
		return nil, true, err
	}
	return opened, true, nil
}

func openVerifiedRoot(path string, expected fs.FileInfo) (*os.Root, error) {
	root, err := os.OpenRoot(path)
	if err != nil {
		return nil, wrap(ErrUnsafeStore, "open store root", err)
	}
	opened, statErr := root.Stat(".")
	if statErr != nil || !safeDirectoryInfo(opened) || !os.SameFile(expected, opened) {
		_ = root.Close()
		return nil, wrap(ErrUnsafeStore, "verify store root", statErr)
	}
	return root, nil
}

func finishOpenStore(path string, root *os.Root, rootInfo fs.FileInfo) (*openedStore, error) {
	bin, binInfo, err := openChildRoot(root, binDirectoryName)
	if err != nil {
		return nil, err
	}
	records, recordsInfo, err := openChildRoot(root, recordsDirectoryName)
	if err != nil {
		_ = bin.Close()
		return nil, err
	}
	opened := &openedStore{path: path, root: root, rootInfo: rootInfo, bin: bin, binInfo: binInfo, records: records, recordsInfo: recordsInfo}
	if err := opened.revalidate(); err != nil {
		opened.close()
		return nil, err
	}
	return opened, nil
}

func openChildRoot(root *os.Root, name string) (*os.Root, fs.FileInfo, error) {
	info, err := root.Lstat(name)
	if err != nil || !safeDirectoryInfo(info) {
		return nil, nil, wrap(ErrUnsafeStore, "inspect store directory", err)
	}
	child, err := root.OpenRoot(name)
	if err != nil {
		return nil, nil, wrap(ErrUnsafeStore, "open store directory", err)
	}
	opened, statErr := child.Stat(".")
	if statErr != nil || !safeDirectoryInfo(opened) || !os.SameFile(info, opened) {
		_ = child.Close()
		return nil, nil, wrap(ErrUnsafeStore, "verify store directory", statErr)
	}
	return child, info, nil
}

func (o *openedStore) revalidate() error {
	rootInfo, err := os.Lstat(o.path)
	if err != nil || !safeDirectoryInfo(rootInfo) || !os.SameFile(o.rootInfo, rootInfo) {
		return wrap(ErrUnsafeStore, "revalidate store root", err)
	}
	opened, statErr := o.root.Stat(".")
	if statErr != nil || !safeDirectoryInfo(opened) || !os.SameFile(o.rootInfo, opened) {
		return wrap(ErrUnsafeStore, "revalidate opened root", statErr)
	}
	for _, child := range []struct {
		name string
		info fs.FileInfo
		root *os.Root
	}{{binDirectoryName, o.binInfo, o.bin}, {recordsDirectoryName, o.recordsInfo, o.records}} {
		current, currentErr := o.root.Lstat(child.name)
		pinned, pinnedErr := child.root.Stat(".")
		if currentErr != nil || pinnedErr != nil || !safeDirectoryInfo(current) || !safeDirectoryInfo(pinned) || !os.SameFile(child.info, current) || !os.SameFile(child.info, pinned) {
			return wrap(ErrUnsafeStore, "revalidate store directory", errors.Join(currentErr, pinnedErr))
		}
	}
	return nil
}

func (o *openedStore) acquireLock(create bool) (*storeLock, error) {
	info, err := o.root.Lstat(lockFileName)
	if errors.Is(err, fs.ErrNotExist) && create {
		file, createErr := o.root.OpenFile(lockFileName, os.O_RDWR|os.O_CREATE|os.O_EXCL, 0o600)
		if createErr != nil {
			return nil, wrap(ErrConflict, "create store lock", createErr)
		}
		if closeErr := file.Close(); closeErr != nil {
			return nil, wrap(ErrUnsafeStore, "close store lock", closeErr)
		}
		if syncErr := syncRoot(o.root); syncErr != nil {
			return nil, wrap(ErrUnsafeStore, "sync store lock", syncErr)
		}
		info, err = o.root.Lstat(lockFileName)
	}
	if err != nil || !safePrivateFileInfo(info, 0o600) {
		return nil, wrap(ErrUnsafeStore, "inspect store lock", err)
	}
	file, err := o.root.OpenFile(lockFileName, os.O_RDWR, 0)
	if err != nil {
		return nil, wrap(ErrUnsafeStore, "open store lock", err)
	}
	opened, statErr := file.Stat()
	current, currentErr := o.root.Lstat(lockFileName)
	if statErr != nil || currentErr != nil || !safePrivateFileInfo(opened, 0o600) || !safePrivateFileInfo(current, 0o600) || !os.SameFile(info, opened) || !os.SameFile(opened, current) {
		_ = file.Close()
		return nil, wrap(ErrUnsafeStore, "verify store lock", errors.Join(statErr, currentErr))
	}
	if err := syscall.Flock(int(file.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		_ = file.Close()
		return nil, wrap(ErrConflict, "acquire store lock", err)
	}
	return &storeLock{file: file}, nil
}

func (o *openedStore) inspectRecord(reference wrappershim.Reference) (inspectedRecord, error) {
	name := reference.String()
	directoryInfo, err := o.records.Lstat(name)
	if errors.Is(err, fs.ErrNotExist) {
		return inspectedRecord{}, fs.ErrNotExist
	}
	if err != nil || !safeDirectoryInfo(directoryInfo) {
		return inspectedRecord{}, wrap(ErrTampered, "inspect artifact directory", err)
	}
	recordRoot, err := o.records.OpenRoot(name)
	if err != nil {
		return inspectedRecord{}, wrap(ErrTampered, "open artifact directory", err)
	}
	defer recordRoot.Close()
	openedDirectory, statErr := recordRoot.Stat(".")
	if statErr != nil || !safeDirectoryInfo(openedDirectory) || !os.SameFile(directoryInfo, openedDirectory) {
		return inspectedRecord{}, wrap(ErrTampered, "verify artifact directory", statErr)
	}
	entries, err := readBoundedDirectory(recordRoot, 2)
	if err != nil || len(entries) != 2 || !hasExactRecordEntries(entries) {
		return inspectedRecord{}, wrap(ErrTampered, "inspect artifact entries", err)
	}

	manifestInfo, manifestBytes, err := readPrivateFile(recordRoot, manifestFileName, 0o600, wrappershim.MaxManifestBytes)
	if err != nil {
		return inspectedRecord{}, wrap(ErrTampered, "read artifact manifest", err)
	}
	manifest, err := wrappershim.DecodeManifest(manifestBytes)
	if err != nil || manifest.Reference != reference {
		return inspectedRecord{}, wrap(ErrTampered, "decode artifact manifest", err)
	}
	record := inspectedRecord{manifest: manifest, directory: directoryInfo, manifestInfo: manifestInfo, state: wrappershim.StateTampered}
	shimInfo, shimBytes, err := readPrivateFile(recordRoot, shimFileName, 0o700, wrappershim.MaxShimBytes)
	if err != nil {
		return record, nil
	}
	record.shimInfo = shimInfo
	record.shim = shimBytes
	if err := validateMaterial(manifest, shimBytes); err != nil || int64(shimInfo.Size()) != manifest.MaterialSize {
		return record, nil
	}
	if current, currentErr := o.records.Lstat(name); currentErr != nil || !os.SameFile(directoryInfo, current) {
		return inspectedRecord{}, wrap(ErrTampered, "revalidate artifact directory", currentErr)
	}
	record.state = wrappershim.StateOwnedInactive
	return record, nil
}

func (o *openedStore) activation(record inspectedRecord) (wrappershim.State, *wrappershim.Record, error) {
	if record.state == wrappershim.StateTampered || record.shimInfo == nil {
		return wrappershim.StateTampered, nil, nil
	}
	links, ok := regularLinkCount(record.shimInfo)
	if !ok {
		return wrappershim.StateTampered, nil, nil
	}
	name := record.manifest.Binding.CommandName
	info, err := o.bin.Lstat(name)
	if errors.Is(err, fs.ErrNotExist) {
		if links != 1 {
			return wrappershim.StateTampered, nil, nil
		}
		return wrappershim.StateOwnedInactive, nil, nil
	}
	if err != nil {
		return "", nil, wrap(ErrUnsafeStore, "inspect active command", err)
	}
	if safeShimInfo(info) && os.SameFile(record.shimInfo, info) {
		if links != 2 {
			return wrappershim.StateTampered, nil, nil
		}
		return wrappershim.StateOwnedActive, nil, nil
	}
	if links != 1 {
		return wrappershim.StateTampered, nil, nil
	}
	collision := wrappershim.Record{CommandName: name, State: collisionState(info)}
	return wrappershim.StateOwnedInactive, &collision, nil
}

func (o *openedStore) publishRecord(manifest wrappershim.Manifest, shim []byte) (inspectedRecord, bool, error) {
	manifestBytes, err := manifest.CanonicalBytes()
	if err != nil {
		return inspectedRecord{}, false, wrap(ErrInvalidInput, "encode artifact manifest", err)
	}
	stageName, stageRoot, stageInfo, err := createStagingDirectory(o.records)
	if err != nil {
		return inspectedRecord{}, false, wrap(ErrUnsafeStore, "create artifact staging", err)
	}
	published := false
	defer func() {
		_ = stageRoot.Close()
		if !published {
			_ = o.records.RemoveAll(stageName)
		}
	}()
	manifestInfo, err := writePrivateFile(stageRoot, manifestFileName, manifestBytes, 0o600)
	if err != nil {
		return inspectedRecord{}, false, wrap(ErrUnsafeStore, "write artifact manifest", err)
	}
	shimInfo, err := writePrivateFile(stageRoot, shimFileName, shim, 0o700)
	if err != nil {
		return inspectedRecord{}, false, wrap(ErrUnsafeStore, "write artifact shim", err)
	}
	if err := syncRoot(stageRoot); err != nil {
		return inspectedRecord{}, false, wrap(ErrUnsafeStore, "sync artifact staging", err)
	}
	currentStage, statErr := o.records.Lstat(stageName)
	if statErr != nil || !safeDirectoryInfo(currentStage) || !os.SameFile(stageInfo, currentStage) {
		return inspectedRecord{}, false, wrap(ErrUnsafeStore, "verify artifact staging", statErr)
	}
	if err := o.revalidate(); err != nil {
		return inspectedRecord{}, false, err
	}
	name := manifest.Reference.String()
	if _, err := o.records.Lstat(name); err == nil {
		return inspectedRecord{}, false, wrap(ErrConflict, "publish artifact", nil)
	} else if !errors.Is(err, fs.ErrNotExist) {
		return inspectedRecord{}, false, wrap(ErrUnsafeStore, "inspect artifact target", err)
	}
	if err := o.records.Rename(stageName, name); err != nil {
		return inspectedRecord{}, false, wrap(ErrConflict, "publish artifact", err)
	}
	published = true
	current, statErr := o.records.Lstat(name)
	if statErr != nil || !safeDirectoryInfo(current) || !os.SameFile(stageInfo, current) {
		return inspectedRecord{}, true, uncertain("verify published artifact", statErr)
	}
	if err := syncRoot(o.records); err != nil {
		return inspectedRecord{}, true, uncertain("sync published artifact", err)
	}
	if err := o.revalidate(); err != nil {
		return inspectedRecord{}, true, uncertain("revalidate published artifact", err)
	}
	return inspectedRecord{manifest: manifest.Clone(), directory: stageInfo, manifestInfo: manifestInfo, shimInfo: shimInfo, shim: append([]byte(nil), shim...), state: wrappershim.StateOwnedInactive}, true, nil
}

func (o *openedStore) activate(record inspectedRecord) error {
	if err := o.revalidate(); err != nil {
		return err
	}
	recordRoot, err := o.openVerifiedRecord(record)
	if err != nil {
		return err
	}
	defer recordRoot.Close()
	links, linksOK := regularLinkCount(record.shimInfo)
	if !linksOK || links != 1 {
		return wrap(ErrTampered, "revalidate inactive artifact", nil)
	}
	name := record.manifest.Binding.CommandName
	if _, err := o.bin.Lstat(name); err == nil {
		return wrap(ErrConflict, "activate command", nil)
	} else if !errors.Is(err, fs.ErrNotExist) {
		return wrap(ErrUnsafeStore, "inspect command target", err)
	}
	oldName := filepath.Join(recordsDirectoryName, record.manifest.Reference.String(), shimFileName)
	newName := filepath.Join(binDirectoryName, name)
	if err := o.root.Link(oldName, newName); err != nil {
		return wrap(ErrConflict, "activate command", err)
	}
	info, statErr := o.bin.Lstat(name)
	links, linksOK = regularLinkCount(info)
	if statErr != nil || !safeShimInfo(info) || !os.SameFile(record.shimInfo, info) || !linksOK || links != 2 {
		return uncertain("verify active command", statErr)
	}
	if err := syncRoot(o.bin); err != nil {
		return uncertain("sync active command", err)
	}
	if err := o.revalidate(); err != nil {
		return uncertain("revalidate active command", err)
	}
	return nil
}

func (o *openedStore) removeRecord(record inspectedRecord, active bool) error {
	if err := o.revalidate(); err != nil {
		return err
	}
	referenceName := record.manifest.Reference.String()
	currentDirectory, dirErr := o.records.Lstat(referenceName)
	if dirErr != nil || !os.SameFile(record.directory, currentDirectory) {
		return wrap(ErrTampered, "revalidate artifact before removal", dirErr)
	}
	recordRoot, err := o.openVerifiedRecord(record)
	if err != nil {
		return err
	}
	defer recordRoot.Close()
	wantLinks := uint64(1)
	if active {
		wantLinks = 2
	}
	links, linksOK := regularLinkCount(record.shimInfo)
	if !linksOK || links != wantLinks {
		return wrap(ErrTampered, "revalidate artifact ownership before removal", nil)
	}
	if active {
		current, err := o.bin.Lstat(record.manifest.Binding.CommandName)
		if err != nil || !safeShimInfo(current) || !os.SameFile(record.shimInfo, current) {
			return wrap(ErrTampered, "revalidate active command before removal", err)
		}
		if err := o.bin.Remove(record.manifest.Binding.CommandName); err != nil {
			return wrap(ErrUnsafeStore, "remove active command", err)
		}
		if _, err := o.bin.Lstat(record.manifest.Binding.CommandName); !errors.Is(err, fs.ErrNotExist) {
			return uncertain("verify active command removal", err)
		}
		if err := syncRoot(o.bin); err != nil {
			return uncertain("sync active command removal", err)
		}
	}

	changed := active
	remove := func(name string, expected fs.FileInfo) error {
		current, statErr := recordRoot.Lstat(name)
		if statErr != nil || !os.SameFile(expected, current) {
			return wrap(ErrTampered, "revalidate artifact file before removal", statErr)
		}
		if err := recordRoot.Remove(name); err != nil {
			return err
		}
		changed = true
		return nil
	}
	if err := remove(shimFileName, record.shimInfo); err != nil {
		if changed {
			return uncertain("remove artifact shim", err)
		}
		return wrap(ErrUnsafeStore, "remove artifact shim", err)
	}
	if err := remove(manifestFileName, record.manifestInfo); err != nil {
		return uncertain("remove artifact manifest", err)
	}
	if err := syncRoot(recordRoot); err != nil {
		return uncertain("sync artifact removal", err)
	}
	if err := recordRoot.Close(); err != nil {
		return uncertain("close artifact removal", err)
	}
	if err := o.records.Remove(referenceName); err != nil {
		return uncertain("remove artifact directory", err)
	}
	if err := syncRoot(o.records); err != nil {
		return uncertain("sync artifact directory removal", err)
	}
	if err := o.revalidate(); err != nil {
		return uncertain("revalidate artifact removal", err)
	}
	return nil
}

func (o *openedStore) openVerifiedRecord(record inspectedRecord) (*os.Root, error) {
	referenceName := record.manifest.Reference.String()
	directory, err := o.records.Lstat(referenceName)
	if err != nil || !safeDirectoryInfo(directory) || !os.SameFile(record.directory, directory) {
		return nil, wrap(ErrTampered, "revalidate artifact directory", err)
	}
	recordRoot, err := o.records.OpenRoot(referenceName)
	if err != nil {
		return nil, wrap(ErrTampered, "open artifact directory", err)
	}
	openedDirectory, statErr := recordRoot.Stat(".")
	manifestInfo, manifestErr := recordRoot.Lstat(manifestFileName)
	shimInfo, shimErr := recordRoot.Lstat(shimFileName)
	if statErr != nil || manifestErr != nil || shimErr != nil ||
		!safeDirectoryInfo(openedDirectory) || !os.SameFile(record.directory, openedDirectory) ||
		!safePrivateFileInfo(manifestInfo, 0o600) || !os.SameFile(record.manifestInfo, manifestInfo) ||
		!safeShimInfo(shimInfo) || !os.SameFile(record.shimInfo, shimInfo) {
		_ = recordRoot.Close()
		return nil, wrap(ErrTampered, "verify artifact files", errors.Join(statErr, manifestErr, shimErr))
	}
	return recordRoot, nil
}

func createStagingDirectory(root *os.Root) (string, *os.Root, fs.FileInfo, error) {
	for attempt := 0; attempt < 100; attempt++ {
		var random [16]byte
		if _, err := rand.Read(random[:]); err != nil {
			return "", nil, nil, err
		}
		name := fmt.Sprintf(".stage-%x", random[:])
		if err := root.Mkdir(name, 0o700); errors.Is(err, fs.ErrExist) {
			continue
		} else if err != nil {
			return "", nil, nil, err
		}
		info, err := root.Lstat(name)
		if err != nil || !safeDirectoryInfo(info) {
			_ = root.Remove(name)
			return "", nil, nil, err
		}
		child, err := root.OpenRoot(name)
		if err != nil {
			_ = root.Remove(name)
			return "", nil, nil, err
		}
		opened, statErr := child.Stat(".")
		if statErr != nil || !safeDirectoryInfo(opened) || !os.SameFile(info, opened) {
			_ = child.Close()
			_ = root.Remove(name)
			return "", nil, nil, errors.Join(statErr, ErrUnsafeStore)
		}
		return name, child, info, nil
	}
	return "", nil, nil, fmt.Errorf("could not allocate a unique wrapper shim staging directory")
}

func writePrivateFile(root *os.Root, name string, data []byte, mode fs.FileMode) (fs.FileInfo, error) {
	file, err := root.OpenFile(name, os.O_WRONLY|os.O_CREATE|os.O_EXCL, mode)
	if err != nil {
		return nil, err
	}
	closed := false
	defer func() {
		if !closed {
			_ = file.Close()
		}
	}()
	if err := file.Chmod(mode); err != nil {
		return nil, err
	}
	written, err := file.Write(data)
	if err != nil {
		return nil, err
	}
	if written != len(data) {
		return nil, io.ErrShortWrite
	}
	if err := file.Sync(); err != nil {
		return nil, err
	}
	info, statErr := file.Stat()
	current, currentErr := root.Lstat(name)
	if statErr != nil || currentErr != nil || !safePrivateFileInfo(info, mode) || !safePrivateFileInfo(current, mode) || !os.SameFile(info, current) || info.Size() != int64(len(data)) {
		return nil, errors.Join(statErr, currentErr, ErrUnsafeStore)
	}
	if err := file.Close(); err != nil {
		return nil, err
	}
	closed = true
	return info, nil
}

func readPrivateFile(root *os.Root, name string, mode fs.FileMode, limit int) (fs.FileInfo, []byte, error) {
	info, err := root.Lstat(name)
	if err != nil || !safePrivateFileInfo(info, mode) {
		return nil, nil, errors.Join(err, ErrUnsafeStore)
	}
	file, err := root.Open(name)
	if err != nil {
		return nil, nil, err
	}
	defer file.Close()
	opened, statErr := file.Stat()
	current, currentErr := root.Lstat(name)
	if statErr != nil || currentErr != nil || !safePrivateFileInfo(opened, mode) || !safePrivateFileInfo(current, mode) || !os.SameFile(info, opened) || !os.SameFile(opened, current) {
		return nil, nil, errors.Join(statErr, currentErr, ErrUnsafeStore)
	}
	data, err := io.ReadAll(io.LimitReader(file, int64(limit)+1))
	if err != nil || len(data) > limit || int64(len(data)) != opened.Size() {
		return nil, nil, errors.Join(err, ErrUnsafeStore)
	}
	return opened, data, nil
}

func readBoundedDirectory(root *os.Root, limit int) ([]fs.DirEntry, error) {
	directory, err := root.Open(".")
	if err != nil {
		return nil, wrap(ErrUnsafeStore, "open bounded directory", err)
	}
	defer directory.Close()
	entries, err := directory.ReadDir(limit + 1)
	if err != nil && !errors.Is(err, io.EOF) {
		return nil, wrap(ErrUnsafeStore, "read bounded directory", err)
	}
	if len(entries) > limit {
		return nil, wrap(ErrCapacity, "read bounded directory", nil)
	}
	return entries, nil
}

func hasExactRecordEntries(entries []fs.DirEntry) bool {
	seenManifest := false
	seenShim := false
	for _, entry := range entries {
		switch entry.Name() {
		case manifestFileName:
			seenManifest = true
		case shimFileName:
			seenShim = true
		default:
			return false
		}
	}
	return seenManifest && seenShim
}

func safeDirectoryInfo(info fs.FileInfo) bool {
	return info != nil && info.Mode()&os.ModeSymlink == 0 && info.IsDir() && info.Mode().Perm() == 0o700
}

func safePrivateFileInfo(info fs.FileInfo, mode fs.FileMode) bool {
	return info != nil && info.Mode()&os.ModeSymlink == 0 && info.Mode().IsRegular() && info.Mode().Perm() == mode.Perm()
}

func safeShimInfo(info fs.FileInfo) bool { return safePrivateFileInfo(info, 0o700) }

func regularLinkCount(info fs.FileInfo) (uint64, bool) {
	if !safeShimInfo(info) {
		return 0, false
	}
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok || stat == nil {
		return 0, false
	}
	return uint64(stat.Nlink), true
}

func syncRoot(root *os.Root) error {
	directory, err := root.Open(".")
	if err != nil {
		return err
	}
	defer directory.Close()
	return directory.Sync()
}

func collisionState(info fs.FileInfo) wrappershim.State {
	if info != nil && info.Mode()&os.ModeSymlink != 0 {
		return wrappershim.StateCollisionSymlink
	}
	if info != nil && info.Mode().IsRegular() {
		return wrappershim.StateCollisionForeign
	}
	return wrappershim.StateCollisionSpecial
}

func validateCommandName(name string) error {
	record := wrappershim.Record{CommandName: name, State: wrappershim.StateCollisionForeign}
	return record.Validate()
}

func ownedSummary(manifest wrappershim.Manifest, state wrappershim.State) wrappershim.Record {
	return wrappershim.Record{
		CommandName:    manifest.Binding.CommandName,
		State:          state,
		Reference:      manifest.Reference,
		MaterialSHA256: manifest.MaterialSHA256,
	}
}

func wrap(kind error, operation string, cause error) error {
	if cause == nil {
		return fmt.Errorf("%w: %s", kind, operation)
	}
	return fmt.Errorf("%w: %s: %v", kind, operation, cause)
}

func uncertain(operation string, cause error) error { return wrap(ErrUncertain, operation, cause) }
