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
	"reflect"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"time"

	"github.com/tasuku43/atsura/internal/app/processorcompat"
	"github.com/tasuku43/atsura/internal/domain/bundletrust"
	"github.com/tasuku43/atsura/internal/domain/processorprocess"
	"github.com/tasuku43/atsura/internal/domain/sourceprocess"
	"github.com/tasuku43/atsura/internal/domain/tailoringbundle"
	"github.com/tasuku43/atsura/internal/domain/tailoringplan"
	"github.com/tasuku43/atsura/internal/domain/wrapperbinding"
	"github.com/tasuku43/atsura/internal/infra/specyaml"
	"github.com/tasuku43/atsura/internal/infra/trustfile"
	"github.com/tasuku43/atsura/tools/internal/processormanifest"
)

const (
	maxCommandOutputBytes = 4 * 1024 * 1024
	maxEvidenceBytes      = 16 * 1024
	maxAttemptLogBytes    = 1024 * 1024
	maxSourceStderrBytes  = 256 * 1024
	maxTransformedBytes   = 2 * 1024 * 1024
	commandTimeout        = 40 * time.Second
	fixtureAttemptEnv     = "ATSURA_SOURCE_FIXTURE_ATTEMPT_LOG"
	fixtureModeEnv        = "ATSURA_SOURCE_FIXTURE_MODE"
	goTestAttemptEnv      = "ATSURA_GO_TEST_ATTEMPT_LOG"
	goTestModeEnv         = "ATSURA_GO_TEST_MODE"
	goTestProcessorDrift  = "ATSURA_GO_TEST_PROCESSOR_DRIFT_PATH"
	goAdapterKind         = "atsura.source.go_cli"
	goAdapterContract     = 2
)

var (
	go126VersionPattern         = regexp.MustCompile(`^go1\.26\.(0|[1-9][0-9]*)$`)
	goTestIdentityOutputPattern = regexp.MustCompile(`^PASS\nok[ \t]+example\.com/atsura-artifact-go[ \t]+[0-9]+(?:\.[0-9]+)?s\n$`)
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
	Target                      string                   `json:"target"`
	ObservedHost                string                   `json:"observed_host"`
	ArchiveName                 string                   `json:"archive_name"`
	ArchiveSHA256               string                   `json:"archive_sha256"`
	Version                     string                   `json:"version"`
	Revision                    string                   `json:"revision"`
	HelpContractsVerified       int                      `json:"help_contracts_verified"`
	BundleDigest                string                   `json:"bundle_digest"`
	PlanDigest                  string                   `json:"plan_digest"`
	IssueBundleDigest           string                   `json:"issue_bundle_digest"`
	IssuePlanDigest             string                   `json:"issue_plan_digest"`
	WrapperOutcome              string                   `json:"wrapper_outcome"`
	WrapperCases                []wrapperCaseEvidence    `json:"wrapper_cases"`
	WrapperSourceAttempts       int                      `json:"wrapper_source_process_attempts"`
	CommandsVerified            []string                 `json:"commands_verified"`
	SourceInspectionAttempts    int                      `json:"source_inspection_attempts"`
	ZeroAttemptRejections       int                      `json:"zero_attempt_rejections"`
	PostStartFaults             []string                 `json:"post_start_faults"`
	FixtureAttempts             int                      `json:"fixture_attempts"`
	CredentialEnvironmentAbsent bool                     `json:"credential_environment_absent"`
	SecretCanariesAbsent        bool                     `json:"secret_canaries_absent"`
	TailoredHelp                tailoredHelpEvidence     `json:"tailored_help"`
	GoSource                    goSourceEvidence         `json:"go_source"`
	WrapperLifecycle            wrapperLifecycleEvidence `json:"wrapper_lifecycle"`
}

type tailoredHelpEvidence struct {
	Outcome                           string                      `json:"outcome"`
	BundleDigest                      string                      `json:"bundle_digest"`
	WrapperSourceSHA256               string                      `json:"wrapper_source_sha256"`
	WrapperContractVersion            int                         `json:"wrapper_contract_version"`
	Views                             []tailoredHelpViewEvidence  `json:"views"`
	FallthroughFaults                 []tailoredHelpFaultEvidence `json:"fallthrough_faults"`
	RuntimeNonExecutableDuringSuccess bool                        `json:"runtime_non_executable_during_success"`
	SourceProcessAttempts             int                         `json:"source_process_attempts"`
	ProcessorProcessAttempts          int                         `json:"processor_process_attempts"`
}

type tailoredHelpViewEvidence struct {
	Name         string   `json:"name"`
	Argv         []string `json:"argv"`
	StdoutSHA256 string   `json:"stdout_sha256"`
	StderrSHA256 string   `json:"stderr_sha256"`
}

type tailoredHelpFaultEvidence struct {
	Name                     string   `json:"name"`
	Argv                     []string `json:"argv"`
	Code                     string   `json:"code"`
	SourceProcessAttempts    int      `json:"source_process_attempts"`
	ProcessorProcessAttempts int      `json:"processor_process_attempts"`
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
	Optimizer                goOptimizerEvidence   `json:"optimizer"`
}

type goOptimizerEvidence struct {
	Outcome               string                      `json:"outcome"`
	Processor             *processorArtifactEvidence  `json:"processor,omitempty"`
	Execution             *optimizerExecutionEvidence `json:"execution,omitempty"`
	BundleDigest          string                      `json:"bundle_digest,omitempty"`
	PlanDigest            string                      `json:"plan_digest,omitempty"`
	WrapperSourceSHA256   string                      `json:"wrapper_source_sha256,omitempty"`
	Cases                 []optimizerCaseEvidence     `json:"cases"`
	Faults                []optimizerFaultEvidence    `json:"faults"`
	SourceProcessAttempts int                         `json:"source_process_attempts"`
	ZeroAttemptRejections int                         `json:"zero_attempt_rejections"`
}

type optimizerExecutionEvidence struct {
	CallerArgv                    []string `json:"caller_argv"`
	SourceArgv                    []string `json:"source_argv"`
	SourceStdinMode               string   `json:"source_stdin_mode"`
	SourceWorkingDirectoryMode    string   `json:"source_working_directory_mode"`
	SourceEnvironmentMode         string   `json:"source_environment_mode"`
	SourceMaxAttempts             int      `json:"source_max_attempts"`
	SourceTimeoutMillis           int64    `json:"source_timeout_millis"`
	SourceStdoutLimitBytes        int      `json:"source_stdout_limit_bytes"`
	SourceStderrLimitBytes        int      `json:"source_stderr_limit_bytes"`
	InputFormat                   string   `json:"input_format"`
	OutputFormat                  string   `json:"output_format"`
	AllowOriginalOutput           bool     `json:"allow_original_output"`
	ProcessorArgv                 []string `json:"processor_argv"`
	ProcessorStdinMode            string   `json:"processor_stdin_mode"`
	ProcessorWorkingDirectoryMode string   `json:"processor_working_directory_mode"`
	ProcessorEnvironmentContract  string   `json:"processor_environment_contract"`
	ProcessorMaxAttempts          int      `json:"processor_max_attempts"`
	ProcessorTimeoutMillis        int64    `json:"processor_timeout_millis"`
	ProcessorStdoutLimitBytes     int      `json:"processor_stdout_limit_bytes"`
	ProcessorStderrLimitBytes     int      `json:"processor_stderr_limit_bytes"`
}

type processorArtifactEvidence struct {
	ContractID                string `json:"contract_id"`
	AdapterKind               string `json:"adapter_kind"`
	AdapterContractVersion    int    `json:"adapter_contract_version"`
	Version                   string `json:"version"`
	Target                    string `json:"target"`
	ArchiveName               string `json:"archive_name"`
	ArchiveSHA256             string `json:"archive_sha256"`
	BinarySHA256              string `json:"binary_sha256"`
	BinarySize                int64  `json:"binary_size"`
	ObservationDigest         string `json:"observation_digest"`
	InspectionProcessAttempts int    `json:"inspection_process_attempts"`
}

type optimizerCaseEvidence struct {
	Name                  string `json:"name"`
	Disposition           string `json:"disposition"`
	StdoutSHA256          string `json:"stdout_sha256"`
	StderrSHA256          string `json:"stderr_sha256"`
	SourceExitCode        int    `json:"source_exit_code"`
	SourceProcessAttempts int    `json:"source_process_attempts"`
}

type optimizerFaultEvidence struct {
	Name                  string `json:"name"`
	Code                  string `json:"code"`
	SourceProcessAttempts int    `json:"source_process_attempts"`
}

type preparedProcessor struct {
	executablePath string
	metadata       processormanifest.TargetMetadata
}

// wrapperCaseEvidence retains only argv and defaulting facts, identities,
// digests, conventional status, and attempt counts. Source bytes remain
// confined to the native replay.
type wrapperCaseEvidence struct {
	Name                  string                          `json:"name"`
	WrapperKind           string                          `json:"wrapper_kind"`
	ResultMode            string                          `json:"result_mode"`
	CallerArgv            []string                        `json:"caller_argv"`
	SourceArgv            []string                        `json:"source_argv"`
	OptionDefaults        []tailoringbundle.OptionDefault `json:"option_defaults"`
	AppliedOptionDefaults []tailoringbundle.OptionDefault `json:"applied_option_defaults"`
	BundleDigest          string                          `json:"bundle_digest"`
	PlanDigest            string                          `json:"plan_digest"`
	WrapperSourceSHA256   string                          `json:"wrapper_source_sha256"`
	StdoutSHA256          string                          `json:"stdout_sha256"`
	StderrSHA256          string                          `json:"stderr_sha256"`
	SourceExitCode        int                             `json:"source_exit_code"`
	SourceProcessAttempts int                             `json:"source_process_attempts"`
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
	archive, digest, err := readReleaseArchive(archivePath)
	if err != nil {
		return evidenceDocument{}, fmt.Errorf("archive digest validation failed")
	}
	workRoot, err := os.MkdirTemp("", "atsura-artifact-journey-")
	if err != nil {
		return evidenceDocument{}, fmt.Errorf("temporary workspace creation failed")
	}
	defer os.RemoveAll(workRoot)
	executablePath, err := extractReleaseArchive(archive, configuration.goos, filepath.Join(workRoot, "artifact"))
	if err != nil {
		return evidenceDocument{}, fmt.Errorf("release archive extraction failed")
	}
	executablePath, err = filepath.EvalSymlinks(executablePath)
	if err != nil || !filepath.IsAbs(executablePath) || filepath.Clean(executablePath) != executablePath {
		return evidenceDocument{}, fmt.Errorf("release executable identity is invalid")
	}
	var processor *preparedProcessor
	if configuration.goos != "windows" {
		manifest, loadErr := processormanifest.LoadPinned(configuration.repositoryRoot)
		if loadErr != nil {
			return evidenceDocument{}, fmt.Errorf("pinned processor provenance is invalid")
		}
		metadata, targetErr := manifest.Target(configuration.goos + "/" + configuration.goarch)
		if targetErr != nil {
			return evidenceDocument{}, fmt.Errorf("processor target provenance is unavailable")
		}
		processorArchivePath, inputErr := prepareInputFile(configuration.processorArchive)
		if inputErr != nil {
			return evidenceDocument{}, fmt.Errorf("processor archive input is invalid")
		}
		processorExecutable, extractErr := extractProcessorArchive(processorArchivePath, filepath.Join(workRoot, "processor"), metadata)
		if extractErr != nil {
			return evidenceDocument{}, fmt.Errorf("processor archive extraction failed: %w", extractErr)
		}
		processorExecutable, extractErr = filepath.EvalSymlinks(processorExecutable)
		if extractErr != nil || !filepath.IsAbs(processorExecutable) || filepath.Clean(processorExecutable) != processorExecutable {
			return evidenceDocument{}, fmt.Errorf("processor executable identity is invalid")
		}
		processor = &preparedProcessor{executablePath: processorExecutable, metadata: metadata}
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
		inspectionPayload.Catalog.SchemaVersion != 2 ||
		inspectionPayload.Catalog.Source.RequestedExecutable != "gh" || inspectionPayload.Catalog.Source.ResolvedPath != sourcePath {
		return evidenceDocument{}, fmt.Errorf("source inspection evidence is invalid")
	}
	if err := writePrivate(catalogPath, inspection.stdout); err != nil {
		return evidenceDocument{}, fmt.Errorf("catalog evidence write failed")
	}
	if err := requireAttempts(attemptLog, 4); err != nil {
		return evidenceDocument{}, fmt.Errorf("source inspection attempt evidence is invalid")
	}

	prJourney, multiIssueJourney, multiCommandBoundaries, err := prepareMultiCommandWrapperJourneys(
		ctx, runner, helpEvidence, workRoot, catalogPath, inspectionPayload.CatalogDigest,
		trustPath, attemptLog, 4,
	)
	if err != nil {
		return evidenceDocument{}, fmt.Errorf("multi-command pull-request journey preparation failed: %w", err)
	}
	var prOverrideJourney preparedCommandJourney
	if configuration.goos == "linux" || configuration.goos == "darwin" {
		prOverrideJourney, err = prepareDefaultOverrideJourney(ctx, runner, prJourney, attemptLog, 4)
		if err != nil {
			return evidenceDocument{}, fmt.Errorf("multi-command pull-request override preparation failed: %w", err)
		}
		multiCommandBoundaries = append(multiCommandBoundaries, prOverrideJourney.boundaries...)
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

	if multiIssueJourney.bundleDigest == issueJourney.bundleDigest || multiIssueJourney.planDigest == issueJourney.planDigest {
		return evidenceDocument{}, fmt.Errorf("multi-command issue evidence is not distinct from direct issue evidence")
	}
	wrapperInputs := []packagedWrapperInput{{
		name: "default_applied", journey: prJourney,
		callerArgs: []string{"pr", "list"},
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
		appendArgs := []string{"issue", "list", "--search=append value", "--label=one", "--label=two"}
		wrapperInputs = append(wrapperInputs,
			packagedWrapperInput{
				name: "default_overridden", journey: prOverrideJourney, callerArgs: []string{"pr", "list", "--limit=2"},
				wantStdout: []byte("[{\"id\":101,\"title\":\"Review policy\",\"state\":\"OPEN\"}]\n"),
				wantStderr: []byte{}, wantExitCode: 0,
			},
			packagedWrapperInput{
				name: "append_only", journey: multiIssueJourney, callerArgs: appendArgs,
				wantStdout: []byte{'A', 'P', 'P', ':', 0xff, 0x00},
				wantStderr: []byte("APPERR:\n"), wantExitCode: 23,
			},
			packagedWrapperInput{
				name: "identity", journey: identityJourney, callerArgs: identityArgs,
				wantStdout: []byte{'I', 'D', ':', 0x00, 0xff, '\n'},
				wantStderr: []byte{'I', 'D', 'E', 'R', 'R', ':', 0xfe}, wantExitCode: 0,
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
		ctx, runner, helpEvidence, configuration.goos, executablePath, trustPath, attemptLog, wantedAttempts, workRoot, processor,
	)
	if err != nil {
		return evidenceDocument{}, err
	}
	wrapperLifecycle, wrapperLifecycleBoundaries, addedFixtureAttempts, err := verifyPersistentWrapperLifecycle(
		ctx, runner, helpEvidence, configuration.goos, executablePath, workRoot, attemptLog, wantedAttempts,
		prJourney, wrapperEvidence.cases, goEvidence,
	)
	if err != nil {
		return evidenceDocument{}, err
	}
	wantedAttempts += addedFixtureAttempts

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
	canaryBoundaries = append(canaryBoundaries, wrapperLifecycleBoundaries...)
	canaryBoundaries = append(canaryBoundaries, prJourney.boundaries...)
	canaryBoundaries = append(canaryBoundaries, issueJourney.boundaries...)
	canaryBoundaries = append(canaryBoundaries, multiCommandBoundaries...)
	canaryBoundaries = append(canaryBoundaries, helpEvidence.outputs...)
	if err := scanCanaries(canaryBoundaries...); err != nil {
		return evidenceDocument{}, err
	}
	if err := validateAttemptSequence(attemptBytes, configuration.goos); err != nil {
		return evidenceDocument{}, fmt.Errorf("fixture attempt sequence is invalid")
	}

	return evidenceDocument{SchemaVersion: 9, ArtifactJourney: artifactJourneyEvidence{
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
		TailoredHelp:     wrapperEvidence.tailoredHelp,
		GoSource:         goEvidence,
		WrapperLifecycle: wrapperLifecycle,
	}}, nil
}

type preparedCommandJourney struct {
	bundleLocator         string
	bundleDigest          string
	planDigest            string
	wrapperKind           string
	resultMode            string
	baseInvocation        []string
	sourceArgv            []string
	optionDefaults        []tailoringbundle.OptionDefault
	appliedOptionDefaults []tailoringbundle.OptionDefault
	zeroAttemptRejections int
	boundaries            [][]byte
}

type packagedWrapperEvidence struct {
	outcome               string
	cases                 []wrapperCaseEvidence
	tailoredHelp          tailoredHelpEvidence
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
		SourceProcessAttempts    int `json:"source_process_attempts"`
		ProcessorProcessAttempts int `json:"processor_process_attempts"`
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
	if len(inputs) == 0 || inputs[0].name != "default_applied" {
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
			tailoredHelp: tailoredHelpEvidence{
				Outcome: "platform_not_supported", Views: []tailoredHelpViewEvidence{},
				FallthroughFaults: []tailoredHelpFaultEvidence{},
			},
			boundaries: [][]byte{failure.stdout, failure.stderr},
		}, nil
	}
	if goos != "linux" && goos != "darwin" {
		return packagedWrapperEvidence{}, fmt.Errorf("wrapper journey target is unsupported")
	}
	if len(inputs) != 4 || inputs[1].name != "default_overridden" || inputs[2].name != "append_only" || inputs[3].name != "identity" {
		return packagedWrapperEvidence{}, fmt.Errorf("POSIX wrapper case inventory is invalid")
	}
	for index, input := range inputs {
		if input.journey.wrapperKind == "" || input.journey.resultMode == "" || input.callerArgs == nil || input.wantStdout == nil || input.wantStderr == nil || input.wantExitCode < 0 {
			return packagedWrapperEvidence{}, fmt.Errorf("wrapper case %d is incomplete", index)
		}
	}
	if inputs[0].journey.bundleDigest != inputs[1].journey.bundleDigest ||
		inputs[0].journey.bundleDigest != inputs[2].journey.bundleDigest ||
		inputs[0].journey.bundlePath() != inputs[1].journey.bundlePath() ||
		inputs[0].journey.bundlePath() != inputs[2].journey.bundlePath() ||
		inputs[0].journey.planDigest == inputs[1].journey.planDigest ||
		inputs[0].journey.planDigest == inputs[2].journey.planDigest ||
		inputs[1].journey.planDigest == inputs[2].journey.planDigest ||
		inputs[0].journey.wrapperKind != "transform" || inputs[0].journey.resultMode != "transformed_json" ||
		inputs[1].journey.wrapperKind != "transform" || inputs[1].journey.resultMode != "transformed_json" ||
		inputs[2].journey.wrapperKind != "transform" || inputs[2].journey.resultMode != "source_stream_passthrough" {
		return packagedWrapperEvidence{}, fmt.Errorf("multi-command wrapper binding is invalid")
	}
	if inputs[3].journey.bundleDigest == inputs[0].journey.bundleDigest ||
		inputs[3].journey.planDigest == inputs[0].journey.planDigest ||
		inputs[3].journey.planDigest == inputs[1].journey.planDigest ||
		inputs[3].journey.planDigest == inputs[2].journey.planDigest ||
		inputs[3].journey.wrapperKind != "identity" || inputs[3].journey.resultMode != "source_stream_passthrough" {
		return packagedWrapperEvidence{}, fmt.Errorf("identity wrapper evidence is not independent")
	}
	runtimeDigest, runtimeSize, err := regularFileIdentity(executablePath)
	if err != nil {
		return packagedWrapperEvidence{}, fmt.Errorf("packaged runtime identity is unreadable")
	}

	result := packagedWrapperEvidence{
		outcome: "ordinary_command_verified", cases: make([]wrapperCaseEvidence, 0, len(inputs)),
		zeroAttemptRejections: 1, boundaries: make([][]byte, 0, len(inputs)*8),
	}
	for _, input := range inputs {
		result.boundaries = append(result.boundaries, input.journey.boundaries...)
	}
	render := func(input packagedWrapperInput, wantedAttempts int) (wrapperRenderDocument, []byte, string, error) {
		jsonRender, renderErr := runner.success(ctx, "success", "wrapper", "render", "--bundle", input.journey.bundlePath(), "--format", "json")
		if renderErr != nil {
			return wrapperRenderDocument{}, nil, "", fmt.Errorf("wrapper %s JSON rendering failed", input.name)
		}
		document, decodeErr := decodeWrapperRender(jsonRender.stdout)
		if decodeErr != nil {
			return wrapperRenderDocument{}, nil, "", fmt.Errorf("wrapper %s JSON evidence is invalid", input.name)
		}
		textRender, renderErr := runner.success(ctx, "success", "wrapper", "render", "--bundle", input.journey.bundlePath())
		if renderErr != nil {
			return wrapperRenderDocument{}, nil, "", fmt.Errorf("wrapper %s text rendering failed", input.name)
		}
		if string(textRender.stdout) != document.Wrapper.Source {
			return wrapperRenderDocument{}, nil, "", fmt.Errorf("wrapper %s text and JSON source differ", input.name)
		}
		sourceDigest := digestBytes(textRender.stdout)
		if err := validateWrapperRenderEvidence(document, input.journey, "gh", executablePath, runtimeDigest, runtimeSize, sourceDigest); err != nil {
			return wrapperRenderDocument{}, nil, "", fmt.Errorf("wrapper %s render binding evidence is invalid: %w", input.name, err)
		}
		if err := requireAttempts(attemptLog, wantedAttempts); err != nil {
			return wrapperRenderDocument{}, nil, "", fmt.Errorf("wrapper %s rendering started the source fixture", input.name)
		}
		result.boundaries = append(result.boundaries, jsonRender.stdout, textRender.stdout)
		return document, textRender.stdout, sourceDigest, nil
	}

	sharedDocument, sharedSource, sharedDigest, err := render(inputs[0], existingAttempts)
	if err != nil {
		return packagedWrapperEvidence{}, err
	}
	for _, input := range inputs[1:3] {
		if err := validateWrapperRenderEvidence(sharedDocument, input.journey, "gh", executablePath, runtimeDigest, runtimeSize, sharedDigest); err != nil {
			return packagedWrapperEvidence{}, fmt.Errorf("shared wrapper does not bind the %s plan bundle: %w", input.name, err)
		}
	}
	declaration, declarationErr := help.fault("wrapper run", "bundle_binding_mismatch")
	if declarationErr != nil || declaration.Kind != "rejected" || declaration.Retryable {
		return packagedWrapperEvidence{}, fmt.Errorf("wrapper binding help contract is invalid")
	}
	badDigest := differentDigest(sharedDocument.Wrapper.Bundle.Digest)
	rejectionArgs := []string{
		"--error-format=json", "wrapper", "run",
		fmt.Sprintf("--contract-version=%d", wrapperbinding.ContractVersion),
		"--bundle=" + sharedDocument.Wrapper.Bundle.Locator,
		"--bundle-digest=" + badDigest,
		"--runtime-path=" + sharedDocument.Wrapper.Runtime.ResolvedPath,
		"--runtime-sha256=" + sharedDocument.Wrapper.Runtime.SHA256,
		fmt.Sprintf("--runtime-size=%d", sharedDocument.Wrapper.Runtime.Size),
		"--",
	}
	rejectionArgs = append(rejectionArgs, inputs[0].callerArgs...)
	rejected, rejectionErr := runner.failure(ctx, "success", 10, declaration, rejectionArgs...)
	if rejectionErr != nil {
		return packagedWrapperEvidence{}, fmt.Errorf("wrapper binding rejection evidence is invalid")
	}
	if err := requireAttempts(attemptLog, existingAttempts); err != nil {
		return packagedWrapperEvidence{}, fmt.Errorf("wrapper binding rejection started the source fixture")
	}
	result.boundaries = append(result.boundaries, rejected.stdout, rejected.stderr)

	sharedWrapperPath := filepath.Join(workRoot, "caller-owned-multi-command-wrapper.sh")
	if err := writePrivate(sharedWrapperPath, sharedSource); err != nil {
		return packagedWrapperEvidence{}, fmt.Errorf("multi-command caller-owned fixture write failed")
	}
	sharedFile, err := snapshotRegularFile(sharedWrapperPath)
	if err != nil || sharedFile.sha256 != sharedDigest || sharedFile.size != int64(len(sharedSource)) {
		return packagedWrapperEvidence{}, fmt.Errorf("multi-command caller-owned fixture identity is invalid")
	}
	if err := sharedFile.verify(sharedWrapperPath); err != nil {
		return packagedWrapperEvidence{}, fmt.Errorf("multi-command caller-owned fixture changed before help")
	}
	tailoredHelp, boundaries, helpErr := verifyPackagedTailoredHelp(
		ctx, runner, help, executablePath, sharedWrapperPath, inputs[0].journey.bundleDigest,
		sharedDigest, attemptLog, existingAttempts,
	)
	if helpErr != nil {
		return packagedWrapperEvidence{}, helpErr
	}
	if err := sharedFile.verify(sharedWrapperPath); err != nil {
		return packagedWrapperEvidence{}, fmt.Errorf("multi-command caller-owned fixture changed during help")
	}
	result.tailoredHelp = tailoredHelp
	result.boundaries = append(result.boundaries, boundaries...)

	invoke := func(input packagedWrapperInput, wrapperPath, sourceDigest string, file regularFileSnapshot, wantedAttempts int) error {
		if err := file.verify(wrapperPath); err != nil {
			return fmt.Errorf("wrapper %s caller-owned fixture changed before invocation", input.name)
		}
		invocation, invokeErr := runPOSIXCaller(ctx, runner, "gh", wrapperPath, input.callerArgs, input.wantExitCode)
		if invokeErr != nil {
			return fmt.Errorf("wrapper %s caller-owned invocation failed", input.name)
		}
		if err := file.verify(wrapperPath); err != nil {
			return fmt.Errorf("wrapper %s caller-owned fixture changed during invocation", input.name)
		}
		if !bytes.Equal(invocation.stdout, input.wantStdout) || !bytes.Equal(invocation.stderr, input.wantStderr) || invocation.exitCode != input.wantExitCode {
			return fmt.Errorf("wrapper %s caller-owned result is invalid", input.name)
		}
		if err := requireAttempts(attemptLog, wantedAttempts); err != nil {
			return fmt.Errorf("wrapper %s attempt evidence is invalid", input.name)
		}
		result.cases = append(result.cases, wrapperCaseEvidence{
			Name: input.name, WrapperKind: input.journey.wrapperKind, ResultMode: input.journey.resultMode,
			CallerArgv: append([]string{}, input.callerArgs...), SourceArgv: append([]string{}, input.journey.sourceArgv...),
			OptionDefaults:        cloneOptionDefaultEvidence(input.journey.optionDefaults),
			AppliedOptionDefaults: cloneOptionDefaultEvidence(input.journey.appliedOptionDefaults),
			BundleDigest:          input.journey.bundleDigest, PlanDigest: input.journey.planDigest,
			WrapperSourceSHA256: sourceDigest, StdoutSHA256: digestBytes(invocation.stdout), StderrSHA256: digestBytes(invocation.stderr),
			SourceExitCode: invocation.exitCode, SourceProcessAttempts: 1,
		})
		result.sourceProcessAttempts++
		result.boundaries = append(result.boundaries, invocation.stdout, invocation.stderr)
		return nil
	}
	if err := invoke(inputs[0], sharedWrapperPath, sharedDigest, sharedFile, existingAttempts+1); err != nil {
		return packagedWrapperEvidence{}, err
	}
	if err := invoke(inputs[1], sharedWrapperPath, sharedDigest, sharedFile, existingAttempts+2); err != nil {
		return packagedWrapperEvidence{}, err
	}
	if err := invoke(inputs[2], sharedWrapperPath, sharedDigest, sharedFile, existingAttempts+3); err != nil {
		return packagedWrapperEvidence{}, err
	}

	_, identitySource, identityDigest, err := render(inputs[3], existingAttempts+3)
	if err != nil {
		return packagedWrapperEvidence{}, err
	}
	if identityDigest == sharedDigest {
		return packagedWrapperEvidence{}, fmt.Errorf("identity wrapper source digest is not independent")
	}
	identityWrapperPath := filepath.Join(workRoot, "caller-owned-identity-wrapper.sh")
	if err := writePrivate(identityWrapperPath, identitySource); err != nil {
		return packagedWrapperEvidence{}, fmt.Errorf("identity caller-owned fixture write failed")
	}
	identityFile, err := snapshotRegularFile(identityWrapperPath)
	if err != nil || identityFile.sha256 != identityDigest || identityFile.size != int64(len(identitySource)) {
		return packagedWrapperEvidence{}, fmt.Errorf("identity caller-owned fixture identity is invalid")
	}
	if err := invoke(inputs[3], identityWrapperPath, identityDigest, identityFile, existingAttempts+4); err != nil {
		return packagedWrapperEvidence{}, err
	}
	return result, nil
}

func verifyPackagedTailoredHelp(
	ctx context.Context,
	runner journeyRunner,
	help packagedHelpEvidence,
	executablePath, wrapperPath, bundleDigest, wrapperSourceDigest, attemptLog string,
	existingAttempts int,
) (tailoredHelpEvidence, [][]byte, error) {
	if !digestValue(bundleDigest) || !digestValue(wrapperSourceDigest) {
		return tailoredHelpEvidence{}, nil, fmt.Errorf("tailored help binding identity is invalid")
	}
	tests := []struct {
		name string
		argv []string
	}{
		{name: "root", argv: []string{"--help"}},
		{name: "issue_namespace", argv: []string{"issue", "--help"}},
		{name: "issue_exact_command", argv: []string{"issue", "list", "--help"}},
		{name: "pr_namespace", argv: []string{"pr", "--help"}},
		{name: "pr_exact_command", argv: []string{"pr", "list", "--help"}},
	}
	views := make([]tailoredHelpViewEvidence, 0, len(tests))
	boundaries := make([][]byte, 0, len(tests)*2+4)
	err := withNonExecutableRuntime(executablePath, func() error {
		for _, test := range tests {
			wanted, err := expectedTailoredHelp(bundleDigest, test.name)
			if err != nil {
				return err
			}
			outcome, err := runPOSIXCaller(ctx, runner, "gh", wrapperPath, test.argv, 0)
			if err != nil || !bytes.Equal(outcome.stdout, wanted) || len(outcome.stderr) != 0 {
				return fmt.Errorf("tailored help %s view is invalid", test.name)
			}
			if err := requireAttempts(attemptLog, existingAttempts); err != nil {
				return fmt.Errorf("tailored help %s view started the source fixture", test.name)
			}
			views = append(views, tailoredHelpViewEvidence{
				Name: test.name, Argv: append([]string{}, test.argv...),
				StdoutSHA256: digestBytes(outcome.stdout), StderrSHA256: digestBytes(outcome.stderr),
			})
			boundaries = append(boundaries, outcome.stdout, outcome.stderr)
		}
		return nil
	})
	if err != nil {
		return tailoredHelpEvidence{}, nil, fmt.Errorf("non-executable-runtime tailored help failed: %w", err)
	}

	faultTests := []struct {
		name, code, kind string
		argv             []string
		exit             int
	}{
		{name: "hidden_command", code: "command_not_in_surface", kind: "not_found", argv: []string{"api", "--help"}, exit: 6},
		{name: "unknown_selector", code: "invalid_invocation", kind: "invalid_input", argv: []string{"unknown", "--help"}, exit: 2},
	}
	faults := make([]tailoredHelpFaultEvidence, 0, len(faultTests))
	for _, test := range faultTests {
		declaration, err := help.fault("wrapper run", test.code)
		if err != nil || declaration.Kind != test.kind || declaration.Retryable {
			return tailoredHelpEvidence{}, nil, fmt.Errorf("tailored help %s fault contract is invalid", test.name)
		}
		outcome, err := runPOSIXCaller(ctx, runner, "gh", wrapperPath, test.argv, test.exit)
		if err != nil || len(outcome.stdout) != 0 || validateFault(outcome.stderr, declaration) != nil {
			return tailoredHelpEvidence{}, nil, fmt.Errorf("tailored help %s fallthrough is invalid", test.name)
		}
		if err := requireAttempts(attemptLog, existingAttempts); err != nil {
			return tailoredHelpEvidence{}, nil, fmt.Errorf("tailored help %s fallthrough started the source fixture", test.name)
		}
		faults = append(faults, tailoredHelpFaultEvidence{
			Name: test.name, Argv: append([]string{}, test.argv...), Code: test.code,
			SourceProcessAttempts: 0, ProcessorProcessAttempts: 0,
		})
		boundaries = append(boundaries, outcome.stdout, outcome.stderr)
	}
	return tailoredHelpEvidence{
		Outcome: "compiled_views_verified", BundleDigest: bundleDigest,
		WrapperSourceSHA256: wrapperSourceDigest, WrapperContractVersion: wrapperbinding.ContractVersion,
		Views: views, FallthroughFaults: faults, RuntimeNonExecutableDuringSuccess: true,
		SourceProcessAttempts: 0, ProcessorProcessAttempts: 0,
	}, boundaries, nil
}

func expectedTailoredHelp(bundleDigest, view string) ([]byte, error) {
	if !digestValue(bundleDigest) {
		return nil, fmt.Errorf("tailored help bundle digest is invalid")
	}
	lines := []string{
		"Atsura tailored help",
		"Bundle digest: " + bundleDigest,
	}
	switch view {
	case "root":
		lines = append(lines, "Commands:", "  issue list", "  pr list")
	case "issue_namespace":
		lines = append(lines, "Commands:", "  issue list")
	case "pr_namespace":
		lines = append(lines, "Commands:", "  pr list")
	case "issue_exact_command":
		lines = append(lines,
			"Command: issue list",
			"Source summary: List issues",
			"Tailoring reason: Append one fixed reviewed source argument and preserve its streams.",
			"Options:",
			"  --label=<value> (value required)",
			"  --search=<value> (value required)",
		)
	case "pr_exact_command":
		lines = append(lines,
			"Command: pr list",
			"Source summary: List pull requests",
			"Tailoring reason: Return one reviewed compact result.",
			"Options:",
			"  --limit=<value> (value required; default when omitted: \"30\")",
		)
	default:
		return nil, fmt.Errorf("tailored help view is unsupported")
	}
	return []byte(strings.Join(lines, "\n") + "\n"), nil
}

func withNonExecutableRuntime(path string, action func() error) error {
	if action == nil {
		return fmt.Errorf("non-executable runtime action is required")
	}
	root, name, err := openParentRoot(path)
	if err != nil {
		return err
	}
	defer root.Close()
	before, err := root.Lstat(name)
	if err != nil || before.Mode()&os.ModeSymlink != 0 || !before.Mode().IsRegular() || before.Mode().Perm()&0o111 == 0 {
		return fmt.Errorf("runtime executable mode is invalid")
	}
	originalMode := before.Mode().Perm()
	if err := root.Chmod(name, originalMode&^0o111); err != nil {
		return fmt.Errorf("runtime executable mode could not be disabled")
	}
	restore := func(actionErr error) error {
		current, currentErr := root.Lstat(name)
		if currentErr != nil || !os.SameFile(before, current) || !current.Mode().IsRegular() {
			return errors.Join(actionErr, fmt.Errorf("runtime identity changed before mode restoration"))
		}
		if err := root.Chmod(name, originalMode); err != nil {
			return errors.Join(actionErr, fmt.Errorf("runtime executable mode could not be restored"))
		}
		restored, err := root.Lstat(name)
		if err != nil || !os.SameFile(before, restored) || !restored.Mode().IsRegular() || restored.Mode().Perm() != originalMode {
			return errors.Join(actionErr, fmt.Errorf("runtime executable mode restoration is invalid"))
		}
		return actionErr
	}
	disabled, err := root.Lstat(name)
	if err != nil || !os.SameFile(before, disabled) || !disabled.Mode().IsRegular() || disabled.Mode().Perm()&0o111 != 0 {
		return restore(fmt.Errorf("runtime non-executable identity is invalid"))
	}
	actionErr := action()
	after, afterErr := root.Lstat(name)
	if afterErr != nil || !os.SameFile(before, after) || !after.Mode().IsRegular() || after.Mode().Perm()&0o111 != 0 {
		actionErr = errors.Join(actionErr, fmt.Errorf("runtime non-executable identity changed during action"))
	}
	return restore(actionErr)
}

func verifyGoSourceJourney(
	ctx context.Context,
	runner journeyRunner,
	help packagedHelpEvidence,
	goos, executablePath, trustPath, attemptLog string,
	existingFixtureAttempts int,
	workRoot string,
	processor *preparedProcessor,
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
		inspectionPayload.Catalog.SchemaVersion != 2 ||
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

	boundaries := [][]byte{inspection.stdout}
	var processorInspection processorInspectionEvidence
	processorObservationPath := ""
	if goos == "windows" {
		if processor != nil {
			return goSourceEvidence{}, nil, fmt.Errorf("Windows unexpectedly received processor provenance")
		}
		declaration, declarationErr := help.fault("processor inspect", "unsupported_processor_platform")
		if declarationErr != nil || declaration.Kind != "unsupported" || declaration.Retryable {
			return goSourceEvidence{}, nil, fmt.Errorf("Windows processor platform help contract is invalid")
		}
		failure, failureErr := runner.failure(ctx, "success", 12, declaration,
			"--error-format=json", "processor", "inspect", "--adapter=rtk", "--executable", goExecutable)
		if failureErr != nil {
			return goSourceEvidence{}, nil, fmt.Errorf("Windows processor platform rejection is invalid")
		}
		boundaries = append(boundaries, failure.stdout, failure.stderr)
	} else {
		if processor == nil || processor.metadata.Target() != goos+"/"+runtime.GOARCH {
			return goSourceEvidence{}, nil, fmt.Errorf("POSIX processor provenance does not match the native target")
		}
		processorOutput, inspectErr := runner.success(ctx, "success", "processor", "inspect", "--adapter=rtk", "--executable", processor.executablePath)
		if inspectErr != nil {
			return goSourceEvidence{}, nil, fmt.Errorf("processor inspection failed")
		}
		processorInspection, inspectErr = decodeProcessorInspection(processorOutput.stdout)
		if inspectErr != nil || validateProcessorInspection(processorInspection, *processor) != nil {
			return goSourceEvidence{}, nil, fmt.Errorf("processor inspection evidence is invalid")
		}
		processorObservationPath = filepath.Join(workRoot, "processor-observation.json")
		if err := writePrivate(processorObservationPath, processorOutput.stdout); err != nil {
			return goSourceEvidence{}, nil, fmt.Errorf("processor observation write failed")
		}
		boundaries = append(boundaries, processorOutput.stdout)
	}

	draftArguments := []string{"spec", "init", "--catalog", catalogPath, "--", "test"}
	draft, err := runner.success(ctx, "success", draftArguments...)
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
	buildArguments := []string{"bundle", "build", "--catalog", catalogPath, "--spec", specificationPath}
	built, err := runner.success(ctx, "success", buildArguments...)
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
		previewPayload.Plan.SchemaVersion != tailoringplan.SchemaVersion ||
		previewPayload.Plan.WrapperKind != "identity" || previewPayload.Plan.ResultMode != "source_stream_passthrough" {
		return goSourceEvidence{}, nil, fmt.Errorf("Go source preview evidence is invalid")
	}
	if err := validateGoIdentityPreviewPlan(previewPayload.Plan, bundleDigest, inspectionPayload); err != nil {
		return goSourceEvidence{}, nil, fmt.Errorf("Go identity plan contract is invalid: %w", err)
	}
	if err := requireGoTestAttempts(goAttemptLog, 0); err != nil {
		return goSourceEvidence{}, nil, fmt.Errorf("Go source preparation started the test command")
	}
	journey := preparedCommandJourney{
		bundleLocator: bundlePath,
		bundleDigest:  bundleDigest, planDigest: previewPayload.PlanDigest,
		wrapperKind: string(previewPayload.Plan.WrapperKind), resultMode: string(previewPayload.Plan.ResultMode),
		baseInvocation: baseInvocation, sourceArgv: append([]string{}, previewPayload.Plan.Stages.Invoke.Args...),
		optionDefaults:        cloneOptionDefaultEvidence(previewPayload.Plan.Stages.Invoke.OptionDefaults),
		appliedOptionDefaults: cloneOptionDefaultEvidence(previewPayload.Plan.Stages.Invoke.AppliedOptionDefaults),
	}
	boundaries = append(boundaries, draft.stdout, validation.stdout, built.stdout, status.stdout, preview.stdout)
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
		evidence.Optimizer = goOptimizerEvidence{
			Outcome: "platform_not_supported", Cases: []optimizerCaseEvidence{}, Faults: []optimizerFaultEvidence{}, ZeroAttemptRejections: 1,
		}
		return evidence, append(boundaries, failure.stdout, failure.stderr), nil
	}
	if goos != "linux" && goos != "darwin" {
		return goSourceEvidence{}, nil, fmt.Errorf("Go source wrapper target is unsupported")
	}
	if processor == nil {
		return goSourceEvidence{}, nil, fmt.Errorf("Go optimizer processor is missing")
	}
	runner.environment = replaceEnvironment(runner.environment, map[string]string{
		"GOFLAGS":     "-buildvcs=false -count=1",
		goTestModeEnv: "pass", goTestProcessorDrift: "",
	})

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
	identitySourceDigest := digestBytes(textRender.stdout)
	if err := validateWrapperRenderEvidence(document, journey, "go", executablePath, runtimeDigest, runtimeSize, identitySourceDigest); err != nil {
		return goSourceEvidence{}, nil, fmt.Errorf("Go source wrapper binding evidence is invalid: %w", err)
	}
	if err := requireGoTestAttempts(goAttemptLog, 0); err != nil {
		return goSourceEvidence{}, nil, fmt.Errorf("Go source rendering started the test command")
	}
	identityWrapperPath := filepath.Join(workRoot, "caller-owned-go-identity-wrapper.sh")
	if err := writePrivate(identityWrapperPath, textRender.stdout); err != nil {
		return goSourceEvidence{}, nil, fmt.Errorf("Go source caller-owned wrapper write failed")
	}
	identityRejectionDeclaration, err := help.fault("wrapper run", "wrapper_runtime_not_supported")
	if err != nil || identityRejectionDeclaration.Kind != "unsupported" || identityRejectionDeclaration.Retryable {
		return goSourceEvidence{}, nil, fmt.Errorf("Go identity argv rejection help contract is invalid")
	}
	identityRejection, err := runPOSIXCaller(ctx, runner, "go", identityWrapperPath, []string{"test", "extra"}, 12)
	if err != nil || len(identityRejection.stdout) != 0 || validateFault(identityRejection.stderr, identityRejectionDeclaration) != nil {
		return goSourceEvidence{}, nil, fmt.Errorf("Go identity additional-argument rejection is invalid")
	}
	if err := requireGoTestAttempts(goAttemptLog, 0); err != nil {
		return goSourceEvidence{}, nil, fmt.Errorf("Go identity rejection started the source")
	}
	identityInvocation, err := runPOSIXCaller(ctx, runner, "go", identityWrapperPath, []string{"test"}, 0)
	if err != nil || !goTestIdentityOutputPattern.Match(identityInvocation.stdout) || len(identityInvocation.stderr) != 0 {
		return goSourceEvidence{}, nil, fmt.Errorf("Go identity caller-owned result is invalid")
	}
	if err := requireGoTestAttempts(goAttemptLog, 1); err != nil {
		return goSourceEvidence{}, nil, fmt.Errorf("Go identity one-attempt evidence is invalid")
	}
	evidence.WrapperOutcome = "ordinary_command_verified"
	evidence.WrapperCases = []wrapperCaseEvidence{{
		Name: "go_test_identity", WrapperKind: journey.wrapperKind, ResultMode: journey.resultMode,
		CallerArgv: []string{"test"}, SourceArgv: append([]string{}, journey.sourceArgv...),
		OptionDefaults:        cloneOptionDefaultEvidence(journey.optionDefaults),
		AppliedOptionDefaults: cloneOptionDefaultEvidence(journey.appliedOptionDefaults),
		BundleDigest:          journey.bundleDigest, PlanDigest: journey.planDigest, WrapperSourceSHA256: identitySourceDigest,
		StdoutSHA256: digestBytes(identityInvocation.stdout), StderrSHA256: digestBytes(identityInvocation.stderr),
		SourceExitCode: identityInvocation.exitCode, SourceProcessAttempts: 1,
	}}
	evidence.WrapperSourceAttempts = 1
	evidence.ZeroAttemptRejections = 1
	boundaries = append(boundaries, jsonRender.stdout, textRender.stdout, identityRejection.stdout, identityRejection.stderr, identityInvocation.stdout, identityInvocation.stderr)

	optimizerDraft, err := runner.success(ctx, "success", "spec", "init", "--catalog", catalogPath, "--processor", processorObservationPath, "--", "test")
	if err != nil {
		return goSourceEvidence{}, nil, fmt.Errorf("Go optimizer specification draft failed")
	}
	optimizerSpecificationPath := filepath.Join(workRoot, "go-optimizer-specification.yaml")
	if err := writePrivate(optimizerSpecificationPath, optimizerDraft.stdout); err != nil {
		return goSourceEvidence{}, nil, fmt.Errorf("Go optimizer specification write failed")
	}
	optimizerValidation, err := runner.success(ctx, "success", "spec", "validate", "--catalog", catalogPath, "--spec", optimizerSpecificationPath)
	if err != nil || validateSpecificationEvidence(optimizerValidation.stdout, inspectionPayload.CatalogDigest, 0, 1) != nil {
		return goSourceEvidence{}, nil, fmt.Errorf("Go optimizer specification validation evidence is invalid")
	}
	optimizerBuilt, err := runner.success(ctx, "success", "bundle", "build", "--catalog", catalogPath, "--spec", optimizerSpecificationPath, "--processor", processorObservationPath)
	if err != nil {
		return goSourceEvidence{}, nil, fmt.Errorf("Go optimizer bundle build failed")
	}
	optimizerBundleDigest, err := decodeBundleDigest(optimizerBuilt.stdout)
	if err != nil || optimizerBundleDigest == bundleDigest {
		return goSourceEvidence{}, nil, fmt.Errorf("Go optimizer bundle evidence is invalid")
	}
	optimizerBundlePath := filepath.Join(workRoot, "go-optimizer-bundle.json")
	if err := writePrivate(optimizerBundlePath, optimizerBuilt.stdout); err != nil {
		return goSourceEvidence{}, nil, fmt.Errorf("Go optimizer bundle write failed")
	}
	changed, err = store.Add(ctx, optimizerBundleDigest)
	if err != nil || !changed || store.Inspect(ctx, optimizerBundleDigest) != bundletrust.StateAdopted {
		return goSourceEvidence{}, nil, fmt.Errorf("Go optimizer exact receipt seeding failed")
	}
	processorStatus := processorStatusExpectation{
		Contract: processor.metadata.ContractID(), AdapterKind: processor.metadata.ProcessorKind(), Version: processor.metadata.Version(),
		ResolvedPath: processor.executablePath, SHA256: processor.metadata.BinarySHA256(), Size: processor.metadata.BinarySize(),
	}
	optimizerStatus, err := runner.success(ctx, "success", "bundle", "status", "--bundle", optimizerBundlePath)
	if err != nil || validateStatus(optimizerStatus.stdout, optimizerBundleDigest, bundletrust.StateAdopted, processorStatus) != nil {
		return goSourceEvidence{}, nil, fmt.Errorf("Go optimizer adopted status evidence is invalid")
	}
	optimizerBaseInvocation := []string{"--bundle", optimizerBundlePath, "--", inspectionPayload.Catalog.Source.ResolvedPath, "test"}
	optimizerPreview, err := runner.success(ctx, "success", append([]string{"bundle", "preview"}, optimizerBaseInvocation...)...)
	if err != nil {
		return goSourceEvidence{}, nil, fmt.Errorf("Go optimizer preview failed")
	}
	optimizerPreviewPayload, err := decodePreview(optimizerPreview.stdout)
	if err != nil || optimizerPreviewPayload.SourceProcessAttempts != 0 || !digestValue(optimizerPreviewPayload.PlanDigest) ||
		optimizerPreviewPayload.Plan.SchemaVersion != tailoringplan.SchemaVersion || optimizerPreviewPayload.Plan.WrapperKind != "transform" ||
		optimizerPreviewPayload.Plan.ResultMode != "original_preserving_optimizer" {
		return goSourceEvidence{}, nil, fmt.Errorf("Go optimizer preview evidence is invalid")
	}
	optimizerExecution, err := validateGoOptimizerPreviewPlan(
		optimizerPreviewPayload.Plan, optimizerBundleDigest, inspectionPayload, processorInspection.Observation,
	)
	if err != nil {
		return goSourceEvidence{}, nil, fmt.Errorf("Go optimizer plan contract is invalid: %w", err)
	}
	optimizerJourney := preparedCommandJourney{
		bundleLocator: optimizerBundlePath,
		bundleDigest:  optimizerBundleDigest, planDigest: optimizerPreviewPayload.PlanDigest,
		wrapperKind: string(optimizerPreviewPayload.Plan.WrapperKind), resultMode: string(optimizerPreviewPayload.Plan.ResultMode),
		baseInvocation: optimizerBaseInvocation, sourceArgv: append([]string{}, optimizerPreviewPayload.Plan.Stages.Invoke.Args...),
		optionDefaults:        cloneOptionDefaultEvidence(optimizerPreviewPayload.Plan.Stages.Invoke.OptionDefaults),
		appliedOptionDefaults: cloneOptionDefaultEvidence(optimizerPreviewPayload.Plan.Stages.Invoke.AppliedOptionDefaults),
	}
	if err := requireGoTestAttempts(goAttemptLog, 1); err != nil {
		return goSourceEvidence{}, nil, fmt.Errorf("Go optimizer preparation started the source")
	}
	boundaries = append(boundaries, optimizerDraft.stdout, optimizerValidation.stdout, optimizerBuilt.stdout, optimizerStatus.stdout, optimizerPreview.stdout)

	projectionDeclaration, err := help.fault("bundle execute", "wrapper_runtime_not_supported")
	if err != nil || projectionDeclaration.Kind != "unsupported" || projectionDeclaration.Retryable {
		return goSourceEvidence{}, nil, fmt.Errorf("optimizer projection rejection help contract is invalid")
	}
	projectionArgs := append([]string{"--error-format=json", "bundle", "execute"}, optimizerBaseInvocation...)
	projectionRejection, err := runner.failure(ctx, "success", 12, projectionDeclaration, projectionArgs...)
	if err != nil {
		return goSourceEvidence{}, nil, fmt.Errorf("optimizer projection-only rejection is invalid")
	}
	if err := requireGoTestAttempts(goAttemptLog, 1); err != nil {
		return goSourceEvidence{}, nil, fmt.Errorf("optimizer projection rejection started the source")
	}
	optimizerJSONRender, err := runner.success(ctx, "success", "wrapper", "render", "--bundle", optimizerBundlePath, "--format", "json")
	if err != nil {
		return goSourceEvidence{}, nil, fmt.Errorf("Go optimizer wrapper JSON rendering failed")
	}
	optimizerDocument, err := decodeWrapperRender(optimizerJSONRender.stdout)
	if err != nil {
		return goSourceEvidence{}, nil, fmt.Errorf("Go optimizer wrapper JSON evidence is invalid")
	}
	optimizerTextRender, err := runner.success(ctx, "success", "wrapper", "render", "--bundle", optimizerBundlePath)
	if err != nil || string(optimizerTextRender.stdout) != optimizerDocument.Wrapper.Source {
		return goSourceEvidence{}, nil, fmt.Errorf("Go optimizer wrapper text rendering is invalid")
	}
	optimizerSourceDigest := digestBytes(optimizerTextRender.stdout)
	if err := validateWrapperRenderEvidence(optimizerDocument, optimizerJourney, "go", executablePath, runtimeDigest, runtimeSize, optimizerSourceDigest); err != nil {
		return goSourceEvidence{}, nil, fmt.Errorf("Go optimizer wrapper binding evidence is invalid: %w", err)
	}
	if err := requireGoTestAttempts(goAttemptLog, 1); err != nil {
		return goSourceEvidence{}, nil, fmt.Errorf("Go optimizer rendering started the source")
	}
	optimizerWrapperPath := filepath.Join(workRoot, "caller-owned-go-optimizer-wrapper.sh")
	if err := writePrivate(optimizerWrapperPath, optimizerTextRender.stdout); err != nil {
		return goSourceEvidence{}, nil, fmt.Errorf("Go optimizer caller-owned wrapper write failed")
	}

	processorBytes, err := readExactRegularFile(processor.executablePath, processor.metadata.BinarySize())
	if err != nil || digestBytes(processorBytes) != processor.metadata.BinarySHA256() {
		return goSourceEvidence{}, nil, fmt.Errorf("processor drift fixture identity is invalid")
	}
	driftDeclaration, err := help.fault("wrapper run", "processor_identity_changed")
	if err != nil || driftDeclaration.Kind != "rejected" || driftDeclaration.Retryable {
		return goSourceEvidence{}, nil, fmt.Errorf("processor drift help contract is invalid")
	}
	if err := overwriteRegularFixture(processor.executablePath, []byte("preflight drift"), 0o700); err != nil {
		return goSourceEvidence{}, nil, fmt.Errorf("preflight processor drift setup failed")
	}
	preflightDrift, err := runPOSIXCaller(ctx, runner, "go", optimizerWrapperPath, []string{"test"}, 10)
	if err != nil || len(preflightDrift.stdout) != 0 || validateFault(preflightDrift.stderr, driftDeclaration) != nil {
		return goSourceEvidence{}, nil, fmt.Errorf("preflight processor drift rejection is invalid")
	}
	if err := requireGoTestAttempts(goAttemptLog, 1); err != nil {
		return goSourceEvidence{}, nil, fmt.Errorf("preflight processor drift started the source")
	}
	if err := overwriteRegularFixture(processor.executablePath, processorBytes, 0o700); err != nil {
		return goSourceEvidence{}, nil, fmt.Errorf("preflight processor fixture restoration failed")
	}

	postSourceRunner := runner
	postSourceRunner.environment = replaceEnvironment(runner.environment, map[string]string{
		goTestModeEnv: "pass", goTestProcessorDrift: processor.executablePath,
	})
	postSourceDrift, err := runPOSIXCaller(ctx, postSourceRunner, "go", optimizerWrapperPath, []string{"test"}, 10)
	if err != nil || len(postSourceDrift.stdout) != 0 || validateFault(postSourceDrift.stderr, driftDeclaration) != nil {
		return goSourceEvidence{}, nil, fmt.Errorf("post-source processor drift rejection is invalid")
	}
	if err := requireGoTestAttempts(goAttemptLog, 2); err != nil {
		return goSourceEvidence{}, nil, fmt.Errorf("post-source processor drift attempt evidence is invalid")
	}
	if err := overwriteRegularFixture(processor.executablePath, processorBytes, 0o700); err != nil {
		return goSourceEvidence{}, nil, fmt.Errorf("post-source processor fixture restoration failed")
	}
	if digest, size, identityErr := regularFileIdentity(processor.executablePath); identityErr != nil ||
		digest != processor.metadata.BinarySHA256() || size != processor.metadata.BinarySize() {
		return goSourceEvidence{}, nil, fmt.Errorf("restored processor fixture identity is invalid")
	}

	type optimizerCase struct {
		name, mode, disposition, action string
		exitCode                        int
		stdout                          []byte
	}
	cases := []optimizerCase{
		{name: "optimized_pass", mode: "pass", disposition: "optimized", exitCode: 0, stdout: []byte("Go test: 2 passed in 1 packages")},
		{name: "preserved_before_skip", mode: "skip", disposition: "preserved_before_processor", action: "skip", exitCode: 0},
		{name: "preserved_before_fail", mode: "fail", disposition: "preserved_before_processor", action: "fail", exitCode: 1},
		{name: "preserved_before_ineligible", mode: "no_tests", disposition: "preserved_before_processor", action: "no_tests", exitCode: 0},
	}
	optimizerCases := make([]optimizerCaseEvidence, 0, len(cases))
	for index, test := range cases {
		caseRunner := runner
		caseRunner.environment = replaceEnvironment(runner.environment, map[string]string{
			goTestModeEnv: test.mode, goTestProcessorDrift: "",
		})
		invocation, invokeErr := runPOSIXCaller(ctx, caseRunner, "go", optimizerWrapperPath, []string{"test"}, test.exitCode)
		if invokeErr != nil || len(invocation.stderr) != 0 {
			return goSourceEvidence{}, nil, fmt.Errorf("optimizer case %s invocation failed", test.name)
		}
		if test.stdout != nil {
			if !bytes.Equal(invocation.stdout, test.stdout) {
				return goSourceEvidence{}, nil, fmt.Errorf("optimizer case %s summary is invalid", test.name)
			}
		} else if err := validateGoTestJSONL(invocation.stdout, test.action); err != nil {
			return goSourceEvidence{}, nil, fmt.Errorf("optimizer case %s preservation is invalid: %w", test.name, err)
		}
		if err := requireGoTestAttempts(goAttemptLog, index+3); err != nil {
			return goSourceEvidence{}, nil, fmt.Errorf("optimizer case %s source attempt evidence is invalid", test.name)
		}
		optimizerCases = append(optimizerCases, optimizerCaseEvidence{
			Name: test.name, Disposition: test.disposition,
			StdoutSHA256: digestBytes(invocation.stdout), StderrSHA256: digestBytes(invocation.stderr),
			SourceExitCode: invocation.exitCode, SourceProcessAttempts: 1,
		})
		boundaries = append(boundaries, invocation.stdout, invocation.stderr)
	}
	if err := requireAttempts(attemptLog, existingFixtureAttempts); err != nil {
		return goSourceEvidence{}, nil, fmt.Errorf("Go source journey started the GitHub source fixture")
	}
	evidence.Optimizer = goOptimizerEvidence{
		Outcome:   "reachable_outcomes_verified",
		Execution: optimizerExecution,
		Processor: &processorArtifactEvidence{
			ContractID: processor.metadata.ContractID(), AdapterKind: processor.metadata.ProcessorKind(),
			AdapterContractVersion: processorInspection.Observation.Adapter.ContractVersion,
			Version:                processor.metadata.Version(), Target: processor.metadata.Target(),
			ArchiveName: processor.metadata.ArchiveName(), ArchiveSHA256: processor.metadata.ArchiveSHA256(),
			BinarySHA256: processor.metadata.BinarySHA256(), BinarySize: processor.metadata.BinarySize(),
			ObservationDigest:         processorInspection.ObservationDigest,
			InspectionProcessAttempts: processorInspection.ProcessorProcessAttempts,
		},
		BundleDigest: optimizerBundleDigest, PlanDigest: optimizerPreviewPayload.PlanDigest, WrapperSourceSHA256: optimizerSourceDigest,
		Cases: optimizerCases,
		Faults: []optimizerFaultEvidence{
			{Name: "projection_rejection", Code: "wrapper_runtime_not_supported", SourceProcessAttempts: 0},
			{Name: "preflight_processor_drift", Code: "processor_identity_changed", SourceProcessAttempts: 0},
			{Name: "post_source_processor_drift", Code: "processor_identity_changed", SourceProcessAttempts: 1},
		},
		SourceProcessAttempts: len(cases) + 1, ZeroAttemptRejections: 2,
	}
	boundaries = append(boundaries, optimizerJSONRender.stdout, optimizerTextRender.stdout,
		projectionRejection.stdout, projectionRejection.stderr,
		preflightDrift.stdout, preflightDrift.stderr, postSourceDrift.stdout, postSourceDrift.stderr)
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

func TestMain(m *testing.M) {
	path := os.Getenv("ATSURA_GO_TEST_ATTEMPT_LOG")
	if path == "" {
		os.Exit(97)
	}
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0o600)
	if err != nil {
		os.Exit(96)
	}
	if _, err := io.WriteString(file, "attempt\n"); err != nil {
		_ = file.Close()
		os.Exit(95)
	}
	if err := file.Close(); err != nil {
		os.Exit(94)
	}
	if drift := os.Getenv("ATSURA_GO_TEST_PROCESSOR_DRIFT_PATH"); drift != "" {
		if err := os.WriteFile(drift, []byte("post-source drift"), 0o700); err != nil {
			os.Exit(93)
		}
	}
	if os.Getenv("ATSURA_GO_TEST_MODE") == "no_tests" {
		os.Exit(0)
	}
	os.Exit(m.Run())
}

func TestOne(t *testing.T) {
	applyMode(t)
}

func TestTwo(t *testing.T) {
	applyMode(t)
}

func applyMode(t *testing.T) {
	t.Helper()
	switch os.Getenv("ATSURA_GO_TEST_MODE") {
	case "pass":
	case "skip":
		t.Skip("synthetic skip")
	case "fail":
		t.Fatal("synthetic failure")
	default:
		t.Fatal("unsupported synthetic mode")
	}
}
`)
	if err := writePrivate(filepath.Join(workRoot, "go.mod"), module); err != nil {
		return err
	}
	return writePrivate(filepath.Join(workRoot, "artifact_test.go"), testSource)
}

func requireGoTestAttempts(path string, wanted int) error {
	if wanted < 0 || wanted > 16 {
		return fmt.Errorf("Go source attempt bound is invalid")
	}
	if wanted == 0 {
		if _, err := os.Lstat(path); errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return fmt.Errorf("Go source attempt log exists before execution")
	}
	value, err := readBoundedFile(path, int64(16*len("attempt\n")))
	if err != nil || string(value) != strings.Repeat("attempt\n", wanted) {
		return fmt.Errorf("Go source attempt log is invalid")
	}
	return nil
}

func overwriteRegularFixture(path string, value []byte, mode os.FileMode) error {
	root, name, err := openParentRoot(path)
	if err != nil {
		return err
	}
	defer root.Close()
	before, err := root.Lstat(name)
	if err != nil || before.Mode()&os.ModeSymlink != 0 || !before.Mode().IsRegular() {
		return fmt.Errorf("fixture is not a regular file")
	}
	file, err := root.OpenFile(name, os.O_WRONLY|os.O_TRUNC, mode)
	if err != nil {
		return err
	}
	written, writeErr := file.Write(value)
	closeErr := file.Close()
	if writeErr != nil || written != len(value) || closeErr != nil {
		return fmt.Errorf("fixture replacement failed")
	}
	if err := root.Chmod(name, mode); err != nil {
		return err
	}
	after, err := root.Lstat(name)
	if err != nil || !os.SameFile(before, after) || !after.Mode().IsRegular() || after.Size() != int64(len(value)) {
		return fmt.Errorf("fixture replacement identity is invalid")
	}
	return nil
}

func validateGoTestJSONL(value []byte, wantedAction string) error {
	if len(value) == 0 || len(value) > maxCommandOutputBytes || value[len(value)-1] != '\n' {
		return fmt.Errorf("Go test JSONL framing is invalid")
	}
	foundAction := false
	foundTest := false
	scanner := bufio.NewScanner(bytes.NewReader(value))
	scanner.Buffer(make([]byte, 4096), 256*1024)
	for scanner.Scan() {
		var event struct {
			Action  string `json:"Action"`
			Package string `json:"Package"`
			Test    string `json:"Test"`
		}
		if err := json.Unmarshal(scanner.Bytes(), &event); err != nil || event.Action == "" || event.Package != "example.com/atsura-artifact-go" {
			return fmt.Errorf("Go test JSONL record is invalid")
		}
		if event.Test != "" {
			foundTest = true
		}
		if event.Action == wantedAction {
			foundAction = true
		}
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("Go test JSONL scan failed")
	}
	if wantedAction == "no_tests" {
		if foundTest {
			return fmt.Errorf("ineligible no-tests output unexpectedly names a test")
		}
		return nil
	}
	if !foundAction {
		return fmt.Errorf("Go test JSONL action %q is missing", wantedAction)
	}
	return nil
}

func validateWrapperRenderEvidence(document wrapperRenderDocument, journey preparedCommandJourney, ordinaryCommand, executablePath, runtimeDigest string, runtimeSize int64, sourceDigest string) error {
	if document.SchemaVersion != 2 || document.Wrapper.Command != ordinaryCommand || document.Wrapper.Contract.Version != wrapperbinding.ContractVersion || document.Wrapper.Contract.Shell != "posix" {
		return fmt.Errorf("contract identity mismatch")
	}
	if document.Wrapper.Bundle.Locator != journey.bundlePath() || document.Wrapper.Bundle.Digest != journey.bundleDigest {
		return fmt.Errorf("bundle identity mismatch")
	}
	if document.Wrapper.Runtime.ResolvedPath != executablePath || document.Wrapper.Runtime.SHA256 != runtimeDigest || document.Wrapper.Runtime.Size != runtimeSize {
		return fmt.Errorf("runtime identity mismatch")
	}
	if document.Wrapper.SourceSHA256 != sourceDigest || document.Wrapper.SourceProcessAttempts != 0 || document.Wrapper.ProcessorProcessAttempts != 0 {
		return fmt.Errorf("rendered source identity mismatch")
	}
	return nil
}

func (j preparedCommandJourney) bundlePath() string {
	if !absoluteCleanPath(j.bundleLocator) {
		return ""
	}
	return j.bundleLocator
}

func cloneOptionDefaultEvidence(values []tailoringbundle.OptionDefault) []tailoringbundle.OptionDefault {
	if values == nil {
		return nil
	}
	return append([]tailoringbundle.OptionDefault{}, values...)
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

type regularFileSnapshot struct {
	info   os.FileInfo
	sha256 string
	size   int64
}

func snapshotRegularFile(path string) (regularFileSnapshot, error) {
	file, info, err := openRegularInput(path)
	if err != nil {
		return regularFileSnapshot{}, err
	}
	defer file.Close()
	if info.Size() <= 0 || info.Size() > maxMemberBytes {
		return regularFileSnapshot{}, fmt.Errorf("file size is invalid")
	}
	hash := sha256.New()
	written, err := io.Copy(hash, io.LimitReader(file, maxMemberBytes+1))
	if err != nil || written != info.Size() {
		return regularFileSnapshot{}, fmt.Errorf("file digest failed")
	}
	return regularFileSnapshot{info: info, sha256: fmt.Sprintf("%x", hash.Sum(nil)), size: info.Size()}, nil
}

func (s regularFileSnapshot) verify(path string) error {
	current, err := snapshotRegularFile(path)
	if err != nil || s.info == nil || !os.SameFile(s.info, current.info) || s.sha256 != current.sha256 || s.size != current.size {
		return fmt.Errorf("regular file identity changed")
	}
	return nil
}

func regularFileIdentity(path string) (string, int64, error) {
	snapshot, err := snapshotRegularFile(path)
	if err != nil {
		return "", 0, err
	}
	return snapshot.sha256, snapshot.size, nil
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

func prepareMultiCommandWrapperJourneys(
	ctx context.Context,
	runner journeyRunner,
	help packagedHelpEvidence,
	workRoot, catalogPath, catalogDigest, trustPath, attemptLog string,
	existingAttempts int,
) (preparedCommandJourney, preparedCommandJourney, [][]byte, error) {
	if ctx == nil || !digestValue(catalogDigest) {
		return preparedCommandJourney{}, preparedCommandJourney{}, nil, fmt.Errorf("multi-command journey input is invalid")
	}
	prDraft, err := runner.success(ctx, "success", "spec", "init", "--catalog", catalogPath, "--", "pr", "list")
	if err != nil {
		return preparedCommandJourney{}, preparedCommandJourney{}, nil, fmt.Errorf("pull-request specification draft failed")
	}
	issueDraft, err := runner.success(ctx, "success", "spec", "init", "--catalog", catalogPath, "--", "issue", "list")
	if err != nil {
		return preparedCommandJourney{}, preparedCommandJourney{}, nil, fmt.Errorf("issue specification draft failed")
	}
	prDraftSpecification, err := loadSpecificationDraft(ctx, workRoot, "multi-pr", prDraft.stdout)
	if err != nil || prDraftSpecification.CatalogDigest != catalogDigest {
		return preparedCommandJourney{}, preparedCommandJourney{}, nil, fmt.Errorf("pull-request specification draft is invalid")
	}
	issueDraftSpecification, err := loadSpecificationDraft(ctx, workRoot, "multi-issue", issueDraft.stdout)
	if err != nil || issueDraftSpecification.CatalogDigest != catalogDigest {
		return preparedCommandJourney{}, preparedCommandJourney{}, nil, fmt.Errorf("issue specification draft is invalid")
	}
	prSpecification, err := transformDraft(prDraftSpecification, []string{"pr", "list"})
	if err != nil {
		return preparedCommandJourney{}, preparedCommandJourney{}, nil, fmt.Errorf("pull-request specification edit failed")
	}
	issueSpecification, err := transformSourceStreamDraft(issueDraftSpecification, "append_only", []string{"issue", "list"})
	if err != nil {
		return preparedCommandJourney{}, preparedCommandJourney{}, nil, fmt.Errorf("issue specification edit failed")
	}
	combined, err := combineCommandSpecifications(prSpecification, issueSpecification)
	if err != nil {
		return preparedCommandJourney{}, preparedCommandJourney{}, nil, err
	}
	encodedSpecification, err := specyaml.Encode(combined)
	if err != nil {
		return preparedCommandJourney{}, preparedCommandJourney{}, nil, fmt.Errorf("multi-command specification encoding failed")
	}
	specificationPath := filepath.Join(workRoot, "multi-command-specification.yaml")
	if err := writePrivate(specificationPath, encodedSpecification); err != nil {
		return preparedCommandJourney{}, preparedCommandJourney{}, nil, fmt.Errorf("multi-command specification write failed")
	}
	validation, err := runner.success(ctx, "success", "spec", "validate", "--catalog", catalogPath, "--spec", specificationPath)
	if err != nil || validateSpecificationEvidenceCounts(validation.stdout, catalogDigest, 2, 0, 2) != nil {
		return preparedCommandJourney{}, preparedCommandJourney{}, nil, fmt.Errorf("multi-command specification validation evidence is invalid")
	}
	built, err := runner.success(ctx, "success", "bundle", "build", "--catalog", catalogPath, "--spec", specificationPath)
	if err != nil {
		return preparedCommandJourney{}, preparedCommandJourney{}, nil, fmt.Errorf("multi-command bundle build failed")
	}
	bundleDigest, err := decodeBundleDigest(built.stdout)
	if err != nil {
		return preparedCommandJourney{}, preparedCommandJourney{}, nil, fmt.Errorf("multi-command bundle evidence is invalid")
	}
	bundlePath := filepath.Join(workRoot, "multi-command-bundle.json")
	if err := writePrivate(bundlePath, built.stdout); err != nil {
		return preparedCommandJourney{}, preparedCommandJourney{}, nil, fmt.Errorf("multi-command bundle write failed")
	}
	preAdoptionStatus, err := runner.success(ctx, "success", "bundle", "status", "--bundle", bundlePath)
	if err != nil || validateStatus(preAdoptionStatus.stdout, bundleDigest, bundletrust.StateNotAdopted) != nil {
		return preparedCommandJourney{}, preparedCommandJourney{}, nil, fmt.Errorf("multi-command pre-adoption status evidence is invalid")
	}
	prBaseInvocation := []string{"--bundle", bundlePath, "--", "gh", "pr", "list"}
	issueBaseInvocation := []string{
		"--bundle", bundlePath, "--", "gh", "issue", "list",
		"--search=append value", "--label=one", "--label=two",
	}
	preAdoptionBoundaries := make([][]byte, 0, 4)
	zeroAttemptRejections := 0
	for _, bundleCommand := range []string{"preview", "execute"} {
		declaration, declarationErr := help.fault("bundle "+bundleCommand, "bundle_not_adopted")
		if declarationErr != nil || declaration.Kind != "rejected" || declaration.Retryable {
			return preparedCommandJourney{}, preparedCommandJourney{}, nil, fmt.Errorf("multi-command pre-adoption help contract is invalid")
		}
		arguments := append([]string{"--error-format=json", "bundle", bundleCommand}, prBaseInvocation...)
		failure, failureErr := runner.failure(ctx, "success", 10, declaration, arguments...)
		if failureErr != nil {
			return preparedCommandJourney{}, preparedCommandJourney{}, nil, fmt.Errorf("multi-command pre-adoption rejection evidence is invalid")
		}
		if err := scanCanaries(failure.stdout, failure.stderr); err != nil {
			return preparedCommandJourney{}, preparedCommandJourney{}, nil, err
		}
		preAdoptionBoundaries = append(preAdoptionBoundaries, failure.stdout, failure.stderr)
		zeroAttemptRejections++
	}
	if err := requireAttempts(attemptLog, existingAttempts); err != nil {
		return preparedCommandJourney{}, preparedCommandJourney{}, nil, fmt.Errorf("multi-command pre-adoption rejection started the source fixture")
	}
	store := trustfile.New(trustPath)
	changed, err := store.Add(ctx, bundleDigest)
	if err != nil || !changed || store.Inspect(ctx, bundleDigest) != bundletrust.StateAdopted {
		return preparedCommandJourney{}, preparedCommandJourney{}, nil, fmt.Errorf("multi-command exact receipt seeding failed")
	}
	adoptedStatus, err := runner.success(ctx, "success", "bundle", "status", "--bundle", bundlePath)
	if err != nil || validateStatus(adoptedStatus.stdout, bundleDigest, bundletrust.StateAdopted) != nil {
		return preparedCommandJourney{}, preparedCommandJourney{}, nil, fmt.Errorf("multi-command adopted status evidence is invalid")
	}
	prPreview, err := runner.success(ctx, "success", append([]string{"bundle", "preview"}, prBaseInvocation...)...)
	if err != nil {
		return preparedCommandJourney{}, preparedCommandJourney{}, nil, fmt.Errorf("multi-command pull-request preview failed")
	}
	prEvidence, err := decodePreview(prPreview.stdout)
	if err != nil || prEvidence.SourceProcessAttempts != 0 || !digestValue(prEvidence.PlanDigest) ||
		prEvidence.Plan.SchemaVersion != tailoringplan.SchemaVersion || prEvidence.Plan.BundleDigest != bundleDigest ||
		prEvidence.Plan.WrapperKind != tailoringbundle.WrapperTransform || prEvidence.Plan.ResultMode != tailoringplan.ResultModeTransformedJSON ||
		!reflect.DeepEqual(prEvidence.Plan.MatchedCommand, []string{"pr", "list"}) ||
		!reflect.DeepEqual(prEvidence.Plan.Stages.Invoke.Args, []string{"pr", "list", "--limit=30", "--json=number,title,state"}) ||
		!reflect.DeepEqual(prEvidence.Plan.Stages.Invoke.OptionDefaults, []tailoringbundle.OptionDefault{{Option: "--limit", Value: "30"}}) ||
		!reflect.DeepEqual(prEvidence.Plan.Stages.Invoke.AppliedOptionDefaults, []tailoringbundle.OptionDefault{{Option: "--limit", Value: "30"}}) {
		return preparedCommandJourney{}, preparedCommandJourney{}, nil, fmt.Errorf("multi-command pull-request preview evidence is invalid")
	}
	issuePreview, err := runner.success(ctx, "success", append([]string{"bundle", "preview"}, issueBaseInvocation...)...)
	if err != nil {
		return preparedCommandJourney{}, preparedCommandJourney{}, nil, fmt.Errorf("multi-command issue preview failed")
	}
	issueEvidence, err := decodePreview(issuePreview.stdout)
	if err != nil || issueEvidence.SourceProcessAttempts != 0 || !digestValue(issueEvidence.PlanDigest) ||
		issueEvidence.Plan.SchemaVersion != tailoringplan.SchemaVersion || issueEvidence.Plan.BundleDigest != bundleDigest ||
		issueEvidence.Plan.WrapperKind != tailoringbundle.WrapperTransform || issueEvidence.Plan.ResultMode != tailoringplan.ResultModeSourceStreamPassthrough ||
		!reflect.DeepEqual(issueEvidence.Plan.MatchedCommand, []string{"issue", "list"}) ||
		!reflect.DeepEqual(issueEvidence.Plan.Stages.Invoke.Args, []string{"issue", "list", "--search=append value", "--label=one", "--label=two", "--limit=1"}) ||
		len(issueEvidence.Plan.Stages.Invoke.OptionDefaults) != 0 || len(issueEvidence.Plan.Stages.Invoke.AppliedOptionDefaults) != 0 ||
		issueEvidence.PlanDigest == prEvidence.PlanDigest {
		return preparedCommandJourney{}, preparedCommandJourney{}, nil, fmt.Errorf("multi-command issue preview evidence is invalid")
	}
	if err := requireAttempts(attemptLog, existingAttempts); err != nil {
		return preparedCommandJourney{}, preparedCommandJourney{}, nil, fmt.Errorf("multi-command previews started the source fixture")
	}
	boundaries := [][]byte{
		prDraft.stdout, issueDraft.stdout, encodedSpecification, validation.stdout, built.stdout,
		preAdoptionStatus.stdout, adoptedStatus.stdout, prPreview.stdout, issuePreview.stdout,
	}
	boundaries = append(boundaries, preAdoptionBoundaries...)
	prJourney := preparedCommandJourney{
		bundleLocator: bundlePath,
		bundleDigest:  bundleDigest, planDigest: prEvidence.PlanDigest,
		wrapperKind: string(prEvidence.Plan.WrapperKind), resultMode: string(prEvidence.Plan.ResultMode),
		baseInvocation: prBaseInvocation, sourceArgv: append([]string{}, prEvidence.Plan.Stages.Invoke.Args...),
		optionDefaults:        cloneOptionDefaultEvidence(prEvidence.Plan.Stages.Invoke.OptionDefaults),
		appliedOptionDefaults: cloneOptionDefaultEvidence(prEvidence.Plan.Stages.Invoke.AppliedOptionDefaults),
		zeroAttemptRejections: zeroAttemptRejections,
	}
	issueJourney := preparedCommandJourney{
		bundleLocator: bundlePath,
		bundleDigest:  bundleDigest, planDigest: issueEvidence.PlanDigest,
		wrapperKind: string(issueEvidence.Plan.WrapperKind), resultMode: string(issueEvidence.Plan.ResultMode),
		baseInvocation: issueBaseInvocation, sourceArgv: append([]string{}, issueEvidence.Plan.Stages.Invoke.Args...),
		optionDefaults:        cloneOptionDefaultEvidence(issueEvidence.Plan.Stages.Invoke.OptionDefaults),
		appliedOptionDefaults: cloneOptionDefaultEvidence(issueEvidence.Plan.Stages.Invoke.AppliedOptionDefaults),
	}
	return prJourney, issueJourney, boundaries, nil
}

func prepareDefaultOverrideJourney(
	ctx context.Context,
	runner journeyRunner,
	base preparedCommandJourney,
	attemptLog string,
	existingAttempts int,
) (preparedCommandJourney, error) {
	if ctx == nil || base.bundlePath() == "" || base.bundleDigest == "" || base.planDigest == "" {
		return preparedCommandJourney{}, fmt.Errorf("default override journey input is invalid")
	}
	baseInvocation := []string{"--bundle", base.bundlePath(), "--", "gh", "pr", "list", "--limit=2"}
	preview, err := runner.success(ctx, "success", append([]string{"bundle", "preview"}, baseInvocation...)...)
	if err != nil {
		return preparedCommandJourney{}, fmt.Errorf("default override preview failed")
	}
	evidence, err := decodePreview(preview.stdout)
	declared := []tailoringbundle.OptionDefault{{Option: "--limit", Value: "30"}}
	if err != nil || evidence.SourceProcessAttempts != 0 || !digestValue(evidence.PlanDigest) ||
		evidence.Plan.SchemaVersion != tailoringplan.SchemaVersion || evidence.Plan.BundleDigest != base.bundleDigest ||
		evidence.Plan.WrapperKind != tailoringbundle.WrapperTransform || evidence.Plan.ResultMode != tailoringplan.ResultModeTransformedJSON ||
		!reflect.DeepEqual(evidence.Plan.MatchedCommand, []string{"pr", "list"}) ||
		!reflect.DeepEqual(evidence.Plan.Stages.Invoke.Args, []string{"pr", "list", "--limit=2", "--json=number,title,state"}) ||
		!reflect.DeepEqual(evidence.Plan.Stages.Invoke.OptionDefaults, declared) || len(evidence.Plan.Stages.Invoke.AppliedOptionDefaults) != 0 ||
		evidence.PlanDigest == base.planDigest {
		return preparedCommandJourney{}, fmt.Errorf("default override preview evidence is invalid")
	}
	if err := requireAttempts(attemptLog, existingAttempts); err != nil {
		return preparedCommandJourney{}, fmt.Errorf("default override preview started the source fixture")
	}
	return preparedCommandJourney{
		bundleLocator: base.bundleLocator,
		bundleDigest:  base.bundleDigest, planDigest: evidence.PlanDigest,
		wrapperKind: string(evidence.Plan.WrapperKind), resultMode: string(evidence.Plan.ResultMode),
		baseInvocation: baseInvocation, sourceArgv: append([]string{}, evidence.Plan.Stages.Invoke.Args...),
		optionDefaults:        cloneOptionDefaultEvidence(evidence.Plan.Stages.Invoke.OptionDefaults),
		appliedOptionDefaults: cloneOptionDefaultEvidence(evidence.Plan.Stages.Invoke.AppliedOptionDefaults),
		boundaries:            [][]byte{preview.stdout},
	}, nil
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
	draftSpecification, err := loadSpecificationDraft(ctx, workRoot, prefix+"-projection", draft.stdout)
	if err != nil || draftSpecification.CatalogDigest != catalogDigest {
		return preparedCommandJourney{}, fmt.Errorf("specification draft is invalid")
	}
	transformed, err := transformDraft(draftSpecification, command)
	if err != nil {
		return preparedCommandJourney{}, fmt.Errorf("specification transform edit failed")
	}
	transformedSpecification, err := specyaml.Encode(transformed)
	if err != nil {
		return preparedCommandJourney{}, fmt.Errorf("specification transform encoding failed")
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
	if err != nil || previewEvidence.SourceProcessAttempts != 0 || !digestValue(previewEvidence.PlanDigest) || previewEvidence.Plan.SchemaVersion != tailoringplan.SchemaVersion || previewEvidence.Plan.WrapperKind != "transform" || previewEvidence.Plan.ResultMode != "transformed_json" {
		return preparedCommandJourney{}, fmt.Errorf("adopted preview evidence is invalid")
	}
	if err := requireAttempts(attemptLog, existingAttempts); err != nil {
		return preparedCommandJourney{}, fmt.Errorf("preview zero-attempt evidence is invalid")
	}

	return preparedCommandJourney{
		bundleLocator: bundlePath,
		bundleDigest:  bundleDigest, planDigest: previewEvidence.PlanDigest,
		wrapperKind: string(previewEvidence.Plan.WrapperKind), resultMode: string(previewEvidence.Plan.ResultMode),
		baseInvocation: baseInvocation, sourceArgv: append([]string{}, previewEvidence.Plan.Stages.Invoke.Args...),
		optionDefaults:        cloneOptionDefaultEvidence(previewEvidence.Plan.Stages.Invoke.OptionDefaults),
		appliedOptionDefaults: cloneOptionDefaultEvidence(previewEvidence.Plan.Stages.Invoke.AppliedOptionDefaults),
		zeroAttemptRejections: zeroAttemptRejections,
		boundaries:            [][]byte{draft.stdout, transformedSpecification, validation.stdout, built.stdout, preAdoptionStatus.stdout, adoptedStatus.stdout, preview.stdout},
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
	draftSpecification, err := loadSpecificationDraft(ctx, workRoot, name, draft.stdout)
	if err != nil || draftSpecification.CatalogDigest != catalogDigest {
		return preparedCommandJourney{}, fmt.Errorf("source-stream specification draft is invalid")
	}
	transformed, err := transformSourceStreamDraft(draftSpecification, name, command)
	if err != nil {
		return preparedCommandJourney{}, fmt.Errorf("source-stream specification edit failed")
	}
	specification, err := specyaml.Encode(transformed)
	if err != nil {
		return preparedCommandJourney{}, fmt.Errorf("source-stream specification encoding failed")
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
	if err != nil || previewEvidence.SourceProcessAttempts != 0 || !digestValue(previewEvidence.PlanDigest) || previewEvidence.Plan.SchemaVersion != tailoringplan.SchemaVersion || string(previewEvidence.Plan.WrapperKind) != wantedKind || previewEvidence.Plan.ResultMode != "source_stream_passthrough" ||
		len(previewEvidence.Plan.Stages.Invoke.OptionDefaults) != 0 || len(previewEvidence.Plan.Stages.Invoke.AppliedOptionDefaults) != 0 {
		return preparedCommandJourney{}, fmt.Errorf("source-stream preview evidence is invalid")
	}
	if err := requireAttempts(attemptLog, existingAttempts); err != nil {
		return preparedCommandJourney{}, fmt.Errorf("source-stream preparation started the source fixture")
	}
	return preparedCommandJourney{
		bundleLocator: bundlePath,
		bundleDigest:  bundleDigest, planDigest: previewEvidence.PlanDigest,
		wrapperKind: string(previewEvidence.Plan.WrapperKind), resultMode: string(previewEvidence.Plan.ResultMode),
		baseInvocation: baseInvocation, sourceArgv: append([]string{}, previewEvidence.Plan.Stages.Invoke.Args...),
		optionDefaults:        cloneOptionDefaultEvidence(previewEvidence.Plan.Stages.Invoke.OptionDefaults),
		appliedOptionDefaults: cloneOptionDefaultEvidence(previewEvidence.Plan.Stages.Invoke.AppliedOptionDefaults),
		boundaries:            [][]byte{draft.stdout, specification, validation.stdout, built.stdout, status.stdout, preview.stdout},
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
	expectedHelpFault("legacy_tailoring_schema", "invalid_input", false, "help bundle build", "Rebuild with a schema-5 specification and bundle schema 4."),
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
	expectedHelpFault("legacy_tailoring_schema", "invalid_input", false, "help bundle build", "Rebuild with a schema-5 specification and bundle schema 4."),
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
		expectedHelpFault("legacy_tailoring_schema", "invalid_input", false, "help bundle build", "Rebuild with a schema-5 specification and bundle schema 4."),
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
	expectedHelpFault("invalid_processor_executable", "invalid_input", false, "bundle status", "Reconcile the exact bundle-bound processor executable."),
	expectedHelpFault("unsafe_processor_executable", "invalid_input", false, "bundle status", "Replace or re-inspect the bundle-bound processor as a supported regular executable."),
	expectedHelpFault("processor_identity_unavailable", "unavailable", true, "bundle status", "Retry only after the exact bundle-bound processor identity can be read."),
	expectedHelpFault("processor_identity_changed", "rejected", false, "bundle status", "Rebuild and adopt current processor identity evidence."),
	expectedHelpFault("invalid_processor_identity", "contract", false, "bundle status", "Repair invalid processor identity evidence."),
	expectedHelpFault("bundle_processor_drift", "rejected", false, "bundle status", "Rebuild and adopt current processor identity evidence before rendering a wrapper."),
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
	expectedHelpFault("processor_identity_unavailable", "unavailable", true, "wrapper run", "Retry only because the processor identity could not be read before any source attempt."),
	expectedHelpFault("processor_identity_unavailable_after_source", "unavailable", false, "bundle status", "Reconcile the exact bundle-bound processor identity after the source completed; replay is not known to be safe."),
	expectedHelpFault("processor_identity_changed", "rejected", false, "bundle status", "Rebuild and adopt current processor evidence; do not replay a source attempt that may already have completed."),
	expectedHelpFault("invalid_processor_executable", "invalid_input", false, "bundle status", "Reconcile the bundle-bound processor executable after the completed source attempt."),
	expectedHelpFault("unsafe_processor_executable", "invalid_input", false, "bundle status", "Replace or re-inspect the unsafe bundle-bound processor without replaying the completed source attempt."),
	expectedHelpFault("invalid_processor_identity", "contract", false, "bundle status", "Repair invalid processor identity evidence without replaying the completed source attempt."),
	expectedHelpFault("invalid_processor_process_request", "contract", false, "bundle preview", "Repair the exact plan-derived processor request; the source was not retried."),
	expectedHelpFault("processor_environment_setup_failed_after_source", "unavailable", false, "bundle status", "Reconcile source-owned effects and the isolated processor environment before another invocation."),
	expectedHelpFault("processor_process_start_failed_after_source", "unavailable", false, "bundle status", "Reconcile source-owned effects before another invocation; no fallback bytes were published."),
	expectedHelpFault("processor_stdout_too_large", "contract", false, "bundle status", "Reconcile source-owned effects and reduce processor output within its declared bound."),
	expectedHelpFault("processor_stderr_too_large", "contract", false, "bundle status", "Reconcile source-owned effects and reduce processor stderr within its declared bound."),
	expectedHelpFault("processor_execution_canceled", "canceled", false, "help wrapper run", "Review the exact optimizer runtime outcome; replay is not known to be safe."),
	expectedHelpFault("processor_timeout", "unavailable", false, "bundle status", "Reconcile source-owned effects after the processor timed out."),
	expectedHelpFault("processor_command_failed", "rejected", false, "bundle status", "Reconcile source-owned effects and inspect the processor independently; no fallback bytes were published."),
	expectedHelpFault("processor_process_wait_failed", "unavailable", false, "bundle status", "Reconcile source-owned effects after the processor result could not be collected."),
	expectedHelpFault("processor_cleanup_failed", "unavailable", false, "bundle status", "Reconcile source-owned effects and the isolated processor environment before another invocation."),
	expectedHelpFault("processor_output_not_admitted", "contract", false, "bundle status", "Reconcile source-owned effects and the rejected optimizer result; no fallback bytes were published."),
	expectedHelpFault("unclassified_processor_execution_outcome", "contract", false, "bundle status", "Reconcile source-owned effects after the processor result could not be classified."),
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
	Name          string                `json:"name"`
	Type          string                `json:"type"`
	Description   string                `json:"description"`
	ReferenceKind string                `json:"reference_kind"`
	Schema        *helpSchemaProjection `json:"schema"`
}

type helpOutputSchemaReference struct {
	Command string `json:"command"`
	Field   string `json:"field"`
	ID      string `json:"id"`
	Version int    `json:"version"`
}

type helpPlanResultModeProjection struct {
	Mode            string                            `json:"mode"`
	SuccessVariants []helpPlanResultSuccessProjection `json:"success_variants"`
}

type helpPlanResultSuccessProjection struct {
	Disposition              string `json:"disposition"`
	Stdout                   string `json:"stdout"`
	Stderr                   string `json:"stderr"`
	ExitStatus               string `json:"exit_status"`
	Framing                  string `json:"framing"`
	Projection               string `json:"projection"`
	Delivery                 string `json:"delivery"`
	CrossStreamOrder         string `json:"cross_stream_order"`
	StdoutLimitBytes         int    `json:"stdout_limit_bytes"`
	StderrLimitBytes         int    `json:"stderr_limit_bytes"`
	SourceProcessAttempts    int    `json:"source_process_attempts"`
	ProcessorProcessAttempts int    `json:"processor_process_attempts"`
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
	Effect   string `json:"effect"`
	Role     string `json:"role"`
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
		FixedTarget *struct {
			Kind  string `json:"kind"`
			ID    string `json:"id"`
			Scope string `json:"scope"`
		} `json:"fixed_target"`
		Mutation *struct {
			TargetKind    string   `json:"target_kind"`
			TargetInputs  []string `json:"target_inputs"`
			TargetIDInput string   `json:"target_id_input"`
			Impact        struct {
				Cardinality  string `json:"cardinality"`
				Notification string `json:"notification"`
				AccessChange string `json:"access_change"`
				Destructive  string `json:"destructive"`
			} `json:"impact"`
		} `json:"mutation"`
		Prerequisites []string               `json:"prerequisites"`
		Errors        []helpFaultDeclaration `json:"errors"`
	} `json:"contract"`
	ProducesRefs []struct {
		Kind  string `json:"kind"`
		Field string `json:"field"`
	} `json:"produces_refs"`
	ConsumesRefs []struct {
		Kind     string `json:"kind"`
		Argument string `json:"argument"`
	} `json:"consumes_refs"`
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

var processorObservationSchemaFields = []helpSchemaFieldProjection{
	schemaField("/adapter", "object", true),
	schemaField("/adapter/contract_version", "integer", true),
	schemaField("/adapter/kind", "string", true),
	schemaField("/identity", "object", true),
	schemaField("/identity/resolved_path", "string", true),
	schemaField("/identity/sha256", "string", true),
	schemaField("/identity/size", "integer", true),
	schemaField("/platform", "object", true),
	schemaField("/platform/arch", "string", true),
	schemaField("/platform/os", "string", true),
	schemaField("/probe", "object", true),
	schemaArray("/probe/argv", "string"),
	schemaField("/probe/attempts", "integer", true),
	schemaField("/probe/environment_contract", "string", true),
	schemaField("/schema_version", "integer", true),
	schemaField("/version", "string", true),
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
	schemaArray("/commands/*/wrapper/invoke/option_defaults", "object"),
	schemaField("/commands/*/wrapper/invoke/option_defaults/*/option", "string", true),
	schemaField("/commands/*/wrapper/invoke/option_defaults/*/value", "string", true),
	schemaField("/commands/*/wrapper/kind", "string", true),
	schemaField("/commands/*/wrapper/output", "object", false),
	schemaField("/commands/*/wrapper/output/kind", "string", true),
	schemaField("/commands/*/wrapper/output/optimizer", "object", false),
	schemaField("/commands/*/wrapper/output/optimizer/allow_original_output", "boolean", true),
	schemaField("/commands/*/wrapper/output/optimizer/contract", "string", true),
	schemaField("/commands/*/wrapper/output/optimizer/input", "string", true),
	schemaField("/commands/*/wrapper/output/projection", "object", false),
	schemaField("/commands/*/wrapper/output/projection/input", "string", true),
	schemaArray("/commands/*/wrapper/output/projection/rename", "object"),
	schemaField("/commands/*/wrapper/output/projection/rename/*/from", "string", true),
	schemaField("/commands/*/wrapper/output/projection/rename/*/to", "string", true),
	schemaField("/commands/*/wrapper/output/projection/render", "string", true),
	schemaArray("/commands/*/wrapper/output/projection/select", "string"),
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

var bundleProcessorStatusSchemaFields = []helpSchemaFieldProjection{
	schemaField("/adapter_kind", "string", true),
	schemaField("/contract", "string", true),
	schemaField("/resolved_path", "string", true),
	schemaField("/sha256", "string", true),
	schemaField("/size", "integer", true),
	schemaField("/state", "string", true),
	schemaField("/version", "string", true),
}

func validateCatalogJSONOutput(command helpCommandProjection, envelope string, version int, fields []struct{ name, fieldType string }) error {
	return validateCatalogJSONOutputCoverage(command, envelope, version, "not_applicable", fields)
}

func validateCatalogJSONOutputCoverage(command helpCommandProjection, envelope string, version int, coverage string, fields []struct{ name, fieldType string }) error {
	output := command.Contract.Output
	if output.Authority != "catalog" || strings.Join(output.Formats, ",") != "json" || output.DefaultFormat != "json" ||
		output.Delivery != "complete" || output.CollectionCoverage != coverage || output.JSONEnvelope != envelope ||
		output.JSONSchemaVersion != version || output.PlanSchema != nil || output.JSONShape != "" || output.JSONRendering != "" ||
		output.JSONFraming != "" || len(output.PlanResultModes) != 0 || len(output.Fields) != len(fields) {
		return fmt.Errorf("catalog JSON output contract is invalid")
	}
	for index, wanted := range fields {
		got := output.Fields[index]
		if got.Name != wanted.name || got.Type != wanted.fieldType {
			return fmt.Errorf("catalog JSON output field %d is invalid", index)
		}
		if got.Name == "processors" {
			if got.Schema == nil || got.Schema.ID != "bundle-processor-status" || got.Schema.Version != 1 ||
				len(got.Schema.Fields) != len(bundleProcessorStatusSchemaFields) {
				return fmt.Errorf("processor status schema is invalid")
			}
			for fieldIndex := range bundleProcessorStatusSchemaFields {
				if got.Schema.Fields[fieldIndex] != bundleProcessorStatusSchemaFields[fieldIndex] {
					return fmt.Errorf("processor status schema field is invalid")
				}
			}
		} else if got.Schema != nil {
			return fmt.Errorf("unexpected nested output schema")
		}
	}
	return nil
}

func validateWrapperRenderOutput(command helpCommandProjection) error {
	output := command.Contract.Output
	if output.Authority != "catalog" || strings.Join(output.Formats, ",") != "text,json" || output.DefaultFormat != "text" ||
		output.Delivery != "complete" || output.CollectionCoverage != "not_applicable" ||
		output.JSONEnvelope != "wrapper" || output.JSONSchemaVersion != 2 || output.PlanSchema != nil ||
		output.JSONShape != "" || output.JSONRendering != "" || output.JSONFraming != "" || len(output.Fields) != 8 {
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
		{name: "processor_process_attempts", fieldType: "integer"},
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
	wantedReference := helpOutputSchemaReference{Command: "bundle preview", Field: "plan", ID: "wrapper-plan", Version: tailoringplan.SchemaVersion}
	success := func(disposition, stdout, stderr, exitStatus, framing, projection, crossStreamOrder string, stdoutLimit, stderrLimit, sourceAttempts, processorAttempts int) helpPlanResultSuccessProjection {
		return helpPlanResultSuccessProjection{
			Disposition: disposition, Stdout: stdout, Stderr: stderr, ExitStatus: exitStatus,
			Framing: framing, Projection: projection, Delivery: "buffered_after_completion", CrossStreamOrder: crossStreamOrder,
			StdoutLimitBytes: stdoutLimit, StderrLimitBytes: stderrLimit,
			SourceProcessAttempts: sourceAttempts, ProcessorProcessAttempts: processorAttempts,
		}
	}
	wantedModes := []helpPlanResultModeProjection{
		{
			Mode: "transformed_json", SuccessVariants: []helpPlanResultSuccessProjection{
				success("not_applicable", "compact_json", "empty", "zero", "one_value_lf", "visible_json", "not_applicable", maxTransformedBytes, 0, 1, 0),
			},
		},
		{
			Mode: "source_stream_passthrough", SuccessVariants: []helpPlanResultSuccessProjection{
				success("not_applicable", "exact_bounded_source_bytes", "exact_bounded_source_bytes", "source_conventional", "none", "none", "not_preserved", maxCommandOutputBytes, maxSourceStderrBytes, 1, 0),
			},
		},
		{
			Mode: "original_preserving_optimizer", SuccessVariants: []helpPlanResultSuccessProjection{
				success("preserved_before_processor", "exact_bounded_source_bytes", "exact_bounded_source_bytes", "source_conventional", "none", "none", "not_preserved", maxCommandOutputBytes, maxSourceStderrBytes, 1, 0),
				success("preserved_after_processor", "byte_identical_admitted_input", "empty", "zero", "none", "none", "not_applicable", maxCommandOutputBytes, 0, 1, 1),
				success("optimized", "validated_newline_free_utf8_optimizer_summary", "empty", "zero", "none", "none", "not_applicable", maxCommandOutputBytes, 0, 1, 1),
			},
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
		if left[index].Mode != right[index].Mode || len(left[index].SuccessVariants) != len(right[index].SuccessVariants) {
			return false
		}
		for variantIndex := range left[index].SuccessVariants {
			if left[index].SuccessVariants[variantIndex] != right[index].SuccessVariants[variantIndex] {
				return false
			}
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

var wrapperInstallHelpFaultCodes = []string{
	"bundle_file_not_found", "bundle_file_permission_denied", "unsafe_bundle_file", "bundle_file_too_large", "bundle_file_read_failed",
	"invalid_bundle_file", "legacy_tailoring_schema", "bundle_digest_mismatch", "invalid_wrapper_binding", "invalid_bundle_trust_store",
	"bundle_not_adopted", "bundle_source_drift", "source_executable_not_found", "source_identity_unavailable", "unsafe_source_executable",
	"source_identity_changed", "invalid_source_identity", "invalid_processor_executable", "unsafe_processor_executable", "processor_identity_unavailable",
	"processor_identity_changed", "invalid_processor_identity", "bundle_processor_drift", "wrapper_runtime_not_supported", "wrapper_runtime_unavailable",
	"internal_error", "invalid_arguments", "wrapper_artifact_render_failed", "wrapper_artifact_platform_not_supported", "invalid_wrapper_artifact",
	"wrapper_artifact_store_contract", "wrapper_artifact_store_unsafe", "wrapper_artifact_capacity_exceeded", "wrapper_artifact_not_found",
	"wrapper_artifact_collision", "wrapper_artifact_tampered", "wrapper_artifact_mutation_uncertain", "invalid_mutation_contract",
	"missing_mutation_action", "missing_mutation_policy", "mutation_rejected", "unclassified_mutation_outcome", "output_contract_exceeded",
	"output_encoding_failed", "mutation_output_write_failed", "operation_canceled",
}

var wrapperStatusHelpFaultCodes = []string{
	"invalid_arguments", "wrapper_artifact_platform_not_supported", "invalid_wrapper_artifact", "wrapper_artifact_store_contract",
	"wrapper_artifact_store_unsafe", "wrapper_artifact_capacity_exceeded", "wrapper_artifact_not_found", "wrapper_artifact_collision",
	"wrapper_artifact_tampered", "wrapper_artifact_status_unavailable", "output_contract_exceeded", "output_encoding_failed",
	"internal_error", "output_write_failed", "operation_canceled",
}

var wrapperRemoveHelpFaultCodes = []string{
	"invalid_arguments", "wrapper_artifact_platform_not_supported", "invalid_wrapper_artifact", "wrapper_artifact_store_contract",
	"wrapper_artifact_store_unsafe", "wrapper_artifact_capacity_exceeded", "wrapper_artifact_not_found", "wrapper_artifact_collision",
	"wrapper_artifact_tampered", "wrapper_artifact_mutation_uncertain", "invalid_mutation_contract", "missing_mutation_action",
	"missing_mutation_policy", "mutation_rejected", "unclassified_mutation_outcome", "output_contract_exceeded", "output_encoding_failed",
	"internal_error", "mutation_output_write_failed", "operation_canceled",
}

func validateHelpFaultCodes(got []helpFaultDeclaration, wanted []string) error {
	if len(got) != len(wanted) {
		return fmt.Errorf("fault code inventory length is invalid")
	}
	for index := range wanted {
		if got[index].Code != wanted[index] || got[index].Kind == "" || len(got[index].NextActions) == 0 {
			return fmt.Errorf("fault code inventory entry %d is invalid", index)
		}
		for _, action := range got[index].NextActions {
			if action.Command == "" || action.Reason == "" {
				return fmt.Errorf("fault code inventory recovery %d is invalid", index)
			}
		}
	}
	return nil
}

func verifyPackagedHelp(ctx context.Context, runner journeyRunner) (packagedHelpEvidence, error) {
	root, err := runner.success(ctx, "success", "help", "--format", "agent")
	if err != nil {
		return packagedHelpEvidence{}, fmt.Errorf("packaged root help verification failed")
	}
	wanted := []string{
		"source inspect", "processor inspect", "spec init", "spec validate", "bundle build", "bundle status",
		"bundle trust", "bundle preview", "bundle execute", "wrapper render", "wrapper run", "wrapper install", "wrapper status", "wrapper remove",
	}
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
		case "wrapper install":
			faultErr = validateHelpFaultCodes(command.Contract.Errors, wrapperInstallHelpFaultCodes)
		case "wrapper status":
			faultErr = validateHelpFaultCodes(command.Contract.Errors, wrapperStatusHelpFaultCodes)
		case "wrapper remove":
			faultErr = validateHelpFaultCodes(command.Contract.Errors, wrapperRemoveHelpFaultCodes)
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
	if err := json.Unmarshal(value, &document); err != nil || document.SchemaVersion != 12 || document.View != "index" || document.Program != "atr" {
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
	if err := json.Unmarshal(value, &document); err != nil || document.SchemaVersion != 12 || document.View != "scope" || document.Program != "atr" || document.Scope.Selector != path || document.Scope.Kind != "command" || len(document.Commands) != 1 || document.Commands[0].Path != path {
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
			command.Contract.Output.Fields[2].Description != "Exact bounded offline probe attempts: four for github-cli contract 2 and three for go-cli contract 2." {
			return helpCommandProjection{}, fmt.Errorf("source inspection attempt contract is incomplete")
		}
		if err := validateOutputSchema(command, "catalog", "source-command-catalog", 2, sourceCatalogSchemaFields); err != nil {
			return helpCommandProjection{}, fmt.Errorf("source catalog schema is incomplete")
		}
	case "processor inspect":
		if command.Usage != "atr processor inspect --adapter=rtk --executable <absolute-path>" || validateInputs(command.Contract.Inputs, []helpInputProjection{
			input("--adapter", "flag", "single", "rtk"), input("--executable", "flag", "single"),
		}) != nil {
			return helpCommandProjection{}, fmt.Errorf("processor inspection invocation contract is incomplete")
		}
		if len(command.Contract.Output.Fields) != 3 || command.Contract.Output.Fields[2].Name != "processor_process_attempts" ||
			command.Contract.Output.Fields[2].Description != "Exact isolated processor probe attempts; successful inspection is always one." {
			return helpCommandProjection{}, fmt.Errorf("processor inspection attempt contract is incomplete")
		}
		if err := validateOutputSchema(command, "observation", "processor-observation", 1, processorObservationSchemaFields); err != nil {
			return helpCommandProjection{}, fmt.Errorf("processor observation schema is incomplete")
		}
		for _, marker := range []string{"official RTK v0.43.0", "explicit absolute path", "exactly one no-shell --version", "atsura.processor.rtk_isolated.v2"} {
			if !strings.Contains(prerequisites, marker) {
				return helpCommandProjection{}, fmt.Errorf("processor inspection marker is missing")
			}
		}
	case "spec init":
		if command.Usage != "atr spec init --catalog <path> [--processor <inspection.json>] -- <command>" || validateInputs(command.Contract.Inputs, []helpInputProjection{
			input("--catalog", "flag", "single"), typedInput("--processor", "flag", "text", "single", false), input("command", "argument", "repeatable"),
		}) != nil || !strings.Contains(command.Summary, "schema-5") || !strings.Contains(command.Contract.Outcome, "finite registry-owned optimizer") {
			return helpCommandProjection{}, fmt.Errorf("specification baseline contract is incomplete")
		}
		if err := validateOutputSchema(command, "specification", "tailoring-specification", tailoringbundle.SpecificationSchemaVersion, tailoringSpecificationSchemaFields); err != nil {
			return helpCommandProjection{}, fmt.Errorf("specification schema is incomplete")
		}
		for _, marker := range []string{"Without --processor", "With --processor", "output.kind=projection", "Optimizers require", "Arbitrary shell"} {
			if !strings.Contains(prerequisites, marker) {
				return helpCommandProjection{}, fmt.Errorf("specification authoring marker is missing")
			}
		}
	case "spec validate":
		if command.Usage != "atr spec validate --catalog <path> --spec <path>" || validateInputs(command.Contract.Inputs, []helpInputProjection{input("--catalog", "flag", "single"), input("--spec", "flag", "single")}) != nil {
			return helpCommandProjection{}, fmt.Errorf("specification validation invocation contract is incomplete")
		}
		if err := validateOutputSchema(command, "specification", "tailoring-specification", tailoringbundle.SpecificationSchemaVersion, tailoringSpecificationSchemaFields); err != nil {
			return helpCommandProjection{}, fmt.Errorf("normalized specification schema is incomplete")
		}
	case "bundle build":
		if command.Usage != "atr bundle build --catalog <path> --spec <path> [--processor <inspection.json>]" || validateInputs(command.Contract.Inputs, []helpInputProjection{
			input("--catalog", "flag", "single"), input("--spec", "flag", "single"), typedInput("--processor", "flag", "text", "single", false),
		}) != nil || validateCatalogJSONOutput(command, "build", 2, []struct{ name, fieldType string }{
			{name: "bundle_digest", fieldType: "string"}, {name: "bundle", fieldType: "object"},
		}) != nil {
			return helpCommandProjection{}, fmt.Errorf("bundle build contract is incomplete")
		}
		for _, marker := range []string{"schema-5 specification", "optimizer specification requires exactly one explicit --processor", "without an optimizer rejects that option", "no processor inspection or execution"} {
			if !strings.Contains(prerequisites, marker) {
				return helpCommandProjection{}, fmt.Errorf("bundle build marker is missing")
			}
		}
	case "bundle status":
		if command.Usage != "atr bundle status --bundle <path>" || validateInputs(command.Contract.Inputs, []helpInputProjection{input("--bundle", "flag", "single")}) != nil ||
			validateCatalogJSONOutput(command, "status", 3, []struct{ name, fieldType string }{
				{name: "bundle_digest", fieldType: "string"}, {name: "catalog_digest", fieldType: "string"},
				{name: "specification_digest", fieldType: "string"}, {name: "adoption", fieldType: "string"},
				{name: "source", fieldType: "string"}, {name: "adopted", fieldType: "boolean"},
				{name: "source_path", fieldType: "string"}, {name: "source_sha256", fieldType: "string"},
				{name: "source_version", fieldType: "string"}, {name: "processors", fieldType: "array"},
				{name: "source_process_attempts", fieldType: "integer"}, {name: "processor_process_attempts", fieldType: "integer"},
			}) != nil || !strings.Contains(prerequisites, "source and processor file identities without starting either executable") {
			return helpCommandProjection{}, fmt.Errorf("bundle status contract is incomplete")
		}
	case "bundle trust":
		if command.Usage != "atr bundle trust --bundle <path>" || validateInputs(command.Contract.Inputs, []helpInputProjection{input("--bundle", "flag", "single")}) != nil ||
			validateCatalogJSONOutput(command, "trust", 3, []struct{ name, fieldType string }{
				{name: "bundle_digest", fieldType: "string"}, {name: "adopted", fieldType: "boolean"},
				{name: "already_adopted", fieldType: "boolean"}, {name: "source", fieldType: "string"},
				{name: "processors", fieldType: "array"}, {name: "source_process_attempts", fieldType: "integer"},
				{name: "processor_process_attempts", fieldType: "integer"},
			}) != nil {
			return helpCommandProjection{}, fmt.Errorf("bundle trust contract is incomplete")
		}
		for _, marker := range []string{"exact source and every bound processor identity are current", "interactive controlling terminal", "adoption is not source authorization"} {
			if !strings.Contains(prerequisites, marker) {
				return helpCommandProjection{}, fmt.Errorf("bundle trust marker is missing")
			}
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
		for _, marker := range []string{"Linux or macOS", "source, processor, plus Atsura executable identities", "portable POSIX Name", "fixed-utility", "complete included surface", "exact processor tuple", "caller-owned"} {
			if !strings.Contains(prerequisites, marker) {
				return helpCommandProjection{}, fmt.Errorf("wrapper render admission marker is missing")
			}
		}
	case "wrapper run":
		contractVersion := fmt.Sprintf("%d", wrapperbinding.ContractVersion)
		wantedUsage := "atr wrapper run --contract-version=" + contractVersion + " --bundle=<absolute-path> --bundle-digest=<sha256> --runtime-path=<absolute-path> --runtime-sha256=<sha256> --runtime-size=<bytes> -- [argv]"
		if command.Usage != wantedUsage || validateInputs(command.Contract.Inputs, []helpInputProjection{
			typedInput("--contract-version", "flag", "integer", "single", true, contractVersion),
			typedInput("--bundle", "flag", "text", "single", true),
			typedInput("--bundle-digest", "flag", "text", "single", true),
			typedInput("--runtime-path", "flag", "text", "single", true),
			typedInput("--runtime-sha256", "flag", "text", "single", true),
			typedInput("--runtime-size", "flag", "integer", "single", true),
			typedInput("argv", "argument", "text", "repeatable", false),
		}) != nil || validateWrapperRunOutput(command) != nil {
			return helpCommandProjection{}, fmt.Errorf("wrapper run contract is incomplete")
		}
		for _, marker := range []string{"complete closure emitted by wrapper render", "exact bundle to remain adopted", "transformed_json", "source_stream_passthrough", "original_preserving_optimizer", "preserved_before_processor", "preserved_after_processor", "optimized", "source and processor each start at most once", "processor fault never falls back", "without a shell"} {
			if !strings.Contains(prerequisites, marker) {
				return helpCommandProjection{}, fmt.Errorf("wrapper run admission marker is missing")
			}
		}
	case "wrapper install":
		mutation := command.Contract.Mutation
		fixed := command.Contract.FixedTarget
		if command.Usage != "atr wrapper install --bundle <absolute-path>" || command.Effect != "create" || command.Role != "act" ||
			validateInputs(command.Contract.Inputs, []helpInputProjection{typedInput("--bundle", "flag", "text", "single", true)}) != nil ||
			validateCatalogJSONOutput(command, "installation", 1, []struct{ name, fieldType string }{
				{name: "command", fieldType: "string"}, {name: "path", fieldType: "string"}, {name: "bin_path", fieldType: "string"},
				{name: "already_installed", fieldType: "boolean"}, {name: "source_process_attempts", fieldType: "integer"},
				{name: "processor_process_attempts", fieldType: "integer"},
			}) != nil || fixed == nil || fixed.Kind != "wrapper-shim-store" || fixed.ID != "selected" || fixed.Scope != "tool_local" ||
			mutation == nil || mutation.TargetKind != "wrapper-shim-store" || mutation.TargetInputs == nil || len(mutation.TargetInputs) != 0 || mutation.TargetIDInput != "" ||
			mutation.Impact.Cardinality != "one" || mutation.Impact.Notification != "no" || mutation.Impact.AccessChange != "yes" || mutation.Impact.Destructive != "no" ||
			command.ProducesRefs == nil || len(command.ProducesRefs) != 0 || command.ConsumesRefs == nil || len(command.ConsumesRefs) != 0 {
			return helpCommandProjection{}, fmt.Errorf("wrapper install contract is incomplete")
		}
		for _, marker := range []string{"Linux or macOS", "starts neither source nor processor", "never edits PATH", "startup files", "coding-agent settings"} {
			if !strings.Contains(prerequisites, marker) {
				return helpCommandProjection{}, fmt.Errorf("wrapper install admission marker is missing")
			}
		}
	case "wrapper status":
		if command.Usage != "atr wrapper status" || command.Effect != "read" || command.Role != "discover" ||
			command.Contract.Inputs == nil || len(command.Contract.Inputs) != 0 ||
			validateCatalogJSONOutputCoverage(command, "artifacts", 1, "exhaustive", []struct{ name, fieldType string }{
				{name: "reference", fieldType: "string"}, {name: "command", fieldType: "string"}, {name: "state", fieldType: "string"},
				{name: "path", fieldType: "string"}, {name: "material_sha256", fieldType: "string"},
			}) != nil || command.Contract.FixedTarget != nil || command.Contract.Mutation != nil ||
			len(command.ProducesRefs) != 1 || command.ProducesRefs[0].Kind != "wrapper-shim-artifact" || command.ProducesRefs[0].Field != "reference" ||
			command.ConsumesRefs == nil || len(command.ConsumesRefs) != 0 || len(command.Contract.Output.Fields) != 5 ||
			command.Contract.Output.Fields[0].ReferenceKind != "wrapper-shim-artifact" {
			return helpCommandProjection{}, fmt.Errorf("wrapper status contract is incomplete")
		}
		for _, marker := range []string{"Linux or macOS", "starts no source or processor process", "foreign", "symlinked", "special", "tampered"} {
			if !strings.Contains(prerequisites, marker) {
				return helpCommandProjection{}, fmt.Errorf("wrapper status admission marker is missing")
			}
		}
	case "wrapper remove":
		mutation := command.Contract.Mutation
		if command.Usage != "atr wrapper remove --artifact <reference>" || command.Effect != "write" || command.Role != "act" ||
			validateInputs(command.Contract.Inputs, []helpInputProjection{typedInput("--artifact", "flag", "text", "single", true)}) != nil ||
			validateCatalogJSONOutput(command, "removal", 1, []struct{ name, fieldType string }{
				{name: "command", fieldType: "string"}, {name: "path", fieldType: "string"}, {name: "removed", fieldType: "boolean"},
				{name: "source_process_attempts", fieldType: "integer"}, {name: "processor_process_attempts", fieldType: "integer"},
			}) != nil || command.Contract.FixedTarget != nil || mutation == nil || mutation.TargetKind != "wrapper-shim-artifact" ||
			len(mutation.TargetInputs) != 1 || mutation.TargetInputs[0] != "--artifact" || mutation.TargetIDInput != "--artifact" ||
			mutation.Impact.Cardinality != "one" || mutation.Impact.Notification != "no" || mutation.Impact.AccessChange != "yes" || mutation.Impact.Destructive != "yes" ||
			command.ProducesRefs == nil || len(command.ProducesRefs) != 0 || len(command.ConsumesRefs) != 1 ||
			command.ConsumesRefs[0].Kind != "wrapper-shim-artifact" || command.ConsumesRefs[0].Argument != "--artifact" {
			return helpCommandProjection{}, fmt.Errorf("wrapper remove contract is incomplete")
		}
		for _, marker := range []string{"exact opaque reference", "immutable manifest", "symlinked", "special", "tampered", "unknown", "never deleted"} {
			if !strings.Contains(prerequisites, marker) {
				return helpCommandProjection{}, fmt.Errorf("wrapper remove admission marker is missing")
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
			Argv: []string{"pr", "list", "--limit=30", "--json=number,title,state"},
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
				Argv: []string{"pr", "list", "--limit=30", "--json=number,title,state"},
			},
			fixtureAttemptRecord{
				SchemaVersion: 1, Kind: "runtime", Mode: "success",
				Argv: []string{"pr", "list", "--limit=2", "--json=number,title,state"},
			},
			fixtureAttemptRecord{
				SchemaVersion: 1, Kind: "runtime", Mode: "success",
				Argv: []string{
					"issue", "list", "--search=append value", "--label=one", "--label=two", "--limit=1",
				},
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
				Argv: []string{"pr", "list", "--limit=30", "--json=number,title,state"},
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

func loadSpecificationDraft(ctx context.Context, workRoot, name string, value []byte) (tailoringbundle.Specification, error) {
	if ctx == nil || name == "" || len(value) == 0 {
		return tailoringbundle.Specification{}, fmt.Errorf("specification draft input is incomplete")
	}
	path := filepath.Join(workRoot, name+"-draft.yaml")
	if err := writePrivate(path, value); err != nil {
		return tailoringbundle.Specification{}, err
	}
	return specyaml.New().Load(ctx, path)
}

func transformDraft(value tailoringbundle.Specification, command []string) (tailoringbundle.Specification, error) {
	if len(command) != 2 || command[1] != "list" || (command[0] != "issue" && command[0] != "pr") {
		return tailoringbundle.Specification{}, fmt.Errorf("draft command is unsupported")
	}
	if err := validateIdentityDraft(value, command); err != nil {
		return tailoringbundle.Specification{}, err
	}
	value.Commands = []tailoringbundle.CommandEntry{projectionCommandEntry(command)}
	return tailoringbundle.SortSpecification(value), nil
}

func transformSourceStreamDraft(value tailoringbundle.Specification, name string, command []string) (tailoringbundle.Specification, error) {
	if len(command) != 2 || command[1] != "list" {
		return tailoringbundle.Specification{}, fmt.Errorf("source-stream draft command is unsupported")
	}
	if err := validateIdentityDraft(value, command); err != nil {
		return tailoringbundle.Specification{}, err
	}
	var entry tailoringbundle.CommandEntry
	switch name {
	case "identity":
		if command[0] != "pr" {
			return tailoringbundle.Specification{}, fmt.Errorf("identity draft command is unsupported")
		}
		entry = identityCommandEntry(command)
	case "append_only":
		if command[0] != "issue" {
			return tailoringbundle.Specification{}, fmt.Errorf("append-only draft command is unsupported")
		}
		entry = appendOnlyCommandEntry(command)
	default:
		return tailoringbundle.Specification{}, fmt.Errorf("source-stream draft case is unsupported")
	}
	value.Commands = []tailoringbundle.CommandEntry{entry}
	return tailoringbundle.SortSpecification(value), nil
}

func combineCommandSpecifications(prSpecification, issueSpecification tailoringbundle.Specification) (tailoringbundle.Specification, error) {
	if prSpecification.SchemaVersion != tailoringbundle.SpecificationSchemaVersion ||
		prSpecification.SchemaVersion != issueSpecification.SchemaVersion ||
		prSpecification.CatalogDigest != issueSpecification.CatalogDigest ||
		!digestValue(prSpecification.CatalogDigest) ||
		!reflect.DeepEqual(prSpecification.Surface, issueSpecification.Surface) ||
		len(prSpecification.Commands) != 1 || len(issueSpecification.Commands) != 1 ||
		!reflect.DeepEqual(prSpecification.Commands[0], projectionCommandEntry([]string{"pr", "list"})) ||
		!reflect.DeepEqual(issueSpecification.Commands[0], appendOnlyCommandEntry([]string{"issue", "list"})) {
		return tailoringbundle.Specification{}, fmt.Errorf("command specifications cannot share one bundle")
	}
	return tailoringbundle.SortSpecification(tailoringbundle.Specification{
		SchemaVersion: prSpecification.SchemaVersion,
		CatalogDigest: prSpecification.CatalogDigest,
		Surface:       prSpecification.Surface,
		Commands: []tailoringbundle.CommandEntry{
			issueSpecification.Commands[0],
			prSpecification.Commands[0],
		},
	}), nil
}

func validateIdentityDraft(value tailoringbundle.Specification, command []string) error {
	if !digestValue(value.CatalogDigest) {
		return fmt.Errorf("draft catalog digest is invalid")
	}
	wanted := tailoringbundle.Specification{
		SchemaVersion: tailoringbundle.SpecificationSchemaVersion,
		CatalogDigest: value.CatalogDigest,
		Surface:       tailoringbundle.Surface{Default: tailoringbundle.SurfaceDefaultExclude},
		Commands: []tailoringbundle.CommandEntry{{
			Command: append([]string{}, command...), Presence: tailoringbundle.PresenceInclude,
			Reason: "Include this verified command without transformation.",
			Options: &tailoringbundle.OptionSurface{
				Default: tailoringbundle.SurfaceDefaultInherit, Include: []string{}, Exclude: []string{},
			},
			Wrapper: &tailoringbundle.Wrapper{
				Kind: tailoringbundle.WrapperIdentity, Before: []tailoringbundle.StageAction{},
				Invoke: tailoringbundle.Invocation{OptionDefaults: []tailoringbundle.OptionDefault{}, AppendArgs: []string{}}, After: []tailoringbundle.StageAction{},
			},
		}},
	}
	if !reflect.DeepEqual(value, wanted) {
		return fmt.Errorf("draft shape is not the expected single-command identity wrapper")
	}
	return nil
}

func projectionCommandEntry(command []string) tailoringbundle.CommandEntry {
	optionDefaults := []tailoringbundle.OptionDefault{}
	if reflect.DeepEqual(command, []string{"pr", "list"}) {
		optionDefaults = []tailoringbundle.OptionDefault{{Option: "--limit", Value: "30"}}
	}
	return tailoringbundle.CommandEntry{
		Command: append([]string{}, command...), Presence: tailoringbundle.PresenceInclude,
		Reason: "Return one reviewed compact result.",
		Options: &tailoringbundle.OptionSurface{
			Default: tailoringbundle.SurfaceDefaultExclude, Include: []string{"--limit"}, Exclude: []string{},
		},
		Wrapper: &tailoringbundle.Wrapper{
			Kind: tailoringbundle.WrapperTransform, Before: []tailoringbundle.StageAction{},
			Invoke: tailoringbundle.Invocation{OptionDefaults: optionDefaults, AppendArgs: []string{"--json=number,title,state"}},
			Output: &tailoringbundle.Output{
				Kind: tailoringbundle.OutputKindProjection,
				Projection: &tailoringbundle.Projection{
					Input: "json", Select: []string{"number", "title", "state"},
					Rename: []tailoringbundle.Rename{{From: "number", To: "id"}}, Render: "compact_json",
				},
			},
			After: []tailoringbundle.StageAction{},
		},
	}
}

func appendOnlyCommandEntry(command []string) tailoringbundle.CommandEntry {
	return tailoringbundle.CommandEntry{
		Command: append([]string{}, command...), Presence: tailoringbundle.PresenceInclude,
		Reason: "Append one fixed reviewed source argument and preserve its streams.",
		Options: &tailoringbundle.OptionSurface{
			Default: tailoringbundle.SurfaceDefaultExclude, Include: []string{"--label", "--search"}, Exclude: []string{},
		},
		Wrapper: &tailoringbundle.Wrapper{
			Kind: tailoringbundle.WrapperTransform, Before: []tailoringbundle.StageAction{},
			Invoke: tailoringbundle.Invocation{OptionDefaults: []tailoringbundle.OptionDefault{}, AppendArgs: []string{"--limit=1"}}, After: []tailoringbundle.StageAction{},
		},
	}
}

func identityCommandEntry(command []string) tailoringbundle.CommandEntry {
	return tailoringbundle.CommandEntry{
		Command: append([]string{}, command...), Presence: tailoringbundle.PresenceInclude,
		Reason: "Preserve the exact reviewed source streams.",
		Options: &tailoringbundle.OptionSurface{
			Default: tailoringbundle.SurfaceDefaultExclude,
			Include: []string{"--label", "--repo", "--search"}, Exclude: []string{},
		},
		Wrapper: &tailoringbundle.Wrapper{
			Kind: tailoringbundle.WrapperIdentity, Before: []tailoringbundle.StageAction{},
			Invoke: tailoringbundle.Invocation{OptionDefaults: []tailoringbundle.OptionDefault{}, AppendArgs: []string{}}, After: []tailoringbundle.StageAction{},
		},
	}
}

type inspectionEvidence struct {
	CatalogDigest string `json:"catalog_digest"`
	Catalog       struct {
		SchemaVersion int `json:"schema_version"`
		Adapter       struct {
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

type processorInspectionEvidence struct {
	ObservationDigest        string                       `json:"observation_digest"`
	Observation              processorprocess.Observation `json:"observation"`
	ProcessorProcessAttempts int                          `json:"processor_process_attempts"`
}

func decodeProcessorInspection(value []byte) (processorInspectionEvidence, error) {
	var document struct {
		SchemaVersion int                         `json:"schema_version"`
		Inspection    processorInspectionEvidence `json:"inspection"`
	}
	decoder := json.NewDecoder(bytes.NewReader(value))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&document); err != nil || document.SchemaVersion != 1 {
		return processorInspectionEvidence{}, fmt.Errorf("invalid processor inspection document")
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return processorInspectionEvidence{}, fmt.Errorf("processor inspection contains trailing JSON")
	}
	return document.Inspection, nil
}

func validateProcessorInspection(value processorInspectionEvidence, processor preparedProcessor) error {
	observation := value.Observation
	digest, err := observation.Digest()
	if err != nil || value.ObservationDigest != digest || value.ProcessorProcessAttempts != 1 || observation.Probe.Attempts != 1 {
		return fmt.Errorf("processor observation digest or attempts are invalid")
	}
	if observation.Adapter.Kind != processor.metadata.ProcessorKind() || observation.Adapter.ContractVersion != 1 ||
		observation.Platform.OS+"/"+observation.Platform.Arch != processor.metadata.Target() ||
		observation.Identity.ResolvedPath != processor.executablePath || observation.Identity.SHA256 != processor.metadata.BinarySHA256() ||
		observation.Identity.Size != processor.metadata.BinarySize() || observation.Version != processor.metadata.Version() ||
		len(observation.Probe.Argv) != 1 || observation.Probe.Argv[0] != "--version" ||
		observation.Probe.EnvironmentContract != processorprocess.EnvironmentRTKIsolatedV2 {
		return fmt.Errorf("processor observation does not match pinned provenance")
	}
	return nil
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
	return validateSpecificationEvidenceCounts(value, catalogDigest, 1, wantedIdentity, wantedTransform)
}

func validateSpecificationEvidenceCounts(value []byte, catalogDigest string, wantedCommands, wantedIdentity, wantedTransform int) error {
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
			Specification         struct {
				SchemaVersion int `json:"schema_version"`
			} `json:"specification"`
		} `json:"validation"`
	}
	if err := json.Unmarshal(value, &document); err != nil || document.SchemaVersion != 2 {
		return fmt.Errorf("invalid validation document")
	}
	result := document.Validation
	if wantedCommands <= 0 || !result.Valid || result.CatalogDigest != catalogDigest || !digestValue(result.SpecificationDigest) ||
		result.Specification.SchemaVersion != tailoringbundle.SpecificationSchemaVersion || result.CommandCount != wantedCommands ||
		result.IncludedCount != wantedCommands || result.ExcludedCount != 0 || result.IdentityWrapperCount != wantedIdentity ||
		result.TransformWrapperCount != wantedTransform || wantedIdentity+wantedTransform != wantedCommands {
		return fmt.Errorf("unexpected validation evidence")
	}
	return nil
}

func decodeBundleDigest(value []byte) (string, error) {
	var document struct {
		SchemaVersion int `json:"schema_version"`
		Build         struct {
			BundleDigest string `json:"bundle_digest"`
			Bundle       struct {
				SchemaVersion int `json:"schema_version"`
			} `json:"bundle"`
		} `json:"build"`
	}
	if err := json.Unmarshal(value, &document); err != nil || document.SchemaVersion != 2 || document.Build.Bundle.SchemaVersion != tailoringbundle.BundleSchemaVersion || !digestValue(document.Build.BundleDigest) {
		return "", fmt.Errorf("invalid bundle document")
	}
	return document.Build.BundleDigest, nil
}

type processorStatusExpectation struct {
	Contract, AdapterKind, Version, ResolvedPath, SHA256 string
	Size                                                 int64
}

func validateStatus(value []byte, digest string, state bundletrust.State, wantedProcessors ...processorStatusExpectation) error {
	var document struct {
		SchemaVersion int `json:"schema_version"`
		Status        struct {
			BundleDigest string            `json:"bundle_digest"`
			Adoption     bundletrust.State `json:"adoption"`
			Source       string            `json:"source"`
			Adopted      bool              `json:"adopted"`
			Processors   []struct {
				Contract     string                     `json:"contract"`
				AdapterKind  string                     `json:"adapter_kind"`
				Version      string                     `json:"version"`
				ResolvedPath string                     `json:"resolved_path"`
				SHA256       string                     `json:"sha256"`
				Size         int64                      `json:"size"`
				State        bundletrust.ProcessorState `json:"state"`
			} `json:"processors"`
			SourceProcessAttempts    int `json:"source_process_attempts"`
			ProcessorProcessAttempts int `json:"processor_process_attempts"`
		} `json:"status"`
	}
	if err := json.Unmarshal(value, &document); err != nil || document.SchemaVersion != 3 {
		return fmt.Errorf("invalid status document")
	}
	wantedAdopted := state == bundletrust.StateAdopted
	if document.Status.BundleDigest != digest || document.Status.Adoption != state || document.Status.Source != "current" ||
		document.Status.Adopted != wantedAdopted || document.Status.SourceProcessAttempts != 0 ||
		document.Status.ProcessorProcessAttempts != 0 || len(document.Status.Processors) != len(wantedProcessors) {
		return fmt.Errorf("unexpected status evidence")
	}
	for index, wanted := range wantedProcessors {
		got := document.Status.Processors[index]
		if got.Contract != wanted.Contract || got.AdapterKind != wanted.AdapterKind || got.Version != wanted.Version ||
			got.ResolvedPath != wanted.ResolvedPath || got.SHA256 != wanted.SHA256 || got.Size != wanted.Size || got.State != bundletrust.ProcessorCurrent {
			return fmt.Errorf("unexpected processor status evidence")
		}
	}
	return nil
}

type previewEvidence struct {
	PlanDigest            string             `json:"plan_digest"`
	Plan                  tailoringplan.Plan `json:"plan"`
	SourceProcessAttempts int                `json:"source_process_attempts"`
}

func decodePreview(value []byte) (previewEvidence, error) {
	var document struct {
		SchemaVersion int             `json:"schema_version"`
		Preview       previewEvidence `json:"preview"`
	}
	decoder := json.NewDecoder(bytes.NewReader(value))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&document); err != nil || document.SchemaVersion != 2 {
		return previewEvidence{}, fmt.Errorf("invalid preview document")
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return previewEvidence{}, fmt.Errorf("preview document contains trailing JSON")
	}
	digest, err := document.Preview.Plan.Digest()
	if err != nil || digest != document.Preview.PlanDigest {
		return previewEvidence{}, fmt.Errorf("preview plan digest is invalid")
	}
	return document.Preview, nil
}

func validateGoIdentityPreviewPlan(plan tailoringplan.Plan, bundleDigest string, inspection inspectionEvidence) error {
	if err := plan.Validate(); err != nil {
		return err
	}
	source := inspection.Catalog.Source
	if plan.BundleDigest != bundleDigest || plan.CatalogDigest != inspection.CatalogDigest ||
		plan.Source.RequestedExecutable != source.RequestedExecutable || plan.Source.ResolvedPath != source.ResolvedPath ||
		plan.Source.Version != source.Version || plan.Source.AdapterKind != goAdapterKind ||
		plan.Source.AdapterContractVersion != goAdapterContract || plan.SurfaceOrigin != tailoringplan.SurfaceOriginExplicit ||
		plan.ResultMode != tailoringplan.ResultModeSourceStreamPassthrough || plan.WrapperKind != "identity" || plan.Processor != nil ||
		!reflect.DeepEqual(plan.MatchedCommand, []string{"test"}) ||
		!reflect.DeepEqual(plan.OriginalArgv, []string{source.ResolvedPath, "test"}) ||
		!reflect.DeepEqual(plan.TransformedArgv, []string{source.ResolvedPath, "test"}) ||
		!reflect.DeepEqual(plan.Stages.Invoke.Args, []string{"test"}) ||
		len(plan.Stages.Invoke.OptionDefaults) != 0 || len(plan.Stages.Invoke.AppliedOptionDefaults) != 0 ||
		plan.Stages.Invoke.MaxAttempts != 1 || plan.Stages.Invoke.TimeoutMillis != sourceprocess.MaxTimeout.Milliseconds() ||
		plan.Stages.Invoke.StdoutLimitBytes != sourceprocess.MaxStdoutBytes ||
		plan.Stages.Invoke.StderrLimitBytes != sourceprocess.MaxStderrBytes {
		return fmt.Errorf("identity plan does not match the exact Go test tuple")
	}
	return nil
}

func validateGoOptimizerPreviewPlan(
	plan tailoringplan.Plan,
	bundleDigest string,
	inspection inspectionEvidence,
	observation processorprocess.Observation,
) (*optimizerExecutionEvidence, error) {
	if err := processorcompat.New().VerifyPlan(plan); err != nil {
		return nil, err
	}
	source := inspection.Catalog.Source
	if plan.BundleDigest != bundleDigest || plan.CatalogDigest != inspection.CatalogDigest ||
		plan.Source.RequestedExecutable != source.RequestedExecutable || plan.Source.ResolvedPath != source.ResolvedPath ||
		plan.Source.Version != source.Version || plan.Processor == nil ||
		!reflect.DeepEqual(plan.Processor.Observation, observation) ||
		len(plan.Stages.Invoke.OptionDefaults) != 0 || len(plan.Stages.Invoke.AppliedOptionDefaults) != 0 {
		return nil, fmt.Errorf("optimizer plan does not match inspected source and processor evidence")
	}
	invoke := plan.Stages.Invoke
	processor := plan.Processor
	return &optimizerExecutionEvidence{
		CallerArgv: append([]string(nil), plan.OriginalArgv[1:]...), SourceArgv: append([]string(nil), invoke.Args...),
		SourceStdinMode: string(invoke.StdinMode), SourceWorkingDirectoryMode: string(invoke.WorkingDirectoryMode),
		SourceEnvironmentMode: string(invoke.EnvironmentMode), SourceMaxAttempts: invoke.MaxAttempts,
		SourceTimeoutMillis: invoke.TimeoutMillis, SourceStdoutLimitBytes: invoke.StdoutLimitBytes,
		SourceStderrLimitBytes: invoke.StderrLimitBytes, InputFormat: processor.InputFormat,
		OutputFormat: processor.OutputFormat, AllowOriginalOutput: processor.AllowOriginalOutput,
		ProcessorArgv: append([]string(nil), processor.Execution.Args...), ProcessorStdinMode: processor.Execution.StdinMode,
		ProcessorWorkingDirectoryMode: processor.Execution.WorkingDirectoryMode,
		ProcessorEnvironmentContract:  processor.Execution.EnvironmentContract,
		ProcessorMaxAttempts:          processor.Execution.MaxAttempts, ProcessorTimeoutMillis: processor.Execution.TimeoutMillis,
		ProcessorStdoutLimitBytes: processor.Execution.StdoutLimitBytes,
		ProcessorStderrLimitBytes: processor.Execution.StderrLimitBytes,
	}, nil
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
