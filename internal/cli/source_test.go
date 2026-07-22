package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/tasuku43/atsura/internal/app/sourceinspect"
	"github.com/tasuku43/atsura/internal/domain/sourcecatalog"
	"github.com/tasuku43/atsura/internal/infra/gocli"
)

func TestE2ESourceInspectUsesProductionGoAdapter(t *testing.T) {
	goExecutable, err := exec.LookPath("go")
	if err != nil {
		t.Fatalf("resolve Go executable: %v", err)
	}
	var out, errOut bytes.Buffer
	command := New(strings.NewReader(""), &out, &errOut)
	exit := command.RunContext(context.Background(), []string{"source", "inspect", "--adapter", "go-cli", "--executable", goExecutable})
	if exit != ExitOK || errOut.Len() != 0 {
		t.Fatalf("exit=%d stderr=%q", exit, errOut.String())
	}
	var document sourceInspectionDocument
	if err := json.Unmarshal(out.Bytes(), &document); err != nil {
		t.Fatal(err)
	}
	catalog := document.Inspection.Catalog
	if document.SchemaVersion != 1 || len(document.Inspection.CatalogDigest) != 64 || document.Inspection.SourceProcessAttempts != 3 ||
		catalog.Adapter.Kind != gocli.AdapterKind || catalog.Adapter.ContractVersion != gocli.ContractVersion || catalog.Source.Version != runtime.Version() {
		t.Fatalf("production Go inspection=%+v", document.Inspection)
	}
	foundTest := false
	for _, sourceCommand := range catalog.Commands {
		if len(sourceCommand.Path) == 1 && sourceCommand.Path[0] == "test" {
			foundTest = true
			break
		}
	}
	if !foundTest {
		t.Fatalf("production Go catalog lacks test command: %+v", catalog.Commands)
	}
}

func TestProductionSourceRegistryCoversEveryCatalogAdapter(t *testing.T) {
	spec, found := DefaultCatalog().Lookup("source inspect")
	if !found || len(spec.Agent.Inputs) == 0 {
		t.Fatal("source inspect adapter contract is missing")
	}
	missingExecutable := filepath.Join(t.TempDir(), "not-installed")
	for _, adapter := range spec.Agent.Inputs[0].AllowedValues {
		t.Run(adapter, func(t *testing.T) {
			var out, errOut bytes.Buffer
			command := New(strings.NewReader(""), &out, &errOut)
			exit := command.RunContext(context.Background(), []string{"source", "inspect", "--adapter", adapter, "--executable", missingExecutable})
			if exit == ExitOK || out.Len() != 0 || !strings.Contains(errOut.String(), "source_executable_not_found") || strings.Contains(errOut.String(), "unsupported_source_adapter") {
				t.Fatalf("adapter=%q exit=%d stdout=%q stderr=%q", adapter, exit, out.String(), errOut.String())
			}
		})
	}
}

type cliSourceInspector struct {
	calls int
}

func (f *cliSourceInspector) Inspect(_ context.Context, executable string) (sourcecatalog.Catalog, error) {
	f.calls++
	return sourcecatalog.Catalog{
		SchemaVersion: sourcecatalog.SchemaVersion,
		Adapter:       sourcecatalog.Adapter{Kind: "atsura.source.alternate", ContractVersion: 1},
		Source: sourcecatalog.Source{
			RequestedExecutable: executable, ResolvedPath: "/opt/bin/fixture",
			SHA256: strings.Repeat("a", 64), Size: 42, Version: "1.0.0",
		},
		Probe: sourcecatalog.Probe{IDs: []string{"help", "version"}, Attempts: 2},
		Commands: []sourcecatalog.Command{{
			Path: []string{"item", "list"}, Summary: "List items", Provenance: sourcecatalog.ProvenanceVerifiedBuiltin,
			Options: []sourcecatalog.Option{}, StructuredOutput: []sourcecatalog.StructuredOutput{},
		}},
	}, nil
}

func TestSourceInspectRendersCanonicalEvidence(t *testing.T) {
	var out, errOut bytes.Buffer
	cli := New(strings.NewReader(""), &out, &errOut)
	adapter := &cliSourceInspector{}
	cli.sources = sourceinspect.New(map[string]sourceinspect.InspectorPort{"github-cli": adapter})
	exit := cli.RunContext(context.Background(), []string{"source", "inspect", "--adapter", "github-cli", "--executable", "fixture"})
	if exit != ExitOK || errOut.Len() != 0 || adapter.calls != 1 {
		t.Fatalf("exit = %d, stderr = %q, calls = %d", exit, errOut.String(), adapter.calls)
	}
	var document sourceInspectionDocument
	if err := json.Unmarshal(out.Bytes(), &document); err != nil {
		t.Fatal(err)
	}
	if document.SchemaVersion != 1 || len(document.Inspection.CatalogDigest) != 64 || document.Inspection.SourceProcessAttempts != 2 || document.Inspection.Catalog.Adapter.Kind != "atsura.source.alternate" {
		t.Fatalf("document = %+v", document)
	}
}

func TestSourceInspectRejectsUnknownAdapterBeforeProbe(t *testing.T) {
	var out, errOut bytes.Buffer
	cli := New(strings.NewReader(""), &out, &errOut)
	adapter := &cliSourceInspector{}
	cli.sources = sourceinspect.New(map[string]sourceinspect.InspectorPort{"github-cli": adapter})
	exit := cli.RunContext(context.Background(), []string{"source", "inspect", "--adapter", "other", "--executable", "fixture"})
	if exit != ExitUsage || out.Len() != 0 || adapter.calls != 0 || !strings.Contains(errOut.String(), "invalid_arguments") || !strings.Contains(errOut.String(), "value must be one of github-cli, go-cli") {
		t.Fatalf("exit = %d, stdout = %q, stderr = %q, calls = %d", exit, out.String(), errOut.String(), adapter.calls)
	}
}

func TestSourceInspectOutputFailureIsNotSourceReplayPermission(t *testing.T) {
	var errOut bytes.Buffer
	command := New(strings.NewReader(""), shortWriter{}, &errOut)
	adapter := &cliSourceInspector{}
	command.sources = sourceinspect.New(map[string]sourceinspect.InspectorPort{"github-cli": adapter})
	exit := command.RunContext(context.Background(), []string{"source", "inspect", "--adapter", "github-cli", "--executable", "fixture"})
	if exit != ExitInternal || adapter.calls != 1 || !strings.Contains(errOut.String(), "code: execute_output_write_failed") || !strings.Contains(errOut.String(), "retryable: false") {
		t.Fatalf("exit=%d calls=%d stderr=%q", exit, adapter.calls, errOut.String())
	}
}
