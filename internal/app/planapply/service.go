// Package planapply rebuilds and applies one adopted bundle's wrapper plan
// through the shared, host-neutral source-process boundary.
package planapply

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/tasuku43/atsura/internal/app/portcheck"
	"github.com/tasuku43/atsura/internal/domain/bundletrust"
	"github.com/tasuku43/atsura/internal/domain/fault"
	"github.com/tasuku43/atsura/internal/domain/processorprocess"
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

// ProcessorIdentityPort fingerprints the exact bundle-bound processor without
// starting it or consulting PATH.
type ProcessorIdentityPort interface {
	Identify(context.Context, string) (processorprocess.Identity, error)
}

// ProcessorProcessPort starts at most one exact isolated processor attempt.
type ProcessorProcessPort interface {
	Run(context.Context, processorprocess.Request) (processorprocess.Result, error)
}

// ProcessorCompatibilityPort proves the complete source/processor plan tuple
// without performing I/O.
type ProcessorCompatibilityPort interface {
	VerifyPlan(tailoringplan.Plan) error
}

// OptimizerAdmissionPort independently derives the only output that a finite
// optimizer may return instead of its byte-identical input.
type OptimizerAdmissionPort interface {
	ExpectedSummary([]byte) (string, bool)
}

// ProcessorSupport is optional for projection and source-stream plans. The
// original-preserving optimizer requires exactly one complete support value.
type ProcessorSupport struct {
	Identity      ProcessorIdentityPort
	Processes     ProcessorProcessPort
	Compatibility ProcessorCompatibilityPort
	Admission     OptimizerAdmissionPort
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
// ordinary command spelling. Each Allow field is façade-owned: a plan with a
// disallowed result mode fails before either source or processor execution.
type Request struct {
	BundlePath                       string
	ExpectedBundleDigest             string
	DeriveExecutableFromLoadedBundle bool
	AllowSourceStreamPassthrough     bool
	AllowOriginalPreservingOptimizer bool
	Attempt                          tailoringplan.Attempt
	Command                          CommandContext
}

// TransformedJSONResult is the sole payload for a transformed_json result.
type TransformedJSONResult struct {
	Render   tailoring.RenderFormat
	Output   tailoring.OutputResult
	ExitCode int
}

// SourceStreamResult is the sole payload for a source_stream_passthrough
// result. Bytes are detached from process-adapter storage before they cross the
// application boundary.
type SourceStreamResult struct {
	Stdout   []byte
	Stderr   []byte
	ExitCode int
}

// OptimizerDisposition states which reviewed output became caller-visible.
// It never describes an inferred internal processor branch.
type OptimizerDisposition string

const (
	OptimizerPreservedBeforeProcessor OptimizerDisposition = "preserved_before_processor"
	OptimizerPreservedAfterProcessor  OptimizerDisposition = "preserved_after_processor"
	OptimizerOptimized                OptimizerDisposition = "optimized"
)

// OptimizerResult is the sole payload for original_preserving_optimizer. Its
// bytes are either the exact conventional source result selected before a
// processor attempt or one of the two admitted processor postconditions.
type OptimizerResult struct {
	Stdout      []byte
	Stderr      []byte
	ExitCode    int
	Disposition OptimizerDisposition
}

// Result is a plan-declared union. Exactly one payload is present and agrees
// with ResultMode; shared metadata never duplicates result bytes or rendering
// facts.
type Result struct {
	BundleDigest             string
	PlanDigest               string
	MatchedCommand           []string
	WrapperKind              tailoringbundle.WrapperKind
	ResultMode               tailoringplan.ResultMode
	TransformedJSON          *TransformedJSONResult
	SourceStream             *SourceStreamResult
	Optimizer                *OptimizerResult
	SourceProcessAttempts    int
	ProcessorProcessAttempts int
}

// Validate proves the result union before a presentation boundary interprets
// either payload.
func (r Result) Validate() error {
	if r.SourceProcessAttempts != 1 {
		return fmt.Errorf("plan application result requires exactly one source process attempt")
	}
	switch r.ResultMode {
	case tailoringplan.ResultModeTransformedJSON:
		if r.TransformedJSON == nil || r.SourceStream != nil || r.Optimizer != nil || r.ProcessorProcessAttempts != 0 || r.WrapperKind != tailoringbundle.WrapperTransform {
			return fmt.Errorf("transformed JSON result payload is incomplete or contradictory")
		}
		if r.TransformedJSON.ExitCode != 0 || r.TransformedJSON.Render != tailoring.RenderCompactJSON {
			return fmt.Errorf("transformed JSON result framing is invalid")
		}
		if err := r.TransformedJSON.Output.Validate(); err != nil {
			return fmt.Errorf("transformed JSON result: %w", err)
		}
	case tailoringplan.ResultModeSourceStreamPassthrough:
		if r.TransformedJSON != nil || r.SourceStream == nil || r.Optimizer != nil || r.ProcessorProcessAttempts != 0 || (r.WrapperKind != tailoringbundle.WrapperIdentity && r.WrapperKind != tailoringbundle.WrapperTransform) {
			return fmt.Errorf("source-stream result payload is incomplete or contradictory")
		}
		if r.SourceStream.ExitCode < 0 || len(r.SourceStream.Stdout) > sourceprocess.MaxStdoutBytes || len(r.SourceStream.Stderr) > sourceprocess.MaxStderrBytes {
			return fmt.Errorf("source-stream result exceeds its conventional status or capture bounds")
		}
	case tailoringplan.ResultModeOriginalPreservingOptimizer:
		if r.TransformedJSON != nil || r.SourceStream != nil || r.Optimizer == nil || r.WrapperKind != tailoringbundle.WrapperTransform {
			return fmt.Errorf("optimizer result payload is incomplete or contradictory")
		}
		if r.Optimizer.ExitCode < 0 || len(r.Optimizer.Stdout) > sourceprocess.MaxStdoutBytes || len(r.Optimizer.Stderr) > sourceprocess.MaxStderrBytes {
			return fmt.Errorf("optimizer result exceeds its conventional status or capture bounds")
		}
		switch r.Optimizer.Disposition {
		case OptimizerPreservedBeforeProcessor:
			if r.ProcessorProcessAttempts != 0 {
				return fmt.Errorf("pre-processor preservation cannot contain a processor attempt")
			}
		case OptimizerPreservedAfterProcessor, OptimizerOptimized:
			if r.ProcessorProcessAttempts != 1 || r.Optimizer.ExitCode != 0 || len(r.Optimizer.Stderr) != 0 || len(r.Optimizer.Stdout) == 0 {
				return fmt.Errorf("post-processor optimizer result framing is invalid")
			}
			if r.Optimizer.Disposition == OptimizerOptimized && (!utf8.Valid(r.Optimizer.Stdout) || bytes.IndexAny(r.Optimizer.Stdout, "\r\n") >= 0) {
				return fmt.Errorf("optimized summary must be newline-free UTF-8")
			}
		default:
			return fmt.Errorf("optimizer disposition is missing or invalid")
		}
	default:
		return fmt.Errorf("plan application result mode is missing or invalid")
	}
	return nil
}

// Service owns the one application-level plan application path shared by
// public façades. It is safe to construct once in the composition root and
// inject into each façade that supplies its own CommandContext.
type Service struct {
	bundles           BundlePort
	adoption          AdoptionPort
	identity          IdentityPort
	compatibility     CompatibilityPort
	processes         ProcessPort
	parser            ParserPort
	processor         ProcessorSupport
	processorSupports int
}

func New(bundles BundlePort, adoption AdoptionPort, identity IdentityPort, compatibility CompatibilityPort, processes ProcessPort, parser ParserPort, processor ...ProcessorSupport) *Service {
	service := &Service{
		bundles: bundles, adoption: adoption, identity: identity, compatibility: compatibility, processes: processes, parser: parser,
		processorSupports: len(processor),
	}
	if len(processor) == 1 {
		service.processor = processor[0]
	}
	return service
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

func (s *Service) processorConfigured() bool {
	return s != nil && s.processorSupports == 1 &&
		!portcheck.IsNil(s.processor.Identity) &&
		!portcheck.IsNil(s.processor.Processes) &&
		!portcheck.IsNil(s.processor.Compatibility) &&
		!portcheck.IsNil(s.processor.Admission)
}

// Apply strictly loads and optionally closes over one exact bundle digest,
// rebuilds its plan, and applies one admitted plan result mode once. It never
// consumes preview output or retries a started source. Source bytes cross this
// boundary only for an explicitly allowed source_stream_passthrough plan.
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
	switch plan.ResultMode {
	case tailoringplan.ResultModeTransformedJSON:
		if !present || plan.WrapperKind != tailoringbundle.WrapperTransform {
			return Result{}, fault.New(fault.KindUnsupported, "wrapper_runtime_not_supported", "This runtime slice requires a transforming wrapper with a typed output stage.", false, request.Command.RuntimeHelpAction)
		}
	case tailoringplan.ResultModeSourceStreamPassthrough:
		if !request.AllowSourceStreamPassthrough || present {
			return Result{}, fault.New(fault.KindUnsupported, "wrapper_runtime_not_supported", "This command does not admit the plan-declared source-stream result mode.", false, request.Command.RuntimeHelpAction)
		}
	case tailoringplan.ResultModeOriginalPreservingOptimizer:
		if present {
			return Result{}, fault.New(fault.KindContract, "invalid_wrapper_plan", "The optimizer plan contains a contradictory projection stage.", false, request.Command.PlanPreviewAction)
		}
		if !request.AllowOriginalPreservingOptimizer {
			return optimizerResultBase(bundleDigest, planDigest, plan, 0, 0), fault.New(fault.KindUnsupported, "wrapper_runtime_not_supported", "This command does not admit the plan-declared original-preserving optimizer result mode.", false, request.Command.RuntimeHelpAction)
		}
		if !s.processorConfigured() {
			return optimizerResultBase(bundleDigest, planDigest, plan, 0, 0), fault.New(fault.KindUnsupported, "wrapper_runtime_not_supported", "The original-preserving optimizer runtime is not configured.", false, request.Command.RuntimeHelpAction)
		}
	default:
		return Result{}, fault.New(fault.KindContract, "invalid_wrapper_plan", "The wrapper plan result mode is invalid.", false, request.Command.PlanPreviewAction)
	}
	if err := s.compatibility.VerifyRuntime(plan); err != nil {
		return Result{}, fault.New(fault.KindUnsupported, "wrapper_runtime_not_supported", runtimeAdmissionMessage(err), false, request.Command.RuntimeHelpAction)
	}
	if plan.ResultMode == tailoringplan.ResultModeOriginalPreservingOptimizer {
		if err := s.processor.Compatibility.VerifyPlan(plan); err != nil {
			return optimizerResultBase(bundleDigest, planDigest, plan, 0, 0), fault.New(fault.KindUnsupported, "wrapper_runtime_not_supported", "The source and processor plan tuple is not admitted by this runtime.", false, request.Command.RuntimeHelpAction)
		}
		wantedProcessor := plan.Processor.Observation.Identity
		currentProcessor, err := s.processor.Identity.Identify(ctx, wantedProcessor.ResolvedPath)
		if err != nil {
			return optimizerResultBase(bundleDigest, planDigest, plan, 0, 0), classifyProcessorIdentity(err, request.Command, false)
		}
		if currentProcessor != wantedProcessor {
			return optimizerResultBase(bundleDigest, planDigest, plan, 0, 0), processorIdentityChanged(request.Command)
		}
	}
	processRequest, err := plan.SourceRequest()
	if err != nil {
		return Result{}, fault.Wrap(fault.KindContract, "invalid_source_process_request", "The wrapper plan did not produce a valid bound source request.", false, err, request.Command.PlanPreviewAction)
	}
	if err := ctx.Err(); err != nil {
		return Result{}, err
	}

	processResult, processErr := s.processes.RunBound(ctx, processRequest)
	if plan.ResultMode == tailoringplan.ResultModeSourceStreamPassthrough {
		return sourceStreamResult(ctx, request.Command, plan, bundleDigest, planDigest, processRequest, processResult, processErr)
	}
	if plan.ResultMode == tailoringplan.ResultModeOriginalPreservingOptimizer {
		return s.optimizerResult(ctx, request.Command, plan, bundleDigest, planDigest, processRequest, processResult, processErr)
	}
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
	result := Result{
		BundleDigest: bundleDigest, PlanDigest: planDigest, MatchedCommand: append([]string{}, plan.MatchedCommand...),
		WrapperKind: plan.WrapperKind, ResultMode: plan.ResultMode,
		TransformedJSON:       &TransformedJSONResult{Render: outputPlan.Render, Output: output, ExitCode: processResult.ExitCode},
		SourceProcessAttempts: processResult.Attempts,
	}
	if err := result.Validate(); err != nil {
		return Result{}, fault.Wrap(fault.KindContract, "unclassified_source_execution_outcome", "The source execution result could not be classified as safe to retry.", false, err, request.Command.StatusAction)
	}
	return result, nil
}

func (s *Service) optimizerResult(ctx context.Context, command CommandContext, plan tailoringplan.Plan, bundleDigest, planDigest string, sourceRequest sourceprocess.BoundRequest, sourceResult sourceprocess.Result, sourceErr error) (Result, error) {
	sourceAttempts := boundedAttemptCount(sourceResult.Attempts)
	if err := sourceResult.ValidateBoundCompletion(sourceRequest); err != nil {
		if sourceErr != nil {
			return optimizerResultBase(bundleDigest, planDigest, plan, sourceAttempts, 0), classifyProcess(sourceRequest, sourceResult, sourceErr, command)
		}
		return optimizerResultBase(bundleDigest, planDigest, plan, sourceAttempts, 0), unclassifiedProcess(err, command)
	}
	if sourceErr == nil {
		if sourceResult.ExitCode != 0 {
			return optimizerResultBase(bundleDigest, planDigest, plan, 1, 0), unclassifiedProcess(fmt.Errorf("nil process error accompanied nonzero exit code %d", sourceResult.ExitCode), command)
		}
	} else {
		public, ok := fault.PublicCopy(sourceErr)
		if !ok || public.Kind != fault.KindRejected || public.Code != "source_command_failed" || public.Retryable || sourceResult.ExitCode <= 0 {
			return optimizerResultBase(bundleDigest, planDigest, plan, 1, 0), classifyProcess(sourceRequest, sourceResult, sourceErr, command)
		}
	}
	if err := ctx.Err(); err != nil {
		return optimizerResultBase(bundleDigest, planDigest, plan, 1, 0), fault.Wrap(fault.KindCanceled, "source_output_processing_canceled", "Output processing was canceled after the source process started; replay is not known to be safe.", false, err, command.RuntimeHelpAction)
	}

	// A conventional result outside the frozen pass-only grammar remains the
	// exact plan-authorized source result. This decision is complete before a
	// processor starts and is not a fallback from processor failure.
	if sourceResult.ExitCode != 0 || len(sourceResult.Stderr) != 0 {
		return completedOptimizerResult(command, bundleDigest, planDigest, plan, sourceResult.Stdout, sourceResult.Stderr, sourceResult.ExitCode, OptimizerPreservedBeforeProcessor, 0)
	}
	expectedSummary, eligible := s.processor.Admission.ExpectedSummary(append([]byte(nil), sourceResult.Stdout...))
	if !eligible || !validExpectedSummary(expectedSummary, sourceResult.Stdout) {
		return completedOptimizerResult(command, bundleDigest, planDigest, plan, sourceResult.Stdout, sourceResult.Stderr, sourceResult.ExitCode, OptimizerPreservedBeforeProcessor, 0)
	}

	wantedProcessor := plan.Processor.Observation.Identity
	currentProcessor, err := s.processor.Identity.Identify(ctx, wantedProcessor.ResolvedPath)
	if err != nil {
		return optimizerResultBase(bundleDigest, planDigest, plan, 1, 0), classifyProcessorIdentity(err, command, true)
	}
	if currentProcessor != wantedProcessor {
		return optimizerResultBase(bundleDigest, planDigest, plan, 1, 0), processorIdentityChanged(command)
	}
	processorRequest, err := plan.ProcessorRequest(sourceResult.Stdout)
	if err != nil {
		return optimizerResultBase(bundleDigest, planDigest, plan, 1, 0), fault.Wrap(fault.KindContract, "invalid_processor_process_request", "The optimizer plan did not produce a valid bound processor request.", false, err, command.PlanPreviewAction)
	}
	if err := ctx.Err(); err != nil {
		return optimizerResultBase(bundleDigest, planDigest, plan, 1, 0), fault.Wrap(fault.KindCanceled, "processor_execution_canceled", "Execution was canceled after the source completed and before an optimizer result was available; replay is not known to be safe.", false, err, command.RuntimeHelpAction)
	}

	processorResult, processorErr := s.processor.Processes.Run(ctx, processorRequest)
	processorAttempts := boundedAttemptCount(processorResult.Attempts)
	if processorErr != nil {
		if err := processorResult.Validate(processorRequest, false); err != nil {
			return optimizerResultBase(bundleDigest, planDigest, plan, 1, processorAttempts), unclassifiedProcessor(err, command)
		}
		return optimizerResultBase(bundleDigest, planDigest, plan, 1, processorAttempts), classifyProcessorProcess(processorErr, processorAttempts, command)
	}
	if err := processorResult.Validate(processorRequest, true); err != nil || processorResult.Attempts != 1 {
		return optimizerResultBase(bundleDigest, planDigest, plan, 1, processorAttempts), unclassifiedProcessor(err, command)
	}
	if err := ctx.Err(); err != nil {
		return optimizerResultBase(bundleDigest, planDigest, plan, 1, 1), fault.Wrap(fault.KindCanceled, "processor_execution_canceled", "The caller canceled after the processor started; replay is not known to be safe.", false, err, command.RuntimeHelpAction)
	}
	if len(processorResult.Stderr) != 0 {
		return optimizerResultBase(bundleDigest, planDigest, plan, 1, 1), processorOutputNotAdmitted(command)
	}
	if bytes.Equal(processorResult.Stdout, sourceResult.Stdout) {
		return completedOptimizerResult(command, bundleDigest, planDigest, plan, processorResult.Stdout, nil, sourceResult.ExitCode, OptimizerPreservedAfterProcessor, 1)
	}
	if bytes.Equal(processorResult.Stdout, []byte(expectedSummary)) && len(processorResult.Stdout) < len(sourceResult.Stdout) {
		return completedOptimizerResult(command, bundleDigest, planDigest, plan, processorResult.Stdout, nil, sourceResult.ExitCode, OptimizerOptimized, 1)
	}
	return optimizerResultBase(bundleDigest, planDigest, plan, 1, 1), processorOutputNotAdmitted(command)
}

func validExpectedSummary(summary string, input []byte) bool {
	return summary != "" && utf8.ValidString(summary) && !strings.ContainsAny(summary, "\r\n") && len(summary) < len(input) && len(summary) <= processorprocess.MaxStdoutBytes
}

func optimizerResultBase(bundleDigest, planDigest string, plan tailoringplan.Plan, sourceAttempts, processorAttempts int) Result {
	return Result{
		BundleDigest: bundleDigest, PlanDigest: planDigest, MatchedCommand: append([]string{}, plan.MatchedCommand...),
		WrapperKind: plan.WrapperKind, ResultMode: plan.ResultMode,
		SourceProcessAttempts: sourceAttempts, ProcessorProcessAttempts: processorAttempts,
	}
}

func completedOptimizerResult(command CommandContext, bundleDigest, planDigest string, plan tailoringplan.Plan, stdout, stderr []byte, exitCode int, disposition OptimizerDisposition, processorAttempts int) (Result, error) {
	result := optimizerResultBase(bundleDigest, planDigest, plan, 1, processorAttempts)
	result.Optimizer = &OptimizerResult{
		Stdout: append([]byte(nil), stdout...), Stderr: append([]byte(nil), stderr...), ExitCode: exitCode, Disposition: disposition,
	}
	if err := result.Validate(); err != nil {
		if disposition == OptimizerPreservedBeforeProcessor {
			return optimizerResultBase(bundleDigest, planDigest, plan, 1, processorAttempts), unclassifiedProcess(err, command)
		}
		return optimizerResultBase(bundleDigest, planDigest, plan, 1, processorAttempts), unclassifiedProcessor(err, command)
	}
	return result, nil
}

func boundedAttemptCount(value int) int {
	if value == 1 {
		return 1
	}
	return 0
}

func classifyProcessorIdentity(err error, command CommandContext, afterSource bool) error {
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		if !afterSource {
			return err
		}
		return fault.Wrap(fault.KindCanceled, "processor_execution_canceled", "Processor identity revalidation was canceled after the source completed; replay is not known to be safe.", false, err, command.StatusAction)
	}
	if public, ok := fault.PublicCopy(err); ok {
		if public.Code == "processor_identity_changed" {
			return processorIdentityChanged(command)
		}
		if !afterSource {
			return fault.New(public.Kind, public.Code, public.Message, public.Retryable, command.StatusAction)
		}
		if public.Code == "processor_identity_unavailable" {
			return fault.New(fault.KindUnavailable, "processor_identity_unavailable_after_source", "The bundle-bound processor identity became unavailable after the source completed; replay is not known to be safe.", false, command.StatusAction)
		}
		return fault.New(public.Kind, public.Code, public.Message, false, command.StatusAction)
	}
	if afterSource {
		return fault.Wrap(fault.KindUnavailable, "processor_identity_unavailable_after_source", "The bundle-bound processor identity became unavailable after the source completed; replay is not known to be safe.", false, err, command.StatusAction)
	}
	return fault.Wrap(fault.KindUnavailable, "processor_identity_unavailable", "The bundle-bound processor identity could not be assessed before the source started.", true, err, command.StatusAction)
}

func processorIdentityChanged(command CommandContext) error {
	return fault.New(fault.KindRejected, "processor_identity_changed", "The bundle-bound processor identity is no longer current.", false, command.StatusAction)
}

func classifyProcessorProcess(err error, attempts int, command CommandContext) error {
	public, ok := fault.PublicCopy(err)
	if !ok {
		return unclassifiedProcessor(err, command)
	}
	kind, code, message, safe := safeProcessorFault(attempts, public.Code)
	if !safe {
		return unclassifiedProcessor(err, command)
	}
	return fault.New(kind, code, message, false, command.StatusAction)
}

func safeProcessorFault(attempts int, code string) (fault.Kind, string, string, bool) {
	if attempts == 0 {
		switch code {
		case "processor_identity_changed":
			return fault.KindRejected, code, "The processor identity changed before it could be started after the source completed.", true
		case "processor_identity_unavailable":
			return fault.KindUnavailable, "processor_identity_unavailable_after_source", "The processor identity became unavailable after the source completed.", true
		case "invalid_processor_executable", "unsafe_processor_executable":
			return fault.KindInvalidInput, code, "The bundle-bound processor executable no longer satisfies its executable contract.", true
		case "invalid_processor_identity":
			return fault.KindContract, code, "The bundle-bound processor identity no longer satisfies its identity contract.", true
		case "processor_environment_setup_failed":
			return fault.KindUnavailable, "processor_environment_setup_failed_after_source", "The isolated processor environment could not be prepared after the source completed.", true
		case "processor_process_start_failed":
			return fault.KindUnavailable, "processor_process_start_failed_after_source", "The processor could not be started after the source completed.", true
		case "processor_cleanup_failed":
			return fault.KindUnavailable, code, "The isolated processor environment could not be cleaned up after the source completed.", true
		}
		return "", "", "", false
	}
	switch code {
	case "processor_identity_changed":
		return fault.KindRejected, code, "The processor identity changed during execution.", true
	case "processor_stdout_too_large", "processor_stderr_too_large":
		return fault.KindContract, code, "The processor exceeded its declared output bounds.", true
	case "processor_execution_canceled":
		return fault.KindCanceled, code, "The processor was canceled after it started; replay is not known to be safe.", true
	case "processor_timeout":
		return fault.KindUnavailable, code, "The processor exceeded its declared timeout after the source completed.", true
	case "processor_command_failed":
		return fault.KindRejected, code, "The processor exited without an admitted result after the source completed.", true
	case "processor_process_wait_failed":
		return fault.KindUnavailable, code, "The processor result could not be collected after it started.", true
	case "processor_cleanup_failed":
		return fault.KindUnavailable, code, "The isolated processor environment could not be cleaned up after execution.", true
	default:
		return "", "", "", false
	}
}

func processorOutputNotAdmitted(command CommandContext) error {
	return fault.New(fault.KindContract, "processor_output_not_admitted", "The processor output did not satisfy either admitted original-preserving postcondition.", false, command.StatusAction)
}

func unclassifiedProcessor(cause error, command CommandContext) error {
	return fault.Wrap(fault.KindContract, "unclassified_processor_execution_outcome", "The processor result could not be classified; replay is not known to be safe.", false, cause, command.StatusAction)
}

func sourceStreamResult(ctx context.Context, command CommandContext, plan tailoringplan.Plan, bundleDigest, planDigest string, processRequest sourceprocess.BoundRequest, processResult sourceprocess.Result, processErr error) (Result, error) {
	if err := processResult.ValidateBoundCompletion(processRequest); err != nil {
		if processErr != nil {
			return Result{}, classifyProcess(processRequest, processResult, processErr, command)
		}
		return Result{}, unclassifiedProcess(err, command)
	}
	if processErr == nil {
		if processResult.ExitCode != 0 {
			return Result{}, unclassifiedProcess(fmt.Errorf("nil process error accompanied nonzero exit code %d", processResult.ExitCode), command)
		}
	} else {
		public, ok := fault.PublicCopy(processErr)
		if !ok || public.Kind != fault.KindRejected || public.Code != "source_command_failed" || public.Retryable || processResult.ExitCode <= 0 {
			return Result{}, classifyProcess(processRequest, processResult, processErr, command)
		}
	}
	if err := ctx.Err(); err != nil {
		return Result{}, fault.Wrap(fault.KindCanceled, "source_output_processing_canceled", "Output processing was canceled after the source process started; replay is not known to be safe.", false, err, command.RuntimeHelpAction)
	}
	result := Result{
		BundleDigest: bundleDigest, PlanDigest: planDigest, MatchedCommand: append([]string{}, plan.MatchedCommand...),
		WrapperKind: plan.WrapperKind, ResultMode: plan.ResultMode,
		SourceStream: &SourceStreamResult{
			Stdout:   append([]byte(nil), processResult.Stdout...),
			Stderr:   append([]byte(nil), processResult.Stderr...),
			ExitCode: processResult.ExitCode,
		},
		SourceProcessAttempts: processResult.Attempts,
	}
	if err := result.Validate(); err != nil {
		return Result{}, unclassifiedProcess(err, command)
	}
	return result, nil
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
