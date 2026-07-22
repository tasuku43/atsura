package bundleauthority

import (
	"context"
	"errors"
	"testing"

	"github.com/tasuku43/atsura/internal/domain/bundletrust"
	"github.com/tasuku43/atsura/internal/domain/fault"
	"github.com/tasuku43/atsura/internal/domain/operation"
	"github.com/tasuku43/atsura/internal/domain/processorprocess"
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

type processorIdentityStub struct {
	identity processorprocess.Identity
	err      error
	calls    int
}

func (s *processorIdentityStub) Identify(context.Context, string) (processorprocess.Identity, error) {
	s.calls++
	return s.identity, s.err
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
	catalog := sourcecatalog.Catalog{SchemaVersion: sourcecatalog.SchemaVersion, Adapter: sourcecatalog.Adapter{Kind: "example.test.source", ContractVersion: 1}, Source: sourcecatalog.Source{RequestedExecutable: "tool", ResolvedPath: identity.ResolvedPath, SHA256: identity.SHA256, Size: identity.Size, Version: "1.0"}, Probe: sourcecatalog.Probe{IDs: []string{"help"}, Attempts: 1}, Commands: []sourcecatalog.Command{{Path: []string{"item", "list"}, Summary: "List", Provenance: sourcecatalog.ProvenanceVerifiedBuiltin, Options: []sourcecatalog.Option{}, StructuredOutput: []sourcecatalog.StructuredOutput{{Format: "json", SelectorFlag: "--json", Fields: []string{"id"}}}}}}
	cd, _ := catalog.Digest()
	specification := tailoringbundle.Specification{SchemaVersion: tailoringbundle.SpecificationSchemaVersion, CatalogDigest: cd, Surface: tailoringbundle.Surface{Default: tailoringbundle.SurfaceDefaultExclude}, Commands: []tailoringbundle.CommandEntry{{Command: []string{"item", "list"}, Presence: tailoringbundle.PresenceInclude, Reason: "needed", Options: &tailoringbundle.OptionSurface{Default: tailoringbundle.SurfaceDefaultInherit, Include: []string{}, Exclude: []string{}}, Wrapper: &tailoringbundle.Wrapper{Kind: tailoringbundle.WrapperIdentity, Before: []tailoringbundle.StageAction{}, Invoke: tailoringbundle.Invocation{AppendArgs: []string{}}, After: []tailoringbundle.StageAction{}}}}}
	bundle, _ := tailoringbundle.Compile(catalog, specification)
	digest, _ := bundle.Digest()
	return bundle, digest, identity
}

func optimizerAuthorityFixture(t *testing.T) (tailoringbundle.Bundle, string, sourceprocess.Identity, processorprocess.Identity) {
	t.Helper()
	sourceIdentity := sourceprocess.Identity{ResolvedPath: "/go", SHA256: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", Size: 100}
	processorIdentity := processorprocess.Identity{ResolvedPath: "/rtk", SHA256: "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb", Size: 200}
	catalog := sourcecatalog.Catalog{
		SchemaVersion: sourcecatalog.SchemaVersion,
		Adapter:       sourcecatalog.Adapter{Kind: "atsura.source.go_cli", ContractVersion: 2},
		Source:        sourcecatalog.Source{RequestedExecutable: "go", ResolvedPath: sourceIdentity.ResolvedPath, SHA256: sourceIdentity.SHA256, Size: sourceIdentity.Size, Version: "go1.26.5"},
		Probe:         sourcecatalog.Probe{IDs: []string{"help", "test_help", "version"}, Attempts: 3},
		Commands: []sourcecatalog.Command{{
			Path: []string{"test"}, Summary: "test packages", Provenance: sourcecatalog.ProvenanceVerifiedBuiltin,
			Options: []sourcecatalog.Option{}, StructuredOutput: []sourcecatalog.StructuredOutput{{
				Format: "go_test_jsonl", SelectorFlag: "-json", Fields: []string{"Action", "Elapsed", "FailedBuild", "Output", "Package", "Test", "Time"},
			}},
		}},
	}
	catalogDigest, err := catalog.Digest()
	if err != nil {
		t.Fatal(err)
	}
	const contract = "atsura.output.rtk_go_test_pass.v1"
	specification := tailoringbundle.Specification{
		SchemaVersion: tailoringbundle.SpecificationSchemaVersion, CatalogDigest: catalogDigest,
		Surface: tailoringbundle.Surface{Default: tailoringbundle.SurfaceDefaultExclude},
		Commands: []tailoringbundle.CommandEntry{{
			Command: []string{"test"}, Presence: tailoringbundle.PresenceInclude, Reason: "Optimize exact passing test output.",
			Options: &tailoringbundle.OptionSurface{Default: tailoringbundle.SurfaceDefaultInherit, Include: []string{}, Exclude: []string{}},
			Wrapper: &tailoringbundle.Wrapper{
				Kind: tailoringbundle.WrapperTransform, Before: []tailoringbundle.StageAction{}, Invoke: tailoringbundle.Invocation{AppendArgs: []string{"-json"}},
				Output: &tailoringbundle.Output{Kind: tailoringbundle.OutputKindOptimizer, Optimizer: &tailoringbundle.Optimizer{Input: "go_test_jsonl", Contract: contract, AllowOriginalOutput: true}},
				After:  []tailoringbundle.StageAction{},
			},
		}},
	}
	observation := processorprocess.Observation{
		SchemaVersion: processorprocess.ObservationSchemaVersion,
		Adapter:       processorprocess.Adapter{Kind: "atsura.processor.rtk", ContractVersion: 1},
		Platform:      processorprocess.Platform{OS: "darwin", Arch: "arm64"}, Identity: processorIdentity, Version: "0.43.0",
		Probe: processorprocess.Probe{Argv: []string{"--version"}, EnvironmentContract: processorprocess.EnvironmentRTKIsolatedV1, Attempts: 1},
	}
	binding := tailoringbundle.ProcessorBinding{
		Contract: contract, Observation: observation, InputFormat: "go_test_jsonl", OutputFormat: "go_test_pass_summary", AllowOriginalOutput: true,
		Execution: tailoringbundle.ProcessorExecution{
			Args: []string{"pipe", "--filter=go-test"}, StdinMode: "stage_input", WorkingDirectoryMode: "isolated", EnvironmentContract: processorprocess.EnvironmentRTKIsolatedV1,
			MaxAttempts: 1, TimeoutMillis: processorprocess.MaxTimeout.Milliseconds(), StdoutLimitBytes: processorprocess.MaxStdoutBytes, StderrLimitBytes: processorprocess.MaxStderrBytes,
		},
	}
	bundle, err := tailoringbundle.Compile(catalog, specification, binding)
	if err != nil {
		t.Fatal(err)
	}
	digest, err := bundle.Digest()
	if err != nil {
		t.Fatal(err)
	}
	return bundle, digest, sourceIdentity, processorIdentity
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
	if confirm.seen.SpecificationDigest != bundle.SpecificationDigest || confirm.seen.SurfaceDefault != "exclude" || confirm.seen.IncludedCommandCount != 1 || confirm.seen.IdentityWrapperCount != 1 || confirm.seen.TransformWrapperCount != 0 || confirm.seen.SourceStreamResultCount != 1 {
		t.Fatalf("adoption summary = %+v", confirm.seen)
	}
	confirm.err = errors.New("no")
	trust.adds = 0
	if _, err := New(bundleStub{bundle: bundle, digest: digest}, identityPort, trust, confirm).Trust(context.Background(), trustIntent(), "bundle.json"); err == nil || trust.adds != 0 {
		t.Fatalf("denied error=%v adds=%d", err, trust.adds)
	}
}

func TestStatusAndTrustExposeAndRequireExactProcessorIdentity(t *testing.T) {
	bundle, digest, sourceIdentity, processorIdentity := optimizerAuthorityFixture(t)
	source := &identityStub{identity: sourceIdentity}
	processor := &processorIdentityStub{identity: processorIdentity}
	trust := &trustStub{state: bundletrust.StateNotAdopted}
	confirm := &confirmStub{}
	service := New(bundleStub{bundle: bundle, digest: digest}, source, trust, confirm, processor)

	status, err := service.Status(context.Background(), readIntent(), "bundle.json")
	if err != nil || len(status.Processors) != 1 || status.Processors[0].State != bundletrust.ProcessorCurrent || status.ProcessorProcessAttempts != 0 || processor.calls != 1 {
		t.Fatalf("Status() = %+v, %v; processor calls=%d", status, err, processor.calls)
	}
	result, err := service.Trust(context.Background(), trustIntent(), "bundle.json")
	if err != nil || !result.Adopted || len(result.Processors) != 1 || result.Processors[0].State != bundletrust.ProcessorCurrent || result.ProcessorProcessAttempts != 0 || confirm.calls != 1 || trust.adds != 1 {
		t.Fatalf("Trust() = %+v, %v; confirms=%d adds=%d", result, err, confirm.calls, trust.adds)
	}
	if len(confirm.seen.Processors) != 1 || confirm.seen.Processors[0].ResolvedPath != processorIdentity.ResolvedPath || confirm.seen.Processors[0].SHA256 != processorIdentity.SHA256 || confirm.seen.OptimizerResultCount != 1 {
		t.Fatalf("processor adoption summary = %+v", confirm.seen)
	}

	processor.identity.SHA256 = "cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc"
	trust.adds = 0
	confirm.calls = 0
	status, err = service.Status(context.Background(), readIntent(), "bundle.json")
	if err != nil || status.Processors[0].State != bundletrust.ProcessorDrifted {
		t.Fatalf("drift status = %+v, %v", status, err)
	}
	_, err = service.Trust(context.Background(), trustIntent(), "bundle.json")
	public, ok := fault.PublicCopy(err)
	if !ok || public.Code != "bundle_processor_drift" || trust.adds != 0 || confirm.calls != 0 {
		t.Fatalf("drift trust error=%v adds=%d confirms=%d", err, trust.adds, confirm.calls)
	}
}
