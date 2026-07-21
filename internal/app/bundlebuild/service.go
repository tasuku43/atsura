// Package bundlebuild validates schema-3 tailoring specifications and compiles
// canonical schema-2 bundles.
package bundlebuild

import (
	"context"
	"errors"
	"fmt"

	"github.com/tasuku43/atsura/internal/app/portcheck"
	"github.com/tasuku43/atsura/internal/domain/fault"
	"github.com/tasuku43/atsura/internal/domain/operation"
	"github.com/tasuku43/atsura/internal/domain/sourcecatalog"
	"github.com/tasuku43/atsura/internal/domain/tailoringbundle"
)

type CatalogPort interface {
	Load(context.Context, string) (sourcecatalog.Catalog, error)
}

type SpecificationPort interface {
	Load(context.Context, string) (tailoringbundle.Specification, error)
}

type Service struct {
	catalogs       CatalogPort
	specifications SpecificationPort
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

func New(catalogs CatalogPort, specifications SpecificationPort) *Service {
	return &Service{catalogs: catalogs, specifications: specifications}
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

func (s *Service) Build(ctx context.Context, intent operation.Intent, catalogPath, specificationPath string) (BundleResult, error) {
	if err := s.preflight(ctx, intent, "bundle build"); err != nil {
		return BundleResult{}, err
	}
	catalog, specification, err := s.load(ctx, catalogPath, specificationPath)
	if err != nil {
		return BundleResult{}, err
	}
	bundle, err := tailoringbundle.Compile(catalog, specification)
	if err != nil {
		if errors.Is(err, tailoringbundle.ErrInvalidSpecification) {
			return BundleResult{}, invalidSpecification(err)
		}
		return BundleResult{}, fault.Wrap(fault.KindContract, "invalid_bundle", "The canonical bundle could not be compiled.", false, err, bundleHelp())
	}
	digest, err := bundle.Digest()
	if err != nil {
		return BundleResult{}, fault.Wrap(fault.KindContract, "invalid_bundle", "The canonical bundle could not be encoded.", false, err, bundleHelp())
	}
	return BundleResult{Bundle: bundle, BundleDigest: digest}, nil
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
	return fault.Wrap(fault.KindInvalidInput, "invalid_specification", "The schema-3 tailoring specification is not valid for the selected catalog.", false, err, specificationHelp())
}

func specificationHelp() fault.NextAction {
	return fault.NextAction{Command: "help spec validate", Reason: "Correct the catalog-bound schema-3 tailoring specification."}
}

func bundleHelp() fault.NextAction {
	return fault.NextAction{Command: "help bundle build", Reason: "Review the catalog, specification, and canonical bundle contract."}
}
