package wrapperrun

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/tasuku43/atsura/internal/app/planapply"
	"github.com/tasuku43/atsura/internal/domain/fault"
	"github.com/tasuku43/atsura/internal/domain/operation"
	"github.com/tasuku43/atsura/internal/domain/sourceprocess"
	"github.com/tasuku43/atsura/internal/domain/wrapperbinding"
)

type currentExecutableStub struct {
	locator string
	err     error
	calls   int
}

func (s *currentExecutableStub) CurrentExecutable(context.Context) (string, error) {
	s.calls++
	return s.locator, s.err
}

type identityStub struct {
	value   sourceprocess.Identity
	err     error
	calls   int
	locator string
	after   func()
}

func (s *identityStub) Identify(_ context.Context, locator string) (sourceprocess.Identity, error) {
	s.calls++
	s.locator = locator
	if s.after != nil {
		s.after()
	}
	return s.value, s.err
}

type applierStub struct {
	result  planapply.Result
	err     error
	calls   int
	request planapply.Request
}

func (s *applierStub) Apply(_ context.Context, request planapply.Request) (planapply.Result, error) {
	s.calls++
	s.request = request
	return s.result, s.err
}

type typedNilCurrent struct{}

func (*typedNilCurrent) CurrentExecutable(context.Context) (string, error) {
	panic("typed nil current executable port must not be called")
}

type typedNilIdentity struct{}

func (*typedNilIdentity) Identify(context.Context, string) (sourceprocess.Identity, error) {
	panic("typed nil identity port must not be called")
}

type typedNilApplier struct{}

func (*typedNilApplier) Apply(context.Context, planapply.Request) (planapply.Result, error) {
	panic("typed nil applier must not be called")
}

func runtimeIdentity() sourceprocess.Identity {
	return sourceprocess.Identity{
		ResolvedPath: "/opt/atsura/bin/atr",
		SHA256:       strings.Repeat("b", 64),
		Size:         4242,
	}
}

func runtimeBinding() wrapperbinding.RuntimeInvocation {
	runtime := runtimeIdentity()
	return wrapperbinding.RuntimeInvocation{
		ContractVersion: wrapperbinding.ContractVersion,
		BundleLocator:   "/opt/atsura/bundles/purpose bundle.json",
		BundleDigest:    strings.Repeat("a", 64),
		Runtime: wrapperbinding.RuntimeIdentity{
			ResolvedPath: runtime.ResolvedPath,
			SHA256:       runtime.SHA256,
			Size:         runtime.Size,
		},
	}
}

func executeIntent() operation.Intent {
	return operation.Intent{Command: Command, Effect: operation.EffectExecute}
}

func TestExecuteVerifiesRuntimeAndForwardsExactArgvToBundleDerivedPlan(t *testing.T) {
	binding := runtimeBinding()
	current := &currentExecutableStub{locator: "/proc/current/atr"}
	identity := &identityStub{value: runtimeIdentity()}
	wantResult := planapply.Result{BundleDigest: binding.BundleDigest, PlanDigest: strings.Repeat("c", 64), MatchedCommand: []string{"item", "list"}, SourceProcessAttempts: 1}
	applier := &applierStub{result: wantResult}
	service := New(current, identity, applier)
	args := []string{"item", "list", "", "space value", "-dash", "Unicode 雪", "$(literal)", "--", "ordered"}
	wantArgs := append([]string{}, args...)

	result, err := service.Execute(context.Background(), executeIntent(), binding, args)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(result, wantResult) {
		t.Fatalf("result = %+v, want %+v", result, wantResult)
	}
	if current.calls != 1 || identity.calls != 1 || identity.locator != current.locator || applier.calls != 1 {
		t.Fatalf("current/identity/apply calls = %d/%d/%d, identity locator=%q", current.calls, identity.calls, applier.calls, identity.locator)
	}
	request := applier.request
	if request.BundlePath != binding.BundleLocator || request.ExpectedBundleDigest != binding.BundleDigest || !request.DeriveExecutableFromLoadedBundle || !request.AllowSourceStreamPassthrough {
		t.Fatalf("plan application binding = %+v", request)
	}
	if request.Attempt.Executable != "" || !reflect.DeepEqual(request.Attempt.Args, wantArgs) {
		t.Fatalf("plan application attempt = %+v, want exact argv %#v", request.Attempt, wantArgs)
	}
	if err := request.Command.Validate(); err != nil {
		t.Fatalf("wrapper command context = %+v: %v", request.Command, err)
	}
	if request.Command.RuntimeHelpAction.Command != "help wrapper run" || request.Command.BundleMismatchAction.Command != "wrapper render" {
		t.Fatalf("wrapper recovery context = %+v", request.Command)
	}
	args[0] = "mutated-after-execute"
	if request.Attempt.Args[0] != "item" {
		t.Fatalf("forwarded argv aliases caller storage: %#v", request.Attempt.Args)
	}
}

func TestExecuteRejectsRuntimeIdentityDriftBeforePlanApplication(t *testing.T) {
	wanted := runtimeIdentity()
	tests := []struct {
		name   string
		mutate func(*sourceprocess.Identity)
	}{
		{name: "path", mutate: func(value *sourceprocess.Identity) { value.ResolvedPath = "/opt/atsura/bin/other-atr" }},
		{name: "digest", mutate: func(value *sourceprocess.Identity) { value.SHA256 = strings.Repeat("c", 64) }},
		{name: "size", mutate: func(value *sourceprocess.Identity) { value.Size++ }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			currentIdentity := wanted
			test.mutate(&currentIdentity)
			current := &currentExecutableStub{locator: "/proc/current/atr"}
			identity := &identityStub{value: currentIdentity}
			applier := &applierStub{}

			result, err := New(current, identity, applier).Execute(context.Background(), executeIntent(), runtimeBinding(), []string{"item", "list"})
			public := assertFault(t, err, fault.KindRejected, "wrapper_runtime_drift")
			assertRenderRecovery(t, public)
			if current.calls != 1 || identity.calls != 1 || applier.calls != 0 || result.SourceProcessAttempts != 0 {
				t.Fatalf("result=%+v current/identity/apply calls=%d/%d/%d", result, current.calls, identity.calls, applier.calls)
			}
		})
	}
}

func TestExecuteMapsInvalidOrUnavailableRuntimeStateWithoutLeakingCause(t *testing.T) {
	hostile := "ATSURA_SECRET_RUNTIME_CAUSE"
	tests := []struct {
		name         string
		binding      wrapperbinding.RuntimeInvocation
		current      *currentExecutableStub
		identity     *identityStub
		kind         fault.Kind
		code         string
		wantCurrent  int
		wantIdentity int
	}{
		{
			name: "invalid binding", binding: func() wrapperbinding.RuntimeInvocation {
				value := runtimeBinding()
				value.BundleLocator = "/tmp/" + hostile + "\n"
				return value
			}(),
			current: &currentExecutableStub{}, identity: &identityStub{}, kind: fault.KindInvalidInput, code: "invalid_wrapper_binding",
		},
		{
			name: "current executable unavailable", binding: runtimeBinding(),
			current: &currentExecutableStub{err: errors.New(hostile)}, identity: &identityStub{}, kind: fault.KindUnavailable, code: "wrapper_runtime_unavailable", wantCurrent: 1,
		},
		{
			name: "runtime fingerprint unavailable", binding: runtimeBinding(),
			current: &currentExecutableStub{locator: "/proc/current/atr"}, identity: &identityStub{err: errors.New(hostile)}, kind: fault.KindUnavailable, code: "wrapper_runtime_unavailable", wantCurrent: 1, wantIdentity: 1,
		},
		{
			name: "invalid fingerprint result", binding: runtimeBinding(),
			current: &currentExecutableStub{locator: "/proc/current/atr"}, identity: &identityStub{}, kind: fault.KindUnavailable, code: "wrapper_runtime_unavailable", wantCurrent: 1, wantIdentity: 1,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			applier := &applierStub{}
			result, err := New(test.current, test.identity, applier).Execute(context.Background(), executeIntent(), test.binding, []string{"item", "list"})
			public := assertFault(t, err, test.kind, test.code)
			assertRenderRecovery(t, public)
			if strings.Contains(err.Error(), hostile) || strings.Contains(public.Message, hostile) {
				t.Fatalf("runtime cause leaked: error=%v public=%+v", err, public)
			}
			if test.current.calls != test.wantCurrent || test.identity.calls != test.wantIdentity || applier.calls != 0 || result.SourceProcessAttempts != 0 {
				t.Fatalf("result=%+v current/identity/apply calls=%d/%d/%d", result, test.current.calls, test.identity.calls, applier.calls)
			}
		})
	}
}

func TestExecuteRejectsInvalidIntentAndTypedNilWiringBeforePorts(t *testing.T) {
	binding := runtimeBinding()
	validCurrent := &currentExecutableStub{locator: "/proc/current/atr"}
	validIdentity := &identityStub{value: runtimeIdentity()}
	validApplier := &applierStub{}

	if _, err := New(validCurrent, validIdentity, validApplier).Execute(nil, executeIntent(), binding, []string{}); err == nil {
		t.Fatal("nil context succeeded")
	}
	wrongIntents := []operation.Intent{
		{Command: "bundle execute", Effect: operation.EffectExecute},
		{Command: Command, Effect: operation.EffectRead},
		{Command: Command, Effect: operation.EffectExecute, Target: operation.TargetRef{Kind: "source", ID: "fixture"}},
	}
	for _, intent := range wrongIntents {
		if _, err := New(validCurrent, validIdentity, validApplier).Execute(context.Background(), intent, binding, []string{}); err == nil {
			t.Fatalf("invalid intent succeeded: %+v", intent)
		}
	}
	wirings := []*Service{
		nil,
		New(nil, validIdentity, validApplier),
		New((*typedNilCurrent)(nil), validIdentity, validApplier),
		New(validCurrent, nil, validApplier),
		New(validCurrent, (*typedNilIdentity)(nil), validApplier),
		New(validCurrent, validIdentity, nil),
		New(validCurrent, validIdentity, (*typedNilApplier)(nil)),
	}
	for index, service := range wirings {
		if _, err := service.Execute(context.Background(), executeIntent(), binding, []string{}); err == nil {
			t.Fatalf("invalid wiring %d succeeded", index)
		}
	}
	if validCurrent.calls != 0 || validIdentity.calls != 0 || validApplier.calls != 0 {
		t.Fatalf("invalid preflight reached ports: %d/%d/%d", validCurrent.calls, validIdentity.calls, validApplier.calls)
	}
}

func TestExecutePreservesCancellationBeforePlanApplication(t *testing.T) {
	binding := runtimeBinding()

	preCanceled, cancel := context.WithCancel(context.Background())
	cancel()
	current := &currentExecutableStub{}
	identity := &identityStub{}
	applier := &applierStub{}
	if _, err := New(current, identity, applier).Execute(preCanceled, executeIntent(), binding, []string{}); !errors.Is(err, context.Canceled) {
		t.Fatalf("pre-canceled error = %v", err)
	}
	if current.calls != 0 || identity.calls != 0 || applier.calls != 0 {
		t.Fatalf("pre-canceled calls = %d/%d/%d", current.calls, identity.calls, applier.calls)
	}

	current = &currentExecutableStub{err: context.DeadlineExceeded}
	identity = &identityStub{}
	applier = &applierStub{}
	if _, err := New(current, identity, applier).Execute(context.Background(), executeIntent(), binding, []string{}); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("current-executable cancellation = %v", err)
	}
	if identity.calls != 0 || applier.calls != 0 {
		t.Fatalf("current-executable cancellation reached later ports: %d/%d", identity.calls, applier.calls)
	}

	runtimeCtx, runtimeCancel := context.WithCancel(context.Background())
	current = &currentExecutableStub{locator: "/proc/current/atr"}
	identity = &identityStub{value: runtimeIdentity(), after: runtimeCancel}
	applier = &applierStub{}
	if _, err := New(current, identity, applier).Execute(runtimeCtx, executeIntent(), binding, []string{}); !errors.Is(err, context.Canceled) {
		t.Fatalf("post-fingerprint cancellation = %v", err)
	}
	if applier.calls != 0 {
		t.Fatalf("post-fingerprint cancellation apply calls = %d", applier.calls)
	}
}

func assertFault(t *testing.T, err error, kind fault.Kind, code string) *fault.Error {
	t.Helper()
	public, ok := fault.PublicCopy(err)
	if !ok || public.Kind != kind || public.Code != code || public.Retryable {
		t.Fatalf("error=%v public=%+v want %s/%s non-retryable", err, public, kind, code)
	}
	return public
}

func assertRenderRecovery(t *testing.T, public *fault.Error) {
	t.Helper()
	if len(public.NextActions) != 1 || public.NextActions[0].Command != "wrapper render" {
		t.Fatalf("next actions = %+v", public.NextActions)
	}
}
