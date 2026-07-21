package policyinit

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

func initCatalog(provenance sourcecatalog.Provenance) sourcecatalog.Catalog {
	return sourcecatalog.Catalog{
		SchemaVersion: 1, Adapter: sourcecatalog.Adapter{Kind: "atsura.source.alternate", ContractVersion: 1},
		Source:   sourcecatalog.Source{RequestedExecutable: "fixture", ResolvedPath: "/opt/bin/fixture", SHA256: strings.Repeat("a", 64), Size: 42, Version: "1.0.0"},
		Probe:    sourcecatalog.Probe{IDs: []string{"help", "version"}, Attempts: 2},
		Commands: []sourcecatalog.Command{{Path: []string{"item", "delete"}, Summary: "Delete", Provenance: provenance, Options: []sourcecatalog.Option{}, StructuredOutput: []sourcecatalog.StructuredOutput{}}},
	}
}

func TestInitCreatesOnlyHiddenDenyDraft(t *testing.T) {
	fake := &catalogFake{value: initCatalog(sourcecatalog.ProvenanceVerifiedBuiltin)}
	policy, err := New(fake).Init(context.Background(), operation.Intent{Command: "policy init", Effect: operation.EffectRead}, "catalog", operation.EffectWrite, []string{"item", "delete"})
	if err != nil || fake.calls != 1 {
		t.Fatalf("policy = %+v, calls = %d, error = %v", policy, fake.calls, err)
	}
	rule := policy.Rules[0]
	if rule.Visibility != tailoringbundle.VisibilityHidden || rule.Decision != tailoringbundle.DecisionDeny || rule.Target != nil || rule.Impact != nil || rule.Output != nil || rule.AppendArgs == nil {
		t.Fatalf("rule = %+v", rule)
	}
	if err := policy.Validate(fake.value); err != nil {
		t.Fatal(err)
	}
}

func TestInitRejectsMissingAndUnverifiedCommands(t *testing.T) {
	for _, test := range []struct {
		name       string
		provenance sourcecatalog.Provenance
		command    []string
	}{
		{name: "missing", provenance: sourcecatalog.ProvenanceVerifiedBuiltin, command: []string{"other"}},
		{name: "unverified", provenance: sourcecatalog.ProvenanceUnverifiedDynamic, command: []string{"item", "delete"}},
	} {
		t.Run(test.name, func(t *testing.T) {
			fake := &catalogFake{value: initCatalog(test.provenance)}
			_, err := New(fake).Init(context.Background(), operation.Intent{Command: "policy init", Effect: operation.EffectRead}, "catalog", operation.EffectWrite, test.command)
			if err == nil || fake.calls != 1 {
				t.Fatalf("calls = %d, error = %v", fake.calls, err)
			}
		})
	}
}
