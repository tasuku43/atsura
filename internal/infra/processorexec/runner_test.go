package processorexec

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/tasuku43/atsura/internal/domain/fault"
	"github.com/tasuku43/atsura/internal/domain/processorprocess"
)

func TestProcessorExecHelper(t *testing.T) {
	separator := slices.Index(os.Args, "--")
	if separator < 0 || separator+1 >= len(os.Args) {
		return
	}
	mode := os.Args[separator+1]
	switch mode {
	case "environment":
		input, _ := io.ReadAll(os.Stdin)
		cwd, _ := os.Getwd()
		value := struct {
			Args        []string `json:"args"`
			Input       string   `json:"input"`
			CWD         string   `json:"cwd"`
			Environment []string `json:"environment"`
		}{
			Args:        append([]string(nil), os.Args[separator+2:]...),
			Input:       string(input),
			CWD:         cwd,
			Environment: os.Environ(),
		}
		_ = json.NewEncoder(os.Stdout).Encode(value)
	case "large_stdout":
		_, _ = fmt.Fprint(os.Stdout, strings.Repeat("x", 2048))
	case "large_stderr":
		_, _ = fmt.Fprint(os.Stderr, strings.Repeat("x", 2048))
	case "stderr":
		_, _ = fmt.Fprint(os.Stdout, "output")
		_, _ = fmt.Fprint(os.Stderr, "processor warning")
	case "nonzero":
		_, _ = fmt.Fprint(os.Stdout, "private output")
		_, _ = fmt.Fprint(os.Stderr, "private error")
		os.Exit(7)
	case "sleep":
		time.Sleep(5 * time.Second)
	default:
		_, _ = fmt.Fprint(os.Stdout, "ok")
	}
	os.Exit(0)
}

func helperRequest(t *testing.T, runner *Runner, mode string) processorprocess.Request {
	t.Helper()
	executable, err := filepath.Abs(os.Args[0])
	if err != nil {
		t.Fatal(err)
	}
	executable, err = filepath.EvalSymlinks(executable)
	if err != nil {
		t.Fatal(err)
	}
	identity, err := runner.Identify(context.Background(), executable)
	if err != nil {
		t.Fatal(err)
	}
	return processorprocess.Request{
		Executable: executable,
		Args: []string{
			"-test.run=TestProcessorExecHelper", "--", mode, "literal", "$(not-a-shell)", "",
		},
		Input:               []byte("processor input\x00\n"),
		Timeout:             processorprocess.MaxTimeout,
		StdoutLimit:         4096,
		StderrLimit:         1024,
		ExpectedIdentity:    identity,
		EnvironmentContract: processorprocess.EnvironmentRTKIsolatedV2,
	}
}

func assertProcessorFault(t *testing.T, err error, kind fault.Kind, code, message string, retryable bool) {
	t.Helper()
	public, ok := fault.PublicCopy(err)
	if !ok {
		t.Fatalf("error is not a valid public fault: %v", err)
	}
	if public.Kind != kind || public.Code != code || public.Message != message || public.Retryable != retryable {
		t.Fatalf("public fault = %+v", public)
	}
	if len(public.NextActions) != 1 || public.NextActions[0].Command != "help processor inspect" {
		t.Fatalf("next actions = %+v", public.NextActions)
	}
}

func TestRunUsesExactArgvInputAndIsolatedEnvironment(t *testing.T) {
	t.Setenv("ATSURA_SECRET_CANARY", "must-not-be-inherited")
	t.Setenv("CLAUDE_CONFIG_DIR", "/must/not/be/inherited")
	t.Setenv("PATH", "/must/not/be/inherited")
	runner := New()
	request := helperRequest(t, runner, "environment")
	result, err := runner.Run(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	if err := result.Validate(request, true); err != nil {
		t.Fatal(err)
	}
	var got struct {
		Args        []string `json:"args"`
		Input       string   `json:"input"`
		CWD         string   `json:"cwd"`
		Environment []string `json:"environment"`
	}
	if err := json.Unmarshal(result.Stdout, &got); err != nil {
		t.Fatalf("decode helper output: %v (%q)", err, result.Stdout)
	}
	if !slices.Equal(got.Args, []string{"literal", "$(not-a-shell)", ""}) || got.Input != string(request.Input) {
		t.Fatalf("args/input = %#v / %q", got.Args, got.Input)
	}
	if filepath.Base(got.CWD) != "work" {
		t.Fatalf("cwd=%q", got.CWD)
	}
	environment := strings.Join(got.Environment, "\n")
	for _, required := range []string{
		"RTK_TELEMETRY_DISABLED=1", "RTK_TEE=0", "RTK_NO_TOML=1",
		"LANG=C", "LC_ALL=C", "TZ=UTC", "NO_COLOR=1",
		"HOME=", "XDG_CONFIG_HOME=", "XDG_DATA_HOME=", "XDG_CACHE_HOME=",
		"XDG_STATE_HOME=", "RTK_DB_PATH=",
	} {
		if !strings.Contains(environment, required) {
			t.Fatalf("environment lacks %q: %q", required, environment)
		}
	}
	environmentKeys := make(map[string]struct{}, len(got.Environment))
	orderedKeys := make([]string, 0, len(got.Environment))
	for _, entry := range got.Environment {
		key, _, _ := strings.Cut(entry, "=")
		environmentKeys[key] = struct{}{}
		orderedKeys = append(orderedKeys, key)
	}
	for _, forbidden := range []string{"ATSURA_SECRET_CANARY", "CLAUDE_CONFIG_DIR", "PATH"} {
		if _, exists := environmentKeys[forbidden]; exists {
			t.Fatalf("ambient environment %q leaked: %q", forbidden, environment)
		}
	}
	if runtime.GOOS != "windows" {
		wantedKeys := []string{
			"APPDATA", "HOME", "LANG", "LC_ALL", "LOCALAPPDATA", "NO_COLOR",
			"RTK_DB_PATH", "RTK_NO_TOML", "RTK_TEE", "RTK_TELEMETRY_DISABLED",
			"TEMP", "TMP", "TMPDIR", "TZ", "USERPROFILE", "XDG_CACHE_HOME",
			"XDG_CONFIG_HOME", "XDG_DATA_HOME", "XDG_STATE_HOME",
		}
		if !slices.Equal(orderedKeys, wantedKeys) {
			t.Fatalf("environment keys = %q", orderedKeys)
		}
	}
	root := filepath.Dir(got.CWD)
	if _, err := os.Stat(root); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("isolated root remains: %v", err)
	}
}

func TestRunClassifiesFiniteProcessFailuresAndAttempts(t *testing.T) {
	tests := []struct {
		name      string
		mode      string
		mutate    func(*processorprocess.Request)
		kind      fault.Kind
		code      string
		message   string
		retryable bool
	}{
		{name: "nonzero", mode: "nonzero", kind: fault.KindRejected, code: "processor_command_failed", message: "The processor exited without a successful result."},
		{name: "timeout", mode: "sleep", mutate: func(v *processorprocess.Request) { v.Timeout = 30 * time.Millisecond }, kind: fault.KindUnavailable, code: "processor_timeout", message: "The processor exceeded its declared timeout."},
		{name: "stdout", mode: "large_stdout", mutate: func(v *processorprocess.Request) { v.StdoutLimit = 128 }, kind: fault.KindContract, code: "processor_stdout_too_large", message: "The processor stdout exceeded the 4 MiB limit."},
		{name: "stderr", mode: "large_stderr", mutate: func(v *processorprocess.Request) { v.StderrLimit = 128 }, kind: fault.KindContract, code: "processor_stderr_too_large", message: "The processor stderr exceeded the 64 KiB limit."},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			runner := New()
			request := helperRequest(t, runner, test.mode)
			if test.mutate != nil {
				test.mutate(&request)
			}
			result, err := runner.Run(context.Background(), request)
			if result.Attempts != 1 {
				t.Fatalf("result=%+v error=%v", result, err)
			}
			assertProcessorFault(t, err, test.kind, test.code, test.message, test.retryable)
			if validateErr := result.Validate(request, false); validateErr != nil {
				t.Fatalf("Validate() = %v", validateErr)
			}
		})
	}
}

func TestRunRejectsInvalidMissingAndChangedIdentityBeforeAttempt(t *testing.T) {
	runner := New()
	result, err := runner.Run(context.Background(), processorprocess.Request{})
	if result.Attempts != 0 {
		t.Fatalf("result=%+v", result)
	}
	assertProcessorFault(t, err, fault.KindContract, "invalid_processor_process_request", "The processor process request is invalid.", false)

	request := helperRequest(t, runner, "ok")
	request.ExpectedIdentity.SHA256 = strings.Repeat("b", 64)
	result, err = runner.Run(context.Background(), request)
	if result.Attempts != 0 {
		t.Fatalf("result=%+v", result)
	}
	assertProcessorFault(t, err, fault.KindRejected, "processor_identity_changed", "The processor executable does not match the observed identity.", false)

	missing := filepath.Join(t.TempDir(), "rtk")
	_, err = runner.Identify(context.Background(), missing)
	assertProcessorFault(t, err, fault.KindUnavailable, "processor_identity_unavailable", "The processor executable identity could not be read.", true)
}

func TestRunFailsClosedWhenSetupOrCleanupFails(t *testing.T) {
	runner := New()
	request := helperRequest(t, runner, "ok")
	runner.createRoot = func() (string, error) { return "", errors.New("setup failure") }
	result, err := runner.Run(context.Background(), request)
	if result.Attempts != 0 {
		t.Fatalf("setup result=%+v", result)
	}
	assertProcessorFault(t, err, fault.KindUnavailable, "processor_environment_setup_failed", "The isolated processor environment could not be prepared.", true)

	parent := t.TempDir()
	root := filepath.Join(parent, "atsura-processor-test")
	runner = New()
	request = helperRequest(t, runner, "ok")
	runner.createRoot = func() (string, error) {
		return root, os.Mkdir(root, 0o700)
	}
	runner.removeRoot = func(string) error { return errors.New("cleanup failure") }
	result, err = runner.Run(context.Background(), request)
	if result.Attempts != 1 {
		t.Fatalf("cleanup result=%+v", result)
	}
	assertProcessorFault(t, err, fault.KindUnavailable, "processor_cleanup_failed", "The isolated processor environment could not be removed completely.", false)
	if removeErr := os.RemoveAll(root); removeErr != nil {
		t.Fatal(removeErr)
	}
}

func TestPrepareIsolationRestrictsRootMode(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX directory permission contract")
	}
	root := filepath.Join(t.TempDir(), "atsura-processor-mode")
	if err := os.Mkdir(root, 0o755); err != nil {
		t.Fatal(err)
	}
	runner := New()
	runner.createRoot = func() (string, error) { return root, nil }
	isolated, err := runner.prepareIsolation()
	if err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(root)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o700 {
		t.Fatalf("root mode = %v", info.Mode().Perm())
	}
	if err := runner.cleanupIsolation(isolated); err != nil {
		t.Fatal(err)
	}
}

func TestCleanupRefusesBroadOrUnrelatedRoots(t *testing.T) {
	runner := New()
	unrelated := t.TempDir()
	child := filepath.Join(t.TempDir(), "processor")
	if err := os.Mkdir(child, 0o700); err != nil {
		t.Fatal(err)
	}
	for _, root := range []string{"/", unrelated, child} {
		if err := runner.cleanupIsolation(isolation{root: root}); err == nil {
			t.Fatalf("cleanup accepted %q", root)
		}
		if _, err := os.Stat(root); err != nil {
			t.Fatalf("root %q was changed: %v", root, err)
		}
	}
}

func TestCleanupRequiresOriginalOwnerMarkerAndRootIdentity(t *testing.T) {
	runner := New()
	marked, err := runner.prepareIsolation()
	if err != nil {
		t.Fatal(err)
	}
	markerPath := filepath.Join(marked.root, ownerMarkerName)
	if err := os.Remove(markerPath); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(markerPath, []byte(ownerMarkerContent), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := runner.cleanupIsolation(marked); err == nil {
		t.Fatal("cleanup accepted a replaced owner marker")
	}
	if _, err := os.Stat(marked.root); err != nil {
		t.Fatalf("root with replaced marker was removed: %v", err)
	}
	if err := os.RemoveAll(marked.root); err != nil {
		t.Fatal(err)
	}

	swapped, err := runner.prepareIsolation()
	if err != nil {
		t.Fatal(err)
	}
	moved := swapped.root + "-moved"
	if err := os.Rename(swapped.root, moved); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(swapped.root, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(swapped.root, "unrelated"), []byte("keep"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := runner.cleanupIsolation(swapped); err == nil {
		t.Fatal("cleanup accepted a replaced root")
	}
	if raw, err := os.ReadFile(filepath.Join(swapped.root, "unrelated")); err != nil || string(raw) != "keep" {
		t.Fatalf("replacement root changed: %q, %v", raw, err)
	}
	if err := os.RemoveAll(swapped.root); err != nil {
		t.Fatal(err)
	}
	if err := os.RemoveAll(moved); err != nil {
		t.Fatal(err)
	}
}

func TestCleanupNeverRecursivelyRemovesLateReplacementRoot(t *testing.T) {
	runner := New()
	isolated, err := runner.prepareIsolation()
	if err != nil {
		t.Fatal(err)
	}
	moved := isolated.root + "-moved"
	sentinel := filepath.Join(isolated.root, "unrelated")
	runner.removeRoot = func(string) error {
		if err := os.Rename(isolated.root, moved); err != nil {
			return err
		}
		if err := os.Mkdir(isolated.root, 0o700); err != nil {
			return err
		}
		return os.WriteFile(sentinel, []byte("keep"), 0o600)
	}
	if err := runner.cleanupIsolation(isolated); err == nil {
		t.Fatal("cleanup accepted a root replaced after initial validation")
	}
	if raw, err := os.ReadFile(sentinel); err != nil || string(raw) != "keep" {
		t.Fatalf("late replacement root changed: %q, %v", raw, err)
	}
	if err := os.RemoveAll(isolated.root); err != nil {
		t.Fatal(err)
	}
	if err := os.RemoveAll(moved); err != nil {
		t.Fatal(err)
	}
}

func TestRemovePinnedRootContentsHonorsExactBound(t *testing.T) {
	for _, count := range []int{maxCleanupEntries, maxCleanupEntries + 1} {
		t.Run(fmt.Sprintf("entries_%d", count), func(t *testing.T) {
			path := t.TempDir()
			root, err := os.OpenRoot(path)
			if err != nil {
				t.Fatal(err)
			}
			defer root.Close()
			for index := 0; index < count; index++ {
				name := fmt.Sprintf("entry-%04d", index)
				file, err := root.OpenFile(name, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
				if err != nil {
					t.Fatal(err)
				}
				if err := file.Close(); err != nil {
					t.Fatal(err)
				}
			}
			err = removePinnedRootContents(root)
			if count == maxCleanupEntries {
				if err != nil {
					t.Fatal(err)
				}
				entries, err := os.ReadDir(path)
				if err != nil || len(entries) != 0 {
					t.Fatalf("exact-bound entries=%d err=%v", len(entries), err)
				}
				return
			}
			if err == nil {
				t.Fatal("cleanup accepted more than the bounded entry count")
			}
			entries, readErr := os.ReadDir(path)
			if readErr != nil || len(entries) != 1 {
				t.Fatalf("over-bound entries=%d err=%v", len(entries), readErr)
			}
		})
	}
}

func TestRunClassifiesCancellationAndWaitUncertainty(t *testing.T) {
	runner := New()
	request := helperRequest(t, runner, "sleep")
	ctx, cancel := context.WithCancel(context.Background())
	runner.afterStart = func(string) { cancel() }
	result, err := runner.Run(ctx, request)
	if result.Attempts != 1 {
		t.Fatalf("canceled result=%+v error=%v", result, err)
	}
	assertProcessorFault(t, err, fault.KindCanceled, "processor_execution_canceled", "The caller canceled after the processor started; the outcome is not replay-safe.", false)
}

func TestRunClassifiesWaitUncertainty(t *testing.T) {
	runner := New()
	request := helperRequest(t, runner, "ok")
	runner.wait = func(_ *exec.Cmd, _ error) error { return errors.New("wait uncertain") }
	result, err := runner.Run(context.Background(), request)
	if result.Attempts != 1 {
		t.Fatalf("result=%+v error=%v", result, err)
	}
	assertProcessorFault(t, err, fault.KindUnavailable, "processor_process_wait_failed", "The processor result could not be collected.", false)
}

func copyTestExecutable(t *testing.T) string {
	t.Helper()
	raw, err := os.ReadFile(os.Args[0])
	if err != nil {
		t.Fatal(err)
	}
	name := "processor-helper"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, raw, 0o700); err != nil {
		t.Fatal(err)
	}
	return path
}

func replaceExecutable(path string) error {
	replacement := path + ".replacement"
	if err := os.WriteFile(replacement, []byte("changed executable"), 0o700); err != nil {
		return err
	}
	if err := os.Rename(replacement, path); err == nil {
		return nil
	}
	if runtime.GOOS != "windows" {
		return fmt.Errorf("replace executable")
	}
	deadline := time.Now().Add(5 * time.Second)
	for {
		if err := os.Remove(path); err == nil || errors.Is(err, os.ErrNotExist) {
			return os.Rename(replacement, path)
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("replace executable timeout")
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func copiedRequest(t *testing.T, runner *Runner, path, mode string) processorprocess.Request {
	t.Helper()
	identity, err := runner.Identify(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	return processorprocess.Request{
		Executable: path, Args: []string{"-test.run=TestProcessorExecHelper", "--", mode},
		Timeout: processorprocess.MaxTimeout, StdoutLimit: 4096, StderrLimit: 1024,
		ExpectedIdentity: identity, EnvironmentContract: processorprocess.EnvironmentRTKIsolatedV2,
	}
}

func TestRunDetectsIdentityChangeImmediatelyBeforeAndAfterStart(t *testing.T) {
	beforePath := copyTestExecutable(t)
	beforeRunner := New()
	beforeRequest := copiedRequest(t, beforeRunner, beforePath, "ok")
	var replaceErr error
	beforeRunner.beforeStart = func(path string) { replaceErr = replaceExecutable(path) }
	result, err := beforeRunner.Run(context.Background(), beforeRequest)
	if replaceErr != nil {
		t.Fatal(replaceErr)
	}
	if result.Attempts != 0 {
		t.Fatalf("before result=%+v", result)
	}
	assertProcessorFault(t, err, fault.KindRejected, "processor_identity_changed", "The processor executable changed before it could be started.", false)

	afterPath := copyTestExecutable(t)
	afterRunner := New()
	afterRequest := copiedRequest(t, afterRunner, afterPath, "ok")
	replaceErr = nil
	afterRunner.afterStart = func(path string) { replaceErr = replaceExecutable(path) }
	result, err = afterRunner.Run(context.Background(), afterRequest)
	if replaceErr != nil {
		t.Fatal(replaceErr)
	}
	if result.Attempts != 1 {
		t.Fatalf("after result=%+v", result)
	}
	assertProcessorFault(t, err, fault.KindRejected, "processor_identity_changed", "The processor executable changed during execution.", false)
}
