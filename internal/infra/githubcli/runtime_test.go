package githubcli

import (
	"errors"
	"strings"
	"testing"

	"github.com/tasuku43/atsura/internal/domain/runtimeadmission"
	"github.com/tasuku43/atsura/internal/domain/sourcecatalog"
	"github.com/tasuku43/atsura/internal/domain/sourceprocess"
	"github.com/tasuku43/atsura/internal/domain/tailoringbundle"
	"github.com/tasuku43/atsura/internal/domain/tailoringplan"
)

func runtimePlan(t *testing.T, path []string, appendArgs []string, output *tailoringbundle.Output) tailoringplan.Plan {
	t.Helper()
	command := sourcecatalog.Command{
		Path: path, Summary: "List records", Provenance: sourcecatalog.ProvenanceVerifiedBuiltin,
		Options:          []sourcecatalog.Option{{Name: "--json", TakesValue: true}},
		StructuredOutput: []sourcecatalog.StructuredOutput{{Format: "json", SelectorFlag: "--json", Fields: []string{"number", "title"}}},
	}
	catalog := sourcecatalog.Sort(sourcecatalog.Catalog{
		SchemaVersion: sourcecatalog.SchemaVersion,
		Adapter:       sourcecatalog.Adapter{Kind: AdapterKind, ContractVersion: ContractVersion},
		Source: sourcecatalog.Source{
			RequestedExecutable: "gh", ResolvedPath: "/opt/bin/gh", SHA256: strings.Repeat("a", 64), Size: 2048, Version: "2.72.0",
		},
		Probe:    sourcecatalog.Probe{IDs: []string{"help_reference", "issue_list_help", "pr_list_help", "version"}, Attempts: 4},
		Commands: []sourcecatalog.Command{command},
	})
	catalogDigest, err := catalog.Digest()
	if err != nil {
		t.Fatal(err)
	}
	wrapperKind := tailoringbundle.WrapperTransform
	if output == nil && len(appendArgs) == 0 {
		wrapperKind = tailoringbundle.WrapperIdentity
	}
	entry := tailoringbundle.CommandEntry{
		Command: path, Presence: tailoringbundle.PresenceInclude, Reason: "Return a reviewed compact result.",
		Options: &tailoringbundle.OptionSurface{Default: tailoringbundle.SurfaceDefaultInherit, Include: []string{}, Exclude: []string{}},
		Wrapper: &tailoringbundle.Wrapper{
			Kind: wrapperKind, Before: []tailoringbundle.StageAction{}, Invoke: tailoringbundle.Invocation{AppendArgs: appendArgs}, Output: output, After: []tailoringbundle.StageAction{},
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
	plan, err := tailoringplan.Build(bundleDigest, bundle, sourceprocess.Identity{ResolvedPath: "/opt/bin/gh", SHA256: strings.Repeat("a", 64), Size: 2048}, tailoringplan.Attempt{Executable: "gh", Args: path})
	if err != nil {
		t.Fatal(err)
	}
	return plan
}

func transformRuntimePlan(t *testing.T, path ...string) tailoringplan.Plan {
	t.Helper()
	return runtimePlan(t, path, []string{"--json=number,title"}, runtimeProjectionOutput())
}

func runtimeSurfaceBundle(t *testing.T, path []string, options []sourcecatalog.Option, surface tailoringbundle.OptionSurface, wrapper tailoringbundle.Wrapper) tailoringbundle.Bundle {
	t.Helper()
	catalog := sourcecatalog.Sort(sourcecatalog.Catalog{
		SchemaVersion: sourcecatalog.SchemaVersion,
		Adapter:       sourcecatalog.Adapter{Kind: AdapterKind, ContractVersion: ContractVersion},
		Source: sourcecatalog.Source{
			RequestedExecutable: "gh", ResolvedPath: "/opt/bin/gh", SHA256: strings.Repeat("a", 64), Size: 2048, Version: "2.72.0",
		},
		Probe: sourcecatalog.Probe{IDs: []string{"help_reference", "issue_list_help", "pr_list_help", "version"}, Attempts: 4},
		Commands: []sourcecatalog.Command{{
			Path: path, Summary: "List records", Provenance: sourcecatalog.ProvenanceVerifiedBuiltin,
			Options: options,
			StructuredOutput: []sourcecatalog.StructuredOutput{{
				Format: "json", SelectorFlag: "--json", Fields: []string{"number", "title"},
			}},
		}},
	})
	return compileRuntimeSurface(t, catalog, []tailoringbundle.CommandEntry{{
		Command: path, Presence: tailoringbundle.PresenceInclude, Reason: "Return a reviewed compact result.", Options: &surface, Wrapper: &wrapper,
	}})
}

func compileRuntimeSurface(t *testing.T, catalog sourcecatalog.Catalog, entries []tailoringbundle.CommandEntry) tailoringbundle.Bundle {
	t.Helper()
	catalog = sourcecatalog.Sort(catalog)
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

func admittedSurfaceWrapper() tailoringbundle.Wrapper {
	return tailoringbundle.Wrapper{
		Kind:   tailoringbundle.WrapperTransform,
		Before: []tailoringbundle.StageAction{},
		Invoke: tailoringbundle.Invocation{AppendArgs: []string{"--json=number,title"}},
		Output: runtimeProjectionOutput(),
		After:  []tailoringbundle.StageAction{},
	}
}

func runtimeProjectionOutput() *tailoringbundle.Output {
	return &tailoringbundle.Output{
		Kind: tailoringbundle.OutputKindProjection,
		Projection: &tailoringbundle.Projection{
			Input: "json", Select: []string{"number", "title"}, Rename: []tailoringbundle.Rename{}, Render: "compact_json",
		},
	}
}

func replaceAppendedArgs(t *testing.T, plan tailoringplan.Plan, args []string) tailoringplan.Plan {
	t.Helper()
	plan.Stages.Invoke.AppendedArgs = append([]string{}, args...)
	plan.Stages.Invoke.Args = append(append([]string{}, plan.MatchedCommand...), args...)
	plan.TransformedArgv = append([]string{plan.Source.ResolvedPath}, plan.Stages.Invoke.Args...)
	plan.SpecificationEntry.Wrapper.Invoke.AppendArgs = append([]string{}, args...)
	if err := plan.Validate(); err != nil {
		t.Fatalf("mutated plan is invalid: %v", err)
	}
	return plan
}

func TestVerifyRuntimeProvesSupportedGitHubJSONSelectors(t *testing.T) {
	verifier := NewRuntimeVerifier()
	for _, path := range [][]string{{"issue", "list"}, {"pr", "list"}} {
		t.Run(strings.Join(path, "_"), func(t *testing.T) {
			plan := transformRuntimePlan(t, path...)
			if plan.ResultMode != tailoringplan.ResultModeTransformedJSON {
				t.Fatalf("result mode = %q", plan.ResultMode)
			}
			if err := verifier.VerifyRuntime(plan); err != nil {
				t.Fatal(err)
			}
			plan.Source.Version = "2.72.0-pre.1"
			if err := VerifyRuntime(plan); err != nil {
				t.Fatalf("supported major-2 prerelease: %v", err)
			}
			plan = replaceAppendedArgs(t, plan, []string{"--limit", "1", "--state=all", "--json=number,title"})
			if err := VerifyRuntime(plan); err != nil {
				t.Fatalf("supported filtering argv: %v", err)
			}
		})
	}
}

func TestVerifyRuntimeProvesSupportedGitHubSourceStreamWrappers(t *testing.T) {
	verifier := NewRuntimeVerifier()
	for _, path := range [][]string{{"issue", "list"}, {"pr", "list"}} {
		t.Run(strings.Join(path, "_")+"_identity", func(t *testing.T) {
			plan := runtimePlan(t, path, []string{}, nil)
			if plan.ResultMode != tailoringplan.ResultModeSourceStreamPassthrough || plan.WrapperKind != tailoringbundle.WrapperIdentity {
				t.Fatalf("plan result/wrapper = %q/%q", plan.ResultMode, plan.WrapperKind)
			}
			if err := verifier.VerifyRuntime(plan); err != nil {
				t.Fatal(err)
			}
		})
		t.Run(strings.Join(path, "_")+"_append_only", func(t *testing.T) {
			plan := runtimePlan(t, path, []string{"--limit=1", "--label=one", "--label=two", "--repo=-dash"}, nil)
			if plan.ResultMode != tailoringplan.ResultModeSourceStreamPassthrough || plan.WrapperKind != tailoringbundle.WrapperTransform {
				t.Fatalf("plan result/wrapper = %q/%q", plan.ResultMode, plan.WrapperKind)
			}
			if err := verifier.VerifyRuntime(plan); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestVerifyRuntimeRejectsUnsupportedContracts(t *testing.T) {
	tests := []struct {
		name     string
		mutate   func(tailoringplan.Plan) tailoringplan.Plan
		category error
	}{
		{name: "adapter kind", mutate: func(plan tailoringplan.Plan) tailoringplan.Plan {
			plan.Source.AdapterKind = "atsura.source.alternate"
			return plan
		}, category: ErrRuntimeAdapterContract},
		{name: "contract version", mutate: func(plan tailoringplan.Plan) tailoringplan.Plan { plan.Source.AdapterContractVersion = 1; return plan }, category: ErrRuntimeAdapterContract},
		{name: "major version", mutate: func(plan tailoringplan.Plan) tailoringplan.Plan { plan.Source.Version = "3.0.0"; return plan }, category: ErrRuntimeSourceVersion},
		{name: "malformed version", mutate: func(plan tailoringplan.Plan) tailoringplan.Plan { plan.Source.Version = "2"; return plan }, category: ErrRuntimeSourceVersion},
		{name: "unsupported command", mutate: func(_ tailoringplan.Plan) tailoringplan.Plan { return transformRuntimePlan(t, "release", "list") }, category: ErrRuntimeCommand},
		{name: "invalid plan", mutate: func(plan tailoringplan.Plan) tailoringplan.Plan { plan.SchemaVersion = 0; return plan }, category: ErrRuntimeWrapperOutput},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			plan := test.mutate(transformRuntimePlan(t, "issue", "list"))
			assertRuntimeAdmission(t, VerifyRuntime(plan), ErrRuntimeUnsupported, test.category)
		})
	}
}

func TestVerifyRuntimeRejectsUnprovenSourceStreamArgs(t *testing.T) {
	tests := []struct {
		name     string
		args     []string
		legacy   error
		category error
	}{
		{name: "json output mode", args: []string{"--json=number,title"}, legacy: ErrRuntimeSelector, category: ErrRuntimeSelectorConflict},
		{name: "jq output mode", args: []string{"--jq=.[]"}, legacy: ErrRuntimeSelector, category: ErrRuntimeSelectorConflict},
		{name: "template output mode", args: []string{"--template={{.number}}"}, legacy: ErrRuntimeSelector, category: ErrRuntimeSelectorConflict},
		{name: "web output mode", args: []string{"--web"}, legacy: ErrRuntimeSelector, category: ErrRuntimeSelectorConflict},
		{name: "positional", args: []string{"unexpected"}, legacy: ErrRuntimeUnsupported, category: ErrRuntimeArgvGrammar},
		{name: "append after marker", args: []string{"--", "--limit=1"}, legacy: ErrRuntimeUnsupported, category: ErrRuntimeArgvGrammar},
		{name: "unknown option", args: []string{"--unknown=value"}, legacy: ErrRuntimeUnsupported, category: ErrRuntimeArgvGrammar},
		{name: "missing separated value", args: []string{"--limit"}, legacy: ErrRuntimeUnsupported, category: ErrRuntimeArgvGrammar},
		{name: "dash separated value", args: []string{"--repo", "-dash"}, legacy: ErrRuntimeUnsupported, category: ErrRuntimeArgvGrammar},
		{name: "empty inline value", args: []string{"--limit="}, legacy: ErrRuntimeUnsupported, category: ErrRuntimeArgvGrammar},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			plan := runtimePlan(t, []string{"pr", "list"}, test.args, nil)
			assertRuntimeAdmission(t, VerifyRuntime(plan), test.legacy, test.category)
		})
	}
}

func TestVerifyRuntimeRejectsUnprovenSelectors(t *testing.T) {
	tests := []struct {
		name string
		args []string
	}{
		{name: "missing", args: []string{}},
		{name: "separated", args: []string{"--json", "number,title"}},
		{name: "wrong order", args: []string{"--json=title,number"}},
		{name: "duplicate", args: []string{"--json=number,title", "--json=number,title"}},
		{name: "empty", args: []string{"--json="}},
		{name: "jq conflict", args: []string{"--jq=.[]", "--json=number,title"}},
		{name: "template conflict", args: []string{"--template={{.number}}", "--json=number,title"}},
		{name: "web conflict", args: []string{"--web", "--json=number,title"}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			plan := replaceAppendedArgs(t, transformRuntimePlan(t, "issue", "list"), test.args)
			assertRuntimeAdmission(t, VerifyRuntime(plan), ErrRuntimeSelector, ErrRuntimeSelectorConflict)
		})
	}
}

func TestVerifyRuntimeRejectsUnmodeledArgv(t *testing.T) {
	tests := []struct {
		name string
		args []string
	}{
		{name: "positional", args: []string{"unexpected", "--json=number,title"}},
		{name: "positional after marker", args: []string{"--json=number,title", "--", "unexpected"}},
		{name: "unknown option", args: []string{"--unknown=value", "--json=number,title"}},
		{name: "missing separated value", args: []string{"--limit", "--json=number,title"}},
		{name: "boolean value", args: []string{"--draft=true", "--json=number,title"}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			plan := replaceAppendedArgs(t, transformRuntimePlan(t, "pr", "list"), test.args)
			assertRuntimeAdmission(t, VerifyRuntime(plan), ErrRuntimeUnsupported, ErrRuntimeArgvGrammar)
		})
	}
}

func TestVerifySurfaceAdmitsOneCompleteTransformSurface(t *testing.T) {
	verifier := NewRuntimeVerifier()
	for _, test := range []struct {
		name    string
		path    []string
		options []sourcecatalog.Option
		surface tailoringbundle.OptionSurface
	}{
		{
			name: "exclude by default with one admitted option", path: []string{"pr", "list"},
			options: []sourcecatalog.Option{{Name: "--json", TakesValue: true}, {Name: "--limit", TakesValue: true}},
			surface: tailoringbundle.OptionSurface{Default: tailoringbundle.SurfaceDefaultExclude, Include: []string{"--limit"}, Exclude: []string{}},
		},
		{
			name: "inherit only admitted options", path: []string{"issue", "list"},
			options: []sourcecatalog.Option{{Name: "--json", TakesValue: true}, {Name: "--limit", TakesValue: true}, {Name: "--state", TakesValue: true}},
			surface: tailoringbundle.OptionSurface{Default: tailoringbundle.SurfaceDefaultInherit, Include: []string{}, Exclude: []string{"--json"}},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			bundle := runtimeSurfaceBundle(t, test.path, test.options, test.surface, admittedSurfaceWrapper())
			if err := verifier.VerifySurface(bundle); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestVerifySurfaceAdmitsOneCompleteSourceStreamSurface(t *testing.T) {
	baseOptions := []sourcecatalog.Option{
		{Name: "--json", TakesValue: true},
		{Name: "--label", TakesValue: true},
		{Name: "--limit", TakesValue: true},
		{Name: "--repo", TakesValue: true},
	}
	baseSurface := tailoringbundle.OptionSurface{Default: tailoringbundle.SurfaceDefaultExclude, Include: []string{"--label", "--limit", "--repo"}, Exclude: []string{}}
	tests := []struct {
		name    string
		path    []string
		wrapper tailoringbundle.Wrapper
	}{
		{
			name: "identity", path: []string{"pr", "list"},
			wrapper: tailoringbundle.Wrapper{Kind: tailoringbundle.WrapperIdentity, Before: []tailoringbundle.StageAction{}, Invoke: tailoringbundle.Invocation{AppendArgs: []string{}}, After: []tailoringbundle.StageAction{}},
		},
		{
			name: "append only", path: []string{"issue", "list"},
			wrapper: tailoringbundle.Wrapper{Kind: tailoringbundle.WrapperTransform, Before: []tailoringbundle.StageAction{}, Invoke: tailoringbundle.Invocation{AppendArgs: []string{"--limit=1", "--label=one", "--label=two"}}, After: []tailoringbundle.StageAction{}},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			bundle := runtimeSurfaceBundle(t, test.path, baseOptions, baseSurface, test.wrapper)
			if err := NewRuntimeVerifier().VerifySurface(bundle); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestVerifySurfaceRejectsMixedAndPartialSurfaces(t *testing.T) {
	baseOptions := []sourcecatalog.Option{{Name: "--json", TakesValue: true}, {Name: "--limit", TakesValue: true}}
	baseSurface := tailoringbundle.OptionSurface{Default: tailoringbundle.SurfaceDefaultExclude, Include: []string{"--limit"}, Exclude: []string{}}

	t.Run("mixed commands", func(t *testing.T) {
		bundle := runtimeSurfaceBundle(t, []string{"pr", "list"}, baseOptions, baseSurface, admittedSurfaceWrapper())
		catalog := bundle.Catalog
		catalog.Commands = append(catalog.Commands, sourcecatalog.Command{
			Path: []string{"issue", "list"}, Summary: "List issues", Provenance: sourcecatalog.ProvenanceVerifiedBuiltin,
			Options:          baseOptions,
			StructuredOutput: []sourcecatalog.StructuredOutput{{Format: "json", SelectorFlag: "--json", Fields: []string{"number", "title"}}},
		})
		secondSurface := baseSurface
		secondWrapper := admittedSurfaceWrapper()
		bundle = compileRuntimeSurface(t, catalog, []tailoringbundle.CommandEntry{
			{Command: []string{"issue", "list"}, Presence: tailoringbundle.PresenceInclude, Reason: "Needed.", Options: &secondSurface, Wrapper: &secondWrapper},
			{Command: []string{"pr", "list"}, Presence: tailoringbundle.PresenceInclude, Reason: "Needed.", Options: &baseSurface, Wrapper: ptrWrapper(admittedSurfaceWrapper())},
		})
		assertRuntimeAdmission(t, VerifySurface(bundle), ErrRuntimeUnsupported, ErrRuntimeWrapperOutput)
	})

	tests := []struct {
		name     string
		path     []string
		options  []sourcecatalog.Option
		surface  tailoringbundle.OptionSurface
		wrapper  tailoringbundle.Wrapper
		legacy   error
		category error
	}{
		{
			name: "unsupported command", path: []string{"release", "list"}, options: baseOptions, surface: baseSurface,
			wrapper: admittedSurfaceWrapper(), legacy: ErrRuntimeUnsupported, category: ErrRuntimeCommand,
		},
		{
			name: "exposed selector", path: []string{"pr", "list"}, options: baseOptions,
			surface: tailoringbundle.OptionSurface{Default: tailoringbundle.SurfaceDefaultExclude, Include: []string{"--json"}, Exclude: []string{}},
			wrapper: admittedSurfaceWrapper(), legacy: ErrRuntimeSelector, category: ErrRuntimeSelectorConflict,
		},
		{
			name: "partially admitted option", path: []string{"pr", "list"},
			options: []sourcecatalog.Option{{Name: "--json", TakesValue: true}, {Name: "--unknown", TakesValue: true}},
			surface: tailoringbundle.OptionSurface{Default: tailoringbundle.SurfaceDefaultExclude, Include: []string{"--unknown"}, Exclude: []string{}},
			wrapper: admittedSurfaceWrapper(), legacy: ErrRuntimeUnsupported, category: ErrRuntimeArgvGrammar,
		},
		{
			name: "wrong option cardinality", path: []string{"pr", "list"},
			options: []sourcecatalog.Option{{Name: "--json", TakesValue: true}, {Name: "--limit", TakesValue: false}},
			surface: baseSurface, wrapper: admittedSurfaceWrapper(), legacy: ErrRuntimeUnsupported, category: ErrRuntimeArgvGrammar,
		},
		{
			name: "missing fixed selector", path: []string{"pr", "list"}, options: baseOptions, surface: baseSurface,
			wrapper: tailoringbundle.Wrapper{
				Kind: tailoringbundle.WrapperTransform, Before: []tailoringbundle.StageAction{}, Invoke: tailoringbundle.Invocation{AppendArgs: []string{"--limit=1"}},
				Output: runtimeProjectionOutput(), After: []tailoringbundle.StageAction{},
			},
			legacy: ErrRuntimeSelector, category: ErrRuntimeSelectorConflict,
		},
		{
			name: "source stream exposed selector", path: []string{"pr", "list"}, options: baseOptions,
			surface: tailoringbundle.OptionSurface{Default: tailoringbundle.SurfaceDefaultExclude, Include: []string{"--json"}, Exclude: []string{}},
			wrapper: tailoringbundle.Wrapper{Kind: tailoringbundle.WrapperIdentity, Before: []tailoringbundle.StageAction{}, Invoke: tailoringbundle.Invocation{AppendArgs: []string{}}, After: []tailoringbundle.StageAction{}},
			legacy:  ErrRuntimeSelector, category: ErrRuntimeSelectorConflict,
		},
		{
			name: "source stream unmodeled append", path: []string{"pr", "list"}, options: baseOptions, surface: baseSurface,
			wrapper: tailoringbundle.Wrapper{Kind: tailoringbundle.WrapperTransform, Before: []tailoringbundle.StageAction{}, Invoke: tailoringbundle.Invocation{AppendArgs: []string{"--unknown=1"}}, After: []tailoringbundle.StageAction{}},
			legacy:  ErrRuntimeUnsupported, category: ErrRuntimeArgvGrammar,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			bundle := runtimeSurfaceBundle(t, test.path, test.options, test.surface, test.wrapper)
			assertRuntimeAdmission(t, VerifySurface(bundle), test.legacy, test.category)
		})
	}
}

func ptrWrapper(value tailoringbundle.Wrapper) *tailoringbundle.Wrapper { return &value }

func assertRuntimeAdmission(t *testing.T, err, legacy, category error) {
	t.Helper()
	if err == nil || !errors.Is(err, legacy) || !errors.Is(err, category) {
		t.Fatalf("error=%v, want legacy=%v category=%v", err, legacy, category)
	}
	for _, other := range []error{
		ErrRuntimeAdapterContract,
		ErrRuntimeSourceVersion,
		ErrRuntimeCommand,
		ErrRuntimeWrapperOutput,
		ErrRuntimeArgvGrammar,
		ErrRuntimeSelectorConflict,
	} {
		if other != category && errors.Is(err, other) {
			t.Fatalf("error=%v also matched category=%v", err, other)
		}
	}
	if legacy == ErrRuntimeUnsupported && errors.Is(err, ErrRuntimeSelector) {
		t.Fatalf("unsupported error also matched selector: %v", err)
	}
	if legacy == ErrRuntimeSelector && errors.Is(err, ErrRuntimeUnsupported) {
		t.Fatalf("selector error also matched unsupported: %v", err)
	}
	var categorized interface {
		RuntimeAdmissionCategory() runtimeadmission.Category
	}
	if !errors.As(err, &categorized) || categorized.RuntimeAdmissionCategory() != categoryName(category) {
		t.Fatalf("error=%v does not expose a finite admission category", err)
	}
}

func categoryName(category error) runtimeadmission.Category {
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
	case ErrRuntimeSelectorConflict:
		return runtimeadmission.CategorySelectorConflict
	default:
		return ""
	}
}
