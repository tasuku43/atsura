package sourceinspect

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/tasuku43/atsura/internal/domain/fault"
	"github.com/tasuku43/atsura/internal/domain/operation"
	"github.com/tasuku43/atsura/internal/domain/sourcecatalog"
)

type fakeInspector struct {
	catalog sourcecatalog.Catalog
	err     error
	calls   int
}

func (f *fakeInspector) Inspect(_ context.Context, _ string) (sourcecatalog.Catalog, error) {
	f.calls++
	return f.catalog, f.err
}

func fixtureCatalog() sourcecatalog.Catalog {
	return sourcecatalog.Catalog{
		SchemaVersion: 1,
		Adapter:       sourcecatalog.Adapter{Kind: "atsura.source.alternate", ContractVersion: 1},
		Source:        sourcecatalog.Source{RequestedExecutable: "fixture", ResolvedPath: "/bin/fixture", SHA256: strings.Repeat("a", 64), Size: 10, Version: "1.0.0"},
		Probe:         sourcecatalog.Probe{IDs: []string{"help", "version"}, Attempts: 2},
		Commands:      []sourcecatalog.Command{{Path: []string{"list"}, Summary: "List", Provenance: sourcecatalog.ProvenanceVerifiedBuiltin, Options: []sourcecatalog.Option{}, StructuredOutput: []sourcecatalog.StructuredOutput{}}},
	}
}

func inspectIntent() operation.Intent {
	return operation.Intent{Command: "source inspect", Effect: operation.EffectExecute}
}

func TestInspectAcceptsAlternateAdapterThroughSharedContract(t *testing.T) {
	adapter := &fakeInspector{catalog: fixtureCatalog()}
	result, err := New(map[string]InspectorPort{"alternate": adapter}).Inspect(context.Background(), inspectIntent(), "alternate", "fixture")
	if err != nil || adapter.calls != 1 || len(result.Digest) != 64 || result.Catalog.Adapter.Kind != "atsura.source.alternate" {
		t.Fatalf("result = %+v, calls = %d, error = %v", result, adapter.calls, err)
	}
}

func TestInspectFailsBeforeAdapterForInvalidSelectionAndIntent(t *testing.T) {
	adapter := &fakeInspector{catalog: fixtureCatalog()}
	service := New(map[string]InspectorPort{"alternate": adapter})
	_, err := service.Inspect(context.Background(), inspectIntent(), "missing", "fixture")
	if code(t, err) != "unsupported_source_adapter" || adapter.calls != 0 {
		t.Fatalf("error = %v, calls = %d", err, adapter.calls)
	}
	_, err = service.Inspect(context.Background(), operation.Intent{Command: "doctor", Effect: operation.EffectRead}, "alternate", "fixture")
	if err == nil || adapter.calls != 0 {
		t.Fatalf("error = %v, calls = %d", err, adapter.calls)
	}
	_, err = service.Inspect(context.Background(), operation.Intent{Command: "source inspect", Effect: operation.EffectRead}, "alternate", "fixture")
	if err == nil || adapter.calls != 0 {
		t.Fatalf("read intent error = %v, calls = %d", err, adapter.calls)
	}
}

func TestInspectRejectsAdapterFaultsAndMismatchedEvidence(t *testing.T) {
	tests := []struct {
		name string
		fake fakeInspector
		code string
	}{
		{name: "unsupported version", fake: fakeInspector{err: sourcecatalog.ErrUnsupportedVersion}, code: "unsupported_source_version"},
		{name: "probe", fake: fakeInspector{err: sourcecatalog.ErrInspectionFailed}, code: "source_inspection_failed"},
		{name: "invalid", fake: fakeInspector{catalog: sourcecatalog.Catalog{}}, code: "invalid_source_catalog"},
		{name: "wrong source", fake: fakeInspector{catalog: func() sourcecatalog.Catalog { c := fixtureCatalog(); c.Source.RequestedExecutable = "other"; return c }()}, code: "invalid_source_catalog"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := New(map[string]InspectorPort{"alternate": &test.fake}).Inspect(context.Background(), inspectIntent(), "alternate", "fixture")
			if got := code(t, err); got != test.code || test.fake.calls != 1 {
				t.Fatalf("code = %q, calls = %d, error = %v", got, test.fake.calls, err)
			}
		})
	}
}

func code(t *testing.T, err error) string {
	t.Helper()
	if err == nil {
		t.Fatal("expected error")
	}
	var public *fault.Error
	if !errors.As(err, &public) {
		t.Fatalf("not a public fault: %v", err)
	}
	return public.Code
}
