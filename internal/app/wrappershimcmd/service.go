// Package wrappershimcmd owns the public install, status, and exact-reference
// removal tasks for Atsura-managed host-neutral wrapper shims.
package wrappershimcmd

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"unicode"
	"unicode/utf8"

	"github.com/tasuku43/atsura/internal/app/execution"
	"github.com/tasuku43/atsura/internal/app/portcheck"
	"github.com/tasuku43/atsura/internal/domain/fault"
	"github.com/tasuku43/atsura/internal/domain/operation"
	"github.com/tasuku43/atsura/internal/domain/wrapperbinding"
	"github.com/tasuku43/atsura/internal/domain/wrappershim"
)

const (
	InstallCommand = "wrapper install"
	StatusCommand  = "wrapper status"
	RemoveCommand  = "wrapper remove"

	StoreTargetKind = "wrapper-shim-store"
	StoreTargetID   = "selected"
	ArtifactRefKind = "wrapper-shim-artifact"
)

var (
	InstallImpact = operation.Impact{
		Cardinality: operation.CardinalityOne, Notification: operation.DeclarationNo,
		AccessChange: operation.DeclarationYes, Destructive: operation.DeclarationNo,
	}
	RemoveImpact = operation.Impact{
		Cardinality: operation.CardinalityOne, Notification: operation.DeclarationNo,
		AccessChange: operation.DeclarationYes, Destructive: operation.DeclarationYes,
	}
)

// MaterializerPort reuses the one application-owned adopted-bundle, identity,
// and complete-surface authority shared with wrapper render.
type MaterializerPort interface {
	Materialize(context.Context, string) (wrapperbinding.Binding, error)
}

// RendererPort emits only the fixed executable-shim material for a validated
// product binding. Configuration never supplies executable source.
type RendererPort interface {
	Render(wrapperbinding.Binding) (wrapperbinding.RenderedMaterial, error)
}

// StorePort confines every persistent artifact to one implementation-selected
// user-local root. Its domain-only signatures preserve the four-layer
// dependency direction while allowing infrastructure structural typing.
type StorePort interface {
	BinPath() (string, error)
	Install(context.Context, wrappershim.Manifest, []byte) (wrappershim.Record, bool, error)
	Status(context.Context) (wrappershim.Inventory, error)
	Remove(context.Context, wrappershim.Reference) (wrappershim.Record, error)
}

type InstallResult struct {
	CommandName              string
	Path                     string
	BinPath                  string
	AlreadyInstalled         bool
	SourceProcessAttempts    int
	ProcessorProcessAttempts int
}

type Artifact struct {
	Reference      wrappershim.Reference
	CommandName    string
	State          wrappershim.State
	Path           string
	MaterialSHA256 string
}

type StatusResult struct {
	Artifacts []Artifact
}

type RemoveResult struct {
	CommandName              string
	Path                     string
	Removed                  bool
	SourceProcessAttempts    int
	ProcessorProcessAttempts int
}

type Service struct {
	platform     string
	materializer MaterializerPort
	renderer     RendererPort
	store        StorePort
}

func New(platform string, materializer MaterializerPort, renderer RendererPort, store StorePort) *Service {
	return &Service{platform: platform, materializer: materializer, renderer: renderer, store: store}
}

// Install compiles the existing complete wrapper binding into fixed executable
// bytes, then creates at most one exact active artifact behind the central
// Atsura-owned mutation boundary. It has no source or processor process port.
func (s *Service) Install(ctx context.Context, intent operation.Intent, bundleLocator string) (InstallResult, error) {
	if err := s.preflight(ctx, intent, InstallCommand, operation.EffectCreate); err != nil {
		return InstallResult{}, err
	}
	if err := s.requirePOSIX(); err != nil {
		return InstallResult{}, err
	}
	binPath, err := s.binPath()
	if err != nil {
		return InstallResult{}, err
	}
	binding, err := s.materializer.Materialize(ctx, bundleLocator)
	if err != nil {
		return InstallResult{}, preserveMaterialization(err)
	}
	material, err := s.renderer.Render(binding.Clone())
	if err != nil {
		return InstallResult{}, renderFailure(err)
	}
	if err := material.Validate(); err != nil {
		return InstallResult{}, renderFailure(err)
	}
	manifest, err := wrappershim.NewManifest(binding.Clone(), material.Clone())
	if err != nil {
		return InstallResult{}, renderFailure(err)
	}

	expectedTarget := operation.TargetRef{Kind: StoreTargetKind, ParentID: StoreTargetID}
	request := execution.Request{
		Intent: intent, ExpectedCommand: InstallCommand, ExpectedEffect: operation.EffectCreate,
		ExpectedTarget: expectedTarget, ExpectedImpact: InstallImpact,
	}
	var record wrappershim.Record
	var alreadyInstalled bool
	invoker := execution.New(exactIntentPolicy{expected: intent})
	err = invoker.Invoke(ctx, request, func(ctx context.Context, _ operation.Intent) error {
		current, already, installErr := s.store.Install(ctx, manifest.Clone(), append([]byte(nil), material.Source...))
		if installErr != nil {
			return classifyMutationStoreError(installErr)
		}
		if validationErr := validateInstalledRecord(current, manifest); validationErr != nil {
			return storeContractFailure(validationErr)
		}
		record = current
		alreadyInstalled = already
		return nil
	})
	if err != nil {
		return InstallResult{}, err
	}
	return InstallResult{
		CommandName: record.CommandName, Path: filepath.Join(binPath, record.CommandName), BinPath: binPath,
		AlreadyInstalled: alreadyInstalled, SourceProcessAttempts: 0, ProcessorProcessAttempts: 0,
	}, nil
}

// Status returns only complete valid owned records. Collision and tamper
// observations fail the discovery as structured reconciliation faults so the
// command never emits an empty or fabricated opaque artifact reference.
func (s *Service) Status(ctx context.Context, intent operation.Intent) (StatusResult, error) {
	if err := s.preflight(ctx, intent, StatusCommand, operation.EffectRead); err != nil {
		return StatusResult{}, err
	}
	if err := s.requirePOSIX(); err != nil {
		return StatusResult{}, err
	}
	binPath, err := s.binPath()
	if err != nil {
		return StatusResult{}, err
	}
	inventory, err := s.store.Status(ctx)
	if err != nil {
		return StatusResult{}, classifyStatusStoreError(err)
	}
	if err := inventory.Validate(); err != nil {
		return StatusResult{}, storeContractFailure(err)
	}
	if len(inventory.Collisions) != 0 {
		return StatusResult{}, fault.New(
			fault.KindRejected,
			"wrapper_artifact_collision",
			"The managed wrapper bin directory contains a foreign, symlinked, or special-file collision; no removable artifact references were returned.",
			false,
			statusHelpAction(),
		)
	}
	artifacts := make([]Artifact, 0, len(inventory.Records))
	for _, record := range inventory.Records {
		if record.State == wrappershim.StateTampered {
			return StatusResult{}, fault.New(
				fault.KindRejected,
				"wrapper_artifact_tampered",
				"A managed wrapper artifact no longer matches its immutable manifest; no removable artifact references were returned.",
				false,
				statusHelpAction(),
			)
		}
		if record.State != wrappershim.StateOwnedActive && record.State != wrappershim.StateOwnedInactive {
			return StatusResult{}, storeContractFailure(fmt.Errorf("unexpected inventory state %q", record.State))
		}
		artifacts = append(artifacts, Artifact{
			Reference: record.Reference, CommandName: record.CommandName, State: record.State,
			Path: filepath.Join(binPath, record.CommandName), MaterialSHA256: record.MaterialSHA256,
		})
	}
	return StatusResult{Artifacts: artifacts}, nil
}

// Remove consumes one opaque artifact reference unchanged, validates it once
// in the owning domain, and deletes only the exact store-owned record behind
// the central mutation boundary. Unknown references are not idempotent success.
func (s *Service) Remove(ctx context.Context, intent operation.Intent, artifact string) (RemoveResult, error) {
	if err := s.preflight(ctx, intent, RemoveCommand, operation.EffectWrite); err != nil {
		return RemoveResult{}, err
	}
	if err := s.requirePOSIX(); err != nil {
		return RemoveResult{}, err
	}
	reference, err := wrappershim.ParseReference(artifact)
	if err != nil {
		return RemoveResult{}, fault.New(
			fault.KindInvalidInput,
			"invalid_wrapper_artifact",
			"The artifact must be one exact opaque reference returned by wrapper status.",
			false,
			statusAction(),
		)
	}
	if intent.Target != (operation.TargetRef{Kind: ArtifactRefKind, ID: reference.String()}) {
		return RemoveResult{}, fault.New(
			fault.KindContract,
			"invalid_mutation_contract",
			"The wrapper removal target does not match the exact parsed artifact reference.",
			false,
			statusAction(),
		)
	}
	binPath, err := s.binPath()
	if err != nil {
		return RemoveResult{}, err
	}
	request := execution.Request{
		Intent: intent, ExpectedCommand: RemoveCommand, ExpectedEffect: operation.EffectWrite,
		ExpectedTarget: operation.TargetRef{Kind: ArtifactRefKind, ID: reference.String()}, ExpectedImpact: RemoveImpact,
	}
	var record wrappershim.Record
	invoker := execution.New(exactIntentPolicy{expected: intent})
	err = invoker.Invoke(ctx, request, func(ctx context.Context, _ operation.Intent) error {
		removed, removeErr := s.store.Remove(ctx, reference)
		if removeErr != nil {
			return classifyMutationStoreError(removeErr)
		}
		if validationErr := validateRemovedRecord(removed, reference); validationErr != nil {
			return storeContractFailure(validationErr)
		}
		record = removed
		return nil
	})
	if err != nil {
		return RemoveResult{}, err
	}
	return RemoveResult{
		CommandName: record.CommandName, Path: filepath.Join(binPath, record.CommandName), Removed: true,
		SourceProcessAttempts: 0, ProcessorProcessAttempts: 0,
	}, nil
}

func (s *Service) preflight(ctx context.Context, intent operation.Intent, command string, effect operation.Effect) error {
	if ctx == nil {
		return fmt.Errorf("wrapper shim context is nil")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := intent.Validate(); err != nil || intent.Command != command || intent.Effect != effect {
		return fmt.Errorf("wrapper shim service requires the exact %s intent", command)
	}
	if s == nil || portcheck.IsNil(s.store) {
		return fmt.Errorf("wrapper shim store is not configured")
	}
	if effect == operation.EffectCreate && (portcheck.IsNil(s.materializer) || portcheck.IsNil(s.renderer)) {
		return fmt.Errorf("wrapper shim materialization is not configured")
	}
	return nil
}

func (s *Service) requirePOSIX() error {
	if s.platform == "linux" || s.platform == "darwin" {
		return nil
	}
	return fault.New(
		fault.KindUnsupported,
		"wrapper_artifact_platform_not_supported",
		"Managed wrapper artifacts are supported only on Linux and macOS in this release.",
		false,
		helpAction(),
	)
}

func (s *Service) binPath() (string, error) {
	path, err := s.store.BinPath()
	if err != nil {
		return "", classifyStatusStoreError(err)
	}
	if !validAbsoluteCleanPath(path) {
		return "", storeContractFailure(fmt.Errorf("store returned invalid bin path"))
	}
	return path, nil
}

func validateInstalledRecord(record wrappershim.Record, manifest wrappershim.Manifest) error {
	if err := record.Validate(); err != nil {
		return err
	}
	if record.State != wrappershim.StateOwnedActive || record.Reference != manifest.Reference ||
		record.CommandName != manifest.Binding.CommandName || record.MaterialSHA256 != manifest.MaterialSHA256 {
		return fmt.Errorf("installed record does not match the exact manifest")
	}
	return nil
}

func validateRemovedRecord(record wrappershim.Record, reference wrappershim.Reference) error {
	if err := record.Validate(); err != nil {
		return err
	}
	if record.Reference != reference || (record.State != wrappershim.StateOwnedActive && record.State != wrappershim.StateOwnedInactive) {
		return fmt.Errorf("removed record does not match the exact requested artifact")
	}
	return nil
}

func validAbsoluteCleanPath(value string) bool {
	if value == "" || !filepath.IsAbs(value) || filepath.Clean(value) != value || !utf8.ValidString(value) {
		return false
	}
	for _, character := range value {
		if unicode.Is(unicode.C, character) || character == '\u2028' || character == '\u2029' {
			return false
		}
	}
	return true
}

type exactIntentPolicy struct{ expected operation.Intent }

func (p exactIntentPolicy) Check(_ context.Context, intent operation.Intent) error {
	if intent != p.expected {
		return fmt.Errorf("wrapper shim intent changed before mutation")
	}
	return nil
}

func preserveMaterialization(err error) error {
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return err
	}
	if public, ok := fault.PublicCopy(err); ok {
		action := helpAction()
		switch public.Code {
		case "bundle_not_adopted":
			action = fault.NextAction{Command: "bundle trust", Reason: "Review and adopt the exact bundle before installing its wrapper artifact."}
		case "invalid_bundle_trust_store", "bundle_source_drift", "bundle_processor_drift", "source_executable_not_found", "source_identity_unavailable", "unsafe_source_executable", "source_identity_changed", "invalid_source_identity", "processor_identity_unavailable", "processor_identity_changed", "invalid_processor_executable", "unsafe_processor_executable", "invalid_processor_identity":
			action = fault.NextAction{Command: "bundle status", Reason: "Reconcile the exact bundle, source, processor, and adoption state before installing."}
		case "internal_error":
			action = statusAction()
		}
		return fault.New(public.Kind, public.Code, public.Message, public.Retryable, action)
	}
	return fault.Wrap(fault.KindInternal, "internal_error", "Wrapper artifact installation could not materialize the exact bundle closure.", false, err, statusAction())
}

func renderFailure(err error) error {
	return fault.Wrap(
		fault.KindContract,
		"wrapper_artifact_render_failed",
		"The exact wrapper binding could not be rendered as fixed executable-shim material.",
		false,
		err,
		helpAction(),
	)
}

func storeContractFailure(err error) error {
	return fault.Wrap(
		fault.KindContract,
		"wrapper_artifact_store_contract",
		"The managed wrapper store returned state outside its bounded artifact contract.",
		false,
		err,
		statusAction(),
	)
}

func classifyMutationStoreError(err error) error {
	switch {
	case errors.Is(err, wrappershim.ErrInvalidInput):
		return fault.New(fault.KindInvalidInput, "invalid_wrapper_artifact", "The wrapper artifact request is invalid and no store state was changed.", false, helpAction())
	case errors.Is(err, wrappershim.ErrUnsupported):
		return fault.New(fault.KindUnsupported, "wrapper_artifact_platform_not_supported", "Managed wrapper artifacts are unsupported on this platform.", false, helpAction())
	case errors.Is(err, wrappershim.ErrUnsafeStore):
		return fault.New(fault.KindRejected, "wrapper_artifact_store_unsafe", "The managed wrapper store is not a private regular Atsura-owned hierarchy and was not modified.", false, statusAction())
	case errors.Is(err, wrappershim.ErrCapacity):
		return fault.New(fault.KindRejected, "wrapper_artifact_capacity_exceeded", "The managed wrapper store reached its finite artifact capacity and was not modified.", false, statusAction())
	case errors.Is(err, wrappershim.ErrNotFound):
		return fault.New(fault.KindNotFound, "wrapper_artifact_not_found", "The exact wrapper artifact reference is not present in the managed store.", false, statusAction())
	case errors.Is(err, wrappershim.ErrConflict):
		return fault.New(fault.KindRejected, "wrapper_artifact_collision", "A different or foreign artifact already occupies the managed command path; it was not replaced.", false, statusAction())
	case errors.Is(err, wrappershim.ErrTampered):
		return fault.New(fault.KindRejected, "wrapper_artifact_tampered", "The exact managed artifact no longer matches its immutable manifest and was not modified.", false, statusAction())
	case errors.Is(err, wrappershim.ErrUncertain):
		return fault.New(fault.KindUnavailable, "wrapper_artifact_mutation_uncertain", "The wrapper artifact mutation outcome is uncertain and must not be repeated before reconciliation.", false, statusAction())
	default:
		return err
	}
}

func classifyStatusStoreError(err error) error {
	switch {
	case errors.Is(err, wrappershim.ErrInvalidInput):
		return fault.New(fault.KindContract, "wrapper_artifact_store_contract", "The managed wrapper store configuration is invalid.", false, statusHelpAction())
	case errors.Is(err, wrappershim.ErrUnsupported):
		return fault.New(fault.KindUnsupported, "wrapper_artifact_platform_not_supported", "Managed wrapper artifacts are unsupported on this platform.", false, helpAction())
	case errors.Is(err, wrappershim.ErrUnsafeStore):
		return fault.New(fault.KindRejected, "wrapper_artifact_store_unsafe", "The managed wrapper store is not a private regular Atsura-owned hierarchy.", false, statusHelpAction())
	case errors.Is(err, wrappershim.ErrCapacity):
		return fault.New(fault.KindRejected, "wrapper_artifact_capacity_exceeded", "The managed wrapper store exceeds its finite artifact capacity.", false, statusHelpAction())
	case errors.Is(err, wrappershim.ErrNotFound):
		return fault.New(fault.KindNotFound, "wrapper_artifact_not_found", "The exact wrapper artifact reference is not present in the managed store.", false, statusAction())
	case errors.Is(err, wrappershim.ErrConflict):
		return fault.New(fault.KindRejected, "wrapper_artifact_collision", "The managed wrapper bin directory contains a foreign collision.", false, statusHelpAction())
	case errors.Is(err, wrappershim.ErrTampered):
		return fault.New(fault.KindRejected, "wrapper_artifact_tampered", "A managed wrapper artifact no longer matches its immutable manifest.", false, statusHelpAction())
	case errors.Is(err, wrappershim.ErrUncertain):
		return fault.New(fault.KindUnavailable, "wrapper_artifact_status_unavailable", "The managed wrapper inventory could not be observed completely.", true, statusAction())
	default:
		if public, ok := fault.PublicCopy(err); ok {
			return public
		}
		return fault.Wrap(fault.KindInternal, "internal_error", "The managed wrapper inventory could not be read.", false, err, statusAction())
	}
}

func helpAction() fault.NextAction {
	return fault.NextAction{Command: "help wrapper install", Reason: "Review the exact adopted-bundle and managed POSIX artifact requirements."}
}

func statusAction() fault.NextAction {
	return fault.NextAction{Command: StatusCommand, Reason: "Reconcile the bounded managed artifact inventory without mutation."}
}

func statusHelpAction() fault.NextAction {
	return fault.NextAction{Command: "help wrapper status", Reason: "Review managed artifact ownership, collision, and reconciliation requirements."}
}
