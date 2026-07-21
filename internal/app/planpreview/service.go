// Package planpreview implements deterministic, read-only plan preview.
package planpreview

import (
	"context"
	"errors"
	"fmt"

	"github.com/tasuku43/atsura/internal/app/portcheck"
	"github.com/tasuku43/atsura/internal/domain/fault"
	"github.com/tasuku43/atsura/internal/domain/operation"
	"github.com/tasuku43/atsura/internal/domain/tailoring"
)

// ConfigurationPort loads and validates one policy without executing its
// source command.
type ConfigurationPort interface {
	Load(context.Context, string) (tailoring.Policy, error)
}

// Service coordinates configuration loading and pure plan compilation.
type Service struct {
	configurations ConfigurationPort
}

// New creates a plan preview service.
func New(configurations ConfigurationPort) *Service {
	return &Service{configurations: configurations}
}

// Preview compiles one invocation into a detached plan and never starts a
// source process.
func (s *Service) Preview(ctx context.Context, intent operation.Intent, path string, argv []string) (tailoring.Plan, error) {
	if ctx == nil {
		return tailoring.Plan{}, fmt.Errorf("plan preview context is nil")
	}
	if err := ctx.Err(); err != nil {
		return tailoring.Plan{}, err
	}
	if err := intent.Validate(); err != nil {
		return tailoring.Plan{}, fmt.Errorf("plan preview intent: %w", err)
	}
	if intent.Command != "plan preview" || intent.Effect != operation.EffectRead {
		return tailoring.Plan{}, fmt.Errorf("plan preview requires the plan preview read intent")
	}
	if s == nil || portcheck.IsNil(s.configurations) {
		return tailoring.Plan{}, fmt.Errorf("plan configuration loader is not configured")
	}

	policy, err := s.configurations.Load(ctx, path)
	if contextErr := ctx.Err(); contextErr != nil {
		return tailoring.Plan{}, contextErr
	}
	if err != nil {
		if public, ok := fault.PublicCopy(err); ok {
			return tailoring.Plan{}, public
		}
		return tailoring.Plan{}, internalError(err)
	}
	plan, err := tailoring.Compile(policy, tailoring.Invocation{Argv: argv})
	if err == nil {
		return plan, nil
	}
	switch {
	case errors.Is(err, tailoring.ErrNoMatch):
		return tailoring.Plan{}, fault.Wrap(fault.KindNotFound, "plan_rule_not_matched", "The source command does not match this configuration.", false, err, helpAction())
	case errors.Is(err, tailoring.ErrInvalidInvocation):
		return tailoring.Plan{}, fault.Wrap(fault.KindInvalidInput, "invalid_plan_invocation", "The source command invocation is invalid.", false, err, helpAction())
	case errors.Is(err, tailoring.ErrInvalidPolicy):
		return tailoring.Plan{}, fault.Wrap(fault.KindInvalidInput, "invalid_plan_configuration", "The plan configuration is invalid.", false, err, helpAction())
	default:
		return tailoring.Plan{}, internalError(err)
	}
}

func helpAction() fault.NextAction {
	return fault.NextAction{Command: "help plan preview", Reason: "Review the plan preview contract and correct the input."}
}

func internalError(err error) *fault.Error {
	return fault.Wrap(fault.KindInternal, "internal_error", "The plan preview could not be completed.", false, err, helpAction())
}
