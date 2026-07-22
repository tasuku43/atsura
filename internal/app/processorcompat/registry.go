// Package processorcompat owns the finite source/processor compatibility
// tuples admitted by Atsura. It neither discovers processors nor starts a
// process.
package processorcompat

import (
	"errors"
	"fmt"
	"reflect"
	"regexp"

	"github.com/tasuku43/atsura/internal/domain/processorprocess"
	"github.com/tasuku43/atsura/internal/domain/sourcecatalog"
	"github.com/tasuku43/atsura/internal/domain/tailoringbundle"
	"github.com/tasuku43/atsura/internal/domain/tailoringplan"
)

const (
	// ContractID is the only external output-processor contract admitted by
	// this registry revision.
	ContractID = "atsura.output.rtk_go_test_pass.v1"

	ProcessorAdapterKind     = "atsura.processor.rtk"
	ProcessorContractVersion = 1
	ProcessorVersion         = "0.43.0"
	SourceAdapterKind        = "atsura.source.go_cli"
	SourceContractVersion    = 2
	InputFormat              = "go_test_jsonl"
	OutputFormat             = "go_test_pass_summary"

	DefaultReason = "Use the inspected RTK v0.43.0 pass-only optimizer for the exact no-argument Go test surface."
)

var (
	ErrRegistry    = errors.New("processor compatibility registry is not configured")
	ErrObservation = errors.New("processor observation is not compatible")
	ErrSourceTuple = errors.New("source catalog tuple is not compatible")
	ErrBinding     = errors.New("processor binding is not compatible")
	ErrSurface     = errors.New("processor surface tuple is not compatible")
	ErrPlan        = errors.New("processor plan tuple is not compatible")
)

// ErrorKind classifies one fail-closed compatibility decision.
type ErrorKind string

const (
	ErrorRegistry    ErrorKind = "registry"
	ErrorObservation ErrorKind = "observation"
	ErrorSourceTuple ErrorKind = "source_tuple"
	ErrorBinding     ErrorKind = "binding"
	ErrorSurface     ErrorKind = "surface"
	ErrorPlan        ErrorKind = "plan"
)

// Error is the exported typed result for a rejected compatibility tuple.
// Detail is bounded implementation-owned text and never includes source or
// processor output.
type Error struct {
	Kind   ErrorKind
	Detail string
}

func (e *Error) Error() string {
	if e == nil {
		return "processor compatibility error"
	}
	if e.Detail == "" {
		return fmt.Sprintf("processor compatibility %s rejected", e.Kind)
	}
	return fmt.Sprintf("processor compatibility %s rejected: %s", e.Kind, e.Detail)
}

// Unwrap gives callers a stable broad category while preserving typed detail.
func (e *Error) Unwrap() error {
	if e == nil {
		return nil
	}
	switch e.Kind {
	case ErrorRegistry:
		return ErrRegistry
	case ErrorObservation:
		return ErrObservation
	case ErrorSourceTuple:
		return ErrSourceTuple
	case ErrorBinding:
		return ErrBinding
	case ErrorSurface:
		return ErrSurface
	case ErrorPlan:
		return ErrPlan
	default:
		return nil
	}
}

type artifact struct {
	sha256 string
	size   int64
}

// These are extracted-binary identities from the official RTK v0.43.0
// release. Archive provenance remains in .harness/processors.json. Repeating
// the identity check here ensures a loaded observation cannot manufacture a
// different executable identity after inspection.
var officialArtifacts = map[processorprocess.Platform]artifact{
	{OS: "linux", Arch: "amd64"}:  {sha256: "f160611f3baee17fe4eb3a04c56a8bc3d15fec4274d8838016088d4776c6f628", size: 10083968},
	{OS: "linux", Arch: "arm64"}:  {sha256: "86bd2badb697e41fa4fae805ed1a42d9b2495600260918d6ba9c148bc40013cf", size: 8544624},
	{OS: "darwin", Arch: "amd64"}: {sha256: "22adaa27b3fd6d8906159ba3ff7ca8346e914df112408bcc7a88cda30a3a6107", size: 9006316},
	{OS: "darwin", Arch: "arm64"}: {sha256: "2dab449f32ea744c30b02a3ef9806e3e7d3b356a145332f3f2aaabb5ea48edee", size: 7763408},
}

var stableGo126 = regexp.MustCompile(`^go1\.26\.(0|[1-9][0-9]*)$`)

var goTestEventFields = []string{"Action", "Elapsed", "FailedBuild", "Output", "Package", "Test", "Time"}

// Registry is the closed compatibility authority. It has no dynamic
// registrations, plugins, PATH lookup, or ambient configuration.
type Registry struct{}

// New returns the complete registry for this revision.
func New() *Registry { return &Registry{} }

// VerifyObservation admits only an exact official RTK v0.43.0 observation on
// one of the four platforms accepted by ADR 0012.
func (r *Registry) VerifyObservation(observation processorprocess.Observation) error {
	if err := r.configured(); err != nil {
		return err
	}
	if err := observation.Validate(); err != nil {
		return compatibilityError(ErrorObservation, "observation structure is invalid")
	}
	if observation.Adapter.Kind != ProcessorAdapterKind || observation.Adapter.ContractVersion != ProcessorContractVersion {
		return compatibilityError(ErrorObservation, "processor adapter contract is not admitted")
	}
	if observation.Version != ProcessorVersion {
		return compatibilityError(ErrorObservation, "processor version is not admitted")
	}
	if len(observation.Probe.Argv) != 1 || observation.Probe.Argv[0] != "--version" ||
		observation.Probe.EnvironmentContract != processorprocess.EnvironmentRTKIsolatedV2 || observation.Probe.Attempts != 1 {
		return compatibilityError(ErrorObservation, "processor probe contract is not admitted")
	}
	wanted, exists := officialArtifacts[observation.Platform]
	if !exists {
		return compatibilityError(ErrorObservation, "processor platform is not admitted")
	}
	if observation.Identity.SHA256 != wanted.sha256 || observation.Identity.Size != wanted.size {
		return compatibilityError(ErrorObservation, "processor artifact identity is not admitted")
	}
	return nil
}

// Binding constructs the sole canonical process binding for an admitted
// observation. The executable path and identity come only from that evidence;
// every other process fact is registry-owned.
func (r *Registry) Binding(observation processorprocess.Observation) (tailoringbundle.ProcessorBinding, error) {
	if err := r.VerifyObservation(observation); err != nil {
		return tailoringbundle.ProcessorBinding{}, err
	}
	binding := tailoringbundle.ProcessorBinding{
		Contract:            ContractID,
		Observation:         cloneObservation(observation),
		InputFormat:         InputFormat,
		OutputFormat:        OutputFormat,
		AllowOriginalOutput: true,
		Execution: tailoringbundle.ProcessorExecution{
			Args:                 []string{"pipe", "--filter=go-test"},
			StdinMode:            "stage_input",
			WorkingDirectoryMode: "isolated",
			EnvironmentContract:  processorprocess.EnvironmentRTKIsolatedV2,
			MaxAttempts:          1,
			TimeoutMillis:        processorprocess.MaxTimeout.Milliseconds(),
			StdoutLimitBytes:     processorprocess.MaxStdoutBytes,
			StderrLimitBytes:     processorprocess.MaxStderrBytes,
		},
	}
	if err := binding.Validate(); err != nil {
		return tailoringbundle.ProcessorBinding{}, compatibilityError(ErrorBinding, "canonical binding violates the domain contract")
	}
	return binding, nil
}

// DefaultEntry returns the authoring default only when both explicit
// processor evidence and the exact Go test catalog tuple are present.
func (r *Registry) DefaultEntry(catalog sourcecatalog.Catalog, observation processorprocess.Observation) (tailoringbundle.CommandEntry, error) {
	if err := r.verifySourceCatalog(catalog); err != nil {
		return tailoringbundle.CommandEntry{}, err
	}
	if err := r.VerifyObservation(observation); err != nil {
		return tailoringbundle.CommandEntry{}, err
	}
	return tailoringbundle.CommandEntry{
		Command:  []string{"test"},
		Presence: tailoringbundle.PresenceInclude,
		Reason:   DefaultReason,
		Options: &tailoringbundle.OptionSurface{
			Default: tailoringbundle.SurfaceDefaultInherit,
			Include: []string{},
			Exclude: []string{},
		},
		Wrapper: &tailoringbundle.Wrapper{
			Kind:   tailoringbundle.WrapperTransform,
			Before: []tailoringbundle.StageAction{},
			Invoke: tailoringbundle.Invocation{OptionDefaults: []tailoringbundle.OptionDefault{}, AppendArgs: []string{"-json"}},
			Output: &tailoringbundle.Output{
				Kind: tailoringbundle.OutputKindOptimizer,
				Optimizer: &tailoringbundle.Optimizer{
					Input:               InputFormat,
					Contract:            ContractID,
					AllowOriginalOutput: true,
				},
			},
			After: []tailoringbundle.StageAction{},
		},
	}, nil
}

// VerifySurface proves the complete compiled bundle side of the only admitted
// tuple. Catalogs may retain other observed Go commands, but the tailored
// surface must contain only the exact no-option test transform.
func (r *Registry) VerifySurface(bundle tailoringbundle.Bundle) error {
	if err := r.configured(); err != nil {
		return err
	}
	if err := bundle.Validate(); err != nil {
		return compatibilityError(ErrorSurface, "bundle is invalid")
	}
	if err := r.verifySourceCatalog(bundle.Catalog); err != nil {
		return compatibilityError(ErrorSurface, "bundle source tuple is not admitted")
	}
	if len(bundle.Surface) != 1 || !exactCommand(bundle.Surface[0].Command, "test") ||
		!exactOptionSurface(bundle.Surface[0].Options) || !exactOptimizerWrapper(bundle.Surface[0].Wrapper) {
		return compatibilityError(ErrorSurface, "tailored surface is not the exact Go test optimizer")
	}
	if len(bundle.Processors) != 1 {
		return compatibilityError(ErrorSurface, "surface must bind exactly one processor")
	}
	wanted, err := r.Binding(bundle.Processors[0].Observation)
	if err != nil || !reflect.DeepEqual(bundle.Processors[0], wanted) {
		return compatibilityError(ErrorSurface, "processor binding is not canonical")
	}
	return nil
}

// VerifyPlan proves the invocation-specific side of the tuple. In particular,
// it rejects package patterns, flags, test-binary arguments, and any transform
// other than exact `go test` to `go test -json`.
func (r *Registry) VerifyPlan(plan tailoringplan.Plan) error {
	if err := r.configured(); err != nil {
		return err
	}
	if err := plan.Validate(); err != nil {
		return compatibilityError(ErrorPlan, "plan is invalid")
	}
	if plan.Source.AdapterKind != SourceAdapterKind || plan.Source.AdapterContractVersion != SourceContractVersion || !stableGo126.MatchString(plan.Source.Version) {
		return compatibilityError(ErrorPlan, "plan source tuple is not admitted")
	}
	if plan.ResultMode != tailoringplan.ResultModeOriginalPreservingOptimizer || plan.SurfaceOrigin != tailoringplan.SurfaceOriginExplicit ||
		!exactCommand(plan.MatchedCommand, "test") || !exactOptionSurface(plan.Options) || plan.WrapperKind != tailoringbundle.WrapperTransform {
		return compatibilityError(ErrorPlan, "plan surface is not the exact Go test optimizer")
	}
	if len(plan.OriginalArgv) != 2 || plan.OriginalArgv[1] != "test" ||
		len(plan.TransformedArgv) != 3 || plan.TransformedArgv[0] != plan.Source.ResolvedPath || plan.TransformedArgv[1] != "test" || plan.TransformedArgv[2] != "-json" {
		return compatibilityError(ErrorPlan, "plan argv is outside the no-argument tuple")
	}
	if plan.Stages.Output == nil || !exactOptimizerOutput(*plan.Stages.Output) ||
		len(plan.Stages.Invoke.OptionDefaults) != 0 || len(plan.Stages.Invoke.AppliedOptionDefaults) != 0 ||
		len(plan.Stages.Invoke.AppendedArgs) != 1 || plan.Stages.Invoke.AppendedArgs[0] != "-json" {
		return compatibilityError(ErrorPlan, "plan output stage is not canonical")
	}
	if plan.Processor == nil {
		return compatibilityError(ErrorPlan, "plan has no processor binding")
	}
	wanted, err := r.Binding(plan.Processor.Observation)
	if err != nil || !reflect.DeepEqual(*plan.Processor, wanted) {
		return compatibilityError(ErrorPlan, "plan processor binding is not canonical")
	}
	return nil
}

func (r *Registry) configured() error {
	if r == nil {
		return compatibilityError(ErrorRegistry, "registry is nil")
	}
	return nil
}

func (r *Registry) verifySourceCatalog(catalog sourcecatalog.Catalog) error {
	if err := r.configured(); err != nil {
		return err
	}
	if err := catalog.Validate(); err != nil {
		return compatibilityError(ErrorSourceTuple, "source catalog is invalid")
	}
	if catalog.Adapter.Kind != SourceAdapterKind || catalog.Adapter.ContractVersion != SourceContractVersion || !stableGo126.MatchString(catalog.Source.Version) {
		return compatibilityError(ErrorSourceTuple, "source adapter contract or recorded version is not admitted")
	}
	for _, command := range catalog.Commands {
		if !exactCommand(command.Path, "test") {
			continue
		}
		if command.Provenance != sourcecatalog.ProvenanceVerifiedBuiltin || len(command.Options) != 0 || !exactGoTestOutput(command.StructuredOutput) {
			return compatibilityError(ErrorSourceTuple, "Go test catalog evidence is not exact")
		}
		return nil
	}
	return compatibilityError(ErrorSourceTuple, "Go test command is absent")
}

func exactGoTestOutput(values []sourcecatalog.StructuredOutput) bool {
	return len(values) == 1 && values[0].Format == InputFormat && values[0].SelectorFlag == "-json" && reflect.DeepEqual(values[0].Fields, goTestEventFields)
}

func exactOptionSurface(value tailoringbundle.OptionSurface) bool {
	return value.Default == tailoringbundle.SurfaceDefaultInherit && value.Include != nil && len(value.Include) == 0 && value.Exclude != nil && len(value.Exclude) == 0
}

func exactOptimizerWrapper(value tailoringbundle.Wrapper) bool {
	return value.Kind == tailoringbundle.WrapperTransform && value.Before != nil && len(value.Before) == 0 &&
		value.After != nil && len(value.After) == 0 && value.Invoke.OptionDefaults != nil && len(value.Invoke.OptionDefaults) == 0 &&
		reflect.DeepEqual(value.Invoke.AppendArgs, []string{"-json"}) &&
		value.Output != nil && exactOptimizerOutput(*value.Output)
}

func exactOptimizerOutput(value tailoringbundle.Output) bool {
	return value.Kind == tailoringbundle.OutputKindOptimizer && value.Projection == nil && value.Optimizer != nil &&
		value.Optimizer.Input == InputFormat && value.Optimizer.Contract == ContractID && value.Optimizer.AllowOriginalOutput
}

func exactCommand(value []string, segment string) bool {
	return len(value) == 1 && value[0] == segment
}

func cloneObservation(value processorprocess.Observation) processorprocess.Observation {
	result := value
	result.Probe.Argv = append([]string{}, value.Probe.Argv...)
	return result
}

func compatibilityError(kind ErrorKind, detail string) error {
	return &Error{Kind: kind, Detail: detail}
}
