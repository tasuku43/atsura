package sourceexec

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/tasuku43/atsura/internal/domain/fault"
	"github.com/tasuku43/atsura/internal/domain/sourceprocess"
)

func TestSourceExecHelper(t *testing.T) {
	if os.Getenv("ATSURA_SOURCEEXEC_HELPER") != "1" {
		return
	}
	switch os.Getenv("ATSURA_SOURCEEXEC_MODE") {
	case "echo":
		input, _ := io.ReadAll(os.Stdin)
		fmt.Printf(`{"argv":%q,"stdin_bytes":%d}`, strings.Join(os.Args, "|"), len(input))
	case "stderr":
		fmt.Fprint(os.Stdout, `{}`)
		fmt.Fprint(os.Stderr, "warning\nnext")
	case "large_stdout":
		fmt.Fprint(os.Stdout, strings.Repeat("x", 2048))
	case "large_stderr":
		fmt.Fprint(os.Stderr, strings.Repeat("x", 2048))
	case "sleep":
		time.Sleep(5 * time.Second)
	case "nonzero":
		fmt.Fprint(os.Stdout, `{"raw":"must not become success"}`)
		fmt.Fprint(os.Stderr, "private failure detail")
		os.Exit(7)
	default:
		fmt.Fprint(os.Stdout, `[]`)
	}
	os.Exit(0)
}

func helperRequest(timeout time.Duration) sourceprocess.Request {
	return sourceprocess.Request{
		Executable: os.Args[0], Args: []string{"-test.run=TestSourceExecHelper", "--", "literal", "$(not-a-shell)"},
		Timeout: timeout, StdoutLimit: 4096, StderrLimit: 1024,
	}
}

func boundHelperRequest(t *testing.T, runner *Runner, request sourceprocess.Request) sourceprocess.BoundRequest {
	t.Helper()
	identity, err := runner.Identify(context.Background(), request.Executable)
	if err != nil {
		t.Fatal(err)
	}
	request.Executable = identity.ResolvedPath
	return sourceprocess.BoundRequest{Process: request, ExpectedIdentity: identity}
}

func copyTestExecutable(t *testing.T) string {
	t.Helper()
	value, err := os.ReadFile(os.Args[0])
	if err != nil {
		t.Fatal(err)
	}
	name := "helper"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, value, 0o700); err != nil {
		t.Fatal(err)
	}
	return path
}

func replaceTestExecutable(path string) error {
	replacement := path + ".replacement"
	if err := os.WriteFile(replacement, []byte("changed executable"), 0o700); err != nil {
		return fmt.Errorf("write replacement: %w", err)
	}
	if err := os.Rename(replacement, path); err == nil {
		return nil
	} else if runtime.GOOS != "windows" {
		return fmt.Errorf("replace executable: %w", err)
	}

	deadline := time.Now().Add(5 * time.Second)
	for {
		if err := os.Remove(path); err == nil || errors.Is(err, os.ErrNotExist) {
			if err := os.Rename(replacement, path); err != nil {
				return fmt.Errorf("install replacement: %w", err)
			}
			return nil
		} else if time.Now().After(deadline) {
			return fmt.Errorf("remove running executable before replacement: %w", err)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func assertExactSourceFault(t *testing.T, err error, kind fault.Kind, code, message string, retryable bool) {
	t.Helper()
	public, ok := fault.PublicCopy(err)
	if !ok {
		t.Fatalf("error is not a valid public fault: %v", err)
	}
	if public.Kind != kind || public.Code != code || public.Message != message || public.Retryable != retryable {
		t.Fatalf("public fault=%+v", public)
	}
	if len(public.NextActions) != 1 || public.NextActions[0].Command != "help source inspect" || public.NextActions[0].Reason != "Review the bounded source-inspection process contract and executable." {
		t.Fatalf("recovery=%+v", public.NextActions)
	}
}

func TestRunUsesExactArgvEOFStdinAndOneAttempt(t *testing.T) {
	t.Setenv("ATSURA_SOURCEEXEC_HELPER", "1")
	t.Setenv("ATSURA_SOURCEEXEC_MODE", "echo")
	request := helperRequest(10 * time.Second)
	result, err := New().Run(context.Background(), request)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if err := result.Validate(request, true); err != nil {
		t.Fatalf("result.Validate() = %v", err)
	}
	if result.Attempts != 1 || !strings.Contains(string(result.Stdout), `literal|$(not-a-shell)`) || !strings.Contains(string(result.Stdout), `"stdin_bytes":0`) {
		t.Fatalf("result = %+v, stdout = %s", result, result.Stdout)
	}
}

func TestRunCapturesBoundedSuccessfulStderr(t *testing.T) {
	t.Setenv("ATSURA_SOURCEEXEC_HELPER", "1")
	t.Setenv("ATSURA_SOURCEEXEC_MODE", "stderr")
	result, err := New().Run(context.Background(), helperRequest(10*time.Second))
	if err != nil || string(result.Stdout) != `{}` || string(result.Stderr) != "warning\nnext" {
		t.Fatalf("result = %+v, error = %v", result, err)
	}
}

func TestRunClassifiesPostStartFailuresWithoutRetry(t *testing.T) {
	tests := []struct {
		name    string
		mode    string
		mutate  func(*sourceprocess.Request)
		kind    fault.Kind
		code    string
		message string
	}{
		{name: "nonzero", mode: "nonzero", kind: fault.KindRejected, code: "source_command_failed", message: "The source process exited without a successful result."},
		{name: "timeout", mode: "sleep", mutate: func(value *sourceprocess.Request) { value.Timeout = 30 * time.Millisecond }, kind: fault.KindUnavailable, code: "source_command_timeout", message: "The source process exceeded its declared timeout."},
		{name: "stdout bound", mode: "large_stdout", mutate: func(value *sourceprocess.Request) { value.StdoutLimit = 128 }, kind: fault.KindContract, code: "source_stdout_too_large", message: "The source process stdout exceeded the 4 MiB limit."},
		{name: "stderr bound", mode: "large_stderr", mutate: func(value *sourceprocess.Request) { value.StderrLimit = 128 }, kind: fault.KindContract, code: "source_stderr_too_large", message: "The source process stderr exceeded the 256 KiB limit."},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Setenv("ATSURA_SOURCEEXEC_HELPER", "1")
			t.Setenv("ATSURA_SOURCEEXEC_MODE", test.mode)
			request := helperRequest(10 * time.Second)
			if test.mutate != nil {
				test.mutate(&request)
			}
			result, err := New().Run(context.Background(), request)
			if result.Attempts != 1 {
				t.Fatalf("attempts = %d, error = %v", result.Attempts, err)
			}
			assertExactSourceFault(t, err, test.kind, test.code, test.message, false)
			if validateErr := result.Validate(request, false); validateErr != nil {
				t.Fatalf("result.Validate() = %v", validateErr)
			}
		})
	}
}

func TestRunRejectsPreflightWithoutAttempt(t *testing.T) {
	request := helperRequest(10 * time.Second)
	request.Executable = filepath.Join(t.TempDir(), "missing")
	result, err := New().Run(context.Background(), request)
	if result.Attempts != 0 || result.ExitCode != -1 {
		t.Fatalf("result=%+v", result)
	}
	assertExactSourceFault(t, err, fault.KindNotFound, "source_executable_not_found", "The source executable was not found.", false)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	result, err = New().Run(ctx, helperRequest(10*time.Second))
	if !errors.Is(err, context.Canceled) || result.Attempts != 0 {
		t.Fatalf("canceled result = %+v, error = %v", result, err)
	}
}

func TestRunBoundClassifiesUnavailableIdentityWithExactZeroAttemptFault(t *testing.T) {
	path := filepath.Join(t.TempDir(), "missing")
	request := sourceprocess.BoundRequest{
		Process: sourceprocess.Request{
			Executable: path, Args: []string{}, Timeout: 10 * time.Second,
			StdoutLimit: 4096, StderrLimit: 1024,
		},
		ExpectedIdentity: sourceprocess.Identity{
			ResolvedPath: path, SHA256: strings.Repeat("a", 64), Size: 1,
		},
	}
	result, err := New().RunBound(context.Background(), request)
	if result.Attempts != 0 || result.ExitCode != -1 {
		t.Fatalf("result=%+v", result)
	}
	assertExactSourceFault(t, err, fault.KindUnavailable, "source_identity_unavailable", "The source executable identity could not be read.", true)
}

func TestRunRejectsInvalidRequestWithExactZeroAttemptFault(t *testing.T) {
	result, err := New().Run(context.Background(), sourceprocess.Request{})
	if result.Attempts != 0 || result.ExitCode != -1 {
		t.Fatalf("result=%+v", result)
	}
	assertExactSourceFault(t, err, fault.KindContract, "invalid_source_process_request", "The source process request is invalid.", false)
}

func TestRunBoundRejectsUnsafeExecutableWithExactZeroAttemptFault(t *testing.T) {
	path := filepath.Clean(t.TempDir())
	request := sourceprocess.BoundRequest{
		Process: sourceprocess.Request{
			Executable:  path,
			Args:        []string{},
			Timeout:     10 * time.Second,
			StdoutLimit: 4096,
			StderrLimit: 1024,
		},
		ExpectedIdentity: sourceprocess.Identity{
			ResolvedPath: path,
			SHA256:       strings.Repeat("a", 64),
			Size:         1,
		},
	}
	result, err := New().RunBound(context.Background(), request)
	if result.Attempts != 0 || result.ExitCode != -1 {
		t.Fatalf("result=%+v", result)
	}
	assertExactSourceFault(t, err, fault.KindInvalidInput, "unsafe_source_executable", "The resolved source executable is not a supported regular executable.", false)
}

func TestIdentifyExecutableClassifiesInvalidConstructedIdentityExactly(t *testing.T) {
	t.Chdir(t.TempDir())
	name := "relative-source"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	if err := os.WriteFile(name, []byte("synthetic executable"), 0o700); err != nil {
		t.Fatal(err)
	}
	relative := "./" + name
	if filepath.IsAbs(relative) {
		t.Fatalf("fixture path is absolute: %q", relative)
	}
	identity, err := identifyExecutable(relative)
	if !identity.IsZero() {
		t.Fatalf("identity=%+v", identity)
	}
	assertExactSourceFault(t, err, fault.KindContract, "invalid_source_identity", "The resolved source executable identity is invalid.", false)
}

func TestRunClassifiesNativeStartFailureWithoutAttempt(t *testing.T) {
	name := "invalid-source"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, []byte("not a native executable\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	request := helperRequest(10 * time.Second)
	request.Executable = path
	result, err := New().Run(context.Background(), request)
	if result.Attempts != 0 || result.ExitCode != -1 {
		t.Fatalf("result=%+v error=%v", result, err)
	}
	assertExactSourceFault(t, err, fault.KindUnavailable, "source_process_start_failed", "The source process could not be started.", true)
}

func TestRunClassifiesWaitFailureAfterOneAttempt(t *testing.T) {
	t.Setenv("ATSURA_SOURCEEXEC_HELPER", "1")
	t.Setenv("ATSURA_SOURCEEXEC_MODE", "default")
	runner := &Runner{wait: func(_ *exec.Cmd, waitErr error) error {
		if waitErr != nil {
			t.Fatalf("native wait error=%v", waitErr)
		}
		return errors.New("ATSURA_SECRET_SYNTHETIC_WAIT_CAUSE")
	}}
	result, err := runner.Run(context.Background(), helperRequest(10*time.Second))
	if result.Attempts != 1 {
		t.Fatalf("result=%+v error=%v", result, err)
	}
	assertExactSourceFault(t, err, fault.KindUnavailable, "source_process_wait_failed", "The source process result could not be collected.", false)
}

func TestRunClassifiesCancellationAfterStartAsNonRetryable(t *testing.T) {
	t.Setenv("ATSURA_SOURCEEXEC_HELPER", "1")
	t.Setenv("ATSURA_SOURCEEXEC_MODE", "sleep")
	ctx, cancel := context.WithCancel(context.Background())
	runner := New()
	runner.afterStart = func(string) { cancel() }
	request := helperRequest(10 * time.Second)
	result, err := runner.Run(ctx, request)
	if result.Attempts != 1 {
		t.Fatalf("result=%+v error=%v", result, err)
	}
	assertExactSourceFault(t, err, fault.KindCanceled, "source_execution_canceled", "The caller canceled after the source process started; its downstream outcome is not classified as replay-safe.", false)
}

func TestIdentifyReturnsIdentityWithoutStartingProcess(t *testing.T) {
	identity, err := New().Identify(context.Background(), os.Args[0])
	if err != nil {
		t.Fatal(err)
	}
	if identity.ResolvedPath == "" || len(identity.SHA256) != 64 || identity.Size <= 0 {
		t.Fatalf("identity = %+v", identity)
	}
}

func TestRunDetectsExecutableDriftBeforeAndAfterStart(t *testing.T) {
	beforePath := copyTestExecutable(t)
	beforeRequest := helperRequest(10 * time.Second)
	beforeRequest.Executable = beforePath
	var replaceErr error
	result, err := (&Runner{beforeStart: func(path string) { replaceErr = replaceTestExecutable(path) }}).Run(context.Background(), beforeRequest)
	if replaceErr != nil {
		t.Fatal(replaceErr)
	}
	if result.Attempts != 0 {
		t.Fatalf("before drift attempts=%d error=%v", result.Attempts, err)
	}
	assertExactSourceFault(t, err, fault.KindRejected, "source_identity_changed", "The resolved source executable changed before it could be started.", false)

	t.Setenv("ATSURA_SOURCEEXEC_HELPER", "1")
	t.Setenv("ATSURA_SOURCEEXEC_MODE", "default")
	afterPath := copyTestExecutable(t)
	afterRequest := helperRequest(10 * time.Second)
	afterRequest.Executable = afterPath
	replaceErr = nil
	result, err = (&Runner{afterStart: func(path string) { replaceErr = replaceTestExecutable(path) }}).Run(context.Background(), afterRequest)
	if replaceErr != nil {
		t.Fatal(replaceErr)
	}
	if result.Attempts != 1 {
		t.Fatalf("after drift attempts=%d error=%v", result.Attempts, err)
	}
	assertExactSourceFault(t, err, fault.KindRejected, "source_identity_changed", "The resolved source executable changed during execution.", false)
}

func TestRunBoundRequiresExpectedIdentityAtEveryRaceBoundary(t *testing.T) {
	initialPath := copyTestExecutable(t)
	initialRequest := helperRequest(10 * time.Second)
	initialRequest.Executable = initialPath
	initialRunner := New()
	initialBound := boundHelperRequest(t, initialRunner, initialRequest)
	if err := replaceTestExecutable(initialPath); err != nil {
		t.Fatal(err)
	}
	result, err := initialRunner.RunBound(context.Background(), initialBound)
	if result.Attempts != 0 {
		t.Fatalf("initial mismatch attempts=%d error=%v", result.Attempts, err)
	}
	assertExactSourceFault(t, err, fault.KindRejected, "source_identity_changed", "The resolved source executable does not match the bundle-bound identity.", false)

	beforePath := copyTestExecutable(t)
	beforeRequest := helperRequest(10 * time.Second)
	beforeRequest.Executable = beforePath
	var replaceErr error
	beforeRunner := &Runner{beforeStart: func(path string) { replaceErr = replaceTestExecutable(path) }}
	beforeBound := boundHelperRequest(t, beforeRunner, beforeRequest)
	result, err = beforeRunner.RunBound(context.Background(), beforeBound)
	if replaceErr != nil {
		t.Fatal(replaceErr)
	}
	if result.Attempts != 0 {
		t.Fatalf("pre-start mismatch attempts=%d error=%v", result.Attempts, err)
	}
	assertExactSourceFault(t, err, fault.KindRejected, "source_identity_changed", "The resolved source executable changed before it could be started.", false)

	t.Setenv("ATSURA_SOURCEEXEC_HELPER", "1")
	t.Setenv("ATSURA_SOURCEEXEC_MODE", "default")
	afterPath := copyTestExecutable(t)
	afterRequest := helperRequest(10 * time.Second)
	afterRequest.Executable = afterPath
	replaceErr = nil
	afterRunner := &Runner{afterStart: func(path string) { replaceErr = replaceTestExecutable(path) }}
	afterBound := boundHelperRequest(t, afterRunner, afterRequest)
	result, err = afterRunner.RunBound(context.Background(), afterBound)
	if replaceErr != nil {
		t.Fatal(replaceErr)
	}
	if result.Attempts != 1 {
		t.Fatalf("post-start mismatch attempts=%d error=%v", result.Attempts, err)
	}
	assertExactSourceFault(t, err, fault.KindRejected, "source_identity_changed", "The resolved source executable changed during execution.", false)
	if validateErr := result.ValidateBound(afterBound, false); validateErr != nil {
		t.Fatalf("ValidateBound() = %v", validateErr)
	}
}
