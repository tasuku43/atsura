// Package bundleexecute is the public bundle-execute façade over the shared,
// host-neutral plan application service.
package bundleexecute

import (
	"context"
	"fmt"

	"github.com/tasuku43/atsura/internal/app/planapply"
	"github.com/tasuku43/atsura/internal/domain/fault"
	"github.com/tasuku43/atsura/internal/domain/operation"
	"github.com/tasuku43/atsura/internal/domain/tailoringplan"
)

const Command = "bundle execute"

type BundlePort = planapply.BundlePort
type AdoptionPort = planapply.AdoptionPort
type IdentityPort = planapply.IdentityPort
type CompatibilityPort = planapply.CompatibilityPort
type ProcessPort = planapply.ProcessPort
type ParserPort = planapply.ParserPort
type Result = planapply.Result

type Service struct {
	applier *planapply.Service
}

func New(bundles BundlePort, adoption AdoptionPort, identity IdentityPort, compatibility CompatibilityPort, processes ProcessPort, parser ParserPort) *Service {
	return NewWithApplier(planapply.New(bundles, adoption, identity, compatibility, processes, parser))
}

// NewWithApplier lets the composition root share one plan application service
// with another façade while this package retains bundle-execute intent and
// recovery vocabulary.
func NewWithApplier(applier *planapply.Service) *Service {
	return &Service{applier: applier}
}

// Execute validates the exact public intent before delegating to the one
// shared plan application path.
func (s *Service) Execute(ctx context.Context, intent operation.Intent, bundlePath string, attempt tailoringplan.Attempt) (Result, error) {
	if ctx == nil {
		return Result{}, fmt.Errorf("bundle execution context is nil")
	}
	if err := ctx.Err(); err != nil {
		return Result{}, err
	}
	if err := intent.Validate(); err != nil || intent.Command != Command || intent.Effect != operation.EffectExecute {
		return Result{}, fmt.Errorf("bundle execution requires the exact execute intent")
	}
	if s == nil || !s.applier.Configured() {
		return Result{}, fmt.Errorf("bundle execution adapters are not configured")
	}
	return s.applier.Apply(ctx, planapply.Request{
		BundlePath:                       bundlePath,
		AllowSourceStreamPassthrough:     false,
		AllowOriginalPreservingOptimizer: false,
		Attempt:                          attempt,
		Command:                          commandContext(),
	})
}

func commandContext() planapply.CommandContext {
	return planapply.CommandContext{
		LoadFailureMessage:      "Bundle execution could not load its bundle.",
		RuntimeHelpAction:       executeHelpAction(),
		PlanPreviewAction:       previewAction(),
		StatusAction:            statusAction(),
		TrustAction:             trustAction(),
		ProcessStartRetryAction: fault.NextAction{Command: Command, Reason: "Retry the exact invocation only after confirming that no source process started."},
		BundleMismatchAction:    statusAction(),
	}
}

func executeHelpAction() fault.NextAction {
	return fault.NextAction{Command: "help bundle execute", Reason: "Review the supported transform runtime, exact invocation, and adapter evidence requirements."}
}

func previewAction() fault.NextAction {
	return fault.NextAction{Command: "bundle preview", Reason: "Inspect the freshly resolved wrapper plan without starting the source."}
}

func statusAction() fault.NextAction {
	return fault.NextAction{Command: "bundle status", Reason: "Reconcile bundle adoption and current source identity without executing it."}
}

func trustAction() fault.NextAction {
	return fault.NextAction{Command: "bundle trust", Reason: "Review and adopt the exact bundle digest before execution."}
}
