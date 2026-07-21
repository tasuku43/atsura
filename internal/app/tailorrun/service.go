// Package tailorrun implements the bounded read-only local tailoring outcome.
package tailorrun

import (
	"context"
	"errors"
	"fmt"

	"github.com/tasuku43/atsura/internal/app/portcheck"
	"github.com/tasuku43/atsura/internal/domain/fault"
	"github.com/tasuku43/atsura/internal/domain/operation"
	"github.com/tasuku43/atsura/internal/domain/sourceprocess"
	"github.com/tasuku43/atsura/internal/domain/tailoring"
)

const (
	SourceTimeout     = sourceprocess.MaxTimeout
	SourceStdoutLimit = sourceprocess.MaxStdoutBytes
	SourceStderrLimit = sourceprocess.MaxStderrBytes
)

// ConfigurationPort loads one explicitly selected policy.
type ConfigurationPort interface {
	Load(context.Context, string) (tailoring.Policy, error)
}

// ProcessPort performs zero or one direct source-process attempts.
type ProcessPort interface {
	Run(context.Context, sourceprocess.Request) (sourceprocess.Result, error)
}

// JSONParserPort converts bounded source bytes into typed JSON values.
type JSONParserPort interface {
	Parse(context.Context, []byte) (tailoring.JSONValue, error)
}

// Result is the complete semantic success returned to presentation.
type Result struct {
	Plan                  tailoring.Plan
	Output                tailoring.OutputResult
	SourceStderr          []byte
	SourceProcessAttempts int
}

// Validate rejects adapter or orchestration drift before presentation.
func (r Result) Validate() error {
	if r.Plan.Decision != tailoring.DecisionAllow || r.Plan.Effect != operation.EffectRead || !r.Plan.Executable {
		return fmt.Errorf("run result requires an executable allowed read plan")
	}
	if err := r.Output.Validate(); err != nil {
		return fmt.Errorf("run output: %w", err)
	}
	if r.SourceProcessAttempts != 1 {
		return fmt.Errorf("run result requires exactly one direct source-process attempt")
	}
	if len(r.SourceStderr) > SourceStderrLimit {
		return fmt.Errorf("run stderr exceeds its bound")
	}
	return nil
}

// Service coordinates the controlled local run.
type Service struct {
	configurations ConfigurationPort
	processes      ProcessPort
	parser         JSONParserPort
}

// New creates a local run service.
func New(configurations ConfigurationPort, processes ProcessPort, parser JSONParserPort) *Service {
	return &Service{configurations: configurations, processes: processes, parser: parser}
}

// Run reloads and compiles one policy, admits only allow/read, starts at most
// one process, and transforms only a validated successful result.
func (s *Service) Run(ctx context.Context, intent operation.Intent, path string, argv []string) (Result, error) {
	if ctx == nil {
		return Result{}, fmt.Errorf("tailored run context is nil")
	}
	if err := ctx.Err(); err != nil {
		return Result{}, err
	}
	if err := intent.Validate(); err != nil {
		return Result{}, fmt.Errorf("tailored run intent: %w", err)
	}
	if intent.Command != "run" || intent.Effect != operation.EffectRead {
		return Result{}, fmt.Errorf("tailored run requires the run read intent")
	}
	if s == nil || portcheck.IsNil(s.configurations) || portcheck.IsNil(s.processes) || portcheck.IsNil(s.parser) {
		return Result{}, fmt.Errorf("tailored run dependencies are not configured")
	}

	policy, err := s.configurations.Load(ctx, path)
	if err := afterCall(ctx, err); err != nil {
		return Result{}, mapBoundaryError(err)
	}
	plan, err := tailoring.Compile(policy, tailoring.Invocation{Argv: argv})
	if err != nil {
		return Result{}, mapCompileError(err)
	}
	if plan.Effect != operation.EffectRead {
		return Result{}, fault.Wrap(fault.KindRejected, "unsupported_source_effect", "Schema 1 can execute only a reviewed read source effect.", false, nil, helpAction())
	}
	if plan.Decision != tailoring.DecisionAllow || !plan.Executable {
		return Result{}, fault.New(fault.KindRejected, "policy_rejected", "The selected policy rejected this source invocation.", false, helpAction())
	}
	request := sourceprocess.Request{
		Executable: plan.TransformedArgv[0], Args: append([]string{}, plan.TransformedArgv[1:]...),
		Timeout: SourceTimeout, StdoutLimit: SourceStdoutLimit, StderrLimit: SourceStderrLimit,
	}
	if err := request.Validate(); err != nil {
		return Result{}, fault.Wrap(fault.KindContract, "invalid_source_process_request", "The compiled source process request is invalid.", false, err, helpAction())
	}
	processResult, processErr := s.processes.Run(ctx, request)
	if validationErr := processResult.Validate(request, processErr == nil); validationErr != nil {
		return Result{}, fault.Wrap(fault.KindContract, "invalid_source_process_result", "The source process adapter returned an invalid result.", false, validationErr, helpAction())
	}
	if err := afterCall(ctx, processErr); err != nil {
		return Result{}, mapBoundaryError(err)
	}

	parsed, err := s.parser.Parse(ctx, processResult.Stdout)
	if err := afterCall(ctx, err); err != nil {
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return Result{}, err
		}
		return Result{}, fault.Wrap(fault.KindContract, "source_json_invalid", "The successful source stdout is not valid supported JSON.", false, err, previewAction())
	}
	output, err := tailoring.TransformJSON(plan.Output, parsed)
	if err != nil {
		return Result{}, fault.Wrap(fault.KindContract, "output_transform_failed", "The successful source JSON does not satisfy the declared output plan.", false, err, previewAction())
	}
	result := Result{
		Plan: plan, Output: output, SourceStderr: append([]byte{}, processResult.Stderr...),
		SourceProcessAttempts: processResult.Attempts,
	}
	if err := result.Validate(); err != nil {
		return Result{}, fault.Wrap(fault.KindContract, "invalid_run_result", "The tailored run result is invalid.", false, err, helpAction())
	}
	return result, nil
}

func afterCall(ctx context.Context, err error) error {
	if contextErr := ctx.Err(); contextErr != nil {
		return contextErr
	}
	return err
}

func mapCompileError(err error) error {
	switch {
	case errors.Is(err, tailoring.ErrNoMatch):
		return fault.Wrap(fault.KindNotFound, "plan_rule_not_matched", "The source command does not match this configuration.", false, err, previewAction())
	case errors.Is(err, tailoring.ErrInvalidInvocation):
		return fault.Wrap(fault.KindInvalidInput, "invalid_plan_invocation", "The source command invocation is invalid.", false, err, helpAction())
	case errors.Is(err, tailoring.ErrInvalidPolicy):
		return fault.Wrap(fault.KindInvalidInput, "invalid_plan_configuration", "The plan configuration is invalid.", false, err, helpAction())
	default:
		return fault.Wrap(fault.KindInternal, "internal_error", "The tailored run could not be completed.", false, err, helpAction())
	}
}

func mapBoundaryError(err error) error {
	if public, ok := fault.PublicCopy(err); ok {
		return public
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return err
	}
	return fault.Wrap(fault.KindInternal, "internal_error", "The tailored run could not be completed.", false, err, helpAction())
}

func helpAction() fault.NextAction {
	return fault.NextAction{Command: "help run", Reason: "Review the local run contract and correct the input."}
}

func previewAction() fault.NextAction {
	return fault.NextAction{Command: "plan preview", Reason: "Preview the selected policy and source invocation before retrying."}
}
