package planapply

import (
	"context"
	"errors"
	"reflect"
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

type deriveBundlePort struct {
	bundle tailoringbundle.Bundle
	digest string
	calls  int
}

func (p *deriveBundlePort) Load(context.Context, string) (tailoringbundle.Bundle, string, error) {
	p.calls++
	return p.bundle, p.digest, nil
}

type deriveAdoptionPort struct{ calls int }

func (p *deriveAdoptionPort) Inspect(context.Context, string) bundletrust.State {
	p.calls++
	return bundletrust.StateAdopted
}

type deriveIdentityPort struct {
	value sourceprocess.Identity
	calls int
}

func (p *deriveIdentityPort) Identify(context.Context, string) (sourceprocess.Identity, error) {
	p.calls++
	return p.value, nil
}

type deriveCompatibilityPort struct {
	plan  tailoringplan.Plan
	calls int
}

func (p *deriveCompatibilityPort) VerifyRuntime(plan tailoringplan.Plan) error {
	p.calls++
	p.plan = plan
	return errors.New("stop before source process")
}

type deriveProcessPort struct{ calls int }

func (p *deriveProcessPort) RunBound(context.Context, sourceprocess.BoundRequest) (sourceprocess.Result, error) {
	p.calls++
	return sourceprocess.Result{}, nil
}

type deriveParserPort struct{}

func (*deriveParserPort) Parse(context.Context, []byte) (tailoring.JSONValue, error) {
	return tailoring.JSONValue{}, nil
}

func deriveExecutableBundle(t *testing.T) (tailoringbundle.Bundle, string, sourceprocess.Identity) {
	t.Helper()
	catalog := sourcecatalog.Sort(sourcecatalog.Catalog{
		SchemaVersion: sourcecatalog.SchemaVersion,
		Adapter:       sourcecatalog.Adapter{Kind: "atsura.source.synthetic", ContractVersion: 1},
		Source: sourcecatalog.Source{
			RequestedExecutable: "exact_source_spelling",
			ResolvedPath:        "/opt/source/bin/not-the-command-basename",
			SHA256:              strings.Repeat("a", 64),
			Size:                42,
			Version:             "1.0.0",
		},
		Probe: sourcecatalog.Probe{IDs: []string{"command_help"}, Attempts: 1},
		Commands: []sourcecatalog.Command{{
			Path:       []string{"item", "list"},
			Summary:    "List synthetic items.",
			Provenance: sourcecatalog.ProvenanceVerifiedBuiltin,
			Options:    []sourcecatalog.Option{{Name: "--json", TakesValue: true}},
			StructuredOutput: []sourcecatalog.StructuredOutput{{
				Format: "json", SelectorFlag: "--json", Fields: []string{"id", "name"},
			}},
		}},
	})
	catalogDigest, err := catalog.Digest()
	if err != nil {
		t.Fatal(err)
	}
	specification := tailoringbundle.SortSpecification(tailoringbundle.Specification{
		SchemaVersion: tailoringbundle.SpecificationSchemaVersion,
		CatalogDigest: catalogDigest,
		Surface:       tailoringbundle.Surface{Default: tailoringbundle.SurfaceDefaultExclude},
		Commands: []tailoringbundle.CommandEntry{{
			Command:  []string{"item", "list"},
			Presence: tailoringbundle.PresenceInclude,
			Reason:   "Exercise exact bundle-derived executable spelling.",
			Options: &tailoringbundle.OptionSurface{
				Default: tailoringbundle.SurfaceDefaultInherit,
				Include: []string{},
				Exclude: []string{},
			},
			Wrapper: &tailoringbundle.Wrapper{
				Kind:   tailoringbundle.WrapperTransform,
				Before: []tailoringbundle.StageAction{},
				Invoke: tailoringbundle.Invocation{AppendArgs: []string{"--json=id,name"}},
				Output: &tailoringbundle.Output{Input: "json", Select: []string{"id", "name"}, Rename: []tailoringbundle.Rename{}, Render: string(tailoring.RenderCompactJSON)},
				After:  []tailoringbundle.StageAction{},
			},
		}},
	})
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

func TestApplyDerivesExactExecutableFromTheStrictlyLoadedBundle(t *testing.T) {
	bundle, digest, identity := deriveExecutableBundle(t)
	bundles := &deriveBundlePort{bundle: bundle, digest: digest}
	adoption := &deriveAdoptionPort{}
	identities := &deriveIdentityPort{value: identity}
	compatibility := &deriveCompatibilityPort{}
	processes := &deriveProcessPort{}
	args := []string{"item", "list"}

	_, err := New(bundles, adoption, identities, compatibility, processes, &deriveParserPort{}).Apply(context.Background(), Request{
		BundlePath:                       "/opt/atsura/bundles/purpose.json",
		ExpectedBundleDigest:             digest,
		DeriveExecutableFromLoadedBundle: true,
		Attempt:                          tailoringplan.Attempt{Args: args},
		Command:                          testCommandContext(),
	})
	public, ok := fault.PublicCopy(err)
	if !ok || public.Code != "wrapper_runtime_not_supported" {
		t.Fatalf("error=%v public=%+v", err, public)
	}
	if bundles.calls != 1 || adoption.calls != 1 || identities.calls != 1 || compatibility.calls != 1 || processes.calls != 0 {
		t.Fatalf("load/adoption/identity/compatibility/process calls=%d/%d/%d/%d/%d", bundles.calls, adoption.calls, identities.calls, compatibility.calls, processes.calls)
	}
	wantOriginal := append([]string{bundle.Catalog.Source.RequestedExecutable}, args...)
	if !reflect.DeepEqual(compatibility.plan.OriginalArgv, wantOriginal) {
		t.Fatalf("original argv=%#v, want %#v", compatibility.plan.OriginalArgv, wantOriginal)
	}
	if compatibility.plan.OriginalArgv[0] == "not-the-command-basename" || compatibility.plan.OriginalArgv[0] == bundle.Catalog.Source.ResolvedPath {
		t.Fatalf("bundle executable spelling was basename/path-normalized: %#v", compatibility.plan.OriginalArgv)
	}
	directPlan, err := tailoringplan.Build(digest, bundle, identity, tailoringplan.Attempt{
		Executable: bundle.Catalog.Source.RequestedExecutable,
		Args:       append([]string{}, args...),
	})
	if err != nil {
		t.Fatal(err)
	}
	directDigest, err := directPlan.Digest()
	if err != nil {
		t.Fatal(err)
	}
	derivedDigest, err := compatibility.plan.Digest()
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(compatibility.plan, directPlan) || derivedDigest != directDigest {
		t.Fatalf("bundle-derived plan/digest drifted from direct preview construction: derived=%s direct=%s", derivedDigest, directDigest)
	}
}

func TestApplyRejectsSimultaneousDerivedAndSuppliedExecutableBeforeLoading(t *testing.T) {
	bundles := &deriveBundlePort{}
	adoption := &deriveAdoptionPort{}
	identities := &deriveIdentityPort{}
	compatibility := &deriveCompatibilityPort{}
	processes := &deriveProcessPort{}

	result, err := New(bundles, adoption, identities, compatibility, processes, &deriveParserPort{}).Apply(context.Background(), Request{
		DeriveExecutableFromLoadedBundle: true,
		Attempt: tailoringplan.Attempt{
			Executable: "caller-supplied",
			Args:       []string{"item", "list"},
		},
		Command: testCommandContext(),
	})
	public, ok := fault.PublicCopy(err)
	if !ok || public.Kind != fault.KindInvalidInput || public.Code != "invalid_invocation" || public.Retryable {
		t.Fatalf("result=%+v error=%v public=%+v", result, err, public)
	}
	if bundles.calls != 0 || adoption.calls != 0 || identities.calls != 0 || compatibility.calls != 0 || processes.calls != 0 || result.SourceProcessAttempts != 0 {
		t.Fatalf("result=%+v load/adoption/identity/compatibility/process calls=%d/%d/%d/%d/%d", result, bundles.calls, adoption.calls, identities.calls, compatibility.calls, processes.calls)
	}
}
