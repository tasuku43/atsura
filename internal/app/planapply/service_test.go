package planapply

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tasuku43/atsura/internal/domain/bundletrust"
	"github.com/tasuku43/atsura/internal/domain/fault"
	"github.com/tasuku43/atsura/internal/domain/sourcecatalog"
	"github.com/tasuku43/atsura/internal/domain/sourceprocess"
	"github.com/tasuku43/atsura/internal/domain/tailoring"
	"github.com/tasuku43/atsura/internal/domain/tailoringbundle"
	"github.com/tasuku43/atsura/internal/domain/tailoringplan"
)

type bundleStub struct {
	bundle tailoringbundle.Bundle
	digest string
	err    error
	calls  int
	path   string
}

func (s *bundleStub) Load(_ context.Context, path string) (tailoringbundle.Bundle, string, error) {
	s.calls++
	s.path = path
	return s.bundle, s.digest, s.err
}

type adoptionStub struct {
	state bundletrust.State
	calls int
}

func (s *adoptionStub) Inspect(context.Context, string) bundletrust.State {
	s.calls++
	return s.state
}

type identityStub struct {
	value sourceprocess.Identity
	calls int
}

func (s *identityStub) Identify(context.Context, string) (sourceprocess.Identity, error) {
	s.calls++
	return s.value, nil
}

type compatibilityStub struct{ calls int }

func (s *compatibilityStub) VerifyRuntime(tailoringplan.Plan) error {
	s.calls++
	return nil
}

type processStub struct {
	result  sourceprocess.Result
	err     error
	calls   int
	after   func()
	request sourceprocess.BoundRequest
}

func (s *processStub) RunBound(_ context.Context, request sourceprocess.BoundRequest) (sourceprocess.Result, error) {
	s.calls++
	s.request = request
	if s.after != nil {
		s.after()
	}
	return s.result, s.err
}

type parserStub struct {
	value tailoring.JSONValue
	calls int
}

func (s *parserStub) Parse(context.Context, []byte) (tailoring.JSONValue, error) {
	s.calls++
	return s.value, nil
}

func testCommandContext() CommandContext {
	return CommandContext{
		LoadFailureMessage:      "The fixture could not load its bundle.",
		RuntimeHelpAction:       fault.NextAction{Command: "fixture help", Reason: "Review the fixture runtime contract."},
		PlanPreviewAction:       fault.NextAction{Command: "fixture preview", Reason: "Inspect the fixture plan."},
		StatusAction:            fault.NextAction{Command: "fixture status", Reason: "Reconcile the fixture state."},
		TrustAction:             fault.NextAction{Command: "fixture trust", Reason: "Adopt the fixture bundle."},
		ProcessStartRetryAction: fault.NextAction{Command: "fixture run", Reason: "Retry only after proving zero attempts."},
		BundleMismatchAction:    fault.NextAction{Command: "fixture binding", Reason: "Reconcile the exact fixture binding."},
	}
}

func TestApplyRejectsExpectedBundleDigestMismatchImmediatelyAfterLoad(t *testing.T) {
	loadedDigest := strings.Repeat("a", 64)
	loader := &bundleStub{digest: loadedDigest}
	adoption := &adoptionStub{state: bundletrust.StateAdopted}
	identity := &identityStub{}
	compatibility := &compatibilityStub{}
	process := &processStub{}
	parser := &parserStub{}
	command := testCommandContext()

	result, err := New(loader, adoption, identity, compatibility, process, parser).Apply(context.Background(), Request{
		BundlePath:           "purpose.bundle.json",
		ExpectedBundleDigest: strings.Repeat("b", 64),
		Command:              command,
	})
	public, ok := fault.PublicCopy(err)
	if !ok || public.Kind != fault.KindRejected || public.Code != "bundle_binding_mismatch" || public.Retryable {
		t.Fatalf("result=%+v error=%v public=%+v", result, err, public)
	}
	if len(public.NextActions) != 1 || public.NextActions[0] != command.BundleMismatchAction {
		t.Fatalf("next actions=%+v", public.NextActions)
	}
	if loader.calls != 1 || loader.path != "purpose.bundle.json" || adoption.calls != 0 || identity.calls != 0 || compatibility.calls != 0 || process.calls != 0 || parser.calls != 0 {
		t.Fatalf("calls load/adoption/identity/compatibility/process/parser=%d/%d/%d/%d/%d/%d path=%q", loader.calls, adoption.calls, identity.calls, compatibility.calls, process.calls, parser.calls, loader.path)
	}
	if result.SourceProcessAttempts != 0 || strings.Contains(public.Message, loadedDigest) {
		t.Fatalf("result=%+v public=%+v", result, public)
	}
}

func TestApplyAcceptsExactExpectedBundleDigestThroughOneSuccessfulAttempt(t *testing.T) {
	resolvedPath := filepath.Join(t.TempDir(), "fixture")
	identity := sourceprocess.Identity{ResolvedPath: resolvedPath, SHA256: strings.Repeat("a", 64), Size: 42}
	catalog := sourcecatalog.Catalog{
		SchemaVersion: sourcecatalog.SchemaVersion,
		Adapter:       sourcecatalog.Adapter{Kind: "atsura.source.alternate", ContractVersion: 1},
		Source: sourcecatalog.Source{
			RequestedExecutable: "fixture",
			ResolvedPath:        identity.ResolvedPath,
			SHA256:              identity.SHA256,
			Size:                identity.Size,
			Version:             "1.0.0",
		},
		Probe: sourcecatalog.Probe{IDs: []string{"help"}, Attempts: 1},
		Commands: []sourcecatalog.Command{{
			Path:       []string{"item", "list"},
			Summary:    "List items",
			Provenance: sourcecatalog.ProvenanceVerifiedBuiltin,
			Options:    []sourcecatalog.Option{{Name: "--json", TakesValue: true}},
			StructuredOutput: []sourcecatalog.StructuredOutput{{
				Format: "json", SelectorFlag: "--json", Fields: []string{"id", "name"},
			}},
		}},
	}
	catalogDigest, err := catalog.Digest()
	if err != nil {
		t.Fatal(err)
	}
	bundle, err := tailoringbundle.Compile(catalog, tailoringbundle.Specification{
		SchemaVersion: tailoringbundle.SpecificationSchemaVersion,
		CatalogDigest: catalogDigest,
		Surface:       tailoringbundle.Surface{Default: tailoringbundle.SurfaceDefaultExclude},
		Commands: []tailoringbundle.CommandEntry{{
			Command:  []string{"item", "list"},
			Presence: tailoringbundle.PresenceInclude,
			Reason:   "Return a compact item list.",
			Options: &tailoringbundle.OptionSurface{
				Default: tailoringbundle.SurfaceDefaultInherit,
				Include: []string{},
				Exclude: []string{},
			},
			Wrapper: &tailoringbundle.Wrapper{
				Kind:   tailoringbundle.WrapperTransform,
				Before: []tailoringbundle.StageAction{},
				Invoke: tailoringbundle.Invocation{AppendArgs: []string{"--json=id,name"}},
				Output: &tailoringbundle.Output{Kind: tailoringbundle.OutputKindProjection, Projection: &tailoringbundle.Projection{
					Input:  "json",
					Select: []string{"id", "name"},
					Rename: []tailoringbundle.Rename{{From: "id", To: "item_id"}},
					Render: "compact_json",
				}},
				After: []tailoringbundle.StageAction{},
			},
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	digest, err := bundle.Digest()
	if err != nil {
		t.Fatal(err)
	}

	loader := &bundleStub{bundle: bundle, digest: digest}
	adoption := &adoptionStub{state: bundletrust.StateAdopted}
	identities := &identityStub{value: identity}
	compatibility := &compatibilityStub{}
	process := &processStub{result: sourceprocess.Result{
		Attempts: 1,
		ExitCode: 0,
		Stdout:   []byte(`private source bytes`),
		Identity: identity,
	}}
	parser := &parserStub{value: tailoring.NewJSONArray([]tailoring.JSONValue{
		tailoring.NewJSONObject([]tailoring.JSONField{
			{Name: "id", Value: tailoring.NewJSONNumber("7")},
			{Name: "name", Value: tailoring.NewJSONString("item")},
		}),
	})}

	result, err := New(loader, adoption, identities, compatibility, process, parser).Apply(context.Background(), Request{
		BundlePath:           "purpose.bundle.json",
		ExpectedBundleDigest: digest,
		Attempt: tailoringplan.Attempt{
			Executable: "fixture",
			Args:       []string{"item", "list"},
		},
		Command: testCommandContext(),
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.BundleDigest != digest || result.SourceProcessAttempts != 1 || result.ResultMode != tailoringplan.ResultModeTransformedJSON || result.TransformedJSON == nil || result.SourceStream != nil || result.TransformedJSON.Output.Shape != tailoring.ResultShapeArray || strings.Join(result.TransformedJSON.Output.Fields, ",") != "item_id,name" {
		t.Fatalf("result=%+v", result)
	}
	if err := result.Validate(); err != nil {
		t.Fatalf("result validation: %v", err)
	}
	if loader.calls != 1 || adoption.calls != 1 || identities.calls != 1 || compatibility.calls != 1 || process.calls != 1 || parser.calls != 1 {
		t.Fatalf("calls load/adoption/identity/compatibility/process/parser=%d/%d/%d/%d/%d/%d", loader.calls, adoption.calls, identities.calls, compatibility.calls, process.calls, parser.calls)
	}
}

func TestApplyAllowsNoExpectedDigestAndUsesCallerRecoveryContext(t *testing.T) {
	loader := &bundleStub{digest: strings.Repeat("a", 64)}
	adoption := &adoptionStub{state: bundletrust.StateNotAdopted}
	identity := &identityStub{}
	compatibility := &compatibilityStub{}
	process := &processStub{}
	parser := &parserStub{}
	command := testCommandContext()

	result, err := New(loader, adoption, identity, compatibility, process, parser).Apply(context.Background(), Request{
		BundlePath: "purpose.bundle.json",
		Command:    command,
	})
	public, ok := fault.PublicCopy(err)
	if !ok || public.Code != "bundle_not_adopted" || public.Kind != fault.KindRejected || public.Retryable {
		t.Fatalf("result=%+v error=%v public=%+v", result, err, public)
	}
	if len(public.NextActions) != 1 || public.NextActions[0] != command.TrustAction {
		t.Fatalf("next actions=%+v", public.NextActions)
	}
	if loader.calls != 1 || adoption.calls != 1 || identity.calls != 0 || compatibility.calls != 0 || process.calls != 0 || parser.calls != 0 || result.SourceProcessAttempts != 0 {
		t.Fatalf("result=%+v calls load/adoption/identity/compatibility/process/parser=%d/%d/%d/%d/%d/%d", result, loader.calls, adoption.calls, identity.calls, compatibility.calls, process.calls, parser.calls)
	}
}

func TestApplyRejectsInvalidCommandContextBeforeLoading(t *testing.T) {
	loader := &bundleStub{digest: strings.Repeat("a", 64)}
	service := New(loader, &adoptionStub{}, &identityStub{}, &compatibilityStub{}, &processStub{}, &parserStub{})
	if _, err := service.Apply(context.Background(), Request{}); err == nil || !strings.Contains(err.Error(), "invalid plan application command context") {
		t.Fatalf("error=%v", err)
	}
	if loader.calls != 0 {
		t.Fatalf("load calls=%d", loader.calls)
	}
}
