package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"

	"github.com/tasuku43/atsura/internal/app/bundleauthority"
	"github.com/tasuku43/atsura/internal/app/bundleexecute"
	"github.com/tasuku43/atsura/internal/app/planpreview"
	"github.com/tasuku43/atsura/internal/app/processorcompat"
	"github.com/tasuku43/atsura/internal/domain/bundletrust"
	"github.com/tasuku43/atsura/internal/domain/processorprocess"
	"github.com/tasuku43/atsura/internal/domain/sourcecatalog"
	"github.com/tasuku43/atsura/internal/domain/sourceprocess"
	"github.com/tasuku43/atsura/internal/domain/tailoringbundle"
	"github.com/tasuku43/atsura/internal/domain/tailoringplan"
	"github.com/tasuku43/atsura/internal/infra/bundlejson"
	"github.com/tasuku43/atsura/internal/infra/githubcli"
	"github.com/tasuku43/atsura/internal/infra/gocli"
	"github.com/tasuku43/atsura/internal/infra/processorjson"
	"github.com/tasuku43/atsura/internal/infra/sourceexec"
	"github.com/tasuku43/atsura/internal/infra/sourcejson"
	"github.com/tasuku43/atsura/internal/infra/trustfile"
)

func TestMain(m *testing.M) {
	if len(os.Args) >= 3 && os.Args[1] == "pr" && os.Args[2] == "list" {
		want := []string{"pr", "list", "--limit=1", "--json=number,title,state"}
		if strings.Join(os.Args[1:], "\x00") != strings.Join(want, "\x00") {
			_, _ = fmt.Fprintln(os.Stderr, "synthetic GitHub CLI received unexpected argv")
			os.Exit(2)
		}
		_, _ = fmt.Fprint(os.Stdout, `[{"number":101,"title":"Review policy","state":"OPEN","ignored":"secret-canary"}]`)
		os.Exit(0)
	}
	if len(os.Args) == 2 && os.Args[1] == "test" {
		_, _ = fmt.Fprintln(os.Stdout, "synthetic go test stdout")
		_, _ = fmt.Fprintln(os.Stderr, "synthetic go test stderr")
		os.Exit(0)
	}
	os.Exit(m.Run())
}

type cliTrustStore struct{ state bundletrust.State }

func (s *cliTrustStore) Inspect(context.Context, string) bundletrust.State { return s.state }
func (s *cliTrustStore) Add(context.Context, string) (bool, error) {
	s.state = bundletrust.StateAdopted
	return true, nil
}

type cliConfirmation struct{}

func (cliConfirmation) Confirm(context.Context, bundletrust.Summary) error { return nil }

type cliProcessorIdentity struct {
	identity processorprocess.Identity
	err      error
	calls    int
	path     string
}

func (p *cliProcessorIdentity) Identify(_ context.Context, path string) (processorprocess.Identity, error) {
	p.calls++
	p.path = path
	return p.identity, p.err
}

type cliRuntimeProof struct{ err error }

func (p *cliRuntimeProof) VerifyRuntime(tailoringplan.Plan) error { return p.err }

type cliBoundProcess struct {
	stdout  []byte
	stderr  []byte
	err     error
	calls   int
	request sourceprocess.BoundRequest
}

func (p *cliBoundProcess) RunBound(_ context.Context, request sourceprocess.BoundRequest) (sourceprocess.Result, error) {
	p.calls++
	p.request = request
	if p.err != nil {
		return sourceprocess.Result{Attempts: 1, ExitCode: -1, Identity: request.ExpectedIdentity}, p.err
	}
	return sourceprocess.Result{Attempts: 1, ExitCode: 0, Stdout: append([]byte{}, p.stdout...), Stderr: append([]byte{}, p.stderr...), Identity: request.ExpectedIdentity}, nil
}

func installTrustedBundleAuthority(command *CLI) {
	loader := bundlejson.New()
	runner := sourceexec.New()
	store := &cliTrustStore{state: bundletrust.StateAdopted}
	command.authority = bundleauthority.New(loader, runner, store, cliConfirmation{})
	command.previews = planpreview.New(loader, store, runner)
}

func installTrustedBundleExecution(command *CLI, process *cliBoundProcess) {
	loader := bundlejson.New()
	runner := sourceexec.New()
	store := &cliTrustStore{state: bundletrust.StateAdopted}
	command.authority = bundleauthority.New(loader, runner, store, cliConfirmation{})
	command.previews = planpreview.New(loader, store, runner)
	command.executions = bundleexecute.New(loader, store, runner, &cliRuntimeProof{}, process, sourcejson.New())
}

func installProductionTrustedBundleExecution(command *CLI) {
	loader := bundlejson.New()
	runner := sourceexec.New()
	store := &cliTrustStore{state: bundletrust.StateAdopted}
	command.authority = bundleauthority.New(loader, runner, store, cliConfirmation{})
	command.previews = planpreview.New(loader, store, runner)
	command.executions = bundleexecute.New(loader, store, runner, newRuntimeCompatibility(), runner, sourcejson.New())
}

func installGitHubCompatibilityExecution(command *CLI, process *cliBoundProcess) {
	loader := bundlejson.New()
	runner := sourceexec.New()
	store := &cliTrustStore{state: bundletrust.StateAdopted}
	command.authority = bundleauthority.New(loader, runner, store, cliConfirmation{})
	command.previews = planpreview.New(loader, store, runner)
	command.executions = bundleexecute.New(loader, store, runner, githubcli.NewRuntimeVerifier(), process, sourcejson.New())
}

func bundleArtifactPaths(t *testing.T) (string, string) {
	t.Helper()
	identity, err := sourceexec.New().Identify(context.Background(), os.Args[0])
	if err != nil {
		t.Fatal(err)
	}
	catalog := sourcecatalog.Catalog{
		SchemaVersion: sourcecatalog.SchemaVersion,
		Adapter:       sourcecatalog.Adapter{Kind: "atsura.source.synthetic", ContractVersion: 1},
		Source:        sourcecatalog.Source{RequestedExecutable: os.Args[0], ResolvedPath: identity.ResolvedPath, SHA256: identity.SHA256, Size: identity.Size, Version: "1.0.0"},
		Probe:         sourcecatalog.Probe{IDs: []string{"help", "version"}, Attempts: 2},
		Commands: []sourcecatalog.Command{{
			Path: []string{"item", "list"}, Summary: "List items", Provenance: sourcecatalog.ProvenanceVerifiedBuiltin,
			Options:          []sourcecatalog.Option{{Name: "--json", TakesValue: true}},
			StructuredOutput: []sourcecatalog.StructuredOutput{{Format: "json", SelectorFlag: "--json", Fields: []string{"id", "name"}}},
		}},
	}
	digest, err := catalog.Digest()
	if err != nil {
		t.Fatal(err)
	}
	directory := t.TempDir()
	catalogPath := filepath.Join(directory, "catalog.json")
	catalogDocument := sourceInspectionDocument{SchemaVersion: 1, Inspection: sourceInspectionPayload{CatalogDigest: digest, Catalog: catalog, SourceProcessAttempts: 2}}
	raw, err := json.Marshal(catalogDocument)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(catalogPath, raw, 0o600); err != nil {
		t.Fatal(err)
	}
	specificationPath := filepath.Join(directory, "spec.yaml")
	specification := fmt.Sprintf(`schema_version: %d
catalog_digest: %s
surface:
  default: exclude
commands:
  - command: [item, list]
    presence: include
    reason: Return a compact inventory.
    options:
      default: inherit
      include: []
      exclude: []
    wrapper:
      kind: transform
      before: []
      invoke:
        append_args: ["--json=id,name"]
      output:
        kind: projection
        projection:
          input: json
          select: [id, name]
          rename: []
          render: compact_json
      after: []
`, tailoringbundle.SpecificationSchemaVersion, digest)
	if err := os.WriteFile(specificationPath, []byte(specification), 0o600); err != nil {
		t.Fatal(err)
	}
	return catalogPath, specificationPath
}

func githubRuntimeBundleArtifactPaths(t *testing.T) (string, string) {
	t.Helper()
	identity, err := sourceexec.New().Identify(context.Background(), os.Args[0])
	if err != nil {
		t.Fatal(err)
	}
	catalog := sourcecatalog.Sort(sourcecatalog.Catalog{
		SchemaVersion: sourcecatalog.SchemaVersion,
		Adapter:       sourcecatalog.Adapter{Kind: githubcli.AdapterKind, ContractVersion: githubcli.ContractVersion},
		Source: sourcecatalog.Source{
			RequestedExecutable: os.Args[0], ResolvedPath: identity.ResolvedPath, SHA256: identity.SHA256, Size: identity.Size, Version: "2.72.0",
		},
		Probe: sourcecatalog.Probe{IDs: []string{"help_reference", "issue_list_help", "pr_list_help", "version"}, Attempts: 4},
		Commands: []sourcecatalog.Command{{
			Path: []string{"pr", "list"}, Summary: "List pull requests", Provenance: sourcecatalog.ProvenanceVerifiedBuiltin,
			Options: []sourcecatalog.Option{
				{Name: "--jq", TakesValue: true}, {Name: "--json", TakesValue: true}, {Name: "--limit", TakesValue: true},
				{Name: "--template", TakesValue: true}, {Name: "--web", TakesValue: false},
			},
			StructuredOutput: []sourcecatalog.StructuredOutput{{
				Format: "json", SelectorFlag: "--json", Fields: []string{"number", "state", "title"},
			}},
		}},
	})
	digest, err := catalog.Digest()
	if err != nil {
		t.Fatal(err)
	}
	directory := t.TempDir()
	catalogPath := filepath.Join(directory, "catalog.json")
	catalogDocument := sourceInspectionDocument{SchemaVersion: 1, Inspection: sourceInspectionPayload{CatalogDigest: digest, Catalog: catalog, SourceProcessAttempts: 4}}
	raw, err := json.Marshal(catalogDocument)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(catalogPath, raw, 0o600); err != nil {
		t.Fatal(err)
	}
	specificationPath := filepath.Join(directory, "spec.yaml")
	specification := fmt.Sprintf(`schema_version: %d
catalog_digest: %s
surface:
  default: exclude
commands:
  - command: [pr, list]
    presence: include
    reason: Return a reviewed compact pull request inventory.
    options:
      default: inherit
      include: []
      exclude: []
    wrapper:
      kind: transform
      before: []
      invoke:
        append_args: ["--json=number,title,state"]
      output:
        kind: projection
        projection:
          input: json
          select: [number, title, state]
          rename:
            - from: number
              to: id
          render: compact_json
      after: []
`, tailoringbundle.SpecificationSchemaVersion, digest)
	if err := os.WriteFile(specificationPath, []byte(specification), 0o600); err != nil {
		t.Fatal(err)
	}
	return catalogPath, specificationPath
}

func goRuntimeBundleArtifactPaths(t *testing.T) (string, string) {
	t.Helper()
	identity, err := sourceexec.New().Identify(context.Background(), os.Args[0])
	if err != nil {
		t.Fatal(err)
	}
	catalog := sourcecatalog.Sort(sourcecatalog.Catalog{
		SchemaVersion: sourcecatalog.SchemaVersion,
		Adapter:       sourcecatalog.Adapter{Kind: gocli.AdapterKind, ContractVersion: gocli.ContractVersion},
		Source: sourcecatalog.Source{
			RequestedExecutable: "go", ResolvedPath: identity.ResolvedPath, SHA256: identity.SHA256, Size: identity.Size, Version: runtime.Version(),
		},
		Probe: sourcecatalog.Probe{IDs: []string{"version", "help", "test_help"}, Attempts: 3},
		Commands: []sourcecatalog.Command{{
			Path: []string{"test"}, Summary: "test packages", Provenance: sourcecatalog.ProvenanceVerifiedBuiltin,
			Options: []sourcecatalog.Option{}, StructuredOutput: []sourcecatalog.StructuredOutput{{
				Format:       "go_test_jsonl",
				SelectorFlag: "-json",
				Fields:       []string{"Action", "Elapsed", "FailedBuild", "Output", "Package", "Test", "Time"},
			}},
		}},
	})
	digest, err := catalog.Digest()
	if err != nil {
		t.Fatal(err)
	}
	directory := t.TempDir()
	catalogPath := filepath.Join(directory, "catalog.json")
	catalogDocument := sourceInspectionDocument{SchemaVersion: 1, Inspection: sourceInspectionPayload{CatalogDigest: digest, Catalog: catalog, SourceProcessAttempts: 3}}
	raw, err := json.Marshal(catalogDocument)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(catalogPath, raw, 0o600); err != nil {
		t.Fatal(err)
	}
	specificationPath := filepath.Join(directory, "spec.yaml")
	specification := fmt.Sprintf(`schema_version: %d
catalog_digest: %s
surface:
  default: exclude
commands:
  - command: [test]
    presence: include
    reason: Run the current package tests without added arguments.
    options:
      default: inherit
      include: []
      exclude: []
    wrapper:
      kind: identity
      before: []
      invoke:
        append_args: []
      after: []
`, tailoringbundle.SpecificationSchemaVersion, digest)
	if err := os.WriteFile(specificationPath, []byte(specification), 0o600); err != nil {
		t.Fatal(err)
	}
	return catalogPath, specificationPath
}

func admittedProcessorObservationPath(t *testing.T) (string, processorprocess.Observation) {
	t.Helper()
	observation := testProcessorInspectionResult(t).Observation
	observation.Identity.SHA256 = "2dab449f32ea744c30b02a3ef9806e3e7d3b356a145332f3f2aaabb5ea48edee"
	encoded, err := processorjson.Encode(observation)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "rtk-inspection.json")
	if err := os.WriteFile(path, encoded, 0o600); err != nil {
		t.Fatal(err)
	}
	return path, observation
}

func goOptimizerArtifactPaths(t *testing.T) (string, string, string, processorprocess.Observation) {
	t.Helper()
	catalogPath, specificationPath := goRuntimeBundleArtifactPaths(t)
	processorPath, observation := admittedProcessorObservationPath(t)
	var out, errOut bytes.Buffer
	command := New(strings.NewReader(""), &out, &errOut)
	args := []string{"spec", "init", "--catalog", catalogPath, "--processor", processorPath, "--", "test"}
	if code := command.RunContext(context.Background(), args); code != ExitOK || errOut.Len() != 0 {
		t.Fatalf("optimizer spec init code=%d stderr=%q", code, errOut.String())
	}
	for _, want := range []string{"kind: transform", "append_args:", "- -json", "kind: optimizer", "contract: " + processorcompat.ContractID} {
		if !strings.Contains(out.String(), want) {
			t.Fatalf("optimizer spec lacks %q:\n%s", want, out.String())
		}
	}
	if err := os.WriteFile(specificationPath, out.Bytes(), 0o600); err != nil {
		t.Fatal(err)
	}
	return catalogPath, specificationPath, processorPath, observation
}

func optimizerBundleArtifactPath(t *testing.T, catalogPath, specificationPath, processorPath string) string {
	t.Helper()
	var out, errOut bytes.Buffer
	command := New(strings.NewReader(""), &out, &errOut)
	args := []string{"bundle", "build", "--catalog", catalogPath, "--spec", specificationPath, "--processor", processorPath}
	if code := command.RunContext(context.Background(), args); code != ExitOK || errOut.Len() != 0 {
		t.Fatalf("optimizer bundle build code=%d stderr=%q", code, errOut.String())
	}
	path := filepath.Join(t.TempDir(), "optimizer-bundle.json")
	if err := os.WriteFile(path, out.Bytes(), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func bundleArtifactPath(t *testing.T, catalogPath, specificationPath string) string {
	t.Helper()
	var out, errOut bytes.Buffer
	command := New(strings.NewReader(""), &out, &errOut)
	if code := command.RunContext(context.Background(), bundleCommandArgs("bundle build", catalogPath, specificationPath)); code != ExitOK {
		t.Fatalf("bundle build code = %d, stderr = %q", code, errOut.String())
	}
	path := filepath.Join(t.TempDir(), "bundle.json")
	if err := os.WriteFile(path, out.Bytes(), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func bundleCommandArgs(path, catalog, specification string) []string {
	return []string{strings.Split(path, " ")[0], strings.Split(path, " ")[1], "--catalog", catalog, "--spec", specification}
}

func TestSpecValidateAndBundleBuildCloseCanonicalFileWorkflow(t *testing.T) {
	catalogPath, specificationPath := bundleArtifactPaths(t)
	var validationOut, validationErr bytes.Buffer
	validator := New(strings.NewReader(""), &validationOut, &validationErr)
	if code := validator.RunContext(context.Background(), bundleCommandArgs("spec validate", catalogPath, specificationPath)); code != ExitOK {
		t.Fatalf("spec validate code = %d, stderr = %q", code, validationErr.String())
	}
	var validation specificationValidationDocument
	if err := json.Unmarshal(validationOut.Bytes(), &validation); err != nil {
		t.Fatal(err)
	}
	if !validation.Validation.Valid || len(validation.Validation.SpecificationDigest) != 64 || validation.Validation.CommandCount != 1 || validation.Validation.IncludedCount != 1 || validation.Validation.TransformWrapperCount != 1 {
		t.Fatalf("validation = %+v", validation)
	}

	var buildOut, buildErr bytes.Buffer
	builder := New(strings.NewReader(""), &buildOut, &buildErr)
	if code := builder.RunContext(context.Background(), bundleCommandArgs("bundle build", catalogPath, specificationPath)); code != ExitOK {
		t.Fatalf("bundle build code = %d, stderr = %q", code, buildErr.String())
	}
	var build bundleBuildDocument
	if err := json.Unmarshal(buildOut.Bytes(), &build); err != nil {
		t.Fatal(err)
	}
	digest, err := build.Build.Bundle.Digest()
	if err != nil || digest != build.Build.BundleDigest || build.Build.Bundle.SchemaVersion != tailoringbundle.BundleSchemaVersion ||
		build.Build.Bundle.Processors == nil || build.Build.Bundle.SpecificationDigest != validation.Validation.SpecificationDigest {
		t.Fatalf("build = %+v, digest = %q, error = %v", build, digest, err)
	}
}

func TestProcessorEvidenceClosesGoOptimizerAuthoringAndBuildWorkflow(t *testing.T) {
	catalogPath, specificationPath, processorPath, observation := goOptimizerArtifactPaths(t)
	bundlePath := optimizerBundleArtifactPath(t, catalogPath, specificationPath, processorPath)
	bundle, digest, err := bundlejson.New().Load(context.Background(), bundlePath)
	if err != nil {
		t.Fatal(err)
	}
	if len(digest) != 64 || bundle.SchemaVersion != tailoringbundle.BundleSchemaVersion || len(bundle.Processors) != 1 || len(bundle.Surface) != 1 {
		t.Fatalf("optimizer bundle digest=%q bundle=%+v", digest, bundle)
	}
	binding := bundle.Processors[0]
	entry := bundle.Surface[0]
	if binding.Contract != processorcompat.ContractID || binding.Observation.Identity != observation.Identity ||
		binding.InputFormat != processorcompat.InputFormat || binding.OutputFormat != processorcompat.OutputFormat ||
		entry.Wrapper.Output == nil || entry.Wrapper.Output.Kind != tailoringbundle.OutputKindOptimizer ||
		entry.Wrapper.Output.Optimizer == nil || entry.Wrapper.Output.Optimizer.Contract != processorcompat.ContractID {
		t.Fatalf("optimizer binding=%+v surface=%+v", binding, entry)
	}
}

func TestBundleBuildRejectsMissingAndUnusedProcessorEvidenceBeforeProcessorIO(t *testing.T) {
	t.Run("optimizer requires evidence", func(t *testing.T) {
		catalogPath, specificationPath, _, _ := goOptimizerArtifactPaths(t)
		var out, errOut bytes.Buffer
		command := New(strings.NewReader(""), &out, &errOut)
		if code := command.RunContext(context.Background(), bundleCommandArgs("bundle build", catalogPath, specificationPath)); code != ExitUsage ||
			out.Len() != 0 || !strings.Contains(errOut.String(), "processor_observation_required") {
			t.Fatalf("bundle build missing evidence code=%d stdout=%q stderr=%q", code, out.String(), errOut.String())
		}
	})

	t.Run("identity rejects observation without reading it", func(t *testing.T) {
		catalogPath, specificationPath := goRuntimeBundleArtifactPaths(t)
		missingPath := filepath.Join(t.TempDir(), "must-not-be-read.json")
		var out, errOut bytes.Buffer
		command := New(strings.NewReader(""), &out, &errOut)
		args := []string{"bundle", "build", "--catalog", catalogPath, "--spec", specificationPath, "--processor", missingPath}
		if code := command.RunContext(context.Background(), args); code != ExitUsage || out.Len() != 0 ||
			!strings.Contains(errOut.String(), "processor_observation_not_used") || strings.Contains(errOut.String(), "processor_observation_file_not_found") {
			t.Fatalf("bundle build unused evidence code=%d stdout=%q stderr=%q", code, out.String(), errOut.String())
		}
	})
}

func TestSpecInitRejectsProcessorEvidenceOutsideFiniteCompatibilityRegistry(t *testing.T) {
	catalogPath, _ := bundleArtifactPaths(t)
	processorPath, _ := admittedProcessorObservationPath(t)
	var out, errOut bytes.Buffer
	command := New(strings.NewReader(""), &out, &errOut)
	args := []string{"spec", "init", "--catalog", catalogPath, "--processor", processorPath, "--", "item", "list"}
	if code := command.RunContext(context.Background(), args); code != ExitRejected || out.Len() != 0 || !strings.Contains(errOut.String(), "processor_default_not_admitted") {
		t.Fatalf("spec init incompatible processor code=%d stdout=%q stderr=%q", code, out.String(), errOut.String())
	}
}

func TestBundleStatusAndTrustProjectCurrentProcessorIdentityWithoutProcessAttempts(t *testing.T) {
	catalogPath, specificationPath, processorPath, observation := goOptimizerArtifactPaths(t)
	bundlePath := optimizerBundleArtifactPath(t, catalogPath, specificationPath, processorPath)
	trust := &cliTrustStore{state: bundletrust.StateNotAdopted}
	processorIdentity := &cliProcessorIdentity{identity: observation.Identity}
	newCommand := func() (*CLI, *bytes.Buffer, *bytes.Buffer) {
		var out, errOut bytes.Buffer
		command := New(strings.NewReader(""), &out, &errOut)
		command.authority = bundleauthority.New(bundlejson.New(), sourceexec.New(), trust, cliConfirmation{}, processorIdentity)
		return command, &out, &errOut
	}

	statusCommand, out, errOut := newCommand()
	if code := statusCommand.RunContext(context.Background(), []string{"bundle", "status", "--bundle", bundlePath}); code != ExitOK || errOut.Len() != 0 {
		t.Fatalf("bundle status code=%d stderr=%q", code, errOut.String())
	}
	var status bundleStatusDocument
	if err := json.Unmarshal(out.Bytes(), &status); err != nil {
		t.Fatal(err)
	}
	if status.SchemaVersion != 3 || len(status.Status.Processors) != 1 || status.Status.SourceProcessAttempts != 0 || status.Status.ProcessorProcessAttempts != 0 {
		t.Fatalf("bundle status=%+v", status)
	}
	processor := status.Status.Processors[0]
	if processor.Contract != processorcompat.ContractID || processor.AdapterKind != processorcompat.ProcessorAdapterKind || processor.Version != processorcompat.ProcessorVersion ||
		processor.ResolvedPath != observation.Identity.ResolvedPath || processor.SHA256 != observation.Identity.SHA256 || processor.Size != observation.Identity.Size || processor.State != bundletrust.ProcessorCurrent {
		t.Fatalf("bundle status processor=%+v", processor)
	}
	processorRaw, err := json.Marshal(processor)
	if err != nil {
		t.Fatal(err)
	}
	var processorFields map[string]json.RawMessage
	if err := json.Unmarshal(processorRaw, &processorFields); err != nil {
		t.Fatal(err)
	}
	assertJSONKeys(t, processorFields, []string{"adapter_kind", "contract", "resolved_path", "sha256", "size", "state", "version"})

	trustCommand, out, errOut := newCommand()
	if code := trustCommand.RunContext(context.Background(), []string{"bundle", "trust", "--bundle", bundlePath}); code != ExitOK || errOut.Len() != 0 {
		t.Fatalf("bundle trust code=%d stderr=%q", code, errOut.String())
	}
	var trustDocument bundleTrustDocument
	if err := json.Unmarshal(out.Bytes(), &trustDocument); err != nil {
		t.Fatal(err)
	}
	if trustDocument.SchemaVersion != 3 || !trustDocument.Trust.Adopted || len(trustDocument.Trust.Processors) != 1 ||
		trustDocument.Trust.Processors[0] != processor || trustDocument.Trust.SourceProcessAttempts != 0 || trustDocument.Trust.ProcessorProcessAttempts != 0 ||
		processorIdentity.calls != 2 || processorIdentity.path != observation.Identity.ResolvedPath {
		t.Fatalf("bundle trust=%+v identity=%+v", trustDocument, processorIdentity)
	}
}

func TestBundleTrustRejectsProcessorDriftBeforeAdoption(t *testing.T) {
	catalogPath, specificationPath, processorPath, observation := goOptimizerArtifactPaths(t)
	bundlePath := optimizerBundleArtifactPath(t, catalogPath, specificationPath, processorPath)
	trust := &cliTrustStore{state: bundletrust.StateNotAdopted}
	drifted := observation.Identity
	drifted.SHA256 = strings.Repeat("b", 64)
	processorIdentity := &cliProcessorIdentity{identity: drifted}
	var out, errOut bytes.Buffer
	command := New(strings.NewReader(""), &out, &errOut)
	command.authority = bundleauthority.New(bundlejson.New(), sourceexec.New(), trust, cliConfirmation{}, processorIdentity)
	if code := command.RunContext(context.Background(), []string{"bundle", "trust", "--bundle", bundlePath}); code != ExitRejected || out.Len() != 0 ||
		!strings.Contains(errOut.String(), "bundle_processor_drift") || trust.state != bundletrust.StateNotAdopted || processorIdentity.calls != 1 {
		t.Fatalf("bundle trust drift code=%d stdout=%q stderr=%q trust=%q identity=%+v", code, out.String(), errOut.String(), trust.state, processorIdentity)
	}
}

func TestBundleStatusAndTrustUseExactBundleWithoutSourceProcess(t *testing.T) {
	catalogPath, specificationPath := bundleArtifactPaths(t)
	bundlePath := bundleArtifactPath(t, catalogPath, specificationPath)
	trust := &cliTrustStore{state: bundletrust.StateNotAdopted}
	newCommand := func() (*CLI, *bytes.Buffer, *bytes.Buffer) {
		var out, errOut bytes.Buffer
		command := New(strings.NewReader("redirected input must not confirm"), &out, &errOut)
		command.authority = bundleauthority.New(bundlejson.New(), sourceexec.New(), trust, cliConfirmation{})
		return command, &out, &errOut
	}

	status, out, errOut := newCommand()
	if code := status.RunContext(context.Background(), []string{"bundle", "status", "--bundle", bundlePath}); code != ExitOK {
		t.Fatalf("bundle status code = %d, stderr = %q", code, errOut.String())
	}
	var statusDocument bundleStatusDocument
	if err := json.Unmarshal(out.Bytes(), &statusDocument); err != nil {
		t.Fatal(err)
	}
	if statusDocument.SchemaVersion != 3 || statusDocument.Status.Adoption != bundletrust.StateNotAdopted || statusDocument.Status.Source != bundletrust.SourceCurrent || statusDocument.Status.Adopted ||
		statusDocument.Status.Processors == nil || len(statusDocument.Status.Processors) != 0 || statusDocument.Status.SourceProcessAttempts != 0 || statusDocument.Status.ProcessorProcessAttempts != 0 {
		t.Fatalf("status = %+v", statusDocument.Status)
	}

	trustCommand, out, errOut := newCommand()
	if code := trustCommand.RunContext(context.Background(), []string{"bundle", "trust", "--bundle", bundlePath}); code != ExitOK {
		t.Fatalf("bundle trust code = %d, stderr = %q", code, errOut.String())
	}
	var trustDocument bundleTrustDocument
	if err := json.Unmarshal(out.Bytes(), &trustDocument); err != nil {
		t.Fatal(err)
	}
	if trustDocument.SchemaVersion != 3 || !trustDocument.Trust.Adopted || trustDocument.Trust.AlreadyAdopted || trustDocument.Trust.Processors == nil || len(trustDocument.Trust.Processors) != 0 ||
		trustDocument.Trust.SourceProcessAttempts != 0 || trustDocument.Trust.ProcessorProcessAttempts != 0 || trust.state != bundletrust.StateAdopted {
		t.Fatalf("trust = %+v, store = %q", trustDocument.Trust, trust.state)
	}
}

func TestBundlePreviewReturnsCompleteSchemaTwoPlanWithoutSourceAttempt(t *testing.T) {
	catalogPath, specificationPath := bundleArtifactPaths(t)
	bundlePath := bundleArtifactPath(t, catalogPath, specificationPath)
	var out, errOut bytes.Buffer
	command := New(strings.NewReader(""), &out, &errOut)
	installTrustedBundleAuthority(command)
	args := []string{"bundle", "preview", "--bundle", bundlePath, "--", os.Args[0], "item", "list", "active"}
	if code := command.RunContext(context.Background(), args); code != ExitOK {
		t.Fatalf("bundle preview code=%d stderr=%q", code, errOut.String())
	}
	var top map[string]json.RawMessage
	if err := json.Unmarshal(out.Bytes(), &top); err != nil {
		t.Fatal(err)
	}
	assertJSONKeys(t, top, []string{"preview", "schema_version"})
	var previewObject map[string]json.RawMessage
	if err := json.Unmarshal(top["preview"], &previewObject); err != nil {
		t.Fatal(err)
	}
	assertJSONKeys(t, previewObject, []string{"plan", "plan_digest", "source_process_attempts"})
	var planObject map[string]json.RawMessage
	if err := json.Unmarshal(previewObject["plan"], &planObject); err != nil {
		t.Fatal(err)
	}
	assertJSONKeys(t, planObject, []string{
		"bundle_digest", "catalog_digest", "matched_command", "mode", "options", "original_argv", "processor", "reason", "result_mode",
		"schema_version", "source", "specification_digest", "specification_entry", "stages", "surface_origin",
		"transformed_argv", "wrapper_kind",
	})
	if string(planObject["processor"]) != "null" {
		t.Fatalf("projection plan processor = %s, want null", planObject["processor"])
	}
	var specificationEntry map[string]json.RawMessage
	if err := json.Unmarshal(planObject["specification_entry"], &specificationEntry); err != nil {
		t.Fatal(err)
	}
	var specificationWrapper map[string]json.RawMessage
	if err := json.Unmarshal(specificationEntry["wrapper"], &specificationWrapper); err != nil {
		t.Fatal(err)
	}
	var specificationOutput map[string]json.RawMessage
	if err := json.Unmarshal(specificationWrapper["output"], &specificationOutput); err != nil {
		t.Fatal(err)
	}
	assertJSONKeys(t, specificationOutput, []string{"kind", "projection"})
	var sourceObject map[string]json.RawMessage
	if err := json.Unmarshal(planObject["source"], &sourceObject); err != nil {
		t.Fatal(err)
	}
	assertJSONKeys(t, sourceObject, []string{"adapter_contract_version", "adapter_kind", "requested_executable", "resolved_path", "sha256", "size", "version"})
	var stagesObject map[string]json.RawMessage
	if err := json.Unmarshal(planObject["stages"], &stagesObject); err != nil {
		t.Fatal(err)
	}
	assertJSONKeys(t, stagesObject, []string{"after", "before", "invoke", "order", "output"})
	var outputObject map[string]json.RawMessage
	if err := json.Unmarshal(stagesObject["output"], &outputObject); err != nil {
		t.Fatal(err)
	}
	assertJSONKeys(t, outputObject, []string{"kind", "projection"})
	var projectionObject map[string]json.RawMessage
	if err := json.Unmarshal(outputObject["projection"], &projectionObject); err != nil {
		t.Fatal(err)
	}
	assertJSONKeys(t, projectionObject, []string{"input", "rename", "render", "select"})
	var invokeObject map[string]json.RawMessage
	if err := json.Unmarshal(stagesObject["invoke"], &invokeObject); err != nil {
		t.Fatal(err)
	}
	assertJSONKeys(t, invokeObject, []string{"appended_args", "args", "environment_mode", "executable", "max_attempts", "stderr_limit_bytes", "stdin_mode", "stdout_limit_bytes", "timeout_millis", "working_directory_mode"})
	var document bundlePreviewDocument
	if err := json.Unmarshal(out.Bytes(), &document); err != nil {
		t.Fatal(err)
	}
	preview := document.Preview
	if document.SchemaVersion != 2 || len(preview.PlanDigest) != 64 || preview.SourceProcessAttempts != 0 || preview.Plan.SchemaVersion != tailoringplan.SchemaVersion {
		t.Fatalf("preview=%+v", preview)
	}
	if preview.Plan.Source.ResolvedPath == "" || preview.Plan.WrapperKind != "transform" || preview.Plan.SpecificationEntry == nil || preview.Plan.Stages.Invoke.MaxAttempts != 1 {
		t.Fatalf("plan=%+v", preview.Plan)
	}
	wantOriginal := []string{os.Args[0], "item", "list", "active"}
	if strings.Join(preview.Plan.OriginalArgv, "\x00") != strings.Join(wantOriginal, "\x00") || preview.Plan.TransformedArgv[len(preview.Plan.TransformedArgv)-1] != "--json=id,name" {
		t.Fatalf("argv original=%v transformed=%v", preview.Plan.OriginalArgv, preview.Plan.TransformedArgv)
	}
}

func TestBundleExecuteReturnsSchemaTwoTypedTransformWithPreviewDigest(t *testing.T) {
	catalogPath, specificationPath := bundleArtifactPaths(t)
	bundlePath := bundleArtifactPath(t, catalogPath, specificationPath)

	var previewOut, previewErr bytes.Buffer
	previewCommand := New(strings.NewReader(""), &previewOut, &previewErr)
	installTrustedBundleAuthority(previewCommand)
	args := []string{"bundle", "preview", "--bundle", bundlePath, "--", os.Args[0], "item", "list"}
	if code := previewCommand.RunContext(context.Background(), args); code != ExitOK {
		t.Fatalf("preview code=%d stderr=%q", code, previewErr.String())
	}
	var preview bundlePreviewDocument
	if err := json.Unmarshal(previewOut.Bytes(), &preview); err != nil {
		t.Fatal(err)
	}

	process := &cliBoundProcess{stdout: []byte("[{\"name\":\"line\\n bidi:\\u202e slash:\\\\\",\"id\":0,\"ignored\":\"secret-canary\"}]")}
	var executeOut, executeErr bytes.Buffer
	executeCommand := New(strings.NewReader(""), &executeOut, &executeErr)
	installTrustedBundleExecution(executeCommand, process)
	executeArgs := []string{"bundle", "execute", "--bundle", bundlePath, "--", os.Args[0], "item", "list"}
	if code := executeCommand.RunContext(context.Background(), executeArgs); code != ExitOK {
		t.Fatalf("execute code=%d stderr=%q", code, executeErr.String())
	}
	if process.calls != 1 || strings.Contains(executeOut.String(), "secret-canary") {
		t.Fatalf("process calls=%d output=%s", process.calls, executeOut.String())
	}
	var document struct {
		SchemaVersion int `json:"schema_version"`
		Execution     struct {
			BundleDigest   string   `json:"bundle_digest"`
			PlanDigest     string   `json:"plan_digest"`
			MatchedCommand []string `json:"matched_command"`
			WrapperKind    string   `json:"wrapper_kind"`
			Output         struct {
				Render  string           `json:"render"`
				Shape   string           `json:"shape"`
				Fields  []string         `json:"fields"`
				Records []map[string]any `json:"records"`
			} `json:"output"`
			Source struct {
				ExitCode int `json:"exit_code"`
			} `json:"source"`
			SourceProcessAttempts int `json:"source_process_attempts"`
		} `json:"execution"`
	}
	if err := json.Unmarshal(executeOut.Bytes(), &document); err != nil {
		t.Fatal(err)
	}
	execution := document.Execution
	if document.SchemaVersion != 2 || execution.PlanDigest != preview.Preview.PlanDigest || execution.SourceProcessAttempts != 1 || execution.Source.ExitCode != 0 || execution.Output.Shape != "array" || execution.Output.Render != "compact_json" {
		t.Fatalf("execution=%+v preview=%+v", execution, preview.Preview)
	}
	if strings.Join(execution.Output.Fields, ",") != "id,name" || len(execution.Output.Records) != 1 {
		t.Fatalf("output=%+v", execution.Output)
	}
	record := execution.Output.Records[0]
	if record["id"] != float64(0) || record["name"] != `line\n bidi:\u202E slash:\\` || !strings.Contains(executeOut.String(), `"records":[{"id":0,"name":`) {
		t.Fatalf("record=%+v", record)
	}
	if process.request.Process.Executable != preview.Preview.Plan.Source.ResolvedPath || strings.Join(process.request.Process.Args, "\x00") != "item\x00list\x00--json=id,name" {
		t.Fatalf("request=%+v", process.request)
	}
}

func TestBundleExecuteProductionCompositionRunsSyntheticGitHubFixture(t *testing.T) {
	catalogPath, specificationPath := githubRuntimeBundleArtifactPaths(t)
	bundlePath := bundleArtifactPath(t, catalogPath, specificationPath)
	args := []string{"--bundle", bundlePath, "--", os.Args[0], "pr", "list", "--limit=1"}

	var previewOut, previewErr bytes.Buffer
	previewCommand := New(strings.NewReader(""), &previewOut, &previewErr)
	installProductionTrustedBundleExecution(previewCommand)
	if code := previewCommand.RunContext(context.Background(), append([]string{"bundle", "preview"}, args...)); code != ExitOK {
		t.Fatalf("preview code=%d stderr=%q", code, previewErr.String())
	}
	var preview bundlePreviewDocument
	if err := json.Unmarshal(previewOut.Bytes(), &preview); err != nil {
		t.Fatal(err)
	}

	var executeOut, executeErr bytes.Buffer
	executeCommand := New(strings.NewReader(""), &executeOut, &executeErr)
	installProductionTrustedBundleExecution(executeCommand)
	if code := executeCommand.RunContext(context.Background(), append([]string{"bundle", "execute"}, args...)); code != ExitOK {
		t.Fatalf("execute code=%d stderr=%q", code, executeErr.String())
	}
	if strings.Contains(executeOut.String(), "secret-canary") {
		t.Fatalf("unselected source field leaked: %s", executeOut.String())
	}
	var document struct {
		SchemaVersion int `json:"schema_version"`
		Execution     struct {
			PlanDigest            string `json:"plan_digest"`
			SourceProcessAttempts int    `json:"source_process_attempts"`
			Output                struct {
				Shape   string           `json:"shape"`
				Fields  []string         `json:"fields"`
				Records []map[string]any `json:"records"`
			} `json:"output"`
			Source struct {
				ExitCode int `json:"exit_code"`
			} `json:"source"`
		} `json:"execution"`
	}
	if err := json.Unmarshal(executeOut.Bytes(), &document); err != nil {
		t.Fatal(err)
	}
	execution := document.Execution
	if execution.PlanDigest != preview.Preview.PlanDigest || execution.SourceProcessAttempts != 1 || execution.Source.ExitCode != 0 {
		t.Fatalf("execution=%+v preview=%+v", execution, preview.Preview)
	}
	if execution.Output.Shape != "array" || strings.Join(execution.Output.Fields, ",") != "id,title,state" || len(execution.Output.Records) != 1 {
		t.Fatalf("output=%+v", execution.Output)
	}
	record := execution.Output.Records[0]
	if record["id"] != float64(101) || record["title"] != "Review policy" || record["state"] != "OPEN" {
		t.Fatalf("record=%+v", record)
	}
}

func TestProductionCompositionRendersAndRunsSyntheticGoWrapper(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX wrapper materialization is intentionally unsupported on Windows")
	}
	catalogPath, specificationPath := goRuntimeBundleArtifactPaths(t)
	bundlePath := bundleArtifactPath(t, catalogPath, specificationPath)
	bundle, bundleDigest, err := bundlejson.New().Load(context.Background(), bundlePath)
	if err != nil {
		t.Fatal(err)
	}

	configRoot := t.TempDir()
	configDirectory := filepath.Join(configRoot, "config")
	if runtime.GOOS == "darwin" {
		configDirectory = filepath.Join(configRoot, "Library", "Application Support")
	}
	if err := os.MkdirAll(configDirectory, 0o700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME", configRoot)
	t.Setenv("USERPROFILE", configRoot)
	t.Setenv("XDG_CONFIG_HOME", configDirectory)
	t.Setenv("APPDATA", configDirectory)
	t.Setenv("LOCALAPPDATA", configDirectory)
	trustPath, err := trustfile.DefaultPath()
	if err != nil {
		t.Fatal(err)
	}
	if changed, err := trustfile.New(trustPath).Add(context.Background(), bundleDigest); err != nil || !changed {
		t.Fatalf("adopt Go bundle changed=%v error=%v", changed, err)
	}

	var renderOut, renderErr bytes.Buffer
	renderCommand := New(strings.NewReader(""), &renderOut, &renderErr)
	if code := renderCommand.RunContext(context.Background(), []string{"wrapper", "render", "--bundle", bundlePath, "--format=json"}); code != ExitOK {
		t.Fatalf("render code=%d stderr=%q", code, renderErr.String())
	}
	var rendered wrapperRenderDocument
	if err := json.Unmarshal(renderOut.Bytes(), &rendered); err != nil {
		t.Fatal(err)
	}
	if rendered.Wrapper.Bundle.Digest != bundleDigest || rendered.Wrapper.Command != filepath.Base(bundle.Catalog.Source.RequestedExecutable) || rendered.Wrapper.SourceProcessAttempts != 0 {
		t.Fatalf("rendered Go wrapper=%+v", rendered.Wrapper)
	}

	var runOut, runErr bytes.Buffer
	runCommand := New(strings.NewReader(""), &runOut, &runErr)
	runArgs := []string{
		"wrapper", "run",
		"--contract-version=" + strconv.Itoa(rendered.Wrapper.Contract.Version),
		"--bundle=" + rendered.Wrapper.Bundle.Locator,
		"--bundle-digest=" + rendered.Wrapper.Bundle.Digest,
		"--runtime-path=" + rendered.Wrapper.Runtime.ResolvedPath,
		"--runtime-sha256=" + rendered.Wrapper.Runtime.SHA256,
		"--runtime-size=" + strconv.FormatInt(rendered.Wrapper.Runtime.Size, 10),
		"--", "test",
	}
	if code := runCommand.RunContext(context.Background(), runArgs); code != ExitOK {
		t.Fatalf("wrapper run code=%d stdout=%q stderr=%q", code, runOut.String(), runErr.String())
	}
	if runOut.String() != "synthetic go test stdout\n" || runErr.String() != "synthetic go test stderr\n" {
		t.Fatalf("Go source streams stdout=%q stderr=%q", runOut.String(), runErr.String())
	}
}

func TestBundleExecuteProductionCompatibilityRejectsOutputConflictsBeforeProcess(t *testing.T) {
	catalogPath, specificationPath := githubRuntimeBundleArtifactPaths(t)
	bundlePath := bundleArtifactPath(t, catalogPath, specificationPath)
	tests := [][]string{
		{"pr", "list", "--web"},
		{"pr", "list", "--jq=.[]"},
		{"pr", "list", "--template={{.number}}"},
		{"pr", "list", "unexpected"},
	}
	for _, attempted := range tests {
		t.Run(strings.Join(attempted, "_"), func(t *testing.T) {
			process := &cliBoundProcess{stdout: []byte(`[]`)}
			var out, errOut bytes.Buffer
			command := New(strings.NewReader(""), &out, &errOut)
			installGitHubCompatibilityExecution(command, process)
			args := append([]string{"--error-format=json", "bundle", "execute", "--bundle", bundlePath, "--", os.Args[0]}, attempted...)
			if code := command.RunContext(context.Background(), args); code != ExitUnsupported || out.Len() != 0 || !strings.Contains(errOut.String(), `"code":"wrapper_runtime_not_supported"`) {
				t.Fatalf("code=%d stdout=%q stderr=%q", code, out.String(), errOut.String())
			}
			if process.calls != 0 {
				t.Fatalf("process calls=%d", process.calls)
			}
		})
	}
}

func TestBundleExecuteFinalWriteFailureIsNonRetryableAfterOneAttempt(t *testing.T) {
	catalogPath, specificationPath := bundleArtifactPaths(t)
	bundlePath := bundleArtifactPath(t, catalogPath, specificationPath)
	process := &cliBoundProcess{stdout: []byte(`[]`)}
	var errOut bytes.Buffer
	command := New(strings.NewReader(""), shortWriter{}, &errOut)
	installTrustedBundleExecution(command, process)
	code := command.RunContext(context.Background(), []string{"bundle", "execute", "--bundle", bundlePath, "--", os.Args[0], "item", "list"})
	if code != ExitInternal || process.calls != 1 || !strings.Contains(errOut.String(), "execute_output_write_failed") || !strings.Contains(errOut.String(), "retryable: false") {
		t.Fatalf("code=%d calls=%d stderr=%q", code, process.calls, errOut.String())
	}
}

func TestBundlePreviewRequiresExactAdoptionAndTailoredOptions(t *testing.T) {
	catalogPath, specificationPath := bundleArtifactPaths(t)
	bundlePath := bundleArtifactPath(t, catalogPath, specificationPath)
	loader := bundlejson.New()
	runner := sourceexec.New()
	tests := []struct {
		name  string
		state bundletrust.State
		argv  []string
		code  string
		exit  int
	}{
		{name: "not adopted", state: bundletrust.StateNotAdopted, argv: []string{os.Args[0], "item", "list"}, code: "bundle_not_adopted", exit: ExitRejected},
		{name: "executable mismatch", state: bundletrust.StateAdopted, argv: []string{"other", "item", "list"}, code: "source_executable_mismatch", exit: ExitUsage},
		{name: "unknown option", state: bundletrust.StateAdopted, argv: []string{os.Args[0], "item", "list", "--unknown"}, code: "invalid_invocation", exit: ExitUsage},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var out, errOut bytes.Buffer
			command := New(strings.NewReader(""), &out, &errOut)
			command.previews = planpreview.New(loader, &cliTrustStore{state: test.state}, runner)
			args := append([]string{"--error-format=json", "bundle", "preview", "--bundle", bundlePath, "--"}, test.argv...)
			if exit := command.RunContext(context.Background(), args); exit != test.exit || out.Len() != 0 || !strings.Contains(errOut.String(), `"code":"`+test.code+`"`) {
				t.Fatalf("exit=%d stdout=%q stderr=%q", exit, out.String(), errOut.String())
			}
		})
	}

	raw, err := os.ReadFile(specificationPath)
	if err != nil {
		t.Fatal(err)
	}
	hiddenSpecification := filepath.Join(t.TempDir(), "hidden.yaml")
	if err := os.WriteFile(hiddenSpecification, []byte(strings.Replace(string(raw), "exclude: []", "exclude: [--json]", 1)), 0o600); err != nil {
		t.Fatal(err)
	}
	hiddenBundle := bundleArtifactPath(t, catalogPath, hiddenSpecification)
	var out, errOut bytes.Buffer
	command := New(strings.NewReader(""), &out, &errOut)
	command.previews = planpreview.New(loader, &cliTrustStore{state: bundletrust.StateAdopted}, runner)
	args := []string{"--error-format=json", "bundle", "preview", "--bundle", hiddenBundle, "--", os.Args[0], "item", "list", "--json=id"}
	if exit := command.RunContext(context.Background(), args); exit != ExitNotFound || out.Len() != 0 || !strings.Contains(errOut.String(), `"code":"option_not_in_surface"`) {
		t.Fatalf("exit=%d stdout=%q stderr=%q", exit, out.String(), errOut.String())
	}
}

func TestBundlePreviewOutputFailureIsRetryableReadFailure(t *testing.T) {
	catalogPath, specificationPath := bundleArtifactPaths(t)
	bundlePath := bundleArtifactPath(t, catalogPath, specificationPath)
	var errOut bytes.Buffer
	command := New(strings.NewReader(""), shortWriter{}, &errOut)
	installTrustedBundleAuthority(command)
	args := []string{"bundle", "preview", "--bundle", bundlePath, "--", os.Args[0], "item", "list"}
	if exit := command.RunContext(context.Background(), args); exit != ExitInternal || !strings.Contains(errOut.String(), "code: output_write_failed") || !strings.Contains(errOut.String(), "retryable: true") {
		t.Fatalf("exit=%d stderr=%q", exit, errOut.String())
	}
}

func TestBundlePreviewHelpPublishesExactPositionalOnlyGrammar(t *testing.T) {
	var out, errOut bytes.Buffer
	command := New(strings.NewReader(""), &out, &errOut)
	if exit := command.RunContext(context.Background(), []string{"help", "bundle", "preview"}); exit != ExitOK || errOut.Len() != 0 {
		t.Fatalf("exit=%d stderr=%q", exit, errOut.String())
	}
	for _, required := range []string{"atr bundle preview --bundle <path> -- <source-executable> <argv>", "source-executable", "cardinality: repeatable"} {
		if !strings.Contains(out.String(), required) {
			t.Fatalf("help missing %q:\n%s", required, out.String())
		}
	}
	out.Reset()
	if exit := command.RunContext(context.Background(), []string{"help", "bundle", "preview", "--format=agent"}); exit != ExitOK || errOut.Len() != 0 {
		t.Fatalf("agent help exit=%d stderr=%q", exit, errOut.String())
	}
	for _, required := range []string{`"source_process_attempts"`, `"command_not_in_surface"`, `"option_not_in_surface"`, `"id":"wrapper-plan"`, `"path":"/stages/invoke/max_attempts"`} {
		if !strings.Contains(out.String(), required) {
			t.Fatalf("agent help missing %q:\n%s", required, out.String())
		}
	}
}

func TestSpecInitProducesValidIdentityWrapperDraft(t *testing.T) {
	catalogPath, _ := bundleArtifactPaths(t)
	var out, errOut bytes.Buffer
	command := New(strings.NewReader(""), &out, &errOut)
	args := []string{"spec", "init", "--catalog", catalogPath, "--", "item", "list"}
	if code := command.RunContext(context.Background(), args); code != ExitOK {
		t.Fatalf("spec init code = %d, stderr = %q", code, errOut.String())
	}
	if !strings.Contains(out.String(), "default: exclude") || !strings.Contains(out.String(), "presence: include") || !strings.Contains(out.String(), "kind: identity") || strings.Contains(out.String(), "decision:") {
		t.Fatalf("draft = %s", out.String())
	}
	specificationPath := filepath.Join(t.TempDir(), "draft.yaml")
	if err := os.WriteFile(specificationPath, out.Bytes(), 0o600); err != nil {
		t.Fatal(err)
	}
	var validateOut, validateErr bytes.Buffer
	validator := New(strings.NewReader(""), &validateOut, &validateErr)
	if code := validator.RunContext(context.Background(), bundleCommandArgs("spec validate", catalogPath, specificationPath)); code != ExitOK {
		t.Fatalf("draft validation code = %d, stderr = %q", code, validateErr.String())
	}
}

func TestBundleBuildRejectsCatalogSpecificationMismatch(t *testing.T) {
	catalogPath, specificationPath := bundleArtifactPaths(t)
	raw, err := os.ReadFile(specificationPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(specificationPath, []byte(strings.Replace(string(raw), "catalog_digest: ", "catalog_digest: b", 1)), 0o600); err != nil {
		t.Fatal(err)
	}
	var out, errOut bytes.Buffer
	command := New(strings.NewReader(""), &out, &errOut)
	if code := command.RunContext(context.Background(), bundleCommandArgs("bundle build", catalogPath, specificationPath)); code != ExitUsage || out.Len() != 0 || !strings.Contains(errOut.String(), "invalid_specification") {
		t.Fatalf("code/output/error = %d/%q/%q", code, out.String(), errOut.String())
	}
}
