package bundleexecute

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/tasuku43/atsura/internal/app/planapply"
	"github.com/tasuku43/atsura/internal/app/processorcompat"
	"github.com/tasuku43/atsura/internal/domain/bundletrust"
	"github.com/tasuku43/atsura/internal/domain/fault"
	"github.com/tasuku43/atsura/internal/domain/operation"
	"github.com/tasuku43/atsura/internal/domain/processorprocess"
	"github.com/tasuku43/atsura/internal/domain/runtimeadmission"
	"github.com/tasuku43/atsura/internal/domain/sourcecatalog"
	"github.com/tasuku43/atsura/internal/domain/sourceprocess"
	"github.com/tasuku43/atsura/internal/domain/tailoring"
	"github.com/tasuku43/atsura/internal/domain/tailoringbundle"
	"github.com/tasuku43/atsura/internal/domain/tailoringplan"
)

const (
	runtimeMessageGeneric  = "The source adapter cannot prove this wrapper's runtime output contract."
	runtimeMessageAdapter  = "The wrapper plan's source adapter contract is not admitted by this runtime."
	runtimeMessageVersion  = "The wrapper plan's source version is not admitted by this runtime."
	runtimeMessageCommand  = "The wrapper plan's source command is not admitted by this runtime."
	runtimeMessageOutput   = "The wrapper plan does not declare the admitted transforming JSON output contract."
	runtimeMessageArgv     = "The wrapper plan's source arguments are outside the admitted command grammar."
	runtimeMessageSelector = "The wrapper plan does not contain exactly one admitted JSON selector matching its output fields."
)

type bundleStub struct {
	bundle tailoringbundle.Bundle
	digest string
	err    error
}

func (s *bundleStub) Load(context.Context, string) (tailoringbundle.Bundle, string, error) {
	return s.bundle, s.digest, s.err
}

type adoptionStub struct{ state bundletrust.State }

func (s *adoptionStub) Inspect(context.Context, string) bundletrust.State { return s.state }

type identityStub struct {
	value sourceprocess.Identity
	err   error
}

func (s *identityStub) Identify(context.Context, string) (sourceprocess.Identity, error) {
	return s.value, s.err
}

type compatibilityStub struct {
	err   error
	calls int
	plan  tailoringplan.Plan
}

type categorizedCompatibilityError struct {
	category runtimeadmission.Category
	hostile  string
}

func (e *categorizedCompatibilityError) Error() string { return e.hostile }

func (e *categorizedCompatibilityError) RuntimeAdmissionCategory() runtimeadmission.Category {
	return e.category
}

func (s *compatibilityStub) VerifyRuntime(plan tailoringplan.Plan) error {
	s.calls++
	s.plan = plan
	return s.err
}

type processStub struct {
	result  sourceprocess.Result
	err     error
	calls   int
	request sourceprocess.BoundRequest
}

func (s *processStub) RunBound(_ context.Context, request sourceprocess.BoundRequest) (sourceprocess.Result, error) {
	s.calls++
	s.request = request
	return s.result, s.err
}

type parserStub struct {
	value tailoring.JSONValue
	err   error
	calls int
	input []byte
}

type processorIdentityStub struct {
	value processorprocess.Identity
	calls int
}

func (s *processorIdentityStub) Identify(context.Context, string) (processorprocess.Identity, error) {
	s.calls++
	return s.value, nil
}

type processorProcessStub struct{ calls int }

func (s *processorProcessStub) Run(context.Context, processorprocess.Request) (processorprocess.Result, error) {
	s.calls++
	return processorprocess.Result{ExitCode: -1}, nil
}

type processorCompatibilityStub struct{ calls int }

func (s *processorCompatibilityStub) VerifyPlan(tailoringplan.Plan) error {
	s.calls++
	return nil
}

type optimizerAdmissionStub struct{ calls int }

func (s *optimizerAdmissionStub) ExpectedSummary([]byte) (string, bool) {
	s.calls++
	return "", false
}

func (s *parserStub) Parse(_ context.Context, input []byte) (tailoring.JSONValue, error) {
	s.calls++
	s.input = append([]byte{}, input...)
	return s.value, s.err
}

func executeIntent() operation.Intent {
	return operation.Intent{Command: Command, Effect: operation.EffectExecute}
}

func executeBundle(t *testing.T, transform bool) (tailoringbundle.Bundle, string, sourceprocess.Identity) {
	return executeBundleWithDefaults(t, transform, []tailoringbundle.OptionDefault{})
}

func executeBundleWithDefaults(t *testing.T, transform bool, defaults []tailoringbundle.OptionDefault) (tailoringbundle.Bundle, string, sourceprocess.Identity) {
	t.Helper()
	catalog := sourcecatalog.Catalog{
		SchemaVersion: sourcecatalog.SchemaVersion,
		Adapter:       sourcecatalog.Adapter{Kind: "atsura.source.alternate", ContractVersion: 1},
		Source:        sourcecatalog.Source{RequestedExecutable: "fixture", ResolvedPath: "/opt/bin/fixture", SHA256: strings.Repeat("a", 64), Size: 42, Version: "1.0.0"},
		Probe:         sourcecatalog.Probe{IDs: []string{"help"}, Attempts: 1},
		Commands: []sourcecatalog.Command{{
			Path: []string{"item", "list"}, Summary: "List items", Provenance: sourcecatalog.ProvenanceVerifiedBuiltin,
			Options: []sourcecatalog.Option{
				{Name: "--json", TakesValue: true}, {Name: "--limit", TakesValue: true},
			},
			StructuredOutput: []sourcecatalog.StructuredOutput{{Format: "json", SelectorFlag: "--json", Fields: []string{"id", "name", "state"}}},
		}},
	}
	catalogDigest, err := catalog.Digest()
	if err != nil {
		t.Fatal(err)
	}
	wrapper := &tailoringbundle.Wrapper{Kind: tailoringbundle.WrapperIdentity, Before: []tailoringbundle.StageAction{}, Invoke: tailoringbundle.Invocation{OptionDefaults: []tailoringbundle.OptionDefault{}, AppendArgs: []string{}}, After: []tailoringbundle.StageAction{}}
	if transform {
		wrapper = &tailoringbundle.Wrapper{
			Kind: tailoringbundle.WrapperTransform, Before: []tailoringbundle.StageAction{}, Invoke: tailoringbundle.Invocation{OptionDefaults: append([]tailoringbundle.OptionDefault{}, defaults...), AppendArgs: []string{"--json=id,name,state"}},
			Output: &tailoringbundle.Output{Kind: tailoringbundle.OutputKindProjection, Projection: &tailoringbundle.Projection{Input: "json", Select: []string{"id", "name", "state"}, Rename: []tailoringbundle.Rename{{From: "id", To: "item_id"}}, Render: "compact_json"}},
			After:  []tailoringbundle.StageAction{},
		}
	}
	specification := tailoringbundle.Specification{
		SchemaVersion: tailoringbundle.SpecificationSchemaVersion, CatalogDigest: catalogDigest,
		Surface: tailoringbundle.Surface{Default: tailoringbundle.SurfaceDefaultExclude},
		Commands: []tailoringbundle.CommandEntry{{
			Command: []string{"item", "list"}, Presence: tailoringbundle.PresenceInclude, Reason: "Return the compact inventory.",
			Options: &tailoringbundle.OptionSurface{Default: tailoringbundle.SurfaceDefaultInherit, Include: []string{}, Exclude: []string{}}, Wrapper: wrapper,
		}},
	}
	bundle, err := tailoringbundle.Compile(catalog, specification)
	if err != nil {
		t.Fatal(err)
	}
	digest, err := bundle.Digest()
	if err != nil {
		t.Fatal(err)
	}
	return bundle, digest, sourceprocess.Identity{ResolvedPath: catalog.Source.ResolvedPath, SHA256: catalog.Source.SHA256, Size: catalog.Source.Size}
}

func executeService(t *testing.T, compatibility *compatibilityStub, process *processStub, parser *parserStub) (*Service, tailoringplan.Attempt) {
	return executeServiceWithDefaults(t, compatibility, process, parser, []tailoringbundle.OptionDefault{})
}

func executeServiceWithDefaults(t *testing.T, compatibility *compatibilityStub, process *processStub, parser *parserStub, defaults []tailoringbundle.OptionDefault) (*Service, tailoringplan.Attempt) {
	t.Helper()
	bundle, digest, identity := executeBundleWithDefaults(t, true, defaults)
	applier := planapply.New(&bundleStub{bundle: bundle, digest: digest}, &adoptionStub{state: bundletrust.StateAdopted}, &identityStub{value: identity}, compatibility, process, parser)
	return NewWithApplier(applier), tailoringplan.Attempt{Executable: "fixture", Args: []string{"item", "list"}}
}

func optimizerExecuteBundle(t *testing.T) (tailoringbundle.Bundle, string, sourceprocess.Identity, processorprocess.Identity) {
	t.Helper()
	sourceIdentity := sourceprocess.Identity{ResolvedPath: "/opt/bin/go", SHA256: strings.Repeat("b", 64), Size: 1024}
	processorIdentity := processorprocess.Identity{
		ResolvedPath: "/opt/bin/rtk",
		SHA256:       "2dab449f32ea744c30b02a3ef9806e3e7d3b356a145332f3f2aaabb5ea48edee",
		Size:         7763408,
	}
	catalog := sourcecatalog.Catalog{
		SchemaVersion: sourcecatalog.SchemaVersion,
		Adapter:       sourcecatalog.Adapter{Kind: processorcompat.SourceAdapterKind, ContractVersion: processorcompat.SourceContractVersion},
		Source: sourcecatalog.Source{
			RequestedExecutable: "go", ResolvedPath: sourceIdentity.ResolvedPath, SHA256: sourceIdentity.SHA256, Size: sourceIdentity.Size, Version: "go1.26.5",
		},
		Probe: sourcecatalog.Probe{IDs: []string{"help", "test_help", "version"}, Attempts: 3},
		Commands: []sourcecatalog.Command{{
			Path: []string{"test"}, Summary: "test packages", Provenance: sourcecatalog.ProvenanceVerifiedBuiltin,
			Options: []sourcecatalog.Option{},
			StructuredOutput: []sourcecatalog.StructuredOutput{{
				Format: processorcompat.InputFormat, SelectorFlag: "-json",
				Fields: []string{"Action", "Elapsed", "FailedBuild", "Output", "Package", "Test", "Time"},
			}},
		}},
	}
	observation := processorprocess.Observation{
		SchemaVersion: processorprocess.ObservationSchemaVersion,
		Adapter:       processorprocess.Adapter{Kind: processorcompat.ProcessorAdapterKind, ContractVersion: processorcompat.ProcessorContractVersion},
		Platform:      processorprocess.Platform{OS: "darwin", Arch: "arm64"},
		Identity:      processorIdentity,
		Version:       processorcompat.ProcessorVersion,
		Probe: processorprocess.Probe{
			Argv: []string{"--version"}, EnvironmentContract: processorprocess.EnvironmentRTKIsolatedV2, Attempts: 1,
		},
	}
	registry := processorcompat.New()
	entry, err := registry.DefaultEntry(catalog, observation)
	if err != nil {
		t.Fatal(err)
	}
	binding, err := registry.Binding(observation)
	if err != nil {
		t.Fatal(err)
	}
	catalogDigest, err := catalog.Digest()
	if err != nil {
		t.Fatal(err)
	}
	bundle, err := tailoringbundle.Compile(catalog, tailoringbundle.Specification{
		SchemaVersion: tailoringbundle.SpecificationSchemaVersion,
		CatalogDigest: catalogDigest,
		Surface:       tailoringbundle.Surface{Default: tailoringbundle.SurfaceDefaultExclude},
		Commands:      []tailoringbundle.CommandEntry{entry},
	}, binding)
	if err != nil {
		t.Fatal(err)
	}
	digest, err := bundle.Digest()
	if err != nil {
		t.Fatal(err)
	}
	return bundle, digest, sourceIdentity, processorIdentity
}

func TestExecuteRebuildsPlanRunsBoundSourceOnceAndTransformsTypedJSON(t *testing.T) {
	compatibility := &compatibilityStub{}
	process := &processStub{result: sourceprocess.Result{Attempts: 1, ExitCode: 0, Stdout: []byte(`private raw bytes`), Identity: sourceprocess.Identity{ResolvedPath: "/opt/bin/fixture", SHA256: strings.Repeat("a", 64), Size: 42}}}
	parser := &parserStub{value: tailoring.NewJSONArray([]tailoring.JSONValue{
		tailoring.NewJSONObject([]tailoring.JSONField{
			{Name: "state", Value: tailoring.NewJSONString("OPEN")},
			{Name: "id", Value: tailoring.NewJSONNumber("0")},
			{Name: "name", Value: tailoring.NewJSONString("")},
		}),
	})}
	service, attempt := executeService(t, compatibility, process, parser)
	result, err := service.Execute(context.Background(), executeIntent(), "bundle.json", attempt)
	if err != nil {
		t.Fatal(err)
	}
	if compatibility.calls != 1 || process.calls != 1 || parser.calls != 1 || result.SourceProcessAttempts != 1 || result.ResultMode != tailoringplan.ResultModeTransformedJSON || result.TransformedJSON == nil || result.SourceStream != nil || result.TransformedJSON.ExitCode != 0 || len(result.PlanDigest) != 64 {
		t.Fatalf("result=%+v calls compatibility/process/parser=%d/%d/%d", result, compatibility.calls, process.calls, parser.calls)
	}
	if process.request.ExpectedIdentity != process.result.Identity || strings.Join(process.request.Process.Args, "\x00") != "item\x00list\x00--json=id,name,state" {
		t.Fatalf("bound request=%+v", process.request)
	}
	output := result.TransformedJSON.Output
	if output.Shape != tailoring.ResultShapeArray || strings.Join(output.Fields, ",") != "item_id,name,state" || output.Records[0].ObjectValue[0].Value.NumberValue != "0" || output.Records[0].ObjectValue[1].Value.StringValue != "" {
		t.Fatalf("output=%+v", output)
	}
	if err := result.Validate(); err != nil {
		t.Fatalf("result validation: %v", err)
	}
	if string(parser.input) != "private raw bytes" || compatibility.plan.SchemaVersion != tailoringplan.SchemaVersion {
		t.Fatalf("parser input=%q compatibility plan=%+v", parser.input, compatibility.plan)
	}
}

func TestExecuteAppliesDefaultOnceAndPreservesExplicitCallerOverride(t *testing.T) {
	tests := []struct {
		name        string
		callerArgs  []string
		wantArgs    []string
		wantApplied []tailoringbundle.OptionDefault
	}{
		{
			name: "omitted option", callerArgs: []string{"item", "list"},
			wantArgs:    []string{"item", "list", "--limit=30", "--json=id,name,state"},
			wantApplied: []tailoringbundle.OptionDefault{{Option: "--limit", Value: "30"}},
		},
		{
			name: "caller override", callerArgs: []string{"item", "list", "--limit=2"},
			wantArgs:    []string{"item", "list", "--limit=2", "--json=id,name,state"},
			wantApplied: []tailoringbundle.OptionDefault{},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			compatibility := &compatibilityStub{}
			identity := sourceprocess.Identity{ResolvedPath: "/opt/bin/fixture", SHA256: strings.Repeat("a", 64), Size: 42}
			process := &processStub{result: sourceprocess.Result{Attempts: 1, ExitCode: 0, Stdout: []byte(`private raw bytes`), Identity: identity}}
			parser := &parserStub{value: tailoring.NewJSONArray([]tailoring.JSONValue{
				tailoring.NewJSONObject([]tailoring.JSONField{
					{Name: "id", Value: tailoring.NewJSONNumber("1")},
					{Name: "name", Value: tailoring.NewJSONString("item")},
					{Name: "state", Value: tailoring.NewJSONString("OPEN")},
				}),
			})}
			service, attempt := executeServiceWithDefaults(
				t, compatibility, process, parser,
				[]tailoringbundle.OptionDefault{{Option: "--limit", Value: "30"}},
			)
			attempt.Args = test.callerArgs
			result, err := service.Execute(context.Background(), executeIntent(), "bundle.json", attempt)
			if err != nil {
				t.Fatal(err)
			}
			if result.SourceProcessAttempts != 1 || process.calls != 1 || parser.calls != 1 ||
				!reflect.DeepEqual(process.request.Process.Args, test.wantArgs) ||
				!reflect.DeepEqual(compatibility.plan.Stages.Invoke.AppliedOptionDefaults, test.wantApplied) {
				t.Fatalf("result=%+v request=%+v plan=%+v", result, process.request, compatibility.plan)
			}
		})
	}
}

func TestExecuteRejectsUnsupportedWrapperAndCompatibilityBeforeProcess(t *testing.T) {
	identityBundle, digest, identity := executeBundle(t, false)
	compatibilityIdentity := &compatibilityStub{}
	process := &processStub{}
	parserIdentity := &parserStub{}
	result, err := New(&bundleStub{bundle: identityBundle, digest: digest}, &adoptionStub{state: bundletrust.StateAdopted}, &identityStub{value: identity}, compatibilityIdentity, process, parserIdentity).Execute(context.Background(), executeIntent(), "bundle.json", tailoringplan.Attempt{Executable: "fixture", Args: []string{"item", "list"}})
	assertFault(t, err, "wrapper_runtime_not_supported", false)
	if compatibilityIdentity.calls != 0 || process.calls != 0 || parserIdentity.calls != 0 || result.SourceProcessAttempts != 0 || result.SourceStream != nil || result.TransformedJSON != nil {
		t.Fatalf("identity result=%+v compatibility/process/parser calls=%d/%d/%d", result, compatibilityIdentity.calls, process.calls, parserIdentity.calls)
	}

	compatibility := &compatibilityStub{err: errors.New("unproven selector")}
	service, attempt := executeService(t, compatibility, process, &parserStub{})
	_, err = service.Execute(context.Background(), executeIntent(), "bundle.json", attempt)
	public := assertFault(t, err, "wrapper_runtime_not_supported", false)
	if process.calls != 0 || public.Message != runtimeMessageGeneric || strings.Contains(err.Error(), "unproven selector") {
		t.Fatalf("unsupported adapter public=%+v error=%v process calls=%d", public, err, process.calls)
	}
}

func TestExecuteRejectsOptimizerBeforeSourceOrProcessorExecution(t *testing.T) {
	bundle, digest, sourceIdentity, processorIdentity := optimizerExecuteBundle(t)
	sourceCompatibility := &compatibilityStub{}
	sourceProcess := &processStub{}
	parser := &parserStub{}
	processorIdentityPort := &processorIdentityStub{value: processorIdentity}
	processorProcess := &processorProcessStub{}
	processorCompatibility := &processorCompatibilityStub{}
	admission := &optimizerAdmissionStub{}
	applier := planapply.New(
		&bundleStub{bundle: bundle, digest: digest},
		&adoptionStub{state: bundletrust.StateAdopted},
		&identityStub{value: sourceIdentity},
		sourceCompatibility,
		sourceProcess,
		parser,
		planapply.ProcessorSupport{
			Identity: processorIdentityPort, Processes: processorProcess, Compatibility: processorCompatibility, Admission: admission,
		},
	)

	result, err := NewWithApplier(applier).Execute(
		context.Background(), executeIntent(), "bundle.json", tailoringplan.Attempt{Executable: "go", Args: []string{"test"}},
	)
	public := assertFault(t, err, "wrapper_runtime_not_supported", false)
	if public.Kind != fault.KindUnsupported || len(public.NextActions) != 1 || public.NextActions[0].Command != "help bundle execute" {
		t.Fatalf("public = %+v", public)
	}
	if result.ResultMode != tailoringplan.ResultModeOriginalPreservingOptimizer || result.SourceProcessAttempts != 0 || result.ProcessorProcessAttempts != 0 || result.Optimizer != nil || result.SourceStream != nil || result.TransformedJSON != nil {
		t.Fatalf("result = %+v", result)
	}
	if sourceCompatibility.calls != 0 || sourceProcess.calls != 0 || parser.calls != 0 || processorIdentityPort.calls != 0 || processorProcess.calls != 0 || processorCompatibility.calls != 0 || admission.calls != 0 {
		t.Fatalf("calls source compatibility/process/parser=%d/%d/%d processor identity/process/compatibility/admission=%d/%d/%d/%d",
			sourceCompatibility.calls, sourceProcess.calls, parser.calls,
			processorIdentityPort.calls, processorProcess.calls, processorCompatibility.calls, admission.calls,
		)
	}
}

func TestExecuteMapsFiniteRuntimeAdmissionDiagnosticsBeforeProcess(t *testing.T) {
	tests := []struct {
		name     string
		category runtimeadmission.Category
		message  string
	}{
		{name: "adapter contract", category: runtimeadmission.CategoryAdapterContract, message: runtimeMessageAdapter},
		{name: "source version", category: runtimeadmission.CategorySourceVersion, message: runtimeMessageVersion},
		{name: "command", category: runtimeadmission.CategoryCommand, message: runtimeMessageCommand},
		{name: "wrapper output", category: runtimeadmission.CategoryWrapperOutput, message: runtimeMessageOutput},
		{name: "argv grammar", category: runtimeadmission.CategoryArgvGrammar, message: runtimeMessageArgv},
		{name: "selector conflict", category: runtimeadmission.CategorySelectorConflict, message: runtimeMessageSelector},
		{name: "unknown category", category: "hostile_unknown_category", message: runtimeMessageGeneric},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			hostile := "ATSURA_SECRET_RUNTIME_CAUSE_" + strings.ReplaceAll(string(test.category), "_", "-")
			compatibility := &compatibilityStub{err: &categorizedCompatibilityError{category: test.category, hostile: hostile}}
			process := &processStub{}
			parser := &parserStub{}
			service, attempt := executeService(t, compatibility, process, parser)
			_, err := service.Execute(context.Background(), executeIntent(), "bundle.json", attempt)
			public := assertFault(t, err, "wrapper_runtime_not_supported", false)
			if public.Kind != fault.KindUnsupported || public.Message != test.message || len(public.NextActions) != 1 || public.NextActions[0].Command != "help bundle execute" {
				t.Fatalf("public=%+v", public)
			}
			if compatibility.calls != 1 || process.calls != 0 || parser.calls != 0 {
				t.Fatalf("calls compatibility/process/parser=%d/%d/%d", compatibility.calls, process.calls, parser.calls)
			}
			if strings.Contains(err.Error(), hostile) || strings.Contains(public.Message, hostile) {
				t.Fatalf("hostile compatibility cause leaked: error=%v public=%+v", err, public)
			}
		})
	}
}

func TestExecuteClassifiesPostStartFailuresWithoutRawOutputOrRetry(t *testing.T) {
	identity := sourceprocess.Identity{ResolvedPath: "/opt/bin/fixture", SHA256: strings.Repeat("a", 64), Size: 42}
	tests := []struct {
		name   string
		result sourceprocess.Result
		err    error
		code   string
	}{
		{name: "known post start", result: sourceprocess.Result{Attempts: 1, ExitCode: 7, Stdout: []byte("secret stdout"), Stderr: []byte("secret stderr"), Identity: identity}, err: fault.New(fault.KindRejected, "source_command_failed", "hostile adapter message with secret stderr", false), code: "source_command_failed"},
		{name: "unknown post start", result: sourceprocess.Result{Attempts: 1, ExitCode: -1, Identity: identity}, err: errors.New("secret cause"), code: "unclassified_source_execution_outcome"},
		{name: "invalid success result", result: sourceprocess.Result{ExitCode: -1}, code: "unclassified_source_execution_outcome"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			process := &processStub{result: test.result, err: test.err}
			service, attempt := executeService(t, &compatibilityStub{}, process, &parserStub{})
			_, err := service.Execute(context.Background(), executeIntent(), "bundle.json", attempt)
			public := assertFault(t, err, test.code, false)
			if strings.Contains(public.Message, "secret") || process.calls != 1 {
				t.Fatalf("public=%+v calls=%d", public, process.calls)
			}
		})
	}
}

func TestExecuteRejectsStderrParserAndTransformFailuresAfterOneAttempt(t *testing.T) {
	identity := sourceprocess.Identity{ResolvedPath: "/opt/bin/fixture", SHA256: strings.Repeat("a", 64), Size: 42}
	tests := []struct {
		name   string
		result sourceprocess.Result
		parser *parserStub
		code   string
	}{
		{name: "stderr", result: sourceprocess.Result{Attempts: 1, ExitCode: 0, Stdout: []byte(`[]`), Stderr: []byte("warning secret"), Identity: identity}, parser: &parserStub{}, code: "source_stderr_not_supported"},
		{name: "invalid json", result: sourceprocess.Result{Attempts: 1, ExitCode: 0, Stdout: []byte(`bad`), Identity: identity}, parser: &parserStub{err: errors.New("malformed")}, code: "source_json_invalid"},
		{name: "missing field", result: sourceprocess.Result{Attempts: 1, ExitCode: 0, Stdout: []byte(`[]`), Identity: identity}, parser: &parserStub{value: tailoring.NewJSONArray([]tailoring.JSONValue{tailoring.NewJSONObject([]tailoring.JSONField{{Name: "id", Value: tailoring.NewJSONString("1")}})})}, code: "output_transform_failed"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			process := &processStub{result: test.result}
			service, attempt := executeService(t, &compatibilityStub{}, process, test.parser)
			_, err := service.Execute(context.Background(), executeIntent(), "bundle.json", attempt)
			assertFault(t, err, test.code, false)
			if process.calls != 1 || (test.name == "stderr" && test.parser.calls != 0) {
				t.Fatalf("process/parser calls=%d/%d", process.calls, test.parser.calls)
			}
		})
	}
}

func TestExecutePreflightFailuresStartZeroProcesses(t *testing.T) {
	bundle, digest, identity := executeBundle(t, true)
	tests := []struct {
		name     string
		bundle   *bundleStub
		adoption bundletrust.State
		identity sourceprocess.Identity
		attempt  tailoringplan.Attempt
		code     string
	}{
		{name: "not adopted", bundle: &bundleStub{bundle: bundle, digest: digest}, adoption: bundletrust.StateNotAdopted, identity: identity, attempt: tailoringplan.Attempt{Executable: "fixture", Args: []string{"item", "list"}}, code: "bundle_not_adopted"},
		{name: "drift", bundle: &bundleStub{bundle: bundle, digest: digest}, adoption: bundletrust.StateAdopted, identity: sourceprocess.Identity{ResolvedPath: identity.ResolvedPath, SHA256: strings.Repeat("b", 64), Size: identity.Size}, attempt: tailoringplan.Attempt{Executable: "fixture", Args: []string{"item", "list"}}, code: "bundle_source_drift"},
		{name: "wrong executable", bundle: &bundleStub{bundle: bundle, digest: digest}, adoption: bundletrust.StateAdopted, identity: identity, attempt: tailoringplan.Attempt{Executable: "other", Args: []string{"item", "list"}}, code: "source_executable_mismatch"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			process := &processStub{}
			_, err := New(test.bundle, &adoptionStub{state: test.adoption}, &identityStub{value: test.identity}, &compatibilityStub{}, process, &parserStub{}).Execute(context.Background(), executeIntent(), "bundle.json", test.attempt)
			assertFault(t, err, test.code, false)
			if process.calls != 0 {
				t.Fatalf("process calls=%d", process.calls)
			}
		})
	}
}

func assertFault(t *testing.T, err error, code string, retryable bool) *fault.Error {
	t.Helper()
	public, ok := fault.PublicCopy(err)
	if !ok || public.Code != code || public.Retryable != retryable {
		t.Fatalf("error=%v public=%+v want code=%s retryable=%t", err, public, code, retryable)
	}
	return public
}
