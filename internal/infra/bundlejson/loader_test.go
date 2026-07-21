package bundlejson

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/tasuku43/atsura/internal/domain/fault"
	"github.com/tasuku43/atsura/internal/domain/sourcecatalog"
	"github.com/tasuku43/atsura/internal/domain/tailoringbundle"
	"github.com/tasuku43/atsura/internal/infra/localfile"
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

func assertExactBundleFault(t *testing.T, err error, kind fault.Kind, code string, retryable bool) {
	t.Helper()
	public, ok := fault.PublicCopy(err)
	if !ok || public.Kind != kind || public.Code != code || public.Retryable != retryable {
		t.Fatalf("public fault=%+v error=%v", public, err)
	}
	if len(public.NextActions) != 1 || public.NextActions[0].Command != "bundle build" || public.NextActions[0].Reason != "Build and select a valid canonical bundle document." {
		t.Fatalf("recovery=%+v", public.NextActions)
	}
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
	_, _, err = New().Load(context.Background(), path)
	assertExactBundleFault(t, err, fault.KindRejected, "bundle_digest_mismatch", false)
}

func TestLoaderRejectsLegacyBundleSchemaWithMigrationDiagnostic(t *testing.T) {
	path := filepath.Join(t.TempDir(), "legacy.json")
	data := []byte(`{"schema_version":1,"build":{}}`)
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	_, _, err := New().Load(context.Background(), path)
	assertExactBundleFault(t, err, fault.KindInvalidInput, "legacy_tailoring_schema", false)
}

func TestLoaderRejectsInvalidBundleDocumentWithExactFault(t *testing.T) {
	path := filepath.Join(t.TempDir(), "invalid.json")
	if err := os.WriteFile(path, []byte(`{"schema_version":2,"build":`), 0o600); err != nil {
		t.Fatal(err)
	}
	_, _, err := New().Load(context.Background(), path)
	assertExactBundleFault(t, err, fault.KindInvalidInput, "invalid_bundle_file", false)
}

func TestFileFaultMapsEveryLocalFileFailureToExactPublicContract(t *testing.T) {
	tests := []struct {
		name      string
		cause     error
		kind      fault.Kind
		code      string
		message   string
		retryable bool
	}{
		{
			name:    "not found",
			cause:   localfile.ErrNotFound,
			kind:    fault.KindNotFound,
			code:    "bundle_file_not_found",
			message: "The bundle build JSON was not found.",
		},
		{
			name:    "permission",
			cause:   localfile.ErrPermission,
			kind:    fault.KindPermission,
			code:    "bundle_file_permission_denied",
			message: "The bundle build JSON cannot be read.",
		},
		{
			name:    "unsafe",
			cause:   localfile.ErrUnsafe,
			kind:    fault.KindInvalidInput,
			code:    "unsafe_bundle_file",
			message: "The bundle build JSON must be a stable regular file, not a symbolic link.",
		},
		{
			name:    "too large",
			cause:   localfile.ErrTooLarge,
			kind:    fault.KindInvalidInput,
			code:    "bundle_file_too_large",
			message: "The bundle build JSON exceeds 2 MiB.",
		},
		{
			name:      "read failure",
			cause:     errors.New("private synthetic read cause"),
			kind:      fault.KindUnavailable,
			code:      "bundle_file_read_failed",
			message:   "The bundle build JSON could not be read.",
			retryable: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			public, ok := fault.PublicCopy(fileFault(test.cause))
			if !ok {
				t.Fatal("file fault is not a valid public fault")
			}
			if public.Kind != test.kind || public.Code != test.code || public.Message != test.message || public.Retryable != test.retryable {
				t.Fatalf("public fault=%+v", public)
			}
			if len(public.NextActions) != 1 || public.NextActions[0].Command != "bundle build" || public.NextActions[0].Reason != "Build and select a valid canonical bundle document." {
				t.Fatalf("recovery=%+v", public.NextActions)
			}
		})
	}
}
