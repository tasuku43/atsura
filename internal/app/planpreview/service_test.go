package planpreview

import (
	"context"
	"errors"
	"testing"

	"github.com/tasuku43/atsura/internal/domain/fault"
	"github.com/tasuku43/atsura/internal/domain/operation"
	"github.com/tasuku43/atsura/internal/domain/tailoring"
)

type fakeLoader struct {
	policy tailoring.Policy
	err    error
	calls  int
	after  func()
}

func (f *fakeLoader) Load(context.Context, string) (tailoring.Policy, error) {
	f.calls++
	if f.after != nil {
		f.after()
	}
	return f.policy, f.err
}

func previewIntent() operation.Intent {
	return operation.Intent{Command: "plan preview", Effect: operation.EffectRead}
}

func validPolicy() tailoring.Policy {
	return tailoring.Policy{
		SchemaVersion: 1, Effect: operation.EffectRead, Executable: "gh", ArgsPrefix: []string{"pr", "list"},
		Decision: tailoring.DecisionAllow, Reason: "Keep the response focused.",
		AppendArgs: []string{"--json=number,title,state"},
		Output:     tailoring.OutputPlan{Input: tailoring.InputJSON, Select: []string{"number"}, Rename: []tailoring.Rename{}, Render: tailoring.RenderCompactJSON},
	}
}

func TestPreviewLoadsAndCompilesWithoutExecution(t *testing.T) {
	loader := &fakeLoader{policy: validPolicy()}
	plan, err := New(loader).Preview(context.Background(), previewIntent(), "policy.yaml", []string{"gh", "pr", "list"})
	if err != nil {
		t.Fatalf("Preview() error = %v", err)
	}
	if loader.calls != 1 || plan.SourceProcessAttempts != 0 || len(plan.TransformedArgv) != 4 {
		t.Fatalf("loader calls = %d, plan = %+v", loader.calls, plan)
	}
}

func TestPreviewFailsClosedBeforeOrDuringLoad(t *testing.T) {
	if _, err := New(nil).Preview(context.Background(), previewIntent(), "x", []string{"gh"}); err == nil {
		t.Fatal("nil loader succeeded")
	}
	if _, err := New((*fakeLoader)(nil)).Preview(context.Background(), previewIntent(), "x", []string{"gh"}); err == nil {
		t.Fatal("typed nil loader succeeded")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	loader := &fakeLoader{}
	if _, err := New(loader).Preview(ctx, previewIntent(), "x", []string{"gh"}); !errors.Is(err, context.Canceled) || loader.calls != 0 {
		t.Fatalf("canceled Preview() error = %v, calls = %d", err, loader.calls)
	}
	ctx, cancel = context.WithCancel(context.Background())
	loader = &fakeLoader{policy: validPolicy(), after: cancel}
	if _, err := New(loader).Preview(ctx, previewIntent(), "x", []string{"gh"}); !errors.Is(err, context.Canceled) {
		t.Fatalf("mid-load cancellation error = %v", err)
	}
}

func TestPreviewReturnsStableFaults(t *testing.T) {
	structured := fault.New(fault.KindNotFound, "plan_configuration_not_found", "The plan configuration was not found.", false, helpAction())
	tests := []struct {
		name   string
		loader *fakeLoader
		argv   []string
		code   string
	}{
		{name: "structured load fault", loader: &fakeLoader{err: structured}, argv: []string{"gh"}, code: "plan_configuration_not_found"},
		{name: "raw load fault", loader: &fakeLoader{err: errors.New("secret upstream detail")}, argv: []string{"gh"}, code: "internal_error"},
		{name: "rule mismatch", loader: &fakeLoader{policy: validPolicy()}, argv: []string{"git", "status"}, code: "plan_rule_not_matched"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := New(test.loader).Preview(context.Background(), previewIntent(), "x", test.argv)
			public, ok := fault.PublicCopy(err)
			if !ok || public.Code != test.code {
				t.Fatalf("Preview() error = %#v, code want %q", err, test.code)
			}
		})
	}
}
