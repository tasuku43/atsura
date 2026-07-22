package runtimecompat_test

import (
	"errors"
	"reflect"
	"testing"

	"github.com/tasuku43/atsura/internal/app/planapply"
	"github.com/tasuku43/atsura/internal/app/runtimecompat"
	"github.com/tasuku43/atsura/internal/app/wrapperrender"
	"github.com/tasuku43/atsura/internal/domain/runtimeadmission"
	"github.com/tasuku43/atsura/internal/domain/sourcecatalog"
	"github.com/tasuku43/atsura/internal/domain/tailoringbundle"
	"github.com/tasuku43/atsura/internal/domain/tailoringplan"
)

var (
	_ planapply.CompatibilityPort     = (*runtimecompat.Registry)(nil)
	_ wrapperrender.CompatibilityPort = (*runtimecompat.Registry)(nil)
)

type verifierStub struct {
	runtimeErr   error
	surfaceErr   error
	runtimePlans []tailoringplan.Plan
	surfaces     []tailoringbundle.Bundle
}

func (s *verifierStub) VerifyRuntime(plan tailoringplan.Plan) error {
	s.runtimePlans = append(s.runtimePlans, plan)
	return s.runtimeErr
}

func (s *verifierStub) VerifySurface(bundle tailoringbundle.Bundle) error {
	s.surfaces = append(s.surfaces, bundle)
	return s.surfaceErr
}

func TestRegistryDispatchesExactEvidenceByAdapterKind(t *testing.T) {
	t.Parallel()

	github := &verifierStub{}
	goCLI := &verifierStub{}
	registry := runtimecompat.New(
		runtimecompat.Registration{AdapterKind: "atsura.source.github_cli", Verifier: github},
		runtimecompat.Registration{AdapterKind: "atsura.source.go_cli", Verifier: goCLI},
	)

	plan := tailoringplan.Plan{
		Source:         tailoringplan.SourceIdentity{AdapterKind: "atsura.source.go_cli"},
		MatchedCommand: []string{"test"},
		OriginalArgv:   []string{"go", "test"},
	}
	if err := registry.VerifyRuntime(plan); err != nil {
		t.Fatalf("VerifyRuntime() error = %v", err)
	}
	if len(goCLI.runtimePlans) != 1 || !reflect.DeepEqual(goCLI.runtimePlans[0], plan) {
		t.Fatalf("Go verifier received %#v, want exact plan %#v", goCLI.runtimePlans, plan)
	}
	if len(github.runtimePlans) != 0 {
		t.Fatalf("GitHub verifier runtime calls = %d, want 0", len(github.runtimePlans))
	}

	bundle := tailoringbundle.Bundle{
		Catalog: tailoringbundleCatalog("atsura.source.github_cli"),
		Surface: []tailoringbundle.SurfaceEntry{
			{Command: []string{"issue", "list"}},
			{Command: []string{"pr", "list"}},
		},
	}
	if err := registry.VerifySurface(bundle); err != nil {
		t.Fatalf("VerifySurface() error = %v", err)
	}
	if len(github.surfaces) != 1 || !reflect.DeepEqual(github.surfaces[0], bundle) {
		t.Fatalf("GitHub verifier received %#v, want exact bundle %#v", github.surfaces, bundle)
	}
	if len(goCLI.surfaces) != 0 {
		t.Fatalf("Go verifier surface calls = %d, want 0", len(goCLI.surfaces))
	}
}

func TestRegistryFailsClosedBeforeDispatchWhenConfigurationOrKindIsInvalid(t *testing.T) {
	t.Parallel()

	typedNil := (*verifierStub)(nil)
	tests := []struct {
		name     string
		registry *runtimecompat.Registry
		kind     string
	}{
		{name: "nil registry", registry: nil, kind: "atsura.source.go_cli"},
		{name: "zero registry", registry: &runtimecompat.Registry{}, kind: "atsura.source.go_cli"},
		{name: "no registrations", registry: runtimecompat.New(), kind: "atsura.source.go_cli"},
		{name: "missing kind", registry: runtimecompat.New(runtimecompat.Registration{AdapterKind: "atsura.source.go_cli", Verifier: &verifierStub{}})},
		{name: "unknown kind", registry: runtimecompat.New(runtimecompat.Registration{AdapterKind: "atsura.source.go_cli", Verifier: &verifierStub{}}), kind: "atsura.source.unknown"},
		{name: "nil verifier", registry: runtimecompat.New(runtimecompat.Registration{AdapterKind: "atsura.source.go_cli"}), kind: "atsura.source.go_cli"},
		{name: "typed nil verifier", registry: runtimecompat.New(runtimecompat.Registration{AdapterKind: "atsura.source.go_cli", Verifier: typedNil}), kind: "atsura.source.go_cli"},
		{name: "empty registered kind", registry: runtimecompat.New(runtimecompat.Registration{Verifier: &verifierStub{}}), kind: "atsura.source.go_cli"},
		{
			name: "duplicate registered kind",
			registry: runtimecompat.New(
				runtimecompat.Registration{AdapterKind: "atsura.source.go_cli", Verifier: &verifierStub{}},
				runtimecompat.Registration{AdapterKind: "atsura.source.go_cli", Verifier: &verifierStub{}},
			),
			kind: "atsura.source.go_cli",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			planErr := test.registry.VerifyRuntime(tailoringplan.Plan{Source: tailoringplan.SourceIdentity{AdapterKind: test.kind}})
			assertAdapterContract(t, planErr)
			bundleErr := test.registry.VerifySurface(tailoringbundle.Bundle{Catalog: tailoringbundleCatalog(test.kind)})
			assertAdapterContract(t, bundleErr)
		})
	}
}

func TestRegistryRejectsMisconfiguredVerifierErrors(t *testing.T) {
	t.Parallel()

	typedNil := (*categorizedAdmissionError)(nil)
	tests := []struct {
		name string
		err  error
	}{
		{name: "ordinary error", err: errors.New("unclassified adapter failure")},
		{name: "typed nil categorized error", err: typedNil},
		{name: "unknown category", err: &categorizedAdmissionError{category: runtimeadmission.Category("future_category")}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			verifier := &verifierStub{runtimeErr: test.err, surfaceErr: test.err}
			registry := runtimecompat.New(runtimecompat.Registration{AdapterKind: "atsura.source.go_cli", Verifier: verifier})
			assertAdapterContract(t, registry.VerifyRuntime(tailoringplan.Plan{Source: tailoringplan.SourceIdentity{AdapterKind: "atsura.source.go_cli"}}))
			assertAdapterContract(t, registry.VerifySurface(tailoringbundle.Bundle{Catalog: tailoringbundleCatalog("atsura.source.go_cli")}))
		})
	}
}

func TestRegistryPreservesValidVerifierAdmissionErrors(t *testing.T) {
	t.Parallel()

	for _, category := range []runtimeadmission.Category{
		runtimeadmission.CategoryAdapterContract,
		runtimeadmission.CategorySourceVersion,
		runtimeadmission.CategoryCommand,
		runtimeadmission.CategoryWrapperOutput,
		runtimeadmission.CategoryArgvGrammar,
		runtimeadmission.CategorySelectorConflict,
	} {
		category := category
		t.Run(string(category), func(t *testing.T) {
			t.Parallel()
			want := &categorizedAdmissionError{category: category}
			verifier := &verifierStub{runtimeErr: want, surfaceErr: want}
			registry := runtimecompat.New(runtimecompat.Registration{AdapterKind: "atsura.source.go_cli", Verifier: verifier})

			if got := registry.VerifyRuntime(tailoringplan.Plan{Source: tailoringplan.SourceIdentity{AdapterKind: "atsura.source.go_cli"}}); got != want {
				t.Fatalf("VerifyRuntime() error = %v, want same error %v", got, want)
			}
			if got := registry.VerifySurface(tailoringbundle.Bundle{Catalog: tailoringbundleCatalog("atsura.source.go_cli")}); got != want {
				t.Fatalf("VerifySurface() error = %v, want same error %v", got, want)
			}
		})
	}
}

type categorizedAdmissionError struct {
	category runtimeadmission.Category
}

func (e *categorizedAdmissionError) Error() string { return "categorized runtime admission" }

func (e *categorizedAdmissionError) RuntimeAdmissionCategory() runtimeadmission.Category {
	return e.category
}

func tailoringbundleCatalog(kind string) sourcecatalog.Catalog {
	return sourcecatalog.Catalog{Adapter: sourcecatalog.Adapter{Kind: kind}}
}

func assertAdapterContract(t *testing.T, err error) {
	t.Helper()
	if err == nil {
		t.Fatal("error = nil, want fail-closed adapter contract error")
	}
	if !errors.Is(err, runtimecompat.ErrAdapterContract) {
		t.Fatalf("error = %v, want errors.Is ErrAdapterContract", err)
	}
	var categorized interface {
		RuntimeAdmissionCategory() runtimeadmission.Category
	}
	if !errors.As(err, &categorized) || categorized.RuntimeAdmissionCategory() != runtimeadmission.CategoryAdapterContract {
		t.Fatalf("error category = %v, want %q", categorized, runtimeadmission.CategoryAdapterContract)
	}
}
