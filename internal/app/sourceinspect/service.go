// Package sourceinspect coordinates one bounded source-adapter inspection.
package sourceinspect

import (
	"context"
	"errors"
	"fmt"

	"github.com/tasuku43/atsura/internal/app/portcheck"
	"github.com/tasuku43/atsura/internal/domain/fault"
	"github.com/tasuku43/atsura/internal/domain/operation"
	"github.com/tasuku43/atsura/internal/domain/sourcecatalog"
)

// InspectorPort observes one installed source under an adapter-owned finite
// probe contract.
type InspectorPort interface {
	Inspect(context.Context, string) (sourcecatalog.Catalog, error)
}

// Result is the validated catalog and its canonical digest.
type Result struct {
	Catalog sourcecatalog.Catalog
	Digest  string
}

// Service selects a registered source adapter without embedding vendor
// semantics in the application layer.
type Service struct {
	inspectors map[string]InspectorPort
}

// New creates a service from an explicit adapter registry.
func New(inspectors map[string]InspectorPort) *Service {
	copy := make(map[string]InspectorPort, len(inspectors))
	for kind, inspector := range inspectors {
		copy[kind] = inspector
	}
	return &Service{inspectors: copy}
}

// Inspect executes only the selected adapter's bounded evidence probes.
func (s *Service) Inspect(ctx context.Context, intent operation.Intent, adapter, executable string) (Result, error) {
	if ctx == nil {
		return Result{}, fmt.Errorf("source inspection context is nil")
	}
	if err := ctx.Err(); err != nil {
		return Result{}, err
	}
	if err := intent.Validate(); err != nil || intent.Command != "source inspect" || intent.Effect != operation.EffectExecute {
		return Result{}, fmt.Errorf("source inspection requires the source inspect execute intent")
	}
	if s == nil {
		return Result{}, fmt.Errorf("source inspection service is not configured")
	}
	inspector, exists := s.inspectors[adapter]
	if !exists || portcheck.IsNil(inspector) {
		return Result{}, fault.Wrap(fault.KindInvalidInput, "unsupported_source_adapter", "The selected source adapter is not supported.", false, sourcecatalog.ErrUnsupportedAdapter, helpAction())
	}
	catalog, err := inspector.Inspect(ctx, executable)
	if err != nil {
		return Result{}, classify(err)
	}
	if contextErr := ctx.Err(); contextErr != nil {
		return Result{}, fault.Wrap(fault.KindCanceled, "source_execution_canceled", "The caller canceled after source inspection started; its downstream outcome is not classified as replay-safe.", false, contextErr, helpAction())
	}
	if err := catalog.Validate(); err != nil {
		return Result{}, fault.Wrap(fault.KindContract, "invalid_source_catalog", "The source adapter returned an invalid catalog.", false, err, helpAction())
	}
	if catalog.Source.RequestedExecutable != executable {
		return Result{}, fault.New(fault.KindContract, "invalid_source_catalog", "The source adapter returned catalog evidence for a different executable.", false, helpAction())
	}
	digest, err := catalog.Digest()
	if err != nil {
		return Result{}, fault.Wrap(fault.KindContract, "invalid_source_catalog", "The source catalog could not be encoded canonically.", false, err, helpAction())
	}
	return Result{Catalog: catalog, Digest: digest}, nil
}

func classify(err error) error {
	switch {
	case errors.Is(err, sourcecatalog.ErrUnsupportedVersion):
		return fault.Wrap(fault.KindInvalidInput, "unsupported_source_version", "The installed source version is not supported by this adapter.", false, err, helpAction())
	case errors.Is(err, sourcecatalog.ErrInvalidCatalog):
		return fault.Wrap(fault.KindContract, "invalid_source_catalog", "The source adapter returned an invalid catalog.", false, err, helpAction())
	case errors.Is(err, sourcecatalog.ErrInspectionFailed):
		return fault.Wrap(fault.KindRejected, "source_inspection_failed", "The source CLI could not produce valid bounded inspection evidence.", false, err, helpAction())
	default:
		if public, ok := fault.PublicCopy(err); ok {
			return fault.New(public.Kind, public.Code, public.Message, public.Retryable, helpAction())
		}
		return fault.Wrap(fault.KindInternal, "internal_error", "The source inspection could not be completed.", false, err, helpAction())
	}
}

func helpAction() fault.NextAction {
	return fault.NextAction{Command: "help source inspect", Reason: "Review the source adapter, executable, and compatibility contract."}
}
