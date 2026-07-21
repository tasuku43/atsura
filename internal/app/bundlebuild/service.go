// Package bundlebuild validates schema-2 policy and compiles canonical bundles.
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

type PolicyPort interface {
	Load(context.Context, string) (tailoringbundle.Policy, error)
}

type Service struct {
	catalogs CatalogPort
	policies PolicyPort
}

type PolicyResult struct {
	Policy       tailoringbundle.Policy
	PolicyDigest string
	RuleCount    int
	VisibleCount int
}

type BundleResult struct {
	Bundle       tailoringbundle.Bundle
	BundleDigest string
}

func New(catalogs CatalogPort, policies PolicyPort) *Service {
	return &Service{catalogs: catalogs, policies: policies}
}

func (s *Service) ValidatePolicy(ctx context.Context, intent operation.Intent, catalogPath, policyPath string) (PolicyResult, error) {
	if err := s.preflight(ctx, intent, "policy validate"); err != nil {
		return PolicyResult{}, err
	}
	catalog, policy, err := s.load(ctx, catalogPath, policyPath)
	if err != nil {
		return PolicyResult{}, err
	}
	if err := policy.Validate(catalog); err != nil {
		return PolicyResult{}, invalidPolicy(err)
	}
	digest, err := policy.Digest(catalog)
	if err != nil {
		return PolicyResult{}, invalidPolicy(err)
	}
	visible := 0
	for _, rule := range policy.Rules {
		if rule.Visibility == tailoringbundle.VisibilityVisible {
			visible++
		}
	}
	return PolicyResult{Policy: policy, PolicyDigest: digest, RuleCount: len(policy.Rules), VisibleCount: visible}, nil
}

func (s *Service) Build(ctx context.Context, intent operation.Intent, catalogPath, policyPath string) (BundleResult, error) {
	if err := s.preflight(ctx, intent, "bundle build"); err != nil {
		return BundleResult{}, err
	}
	catalog, policy, err := s.load(ctx, catalogPath, policyPath)
	if err != nil {
		return BundleResult{}, err
	}
	bundle, err := tailoringbundle.Compile(catalog, policy)
	if err != nil {
		if errors.Is(err, tailoringbundle.ErrInvalidPolicy) {
			return BundleResult{}, invalidPolicy(err)
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
	if s == nil || portcheck.IsNil(s.catalogs) || portcheck.IsNil(s.policies) {
		return fmt.Errorf("bundle workflow adapters are not configured")
	}
	return nil
}

func (s *Service) load(ctx context.Context, catalogPath, policyPath string) (sourcecatalog.Catalog, tailoringbundle.Policy, error) {
	catalog, err := s.catalogs.Load(ctx, catalogPath)
	if err != nil {
		return sourcecatalog.Catalog{}, tailoringbundle.Policy{}, preserve(err)
	}
	policy, err := s.policies.Load(ctx, policyPath)
	if err != nil {
		return sourcecatalog.Catalog{}, tailoringbundle.Policy{}, preserve(err)
	}
	if err := ctx.Err(); err != nil {
		return sourcecatalog.Catalog{}, tailoringbundle.Policy{}, err
	}
	return catalog, policy, nil
}

func preserve(err error) error {
	if public, ok := fault.PublicCopy(err); ok {
		return public
	}
	return fault.Wrap(fault.KindInternal, "internal_error", "The bundle workflow could not load its inputs.", false, err, bundleHelp())
}

func invalidPolicy(err error) error {
	return fault.Wrap(fault.KindInvalidInput, "invalid_policy", "The schema-2 policy is not valid for the selected catalog.", false, err, policyHelp())
}

func policyHelp() fault.NextAction {
	return fault.NextAction{Command: "help policy validate", Reason: "Correct the catalog-bound schema-2 policy."}
}

func bundleHelp() fault.NextAction {
	return fault.NextAction{Command: "help bundle build", Reason: "Review the catalog, policy, and canonical bundle contract."}
}
