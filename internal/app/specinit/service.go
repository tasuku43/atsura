// Package specinit creates one schema-5 tailoring specification draft from
// exact verified catalog evidence.
package specinit

import (
	"context"
	"fmt"
	"reflect"
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

// ProcessorObservationPort loads one explicitly selected observation. It does
// not discover or inspect a processor.
type ProcessorObservationPort interface {
	Load(context.Context, string) (processorprocess.Observation, error)
}

// ProcessorCompatibilityPort materializes one registry-owned authoring
// default from exact source and processor evidence.
type ProcessorCompatibilityPort interface {
	DefaultEntry(sourcecatalog.Catalog, processorprocess.Observation) (tailoringbundle.CommandEntry, error)
}

// ProcessorSupport is optional because identity authoring does not require an
// external output processor. When evidence is requested, both ports are
// mandatory and no ambient discovery is attempted.
type ProcessorSupport struct {
	Observations  ProcessorObservationPort
	Compatibility ProcessorCompatibilityPort
}

type Service struct {
	catalogs          CatalogPort
	processor         ProcessorSupport
	processorSupports int
}

func New(catalogs CatalogPort, processor ...ProcessorSupport) *Service {
	service := &Service{catalogs: catalogs, processorSupports: len(processor)}
	if len(processor) == 1 {
		service.processor = processor[0]
	}
	return service
}

// Init creates an exclude-by-default surface containing one verified command
// with inherited options. Without processor evidence it uses an identity
// wrapper. With one explicit observation, the finite compatibility registry
// may replace that entry with its exact typed optimizer default.
func (s *Service) Init(ctx context.Context, intent operation.Intent, catalogPath string, command []string, processorObservationPath ...string) (tailoringbundle.Specification, error) {
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
	observationPath, hasObservation, err := optionalObservationPath(processorObservationPath)
	if err != nil {
		return tailoringbundle.Specification{}, err
	}
	catalog, err := s.catalogs.Load(ctx, catalogPath)
	if err != nil {
		if public, ok := fault.PublicCopy(err); ok {
			return tailoringbundle.Specification{}, public
		}
		return tailoringbundle.Specification{}, fault.Wrap(fault.KindInternal, "internal_error", "The specification draft could not load its catalog.", false, err, helpAction())
	}
	if err := requireVerifiedCommand(catalog, command); err != nil {
		return tailoringbundle.Specification{}, err
	}
	digest, err := catalog.Digest()
	if err != nil {
		return tailoringbundle.Specification{}, fault.Wrap(fault.KindContract, "invalid_source_catalog", "The selected catalog is invalid.", false, err, helpAction())
	}
	entry := identityEntry(command)
	if hasObservation {
		if err := s.requireProcessorSupport(); err != nil {
			return tailoringbundle.Specification{}, err
		}
		observation, err := s.processor.Observations.Load(ctx, observationPath)
		if err != nil {
			return tailoringbundle.Specification{}, preserveProcessorLoad(err)
		}
		entry, err = s.processor.Compatibility.DefaultEntry(catalog, observation)
		if err != nil {
			return tailoringbundle.Specification{}, incompatibleProcessorDefault(err)
		}
		if !validProcessorDefault(entry, command) {
			return tailoringbundle.Specification{}, fault.New(fault.KindContract, "invalid_processor_default", "The processor compatibility registry returned an invalid optimizer default for the selected command.", false, helpAction())
		}
	}
	specification := tailoringbundle.Specification{
		SchemaVersion: tailoringbundle.SpecificationSchemaVersion,
		CatalogDigest: digest,
		Surface:       tailoringbundle.Surface{Default: tailoringbundle.SurfaceDefaultExclude},
		Commands:      []tailoringbundle.CommandEntry{entry},
	}
	if err := specification.Validate(catalog); err != nil {
		return tailoringbundle.Specification{}, fault.Wrap(fault.KindContract, "invalid_specification_draft", "The schema-5 tailoring specification draft is invalid.", false, err, helpAction())
	}
	return specification, nil
}

func validProcessorDefault(entry tailoringbundle.CommandEntry, command []string) bool {
	return reflect.DeepEqual(entry.Command, command) && entry.Presence == tailoringbundle.PresenceInclude &&
		entry.Wrapper != nil && entry.Wrapper.Kind == tailoringbundle.WrapperTransform && entry.Wrapper.Output != nil &&
		entry.Wrapper.Output.Kind == tailoringbundle.OutputKindOptimizer && entry.Wrapper.Output.Optimizer != nil && entry.Wrapper.Output.Projection == nil
}

func identityEntry(command []string) tailoringbundle.CommandEntry {
	return tailoringbundle.CommandEntry{
		Command:  append(make([]string, 0, len(command)), command...),
		Presence: tailoringbundle.PresenceInclude,
		Reason:   "Include this verified command without transformation.",
		Options:  &tailoringbundle.OptionSurface{Default: tailoringbundle.SurfaceDefaultInherit, Include: []string{}, Exclude: []string{}},
		Wrapper: &tailoringbundle.Wrapper{
			Kind: tailoringbundle.WrapperIdentity, Before: []tailoringbundle.StageAction{},
			Invoke: tailoringbundle.Invocation{OptionDefaults: []tailoringbundle.OptionDefault{}, AppendArgs: []string{}}, After: []tailoringbundle.StageAction{},
		},
	}
}

func requireVerifiedCommand(catalog sourcecatalog.Catalog, command []string) error {
	key := strings.Join(command, " ")
	for _, candidate := range catalog.Commands {
		if strings.Join(candidate.Path, " ") != key {
			continue
		}
		if candidate.Provenance != sourcecatalog.ProvenanceVerifiedBuiltin {
			return fault.New(fault.KindRejected, "unverified_catalog_command", "The selected catalog command is not verified built-in evidence.", false, helpAction())
		}
		return nil
	}
	return fault.New(fault.KindNotFound, "catalog_command_not_found", "The exact command path is absent from the selected catalog.", false, helpAction())
}

func optionalObservationPath(values []string) (string, bool, error) {
	if len(values) > 1 {
		return "", false, fault.New(fault.KindInvalidInput, "invalid_processor_observation_selection", "Select at most one processor observation JSON document.", false, helpAction())
	}
	if len(values) == 0 {
		return "", false, nil
	}
	if strings.TrimSpace(values[0]) == "" {
		return "", false, fault.New(fault.KindInvalidInput, "invalid_processor_observation_selection", "The processor observation path must not be empty.", false, helpAction())
	}
	return values[0], true, nil
}

func (s *Service) requireProcessorSupport() error {
	if s.processorSupports != 1 || portcheck.IsNil(s.processor.Observations) || portcheck.IsNil(s.processor.Compatibility) {
		return fmt.Errorf("spec init processor evidence adapters are not configured")
	}
	return nil
}

func preserveProcessorLoad(err error) error {
	if public, ok := fault.PublicCopy(err); ok {
		return public
	}
	return fault.Wrap(fault.KindInternal, "internal_error", "The specification draft could not load its processor observation.", false, err, helpAction())
}

func incompatibleProcessorDefault(err error) error {
	return fault.Wrap(fault.KindRejected, "processor_default_not_admitted", "The selected source command and processor observation do not match an admitted authoring default.", false, err, helpAction())
}

func helpAction() fault.NextAction {
	return fault.NextAction{Command: "help spec init", Reason: "Select one exact verified command and, when wanted, one explicit supported processor observation."}
}
