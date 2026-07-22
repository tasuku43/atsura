package wrappershimcmd

import (
	"context"
	"errors"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/tasuku43/atsura/internal/domain/fault"
	"github.com/tasuku43/atsura/internal/domain/operation"
	"github.com/tasuku43/atsura/internal/domain/wrapperbinding"
	"github.com/tasuku43/atsura/internal/domain/wrappershim"
)

type fakeMaterializer struct {
	binding wrapperbinding.Binding
	err     error
	calls   int
	path    string
}

func (f *fakeMaterializer) Materialize(_ context.Context, path string) (wrapperbinding.Binding, error) {
	f.calls++
	f.path = path
	return f.binding.Clone(), f.err
}

type fakeRenderer struct {
	material wrapperbinding.RenderedMaterial
	err      error
	calls    int
	binding  wrapperbinding.Binding
}

func (f *fakeRenderer) Render(binding wrapperbinding.Binding) (wrapperbinding.RenderedMaterial, error) {
	f.calls++
	f.binding = binding.Clone()
	return f.material.Clone(), f.err
}

type fakeStore struct {
	binPath          string
	binErr           error
	installRecord    wrappershim.Record
	alreadyInstalled bool
	installErr       error
	status           wrappershim.Inventory
	statusErr        error
	removeRecord     wrappershim.Record
	removeErr        error
	binCalls         int
	installCalls     int
	statusCalls      int
	removeCalls      int
	manifest         wrappershim.Manifest
	shim             []byte
	reference        wrappershim.Reference
}

func (f *fakeStore) BinPath() (string, error) {
	f.binCalls++
	return f.binPath, f.binErr
}

func (f *fakeStore) Install(_ context.Context, manifest wrappershim.Manifest, shim []byte) (wrappershim.Record, bool, error) {
	f.installCalls++
	f.manifest = manifest.Clone()
	f.shim = append([]byte(nil), shim...)
	return f.installRecord, f.alreadyInstalled, f.installErr
}

func (f *fakeStore) Status(context.Context) (wrappershim.Inventory, error) {
	f.statusCalls++
	return f.status, f.statusErr
}

func (f *fakeStore) Remove(_ context.Context, reference wrappershim.Reference) (wrappershim.Record, error) {
	f.removeCalls++
	f.reference = reference
	return f.removeRecord, f.removeErr
}

func testBinding(t *testing.T) wrapperbinding.Binding {
	t.Helper()
	root := t.TempDir()
	return wrapperbinding.Binding{
		ContractVersion: wrapperbinding.ContractVersion,
		BundleLocator:   filepath.Join(root, "purpose.json"),
		BundleDigest:    strings.Repeat("a", 64),
		CommandName:     "gh",
		Help: wrapperbinding.CompiledHelp{Commands: []wrapperbinding.HelpCommand{{
			Path: []string{"pr", "list"}, Summary: "List pull requests", Reason: "Return a reviewed result.", Options: []wrapperbinding.HelpOption{},
		}}},
		Runtime: wrapperbinding.RuntimeIdentity{ResolvedPath: filepath.Join(root, "atr"), SHA256: strings.Repeat("b", 64), Size: 4096},
	}
}

func testMaterial(t *testing.T) wrapperbinding.RenderedMaterial {
	t.Helper()
	material, err := wrapperbinding.NewRenderedMaterial([]byte("#!/bin/sh\nexec '/fixed/atr' wrapper run -- \"$@\"\n"))
	if err != nil {
		t.Fatal(err)
	}
	return material
}

func testManifest(t *testing.T) wrappershim.Manifest {
	t.Helper()
	manifest, err := wrappershim.NewManifest(testBinding(t), testMaterial(t))
	if err != nil {
		t.Fatal(err)
	}
	return manifest
}

func installIntent() operation.Intent {
	return operation.Intent{
		Command: InstallCommand, Effect: operation.EffectCreate,
		Target: operation.TargetRef{Kind: StoreTargetKind, ParentID: StoreTargetID}, Impact: InstallImpact,
	}
}

func statusIntent() operation.Intent {
	return operation.Intent{Command: StatusCommand, Effect: operation.EffectRead}
}

func removeIntent(reference wrappershim.Reference) operation.Intent {
	return operation.Intent{
		Command: RemoveCommand, Effect: operation.EffectWrite,
		Target: operation.TargetRef{Kind: ArtifactRefKind, ID: reference.String()}, Impact: RemoveImpact,
	}
}

func newInstallService(t *testing.T) (*Service, *fakeMaterializer, *fakeRenderer, *fakeStore, wrappershim.Manifest) {
	t.Helper()
	binding := testBinding(t)
	material := testMaterial(t)
	manifest, err := wrappershim.NewManifest(binding, material)
	if err != nil {
		t.Fatal(err)
	}
	materializer := &fakeMaterializer{binding: binding}
	renderer := &fakeRenderer{material: material}
	store := &fakeStore{
		binPath: filepath.Join(t.TempDir(), "bin"),
		installRecord: wrappershim.Record{
			CommandName: binding.CommandName, State: wrappershim.StateOwnedActive,
			Reference: manifest.Reference, MaterialSHA256: manifest.MaterialSHA256,
		},
		status: wrappershim.Inventory{Records: []wrappershim.Record{}, Collisions: []wrappershim.Record{}},
	}
	return New("linux", materializer, renderer, store), materializer, renderer, store, manifest
}

func TestInstallUsesExactMaterializationAndCentralMutationBoundary(t *testing.T) {
	service, materializer, renderer, store, manifest := newInstallService(t)
	result, err := service.Install(context.Background(), installIntent(), manifest.Binding.BundleLocator)
	if err != nil {
		t.Fatal(err)
	}
	if materializer.calls != 1 || renderer.calls != 1 || store.installCalls != 1 || materializer.path != manifest.Binding.BundleLocator {
		t.Fatalf("calls materialize/render/install=%d/%d/%d path=%q", materializer.calls, renderer.calls, store.installCalls, materializer.path)
	}
	if !renderer.binding.Equal(manifest.Binding) || !store.manifest.Equal(manifest) || !reflect.DeepEqual(store.shim, testMaterial(t).Source) {
		t.Fatalf("binding/manifest/shim drifted: %+v %+v %q", renderer.binding, store.manifest, store.shim)
	}
	if result.CommandName != "gh" || result.BinPath != store.binPath || result.Path != filepath.Join(store.binPath, "gh") || result.AlreadyInstalled || result.SourceProcessAttempts != 0 || result.ProcessorProcessAttempts != 0 {
		t.Fatalf("result = %+v", result)
	}
	store.shim[0] = 'x'
	if testMaterial(t).Source[0] != '#' {
		t.Fatal("store received caller-owned shim bytes")
	}
}

func TestInstallReportsExactIdempotentArtifactWithoutClaimingReplacement(t *testing.T) {
	service, _, _, store, manifest := newInstallService(t)
	store.alreadyInstalled = true
	result, err := service.Install(context.Background(), installIntent(), manifest.Binding.BundleLocator)
	if err != nil || !result.AlreadyInstalled || store.installCalls != 1 {
		t.Fatalf("result=%+v err=%v calls=%d", result, err, store.installCalls)
	}
}

func TestInstallRejectsMutationContractBeforeStoreAction(t *testing.T) {
	service, materializer, renderer, store, manifest := newInstallService(t)
	intent := installIntent()
	intent.Target.ParentID = "other"
	_, err := service.Install(context.Background(), intent, manifest.Binding.BundleLocator)
	assertFault(t, err, "invalid_mutation_contract", fault.KindContract, false)
	if materializer.calls != 1 || renderer.calls != 1 || store.installCalls != 0 {
		t.Fatalf("calls materialize/render/install=%d/%d/%d", materializer.calls, renderer.calls, store.installCalls)
	}
}

func TestInstallClassifiesRendererAndStoreOutcomes(t *testing.T) {
	t.Run("renderer", func(t *testing.T) {
		service, _, renderer, store, manifest := newInstallService(t)
		renderer.err = errors.New("render")
		_, err := service.Install(context.Background(), installIntent(), manifest.Binding.BundleLocator)
		assertFault(t, err, "wrapper_artifact_render_failed", fault.KindContract, false)
		if store.installCalls != 0 {
			t.Fatalf("install calls=%d", store.installCalls)
		}
	})
	t.Run("collision", func(t *testing.T) {
		service, _, _, store, manifest := newInstallService(t)
		store.installErr = wrappershim.ErrConflict
		_, err := service.Install(context.Background(), installIntent(), manifest.Binding.BundleLocator)
		assertFault(t, err, "wrapper_artifact_collision", fault.KindRejected, false)
	})
	t.Run("unknown post action", func(t *testing.T) {
		service, _, _, store, manifest := newInstallService(t)
		store.installErr = errors.New("unknown")
		_, err := service.Install(context.Background(), installIntent(), manifest.Binding.BundleLocator)
		assertFault(t, err, "unclassified_mutation_outcome", fault.KindContract, false)
	})
	t.Run("uncertain", func(t *testing.T) {
		service, _, _, store, manifest := newInstallService(t)
		store.installErr = wrappershim.ErrUncertain
		_, err := service.Install(context.Background(), installIntent(), manifest.Binding.BundleLocator)
		assertFault(t, err, "wrapper_artifact_mutation_uncertain", fault.KindUnavailable, false)
	})
}

func TestStatusReturnsOnlyCompleteOwnedOpaqueReferences(t *testing.T) {
	service, _, _, store, manifest := newInstallService(t)
	secondReference, _ := wrappershim.NewReference(strings.Repeat("c", 64))
	store.status = wrappershim.Inventory{Records: []wrappershim.Record{
		{CommandName: "gh", State: wrappershim.StateOwnedActive, Reference: manifest.Reference, MaterialSHA256: manifest.MaterialSHA256},
		{CommandName: "go", State: wrappershim.StateOwnedInactive, Reference: secondReference, MaterialSHA256: strings.Repeat("c", 64)},
	}, Collisions: []wrappershim.Record{}}
	result, err := service.Status(context.Background(), statusIntent())
	if err != nil {
		t.Fatal(err)
	}
	want := []Artifact{
		{Reference: manifest.Reference, CommandName: "gh", State: wrappershim.StateOwnedActive, Path: filepath.Join(store.binPath, "gh"), MaterialSHA256: manifest.MaterialSHA256},
		{Reference: secondReference, CommandName: "go", State: wrappershim.StateOwnedInactive, Path: filepath.Join(store.binPath, "go"), MaterialSHA256: strings.Repeat("c", 64)},
	}
	if !reflect.DeepEqual(result.Artifacts, want) || store.statusCalls != 1 {
		t.Fatalf("result=%+v want=%+v calls=%d", result, want, store.statusCalls)
	}
}

func TestStatusFailsClosedOnCollisionTamperAndInvalidInventory(t *testing.T) {
	service, _, _, store, manifest := newInstallService(t)
	tests := []struct {
		name      string
		inventory wrappershim.Inventory
		code      string
		kind      fault.Kind
	}{
		{name: "collision", inventory: wrappershim.Inventory{Records: []wrappershim.Record{}, Collisions: []wrappershim.Record{{CommandName: "gh", State: wrappershim.StateCollisionForeign}}}, code: "wrapper_artifact_collision", kind: fault.KindRejected},
		{name: "tampered", inventory: wrappershim.Inventory{Records: []wrappershim.Record{{CommandName: "gh", State: wrappershim.StateTampered, Reference: manifest.Reference, MaterialSHA256: manifest.MaterialSHA256}}, Collisions: []wrappershim.Record{}}, code: "wrapper_artifact_tampered", kind: fault.KindRejected},
		{name: "invalid", inventory: wrappershim.Inventory{}, code: "wrapper_artifact_store_contract", kind: fault.KindContract},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			store.status = test.inventory
			result, err := service.Status(context.Background(), statusIntent())
			assertFault(t, err, test.code, test.kind, false)
			if result.Artifacts != nil {
				t.Fatalf("partial artifacts=%+v", result.Artifacts)
			}
		})
	}
}

func TestRemoveConsumesExactOpaqueReferenceAndRejectsUnknown(t *testing.T) {
	service, _, _, store, manifest := newInstallService(t)
	store.removeRecord = store.installRecord
	result, err := service.Remove(context.Background(), removeIntent(manifest.Reference), manifest.Reference.String())
	if err != nil {
		t.Fatal(err)
	}
	if store.reference != manifest.Reference || store.removeCalls != 1 || result.CommandName != "gh" || !result.Removed || result.Path != filepath.Join(store.binPath, "gh") || result.SourceProcessAttempts != 0 || result.ProcessorProcessAttempts != 0 {
		t.Fatalf("reference=%q calls=%d result=%+v", store.reference, store.removeCalls, result)
	}

	store.removeCalls = 0
	store.removeErr = wrappershim.ErrNotFound
	_, err = service.Remove(context.Background(), removeIntent(manifest.Reference), manifest.Reference.String())
	assertFault(t, err, "wrapper_artifact_not_found", fault.KindNotFound, false)
	if store.removeCalls != 1 {
		t.Fatalf("remove calls=%d", store.removeCalls)
	}
}

func TestRemoveRejectsMalformedOrMismatchedReferenceBeforeMutation(t *testing.T) {
	service, _, _, store, manifest := newInstallService(t)
	for name, test := range map[string]struct {
		intent operation.Intent
		value  string
		code   string
		kind   fault.Kind
	}{
		"malformed": {intent: removeIntent(manifest.Reference), value: "not-a-reference", code: "invalid_wrapper_artifact", kind: fault.KindInvalidInput},
		"mismatch": {intent: func() operation.Intent {
			current := removeIntent(manifest.Reference)
			current.Target.ID = "different"
			return current
		}(), value: manifest.Reference.String(), code: "invalid_mutation_contract", kind: fault.KindContract},
	} {
		t.Run(name, func(t *testing.T) {
			_, err := service.Remove(context.Background(), test.intent, test.value)
			assertFault(t, err, test.code, test.kind, false)
			if store.removeCalls != 0 {
				t.Fatalf("remove calls=%d", store.removeCalls)
			}
		})
	}
}

func TestUnsupportedPlatformFailsEveryTaskBeforeStoreOrMaterialization(t *testing.T) {
	service, materializer, renderer, store, manifest := newInstallService(t)
	service.platform = "windows"
	for name, run := range map[string]func() error{
		"install": func() error {
			_, err := service.Install(context.Background(), installIntent(), manifest.Binding.BundleLocator)
			return err
		},
		"status": func() error { _, err := service.Status(context.Background(), statusIntent()); return err },
		"remove": func() error {
			_, err := service.Remove(context.Background(), removeIntent(manifest.Reference), manifest.Reference.String())
			return err
		},
	} {
		t.Run(name, func(t *testing.T) {
			assertFault(t, run(), "wrapper_artifact_platform_not_supported", fault.KindUnsupported, false)
		})
	}
	if materializer.calls != 0 || renderer.calls != 0 || store.binCalls != 0 || store.installCalls != 0 || store.statusCalls != 0 || store.removeCalls != 0 {
		t.Fatalf("calls materialize/render/bin/install/status/remove=%d/%d/%d/%d/%d/%d", materializer.calls, renderer.calls, store.binCalls, store.installCalls, store.statusCalls, store.removeCalls)
	}
}

func TestCancellationBeforeTaskStartsMakesZeroCalls(t *testing.T) {
	service, materializer, renderer, store, manifest := newInstallService(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := service.Install(ctx, installIntent(), manifest.Binding.BundleLocator)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("error=%v", err)
	}
	if materializer.calls != 0 || renderer.calls != 0 || store.binCalls != 0 || store.installCalls != 0 {
		t.Fatalf("calls materialize/render/bin/install=%d/%d/%d/%d", materializer.calls, renderer.calls, store.binCalls, store.installCalls)
	}
}

func assertFault(t *testing.T, err error, code string, kind fault.Kind, retryable bool) {
	t.Helper()
	public, ok := fault.PublicCopy(err)
	if !ok || public.Code != code || public.Kind != kind || public.Retryable != retryable {
		t.Fatalf("fault=%+v ok=%t err=%v", public, ok, err)
	}
}
