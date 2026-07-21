package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const planPreviewYAML = `schema_version: 1
command:
  executable: gh
  args_prefix: [pr, list]
decision: allow
reason: Return only pull request identity and status.
invoke:
  append_args: ["--json=number,title,state"]
output:
  input: json
  select: [number, title, state]
  rename:
    - from: number
      to: id
  render: compact_json
`

func planPolicyFile(t *testing.T, value string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "plan.yaml")
	if err := os.WriteFile(path, []byte(value), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestE2EPlanPreviewReturnsDeterministicPlanWithoutStartingSource(t *testing.T) {
	path := planPolicyFile(t, planPreviewYAML)
	var first []byte
	for attempt := 0; attempt < 2; attempt++ {
		var stdout, stderr bytes.Buffer
		command := New(strings.NewReader(""), &stdout, &stderr)
		args := []string{"plan", "preview", "--config", path, "--", "gh", "pr", "list", "--state", "open"}
		if code := runCLI(command, args); code != ExitOK {
			t.Fatalf("plan preview code = %d, stderr = %q", code, stderr.String())
		}
		if stderr.Len() != 0 {
			t.Fatalf("stderr = %q", stderr.String())
		}
		if attempt == 0 {
			first = append([]byte(nil), stdout.Bytes()...)
		} else if !bytes.Equal(first, stdout.Bytes()) {
			t.Fatalf("preview is not deterministic:\n%s\n%s", first, stdout.Bytes())
		}
	}

	var document planDocument
	if err := json.Unmarshal(first, &document); err != nil {
		t.Fatal(err)
	}
	if !document.Plan.Executable || document.Plan.SourceProcessAttempts != 0 {
		t.Fatalf("plan = %+v", document.Plan)
	}
	want := []string{"gh", "pr", "list", "--state", "open", "--json=number,title,state"}
	if strings.Join(document.Plan.TransformedArgv, "\x00") != strings.Join(want, "\x00") {
		t.Fatalf("transformed argv = %v, want %v", document.Plan.TransformedArgv, want)
	}
}

func TestPlanPreviewDenyProducesNoExecutableArgv(t *testing.T) {
	value := strings.Replace(planPreviewYAML, "decision: allow", "decision: deny", 1)
	path := planPolicyFile(t, value)
	var stdout, stderr bytes.Buffer
	command := New(strings.NewReader(""), &stdout, &stderr)
	if code := runCLI(command, []string{"plan", "preview", "--config", path, "--", "gh", "pr", "list"}); code != ExitOK {
		t.Fatalf("code = %d, stderr = %q", code, stderr.String())
	}
	var document planDocument
	if err := json.Unmarshal(stdout.Bytes(), &document); err != nil {
		t.Fatal(err)
	}
	if document.Plan.Executable || document.Plan.TransformedArgv == nil || len(document.Plan.TransformedArgv) != 0 {
		t.Fatalf("deny plan = %+v", document.Plan)
	}
}

func TestPlanPreviewMismatchUsesDeclaredNotFoundFault(t *testing.T) {
	path := planPolicyFile(t, planPreviewYAML)
	var stdout, stderr bytes.Buffer
	command := New(strings.NewReader(""), &stdout, &stderr)
	args := []string{"--error-format=json", "plan", "preview", "--config", path, "--", "git", "status"}
	if code := runCLI(command, args); code != ExitNotFound {
		t.Fatalf("code = %d, stderr = %q", code, stderr.String())
	}
	if stdout.Len() != 0 || !strings.Contains(stderr.String(), `"code":"plan_rule_not_matched"`) {
		t.Fatalf("stdout = %q, stderr = %q", stdout.String(), stderr.String())
	}
}

func TestPlanPreviewProjectsHostileReasonAndChecksOutputWrite(t *testing.T) {
	value := strings.Replace(planPreviewYAML, "reason: Return only pull request identity and status.", "reason: |\n  line one\n  line two", 1)
	path := planPolicyFile(t, value)
	var stdout, stderr bytes.Buffer
	command := New(strings.NewReader(""), &stdout, &stderr)
	if code := runCLI(command, []string{"plan", "preview", "--config", path, "--", "gh", "pr", "list"}); code != ExitOK {
		t.Fatalf("code = %d, stderr = %q", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), `"reason":"line one\\nline two\\n"`) {
		t.Fatalf("unsafe reason was not visibly escaped: %s", stdout.String())
	}

	stderr.Reset()
	command = New(strings.NewReader(""), shortWriter{}, &stderr)
	if code := runCLI(command, []string{"plan", "preview", "--config", path, "--", "gh", "pr", "list"}); code != ExitInternal {
		t.Fatalf("writer failure code = %d, stderr = %q", code, stderr.String())
	}
}
