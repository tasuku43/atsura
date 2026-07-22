// Package specinit creates one schema-4 tailoring specification draft from
// exact verified catalog evidence.
package specinit

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

// Init creates an exclude-by-default surface containing one verified command
// with inherited options and an identity wrapper.
func (s *Service) Init(ctx context.Context, intent operation.Intent, catalogPath string, command []string) (tailoringbundle.Specification, error) {
	if ctx == nil {
		return tailoringbundle.Specification{}, fmt.Errorf("spec init context is nil")
	}
	if err := ctx.Err(); err != nil {
		return tailoringbundle.Specification{}, err
	}
	if err := intent.Validate(); err != nil || intent.Command != "spec init" || intent.Effect != operation.EffectRead {
		return tailoringbundle.Specification{}, fmt.Errorf("spec init requires the spec init read intent")
	}
	if s == nil || portcheck.IsNil(s.catalogs) {
		return tailoringbundle.Specification{}, fmt.Errorf("spec init catalog adapter is not configured")
	}
	catalog, err := s.catalogs.Load(ctx, catalogPath)
	if err != nil {
		if public, ok := fault.PublicCopy(err); ok {
			return tailoringbundle.Specification{}, public
		}
		return tailoringbundle.Specification{}, fault.Wrap(fault.KindInternal, "internal_error", "The specification draft could not load its catalog.", false, err, helpAction())
	}
	key := strings.Join(command, " ")
	verified := false
	for _, candidate := range catalog.Commands {
		if strings.Join(candidate.Path, " ") == key {
			if candidate.Provenance != sourcecatalog.ProvenanceVerifiedBuiltin {
				return tailoringbundle.Specification{}, fault.New(fault.KindRejected, "unverified_catalog_command", "The selected catalog command is not verified built-in evidence.", false, helpAction())
			}
			verified = true
			break
		}
	}
	if !verified {
		return tailoringbundle.Specification{}, fault.New(fault.KindNotFound, "catalog_command_not_found", "The exact command path is absent from the selected catalog.", false, helpAction())
	}
	digest, err := catalog.Digest()
	if err != nil {
		return tailoringbundle.Specification{}, fault.Wrap(fault.KindContract, "invalid_source_catalog", "The selected catalog is invalid.", false, err, helpAction())
	}
	specification := tailoringbundle.Specification{
		SchemaVersion: tailoringbundle.SpecificationSchemaVersion,
		CatalogDigest: digest,
		Surface:       tailoringbundle.Surface{Default: tailoringbundle.SurfaceDefaultExclude},
		Commands: []tailoringbundle.CommandEntry{{
			Command:  append(make([]string, 0, len(command)), command...),
			Presence: tailoringbundle.PresenceInclude,
			Reason:   "Include this verified command without transformation.",
			Options:  &tailoringbundle.OptionSurface{Default: tailoringbundle.SurfaceDefaultInherit, Include: []string{}, Exclude: []string{}},
			Wrapper: &tailoringbundle.Wrapper{
				Kind: tailoringbundle.WrapperIdentity, Before: []tailoringbundle.StageAction{},
				Invoke: tailoringbundle.Invocation{AppendArgs: []string{}}, After: []tailoringbundle.StageAction{},
			},
		}},
	}
	if err := specification.Validate(catalog); err != nil {
		return tailoringbundle.Specification{}, fault.Wrap(fault.KindContract, "invalid_specification_draft", "The schema-4 tailoring specification draft is invalid.", false, err, helpAction())
	}
	return specification, nil
}

func helpAction() fault.NextAction {
	return fault.NextAction{Command: "help spec init", Reason: "Select one exact verified command for an identity wrapper."}
}
