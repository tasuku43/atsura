package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"math"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/tasuku43/atsura/internal/app/bundleexecute"
	"github.com/tasuku43/atsura/internal/app/planapply"
	"github.com/tasuku43/atsura/internal/app/planpreview"
	"github.com/tasuku43/atsura/internal/domain/bundletrust"
	"github.com/tasuku43/atsura/internal/domain/fault"
	"github.com/tasuku43/atsura/internal/domain/operation"
	"github.com/tasuku43/atsura/internal/domain/sourceprocess"
	"github.com/tasuku43/atsura/internal/domain/tailoring"
	"github.com/tasuku43/atsura/internal/domain/tailoringbundle"
	"github.com/tasuku43/atsura/internal/domain/tailoringplan"
	"github.com/tasuku43/atsura/internal/infra/bundlejson"
	"github.com/tasuku43/atsura/internal/infra/githubcli"
	"github.com/tasuku43/atsura/internal/infra/sourceexec"
	"github.com/tasuku43/atsura/internal/infra/sourcejson"
	"github.com/tasuku43/atsura/internal/infra/trustfile"
)

const recoveryCanaryPrefix = "ATSURA_RECOVERY_CANARY_"

var recoveryHostileCanaries = []string{
	recoveryCanaryPrefix + "SECRET",
	recoveryCanaryPrefix + "ESC_\x1b_END",
	recoveryCanaryPrefix + "NEWLINE_\n_END",
	recoveryCanaryPrefix + "BIDI_\u202e_END",
	recoveryCanaryPrefix + "LINE_SEPARATOR_\u2028_END",
	recoveryCanaryPrefix + "PARAGRAPH_SEPARATOR_\u2029_END",
	recoveryCanaryPrefix + "BACKSLASH_\\_END",
	recoveryCanaryPrefix + "PROMPT_IGNORE_PREVIOUS_INSTRUCTIONS_END",
}

// The recovery fixture uses the production CLI, application services, bundle
// codec, trust store, source identity reader, runtime verifier, and JSON
// parser. Narrow controlled ports provide deterministic boundary observations;
// package tests independently prove that the production file, trust, identity,
// and process adapters emit those observations, including native races that
// cannot be forced portably here. The fixture exercises the generic encoder
// directly for defensive failures unreachable from valid typed application
// output. No fixture mode exists in production code.

type recoveryHelpDocument struct {
	Commands []struct {
		Path     string `json:"path"`
		Contract struct {
			Errors []CommandError `json:"errors"`
		} `json:"contract"`
	} `json:"commands"`
}

type recoveryBundlePort struct {
	bundle tailoringbundle.Bundle
	digest string
	err    error
}

func (p *recoveryBundlePort) Load(context.Context, string) (tailoringbundle.Bundle, string, error) {
	return p.bundle, p.digest, p.err
}

type recoveryAdoptionPort struct {
	state  bundletrust.State
	cancel context.CancelFunc
}

func (p *recoveryAdoptionPort) Inspect(context.Context, string) bundletrust.State {
	if p.cancel != nil {
		p.cancel()
	}
	return p.state
}

type recoveryIdentityPort struct {
	identity sourceprocess.Identity
	err      error
}

func (p *recoveryIdentityPort) Identify(context.Context, string) (sourceprocess.Identity, error) {
	return p.identity, p.err
}

type recoveryCompatibilityPort struct{ err error }

func (p *recoveryCompatibilityPort) VerifyRuntime(tailoringplan.Plan) error { return p.err }

type recoveryProcessPort struct {
	attempts int
	exitCode int
	stdout   []byte
	stderr   []byte
	err      error
	cancel   context.CancelFunc
	calls    int
}

type corruptingRecoveryExecutionService struct {
	inner bundleExecutionService
	calls int
}

func (s *corruptingRecoveryExecutionService) Execute(ctx context.Context, intent operation.Intent, bundlePath string, attempt tailoringplan.Attempt) (bundleexecute.Result, error) {
	s.calls++
	result, err := s.inner.Execute(ctx, intent, bundlePath, attempt)
	if err != nil {
		return result, err
	}
	result.TransformedJSON = &planapply.TransformedJSONResult{
		Render: tailoring.RenderCompactJSON,
		Output: tailoring.OutputResult{
			Shape:  tailoring.ResultShapeArray,
			Fields: []string{"id"},
			Records: []tailoring.JSONValue{tailoring.NewJSONObject([]tailoring.JSONField{{
				Name: "id", Value: tailoring.NewJSONNumber("NaN"),
			}})},
		},
		ExitCode: 0,
	}
	return result, nil
}

func (p *recoveryProcessPort) RunBound(_ context.Context, request sourceprocess.BoundRequest) (sourceprocess.Result, error) {
	p.calls++
	result := sourceprocess.Result{Attempts: p.attempts, ExitCode: p.exitCode}
	if p.attempts == 1 {
		result.Identity = request.ExpectedIdentity
		result.Stdout = append([]byte{}, p.stdout...)
		result.Stderr = append([]byte{}, p.stderr...)
	}
	if p.cancel != nil {
		p.cancel()
	}
	return result, p.err
}

type recoveryFixture struct {
	bundlePath string
	trustPath  string
	stateRoots []string
	bundle     tailoringbundle.Bundle
	digest     string
	trust      *trustfile.Store
}

type recoveryObservation struct {
	stdout   []byte
	stderr   []byte
	exitCode int
	attempts int
	state    []byte
}

type recoveryPhaseCase struct {
	code     string
	attempts int
}

func newRecoveryFixture(t *testing.T) recoveryFixture {
	t.Helper()
	catalogPath, specificationPath := githubRuntimeBundleArtifactPaths(t)
	bundlePath := bundleArtifactPath(t, catalogPath, specificationPath)
	bundle, digest, err := bundlejson.New().Load(context.Background(), bundlePath)
	if err != nil {
		t.Fatal(err)
	}
	identity, err := sourceexec.New().Identify(context.Background(), bundle.Catalog.Source.ResolvedPath)
	if err != nil {
		t.Fatal(err)
	}
	wantedIdentity := sourceprocess.Identity{ResolvedPath: bundle.Catalog.Source.ResolvedPath, SHA256: bundle.Catalog.Source.SHA256, Size: bundle.Catalog.Source.Size}
	if identity != wantedIdentity {
		t.Fatalf("recovery fixture source identity=%+v want=%+v", identity, wantedIdentity)
	}
	trustPath := filepath.Join(t.TempDir(), "atsura", "bundle-trust.json")
	store := trustfile.New(trustPath)
	if changed, err := store.Add(context.Background(), digest); err != nil || !changed {
		t.Fatalf("seed exact production trust store: changed=%t err=%v", changed, err)
	}
	return recoveryFixture{
		bundlePath: bundlePath,
		trustPath:  trustPath,
		stateRoots: []string{filepath.Dir(bundlePath), filepath.Dir(trustPath)},
		bundle:     bundle,
		digest:     digest,
		trust:      store,
	}
}

func (f recoveryFixture) arguments(path string) []string {
	parts := strings.Split(path, " ")
	return []string{"--error-format=json", parts[0], parts[1], "--bundle", f.bundlePath, "--", f.bundle.Catalog.Source.RequestedExecutable, "pr", "list", "--limit=1"}
}

func exactRecoveryHelp(t *testing.T, path string) map[string]CommandError {
	t.Helper()
	var stdout, stderr bytes.Buffer
	command := New(strings.NewReader(""), &stdout, &stderr)
	parts := strings.Split(path, " ")
	args := append([]string{"help"}, parts...)
	args = append(args, "--format=agent")
	if exit := command.RunContext(context.Background(), args); exit != ExitOK || stderr.Len() != 0 {
		t.Fatalf("help %s: exit=%d stderr=%q", path, exit, stderr.String())
	}
	var document recoveryHelpDocument
	if err := json.Unmarshal(stdout.Bytes(), &document); err != nil {
		t.Fatal(err)
	}
	if len(document.Commands) != 1 || document.Commands[0].Path != path {
		t.Fatalf("help %s commands=%+v", path, document.Commands)
	}
	result := make(map[string]CommandError, len(document.Commands[0].Contract.Errors))
	for _, declared := range document.Commands[0].Contract.Errors {
		if _, duplicate := result[declared.Code]; duplicate {
			t.Fatalf("help %s duplicated fault %q", path, declared.Code)
		}
		result[declared.Code] = declared
	}
	return result
}

func assertRecoveryObservation(t *testing.T, path string, declared CommandError, observation recoveryObservation, attempts int) {
	t.Helper()
	if observation.exitCode != exitCodeForKind(declared.Kind) {
		t.Fatalf("%s/%s exit=%d want=%d stderr=%s", path, declared.Code, observation.exitCode, exitCodeForKind(declared.Kind), observation.stderr)
	}
	if len(observation.stdout) != 0 {
		t.Fatalf("%s/%s emitted failure stdout: %q", path, declared.Code, observation.stdout)
	}
	var document errorDocument
	if err := json.Unmarshal(observation.stderr, &document); err != nil {
		t.Fatalf("%s/%s invalid fault JSON: %v: %q", path, declared.Code, err, observation.stderr)
	}
	got := document.Error
	if got.Code != declared.Code || got.Kind != declared.Kind || got.Retryable != declared.Retryable || len(got.NextActions) != len(declared.NextActions) {
		t.Fatalf("%s/%s fault=%+v declaration=%+v", path, declared.Code, got, declared)
	}
	for index := range declared.NextActions {
		if got.NextActions[index] != declared.NextActions[index] {
			t.Fatalf("%s/%s recovery[%d]=%+v want=%+v", path, declared.Code, index, got.NextActions[index], declared.NextActions[index])
		}
	}
	if observation.attempts != attempts {
		t.Fatalf("%s/%s attempts=%d want=%d", path, declared.Code, observation.attempts, attempts)
	}
	for _, boundary := range [][]byte{observation.stdout, observation.stderr, observation.state} {
		if bytes.Contains(boundary, []byte(recoveryCanaryPrefix)) {
			t.Fatalf("%s/%s leaked recovery canary", path, declared.Code)
		}
	}
}

type recoveryBoundarySignature struct {
	kind      fault.Kind
	retryable bool
}

var recoveryBoundarySignatures = map[string]recoveryBoundarySignature{
	"bundle_file_not_found":          {kind: fault.KindNotFound},
	"bundle_file_permission_denied":  {kind: fault.KindPermission},
	"unsafe_bundle_file":             {kind: fault.KindInvalidInput},
	"bundle_file_too_large":          {kind: fault.KindInvalidInput},
	"bundle_file_read_failed":        {kind: fault.KindUnavailable, retryable: true},
	"invalid_bundle_file":            {kind: fault.KindInvalidInput},
	"legacy_tailoring_schema":        {kind: fault.KindInvalidInput},
	"bundle_digest_mismatch":         {kind: fault.KindRejected},
	"source_executable_not_found":    {kind: fault.KindNotFound},
	"source_identity_unavailable":    {kind: fault.KindUnavailable, retryable: true},
	"unsafe_source_executable":       {kind: fault.KindInvalidInput},
	"source_identity_changed":        {kind: fault.KindRejected},
	"invalid_source_identity":        {kind: fault.KindContract},
	"invalid_source_process_request": {kind: fault.KindContract},
	"source_process_start_failed":    {kind: fault.KindUnavailable, retryable: true},
	"source_stdout_too_large":        {kind: fault.KindContract},
	"source_stderr_too_large":        {kind: fault.KindContract},
	"source_execution_canceled":      {kind: fault.KindCanceled},
	"source_command_timeout":         {kind: fault.KindUnavailable},
	"source_command_failed":          {kind: fault.KindRejected},
	"source_process_wait_failed":     {kind: fault.KindUnavailable},
}

func publicRecoveryFault(t *testing.T, code string) error {
	t.Helper()
	signature, exists := recoveryBoundarySignatures[code]
	if !exists {
		t.Fatalf("recovery boundary signature for %q is not independently defined", code)
	}
	return fault.Wrap(signature.kind, code, "The controlled recovery boundary failed.", signature.retryable, errors.New(recoveryHostileInput()))
}

func recoveryHostileInput() string {
	return strings.Join(recoveryHostileCanaries, "|")
}

func runRecoveryCommand(t *testing.T, command *CLI, ctx context.Context, args []string, process *recoveryProcessPort, fixture recoveryFixture, stdout, stderr *bytes.Buffer, wantedAttempts, wantedCalls int) recoveryObservation {
	t.Helper()
	exit := command.RunContext(ctx, args)
	attempts := 0
	if process != nil {
		if process.calls != wantedCalls {
			t.Fatalf("source process calls=%d want=%d", process.calls, wantedCalls)
		}
		if process.calls != 0 && process.calls != 1 {
			t.Fatalf("source process was invoked more than once: %d", process.calls)
		}
	}
	if process != nil && process.calls == 1 {
		attempts = process.attempts
	}
	if attempts != wantedAttempts {
		t.Fatalf("source process attempts=%d want=%d", attempts, wantedAttempts)
	}
	return recoveryObservation{stdout: stdout.Bytes(), stderr: stderr.Bytes(), exitCode: exit, attempts: attempts, state: recoveryStateBytes(t, fixture)}
}

func recoveryStateBytes(t *testing.T, fixture recoveryFixture) []byte {
	t.Helper()
	var state []byte
	for _, root := range fixture.stateRoots {
		err := filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if entry.IsDir() {
				return nil
			}
			info, err := entry.Info()
			if err != nil {
				return err
			}
			if !info.Mode().IsRegular() {
				return nil
			}
			raw, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			state = append(state, raw...)
			return nil
		})
		if err != nil {
			t.Fatalf("scan isolated recovery state root %q: %v", root, err)
		}
	}
	return state
}

type recoveryFailWriter struct{}

func (recoveryFailWriter) Write([]byte) (int, error) { return 0, io.ErrClosedPipe }

func invalidEncodingValue() any {
	return struct {
		Value float64 `json:"value"`
	}{Value: math.NaN()}
}

var previewRecoveryCodes = []string{
	"invalid_arguments",
	"bundle_file_not_found",
	"bundle_file_permission_denied",
	"unsafe_bundle_file",
	"bundle_file_too_large",
	"bundle_file_read_failed",
	"invalid_bundle_file",
	"legacy_tailoring_schema",
	"bundle_digest_mismatch",
	"invalid_bundle_trust_store",
	"bundle_not_adopted",
	"bundle_source_drift",
	"source_executable_not_found",
	"source_identity_unavailable",
	"unsafe_source_executable",
	"source_identity_changed",
	"invalid_source_identity",
	"source_executable_mismatch",
	"invalid_invocation",
	"command_not_in_surface",
	"option_not_in_surface",
	"invalid_wrapper_plan",
	"output_contract_exceeded",
	"output_encoding_failed",
	"internal_error",
	"output_write_failed",
	"operation_canceled",
}

var executeRecoveryCases = []recoveryPhaseCase{
	{code: "invalid_arguments"},
	{code: "bundle_file_not_found"},
	{code: "bundle_file_permission_denied"},
	{code: "unsafe_bundle_file"},
	{code: "bundle_file_too_large"},
	{code: "bundle_file_read_failed"},
	{code: "invalid_bundle_file"},
	{code: "legacy_tailoring_schema"},
	{code: "bundle_digest_mismatch"},
	{code: "invalid_bundle_trust_store"},
	{code: "bundle_not_adopted"},
	{code: "bundle_source_drift"},
	{code: "source_executable_not_found"},
	{code: "source_identity_unavailable"},
	{code: "unsafe_source_executable"},
	{code: "source_identity_changed"},
	{code: "invalid_source_identity"},
	{code: "source_executable_mismatch"},
	{code: "invalid_invocation"},
	{code: "command_not_in_surface"},
	{code: "option_not_in_surface"},
	{code: "invalid_wrapper_plan"},
	{code: "wrapper_runtime_not_supported"},
	{code: "invalid_source_process_request"},
	{code: "source_process_start_failed"},
	{code: "unclassified_source_execution_outcome"},
	{code: "internal_error"},
	{code: "operation_canceled"},
	{code: "source_stdout_too_large", attempts: 1},
	{code: "source_stderr_too_large", attempts: 1},
	{code: "source_execution_canceled", attempts: 1},
	{code: "source_command_timeout", attempts: 1},
	{code: "source_identity_changed", attempts: 1},
	{code: "source_command_failed", attempts: 1},
	{code: "source_process_wait_failed", attempts: 1},
	{code: "source_stderr_not_supported", attempts: 1},
	{code: "source_output_processing_canceled", attempts: 1},
	{code: "source_json_invalid", attempts: 1},
	{code: "output_transform_failed", attempts: 1},
	{code: "unclassified_source_execution_outcome", attempts: 1},
	{code: "output_contract_exceeded", attempts: 1},
	{code: "output_encoding_failed", attempts: 1},
	{code: "execute_output_write_failed", attempts: 1},
}

func requireRecoveryCodeSet(t *testing.T, path string, declarations map[string]CommandError, codes []string) {
	t.Helper()
	seen := make(map[string]struct{}, len(codes))
	for _, code := range codes {
		if _, duplicate := seen[code]; duplicate {
			t.Fatalf("%s expected fault %q is duplicated", path, code)
		}
		seen[code] = struct{}{}
		if _, exists := declarations[code]; !exists {
			t.Fatalf("%s expected fault %q is missing from exact help", path, code)
		}
	}
	if len(seen) != len(declarations) {
		for code := range declarations {
			if _, exists := seen[code]; !exists {
				t.Errorf("%s exact help has unexercised fault %q", path, code)
			}
		}
		t.Fatalf("%s exercised fault count=%d help count=%d", path, len(seen), len(declarations))
	}
}

func requireExecuteRecoverySet(t *testing.T, declarations map[string]CommandError) {
	t.Helper()
	unique := make([]string, 0, len(executeRecoveryCases))
	seen := make(map[string]int, len(executeRecoveryCases))
	preStart := 0
	postStart := 0
	for _, item := range executeRecoveryCases {
		seen[item.code]++
		if item.attempts == 0 {
			preStart++
		} else if item.attempts == 1 {
			postStart++
		} else {
			t.Fatalf("bundle execute fault %q has invalid attempt phase %d", item.code, item.attempts)
		}
		if seen[item.code] == 1 {
			unique = append(unique, item.code)
		}
	}
	requireRecoveryCodeSet(t, "bundle execute", declarations, unique)
	if preStart != 28 || postStart != 15 {
		t.Fatalf("bundle execute phase counts pre=%d post=%d want=28/15", preStart, postStart)
	}
	for _, code := range []string{"source_identity_changed", "unclassified_source_execution_outcome"} {
		if seen[code] != 2 {
			t.Fatalf("bundle execute fault %q phase variants=%d want=2", code, seen[code])
		}
	}
	for code, count := range seen {
		if code != "source_identity_changed" && code != "unclassified_source_execution_outcome" && count != 1 {
			t.Fatalf("bundle execute fault %q phase variants=%d want=1", code, count)
		}
	}
}

func cloneRecoveryBundle(t *testing.T, bundle tailoringbundle.Bundle) tailoringbundle.Bundle {
	t.Helper()
	raw, err := json.Marshal(bundle)
	if err != nil {
		t.Fatal(err)
	}
	var clone tailoringbundle.Bundle
	if err := json.Unmarshal(raw, &clone); err != nil {
		t.Fatal(err)
	}
	return clone
}

func compileRecoveryVariant(t *testing.T, fixture recoveryFixture, variant string) (tailoringbundle.Bundle, string) {
	t.Helper()
	specification := cloneRecoveryBundle(t, fixture.bundle).Specification
	switch variant {
	case "command_absent":
		specification.Commands = []tailoringbundle.CommandEntry{}
	case "option_absent":
		specification.Commands[0].Options.Exclude = []string{"--limit"}
	case "identity_wrapper":
		specification.Commands[0].Wrapper = &tailoringbundle.Wrapper{
			Kind: tailoringbundle.WrapperIdentity, Before: []tailoringbundle.StageAction{},
			Invoke: tailoringbundle.Invocation{OptionDefaults: []tailoringbundle.OptionDefault{}, AppendArgs: []string{}}, After: []tailoringbundle.StageAction{},
		}
	default:
		t.Fatalf("unknown recovery bundle variant %q", variant)
	}
	bundle, err := tailoringbundle.Compile(fixture.bundle.Catalog, specification)
	if err != nil {
		t.Fatal(err)
	}
	digest, err := bundle.Digest()
	if err != nil {
		t.Fatal(err)
	}
	return bundle, digest
}

func malformedRecoveryBundle(t *testing.T, fixture recoveryFixture) tailoringbundle.Bundle {
	t.Helper()
	bundle := cloneRecoveryBundle(t, fixture.bundle)
	bundle.Surface[0].Reason = "inconsistent derived surface"
	return bundle
}

func driftRecoveryBundle(t *testing.T, fixture recoveryFixture) (tailoringbundle.Bundle, string, string) {
	t.Helper()
	name := "source"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	path := filepath.Join(t.TempDir(), name)
	raw, err := os.ReadFile(os.Args[0])
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, raw, 0o700); err != nil {
		t.Fatal(err)
	}
	identity, err := sourceexec.New().Identify(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	bundle := cloneRecoveryBundle(t, fixture.bundle)
	bundle.Catalog.Source.RequestedExecutable = path
	bundle.Catalog.Source.ResolvedPath = identity.ResolvedPath
	bundle.Catalog.Source.SHA256 = identity.SHA256
	bundle.Catalog.Source.Size = identity.Size
	catalogDigest, err := bundle.Catalog.Digest()
	if err != nil {
		t.Fatal(err)
	}
	bundle.Specification.CatalogDigest = catalogDigest
	bundle, err = tailoringbundle.Compile(bundle.Catalog, bundle.Specification)
	if err != nil {
		t.Fatal(err)
	}
	digest, err := bundle.Digest()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, append(raw, byte('\n')), 0o700); err != nil {
		t.Fatal(err)
	}
	return bundle, digest, path
}

func replaceRecoveryInvocation(fixture recoveryFixture, args []string, executable string, argv ...string) []string {
	marker := -1
	for index, value := range args {
		if value == "--" {
			marker = index
			break
		}
	}
	if marker < 0 {
		return args
	}
	result := append([]string{}, args[:marker+1]...)
	result = append(result, executable)
	result = append(result, argv...)
	return result
}

func TestBundlePreviewProductionCompositionCoversEveryRecovery(t *testing.T) {
	fixture := newRecoveryFixture(t)
	declarations := exactRecoveryHelp(t, "bundle preview")
	requireRecoveryCodeSet(t, "bundle preview", declarations, previewRecoveryCodes)
	for _, code := range previewRecoveryCodes {
		t.Run(code, func(t *testing.T) {
			observation := runPreviewRecoveryCase(t, fixture, code)
			assertRecoveryObservation(t, "bundle preview", declarations[code], observation, 0)
		})
	}
}

func runPreviewRecoveryCase(t *testing.T, fixture recoveryFixture, code string) recoveryObservation {
	t.Helper()
	var stdout, stderr bytes.Buffer
	var bundles planpreview.BundlePort = &recoveryBundlePort{bundle: fixture.bundle, digest: fixture.digest}
	var adoption planpreview.AdoptionPort = fixture.trust
	var identity planpreview.IdentityPort = sourceexec.New()
	ctx := context.Background()
	args := fixture.arguments("bundle preview")
	out := io.Writer(&stdout)

	switch code {
	case "invalid_arguments":
		args = []string{"--error-format=json", "bundle", "preview"}
	case "bundle_file_not_found", "bundle_file_permission_denied", "unsafe_bundle_file", "bundle_file_too_large", "bundle_file_read_failed", "invalid_bundle_file", "legacy_tailoring_schema", "bundle_digest_mismatch":
		bundles = &recoveryBundlePort{err: publicRecoveryFault(t, code)}
	case "invalid_bundle_trust_store":
		adoption = &recoveryAdoptionPort{state: bundletrust.StateInvalid}
	case "bundle_not_adopted":
		adoption = &recoveryAdoptionPort{state: bundletrust.StateNotAdopted}
	case "bundle_source_drift":
		bundle, digest, executable := driftRecoveryBundle(t, fixture)
		bundles = &recoveryBundlePort{bundle: bundle, digest: digest}
		adoption = &recoveryAdoptionPort{state: bundletrust.StateAdopted}
		args = replaceRecoveryInvocation(fixture, args, executable, "pr", "list", "--limit=1")
	case "source_executable_not_found", "source_identity_unavailable", "unsafe_source_executable", "source_identity_changed", "invalid_source_identity":
		identity = &recoveryIdentityPort{err: publicRecoveryFault(t, code)}
	case "source_executable_mismatch":
		args = replaceRecoveryInvocation(fixture, args, filepath.Join(t.TempDir(), "other"), "pr", "list")
	case "invalid_invocation":
		args = replaceRecoveryInvocation(fixture, args, fixture.bundle.Catalog.Source.RequestedExecutable, "unknown")
	case "command_not_in_surface":
		bundle, digest := compileRecoveryVariant(t, fixture, "command_absent")
		bundles = &recoveryBundlePort{bundle: bundle, digest: digest}
		adoption = &recoveryAdoptionPort{state: bundletrust.StateAdopted}
	case "option_not_in_surface":
		bundle, digest := compileRecoveryVariant(t, fixture, "option_absent")
		bundles = &recoveryBundlePort{bundle: bundle, digest: digest}
		adoption = &recoveryAdoptionPort{state: bundletrust.StateAdopted}
	case "invalid_wrapper_plan":
		bundles = &recoveryBundlePort{bundle: malformedRecoveryBundle(t, fixture), digest: fixture.digest}
	case "output_contract_exceeded":
		command := New(strings.NewReader(""), &stdout, &stderr)
		bound := withCommandPath(withErrorFormat(ctx, errorFormatJSON), "bundle preview")
		exit := command.emitJSONDocument(bound, strings.Repeat("x", maxBundleOutputBytes), "bundle preview")
		return recoveryObservation{stdout: stdout.Bytes(), stderr: stderr.Bytes(), exitCode: exit, state: recoveryStateBytes(t, fixture)}
	case "output_encoding_failed":
		command := New(strings.NewReader(""), &stdout, &stderr)
		bound := withCommandPath(withErrorFormat(ctx, errorFormatJSON), "bundle preview")
		exit := command.emitJSONDocument(bound, invalidEncodingValue(), "bundle preview")
		return recoveryObservation{stdout: stdout.Bytes(), stderr: stderr.Bytes(), exitCode: exit, state: recoveryStateBytes(t, fixture)}
	case "internal_error":
		bundles = &recoveryBundlePort{err: errors.New(recoveryHostileInput())}
	case "output_write_failed":
		out = recoveryFailWriter{}
	case "operation_canceled":
		var cancel context.CancelFunc
		ctx, cancel = context.WithCancel(context.Background())
		adoption = &recoveryAdoptionPort{state: bundletrust.StateAdopted, cancel: cancel}
	default:
		t.Fatalf("unimplemented preview recovery case %q", code)
	}

	command := New(strings.NewReader(""), out, &stderr)
	command.previews = planpreview.New(bundles, adoption, identity)
	exit := command.RunContext(ctx, args)
	return recoveryObservation{stdout: stdout.Bytes(), stderr: stderr.Bytes(), exitCode: exit, state: recoveryStateBytes(t, fixture)}
}

func TestBundleExecuteProductionCompositionCoversEveryPreAndPostStartRecovery(t *testing.T) {
	fixture := newRecoveryFixture(t)
	declarations := exactRecoveryHelp(t, "bundle execute")
	requireExecuteRecoverySet(t, declarations)
	for _, test := range executeRecoveryCases {
		name := test.code + "/pre_start"
		if test.attempts == 1 {
			name = test.code + "/post_start"
		}
		t.Run(name, func(t *testing.T) {
			observation := runExecuteRecoveryCase(t, fixture, test.code, test.attempts)
			assertRecoveryObservation(t, "bundle execute", declarations[test.code], observation, test.attempts)
		})
	}
}

func runExecuteRecoveryCase(t *testing.T, fixture recoveryFixture, code string, wantedAttempts int) recoveryObservation {
	t.Helper()
	var stdout, stderr bytes.Buffer
	var bundles bundleexecute.BundlePort = &recoveryBundlePort{bundle: fixture.bundle, digest: fixture.digest}
	var adoption bundleexecute.AdoptionPort = fixture.trust
	var identity bundleexecute.IdentityPort = sourceexec.New()
	var compatibility bundleexecute.CompatibilityPort = githubcli.NewRuntimeVerifier()
	process := &recoveryProcessPort{
		attempts: 1,
		exitCode: 0,
		stdout: []byte(`[{"number":101,"title":"safe","state":"OPEN","ignored":` +
			quotedJSON(t, recoveryHostileInput()) + `}]`),
	}
	var processes bundleexecute.ProcessPort = process
	var parser bundleexecute.ParserPort = sourcejson.New()
	ctx := context.Background()
	args := fixture.arguments("bundle execute")
	out := io.Writer(&stdout)

	switch code {
	case "invalid_arguments":
		args = []string{"--error-format=json", "bundle", "execute"}
	case "bundle_file_not_found", "bundle_file_permission_denied", "unsafe_bundle_file", "bundle_file_too_large", "bundle_file_read_failed", "invalid_bundle_file", "legacy_tailoring_schema", "bundle_digest_mismatch":
		bundles = &recoveryBundlePort{err: publicRecoveryFault(t, code)}
	case "invalid_bundle_trust_store":
		adoption = &recoveryAdoptionPort{state: bundletrust.StateInvalid}
	case "bundle_not_adopted":
		adoption = &recoveryAdoptionPort{state: bundletrust.StateNotAdopted}
	case "bundle_source_drift":
		bundle, digest, executable := driftRecoveryBundle(t, fixture)
		bundles = &recoveryBundlePort{bundle: bundle, digest: digest}
		adoption = &recoveryAdoptionPort{state: bundletrust.StateAdopted}
		args = replaceRecoveryInvocation(fixture, args, executable, "pr", "list", "--limit=1")
	case "source_executable_not_found", "source_identity_unavailable", "unsafe_source_executable", "invalid_source_identity":
		identity = &recoveryIdentityPort{err: publicRecoveryFault(t, code)}
	case "source_identity_changed":
		if wantedAttempts == 0 {
			identity = &recoveryIdentityPort{err: publicRecoveryFault(t, code)}
		} else {
			process = recoveryFailedProcess(t, code, 1)
			processes = process
		}
	case "source_executable_mismatch":
		args = replaceRecoveryInvocation(fixture, args, filepath.Join(t.TempDir(), "other"), "pr", "list")
	case "invalid_invocation":
		args = replaceRecoveryInvocation(fixture, args, fixture.bundle.Catalog.Source.RequestedExecutable, "unknown")
	case "command_not_in_surface":
		bundle, digest := compileRecoveryVariant(t, fixture, "command_absent")
		bundles = &recoveryBundlePort{bundle: bundle, digest: digest}
		adoption = &recoveryAdoptionPort{state: bundletrust.StateAdopted}
	case "option_not_in_surface":
		bundle, digest := compileRecoveryVariant(t, fixture, "option_absent")
		bundles = &recoveryBundlePort{bundle: bundle, digest: digest}
		adoption = &recoveryAdoptionPort{state: bundletrust.StateAdopted}
	case "invalid_wrapper_plan":
		bundles = &recoveryBundlePort{bundle: malformedRecoveryBundle(t, fixture), digest: fixture.digest}
	case "wrapper_runtime_not_supported":
		compatibility = &recoveryCompatibilityPort{err: errors.New(recoveryHostileInput())}
	case "invalid_source_process_request":
		// A valid plan necessarily produces a valid request. Keep the defensive
		// public recovery executable at the CLI boundary without corrupting a
		// production bundle or adding a fixture-only request builder.
		command := New(strings.NewReader(""), &stdout, &stderr)
		bound := withCommandPath(withErrorFormat(ctx, errorFormatJSON), "bundle execute")
		exit := command.fail(bound, publicRecoveryFault(t, code))
		return recoveryObservation{stdout: stdout.Bytes(), stderr: stderr.Bytes(), exitCode: exit, state: recoveryStateBytes(t, fixture)}
	case "source_process_start_failed":
		process = recoveryFailedProcess(t, code, 0)
		processes = process
	case "unclassified_source_execution_outcome":
		process = &recoveryProcessPort{attempts: wantedAttempts, exitCode: -1, err: errors.New(recoveryHostileInput())}
		processes = process
	case "internal_error":
		bundles = &recoveryBundlePort{err: errors.New(recoveryHostileInput())}
	case "operation_canceled":
		var cancel context.CancelFunc
		ctx, cancel = context.WithCancel(context.Background())
		adoption = &recoveryAdoptionPort{state: bundletrust.StateAdopted, cancel: cancel}
	case "source_stdout_too_large", "source_stderr_too_large", "source_execution_canceled", "source_command_timeout", "source_command_failed", "source_process_wait_failed":
		process = recoveryFailedProcess(t, code, 1)
		processes = process
	case "source_stderr_not_supported":
		process = &recoveryProcessPort{attempts: 1, exitCode: 0, stdout: []byte(`[]`), stderr: []byte(recoveryHostileInput())}
		processes = process
	case "source_output_processing_canceled":
		var cancel context.CancelFunc
		ctx, cancel = context.WithCancel(context.Background())
		process = &recoveryProcessPort{attempts: 1, exitCode: 0, stdout: []byte(`[]`), cancel: cancel}
		processes = process
	case "source_json_invalid":
		process = &recoveryProcessPort{attempts: 1, exitCode: 0, stdout: []byte(`[{` + recoveryHostileInput())}
		processes = process
	case "output_transform_failed":
		process = &recoveryProcessPort{attempts: 1, exitCode: 0, stdout: []byte(`[{"number":101,"title":"safe","ignored":` + quotedJSON(t, recoveryHostileInput()) + `}]`)}
		processes = process
	case "output_contract_exceeded":
		large := strings.Repeat("x", maxBundleOutputBytes+1024)
		process = &recoveryProcessPort{attempts: 1, exitCode: 0, stdout: []byte(`[{"number":101,"title":` + quotedJSON(t, large) + `,"state":"OPEN"}]`)}
		processes = process
	case "output_encoding_failed":
		// A valid bundleexecute service cannot produce malformed typed JSON.
		// Decorate its narrow result boundary only after the production service
		// and controlled source process complete one real attempt.
		execution := &corruptingRecoveryExecutionService{inner: bundleexecute.New(bundles, adoption, identity, compatibility, processes, parser)}
		command := New(strings.NewReader(""), &stdout, &stderr)
		command.executions = execution
		exit := command.RunContext(ctx, args)
		if execution.calls != 1 {
			t.Fatalf("bundle execution service calls=%d want=1", execution.calls)
		}
		if process.calls != 1 || process.attempts != 1 {
			t.Fatalf("source process calls=%d attempts=%d want=1/1", process.calls, process.attempts)
		}
		return recoveryObservation{
			stdout: stdout.Bytes(), stderr: stderr.Bytes(), exitCode: exit,
			attempts: process.attempts,
			state:    recoveryStateBytes(t, fixture),
		}
	case "execute_output_write_failed":
		out = recoveryFailWriter{}
	default:
		t.Fatalf("unimplemented execute recovery case %q/%d", code, wantedAttempts)
	}

	command := New(strings.NewReader(""), out, &stderr)
	command.executions = bundleexecute.New(bundles, adoption, identity, compatibility, processes, parser)
	return runRecoveryCommand(
		t, command, ctx, args, process, fixture, &stdout, &stderr,
		wantedAttempts, expectedRecoveryProcessCalls(code, wantedAttempts),
	)
}

func expectedRecoveryProcessCalls(code string, attempts int) int {
	if attempts == 1 {
		return 1
	}
	if attempts == 0 && (code == "source_process_start_failed" || code == "unclassified_source_execution_outcome") {
		return 1
	}
	return 0
}

func recoveryFailedProcess(t *testing.T, code string, attempts int) *recoveryProcessPort {
	t.Helper()
	exitCode := -1
	if code == "source_command_failed" {
		exitCode = 42
	}
	return &recoveryProcessPort{
		attempts: attempts,
		exitCode: exitCode,
		stdout:   []byte(recoveryHostileInput()),
		stderr:   []byte(recoveryHostileInput()),
		err:      publicRecoveryFault(t, code),
	}
}

func quotedJSON(t *testing.T, value string) string {
	t.Helper()
	raw, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}
