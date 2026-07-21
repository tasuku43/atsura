package tailorrun

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/tasuku43/atsura/internal/domain/fault"
	"github.com/tasuku43/atsura/internal/domain/operation"
	"github.com/tasuku43/atsura/internal/domain/sourceprocess"
	"github.com/tasuku43/atsura/internal/domain/tailoring"
)

type fakeConfiguration struct {
	policy tailoring.Policy
	err    error
	calls  int
}

func (f *fakeConfiguration) Load(context.Context, string) (tailoring.Policy, error) {
	f.calls++
	return f.policy, f.err
}

type fakeProcess struct {
	result  sourceprocess.Result
	err     error
	calls   int
	request sourceprocess.Request
	after   func()
}

func (f *fakeProcess) Run(_ context.Context, request sourceprocess.Request) (sourceprocess.Result, error) {
	f.calls++
	f.request = request
	if f.after != nil {
		f.after()
	}
	return f.result, f.err
}

type fakeParser struct {
	value tailoring.JSONValue
	err   error
	calls int
	input []byte
}

func (f *fakeParser) Parse(_ context.Context, input []byte) (tailoring.JSONValue, error) {
	f.calls++
	f.input = append([]byte{}, input...)
	return f.value, f.err
}

func runIntent() operation.Intent {
	return operation.Intent{Command: "run", Effect: operation.EffectRead}
}

func runPolicy(decision tailoring.Decision) tailoring.Policy {
	return tailoring.Policy{
		SchemaVersion: 1, Effect: operation.EffectRead, Executable: "source", ArgsPrefix: []string{"list"},
		Decision: decision, Reason: "Return only identity and state.", AppendArgs: []string{"--json=id,state"},
		Output: tailoring.OutputPlan{Input: tailoring.InputJSON, Select: []string{"id", "state"}, Rename: []tailoring.Rename{{From: "id", To: "key"}}, Render: tailoring.RenderCompactJSON},
	}
}

func runIdentity() sourceprocess.Identity {
	return sourceprocess.Identity{ResolvedPath: "/synthetic/source", SHA256: "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef", Size: 42}
}

func parsedRecords() tailoring.JSONValue {
	return tailoring.NewJSONArray([]tailoring.JSONValue{tailoring.NewJSONObject([]tailoring.JSONField{
		{Name: "id", Value: tailoring.NewJSONNumber("1")},
		{Name: "state", Value: tailoring.NewJSONString("open")},
		{Name: "ignored", Value: tailoring.NewJSONBool(false)},
	})})
}

func successfulProcess() sourceprocess.Result {
	return sourceprocess.Result{Attempts: 1, ExitCode: 0, Stdout: []byte(`[{"id":1,"state":"open"}]`), Stderr: []byte("warning"), Identity: runIdentity()}
}

func faultCode(t *testing.T, err error) string {
	t.Helper()
	public, ok := fault.PublicCopy(err)
	if !ok {
		t.Fatalf("error is not a public fault: %v", err)
	}
	return public.Code
}

func TestRunCompilesExactArgvAndTransformsOneSuccessfulAttempt(t *testing.T) {
	config := &fakeConfiguration{policy: runPolicy(tailoring.DecisionAllow)}
	process := &fakeProcess{result: successfulProcess()}
	parser := &fakeParser{value: parsedRecords()}
	result, err := New(config, process, parser).Run(context.Background(), runIntent(), "policy.yaml", []string{"source", "list", "--open"})
	if err != nil {
		t.Fatal(err)
	}
	if config.calls != 1 || process.calls != 1 || parser.calls != 1 || result.SourceProcessAttempts != 1 {
		t.Fatalf("calls config=%d process=%d parser=%d result=%+v", config.calls, process.calls, parser.calls, result)
	}
	if process.request.Executable != "source" || !reflect.DeepEqual(process.request.Args, []string{"list", "--open", "--json=id,state"}) || process.request.Timeout != SourceTimeout {
		t.Fatalf("request = %+v", process.request)
	}
	if !reflect.DeepEqual(result.Output.Fields, []string{"key", "state"}) || result.Output.Records[0].ObjectValue[0].Name != "key" || string(result.SourceStderr) != "warning" {
		t.Fatalf("result = %+v", result)
	}
}

func TestRunRejectsPolicyAndMismatchBeforeProcess(t *testing.T) {
	tests := []struct {
		name   string
		policy tailoring.Policy
		argv   []string
		code   string
	}{
		{name: "deny", policy: runPolicy(tailoring.DecisionDeny), argv: []string{"source", "list"}, code: "policy_rejected"},
		{name: "mismatch", policy: runPolicy(tailoring.DecisionAllow), argv: []string{"other", "list"}, code: "plan_rule_not_matched"},
		{name: "create effect", policy: func() tailoring.Policy {
			value := runPolicy(tailoring.DecisionAllow)
			value.Effect = operation.EffectCreate
			return value
		}(), argv: []string{"source", "list"}, code: "invalid_plan_configuration"},
		{name: "missing effect", policy: func() tailoring.Policy {
			value := runPolicy(tailoring.DecisionAllow)
			value.Effect = operation.EffectUnknown
			return value
		}(), argv: []string{"source", "list"}, code: "invalid_plan_configuration"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			config := &fakeConfiguration{policy: test.policy}
			process := &fakeProcess{}
			parser := &fakeParser{}
			_, err := New(config, process, parser).Run(context.Background(), runIntent(), "policy.yaml", test.argv)
			if got := faultCode(t, err); got != test.code || process.calls != 0 || parser.calls != 0 {
				t.Fatalf("code=%q process=%d parser=%d", got, process.calls, parser.calls)
			}
		})
	}
}

func TestRunPreservesProcessFaultAndSkipsParser(t *testing.T) {
	processFault := fault.New(fault.KindRejected, "source_command_failed", "The source process exited without a successful result.", false, helpAction())
	process := &fakeProcess{result: sourceprocess.Result{Attempts: 1, ExitCode: 7, Identity: runIdentity()}, err: processFault}
	parser := &fakeParser{}
	_, err := New(&fakeConfiguration{policy: runPolicy(tailoring.DecisionAllow)}, process, parser).Run(context.Background(), runIntent(), "policy.yaml", []string{"source", "list"})
	if got := faultCode(t, err); got != "source_command_failed" || process.calls != 1 || parser.calls != 0 {
		t.Fatalf("code=%q process=%d parser=%d", got, process.calls, parser.calls)
	}
}

func TestRunRejectsInvalidProcessParserAndTransformResults(t *testing.T) {
	tests := []struct {
		name    string
		process *fakeProcess
		parser  *fakeParser
		code    string
	}{
		{name: "invalid process", process: &fakeProcess{result: sourceprocess.Result{Attempts: 2, ExitCode: 0}}, parser: &fakeParser{}, code: "invalid_source_process_result"},
		{name: "parser", process: &fakeProcess{result: successfulProcess()}, parser: &fakeParser{err: errors.New("private parser detail")}, code: "source_json_invalid"},
		{name: "transform", process: &fakeProcess{result: successfulProcess()}, parser: &fakeParser{value: tailoring.NewJSONObject([]tailoring.JSONField{{Name: "id", Value: tailoring.NewJSONNumber("1")}})}, code: "output_transform_failed"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := New(&fakeConfiguration{policy: runPolicy(tailoring.DecisionAllow)}, test.process, test.parser).Run(context.Background(), runIntent(), "policy.yaml", []string{"source", "list"})
			if got := faultCode(t, err); got != test.code || test.process.calls != 1 {
				t.Fatalf("code=%q process=%d parser=%d", got, test.process.calls, test.parser.calls)
			}
		})
	}
}

func TestRunFailsClosedForCancellationAndTypedNilPorts(t *testing.T) {
	config := &fakeConfiguration{policy: runPolicy(tailoring.DecisionAllow)}
	if _, err := New(config, (*fakeProcess)(nil), &fakeParser{}).Run(context.Background(), runIntent(), "x", []string{"source", "list"}); err == nil {
		t.Fatal("typed nil process succeeded")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	process := &fakeProcess{}
	if _, err := New(config, process, &fakeParser{}).Run(ctx, runIntent(), "x", []string{"source", "list"}); !errors.Is(err, context.Canceled) || process.calls != 0 {
		t.Fatalf("canceled error=%v process=%d", err, process.calls)
	}

	ctx, cancel = context.WithCancel(context.Background())
	process = &fakeProcess{result: successfulProcess(), after: cancel}
	parser := &fakeParser{value: parsedRecords()}
	if _, err := New(config, process, parser).Run(ctx, runIntent(), "x", []string{"source", "list"}); !errors.Is(err, context.Canceled) || parser.calls != 0 {
		t.Fatalf("mid-run cancel error=%v parser=%d", err, parser.calls)
	}
}
