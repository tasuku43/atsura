// Package policyinit creates one fail-closed schema-2 policy draft from exact
// verified catalog evidence.
package policyinit

import (
	"context"
	"fmt"
	"strings"

	"github.com/tasuku43/atsura/internal/app/portcheck"
	"github.com/tasuku43/atsura/internal/domain/fault"
	"github.com/tasuku43/atsura/internal/domain/operation"
	"github.com/tasuku43/atsura/internal/domain/sourcecatalog"
	"github.com/tasuku43/atsura/internal/domain/tailoringbundle"
)

type CatalogPort interface {
	Load(context.Context, string) (sourcecatalog.Catalog, error)
}

type Service struct {
	catalogs CatalogPort
}

func New(catalogs CatalogPort) *Service { return &Service{catalogs: catalogs} }

func (s *Service) Init(ctx context.Context, intent operation.Intent, catalogPath string, effect operation.Effect, command []string) (tailoringbundle.Policy, error) {
	if ctx == nil {
		return tailoringbundle.Policy{}, fmt.Errorf("policy init context is nil")
	}
	if err := ctx.Err(); err != nil {
		return tailoringbundle.Policy{}, err
	}
	if err := intent.Validate(); err != nil || intent.Command != "policy init" || intent.Effect != operation.EffectRead {
		return tailoringbundle.Policy{}, fmt.Errorf("policy init requires the policy init read intent")
	}
	if s == nil || portcheck.IsNil(s.catalogs) {
		return tailoringbundle.Policy{}, fmt.Errorf("policy init catalog adapter is not configured")
	}
	if err := effect.Validate(); err != nil {
		return tailoringbundle.Policy{}, fault.Wrap(fault.KindInvalidInput, "invalid_policy_effect", "The draft effect must be read, create, or write.", false, err, helpAction())
	}
	catalog, err := s.catalogs.Load(ctx, catalogPath)
	if err != nil {
		if public, ok := fault.PublicCopy(err); ok {
			return tailoringbundle.Policy{}, public
		}
		return tailoringbundle.Policy{}, fault.Wrap(fault.KindInternal, "internal_error", "The policy draft could not load its catalog.", false, err, helpAction())
	}
	key := strings.Join(command, " ")
	verified := false
	for _, candidate := range catalog.Commands {
		if strings.Join(candidate.Path, " ") == key {
			if candidate.Provenance != sourcecatalog.ProvenanceVerifiedBuiltin {
				return tailoringbundle.Policy{}, fault.New(fault.KindRejected, "unverified_catalog_command", "The selected catalog command is not verified built-in evidence.", false, helpAction())
			}
			verified = true
			break
		}
	}
	if !verified {
		return tailoringbundle.Policy{}, fault.New(fault.KindNotFound, "catalog_command_not_found", "The exact command path is absent from the selected catalog.", false, helpAction())
	}
	digest, err := catalog.Digest()
	if err != nil {
		return tailoringbundle.Policy{}, fault.Wrap(fault.KindContract, "invalid_source_catalog", "The selected catalog is invalid.", false, err, helpAction())
	}
	policy := tailoringbundle.Policy{SchemaVersion: tailoringbundle.PolicySchemaVersion, CatalogDigest: digest, Rules: []tailoringbundle.Rule{{
		Command: append(make([]string, 0, len(command)), command...), Visibility: tailoringbundle.VisibilityHidden,
		Effect: effect, Decision: tailoringbundle.DecisionDeny, Reason: "Review and tailor this command before enabling it.", AppendArgs: []string{},
	}}}
	if err := policy.Validate(catalog); err != nil {
		return tailoringbundle.Policy{}, fault.Wrap(fault.KindContract, "invalid_policy_draft", "The fail-closed policy draft is invalid.", false, err, helpAction())
	}
	return policy, nil
}

func helpAction() fault.NextAction {
	return fault.NextAction{Command: "help policy init", Reason: "Select one exact verified command and declare its effect."}
}
