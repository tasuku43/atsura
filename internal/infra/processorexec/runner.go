// Package processorexec executes one exact external output processor with
// bounded stdin/stdout/stderr and a fresh isolated environment.
package processorexec

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"

	"github.com/tasuku43/atsura/internal/domain/fault"
	"github.com/tasuku43/atsura/internal/domain/processorprocess"
)

const (
	ownerMarkerName    = ".atsura-processor-owner"
	ownerMarkerContent = "atsura.processor.root.v1\n"
	maxCleanupEntries  = 4096
)

// Runner is the production processor boundary. Its private seams exist only
// so package tests can force identity, wait, setup, and cleanup outcomes.
type Runner struct {
	beforeStart func(string)
	afterStart  func(string)
	wait        func(*exec.Cmd, error) error
	createRoot  func() (string, error)
	removeRoot  func(string) error
	prepared    func(isolation)
}

// New creates an identity-bound isolated processor runner.
func New() *Runner { return &Runner{} }

// Identify fingerprints one explicit absolute processor path without PATH
// lookup or process execution.
func (r *Runner) Identify(ctx context.Context, executable string) (processorprocess.Identity, error) {
	if ctx == nil {
		return processorprocess.Identity{}, fmt.Errorf("processor identity context is nil")
	}
	if err := ctx.Err(); err != nil {
		return processorprocess.Identity{}, err
	}
	if err := processorprocess.ValidateExecutablePath(executable); err != nil {
		return processorprocess.Identity{}, fault.Wrap(fault.KindInvalidInput, "invalid_processor_executable", "The processor executable path must be absolute and clean.", false, err, helpAction())
	}
	identity, err := identifyExecutable(executable)
	if err != nil {
		return processorprocess.Identity{}, err
	}
	if err := ctx.Err(); err != nil {
		return processorprocess.Identity{}, err
	}
	return identity, nil
}

// Run starts at most one processor process after exact identity and isolation
// preflight. It never resolves the executable through PATH or a shell.
func (r *Runner) Run(ctx context.Context, request processorprocess.Request) (processorprocess.Result, error) {
	zero := processorprocess.Result{ExitCode: -1}
	if ctx == nil {
		return zero, fmt.Errorf("processor process context is nil")
	}
	if err := ctx.Err(); err != nil {
		return zero, err
	}
	if err := request.Validate(); err != nil {
		return zero, fault.Wrap(fault.KindContract, "invalid_processor_process_request", "The processor process request is invalid.", false, err, helpAction())
	}

	identity, err := identifyExecutable(request.Executable)
	if err != nil {
		return zero, err
	}
	if identity != request.ExpectedIdentity {
		return zero, fault.New(fault.KindRejected, "processor_identity_changed", "The processor executable does not match the observed identity.", false, helpAction())
	}

	isolated, setupErr := r.prepareIsolation()
	if setupErr != nil {
		return zero, fault.Wrap(fault.KindUnavailable, "processor_environment_setup_failed", "The isolated processor environment could not be prepared.", true, setupErr, helpAction())
	}
	result, processErr := r.runPrepared(ctx, request, isolated)
	if cleanupErr := r.cleanupIsolation(isolated); cleanupErr != nil {
		return result, fault.Wrap(fault.KindUnavailable, "processor_cleanup_failed", "The isolated processor environment could not be removed completely.", false, cleanupErr, helpAction())
	}
	return result, processErr
}

func (r *Runner) runPrepared(ctx context.Context, request processorprocess.Request, isolated isolation) (processorprocess.Result, error) {
	zero := processorprocess.Result{ExitCode: -1}
	if r != nil && r.beforeStart != nil {
		r.beforeStart(request.Executable)
	}
	revalidated, err := identifyExecutable(request.Executable)
	if err != nil || revalidated != request.ExpectedIdentity {
		return zero, fault.Wrap(fault.KindRejected, "processor_identity_changed", "The processor executable changed before it could be started.", false, err, helpAction())
	}

	runCtx, cancel := context.WithTimeout(ctx, request.Timeout)
	defer cancel()
	stdout := &limitedBuffer{limit: request.StdoutLimit, cancel: cancel}
	stderr := &limitedBuffer{limit: request.StderrLimit, cancel: cancel}
	// #nosec G204 -- Request validates an exact absolute executable, fixed argv
	// vector, expected identity, and isolated environment; no shell is involved.
	command := exec.CommandContext(runCtx, request.Executable, request.Args...)
	command.Dir = isolated.work
	command.Env = append([]string(nil), isolated.environment...)
	if request.Input != nil {
		command.Stdin = bytes.NewReader(request.Input)
	}
	command.Stdout = stdout
	command.Stderr = stderr
	command.WaitDelay = 2 * time.Second
	if err := command.Start(); err != nil {
		return zero, fault.Wrap(fault.KindUnavailable, "processor_process_start_failed", "The processor process could not be started.", true, err, helpAction())
	}

	result := processorprocess.Result{Attempts: 1, ExitCode: -1, Identity: request.ExpectedIdentity}
	if r != nil && r.afterStart != nil {
		r.afterStart(request.Executable)
	}
	waitErr := command.Wait()
	if r != nil && r.wait != nil {
		waitErr = r.wait(command, waitErr)
	}
	result.Stdout = stdout.Bytes()
	result.Stderr = stderr.Bytes()
	if command.ProcessState != nil {
		result.ExitCode = command.ProcessState.ExitCode()
	}

	postIdentity, identityErr := identifyExecutable(request.Executable)
	if identityErr != nil || postIdentity != request.ExpectedIdentity {
		return result, fault.Wrap(fault.KindRejected, "processor_identity_changed", "The processor executable changed during execution.", false, identityErr, helpAction())
	}
	if stdout.exceeded {
		return result, fault.New(fault.KindContract, "processor_stdout_too_large", "The processor stdout exceeded the 4 MiB limit.", false, helpAction())
	}
	if stderr.exceeded {
		return result, fault.New(fault.KindContract, "processor_stderr_too_large", "The processor stderr exceeded the 64 KiB limit.", false, helpAction())
	}
	if err := ctx.Err(); err != nil {
		return result, fault.Wrap(fault.KindCanceled, "processor_execution_canceled", "The caller canceled after the processor started; the outcome is not replay-safe.", false, err, helpAction())
	}
	if errors.Is(runCtx.Err(), context.DeadlineExceeded) {
		return result, fault.Wrap(fault.KindUnavailable, "processor_timeout", "The processor exceeded its declared timeout.", false, waitErr, helpAction())
	}
	if waitErr != nil {
		var exitError *exec.ExitError
		if errors.As(waitErr, &exitError) {
			return result, fault.Wrap(fault.KindRejected, "processor_command_failed", "The processor exited without a successful result.", false, waitErr, helpAction())
		}
		return result, fault.Wrap(fault.KindUnavailable, "processor_process_wait_failed", "The processor result could not be collected.", false, waitErr, helpAction())
	}
	return result, nil
}

type isolation struct {
	root         string
	work         string
	environment  []string
	rootInfo     os.FileInfo
	markerInfo   os.FileInfo
	rootHandle   *os.Root
	markerHandle *os.File
}

func (r *Runner) prepareIsolation() (isolation, error) {
	create := func() (string, error) { return os.MkdirTemp("", "atsura-processor-") }
	if r != nil && r.createRoot != nil {
		create = r.createRoot
	}
	root, err := create()
	if err != nil {
		return isolation{}, err
	}
	if !validIsolationRootPath(root) {
		return isolation{}, fmt.Errorf("isolated root must be an absolute clean purpose-specific path")
	}
	info, err := os.Lstat(root)
	if err != nil {
		return isolation{}, fmt.Errorf("isolated root is not a stable directory: %w", err)
	}
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return isolation{}, fmt.Errorf("isolated root is not a stable directory")
	}
	entries, err := os.ReadDir(root)
	if err != nil {
		return isolation{}, fmt.Errorf("isolated root must begin as an empty directory: %w", err)
	}
	if len(entries) != 0 {
		return isolation{}, fmt.Errorf("isolated root must begin as an empty directory")
	}
	// #nosec G302 -- a directory requires owner execute permission; 0700 is the
	// least-privilege usable mode and the restriction is covered by a mode test.
	if err := os.Chmod(root, 0o700); err != nil {
		return isolation{}, err
	}
	parent, name := filepath.Split(root)
	parentHandle, err := os.OpenRoot(parent)
	if err != nil {
		return isolation{}, errors.Join(err, removeUnmarkedRoot(root, info, nil))
	}
	rootHandle, openRootErr := parentHandle.OpenRoot(name)
	closeParentErr := parentHandle.Close()
	if openRootErr != nil || closeParentErr != nil {
		cause := errors.Join(openRootErr, closeParentErr)
		return isolation{}, errors.Join(cause, removeUnmarkedRoot(root, info, rootHandle))
	}
	pinnedRootInfo, err := rootHandle.Stat(".")
	if err != nil || !pinnedRootInfo.IsDir() || !os.SameFile(info, pinnedRootInfo) {
		cause := errors.Join(err, fmt.Errorf("isolated root changed while it was pinned"))
		return isolation{}, errors.Join(cause, removeUnmarkedRoot(root, info, rootHandle))
	}
	markerHandle, markerInfo, err := createOwnerMarker(rootHandle)
	if err != nil {
		cleanupErr := removeUnmarkedRoot(root, pinnedRootInfo, rootHandle)
		if cleanupErr != nil {
			err = errors.Join(err, fmt.Errorf("remove unmarked isolation root: %w", cleanupErr))
		}
		return isolation{}, err
	}
	isolated := isolation{
		root:         root,
		rootInfo:     pinnedRootInfo,
		markerInfo:   markerInfo,
		rootHandle:   rootHandle,
		markerHandle: markerHandle,
	}

	directories := []string{"work", "home", "tmp", "state", "config", "data", "cache", "appdata", "localappdata"}
	paths := make(map[string]string, len(directories))
	for _, name := range directories {
		path := filepath.Join(root, name)
		if err := rootHandle.Mkdir(name, 0o700); err != nil {
			return isolation{}, r.setupFailure(isolated, err)
		}
		paths[name] = path
	}
	environment := []string{
		"APPDATA=" + paths["appdata"],
		"HOME=" + paths["home"],
		"LANG=C",
		"LC_ALL=C",
		"LOCALAPPDATA=" + paths["localappdata"],
		"NO_COLOR=1",
		"RTK_DB_PATH=" + filepath.Join(paths["state"], "rtk.db"),
		"RTK_NO_TOML=1",
		"RTK_TEE=0",
		"RTK_TELEMETRY_DISABLED=1",
		"TEMP=" + paths["tmp"],
		"TMP=" + paths["tmp"],
		"TMPDIR=" + paths["tmp"],
		"TZ=UTC",
		"USERPROFILE=" + paths["home"],
		"XDG_CACHE_HOME=" + paths["cache"],
		"XDG_CONFIG_HOME=" + paths["config"],
		"XDG_DATA_HOME=" + paths["data"],
		"XDG_STATE_HOME=" + paths["state"],
	}
	if runtime.GOOS == "windows" {
		for _, key := range []string{"SystemRoot", "WINDIR"} {
			if value := os.Getenv(key); value != "" && !strings.ContainsRune(value, '\x00') {
				environment = append(environment, key+"="+value)
			}
		}
	}
	sort.Strings(environment)
	isolated.work = paths["work"]
	isolated.environment = environment
	if r != nil && r.prepared != nil {
		r.prepared(isolated)
	}
	return isolated, nil
}

func (r *Runner) cleanupIsolation(isolated isolation) (cleanupErr error) {
	defer func() {
		cleanupErr = errors.Join(cleanupErr, closeIsolationHandles(isolated))
	}()
	if isolated.root == "" {
		return nil
	}
	if !validIsolationRootPath(isolated.root) || isolated.rootInfo == nil || isolated.markerInfo == nil || isolated.rootHandle == nil || isolated.markerHandle == nil {
		return fmt.Errorf("refusing to remove an invalid isolation root")
	}
	pinnedRoot, err := isolated.rootHandle.Stat(".")
	if err != nil || !pinnedRoot.IsDir() || !os.SameFile(isolated.rootInfo, pinnedRoot) {
		return errors.Join(err, fmt.Errorf("isolated root pin changed before cleanup"))
	}
	pinnedMarker, err := isolated.markerHandle.Stat()
	if err != nil || !pinnedMarker.Mode().IsRegular() || !os.SameFile(isolated.markerInfo, pinnedMarker) {
		return errors.Join(err, fmt.Errorf("isolated root owner marker pin changed before cleanup"))
	}
	parent, name := filepath.Split(isolated.root)
	parentRoot, err := os.OpenRoot(parent)
	if err != nil {
		return err
	}
	defer parentRoot.Close()
	currentRoot, err := parentRoot.Lstat(name)
	if err != nil {
		return fmt.Errorf("isolated root identity changed before cleanup: %w", err)
	}
	if !currentRoot.IsDir() || currentRoot.Mode()&os.ModeSymlink != 0 || !os.SameFile(pinnedRoot, currentRoot) {
		return fmt.Errorf("isolated root identity changed before cleanup")
	}
	marker, err := isolated.rootHandle.Lstat(ownerMarkerName)
	if err != nil {
		return fmt.Errorf("isolated root owner marker changed before cleanup: %w", err)
	}
	if !marker.Mode().IsRegular() || marker.Mode()&os.ModeSymlink != 0 || !os.SameFile(pinnedMarker, marker) {
		return fmt.Errorf("isolated root owner marker changed before cleanup")
	}
	currentMarker, err := isolated.rootHandle.Open(ownerMarkerName)
	if err != nil {
		return fmt.Errorf("isolated root owner marker changed before cleanup: %w", err)
	}
	currentMarkerInfo, statErr := currentMarker.Stat()
	markerBytes, readErr := io.ReadAll(io.LimitReader(currentMarker, int64(len(ownerMarkerContent)+1)))
	closeMarkerErr := currentMarker.Close()
	if statErr != nil || readErr != nil || closeMarkerErr != nil {
		return errors.Join(statErr, readErr, closeMarkerErr, fmt.Errorf("isolated root owner marker is invalid"))
	}
	if !currentMarkerInfo.Mode().IsRegular() || !os.SameFile(pinnedMarker, currentMarkerInfo) || !os.SameFile(marker, currentMarkerInfo) {
		return fmt.Errorf("isolated root owner marker changed before cleanup")
	}
	if string(markerBytes) != ownerMarkerContent {
		return fmt.Errorf("isolated root owner marker is invalid")
	}
	if r != nil && r.removeRoot != nil {
		if err := r.removeRoot(isolated.root); err != nil {
			return err
		}
	}
	if err := removePinnedRootContents(isolated.rootHandle); err != nil {
		return err
	}
	currentRoot, err = parentRoot.Lstat(name)
	if err != nil {
		return fmt.Errorf("isolated root identity changed during cleanup: %w", err)
	}
	if !currentRoot.IsDir() || currentRoot.Mode()&os.ModeSymlink != 0 || !os.SameFile(pinnedRoot, currentRoot) {
		return fmt.Errorf("isolated root identity changed during cleanup")
	}
	if err := parentRoot.Remove(name); err != nil {
		return err
	}
	if _, err := parentRoot.Lstat(name); !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("isolated root still exists after cleanup")
	}
	return nil
}

func closeIsolationHandles(isolated isolation) error {
	var markerErr, rootErr error
	if isolated.markerHandle != nil {
		markerErr = isolated.markerHandle.Close()
	}
	if isolated.rootHandle != nil {
		rootErr = isolated.rootHandle.Close()
	}
	return errors.Join(markerErr, rootErr)
}

func removePinnedRootContents(root *os.Root) error {
	removed := 0
	for {
		directory, err := root.Open(".")
		if err != nil {
			return err
		}
		entries, readErr := directory.ReadDir(1)
		closeErr := directory.Close()
		if errors.Is(readErr, io.EOF) {
			readErr = nil
		}
		if readErr != nil || closeErr != nil {
			return errors.Join(readErr, closeErr)
		}
		if len(entries) == 0 {
			return nil
		}
		if len(entries) != 1 {
			return fmt.Errorf("isolated root cleanup returned an invalid entry batch")
		}
		if removed == maxCleanupEntries {
			return fmt.Errorf("isolated root exceeded the bounded cleanup entry count")
		}
		if err := root.RemoveAll(entries[0].Name()); err != nil {
			return err
		}
		removed++
	}
}

func (r *Runner) setupFailure(isolated isolation, cause error) error {
	if cleanupErr := r.cleanupIsolation(isolated); cleanupErr != nil {
		return errors.Join(cause, fmt.Errorf("remove partial isolation root: %w", cleanupErr))
	}
	return cause
}

func createOwnerMarker(rootHandle *os.Root) (*os.File, os.FileInfo, error) {
	marker, err := rootHandle.OpenFile(ownerMarkerName, os.O_RDWR|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return nil, nil, err
	}
	written, writeErr := marker.Write([]byte(ownerMarkerContent))
	if writeErr == nil && written != len(ownerMarkerContent) {
		writeErr = io.ErrShortWrite
	}
	if writeErr == nil {
		writeErr = marker.Sync()
	}
	openedInfo, statErr := marker.Stat()
	if writeErr != nil {
		_ = marker.Close()
		return nil, nil, writeErr
	}
	if statErr != nil {
		_ = marker.Close()
		return nil, nil, fmt.Errorf("opened owner marker is not a stable regular file: %w", statErr)
	}
	if !openedInfo.Mode().IsRegular() {
		_ = marker.Close()
		return nil, nil, fmt.Errorf("opened owner marker is not a stable regular file")
	}
	info, err := rootHandle.Lstat(ownerMarkerName)
	if err != nil {
		_ = marker.Close()
		return nil, nil, fmt.Errorf("owner marker is not a stable regular file: %w", err)
	}
	if !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 || !os.SameFile(openedInfo, info) {
		_ = marker.Close()
		return nil, nil, fmt.Errorf("owner marker is not a stable regular file")
	}
	return marker, openedInfo, nil
}

func removeUnmarkedRoot(root string, expected os.FileInfo, pinned *os.Root) (cleanupErr error) {
	if !validIsolationRootPath(root) || expected == nil {
		return fmt.Errorf("invalid unmarked isolation root")
	}
	parent, name := filepath.Split(root)
	parentRoot, err := os.OpenRoot(parent)
	if err != nil {
		return err
	}
	defer parentRoot.Close()
	current, err := parentRoot.Lstat(name)
	if err != nil {
		return fmt.Errorf("unmarked isolation root identity changed: %w", err)
	}
	if !current.IsDir() || current.Mode()&os.ModeSymlink != 0 || !os.SameFile(expected, current) {
		return fmt.Errorf("unmarked isolation root identity changed")
	}
	if pinned == nil {
		pinned, err = parentRoot.OpenRoot(name)
		if err != nil {
			return err
		}
	}
	defer func() {
		cleanupErr = errors.Join(cleanupErr, pinned.Close())
	}()
	pinnedInfo, err := pinned.Stat(".")
	if err != nil || !pinnedInfo.IsDir() || !os.SameFile(expected, pinnedInfo) || !os.SameFile(current, pinnedInfo) {
		return errors.Join(err, fmt.Errorf("unmarked isolation root identity changed"))
	}
	directory, openErr := pinned.Open(".")
	if openErr != nil {
		return openErr
	}
	entries, readErr := directory.ReadDir(1)
	if errors.Is(readErr, io.EOF) {
		readErr = nil
	}
	closeDirectoryErr := directory.Close()
	if readErr != nil || closeDirectoryErr != nil || len(entries) != 0 {
		return errors.Join(readErr, closeDirectoryErr, fmt.Errorf("unmarked isolation root is not empty"))
	}
	current, err = parentRoot.Lstat(name)
	if err != nil || !current.IsDir() || current.Mode()&os.ModeSymlink != 0 || !os.SameFile(pinnedInfo, current) {
		return errors.Join(err, fmt.Errorf("unmarked isolation root identity changed"))
	}
	if err := parentRoot.Remove(name); err != nil {
		return err
	}
	if _, err := parentRoot.Lstat(name); !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("unmarked isolation root still exists after cleanup")
	}
	return nil
}

func validIsolationRootPath(root string) bool {
	if root == "" || !filepath.IsAbs(root) || filepath.Clean(root) != root {
		return false
	}
	base := filepath.Base(root)
	return strings.HasPrefix(base, "atsura-processor-") && len(base) > len("atsura-processor-") && filepath.Dir(root) != root
}

func identifyExecutable(path string) (processorprocess.Identity, error) {
	if err := processorprocess.ValidateExecutablePath(path); err != nil {
		return processorprocess.Identity{}, fault.Wrap(fault.KindInvalidInput, "invalid_processor_executable", "The processor executable path must be absolute and clean.", false, err, helpAction())
	}
	directory, name := filepath.Split(path)
	root, err := os.OpenRoot(directory)
	if err != nil {
		return processorprocess.Identity{}, identityFault(err)
	}
	defer root.Close()
	info, err := root.Lstat(name)
	if err != nil {
		return processorprocess.Identity{}, identityFault(err)
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() || !platformExecutable(path, info.Mode()) {
		return processorprocess.Identity{}, fault.New(fault.KindInvalidInput, "unsafe_processor_executable", "The processor executable must be a supported regular executable, not a symbolic link.", false, helpAction())
	}
	if info.Size() <= 0 || info.Size() > processorprocess.MaxExecutableBytes {
		return processorprocess.Identity{}, fault.New(fault.KindInvalidInput, "unsafe_processor_executable", "The processor executable exceeds the supported identity bound.", false, helpAction())
	}
	file, err := root.Open(name)
	if err != nil {
		return processorprocess.Identity{}, identityFault(err)
	}
	defer file.Close()
	opened, err := file.Stat()
	if err != nil || !opened.Mode().IsRegular() || !os.SameFile(info, opened) {
		return processorprocess.Identity{}, fault.Wrap(fault.KindRejected, "processor_identity_changed", "The processor executable changed while its identity was read.", false, err, helpAction())
	}
	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return processorprocess.Identity{}, identityFault(err)
	}
	identity := processorprocess.Identity{ResolvedPath: path, SHA256: fmt.Sprintf("%x", hash.Sum(nil)), Size: opened.Size()}
	if err := identity.Validate(); err != nil {
		return processorprocess.Identity{}, fault.Wrap(fault.KindContract, "invalid_processor_identity", "The processor executable identity is invalid.", false, err, helpAction())
	}
	return identity, nil
}

func identityFault(err error) *fault.Error {
	return fault.Wrap(fault.KindUnavailable, "processor_identity_unavailable", "The processor executable identity could not be read.", true, err, helpAction())
}

func helpAction() fault.NextAction {
	return fault.NextAction{Command: "help processor inspect", Reason: "Review the exact processor executable and isolated process contract."}
}

type limitedBuffer struct {
	buffer   bytes.Buffer
	limit    int
	exceeded bool
	cancel   context.CancelFunc
}

func (b *limitedBuffer) Write(value []byte) (int, error) {
	remaining := b.limit - b.buffer.Len()
	if remaining > 0 {
		written := len(value)
		if written > remaining {
			written = remaining
		}
		_, _ = b.buffer.Write(value[:written])
	}
	if len(value) > remaining {
		b.exceeded = true
		b.cancel()
	}
	return len(value), nil
}

func (b *limitedBuffer) Bytes() []byte {
	return append([]byte(nil), b.buffer.Bytes()...)
}
