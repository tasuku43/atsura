// Package planpreview admits one bundle-backed attempted invocation and
// constructs its deterministic wrapper plan without starting the source.
package planpreview

import (
	"context"
	"errors"
	"fmt"

	"github.com/tasuku43/atsura/internal/app/portcheck"
	"github.com/tasuku43/atsura/internal/domain/bundletrust"
	"github.com/tasuku43/atsura/internal/domain/fault"
	"github.com/tasuku43/atsura/internal/domain/operation"
	"github.com/tasuku43/atsura/internal/domain/sourceprocess"
	"github.com/tasuku43/atsura/internal/domain/tailoringbundle"
	"github.com/tasuku43/atsura/internal/domain/tailoringplan"
)

const Command = "bundle preview"

type BundlePort interface {
	Load(context.Context, string) (tailoringbundle.Bundle, string, error)
}

type AdoptionPort interface {
	Inspect(context.Context, string) bundletrust.State
}

type IdentityPort interface {
	Identify(context.Context, string) (sourceprocess.Identity, error)
}

type Result struct {
	Plan                  tailoringplan.Plan
	PlanDigest            string
	SourceProcessAttempts int
}

type Service struct {
	bundles  BundlePort
	adoption AdoptionPort
	identity IdentityPort
}

func New(bundles BundlePort, adoption AdoptionPort, identity IdentityPort) *Service {
	return &Service{bundles: bundles, adoption: adoption, identity: identity}
}

func (s *Service) Preview(ctx context.Context, intent operation.Intent, bundlePath string, attempt tailoringplan.Attempt) (Result, error) {
	if ctx == nil {
		return Result{}, fmt.Errorf("plan preview context is nil")
	}
	if err := ctx.Err(); err != nil {
		return Result{}, err
	}
	if err := intent.Validate(); err != nil || intent.Command != Command || intent.Effect != operation.EffectRead {
		return Result{}, fmt.Errorf("plan preview requires the exact bundle preview read intent")
	}
	if s == nil || portcheck.IsNil(s.bundles) || portcheck.IsNil(s.adoption) || portcheck.IsNil(s.identity) {
		return Result{}, fmt.Errorf("plan preview adapters are not configured")
	}

	bundle, bundleDigest, err := s.bundles.Load(ctx, bundlePath)
	if err != nil {
		return Result{}, preserve(err)
	}
	adoption := s.adoption.Inspect(ctx, bundleDigest)
	if err := ctx.Err(); err != nil {
		return Result{}, err
	}
	switch adoption {
	case bundletrust.StateAdopted:
	case bundletrust.StateInvalid:
		return Result{}, fault.New(fault.KindRejected, "invalid_bundle_trust_store", "The user-local bundle adoption store is invalid.", false, statusAction())
	case bundletrust.StateNotAdopted:
		return Result{}, fault.New(fault.KindRejected, "bundle_not_adopted", "The exact bundle digest has not been adopted by this user.", false, trustAction())
	default:
		return Result{}, fault.New(fault.KindRejected, "invalid_bundle_trust_store", "The user-local bundle adoption store returned an unknown state.", false, statusAction())
	}
	current, err := s.identity.Identify(ctx, bundle.Catalog.Source.ResolvedPath)
	if err != nil {
		return Result{}, classifyIdentity(err)
	}
	wanted := sourceprocess.Identity{ResolvedPath: bundle.Catalog.Source.ResolvedPath, SHA256: bundle.Catalog.Source.SHA256, Size: bundle.Catalog.Source.Size}
	if current != wanted {
		return Result{}, fault.New(fault.KindRejected, "bundle_source_drift", "The bundle source identity is no longer current.", false, statusAction())
	}
	if err := ctx.Err(); err != nil {
		return Result{}, err
	}
	plan, err := tailoringplan.Build(bundleDigest, bundle, current, attempt)
	if err != nil {
		return Result{}, classifyPlan(err)
	}
	digest, err := plan.Digest()
	if err != nil {
		return Result{}, fault.Wrap(fault.KindContract, "invalid_wrapper_plan", "The wrapper plan could not be encoded canonically.", false, err, previewHelpAction())
	}
	return Result{Plan: plan, PlanDigest: digest, SourceProcessAttempts: 0}, nil
}

func classifyPlan(err error) error {
	switch {
	case errors.Is(err, tailoringplan.ErrSourceExecutableMismatch):
		return fault.Wrap(fault.KindInvalidInput, "source_executable_mismatch", "The attempted executable is not the bundle's exact requested executable or resolved path.", false, err, previewHelpAction())
	case errors.Is(err, tailoringplan.ErrCommandNotInSurface):
		return fault.Wrap(fault.KindNotFound, "command_not_in_surface", "The matched source command is absent from the tailored surface, so no plan exists.", false, err, previewHelpAction())
	case errors.Is(err, tailoringplan.ErrOptionNotInSurface):
		return fault.Wrap(fault.KindNotFound, "option_not_in_surface", "An attempted source option is absent from the tailored option surface, so no plan exists.", false, err, previewHelpAction())
	case errors.Is(err, tailoringplan.ErrInvalidInvocation):
		return fault.Wrap(fault.KindInvalidInput, "invalid_invocation", "The attempted source invocation cannot be resolved deterministically from the bundle catalog.", false, err, previewHelpAction())
	case errors.Is(err, tailoringplan.ErrInvalidPlan):
		return fault.Wrap(fault.KindContract, "invalid_wrapper_plan", "The bundle and invocation did not produce a complete wrapper plan.", false, err, previewHelpAction())
	default:
		return fault.Wrap(fault.KindInternal, "internal_error", "The wrapper plan could not be constructed.", false, err, statusAction())
	}
}

func classifyIdentity(err error) error {
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return err
	}
	if public, ok := fault.PublicCopy(err); ok {
		return fault.New(public.Kind, public.Code, public.Message, public.Retryable, statusAction())
	}
	return fault.Wrap(fault.KindInternal, "internal_error", "The current source identity could not be assessed.", false, err, statusAction())
}

func preserve(err error) error {
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return err
	}
	if public, ok := fault.PublicCopy(err); ok {
		return public
	}
	return fault.Wrap(fault.KindInternal, "internal_error", "The plan preview could not load its bundle.", false, err, statusAction())
}

func previewHelpAction() fault.NextAction {
	return fault.NextAction{Command: "help bundle preview", Reason: "Review exact bundle-backed invocation and tailored-surface requirements."}
}

func statusAction() fault.NextAction {
	return fault.NextAction{Command: "bundle status", Reason: "Reconcile bundle adoption and current source identity without executing it."}
}

func trustAction() fault.NextAction {
	return fault.NextAction{Command: "bundle trust", Reason: "Review and adopt the exact bundle digest before constructing a plan."}
}
