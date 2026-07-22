package tailoringplan

import (
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/tasuku43/atsura/internal/domain/sourcecatalog"
	"github.com/tasuku43/atsura/internal/domain/sourceprocess"
	"github.com/tasuku43/atsura/internal/domain/tailoringbundle"
)

func planCatalog() sourcecatalog.Catalog {
	return sourcecatalog.Catalog{
		SchemaVersion: sourcecatalog.SchemaVersion,
		Adapter:       sourcecatalog.Adapter{Kind: "atsura.source.alternate", ContractVersion: 1},
		Source:        sourcecatalog.Source{RequestedExecutable: "fixture", ResolvedPath: "/opt/bin/fixture", SHA256: strings.Repeat("a", 64), Size: 42, Version: "1.2.3"},
		Probe:         sourcecatalog.Probe{IDs: []string{"help", "version"}, Attempts: 2},
		Commands: []sourcecatalog.Command{
			{Path: []string{"item"}, Summary: "Manage items", Provenance: sourcecatalog.ProvenanceVerifiedBuiltin, Options: []sourcecatalog.Option{}, StructuredOutput: []sourcecatalog.StructuredOutput{}},
			{Path: []string{"item", "delete"}, Summary: "Delete an item", Provenance: sourcecatalog.ProvenanceVerifiedBuiltin, Options: []sourcecatalog.Option{{Name: "--force", TakesValue: false}}, StructuredOutput: []sourcecatalog.StructuredOutput{}},
			{Path: []string{"item", "list"}, Summary: "List items", Provenance: sourcecatalog.ProvenanceVerifiedBuiltin, Options: []sourcecatalog.Option{{Name: "--format", TakesValue: true}, {Name: "--json", TakesValue: true}, {Name: "--limit", TakesValue: true}, {Name: "--verbose", TakesValue: false}}, StructuredOutput: []sourcecatalog.StructuredOutput{{Format: "json", SelectorFlag: "--json", Fields: []string{"id", "name"}}}},
			{Path: []string{"plugin", "run"}, Summary: "Run a plugin", Provenance: sourcecatalog.ProvenanceUnverifiedDynamic, Options: []sourcecatalog.Option{}, StructuredOutput: []sourcecatalog.StructuredOutput{}},
		},
	}
}

func planBundle(t *testing.T, defaultSurface tailoringbundle.SurfaceDefault, entries ...tailoringbundle.CommandEntry) (tailoringbundle.Bundle, string) {
	t.Helper()
	catalog := planCatalog()
	catalogDigest, err := catalog.Digest()
	if err != nil {
		t.Fatal(err)
	}
	specification := tailoringbundle.SortSpecification(tailoringbundle.Specification{
		SchemaVersion: tailoringbundle.SpecificationSchemaVersion,
		CatalogDigest: catalogDigest,
		Surface:       tailoringbundle.Surface{Default: defaultSurface},
		Commands:      append([]tailoringbundle.CommandEntry{}, entries...),
	})
	bundle, err := tailoringbundle.Compile(catalog, specification)
	if err != nil {
		t.Fatal(err)
	}
	digest, err := bundle.Digest()
	if err != nil {
		t.Fatal(err)
	}
	return bundle, digest
}

func currentIdentity() sourceprocess.Identity {
	return sourceprocess.Identity{ResolvedPath: "/opt/bin/fixture", SHA256: strings.Repeat("a", 64), Size: 42}
}

func identityEntry(command ...string) tailoringbundle.CommandEntry {
	return tailoringbundle.CommandEntry{
		Command: command, Presence: tailoringbundle.PresenceInclude, Reason: "Needed for this purpose.",
		Options: &tailoringbundle.OptionSurface{Default: tailoringbundle.SurfaceDefaultInherit, Include: []string{}, Exclude: []string{}},
		Wrapper: &tailoringbundle.Wrapper{Kind: tailoringbundle.WrapperIdentity, Before: []tailoringbundle.StageAction{}, Invoke: tailoringbundle.Invocation{OptionDefaults: []tailoringbundle.OptionDefault{}, AppendArgs: []string{}}, After: []tailoringbundle.StageAction{}},
	}
}

func transformEntry() tailoringbundle.CommandEntry {
	return tailoringbundle.CommandEntry{
		Command: []string{"item", "list"}, Presence: tailoringbundle.PresenceInclude, Reason: "Return a compact inventory.",
		Options: &tailoringbundle.OptionSurface{Default: tailoringbundle.SurfaceDefaultInherit, Include: []string{}, Exclude: []string{"--verbose"}},
		Wrapper: &tailoringbundle.Wrapper{
			Kind: tailoringbundle.WrapperTransform, Before: []tailoringbundle.StageAction{},
			Invoke: tailoringbundle.Invocation{OptionDefaults: []tailoringbundle.OptionDefault{}, AppendArgs: []string{"--json=id,name"}},
			Output: &tailoringbundle.Output{Kind: tailoringbundle.OutputKindProjection, Projection: &tailoringbundle.Projection{Input: "json", Select: []string{"id", "name"}, Rename: []tailoringbundle.Rename{{From: "id", To: "item_id"}}, Render: "compact_json"}},
			After:  []tailoringbundle.StageAction{},
		},
	}
}

func defaultTransformEntry() tailoringbundle.CommandEntry {
	return tailoringbundle.CommandEntry{
		Command: []string{"item", "list"}, Presence: tailoringbundle.PresenceInclude, Reason: "Apply reviewed defaults.",
		Options: &tailoringbundle.OptionSurface{Default: tailoringbundle.SurfaceDefaultInherit, Include: []string{}, Exclude: []string{"--verbose"}},
		Wrapper: &tailoringbundle.Wrapper{
			Kind: tailoringbundle.WrapperTransform, Before: []tailoringbundle.StageAction{},
			Invoke: tailoringbundle.Invocation{
				OptionDefaults: []tailoringbundle.OptionDefault{
					{Option: "--limit", Value: "30"},
					{Option: "--format", Value: "compact"},
				},
				AppendArgs: []string{"fixed-tail"},
			},
			After: []tailoringbundle.StageAction{},
		},
	}
}

func TestBuildProducesDeterministicCompleteTransformPlan(t *testing.T) {
	bundle, digest := planBundle(t, tailoringbundle.SurfaceDefaultExclude, transformEntry())
	attempt := Attempt{Executable: "fixture", Args: []string{"item", "list", "--format=json", "active"}}
	first, err := Build(digest, bundle, currentIdentity(), attempt)
	if err != nil {
		t.Fatal(err)
	}
	second, err := Build(digest, bundle, currentIdentity(), attempt)
	if err != nil {
		t.Fatal(err)
	}
	firstDigest, _ := first.Digest()
	secondDigest, _ := second.Digest()
	if firstDigest != secondDigest || len(firstDigest) != 64 {
		t.Fatalf("digests = %q %q", firstDigest, secondDigest)
	}
	if first.Mode != ModeTailored || first.ResultMode != ResultModeTransformedJSON || first.SurfaceOrigin != SurfaceOriginExplicit || first.SpecificationEntry == nil || first.WrapperKind != tailoringbundle.WrapperTransform || first.Stages.Invoke.MaxAttempts != 1 {
		t.Fatalf("plan identity = %+v", first)
	}
	if !reflect.DeepEqual(first.MatchedCommand, []string{"item", "list"}) || !reflect.DeepEqual(first.OriginalArgv, []string{"fixture", "item", "list", "--format=json", "active"}) {
		t.Fatalf("original binding = %+v", first)
	}
	wantTransformed := []string{"/opt/bin/fixture", "item", "list", "--format=json", "active", "--json=id,name"}
	if !reflect.DeepEqual(first.TransformedArgv, wantTransformed) || !reflect.DeepEqual(first.Stages.Invoke.Args, wantTransformed[1:]) || first.Stages.Output == nil {
		t.Fatalf("transformed plan = %+v", first)
	}
	if first.Stages.Invoke.StdinMode != StdinModeClosed || first.Stages.Invoke.WorkingDirectoryMode != WorkingDirectoryModeInherit || first.Stages.Invoke.EnvironmentMode != EnvironmentModeInherit {
		t.Fatalf("process framing = %+v", first.Stages.Invoke)
	}
	request, err := first.SourceRequest()
	if err != nil || request.Process.Executable != first.Source.ResolvedPath || !reflect.DeepEqual(request.Process.Args, first.Stages.Invoke.Args) || request.ExpectedIdentity != currentIdentity() {
		t.Fatalf("SourceRequest() = %+v, error = %v", request, err)
	}
	output, present, err := first.OutputPlan()
	if err != nil || !present || !reflect.DeepEqual(output.Select, []string{"id", "name"}) || output.Rename[0].To != "item_id" {
		t.Fatalf("OutputPlan() = %+v, %t, %v", output, present, err)
	}
	encoded, err := json.Marshal(first)
	if err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range []string{`"decision"`, `"effect"`, `"target"`, `"impact"`, `"permission"`, `"confirmation"`} {
		if strings.Contains(string(encoded), forbidden) {
			t.Fatalf("plan contains retired field %s: %s", forbidden, encoded)
		}
	}
	for _, required := range []string{`"schema_version":6`, `"result_mode":"transformed_json"`, `"option_defaults":[]`, `"applied_option_defaults":[]`, `"before":[]`, `"after":[]`, `"output":{`, `"surface_origin":"explicit"`} {
		if !strings.Contains(string(encoded), required) {
			t.Fatalf("plan missing %s: %s", required, encoded)
		}
	}
}

func TestBuildUsesFullCatalogLongestPrefixBeforeSurface(t *testing.T) {
	excludedChild := tailoringbundle.CommandEntry{Command: []string{"item", "delete"}, Presence: tailoringbundle.PresenceExclude, Reason: "Not part of this purpose."}
	bundle, digest := planBundle(t, tailoringbundle.SurfaceDefaultExclude, identityEntry("item"), excludedChild)
	_, err := Build(digest, bundle, currentIdentity(), Attempt{Executable: "fixture", Args: []string{"item", "delete", "42"}})
	if !errors.Is(err, ErrCommandNotInSurface) {
		t.Fatalf("Build() error = %v", err)
	}
}

func TestBuildDistinguishesInheritedSurfaceAndIdentityWrapper(t *testing.T) {
	bundle, digest := planBundle(t, tailoringbundle.SurfaceDefaultInherit)
	plan, err := Build(digest, bundle, currentIdentity(), Attempt{Executable: "/opt/bin/fixture", Args: []string{"item", "list", "--json", "id"}})
	if err != nil {
		t.Fatal(err)
	}
	if plan.SurfaceOrigin != SurfaceOriginInherited || plan.SpecificationEntry != nil || plan.WrapperKind != tailoringbundle.WrapperIdentity || plan.ResultMode != ResultModeSourceStreamPassthrough || plan.Stages.Output != nil || len(plan.Stages.Invoke.OptionDefaults) != 0 || len(plan.Stages.Invoke.AppliedOptionDefaults) != 0 || len(plan.Stages.Invoke.AppendedArgs) != 0 {
		t.Fatalf("plan = %+v", plan)
	}
	if _, present, err := plan.OutputPlan(); err != nil || present {
		t.Fatalf("identity OutputPlan() present=%t error=%v", present, err)
	}
	encoded, _ := json.Marshal(plan)
	if !strings.Contains(string(encoded), `"result_mode":"source_stream_passthrough"`) || !strings.Contains(string(encoded), `"output":null`) {
		t.Fatalf("identity output is not explicit null: %s", encoded)
	}
}

func TestBuildDerivesSourceStreamResultForAppendOnlyWrapper(t *testing.T) {
	entry := transformEntry()
	entry.Wrapper.Output = nil
	entry.Wrapper.Invoke.AppendArgs = []string{"--format=json"}
	bundle, digest := planBundle(t, tailoringbundle.SurfaceDefaultExclude, entry)
	plan, err := Build(digest, bundle, currentIdentity(), Attempt{Executable: "fixture", Args: []string{"item", "list"}})
	if err != nil {
		t.Fatal(err)
	}
	if plan.WrapperKind != tailoringbundle.WrapperTransform || plan.ResultMode != ResultModeSourceStreamPassthrough || plan.Stages.Output != nil || !reflect.DeepEqual(plan.Stages.Invoke.AppendedArgs, []string{"--format=json"}) {
		t.Fatalf("append-only plan = %+v", plan)
	}
}

func TestBuildSelectsIndependentPlansFromOneMultiCommandBundle(t *testing.T) {
	bundle, digest := planBundle(
		t,
		tailoringbundle.SurfaceDefaultExclude,
		identityEntry("item", "delete"),
		transformEntry(),
	)

	deletePlan, err := Build(digest, bundle, currentIdentity(), Attempt{
		Executable: "fixture",
		Args:       []string{"item", "delete", "--force"},
	})
	if err != nil {
		t.Fatal(err)
	}
	listPlan, err := Build(digest, bundle, currentIdentity(), Attempt{
		Executable: "fixture",
		Args:       []string{"item", "list"},
	})
	if err != nil {
		t.Fatal(err)
	}

	if deletePlan.BundleDigest != digest || listPlan.BundleDigest != digest {
		t.Fatalf("bundle digests = %q %q, want %q", deletePlan.BundleDigest, listPlan.BundleDigest, digest)
	}
	if !reflect.DeepEqual(deletePlan.MatchedCommand, []string{"item", "delete"}) ||
		deletePlan.SpecificationEntry == nil ||
		!reflect.DeepEqual(deletePlan.SpecificationEntry.Command, []string{"item", "delete"}) ||
		deletePlan.WrapperKind != tailoringbundle.WrapperIdentity ||
		deletePlan.ResultMode != ResultModeSourceStreamPassthrough ||
		deletePlan.Stages.Output != nil ||
		len(deletePlan.Stages.Invoke.AppendedArgs) != 0 {
		t.Fatalf("delete plan = %+v", deletePlan)
	}
	if !reflect.DeepEqual(listPlan.MatchedCommand, []string{"item", "list"}) ||
		listPlan.SpecificationEntry == nil ||
		!reflect.DeepEqual(listPlan.SpecificationEntry.Command, []string{"item", "list"}) ||
		listPlan.WrapperKind != tailoringbundle.WrapperTransform ||
		listPlan.ResultMode != ResultModeTransformedJSON ||
		listPlan.Stages.Output == nil ||
		!reflect.DeepEqual(listPlan.Stages.Invoke.AppendedArgs, []string{"--json=id,name"}) {
		t.Fatalf("list plan = %+v", listPlan)
	}
	deleteDigest, err := deletePlan.Digest()
	if err != nil {
		t.Fatal(err)
	}
	listDigest, err := listPlan.Digest()
	if err != nil {
		t.Fatal(err)
	}
	if deleteDigest == listDigest {
		t.Fatalf("command-specific plan digests unexpectedly match: %q", deleteDigest)
	}
}

func TestBuildEnforcesOptionSurfaceAndDeterministicForms(t *testing.T) {
	entry := identityEntry("item", "list")
	entry.Options = &tailoringbundle.OptionSurface{Default: tailoringbundle.SurfaceDefaultExclude, Include: []string{"--json"}, Exclude: []string{}}
	bundle, digest := planBundle(t, tailoringbundle.SurfaceDefaultExclude, entry)
	tests := []struct {
		name string
		args []string
		want error
	}{
		{name: "inline value", args: []string{"item", "list", "--json=id"}},
		{name: "separated value", args: []string{"item", "list", "--json", "id"}},
		{name: "empty positional value", args: []string{"item", "list", ""}},
		{name: "positional marker", args: []string{"item", "list", "--", "--unknown"}},
		{name: "excluded observed option", args: []string{"item", "list", "--verbose"}, want: ErrOptionNotInSurface},
		{name: "unknown long option", args: []string{"item", "list", "--unknown"}, want: ErrInvalidInvocation},
		{name: "unmodeled short option", args: []string{"item", "list", "-v"}, want: ErrInvalidInvocation},
		{name: "missing value", args: []string{"item", "list", "--json", "--verbose"}, want: ErrInvalidInvocation},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := Build(digest, bundle, currentIdentity(), Attempt{Executable: "fixture", Args: test.args})
			if test.want == nil && err != nil {
				t.Fatalf("Build() error = %v", err)
			}
			if test.want != nil && !errors.Is(err, test.want) {
				t.Fatalf("Build() error = %v, want %v", err, test.want)
			}
		})
	}
}

func TestBuildAppliesOptionDefaultsWithExactCallerPrecedenceAndOrdering(t *testing.T) {
	bundle, digest := planBundle(t, tailoringbundle.SurfaceDefaultExclude, defaultTransformEntry())
	declared := []tailoringbundle.OptionDefault{
		{Option: "--limit", Value: "30"},
		{Option: "--format", Value: "compact"},
	}
	tests := []struct {
		name        string
		callerTail  []string
		wantApplied []tailoringbundle.OptionDefault
		wantArgs    []string
	}{
		{
			name: "omitted values apply in declaration order", callerTail: []string{"active"}, wantApplied: declared,
			wantArgs: []string{"item", "list", "--limit=30", "--format=compact", "active", "fixed-tail"},
		},
		{
			name: "inline caller value suppresses one default", callerTail: []string{"--limit=2", "active"}, wantApplied: declared[1:],
			wantArgs: []string{"item", "list", "--format=compact", "--limit=2", "active", "fixed-tail"},
		},
		{
			name: "separated caller value suppresses one default", callerTail: []string{"--limit", "2", "active"}, wantApplied: declared[1:],
			wantArgs: []string{"item", "list", "--format=compact", "--limit", "2", "active", "fixed-tail"},
		},
		{
			name: "inline empty caller value suppresses one default", callerTail: []string{"--limit=", "active"}, wantApplied: declared[1:],
			wantArgs: []string{"item", "list", "--format=compact", "--limit=", "active", "fixed-tail"},
		},
		{
			name: "separated empty caller value suppresses one default", callerTail: []string{"--limit", "", "active"}, wantApplied: declared[1:],
			wantArgs: []string{"item", "list", "--format=compact", "--limit", "", "active", "fixed-tail"},
		},
		{
			name: "repeated caller values remain exact", callerTail: []string{"--limit=2", "--limit", "3"}, wantApplied: declared[1:],
			wantArgs: []string{"item", "list", "--format=compact", "--limit=2", "--limit", "3", "fixed-tail"},
		},
		{
			name: "caller options suppress every matching default", callerTail: []string{"--format=wide", "--limit=2"}, wantApplied: []tailoringbundle.OptionDefault{},
			wantArgs: []string{"item", "list", "--format=wide", "--limit=2", "fixed-tail"},
		},
		{
			name: "matching text after positional marker does not suppress", callerTail: []string{"--", "--limit=2", "--format", "wide"}, wantApplied: declared,
			wantArgs: []string{"item", "list", "--limit=30", "--format=compact", "--", "--limit=2", "--format", "wide", "fixed-tail"},
		},
	}

	var appliedDigest, overriddenDigest string
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			attemptArgs := append([]string{"item", "list"}, test.callerTail...)
			plan, err := Build(digest, bundle, currentIdentity(), Attempt{Executable: "fixture", Args: attemptArgs})
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(plan.Stages.Invoke.OptionDefaults, declared) {
				t.Fatalf("declared defaults = %#v", plan.Stages.Invoke.OptionDefaults)
			}
			if !reflect.DeepEqual(plan.Stages.Invoke.AppliedOptionDefaults, test.wantApplied) {
				t.Fatalf("applied defaults = %#v, want %#v", plan.Stages.Invoke.AppliedOptionDefaults, test.wantApplied)
			}
			if !reflect.DeepEqual(plan.Stages.Invoke.Args, test.wantArgs) {
				t.Fatalf("invoke args = %#v, want %#v", plan.Stages.Invoke.Args, test.wantArgs)
			}
			wantTransformed := append([]string{"/opt/bin/fixture"}, test.wantArgs...)
			if !reflect.DeepEqual(plan.TransformedArgv, wantTransformed) {
				t.Fatalf("transformed argv = %#v, want %#v", plan.TransformedArgv, wantTransformed)
			}
			planDigest, err := plan.Digest()
			if err != nil {
				t.Fatal(err)
			}
			switch test.name {
			case "omitted values apply in declaration order":
				appliedDigest = planDigest
			case "caller options suppress every matching default":
				overriddenDigest = planDigest
			}
		})
	}
	if appliedDigest == "" || overriddenDigest == "" || appliedDigest == overriddenDigest {
		t.Fatalf("applied and overridden plan digests = %q %q", appliedDigest, overriddenDigest)
	}
}

func TestBuildCanonicalizesOneDefaultTokenAndAllowsDefaultOnlyTransform(t *testing.T) {
	entry := defaultTransformEntry()
	entry.Wrapper.Invoke.OptionDefaults = []tailoringbundle.OptionDefault{{Option: "--limit", Value: "-5 value;$(ignored)"}}
	entry.Wrapper.Invoke.AppendArgs = []string{}
	bundle, digest := planBundle(t, tailoringbundle.SurfaceDefaultExclude, entry)
	plan, err := Build(digest, bundle, currentIdentity(), Attempt{Executable: "fixture", Args: []string{"item", "list"}})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"item", "list", "--limit=-5 value;$(ignored)"}
	if plan.WrapperKind != tailoringbundle.WrapperTransform || plan.ResultMode != ResultModeSourceStreamPassthrough || !reflect.DeepEqual(plan.Stages.Invoke.Args, want) {
		t.Fatalf("default-only plan = %+v", plan)
	}
}

func TestShortOptionDoesNotActAsAnExactLongOptionOverride(t *testing.T) {
	defaults := []tailoringbundle.OptionDefault{{Option: "--limit", Value: "30"}}
	if got := appliedDefaults(defaults, []string{"-L", "2"}); !reflect.DeepEqual(got, defaults) {
		t.Fatalf("appliedDefaults() = %#v, want %#v", got, defaults)
	}
	bundle, digest := planBundle(t, tailoringbundle.SurfaceDefaultExclude, defaultTransformEntry())
	if _, err := Build(digest, bundle, currentIdentity(), Attempt{Executable: "fixture", Args: []string{"item", "list", "-L", "2"}}); !errors.Is(err, ErrInvalidInvocation) {
		t.Fatalf("short caller option error = %v", err)
	}
}

func TestBuildRequiresOneActiveStructuredOutputSelector(t *testing.T) {
	entry := transformEntry()
	entry.Wrapper.Invoke.AppendArgs = []string{}
	bundle, digest := planBundle(t, tailoringbundle.SurfaceDefaultExclude, entry)
	tests := []struct {
		name string
		args []string
		want error
	}{
		{name: "attempt supplies selector", args: []string{"item", "list", "--json=id,name"}},
		{name: "selector absent", args: []string{"item", "list"}, want: ErrInvalidInvocation},
		{name: "selector duplicated", args: []string{"item", "list", "--json=id", "--json=name"}, want: ErrInvalidInvocation},
		{name: "selector is positional", args: []string{"item", "list", "--", "--json=id"}, want: ErrInvalidInvocation},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := Build(digest, bundle, currentIdentity(), Attempt{Executable: "fixture", Args: test.args})
			if test.want == nil && err != nil {
				t.Fatalf("Build() error = %v", err)
			}
			if test.want != nil && !errors.Is(err, test.want) {
				t.Fatalf("Build() error = %v, want %v", err, test.want)
			}
		})
	}

	appended := transformEntry()
	appended.Wrapper.Invoke.AppendArgs = []string{"--json=id,name"}
	appendedBundle, appendedDigest := planBundle(t, tailoringbundle.SurfaceDefaultExclude, appended)
	if _, err := Build(appendedDigest, appendedBundle, currentIdentity(), Attempt{Executable: "fixture", Args: []string{"item", "list", "--", "active"}}); !errors.Is(err, ErrInvalidInvocation) {
		t.Fatalf("selector appended after positional marker error = %v", err)
	}
}

func TestBuildRejectsUnknownCommandExecutableDriftAndOversizedTransformedArgv(t *testing.T) {
	bundle, digest := planBundle(t, tailoringbundle.SurfaceDefaultExclude, identityEntry("item"), identityEntry("item", "list"))
	tests := []struct {
		name    string
		current sourceprocess.Identity
		attempt Attempt
		want    error
	}{
		{name: "unknown command", current: currentIdentity(), attempt: Attempt{Executable: "fixture", Args: []string{"unknown"}}, want: ErrInvalidInvocation},
		{name: "ambiguous unknown child", current: currentIdentity(), attempt: Attempt{Executable: "fixture", Args: []string{"item", "unknown"}}, want: ErrInvalidInvocation},
		{name: "explicit parent positional", current: currentIdentity(), attempt: Attempt{Executable: "fixture", Args: []string{"item", "--", "unknown"}}, want: nil},
		{name: "hostile argument", current: currentIdentity(), attempt: Attempt{Executable: "fixture", Args: []string{"item", "list", "bad\u2028value"}}, want: ErrInvalidInvocation},
		{name: "wrong executable", current: currentIdentity(), attempt: Attempt{Executable: "other", Args: []string{"item", "list"}}, want: ErrSourceExecutableMismatch},
		{name: "source drift", current: sourceprocess.Identity{ResolvedPath: "/opt/bin/fixture", SHA256: strings.Repeat("b", 64), Size: 42}, attempt: Attempt{Executable: "fixture", Args: []string{"item", "list"}}, want: ErrInvalidPlan},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := Build(digest, bundle, test.current, test.attempt)
			if test.want == nil && err != nil {
				t.Fatalf("Build() error = %v", err)
			}
			if test.want != nil && !errors.Is(err, test.want) {
				t.Fatalf("Build() error = %v, want %v", err, test.want)
			}
		})
	}

	transform := transformEntry()
	transform.Wrapper.Invoke.AppendArgs = make([]string, 64)
	for index := range transform.Wrapper.Invoke.AppendArgs {
		transform.Wrapper.Invoke.AppendArgs[index] = "value"
	}
	largeBundle, largeDigest := planBundle(t, tailoringbundle.SurfaceDefaultExclude, transform)
	args := []string{"item", "list"}
	for len(args) < sourceprocess.MaxArguments {
		args = append(args, "value")
	}
	if _, err := Build(largeDigest, largeBundle, currentIdentity(), Attempt{Executable: "fixture", Args: args}); !errors.Is(err, ErrInvalidInvocation) {
		t.Fatalf("oversized transformed Build() error = %v", err)
	}
}

func TestPlanValidationAndDigestDetectMutation(t *testing.T) {
	bundle, digest := planBundle(t, tailoringbundle.SurfaceDefaultExclude, transformEntry())
	plan, err := Build(digest, bundle, currentIdentity(), Attempt{Executable: "fixture", Args: []string{"item", "list"}})
	if err != nil {
		t.Fatal(err)
	}
	baseline, _ := plan.Digest()
	plan.Stages.Invoke.Args[0] = "changed"
	if err := plan.Validate(); !errors.Is(err, ErrInvalidPlan) {
		t.Fatalf("Validate() error = %v", err)
	}
	if mutated, err := plan.Digest(); err == nil || mutated == baseline {
		t.Fatalf("mutated digest = %q, error = %v", mutated, err)
	}
}

func TestPlanValidationReDerivesOptionDefaultDecisionsAndInvocation(t *testing.T) {
	bundle, digest := planBundle(t, tailoringbundle.SurfaceDefaultExclude, defaultTransformEntry())
	attempt := Attempt{Executable: "fixture", Args: []string{"item", "list", "active"}}
	tests := []struct {
		name   string
		mutate func(*Plan)
	}{
		{name: "legacy schema", mutate: func(plan *Plan) { plan.SchemaVersion = 5 }},
		{name: "configured option", mutate: func(plan *Plan) {
			plan.Stages.Invoke.OptionDefaults[0].Option = "--Bad"
			plan.SpecificationEntry.Wrapper.Invoke.OptionDefaults[0].Option = "--Bad"
		}},
		{name: "configured value", mutate: func(plan *Plan) {
			plan.Stages.Invoke.OptionDefaults[0].Value = ""
			plan.SpecificationEntry.Wrapper.Invoke.OptionDefaults[0].Value = ""
		}},
		{name: "configured order", mutate: func(plan *Plan) {
			plan.Stages.Invoke.OptionDefaults[0], plan.Stages.Invoke.OptionDefaults[1] = plan.Stages.Invoke.OptionDefaults[1], plan.Stages.Invoke.OptionDefaults[0]
			plan.SpecificationEntry.Wrapper.Invoke.OptionDefaults[0], plan.SpecificationEntry.Wrapper.Invoke.OptionDefaults[1] = plan.SpecificationEntry.Wrapper.Invoke.OptionDefaults[1], plan.SpecificationEntry.Wrapper.Invoke.OptionDefaults[0]
		}},
		{name: "configured defaults absent", mutate: func(plan *Plan) { plan.Stages.Invoke.OptionDefaults = nil }},
		{name: "applied option", mutate: func(plan *Plan) { plan.Stages.Invoke.AppliedOptionDefaults[0].Option = "--format" }},
		{name: "applied value", mutate: func(plan *Plan) { plan.Stages.Invoke.AppliedOptionDefaults[0].Value = "31" }},
		{name: "applied order", mutate: func(plan *Plan) {
			plan.Stages.Invoke.AppliedOptionDefaults[0], plan.Stages.Invoke.AppliedOptionDefaults[1] = plan.Stages.Invoke.AppliedOptionDefaults[1], plan.Stages.Invoke.AppliedOptionDefaults[0]
		}},
		{name: "applied subset", mutate: func(plan *Plan) {
			plan.Stages.Invoke.AppliedOptionDefaults = plan.Stages.Invoke.AppliedOptionDefaults[:1]
		}},
		{name: "applied defaults absent", mutate: func(plan *Plan) { plan.Stages.Invoke.AppliedOptionDefaults = nil }},
		{name: "invoke args", mutate: func(plan *Plan) { plan.Stages.Invoke.Args[2] = "--limit=31" }},
		{name: "transformed argv", mutate: func(plan *Plan) { plan.TransformedArgv[3] = "--limit=31" }},
		{name: "original caller", mutate: func(plan *Plan) { plan.OriginalArgv = []string{"fixture", "item", "list", "--limit=2", "active"} }},
		{name: "caller default missing separated value", mutate: func(plan *Plan) {
			plan.OriginalArgv = []string{"fixture", "item", "list", "--limit"}
		}},
		{name: "caller default followed by positional marker", mutate: func(plan *Plan) {
			plan.OriginalArgv = []string{"fixture", "item", "list", "--limit", "--", "active"}
		}},
		{name: "duplicate coordinated defaults", mutate: func(plan *Plan) {
			plan.Stages.Invoke.OptionDefaults[1].Option = "--limit"
			plan.SpecificationEntry.Wrapper.Invoke.OptionDefaults[1].Option = "--limit"
		}},
		{name: "unsafe coordinated default", mutate: func(plan *Plan) {
			plan.Stages.Invoke.OptionDefaults[0].Value = "bad\nvalue"
			plan.SpecificationEntry.Wrapper.Invoke.OptionDefaults[0].Value = "bad\nvalue"
		}},
		{name: "oversize canonical default argument", mutate: func(plan *Plan) {
			value := strings.Repeat("x", sourceprocess.MaxArgumentBytes-len("--limit=")+1)
			plan.Stages.Invoke.OptionDefaults[0].Value = value
			plan.SpecificationEntry.Wrapper.Invoke.OptionDefaults[0].Value = value
		}},
		{name: "coordinated append overlap", mutate: func(plan *Plan) {
			plan.Stages.Invoke.AppendedArgs = []string{"--limit=1"}
			plan.SpecificationEntry.Wrapper.Invoke.AppendArgs = []string{"--limit=1"}
		}},
		{name: "combined transform bound", mutate: func(plan *Plan) {
			values := make([]tailoringbundle.OptionDefault, tailoringbundle.MaxWrapperArguments+1)
			for index := range values {
				values[index] = tailoringbundle.OptionDefault{Option: "--limit", Value: "30"}
			}
			plan.Stages.Invoke.OptionDefaults = values
			plan.SpecificationEntry.Wrapper.Invoke.OptionDefaults = append([]tailoringbundle.OptionDefault{}, values...)
		}},
		{name: "specification default", mutate: func(plan *Plan) { plan.SpecificationEntry.Wrapper.Invoke.OptionDefaults[0].Value = "31" }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			plan, err := Build(digest, bundle, currentIdentity(), attempt)
			if err != nil {
				t.Fatal(err)
			}
			test.mutate(&plan)
			if err := plan.Validate(); !errors.Is(err, ErrInvalidPlan) {
				t.Fatalf("Validate() error = %v", err)
			}
			if mutatedDigest, err := plan.Digest(); err == nil || mutatedDigest != "" {
				t.Fatalf("Digest() = %q, error = %v", mutatedDigest, err)
			}
		})
	}

	overridden, err := Build(digest, bundle, currentIdentity(), Attempt{Executable: "fixture", Args: []string{"item", "list", "--limit=2"}})
	if err != nil {
		t.Fatal(err)
	}
	overridden.Stages.Invoke.AppliedOptionDefaults = append(
		[]tailoringbundle.OptionDefault{overridden.Stages.Invoke.OptionDefaults[0]},
		overridden.Stages.Invoke.AppliedOptionDefaults...,
	)
	if err := overridden.Validate(); !errors.Is(err, ErrInvalidPlan) {
		t.Fatalf("forged caller-suppressed default Validate() error = %v", err)
	}
}

func TestPlanValidationRejectsMissingUnknownAndContradictoryResultModes(t *testing.T) {
	transformBundle, transformDigest := planBundle(t, tailoringbundle.SurfaceDefaultExclude, transformEntry())
	transformPlan, err := Build(transformDigest, transformBundle, currentIdentity(), Attempt{Executable: "fixture", Args: []string{"item", "list"}})
	if err != nil {
		t.Fatal(err)
	}
	identityBundle, identityDigest := planBundle(t, tailoringbundle.SurfaceDefaultExclude, identityEntry("item", "list"))
	identityPlan, err := Build(identityDigest, identityBundle, currentIdentity(), Attempt{Executable: "fixture", Args: []string{"item", "list"}})
	if err != nil {
		t.Fatal(err)
	}
	tests := []Plan{
		func() Plan { value := transformPlan; value.ResultMode = ""; return value }(),
		func() Plan { value := transformPlan; value.ResultMode = ResultMode("unknown"); return value }(),
		func() Plan {
			value := transformPlan
			value.ResultMode = ResultModeSourceStreamPassthrough
			return value
		}(),
		func() Plan { value := identityPlan; value.ResultMode = ResultModeTransformedJSON; return value }(),
	}
	for index, plan := range tests {
		if err := plan.Validate(); !errors.Is(err, ErrInvalidPlan) {
			t.Errorf("plan %d validation error = %v", index, err)
		}
		if digest, err := plan.Digest(); err == nil || digest != "" {
			t.Errorf("plan %d digest = %q, error = %v", index, digest, err)
		}
	}
}

func TestBuildDetachesPlanFromBundle(t *testing.T) {
	entry := transformEntry()
	entry.Wrapper.Invoke.OptionDefaults = []tailoringbundle.OptionDefault{{Option: "--limit", Value: "30"}}
	bundle, digest := planBundle(t, tailoringbundle.SurfaceDefaultExclude, entry)
	attempt := Attempt{Executable: "fixture", Args: []string{"item", "list"}}
	plan, err := Build(digest, bundle, currentIdentity(), attempt)
	if err != nil {
		t.Fatal(err)
	}
	wantDigest, err := plan.Digest()
	if err != nil {
		t.Fatal(err)
	}
	plan.Options.Exclude[0] = "--changed"
	plan.SpecificationEntry.Command[0] = "changed"
	plan.SpecificationEntry.Options.Exclude[0] = "--changed"
	plan.SpecificationEntry.Wrapper.Invoke.OptionDefaults[0].Value = "changed"
	plan.SpecificationEntry.Wrapper.Invoke.AppendArgs[0] = "changed"
	plan.SpecificationEntry.Wrapper.Output.Projection.Select[0] = "changed"
	plan.Stages.Invoke.OptionDefaults[0].Value = "changed"
	plan.Stages.Invoke.AppliedOptionDefaults[0].Value = "changed"
	plan.Stages.Invoke.AppendedArgs[0] = "changed"
	plan.Stages.Output.Projection.Select[0] = "changed"

	rebuilt, err := Build(digest, bundle, currentIdentity(), attempt)
	if err != nil {
		t.Fatalf("Build() after plan mutation: %v", err)
	}
	gotDigest, err := rebuilt.Digest()
	if err != nil || gotDigest != wantDigest {
		t.Fatalf("rebuilt digest=%q want=%q error=%v", gotDigest, wantDigest, err)
	}
}
