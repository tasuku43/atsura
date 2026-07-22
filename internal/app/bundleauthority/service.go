// Package bundleauthority validates one built bundle against user-local
// adoption and current source identity, and owns the exact-digest receipt
// mutation behind the public bundle trust command.
package bundleauthority

import (
	"context"
	"fmt"

	"github.com/tasuku43/atsura/internal/app/execution"
	"github.com/tasuku43/atsura/internal/app/portcheck"
	"github.com/tasuku43/atsura/internal/domain/bundletrust"
	"github.com/tasuku43/atsura/internal/domain/fault"
	"github.com/tasuku43/atsura/internal/domain/operation"
	"github.com/tasuku43/atsura/internal/domain/processorprocess"
	"github.com/tasuku43/atsura/internal/domain/sourceprocess"
	"github.com/tasuku43/atsura/internal/domain/tailoringbundle"
)

const (
	TrustCommand    = "bundle trust"
	StatusCommand   = "bundle status"
	TrustTargetKind = "bundle-adoption-store"
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

type ProcessorIdentityPort interface {
	Identify(context.Context, string) (processorprocess.Identity, error)
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
	processors   ProcessorIdentityPort
	invalid      bool
}

type ProcessorStatus struct {
	Contract     string
	AdapterKind  string
	Version      string
	ResolvedPath string
	SHA256       string
	Size         int64
	State        bundletrust.ProcessorState
}

type StatusResult struct {
	BundleDigest             string
	CatalogDigest            string
	SpecificationDigest      string
	Adoption                 bundletrust.State
	Source                   bundletrust.SourceState
	Adopted                  bool
	SourcePath               string
	SourceSHA256             string
	SourceVersion            string
	Processors               []ProcessorStatus
	SourceProcessAttempts    int
	ProcessorProcessAttempts int
}

type TrustResult struct {
	BundleDigest             string
	Adopted                  bool
	AlreadyAdopted           bool
	Source                   bundletrust.SourceState
	Processors               []ProcessorStatus
	SourceProcessAttempts    int
	ProcessorProcessAttempts int
}

func New(bundles BundlePort, identity IdentityPort, trust TrustPort, confirmation ConfirmationPort, processors ...ProcessorIdentityPort) *Service {
	service := &Service{bundles: bundles, identity: identity, trust: trust, confirmation: confirmation}
	if len(processors) > 1 {
		service.invalid = true
	} else if len(processors) == 1 {
		service.processors = processors[0]
	}
	return service
}

func (s *Service) Status(ctx context.Context, intent operation.Intent, path string) (StatusResult, error) {
	if err := s.preflight(ctx, intent, StatusCommand, operation.EffectRead); err != nil {
		return StatusResult{}, err
	}
	bundle, digest, err := s.bundles.Load(ctx, path)
	if err != nil {
		return StatusResult{}, preserve(err)
	}
	result := StatusResult{BundleDigest: digest, CatalogDigest: bundle.CatalogDigest, SpecificationDigest: bundle.SpecificationDigest,
		Adoption: s.trust.Inspect(ctx, digest), SourcePath: bundle.Catalog.Source.ResolvedPath,
		SourceSHA256: bundle.Catalog.Source.SHA256, SourceVersion: bundle.Catalog.Source.Version,
		SourceProcessAttempts: 0, ProcessorProcessAttempts: 0,
	}
	result.Source = s.sourceState(ctx, bundle)
	result.Processors = s.processorStates(ctx, bundle)
	result.Adopted = result.Adoption == bundletrust.StateAdopted
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
		return TrustResult{}, fault.New(fault.KindRejected, "bundle_source_drift", "The bundle source identity is not current, so adoption was not recorded.", false, statusAction())
	}
	processorStates := s.processorStates(ctx, bundle)
	for _, processor := range processorStates {
		if processor.State != bundletrust.ProcessorCurrent {
			return TrustResult{}, fault.New(fault.KindRejected, "bundle_processor_drift", "A bundle processor identity is not current, so adoption was not recorded.", false, statusAction())
		}
	}
	if state == bundletrust.StateAdopted {
		return TrustResult{BundleDigest: digest, Adopted: true, AlreadyAdopted: true, Source: sourceState, Processors: processorStates}, nil
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
	return TrustResult{BundleDigest: digest, Adopted: true, Source: sourceState, Processors: processorStates}, nil
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
	if s == nil || s.invalid || portcheck.IsNil(s.bundles) || portcheck.IsNil(s.identity) || portcheck.IsNil(s.trust) {
		return fmt.Errorf("bundle authority adapters are not configured")
	}
	if effect == operation.EffectWrite && portcheck.IsNil(s.confirmation) {
		return fmt.Errorf("bundle adoption confirmation is not configured")
	}
	return nil
}

func (s *Service) processorStates(ctx context.Context, bundle tailoringbundle.Bundle) []ProcessorStatus {
	result := make([]ProcessorStatus, len(bundle.Processors))
	for index, binding := range bundle.Processors {
		observation := binding.Observation
		status := ProcessorStatus{
			Contract: binding.Contract, AdapterKind: observation.Adapter.Kind, Version: observation.Version,
			ResolvedPath: observation.Identity.ResolvedPath, SHA256: observation.Identity.SHA256, Size: observation.Identity.Size,
			State: bundletrust.ProcessorUnavailable,
		}
		if !portcheck.IsNil(s.processors) {
			identity, err := s.processors.Identify(ctx, observation.Identity.ResolvedPath)
			if err == nil {
				if identity == observation.Identity {
					status.State = bundletrust.ProcessorCurrent
				} else {
					status.State = bundletrust.ProcessorDrifted
				}
			}
		}
		result[index] = status
	}
	return result
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
	result := bundletrust.Summary{BundleDigest: digest, CatalogDigest: bundle.CatalogDigest, SpecificationDigest: bundle.SpecificationDigest,
		SourcePath: bundle.Catalog.Source.ResolvedPath, SourceSHA256: bundle.Catalog.Source.SHA256,
		SourceVersion: bundle.Catalog.Source.Version, SurfaceDefault: string(bundle.Specification.Surface.Default),
		IncludedCommandCount: len(bundle.Surface), Processors: make([]bundletrust.ProcessorSummary, len(bundle.Processors))}
	for index, binding := range bundle.Processors {
		observation := binding.Observation
		result.Processors[index] = bundletrust.ProcessorSummary{
			Contract: binding.Contract, AdapterKind: observation.Adapter.Kind, Version: observation.Version,
			ResolvedPath: observation.Identity.ResolvedPath, SHA256: observation.Identity.SHA256, Size: observation.Identity.Size,
			InputFormat: binding.InputFormat, OutputFormat: binding.OutputFormat,
		}
	}
	for _, entry := range bundle.Specification.Commands {
		if entry.Presence == tailoringbundle.PresenceExclude {
			result.ExcludedCommandCount++
		}
	}
	for _, entry := range bundle.Surface {
		result.OptionOverrideCount += len(entry.Options.Include) + len(entry.Options.Exclude)
		result.OptionDefaultCount += len(entry.Wrapper.Invoke.OptionDefaults)
		result.BeforeActionCount += len(entry.Wrapper.Before)
		result.AfterActionCount += len(entry.Wrapper.After)
		switch entry.Wrapper.Kind {
		case tailoringbundle.WrapperIdentity:
			result.IdentityWrapperCount++
		case tailoringbundle.WrapperTransform:
			result.TransformWrapperCount++
		}
		if len(entry.Wrapper.Invoke.OptionDefaults) > 0 || len(entry.Wrapper.Invoke.AppendArgs) > 0 {
			result.ArgvTransformationCount++
		}
		if entry.Wrapper.Output != nil {
			result.OutputTransformationCount++
			if entry.Wrapper.Output.Kind == tailoringbundle.OutputKindOptimizer {
				result.OptimizerResultCount++
			}
		} else {
			result.SourceStreamResultCount++
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
	return fault.NextAction{Command: StatusCommand, Reason: "Reconcile bundle adoption and source drift without executing it."}
}
