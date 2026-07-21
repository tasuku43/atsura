package bundlebuild

import (
	"context"
	"strings"
	"testing"

	"github.com/tasuku43/atsura/internal/domain/operation"
	"github.com/tasuku43/atsura/internal/domain/sourcecatalog"
	"github.com/tasuku43/atsura/internal/domain/tailoringbundle"
)

type catalogFake struct {
	value sourcecatalog.Catalog
	calls int
}

func (f *catalogFake) Load(context.Context, string) (sourcecatalog.Catalog, error) {
	f.calls++
	return f.value, nil
}

type policyFake struct {
	value tailoringbundle.Policy
	calls int
}

func (f *policyFake) Load(context.Context, string) (tailoringbundle.Policy, error) {
	f.calls++
	return f.value, nil
}

func fixtures(t *testing.T) (sourcecatalog.Catalog, tailoringbundle.Policy) {
	t.Helper()
	catalog := sourcecatalog.Catalog{
		SchemaVersion: 1, Adapter: sourcecatalog.Adapter{Kind: "atsura.source.alternate", ContractVersion: 1},
		Source:   sourcecatalog.Source{RequestedExecutable: "fixture", ResolvedPath: "/opt/bin/fixture", SHA256: strings.Repeat("a", 64), Size: 42, Version: "1.0.0"},
		Probe:    sourcecatalog.Probe{IDs: []string{"help", "version"}, Attempts: 2},
		Commands: []sourcecatalog.Command{{Path: []string{"item", "list"}, Summary: "List", Provenance: sourcecatalog.ProvenanceVerifiedBuiltin, Options: []sourcecatalog.Option{{Name: "--json", TakesValue: true}}, StructuredOutput: []sourcecatalog.StructuredOutput{{Format: "json", SelectorFlag: "--json", Fields: []string{"id"}}}}},
	}
	digest, _ := catalog.Digest()
	policy := tailoringbundle.Policy{SchemaVersion: 2, CatalogDigest: digest, Rules: []tailoringbundle.Rule{{
		Command: []string{"item", "list"}, Visibility: tailoringbundle.VisibilityVisible, Effect: operation.EffectRead,
		Decision: tailoringbundle.DecisionAllow, Reason: "List compact items.", AppendArgs: []string{"--json=id"},
		Output: &tailoringbundle.Output{Input: "json", Select: []string{"id"}, Rename: []tailoringbundle.Rename{}, Render: "compact_json"},
	}}}
	return catalog, policy
}

func TestValidateAndBuildUseSameInputs(t *testing.T) {
	catalog, policy := fixtures(t)
	catalogs := &catalogFake{value: catalog}
	policies := &policyFake{value: policy}
	service := New(catalogs, policies)
	validated, err := service.ValidatePolicy(context.Background(), operation.Intent{Command: "policy validate", Effect: operation.EffectRead}, "catalog", "policy")
	if err != nil || len(validated.PolicyDigest) != 64 || validated.RuleCount != 1 || validated.VisibleCount != 1 {
		t.Fatalf("validated = %+v, error = %v", validated, err)
	}
	built, err := service.Build(context.Background(), operation.Intent{Command: "bundle build", Effect: operation.EffectRead}, "catalog", "policy")
	if err != nil || len(built.BundleDigest) != 64 || built.Bundle.PolicyDigest != validated.PolicyDigest {
		t.Fatalf("built = %+v, error = %v", built, err)
	}
	if catalogs.calls != 2 || policies.calls != 2 {
		t.Fatalf("calls = catalog %d, policy %d", catalogs.calls, policies.calls)
	}
}

func TestInvalidIntentAndPolicyFailBeforeLaterAuthority(t *testing.T) {
	catalog, policy := fixtures(t)
	catalogs := &catalogFake{value: catalog}
	policies := &policyFake{value: policy}
	service := New(catalogs, policies)
	_, err := service.Build(context.Background(), operation.Intent{Command: "run", Effect: operation.EffectRead}, "catalog", "policy")
	if err == nil || catalogs.calls != 0 || policies.calls != 0 {
		t.Fatalf("error = %v, calls = %d/%d", err, catalogs.calls, policies.calls)
	}
	policies.value.CatalogDigest = strings.Repeat("b", 64)
	_, err = service.Build(context.Background(), operation.Intent{Command: "bundle build", Effect: operation.EffectRead}, "catalog", "policy")
	if err == nil || catalogs.calls != 1 || policies.calls != 1 {
		t.Fatalf("error = %v, calls = %d/%d", err, catalogs.calls, policies.calls)
	}
}
