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

type specificationFake struct {
	value tailoringbundle.Specification
	calls int
}

func (f *specificationFake) Load(context.Context, string) (tailoringbundle.Specification, error) {
	f.calls++
	return f.value, nil
}

func fixtures(t *testing.T) (sourcecatalog.Catalog, tailoringbundle.Specification) {
	t.Helper()
	catalog := sourcecatalog.Catalog{
		SchemaVersion: sourcecatalog.SchemaVersion, Adapter: sourcecatalog.Adapter{Kind: "atsura.source.alternate", ContractVersion: 1},
		Source:   sourcecatalog.Source{RequestedExecutable: "fixture", ResolvedPath: "/opt/bin/fixture", SHA256: strings.Repeat("a", 64), Size: 42, Version: "1.0.0"},
		Probe:    sourcecatalog.Probe{IDs: []string{"help", "version"}, Attempts: 2},
		Commands: []sourcecatalog.Command{{Path: []string{"item", "list"}, Summary: "List", Provenance: sourcecatalog.ProvenanceVerifiedBuiltin, Options: []sourcecatalog.Option{{Name: "--json", TakesValue: true}}, StructuredOutput: []sourcecatalog.StructuredOutput{{Format: "json", SelectorFlag: "--json", Fields: []string{"id"}}}}},
	}
	digest, _ := catalog.Digest()
	specification := tailoringbundle.Specification{
		SchemaVersion: tailoringbundle.SpecificationSchemaVersion,
		CatalogDigest: digest,
		Surface:       tailoringbundle.Surface{Default: tailoringbundle.SurfaceDefaultExclude},
		Commands: []tailoringbundle.CommandEntry{{
			Command: []string{"item", "list"}, Presence: tailoringbundle.PresenceInclude, Reason: "List compact items.",
			Options: &tailoringbundle.OptionSurface{Default: tailoringbundle.SurfaceDefaultInherit, Include: []string{}, Exclude: []string{}},
			Wrapper: &tailoringbundle.Wrapper{Kind: tailoringbundle.WrapperIdentity, Before: []tailoringbundle.StageAction{}, Invoke: tailoringbundle.Invocation{AppendArgs: []string{}}, After: []tailoringbundle.StageAction{}},
		}},
	}
	return catalog, specification
}

func TestValidateAndBuildUseSameInputs(t *testing.T) {
	catalog, specification := fixtures(t)
	catalogs := &catalogFake{value: catalog}
	specifications := &specificationFake{value: specification}
	service := New(catalogs, specifications)
	validated, err := service.ValidateSpecification(context.Background(), operation.Intent{Command: "spec validate", Effect: operation.EffectRead}, "catalog", "specification")
	if err != nil || len(validated.SpecificationDigest) != 64 || validated.CommandCount != 1 || validated.IncludedCount != 1 || validated.IdentityWrapperCount != 1 || validated.TransformWrapperCount != 0 {
		t.Fatalf("validated = %+v, error = %v", validated, err)
	}
	built, err := service.Build(context.Background(), operation.Intent{Command: "bundle build", Effect: operation.EffectRead}, "catalog", "specification")
	if err != nil || len(built.BundleDigest) != 64 || built.Bundle.SpecificationDigest != validated.SpecificationDigest {
		t.Fatalf("built = %+v, error = %v", built, err)
	}
	if catalogs.calls != 2 || specifications.calls != 2 {
		t.Fatalf("calls = catalog %d, specification %d", catalogs.calls, specifications.calls)
	}
}

func TestInvalidIntentAndSpecificationFailBeforeLaterAuthority(t *testing.T) {
	catalog, specification := fixtures(t)
	catalogs := &catalogFake{value: catalog}
	specifications := &specificationFake{value: specification}
	service := New(catalogs, specifications)
	_, err := service.Build(context.Background(), operation.Intent{Command: "run", Effect: operation.EffectRead}, "catalog", "specification")
	if err == nil || catalogs.calls != 0 || specifications.calls != 0 {
		t.Fatalf("error = %v, calls = %d/%d", err, catalogs.calls, specifications.calls)
	}
	specifications.value.CatalogDigest = strings.Repeat("b", 64)
	_, err = service.Build(context.Background(), operation.Intent{Command: "bundle build", Effect: operation.EffectRead}, "catalog", "specification")
	if err == nil || catalogs.calls != 1 || specifications.calls != 1 {
		t.Fatalf("error = %v, calls = %d/%d", err, catalogs.calls, specifications.calls)
	}
}
