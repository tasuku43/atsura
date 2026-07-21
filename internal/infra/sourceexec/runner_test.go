package sourceexec

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
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

func publicCode(t *testing.T, err error) string {
	t.Helper()
	public, ok := fault.PublicCopy(err)
	if !ok {
		t.Fatalf("error is not a public fault: %v", err)
	}
	return public.Code
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
		name   string
		mode   string
		mutate func(*sourceprocess.Request)
		code   string
	}{
		{name: "nonzero", mode: "nonzero", code: "source_command_failed"},
		{name: "timeout", mode: "sleep", mutate: func(value *sourceprocess.Request) { value.Timeout = 30 * time.Millisecond }, code: "source_command_timeout"},
		{name: "stdout bound", mode: "large_stdout", mutate: func(value *sourceprocess.Request) { value.StdoutLimit = 128 }, code: "source_stdout_too_large"},
		{name: "stderr bound", mode: "large_stderr", mutate: func(value *sourceprocess.Request) { value.StderrLimit = 128 }, code: "source_stderr_too_large"},
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
			if got := publicCode(t, err); got != test.code || result.Attempts != 1 {
				t.Fatalf("code = %q, attempts = %d, error = %v", got, result.Attempts, err)
			}
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
	if got := publicCode(t, err); got != "source_executable_not_found" || result.Attempts != 0 {
		t.Fatalf("code = %q, attempts = %d", got, result.Attempts)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	result, err = New().Run(ctx, helperRequest(10*time.Second))
	if !errors.Is(err, context.Canceled) || result.Attempts != 0 {
		t.Fatalf("canceled result = %+v, error = %v", result, err)
	}
}

func TestRunDetectsExecutableDriftBeforeAndAfterStart(t *testing.T) {
	copyExecutable := func(t *testing.T) string {
		t.Helper()
		value, err := os.ReadFile(os.Args[0])
		if err != nil {
			t.Fatal(err)
		}
		path := filepath.Join(t.TempDir(), "helper")
		if err := os.WriteFile(path, value, 0o700); err != nil {
			t.Fatal(err)
		}
		return path
	}
	replace := func(path string) {
		replacement := path + ".replacement"
		_ = os.WriteFile(replacement, []byte("changed executable"), 0o700)
		_ = os.Rename(replacement, path)
	}

	beforePath := copyExecutable(t)
	beforeRequest := helperRequest(10 * time.Second)
	beforeRequest.Executable = beforePath
	result, err := (&Runner{beforeStart: replace}).Run(context.Background(), beforeRequest)
	if got := publicCode(t, err); got != "source_identity_changed" || result.Attempts != 0 {
		t.Fatalf("before drift code = %q, attempts = %d", got, result.Attempts)
	}

	t.Setenv("ATSURA_SOURCEEXEC_HELPER", "1")
	t.Setenv("ATSURA_SOURCEEXEC_MODE", "default")
	afterPath := copyExecutable(t)
	afterRequest := helperRequest(10 * time.Second)
	afterRequest.Executable = afterPath
	result, err = (&Runner{afterStart: replace}).Run(context.Background(), afterRequest)
	if got := publicCode(t, err); got != "source_identity_changed" || result.Attempts != 1 {
		t.Fatalf("after drift code = %q, attempts = %d", got, result.Attempts)
	}
}
