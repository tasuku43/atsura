// Package runtimecompat dispatches source runtime proof to one explicitly
// registered compatibility verifier. It owns no source grammar or execution
// capability.
package runtimecompat

import (
	"errors"

	"github.com/tasuku43/atsura/internal/app/portcheck"
	"github.com/tasuku43/atsura/internal/domain/runtimeadmission"
	"github.com/tasuku43/atsura/internal/domain/tailoringbundle"
	"github.com/tasuku43/atsura/internal/domain/tailoringplan"
)

var ErrAdapterContract = errors.New("runtime compatibility adapter contract is not admitted")

// Verifier proves both one fresh plan and one complete tailored surface for a
// single source adapter contract. Source-specific implementations remain in
// infrastructure and perform no registration themselves.
type Verifier interface {
	VerifyRuntime(tailoringplan.Plan) error
	VerifySurface(tailoringbundle.Bundle) error
}

// Registration binds one existing namespaced adapter kind to its verifier.
// The verifier remains responsible for contract-version and source-version
// admission after exact adapter-kind dispatch.
type Registration struct {
	AdapterKind string
	Verifier    Verifier
}

// Registry is the finite application-owned compatibility dispatcher. It is
// neither a source catalog nor a plugin or execution registry.
type Registry struct {
	verifiers     map[string]Verifier
	misconfigured bool
}

// New creates a registry from the complete explicit production composition.
// Empty kinds, nil verifiers, and duplicate kinds invalidate the whole
// registry so a partially configured process cannot admit a source contract.
func New(registrations ...Registration) *Registry {
	registry := &Registry{verifiers: make(map[string]Verifier, len(registrations))}
	for _, registration := range registrations {
		if registration.AdapterKind == "" || portcheck.IsNil(registration.Verifier) {
			registry.misconfigured = true
			continue
		}
		if _, exists := registry.verifiers[registration.AdapterKind]; exists {
			registry.misconfigured = true
			continue
		}
		registry.verifiers[registration.AdapterKind] = registration.Verifier
	}
	return registry
}

// VerifyRuntime delegates one plan unchanged to the verifier selected by its
// already-bound source adapter kind.
func (r *Registry) VerifyRuntime(plan tailoringplan.Plan) error {
	verifier, err := r.verifier(plan.Source.AdapterKind)
	if err != nil {
		return err
	}
	return admittedError(verifier.VerifyRuntime(plan))
}

// VerifySurface delegates one bundle unchanged to the verifier selected by
// its already-bound catalog adapter kind.
func (r *Registry) VerifySurface(bundle tailoringbundle.Bundle) error {
	verifier, err := r.verifier(bundle.Catalog.Adapter.Kind)
	if err != nil {
		return err
	}
	return admittedError(verifier.VerifySurface(bundle))
}

func (r *Registry) verifier(adapterKind string) (Verifier, error) {
	if r == nil || r.misconfigured || adapterKind == "" {
		return nil, adapterContractError()
	}
	verifier, exists := r.verifiers[adapterKind]
	if !exists || portcheck.IsNil(verifier) {
		return nil, adapterContractError()
	}
	return verifier, nil
}

type categorizedError interface {
	RuntimeAdmissionCategory() runtimeadmission.Category
}

// admittedError preserves an adapter's finite safe diagnostic while rejecting
// ordinary, typed-nil, or out-of-contract errors as adapter misconfiguration.
func admittedError(err error) error {
	if err == nil {
		return nil
	}
	if portcheck.IsNil(err) {
		return adapterContractError()
	}
	var categorized categorizedError
	if !errors.As(err, &categorized) || portcheck.IsNil(categorized) || !categorized.RuntimeAdmissionCategory().Valid() {
		return adapterContractError()
	}
	return err
}

type admissionError struct{}

func (*admissionError) Error() string { return ErrAdapterContract.Error() }

func (*admissionError) Unwrap() error { return ErrAdapterContract }

func (*admissionError) RuntimeAdmissionCategory() runtimeadmission.Category {
	return runtimeadmission.CategoryAdapterContract
}

func adapterContractError() error { return &admissionError{} }
