package bundlejson

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/tasuku43/atsura/internal/domain/operation"
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
	policy := tailoringbundle.Policy{SchemaVersion: 2, CatalogDigest: catalogDigest, Rules: []tailoringbundle.Rule{{Command: []string{"item", "list"}, Visibility: tailoringbundle.VisibilityVisible, Effect: operation.EffectRead, Decision: tailoringbundle.DecisionAllow, Reason: "needed", AppendArgs: []string{"--json", "id"}, Output: &tailoringbundle.Output{Input: "json", Select: []string{"id"}, Rename: []tailoringbundle.Rename{}, Render: "compact_json"}}}}
	bundle, err := tailoringbundle.Compile(catalog, policy)
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
	document := map[string]any{"schema_version": 1, "build": map[string]any{"bundle_digest": digest, "bundle": bundle}}
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
