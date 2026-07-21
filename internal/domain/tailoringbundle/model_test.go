package tailoringbundle

import (
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/tasuku43/atsura/internal/domain/operation"
	"github.com/tasuku43/atsura/internal/domain/sourcecatalog"
)

func bundleCatalog() sourcecatalog.Catalog {
	return sourcecatalog.Catalog{
		SchemaVersion: 1,
		Adapter:       sourcecatalog.Adapter{Kind: "atsura.source.alternate", ContractVersion: 1},
		Source:        sourcecatalog.Source{RequestedExecutable: "fixture", ResolvedPath: "/opt/bin/fixture", SHA256: strings.Repeat("a", 64), Size: 42, Version: "1.0.0"},
		Probe:         sourcecatalog.Probe{IDs: []string{"help", "version"}, Attempts: 2},
		Commands: []sourcecatalog.Command{
			{Path: []string{"item", "delete"}, Summary: "Delete item", Provenance: sourcecatalog.ProvenanceVerifiedBuiltin, Options: []sourcecatalog.Option{{Name: "--json", TakesValue: true}}, StructuredOutput: []sourcecatalog.StructuredOutput{{Format: "json", SelectorFlag: "--json", Fields: []string{"id"}}}},
			{Path: []string{"item", "list"}, Summary: "List items", Provenance: sourcecatalog.ProvenanceVerifiedBuiltin, Options: []sourcecatalog.Option{{Name: "--json", TakesValue: true}}, StructuredOutput: []sourcecatalog.StructuredOutput{{Format: "json", SelectorFlag: "--json", Fields: []string{"id", "name"}}}},
			{Path: []string{"plugin", "run"}, Summary: "Dynamic plugin", Provenance: sourcecatalog.ProvenanceUnverifiedDynamic, Options: []sourcecatalog.Option{}, StructuredOutput: []sourcecatalog.StructuredOutput{}},
		},
	}
}

func output(fields ...string) *Output {
	return &Output{Input: "json", Select: fields, Rename: []Rename{}, Render: "compact_json"}
}

func bundlePolicy(t *testing.T) Policy {
	t.Helper()
	catalog := bundleCatalog()
	digest, err := catalog.Digest()
	if err != nil {
		t.Fatal(err)
	}
	index := 0
	return Policy{SchemaVersion: 2, CatalogDigest: digest, Rules: []Rule{
		{Command: []string{"item", "delete"}, Visibility: VisibilityHidden, Effect: operation.EffectWrite, Decision: DecisionConfirm, Reason: "Deletion requires review.", AppendArgs: []string{"--json", "id"}, Target: &TargetBinding{Kind: "item", ArgumentIndex: &index}, Impact: &operation.Impact{Cardinality: operation.CardinalityOne, Notification: operation.DeclarationNo, AccessChange: operation.DeclarationNo, Destructive: operation.DeclarationYes}, Output: output("id")},
		{Command: []string{"item", "list"}, Visibility: VisibilityVisible, Effect: operation.EffectRead, Decision: DecisionAllow, Reason: "Expose a compact inventory.", AppendArgs: []string{"--json", "id,name"}, Output: output("id", "name")},
	}}
}

func TestCompileProducesOneDeterministicVendorNeutralBundle(t *testing.T) {
	catalog := bundleCatalog()
	policy := bundlePolicy(t)
	first, err := Compile(catalog, policy)
	if err != nil {
		t.Fatal(err)
	}
	second, err := Compile(catalog, policy)
	if err != nil {
		t.Fatal(err)
	}
	firstBytes, _ := first.CanonicalJSON()
	secondBytes, _ := second.CanonicalJSON()
	firstDigest, _ := first.Digest()
	secondDigest, _ := second.Digest()
	if string(firstBytes) != string(secondBytes) || firstDigest != secondDigest || len(firstDigest) != 64 {
		t.Fatalf("bundle identity mismatch: %q %q", firstDigest, secondDigest)
	}
	if len(first.Surface) != 1 || strings.Join(first.Surface[0].Command, " ") != "item list" {
		t.Fatalf("surface = %+v", first.Surface)
	}
	if strings.Contains(string(firstBytes), "github") || strings.Contains(string(firstBytes), "claude") || strings.Contains(string(firstBytes), "timestamp") {
		t.Fatalf("bundle leaked vendor or ambient fields: %s", firstBytes)
	}
}

func TestPolicyFailsClosedForUnverifiedMutationAndTransformGaps(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*Policy)
	}{
		{name: "unverified command", mutate: func(p *Policy) { p.Rules[1].Command = []string{"plugin", "run"} }},
		{name: "mutation allow", mutate: func(p *Policy) { p.Rules[0].Decision = DecisionAllow }},
		{name: "mutation target", mutate: func(p *Policy) { p.Rules[0].Target = nil }},
		{name: "mutation impact", mutate: func(p *Policy) { p.Rules[0].Impact = nil }},
		{name: "read confirm", mutate: func(p *Policy) { p.Rules[1].Decision = DecisionConfirm }},
		{name: "read target", mutate: func(p *Policy) { index := 0; p.Rules[1].Target = &TargetBinding{Kind: "item", ArgumentIndex: &index} }},
		{name: "missing output", mutate: func(p *Policy) { p.Rules[1].Output = nil }},
		{name: "deny transforms", mutate: func(p *Policy) { p.Rules[1].Decision = DecisionDeny }},
		{name: "unsorted", mutate: func(p *Policy) { p.Rules[0], p.Rules[1] = p.Rules[1], p.Rules[0] }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			policy := bundlePolicy(t)
			test.mutate(&policy)
			if err := policy.Validate(bundleCatalog()); !errors.Is(err, ErrInvalidPolicy) {
				t.Fatalf("Validate() error = %v", err)
			}
		})
	}
}

func TestDeniedMutationNeedsNoTargetInference(t *testing.T) {
	policy := bundlePolicy(t)
	rule := &policy.Rules[0]
	rule.Decision = DecisionDeny
	rule.AppendArgs = []string{}
	rule.Target = nil
	rule.Impact = nil
	rule.Output = nil
	if err := policy.Validate(bundleCatalog()); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
}

func TestBundleDetectsCatalogPolicyAndSurfaceDrift(t *testing.T) {
	bundle, err := Compile(bundleCatalog(), bundlePolicy(t))
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name   string
		mutate func(*Bundle)
	}{
		{name: "catalog", mutate: func(b *Bundle) { b.Catalog.Source.Version = "1.0.1" }},
		{name: "policy", mutate: func(b *Bundle) { b.Policy.Rules[1].Reason = "Changed" }},
		{name: "surface", mutate: func(b *Bundle) { b.Surface[0].Reason = "Changed" }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			copy := bundle
			copy.Catalog = sourcecatalog.Sort(bundle.Catalog)
			copy.Policy = SortPolicy(bundle.Policy)
			copy.Surface = append([]SurfaceEntry(nil), bundle.Surface...)
			test.mutate(&copy)
			if err := copy.Validate(); !errors.Is(err, ErrInvalidBundle) {
				t.Fatalf("Validate() error = %v", err)
			}
		})
	}
}

func TestSortPolicyDetachesAndOrdersRules(t *testing.T) {
	policy := bundlePolicy(t)
	policy.Rules[0], policy.Rules[1] = policy.Rules[1], policy.Rules[0]
	sorted := SortPolicy(policy)
	if strings.Join(sorted.Rules[0].Command, " ") != "item delete" || reflect.DeepEqual(sorted.Rules, policy.Rules) {
		t.Fatalf("sorted = %+v", sorted.Rules)
	}
	sorted.Rules[0].Command[0] = "changed"
	if policy.Rules[1].Command[0] != "item" {
		t.Fatal("SortPolicy mutated input")
	}
}
