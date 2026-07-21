package bundlejson

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/tasuku43/atsura/internal/domain/fault"
	"github.com/tasuku43/atsura/internal/domain/sourcecatalog"
	"github.com/tasuku43/atsura/internal/domain/tailoringbundle"
)

func testBundle(t *testing.T) (tailoringbundle.Bundle, string) {
	t.Helper()
	catalog := sourcecatalog.Catalog{SchemaVersion: 1,
		Adapter:  sourcecatalog.Adapter{Kind: "example.test.source", ContractVersion: 1},
		Source:   sourcecatalog.Source{RequestedExecutable: "tool", ResolvedPath: "/tool", SHA256: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", Size: 1, Version: "1.0"},
		Probe:    sourcecatalog.Probe{IDs: []string{"help"}, Attempts: 1},
		Commands: []sourcecatalog.Command{{Path: []string{"item", "list"}, Summary: "List items", Provenance: sourcecatalog.ProvenanceVerifiedBuiltin, Options: []sourcecatalog.Option{}, StructuredOutput: []sourcecatalog.StructuredOutput{{Format: "json", SelectorFlag: "--json", Fields: []string{"id"}}}}},
	}
	catalogDigest, _ := catalog.Digest()
	specification := tailoringbundle.Specification{SchemaVersion: tailoringbundle.SpecificationSchemaVersion, CatalogDigest: catalogDigest, Surface: tailoringbundle.Surface{Default: tailoringbundle.SurfaceDefaultExclude}, Commands: []tailoringbundle.CommandEntry{{Command: []string{"item", "list"}, Presence: tailoringbundle.PresenceInclude, Reason: "needed", Options: &tailoringbundle.OptionSurface{Default: tailoringbundle.SurfaceDefaultInherit, Include: []string{}, Exclude: []string{}}, Wrapper: &tailoringbundle.Wrapper{Kind: tailoringbundle.WrapperTransform, Before: []tailoringbundle.StageAction{}, Invoke: tailoringbundle.Invocation{AppendArgs: []string{"--json=id"}}, Output: &tailoringbundle.Output{Input: "json", Select: []string{"id"}, Rename: []tailoringbundle.Rename{}, Render: "compact_json"}, After: []tailoringbundle.StageAction{}}}}}
	bundle, err := tailoringbundle.Compile(catalog, specification)
	if err != nil {
		t.Fatal(err)
	}
	digest, _ := bundle.Digest()
	return bundle, digest
}

func TestLoaderAcceptsExactBuildEnvelopeAndRejectsDigestDrift(t *testing.T) {
	bundle, digest := testBundle(t)
	dir := t.TempDir()
	path := filepath.Join(dir, "bundle.json")
	document := map[string]any{"schema_version": 2, "build": map[string]any{"bundle_digest": digest, "bundle": bundle}}
	data, _ := json.Marshal(document)
	if err := os.WriteFile(path, append(data, '\n'), 0o600); err != nil {
		t.Fatal(err)
	}
	got, gotDigest, err := New().Load(context.Background(), path)
	if err != nil || gotDigest != digest || got.CatalogDigest != bundle.CatalogDigest {
		t.Fatalf("Load() = %s, %+v, %v", gotDigest, got, err)
	}
	document["build"].(map[string]any)["bundle_digest"] = "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
	data, _ = json.Marshal(document)
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, _, err := New().Load(context.Background(), path); err == nil {
		t.Fatal("digest drift succeeded")
	}
}

func TestLoaderRejectsLegacyBundleSchemaWithMigrationDiagnostic(t *testing.T) {
	path := filepath.Join(t.TempDir(), "legacy.json")
	data := []byte(`{"schema_version":1,"build":{}}`)
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	_, _, err := New().Load(context.Background(), path)
	public, ok := fault.PublicCopy(err)
	if !ok || public.Code != "legacy_tailoring_schema" {
		t.Fatalf("error = %#v", err)
	}
}
