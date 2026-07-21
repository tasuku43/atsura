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
		SchemaVersion: 1,
		Adapter:       sourcecatalog.Adapter{Kind: "atsura.source.alternate", ContractVersion: 1},
		Source:        sourcecatalog.Source{RequestedExecutable: "fixture", ResolvedPath: "/opt/bin/fixture", SHA256: strings.Repeat("a", 64), Size: 42, Version: "1.2.3"},
		Probe:         sourcecatalog.Probe{IDs: []string{"help", "version"}, Attempts: 2},
		Commands: []sourcecatalog.Command{
			{Path: []string{"item"}, Summary: "Manage items", Provenance: sourcecatalog.ProvenanceVerifiedBuiltin, Options: []sourcecatalog.Option{}, StructuredOutput: []sourcecatalog.StructuredOutput{}},
			{Path: []string{"item", "delete"}, Summary: "Delete an item", Provenance: sourcecatalog.ProvenanceVerifiedBuiltin, Options: []sourcecatalog.Option{{Name: "--force", TakesValue: false}}, StructuredOutput: []sourcecatalog.StructuredOutput{}},
			{Path: []string{"item", "list"}, Summary: "List items", Provenance: sourcecatalog.ProvenanceVerifiedBuiltin, Options: []sourcecatalog.Option{{Name: "--format", TakesValue: true}, {Name: "--json", TakesValue: true}, {Name: "--verbose", TakesValue: false}}, StructuredOutput: []sourcecatalog.StructuredOutput{{Format: "json", SelectorFlag: "--json", Fields: []string{"id", "name"}}}},
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
		Wrapper: &tailoringbundle.Wrapper{Kind: tailoringbundle.WrapperIdentity, Before: []tailoringbundle.StageAction{}, Invoke: tailoringbundle.Invocation{AppendArgs: []string{}}, After: []tailoringbundle.StageAction{}},
	}
}

func transformEntry() tailoringbundle.CommandEntry {
	return tailoringbundle.CommandEntry{
		Command: []string{"item", "list"}, Presence: tailoringbundle.PresenceInclude, Reason: "Return a compact inventory.",
		Options: &tailoringbundle.OptionSurface{Default: tailoringbundle.SurfaceDefaultInherit, Include: []string{}, Exclude: []string{"--verbose"}},
		Wrapper: &tailoringbundle.Wrapper{
			Kind: tailoringbundle.WrapperTransform, Before: []tailoringbundle.StageAction{},
			Invoke: tailoringbundle.Invocation{AppendArgs: []string{"--json=id,name"}},
			Output: &tailoringbundle.Output{Input: "json", Select: []string{"id", "name"}, Rename: []tailoringbundle.Rename{{From: "id", To: "item_id"}}, Render: "compact_json"},
			After:  []tailoringbundle.StageAction{},
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
	if first.Mode != ModeTailored || first.SurfaceOrigin != SurfaceOriginExplicit || first.SpecificationEntry == nil || first.WrapperKind != tailoringbundle.WrapperTransform || first.Stages.Invoke.MaxAttempts != 1 {
		t.Fatalf("plan identity = %+v", first)
	}
	if !reflect.DeepEqual(first.MatchedCommand, []string{"item", "list"}) || !reflect.DeepEqual(first.OriginalArgv, []string{"fixture", "item", "list", "--format=json", "active"}) {
		t.Fatalf("original binding = %+v", first)
	}
	wantTransformed := []string{"/opt/bin/fixture", "item", "list", "--format=json", "active", "--json=id,name"}
	if !reflect.DeepEqual(first.TransformedArgv, wantTransformed) || !reflect.DeepEqual(first.Stages.Invoke.Args, wantTransformed[1:]) || first.Stages.Output == nil {
		t.Fatalf("transformed plan = %+v", first)
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
	for _, required := range []string{`"before":[]`, `"after":[]`, `"output":{`, `"surface_origin":"explicit"`} {
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
	if plan.SurfaceOrigin != SurfaceOriginInherited || plan.SpecificationEntry != nil || plan.WrapperKind != tailoringbundle.WrapperIdentity || plan.Stages.Output != nil || len(plan.Stages.Invoke.AppendedArgs) != 0 {
		t.Fatalf("plan = %+v", plan)
	}
	encoded, _ := json.Marshal(plan)
	if !strings.Contains(string(encoded), `"output":null`) {
		t.Fatalf("identity output is not explicit null: %s", encoded)
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

func TestBuildDetachesPlanFromBundle(t *testing.T) {
	bundle, digest := planBundle(t, tailoringbundle.SurfaceDefaultExclude, transformEntry())
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
	plan.SpecificationEntry.Wrapper.Invoke.AppendArgs[0] = "changed"
	plan.SpecificationEntry.Wrapper.Output.Select[0] = "changed"
	plan.Stages.Invoke.AppendedArgs[0] = "changed"
	plan.Stages.Output.Select[0] = "changed"

	rebuilt, err := Build(digest, bundle, currentIdentity(), attempt)
	if err != nil {
		t.Fatalf("Build() after plan mutation: %v", err)
	}
	gotDigest, err := rebuilt.Digest()
	if err != nil || gotDigest != wantDigest {
		t.Fatalf("rebuilt digest=%q want=%q error=%v", gotDigest, wantDigest, err)
	}
}
