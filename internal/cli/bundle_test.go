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
	s.state = bundletrust.StateTrusted
	return true, nil
}

type cliConfirmation struct{}

func (cliConfirmation) Confirm(context.Context, bundletrust.Summary) error { return nil }

func installTrustedBundleAuthority(command *CLI) {
	command.authority = bundleauthority.New(bundlejson.New(), sourceexec.New(), &cliTrustStore{state: bundletrust.StateTrusted}, cliConfirmation{})
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
	policyPath := filepath.Join(directory, "policy.yaml")
	policy := fmt.Sprintf(`schema_version: 2
catalog_digest: %s
rules:
  - command: [item, list]
    visibility: visible
    effect: read
    decision: allow
    reason: Return a compact inventory.
    append_args: ["--json=id,name"]
    output:
      input: json
      select: [id, name]
      rename: []
      render: compact_json
`, digest)
	if err := os.WriteFile(policyPath, []byte(policy), 0o600); err != nil {
		t.Fatal(err)
	}
	return catalogPath, policyPath
}

func bundleArtifactPath(t *testing.T, catalogPath, policyPath string) string {
	t.Helper()
	var out, errOut bytes.Buffer
	command := New(strings.NewReader(""), &out, &errOut)
	if code := command.RunContext(context.Background(), bundleCommandArgs("bundle build", catalogPath, policyPath)); code != ExitOK {
		t.Fatalf("bundle build code = %d, stderr = %q", code, errOut.String())
	}
	path := filepath.Join(t.TempDir(), "bundle.json")
	if err := os.WriteFile(path, out.Bytes(), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func bundleCommandArgs(path, catalog, policy string) []string {
	return []string{strings.Split(path, " ")[0], strings.Split(path, " ")[1], "--catalog", catalog, "--policy", policy}
}

func TestPolicyValidateAndBundleBuildCloseCanonicalFileWorkflow(t *testing.T) {
	catalogPath, policyPath := bundleArtifactPaths(t)
	var validationOut, validationErr bytes.Buffer
	validator := New(strings.NewReader(""), &validationOut, &validationErr)
	if code := validator.RunContext(context.Background(), bundleCommandArgs("policy validate", catalogPath, policyPath)); code != ExitOK {
		t.Fatalf("policy validate code = %d, stderr = %q", code, validationErr.String())
	}
	var validation policyValidationDocument
	if err := json.Unmarshal(validationOut.Bytes(), &validation); err != nil {
		t.Fatal(err)
	}
	if !validation.Validation.Valid || len(validation.Validation.PolicyDigest) != 64 || validation.Validation.RuleCount != 1 || validation.Validation.VisibleCount != 1 {
		t.Fatalf("validation = %+v", validation)
	}

	var buildOut, buildErr bytes.Buffer
	builder := New(strings.NewReader(""), &buildOut, &buildErr)
	if code := builder.RunContext(context.Background(), bundleCommandArgs("bundle build", catalogPath, policyPath)); code != ExitOK {
		t.Fatalf("bundle build code = %d, stderr = %q", code, buildErr.String())
	}
	var build bundleBuildDocument
	if err := json.Unmarshal(buildOut.Bytes(), &build); err != nil {
		t.Fatal(err)
	}
	digest, err := build.Build.Bundle.Digest()
	if err != nil || digest != build.Build.BundleDigest || build.Build.Bundle.PolicyDigest != validation.Validation.PolicyDigest {
		t.Fatalf("build = %+v, digest = %q, error = %v", build, digest, err)
	}
}

func TestBundleStatusAndTrustUseExactBundleWithoutSourceProcess(t *testing.T) {
	catalogPath, policyPath := bundleArtifactPaths(t)
	bundlePath := bundleArtifactPath(t, catalogPath, policyPath)
	trust := &cliTrustStore{state: bundletrust.StateUntrusted}
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
	if statusDocument.Status.Trust != bundletrust.StateUntrusted || statusDocument.Status.Source != bundletrust.SourceCurrent || statusDocument.Status.Executable || statusDocument.Status.SourceProcessAttempts != 0 {
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
	if !trustDocument.Trust.Trusted || trustDocument.Trust.AlreadyTrusted || trustDocument.Trust.SourceProcessAttempts != 0 || trust.state != bundletrust.StateTrusted {
		t.Fatalf("trust = %+v, store = %q", trustDocument.Trust, trust.state)
	}
}

func TestPolicyInitProducesValidHiddenDenyDraft(t *testing.T) {
	catalogPath, _ := bundleArtifactPaths(t)
	var out, errOut bytes.Buffer
	command := New(strings.NewReader(""), &out, &errOut)
	args := []string{"policy", "init", "--catalog", catalogPath, "--effect", "read", "--", "item", "list"}
	if code := command.RunContext(context.Background(), args); code != ExitOK {
		t.Fatalf("policy init code = %d, stderr = %q", code, errOut.String())
	}
	if !strings.Contains(out.String(), "visibility: hidden") || !strings.Contains(out.String(), "decision: deny") || strings.Contains(out.String(), "output:") {
		t.Fatalf("draft = %s", out.String())
	}
	policyPath := filepath.Join(t.TempDir(), "draft.yaml")
	if err := os.WriteFile(policyPath, out.Bytes(), 0o600); err != nil {
		t.Fatal(err)
	}
	var validateOut, validateErr bytes.Buffer
	validator := New(strings.NewReader(""), &validateOut, &validateErr)
	if code := validator.RunContext(context.Background(), bundleCommandArgs("policy validate", catalogPath, policyPath)); code != ExitOK {
		t.Fatalf("draft validation code = %d, stderr = %q", code, validateErr.String())
	}
}

func TestBundleBuildRejectsCatalogPolicyMismatch(t *testing.T) {
	catalogPath, policyPath := bundleArtifactPaths(t)
	raw, err := os.ReadFile(policyPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(policyPath, []byte(strings.Replace(string(raw), "catalog_digest: ", "catalog_digest: b", 1)), 0o600); err != nil {
		t.Fatal(err)
	}
	var out, errOut bytes.Buffer
	command := New(strings.NewReader(""), &out, &errOut)
	if code := command.RunContext(context.Background(), bundleCommandArgs("bundle build", catalogPath, policyPath)); code != ExitUsage || out.Len() != 0 || !strings.Contains(errOut.String(), "invalid_policy") {
		t.Fatalf("code/output/error = %d/%q/%q", code, out.String(), errOut.String())
	}
}
