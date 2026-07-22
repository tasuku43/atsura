package main

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
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
	maxSourceStderrBytes  = 256 * 1024
	maxTransformedBytes   = 2 * 1024 * 1024
	commandTimeout        = 40 * time.Second
	fixtureAttemptEnv     = "ATSURA_SOURCE_FIXTURE_ATTEMPT_LOG"
	fixtureModeEnv        = "ATSURA_SOURCE_FIXTURE_MODE"
	goTestAttemptEnv      = "ATSURA_GO_TEST_ATTEMPT_LOG"
	goAdapterKind         = "atsura.source.go_cli"
	goAdapterContract     = 1
)

var (
	go126VersionPattern = regexp.MustCompile(`^go1\.26\.(0|[1-9][0-9]*)$`)
	goTestOutputPattern = regexp.MustCompile(`^PASS\nok[ \t]+example\.com/atsura-artifact-go[ \t]+[0-9]+(?:\.[0-9]+)?s\n$`)
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
	Target                      string                `json:"target"`
	ObservedHost                string                `json:"observed_host"`
	ArchiveName                 string                `json:"archive_name"`
	ArchiveSHA256               string                `json:"archive_sha256"`
	Version                     string                `json:"version"`
	Revision                    string                `json:"revision"`
	HelpContractsVerified       int                   `json:"help_contracts_verified"`
	BundleDigest                string                `json:"bundle_digest"`
	PlanDigest                  string                `json:"plan_digest"`
	IssueBundleDigest           string                `json:"issue_bundle_digest"`
	IssuePlanDigest             string                `json:"issue_plan_digest"`
	WrapperOutcome              string                `json:"wrapper_outcome"`
	WrapperCases                []wrapperCaseEvidence `json:"wrapper_cases"`
	WrapperSourceAttempts       int                   `json:"wrapper_source_process_attempts"`
	CommandsVerified            []string              `json:"commands_verified"`
	SourceInspectionAttempts    int                   `json:"source_inspection_attempts"`
	ZeroAttemptRejections       int                   `json:"zero_attempt_rejections"`
	PostStartFaults             []string              `json:"post_start_faults"`
	FixtureAttempts             int                   `json:"fixture_attempts"`
	CredentialEnvironmentAbsent bool                  `json:"credential_environment_absent"`
	SecretCanariesAbsent        bool                  `json:"secret_canaries_absent"`
	GoSource                    goSourceEvidence      `json:"go_source"`
}

type goSourceEvidence struct {
	AdapterKind              string                `json:"adapter_kind"`
	AdapterContractVersion   int                   `json:"adapter_contract_version"`
	SourceVersion            string                `json:"source_version"`
	CatalogDigest            string                `json:"catalog_digest"`
	SourceInspectionAttempts int                   `json:"source_inspection_attempts"`
	CommandsVerified         []string              `json:"commands_verified"`
	BundleDigest             string                `json:"bundle_digest"`
	PlanDigest               string                `json:"plan_digest"`
	WrapperOutcome           string                `json:"wrapper_outcome"`
	WrapperCases             []wrapperCaseEvidence `json:"wrapper_cases"`
	WrapperSourceAttempts    int                   `json:"wrapper_source_process_attempts"`
	ZeroAttemptRejections    int                   `json:"zero_attempt_rejections"`
}

// wrapperCaseEvidence retains only identities, digests, conventional status,
// and attempt counts. Source bytes remain confined to the native replay.
type wrapperCaseEvidence struct {
	Name                  string `json:"name"`
	WrapperKind           string `json:"wrapper_kind"`
	ResultMode            string `json:"result_mode"`
	BundleDigest          string `json:"bundle_digest"`
	PlanDigest            string `json:"plan_digest"`
	WrapperSourceSHA256   string `json:"wrapper_source_sha256"`
	StdoutSHA256          string `json:"stdout_sha256"`
	StderrSHA256          string `json:"stderr_sha256"`
	SourceExitCode        int    `json:"source_exit_code"`
	SourceProcessAttempts int    `json:"source_process_attempts"`
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
	sourceInputPath, err := prepareInputFile(configuration.source)
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
	executablePath, err = filepath.EvalSymlinks(executablePath)
	if err != nil || !filepath.IsAbs(executablePath) || filepath.Clean(executablePath) != executablePath {
		return evidenceDocument{}, fmt.Errorf("release executable identity is invalid")
	}

	sourcePath, sourceBin, err := stageSourceFixture(sourceInputPath, configuration.goos, workRoot)
	if err != nil {
		return evidenceDocument{}, fmt.Errorf("source fixture staging failed")
	}
	attemptLog := filepath.Join(workRoot, "fixture-attempts.jsonl")
	trustPath, environment, err := isolatedEnvironment(workRoot, sourceBin, attemptLog)
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
	inspection, err := runner.success(ctx, "success", "source", "inspect", "--adapter", "github-cli", "--executable", "gh")
	if err != nil {
		return evidenceDocument{}, fmt.Errorf("source inspection failed")
	}
	inspectionPayload, err := decodeInspection(inspection.stdout)
	if err != nil || inspectionPayload.SourceProcessAttempts != 4 || !digestValue(inspectionPayload.CatalogDigest) ||
		inspectionPayload.Catalog.Source.RequestedExecutable != "gh" || inspectionPayload.Catalog.Source.ResolvedPath != sourcePath {
		return evidenceDocument{}, fmt.Errorf("source inspection evidence is invalid")
	}
	if err := writePrivate(catalogPath, inspection.stdout); err != nil {
		return evidenceDocument{}, fmt.Errorf("catalog evidence write failed")
	}
	if err := requireAttempts(attemptLog, 4); err != nil {
		return evidenceDocument{}, fmt.Errorf("source inspection attempt evidence is invalid")
	}

	prJourney, err := prepareCommandJourney(ctx, runner, helpEvidence, workRoot, catalogPath, inspectionPayload.CatalogDigest, "gh", trustPath, attemptLog, 4, []string{"pr", "list"})
	if err != nil {
		return evidenceDocument{}, fmt.Errorf("pull-request journey preparation failed: %w", err)
	}
	zeroAttemptRejections := prJourney.zeroAttemptRejections

	for _, conflict := range []string{"--web", "--jq=.[]", "--template={{.number}}"} {
		declaration, declarationErr := helpEvidence.fault("bundle execute", "option_not_in_surface")
		if declarationErr != nil || declaration.Kind != "not_found" || declaration.Retryable {
			return evidenceDocument{}, fmt.Errorf("runtime conflict help contract is invalid")
		}
		arguments := append([]string{"--error-format=json", "bundle", "execute"}, prJourney.baseInvocation...)
		arguments = append(arguments, conflict)
		failure, err := runner.failure(ctx, "success", 6, declaration, arguments...)
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

	issueJourney, err := prepareCommandJourney(ctx, runner, helpEvidence, workRoot, catalogPath, inspectionPayload.CatalogDigest, "gh", trustPath, attemptLog, wantedAttempts, []string{"issue", "list"})
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

	wrapperInputs := []packagedWrapperInput{{
		name: "transformed_json", journey: prJourney,
		callerArgs: []string{"pr", "list", "--limit=1"},
		wantStdout: []byte("[{\"id\":101,\"title\":\"Review policy\",\"state\":\"OPEN\"}]\n"),
		wantStderr: []byte{}, wantExitCode: 0,
	}}
	if configuration.goos == "linux" || configuration.goos == "darwin" {
		identityArgs := []string{
			"pr", "list",
			"--search=space value;$(touch atsura-artifact-injection)",
			"--label=first",
			"--label=Unicode 雪",
			"--repo=-dash",
		}
		identityJourney, prepareErr := prepareSourceStreamJourney(
			ctx, runner, workRoot, catalogPath, inspectionPayload.CatalogDigest,
			trustPath, attemptLog, wantedAttempts, "identity", []string{"pr", "list"}, identityArgs,
		)
		if prepareErr != nil {
			return evidenceDocument{}, fmt.Errorf("identity wrapper journey preparation failed: %w", prepareErr)
		}
		appendArgs := []string{
			"issue", "list",
			"--search=append value",
			"--label=one",
			"--label=two",
		}
		appendJourney, prepareErr := prepareSourceStreamJourney(
			ctx, runner, workRoot, catalogPath, inspectionPayload.CatalogDigest,
			trustPath, attemptLog, wantedAttempts, "append_only", []string{"issue", "list"}, appendArgs,
		)
		if prepareErr != nil {
			return evidenceDocument{}, fmt.Errorf("append-only wrapper journey preparation failed: %w", prepareErr)
		}
		wrapperInputs = append(wrapperInputs,
			packagedWrapperInput{
				name: "identity", journey: identityJourney, callerArgs: identityArgs,
				wantStdout: []byte{'I', 'D', ':', 0x00, 0xff, '\n'},
				wantStderr: []byte{'I', 'D', 'E', 'R', 'R', ':', 0xfe}, wantExitCode: 0,
			},
			packagedWrapperInput{
				name: "append_only", journey: appendJourney, callerArgs: appendArgs,
				wantStdout: []byte{'A', 'P', 'P', ':', 0xff, 0x00},
				wantStderr: []byte("APPERR:\n"), wantExitCode: 23,
			},
		)
	}

	wrapperEvidence, err := verifyPackagedWrappers(ctx, runner, helpEvidence, configuration.goos, executablePath, wrapperInputs, attemptLog, wantedAttempts, workRoot)
	if err != nil {
		return evidenceDocument{}, err
	}
	zeroAttemptRejections += wrapperEvidence.zeroAttemptRejections
	wantedAttempts += wrapperEvidence.sourceProcessAttempts

	goEvidence, goBoundaries, err := verifyGoSourceJourney(
		ctx, runner, helpEvidence, configuration.goos, executablePath, trustPath, attemptLog, wantedAttempts, workRoot,
	)
	if err != nil {
		return evidenceDocument{}, err
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
	canaryBoundaries = append(canaryBoundaries, wrapperEvidence.boundaries...)
	canaryBoundaries = append(canaryBoundaries, goBoundaries...)
	canaryBoundaries = append(canaryBoundaries, prJourney.boundaries...)
	canaryBoundaries = append(canaryBoundaries, issueJourney.boundaries...)
	canaryBoundaries = append(canaryBoundaries, helpEvidence.outputs...)
	if err := scanCanaries(canaryBoundaries...); err != nil {
		return evidenceDocument{}, err
	}
	if err := validateAttemptSequence(attemptBytes, configuration.goos); err != nil {
		return evidenceDocument{}, fmt.Errorf("fixture attempt sequence is invalid")
	}

	return evidenceDocument{SchemaVersion: 4, ArtifactJourney: artifactJourneyEvidence{
		Target: configuration.goos + "/" + configuration.goarch, ObservedHost: runtime.GOOS + "/" + runtime.GOARCH,
		ArchiveName: filepath.Base(archivePath), ArchiveSHA256: digest,
		Version: strings.TrimPrefix(configuration.tag, "v"), Revision: configuration.revision, HelpContractsVerified: len(helpEvidence.outputs),
		BundleDigest: prJourney.bundleDigest, PlanDigest: prJourney.planDigest,
		IssueBundleDigest: issueJourney.bundleDigest, IssuePlanDigest: issueJourney.planDigest,
		WrapperOutcome: wrapperEvidence.outcome, WrapperCases: wrapperEvidence.cases,
		WrapperSourceAttempts:    wrapperEvidence.sourceProcessAttempts,
		CommandsVerified:         []string{"issue list", "pr list"},
		SourceInspectionAttempts: 4, ZeroAttemptRejections: zeroAttemptRejections,
		PostStartFaults: faultCodes, FixtureAttempts: wantedAttempts,
		CredentialEnvironmentAbsent: true, SecretCanariesAbsent: true,
		GoSource: goEvidence,
	}}, nil
}

type preparedCommandJourney struct {
	bundleDigest          string
	planDigest            string
	wrapperKind           string
	resultMode            string
	baseInvocation        []string
	zeroAttemptRejections int
	boundaries            [][]byte
}

type packagedWrapperEvidence struct {
	outcome               string
	cases                 []wrapperCaseEvidence
	sourceProcessAttempts int
	zeroAttemptRejections int
	boundaries            [][]byte
}

type packagedWrapperInput struct {
	name         string
	journey      preparedCommandJourney
	callerArgs   []string
	wantStdout   []byte
	wantStderr   []byte
	wantExitCode int
}

type wrapperRenderDocument struct {
	SchemaVersion int `json:"schema_version"`
	Wrapper       struct {
		Source       string `json:"source"`
		SourceSHA256 string `json:"source_sha256"`
		Command      string `json:"command"`
		Contract     struct {
			Shell   string `json:"shell"`
			Version int    `json:"version"`
		} `json:"contract"`
		Bundle struct {
			Digest  string `json:"digest"`
			Locator string `json:"locator"`
		} `json:"bundle"`
		Runtime struct {
			ResolvedPath string `json:"resolved_path"`
			SHA256       string `json:"sha256"`
			Size         int64  `json:"size"`
		} `json:"runtime"`
		SourceProcessAttempts int `json:"source_process_attempts"`
	} `json:"wrapper"`
}

func verifyPackagedWrappers(
	ctx context.Context,
	runner journeyRunner,
	help packagedHelpEvidence,
	goos, executablePath string,
	inputs []packagedWrapperInput,
	attemptLog string,
	existingAttempts int,
	workRoot string,
) (packagedWrapperEvidence, error) {
	if len(inputs) == 0 || inputs[0].name != "transformed_json" {
		return packagedWrapperEvidence{}, fmt.Errorf("wrapper case inventory is invalid")
	}
	if goos == "windows" {
		if len(inputs) != 1 {
			return packagedWrapperEvidence{}, fmt.Errorf("Windows wrapper case inventory is invalid")
		}
		declaration, err := help.fault("wrapper render", "wrapper_platform_not_supported")
		if err != nil || declaration.Kind != "unsupported" || declaration.Retryable {
			return packagedWrapperEvidence{}, fmt.Errorf("wrapper platform help contract is invalid")
		}
		failure, err := runner.failure(ctx, "success", 12, declaration,
			"--error-format=json", "wrapper", "render", "--bundle", inputs[0].journey.bundlePath(), "--format", "json")
		if err != nil {
			return packagedWrapperEvidence{}, fmt.Errorf("unsupported wrapper platform evidence is invalid")
		}
		if err := requireAttempts(attemptLog, existingAttempts); err != nil {
			return packagedWrapperEvidence{}, fmt.Errorf("unsupported wrapper platform started the source fixture")
		}
		return packagedWrapperEvidence{
			outcome: "platform_not_supported", cases: []wrapperCaseEvidence{}, zeroAttemptRejections: 1,
			boundaries: [][]byte{failure.stdout, failure.stderr},
		}, nil
	}
	if goos != "linux" && goos != "darwin" {
		return packagedWrapperEvidence{}, fmt.Errorf("wrapper journey target is unsupported")
	}
	if len(inputs) != 3 || inputs[1].name != "identity" || inputs[2].name != "append_only" {
		return packagedWrapperEvidence{}, fmt.Errorf("POSIX wrapper case inventory is invalid")
	}
	runtimeDigest, runtimeSize, err := regularFileIdentity(executablePath)
	if err != nil {
		return packagedWrapperEvidence{}, fmt.Errorf("packaged runtime identity is unreadable")
	}

	result := packagedWrapperEvidence{
		outcome: "ordinary_command_verified", cases: make([]wrapperCaseEvidence, 0, len(inputs)),
		zeroAttemptRejections: 1, boundaries: make([][]byte, 0, len(inputs)*6),
	}
	for index, input := range inputs {
		if input.journey.wrapperKind == "" || input.journey.resultMode == "" || input.callerArgs == nil || input.wantStdout == nil || input.wantStderr == nil || input.wantExitCode < 0 {
			return packagedWrapperEvidence{}, fmt.Errorf("wrapper case %d is incomplete", index)
		}
		result.boundaries = append(result.boundaries, input.journey.boundaries...)
		jsonRender, renderErr := runner.success(ctx, "success", "wrapper", "render", "--bundle", input.journey.bundlePath(), "--format", "json")
		if renderErr != nil {
			return packagedWrapperEvidence{}, fmt.Errorf("wrapper case %d JSON rendering failed", index)
		}
		document, decodeErr := decodeWrapperRender(jsonRender.stdout)
		if decodeErr != nil {
			return packagedWrapperEvidence{}, fmt.Errorf("wrapper case %d JSON evidence is invalid", index)
		}
		textRender, renderErr := runner.success(ctx, "success", "wrapper", "render", "--bundle", input.journey.bundlePath())
		if renderErr != nil {
			return packagedWrapperEvidence{}, fmt.Errorf("wrapper case %d text rendering failed", index)
		}
		if string(textRender.stdout) != document.Wrapper.Source {
			return packagedWrapperEvidence{}, fmt.Errorf("wrapper case %d text and JSON source differ", index)
		}
		sourceDigest := digestBytes(textRender.stdout)
		if err := validateWrapperRenderEvidence(document, input.journey, "gh", executablePath, runtimeDigest, runtimeSize, sourceDigest); err != nil {
			return packagedWrapperEvidence{}, fmt.Errorf("wrapper case %d render binding evidence is invalid: %w", index, err)
		}
		if err := requireAttempts(attemptLog, existingAttempts+index); err != nil {
			return packagedWrapperEvidence{}, fmt.Errorf("wrapper case %d rendering started the source fixture", index)
		}
		result.boundaries = append(result.boundaries, jsonRender.stdout, textRender.stdout)

		if index == 0 {
			declaration, declarationErr := help.fault("wrapper run", "bundle_binding_mismatch")
			if declarationErr != nil || declaration.Kind != "rejected" || declaration.Retryable {
				return packagedWrapperEvidence{}, fmt.Errorf("wrapper binding help contract is invalid")
			}
			badDigest := differentDigest(document.Wrapper.Bundle.Digest)
			rejectionArgs := []string{
				"--error-format=json", "wrapper", "run",
				"--contract-version=1",
				"--bundle=" + document.Wrapper.Bundle.Locator,
				"--bundle-digest=" + badDigest,
				"--runtime-path=" + document.Wrapper.Runtime.ResolvedPath,
				"--runtime-sha256=" + document.Wrapper.Runtime.SHA256,
				fmt.Sprintf("--runtime-size=%d", document.Wrapper.Runtime.Size),
				"--",
			}
			rejectionArgs = append(rejectionArgs, input.callerArgs...)
			rejected, rejectionErr := runner.failure(ctx, "success", 10, declaration, rejectionArgs...)
			if rejectionErr != nil {
				return packagedWrapperEvidence{}, fmt.Errorf("wrapper binding rejection evidence is invalid")
			}
			if err := requireAttempts(attemptLog, existingAttempts); err != nil {
				return packagedWrapperEvidence{}, fmt.Errorf("wrapper binding rejection started the source fixture")
			}
			result.boundaries = append(result.boundaries, rejected.stdout, rejected.stderr)
		}

		wrapperPath := filepath.Join(workRoot, "caller-owned-"+input.name+"-wrapper.sh")
		if err := writePrivate(wrapperPath, textRender.stdout); err != nil {
			return packagedWrapperEvidence{}, fmt.Errorf("wrapper case %d caller-owned fixture write failed", index)
		}
		invocation, invokeErr := runPOSIXCaller(ctx, runner, "gh", wrapperPath, input.callerArgs, input.wantExitCode)
		if invokeErr != nil {
			return packagedWrapperEvidence{}, fmt.Errorf("wrapper case %d caller-owned invocation failed", index)
		}
		if !bytes.Equal(invocation.stdout, input.wantStdout) || !bytes.Equal(invocation.stderr, input.wantStderr) || invocation.exitCode != input.wantExitCode {
			return packagedWrapperEvidence{}, fmt.Errorf("wrapper case %d caller-owned result is invalid", index)
		}
		if err := requireAttempts(attemptLog, existingAttempts+index+1); err != nil {
			return packagedWrapperEvidence{}, fmt.Errorf("wrapper case %d attempt evidence is invalid", index)
		}
		result.cases = append(result.cases, wrapperCaseEvidence{
			Name: input.name, WrapperKind: input.journey.wrapperKind, ResultMode: input.journey.resultMode,
			BundleDigest: input.journey.bundleDigest, PlanDigest: input.journey.planDigest,
			WrapperSourceSHA256: sourceDigest, StdoutSHA256: digestBytes(invocation.stdout), StderrSHA256: digestBytes(invocation.stderr),
			SourceExitCode: invocation.exitCode, SourceProcessAttempts: 1,
		})
		result.sourceProcessAttempts++
		result.boundaries = append(result.boundaries, invocation.stdout, invocation.stderr)
	}
	return result, nil
}

func verifyGoSourceJourney(
	ctx context.Context,
	runner journeyRunner,
	help packagedHelpEvidence,
	goos, executablePath, trustPath, attemptLog string,
	existingFixtureAttempts int,
	workRoot string,
) (goSourceEvidence, [][]byte, error) {
	goExecutable, err := exec.LookPath("go")
	if err != nil {
		return goSourceEvidence{}, nil, fmt.Errorf("Go source executable is unavailable")
	}
	goExecutable, err = filepath.Abs(goExecutable)
	if err != nil || !filepath.IsAbs(goExecutable) || filepath.Clean(goExecutable) != goExecutable {
		return goSourceEvidence{}, nil, fmt.Errorf("Go source executable identity is invalid")
	}
	// Resolve the portable public spelling only through the exact toolchain
	// directory selected by the native artifact runner.
	runner.environment = replaceEnvironment(runner.environment, map[string]string{"PATH": filepath.Dir(goExecutable)})

	inspection, err := runner.success(ctx, "success", "source", "inspect", "--adapter", "go-cli", "--executable", "go")
	if err != nil {
		return goSourceEvidence{}, nil, fmt.Errorf("Go source inspection failed")
	}
	inspectionPayload, err := decodeInspection(inspection.stdout)
	if err != nil || inspectionPayload.SourceProcessAttempts != 3 ||
		inspectionPayload.Catalog.Adapter.Kind != goAdapterKind || inspectionPayload.Catalog.Adapter.ContractVersion != goAdapterContract ||
		inspectionPayload.Catalog.Source.RequestedExecutable != "go" || !go126VersionPattern.MatchString(inspectionPayload.Catalog.Source.Version) ||
		inspectionPayload.Catalog.Probe.Attempts != 3 || !inspectionHasCommand(inspectionPayload, []string{"test"}) ||
		!digestValue(inspectionPayload.CatalogDigest) {
		return goSourceEvidence{}, nil, fmt.Errorf("Go source inspection evidence is invalid")
	}
	if err := requireAttempts(attemptLog, existingFixtureAttempts); err != nil {
		return goSourceEvidence{}, nil, fmt.Errorf("Go source inspection started the GitHub source fixture")
	}

	catalogPath := filepath.Join(workRoot, "go-catalog.json")
	if err := writePrivate(catalogPath, inspection.stdout); err != nil {
		return goSourceEvidence{}, nil, fmt.Errorf("Go catalog evidence write failed")
	}
	goAttemptLog := filepath.Join(workRoot, "go-test-attempts.log")
	if err := createGoTestModule(workRoot); err != nil {
		return goSourceEvidence{}, nil, fmt.Errorf("Go source module fixture creation failed")
	}
	if err := requireGoTestAttempts(goAttemptLog, 0); err != nil {
		return goSourceEvidence{}, nil, fmt.Errorf("Go source attempt fixture is not initially empty")
	}

	draft, err := runner.success(ctx, "success", "spec", "init", "--catalog", catalogPath, "--", "test")
	if err != nil {
		return goSourceEvidence{}, nil, fmt.Errorf("Go source specification draft failed")
	}
	specificationPath := filepath.Join(workRoot, "go-specification.yaml")
	if err := writePrivate(specificationPath, draft.stdout); err != nil {
		return goSourceEvidence{}, nil, fmt.Errorf("Go source specification write failed")
	}
	validation, err := runner.success(ctx, "success", "spec", "validate", "--catalog", catalogPath, "--spec", specificationPath)
	if err != nil || validateSpecificationEvidence(validation.stdout, inspectionPayload.CatalogDigest, 1, 0) != nil {
		return goSourceEvidence{}, nil, fmt.Errorf("Go source specification validation evidence is invalid")
	}
	built, err := runner.success(ctx, "success", "bundle", "build", "--catalog", catalogPath, "--spec", specificationPath)
	if err != nil {
		return goSourceEvidence{}, nil, fmt.Errorf("Go source bundle build failed")
	}
	bundleDigest, err := decodeBundleDigest(built.stdout)
	if err != nil {
		return goSourceEvidence{}, nil, fmt.Errorf("Go source bundle evidence is invalid")
	}
	bundlePath := filepath.Join(workRoot, "go-bundle.json")
	if err := writePrivate(bundlePath, built.stdout); err != nil {
		return goSourceEvidence{}, nil, fmt.Errorf("Go source bundle write failed")
	}
	store := trustfile.New(trustPath)
	changed, err := store.Add(ctx, bundleDigest)
	if err != nil || !changed || store.Inspect(ctx, bundleDigest) != bundletrust.StateAdopted {
		return goSourceEvidence{}, nil, fmt.Errorf("Go source exact receipt seeding failed")
	}
	status, err := runner.success(ctx, "success", "bundle", "status", "--bundle", bundlePath)
	if err != nil || validateStatus(status.stdout, bundleDigest, bundletrust.StateAdopted) != nil {
		return goSourceEvidence{}, nil, fmt.Errorf("Go source adopted status evidence is invalid")
	}
	baseInvocation := []string{"--bundle", bundlePath, "--", inspectionPayload.Catalog.Source.ResolvedPath, "test"}
	preview, err := runner.success(ctx, "success", append([]string{"bundle", "preview"}, baseInvocation...)...)
	if err != nil {
		return goSourceEvidence{}, nil, fmt.Errorf("Go source preview failed")
	}
	previewPayload, err := decodePreview(preview.stdout)
	if err != nil || previewPayload.SourceProcessAttempts != 0 || !digestValue(previewPayload.PlanDigest) ||
		previewPayload.Plan.WrapperKind != "identity" || previewPayload.Plan.ResultMode != "source_stream_passthrough" {
		return goSourceEvidence{}, nil, fmt.Errorf("Go source preview evidence is invalid")
	}
	if err := requireGoTestAttempts(goAttemptLog, 0); err != nil {
		return goSourceEvidence{}, nil, fmt.Errorf("Go source preparation started the test command")
	}
	journey := preparedCommandJourney{
		bundleDigest: bundleDigest, planDigest: previewPayload.PlanDigest,
		wrapperKind: previewPayload.Plan.WrapperKind, resultMode: previewPayload.Plan.ResultMode,
		baseInvocation: baseInvocation,
	}
	boundaries := [][]byte{inspection.stdout, draft.stdout, validation.stdout, built.stdout, status.stdout, preview.stdout}
	evidence := goSourceEvidence{
		AdapterKind: goAdapterKind, AdapterContractVersion: goAdapterContract,
		SourceVersion: inspectionPayload.Catalog.Source.Version, CatalogDigest: inspectionPayload.CatalogDigest,
		SourceInspectionAttempts: 3, CommandsVerified: []string{"test"},
		BundleDigest: bundleDigest, PlanDigest: previewPayload.PlanDigest,
	}

	if goos == "windows" {
		declaration, declarationErr := help.fault("wrapper render", "wrapper_platform_not_supported")
		if declarationErr != nil || declaration.Kind != "unsupported" || declaration.Retryable {
			return goSourceEvidence{}, nil, fmt.Errorf("Go source Windows wrapper help contract is invalid")
		}
		failure, failureErr := runner.failure(ctx, "success", 12, declaration,
			"--error-format=json", "wrapper", "render", "--bundle", bundlePath, "--format", "json")
		if failureErr != nil {
			return goSourceEvidence{}, nil, fmt.Errorf("Go source unsupported wrapper evidence is invalid")
		}
		if err := requireGoTestAttempts(goAttemptLog, 0); err != nil {
			return goSourceEvidence{}, nil, fmt.Errorf("Go source unsupported wrapper started the test command")
		}
		evidence.WrapperOutcome = "platform_not_supported"
		evidence.WrapperCases = []wrapperCaseEvidence{}
		evidence.ZeroAttemptRejections = 1
		return evidence, append(boundaries, failure.stdout, failure.stderr), nil
	}
	if goos != "linux" && goos != "darwin" {
		return goSourceEvidence{}, nil, fmt.Errorf("Go source wrapper target is unsupported")
	}

	runtimeDigest, runtimeSize, err := regularFileIdentity(executablePath)
	if err != nil {
		return goSourceEvidence{}, nil, fmt.Errorf("Go source packaged runtime identity is unreadable")
	}
	jsonRender, err := runner.success(ctx, "success", "wrapper", "render", "--bundle", bundlePath, "--format", "json")
	if err != nil {
		return goSourceEvidence{}, nil, fmt.Errorf("Go source wrapper JSON rendering failed")
	}
	document, err := decodeWrapperRender(jsonRender.stdout)
	if err != nil {
		return goSourceEvidence{}, nil, fmt.Errorf("Go source wrapper JSON evidence is invalid")
	}
	textRender, err := runner.success(ctx, "success", "wrapper", "render", "--bundle", bundlePath)
	if err != nil || string(textRender.stdout) != document.Wrapper.Source {
		return goSourceEvidence{}, nil, fmt.Errorf("Go source wrapper text rendering is invalid")
	}
	sourceDigest := digestBytes(textRender.stdout)
	if err := validateWrapperRenderEvidence(document, journey, "go", executablePath, runtimeDigest, runtimeSize, sourceDigest); err != nil {
		return goSourceEvidence{}, nil, fmt.Errorf("Go source wrapper binding evidence is invalid: %w", err)
	}
	if err := requireGoTestAttempts(goAttemptLog, 0); err != nil {
		return goSourceEvidence{}, nil, fmt.Errorf("Go source rendering started the test command")
	}
	wrapperPath := filepath.Join(workRoot, "caller-owned-go-wrapper.sh")
	if err := writePrivate(wrapperPath, textRender.stdout); err != nil {
		return goSourceEvidence{}, nil, fmt.Errorf("Go source caller-owned wrapper write failed")
	}
	rejectionDeclaration, err := help.fault("wrapper run", "wrapper_runtime_not_supported")
	if err != nil || rejectionDeclaration.Kind != "unsupported" || rejectionDeclaration.Retryable {
		return goSourceEvidence{}, nil, fmt.Errorf("Go source argv rejection help contract is invalid")
	}
	rejection, err := runPOSIXCaller(ctx, runner, "go", wrapperPath, []string{"test", "extra"}, 12)
	if err != nil || len(rejection.stdout) != 0 || validateFault(rejection.stderr, rejectionDeclaration) != nil {
		return goSourceEvidence{}, nil, fmt.Errorf("Go source additional-argument rejection is invalid")
	}
	if err := requireGoTestAttempts(goAttemptLog, 0); err != nil {
		return goSourceEvidence{}, nil, fmt.Errorf("Go source additional-argument rejection started the test command")
	}
	invocation, err := runPOSIXCaller(ctx, runner, "go", wrapperPath, []string{"test"}, 0)
	if err != nil {
		return goSourceEvidence{}, nil, fmt.Errorf("Go source caller-owned invocation failed")
	}
	if !goTestOutputPattern.Match(invocation.stdout) || len(invocation.stderr) != 0 {
		return goSourceEvidence{}, nil, fmt.Errorf(
			"Go source caller-owned result is invalid (stdout_bytes=%d stderr_bytes=%d lines=%d ok_prefix=%t module_marker=%t final_lf=%t)",
			len(invocation.stdout), len(invocation.stderr), bytes.Count(invocation.stdout, []byte{'\n'}),
			bytes.HasPrefix(invocation.stdout, []byte("ok")), bytes.Contains(invocation.stdout, []byte("example.com/atsura-artifact-go")),
			bytes.HasSuffix(invocation.stdout, []byte{'\n'}),
		)
	}
	if err := requireGoTestAttempts(goAttemptLog, 1); err != nil {
		return goSourceEvidence{}, nil, fmt.Errorf("Go source one-attempt evidence is invalid")
	}
	if err := requireAttempts(attemptLog, existingFixtureAttempts); err != nil {
		return goSourceEvidence{}, nil, fmt.Errorf("Go source journey started the GitHub source fixture")
	}
	evidence.WrapperOutcome = "ordinary_command_verified"
	evidence.WrapperCases = []wrapperCaseEvidence{{
		Name: "go_test_identity", WrapperKind: journey.wrapperKind, ResultMode: journey.resultMode,
		BundleDigest: journey.bundleDigest, PlanDigest: journey.planDigest,
		WrapperSourceSHA256: sourceDigest, StdoutSHA256: digestBytes(invocation.stdout), StderrSHA256: digestBytes(invocation.stderr),
		SourceExitCode: invocation.exitCode, SourceProcessAttempts: 1,
	}}
	evidence.WrapperSourceAttempts = 1
	evidence.ZeroAttemptRejections = 1
	boundaries = append(boundaries, jsonRender.stdout, textRender.stdout, rejection.stdout, rejection.stderr, invocation.stdout, invocation.stderr)
	return evidence, boundaries, nil
}

func createGoTestModule(workRoot string) error {
	module := []byte("module example.com/atsura-artifact-go\n\ngo 1.26.0\n")
	testSource := []byte(`package artifactgo

import (
	"io"
	"os"
	"testing"
)

func TestPass(t *testing.T) {
	path := os.Getenv("ATSURA_GO_TEST_ATTEMPT_LOG")
	if path == "" {
		t.Fatal("attempt log is not configured")
	}
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0o600)
	if err != nil {
		t.Fatal("attempt log could not be opened")
	}
	if _, err := io.WriteString(file, "attempt\n"); err != nil {
		_ = file.Close()
		t.Fatal("attempt log could not be written")
	}
	if err := file.Close(); err != nil {
		t.Fatal("attempt log could not be closed")
	}
}
`)
	if err := writePrivate(filepath.Join(workRoot, "go.mod"), module); err != nil {
		return err
	}
	return writePrivate(filepath.Join(workRoot, "artifact_test.go"), testSource)
}

func requireGoTestAttempts(path string, wanted int) error {
	if wanted < 0 || wanted > 1 {
		return fmt.Errorf("Go source attempt bound is invalid")
	}
	if wanted == 0 {
		if _, err := os.Lstat(path); errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return fmt.Errorf("Go source attempt log exists before execution")
	}
	value, err := readBoundedFile(path, 64)
	if err != nil || string(value) != "attempt\n" {
		return fmt.Errorf("Go source attempt log is invalid")
	}
	return nil
}

func validateWrapperRenderEvidence(document wrapperRenderDocument, journey preparedCommandJourney, ordinaryCommand, executablePath, runtimeDigest string, runtimeSize int64, sourceDigest string) error {
	if document.SchemaVersion != 1 || document.Wrapper.Command != ordinaryCommand || document.Wrapper.Contract.Version != 1 || document.Wrapper.Contract.Shell != "posix" {
		return fmt.Errorf("contract identity mismatch")
	}
	if document.Wrapper.Bundle.Locator != journey.bundlePath() || document.Wrapper.Bundle.Digest != journey.bundleDigest {
		return fmt.Errorf("bundle identity mismatch")
	}
	if document.Wrapper.Runtime.ResolvedPath != executablePath || document.Wrapper.Runtime.SHA256 != runtimeDigest || document.Wrapper.Runtime.Size != runtimeSize {
		return fmt.Errorf("runtime identity mismatch")
	}
	if document.Wrapper.SourceSHA256 != sourceDigest || document.Wrapper.SourceProcessAttempts != 0 {
		return fmt.Errorf("rendered source identity mismatch")
	}
	return nil
}

func (j preparedCommandJourney) bundlePath() string {
	if len(j.baseInvocation) < 2 || j.baseInvocation[0] != "--bundle" {
		return ""
	}
	return j.baseInvocation[1]
}

func decodeWrapperRender(value []byte) (wrapperRenderDocument, error) {
	decoder := json.NewDecoder(bytes.NewReader(value))
	decoder.DisallowUnknownFields()
	var document wrapperRenderDocument
	if err := decoder.Decode(&document); err != nil {
		return wrapperRenderDocument{}, err
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return wrapperRenderDocument{}, fmt.Errorf("wrapper render contains trailing JSON")
	}
	return document, nil
}

func regularFileIdentity(path string) (string, int64, error) {
	file, info, err := openRegularInput(path)
	if err != nil {
		return "", 0, err
	}
	defer file.Close()
	if info.Size() <= 0 || info.Size() > maxMemberBytes {
		return "", 0, fmt.Errorf("file size is invalid")
	}
	hash := sha256.New()
	written, err := io.Copy(hash, io.LimitReader(file, maxMemberBytes+1))
	if err != nil || written != info.Size() {
		return "", 0, fmt.Errorf("file digest failed")
	}
	return fmt.Sprintf("%x", hash.Sum(nil)), info.Size(), nil
}

func differentDigest(value string) string {
	if strings.HasPrefix(value, "0") {
		return "1" + value[1:]
	}
	return "0" + value[1:]
}

func digestBytes(value []byte) string {
	return fmt.Sprintf("%x", sha256.Sum256(value))
}

func runPOSIXCaller(ctx context.Context, runner journeyRunner, ordinaryCommand, wrapperPath string, callerArgs []string, wantedExit int) (commandOutcome, error) {
	if (ordinaryCommand != "gh" && ordinaryCommand != "go") || callerArgs == nil || wantedExit < 0 {
		return commandOutcome{}, fmt.Errorf("generic caller contract is invalid")
	}
	injectionPath := filepath.Join(runner.directory, "atsura-artifact-injection")
	if _, err := os.Lstat(injectionPath); !errors.Is(err, os.ErrNotExist) {
		return commandOutcome{}, fmt.Errorf("generic caller injection canary is not absent")
	}
	runContext, cancel := context.WithTimeout(ctx, commandTimeout)
	defer cancel()
	shellArgs := []string{"-s", "--", wrapperPath}
	shellArgs = append(shellArgs, callerArgs...)
	// /bin/sh is the fixed generic caller fixture. The validated wrapper path
	// and finite hostile argv are positional values; "$@" does not reconstruct
	// them as shell source.
	// #nosec G204 -- every variable value is passed as a positional argument.
	command := exec.CommandContext(runContext, "/bin/sh", shellArgs...)
	command.Dir = runner.directory
	command.Env = replaceEnvironment(runner.environment, map[string]string{fixtureModeEnv: "success"})
	command.Stdin = strings.NewReader("set -eu\n. \"$1\"\nshift\n" + ordinaryCommand + " \"$@\"\n")
	command.WaitDelay = 2 * time.Second
	stdout := &boundedBuffer{limit: maxCommandOutputBytes}
	stderr := &boundedBuffer{limit: maxCommandOutputBytes}
	command.Stdout = stdout
	command.Stderr = stderr
	err := command.Run()
	outcome := commandOutcome{stdout: stdout.Bytes(), stderr: stderr.Bytes(), exitCode: -1}
	if stdout.exceeded || stderr.exceeded || runContext.Err() != nil {
		return commandOutcome{}, fmt.Errorf("generic caller exceeded its bound")
	}
	if command.ProcessState != nil {
		outcome.exitCode = command.ProcessState.ExitCode()
	}
	if outcome.exitCode != wantedExit {
		return commandOutcome{}, fmt.Errorf("generic caller returned an unexpected status")
	}
	if wantedExit == 0 && err != nil {
		return commandOutcome{}, fmt.Errorf("generic caller did not succeed")
	}
	if wantedExit != 0 {
		var exitError *exec.ExitError
		if !errors.As(err, &exitError) {
			return commandOutcome{}, fmt.Errorf("generic caller did not return a conventional source status")
		}
	}
	if err := scanCanaries(outcome.stdout, outcome.stderr); err != nil {
		return commandOutcome{}, err
	}
	if _, err := os.Lstat(injectionPath); !errors.Is(err, os.ErrNotExist) {
		return commandOutcome{}, fmt.Errorf("generic caller evaluated a hostile argv element")
	}
	return outcome, nil
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
	if err != nil || validateSpecificationEvidence(validation.stdout, catalogDigest, 0, 1) != nil {
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
	if err != nil || previewEvidence.SourceProcessAttempts != 0 || !digestValue(previewEvidence.PlanDigest) || previewEvidence.Plan.WrapperKind != "transform" || previewEvidence.Plan.ResultMode != "transformed_json" {
		return preparedCommandJourney{}, fmt.Errorf("adopted preview evidence is invalid")
	}
	if err := requireAttempts(attemptLog, existingAttempts); err != nil {
		return preparedCommandJourney{}, fmt.Errorf("preview zero-attempt evidence is invalid")
	}

	return preparedCommandJourney{
		bundleDigest: bundleDigest, planDigest: previewEvidence.PlanDigest,
		wrapperKind: previewEvidence.Plan.WrapperKind, resultMode: previewEvidence.Plan.ResultMode,
		baseInvocation: baseInvocation, zeroAttemptRejections: zeroAttemptRejections,
		boundaries: [][]byte{draft.stdout, transformedSpecification, validation.stdout, built.stdout, preAdoptionStatus.stdout, adoptedStatus.stdout, preview.stdout},
	}, nil
}

func prepareSourceStreamJourney(
	ctx context.Context,
	runner journeyRunner,
	workRoot, catalogPath, catalogDigest, trustPath, attemptLog string,
	existingAttempts int,
	name string,
	command, invocationArgs []string,
) (preparedCommandJourney, error) {
	if len(command) != 2 || len(invocationArgs) < len(command) || strings.Join(invocationArgs[:len(command)], "\x00") != strings.Join(command, "\x00") {
		return preparedCommandJourney{}, fmt.Errorf("source-stream journey invocation is invalid")
	}
	draftArguments := []string{"spec", "init", "--catalog", catalogPath, "--"}
	draftArguments = append(draftArguments, command...)
	draft, err := runner.success(ctx, "success", draftArguments...)
	if err != nil {
		return preparedCommandJourney{}, fmt.Errorf("source-stream specification draft failed")
	}
	specification, err := transformSourceStreamDraft(draft.stdout, name, command)
	if err != nil {
		return preparedCommandJourney{}, fmt.Errorf("source-stream specification edit failed")
	}
	specificationPath := filepath.Join(workRoot, name+"-specification.yaml")
	if err := writePrivate(specificationPath, specification); err != nil {
		return preparedCommandJourney{}, fmt.Errorf("source-stream specification write failed")
	}
	wantedIdentity, wantedTransform := 0, 1
	if name == "identity" {
		wantedIdentity, wantedTransform = 1, 0
	}
	validation, err := runner.success(ctx, "success", "spec", "validate", "--catalog", catalogPath, "--spec", specificationPath)
	if err != nil || validateSpecificationEvidence(validation.stdout, catalogDigest, wantedIdentity, wantedTransform) != nil {
		return preparedCommandJourney{}, fmt.Errorf("source-stream specification validation evidence is invalid")
	}
	built, err := runner.success(ctx, "success", "bundle", "build", "--catalog", catalogPath, "--spec", specificationPath)
	if err != nil {
		return preparedCommandJourney{}, fmt.Errorf("source-stream bundle build failed")
	}
	bundleDigest, err := decodeBundleDigest(built.stdout)
	if err != nil {
		return preparedCommandJourney{}, fmt.Errorf("source-stream bundle evidence is invalid")
	}
	bundlePath := filepath.Join(workRoot, name+"-bundle.json")
	if err := writePrivate(bundlePath, built.stdout); err != nil {
		return preparedCommandJourney{}, fmt.Errorf("source-stream bundle write failed")
	}
	store := trustfile.New(trustPath)
	changed, err := store.Add(ctx, bundleDigest)
	if err != nil || !changed || store.Inspect(ctx, bundleDigest) != bundletrust.StateAdopted {
		return preparedCommandJourney{}, fmt.Errorf("source-stream exact receipt seeding failed")
	}
	status, err := runner.success(ctx, "success", "bundle", "status", "--bundle", bundlePath)
	if err != nil || validateStatus(status.stdout, bundleDigest, bundletrust.StateAdopted) != nil {
		return preparedCommandJourney{}, fmt.Errorf("source-stream adopted status evidence is invalid")
	}
	baseInvocation := []string{"--bundle", bundlePath, "--", "gh"}
	baseInvocation = append(baseInvocation, invocationArgs...)
	preview, err := runner.success(ctx, "success", append([]string{"bundle", "preview"}, baseInvocation...)...)
	if err != nil {
		return preparedCommandJourney{}, fmt.Errorf("source-stream adopted preview failed")
	}
	previewEvidence, err := decodePreview(preview.stdout)
	wantedKind := "transform"
	if name == "identity" {
		wantedKind = "identity"
	}
	if err != nil || previewEvidence.SourceProcessAttempts != 0 || !digestValue(previewEvidence.PlanDigest) || previewEvidence.Plan.WrapperKind != wantedKind || previewEvidence.Plan.ResultMode != "source_stream_passthrough" {
		return preparedCommandJourney{}, fmt.Errorf("source-stream preview evidence is invalid")
	}
	if err := requireAttempts(attemptLog, existingAttempts); err != nil {
		return preparedCommandJourney{}, fmt.Errorf("source-stream preparation started the source fixture")
	}
	return preparedCommandJourney{
		bundleDigest: bundleDigest, planDigest: previewEvidence.PlanDigest,
		wrapperKind: previewEvidence.Plan.WrapperKind, resultMode: previewEvidence.Plan.ResultMode,
		baseInvocation: baseInvocation,
		boundaries:     [][]byte{draft.stdout, specification, validation.stdout, built.stdout, status.stdout, preview.stdout},
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

func wrapperBundleFileHelpFaults(command, invalidReason string) []helpFaultDeclaration {
	return []helpFaultDeclaration{
		expectedHelpFault("invalid_arguments", "invalid_input", false, "help "+command, invalidReason),
		expectedHelpFault("bundle_file_not_found", "not_found", false, "bundle build", "Build and select a canonical bundle document."),
		expectedHelpFault("bundle_file_permission_denied", "permission", false, "bundle status", "Correct bundle file permissions."),
		expectedHelpFault("unsafe_bundle_file", "invalid_input", false, "bundle build", "Use a stable regular bundle file."),
		expectedHelpFault("bundle_file_too_large", "invalid_input", false, "bundle build", "Build a bundle within the 2 MiB limit."),
		expectedHelpFault("bundle_file_read_failed", "unavailable", true, "bundle status", "Retry after the bundle file is readable."),
		expectedHelpFault("invalid_bundle_file", "invalid_input", false, "bundle build", "Rebuild and review strict canonical bundle JSON."),
		expectedHelpFault("legacy_tailoring_schema", "invalid_input", false, "help bundle build", "Rebuild with a schema-3 specification and bundle schema 2."),
		expectedHelpFault("bundle_digest_mismatch", "rejected", false, "bundle build", "Rebuild and review the changed bundle content."),
	}
}

var wrapperRenderHelpFaults = append(
	wrapperBundleFileHelpFaults("wrapper render", "Pass one exact absolute bundle path and choose text or JSON output."),
	expectedHelpFault("invalid_wrapper_binding", "invalid_input", false, "help wrapper render", "Use an absolute clean bundle path whose requested executable is one portable POSIX command name."),
	expectedHelpFault("wrapper_platform_not_supported", "unsupported", false, "help wrapper render", "Render the POSIX function on a supported Linux or macOS runtime."),
	expectedHelpFault("invalid_bundle_trust_store", "rejected", false, "bundle status", "Repair or reconcile invalid user-local adoption state."),
	expectedHelpFault("bundle_not_adopted", "rejected", false, "bundle trust", "Review and adopt the exact bundle digest before rendering a wrapper."),
	expectedHelpFault("bundle_source_drift", "rejected", false, "bundle status", "Rebuild and adopt current source evidence before rendering a wrapper."),
	expectedHelpFault("source_executable_not_found", "not_found", false, "bundle status", "Reconcile the missing bundle-bound source executable."),
	expectedHelpFault("source_identity_unavailable", "unavailable", true, "bundle status", "Retry after the bundle-bound source identity can be read."),
	expectedHelpFault("unsafe_source_executable", "invalid_input", false, "bundle status", "Select and inspect a supported regular source executable."),
	expectedHelpFault("source_identity_changed", "rejected", false, "bundle status", "Rebuild from stable current source identity evidence."),
	expectedHelpFault("invalid_source_identity", "contract", false, "bundle status", "Repair invalid source identity evidence."),
	expectedHelpFault("wrapper_runtime_not_supported", "unsupported", false, "help wrapper render", "Review the exact adopted-bundle, runtime, surface, and POSIX wrapper requirements."),
	expectedHelpFault("wrapper_runtime_unavailable", "unavailable", false, "help wrapper render", "Retry only after the current Atsura executable identity is readable and stable."),
	expectedHelpFault("wrapper_render_failed", "contract", false, "help wrapper render", "Repair the fixed POSIX renderer or its validated product binding."),
	expectedHelpFault("output_contract_exceeded", "contract", false, "help wrapper render", "Reduce the bounded generated wrapper output."),
	expectedHelpFault("output_encoding_failed", "contract", false, "help wrapper render", "Repair deterministic wrapper review JSON."),
	expectedHelpFault("internal_error", "internal", false, "bundle status", "Inspect bundle, adoption, source, runtime, and renderer wiring."),
	expectedHelpFault("output_write_failed", "internal", true, "wrapper render", "Retry with a writable output stream."),
	expectedHelpFault("operation_canceled", "canceled", true, "wrapper render", "Retry when the caller is ready."),
)

var wrapperRunHelpFaults = append(
	wrapperBundleFileHelpFaults("wrapper run", "Use only the complete render-produced binding flags and forward argv after --."),
	expectedHelpFault("invalid_wrapper_binding", "invalid_input", false, "wrapper render", "Render a complete binding from the exact current bundle and Atsura runtime."),
	expectedHelpFault("wrapper_runtime_unavailable", "unavailable", false, "wrapper render", "Render again only after the current Atsura executable identity is readable and stable."),
	expectedHelpFault("wrapper_runtime_drift", "rejected", false, "wrapper render", "Render a new binding from the exact current Atsura runtime."),
	expectedHelpFault("bundle_binding_mismatch", "rejected", false, "wrapper render", "Render a new wrapper from the exact current bundle bytes."),
	expectedHelpFault("invalid_bundle_trust_store", "rejected", false, "bundle status", "Repair or reconcile invalid user-local adoption state."),
	expectedHelpFault("bundle_not_adopted", "rejected", false, "bundle trust", "Review and adopt the exact bundle digest before execution."),
	expectedHelpFault("bundle_source_drift", "rejected", false, "bundle status", "Rebuild and adopt current source evidence before execution."),
	expectedHelpFault("source_executable_not_found", "not_found", false, "bundle status", "Reconcile the missing bundle-bound source executable."),
	expectedHelpFault("source_identity_unavailable", "unavailable", true, "bundle status", "Retry after the bundle-bound source identity can be read."),
	expectedHelpFault("unsafe_source_executable", "invalid_input", false, "bundle status", "Select and inspect a supported regular source executable."),
	expectedHelpFault("source_identity_changed", "rejected", false, "bundle status", "Rebuild from stable current source identity evidence; do not replay a started operation."),
	expectedHelpFault("invalid_source_identity", "contract", false, "bundle status", "Repair invalid source identity evidence."),
	expectedHelpFault("source_executable_mismatch", "invalid_input", false, "wrapper render", "Render a new wrapper whose command spelling comes from the exact bundle."),
	expectedHelpFault("invalid_invocation", "invalid_input", false, "help wrapper run", "Use a cataloged command path and deterministic observed long-option grammar."),
	expectedHelpFault("command_not_in_surface", "not_found", false, "help wrapper run", "Use a command present in the compiled tailored surface."),
	expectedHelpFault("option_not_in_surface", "not_found", false, "help wrapper run", "Use only options present in the matched command's tailored option surface."),
	expectedHelpFault("invalid_wrapper_plan", "contract", false, "bundle preview", "Inspect the fresh plan and repair incomplete wrapper construction."),
	expectedHelpFault("wrapper_runtime_not_supported", "unsupported", false, "help wrapper run", "Review the supported generated-wrapper runtime contract."),
	expectedHelpFault("invalid_source_process_request", "contract", false, "bundle preview", "Inspect the exact plan-derived source request before execution."),
	expectedHelpFault("source_process_start_failed", "unavailable", true, "wrapper run", "Retry the exact generated invocation only when the result proves no source process started."),
	expectedHelpFault("source_stdout_too_large", "contract", false, "help wrapper run", "Reduce source output within the declared bound; the source was not retried."),
	expectedHelpFault("source_stderr_too_large", "contract", false, "help wrapper run", "Reduce source stderr within the declared bound; the source was not retried."),
	expectedHelpFault("source_execution_canceled", "canceled", false, "bundle status", "Reconcile source-owned effects before considering another invocation."),
	expectedHelpFault("source_command_timeout", "unavailable", false, "bundle status", "Reconcile source-owned effects after the timed-out attempt."),
	expectedHelpFault("source_command_failed", "rejected", false, "help wrapper run", "Inspect the source command independently; Atsura does not expose raw failure output or retry it."),
	expectedHelpFault("source_process_wait_failed", "unavailable", false, "bundle status", "Reconcile source-owned effects after the unclassified wait outcome."),
	expectedHelpFault("source_stderr_not_supported", "contract", false, "help wrapper run", "Use a successful source invocation with empty stderr for this initial transform runtime."),
	expectedHelpFault("source_output_processing_canceled", "canceled", false, "bundle status", "The source already ran; reconcile before considering another invocation."),
	expectedHelpFault("source_json_invalid", "contract", false, "bundle preview", "Repair the source output selector or adapter contract; raw output is not a fallback."),
	expectedHelpFault("output_transform_failed", "contract", false, "bundle preview", "Repair selected fields and typed transform expectations; raw output is not a fallback."),
	expectedHelpFault("unclassified_source_execution_outcome", "contract", false, "bundle status", "Reconcile source-owned effects before considering another invocation."),
	expectedHelpFault("output_contract_exceeded", "contract", false, "bundle preview", "Inspect the bounded fresh-plan result; the source was not retried."),
	expectedHelpFault("output_encoding_failed", "contract", false, "bundle preview", "Repair deterministic compact wrapper JSON; the source was not retried."),
	expectedHelpFault("internal_error", "internal", false, "bundle status", "Inspect wrapper execution wiring without replaying the source."),
	expectedHelpFault("execute_output_write_failed", "internal", false, "bundle status", "The source completed; reconcile before considering another invocation."),
	expectedHelpFault("operation_canceled", "canceled", true, "wrapper run", "Retry only because cancellation occurred before a source attempt."),
)

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
	Name        string                `json:"name"`
	Type        string                `json:"type"`
	Description string                `json:"description"`
	Schema      *helpSchemaProjection `json:"schema"`
}

type helpOutputSchemaReference struct {
	Command string `json:"command"`
	Field   string `json:"field"`
	ID      string `json:"id"`
	Version int    `json:"version"`
}

type helpPlanResultModeProjection struct {
	Mode             string `json:"mode"`
	Stdout           string `json:"stdout"`
	Stderr           string `json:"stderr"`
	ExitStatus       string `json:"exit_status"`
	Framing          string `json:"framing"`
	Projection       string `json:"projection"`
	Delivery         string `json:"delivery"`
	CrossStreamOrder string `json:"cross_stream_order"`
	StdoutLimitBytes int    `json:"stdout_limit_bytes"`
	StderrLimitBytes int    `json:"stderr_limit_bytes"`
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
			Authority          string                         `json:"authority"`
			Formats            []string                       `json:"formats"`
			DefaultFormat      string                         `json:"default_format"`
			Fields             []helpOutputFieldProjection    `json:"fields"`
			Delivery           string                         `json:"delivery"`
			CollectionCoverage string                         `json:"collection_coverage"`
			JSONEnvelope       string                         `json:"json_envelope"`
			JSONSchemaVersion  int                            `json:"json_schema_version"`
			PlanSchema         *helpOutputSchemaReference     `json:"plan_schema"`
			JSONShape          string                         `json:"json_shape"`
			JSONRendering      string                         `json:"json_rendering"`
			JSONFraming        string                         `json:"json_framing"`
			PlanResultModes    []helpPlanResultModeProjection `json:"plan_result_modes"`
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

func typedInput(name, source, valueKind, cardinality string, required bool, allowed ...string) helpInputProjection {
	if allowed == nil {
		allowed = []string{}
	}
	return helpInputProjection{Name: name, Source: source, Required: required, ValueKind: valueKind, Cardinality: cardinality, AllowedValues: allowed}
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

func validateWrapperRenderOutput(command helpCommandProjection) error {
	output := command.Contract.Output
	if output.Authority != "catalog" || strings.Join(output.Formats, ",") != "text,json" || output.DefaultFormat != "text" ||
		output.Delivery != "complete" || output.CollectionCoverage != "not_applicable" ||
		output.JSONEnvelope != "wrapper" || output.JSONSchemaVersion != 1 || output.PlanSchema != nil ||
		output.JSONShape != "" || output.JSONRendering != "" || output.JSONFraming != "" || len(output.Fields) != 7 {
		return fmt.Errorf("wrapper render output contract is invalid")
	}
	wanted := []struct {
		name, fieldType, schemaID string
		fields                    []helpSchemaFieldProjection
	}{
		{name: "source", fieldType: "string"},
		{name: "source_sha256", fieldType: "string"},
		{name: "command", fieldType: "string"},
		{name: "contract", fieldType: "object", schemaID: "wrapper-contract", fields: []helpSchemaFieldProjection{
			schemaField("/shell", "string", true), schemaField("/version", "integer", true),
		}},
		{name: "bundle", fieldType: "object", schemaID: "wrapper-bundle-binding", fields: []helpSchemaFieldProjection{
			schemaField("/digest", "string", true), schemaField("/locator", "string", true),
		}},
		{name: "runtime", fieldType: "object", schemaID: "wrapper-runtime-binding", fields: []helpSchemaFieldProjection{
			schemaField("/resolved_path", "string", true), schemaField("/sha256", "string", true), schemaField("/size", "integer", true),
		}},
		{name: "source_process_attempts", fieldType: "integer"},
	}
	for index, expected := range wanted {
		actual := output.Fields[index]
		if actual.Name != expected.name || actual.Type != expected.fieldType {
			return fmt.Errorf("wrapper render output field %d is invalid", index)
		}
		if expected.schemaID == "" {
			if actual.Schema != nil {
				return fmt.Errorf("wrapper render scalar field has a schema")
			}
			continue
		}
		if actual.Schema == nil || actual.Schema.ID != expected.schemaID || actual.Schema.Version != 1 ||
			len(actual.Schema.Fields) != len(expected.fields) {
			return fmt.Errorf("wrapper render nested schema is invalid")
		}
		for fieldIndex := range expected.fields {
			if actual.Schema.Fields[fieldIndex] != expected.fields[fieldIndex] {
				return fmt.Errorf("wrapper render nested schema field is invalid")
			}
		}
	}
	return nil
}

func validateWrapperRunOutput(command helpCommandProjection) error {
	output := command.Contract.Output
	wantedReference := helpOutputSchemaReference{Command: "bundle preview", Field: "plan", ID: "wrapper-plan", Version: 4}
	wantedModes := []helpPlanResultModeProjection{
		{
			Mode: "transformed_json", Stdout: "compact_json", Stderr: "empty", ExitStatus: "zero",
			Framing: "one_value_lf", Projection: "visible_json", Delivery: "buffered_after_completion", CrossStreamOrder: "not_applicable",
			StdoutLimitBytes: maxTransformedBytes, StderrLimitBytes: 0,
		},
		{
			Mode: "source_stream_passthrough", Stdout: "exact_bounded_source_bytes", Stderr: "exact_bounded_source_bytes", ExitStatus: "source_conventional",
			Framing: "none", Projection: "none", Delivery: "buffered_after_completion", CrossStreamOrder: "not_preserved",
			StdoutLimitBytes: maxCommandOutputBytes, StderrLimitBytes: maxSourceStderrBytes,
		},
	}
	if output.Authority != "fresh_wrapper_plan" || strings.Join(output.Formats, ",") != "plan_result" || output.DefaultFormat != "plan_result" ||
		len(output.Fields) != 0 || output.Delivery != "complete" || output.CollectionCoverage != "not_applicable" ||
		output.JSONEnvelope != "" || output.JSONSchemaVersion != 0 || output.PlanSchema == nil || *output.PlanSchema != wantedReference ||
		output.JSONShape != "" || output.JSONRendering != "" || output.JSONFraming != "" || !equalPlanResultModes(output.PlanResultModes, wantedModes) {
		return fmt.Errorf("wrapper run output authority is invalid")
	}
	return nil
}

func equalPlanResultModes(left, right []helpPlanResultModeProjection) bool {
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
	wanted := []string{"source inspect", "spec init", "spec validate", "bundle preview", "bundle execute", "wrapper render", "wrapper run"}
	if err := validateRootHelp(root.stdout, wanted); err != nil {
		return packagedHelpEvidence{}, fmt.Errorf("packaged root help contract is invalid")
	}
	result := packagedHelpEvidence{outputs: [][]byte{root.stdout}, commands: make(map[string]helpCommandProjection, len(wanted))}
	for _, path := range wanted {
		arguments := append([]string{"help"}, strings.Split(path, " ")...)
		arguments = append(arguments, "--format", "agent")
		output, runErr := runner.success(ctx, "success", arguments...)
		command, validationErr := validateScopedHelp(path, output.stdout)
		if runErr != nil {
			return packagedHelpEvidence{}, fmt.Errorf("packaged scoped help could not be read for %s", path)
		}
		if validationErr != nil {
			return packagedHelpEvidence{}, fmt.Errorf("packaged scoped help contract is invalid for %s: %w", path, validationErr)
		}
		var faultErr error
		switch path {
		case "bundle preview":
			faultErr = validateHelpFaultMatrix(command.Contract.Errors, bundlePreviewHelpFaults)
		case "bundle execute":
			faultErr = validateHelpFaultMatrix(command.Contract.Errors, bundleExecuteHelpFaults)
		case "wrapper render":
			faultErr = validateHelpFaultMatrix(command.Contract.Errors, wrapperRenderHelpFaults)
		case "wrapper run":
			faultErr = validateHelpFaultMatrix(command.Contract.Errors, wrapperRunHelpFaults)
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
	if err := json.Unmarshal(value, &document); err != nil || document.SchemaVersion != 10 || document.View != "index" || document.Program != "atr" {
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
	if err := json.Unmarshal(value, &document); err != nil || document.SchemaVersion != 10 || document.View != "scope" || document.Program != "atr" || document.Scope.Selector != path || document.Scope.Kind != "command" || len(document.Commands) != 1 || document.Commands[0].Path != path {
		return helpCommandProjection{}, fmt.Errorf("invalid scoped agent help")
	}
	command := document.Commands[0]
	prerequisites := strings.Join(command.Contract.Prerequisites, "\n")
	switch path {
	case "source inspect":
		if command.Usage != "atr source inspect --adapter=github-cli|go-cli --executable <path-or-name>" || validateInputs(command.Contract.Inputs, []helpInputProjection{input("--adapter", "flag", "single", "github-cli", "go-cli"), input("--executable", "flag", "single")}) != nil {
			return helpCommandProjection{}, fmt.Errorf("source inspection invocation contract is incomplete")
		}
		if len(command.Contract.Output.Fields) != 3 || command.Contract.Output.Fields[2].Name != "source_process_attempts" ||
			command.Contract.Output.Fields[2].Description != "Exact bounded offline probe attempts: four for github-cli contract 2 and three for go-cli contract 1." {
			return helpCommandProjection{}, fmt.Errorf("source inspection attempt contract is incomplete")
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
	case "wrapper render":
		if command.Usage != "atr wrapper render --bundle <absolute-path> [--format text|json]" || validateInputs(command.Contract.Inputs, []helpInputProjection{
			typedInput("--bundle", "flag", "text", "single", true),
			typedInput("--format", "flag", "text", "single", false, "text", "json"),
		}) != nil || validateWrapperRenderOutput(command) != nil {
			return helpCommandProjection{}, fmt.Errorf("wrapper render contract is incomplete")
		}
		for _, marker := range []string{"Linux or macOS", "portable non-reserved POSIX Name", "complete included surface", "caller-owned"} {
			if !strings.Contains(prerequisites, marker) {
				return helpCommandProjection{}, fmt.Errorf("wrapper render admission marker is missing")
			}
		}
	case "wrapper run":
		if command.Usage != "atr wrapper run --contract-version=1 --bundle=<absolute-path> --bundle-digest=<sha256> --runtime-path=<absolute-path> --runtime-sha256=<sha256> --runtime-size=<bytes> -- [argv]" || validateInputs(command.Contract.Inputs, []helpInputProjection{
			typedInput("--contract-version", "flag", "integer", "single", true, "1"),
			typedInput("--bundle", "flag", "text", "single", true),
			typedInput("--bundle-digest", "flag", "text", "single", true),
			typedInput("--runtime-path", "flag", "text", "single", true),
			typedInput("--runtime-sha256", "flag", "text", "single", true),
			typedInput("--runtime-size", "flag", "integer", "single", true),
			typedInput("argv", "argument", "text", "repeatable", false),
		}) != nil || validateWrapperRunOutput(command) != nil {
			return helpCommandProjection{}, fmt.Errorf("wrapper run contract is incomplete")
		}
		for _, marker := range []string{"complete closure emitted by wrapper render", "exact bundle to remain adopted", "transformed_json", "source_stream_passthrough", "without a shell"} {
			if !strings.Contains(prerequisites, marker) {
				return helpCommandProjection{}, fmt.Errorf("wrapper run admission marker is missing")
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

func stageSourceFixture(sourcePath, goos, workRoot string) (string, string, error) {
	source, info, err := openRegularInput(sourcePath)
	if err != nil {
		return "", "", err
	}
	defer source.Close()
	if info.Size() <= 0 || info.Size() > maxMemberBytes {
		return "", "", fmt.Errorf("source fixture size is invalid")
	}

	binPath := filepath.Join(workRoot, "source-bin")
	if err := os.Mkdir(binPath, 0o700); err != nil {
		return "", "", err
	}
	root, err := os.OpenRoot(binPath)
	if err != nil {
		return "", "", err
	}
	defer root.Close()
	name := "gh"
	if goos == "windows" {
		name = "gh.exe"
	}
	target, err := root.OpenFile(name, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o700)
	if err != nil {
		return "", "", err
	}
	written, copyErr := io.CopyN(target, source, info.Size())
	closeErr := target.Close()
	if copyErr != nil || closeErr != nil || written != info.Size() {
		return "", "", fmt.Errorf("source fixture copy failed")
	}
	var trailing [1]byte
	if count, readErr := source.Read(trailing[:]); count != 0 || (readErr != nil && readErr != io.EOF) {
		return "", "", fmt.Errorf("source fixture changed while copying")
	}
	if err := root.Chmod(name, 0o700); err != nil {
		return "", "", err
	}
	staged, err := root.Stat(name)
	if err != nil || !staged.Mode().IsRegular() || staged.Size() != info.Size() {
		return "", "", fmt.Errorf("staged source fixture is invalid")
	}
	resolvedBin, err := filepath.EvalSymlinks(binPath)
	if err != nil || !filepath.IsAbs(resolvedBin) || filepath.Clean(resolvedBin) != resolvedBin {
		return "", "", fmt.Errorf("staged source directory identity is invalid")
	}
	return filepath.Join(resolvedBin, name), resolvedBin, nil
}

func isolatedEnvironment(workRoot, sourceBin, attemptLog string) (string, []string, error) {
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
		"PATH":        sourceBin,
		"GO111MODULE": "on", "GOENV": "off", "GOEXPERIMENT": "", "GOFIPS140": "off",
		"GOFLAGS": "-buildvcs=false", "GOTOOLCHAIN": "local", "GOWORK": "off",
		"GOPROXY": "off", "GOSUMDB": "off", "CGO_ENABLED": "0",
		"GOCACHE": filepath.Join(workRoot, "go-cache"), "GOMODCACHE": filepath.Join(workRoot, "go-mod-cache"),
		"GOPATH": filepath.Join(workRoot, "go-path"), "LC_ALL": "C",
		fixtureAttemptEnv: attemptLog, fixtureModeEnv: "success",
		goTestAttemptEnv: filepath.Join(workRoot, "go-test-attempts.log"),
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

func validateAttemptSequence(value []byte, goos string) error {
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
	switch goos {
	case "linux", "darwin":
		expected = append(expected,
			fixtureAttemptRecord{
				SchemaVersion: 1, Kind: "runtime", Mode: "success",
				Argv: []string{"pr", "list", "--limit=1", "--json=number,title,state"},
			},
			fixtureAttemptRecord{
				SchemaVersion: 1, Kind: "runtime", Mode: "success",
				Argv: []string{
					"pr", "list",
					"--search=space value;$(touch atsura-artifact-injection)",
					"--label=first", "--label=Unicode 雪", "--repo=-dash",
				},
			},
			fixtureAttemptRecord{
				SchemaVersion: 1, Kind: "runtime", Mode: "success",
				Argv: []string{
					"issue", "list", "--search=append value", "--label=one", "--label=two", "--limit=1",
				},
			},
		)
	case "windows":
	default:
		return fmt.Errorf("attempt sequence platform is unsupported")
	}
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
		{"default: inherit", "default: exclude"},
		{"include: []", "include: [--limit]"},
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

func transformSourceStreamDraft(value []byte, name string, command []string) ([]byte, error) {
	if len(command) != 2 || command[1] != "list" {
		return nil, fmt.Errorf("source-stream draft command is unsupported")
	}
	wantedPrefix := ""
	wantedInclude := ""
	wantedKind := "identity"
	wantedAppend := "[]"
	switch name {
	case "identity":
		if command[0] != "pr" {
			return nil, fmt.Errorf("identity draft command is unsupported")
		}
		wantedPrefix = "Preserve the exact reviewed source streams."
		wantedInclude = "[--label, --repo, --search]"
	case "append_only":
		if command[0] != "issue" {
			return nil, fmt.Errorf("append-only draft command is unsupported")
		}
		wantedPrefix = "Append one fixed reviewed source argument and preserve its streams."
		wantedInclude = "[--label, --search]"
		wantedKind = "transform"
		wantedAppend = `["--limit=1"]`
	default:
		return nil, fmt.Errorf("source-stream draft case is unsupported")
	}
	text := string(value)
	replacements := [][2]string{
		{"reason: Include this verified command without transformation.", "reason: " + wantedPrefix},
		{"default: inherit", "default: exclude"},
		{"include: []", "include: " + wantedInclude},
		{"kind: identity", "kind: " + wantedKind},
		{"append_args: []", "append_args: " + wantedAppend},
	}
	for _, replacement := range replacements {
		if strings.Count(text, replacement[0]) != 1 {
			return nil, fmt.Errorf("source-stream draft shape is not the expected single-command identity wrapper")
		}
		text = strings.Replace(text, replacement[0], replacement[1], 1)
	}
	if strings.Count(text, "- "+command[0]+"\n") != 1 || strings.Count(text, "- "+command[1]+"\n") != 1 || strings.Contains(text, "output:") {
		return nil, fmt.Errorf("source-stream draft command shape is invalid")
	}
	return []byte(text), nil
}

type inspectionEvidence struct {
	CatalogDigest string `json:"catalog_digest"`
	Catalog       struct {
		Adapter struct {
			Kind            string `json:"kind"`
			ContractVersion int    `json:"contract_version"`
		} `json:"adapter"`
		Source struct {
			RequestedExecutable string `json:"requested_executable"`
			ResolvedPath        string `json:"resolved_path"`
			Version             string `json:"version"`
		} `json:"source"`
		Probe struct {
			Attempts int `json:"attempts"`
		} `json:"probe"`
		Commands []struct {
			Path []string `json:"path"`
		} `json:"commands"`
	} `json:"catalog"`
	SourceProcessAttempts int `json:"source_process_attempts"`
}

func inspectionHasCommand(inspection inspectionEvidence, wanted []string) bool {
	matches := 0
	for _, command := range inspection.Catalog.Commands {
		if strings.Join(command.Path, "\x00") == strings.Join(wanted, "\x00") {
			matches++
		}
	}
	return matches == 1
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

func validateSpecificationEvidence(value []byte, catalogDigest string, wantedIdentity, wantedTransform int) error {
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
	if !result.Valid || result.CatalogDigest != catalogDigest || !digestValue(result.SpecificationDigest) || result.CommandCount != 1 || result.IncludedCount != 1 || result.ExcludedCount != 0 || result.IdentityWrapperCount != wantedIdentity || result.TransformWrapperCount != wantedTransform || wantedIdentity+wantedTransform != 1 {
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
	PlanDigest string `json:"plan_digest"`
	Plan       struct {
		WrapperKind string `json:"wrapper_kind"`
		ResultMode  string `json:"result_mode"`
	} `json:"plan"`
	SourceProcessAttempts int `json:"source_process_attempts"`
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
