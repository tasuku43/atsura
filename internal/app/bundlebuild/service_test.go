package bundlebuild

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tasuku43/atsura/internal/app/processorcompat"
	"github.com/tasuku43/atsura/internal/domain/fault"
	"github.com/tasuku43/atsura/internal/domain/operation"
	"github.com/tasuku43/atsura/internal/domain/processorprocess"
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

type observationFake struct {
	value processorprocess.Observation
	err   error
	calls int
	path  string
}

func (f *observationFake) Load(_ context.Context, path string) (processorprocess.Observation, error) {
	f.calls++
	f.path = path
	return f.value, f.err
}

type compatibilityFake struct {
	registry     *processorcompat.Registry
	bindingErr   error
	verifyErr    error
	bindingCalls int
	verifyCalls  int
}

func (f *compatibilityFake) Binding(observation processorprocess.Observation) (tailoringbundle.ProcessorBinding, error) {
	f.bindingCalls++
	if f.bindingErr != nil {
		return tailoringbundle.ProcessorBinding{}, f.bindingErr
	}
	return f.registry.Binding(observation)
}

func (f *compatibilityFake) VerifySurface(bundle tailoringbundle.Bundle) error {
	f.verifyCalls++
	if f.verifyErr != nil {
		return f.verifyErr
	}
	return f.registry.VerifySurface(bundle)
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
	if built.Bundle.Processors == nil || len(built.Bundle.Processors) != 0 {
		t.Fatalf("identity bundle processor bindings = %+v", built.Bundle.Processors)
	}
	if catalogs.calls != 2 || specifications.calls != 2 {
		t.Fatalf("calls = catalog %d, specification %d", catalogs.calls, specifications.calls)
	}
}

func TestBuildBindsExactProcessorObservationIntoOptimizerBundle(t *testing.T) {
	catalog := optimizerCatalog(t)
	observation := optimizerObservation(t)
	registry := processorcompat.New()
	entry, err := registry.DefaultEntry(catalog, observation)
	if err != nil {
		t.Fatal(err)
	}
	specification := specificationFor(t, catalog, entry)
	catalogs := &catalogFake{value: catalog}
	specifications := &specificationFake{value: specification}
	observations := &observationFake{value: observation}
	compatibility := &compatibilityFake{registry: registry}
	service := New(catalogs, specifications, ProcessorSupport{Observations: observations, Compatibility: compatibility})

	validated, err := service.ValidateSpecification(context.Background(), operation.Intent{Command: "spec validate", Effect: operation.EffectRead}, "catalog", "specification")
	if err != nil || validated.TransformWrapperCount != 1 {
		t.Fatalf("validated = %+v, error = %v", validated, err)
	}
	if observations.calls != 0 || compatibility.bindingCalls != 0 || compatibility.verifyCalls != 0 {
		t.Fatalf("spec validation consulted processor evidence: observation %d, binding %d, verify %d", observations.calls, compatibility.bindingCalls, compatibility.verifyCalls)
	}

	built, err := service.Build(
		context.Background(), operation.Intent{Command: "bundle build", Effect: operation.EffectRead},
		"catalog", "specification", "processor.json",
	)
	if err != nil {
		t.Fatal(err)
	}
	if observations.calls != 1 || observations.path != "processor.json" || compatibility.bindingCalls != 1 || compatibility.verifyCalls != 1 {
		t.Fatalf("calls = observation %d/%q, binding %d, verify %d", observations.calls, observations.path, compatibility.bindingCalls, compatibility.verifyCalls)
	}
	if len(built.Bundle.Processors) != 1 || built.Bundle.Processors[0].Contract != processorcompat.ContractID || len(built.BundleDigest) != 64 {
		t.Fatalf("built = %+v", built)
	}
	if err := built.Bundle.Validate(); err != nil {
		t.Fatal(err)
	}
}

func TestBuildProcessorEvidenceFailureMatrix(t *testing.T) {
	intent := operation.Intent{Command: "bundle build", Effect: operation.EffectRead}
	catalog := optimizerCatalog(t)
	observation := optimizerObservation(t)
	registry := processorcompat.New()
	entry, err := registry.DefaultEntry(catalog, observation)
	if err != nil {
		t.Fatal(err)
	}
	optimizerSpecification := specificationFor(t, catalog, entry)

	t.Run("optimizer requires evidence", func(t *testing.T) {
		catalogs := &catalogFake{value: catalog}
		specifications := &specificationFake{value: optimizerSpecification}
		observations := &observationFake{value: observation}
		compatibility := &compatibilityFake{registry: registry}
		_, err := New(catalogs, specifications, ProcessorSupport{Observations: observations, Compatibility: compatibility}).Build(
			context.Background(), intent, "catalog", "specification",
		)
		assertPublicCode(t, err, "processor_observation_required")
		if observations.calls != 0 || compatibility.bindingCalls != 0 || compatibility.verifyCalls != 0 {
			t.Fatalf("processor calls = observation %d, binding %d, verify %d", observations.calls, compatibility.bindingCalls, compatibility.verifyCalls)
		}
	})

	t.Run("identity rejects extraneous evidence", func(t *testing.T) {
		identityCatalog, identitySpecification := fixtures(t)
		observations := &observationFake{err: errors.New("must not load")}
		compatibility := &compatibilityFake{registry: registry}
		_, err := New(
			&catalogFake{value: identityCatalog}, &specificationFake{value: identitySpecification},
			ProcessorSupport{Observations: observations, Compatibility: compatibility},
		).Build(context.Background(), intent, "catalog", "specification", "processor")
		assertPublicCode(t, err, "processor_observation_not_used")
		if observations.calls != 0 || compatibility.bindingCalls != 0 || compatibility.verifyCalls != 0 {
			t.Fatalf("processor calls = observation %d, binding %d, verify %d", observations.calls, compatibility.bindingCalls, compatibility.verifyCalls)
		}
	})

	t.Run("projection rejects extraneous evidence", func(t *testing.T) {
		projectionCatalog, projectionSpecification := fixtures(t)
		projectionSpecification.Commands[0].Wrapper = &tailoringbundle.Wrapper{
			Kind:   tailoringbundle.WrapperTransform,
			Before: []tailoringbundle.StageAction{},
			Invoke: tailoringbundle.Invocation{AppendArgs: []string{"--json=id"}},
			Output: &tailoringbundle.Output{
				Kind: tailoringbundle.OutputKindProjection,
				Projection: &tailoringbundle.Projection{
					Input: "json", Select: []string{"id"}, Rename: []tailoringbundle.Rename{}, Render: "compact_json",
				},
			},
			After: []tailoringbundle.StageAction{},
		}
		observations := &observationFake{err: errors.New("must not load")}
		compatibility := &compatibilityFake{registry: registry}
		_, err := New(
			&catalogFake{value: projectionCatalog}, &specificationFake{value: projectionSpecification},
			ProcessorSupport{Observations: observations, Compatibility: compatibility},
		).Build(context.Background(), intent, "catalog", "specification", "processor")
		assertPublicCode(t, err, "processor_observation_not_used")
		if observations.calls != 0 || compatibility.bindingCalls != 0 || compatibility.verifyCalls != 0 {
			t.Fatalf("processor calls = observation %d, binding %d, verify %d", observations.calls, compatibility.bindingCalls, compatibility.verifyCalls)
		}
	})

	t.Run("loader fault is preserved", func(t *testing.T) {
		observations := &observationFake{err: fault.New(fault.KindNotFound, "processor_observation_file_not_found", "The processor observation JSON was not found.", false, processorHelp())}
		compatibility := &compatibilityFake{registry: registry}
		_, err := New(
			&catalogFake{value: catalog}, &specificationFake{value: optimizerSpecification},
			ProcessorSupport{Observations: observations, Compatibility: compatibility},
		).Build(context.Background(), intent, "catalog", "specification", "processor")
		assertPublicCode(t, err, "processor_observation_file_not_found")
		if observations.calls != 1 || compatibility.bindingCalls != 0 || compatibility.verifyCalls != 0 {
			t.Fatalf("processor calls = observation %d, binding %d, verify %d", observations.calls, compatibility.bindingCalls, compatibility.verifyCalls)
		}
	})

	t.Run("binding rejection", func(t *testing.T) {
		observations := &observationFake{value: observation}
		compatibility := &compatibilityFake{registry: registry, bindingErr: errors.New("not admitted")}
		_, err := New(
			&catalogFake{value: catalog}, &specificationFake{value: optimizerSpecification},
			ProcessorSupport{Observations: observations, Compatibility: compatibility},
		).Build(context.Background(), intent, "catalog", "specification", "processor")
		assertPublicCode(t, err, "processor_compatibility_not_admitted")
		if compatibility.bindingCalls != 1 || compatibility.verifyCalls != 0 {
			t.Fatalf("binding/verify calls = %d/%d", compatibility.bindingCalls, compatibility.verifyCalls)
		}
	})

	t.Run("complete surface rejection", func(t *testing.T) {
		observations := &observationFake{value: observation}
		compatibility := &compatibilityFake{registry: registry, verifyErr: errors.New("surface not admitted")}
		_, err := New(
			&catalogFake{value: catalog}, &specificationFake{value: optimizerSpecification},
			ProcessorSupport{Observations: observations, Compatibility: compatibility},
		).Build(context.Background(), intent, "catalog", "specification", "processor")
		assertPublicCode(t, err, "processor_compatibility_not_admitted")
		if compatibility.bindingCalls != 1 || compatibility.verifyCalls != 1 {
			t.Fatalf("binding/verify calls = %d/%d", compatibility.bindingCalls, compatibility.verifyCalls)
		}
	})

	t.Run("invalid selection fails before loading", func(t *testing.T) {
		catalogs := &catalogFake{value: catalog}
		specifications := &specificationFake{value: optimizerSpecification}
		for _, selection := range [][]string{{" \t"}, {"one", "two"}} {
			_, err := New(catalogs, specifications).Build(context.Background(), intent, "catalog", "specification", selection...)
			assertPublicCode(t, err, "invalid_processor_observation_selection")
			if catalogs.calls != 0 || specifications.calls != 0 {
				t.Fatalf("load calls = %d/%d", catalogs.calls, specifications.calls)
			}
		}
	})
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

func optimizerCatalog(t *testing.T) sourcecatalog.Catalog {
	t.Helper()
	return sourcecatalog.Catalog{
		SchemaVersion: sourcecatalog.SchemaVersion,
		Adapter:       sourcecatalog.Adapter{Kind: processorcompat.SourceAdapterKind, ContractVersion: processorcompat.SourceContractVersion},
		Source: sourcecatalog.Source{
			RequestedExecutable: "go", ResolvedPath: filepath.Join(t.TempDir(), "go"), SHA256: strings.Repeat("b", 64), Size: 1024, Version: "go1.26.5",
		},
		Probe: sourcecatalog.Probe{IDs: []string{"help", "test_help", "version"}, Attempts: 3},
		Commands: []sourcecatalog.Command{{
			Path: []string{"test"}, Summary: "test packages", Provenance: sourcecatalog.ProvenanceVerifiedBuiltin,
			Options: []sourcecatalog.Option{},
			StructuredOutput: []sourcecatalog.StructuredOutput{{
				Format: processorcompat.InputFormat, SelectorFlag: "-json",
				Fields: []string{"Action", "Elapsed", "FailedBuild", "Output", "Package", "Test", "Time"},
			}},
		}},
	}
}

func optimizerObservation(t *testing.T) processorprocess.Observation {
	t.Helper()
	return processorprocess.Observation{
		SchemaVersion: processorprocess.ObservationSchemaVersion,
		Adapter:       processorprocess.Adapter{Kind: processorcompat.ProcessorAdapterKind, ContractVersion: processorcompat.ProcessorContractVersion},
		Platform:      processorprocess.Platform{OS: "darwin", Arch: "arm64"},
		Identity: processorprocess.Identity{
			ResolvedPath: filepath.Join(t.TempDir(), "rtk"),
			SHA256:       "2dab449f32ea744c30b02a3ef9806e3e7d3b356a145332f3f2aaabb5ea48edee",
			Size:         7763408,
		},
		Version: processorcompat.ProcessorVersion,
		Probe: processorprocess.Probe{
			Argv: []string{"--version"}, EnvironmentContract: processorprocess.EnvironmentRTKIsolatedV1, Attempts: 1,
		},
	}
}

func specificationFor(t *testing.T, catalog sourcecatalog.Catalog, entry tailoringbundle.CommandEntry) tailoringbundle.Specification {
	t.Helper()
	digest, err := catalog.Digest()
	if err != nil {
		t.Fatal(err)
	}
	return tailoringbundle.Specification{
		SchemaVersion: tailoringbundle.SpecificationSchemaVersion,
		CatalogDigest: digest,
		Surface:       tailoringbundle.Surface{Default: tailoringbundle.SurfaceDefaultExclude},
		Commands:      []tailoringbundle.CommandEntry{entry},
	}
}

func assertPublicCode(t *testing.T, err error, code string) {
	t.Helper()
	public, ok := fault.PublicCopy(err)
	if !ok || public.Code != code || public.Retryable {
		t.Fatalf("fault = %+v, error = %v, want code %q", public, err, code)
	}
}
