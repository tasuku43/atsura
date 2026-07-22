package wrapperbinding

import (
	"encoding/json"
	"errors"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/tasuku43/atsura/internal/domain/sourcecatalog"
	"github.com/tasuku43/atsura/internal/domain/sourceprocess"
	"github.com/tasuku43/atsura/internal/domain/tailoringbundle"
)

func testAbsolutePath(t *testing.T, elements ...string) string {
	t.Helper()
	path, err := filepath.Abs(filepath.Join(append([]string{"testdata"}, elements...)...))
	if err != nil {
		t.Fatal(err)
	}
	return path
}

func testBundle(t *testing.T, requestedExecutable string) (tailoringbundle.Bundle, string) {
	t.Helper()
	catalog := sourcecatalog.Sort(sourcecatalog.Catalog{
		SchemaVersion: sourcecatalog.SchemaVersion,
		Adapter:       sourcecatalog.Adapter{Kind: "atsura.source.synthetic", ContractVersion: 1},
		Source: sourcecatalog.Source{
			RequestedExecutable: requestedExecutable,
			ResolvedPath:        testAbsolutePath(t, "source", "physical-source"),
			SHA256:              strings.Repeat("a", 64),
			Size:                42,
			Version:             "1.0.0",
		},
		Probe: sourcecatalog.Probe{IDs: []string{"command_help"}, Attempts: 1},
		Commands: []sourcecatalog.Command{{
			Path:             []string{"item", "list"},
			Summary:          "List synthetic items.",
			Provenance:       sourcecatalog.ProvenanceVerifiedBuiltin,
			Options:          []sourcecatalog.Option{},
			StructuredOutput: []sourcecatalog.StructuredOutput{},
		}},
	})
	catalogDigest, err := catalog.Digest()
	if err != nil {
		t.Fatal(err)
	}
	specification := tailoringbundle.SortSpecification(tailoringbundle.Specification{
		SchemaVersion: tailoringbundle.SpecificationSchemaVersion,
		CatalogDigest: catalogDigest,
		Surface:       tailoringbundle.Surface{Default: tailoringbundle.SurfaceDefaultExclude},
		Commands: []tailoringbundle.CommandEntry{{
			Command:  []string{"item", "list"},
			Presence: tailoringbundle.PresenceInclude,
			Reason:   "Needed by the wrapper fixture.",
			Options: &tailoringbundle.OptionSurface{
				Default: tailoringbundle.SurfaceDefaultInherit,
				Include: []string{},
				Exclude: []string{},
			},
			Wrapper: &tailoringbundle.Wrapper{
				Kind:   tailoringbundle.WrapperIdentity,
				Before: []tailoringbundle.StageAction{},
				Invoke: tailoringbundle.Invocation{AppendArgs: []string{}},
				After:  []tailoringbundle.StageAction{},
			},
		}},
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

func testRuntime(t *testing.T) sourceprocess.Identity {
	t.Helper()
	return sourceprocess.Identity{
		ResolvedPath: testAbsolutePath(t, "runtime", "atr"),
		SHA256:       strings.Repeat("b", 64),
		Size:         84,
	}
}

func TestNewDerivesExactCommandNameFromBundle(t *testing.T) {
	bundle, digest := testBundle(t, "gh")
	locator := testAbsolutePath(t, "bundle", "not-the-command-name.json")
	runtime := testRuntime(t)

	binding, err := New(locator, digest, bundle, runtime)
	if err != nil {
		t.Fatal(err)
	}
	if binding.CommandName != "gh" || binding.BundleLocator != locator || binding.BundleDigest != digest {
		t.Fatalf("binding = %+v", binding)
	}
	if binding.Runtime.SourceProcessIdentity() != runtime {
		t.Fatalf("runtime = %+v", binding.Runtime)
	}

	encoded, err := json.Marshal(binding)
	if err != nil {
		t.Fatal(err)
	}
	var top map[string]json.RawMessage
	if err := json.Unmarshal(encoded, &top); err != nil {
		t.Fatal(err)
	}
	wantKeys := []string{"bundle_digest", "bundle_locator", "command_name", "contract_version", "runtime"}
	gotKeys := make([]string, 0, len(top))
	for key := range top {
		gotKeys = append(gotKeys, key)
	}
	sort.Strings(gotKeys)
	if !reflect.DeepEqual(gotKeys, wantKeys) {
		t.Fatalf("binding keys = %v, want %v", gotKeys, wantKeys)
	}
	var runtimeKeys map[string]json.RawMessage
	if err := json.Unmarshal(top["runtime"], &runtimeKeys); err != nil {
		t.Fatal(err)
	}
	wantRuntimeKeys := []string{"resolved_path", "sha256", "size"}
	gotRuntimeKeys := make([]string, 0, len(runtimeKeys))
	for key := range runtimeKeys {
		gotRuntimeKeys = append(gotRuntimeKeys, key)
	}
	sort.Strings(gotRuntimeKeys)
	if !reflect.DeepEqual(gotRuntimeKeys, wantRuntimeKeys) {
		t.Fatalf("runtime keys = %v, want %v", gotRuntimeKeys, wantRuntimeKeys)
	}
}

func TestRuntimeInvocationOmitsCommandAndRetainsExactClosure(t *testing.T) {
	bundle, digest := testBundle(t, "gh")
	locator := testAbsolutePath(t, "bundle", "purpose.json")
	runtime := testRuntime(t)
	binding, err := New(locator, digest, bundle, runtime)
	if err != nil {
		t.Fatal(err)
	}

	invocation := binding.RuntimeInvocation()
	if err := invocation.Validate(); err != nil {
		t.Fatal(err)
	}
	if invocation.ContractVersion != ContractVersion || invocation.BundleLocator != locator || invocation.BundleDigest != digest || invocation.Runtime.SourceProcessIdentity() != runtime {
		t.Fatalf("runtime invocation = %+v", invocation)
	}
	encoded, err := json.Marshal(invocation)
	if err != nil {
		t.Fatal(err)
	}
	var document map[string]json.RawMessage
	if err := json.Unmarshal(encoded, &document); err != nil {
		t.Fatal(err)
	}
	wantKeys := []string{"bundle_digest", "bundle_locator", "contract_version", "runtime"}
	gotKeys := make([]string, 0, len(document))
	for key := range document {
		gotKeys = append(gotKeys, key)
	}
	sort.Strings(gotKeys)
	if !reflect.DeepEqual(gotKeys, wantKeys) {
		t.Fatalf("runtime invocation keys = %v, want %v; document=%s", gotKeys, wantKeys, encoded)
	}
	if strings.Contains(string(encoded), "command") || strings.Contains(string(encoded), "gh") {
		t.Fatalf("runtime invocation leaked command spelling: %s", encoded)
	}
}

func TestRuntimeInvocationValidationRejectsInvalidClosure(t *testing.T) {
	valid := RuntimeInvocation{
		ContractVersion: ContractVersion,
		BundleLocator:   testAbsolutePath(t, "bundle.json"),
		BundleDigest:    strings.Repeat("a", 64),
		Runtime:         RuntimeIdentity{ResolvedPath: testAbsolutePath(t, "atr"), SHA256: strings.Repeat("b", 64), Size: 42},
	}
	tests := []struct {
		name   string
		mutate func(*RuntimeInvocation)
	}{
		{name: "contract", mutate: func(value *RuntimeInvocation) { value.ContractVersion++ }},
		{name: "bundle locator", mutate: func(value *RuntimeInvocation) { value.BundleLocator = "bundle.json" }},
		{name: "bundle digest", mutate: func(value *RuntimeInvocation) { value.BundleDigest = strings.Repeat("A", 64) }},
		{name: "runtime path", mutate: func(value *RuntimeInvocation) { value.Runtime.ResolvedPath = "atr" }},
		{name: "runtime digest", mutate: func(value *RuntimeInvocation) { value.Runtime.SHA256 = "invalid" }},
		{name: "runtime size", mutate: func(value *RuntimeInvocation) { value.Runtime.Size = 0 }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			candidate := valid
			test.mutate(&candidate)
			if err := candidate.Validate(); !errors.Is(err, ErrInvalidBinding) {
				t.Fatalf("Validate() error = %v", err)
			}
		})
	}
}

func TestNewRejectsNonPortableBundleCommandInsteadOfTakingPathBase(t *testing.T) {
	bundle, digest := testBundle(t, "path/to/gh")
	_, err := New(testAbsolutePath(t, "bundle.json"), digest, bundle, testRuntime(t))
	if !errors.Is(err, ErrInvalidBinding) || !strings.Contains(err.Error(), "must match [A-Za-z_][A-Za-z0-9_]*") {
		t.Fatalf("New() error = %v", err)
	}
}

func TestBindingValidationRejectsDriftAndUnsafeStructure(t *testing.T) {
	bundle, digest := testBundle(t, "gh")
	valid, err := New(testAbsolutePath(t, "bundle.json"), digest, bundle, testRuntime(t))
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name   string
		mutate func(*Binding)
	}{
		{name: "contract version", mutate: func(value *Binding) { value.ContractVersion++ }},
		{name: "relative bundle", mutate: func(value *Binding) { value.BundleLocator = "bundle.json" }},
		{name: "unclean bundle", mutate: func(value *Binding) {
			value.BundleLocator += string(filepath.Separator) + ".." + string(filepath.Separator) + "bundle.json"
		}},
		{name: "bundle control", mutate: func(value *Binding) { value.BundleLocator += "\nunsafe" }},
		{name: "bundle format", mutate: func(value *Binding) { value.BundleLocator += "\u2066unsafe" }},
		{name: "bundle invalid utf8", mutate: func(value *Binding) { value.BundleLocator += string([]byte{0xff}) }},
		{name: "bundle unbounded", mutate: func(value *Binding) {
			value.BundleLocator = testAbsolutePath(t, strings.Repeat("x", MaxBundleLocatorBytes))
		}},
		{name: "bundle digest", mutate: func(value *Binding) { value.BundleDigest = strings.Repeat("A", 64) }},
		{name: "command syntax", mutate: func(value *Binding) { value.CommandName = "gh-tool" }},
		{name: "runtime relative", mutate: func(value *Binding) { value.Runtime.ResolvedPath = "atr" }},
		{name: "runtime control", mutate: func(value *Binding) { value.Runtime.ResolvedPath += "\tescape" }},
		{name: "runtime digest", mutate: func(value *Binding) { value.Runtime.SHA256 = "no" }},
		{name: "runtime size", mutate: func(value *Binding) { value.Runtime.Size = 0 }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			candidate := valid
			test.mutate(&candidate)
			if err := candidate.Validate(); !errors.Is(err, ErrInvalidBinding) {
				t.Fatalf("Validate() error = %v", err)
			}
		})
	}
}

func TestValidateAgainstBundleRejectsDigestAndCommandMismatch(t *testing.T) {
	bundle, digest := testBundle(t, "gh")
	binding, err := New(testAbsolutePath(t, "bundle.json"), digest, bundle, testRuntime(t))
	if err != nil {
		t.Fatal(err)
	}

	wrongDigest := binding
	wrongDigest.BundleDigest = strings.Repeat("c", 64)
	if err := wrongDigest.ValidateAgainstBundle(bundle); !errors.Is(err, ErrInvalidBinding) {
		t.Fatalf("digest mismatch error = %v", err)
	}
	wrongCommand := binding
	wrongCommand.CommandName = "git"
	if err := wrongCommand.ValidateAgainstBundle(bundle); !errors.Is(err, ErrInvalidBinding) {
		t.Fatalf("command mismatch error = %v", err)
	}
}

func TestValidateCommandNameRejectsReservedWordsAndSpecialBuiltins(t *testing.T) {
	valid := []string{"gh", "git_2", "_tailored"}
	for _, value := range valid {
		if err := ValidateCommandName(value); err != nil {
			t.Errorf("ValidateCommandName(%q) = %v", value, err)
		}
	}
	invalid := []string{"", "2gh", "gh-tool", "gh.tool", "\u30ae\u30c3\u30c8", "if", "done", "eval", "exec", "return", strings.Repeat("x", MaxCommandNameBytes+1)}
	for _, value := range invalid {
		if err := ValidateCommandName(value); err == nil {
			t.Errorf("ValidateCommandName(%q) succeeded", value)
		}
	}
}

func TestRenderedMaterialBindsDetachedBoundedSource(t *testing.T) {
	source := []byte("gh() {\n  '/opt/bin/atr' -- \"$@\"\n}\n")
	material, err := NewRenderedMaterial(source)
	if err != nil {
		t.Fatal(err)
	}
	source[0] = 'x'
	if string(material.Source[:2]) != "gh" {
		t.Fatalf("material source was not detached: %q", material.Source)
	}
	if err := material.Validate(); err != nil {
		t.Fatal(err)
	}
	clone := material.Clone()
	clone.Source[0] = 'x'
	if material.Source[0] != 'g' {
		t.Fatal("Clone shared the source buffer")
	}
}

func TestRenderedMaterialRejectsInvalidSourceAndDigest(t *testing.T) {
	valid, err := NewRenderedMaterial([]byte("gh() { :; }\n"))
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name     string
		material RenderedMaterial
	}{
		{name: "empty", material: RenderedMaterial{SHA256: strings.Repeat("a", 64)}},
		{name: "nul", material: RenderedMaterial{Source: []byte("x\x00y"), SHA256: strings.Repeat("a", 64)}},
		{name: "invalid utf8", material: RenderedMaterial{Source: []byte{0xff}, SHA256: strings.Repeat("a", 64)}},
		{name: "unbounded", material: RenderedMaterial{Source: []byte(strings.Repeat("x", MaxRenderedSourceBytes+1)), SHA256: strings.Repeat("a", 64)}},
		{name: "uppercase digest", material: RenderedMaterial{Source: valid.Source, SHA256: strings.Repeat("A", 64)}},
		{name: "digest drift", material: RenderedMaterial{Source: append(valid.Clone().Source, 'x'), SHA256: valid.SHA256}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if err := test.material.Validate(); !errors.Is(err, ErrInvalidRenderedSource) {
				t.Fatalf("Validate() error = %v", err)
			}
		})
	}
}
