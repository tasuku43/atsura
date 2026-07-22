// Package wrapperrender materializes one adopted purpose bundle as a
// deterministic host-neutral POSIX wrapper without starting the source CLI.
package wrapperrender

import (
	"context"
	"errors"
	"fmt"

	"github.com/tasuku43/atsura/internal/app/portcheck"
	"github.com/tasuku43/atsura/internal/domain/bundletrust"
	"github.com/tasuku43/atsura/internal/domain/fault"
	"github.com/tasuku43/atsura/internal/domain/operation"
	"github.com/tasuku43/atsura/internal/domain/processorprocess"
	"github.com/tasuku43/atsura/internal/domain/sourceprocess"
	"github.com/tasuku43/atsura/internal/domain/tailoringbundle"
	"github.com/tasuku43/atsura/internal/domain/wrapperbinding"
)

const Command = "wrapper render"

type BundlePort interface {
	Load(context.Context, string) (tailoringbundle.Bundle, string, error)
}

type AdoptionPort interface {
	Inspect(context.Context, string) bundletrust.State
}

type IdentityPort interface {
	Identify(context.Context, string) (sourceprocess.Identity, error)
}

// CurrentExecutablePort returns the locator of the Atsura process producing
// the wrapper. IdentityPort resolves and fingerprints it before it becomes a
// binding authority.
type CurrentExecutablePort interface {
	CurrentExecutable(context.Context) (string, error)
}

// CompatibilityPort proves that the complete included surface, rather than
// one sample invocation, belongs to one maintained runtime contract.
type CompatibilityPort interface {
	VerifySurface(tailoringbundle.Bundle) error
}

type ProcessorIdentityPort interface {
	Identify(context.Context, string) (processorprocess.Identity, error)
}

// ProcessorCompatibilityPort proves the exact source/processor tuple only
// when a bundle declares an external output processor.
type ProcessorCompatibilityPort interface {
	VerifySurface(tailoringbundle.Bundle) error
}

type ProcessorPorts struct {
	Identity      ProcessorIdentityPort
	Compatibility ProcessorCompatibilityPort
}

// RendererPort accepts only the validated product binding and returns bounded
// deterministic material. Shell syntax remains infrastructure-owned.
type RendererPort interface {
	Render(wrapperbinding.Binding) (wrapperbinding.RenderedMaterial, error)
}

type Result struct {
	Binding                  wrapperbinding.Binding
	Material                 wrapperbinding.RenderedMaterial
	SourceProcessAttempts    int
	ProcessorProcessAttempts int
}

type Service struct {
	platform      string
	bundles       BundlePort
	adoption      AdoptionPort
	identity      IdentityPort
	current       CurrentExecutablePort
	compatibility CompatibilityPort
	renderer      RendererPort
	processors    ProcessorPorts
	invalid       bool
}

// New creates the read-only wrapper renderer. platform is the composition
// root's GOOS observation; only the first slice's POSIX targets are admitted.
func New(platform string, bundles BundlePort, adoption AdoptionPort, identity IdentityPort, current CurrentExecutablePort, compatibility CompatibilityPort, renderer RendererPort, processors ...ProcessorPorts) *Service {
	service := &Service{
		platform: platform, bundles: bundles, adoption: adoption, identity: identity,
		current: current, compatibility: compatibility, renderer: renderer,
	}
	if len(processors) > 1 {
		service.invalid = true
	} else if len(processors) == 1 {
		service.processors = processors[0]
	}
	return service
}

// Render strictly loads one exact adopted bundle once, revalidates source and
// runtime identity, proves the whole surface, and emits fixed wrapper material.
// No source process port is available to this service.
func (s *Service) Render(ctx context.Context, intent operation.Intent, bundleLocator string) (Result, error) {
	if ctx == nil {
		return Result{}, fmt.Errorf("wrapper render context is nil")
	}
	if err := ctx.Err(); err != nil {
		return Result{}, err
	}
	if err := intent.Validate(); err != nil || intent.Command != Command || intent.Effect != operation.EffectRead {
		return Result{}, fmt.Errorf("wrapper render requires the exact read intent")
	}
	if !s.configured() {
		return Result{}, fmt.Errorf("wrapper render adapters are not configured")
	}
	if err := wrapperbinding.ValidateBundleLocator(bundleLocator); err != nil {
		return Result{}, fault.New(
			fault.KindInvalidInput,
			"invalid_wrapper_binding",
			"The wrapper bundle locator must be one exact absolute clean path.",
			false,
			helpAction(),
		)
	}
	if s.platform != "linux" && s.platform != "darwin" {
		return Result{}, fault.New(
			fault.KindUnsupported,
			"wrapper_platform_not_supported",
			"POSIX wrapper rendering is supported only on Linux and macOS in this release.",
			false,
			helpAction(),
		)
	}

	bundle, bundleDigest, err := s.bundles.Load(ctx, bundleLocator)
	if err != nil {
		return Result{}, preserveLoad(err)
	}
	switch state := s.adoption.Inspect(ctx, bundleDigest); state {
	case bundletrust.StateAdopted:
	case bundletrust.StateInvalid:
		return Result{}, fault.New(fault.KindRejected, "invalid_bundle_trust_store", "The user-local bundle adoption store is invalid.", false, statusAction())
	case bundletrust.StateNotAdopted:
		return Result{}, fault.New(fault.KindRejected, "bundle_not_adopted", "The exact bundle digest has not been adopted by this user.", false, trustAction())
	default:
		return Result{}, fault.New(fault.KindRejected, "invalid_bundle_trust_store", "The user-local bundle adoption store returned an unknown state.", false, statusAction())
	}
	if err := ctx.Err(); err != nil {
		return Result{}, err
	}

	currentSource, err := s.identity.Identify(ctx, bundle.Catalog.Source.ResolvedPath)
	if err != nil {
		return Result{}, classifySourceIdentity(err)
	}
	wantedSource := sourceprocess.Identity{
		ResolvedPath: bundle.Catalog.Source.ResolvedPath,
		SHA256:       bundle.Catalog.Source.SHA256,
		Size:         bundle.Catalog.Source.Size,
	}
	if currentSource != wantedSource {
		return Result{}, fault.New(fault.KindRejected, "bundle_source_drift", "The bundle source identity is no longer current.", false, statusAction())
	}
	if err := ctx.Err(); err != nil {
		return Result{}, err
	}
	if err := s.compatibility.VerifySurface(bundle); err != nil {
		return Result{}, fault.New(
			fault.KindUnsupported,
			"wrapper_runtime_not_supported",
			"The complete tailored surface is not admitted by one maintained wrapper runtime contract.",
			false,
			helpAction(),
		)
	}
	if len(bundle.Processors) > 0 {
		if portcheck.IsNil(s.processors.Identity) || portcheck.IsNil(s.processors.Compatibility) {
			return Result{}, fault.New(fault.KindUnsupported, "wrapper_runtime_not_supported", "This runtime has no complete external processor compatibility boundary.", false, helpAction())
		}
		for _, binding := range bundle.Processors {
			identity, err := s.processors.Identity.Identify(ctx, binding.Observation.Identity.ResolvedPath)
			if err != nil {
				return Result{}, classifyProcessorIdentity(err)
			}
			if identity != binding.Observation.Identity {
				return Result{}, fault.New(fault.KindRejected, "bundle_processor_drift", "A bundle processor identity is no longer current.", false, statusAction())
			}
		}
		if err := s.processors.Compatibility.VerifySurface(bundle); err != nil {
			return Result{}, fault.New(fault.KindUnsupported, "wrapper_runtime_not_supported", "The external processor tuple is not admitted by this wrapper runtime.", false, helpAction())
		}
	}

	runtimeLocator, err := s.current.CurrentExecutable(ctx)
	if err != nil {
		return Result{}, classifyRuntimeIdentity(err)
	}
	if err := ctx.Err(); err != nil {
		return Result{}, err
	}
	runtimeIdentity, err := s.identity.Identify(ctx, runtimeLocator)
	if err != nil {
		return Result{}, classifyRuntimeIdentity(err)
	}
	if err := runtimeIdentity.Validate(); err != nil {
		return Result{}, runtimeUnavailable()
	}
	if err := ctx.Err(); err != nil {
		return Result{}, err
	}

	binding, err := wrapperbinding.New(bundleLocator, bundleDigest, bundle, runtimeIdentity)
	if err != nil {
		return Result{}, fault.New(
			fault.KindInvalidInput,
			"invalid_wrapper_binding",
			"The adopted bundle cannot produce a complete portable wrapper binding.",
			false,
			helpAction(),
		)
	}
	// The renderer is a controlled port but still receives a detached copy so
	// slice-bearing compiled help cannot mutate the binding returned to the
	// caller or weaken the bundle-derived closure.
	material, err := s.renderer.Render(binding.Clone())
	if err != nil {
		return Result{}, fault.New(
			fault.KindContract,
			"wrapper_render_failed",
			"The validated wrapper binding could not be rendered by the fixed POSIX contract.",
			false,
			helpAction(),
		)
	}
	if err := material.Validate(); err != nil {
		return Result{}, fault.New(
			fault.KindContract,
			"wrapper_render_failed",
			"The wrapper renderer returned material outside its bounded deterministic contract.",
			false,
			helpAction(),
		)
	}
	if err := ctx.Err(); err != nil {
		return Result{}, err
	}
	return Result{Binding: binding.Clone(), Material: material.Clone(), SourceProcessAttempts: 0, ProcessorProcessAttempts: 0}, nil
}

func (s *Service) configured() bool {
	return s != nil && !s.invalid &&
		!portcheck.IsNil(s.bundles) &&
		!portcheck.IsNil(s.adoption) &&
		!portcheck.IsNil(s.identity) &&
		!portcheck.IsNil(s.current) &&
		!portcheck.IsNil(s.compatibility) &&
		!portcheck.IsNil(s.renderer)
}

func classifyProcessorIdentity(err error) error {
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return err
	}
	if public, ok := fault.PublicCopy(err); ok {
		return fault.New(public.Kind, public.Code, public.Message, public.Retryable, statusAction())
	}
	return fault.Wrap(fault.KindUnavailable, "processor_identity_unavailable", "A bundle processor identity could not be assessed.", false, err, statusAction())
}

func preserveLoad(err error) error {
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return err
	}
	if public, ok := fault.PublicCopy(err); ok {
		return public
	}
	return fault.Wrap(fault.KindInternal, "internal_error", "Wrapper rendering could not load its bundle.", false, err, statusAction())
}

func classifySourceIdentity(err error) error {
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return err
	}
	if public, ok := fault.PublicCopy(err); ok {
		return fault.New(public.Kind, public.Code, public.Message, public.Retryable, statusAction())
	}
	return fault.Wrap(fault.KindInternal, "internal_error", "The current source identity could not be assessed.", false, err, statusAction())
}

func classifyRuntimeIdentity(err error) error {
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return err
	}
	return runtimeUnavailable()
}

func runtimeUnavailable() error {
	return fault.New(
		fault.KindUnavailable,
		"wrapper_runtime_unavailable",
		"The current Atsura runtime identity could not be verified.",
		false,
		helpAction(),
	)
}

func helpAction() fault.NextAction {
	return fault.NextAction{Command: "help wrapper render", Reason: "Review the exact adopted-bundle, runtime, surface, and POSIX wrapper requirements."}
}

func statusAction() fault.NextAction {
	return fault.NextAction{Command: "bundle status", Reason: "Reconcile bundle adoption and current source identity without executing it."}
}

func trustAction() fault.NextAction {
	return fault.NextAction{Command: "bundle trust", Reason: "Review and adopt the exact bundle digest before rendering a wrapper."}
}
