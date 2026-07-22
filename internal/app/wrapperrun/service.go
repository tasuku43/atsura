// Package wrapperrun applies one host-neutral generated wrapper invocation.
package wrapperrun

import (
	"context"
	"errors"
	"fmt"

	"github.com/tasuku43/atsura/internal/app/planapply"
	"github.com/tasuku43/atsura/internal/app/portcheck"
	"github.com/tasuku43/atsura/internal/domain/fault"
	"github.com/tasuku43/atsura/internal/domain/operation"
	"github.com/tasuku43/atsura/internal/domain/sourceprocess"
	"github.com/tasuku43/atsura/internal/domain/tailoringplan"
	"github.com/tasuku43/atsura/internal/domain/wrapperbinding"
)

const Command = "wrapper run"

// CurrentExecutablePort returns the locator of the process currently serving
// wrapper run. The locator is fingerprinted before it becomes authority.
type CurrentExecutablePort interface {
	CurrentExecutable(context.Context) (string, error)
}

// IdentityPort fingerprints the current Atsura runtime without starting a
// source process.
type IdentityPort interface {
	Identify(context.Context, string) (sourceprocess.Identity, error)
}

// Applier is the one shared fresh-plan application boundary. Keeping this
// interface narrow lets the wrapper façade prove that invalid runtime state
// cannot reach source execution.
type Applier interface {
	Apply(context.Context, planapply.Request) (planapply.Result, error)
}

type Result = planapply.Result

type Service struct {
	current  CurrentExecutablePort
	identity IdentityPort
	applier  Applier
}

func New(current CurrentExecutablePort, identity IdentityPort, applier Applier) *Service {
	return &Service{current: current, identity: identity, applier: applier}
}

// Execute verifies the render-produced bundle/runtime closure, then forwards
// exact argv to the shared plan application path. The source executable
// spelling is intentionally absent from this API and is derived only after the
// applier strictly loads the bound bundle.
func (s *Service) Execute(ctx context.Context, intent operation.Intent, binding wrapperbinding.RuntimeInvocation, args []string) (Result, error) {
	if ctx == nil {
		return Result{}, fmt.Errorf("wrapper run context is nil")
	}
	if err := ctx.Err(); err != nil {
		return Result{}, err
	}
	if err := intent.Validate(); err != nil || intent.Command != Command || intent.Effect != operation.EffectExecute {
		return Result{}, fmt.Errorf("wrapper run requires the exact execute intent")
	}
	if s == nil || portcheck.IsNil(s.current) || portcheck.IsNil(s.identity) || portcheck.IsNil(s.applier) {
		return Result{}, fmt.Errorf("wrapper run adapters are not configured")
	}
	if err := binding.Validate(); err != nil {
		return Result{}, fault.New(
			fault.KindInvalidInput,
			"invalid_wrapper_binding",
			"The generated wrapper binding is incomplete or invalid.",
			false,
			renderAction(),
		)
	}

	locator, err := s.current.CurrentExecutable(ctx)
	if err != nil {
		return Result{}, classifyRuntimeUnavailable(err)
	}
	if err := ctx.Err(); err != nil {
		return Result{}, err
	}
	current, err := s.identity.Identify(ctx, locator)
	if err != nil {
		return Result{}, classifyRuntimeUnavailable(err)
	}
	if err := ctx.Err(); err != nil {
		return Result{}, err
	}
	if err := current.Validate(); err != nil {
		return Result{}, runtimeUnavailable()
	}
	if current != binding.Runtime.SourceProcessIdentity() {
		return Result{}, fault.New(
			fault.KindRejected,
			"wrapper_runtime_drift",
			"The current Atsura runtime does not match the exact generated wrapper binding.",
			false,
			renderAction(),
		)
	}

	forwarded := append([]string{}, args...)
	return s.applier.Apply(ctx, planapply.Request{
		BundlePath:                       binding.BundleLocator,
		ExpectedBundleDigest:             binding.BundleDigest,
		DeriveExecutableFromLoadedBundle: true,
		AllowSourceStreamPassthrough:     true,
		AllowOriginalPreservingOptimizer: true,
		Attempt:                          tailoringplan.Attempt{Args: forwarded},
		Command:                          commandContext(),
	})
}

func classifyRuntimeUnavailable(err error) error {
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
		renderAction(),
	)
}

func commandContext() planapply.CommandContext {
	return planapply.CommandContext{
		LoadFailureMessage:      "Wrapper execution could not load its exact bound bundle.",
		RuntimeHelpAction:       fault.NextAction{Command: "help wrapper run", Reason: "Review the supported generated-wrapper runtime contract."},
		PlanPreviewAction:       fault.NextAction{Command: "bundle preview", Reason: "Inspect the freshly resolved wrapper plan without starting the source."},
		StatusAction:            fault.NextAction{Command: "bundle status", Reason: "Reconcile bundle adoption and current source identity without executing it."},
		TrustAction:             fault.NextAction{Command: "bundle trust", Reason: "Review and adopt the exact bundle digest before execution."},
		ProcessStartRetryAction: fault.NextAction{Command: Command, Reason: "Retry the exact generated wrapper invocation only after confirming that no source process started."},
		BundleMismatchAction:    renderAction(),
	}
}

func renderAction() fault.NextAction {
	return fault.NextAction{
		Command: "wrapper render",
		Reason:  "Render a new wrapper binding from the exact current bundle and Atsura runtime.",
	}
}
