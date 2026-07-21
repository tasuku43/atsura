package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tasuku43/atsura/internal/app/bundleauthority"
	"github.com/tasuku43/atsura/internal/domain/bundletrust"
	"github.com/tasuku43/atsura/internal/domain/sourcecatalog"
	"github.com/tasuku43/atsura/internal/infra/bundlejson"
	"github.com/tasuku43/atsura/internal/infra/sourceexec"
)

type cliTrustStore struct{ state bundletrust.State }

func (s *cliTrustStore) Inspect(context.Context, string) bundletrust.State { return s.state }
func (s *cliTrustStore) Add(context.Context, string) (bool, error) {
	s.state = bundletrust.StateAdopted
	return true, nil
}

type cliConfirmation struct{}

func (cliConfirmation) Confirm(context.Context, bundletrust.Summary) error { return nil }

func installTrustedBundleAuthority(command *CLI) {
	command.authority = bundleauthority.New(bundlejson.New(), sourceexec.New(), &cliTrustStore{state: bundletrust.StateAdopted}, cliConfirmation{})
}

func bundleArtifactPaths(t *testing.T) (string, string) {
	t.Helper()
	identity, err := sourceexec.New().Identify(context.Background(), os.Args[0])
	if err != nil {
		t.Fatal(err)
	}
	catalog := sourcecatalog.Catalog{
		SchemaVersion: 1,
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
	specification := fmt.Sprintf(`schema_version: 3
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
        input: json
        select: [id, name]
        rename: []
        render: compact_json
      after: []
`, digest)
	if err := os.WriteFile(specificationPath, []byte(specification), 0o600); err != nil {
		t.Fatal(err)
	}
	return catalogPath, specificationPath
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
	if err != nil || digest != build.Build.BundleDigest || build.Build.Bundle.SpecificationDigest != validation.Validation.SpecificationDigest {
		t.Fatalf("build = %+v, digest = %q, error = %v", build, digest, err)
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
	if statusDocument.Status.Adoption != bundletrust.StateNotAdopted || statusDocument.Status.Source != bundletrust.SourceCurrent || statusDocument.Status.Adopted || statusDocument.Status.SourceProcessAttempts != 0 {
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
	if !trustDocument.Trust.Adopted || trustDocument.Trust.AlreadyAdopted || trustDocument.Trust.SourceProcessAttempts != 0 || trust.state != bundletrust.StateAdopted {
		t.Fatalf("trust = %+v, store = %q", trustDocument.Trust, trust.state)
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
