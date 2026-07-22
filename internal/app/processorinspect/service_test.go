package processorinspect

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tasuku43/atsura/internal/domain/fault"
	"github.com/tasuku43/atsura/internal/domain/operation"
	"github.com/tasuku43/atsura/internal/domain/processorprocess"
)

type fakeInspector struct {
	observation processorprocess.Observation
	err         error
	calls       int
	cancel      context.CancelFunc
}

func (f *fakeInspector) Inspect(_ context.Context, _ string) (processorprocess.Observation, error) {
	f.calls++
	if f.cancel != nil {
		f.cancel()
	}
	return f.observation, f.err
}

func observationFixture(t *testing.T) processorprocess.Observation {
	t.Helper()
	return processorprocess.Observation{
		SchemaVersion: processorprocess.ObservationSchemaVersion,
		Adapter:       processorprocess.Adapter{Kind: "atsura.processor.alternate", ContractVersion: 1},
		Platform:      processorprocess.Platform{OS: "linux", Arch: "amd64"},
		Identity: processorprocess.Identity{
			ResolvedPath: filepath.Join(t.TempDir(), "processor"), SHA256: strings.Repeat("a", 64), Size: 42,
		},
		Version: "1.0.0",
		Probe: processorprocess.Probe{
			Argv: []string{"--version"}, EnvironmentContract: processorprocess.EnvironmentRTKIsolatedV2, Attempts: 1,
		},
	}
}

func intent() operation.Intent {
	return operation.Intent{Command: Command, Effect: operation.EffectExecute}
}

func serviceFor(fake *fakeInspector) *Service {
	return New(Registration{Selector: "alternate", AdapterKind: "atsura.processor.alternate", Inspector: fake})
}

func publicCode(t *testing.T, err error) string {
	t.Helper()
	public, ok := fault.PublicCopy(err)
	if !ok {
		t.Fatalf("error is not public: %v", err)
	}
	if len(public.NextActions) != 1 || public.NextActions[0].Command != "help processor inspect" {
		t.Fatalf("next actions = %+v", public.NextActions)
	}
	return public.Code
}

func TestInspectAcceptsFiniteAdapterAndReturnsCanonicalDigest(t *testing.T) {
	observation := observationFixture(t)
	fake := &fakeInspector{observation: observation}
	result, err := serviceFor(fake).Inspect(context.Background(), intent(), "alternate", observation.Identity.ResolvedPath)
	if err != nil || fake.calls != 1 || result.Observation.Identity != observation.Identity || len(result.Digest) != 64 {
		t.Fatalf("result=%+v calls=%d error=%v", result, fake.calls, err)
	}
	wantDigest, _ := observation.Digest()
	if result.Digest != wantDigest {
		t.Fatalf("digest=%q want=%q", result.Digest, wantDigest)
	}
}

func TestInspectRejectsSelectionPathIntentAndRegistryBeforeAdapterCall(t *testing.T) {
	observation := observationFixture(t)
	fake := &fakeInspector{observation: observation}
	service := serviceFor(fake)
	if _, err := service.Inspect(context.Background(), intent(), "missing", observation.Identity.ResolvedPath); publicCode(t, err) != "unsupported_processor_adapter" {
		t.Fatalf("selection error = %v", err)
	}
	if _, err := service.Inspect(context.Background(), intent(), "alternate", "rtk"); publicCode(t, err) != "invalid_processor_executable" {
		t.Fatalf("path error = %v", err)
	}
	if _, err := service.Inspect(context.Background(), operation.Intent{Command: Command, Effect: operation.EffectRead}, "alternate", observation.Identity.ResolvedPath); err == nil {
		t.Fatal("read intent succeeded")
	}
	if fake.calls != 0 {
		t.Fatalf("adapter calls=%d", fake.calls)
	}

	typedNil := (*fakeInspector)(nil)
	misconfigured := New(Registration{Selector: "alternate", AdapterKind: "atsura.processor.alternate", Inspector: typedNil})
	if _, err := misconfigured.Inspect(context.Background(), intent(), "alternate", observation.Identity.ResolvedPath); publicCode(t, err) != "processor_adapter_contract" {
		t.Fatalf("typed-nil error=%v", err)
	}
	duplicate := New(
		Registration{Selector: "alternate", AdapterKind: "atsura.processor.alternate", Inspector: fake},
		Registration{Selector: "alternate", AdapterKind: "atsura.processor.other", Inspector: fake},
	)
	if _, err := duplicate.Inspect(context.Background(), intent(), "alternate", observation.Identity.ResolvedPath); publicCode(t, err) != "processor_adapter_contract" {
		t.Fatalf("duplicate error=%v", err)
	}
	malformed := New(Registration{Selector: "alternate", AdapterKind: "rtk", Inspector: fake})
	if _, err := malformed.Inspect(context.Background(), intent(), "alternate", observation.Identity.ResolvedPath); publicCode(t, err) != "processor_adapter_contract" {
		t.Fatalf("malformed error=%v", err)
	}
}

func TestInspectMapsFiniteAdapterErrors(t *testing.T) {
	tests := []struct {
		name string
		err  error
		code string
	}{
		{name: "platform", err: processorprocess.ErrUnsupportedPlatform, code: "unsupported_processor_platform"},
		{name: "version", err: processorprocess.ErrUnsupportedVersion, code: "unsupported_processor_version"},
		{name: "artifact", err: processorprocess.ErrUnsupportedArtifact, code: "unsupported_processor_artifact"},
		{name: "observation", err: processorprocess.ErrInvalidObservation, code: "invalid_processor_observation"},
		{name: "identity", err: processorprocess.ErrInvalidIdentity, code: "invalid_processor_observation"},
		{name: "request", err: processorprocess.ErrInvalidRequest, code: "invalid_processor_observation"},
		{name: "result", err: processorprocess.ErrInvalidResult, code: "invalid_processor_observation"},
		{name: "inspection", err: processorprocess.ErrInspectionFailed, code: "processor_inspection_failed"},
		{name: "canceled", err: context.Canceled, code: "processor_execution_canceled"},
		{name: "unknown", err: errors.New("secret-bearing adapter error"), code: "internal_error"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			observation := observationFixture(t)
			fake := &fakeInspector{observation: observation, err: test.err}
			_, err := serviceFor(fake).Inspect(context.Background(), intent(), "alternate", observation.Identity.ResolvedPath)
			if code := publicCode(t, err); code != test.code || fake.calls != 1 {
				t.Fatalf("code=%q calls=%d error=%v", code, fake.calls, err)
			}
		})
	}
}

func TestInspectPreservesValidPublicFaultWithoutItsPrivateRecovery(t *testing.T) {
	observation := observationFixture(t)
	upstream := fault.Wrap(fault.KindUnavailable, "processor_identity_unavailable", "The processor executable identity could not be read.", true, errors.New("private path detail"), fault.NextAction{Command: "private command", Reason: "private reason"})
	fake := &fakeInspector{observation: observation, err: upstream}
	_, err := serviceFor(fake).Inspect(context.Background(), intent(), "alternate", observation.Identity.ResolvedPath)
	if code := publicCode(t, err); code != "processor_identity_unavailable" {
		t.Fatalf("code=%q error=%v", code, err)
	}
	public, _ := fault.PublicCopy(err)
	if !public.Retryable || public.Kind != fault.KindUnavailable {
		t.Fatalf("public=%+v", public)
	}
}

func TestInspectRejectsInvalidMismatchedAndLateCanceledObservation(t *testing.T) {
	observation := observationFixture(t)
	invalid := observation
	invalid.SchemaVersion = 2
	fake := &fakeInspector{observation: invalid}
	if _, err := serviceFor(fake).Inspect(context.Background(), intent(), "alternate", observation.Identity.ResolvedPath); publicCode(t, err) != "invalid_processor_observation" {
		t.Fatalf("invalid error=%v", err)
	}

	mismatched := observation
	mismatched.Adapter.Kind = "atsura.processor.other"
	fake = &fakeInspector{observation: mismatched}
	if _, err := serviceFor(fake).Inspect(context.Background(), intent(), "alternate", observation.Identity.ResolvedPath); publicCode(t, err) != "invalid_processor_observation" {
		t.Fatalf("adapter mismatch error=%v", err)
	}

	wrongPath := observation
	wrongPath.Identity.ResolvedPath = filepath.Join(t.TempDir(), "other")
	fake = &fakeInspector{observation: wrongPath}
	if _, err := serviceFor(fake).Inspect(context.Background(), intent(), "alternate", observation.Identity.ResolvedPath); publicCode(t, err) != "invalid_processor_observation" {
		t.Fatalf("path mismatch error=%v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	fake = &fakeInspector{observation: observation, cancel: cancel}
	if _, err := serviceFor(fake).Inspect(ctx, intent(), "alternate", observation.Identity.ResolvedPath); publicCode(t, err) != "processor_execution_canceled" {
		t.Fatalf("late cancel error=%v", err)
	}
}

func TestInspectHonorsCanceledAndNilContextBeforeAdapter(t *testing.T) {
	observation := observationFixture(t)
	fake := &fakeInspector{observation: observation}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := serviceFor(fake).Inspect(ctx, intent(), "alternate", observation.Identity.ResolvedPath); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled error=%v", err)
	}
	if _, err := serviceFor(fake).Inspect(nil, intent(), "alternate", observation.Identity.ResolvedPath); err == nil {
		t.Fatal("nil context succeeded")
	}
	if fake.calls != 0 {
		t.Fatalf("calls=%d", fake.calls)
	}
}
