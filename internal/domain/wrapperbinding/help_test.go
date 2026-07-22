package wrapperbinding

import (
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/tasuku43/atsura/internal/domain/sourcecatalog"
	"github.com/tasuku43/atsura/internal/domain/tailoringbundle"
)

func compiledHelpBundle(t *testing.T) (tailoringbundle.Bundle, string) {
	t.Helper()
	catalog := sourcecatalog.Sort(sourcecatalog.Catalog{
		SchemaVersion: sourcecatalog.SchemaVersion,
		Adapter:       sourcecatalog.Adapter{Kind: "atsura.source.synthetic", ContractVersion: 1},
		Source: sourcecatalog.Source{
			RequestedExecutable: "fixture",
			ResolvedPath:        testAbsolutePath(t, "source", "fixture"),
			SHA256:              strings.Repeat("a", 64),
			Size:                42,
			Version:             "1.0.0",
		},
		Probe: sourcecatalog.Probe{IDs: []string{"help"}, Attempts: 1},
		Commands: []sourcecatalog.Command{
			{Path: []string{"issue", "list"}, Summary: "List issues.", Provenance: sourcecatalog.ProvenanceVerifiedBuiltin, Options: []sourcecatalog.Option{{Name: "--state", TakesValue: true}}, StructuredOutput: []sourcecatalog.StructuredOutput{}},
			{Path: []string{"item"}, Summary: "Manage items.", Provenance: sourcecatalog.ProvenanceVerifiedBuiltin, Options: []sourcecatalog.Option{{Name: "--format", TakesValue: true}, {Name: "--verbose", TakesValue: false}}, StructuredOutput: []sourcecatalog.StructuredOutput{}},
			{Path: []string{"item", "delete"}, Summary: "HIDDEN_SUMMARY_CANARY", Provenance: sourcecatalog.ProvenanceVerifiedBuiltin, Options: []sourcecatalog.Option{{Name: "--force", TakesValue: false}}, StructuredOutput: []sourcecatalog.StructuredOutput{}},
			{Path: []string{"item", "list"}, Summary: "List $items; `literally` with 100% fidelity.", Provenance: sourcecatalog.ProvenanceVerifiedBuiltin, Options: []sourcecatalog.Option{{Name: "--json", TakesValue: true}, {Name: "--limit", TakesValue: true}, {Name: "--web", TakesValue: false}}, StructuredOutput: []sourcecatalog.StructuredOutput{}},
			{Path: []string{"item", "list", "recent"}, Summary: "List recent items.", Provenance: sourcecatalog.ProvenanceVerifiedBuiltin, Options: []sourcecatalog.Option{}, StructuredOutput: []sourcecatalog.StructuredOutput{}},
			{Path: []string{"plugin", "run"}, Summary: "HIDDEN_PLUGIN_CANARY", Provenance: sourcecatalog.ProvenanceUnverifiedDynamic, Options: []sourcecatalog.Option{}, StructuredOutput: []sourcecatalog.StructuredOutput{}},
		},
	})
	digest, err := catalog.Digest()
	if err != nil {
		t.Fatal(err)
	}
	identity := func() *tailoringbundle.Wrapper {
		return &tailoringbundle.Wrapper{
			Kind: tailoringbundle.WrapperIdentity, Before: []tailoringbundle.StageAction{},
			Invoke: tailoringbundle.Invocation{AppendArgs: []string{}}, After: []tailoringbundle.StageAction{},
		}
	}
	specification := tailoringbundle.SortSpecification(tailoringbundle.Specification{
		SchemaVersion: tailoringbundle.SpecificationSchemaVersion,
		CatalogDigest: digest,
		Surface:       tailoringbundle.Surface{Default: tailoringbundle.SurfaceDefaultExclude},
		Commands: []tailoringbundle.CommandEntry{
			{Command: []string{"issue", "list"}, Presence: tailoringbundle.PresenceInclude, Reason: "Needed for issue triage.", Options: &tailoringbundle.OptionSurface{Default: tailoringbundle.SurfaceDefaultInherit, Include: []string{}, Exclude: []string{}}, Wrapper: identity()},
			{Command: []string{"item"}, Presence: tailoringbundle.PresenceInclude, Reason: "Expose item navigation.", Options: &tailoringbundle.OptionSurface{Default: tailoringbundle.SurfaceDefaultInherit, Include: []string{}, Exclude: []string{"--verbose"}}, Wrapper: identity()},
			{Command: []string{"item", "delete"}, Presence: tailoringbundle.PresenceExclude, Reason: "HIDDEN_REASON_CANARY"},
			{Command: []string{"item", "list"}, Presence: tailoringbundle.PresenceInclude, Reason: "Use 'quoted' inventory; $(never execute).", Options: &tailoringbundle.OptionSurface{Default: tailoringbundle.SurfaceDefaultExclude, Include: []string{"--limit", "--web"}, Exclude: []string{}}, Wrapper: identity()},
			{Command: []string{"item", "list", "recent"}, Presence: tailoringbundle.PresenceInclude, Reason: "Expose the recent subset.", Options: &tailoringbundle.OptionSurface{Default: tailoringbundle.SurfaceDefaultExclude, Include: []string{}, Exclude: []string{}}, Wrapper: identity()},
		},
	})
	bundle, err := tailoringbundle.Compile(catalog, specification)
	if err != nil {
		t.Fatal(err)
	}
	bundleDigest, err := bundle.Digest()
	if err != nil {
		t.Fatal(err)
	}
	return bundle, bundleDigest
}

func TestCompileHelpDerivesRootNamespaceExactAndCombinedViews(t *testing.T) {
	bundle, _ := compiledHelpBundle(t)
	help, err := CompileHelp(bundle)
	if err != nil {
		t.Fatal(err)
	}
	if err := help.Validate(); err != nil {
		t.Fatal(err)
	}
	if len(help.Commands) != 4 {
		t.Fatalf("commands = %+v", help.Commands)
	}
	encoded, err := json.Marshal(help)
	if err != nil {
		t.Fatal(err)
	}
	for _, hidden := range []string{"item delete", "HIDDEN_SUMMARY_CANARY", "HIDDEN_REASON_CANARY", "HIDDEN_PLUGIN_CANARY", "--json", "--verbose"} {
		if strings.Contains(string(encoded), hidden) {
			t.Fatalf("compiled help leaked %q: %s", hidden, encoded)
		}
	}

	views, err := help.Views()
	if err != nil {
		t.Fatal(err)
	}
	wantSelectors := []string{"", "issue", "issue list", "item", "item list", "item list recent"}
	gotSelectors := make([]string, len(views))
	for index := range views {
		gotSelectors[index] = strings.Join(views[index].Selector, " ")
	}
	if !reflect.DeepEqual(gotSelectors, wantSelectors) {
		t.Fatalf("selectors = %v, want %v", gotSelectors, wantSelectors)
	}
	if views[0].Exact != nil || !reflect.DeepEqual(joinHelpPaths(views[0].Descendants), []string{"issue list", "item", "item list", "item list recent"}) {
		t.Fatalf("root view = %+v", views[0])
	}
	if views[1].Exact != nil || !reflect.DeepEqual(joinHelpPaths(views[1].Descendants), []string{"issue list"}) {
		t.Fatalf("pure namespace view = %+v", views[1])
	}
	item := views[3]
	if item.Exact == nil || strings.Join(item.Exact.Path, " ") != "item" || !reflect.DeepEqual(joinHelpPaths(item.Descendants), []string{"item list", "item list recent"}) ||
		!reflect.DeepEqual(item.Exact.Options, []HelpOption{{Name: "--format", TakesValue: true}}) {
		t.Fatalf("combined item view = %+v", item)
	}
	itemList := views[4]
	if itemList.Exact == nil || !reflect.DeepEqual(itemList.Exact.Options, []HelpOption{{Name: "--limit", TakesValue: true}, {Name: "--web", TakesValue: false}}) ||
		!reflect.DeepEqual(joinHelpPaths(itemList.Descendants), []string{"item list recent"}) {
		t.Fatalf("combined list view = %+v", itemList)
	}
	leaf := views[5]
	if leaf.Exact == nil || leaf.Descendants == nil || len(leaf.Descendants) != 0 || leaf.Exact.Options == nil || len(leaf.Exact.Options) != 0 {
		t.Fatalf("leaf view = %+v", leaf)
	}
}

func TestCompiledHelpCloneViewsAndBindingAreDeeplyDetached(t *testing.T) {
	bundle, digest := compiledHelpBundle(t)
	help, err := CompileHelp(bundle)
	if err != nil {
		t.Fatal(err)
	}
	clone := help.Clone()
	if !help.Equal(clone) {
		t.Fatal("Clone changed semantic help")
	}
	clone.Commands[0].Path[0] = "changed"
	clone.Commands[0].Options[0].Name = "--changed"
	if help.Commands[0].Path[0] != "issue" || help.Commands[0].Options[0].Name != "--state" {
		t.Fatal("Clone shared nested help state")
	}

	views, err := help.Views()
	if err != nil {
		t.Fatal(err)
	}
	views[0].Selector = append(views[0].Selector, "changed")
	views[0].Descendants[0][0] = "changed"
	views[3].Exact.Path[0] = "changed"
	again, err := help.Views()
	if err != nil {
		t.Fatal(err)
	}
	if len(again[0].Selector) != 0 || again[0].Descendants[0][0] != "issue" || again[3].Exact.Path[0] != "item" {
		t.Fatal("Views returned aliased help state")
	}

	binding, err := New(testAbsolutePath(t, "purpose.json"), digest, bundle, testRuntime(t))
	if err != nil {
		t.Fatal(err)
	}
	bindingClone := binding.Clone()
	if !binding.Equal(bindingClone) {
		t.Fatal("Binding.Clone changed semantic binding")
	}
	bindingClone.Help.Commands[0].Path[0] = "changed"
	if binding.Help.Commands[0].Path[0] != "issue" || binding.Equal(bindingClone) {
		t.Fatal("Binding.Clone shared or ignored help state")
	}
	if !(CompiledHelp{}).Equal((CompiledHelp{}).Clone()) {
		t.Fatal("Clone changed a nil command list into a different representation")
	}
	invalid := CompiledHelp{Commands: []HelpCommand{{Path: []string{"item"}}}}
	if !invalid.Equal(invalid.Clone()) || invalid.Clone().Commands[0].Options != nil {
		t.Fatal("Clone changed a nil option list into a different representation")
	}
}

func TestBindingValidationRecomputesCompiledHelpFromBundle(t *testing.T) {
	bundle, digest := compiledHelpBundle(t)
	binding, err := New(testAbsolutePath(t, "purpose.json"), digest, bundle, testRuntime(t))
	if err != nil {
		t.Fatal(err)
	}
	if binding.ContractVersion != ContractVersion {
		t.Fatalf("contract version = %d, want %d", binding.ContractVersion, ContractVersion)
	}
	if err := binding.ValidateAgainstBundle(bundle); err != nil {
		t.Fatalf("binding = %+v, error = %v", binding, err)
	}

	drifted := binding.Clone()
	drifted.Help.Commands[0].Summary = "Changed but structurally valid."
	if err := drifted.ValidateAgainstBundle(bundle); !errors.Is(err, ErrInvalidBinding) || !strings.Contains(err.Error(), "does not match") {
		t.Fatalf("help drift error = %v", err)
	}
	unsafe := binding.Clone()
	unsafe.Help.Commands[0].Reason = "unsafe\nline"
	if err := unsafe.Validate(); !errors.Is(err, ErrInvalidBinding) || !strings.Contains(err.Error(), "compiled help") {
		t.Fatalf("unsafe help error = %v", err)
	}
}

func TestCompiledHelpRejectsStructuralAndCanonicalDrift(t *testing.T) {
	bundle, _ := compiledHelpBundle(t)
	valid, err := CompileHelp(bundle)
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name   string
		mutate func(*CompiledHelp)
	}{
		{name: "nil commands", mutate: func(value *CompiledHelp) { value.Commands = nil }},
		{name: "empty commands", mutate: func(value *CompiledHelp) { value.Commands = []HelpCommand{} }},
		{name: "nil path", mutate: func(value *CompiledHelp) { value.Commands[0].Path = nil }},
		{name: "invalid path", mutate: func(value *CompiledHelp) { value.Commands[0].Path[0] = "Issue" }},
		{name: "structural summary", mutate: func(value *CompiledHelp) { value.Commands[0].Summary = "line\nfeed" }},
		{name: "format reason", mutate: func(value *CompiledHelp) { value.Commands[0].Reason = "bidi\u202e" }},
		{name: "nil options", mutate: func(value *CompiledHelp) { value.Commands[0].Options = nil }},
		{name: "invalid option", mutate: func(value *CompiledHelp) { value.Commands[0].Options[0].Name = "-state" }},
		{name: "unsorted commands", mutate: func(value *CompiledHelp) { value.Commands[0], value.Commands[1] = value.Commands[1], value.Commands[0] }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			candidate := valid.Clone()
			test.mutate(&candidate)
			if err := candidate.Validate(); !errors.Is(err, ErrInvalidCompiledHelp) {
				t.Fatalf("Validate() error = %v", err)
			}
		})
	}
}

func TestCompiledHelpEnforcesViewLineAndLiteralBudgets(t *testing.T) {
	t.Run("views", func(t *testing.T) {
		commands := make([]HelpCommand, MaxCompiledHelpViews)
		for index := range commands {
			commands[index] = boundedHelpCommand([]string{fmt.Sprintf("c%03d", index)}, []HelpOption{})
		}
		value := CompiledHelp{Commands: commands}
		if err := value.Validate(); !errors.Is(err, ErrInvalidCompiledHelp) || !strings.Contains(err.Error(), "view") {
			t.Fatalf("view bound error = %v", err)
		}
	})

	t.Run("semantic lines", func(t *testing.T) {
		options := make([]HelpOption, sourcecatalog.MaxOptions)
		for index := range options {
			options[index] = HelpOption{Name: fmt.Sprintf("--o%03d", index)}
		}
		commands := make([]HelpCommand, 16)
		for index := range commands {
			commands[index] = boundedHelpCommand([]string{"area", fmt.Sprintf("c%02d", index)}, options)
		}
		value := CompiledHelp{Commands: commands}
		if err := value.Validate(); !errors.Is(err, ErrInvalidCompiledHelp) || !strings.Contains(err.Error(), "line") {
			t.Fatalf("line bound error = %v", err)
		}
	})

	t.Run("literal bytes", func(t *testing.T) {
		options := make([]HelpOption, sourcecatalog.MaxOptions)
		for index := range options {
			options[index] = HelpOption{Name: "--" + fmt.Sprintf("o%03d", index) + strings.Repeat("x", 123), TakesValue: true}
		}
		commands := []HelpCommand{
			boundedHelpCommand([]string{"area", "alpha"}, options),
			boundedHelpCommand([]string{"area", "beta"}, options),
		}
		for index := range commands {
			commands[index].Summary = strings.Repeat("s", sourcecatalog.MaxTextBytes)
			commands[index].Reason = strings.Repeat("r", sourcecatalog.MaxTextBytes)
		}
		value := CompiledHelp{Commands: commands}
		if err := value.Validate(); !errors.Is(err, ErrInvalidCompiledHelp) || !strings.Contains(err.Error(), "literal payload") {
			t.Fatalf("literal bound error = %v", err)
		}
	})
}

func boundedHelpCommand(path []string, options []HelpOption) HelpCommand {
	return HelpCommand{Path: append([]string{}, path...), Summary: "Summary.", Reason: "Reason.", Options: append([]HelpOption{}, options...)}
}

func joinHelpPaths(paths [][]string) []string {
	result := make([]string, len(paths))
	for index, path := range paths {
		result[index] = strings.Join(path, " ")
	}
	return result
}
