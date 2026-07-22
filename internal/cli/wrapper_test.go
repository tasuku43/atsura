package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"

	"github.com/tasuku43/atsura/internal/app/planapply"
	"github.com/tasuku43/atsura/internal/app/wrapperrender"
	"github.com/tasuku43/atsura/internal/app/wrapperrun"
	"github.com/tasuku43/atsura/internal/domain/fault"
	"github.com/tasuku43/atsura/internal/domain/operation"
	"github.com/tasuku43/atsura/internal/domain/tailoring"
	"github.com/tasuku43/atsura/internal/domain/tailoringbundle"
	"github.com/tasuku43/atsura/internal/domain/tailoringplan"
	"github.com/tasuku43/atsura/internal/domain/wrapperbinding"
)

type cliWrapperRenderStub struct {
	result wrapperrender.Result
	err    error
	calls  int
	path   string
}

func (s *cliWrapperRenderStub) Render(_ context.Context, _ operation.Intent, path string) (wrapperrender.Result, error) {
	s.calls++
	s.path = path
	return s.result, s.err
}

type cliWrapperRunStub struct {
	result  wrapperrun.Result
	err     error
	calls   int
	binding wrapperbinding.RuntimeInvocation
	args    []string
}

type orderedWriter struct {
	label  string
	order  *[]string
	buffer bytes.Buffer
}

func (w *orderedWriter) Write(value []byte) (int, error) {
	*w.order = append(*w.order, w.label)
	return w.buffer.Write(value)
}

type partialFirstWriter struct {
	calls      int
	firstLimit int
	buffer     bytes.Buffer
}

func (w *partialFirstWriter) Write(value []byte) (int, error) {
	w.calls++
	if w.calls == 1 {
		kept := w.firstLimit
		if kept > len(value) {
			kept = len(value)
		}
		_, _ = w.buffer.Write(value[:kept])
		return kept, nil
	}
	return w.buffer.Write(value)
}

func (s *cliWrapperRunStub) Execute(_ context.Context, _ operation.Intent, binding wrapperbinding.RuntimeInvocation, args []string) (wrapperrun.Result, error) {
	s.calls++
	s.binding = binding
	s.args = append([]string{}, args...)
	return s.result, s.err
}

func testWrapperRenderResult(t *testing.T) wrapperrender.Result {
	t.Helper()
	source := []byte("gh() {\n  : fixed wrapper fixture\n}\n")
	material, err := wrapperbinding.NewRenderedMaterial(source)
	if err != nil {
		t.Fatal(err)
	}
	return wrapperrender.Result{
		Binding: wrapperbinding.Binding{
			ContractVersion: wrapperbinding.ContractVersion,
			BundleLocator:   filepath.Join(t.TempDir(), "purpose bundle.json"),
			BundleDigest:    strings.Repeat("a", 64),
			CommandName:     "gh",
			Help: wrapperbinding.CompiledHelp{Commands: []wrapperbinding.HelpCommand{{
				Path: []string{"pr", "list"}, Summary: "List pull requests", Reason: "Return one reviewed result.", Options: []wrapperbinding.HelpOption{},
			}}},
			Runtime: wrapperbinding.RuntimeIdentity{
				ResolvedPath: filepath.Join(t.TempDir(), "atr"),
				SHA256:       strings.Repeat("b", 64),
				Size:         4096,
			},
		},
		Material:              material,
		SourceProcessAttempts: 0,
	}
}

func testWrapperRunResult(shape tailoring.ResultShape) wrapperrun.Result {
	record := tailoring.NewJSONObject([]tailoring.JSONField{{
		Name: "name", Value: tailoring.NewJSONString("line\n sep:\u2028 bidi:\u202e slash:\\"),
	}})
	return wrapperrun.Result{
		BundleDigest:   strings.Repeat("a", 64),
		PlanDigest:     strings.Repeat("c", 64),
		MatchedCommand: []string{"pr", "list"},
		WrapperKind:    tailoringbundle.WrapperTransform,
		ResultMode:     tailoringplan.ResultModeTransformedJSON,
		TransformedJSON: &planapply.TransformedJSONResult{
			Render: tailoring.RenderCompactJSON,
			Output: tailoring.OutputResult{
				Shape: shape, Fields: []string{"name"}, Records: []tailoring.JSONValue{record},
			},
			ExitCode: 0,
		},
		SourceProcessAttempts: 1,
	}
}

func testWrapperSourceStreamResult(stdout, stderr []byte, status int, kind tailoringbundle.WrapperKind) wrapperrun.Result {
	return wrapperrun.Result{
		BundleDigest:          strings.Repeat("a", 64),
		PlanDigest:            strings.Repeat("c", 64),
		MatchedCommand:        []string{"pr", "list"},
		WrapperKind:           kind,
		ResultMode:            tailoringplan.ResultModeSourceStreamPassthrough,
		SourceStream:          &planapply.SourceStreamResult{Stdout: append([]byte(nil), stdout...), Stderr: append([]byte(nil), stderr...), ExitCode: status},
		SourceProcessAttempts: 1,
	}
}

func testWrapperOptimizerResult(stdout, stderr []byte, status int, disposition planapply.OptimizerDisposition, processorAttempts int) wrapperrun.Result {
	return wrapperrun.Result{
		BundleDigest:             strings.Repeat("a", 64),
		PlanDigest:               strings.Repeat("c", 64),
		MatchedCommand:           []string{"test"},
		WrapperKind:              tailoringbundle.WrapperTransform,
		ResultMode:               tailoringplan.ResultModeOriginalPreservingOptimizer,
		Optimizer:                &planapply.OptimizerResult{Stdout: append([]byte(nil), stdout...), Stderr: append([]byte(nil), stderr...), ExitCode: status, Disposition: disposition},
		SourceProcessAttempts:    1,
		ProcessorProcessAttempts: processorAttempts,
	}
}

func wrapperRunInvocation(binding wrapperbinding.RuntimeInvocation, argv ...string) []string {
	args := []string{
		"wrapper", "run",
		"--contract-version=" + strconv.Itoa(wrapperbinding.ContractVersion),
		"--bundle=" + binding.BundleLocator,
		"--bundle-digest=" + binding.BundleDigest,
		"--runtime-path=" + binding.Runtime.ResolvedPath,
		"--runtime-sha256=" + binding.Runtime.SHA256,
		"--runtime-size=" + strconv.FormatInt(binding.Runtime.Size, 10),
		"--",
	}
	return append(args, argv...)
}

func TestWrapperCatalogPublishesExactHostNeutralContracts(t *testing.T) {
	catalog := DefaultCatalog()
	render, found := catalog.Lookup("wrapper render")
	if !found {
		t.Fatal("wrapper render is missing")
	}
	run, found := catalog.Lookup("wrapper run")
	if !found {
		t.Fatal("wrapper run is missing")
	}
	if render.Role != RoleUtility || render.Effect != operation.EffectRead || run.Role != RoleUtility || run.Effect != operation.EffectExecute ||
		render.Agent.CapabilityID != "tailoring.wrapper.materialize" || run.Agent.CapabilityID != render.Agent.CapabilityID {
		t.Fatalf("render/run contracts = %+v / %+v", render, run)
	}
	wantRunArgs := "--contract-version=" + strconv.Itoa(wrapperbinding.ContractVersion) + " --bundle=<absolute-path> --bundle-digest=<sha256> --runtime-path=<absolute-path> --runtime-sha256=<sha256> --runtime-size=<bytes> -- [argv]"
	if render.Args != "--bundle <absolute-path> [--format text|json]" || run.Args != wantRunArgs {
		t.Fatalf("render/run grammar = %q / %q", render.Args, run.Args)
	}
	wantRenderFields := []string{"source", "source_sha256", "command", "contract", "bundle", "runtime", "source_process_attempts", "processor_process_attempts"}
	gotRenderFields := make([]string, len(render.Agent.Output.Fields))
	for index, field := range render.Agent.Output.Fields {
		gotRenderFields[index] = field.Name
	}
	if !reflect.DeepEqual(gotRenderFields, wantRenderFields) || render.Agent.Output.JSONEnvelope != "wrapper" || render.Agent.Output.JSONSchemaVersion != 2 {
		t.Fatalf("render output = %+v", render.Agent.Output)
	}
	if run.Agent.Output.Authority != OutputAuthorityFreshWrapperPlan ||
		!reflect.DeepEqual(run.Agent.Output.Formats, []OutputFormat{OutputFormatPlanResult}) ||
		!reflect.DeepEqual(run.Agent.Output.PlanResultModes, freshPlanResultModes()) ||
		run.Agent.Output.PlanSchema == nil || *run.Agent.Output.PlanSchema != (OutputSchemaReference{Command: "bundle preview", Field: "plan", ID: "wrapper-plan", Version: tailoringplan.SchemaVersion}) {
		t.Fatalf("run output = %+v", run.Agent.Output)
	}
	if len(run.Agent.Inputs) != 7 || run.Agent.Inputs[6].Name != "argv" || run.Agent.Inputs[6].Required || run.Agent.Inputs[6].Cardinality != InputCardinalityRepeatable {
		t.Fatalf("run inputs = %+v", run.Agent.Inputs)
	}
	if run.Agent.Inputs[5].Minimum == nil || *run.Agent.Inputs[5].Minimum != 1 || run.Agent.Inputs[5].Maximum == nil {
		t.Fatalf("runtime size input = %+v", run.Agent.Inputs[5])
	}
	if err := catalog.Validate(); err != nil {
		t.Fatal(err)
	}
}

func TestFreshPlanResultModesPublishExactOptimizerSuccessUnion(t *testing.T) {
	modes := freshPlanResultModes()
	if len(modes) != 3 || modes[2].Mode != tailoringplan.ResultModeOriginalPreservingOptimizer || len(modes[2].SuccessVariants) != 3 {
		t.Fatalf("plan result modes = %+v", modes)
	}
	want := []PlanResultSuccessContract{
		{
			Disposition: PlanResultDispositionPreservedBeforeProcessor,
			Stdout:      PlanResultStreamExactSourceBytes, Stderr: PlanResultStreamExactSourceBytes,
			ExitStatus: PlanResultExitStatusSourceConventional, Framing: PlanResultFramingNone, Projection: PlanResultProjectionNone,
			Delivery: PlanResultDeliveryBufferedAfterCompletion, CrossStreamOrder: PlanResultCrossStreamOrderNotPreserved,
			StdoutLimitBytes: 4 * 1024 * 1024, StderrLimitBytes: 256 * 1024,
			SourceProcessAttempts: 1, ProcessorProcessAttempts: 0,
		},
		{
			Disposition: PlanResultDispositionPreservedAfterProcessor,
			Stdout:      PlanResultStreamExactAdmittedInputBytes, Stderr: PlanResultStreamEmpty,
			ExitStatus: PlanResultExitStatusZero, Framing: PlanResultFramingNone, Projection: PlanResultProjectionNone,
			Delivery: PlanResultDeliveryBufferedAfterCompletion, CrossStreamOrder: PlanResultCrossStreamOrderNotApplicable,
			StdoutLimitBytes: 4 * 1024 * 1024, StderrLimitBytes: 0,
			SourceProcessAttempts: 1, ProcessorProcessAttempts: 1,
		},
		{
			Disposition: PlanResultDispositionOptimized,
			Stdout:      PlanResultStreamValidatedOptimizerSummary, Stderr: PlanResultStreamEmpty,
			ExitStatus: PlanResultExitStatusZero, Framing: PlanResultFramingNone, Projection: PlanResultProjectionNone,
			Delivery: PlanResultDeliveryBufferedAfterCompletion, CrossStreamOrder: PlanResultCrossStreamOrderNotApplicable,
			StdoutLimitBytes: 4 * 1024 * 1024, StderrLimitBytes: 0,
			SourceProcessAttempts: 1, ProcessorProcessAttempts: 1,
		},
	}
	if !reflect.DeepEqual(modes[2].SuccessVariants, want) {
		t.Fatalf("optimizer success variants = %+v, want %+v", modes[2].SuccessVariants, want)
	}
	for index, mode := range modes[:2] {
		if len(mode.SuccessVariants) != 1 || mode.SuccessVariants[0].Disposition != PlanResultDispositionNotApplicable ||
			mode.SuccessVariants[0].SourceProcessAttempts != 1 || mode.SuccessVariants[0].ProcessorProcessAttempts != 0 {
			t.Fatalf("non-optimizer mode %d = %+v", index, mode)
		}
	}
}

func TestWrapperCatalogDeclaresCompleteFacadeFaultInventories(t *testing.T) {
	render, _ := DefaultCatalog().Lookup("wrapper render")
	run, _ := DefaultCatalog().Lookup("wrapper run")
	wantRender := []string{
		"invalid_arguments", "bundle_file_not_found", "bundle_file_permission_denied", "unsafe_bundle_file", "bundle_file_too_large", "bundle_file_read_failed", "invalid_bundle_file", "legacy_tailoring_schema", "bundle_digest_mismatch",
		"invalid_wrapper_binding", "wrapper_platform_not_supported", "invalid_bundle_trust_store", "bundle_not_adopted", "bundle_source_drift", "source_executable_not_found", "source_identity_unavailable", "unsafe_source_executable", "source_identity_changed", "invalid_source_identity",
		"invalid_processor_executable", "unsafe_processor_executable", "processor_identity_unavailable", "processor_identity_changed", "invalid_processor_identity", "bundle_processor_drift",
		"wrapper_runtime_not_supported", "wrapper_runtime_unavailable", "wrapper_render_failed", "output_contract_exceeded", "output_encoding_failed", "internal_error", "output_write_failed", "operation_canceled",
	}
	assertCommandErrorCodes(t, render.Agent.Errors, wantRender)

	wantRun := make([]string, 0, len(bundleExecuteErrors())+4)
	for _, declared := range bundleExecuteErrors() {
		wantRun = append(wantRun, declared.Code)
	}
	wantRun = append(wantRun,
		"invalid_wrapper_binding", "wrapper_runtime_unavailable", "wrapper_runtime_drift", "bundle_binding_mismatch",
		"processor_identity_unavailable", "processor_identity_unavailable_after_source", "processor_identity_changed", "invalid_processor_executable", "unsafe_processor_executable", "invalid_processor_identity", "invalid_processor_process_request",
		"processor_environment_setup_failed_after_source", "processor_process_start_failed_after_source", "processor_stdout_too_large", "processor_stderr_too_large", "processor_execution_canceled", "processor_timeout", "processor_command_failed", "processor_process_wait_failed", "processor_cleanup_failed", "processor_output_not_admitted", "unclassified_processor_execution_outcome",
	)
	assertCommandErrorCodes(t, run.Agent.Errors, wantRun)
}

func TestWrapperRunModeNeutralRecoveriesMatchScopedHelp(t *testing.T) {
	declarations := exactRecoveryHelp(t, "wrapper run")
	wantRuntime := fault.NextAction{Command: "help wrapper run", Reason: "Review the supported generated-wrapper runtime contract."}
	wantResult := wrapperResultRecovery()
	for code, want := range map[string]fault.NextAction{
		"wrapper_runtime_not_supported": wantRuntime,
		"output_contract_exceeded":      wantResult,
	} {
		declared, ok := declarations[code]
		if !ok || len(declared.NextActions) != 1 || declared.NextActions[0] != want {
			t.Fatalf("%s declaration=%+v want recovery=%+v", code, declared, want)
		}
	}

	binding := testWrapperRenderResult(t).Binding.RuntimeInvocation()
	malformed := testWrapperSourceStreamResult([]byte("secret stdout"), []byte("secret stderr"), 0, tailoringbundle.WrapperIdentity)
	malformed.SourceProcessAttempts = 0
	stub := &cliWrapperRunStub{result: malformed}
	var stdout, stderr bytes.Buffer
	command := New(strings.NewReader(""), &stdout, &stderr)
	command.wrapperRuns = stub
	args := append([]string{"--error-format=json"}, wrapperRunInvocation(binding, "pr", "list")...)
	exit := command.RunContext(context.Background(), args)
	assertRecoveryObservation(t, "wrapper run", declarations["output_contract_exceeded"], recoveryObservation{
		stdout: stdout.Bytes(), stderr: stderr.Bytes(), exitCode: exit, attempts: 0,
	}, 0)
}

func assertCommandErrorCodes(t *testing.T, declared []CommandError, want []string) {
	t.Helper()
	gotSet := make(map[string]struct{}, len(declared))
	for _, current := range declared {
		gotSet[current.Code] = struct{}{}
	}
	wantSet := make(map[string]struct{}, len(want))
	for _, current := range want {
		wantSet[current] = struct{}{}
	}
	if !reflect.DeepEqual(gotSet, wantSet) || len(declared) != len(wantSet) {
		t.Fatalf("fault codes=%v want=%v declared=%d", gotSet, wantSet, len(declared))
	}
}

func TestWrapperScopedAgentHelpPinsDynamicFramingAndHasNoHostKeys(t *testing.T) {
	var stdout, stderr bytes.Buffer
	command := New(strings.NewReader(""), &stdout, &stderr)
	if code := command.RunContext(context.Background(), []string{"help", "wrapper", "run", "--format=agent"}); code != ExitOK {
		t.Fatalf("code=%d stderr=%q", code, stderr.String())
	}
	var document map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &document); err != nil {
		t.Fatal(err)
	}
	encoded := stdout.String()
	for _, required := range []string{`"authority":"fresh_wrapper_plan"`, `"plan_result_modes"`, `"mode":"transformed_json"`, `"mode":"source_stream_passthrough"`, `"mode":"original_preserving_optimizer"`, `"disposition":"preserved_before_processor"`, `"disposition":"preserved_after_processor"`, `"disposition":"optimized"`, `"command":"bundle preview"`, `"id":"wrapper-plan"`, `"version":` + strconv.Itoa(tailoringplan.SchemaVersion)} {
		if !strings.Contains(encoded, required) {
			t.Errorf("scoped help lacks %s: %s", required, encoded)
		}
	}
	forbidden := map[string]struct{}{"host": {}, "hook": {}, "permission": {}, "settings": {}, "session": {}, "transcript": {}, "model": {}, "claude": {}, "codex": {}}
	var walk func(any)
	walk = func(value any) {
		switch current := value.(type) {
		case map[string]any:
			for key, nested := range current {
				if _, exists := forbidden[strings.ToLower(key)]; exists {
					t.Errorf("host-specific key %q in scoped help", key)
				}
				walk(nested)
			}
		case []any:
			for _, nested := range current {
				walk(nested)
			}
		}
	}
	walk(document)
}

func TestWrapperRenderRawAndJSONOutputsDescribeIdenticalMaterial(t *testing.T) {
	result := testWrapperRenderResult(t)
	rawStub := &cliWrapperRenderStub{result: result}
	var rawOut, rawErr bytes.Buffer
	raw := New(strings.NewReader(""), &rawOut, &rawErr)
	raw.wrapperRenders = rawStub
	if code := raw.RunContext(context.Background(), []string{"wrapper", "render", "--bundle", result.Binding.BundleLocator}); code != ExitOK {
		t.Fatalf("raw code=%d stderr=%q", code, rawErr.String())
	}
	if !bytes.Equal(rawOut.Bytes(), result.Material.Source) || rawErr.Len() != 0 || rawStub.path != result.Binding.BundleLocator {
		t.Fatalf("raw output=%q stderr=%q path=%q", rawOut.String(), rawErr.String(), rawStub.path)
	}

	jsonStub := &cliWrapperRenderStub{result: result}
	var jsonOut, jsonErr bytes.Buffer
	review := New(strings.NewReader(""), &jsonOut, &jsonErr)
	review.wrapperRenders = jsonStub
	if code := review.RunContext(context.Background(), []string{"wrapper", "render", "--bundle", result.Binding.BundleLocator, "--format=json"}); code != ExitOK {
		t.Fatalf("JSON code=%d stderr=%q", code, jsonErr.String())
	}
	var top map[string]json.RawMessage
	if err := json.Unmarshal(jsonOut.Bytes(), &top); err != nil {
		t.Fatal(err)
	}
	assertJSONKeys(t, top, []string{"schema_version", "wrapper"})
	var payload map[string]json.RawMessage
	if err := json.Unmarshal(top["wrapper"], &payload); err != nil {
		t.Fatal(err)
	}
	assertJSONKeys(t, payload, []string{"bundle", "command", "contract", "runtime", "source", "source_process_attempts", "processor_process_attempts", "source_sha256"})
	var contract, bundle, runtime map[string]json.RawMessage
	if err := json.Unmarshal(payload["contract"], &contract); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(payload["bundle"], &bundle); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(payload["runtime"], &runtime); err != nil {
		t.Fatal(err)
	}
	assertJSONKeys(t, contract, []string{"shell", "version"})
	assertJSONKeys(t, bundle, []string{"digest", "locator"})
	assertJSONKeys(t, runtime, []string{"resolved_path", "sha256", "size"})
	var source, digest, commandName string
	var sourceAttempts, processorAttempts int
	if err := json.Unmarshal(payload["source"], &source); err != nil {
		t.Fatal(err)
	}
	_ = json.Unmarshal(payload["source_sha256"], &digest)
	_ = json.Unmarshal(payload["command"], &commandName)
	_ = json.Unmarshal(payload["source_process_attempts"], &sourceAttempts)
	_ = json.Unmarshal(payload["processor_process_attempts"], &processorAttempts)
	if source != string(rawOut.Bytes()) || digest != result.Material.SHA256 || commandName != "gh" || sourceAttempts != 0 || processorAttempts != 0 || jsonErr.Len() != 0 {
		t.Fatalf("review source/digest/command/attempts = %q %q %q %d/%d", source, digest, commandName, sourceAttempts, processorAttempts)
	}
}

func TestWrapperRunAcceptsZeroArgvAndEmitsOneProjectedJSONValueLF(t *testing.T) {
	render := testWrapperRenderResult(t)
	binding := render.Binding.RuntimeInvocation()
	stub := &cliWrapperRunStub{result: testWrapperRunResult(tailoring.ResultShapeObject)}
	var stdout, stderr bytes.Buffer
	command := New(strings.NewReader(""), &stdout, &stderr)
	command.wrapperRuns = stub
	if code := command.RunContext(context.Background(), wrapperRunInvocation(binding)); code != ExitOK {
		t.Fatalf("code=%d stderr=%q", code, stderr.String())
	}
	want, err := json.Marshal(struct {
		Name string `json:"name"`
	}{Name: `line\n sep:\u2028 bidi:\u202E slash:\\`})
	if err != nil {
		t.Fatal(err)
	}
	want = append(want, '\n')
	if !bytes.Equal(stdout.Bytes(), want) || stderr.Len() != 0 || stub.calls != 1 || len(stub.args) != 0 || stub.binding != binding {
		t.Fatalf("stdout=%q want=%q stderr=%q calls=%d args=%q binding=%+v", stdout.String(), want, stderr.String(), stub.calls, stub.args, stub.binding)
	}
	if bytes.Count(stdout.Bytes(), []byte{'\n'}) != 1 || !json.Valid(bytes.TrimSuffix(stdout.Bytes(), []byte{'\n'})) {
		t.Fatalf("wrapper framing=%q", stdout.Bytes())
	}
}

func TestWrapperRunEmitsExactSourceStreamsInFixedFinalOrderAndReturnsSourceStatus(t *testing.T) {
	binding := testWrapperRenderResult(t).Binding.RuntimeInvocation()
	wantStdout := []byte{'o', 0, 0xff, '\n'}
	wantStderr := []byte{'e', 0xfe}
	stub := &cliWrapperRunStub{result: testWrapperSourceStreamResult(wantStdout, wantStderr, 23, tailoringbundle.WrapperTransform)}
	order := []string{}
	stdout := &orderedWriter{label: "stdout", order: &order}
	stderr := &orderedWriter{label: "stderr", order: &order}
	command := New(strings.NewReader(""), stdout, stderr)
	command.wrapperRuns = stub
	if code := command.RunContext(context.Background(), wrapperRunInvocation(binding, "issue", "list")); code != 23 {
		t.Fatalf("code=%d stdout=%x stderr=%x", code, stdout.buffer.Bytes(), stderr.buffer.Bytes())
	}
	if !bytes.Equal(stdout.buffer.Bytes(), wantStdout) || !bytes.Equal(stderr.buffer.Bytes(), wantStderr) || !reflect.DeepEqual(order, []string{"stdout", "stderr"}) || stub.calls != 1 {
		t.Fatalf("stdout=%x stderr=%x order=%v calls=%d", stdout.buffer.Bytes(), stderr.buffer.Bytes(), order, stub.calls)
	}
}

func TestWrapperRunSourceStreamAllowsZeroStatusWithNonemptyStderr(t *testing.T) {
	binding := testWrapperRenderResult(t).Binding.RuntimeInvocation()
	stub := &cliWrapperRunStub{result: testWrapperSourceStreamResult([]byte("ok"), []byte("warning\n"), 0, tailoringbundle.WrapperIdentity)}
	var stdout, stderr bytes.Buffer
	command := New(strings.NewReader(""), &stdout, &stderr)
	command.wrapperRuns = stub
	if code := command.RunContext(context.Background(), wrapperRunInvocation(binding, "pr", "list")); code != ExitOK {
		t.Fatalf("code=%d stdout=%q stderr=%q", code, stdout.Bytes(), stderr.Bytes())
	}
	if !bytes.Equal(stdout.Bytes(), []byte("ok")) || !bytes.Equal(stderr.Bytes(), []byte("warning\n")) {
		t.Fatalf("stdout=%q stderr=%q", stdout.Bytes(), stderr.Bytes())
	}
}

func TestWrapperRunEmitsExactOptimizerSuccessVariants(t *testing.T) {
	binding := testWrapperRenderResult(t).Binding.RuntimeInvocation()
	tests := []struct {
		name              string
		stdout            []byte
		stderr            []byte
		status            int
		disposition       planapply.OptimizerDisposition
		processorAttempts int
	}{
		{
			name: "preserved before processor", stdout: []byte{'{', 0, 0xff, '}', '\n'}, stderr: []byte("source warning\n"), status: 23,
			disposition: planapply.OptimizerPreservedBeforeProcessor, processorAttempts: 0,
		},
		{
			name: "preserved after processor", stdout: []byte("exact admitted input\n"), status: 0,
			disposition: planapply.OptimizerPreservedAfterProcessor, processorAttempts: 1,
		},
		{
			name: "optimized", stdout: []byte("Go test: 2 passed in 1 packages"), status: 0,
			disposition: planapply.OptimizerOptimized, processorAttempts: 1,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			stub := &cliWrapperRunStub{result: testWrapperOptimizerResult(test.stdout, test.stderr, test.status, test.disposition, test.processorAttempts)}
			order := []string{}
			stdout := &orderedWriter{label: "stdout", order: &order}
			stderr := &orderedWriter{label: "stderr", order: &order}
			command := New(strings.NewReader(""), stdout, stderr)
			command.wrapperRuns = stub
			if code := command.RunContext(context.Background(), wrapperRunInvocation(binding, "test")); code != test.status {
				t.Fatalf("code=%d stdout=%x stderr=%x", code, stdout.buffer.Bytes(), stderr.buffer.Bytes())
			}
			if !bytes.Equal(stdout.buffer.Bytes(), test.stdout) || !bytes.Equal(stderr.buffer.Bytes(), test.stderr) ||
				!reflect.DeepEqual(order, []string{"stdout", "stderr"}) || stub.calls != 1 {
				t.Fatalf("stdout=%x stderr=%x order=%v calls=%d", stdout.buffer.Bytes(), stderr.buffer.Bytes(), order, stub.calls)
			}
		})
	}
}

func TestWrapperRunRejectsMalformedOptimizerResultWithoutPublishingBytes(t *testing.T) {
	binding := testWrapperRenderResult(t).Binding.RuntimeInvocation()
	result := testWrapperOptimizerResult([]byte("ATSURA_MALFORMED_OPTIMIZER_STDOUT"), nil, 0, planapply.OptimizerOptimized, 0)
	stub := &cliWrapperRunStub{result: result}
	var stdout, stderr bytes.Buffer
	command := New(strings.NewReader(""), &stdout, &stderr)
	command.wrapperRuns = stub
	args := append([]string{"--error-format=json"}, wrapperRunInvocation(binding, "test")...)
	if code := command.RunContext(context.Background(), args); code != ExitContract || stdout.Len() != 0 || stub.calls != 1 {
		t.Fatalf("code=%d stdout=%q stderr=%q calls=%d", code, stdout.String(), stderr.String(), stub.calls)
	}
	if !strings.Contains(stderr.String(), `"code":"output_contract_exceeded"`) || bytes.Contains(stderr.Bytes(), result.Optimizer.Stdout) {
		t.Fatalf("stderr=%q", stderr.String())
	}
}

func TestWrapperRunProcessorFailurePublishesNoFallbackOrIntermediateBytes(t *testing.T) {
	binding := testWrapperRenderResult(t).Binding.RuntimeInvocation()
	result := testWrapperOptimizerResult([]byte("ATSURA_SOURCE_FALLBACK_SECRET"), []byte("ATSURA_PROCESSOR_STDERR_SECRET"), 0, planapply.OptimizerPreservedBeforeProcessor, 0)
	stub := &cliWrapperRunStub{result: result, err: fault.New(
		fault.KindRejected,
		"processor_command_failed",
		"The processor exited without an admitted result after the source completed.",
		false,
		fault.NextAction{Command: "bundle status", Reason: "Reconcile source-owned effects and processor state."},
	)}
	var stdout, stderr bytes.Buffer
	command := New(strings.NewReader(""), &stdout, &stderr)
	command.wrapperRuns = stub
	args := append([]string{"--error-format=json"}, wrapperRunInvocation(binding, "test")...)
	if code := command.RunContext(context.Background(), args); code != ExitRejected || stdout.Len() != 0 || stub.calls != 1 {
		t.Fatalf("code=%d stdout=%q stderr=%q calls=%d", code, stdout.String(), stderr.String(), stub.calls)
	}
	if !strings.Contains(stderr.String(), `"code":"processor_command_failed"`) || !strings.Contains(stderr.String(), `"retryable":false`) ||
		bytes.Contains(stderr.Bytes(), result.Optimizer.Stdout) || bytes.Contains(stderr.Bytes(), result.Optimizer.Stderr) {
		t.Fatalf("stderr=%q", stderr.String())
	}
}

func TestWrapperRunSourceStreamFinalWriteFailuresAreNonRetryable(t *testing.T) {
	binding := testWrapperRenderResult(t).Binding.RuntimeInvocation()
	sourceStdout := []byte("ATSURA_SOURCE_STREAM_SECRET_STDOUT")
	sourceStderr := []byte("ATSURA_SOURCE_STREAM_SECRET_STDERR")
	result := testWrapperSourceStreamResult(sourceStdout, sourceStderr, 23, tailoringbundle.WrapperIdentity)

	t.Run("stdout", func(t *testing.T) {
		stub := &cliWrapperRunStub{result: result}
		stdout := &partialFirstWriter{firstLimit: 7}
		var stderr bytes.Buffer
		command := New(strings.NewReader(""), stdout, &stderr)
		command.wrapperRuns = stub
		args := append([]string{"--error-format=json"}, wrapperRunInvocation(binding, "pr", "list")...)
		if code := command.RunContext(context.Background(), args); code != ExitInternal || stub.calls != 1 {
			t.Fatalf("code=%d calls=%d stderr=%q", code, stub.calls, stderr.String())
		}
		if !bytes.Equal(stdout.buffer.Bytes(), sourceStdout[:stdout.firstLimit]) || !strings.Contains(stderr.String(), `"code":"execute_output_write_failed"`) || !strings.Contains(stderr.String(), `"retryable":false`) || bytes.Contains(stderr.Bytes(), sourceStdout) || bytes.Contains(stderr.Bytes(), sourceStderr) {
			t.Fatalf("stdout=%q stderr=%q", stdout.buffer.Bytes(), stderr.String())
		}
	})

	t.Run("stderr after stdout", func(t *testing.T) {
		stub := &cliWrapperRunStub{result: result}
		var stdout bytes.Buffer
		stderr := &partialFirstWriter{firstLimit: 9}
		command := New(strings.NewReader(""), &stdout, stderr)
		command.wrapperRuns = stub
		args := append([]string{"--error-format=json"}, wrapperRunInvocation(binding, "pr", "list")...)
		if code := command.RunContext(context.Background(), args); code != ExitInternal || stub.calls != 1 {
			t.Fatalf("code=%d calls=%d stderr=%q", code, stub.calls, stderr.buffer.String())
		}
		if !bytes.Equal(stdout.Bytes(), sourceStdout) || !bytes.HasPrefix(stderr.buffer.Bytes(), sourceStderr[:stderr.firstLimit]) || !strings.Contains(stderr.buffer.String(), `"code":"execute_output_write_failed"`) || !strings.Contains(stderr.buffer.String(), `"retryable":false`) || bytes.Contains(stderr.buffer.Bytes(), sourceStderr[stderr.firstLimit:]) {
			t.Fatalf("stdout=%q stderr=%q", stdout.Bytes(), stderr.buffer.String())
		}
	})
}

func TestWrapperRunRequiresExplicitForwardingBoundary(t *testing.T) {
	binding := testWrapperRenderResult(t).Binding.RuntimeInvocation()
	stub := &cliWrapperRunStub{result: testWrapperRunResult(tailoring.ResultShapeObject)}
	var stdout, stderr bytes.Buffer
	command := New(strings.NewReader(""), &stdout, &stderr)
	command.wrapperRuns = stub
	args := wrapperRunInvocation(binding)
	args = append(args[:len(args)-1], "pr", "list")
	if code := command.RunContext(context.Background(), args); code != ExitUsage || stdout.Len() != 0 || stub.calls != 0 {
		t.Fatalf("code=%d stdout=%q stderr=%q calls=%d", code, stdout.String(), stderr.String(), stub.calls)
	}
	if !strings.Contains(stderr.String(), "requires the explicit -- boundary") {
		t.Fatalf("stderr=%q", stderr.String())
	}
}

func TestWrapperRunRejectsRetiredContractOneBeforeTheApplicationPort(t *testing.T) {
	binding := testWrapperRenderResult(t).Binding.RuntimeInvocation()
	stub := &cliWrapperRunStub{result: testWrapperRunResult(tailoring.ResultShapeObject)}
	var stdout, stderr bytes.Buffer
	command := New(strings.NewReader(""), &stdout, &stderr)
	command.wrapperRuns = stub
	args := wrapperRunInvocation(binding, "pr", "list")
	for index := range args {
		if strings.HasPrefix(args[index], "--contract-version=") {
			args[index] = "--contract-version=1"
		}
	}
	args = append([]string{"--error-format=json"}, args...)
	if code := command.RunContext(context.Background(), args); code != ExitUsage || stdout.Len() != 0 || stub.calls != 0 {
		t.Fatalf("code=%d stdout=%q stderr=%q calls=%d", code, stdout.String(), stderr.String(), stub.calls)
	}
	if !strings.Contains(stderr.String(), `"code":"invalid_arguments"`) || !strings.Contains(stderr.String(), "value must be one of 2") {
		t.Fatalf("stderr=%q", stderr.String())
	}
}

func TestWrapperRunPreservesArrayShapeAndExactArgv(t *testing.T) {
	binding := testWrapperRenderResult(t).Binding.RuntimeInvocation()
	stub := &cliWrapperRunStub{result: testWrapperRunResult(tailoring.ResultShapeArray)}
	var stdout, stderr bytes.Buffer
	command := New(strings.NewReader(""), &stdout, &stderr)
	command.wrapperRuns = stub
	argv := []string{"pr", "list", "", "two words", "--limit=1", "雪", "$(literal)"}
	if code := command.RunContext(context.Background(), wrapperRunInvocation(binding, argv...)); code != ExitOK {
		t.Fatalf("code=%d stderr=%q", code, stderr.String())
	}
	if !reflect.DeepEqual(stub.args, argv) || !strings.HasPrefix(stdout.String(), "[{\"name\":") || !strings.HasSuffix(stdout.String(), "}]\n") || strings.Contains(stdout.String(), "schema_version") || strings.Contains(stdout.String(), "execution") {
		t.Fatalf("args=%q stdout=%q", stub.args, stdout.String())
	}
}

func TestWrapperRunFailureUsesStructuredStderrAndNoStdout(t *testing.T) {
	binding := testWrapperRenderResult(t).Binding.RuntimeInvocation()
	stub := &cliWrapperRunStub{err: fault.New(
		fault.KindRejected,
		"wrapper_runtime_drift",
		"The current Atsura runtime does not match the exact generated wrapper binding.",
		false,
		fault.NextAction{Command: "wrapper render", Reason: "Render a new wrapper binding from the exact current bundle and Atsura runtime."},
	)}
	var stdout, stderr bytes.Buffer
	command := New(strings.NewReader(""), &stdout, &stderr)
	command.wrapperRuns = stub
	args := append([]string{"--error-format=json"}, wrapperRunInvocation(binding, "pr", "list")...)
	if code := command.RunContext(context.Background(), args); code != ExitRejected || stdout.Len() != 0 {
		t.Fatalf("code=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	var document map[string]json.RawMessage
	if err := json.Unmarshal(stderr.Bytes(), &document); err != nil {
		t.Fatal(err)
	}
	assertJSONKeys(t, document, []string{"error", "schema_version"})
	var public struct {
		Code      string `json:"code"`
		Retryable bool   `json:"retryable"`
	}
	if err := json.Unmarshal(document["error"], &public); err != nil {
		t.Fatal(err)
	}
	if public.Code != "wrapper_runtime_drift" || public.Retryable {
		t.Fatalf("structured fault=%+v", public)
	}
}

func TestWrapperRunFinalWriteFailureIsNonRetryable(t *testing.T) {
	binding := testWrapperRenderResult(t).Binding.RuntimeInvocation()
	stub := &cliWrapperRunStub{result: testWrapperRunResult(tailoring.ResultShapeObject)}
	var stderr bytes.Buffer
	command := New(strings.NewReader(""), shortWriter{}, &stderr)
	command.wrapperRuns = stub
	args := append([]string{"--error-format=json"}, wrapperRunInvocation(binding, "pr", "list")...)
	if code := command.RunContext(context.Background(), args); code != ExitInternal || stub.calls != 1 {
		t.Fatalf("code=%d calls=%d stderr=%q", code, stub.calls, stderr.String())
	}
	if !strings.Contains(stderr.String(), `"code":"execute_output_write_failed"`) || !strings.Contains(stderr.String(), `"retryable":false`) {
		t.Fatalf("stderr=%q", stderr.String())
	}
}

func TestEncodeWrapperPlanResultRejectsInvalidSuccessFraming(t *testing.T) {
	for name, mutate := range map[string]func(*wrapperrun.Result){
		"wrong render":   func(value *wrapperrun.Result) { value.TransformedJSON.Render = "pretty_json" },
		"zero attempts":  func(value *wrapperrun.Result) { value.SourceProcessAttempts = 0 },
		"invalid output": func(value *wrapperrun.Result) { value.TransformedJSON.Output.Records = nil },
	} {
		t.Run(name, func(t *testing.T) {
			candidate := testWrapperRunResult(tailoring.ResultShapeObject)
			mutate(&candidate)
			if encoded, err := encodeWrapperPlanResult(candidate.TransformedJSON.Output, candidate.TransformedJSON.Render, candidate.SourceProcessAttempts); err == nil || encoded != nil {
				t.Fatalf("encoded=%q err=%v", encoded, err)
			}
		})
	}
}
