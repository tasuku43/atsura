package catalogjson

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tasuku43/atsura/internal/domain/fault"
	"github.com/tasuku43/atsura/internal/domain/sourcecatalog"
)

func catalogFixture() sourcecatalog.Catalog {
	return sourcecatalog.Catalog{
		SchemaVersion: 1,
		Adapter:       sourcecatalog.Adapter{Kind: "atsura.source.synthetic", ContractVersion: 1},
		Source:        sourcecatalog.Source{RequestedExecutable: "fixture", ResolvedPath: "/opt/bin/fixture", SHA256: strings.Repeat("a", 64), Size: 42, Version: "1.0.0"},
		Probe:         sourcecatalog.Probe{IDs: []string{"help", "version"}, Attempts: 2},
		Commands:      []sourcecatalog.Command{{Path: []string{"item", "list"}, Summary: "List", Provenance: sourcecatalog.ProvenanceVerifiedBuiltin, Options: []sourcecatalog.Option{}, StructuredOutput: []sourcecatalog.StructuredOutput{}}},
	}
}

func writeCatalog(t *testing.T, value any) string {
	t.Helper()
	raw, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "catalog.json")
	if err := os.WriteFile(path, raw, 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestLoadSourceInspectionEnvelope(t *testing.T) {
	catalog := catalogFixture()
	digest, _ := catalog.Digest()
	path := writeCatalog(t, document{SchemaVersion: 1, Inspection: inspection{CatalogDigest: digest, Catalog: catalog, SourceProcessAttempts: 2}})
	loaded, err := New().Load(context.Background(), path)
	if err != nil || loaded.Source.Version != "1.0.0" {
		t.Fatalf("Load() = %+v, %v", loaded, err)
	}
}

func TestLoadRejectsDigestMismatchAndUnknownFields(t *testing.T) {
	catalog := catalogFixture()
	path := writeCatalog(t, document{SchemaVersion: 1, Inspection: inspection{CatalogDigest: strings.Repeat("b", 64), Catalog: catalog, SourceProcessAttempts: 2}})
	_, err := New().Load(context.Background(), path)
	public, ok := fault.PublicCopy(err)
	if !ok || public.Code != "catalog_digest_mismatch" {
		t.Fatalf("error = %v", err)
	}
	rawPath := filepath.Join(t.TempDir(), "unknown.json")
	if err := os.WriteFile(rawPath, []byte(`{"schema_version":1,"unknown":true,"inspection":{}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err = New().Load(context.Background(), rawPath)
	public, ok = fault.PublicCopy(err)
	if !ok || public.Code != "invalid_catalog_file" {
		t.Fatalf("error = %v", err)
	}
}
