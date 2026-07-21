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
	ArchiveSHA256               string   `json:"archive_sha256"`
	Version                     string   `json:"version"`
	HelpContractsVerified       int      `json:"help_contracts_verified"`
	BundleDigest                string   `json:"bundle_digest"`
	PlanDigest                  string   `json:"plan_digest"`
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
	helpOutputs, err := verifyPackagedHelp(ctx, runner)
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

	draft, err := runner.success(ctx, "success", "spec", "init", "--catalog", catalogPath, "--", "pr", "list")
	if err != nil {
		return evidenceDocument{}, fmt.Errorf("specification draft failed")
	}
	transformedSpecification, err := transformDraft(draft.stdout)
	if err != nil {
		return evidenceDocument{}, fmt.Errorf("specification transform edit failed")
	}
	specificationPath := filepath.Join(workRoot, "specification.yaml")
	if err := writePrivate(specificationPath, transformedSpecification); err != nil {
		return evidenceDocument{}, fmt.Errorf("specification evidence write failed")
	}
	validation, err := runner.success(ctx, "success", "spec", "validate", "--catalog", catalogPath, "--spec", specificationPath)
	if err != nil {
		return evidenceDocument{}, fmt.Errorf("specification validation failed")
	}
	if err := validateSpecificationEvidence(validation.stdout, inspectionPayload.CatalogDigest); err != nil {
		return evidenceDocument{}, fmt.Errorf("specification validation evidence is invalid")
	}
	built, err := runner.success(ctx, "success", "bundle", "build", "--catalog", catalogPath, "--spec", specificationPath)
	if err != nil {
		return evidenceDocument{}, fmt.Errorf("bundle build failed")
	}
	bundleDigest, err := decodeBundleDigest(built.stdout)
	if err != nil {
		return evidenceDocument{}, fmt.Errorf("bundle build evidence is invalid")
	}
	bundlePath := filepath.Join(workRoot, "bundle.json")
	if err := writePrivate(bundlePath, built.stdout); err != nil {
		return evidenceDocument{}, fmt.Errorf("bundle evidence write failed")
	}

	status, err := runner.success(ctx, "success", "bundle", "status", "--bundle", bundlePath)
	if err != nil || validateStatus(status.stdout, bundleDigest, bundletrust.StateNotAdopted) != nil {
		return evidenceDocument{}, fmt.Errorf("pre-adoption status evidence is invalid")
	}
	baseInvocation := []string{"--bundle", bundlePath, "--", sourcePath, "pr", "list", "--limit=1"}
	zeroAttemptRejections := 0
	for _, command := range []string{"preview", "execute"} {
		arguments := append([]string{"--error-format=json", "bundle", command}, baseInvocation...)
		failure, err := runner.failure(ctx, "success", 10, "bundle_not_adopted", false, arguments...)
		if err != nil {
			return evidenceDocument{}, fmt.Errorf("pre-adoption rejection evidence is invalid")
		}
		if err := scanCanaries(failure.stdout, failure.stderr); err != nil {
			return evidenceDocument{}, err
		}
		zeroAttemptRejections++
	}
	if err := requireAttempts(attemptLog, 4); err != nil {
		return evidenceDocument{}, fmt.Errorf("pre-adoption zero-attempt evidence is invalid")
	}

	store := trustfile.New(trustPath)
	changed, err := store.Add(ctx, bundleDigest)
	if err != nil || !changed || store.Inspect(ctx, bundleDigest) != bundletrust.StateAdopted {
		return evidenceDocument{}, fmt.Errorf("isolated exact receipt seeding failed")
	}
	status, err = runner.success(ctx, "success", "bundle", "status", "--bundle", bundlePath)
	if err != nil || validateStatus(status.stdout, bundleDigest, bundletrust.StateAdopted) != nil {
		return evidenceDocument{}, fmt.Errorf("adopted status evidence is invalid")
	}

	preview, err := runner.success(ctx, "success", append([]string{"bundle", "preview"}, baseInvocation...)...)
	if err != nil {
		return evidenceDocument{}, fmt.Errorf("adopted preview failed")
	}
	previewEvidence, err := decodePreview(preview.stdout)
	if err != nil || previewEvidence.SourceProcessAttempts != 0 || !digestValue(previewEvidence.PlanDigest) {
		return evidenceDocument{}, fmt.Errorf("adopted preview evidence is invalid")
	}
	if err := requireAttempts(attemptLog, 4); err != nil {
		return evidenceDocument{}, fmt.Errorf("preview zero-attempt evidence is invalid")
	}

	for _, conflict := range []string{"--web", "--jq=.[]", "--template={{.number}}"} {
		arguments := []string{"--error-format=json", "bundle", "execute", "--bundle", bundlePath, "--", sourcePath, "pr", "list", "--limit=1", conflict}
		failure, err := runner.failure(ctx, "success", 12, "wrapper_runtime_not_supported", false, arguments...)
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
	}{
		{mode: "command_failure", exit: 10, code: "source_command_failed"},
		{mode: "stderr", exit: 13, code: "source_stderr_not_supported"},
		{mode: "malformed", exit: 13, code: "source_json_invalid"},
		{mode: "missing_field", exit: 13, code: "output_transform_failed"},
	}
	faultCodes := make([]string, 0, len(postStartFaults))
	wantedAttempts := 4
	for _, test := range postStartFaults {
		arguments := append([]string{"--error-format=json", "bundle", "execute"}, baseInvocation...)
		failure, err := runner.failure(ctx, test.mode, test.exit, test.code, false, arguments...)
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

	execution, err := runner.success(ctx, "success", append([]string{"bundle", "execute"}, baseInvocation...)...)
	if err != nil {
		return evidenceDocument{}, fmt.Errorf("successful transform execution failed")
	}
	executionEvidence, err := decodeExecution(execution.stdout)
	if err != nil || executionEvidence.PlanDigest != previewEvidence.PlanDigest || executionEvidence.BundleDigest != bundleDigest || executionEvidence.SourceProcessAttempts != 1 {
		return evidenceDocument{}, fmt.Errorf("successful transform execution evidence is invalid")
	}
	if err := validateSelectedOutput(executionEvidence); err != nil {
		return evidenceDocument{}, fmt.Errorf("selected transform output evidence is invalid")
	}
	wantedAttempts++
	if err := requireAttempts(attemptLog, wantedAttempts); err != nil {
		return evidenceDocument{}, fmt.Errorf("successful execution one-attempt evidence is invalid")
	}

	trustBytes, err := readBoundedFile(trustPath, maxAttemptLogBytes)
	if err != nil {
		return evidenceDocument{}, fmt.Errorf("isolated receipt evidence is unreadable")
	}
	attemptBytes, err := readBoundedFile(attemptLog, maxAttemptLogBytes)
	if err != nil {
		return evidenceDocument{}, fmt.Errorf("fixture attempt evidence is unreadable")
	}
	canaryBoundaries := [][]byte{inspection.stdout, draft.stdout, transformedSpecification, validation.stdout, built.stdout, status.stdout, preview.stdout, execution.stdout, trustBytes, attemptBytes}
	canaryBoundaries = append(canaryBoundaries, helpOutputs...)
	if err := scanCanaries(canaryBoundaries...); err != nil {
		return evidenceDocument{}, err
	}
	if err := validateAttemptSequence(attemptBytes); err != nil {
		return evidenceDocument{}, fmt.Errorf("fixture attempt sequence is invalid")
	}

	return evidenceDocument{SchemaVersion: 1, ArtifactJourney: artifactJourneyEvidence{
		Target: configuration.goos + "/" + configuration.goarch, ArchiveSHA256: digest,
		Version: strings.TrimPrefix(configuration.tag, "v"), HelpContractsVerified: len(helpOutputs), BundleDigest: bundleDigest, PlanDigest: previewEvidence.PlanDigest,
		SourceInspectionAttempts: 4, ZeroAttemptRejections: zeroAttemptRejections,
		PostStartFaults: faultCodes, FixtureAttempts: wantedAttempts,
		CredentialEnvironmentAbsent: true, SecretCanariesAbsent: true,
	}}, nil
}

type helpSchemaProjection struct {
	ID      string `json:"id"`
	Version int    `json:"version"`
}

type helpCommandProjection struct {
	Path     string `json:"path"`
	Summary  string `json:"summary"`
	Usage    string `json:"usage"`
	Contract struct {
		Outcome string `json:"outcome"`
		Inputs  []struct {
			Name          string   `json:"name"`
			AllowedValues []string `json:"allowed_values"`
		} `json:"inputs"`
		Output struct {
			Fields []struct {
				Name   string                `json:"name"`
				Schema *helpSchemaProjection `json:"schema"`
			} `json:"fields"`
		} `json:"output"`
		Prerequisites []string `json:"prerequisites"`
	} `json:"contract"`
}

func verifyPackagedHelp(ctx context.Context, runner journeyRunner) ([][]byte, error) {
	root, err := runner.success(ctx, "success", "help", "--format", "agent")
	if err != nil {
		return nil, fmt.Errorf("packaged root help verification failed")
	}
	wanted := []string{"source inspect", "spec init", "spec validate", "bundle execute"}
	if err := validateRootHelp(root.stdout, wanted); err != nil {
		return nil, fmt.Errorf("packaged root help contract is invalid")
	}
	outputs := [][]byte{root.stdout}
	for _, path := range wanted {
		arguments := append([]string{"help"}, strings.Split(path, " ")...)
		arguments = append(arguments, "--format", "agent")
		result, runErr := runner.success(ctx, "success", arguments...)
		if runErr != nil || validateScopedHelp(path, result.stdout) != nil {
			return nil, fmt.Errorf("packaged scoped help contract is invalid for %s", path)
		}
		outputs = append(outputs, result.stdout)
	}
	return outputs, nil
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
	if err := json.Unmarshal(value, &document); err != nil || document.SchemaVersion != 8 || document.View != "index" || document.Program != "atr" {
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

func validateScopedHelp(path string, value []byte) error {
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
	if err := json.Unmarshal(value, &document); err != nil || document.SchemaVersion != 8 || document.View != "scope" || document.Program != "atr" || document.Scope.Selector != path || document.Scope.Kind != "command" || len(document.Commands) != 1 || document.Commands[0].Path != path {
		return fmt.Errorf("invalid scoped agent help")
	}
	command := document.Commands[0]
	schema := func(name string) *helpSchemaProjection {
		for _, field := range command.Contract.Output.Fields {
			if field.Name == name {
				return field.Schema
			}
		}
		return nil
	}
	prerequisites := strings.Join(command.Contract.Prerequisites, "\n")
	switch path {
	case "source inspect":
		if command.Usage != "atr source inspect --adapter=github-cli --executable <path-or-name>" || len(command.Contract.Inputs) < 1 || command.Contract.Inputs[0].Name != "--adapter" || strings.Join(command.Contract.Inputs[0].AllowedValues, ",") != "github-cli" {
			return fmt.Errorf("source inspection invocation contract is incomplete")
		}
		if nested := schema("catalog"); nested == nil || nested.ID != "source-command-catalog" || nested.Version != 1 {
			return fmt.Errorf("source catalog schema is incomplete")
		}
	case "spec init":
		if !strings.Contains(command.Summary, "authoring baseline") || !strings.Contains(command.Contract.Outcome, "identity wrapper") {
			return fmt.Errorf("specification baseline contract is incomplete")
		}
		if nested := schema("specification"); nested == nil || nested.ID != "tailoring-specification" || nested.Version != 3 {
			return fmt.Errorf("specification schema is incomplete")
		}
		for _, marker := range []string{"kind=transform", "output.select", "output.rename", "output.render=compact_json", "Shell, script, jq, plugin, RTK"} {
			if !strings.Contains(prerequisites, marker) {
				return fmt.Errorf("specification authoring marker is missing")
			}
		}
	case "spec validate":
		if nested := schema("specification"); nested == nil || nested.ID != "tailoring-specification" || nested.Version != 3 {
			return fmt.Errorf("normalized specification schema is incomplete")
		}
	case "bundle execute":
		for _, marker := range []string{"atsura.source.github_cli contract 2", "issue list or pr list", "--json=<ordered-select>", "--jq, --template, or --web", "source-owned authentication", "Successful source stderr must be empty"} {
			if !strings.Contains(prerequisites, marker) {
				return fmt.Errorf("runtime admission marker is missing")
			}
		}
	default:
		return fmt.Errorf("unsupported help contract")
	}
	return nil
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

func (r journeyRunner) failure(ctx context.Context, mode string, exit int, code string, retryable bool, arguments ...string) (commandOutcome, error) {
	outcome, err := r.command(ctx, mode, arguments...)
	if err != nil || outcome.exitCode != exit || len(outcome.stdout) != 0 {
		return commandOutcome{}, fmt.Errorf("command did not produce the expected failure boundary")
	}
	if err := validateFault(outcome.stderr, code, retryable); err != nil {
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

func transformDraft(value []byte) ([]byte, error) {
	text := string(value)
	replacements := [][2]string{
		{"reason: Include this verified command without transformation.", "reason: Return one reviewed compact pull request."},
		{"kind: identity", "kind: transform"},
		{"append_args: []", `append_args: ["--json=number,title,state"]`},
	}
	for _, replacement := range replacements {
		if strings.Count(text, replacement[0]) != 1 {
			return nil, fmt.Errorf("draft shape is not the expected single-command identity wrapper")
		}
		text = strings.Replace(text, replacement[0], replacement[1], 1)
	}
	if !strings.Contains(text, "- pr\n") || !strings.Contains(text, "- list\n") || strings.Contains(text, "output:") {
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

func validateSelectedOutput(result executionEvidence) error {
	if strings.Join(result.MatchedCommand, " ") != "pr list" || result.WrapperKind != "transform" || result.Output.Render != "compact_json" || result.Output.Shape != "array" || strings.Join(result.Output.Fields, ",") != "id,title,state" || result.Source.ExitCode != 0 || len(result.Output.Records) != 1 {
		return fmt.Errorf("unexpected execution metadata")
	}
	record := result.Output.Records[0]
	if len(record) != 3 || string(record["id"]) != "101" || string(record["title"]) != `"Review policy"` || string(record["state"]) != `"OPEN"` {
		return fmt.Errorf("unexpected selected record")
	}
	return nil
}

func validateFault(value []byte, code string, retryable bool) error {
	var document struct {
		SchemaVersion int `json:"schema_version"`
		Error         struct {
			Code      string `json:"code"`
			Retryable bool   `json:"retryable"`
		} `json:"error"`
	}
	if err := json.Unmarshal(value, &document); err != nil || document.SchemaVersion != 1 || document.Error.Code != code || document.Error.Retryable != retryable {
		return fmt.Errorf("unexpected structured fault evidence")
	}
	return nil
}

func digestValue(value string) bool {
	return lowercaseHex(value, 64)
}
