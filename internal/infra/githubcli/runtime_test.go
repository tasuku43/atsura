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
	return runtimePlan(t, path, []string{"--json=number,title"}, &tailoringbundle.Output{
		Input: "json", Select: []string{"number", "title"}, Rename: []tailoringbundle.Rename{}, Render: "compact_json",
	})
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
		{name: "identity output", mutate: func(_ tailoringplan.Plan) tailoringplan.Plan {
			return runtimePlan(t, []string{"issue", "list"}, []string{}, nil)
		}, category: ErrRuntimeWrapperOutput},
		{name: "invalid plan", mutate: func(plan tailoringplan.Plan) tailoringplan.Plan { plan.SchemaVersion = 0; return plan }, category: ErrRuntimeWrapperOutput},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			plan := test.mutate(transformRuntimePlan(t, "issue", "list"))
			assertRuntimeAdmission(t, VerifyRuntime(plan), ErrRuntimeUnsupported, test.category)
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
