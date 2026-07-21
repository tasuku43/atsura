// Package bundleexecute applies one freshly rebuilt, adapter-proven wrapper
// plan through an identity-bound source process.
package bundleexecute

import (
	"context"
	"errors"
	"fmt"

	"github.com/tasuku43/atsura/internal/app/portcheck"
	"github.com/tasuku43/atsura/internal/domain/bundletrust"
	"github.com/tasuku43/atsura/internal/domain/fault"
	"github.com/tasuku43/atsura/internal/domain/operation"
	"github.com/tasuku43/atsura/internal/domain/runtimeadmission"
	"github.com/tasuku43/atsura/internal/domain/sourceprocess"
	"github.com/tasuku43/atsura/internal/domain/tailoring"
	"github.com/tasuku43/atsura/internal/domain/tailoringbundle"
	"github.com/tasuku43/atsura/internal/domain/tailoringplan"
)

const Command = "bundle execute"

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

// Execute rebuilds and applies one transform plan. It never consumes preview
// output, retries a started source, or returns raw source bytes.
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
	if s == nil || portcheck.IsNil(s.bundles) || portcheck.IsNil(s.adoption) || portcheck.IsNil(s.identity) || portcheck.IsNil(s.compatibility) || portcheck.IsNil(s.processes) || portcheck.IsNil(s.parser) {
		return Result{}, fmt.Errorf("bundle execution adapters are not configured")
	}

	bundle, bundleDigest, err := s.bundles.Load(ctx, bundlePath)
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
	current, err := s.identity.Identify(ctx, bundle.Catalog.Source.ResolvedPath)
	if err != nil {
		return Result{}, classifyIdentity(err)
	}
	wanted := sourceprocess.Identity{ResolvedPath: bundle.Catalog.Source.ResolvedPath, SHA256: bundle.Catalog.Source.SHA256, Size: bundle.Catalog.Source.Size}
	if current != wanted {
		return Result{}, fault.New(fault.KindRejected, "bundle_source_drift", "The bundle source identity is no longer current.", false, statusAction())
	}
	plan, err := tailoringplan.Build(bundleDigest, bundle, current, attempt)
	if err != nil {
		return Result{}, classifyPlan(err)
	}
	planDigest, err := plan.Digest()
	if err != nil {
		return Result{}, fault.Wrap(fault.KindContract, "invalid_wrapper_plan", "The wrapper plan could not be encoded canonically.", false, err, previewAction())
	}
	outputPlan, present, err := plan.OutputPlan()
	if err != nil {
		return Result{}, fault.Wrap(fault.KindContract, "invalid_wrapper_plan", "The wrapper output stage is invalid.", false, err, previewAction())
	}
	if !present || plan.WrapperKind != tailoringbundle.WrapperTransform {
		return Result{}, fault.New(fault.KindUnsupported, "wrapper_runtime_not_supported", "This runtime slice requires a transforming wrapper with a typed output stage.", false, executeHelpAction())
	}
	if err := s.compatibility.VerifyRuntime(plan); err != nil {
		return Result{}, fault.New(fault.KindUnsupported, "wrapper_runtime_not_supported", runtimeAdmissionMessage(err), false, executeHelpAction())
	}
	request, err := plan.SourceRequest()
	if err != nil {
		return Result{}, fault.Wrap(fault.KindContract, "invalid_source_process_request", "The wrapper plan did not produce a valid bound source request.", false, err, previewAction())
	}
	if err := ctx.Err(); err != nil {
		return Result{}, err
	}

	processResult, processErr := s.processes.RunBound(ctx, request)
	if processErr != nil {
		return Result{}, classifyProcess(request, processResult, processErr)
	}
	if err := processResult.ValidateBound(request, true); err != nil || processResult.Attempts != 1 {
		return Result{}, unclassifiedProcess(err)
	}
	if len(processResult.Stderr) != 0 {
		return Result{}, fault.New(fault.KindContract, "source_stderr_not_supported", "The source succeeded with stderr, which this transform runtime does not yet represent.", false, executeHelpAction())
	}
	if err := ctx.Err(); err != nil {
		return Result{}, fault.Wrap(fault.KindCanceled, "source_output_processing_canceled", "Output processing was canceled after the source process started; replay is not known to be safe.", false, err, executeHelpAction())
	}
	parsed, err := s.parser.Parse(ctx, processResult.Stdout)
	if err != nil {
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return Result{}, fault.Wrap(fault.KindCanceled, "source_output_processing_canceled", "Output processing was canceled after the source process started; replay is not known to be safe.", false, err, executeHelpAction())
		}
		return Result{}, fault.Wrap(fault.KindContract, "source_json_invalid", "The successful source stdout did not satisfy the declared bounded JSON contract.", false, err, executeHelpAction())
	}
	output, err := tailoring.TransformJSON(outputPlan, parsed)
	if err != nil {
		return Result{}, fault.Wrap(fault.KindContract, "output_transform_failed", "The successful source JSON did not satisfy the declared output transformation.", false, err, executeHelpAction())
	}
	if err := ctx.Err(); err != nil {
		return Result{}, fault.Wrap(fault.KindCanceled, "source_output_processing_canceled", "Output processing was canceled after the source process started; replay is not known to be safe.", false, err, executeHelpAction())
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

func classifyProcess(request sourceprocess.BoundRequest, result sourceprocess.Result, err error) error {
	if validateErr := result.ValidateBound(request, false); validateErr != nil {
		return unclassifiedProcess(validateErr)
	}
	public, ok := fault.PublicCopy(err)
	if !ok {
		return unclassifiedProcess(err)
	}
	action := executeHelpAction()
	if result.Attempts == 0 {
		switch public.Code {
		case "source_identity_changed", "source_identity_unavailable", "unsafe_source_executable", "source_executable_not_found", "invalid_source_identity":
			action = statusAction()
		case "source_process_start_failed":
			action = fault.NextAction{Command: Command, Reason: "Retry the exact invocation only after confirming that no source process started."}
		}
		kind, message, retryable, safe := safeProcessFault(0, public.Code)
		if !safe {
			return unclassifiedProcess(err)
		}
		return fault.New(kind, public.Code, message, retryable, action)
	}
	kind, message, _, safe := safeProcessFault(1, public.Code)
	if !safe {
		return unclassifiedProcess(err)
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

func unclassifiedProcess(cause error) error {
	return fault.Wrap(fault.KindContract, "unclassified_source_execution_outcome", "The source execution result could not be classified as safe to retry.", false, cause, statusAction())
}

func classifyPlan(err error) error {
	switch {
	case errors.Is(err, tailoringplan.ErrSourceExecutableMismatch):
		return fault.Wrap(fault.KindInvalidInput, "source_executable_mismatch", "The attempted executable is not the bundle's exact requested executable or resolved path.", false, err, executeHelpAction())
	case errors.Is(err, tailoringplan.ErrCommandNotInSurface):
		return fault.Wrap(fault.KindNotFound, "command_not_in_surface", "The matched source command is absent from the tailored surface, so no execution plan exists.", false, err, executeHelpAction())
	case errors.Is(err, tailoringplan.ErrOptionNotInSurface):
		return fault.Wrap(fault.KindNotFound, "option_not_in_surface", "An attempted source option is absent from the tailored option surface, so no execution plan exists.", false, err, executeHelpAction())
	case errors.Is(err, tailoringplan.ErrInvalidInvocation):
		return fault.Wrap(fault.KindInvalidInput, "invalid_invocation", "The attempted source invocation cannot be resolved deterministically from the bundle catalog.", false, err, executeHelpAction())
	case errors.Is(err, tailoringplan.ErrInvalidPlan):
		return fault.Wrap(fault.KindContract, "invalid_wrapper_plan", "The bundle and invocation did not produce a complete wrapper plan.", false, err, previewAction())
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

func preserveLoad(err error) error {
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return err
	}
	if public, ok := fault.PublicCopy(err); ok {
		return public
	}
	return fault.Wrap(fault.KindInternal, "internal_error", "Bundle execution could not load its bundle.", false, err, statusAction())
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
