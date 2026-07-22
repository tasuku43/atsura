package specinit

import (
	"context"
	"strings"
	"testing"

	"github.com/tasuku43/atsura/internal/domain/fault"
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
		SchemaVersion: sourcecatalog.SchemaVersion, Adapter: sourcecatalog.Adapter{Kind: "atsura.source.alternate", ContractVersion: 1},
		Source:   sourcecatalog.Source{RequestedExecutable: "fixture", ResolvedPath: "/opt/bin/fixture", SHA256: strings.Repeat("a", 64), Size: 42, Version: "1.0.0"},
		Probe:    sourcecatalog.Probe{IDs: []string{"help", "version"}, Attempts: 2},
		Commands: []sourcecatalog.Command{{Path: []string{"item", "list"}, Summary: "List", Provenance: provenance, Options: []sourcecatalog.Option{{Name: "--json", TakesValue: true}}, StructuredOutput: []sourcecatalog.StructuredOutput{{Format: "json", SelectorFlag: "--json", Fields: []string{"id"}}}}},
	}
}

func TestInitCreatesIncludedIdentityWrapperDraft(t *testing.T) {
	fake := &catalogFake{value: initCatalog(sourcecatalog.ProvenanceVerifiedBuiltin)}
	specification, err := New(fake).Init(context.Background(), operation.Intent{Command: "spec init", Effect: operation.EffectRead}, "catalog", []string{"item", "list"})
	if err != nil || fake.calls != 1 {
		t.Fatalf("specification = %+v, calls = %d, error = %v", specification, fake.calls, err)
	}
	if specification.SchemaVersion != tailoringbundle.SpecificationSchemaVersion || specification.Surface.Default != tailoringbundle.SurfaceDefaultExclude || len(specification.Commands) != 1 {
		t.Fatalf("specification = %+v", specification)
	}
	entry := specification.Commands[0]
	if entry.Presence != tailoringbundle.PresenceInclude || entry.Options == nil || entry.Options.Default != tailoringbundle.SurfaceDefaultInherit || entry.Wrapper == nil || entry.Wrapper.Kind != tailoringbundle.WrapperIdentity {
		t.Fatalf("entry = %+v", entry)
	}
	if entry.Options.Include == nil || entry.Options.Exclude == nil || entry.Wrapper.Before == nil || entry.Wrapper.After == nil || entry.Wrapper.Invoke.AppendArgs == nil {
		t.Fatalf("explicit empty lists were lost: %+v", entry)
	}
	if err := specification.Validate(fake.value); err != nil {
		t.Fatal(err)
	}
}

func TestInitRejectsMissingAndUnverifiedCommands(t *testing.T) {
	for _, test := range []struct {
		name       string
		provenance sourcecatalog.Provenance
		command    []string
		code       string
	}{
		{name: "missing", provenance: sourcecatalog.ProvenanceVerifiedBuiltin, command: []string{"other"}, code: "catalog_command_not_found"},
		{name: "unverified", provenance: sourcecatalog.ProvenanceUnverifiedDynamic, command: []string{"item", "list"}, code: "unverified_catalog_command"},
	} {
		t.Run(test.name, func(t *testing.T) {
			fake := &catalogFake{value: initCatalog(test.provenance)}
			_, err := New(fake).Init(context.Background(), operation.Intent{Command: "spec init", Effect: operation.EffectRead}, "catalog", test.command)
			public, ok := fault.PublicCopy(err)
			if !ok || public.Code != test.code || fake.calls != 1 {
				t.Fatalf("calls = %d, error = %v", fake.calls, err)
			}
		})
	}
}

func TestInitRejectsWrongIntentBeforeLoading(t *testing.T) {
	fake := &catalogFake{value: initCatalog(sourcecatalog.ProvenanceVerifiedBuiltin)}
	_, err := New(fake).Init(context.Background(), operation.Intent{Command: "policy init", Effect: operation.EffectRead}, "catalog", []string{"item", "list"})
	if err == nil || fake.calls != 0 {
		t.Fatalf("calls = %d, error = %v", fake.calls, err)
	}
}
