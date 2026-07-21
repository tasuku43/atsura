package tailoringyaml

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tasuku43/atsura/internal/domain/fault"
	"github.com/tasuku43/atsura/internal/domain/tailoring"
)

const validYAML = `schema_version: 1
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

func writePolicy(t *testing.T, value string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "policy.yaml")
	if err := os.WriteFile(path, []byte(value), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func faultCode(t *testing.T, err error) string {
	t.Helper()
	public, ok := fault.PublicCopy(err)
	if !ok {
		t.Fatalf("error is not a public fault: %v", err)
	}
	return public.Code
}

func TestLoadReturnsValidatedPolicy(t *testing.T) {
	policy, err := New().Load(context.Background(), writePolicy(t, validYAML))
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if err := policy.Validate(); err != nil {
		t.Fatalf("policy.Validate() = %v", err)
	}
	if policy.Decision != tailoring.DecisionAllow || len(policy.Output.Rename) != 1 {
		t.Fatalf("policy = %+v", policy)
	}
}

func TestLoadRejectsUntrustedYAMLShapes(t *testing.T) {
	tests := map[string]string{
		"unknown field":        strings.Replace(validYAML, "decision: allow", "unknown: value\ndecision: allow", 1),
		"duplicate key":        strings.Replace(validYAML, "decision: allow", "decision: deny\ndecision: allow", 1),
		"unsupported decision": strings.Replace(validYAML, "decision: allow", "decision: confirm", 1),
		"multiple documents":   validYAML + "---\n" + validYAML,
		"alias":                strings.Replace(validYAML, "select: [number, title, state]", "select: &fields [number, title, state]\n  rename: *fields", 1),
	}
	for name, value := range tests {
		t.Run(name, func(t *testing.T) {
			_, err := New().Load(context.Background(), writePolicy(t, value))
			if got := faultCode(t, err); got != "invalid_plan_configuration" {
				t.Fatalf("code = %q", got)
			}
		})
	}
}

func TestLoadRejectsUnsafeOrUnavailableFiles(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "missing.yaml")
	if got := faultCode(t, loadError(missing)); got != "plan_configuration_not_found" {
		t.Fatalf("missing code = %q", got)
	}

	target := writePolicy(t, validYAML)
	link := filepath.Join(t.TempDir(), "link.yaml")
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}
	if got := faultCode(t, loadError(link)); got != "unsafe_plan_configuration" {
		t.Fatalf("symlink code = %q", got)
	}

	oversized := writePolicy(t, strings.Repeat("x", maxConfigurationBytes+1))
	if got := faultCode(t, loadError(oversized)); got != "plan_configuration_too_large" {
		t.Fatalf("oversized code = %q", got)
	}
}

func TestLoadHonorsCanceledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := New().Load(ctx, writePolicy(t, validYAML))
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Load() error = %v", err)
	}
}

func loadError(path string) error {
	_, err := New().Load(context.Background(), path)
	return err
}
