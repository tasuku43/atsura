package processorcompat_test

import (
	"errors"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/tasuku43/atsura/internal/app/processorcompat"
	"github.com/tasuku43/atsura/internal/domain/processorprocess"
	"github.com/tasuku43/atsura/internal/domain/sourcecatalog"
	"github.com/tasuku43/atsura/internal/domain/sourceprocess"
	"github.com/tasuku43/atsura/internal/domain/tailoringbundle"
	"github.com/tasuku43/atsura/internal/domain/tailoringplan"
)

type artifact struct {
	sha256 string
	size   int64
}

var artifacts = map[processorprocess.Platform]artifact{
	{OS: "linux", Arch: "amd64"}:  {sha256: "f160611f3baee17fe4eb3a04c56a8bc3d15fec4274d8838016088d4776c6f628", size: 10083968},
	{OS: "linux", Arch: "arm64"}:  {sha256: "86bd2badb697e41fa4fae805ed1a42d9b2495600260918d6ba9c148bc40013cf", size: 8544624},
	{OS: "darwin", Arch: "amd64"}: {sha256: "22adaa27b3fd6d8906159ba3ff7ca8346e914df112408bcc7a88cda30a3a6107", size: 9006316},
	{OS: "darwin", Arch: "arm64"}: {sha256: "2dab449f32ea744c30b02a3ef9806e3e7d3b356a145332f3f2aaabb5ea48edee", size: 7763408},
}

func TestVerifyObservationAdmitsOnlyExactRTKMatrix(t *testing.T) {
	t.Parallel()

	registry := processorcompat.New()
	for platform := range artifacts {
		platform := platform
		t.Run(platform.OS+"_"+platform.Arch, func(t *testing.T) {
			t.Parallel()
			if err := registry.VerifyObservation(observationFixture(t, platform)); err != nil {
				t.Fatalf("VerifyObservation() error = %v", err)
			}
		})
	}
}

func TestVerifyObservationRejectsEveryTupleDimension(t *testing.T) {
	t.Parallel()

	base := observationFixture(t, processorprocess.Platform{OS: "darwin", Arch: "arm64"})
	tests := []struct {
		name   string
		mutate func(*processorprocess.Observation)
	}{
		{name: "schema", mutate: func(v *processorprocess.Observation) { v.SchemaVersion++ }},
		{name: "adapter", mutate: func(v *processorprocess.Observation) { v.Adapter.Kind = "atsura.processor.other" }},
		{name: "contract", mutate: func(v *processorprocess.Observation) { v.Adapter.ContractVersion++ }},
		{name: "version", mutate: func(v *processorprocess.Observation) { v.Version = "0.44.0" }},
		{name: "windows", mutate: func(v *processorprocess.Observation) { v.Platform.OS = "windows" }},
		{name: "architecture", mutate: func(v *processorprocess.Observation) { v.Platform.Arch = "riscv64" }},
		{name: "probe argv", mutate: func(v *processorprocess.Observation) { v.Probe.Argv = []string{"version"} }},
		{name: "probe environment", mutate: func(v *processorprocess.Observation) { v.Probe.EnvironmentContract = "atsura.processor.other.v1" }},
		{name: "retired host-specific probe environment", mutate: func(v *processorprocess.Observation) {
			v.Probe.EnvironmentContract = "atsura.processor.rtk_isolated.v1"
		}},
		{name: "probe attempts", mutate: func(v *processorprocess.Observation) { v.Probe.Attempts = 2 }},
		{name: "artifact hash", mutate: func(v *processorprocess.Observation) { v.Identity.SHA256 = strings.Repeat("a", 64) }},
		{name: "artifact size", mutate: func(v *processorprocess.Observation) { v.Identity.Size++ }},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			value := cloneObservation(base)
			test.mutate(&value)
			assertCompatibilityError(t, processorcompat.New().VerifyObservation(value), processorcompat.ErrObservation, processorcompat.ErrorObservation)
		})
	}

	var registry *processorcompat.Registry
	assertCompatibilityError(t, registry.VerifyObservation(base), processorcompat.ErrRegistry, processorcompat.ErrorRegistry)
}

func TestBindingIsCanonicalBoundedAndDetached(t *testing.T) {
	t.Parallel()

	observation := observationFixture(t, processorprocess.Platform{OS: "linux", Arch: "amd64"})
	binding, err := processorcompat.New().Binding(observation)
	if err != nil {
		t.Fatal(err)
	}
	if binding.Contract != processorcompat.ContractID || binding.InputFormat != processorcompat.InputFormat ||
		binding.OutputFormat != processorcompat.OutputFormat || !binding.AllowOriginalOutput {
		t.Fatalf("binding contract = %+v", binding)
	}
	wantExecution := tailoringbundle.ProcessorExecution{
		Args:                 []string{"pipe", "--filter=go-test"},
		StdinMode:            "stage_input",
		WorkingDirectoryMode: "isolated",
		EnvironmentContract:  processorprocess.EnvironmentRTKIsolatedV2,
		MaxAttempts:          1,
		TimeoutMillis:        processorprocess.MaxTimeout.Milliseconds(),
		StdoutLimitBytes:     processorprocess.MaxStdoutBytes,
		StderrLimitBytes:     processorprocess.MaxStderrBytes,
	}
	if !reflect.DeepEqual(binding.Execution, wantExecution) {
		t.Fatalf("execution = %+v, want %+v", binding.Execution, wantExecution)
	}
	if err := binding.Validate(); err != nil {
		t.Fatalf("binding.Validate() error = %v", err)
	}
	observation.Probe.Argv[0] = "changed"
	if binding.Observation.Probe.Argv[0] != "--version" {
		t.Fatal("binding aliases caller-owned observation")
	}
}

func TestDefaultEntryRequiresExactCatalogAndExplicitObservation(t *testing.T) {
	t.Parallel()

	registry := processorcompat.New()
	catalog := catalogFixture(t)
	observation := observationFixture(t, processorprocess.Platform{OS: "darwin", Arch: "amd64"})
	entry, err := registry.DefaultEntry(catalog, observation)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(entry.Command, []string{"test"}) || entry.Presence != tailoringbundle.PresenceInclude ||
		entry.Reason != processorcompat.DefaultReason || entry.Options == nil || entry.Wrapper == nil {
		t.Fatalf("entry = %+v", entry)
	}
	if entry.Options.Default != tailoringbundle.SurfaceDefaultInherit || len(entry.Options.Include) != 0 || len(entry.Options.Exclude) != 0 {
		t.Fatalf("options = %+v", entry.Options)
	}
	if entry.Wrapper.Output == nil || entry.Wrapper.Output.Optimizer == nil ||
		entry.Wrapper.Output.Optimizer.Contract != processorcompat.ContractID ||
		!reflect.DeepEqual(entry.Wrapper.Invoke.AppendArgs, []string{"-json"}) {
		t.Fatalf("wrapper = %+v", entry.Wrapper)
	}

	tests := []struct {
		name   string
		mutate func(*sourcecatalog.Catalog)
	}{
		{name: "adapter", mutate: func(v *sourcecatalog.Catalog) { v.Adapter.Kind = "atsura.source.other" }},
		{name: "contract", mutate: func(v *sourcecatalog.Catalog) { v.Adapter.ContractVersion = 1 }},
		{name: "older Go", mutate: func(v *sourcecatalog.Catalog) { v.Source.Version = "go1.25.9" }},
		{name: "newer Go", mutate: func(v *sourcecatalog.Catalog) { v.Source.Version = "go1.27.0" }},
		{name: "leading-zero patch", mutate: func(v *sourcecatalog.Catalog) { v.Source.Version = "go1.26.05" }},
		{name: "command absent", mutate: func(v *sourcecatalog.Catalog) { v.Commands[0].Path = []string{"version"} }},
		{name: "unverified command", mutate: func(v *sourcecatalog.Catalog) { v.Commands[0].Provenance = sourcecatalog.ProvenanceObservedExtension }},
		{name: "option surface", mutate: func(v *sourcecatalog.Catalog) { v.Commands[0].Options = []sourcecatalog.Option{{Name: "--run"}} }},
		{name: "format", mutate: func(v *sourcecatalog.Catalog) { v.Commands[0].StructuredOutput[0].Format = "other_jsonl" }},
		{name: "selector", mutate: func(v *sourcecatalog.Catalog) { v.Commands[0].StructuredOutput[0].SelectorFlag = "--json" }},
		{name: "fields", mutate: func(v *sourcecatalog.Catalog) {
			v.Commands[0].StructuredOutput[0].Fields = []string{"Action", "Package"}
		}},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			value := cloneCatalog(catalog)
			test.mutate(&value)
			_, err := registry.DefaultEntry(value, observation)
			assertCompatibilityError(t, err, processorcompat.ErrSourceTuple, processorcompat.ErrorSourceTuple)
		})
	}

	badObservation := cloneObservation(observation)
	badObservation.Version = "0.44.0"
	_, err = registry.DefaultEntry(catalog, badObservation)
	assertCompatibilityError(t, err, processorcompat.ErrObservation, processorcompat.ErrorObservation)
}

func TestVerifySurfaceAcceptsCanonicalTupleAndRejectsCompatibleLookingVariants(t *testing.T) {
	t.Parallel()

	registry := processorcompat.New()
	bundle := canonicalBundle(t, registry)
	if err := registry.VerifySurface(bundle); err != nil {
		t.Fatalf("VerifySurface() error = %v", err)
	}

	t.Run("alternate processor argv", func(t *testing.T) {
		value := canonicalBundle(t, registry)
		entry := value.Specification.Commands[0]
		binding := value.Processors[0]
		binding.Execution.Args = []string{"pipe", "--filter=raw"}
		value = compileBundle(t, value.Catalog, entry, binding)
		assertCompatibilityError(t, registry.VerifySurface(value), processorcompat.ErrSurface, processorcompat.ErrorSurface)
	})

	t.Run("alternate contract", func(t *testing.T) {
		value := canonicalBundle(t, registry)
		entry := value.Specification.Commands[0]
		entry.Wrapper.Output.Optimizer.Contract = "atsura.output.other.v1"
		binding := value.Processors[0]
		binding.Contract = "atsura.output.other.v1"
		value = compileBundle(t, value.Catalog, entry, binding)
		assertCompatibilityError(t, registry.VerifySurface(value), processorcompat.ErrSurface, processorcompat.ErrorSurface)
	})

	t.Run("semantically empty but noncanonical option default", func(t *testing.T) {
		value := canonicalBundle(t, registry)
		entry := value.Specification.Commands[0]
		entry.Options.Default = tailoringbundle.SurfaceDefaultExclude
		value = compileBundle(t, value.Catalog, entry, value.Processors[0])
		assertCompatibilityError(t, registry.VerifySurface(value), processorcompat.ErrSurface, processorcompat.ErrorSurface)
	})

	t.Run("identity wrapper", func(t *testing.T) {
		catalog := catalogFixture(t)
		entry := tailoringbundle.CommandEntry{
			Command: []string{"test"}, Presence: tailoringbundle.PresenceInclude, Reason: "identity",
			Options: &tailoringbundle.OptionSurface{Default: tailoringbundle.SurfaceDefaultInherit, Include: []string{}, Exclude: []string{}},
			Wrapper: &tailoringbundle.Wrapper{Kind: tailoringbundle.WrapperIdentity, Before: []tailoringbundle.StageAction{}, Invoke: tailoringbundle.Invocation{OptionDefaults: []tailoringbundle.OptionDefault{}, AppendArgs: []string{}}, After: []tailoringbundle.StageAction{}},
		}
		value := compileBundle(t, catalog, entry)
		assertCompatibilityError(t, registry.VerifySurface(value), processorcompat.ErrSurface, processorcompat.ErrorSurface)
	})

	t.Run("source version", func(t *testing.T) {
		value := canonicalBundle(t, registry)
		catalog := cloneCatalog(value.Catalog)
		catalog.Source.Version = "go1.27.0"
		value = compileBundle(t, catalog, value.Specification.Commands[0], value.Processors[0])
		assertCompatibilityError(t, registry.VerifySurface(value), processorcompat.ErrSurface, processorcompat.ErrorSurface)
	})
}

func TestVerifyPlanRequiresExactNoArgumentInvocationAndCanonicalBinding(t *testing.T) {
	t.Parallel()

	registry := processorcompat.New()
	bundle := canonicalBundle(t, registry)
	plan := buildPlan(t, bundle, []string{"test"})
	if err := registry.VerifyPlan(plan); err != nil {
		t.Fatalf("VerifyPlan() error = %v", err)
	}

	withEditedReason := plan
	withEditedReason.Reason = "A reviewed local reason."
	withEditedReason.SpecificationEntry = cloneEntry(plan.SpecificationEntry)
	withEditedReason.SpecificationEntry.Reason = withEditedReason.Reason
	if err := registry.VerifyPlan(withEditedReason); err != nil {
		t.Fatalf("non-semantic reason edit rejected: %v", err)
	}

	t.Run("package pattern", func(t *testing.T) {
		value := buildPlan(t, bundle, []string{"test", "./..."})
		assertCompatibilityError(t, registry.VerifyPlan(value), processorcompat.ErrPlan, processorcompat.ErrorPlan)
	})

	t.Run("source version", func(t *testing.T) {
		value := plan
		value.Source.Version = "go1.27.0"
		assertCompatibilityError(t, registry.VerifyPlan(value), processorcompat.ErrPlan, processorcompat.ErrorPlan)
	})

	t.Run("processor argv", func(t *testing.T) {
		value := canonicalBundle(t, registry)
		entry := value.Specification.Commands[0]
		binding := value.Processors[0]
		binding.Execution.Args = []string{"pipe", "--filter=raw"}
		value = compileBundle(t, value.Catalog, entry, binding)
		otherPlan := buildPlan(t, value, []string{"test"})
		assertCompatibilityError(t, registry.VerifyPlan(otherPlan), processorcompat.ErrPlan, processorcompat.ErrorPlan)
	})

	t.Run("transformed argv", func(t *testing.T) {
		value := plan
		value.TransformedArgv = append([]string{}, value.TransformedArgv...)
		value.TransformedArgv[2] = "-x"
		assertCompatibilityError(t, registry.VerifyPlan(value), processorcompat.ErrPlan, processorcompat.ErrorPlan)
	})
}

func observationFixture(t *testing.T, platform processorprocess.Platform) processorprocess.Observation {
	t.Helper()
	identity, ok := artifacts[platform]
	if !ok {
		t.Fatalf("missing artifact fixture for %+v", platform)
	}
	return processorprocess.Observation{
		SchemaVersion: processorprocess.ObservationSchemaVersion,
		Adapter: processorprocess.Adapter{
			Kind: processorcompat.ProcessorAdapterKind, ContractVersion: processorcompat.ProcessorContractVersion,
		},
		Platform: platform,
		Identity: processorprocess.Identity{
			ResolvedPath: filepath.Join(t.TempDir(), "rtk"), SHA256: identity.sha256, Size: identity.size,
		},
		Version: processorcompat.ProcessorVersion,
		Probe: processorprocess.Probe{
			Argv: []string{"--version"}, EnvironmentContract: processorprocess.EnvironmentRTKIsolatedV2, Attempts: 1,
		},
	}
}

func catalogFixture(t *testing.T) sourcecatalog.Catalog {
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

func canonicalBundle(t *testing.T, registry *processorcompat.Registry) tailoringbundle.Bundle {
	t.Helper()
	catalog := catalogFixture(t)
	observation := observationFixture(t, processorprocess.Platform{OS: "darwin", Arch: "arm64"})
	entry, err := registry.DefaultEntry(catalog, observation)
	if err != nil {
		t.Fatal(err)
	}
	binding, err := registry.Binding(observation)
	if err != nil {
		t.Fatal(err)
	}
	return compileBundle(t, catalog, entry, binding)
}

func compileBundle(t *testing.T, catalog sourcecatalog.Catalog, entry tailoringbundle.CommandEntry, processors ...tailoringbundle.ProcessorBinding) tailoringbundle.Bundle {
	t.Helper()
	digest, err := catalog.Digest()
	if err != nil {
		t.Fatal(err)
	}
	specification := tailoringbundle.Specification{
		SchemaVersion: tailoringbundle.SpecificationSchemaVersion,
		CatalogDigest: digest,
		Surface:       tailoringbundle.Surface{Default: tailoringbundle.SurfaceDefaultExclude},
		Commands:      []tailoringbundle.CommandEntry{entry},
	}
	bundle, err := tailoringbundle.Compile(catalog, specification, processors...)
	if err != nil {
		t.Fatal(err)
	}
	return bundle
}

func buildPlan(t *testing.T, bundle tailoringbundle.Bundle, args []string) tailoringplan.Plan {
	t.Helper()
	digest, err := bundle.Digest()
	if err != nil {
		t.Fatal(err)
	}
	plan, err := tailoringplan.Build(digest, bundle, sourceprocess.Identity{
		ResolvedPath: bundle.Catalog.Source.ResolvedPath,
		SHA256:       bundle.Catalog.Source.SHA256,
		Size:         bundle.Catalog.Source.Size,
	}, tailoringplan.Attempt{Executable: bundle.Catalog.Source.RequestedExecutable, Args: args})
	if err != nil {
		t.Fatal(err)
	}
	return plan
}

func cloneObservation(value processorprocess.Observation) processorprocess.Observation {
	result := value
	result.Probe.Argv = append([]string{}, value.Probe.Argv...)
	return result
}

func cloneCatalog(value sourcecatalog.Catalog) sourcecatalog.Catalog {
	result := value
	result.Probe.IDs = append([]string{}, value.Probe.IDs...)
	result.Commands = append([]sourcecatalog.Command{}, value.Commands...)
	for index := range result.Commands {
		result.Commands[index].Path = append([]string{}, value.Commands[index].Path...)
		result.Commands[index].Options = append([]sourcecatalog.Option{}, value.Commands[index].Options...)
		result.Commands[index].StructuredOutput = append([]sourcecatalog.StructuredOutput{}, value.Commands[index].StructuredOutput...)
		for outputIndex := range result.Commands[index].StructuredOutput {
			result.Commands[index].StructuredOutput[outputIndex].Fields = append([]string{}, value.Commands[index].StructuredOutput[outputIndex].Fields...)
		}
	}
	return result
}

func cloneEntry(value *tailoringbundle.CommandEntry) *tailoringbundle.CommandEntry {
	if value == nil {
		return nil
	}
	result := *value
	result.Command = append([]string{}, value.Command...)
	if value.Options != nil {
		options := *value.Options
		options.Include = append([]string{}, value.Options.Include...)
		options.Exclude = append([]string{}, value.Options.Exclude...)
		result.Options = &options
	}
	if value.Wrapper != nil {
		wrapper := *value.Wrapper
		wrapper.Before = append([]tailoringbundle.StageAction{}, value.Wrapper.Before...)
		wrapper.Invoke.AppendArgs = append([]string{}, value.Wrapper.Invoke.AppendArgs...)
		wrapper.After = append([]tailoringbundle.StageAction{}, value.Wrapper.After...)
		if value.Wrapper.Output != nil {
			output := *value.Wrapper.Output
			if output.Optimizer != nil {
				optimizer := *output.Optimizer
				output.Optimizer = &optimizer
			}
			wrapper.Output = &output
		}
		result.Wrapper = &wrapper
	}
	return &result
}

func assertCompatibilityError(t *testing.T, err, sentinel error, kind processorcompat.ErrorKind) {
	t.Helper()
	if err == nil || !errors.Is(err, sentinel) {
		t.Fatalf("error = %v, want errors.Is(%v)", err, sentinel)
	}
	var typed *processorcompat.Error
	if !errors.As(err, &typed) || typed.Kind != kind {
		t.Fatalf("typed error = %#v, want kind %q", typed, kind)
	}
}
