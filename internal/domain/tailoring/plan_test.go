package tailoring

import (
	"errors"
	"reflect"
	"testing"
)

func TestCompileBuildsDeterministicAllowPlan(t *testing.T) {
	policy := validPolicy()
	invocation := Invocation{Argv: []string{"gh", "pr", "list", "--state", "open"}}

	first, err := Compile(policy, invocation)
	if err != nil {
		t.Fatal(err)
	}
	second, err := Compile(policy, invocation)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(first, second) {
		t.Fatalf("plans differ:\nfirst:  %+v\nsecond: %+v", first, second)
	}
	if !first.Executable || first.Decision != DecisionAllow {
		t.Fatalf("decision = %q, executable = %t", first.Decision, first.Executable)
	}
	wantOriginal := []string{"gh", "pr", "list", "--state", "open"}
	wantTransformed := []string{"gh", "pr", "list", "--state", "open", "--json=number,title,state"}
	if !reflect.DeepEqual(first.OriginalArgv, wantOriginal) || !reflect.DeepEqual(first.TransformedArgv, wantTransformed) {
		t.Fatalf("argv = original %v, transformed %v", first.OriginalArgv, first.TransformedArgv)
	}
	if first.MatchedCommand != "gh pr list" || first.SourceExecutable != "gh" || first.SourceProcessAttempts != 0 {
		t.Fatalf("identity or attempts = %+v", first)
	}
	if !reflect.DeepEqual(first.Output.Rename, []Rename{{From: "number", To: "id"}}) {
		t.Fatalf("output rename = %+v", first.Output.Rename)
	}
}

func TestCompileDenyHasNoExecutableInvocation(t *testing.T) {
	policy := validPolicy()
	policy.Decision = DecisionDeny
	plan, err := Compile(policy, Invocation{Argv: []string{"gh", "pr", "list"}})
	if err != nil {
		t.Fatal(err)
	}
	if plan.Executable || plan.Decision != DecisionDeny {
		t.Fatalf("decision = %q, executable = %t", plan.Decision, plan.Executable)
	}
	if plan.TransformedArgv == nil || len(plan.TransformedArgv) != 0 {
		t.Fatalf("deny transformed argv = %#v, want explicit empty", plan.TransformedArgv)
	}
}

func TestCompileRejectsInvocationMismatch(t *testing.T) {
	policy := validPolicy()
	for _, argv := range [][]string{
		{"glab", "pr", "list"},
		{"gh", "issue", "list"},
		{"gh", "pr"},
	} {
		if _, err := Compile(policy, Invocation{Argv: argv}); !errors.Is(err, ErrNoMatch) {
			t.Fatalf("Compile(%v) error = %v, want ErrNoMatch", argv, err)
		}
	}
}

func TestPolicyValidationFailsClosed(t *testing.T) {
	tests := map[string]func(*Policy){
		"schema":           func(policy *Policy) { policy.SchemaVersion = 2 },
		"executable":       func(policy *Policy) { policy.Executable = "" },
		"prefix nil":       func(policy *Policy) { policy.ArgsPrefix = nil },
		"decision":         func(policy *Policy) { policy.Decision = "confirm" },
		"reason":           func(policy *Policy) { policy.Reason = "" },
		"append nil":       func(policy *Policy) { policy.AppendArgs = nil },
		"input":            func(policy *Policy) { policy.Output.Input = "text" },
		"select nil":       func(policy *Policy) { policy.Output.Select = nil },
		"select duplicate": func(policy *Policy) { policy.Output.Select = []string{"number", "number"} },
		"rename nil":       func(policy *Policy) { policy.Output.Rename = nil },
		"rename missing":   func(policy *Policy) { policy.Output.Rename = []Rename{{From: "missing", To: "id"}} },
		"rename collision": func(policy *Policy) { policy.Output.Rename = []Rename{{From: "number", To: "title"}} },
		"render":           func(policy *Policy) { policy.Output.Render = "table" },
		"unsafe arg":       func(policy *Policy) { policy.AppendArgs = []string{"ok\nnot-ok"} },
	}
	for name, mutate := range tests {
		t.Run(name, func(t *testing.T) {
			policy := validPolicy()
			mutate(&policy)
			if err := policy.Validate(); !errors.Is(err, ErrInvalidPolicy) {
				t.Fatalf("Validate() error = %v, want ErrInvalidPolicy", err)
			}
		})
	}
}

func TestCompileReturnsDetachedPlan(t *testing.T) {
	policy := validPolicy()
	invocation := Invocation{Argv: []string{"gh", "pr", "list"}}
	plan, err := Compile(policy, invocation)
	if err != nil {
		t.Fatal(err)
	}

	policy.AppendArgs[0] = "changed"
	policy.Output.Select[0] = "changed"
	policy.Output.Rename[0].To = "changed"
	invocation.Argv[0] = "changed"

	if plan.OriginalArgv[0] != "gh" || plan.TransformedArgv[len(plan.TransformedArgv)-1] != "--json=number,title,state" ||
		plan.Output.Select[0] != "number" || plan.Output.Rename[0].To != "id" {
		t.Fatalf("plan aliases input: %+v", plan)
	}
}

func validPolicy() Policy {
	return Policy{
		SchemaVersion: 1,
		Executable:    "gh",
		ArgsPrefix:    []string{"pr", "list"},
		Decision:      DecisionAllow,
		Reason:        "Return only pull request identity and status.",
		AppendArgs:    []string{"--json=number,title,state"},
		Output: OutputPlan{
			Input:  InputJSON,
			Select: []string{"number", "title", "state"},
			Rename: []Rename{{From: "number", To: "id"}},
			Render: RenderCompactJSON,
		},
	}
}
