// Package planapply rebuilds and applies one adopted bundle's wrapper plan
// through the shared, host-neutral source-process boundary.
package planapply

import (
	"context"
	"errors"
	"fmt"

	"github.com/tasuku43/atsura/internal/app/portcheck"
	"github.com/tasuku43/atsura/internal/domain/bundletrust"
	"github.com/tasuku43/atsura/internal/domain/fault"
	"github.com/tasuku43/atsura/internal/domain/runtimeadmission"
	"github.com/tasuku43/atsura/internal/domain/sourceprocess"
	"github.com/tasuku43/atsura/internal/domain/tailoring"
	"github.com/tasuku43/atsura/internal/domain/tailoringbundle"
	"github.com/tasuku43/atsura/internal/domain/tailoringplan"
)

type BundlePort interface {
	Load(context.Context, string) (tailoringbundle.Bundle, string, error)
}

type AdoptionPort interface {
	Inspect(context.Context, string) bundletrust.State
}

type IdentityPort interface {
	Identify(context.Context, string) (sourceprocess.Identity, error)
}

// CompatibilityPort proves source-adapter runtime behavior without starting
// the source. Implementations branch only on exact adapter contracts.
type CompatibilityPort interface {
	VerifyRuntime(tailoringplan.Plan) error
}

type runtimeAdmissionCategorized interface {
	RuntimeAdmissionCategory() runtimeadmission.Category
}

const (
	runtimeMessageGeneric  = "The source adapter cannot prove this wrapper's runtime output contract."
	runtimeMessageAdapter  = "The wrapper plan's source adapter contract is not admitted by this runtime."
	runtimeMessageVersion  = "The wrapper plan's source version is not admitted by this runtime."
	runtimeMessageCommand  = "The wrapper plan's source command is not admitted by this runtime."
	runtimeMessageOutput   = "The wrapper plan does not declare the admitted transforming JSON output contract."
	runtimeMessageArgv     = "The wrapper plan's source arguments are outside the admitted command grammar."
	runtimeMessageSelector = "The wrapper plan does not contain exactly one admitted JSON selector matching its output fields."
)

type ProcessPort interface {
	RunBound(context.Context, sourceprocess.BoundRequest) (sourceprocess.Result, error)
}

type ParserPort interface {
	Parse(context.Context, []byte) (tailoring.JSONValue, error)
}

// CommandContext supplies the public recovery vocabulary of the façade that
// asked for plan application. The shared runtime owns no public command path
// and does not maintain a second command or recovery registry.
type CommandContext struct {
	LoadFailureMessage      string
	RuntimeHelpAction       fault.NextAction
	PlanPreviewAction       fault.NextAction
	StatusAction            fault.NextAction
	TrustAction             fault.NextAction
	ProcessStartRetryAction fault.NextAction
	BundleMismatchAction    fault.NextAction
}

func (c CommandContext) Validate() error {
	probe := fault.New(
		fault.KindInternal,
		"invalid_plan_application_context",
		c.LoadFailureMessage,
		false,
		c.RuntimeHelpAction,
		c.PlanPreviewAction,
		c.StatusAction,
		c.TrustAction,
		c.ProcessStartRetryAction,
		c.BundleMismatchAction,
	)
	if err := probe.Validate(); err != nil {
		return fmt.Errorf("invalid plan application command context: %w", err)
	}
	return nil
}

// Request identifies one fresh application of a bundle-backed plan. An empty
// ExpectedBundleDigest preserves the direct gateway's existing behavior. A
// non-empty value closes a generated binding over one exact loaded bundle.
// DeriveExecutableFromLoadedBundle is exclusive with Attempt.Executable and
// lets a generated wrapper forward argv without accepting a second copy of the
// ordinary command spelling.
type Request struct {
	BundlePath                       string
	ExpectedBundleDigest             string
	DeriveExecutableFromLoadedBundle bool
	Attempt                          tailoringplan.Attempt
	Command                          CommandContext
}

type Result struct {
	BundleDigest          string
	PlanDigest            string
	MatchedCommand        []string
	WrapperKind           tailoringbundle.WrapperKind
	Render                tailoring.RenderFormat
	Output                tailoring.OutputResult
	SourceExitCode        int
	SourceProcessAttempts int
}

// Service owns the one application-level plan application path shared by
// public façades. It is safe to construct once in the composition root and
// inject into each façade that supplies its own CommandContext.
type Service struct {
	bundles       BundlePort
	adoption      AdoptionPort
	identity      IdentityPort
	compatibility CompatibilityPort
	processes     ProcessPort
	parser        ParserPort
}

func New(bundles BundlePort, adoption AdoptionPort, identity IdentityPort, compatibility CompatibilityPort, processes ProcessPort, parser ParserPort) *Service {
	return &Service{bundles: bundles, adoption: adoption, identity: identity, compatibility: compatibility, processes: processes, parser: parser}
}

// Configured reports whether every controlled port needed by Apply is bound.
// A façade can use this to retain its own wiring diagnostic before delegation.
func (s *Service) Configured() bool {
	return s != nil &&
		!portcheck.IsNil(s.bundles) &&
		!portcheck.IsNil(s.adoption) &&
		!portcheck.IsNil(s.identity) &&
		!portcheck.IsNil(s.compatibility) &&
		!portcheck.IsNil(s.processes) &&
		!portcheck.IsNil(s.parser)
}

// Apply strictly loads and optionally closes over one exact bundle digest,
// rebuilds its plan, and applies the admitted transform once. It never consumes
// preview output, retries a started source, or returns raw source bytes.
func (s *Service) Apply(ctx context.Context, request Request) (Result, error) {
	if ctx == nil {
		return Result{}, fmt.Errorf("plan application context is nil")
	}
	if err := ctx.Err(); err != nil {
		return Result{}, err
	}
	if !s.Configured() {
		return Result{}, fmt.Errorf("plan application adapters are not configured")
	}
	if err := request.Command.Validate(); err != nil {
		return Result{}, err
	}
	if request.DeriveExecutableFromLoadedBundle && request.Attempt.Executable != "" {
		return Result{}, fault.New(
			fault.KindInvalidInput,
			"invalid_invocation",
			"A bundle-derived wrapper invocation must not supply another source executable spelling.",
			false,
			request.Command.RuntimeHelpAction,
		)
	}

	bundle, bundleDigest, err := s.bundles.Load(ctx, request.BundlePath)
	if err != nil {
		return Result{}, preserveLoad(err, request.Command)
	}
	if request.ExpectedBundleDigest != "" && bundleDigest != request.ExpectedBundleDigest {
		return Result{}, fault.New(
			fault.KindRejected,
			"bundle_binding_mismatch",
			"The loaded bundle digest does not match the exact expected bundle binding.",
			false,
			request.Command.BundleMismatchAction,
		)
	}
	switch state := s.adoption.Inspect(ctx, bundleDigest); state {
	case bundletrust.StateAdopted:
	case bundletrust.StateInvalid:
		return Result{}, fault.New(fault.KindRejected, "invalid_bundle_trust_store", "The user-local bundle adoption store is invalid.", false, request.Command.StatusAction)
	case bundletrust.StateNotAdopted:
		return Result{}, fault.New(fault.KindRejected, "bundle_not_adopted", "The exact bundle digest has not been adopted by this user.", false, request.Command.TrustAction)
	default:
		return Result{}, fault.New(fault.KindRejected, "invalid_bundle_trust_store", "The user-local bundle adoption store returned an unknown state.", false, request.Command.StatusAction)
	}
	if err := ctx.Err(); err != nil {
		return Result{}, err
	}
	current, err := s.identity.Identify(ctx, bundle.Catalog.Source.ResolvedPath)
	if err != nil {
		return Result{}, classifyIdentity(err, request.Command)
	}
	wanted := sourceprocess.Identity{ResolvedPath: bundle.Catalog.Source.ResolvedPath, SHA256: bundle.Catalog.Source.SHA256, Size: bundle.Catalog.Source.Size}
	if current != wanted {
		return Result{}, fault.New(fault.KindRejected, "bundle_source_drift", "The bundle source identity is no longer current.", false, request.Command.StatusAction)
	}
	attempt := request.Attempt
	if request.DeriveExecutableFromLoadedBundle {
		attempt.Executable = bundle.Catalog.Source.RequestedExecutable
	}
	plan, err := tailoringplan.Build(bundleDigest, bundle, current, attempt)
	if err != nil {
		return Result{}, classifyPlan(err, request.Command)
	}
	planDigest, err := plan.Digest()
	if err != nil {
		return Result{}, fault.Wrap(fault.KindContract, "invalid_wrapper_plan", "The wrapper plan could not be encoded canonically.", false, err, request.Command.PlanPreviewAction)
	}
	outputPlan, present, err := plan.OutputPlan()
	if err != nil {
		return Result{}, fault.Wrap(fault.KindContract, "invalid_wrapper_plan", "The wrapper output stage is invalid.", false, err, request.Command.PlanPreviewAction)
	}
	if !present || plan.WrapperKind != tailoringbundle.WrapperTransform {
		return Result{}, fault.New(fault.KindUnsupported, "wrapper_runtime_not_supported", "This runtime slice requires a transforming wrapper with a typed output stage.", false, request.Command.RuntimeHelpAction)
	}
	if err := s.compatibility.VerifyRuntime(plan); err != nil {
		return Result{}, fault.New(fault.KindUnsupported, "wrapper_runtime_not_supported", runtimeAdmissionMessage(err), false, request.Command.RuntimeHelpAction)
	}
	processRequest, err := plan.SourceRequest()
	if err != nil {
		return Result{}, fault.Wrap(fault.KindContract, "invalid_source_process_request", "The wrapper plan did not produce a valid bound source request.", false, err, request.Command.PlanPreviewAction)
	}
	if err := ctx.Err(); err != nil {
		return Result{}, err
	}

	processResult, processErr := s.processes.RunBound(ctx, processRequest)
	if processErr != nil {
		return Result{}, classifyProcess(processRequest, processResult, processErr, request.Command)
	}
	if err := processResult.ValidateBound(processRequest, true); err != nil || processResult.Attempts != 1 {
		return Result{}, unclassifiedProcess(err, request.Command)
	}
	if len(processResult.Stderr) != 0 {
		return Result{}, fault.New(fault.KindContract, "source_stderr_not_supported", "The source succeeded with stderr, which this transform runtime does not yet represent.", false, request.Command.RuntimeHelpAction)
	}
	if err := ctx.Err(); err != nil {
		return Result{}, fault.Wrap(fault.KindCanceled, "source_output_processing_canceled", "Output processing was canceled after the source process started; replay is not known to be safe.", false, err, request.Command.RuntimeHelpAction)
	}
	parsed, err := s.parser.Parse(ctx, processResult.Stdout)
	if err != nil {
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return Result{}, fault.Wrap(fault.KindCanceled, "source_output_processing_canceled", "Output processing was canceled after the source process started; replay is not known to be safe.", false, err, request.Command.RuntimeHelpAction)
		}
		return Result{}, fault.Wrap(fault.KindContract, "source_json_invalid", "The successful source stdout did not satisfy the declared bounded JSON contract.", false, err, request.Command.RuntimeHelpAction)
	}
	output, err := tailoring.TransformJSON(outputPlan, parsed)
	if err != nil {
		return Result{}, fault.Wrap(fault.KindContract, "output_transform_failed", "The successful source JSON did not satisfy the declared output transformation.", false, err, request.Command.RuntimeHelpAction)
	}
	if err := ctx.Err(); err != nil {
		return Result{}, fault.Wrap(fault.KindCanceled, "source_output_processing_canceled", "Output processing was canceled after the source process started; replay is not known to be safe.", false, err, request.Command.RuntimeHelpAction)
	}
	return Result{
		BundleDigest: bundleDigest, PlanDigest: planDigest, MatchedCommand: append([]string{}, plan.MatchedCommand...),
		WrapperKind: plan.WrapperKind, Render: outputPlan.Render, Output: output,
		SourceExitCode: processResult.ExitCode, SourceProcessAttempts: processResult.Attempts,
	}, nil
}

func runtimeAdmissionMessage(err error) string {
	var categorized runtimeAdmissionCategorized
	if !errors.As(err, &categorized) {
		return runtimeMessageGeneric
	}
	switch categorized.RuntimeAdmissionCategory() {
	case runtimeadmission.CategoryAdapterContract:
		return runtimeMessageAdapter
	case runtimeadmission.CategorySourceVersion:
		return runtimeMessageVersion
	case runtimeadmission.CategoryCommand:
		return runtimeMessageCommand
	case runtimeadmission.CategoryWrapperOutput:
		return runtimeMessageOutput
	case runtimeadmission.CategoryArgvGrammar:
		return runtimeMessageArgv
	case runtimeadmission.CategorySelectorConflict:
		return runtimeMessageSelector
	default:
		return runtimeMessageGeneric
	}
}

func classifyProcess(request sourceprocess.BoundRequest, result sourceprocess.Result, err error, command CommandContext) error {
	if validateErr := result.ValidateBound(request, false); validateErr != nil {
		return unclassifiedProcess(validateErr, command)
	}
	public, ok := fault.PublicCopy(err)
	if !ok {
		return unclassifiedProcess(err, command)
	}
	action := command.RuntimeHelpAction
	if result.Attempts == 0 {
		switch public.Code {
		case "source_identity_changed", "source_identity_unavailable", "unsafe_source_executable", "source_executable_not_found", "invalid_source_identity":
			action = command.StatusAction
		case "source_process_start_failed":
			action = command.ProcessStartRetryAction
		}
		kind, message, retryable, safe := safeProcessFault(0, public.Code)
		if !safe {
			return unclassifiedProcess(err, command)
		}
		return fault.New(kind, public.Code, message, retryable, action)
	}
	kind, message, _, safe := safeProcessFault(1, public.Code)
	if !safe {
		return unclassifiedProcess(err, command)
	}
	return fault.New(kind, public.Code, message, false, action)
}

func safeProcessFault(attempts int, code string) (fault.Kind, string, bool, bool) {
	if attempts == 0 {
		switch code {
		case "source_identity_changed":
			return fault.KindRejected, "The source executable identity changed before it could be started.", false, true
		case "source_identity_unavailable":
			return fault.KindUnavailable, "The source executable identity could not be read before start.", true, true
		case "unsafe_source_executable":
			return fault.KindInvalidInput, "The source executable is not a supported regular executable.", false, true
		case "source_executable_not_found":
			return fault.KindNotFound, "The source executable was not found before start.", false, true
		case "invalid_source_identity":
			return fault.KindContract, "The source executable identity is invalid.", false, true
		case "source_process_start_failed":
			return fault.KindUnavailable, "The source process could not be started.", true, true
		}
		return "", "", false, false
	}
	switch code {
	case "source_stdout_too_large":
		return fault.KindContract, "The source process stdout exceeded the declared limit.", false, true
	case "source_stderr_too_large":
		return fault.KindContract, "The source process stderr exceeded the declared limit.", false, true
	case "source_execution_canceled":
		return fault.KindCanceled, "The caller canceled after the source process started; replay is not known to be safe.", false, true
	case "source_command_timeout":
		return fault.KindUnavailable, "The source process exceeded its declared timeout.", false, true
	case "source_identity_changed":
		return fault.KindRejected, "The source executable identity changed during execution.", false, true
	case "source_command_failed":
		return fault.KindRejected, "The source process exited without a successful result.", false, true
	case "source_process_wait_failed":
		return fault.KindUnavailable, "The source process result could not be collected.", false, true
	default:
		return "", "", false, false
	}
}

func unclassifiedProcess(cause error, command CommandContext) error {
	return fault.Wrap(fault.KindContract, "unclassified_source_execution_outcome", "The source execution result could not be classified as safe to retry.", false, cause, command.StatusAction)
}

func classifyPlan(err error, command CommandContext) error {
	switch {
	case errors.Is(err, tailoringplan.ErrSourceExecutableMismatch):
		return fault.Wrap(fault.KindInvalidInput, "source_executable_mismatch", "The attempted executable is not the bundle's exact requested executable or resolved path.", false, err, command.RuntimeHelpAction)
	case errors.Is(err, tailoringplan.ErrCommandNotInSurface):
		return fault.Wrap(fault.KindNotFound, "command_not_in_surface", "The matched source command is absent from the tailored surface, so no execution plan exists.", false, err, command.RuntimeHelpAction)
	case errors.Is(err, tailoringplan.ErrOptionNotInSurface):
		return fault.Wrap(fault.KindNotFound, "option_not_in_surface", "An attempted source option is absent from the tailored option surface, so no execution plan exists.", false, err, command.RuntimeHelpAction)
	case errors.Is(err, tailoringplan.ErrInvalidInvocation):
		return fault.Wrap(fault.KindInvalidInput, "invalid_invocation", "The attempted source invocation cannot be resolved deterministically from the bundle catalog.", false, err, command.RuntimeHelpAction)
	case errors.Is(err, tailoringplan.ErrInvalidPlan):
		return fault.Wrap(fault.KindContract, "invalid_wrapper_plan", "The bundle and invocation did not produce a complete wrapper plan.", false, err, command.PlanPreviewAction)
	default:
		return fault.Wrap(fault.KindInternal, "internal_error", "The wrapper plan could not be constructed.", false, err, command.StatusAction)
	}
}

func classifyIdentity(err error, command CommandContext) error {
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return err
	}
	if public, ok := fault.PublicCopy(err); ok {
		return fault.New(public.Kind, public.Code, public.Message, public.Retryable, command.StatusAction)
	}
	return fault.Wrap(fault.KindInternal, "internal_error", "The current source identity could not be assessed.", false, err, command.StatusAction)
}

func preserveLoad(err error, command CommandContext) error {
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return err
	}
	if public, ok := fault.PublicCopy(err); ok {
		return public
	}
	return fault.Wrap(fault.KindInternal, "internal_error", command.LoadFailureMessage, false, err, command.StatusAction)
}
