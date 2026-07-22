package specinit

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

type defaultFake struct {
	entry tailoringbundle.CommandEntry
	err   error
	calls int
}

func (f *defaultFake) DefaultEntry(sourcecatalog.Catalog, processorprocess.Observation) (tailoringbundle.CommandEntry, error) {
	f.calls++
	return f.entry, f.err
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

func TestInitMaterializesExactProcessorDefaultOnlyFromExplicitEvidence(t *testing.T) {
	catalog := goCatalog(t)
	catalogs := &catalogFake{value: catalog}
	observations := &observationFake{value: rtkObservation(t)}
	service := New(catalogs, ProcessorSupport{Observations: observations, Compatibility: processorcompat.New()})

	specification, err := service.Init(
		context.Background(),
		operation.Intent{Command: "spec init", Effect: operation.EffectRead},
		"catalog.json",
		[]string{"test"},
		"processor.json",
	)
	if err != nil {
		t.Fatal(err)
	}
	if catalogs.calls != 1 || observations.calls != 1 || observations.path != "processor.json" {
		t.Fatalf("catalog calls = %d, observation calls/path = %d/%q", catalogs.calls, observations.calls, observations.path)
	}
	if len(specification.Commands) != 1 {
		t.Fatalf("specification = %+v", specification)
	}
	entry := specification.Commands[0]
	if entry.Reason != processorcompat.DefaultReason || entry.Wrapper == nil || entry.Wrapper.Kind != tailoringbundle.WrapperTransform ||
		entry.Wrapper.Output == nil || entry.Wrapper.Output.Kind != tailoringbundle.OutputKindOptimizer || entry.Wrapper.Output.Optimizer == nil ||
		entry.Wrapper.Output.Optimizer.Contract != processorcompat.ContractID {
		t.Fatalf("processor default = %+v", entry)
	}
	if err := specification.Validate(catalog); err != nil {
		t.Fatal(err)
	}
}

func TestInitWithoutProcessorEvidenceNeverConsultsProcessorPorts(t *testing.T) {
	catalogs := &catalogFake{value: initCatalog(sourcecatalog.ProvenanceVerifiedBuiltin)}
	observations := &observationFake{err: errors.New("must not load")}
	compatibility := &defaultFake{err: errors.New("must not materialize")}

	specification, err := New(catalogs, ProcessorSupport{Observations: observations, Compatibility: compatibility}).Init(
		context.Background(), operation.Intent{Command: "spec init", Effect: operation.EffectRead}, "catalog", []string{"item", "list"},
	)
	if err != nil || specification.Commands[0].Wrapper.Kind != tailoringbundle.WrapperIdentity {
		t.Fatalf("specification = %+v, error = %v", specification, err)
	}
	if observations.calls != 0 || compatibility.calls != 0 {
		t.Fatalf("processor calls = observation %d, compatibility %d", observations.calls, compatibility.calls)
	}
}

func TestInitProcessorEvidenceFailureMatrix(t *testing.T) {
	intent := operation.Intent{Command: "spec init", Effect: operation.EffectRead}
	observation := rtkObservation(t)

	t.Run("unsupported source tuple", func(t *testing.T) {
		catalogs := &catalogFake{value: initCatalog(sourcecatalog.ProvenanceVerifiedBuiltin)}
		observations := &observationFake{value: observation}
		_, err := New(catalogs, ProcessorSupport{Observations: observations, Compatibility: processorcompat.New()}).Init(
			context.Background(), intent, "catalog", []string{"item", "list"}, "processor",
		)
		assertPublicCode(t, err, "processor_default_not_admitted")
		if catalogs.calls != 1 || observations.calls != 1 {
			t.Fatalf("calls = catalog %d, observation %d", catalogs.calls, observations.calls)
		}
	})

	t.Run("loader public fault", func(t *testing.T) {
		catalogs := &catalogFake{value: goCatalog(t)}
		observations := &observationFake{err: fault.New(fault.KindNotFound, "processor_observation_file_not_found", "The processor observation JSON was not found.", false, helpAction())}
		compatibility := &defaultFake{}
		_, err := New(catalogs, ProcessorSupport{Observations: observations, Compatibility: compatibility}).Init(
			context.Background(), intent, "catalog", []string{"test"}, "processor",
		)
		assertPublicCode(t, err, "processor_observation_file_not_found")
		if observations.calls != 1 || compatibility.calls != 0 {
			t.Fatalf("calls = observation %d, compatibility %d", observations.calls, compatibility.calls)
		}
	})

	for _, test := range []struct {
		name  string
		entry tailoringbundle.CommandEntry
	}{
		{name: "registry returns another command", entry: identityEntry([]string{"other"})},
		{name: "registry ignores evidence", entry: identityEntry([]string{"test"})},
	} {
		test := test
		t.Run(test.name, func(t *testing.T) {
			catalogs := &catalogFake{value: goCatalog(t)}
			observations := &observationFake{value: observation}
			compatibility := &defaultFake{entry: test.entry}
			_, err := New(catalogs, ProcessorSupport{Observations: observations, Compatibility: compatibility}).Init(
				context.Background(), intent, "catalog", []string{"test"}, "processor",
			)
			assertPublicCode(t, err, "invalid_processor_default")
			if compatibility.calls != 1 {
				t.Fatalf("compatibility calls = %d", compatibility.calls)
			}
		})
	}

	for _, selection := range [][]string{{""}, {" \t"}, {"one", "two"}} {
		selection := selection
		t.Run("invalid selection", func(t *testing.T) {
			catalogs := &catalogFake{value: goCatalog(t)}
			_, err := New(catalogs).Init(context.Background(), intent, "catalog", []string{"test"}, selection...)
			assertPublicCode(t, err, "invalid_processor_observation_selection")
			if catalogs.calls != 0 {
				t.Fatalf("catalog calls = %d", catalogs.calls)
			}
		})
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

func goCatalog(t *testing.T) sourcecatalog.Catalog {
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

func rtkObservation(t *testing.T) processorprocess.Observation {
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
			Argv: []string{"--version"}, EnvironmentContract: processorprocess.EnvironmentRTKIsolatedV2, Attempts: 1,
		},
	}
}

func assertPublicCode(t *testing.T, err error, code string) {
	t.Helper()
	public, ok := fault.PublicCopy(err)
	if !ok || public.Code != code || public.Retryable {
		t.Fatalf("fault = %+v, error = %v, want code %q", public, err, code)
	}
}
