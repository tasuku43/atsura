// Package bundleauthority validates one built bundle against user-local trust
// and current source identity, and owns the exact-digest trust mutation.
package bundleauthority

import (
	"context"
	"fmt"

	"github.com/tasuku43/atsura/internal/app/execution"
	"github.com/tasuku43/atsura/internal/app/portcheck"
	"github.com/tasuku43/atsura/internal/domain/bundletrust"
	"github.com/tasuku43/atsura/internal/domain/fault"
	"github.com/tasuku43/atsura/internal/domain/operation"
	"github.com/tasuku43/atsura/internal/domain/sourceprocess"
	"github.com/tasuku43/atsura/internal/domain/tailoringbundle"
)

const (
	TrustCommand    = "bundle trust"
	StatusCommand   = "bundle status"
	TrustTargetKind = "bundle-trust-store"
	TrustTargetID   = "selected"
)

var TrustImpact = operation.Impact{
	Cardinality: operation.CardinalityOne, Notification: operation.DeclarationNo,
	AccessChange: operation.DeclarationYes, Destructive: operation.DeclarationNo,
}

type BundlePort interface {
	Load(context.Context, string) (tailoringbundle.Bundle, string, error)
}

type IdentityPort interface {
	Identify(context.Context, string) (sourceprocess.Identity, error)
}

type TrustPort interface {
	Inspect(context.Context, string) bundletrust.State
	Add(context.Context, string) (bool, error)
}

type ConfirmationPort interface {
	Confirm(context.Context, bundletrust.Summary) error
}

type Service struct {
	bundles      BundlePort
	identity     IdentityPort
	trust        TrustPort
	confirmation ConfirmationPort
}

type StatusResult struct {
	BundleDigest          string
	CatalogDigest         string
	PolicyDigest          string
	Trust                 bundletrust.State
	Source                bundletrust.SourceState
	Executable            bool
	SourcePath            string
	SourceSHA256          string
	SourceVersion         string
	SourceProcessAttempts int
}

type TrustResult struct {
	BundleDigest          string
	Trusted               bool
	AlreadyTrusted        bool
	Source                bundletrust.SourceState
	SourceProcessAttempts int
}

func New(bundles BundlePort, identity IdentityPort, trust TrustPort, confirmation ConfirmationPort) *Service {
	return &Service{bundles: bundles, identity: identity, trust: trust, confirmation: confirmation}
}

func (s *Service) Status(ctx context.Context, intent operation.Intent, path string) (StatusResult, error) {
	if err := s.preflight(ctx, intent, StatusCommand, operation.EffectRead); err != nil {
		return StatusResult{}, err
	}
	bundle, digest, err := s.bundles.Load(ctx, path)
	if err != nil {
		return StatusResult{}, preserve(err)
	}
	result := StatusResult{BundleDigest: digest, CatalogDigest: bundle.CatalogDigest, PolicyDigest: bundle.PolicyDigest,
		Trust: s.trust.Inspect(ctx, digest), SourcePath: bundle.Catalog.Source.ResolvedPath,
		SourceSHA256: bundle.Catalog.Source.SHA256, SourceVersion: bundle.Catalog.Source.Version,
		SourceProcessAttempts: 0,
	}
	result.Source = s.sourceState(ctx, bundle)
	result.Executable = result.Trust == bundletrust.StateTrusted && result.Source == bundletrust.SourceCurrent
	return result, nil
}

func (s *Service) Trust(ctx context.Context, intent operation.Intent, path string) (TrustResult, error) {
	if err := s.preflight(ctx, intent, TrustCommand, operation.EffectWrite); err != nil {
		return TrustResult{}, err
	}
	bundle, digest, err := s.bundles.Load(ctx, path)
	if err != nil {
		return TrustResult{}, preserve(err)
	}
	state := s.trust.Inspect(ctx, digest)
	if state == bundletrust.StateInvalid {
		return TrustResult{}, fault.New(fault.KindRejected, "invalid_bundle_trust_store", "The user-local trust store is invalid and was not modified.", false, statusAction())
	}
	sourceState := s.sourceState(ctx, bundle)
	if sourceState != bundletrust.SourceCurrent {
		return TrustResult{}, fault.New(fault.KindRejected, "bundle_source_drift", "The bundle source identity is not current, so trust was not granted.", false, statusAction())
	}
	if state == bundletrust.StateTrusted {
		return TrustResult{BundleDigest: digest, Trusted: true, AlreadyTrusted: true, Source: sourceState}, nil
	}
	summary := summarize(bundle, digest)
	policy := confirmationPolicy{confirmation: s.confirmation, summary: summary, expected: intent}
	request := execution.Request{Intent: intent, ExpectedCommand: TrustCommand, ExpectedEffect: operation.EffectWrite,
		ExpectedTarget: operation.TargetRef{Kind: TrustTargetKind, ID: TrustTargetID}, ExpectedImpact: TrustImpact}
	err = execution.New(policy).Invoke(ctx, request, func(ctx context.Context, _ operation.Intent) error {
		_, err := s.trust.Add(ctx, digest)
		return err
	})
	if err != nil {
		return TrustResult{}, err
	}
	return TrustResult{BundleDigest: digest, Trusted: true, Source: sourceState}, nil
}

func (s *Service) preflight(ctx context.Context, intent operation.Intent, command string, effect operation.Effect) error {
	if ctx == nil {
		return fmt.Errorf("bundle authority context is nil")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := intent.Validate(); err != nil || intent.Command != command || intent.Effect != effect {
		return fmt.Errorf("bundle authority requires the exact %s intent", command)
	}
	if s == nil || portcheck.IsNil(s.bundles) || portcheck.IsNil(s.identity) || portcheck.IsNil(s.trust) {
		return fmt.Errorf("bundle authority adapters are not configured")
	}
	if effect != operation.EffectRead && portcheck.IsNil(s.confirmation) {
		return fmt.Errorf("bundle trust confirmation is not configured")
	}
	return nil
}

func (s *Service) sourceState(ctx context.Context, bundle tailoringbundle.Bundle) bundletrust.SourceState {
	identity, err := s.identity.Identify(ctx, bundle.Catalog.Source.ResolvedPath)
	if err != nil {
		return bundletrust.SourceUnavailable
	}
	if identity.ResolvedPath != bundle.Catalog.Source.ResolvedPath || identity.SHA256 != bundle.Catalog.Source.SHA256 || identity.Size != bundle.Catalog.Source.Size {
		return bundletrust.SourceDrifted
	}
	return bundletrust.SourceCurrent
}

func summarize(bundle tailoringbundle.Bundle, digest string) bundletrust.Summary {
	result := bundletrust.Summary{BundleDigest: digest, CatalogDigest: bundle.CatalogDigest, PolicyDigest: bundle.PolicyDigest,
		SourcePath: bundle.Catalog.Source.ResolvedPath, SourceSHA256: bundle.Catalog.Source.SHA256,
		SourceVersion: bundle.Catalog.Source.Version, VisibleCount: len(bundle.Surface)}
	for _, rule := range bundle.Policy.Rules {
		switch rule.Effect {
		case operation.EffectRead:
			result.ReadCount++
		case operation.EffectCreate:
			result.CreateCount++
		case operation.EffectWrite:
			result.WriteCount++
		}
		switch rule.Decision {
		case tailoringbundle.DecisionAllow:
			result.AllowCount++
		case tailoringbundle.DecisionConfirm:
			result.ConfirmCount++
		case tailoringbundle.DecisionDeny:
			result.DenyCount++
		}
	}
	return result
}

type confirmationPolicy struct {
	confirmation ConfirmationPort
	summary      bundletrust.Summary
	expected     operation.Intent
}

func (p confirmationPolicy) Check(ctx context.Context, intent operation.Intent) error {
	if intent != p.expected || portcheck.IsNil(p.confirmation) {
		return fmt.Errorf("confirmation intent mismatch")
	}
	return p.confirmation.Confirm(ctx, p.summary)
}

func preserve(err error) error {
	if public, ok := fault.PublicCopy(err); ok {
		return public
	}
	return fault.Wrap(fault.KindInternal, "internal_error", "The bundle authority could not load its inputs.", false, err, statusAction())
}
func statusAction() fault.NextAction {
	return fault.NextAction{Command: StatusCommand, Reason: "Reconcile bundle trust and source drift without executing it."}
}
