package tailoringbundle

import (
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/tasuku43/atsura/internal/domain/sourcecatalog"
)

func bundleCatalog() sourcecatalog.Catalog {
	return sourcecatalog.Catalog{
		SchemaVersion: sourcecatalog.SchemaVersion,
		Adapter:       sourcecatalog.Adapter{Kind: "atsura.source.alternate", ContractVersion: 1},
		Source:        sourcecatalog.Source{RequestedExecutable: "fixture", ResolvedPath: "/opt/bin/fixture", SHA256: strings.Repeat("a", 64), Size: 42, Version: "1.0.0"},
		Probe:         sourcecatalog.Probe{IDs: []string{"help", "version"}, Attempts: 2},
		Commands: []sourcecatalog.Command{
			{Path: []string{"item", "delete"}, Summary: "Delete item", Provenance: sourcecatalog.ProvenanceVerifiedBuiltin, Options: []sourcecatalog.Option{{Name: "--json", TakesValue: true}}, StructuredOutput: []sourcecatalog.StructuredOutput{{Format: "json", SelectorFlag: "--json", Fields: []string{"id"}}}},
			{Path: []string{"item", "list"}, Summary: "List items", Provenance: sourcecatalog.ProvenanceVerifiedBuiltin, Options: []sourcecatalog.Option{{Name: "--format", TakesValue: true}, {Name: "--json", TakesValue: true}, {Name: "--limit", TakesValue: true}, {Name: "--verbose", TakesValue: false}}, StructuredOutput: []sourcecatalog.StructuredOutput{{Format: "json", SelectorFlag: "--json", Fields: []string{"id", "name"}}}},
			{Path: []string{"plugin", "run"}, Summary: "Dynamic plugin", Provenance: sourcecatalog.ProvenanceUnverifiedDynamic, Options: []sourcecatalog.Option{}, StructuredOutput: []sourcecatalog.StructuredOutput{}},
		},
	}
}

func identityWrapper() *Wrapper {
	return &Wrapper{Kind: WrapperIdentity, Before: []StageAction{}, Invoke: Invocation{OptionDefaults: []OptionDefault{}, AppendArgs: []string{}}, After: []StageAction{}}
}

func transformingWrapper() *Wrapper {
	return &Wrapper{
		Kind: WrapperTransform, Before: []StageAction{}, Invoke: Invocation{OptionDefaults: []OptionDefault{}, AppendArgs: []string{"--json=id,name"}},
		Output: &Output{Kind: OutputKindProjection, Projection: &Projection{Input: "json", Select: []string{"id", "name"}, Rename: []Rename{}, Render: "compact_json"}}, After: []StageAction{},
	}
}

func defaultingWrapper(defaults ...OptionDefault) *Wrapper {
	return &Wrapper{
		Kind: WrapperTransform, Before: []StageAction{},
		Invoke: Invocation{OptionDefaults: append([]OptionDefault{}, defaults...), AppendArgs: []string{}},
		After:  []StageAction{},
	}
}

func inheritedOptions() *OptionSurface {
	return &OptionSurface{Default: SurfaceDefaultInherit, Include: []string{}, Exclude: []string{}}
}

func specification(t *testing.T, defaultMembership SurfaceDefault, entries ...CommandEntry) Specification {
	t.Helper()
	digest, err := bundleCatalog().Digest()
	if err != nil {
		t.Fatal(err)
	}
	return SortSpecification(Specification{SchemaVersion: SpecificationSchemaVersion, CatalogDigest: digest, Surface: Surface{Default: defaultMembership}, Commands: entries})
}

func TestCompileProducesOneDeterministicVendorNeutralBundle(t *testing.T) {
	spec := specification(t, SurfaceDefaultExclude,
		CommandEntry{Command: []string{"item", "delete"}, Presence: PresenceExclude, Reason: "Deletion is outside this purpose."},
		CommandEntry{Command: []string{"item", "list"}, Presence: PresenceInclude, Reason: "Expose a compact inventory.", Options: inheritedOptions(), Wrapper: transformingWrapper()},
	)
	first, err := Compile(bundleCatalog(), spec)
	if err != nil {
		t.Fatal(err)
	}
	second, err := Compile(bundleCatalog(), spec)
	if err != nil {
		t.Fatal(err)
	}
	firstBytes, _ := first.CanonicalJSON()
	secondBytes, _ := second.CanonicalJSON()
	firstDigest, _ := first.Digest()
	secondDigest, _ := second.Digest()
	if string(firstBytes) != string(secondBytes) || firstDigest != secondDigest || len(firstDigest) != 64 {
		t.Fatalf("bundle identity mismatch: %q %q", firstDigest, secondDigest)
	}
	if first.SchemaVersion != BundleSchemaVersion || first.Specification.SchemaVersion != SpecificationSchemaVersion || len(first.Surface) != 1 || strings.Join(first.Surface[0].Command, " ") != "item list" {
		t.Fatalf("bundle = %+v", first)
	}
	encoded := string(firstBytes)
	for _, retired := range []string{"policy", "decision", "effect", "impact", "target", "confirm", "deny"} {
		if strings.Contains(encoded, `"`+retired+`"`) {
			t.Fatalf("bundle retained authorization field %q: %s", retired, encoded)
		}
	}
}

func TestSurfaceMembershipTruthTable(t *testing.T) {
	tests := []struct {
		name              string
		defaultMembership SurfaceDefault
		entries           []CommandEntry
		want              []string
	}{
		{name: "exclude default omits unlisted", defaultMembership: SurfaceDefaultExclude, entries: []CommandEntry{}, want: []string{}},
		{name: "exclude default explicit include", defaultMembership: SurfaceDefaultExclude, entries: []CommandEntry{{Command: []string{"item", "list"}, Presence: PresenceInclude, Reason: "Needed.", Options: inheritedOptions(), Wrapper: identityWrapper()}}, want: []string{"item list"}},
		{name: "inherit default includes verified only", defaultMembership: SurfaceDefaultInherit, entries: []CommandEntry{}, want: []string{"item delete", "item list"}},
		{name: "inherit default explicit exclude", defaultMembership: SurfaceDefaultInherit, entries: []CommandEntry{{Command: []string{"item", "delete"}, Presence: PresenceExclude, Reason: "Not part of this purpose."}}, want: []string{"item list"}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			bundle, err := Compile(bundleCatalog(), specification(t, test.defaultMembership, test.entries...))
			if err != nil {
				t.Fatal(err)
			}
			got := make([]string, len(bundle.Surface))
			for index, entry := range bundle.Surface {
				got[index] = strings.Join(entry.Command, " ")
			}
			if !reflect.DeepEqual(got, test.want) {
				t.Fatalf("surface = %v, want %v", got, test.want)
			}
		})
	}
}

func TestResolveReturnsDetachedEntryAndSurfaceAbsence(t *testing.T) {
	bundle, err := Compile(bundleCatalog(), specification(t, SurfaceDefaultExclude, CommandEntry{
		Command: []string{"item", "list"}, Presence: PresenceInclude, Reason: "Needed.", Options: inheritedOptions(), Wrapper: identityWrapper(),
	}))
	if err != nil {
		t.Fatal(err)
	}
	entry, err := bundle.Resolve([]string{"item", "list"})
	if err != nil || entry.Wrapper.Kind != WrapperIdentity {
		t.Fatalf("entry = %+v, error = %v", entry, err)
	}
	entry.Command[0] = "changed"
	if bundle.Surface[0].Command[0] != "item" {
		t.Fatal("Resolve returned aliased surface data")
	}
	if _, err := bundle.Resolve([]string{"item", "delete"}); !errors.Is(err, ErrCommandNotInSurface) {
		t.Fatalf("Resolve() error = %v", err)
	}
}

func TestMembershipOptionsAndWrapperAreIndependent(t *testing.T) {
	base := CommandEntry{Command: []string{"item", "list"}, Presence: PresenceInclude, Reason: "Needed.", Options: inheritedOptions(), Wrapper: identityWrapper()}
	variants := []CommandEntry{base, base, base}
	variants[1].Options = &OptionSurface{Default: SurfaceDefaultExclude, Include: []string{"--json"}, Exclude: []string{}}
	variants[2].Wrapper = transformingWrapper()
	for index, entry := range variants {
		bundle, err := Compile(bundleCatalog(), specification(t, SurfaceDefaultExclude, entry))
		if err != nil {
			t.Fatalf("variant %d: %v", index, err)
		}
		if len(bundle.Surface) != 1 {
			t.Fatalf("variant %d surface = %+v", index, bundle.Surface)
		}
	}
	if variants[0].Options.Default == variants[1].Options.Default || variants[0].Wrapper.Kind == variants[2].Wrapper.Kind {
		t.Fatal("test variants did not independently change options and wrapper")
	}
}

func TestOptionSurfaceIncludedOptionsProjectsExactCatalogOrderAndArity(t *testing.T) {
	command := sourcecatalog.Command{Options: []sourcecatalog.Option{
		{Name: "--format", TakesValue: true},
		{Name: "--json", TakesValue: true},
		{Name: "--verbose", TakesValue: false},
	}}
	tests := []struct {
		name    string
		surface OptionSurface
		want    []sourcecatalog.Option
	}{
		{
			name:    "inherit all",
			surface: OptionSurface{Default: SurfaceDefaultInherit, Include: []string{}, Exclude: []string{}},
			want:    command.Options,
		},
		{
			name:    "inherit except exact exclusion",
			surface: OptionSurface{Default: SurfaceDefaultInherit, Include: []string{}, Exclude: []string{"--json"}},
			want:    []sourcecatalog.Option{{Name: "--format", TakesValue: true}, {Name: "--verbose", TakesValue: false}},
		},
		{
			name:    "exclude except exact inclusions",
			surface: OptionSurface{Default: SurfaceDefaultExclude, Include: []string{"--json", "--verbose"}, Exclude: []string{}},
			want:    []sourcecatalog.Option{{Name: "--json", TakesValue: true}, {Name: "--verbose", TakesValue: false}},
		},
		{
			name:    "exclude all",
			surface: OptionSurface{Default: SurfaceDefaultExclude, Include: []string{}, Exclude: []string{}},
			want:    []sourcecatalog.Option{},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := test.surface.IncludedOptions(command)
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(got, test.want) {
				t.Fatalf("IncludedOptions() = %+v, want %+v", got, test.want)
			}
			if got == nil {
				t.Fatal("IncludedOptions returned a nil explicit set")
			}
			if len(got) > 0 {
				got[0].Name = "--changed"
				if command.Options[0].Name != "--format" {
					t.Fatal("IncludedOptions aliased catalog options")
				}
			}
		})
	}
}

func TestOptionSurfaceIncludedOptionsRejectsInvalidSurfaceAndCommandEvidence(t *testing.T) {
	validCommand := sourcecatalog.Command{Options: []sourcecatalog.Option{{Name: "--json", TakesValue: true}}}
	validSurface := OptionSurface{Default: SurfaceDefaultInherit, Include: []string{}, Exclude: []string{}}
	tests := []struct {
		name    string
		command sourcecatalog.Command
		surface OptionSurface
	}{
		{name: "nil command options", command: sourcecatalog.Command{}, surface: validSurface},
		{name: "invalid command option", command: sourcecatalog.Command{Options: []sourcecatalog.Option{{Name: "-json", TakesValue: true}}}, surface: validSurface},
		{name: "unsorted command options", command: sourcecatalog.Command{Options: []sourcecatalog.Option{{Name: "--verbose"}, {Name: "--json", TakesValue: true}}}, surface: validSurface},
		{name: "invalid surface default", command: validCommand, surface: OptionSurface{Default: "future", Include: []string{}, Exclude: []string{}}},
		{name: "unknown included option", command: validCommand, surface: OptionSurface{Default: SurfaceDefaultExclude, Include: []string{"--unknown"}, Exclude: []string{}}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got, err := test.surface.IncludedOptions(test.command); got != nil || err == nil {
				t.Fatalf("IncludedOptions() = %+v, %v", got, err)
			}
		})
	}
}

func TestSpecificationRejectsInvalidMembershipOptionsAndWrappers(t *testing.T) {
	valid := CommandEntry{Command: []string{"item", "list"}, Presence: PresenceInclude, Reason: "Needed.", Options: inheritedOptions(), Wrapper: identityWrapper()}
	tests := []struct {
		name   string
		mutate func(*Specification)
	}{
		{name: "invalid default", mutate: func(s *Specification) { s.Surface.Default = "future" }},
		{name: "missing commands list", mutate: func(s *Specification) { s.Commands = nil }},
		{name: "duplicate command", mutate: func(s *Specification) {
			s.Commands = append(s.Commands, s.Commands[0])
		}},
		{name: "unverified command", mutate: func(s *Specification) { s.Commands[0].Command = []string{"plugin", "run"} }},
		{name: "excluded with wrapper", mutate: func(s *Specification) { s.Commands[0].Presence = PresenceExclude; s.Commands[0].Options = nil }},
		{name: "included without options", mutate: func(s *Specification) { s.Commands[0].Options = nil }},
		{name: "included without wrapper", mutate: func(s *Specification) { s.Commands[0].Wrapper = nil }},
		{name: "inherit with include override", mutate: func(s *Specification) { s.Commands[0].Options.Include = []string{"--json"} }},
		{name: "exclude with exclude override", mutate: func(s *Specification) {
			s.Commands[0].Options = &OptionSurface{Default: SurfaceDefaultExclude, Include: []string{}, Exclude: []string{"--json"}}
		}},
		{name: "unknown option", mutate: func(s *Specification) { s.Commands[0].Options.Exclude = []string{"--unknown"} }},
		{name: "unsorted options", mutate: func(s *Specification) { s.Commands[0].Options.Exclude = []string{"--json", "--format"} }},
		{name: "identity transforms argv", mutate: func(s *Specification) { s.Commands[0].Wrapper.Invoke.AppendArgs = []string{"--json"} }},
		{name: "identity inserts option default", mutate: func(s *Specification) {
			s.Commands[0].Wrapper.Invoke.OptionDefaults = []OptionDefault{{Option: "--limit", Value: "30"}}
		}},
		{name: "identity transforms output", mutate: func(s *Specification) { s.Commands[0].Wrapper.Output = transformingWrapper().Output }},
		{name: "empty transform", mutate: func(s *Specification) { s.Commands[0].Wrapper.Kind = WrapperTransform }},
		{name: "before action", mutate: func(s *Specification) { s.Commands[0].Wrapper.Before = []StageAction{{Kind: "future"}} }},
		{name: "missing explicit after", mutate: func(s *Specification) { s.Commands[0].Wrapper.After = nil }},
		{name: "missing explicit option defaults", mutate: func(s *Specification) { s.Commands[0].Wrapper.Invoke.OptionDefaults = nil }},
		{name: "missing explicit append args", mutate: func(s *Specification) { s.Commands[0].Wrapper.Invoke.AppendArgs = nil }},
		{name: "unobserved output field", mutate: func(s *Specification) {
			s.Commands[0].Wrapper = transformingWrapper()
			s.Commands[0].Wrapper.Output.Projection.Select = []string{"unknown"}
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			spec := specification(t, SurfaceDefaultExclude, valid)
			test.mutate(&spec)
			if err := spec.Validate(bundleCatalog()); !errors.Is(err, ErrInvalidSpecification) {
				t.Fatalf("Validate() error = %v", err)
			}
		})
	}
}

func TestOptionDefaultsPreserveDeclarationOrderAndAffectCanonicalIdentity(t *testing.T) {
	defaults := []OptionDefault{{Option: "--limit", Value: "30"}, {Option: "--format", Value: "table"}}
	spec := specification(t, SurfaceDefaultExclude, CommandEntry{
		Command: []string{"item", "list"}, Presence: PresenceInclude, Reason: "Apply stable defaults.",
		Options: inheritedOptions(), Wrapper: defaultingWrapper(defaults...),
	})
	bundle, err := Compile(bundleCatalog(), spec)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(bundle.Specification.Commands[0].Wrapper.Invoke.OptionDefaults, defaults) ||
		!reflect.DeepEqual(bundle.Surface[0].Wrapper.Invoke.OptionDefaults, defaults) {
		t.Fatalf("option default order changed: %+v", bundle.Surface[0].Wrapper.Invoke.OptionDefaults)
	}
	raw, err := bundle.CanonicalJSON()
	if err != nil {
		t.Fatal(err)
	}
	wantJSON := `"option_defaults":[{"option":"--limit","value":"30"},{"option":"--format","value":"table"}]`
	if !strings.Contains(string(raw), wantJSON) {
		t.Fatalf("canonical bundle did not retain declaration order: %s", raw)
	}
	resolved, err := bundle.Resolve([]string{"item", "list"})
	if err != nil {
		t.Fatal(err)
	}
	resolved.Wrapper.Invoke.OptionDefaults[0].Value = "changed"
	if bundle.Surface[0].Wrapper.Invoke.OptionDefaults[0].Value != "30" {
		t.Fatal("Resolve returned aliased option defaults")
	}

	reordered := specification(t, SurfaceDefaultExclude, CommandEntry{
		Command: []string{"item", "list"}, Presence: PresenceInclude, Reason: "Apply stable defaults.",
		Options: inheritedOptions(), Wrapper: defaultingWrapper(defaults[1], defaults[0]),
	})
	reorderedBundle, err := Compile(bundleCatalog(), reordered)
	if err != nil {
		t.Fatal(err)
	}
	firstDigest, _ := bundle.Digest()
	secondDigest, _ := reorderedBundle.Digest()
	if firstDigest == secondDigest {
		t.Fatal("declaration-order change did not affect bundle identity")
	}
}

func TestOptionDefaultsValidateCatalogSurfaceValuesAndAppendOverlap(t *testing.T) {
	valid := CommandEntry{
		Command: []string{"item", "list"}, Presence: PresenceInclude, Reason: "Apply stable defaults.",
		Options: inheritedOptions(), Wrapper: defaultingWrapper(OptionDefault{Option: "--limit", Value: "30"}),
	}
	tests := []struct {
		name   string
		mutate func(*CommandEntry)
	}{
		{name: "unknown", mutate: func(entry *CommandEntry) { entry.Wrapper.Invoke.OptionDefaults[0].Option = "--unknown" }},
		{name: "excluded", mutate: func(entry *CommandEntry) { entry.Options.Exclude = []string{"--limit"} }},
		{name: "flag without value", mutate: func(entry *CommandEntry) { entry.Wrapper.Invoke.OptionDefaults[0].Option = "--verbose" }},
		{name: "structured output selector", mutate: func(entry *CommandEntry) { entry.Wrapper.Invoke.OptionDefaults[0].Option = "--json" }},
		{name: "empty value", mutate: func(entry *CommandEntry) { entry.Wrapper.Invoke.OptionDefaults[0].Value = "" }},
		{name: "oversize value", mutate: func(entry *CommandEntry) { entry.Wrapper.Invoke.OptionDefaults[0].Value = strings.Repeat("x", 4097) }},
		{name: "unsafe control value", mutate: func(entry *CommandEntry) { entry.Wrapper.Invoke.OptionDefaults[0].Value = "unsafe\nvalue" }},
		{name: "unsafe format value", mutate: func(entry *CommandEntry) { entry.Wrapper.Invoke.OptionDefaults[0].Value = "unsafe\u2060value" }},
		{name: "unsafe line separator value", mutate: func(entry *CommandEntry) { entry.Wrapper.Invoke.OptionDefaults[0].Value = "unsafe\u2028value" }},
		{name: "duplicate", mutate: func(entry *CommandEntry) {
			entry.Wrapper.Invoke.OptionDefaults = append(entry.Wrapper.Invoke.OptionDefaults, OptionDefault{Option: "--limit", Value: "40"})
		}},
		{name: "inline append overlap", mutate: func(entry *CommandEntry) { entry.Wrapper.Invoke.AppendArgs = []string{"--limit=40"} }},
		{name: "separated append overlap", mutate: func(entry *CommandEntry) { entry.Wrapper.Invoke.AppendArgs = []string{"--limit", "40"} }},
		{name: "combined bound", mutate: func(entry *CommandEntry) {
			entry.Wrapper.Invoke.AppendArgs = make([]string, MaxWrapperArguments)
			for index := range entry.Wrapper.Invoke.AppendArgs {
				entry.Wrapper.Invoke.AppendArgs[index] = "argument"
			}
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			entry := valid
			entry.Options = &OptionSurface{Default: valid.Options.Default, Include: []string{}, Exclude: []string{}}
			entry.Wrapper = defaultingWrapper(valid.Wrapper.Invoke.OptionDefaults...)
			test.mutate(&entry)
			if err := specification(t, SurfaceDefaultExclude, entry).Validate(bundleCatalog()); !errors.Is(err, ErrInvalidSpecification) {
				t.Fatalf("Validate() error = %v", err)
			}
		})
	}

	for _, appendArgs := range [][]string{
		{"--", "--limit=40"},
		{"--limitish=40"},
	} {
		entry := valid
		entry.Wrapper = defaultingWrapper(valid.Wrapper.Invoke.OptionDefaults...)
		entry.Wrapper.Invoke.AppendArgs = append([]string{}, appendArgs...)
		if err := specification(t, SurfaceDefaultExclude, entry).Validate(bundleCatalog()); err != nil {
			t.Fatalf("non-overlapping append args %q rejected: %v", appendArgs, err)
		}
	}

	boundary := valid
	boundary.Wrapper = defaultingWrapper(OptionDefault{Option: "--limit", Value: strings.Repeat("x", 4096)})
	boundary.Wrapper.Invoke.AppendArgs = make([]string, MaxWrapperArguments-1)
	for index := range boundary.Wrapper.Invoke.AppendArgs {
		boundary.Wrapper.Invoke.AppendArgs[index] = "argument"
	}
	if err := specification(t, SurfaceDefaultExclude, boundary).Validate(bundleCatalog()); err != nil {
		t.Fatalf("exact option-default and combined bounds rejected: %v", err)
	}
}

func TestBundleDetectsCatalogSpecificationAndSurfaceDrift(t *testing.T) {
	bundle, err := Compile(bundleCatalog(), specification(t, SurfaceDefaultExclude, CommandEntry{
		Command: []string{"item", "list"}, Presence: PresenceInclude, Reason: "Needed.", Options: inheritedOptions(), Wrapper: identityWrapper(),
	}))
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name   string
		mutate func(*Bundle)
	}{
		{name: "catalog", mutate: func(b *Bundle) { b.Catalog.Source.Version = "1.0.1" }},
		{name: "specification", mutate: func(b *Bundle) { b.Specification.Commands[0].Reason = "Changed." }},
		{name: "surface", mutate: func(b *Bundle) { b.Surface[0].Reason = "Changed." }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			copy := bundle
			copy.Catalog = sourcecatalog.Sort(bundle.Catalog)
			copy.Specification = SortSpecification(bundle.Specification)
			copy.Surface = append([]SurfaceEntry(nil), bundle.Surface...)
			test.mutate(&copy)
			if err := copy.Validate(); !errors.Is(err, ErrInvalidBundle) {
				t.Fatalf("Validate() error = %v", err)
			}
		})
	}
}

func TestSortSpecificationDetachesAndCanonicalizesSetOrder(t *testing.T) {
	spec := specification(t, SurfaceDefaultExclude,
		CommandEntry{Command: []string{"item", "list"}, Presence: PresenceInclude, Reason: "Needed.", Options: &OptionSurface{Default: SurfaceDefaultInherit, Include: []string{}, Exclude: []string{"--json", "--format"}}, Wrapper: identityWrapper()},
		CommandEntry{Command: []string{"item", "delete"}, Presence: PresenceExclude, Reason: "Not needed."},
	)
	if strings.Join(spec.Commands[0].Command, " ") != "item delete" || !reflect.DeepEqual(spec.Commands[1].Options.Exclude, []string{"--format", "--json"}) {
		t.Fatalf("sorted = %+v", spec.Commands)
	}
	spec.Commands[0].Command[0] = "changed"
	if strings.Join(spec.Commands[1].Command, " ") != "item list" {
		t.Fatal("unexpected sorted command order")
	}
}
