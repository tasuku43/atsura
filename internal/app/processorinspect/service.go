// Package processorinspect coordinates one explicit finite processor
// observation without installing, discovering, or configuring the processor.
package processorinspect

import (
	"context"
	"errors"
	"fmt"

	"github.com/tasuku43/atsura/internal/app/portcheck"
	"github.com/tasuku43/atsura/internal/domain/fault"
	"github.com/tasuku43/atsura/internal/domain/operation"
	"github.com/tasuku43/atsura/internal/domain/processorprocess"
)

const Command = "processor inspect"

// InspectorPort observes one explicitly selected processor executable under a
// finite adapter-owned contract.
type InspectorPort interface {
	Inspect(context.Context, string) (processorprocess.Observation, error)
}

// Registration binds one public selector to one exact namespaced adapter kind.
// The application owns the finite selector registry but no vendor semantics.
type Registration struct {
	Selector    string
	AdapterKind string
	Inspector   InspectorPort
}

type registeredInspector struct {
	adapterKind string
	inspector   InspectorPort
}

// Service selects a configured processor inspector and validates its evidence.
type Service struct {
	inspectors    map[string]registeredInspector
	misconfigured bool
}

// Result is one validated observation and its canonical digest.
type Result struct {
	Observation processorprocess.Observation
	Digest      string
}

// New creates a finite registry. Empty, duplicate, or nil registrations
// invalidate the whole service so partial composition cannot be used.
func New(registrations ...Registration) *Service {
	service := &Service{inspectors: make(map[string]registeredInspector, len(registrations))}
	for _, registration := range registrations {
		if registration.Selector == "" || processorprocess.ValidateAdapterKind(registration.AdapterKind) != nil || portcheck.IsNil(registration.Inspector) {
			service.misconfigured = true
			continue
		}
		if _, exists := service.inspectors[registration.Selector]; exists {
			service.misconfigured = true
			continue
		}
		service.inspectors[registration.Selector] = registeredInspector{adapterKind: registration.AdapterKind, inspector: registration.Inspector}
	}
	return service
}

// Inspect performs exactly the selected adapter's bounded execute operation.
func (s *Service) Inspect(ctx context.Context, intent operation.Intent, selector, executable string) (Result, error) {
	if ctx == nil {
		return Result{}, fmt.Errorf("processor inspection context is nil")
	}
	if err := ctx.Err(); err != nil {
		return Result{}, err
	}
	if err := intent.Validate(); err != nil || intent.Command != Command || intent.Effect != operation.EffectExecute {
		return Result{}, fmt.Errorf("processor inspection requires the processor inspect execute intent")
	}
	if s == nil || s.misconfigured {
		return Result{}, fault.New(fault.KindContract, "processor_adapter_contract", "The processor adapter registry is not configured completely.", false, helpAction())
	}
	registration, exists := s.inspectors[selector]
	if !exists || portcheck.IsNil(registration.inspector) {
		return Result{}, fault.New(fault.KindInvalidInput, "unsupported_processor_adapter", "The selected processor adapter is not supported.", false, helpAction())
	}
	if err := processorprocess.ValidateExecutablePath(executable); err != nil {
		return Result{}, fault.Wrap(fault.KindInvalidInput, "invalid_processor_executable", "The processor executable path must be absolute and clean.", false, err, helpAction())
	}

	observation, err := registration.inspector.Inspect(ctx, executable)
	if err != nil {
		return Result{}, classify(err)
	}
	if contextErr := ctx.Err(); contextErr != nil {
		return Result{}, fault.Wrap(fault.KindCanceled, "processor_execution_canceled", "The caller canceled after processor inspection started; the outcome is not replay-safe.", false, contextErr, helpAction())
	}
	if err := observation.Validate(); err != nil {
		return Result{}, fault.Wrap(fault.KindContract, "invalid_processor_observation", "The processor adapter returned an invalid observation.", false, err, helpAction())
	}
	if observation.Adapter.Kind != registration.adapterKind || observation.Identity.ResolvedPath != executable {
		return Result{}, fault.New(fault.KindContract, "invalid_processor_observation", "The processor adapter returned evidence for a different adapter or executable.", false, helpAction())
	}
	digest, err := observation.Digest()
	if err != nil {
		return Result{}, fault.Wrap(fault.KindContract, "invalid_processor_observation", "The processor observation could not be encoded canonically.", false, err, helpAction())
	}
	return Result{Observation: observation, Digest: digest}, nil
}

func classify(err error) error {
	if public, ok := fault.PublicCopy(err); ok {
		return fault.New(public.Kind, public.Code, public.Message, public.Retryable, helpAction())
	}
	switch {
	case errors.Is(err, context.Canceled), errors.Is(err, context.DeadlineExceeded):
		return fault.Wrap(fault.KindCanceled, "processor_execution_canceled", "The caller canceled after processor inspection started; the outcome is not replay-safe.", false, err, helpAction())
	case errors.Is(err, processorprocess.ErrUnsupportedPlatform):
		return fault.Wrap(fault.KindUnsupported, "unsupported_processor_platform", "The processor adapter does not support this native platform.", false, err, helpAction())
	case errors.Is(err, processorprocess.ErrUnsupportedVersion):
		return fault.Wrap(fault.KindInvalidInput, "unsupported_processor_version", "The installed processor version is not supported by this adapter.", false, err, helpAction())
	case errors.Is(err, processorprocess.ErrUnsupportedArtifact):
		return fault.Wrap(fault.KindRejected, "unsupported_processor_artifact", "The processor executable does not match a maintained official artifact.", false, err, helpAction())
	case errors.Is(err, processorprocess.ErrInvalidObservation), errors.Is(err, processorprocess.ErrInvalidIdentity),
		errors.Is(err, processorprocess.ErrInvalidRequest), errors.Is(err, processorprocess.ErrInvalidResult):
		return fault.Wrap(fault.KindContract, "invalid_processor_observation", "The processor adapter returned an invalid observation.", false, err, helpAction())
	case errors.Is(err, processorprocess.ErrInspectionFailed):
		return fault.Wrap(fault.KindRejected, "processor_inspection_failed", "The processor could not produce valid bounded inspection evidence.", false, err, helpAction())
	default:
		return fault.Wrap(fault.KindInternal, "internal_error", "The processor inspection could not be completed.", false, err, helpAction())
	}
}

func helpAction() fault.NextAction {
	return fault.NextAction{Command: "help processor inspect", Reason: "Review the processor adapter, exact executable, and compatibility contract."}
}
