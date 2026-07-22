package gocli

import (
	"errors"
	"strings"
	"testing"

	"github.com/tasuku43/atsura/internal/domain/runtimeadmission"
	"github.com/tasuku43/atsura/internal/domain/sourcecatalog"
	"github.com/tasuku43/atsura/internal/domain/tailoringbundle"
	"github.com/tasuku43/atsura/internal/domain/tailoringplan"
)

func goRuntimePlan(t *testing.T, path, callerArgs, appendArgs []string, output *tailoringbundle.Output) tailoringplan.Plan {
	t.Helper()
	options := []sourcecatalog.Option{}
	structured := []sourcecatalog.StructuredOutput{}
	if output != nil {
		options = []sourcecatalog.Option{{Name: "--json", TakesValue: true}}
		structured = []sourcecatalog.StructuredOutput{{Format: "json", SelectorFlag: "--json", Fields: []string{"result"}}}
	}
	command := sourcecatalog.Command{
		Path: path, Summary: "Synthetic Go command", Provenance: sourcecatalog.ProvenanceVerifiedBuiltin,
		Options: options, StructuredOutput: structured,
	}
	catalog := goRuntimeCatalog([]sourcecatalog.Command{command}, AdapterKind, ContractVersion, "go1.26.5")
	catalogDigest, err := catalog.Digest()
	if err != nil {
		t.Fatal(err)
	}
	wrapperKind := tailoringbundle.WrapperTransform
	if len(appendArgs) == 0 && output == nil {
		wrapperKind = tailoringbundle.WrapperIdentity
	}
	entry := tailoringbundle.CommandEntry{
		Command: path, Presence: tailoringbundle.PresenceInclude, Reason: "Run the reviewed no-argument test command.",
		Options: &tailoringbundle.OptionSurface{Default: tailoringbundle.SurfaceDefaultInherit, Include: []string{}, Exclude: []string{}},
		Wrapper: &tailoringbundle.Wrapper{
			Kind: wrapperKind, Before: []tailoringbundle.StageAction{},
			Invoke: tailoringbundle.Invocation{AppendArgs: appendArgs}, Output: output, After: []tailoringbundle.StageAction{},
		},
	}
	specification := tailoringbundle.SortSpecification(tailoringbundle.Specification{
		SchemaVersion: tailoringbundle.SpecificationSchemaVersion,
		CatalogDigest: catalogDigest,
		Surface:       tailoringbundle.Surface{Default: tailoringbundle.SurfaceDefaultExclude},
		Commands:      []tailoringbundle.CommandEntry{entry},
	})
	bundle, err := tailoringbundle.Compile(catalog, specification)
	if err != nil {
		t.Fatal(err)
	}
	bundleDigest, err := bundle.Digest()
	if err != nil {
		t.Fatal(err)
	}
	args := append(append([]string{}, path...), callerArgs...)
	plan, err := tailoringplan.Build(bundleDigest, bundle, goIdentity("a"), tailoringplan.Attempt{Executable: "go", Args: args})
	if err != nil {
		t.Fatal(err)
	}
	return plan
}

func goRuntimeCatalog(commands []sourcecatalog.Command, adapterKind string, contract int, version string) sourcecatalog.Catalog {
	return sourcecatalog.Sort(sourcecatalog.Catalog{
		SchemaVersion: sourcecatalog.SchemaVersion,
		Adapter:       sourcecatalog.Adapter{Kind: adapterKind, ContractVersion: contract},
		Source: sourcecatalog.Source{
			RequestedExecutable: "go", ResolvedPath: "/opt/go/bin/go", SHA256: strings.Repeat("a", 64), Size: 14_500_192, Version: version,
		},
		Probe:    sourcecatalog.Probe{IDs: []string{"help", "test_help", "version"}, Attempts: 3},
		Commands: commands,
	})
}

func goSurfaceBundle(t *testing.T, commands []sourcecatalog.Command, entries []tailoringbundle.CommandEntry, adapterKind string, contract int, version string) tailoringbundle.Bundle {
	t.Helper()
	catalog := goRuntimeCatalog(commands, adapterKind, contract, version)
	digest, err := catalog.Digest()
	if err != nil {
		t.Fatal(err)
	}
	specification := tailoringbundle.SortSpecification(tailoringbundle.Specification{
		SchemaVersion: tailoringbundle.SpecificationSchemaVersion,
		CatalogDigest: digest,
		Surface:       tailoringbundle.Surface{Default: tailoringbundle.SurfaceDefaultExclude},
		Commands:      entries,
	})
	bundle, err := tailoringbundle.Compile(catalog, specification)
	if err != nil {
		t.Fatal(err)
	}
	return bundle
}

func goCommand(path ...string) sourcecatalog.Command {
	return sourcecatalog.Command{
		Path: path, Summary: "Synthetic Go command", Provenance: sourcecatalog.ProvenanceVerifiedBuiltin,
		Options: []sourcecatalog.Option{}, StructuredOutput: []sourcecatalog.StructuredOutput{},
	}
}

func goIdentityEntry(path []string, optionDefault tailoringbundle.SurfaceDefault) tailoringbundle.CommandEntry {
	return tailoringbundle.CommandEntry{
		Command: path, Presence: tailoringbundle.PresenceInclude, Reason: "Run the reviewed no-argument test command.",
		Options: &tailoringbundle.OptionSurface{Default: optionDefault, Include: []string{}, Exclude: []string{}},
		Wrapper: &tailoringbundle.Wrapper{
			Kind: tailoringbundle.WrapperIdentity, Before: []tailoringbundle.StageAction{},
			Invoke: tailoringbundle.Invocation{AppendArgs: []string{}}, After: []tailoringbundle.StageAction{},
		},
	}
}

func TestVerifyRuntimeAdmitsOnlyExactNoArgumentGoTestIdentityPlan(t *testing.T) {
	plan := goRuntimePlan(t, []string{"test"}, []string{}, []string{}, nil)
	if plan.ResultMode != tailoringplan.ResultModeSourceStreamPassthrough || plan.WrapperKind != tailoringbundle.WrapperIdentity {
		t.Fatalf("result/wrapper = %q/%q", plan.ResultMode, plan.WrapperKind)
	}
	if err := NewRuntimeVerifier().VerifyRuntime(plan); err != nil {
		t.Fatal(err)
	}
	if err := VerifyRuntime(plan); err != nil {
		t.Fatal(err)
	}
}

func TestVerifyRuntimeSourceContractTruthTable(t *testing.T) {
	base := goRuntimePlan(t, []string{"test"}, []string{}, []string{}, nil)
	tests := []struct {
		name     string
		mutate   func(tailoringplan.Plan) tailoringplan.Plan
		category error
	}{
		{name: "adapter kind", mutate: func(plan tailoringplan.Plan) tailoringplan.Plan {
			plan.Source.AdapterKind = "atsura.source.other"
			return plan
		}, category: ErrRuntimeAdapterContract},
		{name: "contract zero", mutate: func(plan tailoringplan.Plan) tailoringplan.Plan { plan.Source.AdapterContractVersion = 0; return plan }, category: ErrRuntimeWrapperOutput},
		{name: "contract two", mutate: func(plan tailoringplan.Plan) tailoringplan.Plan { plan.Source.AdapterContractVersion = 2; return plan }, category: ErrRuntimeAdapterContract},
		{name: "missing go prefix", mutate: func(plan tailoringplan.Plan) tailoringplan.Plan { plan.Source.Version = "1.26.5"; return plan }, category: ErrRuntimeSourceVersion},
		{name: "older minor", mutate: func(plan tailoringplan.Plan) tailoringplan.Plan { plan.Source.Version = "go1.25.9"; return plan }, category: ErrRuntimeSourceVersion},
		{name: "newer minor", mutate: func(plan tailoringplan.Plan) tailoringplan.Plan { plan.Source.Version = "go1.27.0"; return plan }, category: ErrRuntimeSourceVersion},
		{name: "prerelease", mutate: func(plan tailoringplan.Plan) tailoringplan.Plan { plan.Source.Version = "go1.26rc1"; return plan }, category: ErrRuntimeSourceVersion},
		{name: "build suffix", mutate: func(plan tailoringplan.Plan) tailoringplan.Plan { plan.Source.Version = "go1.26.5+local"; return plan }, category: ErrRuntimeSourceVersion},
		{name: "leading zero patch", mutate: func(plan tailoringplan.Plan) tailoringplan.Plan { plan.Source.Version = "go1.26.05"; return plan }, category: ErrRuntimeSourceVersion},
		{name: "unsupported command", mutate: func(_ tailoringplan.Plan) tailoringplan.Plan {
			return goRuntimePlan(t, []string{"version"}, []string{}, []string{}, nil)
		}, category: ErrRuntimeCommand},
		{name: "nested command", mutate: func(_ tailoringplan.Plan) tailoringplan.Plan {
			return goRuntimePlan(t, []string{"test", "internal"}, []string{}, []string{}, nil)
		}, category: ErrRuntimeCommand},
		{name: "invalid plan", mutate: func(plan tailoringplan.Plan) tailoringplan.Plan { plan.SchemaVersion = 0; return plan }, category: ErrRuntimeWrapperOutput},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assertGoAdmission(t, VerifyRuntime(test.mutate(base)), test.category)
		})
	}

	for _, version := range []string{"go1.26.0", "go1.26.1", "go1.26.999"} {
		t.Run("admit "+version, func(t *testing.T) {
			plan := base
			plan.Source.Version = version
			if err := VerifyRuntime(plan); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestVerifyRuntimeRejectsEveryArgvElementAfterTest(t *testing.T) {
	for _, args := range [][]string{
		{"package"},
		{"."},
		{"./..."},
		{"--"},
		{"--", "package"},
		{"--", "-test.v"},
		{"-v"},
		{"-run=TestOne"},
		{"-args"},
		{"--json"},
	} {
		name := strings.Join(args, "_")
		t.Run(name, func(t *testing.T) {
			plan := withGoInvocationArgs(t, goRuntimePlan(t, []string{"test"}, []string{}, []string{}, nil), args)
			assertGoAdmission(t, VerifyRuntime(plan), ErrRuntimeArgvGrammar)
		})
	}
}

func TestVerifyRuntimeRejectsTransformsAndOutputModes(t *testing.T) {
	t.Run("append transform", func(t *testing.T) {
		plan := goRuntimePlan(t, []string{"test"}, []string{}, []string{"-json"}, nil)
		assertGoAdmission(t, VerifyRuntime(plan), ErrRuntimeWrapperOutput)
	})
	t.Run("structured output", func(t *testing.T) {
		output := &tailoringbundle.Output{Input: "json", Select: []string{"result"}, Rename: []tailoringbundle.Rename{}, Render: "compact_json"}
		plan := goRuntimePlan(t, []string{"test"}, []string{}, []string{"--json=result"}, output)
		assertGoAdmission(t, VerifyRuntime(plan), ErrRuntimeWrapperOutput)
	})
	t.Run("before stage", func(t *testing.T) {
		plan := goRuntimePlan(t, []string{"test"}, []string{}, []string{}, nil)
		plan.Stages.Before = []tailoringbundle.StageAction{{Kind: "future"}}
		assertGoAdmission(t, VerifyRuntime(plan), ErrRuntimeWrapperOutput)
	})
	t.Run("result mode", func(t *testing.T) {
		plan := goRuntimePlan(t, []string{"test"}, []string{}, []string{}, nil)
		plan.ResultMode = tailoringplan.ResultModeTransformedJSON
		assertGoAdmission(t, VerifyRuntime(plan), ErrRuntimeWrapperOutput)
	})
}

func TestVerifySurfaceAdmitsOnlyOneExactGoTestIdentitySurface(t *testing.T) {
	commands := []sourcecatalog.Command{goCommand("build"), goCommand("test"), goCommand("version")}
	for _, optionDefault := range []tailoringbundle.SurfaceDefault{tailoringbundle.SurfaceDefaultExclude, tailoringbundle.SurfaceDefaultInherit} {
		t.Run(string(optionDefault), func(t *testing.T) {
			bundle := goSurfaceBundle(t, commands, []tailoringbundle.CommandEntry{goIdentityEntry([]string{"test"}, optionDefault)}, AdapterKind, ContractVersion, "go1.26.5")
			if len(bundle.Catalog.Commands) != 3 || len(bundle.Surface) != 1 {
				t.Fatalf("catalog/surface = %d/%d", len(bundle.Catalog.Commands), len(bundle.Surface))
			}
			if err := NewRuntimeVerifier().VerifySurface(bundle); err != nil {
				t.Fatal(err)
			}
			if err := VerifySurface(bundle); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestVerifySurfaceContractTruthTable(t *testing.T) {
	identityTest := goIdentityEntry([]string{"test"}, tailoringbundle.SurfaceDefaultExclude)
	tests := []struct {
		name     string
		commands []sourcecatalog.Command
		entries  []tailoringbundle.CommandEntry
		adapter  string
		contract int
		version  string
		category error
		mutate   func(tailoringbundle.Bundle) tailoringbundle.Bundle
	}{
		{name: "adapter", commands: []sourcecatalog.Command{goCommand("test")}, entries: []tailoringbundle.CommandEntry{identityTest}, adapter: "atsura.source.other", contract: 1, version: "go1.26.5", category: ErrRuntimeAdapterContract},
		{name: "contract", commands: []sourcecatalog.Command{goCommand("test")}, entries: []tailoringbundle.CommandEntry{identityTest}, adapter: AdapterKind, contract: 2, version: "go1.26.5", category: ErrRuntimeAdapterContract},
		{name: "version", commands: []sourcecatalog.Command{goCommand("test")}, entries: []tailoringbundle.CommandEntry{identityTest}, adapter: AdapterKind, contract: 1, version: "go1.27.0", category: ErrRuntimeSourceVersion},
		{name: "empty surface", commands: []sourcecatalog.Command{goCommand("test")}, entries: []tailoringbundle.CommandEntry{}, adapter: AdapterKind, contract: 1, version: "go1.26.5", category: ErrRuntimeWrapperOutput},
		{name: "unsupported command", commands: []sourcecatalog.Command{goCommand("version")}, entries: []tailoringbundle.CommandEntry{goIdentityEntry([]string{"version"}, tailoringbundle.SurfaceDefaultExclude)}, adapter: AdapterKind, contract: 1, version: "go1.26.5", category: ErrRuntimeCommand},
		{name: "mixed surface", commands: []sourcecatalog.Command{goCommand("test"), goCommand("version")}, entries: []tailoringbundle.CommandEntry{identityTest, goIdentityEntry([]string{"version"}, tailoringbundle.SurfaceDefaultExclude)}, adapter: AdapterKind, contract: 1, version: "go1.26.5", category: ErrRuntimeWrapperOutput},
		{name: "invalid bundle", commands: []sourcecatalog.Command{goCommand("test")}, entries: []tailoringbundle.CommandEntry{identityTest}, adapter: AdapterKind, contract: 1, version: "go1.26.5", category: ErrRuntimeWrapperOutput, mutate: func(bundle tailoringbundle.Bundle) tailoringbundle.Bundle { bundle.SchemaVersion = 0; return bundle }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			bundle := goSurfaceBundle(t, test.commands, test.entries, test.adapter, test.contract, test.version)
			if test.mutate != nil {
				bundle = test.mutate(bundle)
			}
			assertGoAdmission(t, VerifySurface(bundle), test.category)
		})
	}
}

func TestVerifySurfaceRejectsObservedGrammarAndTransforms(t *testing.T) {
	t.Run("observed option even when excluded", func(t *testing.T) {
		command := goCommand("test")
		command.Options = []sourcecatalog.Option{{Name: "--json", TakesValue: true}}
		bundle := goSurfaceBundle(t, []sourcecatalog.Command{command}, []tailoringbundle.CommandEntry{goIdentityEntry([]string{"test"}, tailoringbundle.SurfaceDefaultExclude)}, AdapterKind, ContractVersion, "go1.26.5")
		assertGoAdmission(t, VerifySurface(bundle), ErrRuntimeArgvGrammar)
	})
	t.Run("observed option included", func(t *testing.T) {
		command := goCommand("test")
		command.Options = []sourcecatalog.Option{{Name: "--json", TakesValue: true}}
		entry := goIdentityEntry([]string{"test"}, tailoringbundle.SurfaceDefaultExclude)
		entry.Options.Include = []string{"--json"}
		bundle := goSurfaceBundle(t, []sourcecatalog.Command{command}, []tailoringbundle.CommandEntry{entry}, AdapterKind, ContractVersion, "go1.26.5")
		assertGoAdmission(t, VerifySurface(bundle), ErrRuntimeArgvGrammar)
	})
	t.Run("structured output", func(t *testing.T) {
		command := goCommand("test")
		command.StructuredOutput = []sourcecatalog.StructuredOutput{{Format: "json", SelectorFlag: "--json", Fields: []string{}}}
		bundle := goSurfaceBundle(t, []sourcecatalog.Command{command}, []tailoringbundle.CommandEntry{goIdentityEntry([]string{"test"}, tailoringbundle.SurfaceDefaultExclude)}, AdapterKind, ContractVersion, "go1.26.5")
		assertGoAdmission(t, VerifySurface(bundle), ErrRuntimeWrapperOutput)
	})
	t.Run("append transform", func(t *testing.T) {
		entry := goIdentityEntry([]string{"test"}, tailoringbundle.SurfaceDefaultExclude)
		entry.Wrapper.Kind = tailoringbundle.WrapperTransform
		entry.Wrapper.Invoke.AppendArgs = []string{"-json"}
		bundle := goSurfaceBundle(t, []sourcecatalog.Command{goCommand("test")}, []tailoringbundle.CommandEntry{entry}, AdapterKind, ContractVersion, "go1.26.5")
		assertGoAdmission(t, VerifySurface(bundle), ErrRuntimeWrapperOutput)
	})
}

func withGoInvocationArgs(t *testing.T, plan tailoringplan.Plan, afterTest []string) tailoringplan.Plan {
	t.Helper()
	args := append([]string{"test"}, afterTest...)
	plan.OriginalArgv = append([]string{"go"}, args...)
	plan.Stages.Invoke.Args = append([]string{}, args...)
	plan.TransformedArgv = append([]string{plan.Source.ResolvedPath}, args...)
	if err := plan.Validate(); err != nil {
		t.Fatalf("mutated plan is invalid: %v", err)
	}
	return plan
}

func assertGoAdmission(t *testing.T, err, category error) {
	t.Helper()
	if err == nil || !errors.Is(err, ErrRuntimeUnsupported) || !errors.Is(err, category) {
		t.Fatalf("error = %v, want unsupported and %v", err, category)
	}
	for _, other := range []error{
		ErrRuntimeAdapterContract,
		ErrRuntimeSourceVersion,
		ErrRuntimeCommand,
		ErrRuntimeWrapperOutput,
		ErrRuntimeArgvGrammar,
	} {
		if other != category && errors.Is(err, other) {
			t.Fatalf("error = %v also matched category %v", err, other)
		}
	}
	var categorized interface {
		RuntimeAdmissionCategory() runtimeadmission.Category
	}
	if !errors.As(err, &categorized) || categorized.RuntimeAdmissionCategory() != goCategoryName(category) {
		t.Fatalf("error = %v does not expose category %q", err, goCategoryName(category))
	}
}

func goCategoryName(category error) runtimeadmission.Category {
	switch category {
	case ErrRuntimeAdapterContract:
		return runtimeadmission.CategoryAdapterContract
	case ErrRuntimeSourceVersion:
		return runtimeadmission.CategorySourceVersion
	case ErrRuntimeCommand:
		return runtimeadmission.CategoryCommand
	case ErrRuntimeWrapperOutput:
		return runtimeadmission.CategoryWrapperOutput
	case ErrRuntimeArgvGrammar:
		return runtimeadmission.CategoryArgvGrammar
	default:
		return ""
	}
}
