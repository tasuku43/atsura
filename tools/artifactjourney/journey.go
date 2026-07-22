package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"

	"github.com/tasuku43/atsura/internal/domain/bundletrust"
	"github.com/tasuku43/atsura/internal/infra/trustfile"
)

const (
	maxCommandOutputBytes = 4 * 1024 * 1024
	maxEvidenceBytes      = 8 * 1024
	maxAttemptLogBytes    = 1024 * 1024
	commandTimeout        = 40 * time.Second
	fixtureAttemptEnv     = "ATSURA_SOURCE_FIXTURE_ATTEMPT_LOG"
	fixtureModeEnv        = "ATSURA_SOURCE_FIXTURE_MODE"
)

var secretCanaries = []string{
	"ATSURA_SECRET_STDOUT_CANARY",
	"ATSURA_SECRET_STDERR_CANARY",
	"ATSURA_SECRET_UNSELECTED_CANARY",
}

type evidenceDocument struct {
	SchemaVersion   int                     `json:"schema_version"`
	ArtifactJourney artifactJourneyEvidence `json:"artifact_journey"`
}

type artifactJourneyEvidence struct {
	Target                      string   `json:"target"`
	ObservedHost                string   `json:"observed_host"`
	ArchiveName                 string   `json:"archive_name"`
	ArchiveSHA256               string   `json:"archive_sha256"`
	Version                     string   `json:"version"`
	Revision                    string   `json:"revision"`
	HelpContractsVerified       int      `json:"help_contracts_verified"`
	BundleDigest                string   `json:"bundle_digest"`
	PlanDigest                  string   `json:"plan_digest"`
	IssueBundleDigest           string   `json:"issue_bundle_digest"`
	IssuePlanDigest             string   `json:"issue_plan_digest"`
	CommandsVerified            []string `json:"commands_verified"`
	SourceInspectionAttempts    int      `json:"source_inspection_attempts"`
	ZeroAttemptRejections       int      `json:"zero_attempt_rejections"`
	PostStartFaults             []string `json:"post_start_faults"`
	FixtureAttempts             int      `json:"fixture_attempts"`
	CredentialEnvironmentAbsent bool     `json:"credential_environment_absent"`
	SecretCanariesAbsent        bool     `json:"secret_canaries_absent"`
}

type commandOutcome struct {
	stdout   []byte
	stderr   []byte
	exitCode int
}

type boundedBuffer struct {
	value    bytes.Buffer
	limit    int
	exceeded bool
}

func (b *boundedBuffer) Write(value []byte) (int, error) {
	remaining := b.limit - b.value.Len()
	if remaining > 0 {
		kept := len(value)
		if kept > remaining {
			kept = remaining
		}
		_, _ = b.value.Write(value[:kept])
	}
	if len(value) > remaining {
		b.exceeded = true
	}
	return len(value), nil
}

func (b *boundedBuffer) Bytes() []byte { return append([]byte(nil), b.value.Bytes()...) }

func verifyArtifactJourney(ctx context.Context, configuration options) (evidenceDocument, error) {
	if ctx == nil {
		return evidenceDocument{}, fmt.Errorf("context is required")
	}
	archivePath, err := prepareInputFile(configuration.archive)
	if err != nil {
		return evidenceDocument{}, fmt.Errorf("archive input is invalid")
	}
	sourcePath, err := prepareInputFile(configuration.source)
	if err != nil {
		return evidenceDocument{}, fmt.Errorf("source input is invalid")
	}
	wantedArchiveName := fmt.Sprintf("atr_%s_%s_%s.tar.gz", configuration.tag, configuration.goos, configuration.goarch)
	if configuration.goos == "windows" {
		wantedArchiveName = fmt.Sprintf("atr_%s_windows_%s.zip", configuration.tag, configuration.goarch)
	}
	if filepath.Base(archivePath) != wantedArchiveName {
		return evidenceDocument{}, fmt.Errorf("archive name does not match target identity")
	}
	digest, err := archiveDigest(archivePath)
	if err != nil {
		return evidenceDocument{}, fmt.Errorf("archive digest validation failed")
	}
	workRoot, err := os.MkdirTemp("", "atsura-artifact-journey-")
	if err != nil {
		return evidenceDocument{}, fmt.Errorf("temporary workspace creation failed")
	}
	defer os.RemoveAll(workRoot)
	executablePath, err := extractReleaseArchive(archivePath, configuration.goos, filepath.Join(workRoot, "artifact"))
	if err != nil {
		return evidenceDocument{}, fmt.Errorf("release archive extraction failed")
	}

	attemptLog := filepath.Join(workRoot, "fixture-attempts.jsonl")
	trustPath, environment, err := isolatedEnvironment(workRoot, attemptLog)
	if err != nil {
		return evidenceDocument{}, err
	}
	runner := journeyRunner{executable: executablePath, directory: workRoot, environment: environment}

	wantedVersion := fmt.Sprintf("atr %s (%s)\n", strings.TrimPrefix(configuration.tag, "v"), configuration.revision)
	version, err := runner.success(ctx, "success", "version")
	if err != nil || string(version.stdout) != wantedVersion || len(version.stderr) != 0 {
		return evidenceDocument{}, fmt.Errorf("packaged version verification failed")
	}
	helpEvidence, err := verifyPackagedHelp(ctx, runner)
	if err != nil {
		return evidenceDocument{}, err
	}
	if err := requireAttempts(attemptLog, 0); err != nil {
		return evidenceDocument{}, fmt.Errorf("packaged help started the source fixture")
	}

	catalogPath := filepath.Join(workRoot, "catalog.json")
	inspection, err := runner.success(ctx, "success", "source", "inspect", "--adapter", "github-cli", "--executable", sourcePath)
	if err != nil {
		return evidenceDocument{}, fmt.Errorf("source inspection failed")
	}
	inspectionPayload, err := decodeInspection(inspection.stdout)
	if err != nil || inspectionPayload.SourceProcessAttempts != 4 || !digestValue(inspectionPayload.CatalogDigest) {
		return evidenceDocument{}, fmt.Errorf("source inspection evidence is invalid")
	}
	if err := writePrivate(catalogPath, inspection.stdout); err != nil {
		return evidenceDocument{}, fmt.Errorf("catalog evidence write failed")
	}
	if err := requireAttempts(attemptLog, 4); err != nil {
		return evidenceDocument{}, fmt.Errorf("source inspection attempt evidence is invalid")
	}

	prJourney, err := prepareCommandJourney(ctx, runner, helpEvidence, workRoot, catalogPath, inspectionPayload.CatalogDigest, sourcePath, trustPath, attemptLog, 4, []string{"pr", "list"})
	if err != nil {
		return evidenceDocument{}, fmt.Errorf("pull-request journey preparation failed: %w", err)
	}
	zeroAttemptRejections := prJourney.zeroAttemptRejections

	for _, conflict := range []string{"--web", "--jq=.[]", "--template={{.number}}"} {
		declaration, declarationErr := helpEvidence.fault("bundle execute", "wrapper_runtime_not_supported")
		if declarationErr != nil || declaration.Kind != "unsupported" || declaration.Retryable {
			return evidenceDocument{}, fmt.Errorf("runtime conflict help contract is invalid")
		}
		arguments := append([]string{"--error-format=json", "bundle", "execute"}, prJourney.baseInvocation...)
		arguments = append(arguments, conflict)
		failure, err := runner.failure(ctx, "success", 12, declaration, arguments...)
		if err != nil {
			return evidenceDocument{}, fmt.Errorf("runtime conflict rejection evidence is invalid")
		}
		if err := scanCanaries(failure.stdout, failure.stderr); err != nil {
			return evidenceDocument{}, err
		}
		zeroAttemptRejections++
	}
	if err := requireAttempts(attemptLog, 4); err != nil {
		return evidenceDocument{}, fmt.Errorf("runtime conflict zero-attempt evidence is invalid")
	}

	postStartFaults := []struct {
		mode string
		exit int
		code string
		kind string
	}{
		{mode: "command_failure", exit: 10, code: "source_command_failed", kind: "rejected"},
		{mode: "stderr", exit: 13, code: "source_stderr_not_supported", kind: "contract"},
		{mode: "malformed", exit: 13, code: "source_json_invalid", kind: "contract"},
		{mode: "missing_field", exit: 13, code: "output_transform_failed", kind: "contract"},
	}
	faultCodes := make([]string, 0, len(postStartFaults))
	wantedAttempts := 4
	for _, test := range postStartFaults {
		declaration, declarationErr := helpEvidence.fault("bundle execute", test.code)
		if declarationErr != nil || declaration.Kind != test.kind || declaration.Retryable {
			return evidenceDocument{}, fmt.Errorf("post-start help contract is invalid")
		}
		arguments := append([]string{"--error-format=json", "bundle", "execute"}, prJourney.baseInvocation...)
		failure, err := runner.failure(ctx, test.mode, test.exit, declaration, arguments...)
		if err != nil {
			return evidenceDocument{}, fmt.Errorf("post-start fault evidence is invalid")
		}
		if err := scanCanaries(failure.stdout, failure.stderr); err != nil {
			return evidenceDocument{}, err
		}
		wantedAttempts++
		if err := requireAttempts(attemptLog, wantedAttempts); err != nil {
			return evidenceDocument{}, fmt.Errorf("post-start one-attempt evidence is invalid")
		}
		faultCodes = append(faultCodes, test.code)
	}

	prExecution, err := runner.success(ctx, "success", append([]string{"bundle", "execute"}, prJourney.baseInvocation...)...)
	if err != nil {
		return evidenceDocument{}, fmt.Errorf("successful transform execution failed")
	}
	prExecutionEvidence, err := decodeExecution(prExecution.stdout)
	if err != nil || prExecutionEvidence.PlanDigest != prJourney.planDigest || prExecutionEvidence.BundleDigest != prJourney.bundleDigest || prExecutionEvidence.SourceProcessAttempts != 1 {
		return evidenceDocument{}, fmt.Errorf("successful transform execution evidence is invalid")
	}
	if err := validateSelectedOutput(prExecutionEvidence, "pr list"); err != nil {
		return evidenceDocument{}, fmt.Errorf("selected transform output evidence is invalid")
	}
	wantedAttempts++
	if err := requireAttempts(attemptLog, wantedAttempts); err != nil {
		return evidenceDocument{}, fmt.Errorf("successful execution one-attempt evidence is invalid")
	}

	issueJourney, err := prepareCommandJourney(ctx, runner, helpEvidence, workRoot, catalogPath, inspectionPayload.CatalogDigest, sourcePath, trustPath, attemptLog, wantedAttempts, []string{"issue", "list"})
	if err != nil {
		return evidenceDocument{}, fmt.Errorf("issue journey preparation failed: %w", err)
	}
	zeroAttemptRejections += issueJourney.zeroAttemptRejections
	issueExecution, err := runner.success(ctx, "success", append([]string{"bundle", "execute"}, issueJourney.baseInvocation...)...)
	if err != nil {
		return evidenceDocument{}, fmt.Errorf("successful issue transform execution failed")
	}
	issueExecutionEvidence, err := decodeExecution(issueExecution.stdout)
	if err != nil || issueExecutionEvidence.PlanDigest != issueJourney.planDigest || issueExecutionEvidence.BundleDigest != issueJourney.bundleDigest || issueExecutionEvidence.SourceProcessAttempts != 1 {
		return evidenceDocument{}, fmt.Errorf("successful issue transform execution evidence is invalid")
	}
	if err := validateSelectedOutput(issueExecutionEvidence, "issue list"); err != nil {
		return evidenceDocument{}, fmt.Errorf("selected issue transform output evidence is invalid")
	}
	wantedAttempts++
	if err := requireAttempts(attemptLog, wantedAttempts); err != nil {
		return evidenceDocument{}, fmt.Errorf("successful issue execution one-attempt evidence is invalid")
	}
	if prJourney.bundleDigest == issueJourney.bundleDigest || prJourney.planDigest == issueJourney.planDigest {
		return evidenceDocument{}, fmt.Errorf("command-specific bundle or plan identity was not distinct")
	}

	trustBytes, err := readBoundedFile(trustPath, maxAttemptLogBytes)
	if err != nil {
		return evidenceDocument{}, fmt.Errorf("isolated receipt evidence is unreadable")
	}
	attemptBytes, err := readBoundedFile(attemptLog, maxAttemptLogBytes)
	if err != nil {
		return evidenceDocument{}, fmt.Errorf("fixture attempt evidence is unreadable")
	}
	canaryBoundaries := [][]byte{inspection.stdout, prExecution.stdout, issueExecution.stdout, trustBytes, attemptBytes}
	canaryBoundaries = append(canaryBoundaries, prJourney.boundaries...)
	canaryBoundaries = append(canaryBoundaries, issueJourney.boundaries...)
	canaryBoundaries = append(canaryBoundaries, helpEvidence.outputs...)
	if err := scanCanaries(canaryBoundaries...); err != nil {
		return evidenceDocument{}, err
	}
	if err := validateAttemptSequence(attemptBytes); err != nil {
		return evidenceDocument{}, fmt.Errorf("fixture attempt sequence is invalid")
	}

	return evidenceDocument{SchemaVersion: 1, ArtifactJourney: artifactJourneyEvidence{
		Target: configuration.goos + "/" + configuration.goarch, ObservedHost: runtime.GOOS + "/" + runtime.GOARCH,
		ArchiveName: filepath.Base(archivePath), ArchiveSHA256: digest,
		Version: strings.TrimPrefix(configuration.tag, "v"), Revision: configuration.revision, HelpContractsVerified: len(helpEvidence.outputs),
		BundleDigest: prJourney.bundleDigest, PlanDigest: prJourney.planDigest,
		IssueBundleDigest: issueJourney.bundleDigest, IssuePlanDigest: issueJourney.planDigest,
		CommandsVerified:         []string{"issue list", "pr list"},
		SourceInspectionAttempts: 4, ZeroAttemptRejections: zeroAttemptRejections,
		PostStartFaults: faultCodes, FixtureAttempts: wantedAttempts,
		CredentialEnvironmentAbsent: true, SecretCanariesAbsent: true,
	}}, nil
}

type preparedCommandJourney struct {
	bundleDigest          string
	planDigest            string
	baseInvocation        []string
	zeroAttemptRejections int
	boundaries            [][]byte
}

func prepareCommandJourney(
	ctx context.Context,
	runner journeyRunner,
	help packagedHelpEvidence,
	workRoot, catalogPath, catalogDigest, sourcePath, trustPath, attemptLog string,
	existingAttempts int,
	command []string,
) (preparedCommandJourney, error) {
	if len(command) != 2 || command[1] != "list" || (command[0] != "issue" && command[0] != "pr") {
		return preparedCommandJourney{}, fmt.Errorf("command journey is unsupported")
	}
	prefix := command[0]
	draftArguments := []string{"spec", "init", "--catalog", catalogPath, "--"}
	draftArguments = append(draftArguments, command...)
	draft, err := runner.success(ctx, "success", draftArguments...)
	if err != nil {
		return preparedCommandJourney{}, fmt.Errorf("specification draft failed")
	}
	transformedSpecification, err := transformDraft(draft.stdout, command)
	if err != nil {
		return preparedCommandJourney{}, fmt.Errorf("specification transform edit failed")
	}
	specificationPath := filepath.Join(workRoot, prefix+"-specification.yaml")
	if err := writePrivate(specificationPath, transformedSpecification); err != nil {
		return preparedCommandJourney{}, fmt.Errorf("specification evidence write failed")
	}
	validation, err := runner.success(ctx, "success", "spec", "validate", "--catalog", catalogPath, "--spec", specificationPath)
	if err != nil || validateSpecificationEvidence(validation.stdout, catalogDigest) != nil {
		return preparedCommandJourney{}, fmt.Errorf("specification validation evidence is invalid")
	}
	built, err := runner.success(ctx, "success", "bundle", "build", "--catalog", catalogPath, "--spec", specificationPath)
	if err != nil {
		return preparedCommandJourney{}, fmt.Errorf("bundle build failed")
	}
	bundleDigest, err := decodeBundleDigest(built.stdout)
	if err != nil {
		return preparedCommandJourney{}, fmt.Errorf("bundle build evidence is invalid")
	}
	bundlePath := filepath.Join(workRoot, prefix+"-bundle.json")
	if err := writePrivate(bundlePath, built.stdout); err != nil {
		return preparedCommandJourney{}, fmt.Errorf("bundle evidence write failed")
	}

	preAdoptionStatus, err := runner.success(ctx, "success", "bundle", "status", "--bundle", bundlePath)
	if err != nil || validateStatus(preAdoptionStatus.stdout, bundleDigest, bundletrust.StateNotAdopted) != nil {
		return preparedCommandJourney{}, fmt.Errorf("pre-adoption status evidence is invalid")
	}
	baseInvocation := []string{"--bundle", bundlePath, "--", sourcePath}
	baseInvocation = append(baseInvocation, command...)
	baseInvocation = append(baseInvocation, "--limit=1")
	zeroAttemptRejections := 0
	for _, bundleCommand := range []string{"preview", "execute"} {
		helpPath := "bundle " + bundleCommand
		declaration, declarationErr := help.fault(helpPath, "bundle_not_adopted")
		if declarationErr != nil || declaration.Kind != "rejected" || declaration.Retryable {
			return preparedCommandJourney{}, fmt.Errorf("pre-adoption help contract is invalid")
		}
		arguments := append([]string{"--error-format=json", "bundle", bundleCommand}, baseInvocation...)
		failure, failureErr := runner.failure(ctx, "success", 10, declaration, arguments...)
		if failureErr != nil {
			return preparedCommandJourney{}, fmt.Errorf("pre-adoption rejection evidence is invalid")
		}
		if err := scanCanaries(failure.stdout, failure.stderr); err != nil {
			return preparedCommandJourney{}, err
		}
		zeroAttemptRejections++
	}
	if err := requireAttempts(attemptLog, existingAttempts); err != nil {
		return preparedCommandJourney{}, fmt.Errorf("pre-adoption zero-attempt evidence is invalid")
	}

	store := trustfile.New(trustPath)
	changed, err := store.Add(ctx, bundleDigest)
	if err != nil || !changed || store.Inspect(ctx, bundleDigest) != bundletrust.StateAdopted {
		return preparedCommandJourney{}, fmt.Errorf("isolated exact receipt seeding failed")
	}
	adoptedStatus, err := runner.success(ctx, "success", "bundle", "status", "--bundle", bundlePath)
	if err != nil || validateStatus(adoptedStatus.stdout, bundleDigest, bundletrust.StateAdopted) != nil {
		return preparedCommandJourney{}, fmt.Errorf("adopted status evidence is invalid")
	}

	preview, err := runner.success(ctx, "success", append([]string{"bundle", "preview"}, baseInvocation...)...)
	if err != nil {
		return preparedCommandJourney{}, fmt.Errorf("adopted preview failed")
	}
	previewEvidence, err := decodePreview(preview.stdout)
	if err != nil || previewEvidence.SourceProcessAttempts != 0 || !digestValue(previewEvidence.PlanDigest) {
		return preparedCommandJourney{}, fmt.Errorf("adopted preview evidence is invalid")
	}
	if err := requireAttempts(attemptLog, existingAttempts); err != nil {
		return preparedCommandJourney{}, fmt.Errorf("preview zero-attempt evidence is invalid")
	}

	return preparedCommandJourney{
		bundleDigest: bundleDigest, planDigest: previewEvidence.PlanDigest,
		baseInvocation: baseInvocation, zeroAttemptRejections: zeroAttemptRejections,
		boundaries: [][]byte{draft.stdout, transformedSpecification, validation.stdout, built.stdout, preAdoptionStatus.stdout, adoptedStatus.stdout, preview.stdout},
	}, nil
}

type helpSchemaProjection struct {
	ID      string                      `json:"id"`
	Version int                         `json:"version"`
	Fields  []helpSchemaFieldProjection `json:"fields"`
}

type helpSchemaFieldProjection struct {
	Path        string `json:"path"`
	Type        string `json:"type"`
	ElementType string `json:"element_type,omitempty"`
	Required    bool   `json:"required"`
	Nullable    bool   `json:"nullable"`
}

type helpNextAction struct {
	Command string `json:"command"`
	Reason  string `json:"reason"`
}

type helpFaultDeclaration struct {
	Code        string           `json:"code"`
	Kind        string           `json:"kind"`
	Retryable   bool             `json:"retryable"`
	NextActions []helpNextAction `json:"next_actions"`
}

func expectedHelpFault(code, kind string, retryable bool, command, reason string) helpFaultDeclaration {
	return helpFaultDeclaration{
		Code: code, Kind: kind, Retryable: retryable,
		NextActions: []helpNextAction{{Command: command, Reason: reason}},
	}
}

var bundlePreviewHelpFaults = []helpFaultDeclaration{
	expectedHelpFault("invalid_arguments", "invalid_input", false, "help bundle preview", "Pass one bundle path, the exact source executable, and at least one source argv element after --."),
	expectedHelpFault("bundle_file_not_found", "not_found", false, "bundle build", "Build and select a canonical bundle document."),
	expectedHelpFault("bundle_file_permission_denied", "permission", false, "bundle status", "Correct bundle file permissions."),
	expectedHelpFault("unsafe_bundle_file", "invalid_input", false, "bundle build", "Use a stable regular bundle file."),
	expectedHelpFault("bundle_file_too_large", "invalid_input", false, "bundle build", "Build a bundle within the 2 MiB limit."),
	expectedHelpFault("bundle_file_read_failed", "unavailable", true, "bundle status", "Retry after the bundle file is readable."),
	expectedHelpFault("invalid_bundle_file", "invalid_input", false, "bundle build", "Rebuild and review strict canonical bundle JSON."),
	expectedHelpFault("legacy_tailoring_schema", "invalid_input", false, "help bundle build", "Rebuild with a schema-3 specification and bundle schema 2."),
	expectedHelpFault("bundle_digest_mismatch", "rejected", false, "bundle build", "Rebuild and review the changed bundle content."),
	expectedHelpFault("invalid_bundle_trust_store", "rejected", false, "bundle status", "Repair or reconcile invalid user-local adoption state."),
	expectedHelpFault("bundle_not_adopted", "rejected", false, "bundle trust", "Review and adopt the exact bundle digest before previewing a plan."),
	expectedHelpFault("bundle_source_drift", "rejected", false, "bundle status", "Rebuild and adopt current source evidence before previewing a plan."),
	expectedHelpFault("source_executable_not_found", "not_found", false, "bundle status", "Reconcile the missing bundle-bound source executable."),
	expectedHelpFault("source_identity_unavailable", "unavailable", true, "bundle status", "Retry after the bundle-bound source identity can be read."),
	expectedHelpFault("unsafe_source_executable", "invalid_input", false, "bundle status", "Select and inspect a supported regular source executable."),
	expectedHelpFault("source_identity_changed", "rejected", false, "bundle status", "Rebuild from stable current source identity evidence."),
	expectedHelpFault("invalid_source_identity", "contract", false, "bundle status", "Repair invalid source identity evidence."),
	expectedHelpFault("source_executable_mismatch", "invalid_input", false, "help bundle preview", "Use the exact requested executable or resolved path recorded in the bundle."),
	expectedHelpFault("invalid_invocation", "invalid_input", false, "help bundle preview", "Use a cataloged command path and deterministic observed long-option grammar."),
	expectedHelpFault("command_not_in_surface", "not_found", false, "help bundle preview", "Select a command present in the compiled tailored surface."),
	expectedHelpFault("option_not_in_surface", "not_found", false, "help bundle preview", "Use only options present in the matched command's tailored option surface."),
	expectedHelpFault("invalid_wrapper_plan", "contract", false, "help bundle preview", "Repair the bundle or plan constructor so it produces one complete typed plan."),
	expectedHelpFault("output_contract_exceeded", "contract", false, "help bundle preview", "Reduce the bounded invocation and plan output."),
	expectedHelpFault("output_encoding_failed", "contract", false, "help bundle preview", "Repair deterministic schema-2 preview JSON."),
	expectedHelpFault("internal_error", "internal", false, "bundle status", "Inspect bundle, adoption, source identity, and plan wiring."),
	expectedHelpFault("output_write_failed", "internal", true, "bundle preview", "Retry with a writable output stream."),
	expectedHelpFault("operation_canceled", "canceled", true, "bundle preview", "Retry when the caller is ready."),
}

var bundleExecuteHelpFaults = []helpFaultDeclaration{
	expectedHelpFault("invalid_arguments", "invalid_input", false, "help bundle execute", "Pass one bundle path, the exact source executable, and at least one source argv element after --."),
	expectedHelpFault("bundle_file_not_found", "not_found", false, "bundle build", "Build and select a canonical bundle document."),
	expectedHelpFault("bundle_file_permission_denied", "permission", false, "bundle status", "Correct bundle file permissions."),
	expectedHelpFault("unsafe_bundle_file", "invalid_input", false, "bundle build", "Use a stable regular bundle file."),
	expectedHelpFault("bundle_file_too_large", "invalid_input", false, "bundle build", "Build a bundle within the 2 MiB limit."),
	expectedHelpFault("bundle_file_read_failed", "unavailable", true, "bundle status", "Retry after the bundle file is readable."),
	expectedHelpFault("invalid_bundle_file", "invalid_input", false, "bundle build", "Rebuild and review strict canonical bundle JSON."),
	expectedHelpFault("legacy_tailoring_schema", "invalid_input", false, "help bundle build", "Rebuild with a schema-3 specification and bundle schema 2."),
	expectedHelpFault("bundle_digest_mismatch", "rejected", false, "bundle build", "Rebuild and review the changed bundle content."),
	expectedHelpFault("invalid_bundle_trust_store", "rejected", false, "bundle status", "Repair or reconcile invalid user-local adoption state."),
	expectedHelpFault("bundle_not_adopted", "rejected", false, "bundle trust", "Review and adopt the exact bundle digest before execution."),
	expectedHelpFault("bundle_source_drift", "rejected", false, "bundle status", "Rebuild and adopt current source evidence before execution."),
	expectedHelpFault("source_executable_not_found", "not_found", false, "bundle status", "Reconcile the missing bundle-bound source executable."),
	expectedHelpFault("source_identity_unavailable", "unavailable", true, "bundle status", "Retry after the bundle-bound source identity can be read."),
	expectedHelpFault("unsafe_source_executable", "invalid_input", false, "bundle status", "Select and inspect a supported regular source executable."),
	expectedHelpFault("source_identity_changed", "rejected", false, "bundle status", "Rebuild from stable current source identity evidence; do not replay a started operation."),
	expectedHelpFault("invalid_source_identity", "contract", false, "bundle status", "Repair invalid source identity evidence."),
	expectedHelpFault("source_executable_mismatch", "invalid_input", false, "help bundle execute", "Use the exact requested executable or resolved path recorded in the bundle."),
	expectedHelpFault("invalid_invocation", "invalid_input", false, "help bundle execute", "Use a cataloged command path and deterministic observed long-option grammar."),
	expectedHelpFault("command_not_in_surface", "not_found", false, "help bundle execute", "Select a command present in the compiled tailored surface."),
	expectedHelpFault("option_not_in_surface", "not_found", false, "help bundle execute", "Use only options present in the matched command's tailored option surface."),
	expectedHelpFault("invalid_wrapper_plan", "contract", false, "bundle preview", "Inspect the fresh plan and repair incomplete wrapper construction."),
	expectedHelpFault("wrapper_runtime_not_supported", "unsupported", false, "help bundle execute", "Use a transform wrapper and source adapter contract with accepted JSON selector behavior."),
	expectedHelpFault("invalid_source_process_request", "contract", false, "bundle preview", "Inspect the exact plan-derived source request before execution."),
	expectedHelpFault("source_process_start_failed", "unavailable", true, "bundle execute", "Retry the same invocation only when the result proves no source process started."),
	expectedHelpFault("source_stdout_too_large", "contract", false, "help bundle execute", "Reduce source output within the declared bound; the source was not retried."),
	expectedHelpFault("source_stderr_too_large", "contract", false, "help bundle execute", "Reduce source stderr within the declared bound; the source was not retried."),
	expectedHelpFault("source_execution_canceled", "canceled", false, "bundle status", "Reconcile source-owned effects before considering another invocation."),
	expectedHelpFault("source_command_timeout", "unavailable", false, "bundle status", "Reconcile source-owned effects after the timed-out attempt."),
	expectedHelpFault("source_command_failed", "rejected", false, "help bundle execute", "Inspect the source command independently; Atsura does not expose raw failure output or retry it."),
	expectedHelpFault("source_process_wait_failed", "unavailable", false, "bundle status", "Reconcile source-owned effects after the unclassified wait outcome."),
	expectedHelpFault("source_stderr_not_supported", "contract", false, "help bundle execute", "Use a successful source invocation with empty stderr for this initial transform runtime."),
	expectedHelpFault("source_output_processing_canceled", "canceled", false, "bundle status", "The source already ran; reconcile before considering another invocation."),
	expectedHelpFault("source_json_invalid", "contract", false, "bundle preview", "Repair the source output selector or adapter contract; raw output is not a fallback."),
	expectedHelpFault("output_transform_failed", "contract", false, "bundle preview", "Repair selected fields and typed transform expectations; raw output is not a fallback."),
	expectedHelpFault("unclassified_source_execution_outcome", "contract", false, "bundle status", "Reconcile source-owned effects before considering another invocation."),
	expectedHelpFault("output_contract_exceeded", "contract", false, "bundle preview", "Reduce the bounded transformed result; the source was not retried."),
	expectedHelpFault("output_encoding_failed", "contract", false, "bundle preview", "Repair deterministic schema-2 execution JSON; the source was not retried."),
	expectedHelpFault("internal_error", "internal", false, "bundle status", "Inspect bundle execution wiring without replaying the source."),
	expectedHelpFault("execute_output_write_failed", "internal", false, "bundle status", "The source completed; reconcile before considering another invocation."),
	expectedHelpFault("operation_canceled", "canceled", true, "bundle execute", "Retry only because cancellation occurred before a source attempt."),
}

func validateHelpFaultMatrix(got, wanted []helpFaultDeclaration) error {
	if len(got) != len(wanted) {
		return fmt.Errorf("fault inventory length is invalid")
	}
	seen := make(map[string]struct{}, len(got))
	for index := range wanted {
		actual := got[index]
		expected := wanted[index]
		if _, exists := seen[actual.Code]; exists {
			return fmt.Errorf("fault %q is duplicated", actual.Code)
		}
		seen[actual.Code] = struct{}{}
		if actual.Code != expected.Code || actual.Kind != expected.Kind || actual.Retryable != expected.Retryable || len(actual.NextActions) != len(expected.NextActions) {
			return fmt.Errorf("fault signature %d is invalid", index)
		}
		for actionIndex := range expected.NextActions {
			if actual.NextActions[actionIndex] != expected.NextActions[actionIndex] {
				return fmt.Errorf("fault recovery %d/%d is invalid", index, actionIndex)
			}
		}
	}
	return nil
}

type helpOutputFieldProjection struct {
	Name   string                `json:"name"`
	Type   string                `json:"type"`
	Schema *helpSchemaProjection `json:"schema"`
}

type helpInputProjection struct {
	Name          string   `json:"name"`
	Source        string   `json:"source"`
	Required      bool     `json:"required"`
	ValueKind     string   `json:"value_kind"`
	Cardinality   string   `json:"cardinality"`
	AllowedValues []string `json:"allowed_values"`
}

type helpCommandProjection struct {
	Path     string `json:"path"`
	Summary  string `json:"summary"`
	Usage    string `json:"usage"`
	Contract struct {
		Outcome string                `json:"outcome"`
		Inputs  []helpInputProjection `json:"inputs"`
		Output  struct {
			Fields []helpOutputFieldProjection `json:"fields"`
		} `json:"output"`
		Prerequisites []string               `json:"prerequisites"`
		Errors        []helpFaultDeclaration `json:"errors"`
	} `json:"contract"`
}

func input(name, source, cardinality string, allowed ...string) helpInputProjection {
	if allowed == nil {
		allowed = []string{}
	}
	return helpInputProjection{Name: name, Source: source, Required: true, ValueKind: "text", Cardinality: cardinality, AllowedValues: allowed}
}

type packagedHelpEvidence struct {
	outputs  [][]byte
	commands map[string]helpCommandProjection
}

func schemaField(path, fieldType string, required bool) helpSchemaFieldProjection {
	return helpSchemaFieldProjection{Path: path, Type: fieldType, Required: required}
}

func schemaArray(path, elementType string) helpSchemaFieldProjection {
	return helpSchemaFieldProjection{Path: path, Type: "array", ElementType: elementType, Required: true}
}

var sourceCatalogSchemaFields = []helpSchemaFieldProjection{
	schemaField("/adapter", "object", true),
	schemaField("/adapter/contract_version", "integer", true),
	schemaField("/adapter/kind", "string", true),
	schemaArray("/commands", "object"),
	schemaArray("/commands/*/options", "object"),
	schemaField("/commands/*/options/*/name", "string", true),
	schemaField("/commands/*/options/*/takes_value", "boolean", true),
	schemaArray("/commands/*/path", "string"),
	schemaField("/commands/*/provenance", "string", true),
	schemaArray("/commands/*/structured_output", "object"),
	schemaArray("/commands/*/structured_output/*/fields", "string"),
	schemaField("/commands/*/structured_output/*/format", "string", true),
	schemaField("/commands/*/structured_output/*/selector_flag", "string", true),
	schemaField("/commands/*/summary", "string", true),
	schemaField("/probe", "object", true),
	schemaField("/probe/attempts", "integer", true),
	schemaArray("/probe/ids", "string"),
	schemaField("/schema_version", "integer", true),
	schemaField("/source", "object", true),
	schemaField("/source/requested_executable", "string", true),
	schemaField("/source/resolved_path", "string", true),
	schemaField("/source/sha256", "string", true),
	schemaField("/source/size", "integer", true),
	schemaField("/source/version", "string", true),
}

var tailoringSpecificationSchemaFields = []helpSchemaFieldProjection{
	schemaField("/catalog_digest", "string", true),
	schemaArray("/commands", "object"),
	schemaArray("/commands/*/command", "string"),
	schemaField("/commands/*/options", "object", false),
	schemaField("/commands/*/options/default", "string", true),
	schemaArray("/commands/*/options/exclude", "string"),
	schemaArray("/commands/*/options/include", "string"),
	schemaField("/commands/*/presence", "string", true),
	schemaField("/commands/*/reason", "string", true),
	schemaField("/commands/*/wrapper", "object", false),
	schemaArray("/commands/*/wrapper/after", "object"),
	schemaArray("/commands/*/wrapper/before", "object"),
	schemaField("/commands/*/wrapper/invoke", "object", true),
	schemaArray("/commands/*/wrapper/invoke/append_args", "string"),
	schemaField("/commands/*/wrapper/kind", "string", true),
	schemaField("/commands/*/wrapper/output", "object", false),
	schemaField("/commands/*/wrapper/output/input", "string", true),
	schemaArray("/commands/*/wrapper/output/rename", "object"),
	schemaField("/commands/*/wrapper/output/rename/*/from", "string", true),
	schemaField("/commands/*/wrapper/output/rename/*/to", "string", true),
	schemaField("/commands/*/wrapper/output/render", "string", true),
	schemaArray("/commands/*/wrapper/output/select", "string"),
	schemaField("/schema_version", "integer", true),
	schemaField("/surface", "object", true),
	schemaField("/surface/default", "string", true),
}

func (h packagedHelpEvidence) fault(path, code string) (helpFaultDeclaration, error) {
	command, present := h.commands[path]
	if !present {
		return helpFaultDeclaration{}, fmt.Errorf("packaged help command is missing")
	}
	var result helpFaultDeclaration
	found := false
	for _, declaration := range command.Contract.Errors {
		if declaration.Code != code {
			continue
		}
		if found {
			return helpFaultDeclaration{}, fmt.Errorf("packaged help fault is duplicated")
		}
		result = declaration
		found = true
	}
	if !found || result.Kind == "" || len(result.NextActions) == 0 {
		return helpFaultDeclaration{}, fmt.Errorf("packaged help fault is incomplete")
	}
	for _, action := range result.NextActions {
		if action.Command == "" || action.Reason == "" {
			return helpFaultDeclaration{}, fmt.Errorf("packaged help recovery is incomplete")
		}
	}
	return result, nil
}

func validateOutputSchema(command helpCommandProjection, fieldName, schemaID string, version int, wanted []helpSchemaFieldProjection) error {
	var schema *helpSchemaProjection
	found := false
	for _, field := range command.Contract.Output.Fields {
		if field.Name != fieldName {
			continue
		}
		if found || field.Type != "object" || field.Schema == nil {
			return fmt.Errorf("structured output field is invalid")
		}
		schema = field.Schema
		found = true
	}
	if !found || schema.ID != schemaID || schema.Version != version || len(schema.Fields) != len(wanted) {
		return fmt.Errorf("structured output schema identity is invalid")
	}
	for index := range wanted {
		got := schema.Fields[index]
		expected := wanted[index]
		if got.Path != expected.Path || got.Type != expected.Type || got.ElementType != expected.ElementType || got.Required != expected.Required || got.Nullable != expected.Nullable {
			return fmt.Errorf("structured output schema field %d is invalid", index)
		}
	}
	return nil
}

func validateInputs(got, wanted []helpInputProjection) error {
	if len(got) != len(wanted) {
		return fmt.Errorf("input inventory length is invalid")
	}
	for index := range wanted {
		if got[index].Name != wanted[index].Name || got[index].Source != wanted[index].Source || got[index].Required != wanted[index].Required || got[index].ValueKind != wanted[index].ValueKind || got[index].Cardinality != wanted[index].Cardinality || strings.Join(got[index].AllowedValues, "\x00") != strings.Join(wanted[index].AllowedValues, "\x00") {
			return fmt.Errorf("input inventory field %d is invalid", index)
		}
	}
	return nil
}

func verifyPackagedHelp(ctx context.Context, runner journeyRunner) (packagedHelpEvidence, error) {
	root, err := runner.success(ctx, "success", "help", "--format", "agent")
	if err != nil {
		return packagedHelpEvidence{}, fmt.Errorf("packaged root help verification failed")
	}
	wanted := []string{"source inspect", "spec init", "spec validate", "bundle preview", "bundle execute"}
	if err := validateRootHelp(root.stdout, wanted); err != nil {
		return packagedHelpEvidence{}, fmt.Errorf("packaged root help contract is invalid")
	}
	result := packagedHelpEvidence{outputs: [][]byte{root.stdout}, commands: make(map[string]helpCommandProjection, len(wanted))}
	for _, path := range wanted {
		arguments := append([]string{"help"}, strings.Split(path, " ")...)
		arguments = append(arguments, "--format", "agent")
		output, runErr := runner.success(ctx, "success", arguments...)
		command, validationErr := validateScopedHelp(path, output.stdout)
		if runErr != nil || validationErr != nil {
			return packagedHelpEvidence{}, fmt.Errorf("packaged scoped help contract is invalid for %s", path)
		}
		var faultErr error
		switch path {
		case "bundle preview":
			faultErr = validateHelpFaultMatrix(command.Contract.Errors, bundlePreviewHelpFaults)
		case "bundle execute":
			faultErr = validateHelpFaultMatrix(command.Contract.Errors, bundleExecuteHelpFaults)
		}
		if faultErr != nil {
			return packagedHelpEvidence{}, fmt.Errorf("packaged scoped help fault contract is invalid for %s: %w", path, faultErr)
		}
		result.outputs = append(result.outputs, output.stdout)
		result.commands[path] = command
	}
	return result, nil
}

func validateRootHelp(value []byte, wanted []string) error {
	var document struct {
		SchemaVersion int    `json:"schema_version"`
		View          string `json:"view"`
		Program       string `json:"program"`
		Commands      []struct {
			Path string `json:"path"`
		} `json:"commands"`
	}
	if err := json.Unmarshal(value, &document); err != nil || document.SchemaVersion != 9 || document.View != "index" || document.Program != "atr" {
		return fmt.Errorf("invalid root agent help")
	}
	paths := make(map[string]struct{}, len(document.Commands))
	for _, command := range document.Commands {
		paths[command.Path] = struct{}{}
	}
	for _, path := range wanted {
		if _, present := paths[path]; !present {
			return fmt.Errorf("root agent help is missing %s", path)
		}
	}
	return nil
}

func validateScopedHelp(path string, value []byte) (helpCommandProjection, error) {
	var document struct {
		SchemaVersion int    `json:"schema_version"`
		View          string `json:"view"`
		Program       string `json:"program"`
		Scope         struct {
			Selector string `json:"selector"`
			Kind     string `json:"kind"`
		} `json:"scope"`
		Commands []helpCommandProjection `json:"commands"`
	}
	if err := json.Unmarshal(value, &document); err != nil || document.SchemaVersion != 9 || document.View != "scope" || document.Program != "atr" || document.Scope.Selector != path || document.Scope.Kind != "command" || len(document.Commands) != 1 || document.Commands[0].Path != path {
		return helpCommandProjection{}, fmt.Errorf("invalid scoped agent help")
	}
	command := document.Commands[0]
	prerequisites := strings.Join(command.Contract.Prerequisites, "\n")
	switch path {
	case "source inspect":
		if command.Usage != "atr source inspect --adapter=github-cli --executable <path-or-name>" || validateInputs(command.Contract.Inputs, []helpInputProjection{input("--adapter", "flag", "single", "github-cli"), input("--executable", "flag", "single")}) != nil {
			return helpCommandProjection{}, fmt.Errorf("source inspection invocation contract is incomplete")
		}
		if err := validateOutputSchema(command, "catalog", "source-command-catalog", 1, sourceCatalogSchemaFields); err != nil {
			return helpCommandProjection{}, fmt.Errorf("source catalog schema is incomplete")
		}
	case "spec init":
		if command.Usage != "atr spec init --catalog <path> -- <command>" || validateInputs(command.Contract.Inputs, []helpInputProjection{input("--catalog", "flag", "single"), input("command", "argument", "repeatable")}) != nil || !strings.Contains(command.Summary, "authoring baseline") || !strings.Contains(command.Contract.Outcome, "identity wrapper") {
			return helpCommandProjection{}, fmt.Errorf("specification baseline contract is incomplete")
		}
		if err := validateOutputSchema(command, "specification", "tailoring-specification", 3, tailoringSpecificationSchemaFields); err != nil {
			return helpCommandProjection{}, fmt.Errorf("specification schema is incomplete")
		}
		for _, marker := range []string{"kind=transform", "output.select", "output.rename", "output.render=compact_json", "Shell, script, jq, plugin, RTK"} {
			if !strings.Contains(prerequisites, marker) {
				return helpCommandProjection{}, fmt.Errorf("specification authoring marker is missing")
			}
		}
	case "spec validate":
		if command.Usage != "atr spec validate --catalog <path> --spec <path>" || validateInputs(command.Contract.Inputs, []helpInputProjection{input("--catalog", "flag", "single"), input("--spec", "flag", "single")}) != nil {
			return helpCommandProjection{}, fmt.Errorf("specification validation invocation contract is incomplete")
		}
		if err := validateOutputSchema(command, "specification", "tailoring-specification", 3, tailoringSpecificationSchemaFields); err != nil {
			return helpCommandProjection{}, fmt.Errorf("normalized specification schema is incomplete")
		}
	case "bundle preview":
		if command.Usage != "atr bundle preview --bundle <path> -- <source-executable> <argv>" || validateInputs(command.Contract.Inputs, []helpInputProjection{input("--bundle", "flag", "single"), input("source-executable", "argument", "single"), input("argv", "argument", "repeatable")}) != nil || !strings.Contains(prerequisites, "never treats adoption as source authorization") {
			return helpCommandProjection{}, fmt.Errorf("preview contract is incomplete")
		}
	case "bundle execute":
		if command.Usage != "atr bundle execute --bundle <path> -- <source-executable> <argv>" || validateInputs(command.Contract.Inputs, []helpInputProjection{input("--bundle", "flag", "single"), input("source-executable", "argument", "single"), input("argv", "argument", "repeatable")}) != nil {
			return helpCommandProjection{}, fmt.Errorf("runtime invocation contract is incomplete")
		}
		for _, marker := range []string{"atsura.source.github_cli contract 2", "issue list or pr list", "--json=<ordered-select>", "--jq, --template, or --web", "source-owned authentication", "Successful source stderr must be empty"} {
			if !strings.Contains(prerequisites, marker) {
				return helpCommandProjection{}, fmt.Errorf("runtime admission marker is missing")
			}
		}
	default:
		return helpCommandProjection{}, fmt.Errorf("unsupported help contract")
	}
	if len(command.Contract.Errors) == 0 {
		return helpCommandProjection{}, fmt.Errorf("fault contract is missing")
	}
	return command, nil
}

type journeyRunner struct {
	executable  string
	directory   string
	environment []string
}

func (r journeyRunner) success(ctx context.Context, mode string, arguments ...string) (commandOutcome, error) {
	outcome, err := r.command(ctx, mode, arguments...)
	if err != nil || outcome.exitCode != 0 || len(outcome.stderr) != 0 {
		return commandOutcome{}, fmt.Errorf("command did not succeed cleanly")
	}
	if err := scanCanaries(outcome.stdout, outcome.stderr); err != nil {
		return commandOutcome{}, err
	}
	return outcome, nil
}

func (r journeyRunner) failure(ctx context.Context, mode string, exit int, declaration helpFaultDeclaration, arguments ...string) (commandOutcome, error) {
	outcome, err := r.command(ctx, mode, arguments...)
	if err != nil || outcome.exitCode != exit || len(outcome.stdout) != 0 {
		return commandOutcome{}, fmt.Errorf("command did not produce the expected failure boundary")
	}
	if err := validateFault(outcome.stderr, declaration); err != nil {
		return commandOutcome{}, err
	}
	return outcome, nil
}

func (r journeyRunner) command(ctx context.Context, mode string, arguments ...string) (commandOutcome, error) {
	runContext, cancel := context.WithTimeout(ctx, commandTimeout)
	defer cancel()
	// #nosec G204 -- the exact extracted atr path is a validated regular archive member; argv is the finite harness grammar and no shell is used.
	command := exec.CommandContext(runContext, r.executable, arguments...)
	command.Dir = r.directory
	command.Env = replaceEnvironment(r.environment, map[string]string{fixtureModeEnv: mode})
	command.Stdin = nil
	command.WaitDelay = 2 * time.Second
	stdout := &boundedBuffer{limit: maxCommandOutputBytes}
	stderr := &boundedBuffer{limit: maxCommandOutputBytes}
	command.Stdout = stdout
	command.Stderr = stderr
	err := command.Run()
	outcome := commandOutcome{stdout: stdout.Bytes(), stderr: stderr.Bytes(), exitCode: -1}
	if stdout.exceeded || stderr.exceeded {
		return commandOutcome{}, fmt.Errorf("command output exceeded the journey bound")
	}
	if runContext.Err() != nil {
		return commandOutcome{}, fmt.Errorf("command exceeded the journey timeout")
	}
	if command.ProcessState != nil {
		outcome.exitCode = command.ProcessState.ExitCode()
	}
	if err == nil {
		return outcome, nil
	}
	var exitError *exec.ExitError
	if errors.As(err, &exitError) {
		return outcome, nil
	}
	return commandOutcome{}, fmt.Errorf("command could not be started or collected")
}

func isolatedEnvironment(workRoot, attemptLog string) (string, []string, error) {
	home := filepath.Join(workRoot, "home")
	config := filepath.Join(workRoot, "config")
	if runtime.GOOS == "darwin" {
		config = filepath.Join(home, "Library", "Application Support")
	}
	if err := os.MkdirAll(home, 0o700); err != nil {
		return "", nil, fmt.Errorf("isolated home creation failed")
	}
	if err := os.MkdirAll(config, 0o700); err != nil {
		return "", nil, fmt.Errorf("isolated config creation failed")
	}
	environment := replaceEnvironment(minimalChildEnvironment(os.Environ()), map[string]string{
		"HOME": home, "USERPROFILE": home, "XDG_CONFIG_HOME": config,
		"APPDATA": config, "LOCALAPPDATA": config,
		fixtureAttemptEnv: attemptLog, fixtureModeEnv: "success",
	})
	return filepath.Join(config, "atsura", "bundle-trust.json"), environment, nil
}

func minimalChildEnvironment(base []string) []string {
	allowed := map[string]struct{}{
		"COMSPEC": {}, "PATHEXT": {}, "SYSTEMROOT": {}, "TEMP": {},
		"TMP": {}, "TMPDIR": {}, "WINDIR": {},
	}
	result := make([]string, 0, len(allowed))
	for _, item := range base {
		key, _, present := strings.Cut(item, "=")
		if !present {
			continue
		}
		if _, keep := allowed[strings.ToUpper(key)]; keep {
			result = append(result, item)
		}
	}
	return result
}

func replaceEnvironment(base []string, replacements map[string]string) []string {
	keys := make(map[string]struct{}, len(replacements))
	for key := range replacements {
		keys[strings.ToUpper(key)] = struct{}{}
	}
	result := make([]string, 0, len(base)+len(replacements))
	for _, item := range base {
		key, _, present := strings.Cut(item, "=")
		if _, replaced := keys[strings.ToUpper(key)]; present && replaced {
			continue
		}
		result = append(result, item)
	}
	names := make([]string, 0, len(replacements))
	for key := range replacements {
		names = append(names, key)
	}
	sort.Strings(names)
	for _, key := range names {
		result = append(result, key+"="+replacements[key])
	}
	return result
}

func writePrivate(path string, value []byte) error {
	root, name, err := openParentRoot(path)
	if err != nil {
		return err
	}
	defer root.Close()
	file, err := root.OpenFile(name, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return err
	}
	written, writeErr := file.Write(value)
	closeErr := file.Close()
	if writeErr != nil || written != len(value) {
		return fmt.Errorf("short write")
	}
	return closeErr
}

func readBoundedFile(path string, limit int64) ([]byte, error) {
	root, name, err := openParentRoot(path)
	if err != nil {
		return nil, err
	}
	defer root.Close()
	file, err := root.Open(name)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	value, err := io.ReadAll(io.LimitReader(file, limit+1))
	if err != nil || int64(len(value)) > limit {
		return nil, fmt.Errorf("file exceeds bound")
	}
	return value, nil
}

func openParentRoot(path string) (*os.Root, string, error) {
	if !filepath.IsAbs(path) || filepath.Clean(path) != path {
		return nil, "", fmt.Errorf("path is not absolute and clean")
	}
	parent, name := filepath.Split(path)
	if name == "" || filepath.Base(name) != name {
		return nil, "", fmt.Errorf("path does not name a file")
	}
	root, err := os.OpenRoot(parent)
	if err != nil {
		return nil, "", err
	}
	return root, name, nil
}

func requireAttempts(path string, wanted int) error {
	value, err := readBoundedFile(path, maxAttemptLogBytes)
	if errors.Is(err, os.ErrNotExist) && wanted == 0 {
		return nil
	}
	if err != nil {
		return err
	}
	scanner := bufio.NewScanner(bytes.NewReader(value))
	scanner.Buffer(make([]byte, 4096), maxAttemptLogBytes)
	count := 0
	for scanner.Scan() {
		line := bytes.TrimSpace(scanner.Bytes())
		if len(line) == 0 || !json.Valid(line) || line[0] != '{' {
			return fmt.Errorf("attempt log is not object JSONL")
		}
		count++
	}
	if err := scanner.Err(); err != nil || count != wanted {
		return fmt.Errorf("attempt count mismatch")
	}
	return nil
}

type fixtureAttemptRecord struct {
	SchemaVersion int      `json:"schema_version"`
	Kind          string   `json:"kind"`
	Mode          string   `json:"mode"`
	Argv          []string `json:"argv"`
}

func validateAttemptSequence(value []byte) error {
	expected := []fixtureAttemptRecord{
		{SchemaVersion: 1, Kind: "probe", Mode: "success", Argv: []string{"version"}},
		{SchemaVersion: 1, Kind: "probe", Mode: "success", Argv: []string{"help", "reference"}},
		{SchemaVersion: 1, Kind: "probe", Mode: "success", Argv: []string{"issue", "list", "--help"}},
		{SchemaVersion: 1, Kind: "probe", Mode: "success", Argv: []string{"pr", "list", "--help"}},
	}
	for _, mode := range []string{"command_failure", "stderr", "malformed", "missing_field", "success"} {
		expected = append(expected, fixtureAttemptRecord{
			SchemaVersion: 1, Kind: "runtime", Mode: mode,
			Argv: []string{"pr", "list", "--limit=1", "--json=number,title,state"},
		})
	}
	expected = append(expected, fixtureAttemptRecord{
		SchemaVersion: 1, Kind: "runtime", Mode: "success",
		Argv: []string{"issue", "list", "--limit=1", "--json=number,title,state"},
	})
	scanner := bufio.NewScanner(bytes.NewReader(value))
	scanner.Buffer(make([]byte, 4096), maxAttemptLogBytes)
	actual := make([]fixtureAttemptRecord, 0, len(expected))
	for scanner.Scan() {
		var record fixtureAttemptRecord
		decoder := json.NewDecoder(bytes.NewReader(scanner.Bytes()))
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&record); err != nil {
			return fmt.Errorf("attempt record is invalid")
		}
		if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
			return fmt.Errorf("attempt record contains trailing data")
		}
		actual = append(actual, record)
	}
	if err := scanner.Err(); err != nil || len(actual) != len(expected) {
		return fmt.Errorf("attempt sequence length mismatch")
	}
	for index := range expected {
		if actual[index].SchemaVersion != expected[index].SchemaVersion || actual[index].Kind != expected[index].Kind || actual[index].Mode != expected[index].Mode || strings.Join(actual[index].Argv, "\x00") != strings.Join(expected[index].Argv, "\x00") {
			return fmt.Errorf("attempt sequence mismatch at index %d", index)
		}
	}
	return nil
}

func scanCanaries(values ...[]byte) error {
	for _, value := range values {
		for _, canary := range secretCanaries {
			if bytes.Contains(value, []byte(canary)) {
				return fmt.Errorf("secret canary reached a public or persistent boundary")
			}
		}
	}
	return nil
}

func transformDraft(value []byte, command []string) ([]byte, error) {
	if len(command) != 2 || command[1] != "list" || (command[0] != "issue" && command[0] != "pr") {
		return nil, fmt.Errorf("draft command is unsupported")
	}
	text := string(value)
	replacements := [][2]string{
		{"reason: Include this verified command without transformation.", "reason: Return one reviewed compact result."},
		{"kind: identity", "kind: transform"},
		{"append_args: []", `append_args: ["--json=number,title,state"]`},
	}
	for _, replacement := range replacements {
		if strings.Count(text, replacement[0]) != 1 {
			return nil, fmt.Errorf("draft shape is not the expected single-command identity wrapper")
		}
		text = strings.Replace(text, replacement[0], replacement[1], 1)
	}
	if strings.Count(text, "- "+command[0]+"\n") != 1 || strings.Count(text, "- "+command[1]+"\n") != 1 || strings.Contains(text, "output:") {
		return nil, fmt.Errorf("draft command shape is invalid")
	}
	lines := strings.Split(text, "\n")
	inserted := false
	for index, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed != "after: []" {
			continue
		}
		if inserted {
			return nil, fmt.Errorf("draft has multiple wrapper completion points")
		}
		indent := line[:len(line)-len(strings.TrimLeft(line, " \t"))]
		output := []string{
			indent + "output:",
			indent + "    input: json",
			indent + "    select: [number, title, state]",
			indent + "    rename:",
			indent + "        - from: number",
			indent + "          to: id",
			indent + "    render: compact_json",
		}
		lines = append(lines[:index], append(output, lines[index:]...)...)
		inserted = true
		break
	}
	if !inserted {
		return nil, fmt.Errorf("draft wrapper completion point is missing")
	}
	return []byte(strings.Join(lines, "\n")), nil
}

type inspectionEvidence struct {
	CatalogDigest         string `json:"catalog_digest"`
	SourceProcessAttempts int    `json:"source_process_attempts"`
}

func decodeInspection(value []byte) (inspectionEvidence, error) {
	var document struct {
		SchemaVersion int                `json:"schema_version"`
		Inspection    inspectionEvidence `json:"inspection"`
	}
	if err := json.Unmarshal(value, &document); err != nil || document.SchemaVersion != 1 {
		return inspectionEvidence{}, fmt.Errorf("invalid inspection document")
	}
	return document.Inspection, nil
}

func validateSpecificationEvidence(value []byte, catalogDigest string) error {
	var document struct {
		SchemaVersion int `json:"schema_version"`
		Validation    struct {
			Valid                 bool   `json:"valid"`
			CatalogDigest         string `json:"catalog_digest"`
			SpecificationDigest   string `json:"specification_digest"`
			CommandCount          int    `json:"command_count"`
			IncludedCount         int    `json:"included_count"`
			ExcludedCount         int    `json:"excluded_count"`
			IdentityWrapperCount  int    `json:"identity_wrapper_count"`
			TransformWrapperCount int    `json:"transform_wrapper_count"`
		} `json:"validation"`
	}
	if err := json.Unmarshal(value, &document); err != nil || document.SchemaVersion != 2 {
		return fmt.Errorf("invalid validation document")
	}
	result := document.Validation
	if !result.Valid || result.CatalogDigest != catalogDigest || !digestValue(result.SpecificationDigest) || result.CommandCount != 1 || result.IncludedCount != 1 || result.ExcludedCount != 0 || result.IdentityWrapperCount != 0 || result.TransformWrapperCount != 1 {
		return fmt.Errorf("unexpected validation evidence")
	}
	return nil
}

func decodeBundleDigest(value []byte) (string, error) {
	var document struct {
		SchemaVersion int `json:"schema_version"`
		Build         struct {
			BundleDigest string `json:"bundle_digest"`
		} `json:"build"`
	}
	if err := json.Unmarshal(value, &document); err != nil || document.SchemaVersion != 2 || !digestValue(document.Build.BundleDigest) {
		return "", fmt.Errorf("invalid bundle document")
	}
	return document.Build.BundleDigest, nil
}

func validateStatus(value []byte, digest string, state bundletrust.State) error {
	var document struct {
		SchemaVersion int `json:"schema_version"`
		Status        struct {
			BundleDigest          string            `json:"bundle_digest"`
			Adoption              bundletrust.State `json:"adoption"`
			Source                string            `json:"source"`
			Adopted               bool              `json:"adopted"`
			SourceProcessAttempts int               `json:"source_process_attempts"`
		} `json:"status"`
	}
	if err := json.Unmarshal(value, &document); err != nil || document.SchemaVersion != 2 {
		return fmt.Errorf("invalid status document")
	}
	wantedAdopted := state == bundletrust.StateAdopted
	if document.Status.BundleDigest != digest || document.Status.Adoption != state || document.Status.Source != "current" || document.Status.Adopted != wantedAdopted || document.Status.SourceProcessAttempts != 0 {
		return fmt.Errorf("unexpected status evidence")
	}
	return nil
}

type previewEvidence struct {
	PlanDigest            string `json:"plan_digest"`
	SourceProcessAttempts int    `json:"source_process_attempts"`
}

func decodePreview(value []byte) (previewEvidence, error) {
	var document struct {
		SchemaVersion int             `json:"schema_version"`
		Preview       previewEvidence `json:"preview"`
	}
	if err := json.Unmarshal(value, &document); err != nil || document.SchemaVersion != 2 {
		return previewEvidence{}, fmt.Errorf("invalid preview document")
	}
	return document.Preview, nil
}

type executionEvidence struct {
	BundleDigest   string   `json:"bundle_digest"`
	PlanDigest     string   `json:"plan_digest"`
	MatchedCommand []string `json:"matched_command"`
	WrapperKind    string   `json:"wrapper_kind"`
	Output         struct {
		Render  string                       `json:"render"`
		Shape   string                       `json:"shape"`
		Fields  []string                     `json:"fields"`
		Records []map[string]json.RawMessage `json:"records"`
	} `json:"output"`
	Source struct {
		ExitCode int `json:"exit_code"`
	} `json:"source"`
	SourceProcessAttempts int `json:"source_process_attempts"`
}

func decodeExecution(value []byte) (executionEvidence, error) {
	var document struct {
		SchemaVersion int               `json:"schema_version"`
		Execution     executionEvidence `json:"execution"`
	}
	if err := json.Unmarshal(value, &document); err != nil || document.SchemaVersion != 2 {
		return executionEvidence{}, fmt.Errorf("invalid execution document")
	}
	return document.Execution, nil
}

func validateSelectedOutput(result executionEvidence, command string) error {
	if strings.Join(result.MatchedCommand, " ") != command || result.WrapperKind != "transform" || result.Output.Render != "compact_json" || result.Output.Shape != "array" || strings.Join(result.Output.Fields, ",") != "id,title,state" || result.Source.ExitCode != 0 || len(result.Output.Records) != 1 {
		return fmt.Errorf("unexpected execution metadata")
	}
	wantedID := "101"
	wantedTitle := `"Review policy"`
	if command == "issue list" {
		wantedID = "202"
		wantedTitle = `"Fix deterministic wrapper"`
	} else if command != "pr list" {
		return fmt.Errorf("unsupported selected-output command")
	}
	record := result.Output.Records[0]
	if len(record) != 3 || string(record["id"]) != wantedID || string(record["title"]) != wantedTitle || string(record["state"]) != `"OPEN"` {
		return fmt.Errorf("unexpected selected record")
	}
	return nil
}

func validateFault(value []byte, declaration helpFaultDeclaration) error {
	var document struct {
		SchemaVersion int `json:"schema_version"`
		Error         struct {
			Code        string           `json:"code"`
			Kind        string           `json:"kind"`
			Retryable   bool             `json:"retryable"`
			NextActions []helpNextAction `json:"next_actions"`
		} `json:"error"`
	}
	if err := json.Unmarshal(value, &document); err != nil || document.SchemaVersion != 1 || document.Error.Code != declaration.Code || document.Error.Kind != declaration.Kind || document.Error.Retryable != declaration.Retryable || !equalNextActions(document.Error.NextActions, declaration.NextActions) {
		return fmt.Errorf("unexpected structured fault evidence")
	}
	return nil
}

func equalNextActions(left, right []helpNextAction) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}

func digestValue(value string) bool {
	return lowercaseHex(value, 64)
}
