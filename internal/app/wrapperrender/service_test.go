package wrapperrender

import (
	"context"
	"errors"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/tasuku43/atsura/internal/domain/bundletrust"
	"github.com/tasuku43/atsura/internal/domain/fault"
	"github.com/tasuku43/atsura/internal/domain/operation"
	"github.com/tasuku43/atsura/internal/domain/processorprocess"
	"github.com/tasuku43/atsura/internal/domain/sourcecatalog"
	"github.com/tasuku43/atsura/internal/domain/sourceprocess"
	"github.com/tasuku43/atsura/internal/domain/tailoringbundle"
	"github.com/tasuku43/atsura/internal/domain/wrapperbinding"
)

func renderIntent() operation.Intent {
	return operation.Intent{Command: Command, Effect: operation.EffectRead}
}

func renderBundle(t *testing.T, requestedExecutable string) (tailoringbundle.Bundle, string) {
	t.Helper()
	sourcePath := filepath.Join(t.TempDir(), "source", "gh")
	catalog := sourcecatalog.Sort(sourcecatalog.Catalog{
		SchemaVersion: sourcecatalog.SchemaVersion,
		Adapter:       sourcecatalog.Adapter{Kind: "atsura.source.synthetic", ContractVersion: 1},
		Source: sourcecatalog.Source{
			RequestedExecutable: requestedExecutable,
			ResolvedPath:        sourcePath,
			SHA256:              strings.Repeat("a", 64),
			Size:                2048,
			Version:             "2.72.0",
		},
		Probe: sourcecatalog.Probe{IDs: []string{"command_help"}, Attempts: 1},
		Commands: []sourcecatalog.Command{{
			Path: []string{"pr", "list"}, Summary: "List pull requests", Provenance: sourcecatalog.ProvenanceVerifiedBuiltin,
			Options:          []sourcecatalog.Option{{Name: "--json", TakesValue: true}, {Name: "--limit", TakesValue: true}},
			StructuredOutput: []sourcecatalog.StructuredOutput{{Format: "json", SelectorFlag: "--json", Fields: []string{"number", "title"}}},
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
			Command: []string{"pr", "list"}, Presence: tailoringbundle.PresenceInclude, Reason: "Return a compact reviewed result.",
			Options: &tailoringbundle.OptionSurface{Default: tailoringbundle.SurfaceDefaultExclude, Include: []string{"--limit"}, Exclude: []string{}},
			Wrapper: &tailoringbundle.Wrapper{
				Kind: tailoringbundle.WrapperTransform, Before: []tailoringbundle.StageAction{},
				Invoke: tailoringbundle.Invocation{AppendArgs: []string{"--json=number,title"}},
				Output: &tailoringbundle.Output{Kind: tailoringbundle.OutputKindProjection, Projection: &tailoringbundle.Projection{Input: "json", Select: []string{"number", "title"}, Rename: []tailoringbundle.Rename{}, Render: "compact_json"}},
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
	return bundle, digest
}

func renderProcessorBundle(t *testing.T) (tailoringbundle.Bundle, string) {
	t.Helper()
	sourcePath := filepath.Join(t.TempDir(), "source", "go")
	processorPath := filepath.Join(t.TempDir(), "processor", "rtk")
	catalog := sourcecatalog.Sort(sourcecatalog.Catalog{
		SchemaVersion: sourcecatalog.SchemaVersion,
		Adapter:       sourcecatalog.Adapter{Kind: "atsura.source.synthetic", ContractVersion: 1},
		Source: sourcecatalog.Source{
			RequestedExecutable: "go",
			ResolvedPath:        sourcePath,
			SHA256:              strings.Repeat("a", 64),
			Size:                14_500_192,
			Version:             "go1.26.5",
		},
		Probe: sourcecatalog.Probe{IDs: []string{"command_help", "version"}, Attempts: 2},
		Commands: []sourcecatalog.Command{{
			Path:       []string{"test"},
			Summary:    "Run package tests",
			Provenance: sourcecatalog.ProvenanceVerifiedBuiltin,
			Options:    []sourcecatalog.Option{},
			StructuredOutput: []sourcecatalog.StructuredOutput{{
				Format:       "go_test_jsonl",
				SelectorFlag: "-json",
				Fields:       []string{"Action", "Elapsed", "FailedBuild", "Output", "Package", "Test", "Time"},
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
			Command:  []string{"test"},
			Presence: tailoringbundle.PresenceInclude,
			Reason:   "Return a compact reviewed test result.",
			Options:  &tailoringbundle.OptionSurface{Default: tailoringbundle.SurfaceDefaultInherit, Include: []string{}, Exclude: []string{}},
			Wrapper: &tailoringbundle.Wrapper{
				Kind:   tailoringbundle.WrapperTransform,
				Before: []tailoringbundle.StageAction{},
				Invoke: tailoringbundle.Invocation{AppendArgs: []string{"-json"}},
				Output: &tailoringbundle.Output{
					Kind: tailoringbundle.OutputKindOptimizer,
					Optimizer: &tailoringbundle.Optimizer{
						Input: "go_test_jsonl", Contract: "atsura.output.rtk_go_test_pass.v1", AllowOriginalOutput: true,
					},
				},
				After: []tailoringbundle.StageAction{},
			},
		}},
	})
	processor := tailoringbundle.ProcessorBinding{
		Contract: "atsura.output.rtk_go_test_pass.v1",
		Observation: processorprocess.Observation{
			SchemaVersion: processorprocess.ObservationSchemaVersion,
			Adapter:       processorprocess.Adapter{Kind: "atsura.processor.rtk", ContractVersion: 1},
			Platform:      processorprocess.Platform{OS: "darwin", Arch: "arm64"},
			Identity: processorprocess.Identity{
				ResolvedPath: processorPath,
				SHA256:       strings.Repeat("b", 64),
				Size:         7_763_408,
			},
			Version: "0.43.0",
			Probe: processorprocess.Probe{
				Argv: []string{"--version"}, EnvironmentContract: processorprocess.EnvironmentRTKIsolatedV1, Attempts: 1,
			},
		},
		InputFormat: "go_test_jsonl", OutputFormat: "go_test_pass_summary", AllowOriginalOutput: true,
		Execution: tailoringbundle.ProcessorExecution{
			Args: []string{"pipe", "--filter=go-test"}, StdinMode: "stage_input", WorkingDirectoryMode: "isolated",
			EnvironmentContract: processorprocess.EnvironmentRTKIsolatedV1, MaxAttempts: 1, TimeoutMillis: 5_000,
			StdoutLimitBytes: processorprocess.MaxStdoutBytes, StderrLimitBytes: processorprocess.MaxStderrBytes,
		},
	}
	bundle, err := tailoringbundle.Compile(catalog, specification, processor)
	if err != nil {
		t.Fatal(err)
	}
	digest, err := bundle.Digest()
	if err != nil {
		t.Fatal(err)
	}
	return bundle, digest
}

type renderBundlePort struct {
	bundle tailoringbundle.Bundle
	digest string
	err    error
	calls  []string
}

func (p *renderBundlePort) Load(_ context.Context, path string) (tailoringbundle.Bundle, string, error) {
	p.calls = append(p.calls, path)
	return p.bundle, p.digest, p.err
}

type renderAdoptionPort struct {
	state bundletrust.State
	calls []string
}

func (p *renderAdoptionPort) Inspect(_ context.Context, digest string) bundletrust.State {
	p.calls = append(p.calls, digest)
	return p.state
}

type renderIdentityPort struct {
	identities map[string]sourceprocess.Identity
	errors     map[string]error
	calls      []string
}

func (p *renderIdentityPort) Identify(_ context.Context, locator string) (sourceprocess.Identity, error) {
	p.calls = append(p.calls, locator)
	return p.identities[locator], p.errors[locator]
}

type renderCurrentPort struct {
	locator string
	err     error
	calls   int
	events  *[]string
}

func (p *renderCurrentPort) CurrentExecutable(context.Context) (string, error) {
	p.calls++
	if p.events != nil {
		*p.events = append(*p.events, "runtime_current")
	}
	return p.locator, p.err
}

type renderCompatibilityPort struct {
	err    error
	calls  int
	bundle tailoringbundle.Bundle
}

func (p *renderCompatibilityPort) VerifySurface(bundle tailoringbundle.Bundle) error {
	p.calls++
	p.bundle = bundle
	return p.err
}

type renderMaterialPort struct {
	material wrapperbinding.RenderedMaterial
	err      error
	calls    int
	binding  wrapperbinding.Binding
	events   *[]string
}

func (p *renderMaterialPort) Render(binding wrapperbinding.Binding) (wrapperbinding.RenderedMaterial, error) {
	p.calls++
	p.binding = binding
	if p.events != nil {
		*p.events = append(*p.events, "runtime_render")
	}
	return p.material, p.err
}

type renderProcessorIdentityPort struct {
	identities map[string]processorprocess.Identity
	errors     map[string]error
	calls      []string
	events     *[]string
}

func (p *renderProcessorIdentityPort) Identify(_ context.Context, locator string) (processorprocess.Identity, error) {
	p.calls = append(p.calls, locator)
	if p.events != nil {
		*p.events = append(*p.events, "processor_identity")
	}
	return p.identities[locator], p.errors[locator]
}

type renderProcessorCompatibilityPort struct {
	err    error
	calls  int
	bundle tailoringbundle.Bundle
	events *[]string
}

func (p *renderProcessorCompatibilityPort) VerifySurface(bundle tailoringbundle.Bundle) error {
	p.calls++
	p.bundle = bundle
	if p.events != nil {
		*p.events = append(*p.events, "processor_compatibility")
	}
	return p.err
}

type renderFixture struct {
	service       *Service
	bundlePath    string
	bundle        tailoringbundle.Bundle
	digest        string
	bundles       *renderBundlePort
	adoption      *renderAdoptionPort
	identity      *renderIdentityPort
	current       *renderCurrentPort
	compatibility *renderCompatibilityPort
	renderer      *renderMaterialPort
	processorID   *renderProcessorIdentityPort
	processorComp *renderProcessorCompatibilityPort
	events        []string
}

func newRenderFixture(t *testing.T, requestedExecutable string) *renderFixture {
	t.Helper()
	bundle, digest := renderBundle(t, requestedExecutable)
	bundlePath := filepath.Join(t.TempDir(), "purpose bundle.json")
	runtimeLocator := filepath.Join(t.TempDir(), "runtime", "atr")
	material, err := wrapperbinding.NewRenderedMaterial([]byte("gh() {\n  'fixed-runtime' -- \"$@\"\n}\n"))
	if err != nil {
		t.Fatal(err)
	}
	fixture := &renderFixture{
		bundlePath: bundlePath,
		bundle:     bundle,
		digest:     digest,
		bundles:    &renderBundlePort{bundle: bundle, digest: digest},
		adoption:   &renderAdoptionPort{state: bundletrust.StateAdopted},
		identity: &renderIdentityPort{identities: map[string]sourceprocess.Identity{
			bundle.Catalog.Source.ResolvedPath: {ResolvedPath: bundle.Catalog.Source.ResolvedPath, SHA256: bundle.Catalog.Source.SHA256, Size: bundle.Catalog.Source.Size},
			runtimeLocator:                     {ResolvedPath: runtimeLocator, SHA256: strings.Repeat("b", 64), Size: 4096},
		}, errors: map[string]error{}},
		current:       &renderCurrentPort{locator: runtimeLocator},
		compatibility: &renderCompatibilityPort{},
		renderer:      &renderMaterialPort{material: material},
	}
	fixture.service = New("darwin", fixture.bundles, fixture.adoption, fixture.identity, fixture.current, fixture.compatibility, fixture.renderer)
	return fixture
}

func newProcessorRenderFixture(t *testing.T) *renderFixture {
	t.Helper()
	fixture := newRenderFixture(t, "go")
	bundle, digest := renderProcessorBundle(t)
	fixture.bundle = bundle
	fixture.digest = digest
	fixture.bundles.bundle = bundle
	fixture.bundles.digest = digest
	fixture.identity.identities[bundle.Catalog.Source.ResolvedPath] = sourceprocess.Identity{
		ResolvedPath: bundle.Catalog.Source.ResolvedPath,
		SHA256:       bundle.Catalog.Source.SHA256,
		Size:         bundle.Catalog.Source.Size,
	}
	processorIdentity := bundle.Processors[0].Observation.Identity
	fixture.processorID = &renderProcessorIdentityPort{
		identities: map[string]processorprocess.Identity{processorIdentity.ResolvedPath: processorIdentity},
		errors:     map[string]error{},
		events:     &fixture.events,
	}
	fixture.processorComp = &renderProcessorCompatibilityPort{events: &fixture.events}
	fixture.current.events = &fixture.events
	fixture.renderer.events = &fixture.events
	fixture.service = New(
		"darwin",
		fixture.bundles,
		fixture.adoption,
		fixture.identity,
		fixture.current,
		fixture.compatibility,
		fixture.renderer,
		ProcessorPorts{Identity: fixture.processorID, Compatibility: fixture.processorComp},
	)
	return fixture
}

func TestRenderProducesExactBindingAndDeterministicMaterialWithoutSourceAttempt(t *testing.T) {
	fixture := newRenderFixture(t, "gh")
	result, err := fixture.service.Render(context.Background(), renderIntent(), fixture.bundlePath)
	if err != nil {
		t.Fatal(err)
	}
	if result.SourceProcessAttempts != 0 || result.ProcessorProcessAttempts != 0 || len(fixture.bundles.calls) != 1 || len(fixture.adoption.calls) != 1 || fixture.compatibility.calls != 1 || fixture.current.calls != 1 || fixture.renderer.calls != 1 {
		t.Fatalf("unexpected call/attempt evidence: result=%+v fixture=%+v", result, fixture)
	}
	wantIdentityCalls := []string{fixture.bundle.Catalog.Source.ResolvedPath, fixture.current.locator}
	if !reflect.DeepEqual(fixture.identity.calls, wantIdentityCalls) {
		t.Fatalf("identity calls = %v, want %v", fixture.identity.calls, wantIdentityCalls)
	}
	if result.Binding.BundleLocator != fixture.bundlePath || result.Binding.BundleDigest != fixture.digest || result.Binding.CommandName != "gh" || result.Binding.Runtime.ResolvedPath != fixture.current.locator {
		t.Fatalf("binding = %+v", result.Binding)
	}
	if fixture.renderer.binding != result.Binding || !reflect.DeepEqual(fixture.compatibility.bundle, fixture.bundle) {
		t.Fatal("renderer or compatibility port received different authority")
	}
	result.Material.Source[0] = 'x'
	if fixture.renderer.material.Source[0] != 'g' {
		t.Fatal("result shared the renderer's source buffer")
	}
}

func TestRenderProcessorBoundBundleVerifiesExactIdentityAndCompatibilityBeforeRuntimeRender(t *testing.T) {
	fixture := newProcessorRenderFixture(t)
	result, err := fixture.service.Render(context.Background(), renderIntent(), fixture.bundlePath)
	if err != nil {
		t.Fatal(err)
	}
	wantedProcessorPath := fixture.bundle.Processors[0].Observation.Identity.ResolvedPath
	if !reflect.DeepEqual(fixture.processorID.calls, []string{wantedProcessorPath}) {
		t.Fatalf("processor identity calls = %v, want exact path %q", fixture.processorID.calls, wantedProcessorPath)
	}
	if fixture.processorComp.calls != 1 || !reflect.DeepEqual(fixture.processorComp.bundle, fixture.bundle) {
		t.Fatal("processor compatibility did not receive the exact bundle once")
	}
	wantEvents := []string{"processor_identity", "processor_compatibility", "runtime_current", "runtime_render"}
	if !reflect.DeepEqual(fixture.events, wantEvents) {
		t.Fatalf("boundary order = %v, want %v", fixture.events, wantEvents)
	}
	if result.SourceProcessAttempts != 0 || result.ProcessorProcessAttempts != 0 {
		t.Fatalf("render started a process: %+v", result)
	}
}

func TestRenderProcessorBoundBundleFailsClosedBeforeRuntimeRender(t *testing.T) {
	want := errors.New("synthetic processor boundary failure")
	tests := []struct {
		name              string
		mutate            func(*renderFixture)
		wantCode          string
		wantIdentity      int
		wantCompatibility int
		wantAction        fault.NextAction
	}{
		{
			name: "missing processor ports",
			mutate: func(f *renderFixture) {
				f.service = New("darwin", f.bundles, f.adoption, f.identity, f.current, f.compatibility, f.renderer)
			},
			wantCode:   "wrapper_runtime_not_supported",
			wantAction: helpAction(),
		},
		{
			name: "processor drift",
			mutate: func(f *renderFixture) {
				path := f.bundle.Processors[0].Observation.Identity.ResolvedPath
				identity := f.processorID.identities[path]
				identity.SHA256 = strings.Repeat("c", 64)
				f.processorID.identities[path] = identity
			},
			wantCode:     "bundle_processor_drift",
			wantIdentity: 1,
			wantAction:   statusAction(),
		},
		{
			name: "processor identity unavailable",
			mutate: func(f *renderFixture) {
				path := f.bundle.Processors[0].Observation.Identity.ResolvedPath
				f.processorID.errors[path] = want
			},
			wantCode:     "processor_identity_unavailable",
			wantIdentity: 1,
			wantAction:   statusAction(),
		},
		{
			name: "processor compatibility rejected",
			mutate: func(f *renderFixture) {
				f.processorComp.err = want
			},
			wantCode:          "wrapper_runtime_not_supported",
			wantIdentity:      1,
			wantCompatibility: 1,
			wantAction:        helpAction(),
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			fixture := newProcessorRenderFixture(t)
			test.mutate(fixture)
			result, err := fixture.service.Render(context.Background(), renderIntent(), fixture.bundlePath)
			public := assertRenderFault(t, err, test.wantCode)
			if len(public.NextActions) != 1 || public.NextActions[0] != test.wantAction {
				t.Fatalf("recovery = %+v, want %+v", public.NextActions, test.wantAction)
			}
			if len(fixture.processorID.calls) != test.wantIdentity || fixture.processorComp.calls != test.wantCompatibility {
				t.Fatalf("processor calls = identity %d, compatibility %d; want %d, %d", len(fixture.processorID.calls), fixture.processorComp.calls, test.wantIdentity, test.wantCompatibility)
			}
			if fixture.current.calls != 0 || fixture.renderer.calls != 0 {
				t.Fatalf("failed processor validation reached runtime: current=%d renderer=%d", fixture.current.calls, fixture.renderer.calls)
			}
			if result.SourceProcessAttempts != 0 || result.ProcessorProcessAttempts != 0 || len(result.Material.Source) != 0 {
				t.Fatalf("failed render returned material or attempts: %+v", result)
			}
		})
	}
}

func TestRenderRejectsBeforeLaterBoundaries(t *testing.T) {
	want := errors.New("synthetic boundary failure")
	tests := []struct {
		name      string
		mutate    func(*renderFixture)
		wantCode  string
		wantCalls [6]int // load, adoption, identity, compatibility, current, renderer
	}{
		{name: "relative locator", mutate: func(f *renderFixture) { f.bundlePath = "bundle.json" }, wantCode: "invalid_wrapper_binding"},
		{name: "unsupported platform", mutate: func(f *renderFixture) { f.service.platform = "windows" }, wantCode: "wrapper_platform_not_supported"},
		{name: "load failure", mutate: func(f *renderFixture) { f.bundles.err = want }, wantCode: "internal_error", wantCalls: [6]int{1}},
		{name: "not adopted", mutate: func(f *renderFixture) { f.adoption.state = bundletrust.StateNotAdopted }, wantCode: "bundle_not_adopted", wantCalls: [6]int{1, 1}},
		{name: "invalid adoption store", mutate: func(f *renderFixture) { f.adoption.state = bundletrust.StateInvalid }, wantCode: "invalid_bundle_trust_store", wantCalls: [6]int{1, 1}},
		{name: "source drift", mutate: func(f *renderFixture) {
			identity := f.identity.identities[f.bundle.Catalog.Source.ResolvedPath]
			identity.SHA256 = strings.Repeat("c", 64)
			f.identity.identities[f.bundle.Catalog.Source.ResolvedPath] = identity
		}, wantCode: "bundle_source_drift", wantCalls: [6]int{1, 1, 1}},
		{name: "surface unsupported", mutate: func(f *renderFixture) { f.compatibility.err = want }, wantCode: "wrapper_runtime_not_supported", wantCalls: [6]int{1, 1, 1, 1}},
		{name: "runtime locator unavailable", mutate: func(f *renderFixture) { f.current.err = want }, wantCode: "wrapper_runtime_unavailable", wantCalls: [6]int{1, 1, 1, 1, 1}},
		{name: "runtime identity unavailable", mutate: func(f *renderFixture) { f.identity.errors[f.current.locator] = want }, wantCode: "wrapper_runtime_unavailable", wantCalls: [6]int{1, 1, 2, 1, 1}},
		{name: "renderer failure", mutate: func(f *renderFixture) { f.renderer.err = want }, wantCode: "wrapper_render_failed", wantCalls: [6]int{1, 1, 2, 1, 1, 1}},
		{name: "invalid rendered result", mutate: func(f *renderFixture) { f.renderer.material.SHA256 = strings.Repeat("c", 64) }, wantCode: "wrapper_render_failed", wantCalls: [6]int{1, 1, 2, 1, 1, 1}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			fixture := newRenderFixture(t, "gh")
			test.mutate(fixture)
			result, err := fixture.service.Render(context.Background(), renderIntent(), fixture.bundlePath)
			public := assertRenderFault(t, err, test.wantCode)
			if test.wantCode == "wrapper_runtime_not_supported" && (len(public.NextActions) != 1 || public.NextActions[0] != helpAction()) {
				t.Fatalf("runtime recovery=%+v want=%+v", public.NextActions, helpAction())
			}
			if result.SourceProcessAttempts != 0 || result.ProcessorProcessAttempts != 0 || len(result.Material.Source) != 0 {
				t.Fatalf("failed render returned material or attempts: %+v", result)
			}
			got := [6]int{len(fixture.bundles.calls), len(fixture.adoption.calls), len(fixture.identity.calls), fixture.compatibility.calls, fixture.current.calls, fixture.renderer.calls}
			if got != test.wantCalls {
				t.Fatalf("calls = %v, want %v", got, test.wantCalls)
			}
		})
	}
}

func TestRenderRejectsUnsafeCommandNameAfterAuthorityChecks(t *testing.T) {
	fixture := newRenderFixture(t, "path/to/gh")
	_, err := fixture.service.Render(context.Background(), renderIntent(), fixture.bundlePath)
	assertRenderFault(t, err, "invalid_wrapper_binding")
	if fixture.renderer.calls != 0 || fixture.current.calls != 1 || len(fixture.identity.calls) != 2 {
		t.Fatalf("unexpected calls: identity=%v current=%d renderer=%d", fixture.identity.calls, fixture.current.calls, fixture.renderer.calls)
	}
}

func TestRenderRejectsWrongIntentCanceledContextAndMissingPorts(t *testing.T) {
	fixture := newRenderFixture(t, "gh")
	if _, err := fixture.service.Render(context.Background(), operation.Intent{Command: Command, Effect: operation.EffectExecute}, fixture.bundlePath); err == nil {
		t.Fatal("execute intent succeeded")
	}
	if _, err := fixture.service.Render(nil, renderIntent(), fixture.bundlePath); err == nil {
		t.Fatal("nil context succeeded")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := fixture.service.Render(ctx, renderIntent(), fixture.bundlePath); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled error = %v", err)
	}
	fixture.service.renderer = (*renderMaterialPort)(nil)
	if _, err := fixture.service.Render(context.Background(), renderIntent(), fixture.bundlePath); err == nil {
		t.Fatal("typed-nil renderer succeeded")
	}
	if len(fixture.bundles.calls) != 0 {
		t.Fatalf("preflight failures loaded bundle: %v", fixture.bundles.calls)
	}
}

func assertRenderFault(t *testing.T, err error, code string) *fault.Error {
	t.Helper()
	public, ok := fault.PublicCopy(err)
	if !ok || public.Code != code || public.Retryable {
		t.Fatalf("fault = %#v, err=%v; want code=%s non-retryable", public, err, code)
	}
	return public
}
