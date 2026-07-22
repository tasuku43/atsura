package planpreview

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/tasuku43/atsura/internal/domain/bundletrust"
	"github.com/tasuku43/atsura/internal/domain/fault"
	"github.com/tasuku43/atsura/internal/domain/operation"
	"github.com/tasuku43/atsura/internal/domain/sourcecatalog"
	"github.com/tasuku43/atsura/internal/domain/sourceprocess"
	"github.com/tasuku43/atsura/internal/domain/tailoringbundle"
	"github.com/tasuku43/atsura/internal/domain/tailoringplan"
)

type bundleStub struct {
	bundle tailoringbundle.Bundle
	digest string
	err    error
	calls  int
}

func (s *bundleStub) Load(context.Context, string) (tailoringbundle.Bundle, string, error) {
	s.calls++
	return s.bundle, s.digest, s.err
}

type adoptionStub struct {
	state  bundletrust.State
	calls  int
	cancel context.CancelFunc
}

func (s *adoptionStub) Inspect(context.Context, string) bundletrust.State {
	s.calls++
	if s.cancel != nil {
		s.cancel()
	}
	return s.state
}

type identityStub struct {
	identity sourceprocess.Identity
	err      error
	calls    int
}

func (s *identityStub) Identify(context.Context, string) (sourceprocess.Identity, error) {
	s.calls++
	return s.identity, s.err
}

func previewIntent() operation.Intent {
	return operation.Intent{Command: Command, Effect: operation.EffectRead}
}

func previewBundle(t *testing.T) (tailoringbundle.Bundle, string, sourceprocess.Identity) {
	return previewBundleWithDefaults(t, []tailoringbundle.OptionDefault{})
}

func previewBundleWithDefaults(t *testing.T, defaults []tailoringbundle.OptionDefault) (tailoringbundle.Bundle, string, sourceprocess.Identity) {
	t.Helper()
	catalog := sourcecatalog.Catalog{
		SchemaVersion: sourcecatalog.SchemaVersion,
		Adapter:       sourcecatalog.Adapter{Kind: "atsura.source.alternate", ContractVersion: 1},
		Source:        sourcecatalog.Source{RequestedExecutable: "fixture", ResolvedPath: "/opt/bin/fixture", SHA256: strings.Repeat("a", 64), Size: 42, Version: "1.0.0"},
		Probe:         sourcecatalog.Probe{IDs: []string{"help"}, Attempts: 1},
		Commands: []sourcecatalog.Command{{Path: []string{"item", "list"}, Summary: "List items", Provenance: sourcecatalog.ProvenanceVerifiedBuiltin, Options: []sourcecatalog.Option{
			{Name: "--json", TakesValue: true}, {Name: "--limit", TakesValue: true},
		}, StructuredOutput: []sourcecatalog.StructuredOutput{}}},
	}
	catalogDigest, err := catalog.Digest()
	if err != nil {
		t.Fatal(err)
	}
	wrapperKind := tailoringbundle.WrapperIdentity
	if len(defaults) > 0 {
		wrapperKind = tailoringbundle.WrapperTransform
	}
	specification := tailoringbundle.Specification{
		SchemaVersion: tailoringbundle.SpecificationSchemaVersion,
		CatalogDigest: catalogDigest,
		Surface:       tailoringbundle.Surface{Default: tailoringbundle.SurfaceDefaultExclude},
		Commands: []tailoringbundle.CommandEntry{{
			Command: []string{"item", "list"}, Presence: tailoringbundle.PresenceInclude, Reason: "Needed.",
			Options: &tailoringbundle.OptionSurface{Default: tailoringbundle.SurfaceDefaultInherit, Include: []string{}, Exclude: []string{}},
			Wrapper: &tailoringbundle.Wrapper{Kind: wrapperKind, Before: []tailoringbundle.StageAction{}, Invoke: tailoringbundle.Invocation{OptionDefaults: append([]tailoringbundle.OptionDefault{}, defaults...), AppendArgs: []string{}}, After: []tailoringbundle.StageAction{}},
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
	identity := sourceprocess.Identity{ResolvedPath: catalog.Source.ResolvedPath, SHA256: catalog.Source.SHA256, Size: catalog.Source.Size}
	return bundle, digest, identity
}

func TestPreviewDerivesCanonicalDefaultAndPreservesCallerOverrideWithoutStartingSource(t *testing.T) {
	bundle, digest, identity := previewBundleWithDefaults(t, []tailoringbundle.OptionDefault{{Option: "--limit", Value: "30"}})
	tests := []struct {
		name        string
		args        []string
		wantArgs    []string
		wantApplied []tailoringbundle.OptionDefault
	}{
		{
			name: "omitted option receives canonical default", args: []string{"item", "list"},
			wantArgs:    []string{"item", "list", "--limit=30"},
			wantApplied: []tailoringbundle.OptionDefault{{Option: "--limit", Value: "30"}},
		},
		{
			name: "explicit caller value wins", args: []string{"item", "list", "--limit=2"},
			wantArgs: []string{"item", "list", "--limit=2"}, wantApplied: []tailoringbundle.OptionDefault{},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result, err := New(
				&bundleStub{bundle: bundle, digest: digest},
				&adoptionStub{state: bundletrust.StateAdopted},
				&identityStub{identity: identity},
			).Preview(context.Background(), previewIntent(), "bundle.json", tailoringplan.Attempt{Executable: "fixture", Args: test.args})
			if err != nil {
				t.Fatal(err)
			}
			request, err := result.Plan.SourceRequest()
			if err != nil {
				t.Fatal(err)
			}
			if result.SourceProcessAttempts != 0 || !reflect.DeepEqual(request.Process.Args, test.wantArgs) ||
				!reflect.DeepEqual(result.Plan.Stages.Invoke.AppliedOptionDefaults, test.wantApplied) {
				t.Fatalf("result=%+v request=%+v", result, request)
			}
		})
	}
}

func TestPreviewRequiresAdoptionAndCurrentIdentityThenBuildsPurePlan(t *testing.T) {
	bundle, digest, identity := previewBundle(t)
	bundles := &bundleStub{bundle: bundle, digest: digest}
	adoption := &adoptionStub{state: bundletrust.StateAdopted}
	identities := &identityStub{identity: identity}
	result, err := New(bundles, adoption, identities).Preview(context.Background(), previewIntent(), "bundle.json", tailoringplan.Attempt{Executable: "fixture", Args: []string{"item", "list", "--json=id"}})
	if err != nil {
		t.Fatal(err)
	}
	if bundles.calls != 1 || adoption.calls != 1 || identities.calls != 1 || result.SourceProcessAttempts != 0 || result.PlanDigest == "" {
		t.Fatalf("result=%+v calls=%d/%d/%d", result, bundles.calls, adoption.calls, identities.calls)
	}
	if result.Plan.Stages.Invoke.MaxAttempts != 1 || result.Plan.Stages.Invoke.Executable != identity.ResolvedPath {
		t.Fatalf("plan=%+v", result.Plan)
	}
}

func TestPreviewFailsInOrderBeforePlanConstruction(t *testing.T) {
	bundle, digest, identity := previewBundle(t)
	loadFailure := fault.New(fault.KindInvalidInput, "invalid_bundle_file", "Bundle is invalid.", false, fault.NextAction{Command: "bundle build", Reason: "Rebuild it."})
	identityFailure := fault.New(fault.KindUnavailable, "source_identity_unavailable", "Source identity is unavailable.", true, fault.NextAction{Command: "help source inspect", Reason: "Inspect it."})
	tests := []struct {
		name          string
		bundleError   error
		adoption      bundletrust.State
		identity      sourceprocess.Identity
		identityError error
		wantCode      string
		wantAdoption  int
		wantIdentity  int
	}{
		{name: "bundle load", bundleError: loadFailure, adoption: bundletrust.StateAdopted, identity: identity, wantCode: "invalid_bundle_file"},
		{name: "invalid adoption store", adoption: bundletrust.StateInvalid, identity: identity, wantCode: "invalid_bundle_trust_store", wantAdoption: 1},
		{name: "unknown adoption state", adoption: bundletrust.State("unexpected"), identity: identity, wantCode: "invalid_bundle_trust_store", wantAdoption: 1},
		{name: "not adopted", adoption: bundletrust.StateNotAdopted, identity: identity, wantCode: "bundle_not_adopted", wantAdoption: 1},
		{name: "identity unavailable", adoption: bundletrust.StateAdopted, identity: identity, identityError: identityFailure, wantCode: "source_identity_unavailable", wantAdoption: 1, wantIdentity: 1},
		{name: "source drift", adoption: bundletrust.StateAdopted, identity: sourceprocess.Identity{ResolvedPath: identity.ResolvedPath, SHA256: strings.Repeat("b", 64), Size: identity.Size}, wantCode: "bundle_source_drift", wantAdoption: 1, wantIdentity: 1},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			bundles := &bundleStub{bundle: bundle, digest: digest, err: test.bundleError}
			adoption := &adoptionStub{state: test.adoption}
			identities := &identityStub{identity: test.identity, err: test.identityError}
			result, err := New(bundles, adoption, identities).Preview(context.Background(), previewIntent(), "bundle.json", tailoringplan.Attempt{Executable: "fixture", Args: []string{"item", "list"}})
			if result.SourceProcessAttempts != 0 {
				t.Fatalf("result=%+v", result)
			}
			public, ok := fault.PublicCopy(err)
			if !ok || public.Code != test.wantCode || public.Retryable != (test.wantCode == "source_identity_unavailable") {
				t.Fatalf("error=%v public=%+v", err, public)
			}
			if adoption.calls != test.wantAdoption || identities.calls != test.wantIdentity {
				t.Fatalf("calls adoption=%d identity=%d", adoption.calls, identities.calls)
			}
		})
	}
}

func TestPreviewMapsPurePlanFailuresWithoutProcessFacts(t *testing.T) {
	bundle, digest, identity := previewBundle(t)
	tests := []struct {
		name    string
		attempt tailoringplan.Attempt
		code    string
		kind    fault.Kind
	}{
		{name: "source mismatch", attempt: tailoringplan.Attempt{Executable: "other", Args: []string{"item", "list"}}, code: "source_executable_mismatch", kind: fault.KindInvalidInput},
		{name: "unknown command", attempt: tailoringplan.Attempt{Executable: "fixture", Args: []string{"other"}}, code: "invalid_invocation", kind: fault.KindInvalidInput},
		{name: "hidden option", attempt: tailoringplan.Attempt{Executable: "fixture", Args: []string{"item", "list", "--unknown"}}, code: "invalid_invocation", kind: fault.KindInvalidInput},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result, err := New(&bundleStub{bundle: bundle, digest: digest}, &adoptionStub{state: bundletrust.StateAdopted}, &identityStub{identity: identity}).Preview(context.Background(), previewIntent(), "bundle.json", test.attempt)
			public, ok := fault.PublicCopy(err)
			if result.SourceProcessAttempts != 0 || !ok || public.Code != test.code || public.Kind != test.kind || public.Retryable {
				t.Fatalf("result=%+v error=%+v", result, public)
			}
		})
	}
}

func TestPreviewRejectsInvalidWiringIntentAndCancellation(t *testing.T) {
	bundle, digest, identity := previewBundle(t)
	valid := New(&bundleStub{bundle: bundle, digest: digest}, &adoptionStub{state: bundletrust.StateAdopted}, &identityStub{identity: identity})
	if _, err := valid.Preview(context.Background(), operation.Intent{Command: Command, Effect: operation.EffectExecute}, "bundle.json", tailoringplan.Attempt{}); err == nil {
		t.Fatal("invalid intent succeeded")
	}
	if _, err := New(nil, &adoptionStub{}, &identityStub{}).Preview(context.Background(), previewIntent(), "bundle.json", tailoringplan.Attempt{}); err == nil {
		t.Fatal("invalid wiring succeeded")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := valid.Preview(ctx, previewIntent(), "bundle.json", tailoringplan.Attempt{}); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancellation error=%v", err)
	}
	duringAdoption, cancelDuringAdoption := context.WithCancel(context.Background())
	canceling := New(&bundleStub{bundle: bundle, digest: digest}, &adoptionStub{state: bundletrust.StateInvalid, cancel: cancelDuringAdoption}, &identityStub{identity: identity})
	if _, err := canceling.Preview(duringAdoption, previewIntent(), "bundle.json", tailoringplan.Attempt{}); !errors.Is(err, context.Canceled) {
		t.Fatalf("adoption cancellation error=%v", err)
	}
	loadCanceled := New(&bundleStub{err: context.Canceled}, &adoptionStub{}, &identityStub{})
	if _, err := loadCanceled.Preview(context.Background(), previewIntent(), "bundle.json", tailoringplan.Attempt{}); !errors.Is(err, context.Canceled) {
		t.Fatalf("load cancellation error=%v", err)
	}
	identityCanceled := New(&bundleStub{bundle: bundle, digest: digest}, &adoptionStub{state: bundletrust.StateAdopted}, &identityStub{err: context.DeadlineExceeded})
	if _, err := identityCanceled.Preview(context.Background(), previewIntent(), "bundle.json", tailoringplan.Attempt{}); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("identity cancellation error=%v", err)
	}
}
