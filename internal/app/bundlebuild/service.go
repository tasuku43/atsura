// Package bundlebuild validates schema-5 tailoring specifications and compiles
// canonical schema-4 bundles.
package bundlebuild

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/tasuku43/atsura/internal/app/portcheck"
	"github.com/tasuku43/atsura/internal/domain/fault"
	"github.com/tasuku43/atsura/internal/domain/operation"
	"github.com/tasuku43/atsura/internal/domain/processorprocess"
	"github.com/tasuku43/atsura/internal/domain/sourcecatalog"
	"github.com/tasuku43/atsura/internal/domain/tailoringbundle"
)

type CatalogPort interface {
	Load(context.Context, string) (sourcecatalog.Catalog, error)
}

type SpecificationPort interface {
	Load(context.Context, string) (tailoringbundle.Specification, error)
}

// ProcessorObservationPort loads one explicitly selected observation. Bundle
// compilation never discovers or inspects a processor.
type ProcessorObservationPort interface {
	Load(context.Context, string) (processorprocess.Observation, error)
}

// ProcessorCompatibilityPort creates the registry-owned process binding and
// proves that the complete compiled surface is the exact admitted tuple.
type ProcessorCompatibilityPort interface {
	Binding(processorprocess.Observation) (tailoringbundle.ProcessorBinding, error)
	VerifySurface(tailoringbundle.Bundle) error
}

// ProcessorSupport is optional for bundles with no optimizer stage. Optimizer
// bundles require exactly one explicitly selected observation and both ports.
type ProcessorSupport struct {
	Observations  ProcessorObservationPort
	Compatibility ProcessorCompatibilityPort
}

type Service struct {
	catalogs          CatalogPort
	specifications    SpecificationPort
	processor         ProcessorSupport
	processorSupports int
}

type SpecificationResult struct {
	Specification         tailoringbundle.Specification
	SpecificationDigest   string
	CommandCount          int
	IncludedCount         int
	ExcludedCount         int
	IdentityWrapperCount  int
	TransformWrapperCount int
}

type BundleResult struct {
	Bundle       tailoringbundle.Bundle
	BundleDigest string
}

func New(catalogs CatalogPort, specifications SpecificationPort, processor ...ProcessorSupport) *Service {
	service := &Service{catalogs: catalogs, specifications: specifications, processorSupports: len(processor)}
	if len(processor) == 1 {
		service.processor = processor[0]
	}
	return service
}

func (s *Service) ValidateSpecification(ctx context.Context, intent operation.Intent, catalogPath, specificationPath string) (SpecificationResult, error) {
	if err := s.preflight(ctx, intent, "spec validate"); err != nil {
		return SpecificationResult{}, err
	}
	catalog, specification, err := s.load(ctx, catalogPath, specificationPath)
	if err != nil {
		return SpecificationResult{}, err
	}
	if err := specification.Validate(catalog); err != nil {
		return SpecificationResult{}, invalidSpecification(err)
	}
	digest, err := specification.Digest(catalog)
	if err != nil {
		return SpecificationResult{}, invalidSpecification(err)
	}
	result := SpecificationResult{
		Specification: specification, SpecificationDigest: digest,
		CommandCount: len(specification.Commands),
	}
	for _, entry := range specification.Commands {
		switch entry.Presence {
		case tailoringbundle.PresenceInclude:
			result.IncludedCount++
			if entry.Wrapper != nil && entry.Wrapper.Kind == tailoringbundle.WrapperIdentity {
				result.IdentityWrapperCount++
			} else if entry.Wrapper != nil && entry.Wrapper.Kind == tailoringbundle.WrapperTransform {
				result.TransformWrapperCount++
			}
		case tailoringbundle.PresenceExclude:
			result.ExcludedCount++
		}
	}
	return result, nil
}

func (s *Service) Build(ctx context.Context, intent operation.Intent, catalogPath, specificationPath string, processorObservationPath ...string) (BundleResult, error) {
	if err := s.preflight(ctx, intent, "bundle build"); err != nil {
		return BundleResult{}, err
	}
	observationPath, hasObservation, err := optionalObservationPath(processorObservationPath)
	if err != nil {
		return BundleResult{}, err
	}
	catalog, specification, err := s.load(ctx, catalogPath, specificationPath)
	if err != nil {
		return BundleResult{}, err
	}
	if err := specification.Validate(catalog); err != nil {
		return BundleResult{}, invalidSpecification(err)
	}
	hasOptimizer := specificationUsesOptimizer(specification)
	if hasOptimizer && !hasObservation {
		return BundleResult{}, fault.New(fault.KindInvalidInput, "processor_observation_required", "An optimizer specification requires one explicit processor observation JSON document.", false, processorHelp())
	}
	if !hasOptimizer && hasObservation {
		return BundleResult{}, fault.New(fault.KindInvalidInput, "processor_observation_not_used", "This specification has no optimizer stage and cannot use processor observation evidence.", false, bundleHelp())
	}

	processors := []tailoringbundle.ProcessorBinding{}
	if hasOptimizer {
		if err := s.requireProcessorSupport(); err != nil {
			return BundleResult{}, err
		}
		observation, err := s.processor.Observations.Load(ctx, observationPath)
		if err != nil {
			return BundleResult{}, preserveProcessorLoad(err)
		}
		binding, err := s.processor.Compatibility.Binding(observation)
		if err != nil {
			return BundleResult{}, incompatibleProcessor(err)
		}
		processors = append(processors, binding)
	}

	bundle, err := tailoringbundle.Compile(catalog, specification, processors...)
	if err != nil {
		if errors.Is(err, tailoringbundle.ErrInvalidSpecification) {
			return BundleResult{}, invalidSpecification(err)
		}
		if hasOptimizer {
			return BundleResult{}, incompatibleProcessor(err)
		}
		return BundleResult{}, fault.Wrap(fault.KindContract, "invalid_bundle", "The canonical bundle could not be compiled.", false, err, bundleHelp())
	}
	if hasOptimizer {
		if err := s.processor.Compatibility.VerifySurface(bundle); err != nil {
			return BundleResult{}, incompatibleProcessor(err)
		}
	}
	digest, err := bundle.Digest()
	if err != nil {
		return BundleResult{}, fault.Wrap(fault.KindContract, "invalid_bundle", "The canonical bundle could not be encoded.", false, err, bundleHelp())
	}
	return BundleResult{Bundle: bundle, BundleDigest: digest}, nil
}

func specificationUsesOptimizer(specification tailoringbundle.Specification) bool {
	for _, entry := range specification.Commands {
		if entry.Presence == tailoringbundle.PresenceInclude && entry.Wrapper != nil && entry.Wrapper.Output != nil && entry.Wrapper.Output.Kind == tailoringbundle.OutputKindOptimizer {
			return true
		}
	}
	return false
}

func optionalObservationPath(values []string) (string, bool, error) {
	if len(values) > 1 {
		return "", false, fault.New(fault.KindInvalidInput, "invalid_processor_observation_selection", "Select at most one processor observation JSON document.", false, bundleHelp())
	}
	if len(values) == 0 {
		return "", false, nil
	}
	if strings.TrimSpace(values[0]) == "" {
		return "", false, fault.New(fault.KindInvalidInput, "invalid_processor_observation_selection", "The processor observation path must not be empty.", false, bundleHelp())
	}
	return values[0], true, nil
}

func (s *Service) requireProcessorSupport() error {
	if s.processorSupports != 1 || portcheck.IsNil(s.processor.Observations) || portcheck.IsNil(s.processor.Compatibility) {
		return fmt.Errorf("bundle workflow processor evidence adapters are not configured")
	}
	return nil
}

func preserveProcessorLoad(err error) error {
	if public, ok := fault.PublicCopy(err); ok {
		return public
	}
	return fault.Wrap(fault.KindInternal, "internal_error", "The bundle workflow could not load its processor observation.", false, err, processorHelp())
}

func incompatibleProcessor(err error) error {
	return fault.Wrap(fault.KindRejected, "processor_compatibility_not_admitted", "The processor observation and optimizer specification do not match an admitted compatibility contract.", false, err, processorHelp())
}

func processorHelp() fault.NextAction {
	return fault.NextAction{Command: "processor inspect", Reason: "Generate fresh evidence for an explicitly selected supported processor executable."}
}

func (s *Service) preflight(ctx context.Context, intent operation.Intent, command string) error {
	if ctx == nil {
		return fmt.Errorf("bundle workflow context is nil")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := intent.Validate(); err != nil || intent.Command != command || intent.Effect != operation.EffectRead {
		return fmt.Errorf("bundle workflow requires the %s read intent", command)
	}
	if s == nil || portcheck.IsNil(s.catalogs) || portcheck.IsNil(s.specifications) {
		return fmt.Errorf("bundle workflow adapters are not configured")
	}
	return nil
}

func (s *Service) load(ctx context.Context, catalogPath, specificationPath string) (sourcecatalog.Catalog, tailoringbundle.Specification, error) {
	catalog, err := s.catalogs.Load(ctx, catalogPath)
	if err != nil {
		return sourcecatalog.Catalog{}, tailoringbundle.Specification{}, preserve(err)
	}
	specification, err := s.specifications.Load(ctx, specificationPath)
	if err != nil {
		return sourcecatalog.Catalog{}, tailoringbundle.Specification{}, preserve(err)
	}
	if err := ctx.Err(); err != nil {
		return sourcecatalog.Catalog{}, tailoringbundle.Specification{}, err
	}
	return catalog, specification, nil
}

func preserve(err error) error {
	if public, ok := fault.PublicCopy(err); ok {
		return public
	}
	return fault.Wrap(fault.KindInternal, "internal_error", "The bundle workflow could not load its inputs.", false, err, bundleHelp())
}

func invalidSpecification(err error) error {
	return fault.Wrap(fault.KindInvalidInput, "invalid_specification", "The schema-5 tailoring specification is not valid for the selected catalog.", false, err, specificationHelp())
}

func specificationHelp() fault.NextAction {
	return fault.NextAction{Command: "help spec validate", Reason: "Correct the catalog-bound schema-5 tailoring specification."}
}

func bundleHelp() fault.NextAction {
	return fault.NextAction{Command: "help bundle build", Reason: "Review the catalog, specification, and canonical bundle contract."}
}
