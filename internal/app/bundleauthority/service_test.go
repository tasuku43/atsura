package bundleauthority

import (
	"context"
	"errors"
	"testing"

	"github.com/tasuku43/atsura/internal/domain/bundletrust"
	"github.com/tasuku43/atsura/internal/domain/operation"
	"github.com/tasuku43/atsura/internal/domain/sourcecatalog"
	"github.com/tasuku43/atsura/internal/domain/sourceprocess"
	"github.com/tasuku43/atsura/internal/domain/tailoringbundle"
)

type bundleStub struct {
	bundle tailoringbundle.Bundle
	digest string
	err    error
}

func (s bundleStub) Load(context.Context, string) (tailoringbundle.Bundle, string, error) {
	return s.bundle, s.digest, s.err
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

type trustStub struct {
	state bundletrust.State
	adds  int
}

func (s *trustStub) Inspect(context.Context, string) bundletrust.State { return s.state }
func (s *trustStub) Add(context.Context, string) (bool, error)         { s.adds++; return true, nil }

type confirmStub struct {
	err   error
	calls int
	seen  bundletrust.Summary
}

func (s *confirmStub) Confirm(_ context.Context, summary bundletrust.Summary) error {
	s.calls++
	s.seen = summary
	return s.err
}

func authorityFixture() (tailoringbundle.Bundle, string, sourceprocess.Identity) {
	identity := sourceprocess.Identity{ResolvedPath: "/tool", SHA256: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", Size: 1}
	catalog := sourcecatalog.Catalog{SchemaVersion: 1, Adapter: sourcecatalog.Adapter{Kind: "example.test.source", ContractVersion: 1}, Source: sourcecatalog.Source{RequestedExecutable: "tool", ResolvedPath: identity.ResolvedPath, SHA256: identity.SHA256, Size: identity.Size, Version: "1.0"}, Probe: sourcecatalog.Probe{IDs: []string{"help"}, Attempts: 1}, Commands: []sourcecatalog.Command{{Path: []string{"item", "list"}, Summary: "List", Provenance: sourcecatalog.ProvenanceVerifiedBuiltin, Options: []sourcecatalog.Option{}, StructuredOutput: []sourcecatalog.StructuredOutput{{Format: "json", SelectorFlag: "--json", Fields: []string{"id"}}}}}}
	cd, _ := catalog.Digest()
	specification := tailoringbundle.Specification{SchemaVersion: tailoringbundle.SpecificationSchemaVersion, CatalogDigest: cd, Surface: tailoringbundle.Surface{Default: tailoringbundle.SurfaceDefaultExclude}, Commands: []tailoringbundle.CommandEntry{{Command: []string{"item", "list"}, Presence: tailoringbundle.PresenceInclude, Reason: "needed", Options: &tailoringbundle.OptionSurface{Default: tailoringbundle.SurfaceDefaultInherit, Include: []string{}, Exclude: []string{}}, Wrapper: &tailoringbundle.Wrapper{Kind: tailoringbundle.WrapperIdentity, Before: []tailoringbundle.StageAction{}, Invoke: tailoringbundle.Invocation{AppendArgs: []string{}}, After: []tailoringbundle.StageAction{}}}}}
	bundle, _ := tailoringbundle.Compile(catalog, specification)
	digest, _ := bundle.Digest()
	return bundle, digest, identity
}

func readIntent() operation.Intent {
	return operation.Intent{Command: StatusCommand, Effect: operation.EffectRead}
}
func trustIntent() operation.Intent {
	return operation.Intent{Command: TrustCommand, Effect: operation.EffectWrite, Target: operation.TargetRef{Kind: TrustTargetKind, ID: TrustTargetID}, Impact: TrustImpact}
}

func TestStatusReportsAdoptionAndSourceDriftAsIndependentFacts(t *testing.T) {
	bundle, digest, identity := authorityFixture()
	identities := &identityStub{identity: identity}
	trust := &trustStub{state: bundletrust.StateAdopted}
	result, err := New(bundleStub{bundle: bundle, digest: digest}, identities, trust, nil).Status(context.Background(), readIntent(), "bundle.json")
	if err != nil || !result.Adopted || result.Source != bundletrust.SourceCurrent || result.SourceProcessAttempts != 0 {
		t.Fatalf("Status() = %+v, %v", result, err)
	}
	identities.identity.SHA256 = "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
	result, err = New(bundleStub{bundle: bundle, digest: digest}, identities, trust, nil).Status(context.Background(), readIntent(), "bundle.json")
	if err != nil || !result.Adopted || result.Source != bundletrust.SourceDrifted {
		t.Fatalf("drift Status() = %+v, %v", result, err)
	}
}

func TestTrustConfirmsThenMutatesExactlyOnceAndFailsClosed(t *testing.T) {
	bundle, digest, identity := authorityFixture()
	identityPort := &identityStub{identity: identity}
	trust := &trustStub{state: bundletrust.StateNotAdopted}
	confirm := &confirmStub{}
	result, err := New(bundleStub{bundle: bundle, digest: digest}, identityPort, trust, confirm).Trust(context.Background(), trustIntent(), "bundle.json")
	if err != nil || !result.Adopted || confirm.calls != 1 || trust.adds != 1 {
		t.Fatalf("Trust() = %+v, %v; confirms=%d adds=%d", result, err, confirm.calls, trust.adds)
	}
	if confirm.seen.SpecificationDigest != bundle.SpecificationDigest || confirm.seen.SurfaceDefault != "exclude" || confirm.seen.IncludedCommandCount != 1 || confirm.seen.IdentityWrapperCount != 1 || confirm.seen.TransformWrapperCount != 0 {
		t.Fatalf("adoption summary = %+v", confirm.seen)
	}
	confirm.err = errors.New("no")
	trust.adds = 0
	if _, err := New(bundleStub{bundle: bundle, digest: digest}, identityPort, trust, confirm).Trust(context.Background(), trustIntent(), "bundle.json"); err == nil || trust.adds != 0 {
		t.Fatalf("denied error=%v adds=%d", err, trust.adds)
	}
}
