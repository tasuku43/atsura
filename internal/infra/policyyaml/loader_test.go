package policyyaml

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tasuku43/atsura/internal/domain/fault"
	"github.com/tasuku43/atsura/internal/domain/operation"
	"github.com/tasuku43/atsura/internal/domain/tailoringbundle"
)

const policyFixture = `schema_version: 2
catalog_digest: aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa
rules:
  - command: [item, delete]
    visibility: hidden
    effect: write
    decision: confirm
    reason: Review deletion.
    append_args: ["--json=id"]
    target:
      kind: item
      argument_index: 0
    impact:
      cardinality: one
      notification: no
      access_change: no
      destructive: yes
    output:
      input: json
      select: [id]
      rename: []
      render: compact_json
  - command: [item, list]
    visibility: visible
    effect: read
    decision: allow
    reason: Compact inventory.
    append_args: ["--json=id,name"]
    output:
      input: json
      select: [id, name]
      rename: []
      render: compact_json
`

func writeFixture(t *testing.T, value string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "policy.yaml")
	if err := os.WriteFile(path, []byte(value), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestLoadStrictSchema2Policy(t *testing.T) {
	policy, err := New().Load(context.Background(), writeFixture(t, policyFixture))
	if err != nil {
		t.Fatal(err)
	}
	if policy.SchemaVersion != 2 || len(policy.Rules) != 2 || policy.Rules[0].Effect != operation.EffectWrite || policy.Rules[0].Decision != tailoringbundle.DecisionConfirm {
		t.Fatalf("policy = %+v", policy)
	}
	if policy.Rules[0].Impact == nil || policy.Rules[0].Impact.Destructive != operation.DeclarationYes {
		t.Fatalf("impact = %+v", policy.Rules[0].Impact)
	}
}

func TestEncodeFailClosedDraftRoundTrips(t *testing.T) {
	policy := tailoringbundle.Policy{SchemaVersion: 2, CatalogDigest: strings.Repeat("a", 64), Rules: []tailoringbundle.Rule{{
		Command: []string{"item", "delete"}, Visibility: tailoringbundle.VisibilityHidden, Effect: operation.EffectWrite,
		Decision: tailoringbundle.DecisionDeny, Reason: "Review and tailor this command before enabling it.", AppendArgs: []string{},
	}}}
	raw, err := Encode(policy)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), "target:") || strings.Contains(string(raw), "output:") || !strings.Contains(string(raw), "append_args: []") {
		t.Fatalf("encoded draft = %s", raw)
	}
	decoded, err := decode(raw)
	if err != nil || decoded.Rules[0].Decision != tailoringbundle.DecisionDeny || decoded.Rules[0].AppendArgs == nil {
		t.Fatalf("decoded = %+v, error = %v", decoded, err)
	}
}

func TestLoadRejectsUnknownAliasMultipleAndOversize(t *testing.T) {
	tests := []struct {
		name  string
		value string
		code  string
	}{
		{name: "unknown", value: strings.Replace(policyFixture, "rules:", "unknown: true\nrules:", 1), code: "invalid_policy_yaml"},
		{name: "alias", value: strings.Replace(policyFixture, "command: [item, delete]", "command: &path [item, delete]", 1) + "extra: *path\n", code: "invalid_policy_yaml"},
		{name: "multiple", value: policyFixture + "---\n" + policyFixture, code: "invalid_policy_yaml"},
		{name: "oversize", value: strings.Repeat("x", int(maxPolicyBytes)+1), code: "policy_file_too_large"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := New().Load(context.Background(), writeFixture(t, test.value))
			public, ok := fault.PublicCopy(err)
			if !ok || public.Code != test.code {
				t.Fatalf("error = %v", err)
			}
		})
	}
}
