// Package sourceexec executes one bounded source process without shell
// interpolation and records private executable identity evidence.
package sourceexec

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
	"time"

	"github.com/tasuku43/atsura/internal/domain/fault"
	"github.com/tasuku43/atsura/internal/domain/sourceprocess"
)

// Runner is the production direct-process adapter. Its unexported test seams
// only let package tests deterministically exercise identity and wait
// boundaries; they are unrelated to coding-agent host hooks.
type Runner struct {
	beforeStart func(string)
	afterStart  func(string)
	wait        func(*exec.Cmd, error) error
}

// New creates a bounded process runner.
func New() *Runner { return &Runner{} }

// Identify resolves and fingerprints one executable without starting it.
// Status and trust use this read-only boundary to detect source drift.
func (r *Runner) Identify(ctx context.Context, executable string) (sourceprocess.Identity, error) {
	if ctx == nil {
		return sourceprocess.Identity{}, fmt.Errorf("source identity context is nil")
	}
	if err := ctx.Err(); err != nil {
		return sourceprocess.Identity{}, err
	}
	resolved, err := resolveExecutable(executable)
	if err != nil {
		return sourceprocess.Identity{}, err
	}
	identity, err := identifyExecutable(resolved)
	if err != nil {
		return sourceprocess.Identity{}, err
	}
	if err := ctx.Err(); err != nil {
		return sourceprocess.Identity{}, err
	}
	return identity, nil
}

// Run resolves, fingerprints, revalidates, and starts at most one process.
func (r *Runner) Run(ctx context.Context, request sourceprocess.Request) (sourceprocess.Result, error) {
	return r.run(ctx, request, nil)
}

// RunBound starts at most one process only when every executable observation
// matches identity evidence supplied by the caller.
func (r *Runner) RunBound(ctx context.Context, request sourceprocess.BoundRequest) (sourceprocess.Result, error) {
	return r.run(ctx, request.Process, &request.ExpectedIdentity)
}

func (r *Runner) run(ctx context.Context, request sourceprocess.Request, expected *sourceprocess.Identity) (sourceprocess.Result, error) {
	zero := sourceprocess.Result{ExitCode: -1}
	if ctx == nil {
		return zero, fmt.Errorf("source process context is nil")
	}
	if err := ctx.Err(); err != nil {
		return zero, err
	}
	if expected != nil {
		if err := (sourceprocess.BoundRequest{Process: request, ExpectedIdentity: *expected}).Validate(); err != nil {
			return zero, fault.Wrap(fault.KindContract, "invalid_source_process_request", "The source process request is invalid.", false, err, helpAction())
		}
	} else if err := request.Validate(); err != nil {
		return zero, fault.Wrap(fault.KindContract, "invalid_source_process_request", "The source process request is invalid.", false, err, helpAction())
	}
	resolved := request.Executable
	if expected == nil {
		var err error
		resolved, err = resolveExecutable(request.Executable)
		if err != nil {
			return zero, err
		}
	}
	identity, err := identifyExecutable(resolved)
	if err != nil {
		return zero, err
	}
	authority := identity
	if expected != nil {
		authority = *expected
		if identity != authority {
			return zero, fault.New(fault.KindRejected, "source_identity_changed", "The resolved source executable does not match the bundle-bound identity.", false, helpAction())
		}
	}
	if r != nil && r.beforeStart != nil {
		r.beforeStart(resolved)
	}
	revalidated, err := identifyExecutable(resolved)
	if err != nil || revalidated != authority {
		return zero, fault.Wrap(fault.KindRejected, "source_identity_changed", "The resolved source executable changed before it could be started.", false, err, helpAction())
	}

	runCtx, cancel := context.WithTimeout(ctx, request.Timeout)
	defer cancel()
	stdout := &limitedBuffer{limit: request.StdoutLimit, cancel: cancel}
	stderr := &limitedBuffer{limit: request.StderrLimit, cancel: cancel}
	// #nosec G204 -- the product executes the validated regular executable and
	// argv vector above; no string is interpreted by a shell.
	command := exec.CommandContext(runCtx, authority.ResolvedPath, request.Args...)
	command.Stdin = nil
	command.Stdout = stdout
	command.Stderr = stderr
	command.WaitDelay = 2 * time.Second
	if err := command.Start(); err != nil {
		return zero, fault.Wrap(fault.KindUnavailable, "source_process_start_failed", "The source process could not be started.", true, err, helpAction())
	}
	result := sourceprocess.Result{Attempts: 1, ExitCode: -1, Identity: authority}
	if r != nil && r.afterStart != nil {
		r.afterStart(resolved)
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

	if stdout.exceeded {
		return result, fault.New(fault.KindContract, "source_stdout_too_large", "The source process stdout exceeded the 4 MiB limit.", false, helpAction())
	}
	if stderr.exceeded {
		return result, fault.New(fault.KindContract, "source_stderr_too_large", "The source process stderr exceeded the 256 KiB limit.", false, helpAction())
	}
	if err := ctx.Err(); err != nil {
		return result, fault.Wrap(fault.KindCanceled, "source_execution_canceled", "The caller canceled after the source process started; its downstream outcome is not classified as replay-safe.", false, err, helpAction())
	}
	if errors.Is(runCtx.Err(), context.DeadlineExceeded) {
		return result, fault.Wrap(fault.KindUnavailable, "source_command_timeout", "The source process exceeded its declared timeout.", false, waitErr, helpAction())
	}
	postIdentity, identityErr := identifyExecutable(resolved)
	if identityErr != nil || postIdentity != authority {
		return result, fault.Wrap(fault.KindRejected, "source_identity_changed", "The resolved source executable changed during execution.", false, identityErr, helpAction())
	}
	if waitErr != nil {
		var exitError *exec.ExitError
		if errors.As(waitErr, &exitError) {
			return result, fault.Wrap(fault.KindRejected, "source_command_failed", "The source process exited without a successful result.", false, waitErr, helpAction())
		}
		return result, fault.Wrap(fault.KindUnavailable, "source_process_wait_failed", "The source process result could not be collected.", false, waitErr, helpAction())
	}
	return result, nil
}

func resolveExecutable(value string) (string, error) {
	resolved, err := exec.LookPath(value)
	if err != nil {
		return "", fault.Wrap(fault.KindNotFound, "source_executable_not_found", "The source executable was not found.", false, err, helpAction())
	}
	absolute, err := filepath.Abs(resolved)
	if err != nil {
		return "", fault.Wrap(fault.KindUnavailable, "source_identity_unavailable", "The source executable identity could not be resolved.", true, err, helpAction())
	}
	evaluated, err := filepath.EvalSymlinks(absolute)
	if err != nil {
		return "", fault.Wrap(fault.KindUnavailable, "source_identity_unavailable", "The source executable identity could not be resolved.", true, err, helpAction())
	}
	return filepath.Clean(evaluated), nil
}

func identifyExecutable(path string) (sourceprocess.Identity, error) {
	directory, name := filepath.Split(path)
	root, err := os.OpenRoot(directory)
	if err != nil {
		return sourceprocess.Identity{}, identityFault(err)
	}
	defer root.Close()
	info, err := root.Lstat(name)
	if err != nil {
		return sourceprocess.Identity{}, identityFault(err)
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() || !platformExecutable(path, info.Mode()) {
		return sourceprocess.Identity{}, fault.New(fault.KindInvalidInput, "unsafe_source_executable", "The resolved source executable is not a supported regular executable.", false, helpAction())
	}
	if info.Size() <= 0 || info.Size() > sourceprocess.MaxExecutableBytes {
		return sourceprocess.Identity{}, fault.New(fault.KindInvalidInput, "unsafe_source_executable", "The resolved source executable exceeds the supported identity bound.", false, helpAction())
	}
	file, err := root.Open(name)
	if err != nil {
		return sourceprocess.Identity{}, identityFault(err)
	}
	defer file.Close()
	opened, err := file.Stat()
	if err != nil || !opened.Mode().IsRegular() || !os.SameFile(info, opened) {
		return sourceprocess.Identity{}, fault.Wrap(fault.KindRejected, "source_identity_changed", "The source executable changed while its identity was read.", false, err, helpAction())
	}
	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return sourceprocess.Identity{}, identityFault(err)
	}
	identity := sourceprocess.Identity{ResolvedPath: path, SHA256: fmt.Sprintf("%x", hash.Sum(nil)), Size: opened.Size()}
	if err := identity.Validate(); err != nil {
		return sourceprocess.Identity{}, fault.Wrap(fault.KindContract, "invalid_source_identity", "The resolved source executable identity is invalid.", false, err, helpAction())
	}
	return identity, nil
}

func identityFault(err error) *fault.Error {
	return fault.Wrap(fault.KindUnavailable, "source_identity_unavailable", "The source executable identity could not be read.", true, err, helpAction())
}

func helpAction() fault.NextAction {
	return fault.NextAction{Command: "help source inspect", Reason: "Review the bounded source-inspection process contract and executable."}
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
