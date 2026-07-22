package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/tasuku43/atsura/internal/domain/wrappershim"
	"github.com/tasuku43/atsura/internal/infra/posixshim"
)

const maxLifecycleSnapshotBytes = 8 * 1024 * 1024

type wrapperLifecycleEvidence struct {
	Outcome                            string                             `json:"outcome"`
	ContractVersion                    int                                `json:"contract_version"`
	BinPathSHA256                      string                             `json:"bin_path_sha256"`
	PathPrecedence                     string                             `json:"path_precedence"`
	PathCommands                       []string                           `json:"path_commands"`
	StatusSnapshots                    []wrapperLifecycleStatusEvidence   `json:"status_snapshots"`
	Artifacts                          []installedWrapperArtifactEvidence `json:"artifacts"`
	Faults                             []wrapperLifecycleFaultEvidence    `json:"faults"`
	StoreMutationAttempts              int                                `json:"store_mutation_attempts"`
	SourceProcessAttempts              int                                `json:"source_process_attempts"`
	ProcessorProcessAttempts           int                                `json:"processor_process_attempts"`
	ManagementSourceProcessAttempts    int                                `json:"management_source_process_attempts"`
	ManagementProcessorProcessAttempts int                                `json:"management_processor_process_attempts"`
	ZeroAttemptRejections              int                                `json:"zero_attempt_rejections"`
}

type wrapperLifecycleStatusEvidence struct {
	Name       string   `json:"name"`
	References []string `json:"references"`
}

type installedWrapperArtifactEvidence struct {
	Name                     string   `json:"name"`
	CommandName              string   `json:"command_name"`
	Reference                string   `json:"reference"`
	MaterialSHA256           string   `json:"material_sha256"`
	StatusState              string   `json:"status_state"`
	BundleDigest             string   `json:"bundle_digest"`
	PlanDigest               string   `json:"plan_digest"`
	ExecutionCase            string   `json:"execution_case"`
	CallerArgv               []string `json:"caller_argv"`
	SourceArgv               []string `json:"source_argv"`
	WrapperKind              string   `json:"wrapper_kind"`
	ResultMode               string   `json:"result_mode"`
	HelpStdoutSHA256         string   `json:"help_stdout_sha256"`
	HelpStderrSHA256         string   `json:"help_stderr_sha256"`
	HelpSourceAttempts       int      `json:"help_source_process_attempts"`
	HelpProcessorAttempts    int      `json:"help_processor_process_attempts"`
	StdoutSHA256             string   `json:"stdout_sha256"`
	StderrSHA256             string   `json:"stderr_sha256"`
	SourceExitCode           int      `json:"source_exit_code"`
	SourceProcessAttempts    int      `json:"source_process_attempts"`
	ProcessorProcessAttempts int      `json:"processor_process_attempts"`
	RemoveReference          string   `json:"remove_reference"`
	RemovalOutcome           string   `json:"removal_outcome"`
}

type wrapperLifecycleFaultEvidence struct {
	Name                     string `json:"name"`
	Code                     string `json:"code"`
	FilesystemStateUnchanged bool   `json:"filesystem_state_unchanged"`
	SourceProcessAttempts    int    `json:"source_process_attempts"`
	ProcessorProcessAttempts int    `json:"processor_process_attempts"`
}

type wrapperInstallResult struct {
	Command                  string
	Path                     string
	BinPath                  string
	AlreadyInstalled         bool
	SourceProcessAttempts    int
	ProcessorProcessAttempts int
}

type wrapperStatusArtifact struct {
	Reference      string `json:"reference"`
	Command        string `json:"command"`
	State          string `json:"state"`
	Path           string `json:"path"`
	MaterialSHA256 string `json:"material_sha256"`
}

type wrapperRemoveResult struct {
	Command                  string
	Path                     string
	Removed                  bool
	SourceProcessAttempts    int
	ProcessorProcessAttempts int
}

type managedFilesystemSnapshot struct{ digest string }

func verifyPersistentWrapperLifecycle(
	ctx context.Context,
	runner journeyRunner,
	help packagedHelpEvidence,
	goos, executablePath, workRoot, attemptLog string,
	existingFixtureAttempts int,
	githubJourney preparedCommandJourney,
	githubCases []wrapperCaseEvidence,
	goEvidence goSourceEvidence,
) (wrapperLifecycleEvidence, [][]byte, int, error) {
	if ctx == nil || workRoot == "" || githubJourney.bundlePath() == "" {
		return wrapperLifecycleEvidence{}, nil, 0, fmt.Errorf("persistent wrapper lifecycle input is incomplete")
	}
	storeRoot, err := expectedManagedStoreRoot(runner.environment, goos)
	if err != nil || !pathWithin(workRoot, storeRoot) {
		return wrapperLifecycleEvidence{}, nil, 0, fmt.Errorf("managed wrapper store identity is invalid")
	}
	initialStore, err := snapshotManagedFilesystem(storeRoot)
	if err != nil || initialStore.digest != digestBytes([]byte("absent")) {
		return wrapperLifecycleEvidence{}, nil, 0, fmt.Errorf("managed wrapper store is not initially absent")
	}
	if goos == "windows" {
		return verifyUnsupportedWrapperLifecycle(
			ctx, runner, help, storeRoot, githubJourney.bundlePath(), attemptLog, existingFixtureAttempts, workRoot,
		)
	}
	if goos != "linux" && goos != "darwin" {
		return wrapperLifecycleEvidence{}, nil, 0, fmt.Errorf("persistent wrapper lifecycle platform is unsupported")
	}
	githubCase, err := exactWrapperCase(githubCases, "default_applied")
	if err != nil {
		return wrapperLifecycleEvidence{}, nil, 0, err
	}
	goCase, err := exactWrapperCase(goEvidence.WrapperCases, "go_test_identity")
	if err != nil {
		return wrapperLifecycleEvidence{}, nil, 0, err
	}
	goBundlePath := filepath.Join(workRoot, "go-bundle.json")
	if githubCase.BundleDigest != githubJourney.bundleDigest || githubCase.PlanDigest != githubJourney.planDigest ||
		goCase.BundleDigest != goEvidence.BundleDigest || goCase.PlanDigest != goEvidence.PlanDigest {
		return wrapperLifecycleEvidence{}, nil, 0, fmt.Errorf("persistent wrapper lifecycle outer binding is invalid")
	}
	goAttemptLog := filepath.Join(workRoot, "go-test-attempts.log")
	existingGoAttempts := goEvidence.WrapperSourceAttempts + goEvidence.Optimizer.SourceProcessAttempts
	if err := requireAttempts(attemptLog, existingFixtureAttempts); err != nil {
		return wrapperLifecycleEvidence{}, nil, 0, fmt.Errorf("persistent wrapper lifecycle GitHub baseline is invalid")
	}
	if err := requireGoTestAttempts(goAttemptLog, existingGoAttempts); err != nil {
		return wrapperLifecycleEvidence{}, nil, 0, fmt.Errorf("persistent wrapper lifecycle Go baseline is invalid")
	}

	boundaries := make([][]byte, 0, 48)
	initialStatusOutput, err := runner.success(ctx, "success", "wrapper", "status")
	if err != nil {
		return wrapperLifecycleEvidence{}, nil, 0, fmt.Errorf("initial wrapper status failed")
	}
	initialStatus, err := decodeWrapperStatus(initialStatusOutput.stdout)
	if err != nil || len(initialStatus) != 0 {
		return wrapperLifecycleEvidence{}, nil, 0, fmt.Errorf("initial wrapper status is not empty")
	}
	boundaries = append(boundaries, initialStatusOutput.stdout)

	githubInstallOutput, err := runner.success(ctx, "success", "wrapper", "install", "--bundle", githubJourney.bundlePath())
	if err != nil {
		return wrapperLifecycleEvidence{}, nil, 0, fmt.Errorf("GitHub wrapper installation failed")
	}
	githubInstall, err := decodeWrapperInstall(githubInstallOutput.stdout)
	if err != nil || githubInstall.Command != "gh" || githubInstall.AlreadyInstalled ||
		githubInstall.SourceProcessAttempts != 0 || githubInstall.ProcessorProcessAttempts != 0 {
		return wrapperLifecycleEvidence{}, nil, 0, fmt.Errorf("GitHub wrapper installation evidence is invalid")
	}
	wantedBinPath := filepath.Join(storeRoot, "bin")
	if githubInstall.BinPath != wantedBinPath || githubInstall.Path != filepath.Join(wantedBinPath, "gh") {
		return wrapperLifecycleEvidence{}, nil, 0, fmt.Errorf("GitHub wrapper installation path is invalid")
	}
	boundaries = append(boundaries, githubInstallOutput.stdout)
	githubOnlyOutput, err := runner.success(ctx, "success", "wrapper", "status")
	if err != nil {
		return wrapperLifecycleEvidence{}, nil, 0, fmt.Errorf("GitHub wrapper status failed")
	}
	githubOnly, err := decodeWrapperStatus(githubOnlyOutput.stdout)
	if err != nil || !statusCommands(githubOnly, "gh") {
		return wrapperLifecycleEvidence{}, nil, 0, fmt.Errorf("GitHub wrapper status evidence is invalid")
	}
	boundaries = append(boundaries, githubOnlyOutput.stdout)

	goInstallOutput, err := runner.success(ctx, "success", "wrapper", "install", "--bundle", goBundlePath)
	if err != nil {
		return wrapperLifecycleEvidence{}, nil, 0, fmt.Errorf("Go wrapper installation failed")
	}
	goInstall, err := decodeWrapperInstall(goInstallOutput.stdout)
	if err != nil || goInstall.Command != "go" || goInstall.AlreadyInstalled || goInstall.BinPath != wantedBinPath ||
		goInstall.Path != filepath.Join(wantedBinPath, "go") || goInstall.SourceProcessAttempts != 0 || goInstall.ProcessorProcessAttempts != 0 {
		return wrapperLifecycleEvidence{}, nil, 0, fmt.Errorf("Go wrapper installation evidence is invalid")
	}
	boundaries = append(boundaries, goInstallOutput.stdout)
	installedStatusOutput, err := runner.success(ctx, "success", "wrapper", "status")
	if err != nil {
		return wrapperLifecycleEvidence{}, nil, 0, fmt.Errorf("installed wrapper status failed")
	}
	installedStatus, err := decodeWrapperStatus(installedStatusOutput.stdout)
	if err != nil || !statusCommands(installedStatus, "gh", "go") {
		return wrapperLifecycleEvidence{}, nil, 0, fmt.Errorf("installed wrapper status evidence is invalid")
	}
	boundaries = append(boundaries, installedStatusOutput.stdout)

	githubStatus := installedStatus[0]
	goStatus := installedStatus[1]
	githubManifest, githubShim, err := inspectManagedArtifact(storeRoot, executablePath, githubJourney.bundlePath(), githubCase.BundleDigest, githubStatus)
	if err != nil {
		return wrapperLifecycleEvidence{}, nil, 0, fmt.Errorf("GitHub managed artifact is invalid: %w", err)
	}
	goManifest, _, err := inspectManagedArtifact(storeRoot, executablePath, goBundlePath, goCase.BundleDigest, goStatus)
	if err != nil {
		return wrapperLifecycleEvidence{}, nil, 0, fmt.Errorf("Go managed artifact is invalid: %w", err)
	}
	boundaries = append(boundaries, githubManifest, githubShim, goManifest)
	if err := requireAttempts(attemptLog, existingFixtureAttempts); err != nil {
		return wrapperLifecycleEvidence{}, nil, 0, fmt.Errorf("wrapper management started the GitHub source")
	}
	if err := requireGoTestAttempts(goAttemptLog, existingGoAttempts); err != nil {
		return wrapperLifecycleEvidence{}, nil, 0, fmt.Errorf("wrapper management started the Go source")
	}

	githubHelp, err := runInstalledPATHCommand(ctx, runner, wantedBinPath, "gh", []string{"--help"}, 0)
	if err != nil {
		return wrapperLifecycleEvidence{}, nil, 0, fmt.Errorf("installed GitHub help failed")
	}
	wantedGitHubHelp, err := expectedTailoredHelp(githubCase.BundleDigest, "root")
	if err != nil || !bytes.Equal(githubHelp.stdout, wantedGitHubHelp) || len(githubHelp.stderr) != 0 {
		return wrapperLifecycleEvidence{}, nil, 0, fmt.Errorf("installed GitHub help is not the compiled wrapper view")
	}
	goHelp, err := runInstalledPATHCommand(ctx, runner, wantedBinPath, "go", []string{"--help"}, 0)
	if err != nil {
		return wrapperLifecycleEvidence{}, nil, 0, fmt.Errorf("installed Go help failed")
	}
	wantedGoHelp := expectedGoTailoredHelp(goCase.BundleDigest)
	if !bytes.Equal(goHelp.stdout, wantedGoHelp) || len(goHelp.stderr) != 0 {
		return wrapperLifecycleEvidence{}, nil, 0, fmt.Errorf("installed Go help is not the compiled wrapper view")
	}
	if err := requireAttempts(attemptLog, existingFixtureAttempts); err != nil {
		return wrapperLifecycleEvidence{}, nil, 0, fmt.Errorf("installed help started the GitHub source")
	}
	if err := requireGoTestAttempts(goAttemptLog, existingGoAttempts); err != nil {
		return wrapperLifecycleEvidence{}, nil, 0, fmt.Errorf("installed help started the Go source")
	}
	boundaries = append(boundaries, githubHelp.stdout, githubHelp.stderr, goHelp.stdout, goHelp.stderr)

	githubInvocation, err := runInstalledPATHCommand(ctx, runner, wantedBinPath, "gh", githubCase.CallerArgv, githubCase.SourceExitCode)
	if err != nil || digestBytes(githubInvocation.stdout) != githubCase.StdoutSHA256 ||
		digestBytes(githubInvocation.stderr) != githubCase.StderrSHA256 {
		return wrapperLifecycleEvidence{}, nil, 0, fmt.Errorf("installed GitHub execution does not match wrapper-run evidence")
	}
	if err := requireAttempts(attemptLog, existingFixtureAttempts+1); err != nil {
		return wrapperLifecycleEvidence{}, nil, 0, fmt.Errorf("installed GitHub execution attempt evidence is invalid")
	}
	if err := requireGoTestAttempts(goAttemptLog, existingGoAttempts); err != nil {
		return wrapperLifecycleEvidence{}, nil, 0, fmt.Errorf("installed GitHub execution started the Go source")
	}
	goInvocation, err := runInstalledPATHCommand(ctx, runner, wantedBinPath, "go", goCase.CallerArgv, goCase.SourceExitCode)
	if err != nil || !goTestIdentityOutputPattern.Match(goInvocation.stdout) || len(goInvocation.stderr) != 0 {
		return wrapperLifecycleEvidence{}, nil, 0, fmt.Errorf("installed Go execution does not match wrapper-run semantics")
	}
	if err := requireAttempts(attemptLog, existingFixtureAttempts+1); err != nil {
		return wrapperLifecycleEvidence{}, nil, 0, fmt.Errorf("installed Go execution started the GitHub source unexpectedly")
	}
	if err := requireGoTestAttempts(goAttemptLog, existingGoAttempts+1); err != nil {
		return wrapperLifecycleEvidence{}, nil, 0, fmt.Errorf("installed Go execution attempt evidence is invalid")
	}
	boundaries = append(boundaries, githubInvocation.stdout, githubInvocation.stderr, goInvocation.stdout, goInvocation.stderr)

	processCounts := func() error {
		if err := requireAttempts(attemptLog, existingFixtureAttempts+1); err != nil {
			return err
		}
		return requireGoTestAttempts(goAttemptLog, existingGoAttempts+1)
	}
	faults := make([]wrapperLifecycleFaultEvidence, 0, 6)
	unknownReference, err := wrappershim.NewReference(differentDigest(githubStatus.MaterialSHA256))
	if err != nil || unknownReference.String() == githubStatus.Reference || unknownReference.String() == goStatus.Reference {
		return wrapperLifecycleEvidence{}, nil, 0, fmt.Errorf("unknown wrapper reference fixture is invalid")
	}
	unknownFault, err := runUnchangedLifecycleFault(
		ctx, runner, help, storeRoot, "unknown_reference_remove", "wrapper remove", "wrapper_artifact_not_found", 6,
		[]string{"--error-format=json", "wrapper", "remove", "--artifact", unknownReference.String()}, processCounts,
	)
	if err != nil {
		return wrapperLifecycleEvidence{}, nil, 0, err
	}
	faults = append(faults, unknownFault.evidence)
	boundaries = append(boundaries, unknownFault.boundaries...)

	githubManifestPath := filepath.Join(storeRoot, "records", githubStatus.Reference, "manifest.json")
	tampered := append(append([]byte{}, githubManifest...), []byte("{}\n")...)
	if err := overwriteRegularFixture(githubManifestPath, tampered, 0o600); err != nil {
		return wrapperLifecycleEvidence{}, nil, 0, fmt.Errorf("managed manifest tamper fixture failed")
	}
	tamperedStatusFault, err := runUnchangedLifecycleFault(
		ctx, runner, help, storeRoot, "tampered_status", "wrapper status", "wrapper_artifact_tampered", 10,
		[]string{"--error-format=json", "wrapper", "status"}, processCounts,
	)
	if err != nil {
		return wrapperLifecycleEvidence{}, nil, 0, err
	}
	tamperedRemoveFault, err := runUnchangedLifecycleFault(
		ctx, runner, help, storeRoot, "tampered_remove", "wrapper remove", "wrapper_artifact_tampered", 10,
		[]string{"--error-format=json", "wrapper", "remove", "--artifact", githubStatus.Reference}, processCounts,
	)
	if err != nil {
		return wrapperLifecycleEvidence{}, nil, 0, err
	}
	faults = append(faults, tamperedStatusFault.evidence, tamperedRemoveFault.evidence)
	boundaries = append(boundaries, tamperedStatusFault.boundaries...)
	boundaries = append(boundaries, tamperedRemoveFault.boundaries...)
	if err := overwriteRegularFixture(githubManifestPath, githubManifest, 0o600); err != nil {
		return wrapperLifecycleEvidence{}, nil, 0, fmt.Errorf("managed manifest fixture restoration failed")
	}
	restoredStatusOutput, err := runner.success(ctx, "success", "wrapper", "status")
	if err != nil {
		return wrapperLifecycleEvidence{}, nil, 0, fmt.Errorf("restored wrapper status failed")
	}
	restoredStatus, err := decodeWrapperStatus(restoredStatusOutput.stdout)
	if err != nil || !sameStatusArtifacts(installedStatus, restoredStatus) {
		return wrapperLifecycleEvidence{}, nil, 0, fmt.Errorf("restored wrapper status changed artifact references")
	}
	boundaries = append(boundaries, restoredStatusOutput.stdout)

	githubRemoveOutput, err := runner.success(ctx, "success", "wrapper", "remove", "--artifact", githubStatus.Reference)
	if err != nil {
		return wrapperLifecycleEvidence{}, nil, 0, fmt.Errorf("exact GitHub wrapper removal failed")
	}
	githubRemove, err := decodeWrapperRemove(githubRemoveOutput.stdout)
	if err != nil || githubRemove.Command != "gh" || githubRemove.Path != filepath.Join(wantedBinPath, "gh") || !githubRemove.Removed ||
		githubRemove.SourceProcessAttempts != 0 || githubRemove.ProcessorProcessAttempts != 0 {
		return wrapperLifecycleEvidence{}, nil, 0, fmt.Errorf("exact GitHub wrapper removal evidence is invalid")
	}
	boundaries = append(boundaries, githubRemoveOutput.stdout)
	afterGitHubOutput, err := runner.success(ctx, "success", "wrapper", "status")
	if err != nil {
		return wrapperLifecycleEvidence{}, nil, 0, fmt.Errorf("post-GitHub-removal status failed")
	}
	afterGitHub, err := decodeWrapperStatus(afterGitHubOutput.stdout)
	if err != nil || !statusCommands(afterGitHub, "go") || afterGitHub[0].Reference != goStatus.Reference {
		return wrapperLifecycleEvidence{}, nil, 0, fmt.Errorf("GitHub removal did not preserve the exact Go artifact")
	}
	boundaries = append(boundaries, afterGitHubOutput.stdout)
	collisionFaults, collisionBoundaries, err := verifyCollisionFaults(
		ctx, runner, help, storeRoot, workRoot, afterGitHub, processCounts,
	)
	if err != nil {
		return wrapperLifecycleEvidence{}, nil, 0, err
	}
	faults = append(faults, collisionFaults...)
	boundaries = append(boundaries, collisionBoundaries...)
	goRemoveOutput, err := runner.success(ctx, "success", "wrapper", "remove", "--artifact", goStatus.Reference)
	if err != nil {
		return wrapperLifecycleEvidence{}, nil, 0, fmt.Errorf("exact Go wrapper removal failed")
	}
	goRemove, err := decodeWrapperRemove(goRemoveOutput.stdout)
	if err != nil || goRemove.Command != "go" || goRemove.Path != filepath.Join(wantedBinPath, "go") || !goRemove.Removed ||
		goRemove.SourceProcessAttempts != 0 || goRemove.ProcessorProcessAttempts != 0 {
		return wrapperLifecycleEvidence{}, nil, 0, fmt.Errorf("exact Go wrapper removal evidence is invalid")
	}
	boundaries = append(boundaries, goRemoveOutput.stdout)
	finalStatusOutput, err := runner.success(ctx, "success", "wrapper", "status")
	if err != nil {
		return wrapperLifecycleEvidence{}, nil, 0, fmt.Errorf("final wrapper status failed")
	}
	finalStatus, err := decodeWrapperStatus(finalStatusOutput.stdout)
	if err != nil || len(finalStatus) != 0 {
		return wrapperLifecycleEvidence{}, nil, 0, fmt.Errorf("final wrapper status is not empty")
	}
	boundaries = append(boundaries, finalStatusOutput.stdout)
	if err := processCounts(); err != nil {
		return wrapperLifecycleEvidence{}, nil, 0, fmt.Errorf("wrapper removal started a source process")
	}

	githubArtifact := lifecycleArtifactEvidence("gh_pr_list", "gh", githubStatus, githubCase, githubHelp, githubInvocation)
	githubArtifact.RemoveReference = githubStatus.Reference
	githubArtifact.RemovalOutcome = "removed"
	goArtifact := lifecycleArtifactEvidence("go_test", "go", goStatus, goCase, goHelp, goInvocation)
	goArtifact.RemoveReference = goStatus.Reference
	goArtifact.RemovalOutcome = "removed"
	return wrapperLifecycleEvidence{
		Outcome: "installed_artifacts_verified", ContractVersion: wrappershim.ContractVersion,
		BinPathSHA256: digestBytes([]byte(wantedBinPath)), PathPrecedence: "reported_bin_first",
		PathCommands: []string{"gh", "go"},
		StatusSnapshots: []wrapperLifecycleStatusEvidence{
			{Name: "initial", References: []string{}},
			{Name: "installed", References: statusReferences(installedStatus)},
			{Name: "after_gh_remove", References: statusReferences(afterGitHub)},
			{Name: "final", References: []string{}},
		},
		Artifacts: []installedWrapperArtifactEvidence{githubArtifact, goArtifact}, Faults: faults,
		StoreMutationAttempts: 4, SourceProcessAttempts: 2, ProcessorProcessAttempts: 0,
		ManagementSourceProcessAttempts: 0, ManagementProcessorProcessAttempts: 0, ZeroAttemptRejections: len(faults),
	}, boundaries, 1, nil
}

type lifecycleFaultResult struct {
	evidence   wrapperLifecycleFaultEvidence
	boundaries [][]byte
}

func runUnchangedLifecycleFault(
	ctx context.Context,
	runner journeyRunner,
	help packagedHelpEvidence,
	storeRoot, name, helpPath, code string,
	exit int,
	arguments []string,
	processCounts func() error,
) (lifecycleFaultResult, error) {
	before, err := snapshotManagedFilesystem(storeRoot)
	if err != nil {
		return lifecycleFaultResult{}, fmt.Errorf("%s filesystem baseline failed", name)
	}
	declaration, err := help.fault(helpPath, code)
	if err != nil || declaration.Retryable {
		return lifecycleFaultResult{}, fmt.Errorf("%s help contract is invalid", name)
	}
	failure, err := runner.failure(ctx, "success", exit, declaration, arguments...)
	if err != nil {
		return lifecycleFaultResult{}, fmt.Errorf("%s public failure is invalid", name)
	}
	after, err := snapshotManagedFilesystem(storeRoot)
	if err != nil || before.digest != after.digest {
		return lifecycleFaultResult{}, fmt.Errorf("%s changed managed filesystem state", name)
	}
	if err := processCounts(); err != nil {
		return lifecycleFaultResult{}, fmt.Errorf("%s started a source process", name)
	}
	return lifecycleFaultResult{
		evidence: wrapperLifecycleFaultEvidence{
			Name: name, Code: code, FilesystemStateUnchanged: true,
			SourceProcessAttempts: 0, ProcessorProcessAttempts: 0,
		},
		boundaries: [][]byte{failure.stdout, failure.stderr},
	}, nil
}

func verifyCollisionFaults(
	ctx context.Context,
	runner journeyRunner,
	help packagedHelpEvidence,
	storeRoot, workRoot string,
	expectedStatus []wrapperStatusArtifact,
	processCounts func() error,
) ([]wrapperLifecycleFaultEvidence, [][]byte, error) {
	if len(expectedStatus) == 0 {
		return nil, nil, fmt.Errorf("collision evidence requires an owned artifact baseline")
	}
	binPath := filepath.Join(storeRoot, "bin")
	commandPath := filepath.Join(binPath, "gh")
	faults := make([]wrapperLifecycleFaultEvidence, 0, 3)
	boundaries := make([][]byte, 0, 8)
	run := func(name string, create func() error, verify func() error) error {
		if err := create(); err != nil {
			return fmt.Errorf("%s fixture creation failed", name)
		}
		result, err := runUnchangedLifecycleFault(
			ctx, runner, help, storeRoot, name, "wrapper status", "wrapper_artifact_collision", 10,
			[]string{"--error-format=json", "wrapper", "status"}, processCounts,
		)
		if err != nil {
			return err
		}
		for _, artifact := range expectedStatus {
			for _, boundary := range result.boundaries {
				if bytes.Contains(boundary, []byte(artifact.Reference)) {
					return fmt.Errorf("%s exposed an owned artifact reference", name)
				}
			}
		}
		if verify != nil {
			if err := verify(); err != nil {
				return fmt.Errorf("%s external fixture changed", name)
			}
		}
		if err := os.Remove(commandPath); err != nil {
			return fmt.Errorf("%s fixture cleanup failed", name)
		}
		restoredOutput, err := runner.success(ctx, "success", "wrapper", "status")
		if err != nil {
			return fmt.Errorf("%s cleanup status failed", name)
		}
		restored, err := decodeWrapperStatus(restoredOutput.stdout)
		if err != nil || !sameStatusArtifacts(expectedStatus, restored) {
			return fmt.Errorf("%s cleanup did not restore the owned artifact baseline", name)
		}
		faults = append(faults, result.evidence)
		boundaries = append(boundaries, result.boundaries...)
		boundaries = append(boundaries, restoredOutput.stdout)
		return nil
	}
	if err := run("foreign_collision_status", func() error {
		if err := writePrivate(commandPath, []byte("foreign fixture\n")); err != nil {
			return err
		}
		// #nosec G302 -- the fixture must look like a foreign executable shim;
		// it remains owner-only inside the isolated journey store.
		return os.Chmod(commandPath, 0o700)
	}, nil); err != nil {
		return nil, nil, err
	}
	sentinelPath := filepath.Join(workRoot, "wrapper-collision-sentinel")
	if err := writePrivate(sentinelPath, []byte("sentinel\n")); err != nil {
		return nil, nil, fmt.Errorf("symlink collision sentinel creation failed")
	}
	sentinel, err := snapshotRegularFile(sentinelPath)
	if err != nil {
		return nil, nil, fmt.Errorf("symlink collision sentinel identity is invalid")
	}
	if err := run("symlink_collision_status", func() error {
		return os.Symlink(sentinelPath, commandPath)
	}, func() error { return sentinel.verify(sentinelPath) }); err != nil {
		return nil, nil, err
	}
	if err := run("special_collision_status", func() error {
		return createSpecialLifecycleFile(commandPath)
	}, nil); err != nil {
		return nil, nil, err
	}
	return faults, append(boundaries, []byte(sentinel.sha256)), nil
}

func verifyUnsupportedWrapperLifecycle(
	ctx context.Context,
	runner journeyRunner,
	help packagedHelpEvidence,
	storeRoot, bundlePath, attemptLog string,
	existingFixtureAttempts int,
	workRoot string,
) (wrapperLifecycleEvidence, [][]byte, int, error) {
	goAttemptLog := filepath.Join(workRoot, "go-test-attempts.log")
	if err := requireAttempts(attemptLog, existingFixtureAttempts); err != nil {
		return wrapperLifecycleEvidence{}, nil, 0, fmt.Errorf("Windows wrapper lifecycle source baseline is invalid")
	}
	if err := requireGoTestAttempts(goAttemptLog, 0); err != nil {
		return wrapperLifecycleEvidence{}, nil, 0, fmt.Errorf("Windows wrapper lifecycle Go baseline is invalid")
	}
	tests := []struct {
		name, helpPath string
		arguments      []string
	}{
		{name: "install", helpPath: "wrapper install", arguments: []string{"--error-format=json", "wrapper", "install", "--bundle", bundlePath}},
		{name: "status", helpPath: "wrapper status", arguments: []string{"--error-format=json", "wrapper", "status"}},
		{name: "remove", helpPath: "wrapper remove", arguments: []string{"--error-format=json", "wrapper", "remove", "--artifact", "wsh1_" + strings.Repeat("0", wrappershim.DigestBytes)}},
	}
	faults := make([]wrapperLifecycleFaultEvidence, 0, len(tests))
	boundaries := make([][]byte, 0, len(tests)*2)
	for _, test := range tests {
		before, err := snapshotManagedFilesystem(storeRoot)
		if err != nil || before.digest != digestBytes([]byte("absent")) {
			return wrapperLifecycleEvidence{}, nil, 0, fmt.Errorf("Windows %s store baseline is invalid", test.name)
		}
		declaration, err := help.fault(test.helpPath, "wrapper_artifact_platform_not_supported")
		if err != nil || declaration.Kind != "unsupported" || declaration.Retryable {
			return wrapperLifecycleEvidence{}, nil, 0, fmt.Errorf("Windows %s help contract is invalid", test.name)
		}
		failure, err := runner.failure(ctx, "success", 12, declaration, test.arguments...)
		if err != nil {
			return wrapperLifecycleEvidence{}, nil, 0, fmt.Errorf("Windows %s result is invalid", test.name)
		}
		after, err := snapshotManagedFilesystem(storeRoot)
		if err != nil || before.digest != after.digest {
			return wrapperLifecycleEvidence{}, nil, 0, fmt.Errorf("Windows %s changed managed filesystem state", test.name)
		}
		if err := requireAttempts(attemptLog, existingFixtureAttempts); err != nil {
			return wrapperLifecycleEvidence{}, nil, 0, fmt.Errorf("Windows %s started the GitHub source", test.name)
		}
		if err := requireGoTestAttempts(goAttemptLog, 0); err != nil {
			return wrapperLifecycleEvidence{}, nil, 0, fmt.Errorf("Windows %s started the Go source", test.name)
		}
		faults = append(faults, wrapperLifecycleFaultEvidence{
			Name: test.name, Code: "wrapper_artifact_platform_not_supported", FilesystemStateUnchanged: true,
			SourceProcessAttempts: 0, ProcessorProcessAttempts: 0,
		})
		boundaries = append(boundaries, failure.stdout, failure.stderr)
	}
	return wrapperLifecycleEvidence{
		Outcome: "platform_not_supported", ContractVersion: 0, BinPathSHA256: "", PathPrecedence: "not_applicable",
		PathCommands: []string{}, StatusSnapshots: []wrapperLifecycleStatusEvidence{}, Artifacts: []installedWrapperArtifactEvidence{}, Faults: faults,
		StoreMutationAttempts: 0, SourceProcessAttempts: 0, ProcessorProcessAttempts: 0,
		ManagementSourceProcessAttempts: 0, ManagementProcessorProcessAttempts: 0, ZeroAttemptRejections: len(faults),
	}, boundaries, 0, nil
}

func lifecycleArtifactEvidence(
	name, commandName string,
	status wrapperStatusArtifact,
	outer wrapperCaseEvidence,
	help, invocation commandOutcome,
) installedWrapperArtifactEvidence {
	return installedWrapperArtifactEvidence{
		Name: name, CommandName: commandName, Reference: status.Reference, MaterialSHA256: status.MaterialSHA256,
		StatusState: status.State, BundleDigest: outer.BundleDigest, PlanDigest: outer.PlanDigest, ExecutionCase: outer.Name,
		CallerArgv: append([]string{}, outer.CallerArgv...), SourceArgv: append([]string{}, outer.SourceArgv...),
		WrapperKind: outer.WrapperKind, ResultMode: outer.ResultMode,
		HelpStdoutSHA256: digestBytes(help.stdout), HelpStderrSHA256: digestBytes(help.stderr),
		HelpSourceAttempts: 0, HelpProcessorAttempts: 0,
		StdoutSHA256: digestBytes(invocation.stdout), StderrSHA256: digestBytes(invocation.stderr), SourceExitCode: invocation.exitCode,
		SourceProcessAttempts: 1, ProcessorProcessAttempts: 0,
	}
}

func inspectManagedArtifact(
	storeRoot, executablePath, bundlePath, bundleDigest string,
	status wrapperStatusArtifact,
) ([]byte, []byte, error) {
	reference, err := wrappershim.ParseReference(status.Reference)
	if err != nil || status.MaterialSHA256 == "" || status.State != "owned_active" || status.Command == "" {
		return nil, nil, fmt.Errorf("status artifact is invalid")
	}
	materialDigest, err := reference.Digest()
	if err != nil || materialDigest != status.MaterialSHA256 {
		return nil, nil, fmt.Errorf("status reference does not bind material")
	}
	binPath := filepath.Join(storeRoot, "bin", status.Command)
	if status.Path != binPath {
		return nil, nil, fmt.Errorf("status path is invalid")
	}
	recordRoot := filepath.Join(storeRoot, "records", status.Reference)
	manifestBytes, err := readBoundedFile(filepath.Join(recordRoot, "manifest.json"), wrappershim.MaxManifestBytes)
	if err != nil {
		return nil, nil, fmt.Errorf("manifest is unreadable")
	}
	manifest, err := wrappershim.DecodeManifest(manifestBytes)
	if err != nil || manifest.Reference != reference || manifest.MaterialSHA256 != status.MaterialSHA256 ||
		manifest.Binding.CommandName != status.Command || manifest.Binding.BundleLocator != bundlePath || manifest.Binding.BundleDigest != bundleDigest {
		return nil, nil, fmt.Errorf("manifest binding is invalid")
	}
	runtimeDigest, runtimeSize, err := regularFileIdentity(executablePath)
	if err != nil || manifest.Binding.Runtime.ResolvedPath != executablePath || manifest.Binding.Runtime.SHA256 != runtimeDigest || manifest.Binding.Runtime.Size != runtimeSize {
		return nil, nil, fmt.Errorf("manifest runtime binding is invalid")
	}
	shimPath := filepath.Join(recordRoot, "shim")
	shim, err := readBoundedFile(shimPath, wrappershim.MaxShimBytes)
	if err != nil || digestBytes(shim) != status.MaterialSHA256 || int64(len(shim)) != manifest.MaterialSize {
		return nil, nil, fmt.Errorf("managed shim material is invalid")
	}
	rendered, err := posixshim.Render(manifest.Binding.Clone())
	if err != nil || rendered.SHA256 != status.MaterialSHA256 || !bytes.Equal(rendered.Source, shim) {
		return nil, nil, fmt.Errorf("managed shim is not deterministic fixed material")
	}
	shimInfo, err := os.Lstat(shimPath)
	if err != nil || !shimInfo.Mode().IsRegular() || shimInfo.Mode().Perm() != 0o700 {
		return nil, nil, fmt.Errorf("managed record shim mode is invalid")
	}
	activeInfo, err := os.Lstat(binPath)
	if err != nil || !activeInfo.Mode().IsRegular() || activeInfo.Mode().Perm() != 0o700 || !os.SameFile(shimInfo, activeInfo) {
		return nil, nil, fmt.Errorf("managed active shim identity is invalid")
	}
	return manifestBytes, shim, nil
}

func decodeWrapperInstall(value []byte) (wrapperInstallResult, error) {
	var document struct {
		SchemaVersion int `json:"schema_version"`
		Installation  struct {
			Command                  string `json:"command"`
			Path                     string `json:"path"`
			BinPath                  string `json:"bin_path"`
			AlreadyInstalled         *bool  `json:"already_installed"`
			SourceProcessAttempts    *int   `json:"source_process_attempts"`
			ProcessorProcessAttempts *int   `json:"processor_process_attempts"`
		} `json:"installation"`
	}
	if err := decodeLifecycleJSON(value, &document); err != nil || document.SchemaVersion != 1 ||
		document.Installation.Command == "" || !absoluteCleanPath(document.Installation.Path) || !absoluteCleanPath(document.Installation.BinPath) ||
		document.Installation.AlreadyInstalled == nil || document.Installation.SourceProcessAttempts == nil || document.Installation.ProcessorProcessAttempts == nil {
		return wrapperInstallResult{}, fmt.Errorf("wrapper install JSON is invalid")
	}
	return wrapperInstallResult{
		Command: document.Installation.Command, Path: document.Installation.Path, BinPath: document.Installation.BinPath,
		AlreadyInstalled:         *document.Installation.AlreadyInstalled,
		SourceProcessAttempts:    *document.Installation.SourceProcessAttempts,
		ProcessorProcessAttempts: *document.Installation.ProcessorProcessAttempts,
	}, nil
}

func decodeWrapperStatus(value []byte) ([]wrapperStatusArtifact, error) {
	var document struct {
		SchemaVersion int                     `json:"schema_version"`
		Artifacts     []wrapperStatusArtifact `json:"artifacts"`
	}
	if err := decodeLifecycleJSON(value, &document); err != nil || document.SchemaVersion != 1 || document.Artifacts == nil || len(document.Artifacts) > wrappershim.MaxArtifacts {
		return nil, fmt.Errorf("wrapper status JSON is invalid")
	}
	previous := ""
	for _, artifact := range document.Artifacts {
		reference, err := wrappershim.ParseReference(artifact.Reference)
		if err != nil || artifact.Command == "" || (artifact.State != "owned_active" && artifact.State != "owned_inactive") || !absoluteCleanPath(artifact.Path) || artifact.MaterialSHA256 == "" {
			return nil, fmt.Errorf("wrapper status artifact is invalid")
		}
		digest, err := reference.Digest()
		if err != nil || digest != artifact.MaterialSHA256 {
			return nil, fmt.Errorf("wrapper status artifact binding is invalid")
		}
		key := artifact.Command + "\x00" + artifact.State + "\x00" + artifact.Reference
		if previous != "" && key <= previous {
			return nil, fmt.Errorf("wrapper status artifacts are not canonical")
		}
		previous = key
	}
	return append([]wrapperStatusArtifact{}, document.Artifacts...), nil
}

func decodeWrapperRemove(value []byte) (wrapperRemoveResult, error) {
	var document struct {
		SchemaVersion int `json:"schema_version"`
		Removal       struct {
			Command                  string `json:"command"`
			Path                     string `json:"path"`
			Removed                  *bool  `json:"removed"`
			SourceProcessAttempts    *int   `json:"source_process_attempts"`
			ProcessorProcessAttempts *int   `json:"processor_process_attempts"`
		} `json:"removal"`
	}
	if err := decodeLifecycleJSON(value, &document); err != nil || document.SchemaVersion != 1 || document.Removal.Command == "" ||
		!absoluteCleanPath(document.Removal.Path) || document.Removal.Removed == nil ||
		document.Removal.SourceProcessAttempts == nil || document.Removal.ProcessorProcessAttempts == nil {
		return wrapperRemoveResult{}, fmt.Errorf("wrapper remove JSON is invalid")
	}
	return wrapperRemoveResult{
		Command: document.Removal.Command, Path: document.Removal.Path, Removed: *document.Removal.Removed,
		SourceProcessAttempts:    *document.Removal.SourceProcessAttempts,
		ProcessorProcessAttempts: *document.Removal.ProcessorProcessAttempts,
	}, nil
}

func decodeLifecycleJSON(value []byte, output any) error {
	if len(value) == 0 || len(value) > maxCommandOutputBytes {
		return fmt.Errorf("lifecycle JSON size is invalid")
	}
	decoder := json.NewDecoder(bytes.NewReader(value))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(output); err != nil {
		return err
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return fmt.Errorf("lifecycle JSON contains trailing data")
	}
	return nil
}

func runInstalledPATHCommand(ctx context.Context, runner journeyRunner, binPath, commandName string, arguments []string, wantedExit int) (commandOutcome, error) {
	if ctx == nil || !absoluteCleanPath(binPath) || (commandName != "gh" && commandName != "go") || arguments == nil || wantedExit < 0 {
		return commandOutcome{}, fmt.Errorf("installed PATH caller input is invalid")
	}
	envPath := "/usr/bin/env"
	if snapshot, err := snapshotRegularFile(envPath); err != nil || snapshot.size <= 0 {
		return commandOutcome{}, fmt.Errorf("fixed PATH caller is unavailable")
	}
	injectionPath := filepath.Join(runner.directory, "atsura-artifact-injection")
	if _, err := os.Lstat(injectionPath); !errors.Is(err, os.ErrNotExist) {
		return commandOutcome{}, fmt.Errorf("installed PATH caller injection canary is not absent")
	}
	runContext, cancel := context.WithTimeout(ctx, commandTimeout)
	defer cancel()
	argv := append([]string{commandName}, arguments...)
	// /usr/bin/env is the fixed no-shell caller fixture. Its child PATH begins
	// with only the reported managed bin directory, so ordinary spelling cannot
	// resolve an ambient source executable.
	// #nosec G204 -- the executable and command name are fixed; arguments are the finite journey grammar.
	command := exec.CommandContext(runContext, envPath, argv...)
	command.Dir = runner.directory
	command.Env = replaceEnvironment(runner.environment, map[string]string{
		"PATH": binPath, fixtureModeEnv: "success", goTestModeEnv: "pass", "GOFLAGS": "-buildvcs=false -count=1",
	})
	command.Stdin = nil
	command.WaitDelay = 2 * time.Second
	stdout := &boundedBuffer{limit: maxCommandOutputBytes}
	stderr := &boundedBuffer{limit: maxCommandOutputBytes}
	command.Stdout = stdout
	command.Stderr = stderr
	err := command.Run()
	outcome := commandOutcome{stdout: stdout.Bytes(), stderr: stderr.Bytes(), exitCode: -1}
	if stdout.exceeded || stderr.exceeded || runContext.Err() != nil {
		return commandOutcome{}, fmt.Errorf("installed PATH caller exceeded its bound")
	}
	if command.ProcessState != nil {
		outcome.exitCode = command.ProcessState.ExitCode()
	}
	if outcome.exitCode != wantedExit {
		return commandOutcome{}, fmt.Errorf("installed PATH caller returned an unexpected status")
	}
	if wantedExit == 0 && err != nil {
		return commandOutcome{}, fmt.Errorf("installed PATH caller did not succeed")
	}
	if wantedExit != 0 {
		var exitError *exec.ExitError
		if !errors.As(err, &exitError) {
			return commandOutcome{}, fmt.Errorf("installed PATH caller did not return a conventional failure")
		}
	}
	if _, err := os.Lstat(injectionPath); !errors.Is(err, os.ErrNotExist) {
		return commandOutcome{}, fmt.Errorf("installed PATH caller evaluated hostile argv")
	}
	return outcome, nil
}

func expectedManagedStoreRoot(environment []string, goos string) (string, error) {
	value := func(name string) string {
		for _, item := range environment {
			key, current, present := strings.Cut(item, "=")
			if present && strings.EqualFold(key, name) {
				return current
			}
		}
		return ""
	}
	var config string
	switch goos {
	case "linux":
		config = value("XDG_CONFIG_HOME")
	case "darwin":
		config = filepath.Join(value("HOME"), "Library", "Application Support")
	case "windows":
		config = value("APPDATA")
	default:
		return "", fmt.Errorf("unsupported managed store platform")
	}
	if !absoluteCleanPath(config) {
		return "", fmt.Errorf("managed store configuration root is invalid")
	}
	return filepath.Join(config, "atsura", "wrapper-shims", "v1"), nil
}

func snapshotManagedFilesystem(root string) (managedFilesystemSnapshot, error) {
	if !absoluteCleanPath(root) {
		return managedFilesystemSnapshot{}, fmt.Errorf("managed filesystem root is invalid")
	}
	if _, err := os.Lstat(root); errors.Is(err, os.ErrNotExist) {
		return managedFilesystemSnapshot{digest: digestBytes([]byte("absent"))}, nil
	} else if err != nil {
		return managedFilesystemSnapshot{}, err
	}
	entries := make([]string, 0, 32)
	var readBytes int64
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if len(entries) >= wrappershim.MaxArtifacts*8+16 {
			return fmt.Errorf("managed filesystem snapshot entry bound exceeded")
		}
		relative, err := filepath.Rel(root, path)
		if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
			return fmt.Errorf("managed filesystem snapshot escaped its root")
		}
		info, err := os.Lstat(path)
		if err != nil {
			return err
		}
		line := filepath.ToSlash(relative) + "\x00" + info.Mode().String()
		switch {
		case info.Mode()&os.ModeSymlink != 0:
			target, err := os.Readlink(path)
			if err != nil {
				return err
			}
			line += "\x00" + target
		case info.Mode().IsRegular():
			value, err := readBoundedFile(path, maxLifecycleSnapshotBytes-readBytes)
			if err != nil {
				return err
			}
			readBytes += int64(len(value))
			if readBytes > maxLifecycleSnapshotBytes {
				return fmt.Errorf("managed filesystem snapshot byte bound exceeded")
			}
			line += "\x00" + digestBytes(value)
		}
		entries = append(entries, line)
		return nil
	})
	if err != nil {
		return managedFilesystemSnapshot{}, err
	}
	sort.Strings(entries)
	return managedFilesystemSnapshot{digest: digestBytes([]byte(strings.Join(entries, "\n")))}, nil
}

func exactWrapperCase(cases []wrapperCaseEvidence, name string) (wrapperCaseEvidence, error) {
	var result wrapperCaseEvidence
	found := false
	for _, current := range cases {
		if current.Name != name {
			continue
		}
		if found {
			return wrapperCaseEvidence{}, fmt.Errorf("wrapper case %s is duplicated", name)
		}
		result = current
		found = true
	}
	if !found {
		return wrapperCaseEvidence{}, fmt.Errorf("wrapper case %s is missing", name)
	}
	return result, nil
}

func expectedGoTailoredHelp(bundleDigest string) []byte {
	return []byte("Atsura tailored help\nBundle digest: " + bundleDigest + "\nCommands:\n  test\n")
}

func statusCommands(artifacts []wrapperStatusArtifact, commands ...string) bool {
	if len(artifacts) != len(commands) {
		return false
	}
	for index := range commands {
		if artifacts[index].Command != commands[index] {
			return false
		}
	}
	return true
}

func statusReferences(artifacts []wrapperStatusArtifact) []string {
	result := make([]string, len(artifacts))
	for index := range artifacts {
		result[index] = artifacts[index].Reference
	}
	return result
}

func sameStatusArtifacts(left, right []wrapperStatusArtifact) bool {
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

func absoluteCleanPath(value string) bool {
	return filepath.IsAbs(value) && filepath.Clean(value) == value
}

func pathWithin(root, path string) bool {
	if !absoluteCleanPath(root) || !absoluteCleanPath(path) {
		return false
	}
	relative, err := filepath.Rel(root, path)
	return err == nil && relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator))
}
