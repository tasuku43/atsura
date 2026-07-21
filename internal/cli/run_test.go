package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

const sourceHelperModeEnvironment = "ATSURA_SOURCE_HELPER_MODE"

func TestCLISourceHelper(t *testing.T) {
	mode := os.Getenv(sourceHelperModeEnvironment)
	if mode == "" {
		return
	}
	if marker := os.Getenv("ATSURA_SOURCE_HELPER_MARKER"); marker != "" {
		_ = os.WriteFile(marker, []byte("started"), 0o600)
	}
	switch mode {
	case "success":
		_, _ = os.Stdout.WriteString(`[{"number":1,"title":"One","state":"OPEN","ignored":"x"},{"number":2,"title":"Two","state":"CLOSED","ignored":null}]`)
	case "stderr":
		_, _ = os.Stdout.WriteString(`{"number":1,"title":"One","state":"OPEN"}`)
		_, _ = os.Stderr.Write([]byte("warning\n\x1b\xff"))
	case "malformed":
		_, _ = os.Stdout.WriteString(`{"number":`)
	case "failed":
		_, _ = os.Stdout.WriteString("private raw stdout")
		_, _ = os.Stderr.WriteString("private raw stderr")
		os.Exit(7)
	default:
		os.Exit(9)
	}
	os.Exit(0)
}

func runPolicyYAML(executable, decision string) string {
	return `schema_version: 1
effect: read
command:
  executable: ` + strconv.Quote(executable) + `
  args_prefix: ["-test.run=^TestCLISourceHelper$", "--"]
decision: ` + decision + `
reason: Return only the reviewed fields.
invoke:
  append_args: ["--source-json", "$(not-a-shell)"]
output:
  input: json
  select: [number, title, state]
  rename:
    - from: number
      to: id
  render: compact_json
`
}

func runPolicyFile(t *testing.T, decision string) string {
	t.Helper()
	return planPolicyFile(t, runPolicyYAML(os.Args[0], decision))
}

func runSourceArgs(path string) []string {
	return []string{"run", "--config", path, "--", os.Args[0], "-test.run=^TestCLISourceHelper$", "--"}
}

func TestE2ERunExecutesOnceAndReturnsOnlySelectedJSON(t *testing.T) {
	t.Setenv(sourceHelperModeEnvironment, "success")
	marker := filepath.Join(t.TempDir(), "started")
	t.Setenv("ATSURA_SOURCE_HELPER_MARKER", marker)
	path := runPolicyFile(t, "allow")
	var stdout, stderr bytes.Buffer
	command := New(strings.NewReader(""), &stdout, &stderr)
	if code := runCLI(command, runSourceArgs(path)); code != ExitOK {
		t.Fatalf("run code = %d, stderr = %q", code, stderr.String())
	}
	want := `{"schema_version":1,"execution":{"decision":"allow","matched_command":` + strconv.Quote(os.Args[0]+" -test.run=^TestCLISourceHelper$ --") + `,"reason":"Return only the reviewed fields.","result_shape":"array","fields":["id","title","state"],"records":[{"id":1,"title":"One","state":"OPEN"},{"id":2,"title":"Two","state":"CLOSED"}],"source_process_attempts":1}}` + "\n"
	if stdout.String() != want || stderr.Len() != 0 {
		t.Fatalf("stdout = %q, want %q; stderr = %q", stdout.String(), want, stderr.String())
	}
	if _, err := os.Stat(marker); err != nil {
		t.Fatalf("source marker: %v", err)
	}
}

func TestRunDenyAndMismatchStartNoProcess(t *testing.T) {
	t.Setenv(sourceHelperModeEnvironment, "success")
	marker := filepath.Join(t.TempDir(), "started")
	t.Setenv("ATSURA_SOURCE_HELPER_MARKER", marker)

	for _, current := range []struct {
		name      string
		path      string
		args      []string
		wantCode  int
		wantFault string
	}{
		{name: "deny", path: runPolicyFile(t, "deny"), wantCode: ExitRejected, wantFault: "policy_rejected"},
		{name: "mismatch", path: runPolicyFile(t, "allow"), args: []string{"git", "status"}, wantCode: ExitNotFound, wantFault: "plan_rule_not_matched"},
	} {
		t.Run(current.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			command := New(strings.NewReader(""), &stdout, &stderr)
			invocation := runSourceArgs(current.path)
			if current.args != nil {
				invocation = []string{"run", "--config", current.path, "--"}
				invocation = append(invocation, current.args...)
			}
			if code := runCLI(command, invocation); code != current.wantCode {
				t.Fatalf("code = %d, stderr = %q", code, stderr.String())
			}
			if stdout.Len() != 0 || !strings.Contains(stderr.String(), "code: "+current.wantFault) {
				t.Fatalf("stdout = %q, stderr = %q", stdout.String(), stderr.String())
			}
		})
	}
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatalf("source process unexpectedly started: %v", err)
	}
}

func TestRunFailuresDoNotExposeRawSourceOutput(t *testing.T) {
	path := runPolicyFile(t, "allow")
	for _, current := range []struct {
		mode string
		code int
	}{
		{mode: "failed", code: ExitRejected},
		{mode: "malformed", code: ExitContract},
	} {
		t.Run(current.mode, func(t *testing.T) {
			t.Setenv(sourceHelperModeEnvironment, current.mode)
			var stdout, stderr bytes.Buffer
			command := New(strings.NewReader(""), &stdout, &stderr)
			if code := runCLI(command, runSourceArgs(path)); code != current.code {
				t.Fatalf("code = %d, stderr = %q", code, stderr.String())
			}
			if stdout.Len() != 0 || strings.Contains(stderr.String(), "private raw") {
				t.Fatalf("stdout = %q, stderr = %q", stdout.String(), stderr.String())
			}
		})
	}
}

func TestRunProjectsSuccessfulSourceStderr(t *testing.T) {
	t.Setenv(sourceHelperModeEnvironment, "stderr")
	path := runPolicyFile(t, "allow")
	var stdout, stderr bytes.Buffer
	command := New(strings.NewReader(""), &stdout, &stderr)
	if code := runCLI(command, runSourceArgs(path)); code != ExitOK {
		t.Fatalf("code = %d, stderr = %q", code, stderr.String())
	}
	if !json.Valid(stdout.Bytes()) || stderr.String() != `source_stderr: warning\n\u001B\xFF`+"\n" {
		t.Fatalf("stdout = %q, stderr = %q", stdout.String(), stderr.String())
	}
}

func TestRunChecksBothOutputWriters(t *testing.T) {
	t.Setenv(sourceHelperModeEnvironment, "success")
	path := runPolicyFile(t, "allow")
	var stderr bytes.Buffer
	if code := runCLI(New(strings.NewReader(""), shortWriter{}, &stderr), runSourceArgs(path)); code != ExitInternal {
		t.Fatalf("stdout writer failure code = %d, stderr = %q", code, stderr.String())
	}

	t.Setenv(sourceHelperModeEnvironment, "stderr")
	var stdout bytes.Buffer
	if code := runCLI(New(strings.NewReader(""), &stdout, shortWriter{}), runSourceArgs(path)); code != ExitInternal {
		t.Fatalf("stderr writer failure code = %d", code)
	}
}

func TestSafeExternalBytesDistinguishesInvalidAndStructuralBytes(t *testing.T) {
	if got, want := safeExternalBytes([]byte{'a', '\n', 0x1b, 0xff, '\\'}), `a\n\u001B\xFF\\`; got != want {
		t.Fatalf("safeExternalBytes() = %q, want %q", got, want)
	}
}
