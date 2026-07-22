// Package tailoringplan constructs one deterministic wrapper plan from a
// validated bundle and attempted source invocation. It performs no I/O.
package tailoringplan

import (
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"sort"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/tasuku43/atsura/internal/domain/processorprocess"
	"github.com/tasuku43/atsura/internal/domain/sourcecatalog"
	"github.com/tasuku43/atsura/internal/domain/sourceprocess"
	"github.com/tasuku43/atsura/internal/domain/tailoring"
	"github.com/tasuku43/atsura/internal/domain/tailoringbundle"
)

const SchemaVersion = 6

var (
	ErrInvalidInvocation        = errors.New("invalid tailored invocation")
	ErrSourceExecutableMismatch = errors.New("source executable does not match bundle")
	ErrCommandNotInSurface      = errors.New("command not in tailored surface")
	ErrOptionNotInSurface       = errors.New("option not in tailored surface")
	ErrInvalidPlan              = errors.New("invalid wrapper plan")
)

// Mode distinguishes normal tailored resolution from future explicit raw
// execution. This slice constructs only tailored plans.
type Mode string

const ModeTailored Mode = "tailored"

// ResultMode states which plan-owned success boundary applies after the source
// process conventionally completes.
type ResultMode string

const (
	ResultModeTransformedJSON             ResultMode = "transformed_json"
	ResultModeSourceStreamPassthrough     ResultMode = "source_stream_passthrough"
	ResultModeOriginalPreservingOptimizer ResultMode = "original_preserving_optimizer"
)

// Attempt is the caller's exact source executable spelling and argv.
type Attempt struct {
	Executable string
	Args       []string
}

// SourceIdentity is the exact bundle-bound source evidence included in a plan.
type SourceIdentity struct {
	RequestedExecutable    string `json:"requested_executable"`
	ResolvedPath           string `json:"resolved_path"`
	SHA256                 string `json:"sha256"`
	Size                   int64  `json:"size"`
	Version                string `json:"version"`
	AdapterKind            string `json:"adapter_kind"`
	AdapterContractVersion int    `json:"adapter_contract_version"`
}

// SurfaceOrigin states whether the matched surface entry was explicit in the
// specification or inherited from its default.
type SurfaceOrigin string

const (
	SurfaceOriginExplicit  SurfaceOrigin = "explicit"
	SurfaceOriginInherited SurfaceOrigin = "inherited"
)

// Invocation is the exact no-shell source invocation produced by the wrapper.
type Invocation struct {
	Executable            string                          `json:"executable"`
	Args                  []string                        `json:"args"`
	OptionDefaults        []tailoringbundle.OptionDefault `json:"option_defaults"`
	AppliedOptionDefaults []tailoringbundle.OptionDefault `json:"applied_option_defaults"`
	AppendedArgs          []string                        `json:"appended_args"`
	StdinMode             StdinMode                       `json:"stdin_mode"`
	WorkingDirectoryMode  WorkingDirectoryMode            `json:"working_directory_mode"`
	EnvironmentMode       EnvironmentMode                 `json:"environment_mode"`
	MaxAttempts           int                             `json:"max_attempts"`
	TimeoutMillis         int64                           `json:"timeout_millis"`
	StdoutLimitBytes      int                             `json:"stdout_limit_bytes"`
	StderrLimitBytes      int                             `json:"stderr_limit_bytes"`
}

type StdinMode string
type WorkingDirectoryMode string
type EnvironmentMode string

const (
	StdinModeClosed             StdinMode            = "closed"
	WorkingDirectoryModeInherit WorkingDirectoryMode = "inherit"
	EnvironmentModeInherit      EnvironmentMode      = "inherit"
)

// StageKind is one fixed position in the wrapper pipeline.
type StageKind string

const (
	StageBefore StageKind = "before"
	StageInvoke StageKind = "invoke"
	StageOutput StageKind = "output"
	StageAfter  StageKind = "after"
)

// Stages preserves the wrapper's ordered, typed execution boundaries.
type Stages struct {
	Order  []StageKind                   `json:"order"`
	Before []tailoringbundle.StageAction `json:"before"`
	Invoke Invocation                    `json:"invoke"`
	Output *tailoringbundle.Output       `json:"output"`
	After  []tailoringbundle.StageAction `json:"after"`
}

// Plan is the canonical complete result shared by preview and supported runtime.
type Plan struct {
	SchemaVersion       int                               `json:"schema_version"`
	Mode                Mode                              `json:"mode"`
	ResultMode          ResultMode                        `json:"result_mode"`
	BundleDigest        string                            `json:"bundle_digest"`
	CatalogDigest       string                            `json:"catalog_digest"`
	SpecificationDigest string                            `json:"specification_digest"`
	Source              SourceIdentity                    `json:"source"`
	MatchedCommand      []string                          `json:"matched_command"`
	SurfaceOrigin       SurfaceOrigin                     `json:"surface_origin"`
	SpecificationEntry  *tailoringbundle.CommandEntry     `json:"specification_entry"`
	Reason              string                            `json:"reason"`
	Options             tailoringbundle.OptionSurface     `json:"options"`
	WrapperKind         tailoringbundle.WrapperKind       `json:"wrapper_kind"`
	OriginalArgv        []string                          `json:"original_argv"`
	TransformedArgv     []string                          `json:"transformed_argv"`
	Processor           *tailoringbundle.ProcessorBinding `json:"processor"`
	Stages              Stages                            `json:"stages"`
}

// Build resolves one attempted invocation into a complete tailored plan.
func Build(bundleDigest string, bundle tailoringbundle.Bundle, current sourceprocess.Identity, attempt Attempt) (Plan, error) {
	if err := bundle.Validate(); err != nil {
		return Plan{}, invalidPlan("bundle: %v", err)
	}
	wantedDigest, err := bundle.Digest()
	if err != nil || wantedDigest != bundleDigest {
		return Plan{}, invalidPlan("bundle digest is invalid or mismatched")
	}
	if err := attempt.validate(); err != nil {
		return Plan{}, err
	}
	if attempt.Executable != bundle.Catalog.Source.RequestedExecutable && attempt.Executable != bundle.Catalog.Source.ResolvedPath {
		return Plan{}, fmt.Errorf("%w: expected exact requested executable or resolved path", ErrSourceExecutableMismatch)
	}
	wantedIdentity := sourceprocess.Identity{ResolvedPath: bundle.Catalog.Source.ResolvedPath, SHA256: bundle.Catalog.Source.SHA256, Size: bundle.Catalog.Source.Size}
	if err := current.Validate(); err != nil || current != wantedIdentity {
		return Plan{}, invalidPlan("current source identity does not match the bundle")
	}

	command, ok := longestCommandPrefix(bundle.Catalog.Commands, attempt.Args)
	if !ok {
		return Plan{}, invalidInvocation("argv does not begin with a cataloged command path")
	}
	if ambiguousDescendant(bundle.Catalog.Commands, command.Path, attempt.Args) {
		return Plan{}, invalidInvocation("argv after command %q is ambiguous with an unobserved descendant; use -- before positional data", strings.Join(command.Path, " "))
	}
	entry, err := bundle.Resolve(command.Path)
	if err != nil {
		if errors.Is(err, tailoringbundle.ErrCommandNotInSurface) {
			return Plan{}, fmt.Errorf("%w: %q", ErrCommandNotInSurface, strings.Join(command.Path, " "))
		}
		return Plan{}, invalidPlan("surface resolution: %v", err)
	}
	if err := validateTailoredOptions(command, entry.Options, attempt.Args[len(command.Path):]); err != nil {
		return Plan{}, err
	}

	optionDefaults := cloneOptionDefaults(entry.Wrapper.Invoke.OptionDefaults)
	appliedOptionDefaults := appliedDefaults(optionDefaults, attempt.Args[len(command.Path):])
	transformedArgs := make([]string, 0, len(attempt.Args)+len(appliedOptionDefaults)+len(entry.Wrapper.Invoke.AppendArgs))
	transformedArgs = append(transformedArgs, command.Path...)
	for _, optionDefault := range appliedOptionDefaults {
		transformedArgs = append(transformedArgs, optionDefault.Option+"="+optionDefault.Value)
	}
	transformedArgs = append(transformedArgs, attempt.Args[len(command.Path):]...)
	transformedArgs = append(transformedArgs, entry.Wrapper.Invoke.AppendArgs...)
	if entry.Wrapper.Output != nil {
		if err := validateOutputSelector(command, *entry.Wrapper.Output, transformedArgs[len(command.Path):]); err != nil {
			return Plan{}, err
		}
	}
	request := sourceprocess.Request{
		Executable:  bundle.Catalog.Source.ResolvedPath,
		Args:        transformedArgs,
		Timeout:     sourceprocess.MaxTimeout,
		StdoutLimit: sourceprocess.MaxStdoutBytes,
		StderrLimit: sourceprocess.MaxStderrBytes,
	}
	if err := request.Validate(); err != nil {
		return Plan{}, invalidInvocation("transformed invocation exceeds the source-process contract: %v", err)
	}
	var processor *tailoringbundle.ProcessorBinding
	if entry.Wrapper.Output != nil && entry.Wrapper.Output.Kind == tailoringbundle.OutputKindOptimizer && entry.Wrapper.Output.Optimizer != nil {
		binding, found, err := bundle.Processor(entry.Wrapper.Output.Optimizer.Contract)
		if err != nil {
			return Plan{}, invalidPlan("processor binding: %v", err)
		}
		if !found {
			return Plan{}, invalidPlan("optimizer contract has no processor binding")
		}
		processor = &binding
	}

	origin := SurfaceOriginInherited
	var appliedEntry *tailoringbundle.CommandEntry
	for _, specificationEntry := range bundle.Specification.Commands {
		if reflect.DeepEqual(specificationEntry.Command, entry.Command) {
			origin = SurfaceOriginExplicit
			copy := cloneCommandEntry(specificationEntry)
			appliedEntry = &copy
			break
		}
	}
	plan := Plan{
		SchemaVersion:       SchemaVersion,
		Mode:                ModeTailored,
		ResultMode:          resultModeFor(entry.Wrapper.Output),
		BundleDigest:        bundleDigest,
		CatalogDigest:       bundle.CatalogDigest,
		SpecificationDigest: bundle.SpecificationDigest,
		Source: SourceIdentity{
			RequestedExecutable:    bundle.Catalog.Source.RequestedExecutable,
			ResolvedPath:           bundle.Catalog.Source.ResolvedPath,
			SHA256:                 bundle.Catalog.Source.SHA256,
			Size:                   bundle.Catalog.Source.Size,
			Version:                bundle.Catalog.Source.Version,
			AdapterKind:            bundle.Catalog.Adapter.Kind,
			AdapterContractVersion: bundle.Catalog.Adapter.ContractVersion,
		},
		MatchedCommand:     append([]string{}, entry.Command...),
		SurfaceOrigin:      origin,
		SpecificationEntry: appliedEntry,
		Reason:             entry.Reason,
		Options:            cloneOptions(entry.Options),
		WrapperKind:        entry.Wrapper.Kind,
		OriginalArgv:       append([]string{attempt.Executable}, attempt.Args...),
		TransformedArgv:    append([]string{bundle.Catalog.Source.ResolvedPath}, transformedArgs...),
		Processor:          cloneProcessor(processor),
		Stages: Stages{
			Order:  []StageKind{StageBefore, StageInvoke, StageOutput, StageAfter},
			Before: append([]tailoringbundle.StageAction{}, entry.Wrapper.Before...),
			Invoke: Invocation{
				Executable:            bundle.Catalog.Source.ResolvedPath,
				Args:                  append([]string{}, transformedArgs...),
				OptionDefaults:        optionDefaults,
				AppliedOptionDefaults: cloneOptionDefaults(appliedOptionDefaults),
				AppendedArgs:          append([]string{}, entry.Wrapper.Invoke.AppendArgs...),
				StdinMode:             StdinModeClosed,
				WorkingDirectoryMode:  WorkingDirectoryModeInherit,
				EnvironmentMode:       EnvironmentModeInherit,
				MaxAttempts:           1,
				TimeoutMillis:         sourceprocess.MaxTimeout.Milliseconds(),
				StdoutLimitBytes:      sourceprocess.MaxStdoutBytes,
				StderrLimitBytes:      sourceprocess.MaxStderrBytes,
			},
			Output: cloneOutput(entry.Wrapper.Output),
			After:  append([]tailoringbundle.StageAction{}, entry.Wrapper.After...),
		},
	}
	if err := plan.Validate(); err != nil {
		return Plan{}, err
	}
	return plan, nil
}

// Validate proves that a detached plan is complete and internally coherent.
func (p Plan) Validate() error {
	if p.SchemaVersion != SchemaVersion || p.Mode != ModeTailored {
		return invalidPlan("schema version and tailored mode are required")
	}
	switch p.ResultMode {
	case ResultModeTransformedJSON:
		if p.Stages.Output == nil || p.Stages.Output.Kind != tailoringbundle.OutputKindProjection || p.Processor != nil {
			return invalidPlan("transformed JSON result mode requires a projection output stage")
		}
	case ResultModeSourceStreamPassthrough:
		if p.Stages.Output != nil || p.Processor != nil {
			return invalidPlan("source-stream result mode cannot contain an output stage")
		}
	case ResultModeOriginalPreservingOptimizer:
		if p.Stages.Output == nil || p.Stages.Output.Kind != tailoringbundle.OutputKindOptimizer || p.Stages.Output.Optimizer == nil || p.Processor == nil {
			return invalidPlan("original-preserving optimizer result mode requires an optimizer output stage")
		}
		if err := p.Processor.Validate(); err != nil || p.Processor.Contract != p.Stages.Output.Optimizer.Contract || p.Processor.InputFormat != p.Stages.Output.Optimizer.Input || p.Processor.AllowOriginalOutput != p.Stages.Output.Optimizer.AllowOriginalOutput {
			return invalidPlan("optimizer output stage and processor binding disagree")
		}
	default:
		return invalidPlan("result mode is invalid")
	}
	for name, digest := range map[string]string{
		"bundle": p.BundleDigest, "catalog": p.CatalogDigest, "specification": p.SpecificationDigest,
	} {
		if !validDigest(digest) {
			return invalidPlan("%s digest is invalid", name)
		}
	}
	identity := sourceprocess.Identity{ResolvedPath: p.Source.ResolvedPath, SHA256: p.Source.SHA256, Size: p.Source.Size}
	if err := identity.Validate(); err != nil || validateText(p.Source.RequestedExecutable, 256) != nil || validateText(p.Source.Version, 256) != nil || !validNamespaced(p.Source.AdapterKind) || p.Source.AdapterContractVersion <= 0 {
		return invalidPlan("source identity is invalid")
	}
	if p.SurfaceOrigin != SurfaceOriginExplicit && p.SurfaceOrigin != SurfaceOriginInherited {
		return invalidPlan("surface origin is invalid")
	}
	if p.SurfaceOrigin == SurfaceOriginExplicit && p.SpecificationEntry == nil {
		return invalidPlan("explicit surface origin requires a specification entry")
	}
	if p.SurfaceOrigin == SurfaceOriginInherited && p.SpecificationEntry != nil {
		return invalidPlan("inherited surface origin cannot contain a specification entry")
	}
	if len(p.MatchedCommand) == 0 || len(p.MatchedCommand) > sourcecatalog.MaxCommandSegments {
		return invalidPlan("matched command is missing or unbounded")
	}
	for _, segment := range p.MatchedCommand {
		if !validStableName(segment) {
			return invalidPlan("matched command segment is invalid")
		}
	}
	if err := validateText(p.Reason, sourcecatalog.MaxTextBytes); err != nil {
		return invalidPlan("reason: %v", err)
	}
	if err := validateOptions(p.Options); err != nil {
		return invalidPlan("options: %v", err)
	}
	if p.OriginalArgv == nil || p.TransformedArgv == nil || len(p.OriginalArgv) < 2 || len(p.TransformedArgv) < 2 {
		return invalidPlan("original and transformed argv must be explicit and include a command")
	}
	if p.OriginalArgv[0] != p.Source.RequestedExecutable && p.OriginalArgv[0] != p.Source.ResolvedPath {
		return invalidPlan("original executable does not match source evidence")
	}
	originalRequest := sourceprocess.Request{Executable: p.OriginalArgv[0], Args: p.OriginalArgv[1:], Timeout: sourceprocess.MaxTimeout, StdoutLimit: sourceprocess.MaxStdoutBytes, StderrLimit: sourceprocess.MaxStderrBytes}
	if err := originalRequest.Validate(); err != nil {
		return invalidPlan("original invocation: %v", err)
	}
	if p.TransformedArgv[0] != p.Source.ResolvedPath || !reflect.DeepEqual(p.Stages.Invoke.Args, p.TransformedArgv[1:]) || p.Stages.Invoke.Executable != p.Source.ResolvedPath {
		return invalidPlan("transformed argv and invoke stage disagree")
	}
	if !hasPrefix(p.OriginalArgv[1:], p.MatchedCommand) {
		return invalidPlan("matched command is not an original argv prefix")
	}
	if !reflect.DeepEqual(p.Stages.Order, []StageKind{StageBefore, StageInvoke, StageOutput, StageAfter}) {
		return invalidPlan("stage order is invalid")
	}
	if p.Stages.Before == nil || p.Stages.After == nil || p.Stages.Invoke.Args == nil || p.Stages.Invoke.OptionDefaults == nil || p.Stages.Invoke.AppliedOptionDefaults == nil || p.Stages.Invoke.AppendedArgs == nil {
		return invalidPlan("stage lists must be explicit")
	}
	callerArgs := p.OriginalArgv[1:]
	callerTail := callerArgs[len(p.MatchedCommand):]
	if err := validatePlanOptionDefaults(p.Stages.Invoke.OptionDefaults, p.Stages.Invoke.AppendedArgs, callerTail); err != nil {
		return invalidPlan("option defaults: %v", err)
	}
	wantAppliedOptionDefaults := appliedDefaults(p.Stages.Invoke.OptionDefaults, callerTail)
	if !reflect.DeepEqual(wantAppliedOptionDefaults, p.Stages.Invoke.AppliedOptionDefaults) {
		return invalidPlan("applied option defaults do not match original argv")
	}
	wantArgs := append([]string{}, p.MatchedCommand...)
	for _, optionDefault := range wantAppliedOptionDefaults {
		wantArgs = append(wantArgs, optionDefault.Option+"="+optionDefault.Value)
	}
	wantArgs = append(wantArgs, callerTail...)
	wantArgs = append(wantArgs, p.Stages.Invoke.AppendedArgs...)
	if !reflect.DeepEqual(wantArgs, p.Stages.Invoke.Args) {
		return invalidPlan("invoke args do not contain the exact applied defaults, caller tail, and appended args")
	}
	if p.Stages.Invoke.StdinMode != StdinModeClosed || p.Stages.Invoke.WorkingDirectoryMode != WorkingDirectoryModeInherit || p.Stages.Invoke.EnvironmentMode != EnvironmentModeInherit {
		return invalidPlan("source process framing is invalid")
	}
	if p.Stages.Invoke.MaxAttempts != 1 || p.Stages.Invoke.TimeoutMillis != sourceprocess.MaxTimeout.Milliseconds() || p.Stages.Invoke.StdoutLimitBytes != sourceprocess.MaxStdoutBytes || p.Stages.Invoke.StderrLimitBytes != sourceprocess.MaxStderrBytes {
		return invalidPlan("source process bounds are invalid")
	}
	switch p.WrapperKind {
	case tailoringbundle.WrapperIdentity:
		if len(p.Stages.Before) != 0 || len(p.Stages.After) != 0 || len(p.Stages.Invoke.OptionDefaults) != 0 || len(p.Stages.Invoke.AppliedOptionDefaults) != 0 || len(p.Stages.Invoke.AppendedArgs) != 0 || p.Stages.Output != nil {
			return invalidPlan("identity wrapper contains a transformation")
		}
	case tailoringbundle.WrapperTransform:
		if len(p.Stages.Before) != 0 || len(p.Stages.After) != 0 || (len(p.Stages.Invoke.OptionDefaults) == 0 && len(p.Stages.Invoke.AppendedArgs) == 0 && p.Stages.Output == nil) {
			return invalidPlan("transform wrapper is incomplete")
		}
		if p.Stages.Output != nil {
			if err := validateOutput(*p.Stages.Output); err != nil {
				return invalidPlan("output: %v", err)
			}
		}
	default:
		return invalidPlan("wrapper kind is invalid")
	}
	if p.SpecificationEntry != nil {
		entry := p.SpecificationEntry
		if entry.Presence != tailoringbundle.PresenceInclude || !reflect.DeepEqual(entry.Command, p.MatchedCommand) || entry.Reason != p.Reason || entry.Options == nil || entry.Wrapper == nil || !reflect.DeepEqual(*entry.Options, p.Options) {
			return invalidPlan("specification entry does not match the surface binding")
		}
		wrapper := tailoringbundle.Wrapper{
			Kind:   p.WrapperKind,
			Before: append([]tailoringbundle.StageAction{}, p.Stages.Before...),
			Invoke: tailoringbundle.Invocation{
				OptionDefaults: cloneOptionDefaults(p.Stages.Invoke.OptionDefaults),
				AppendArgs:     append([]string{}, p.Stages.Invoke.AppendedArgs...),
			},
			Output: cloneOutput(p.Stages.Output),
			After:  append([]tailoringbundle.StageAction{}, p.Stages.After...),
		}
		if !reflect.DeepEqual(*entry.Wrapper, wrapper) {
			return invalidPlan("specification entry does not match the wrapper stages")
		}
	}
	request := sourceprocess.Request{Executable: p.Stages.Invoke.Executable, Args: p.Stages.Invoke.Args, Timeout: sourceprocess.MaxTimeout, StdoutLimit: sourceprocess.MaxStdoutBytes, StderrLimit: sourceprocess.MaxStderrBytes}
	if err := request.Validate(); err != nil {
		return invalidPlan("source invocation: %v", err)
	}
	return nil
}

// SourceRequest returns the sole exact process contract represented by this
// plan. Runtime callers do not reconstruct executable identity or bounds.
func (p Plan) SourceRequest() (sourceprocess.BoundRequest, error) {
	if err := p.Validate(); err != nil {
		return sourceprocess.BoundRequest{}, err
	}
	request := sourceprocess.BoundRequest{
		Process: sourceprocess.Request{
			Executable:  p.Stages.Invoke.Executable,
			Args:        append([]string{}, p.Stages.Invoke.Args...),
			Timeout:     time.Duration(p.Stages.Invoke.TimeoutMillis) * time.Millisecond,
			StdoutLimit: p.Stages.Invoke.StdoutLimitBytes,
			StderrLimit: p.Stages.Invoke.StderrLimitBytes,
		},
		ExpectedIdentity: sourceprocess.Identity{ResolvedPath: p.Source.ResolvedPath, SHA256: p.Source.SHA256, Size: p.Source.Size},
	}
	if err := request.Validate(); err != nil {
		return sourceprocess.BoundRequest{}, invalidPlan("source request: %v", err)
	}
	return request, nil
}

// ProcessorRequest returns the sole exact isolated processor process contract
// represented by an optimizer plan for one already-admitted bounded input.
func (p Plan) ProcessorRequest(input []byte) (processorprocess.Request, error) {
	if err := p.Validate(); err != nil {
		return processorprocess.Request{}, err
	}
	if p.ResultMode != ResultModeOriginalPreservingOptimizer || p.Processor == nil {
		return processorprocess.Request{}, invalidPlan("plan has no processor request")
	}
	binding := p.Processor
	request := processorprocess.Request{
		Executable:          binding.Observation.Identity.ResolvedPath,
		Args:                append([]string{}, binding.Execution.Args...),
		Input:               append([]byte(nil), input...),
		Timeout:             time.Duration(binding.Execution.TimeoutMillis) * time.Millisecond,
		StdoutLimit:         binding.Execution.StdoutLimitBytes,
		StderrLimit:         binding.Execution.StderrLimitBytes,
		ExpectedIdentity:    binding.Observation.Identity,
		EnvironmentContract: binding.Execution.EnvironmentContract,
	}
	if err := request.Validate(); err != nil {
		return processorprocess.Request{}, invalidPlan("processor request: %v", err)
	}
	return request, nil
}

// OutputPlan returns a detached typed output transform. A valid plan without
// an output stage returns present=false.
func (p Plan) OutputPlan() (tailoring.OutputPlan, bool, error) {
	if err := p.Validate(); err != nil {
		return tailoring.OutputPlan{}, false, err
	}
	if p.Stages.Output == nil || p.Stages.Output.Kind != tailoringbundle.OutputKindProjection {
		return tailoring.OutputPlan{}, false, nil
	}
	if p.Stages.Output.Projection == nil || p.Stages.Output.Optimizer != nil {
		return tailoring.OutputPlan{}, false, invalidPlan("projection output union is incomplete or contradictory")
	}
	projection := p.Stages.Output.Projection
	renames := make([]tailoring.Rename, len(projection.Rename))
	for index, rename := range projection.Rename {
		renames[index] = tailoring.Rename{From: rename.From, To: rename.To}
	}
	result := tailoring.OutputPlan{Input: tailoring.InputFormat(projection.Input), Select: append([]string{}, projection.Select...), Rename: renames, Render: tailoring.RenderFormat(projection.Render)}
	if err := result.Validate(); err != nil {
		return tailoring.OutputPlan{}, false, invalidPlan("output plan: %v", err)
	}
	return result, true, nil
}

// OptimizerPlan returns the detached finite optimizer declaration. A valid
// non-optimizer plan returns present=false.
func (p Plan) OptimizerPlan() (tailoringbundle.Optimizer, bool, error) {
	if err := p.Validate(); err != nil {
		return tailoringbundle.Optimizer{}, false, err
	}
	if p.Stages.Output == nil || p.Stages.Output.Kind != tailoringbundle.OutputKindOptimizer {
		return tailoringbundle.Optimizer{}, false, nil
	}
	if p.Stages.Output.Optimizer == nil || p.Stages.Output.Projection != nil {
		return tailoringbundle.Optimizer{}, false, invalidPlan("optimizer output union is incomplete or contradictory")
	}
	return *p.Stages.Output.Optimizer, true, nil
}

// CanonicalJSON returns the sole digest representation for a complete plan.
func (p Plan) CanonicalJSON() ([]byte, error) {
	if err := p.Validate(); err != nil {
		return nil, err
	}
	encoded, err := json.Marshal(p)
	if err != nil {
		return nil, fmt.Errorf("encode canonical wrapper plan: %w", err)
	}
	return append(encoded, '\n'), nil
}

// Digest returns the lowercase SHA-256 identity of CanonicalJSON.
func (p Plan) Digest() (string, error) {
	encoded, err := p.CanonicalJSON()
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%x", sha256.Sum256(encoded)), nil
}

func (a Attempt) validate() error {
	if a.Args == nil {
		return invalidInvocation("argv must be an explicit list")
	}
	request := sourceprocess.Request{Executable: a.Executable, Args: a.Args, Timeout: sourceprocess.MaxTimeout, StdoutLimit: sourceprocess.MaxStdoutBytes, StderrLimit: sourceprocess.MaxStderrBytes}
	if err := request.Validate(); err != nil {
		return invalidInvocation("%v", err)
	}
	if len(a.Args) == 0 {
		return invalidInvocation("argv must contain a cataloged command")
	}
	return nil
}

func longestCommandPrefix(commands []sourcecatalog.Command, args []string) (sourcecatalog.Command, bool) {
	var match sourcecatalog.Command
	found := false
	for _, command := range commands {
		if len(command.Path) <= len(match.Path) || !hasPrefix(args, command.Path) {
			continue
		}
		match = command
		found = true
	}
	return match, found
}

func ambiguousDescendant(commands []sourcecatalog.Command, matched, args []string) bool {
	if len(args) <= len(matched) || strings.HasPrefix(args[len(matched)], "-") {
		return false
	}
	for _, command := range commands {
		if len(command.Path) > len(matched) && hasPrefix(command.Path, matched) {
			return true
		}
	}
	return false
}

func validateTailoredOptions(command sourcecatalog.Command, surface tailoringbundle.OptionSurface, args []string) error {
	observed := make(map[string]sourcecatalog.Option, len(command.Options))
	for _, option := range command.Options {
		observed[option.Name] = option
	}
	allowed := make(map[string]bool, len(command.Options))
	for _, option := range command.Options {
		allowed[option.Name] = surface.Default == tailoringbundle.SurfaceDefaultInherit
	}
	for _, name := range surface.Include {
		allowed[name] = true
	}
	for _, name := range surface.Exclude {
		allowed[name] = false
	}

	positionalOnly := false
	for index := 0; index < len(args); index++ {
		argument := args[index]
		if positionalOnly {
			continue
		}
		if argument == "--" {
			positionalOnly = true
			continue
		}
		if strings.HasPrefix(argument, "--") {
			name, value, inline := strings.Cut(argument, "=")
			option, exists := observed[name]
			if !exists {
				return invalidInvocation("option %q is not cataloged for command %q", name, strings.Join(command.Path, " "))
			}
			if !allowed[name] {
				return fmt.Errorf("%w: %q", ErrOptionNotInSurface, name)
			}
			if option.TakesValue {
				if inline {
					_ = value // Explicit empty is preserved and delegated to the source CLI.
					continue
				}
				if index+1 >= len(args) || strings.HasPrefix(args[index+1], "-") {
					return invalidInvocation("option %q requires a value; use %s=<value> for a dash-prefixed value", name, name)
				}
				index++
				continue
			}
			if inline {
				return invalidInvocation("option %q does not take a value", name)
			}
			continue
		}
		if strings.HasPrefix(argument, "-") {
			return invalidInvocation("short or unmodeled option %q is not supported by this tailored surface", argument)
		}
	}
	return nil
}

// appliedDefaults returns the declaration-ordered subset whose exact long
// option name is absent from the caller's active option region. Build invokes
// this only after caller grammar validation; the scan deliberately stops at
// the first exact positional-only marker and does not interpret short aliases.
func appliedDefaults(defaults []tailoringbundle.OptionDefault, callerTail []string) []tailoringbundle.OptionDefault {
	present := make(map[string]struct{}, len(defaults))
	for _, argument := range callerTail {
		if argument == "--" {
			break
		}
		if !strings.HasPrefix(argument, "--") {
			continue
		}
		name, _, _ := strings.Cut(argument, "=")
		present[name] = struct{}{}
	}

	result := make([]tailoringbundle.OptionDefault, 0, len(defaults))
	for _, optionDefault := range defaults {
		if _, exists := present[optionDefault.Option]; exists {
			continue
		}
		result = append(result, optionDefault)
	}
	return result
}

// validatePlanOptionDefaults proves the finite structural facts available in
// a detached plan. Catalog arity, tailored membership, and selector ownership
// remain bundle-owned, but malformed values, duplicates, append overlap, and
// incomplete explicit caller forms still fail without recovering the bundle.
func validatePlanOptionDefaults(defaults []tailoringbundle.OptionDefault, appendArgs, callerTail []string) error {
	if len(defaults)+len(appendArgs) > tailoringbundle.MaxWrapperArguments {
		return fmt.Errorf("combined option defaults and appended args exceed their bound")
	}
	appended := activeLongOptionNames(appendArgs)
	configured := make(map[string]struct{}, len(defaults))
	for _, optionDefault := range defaults {
		if !strings.HasPrefix(optionDefault.Option, "--") || !validStableName(strings.TrimPrefix(optionDefault.Option, "--")) {
			return fmt.Errorf("option %q is invalid", optionDefault.Option)
		}
		if err := validateText(optionDefault.Value, sourceprocess.MaxArgumentBytes); err != nil {
			return fmt.Errorf("option %q value: %v", optionDefault.Option, err)
		}
		if len(optionDefault.Option)+1+len(optionDefault.Value) > sourceprocess.MaxArgumentBytes {
			return fmt.Errorf("option %q canonical argument exceeds its bound", optionDefault.Option)
		}
		if _, exists := configured[optionDefault.Option]; exists {
			return fmt.Errorf("option %q is duplicated", optionDefault.Option)
		}
		configured[optionDefault.Option] = struct{}{}
		if _, exists := appended[optionDefault.Option]; exists {
			return fmt.Errorf("option %q overlaps appended args", optionDefault.Option)
		}
	}

	for index := 0; index < len(callerTail); index++ {
		argument := callerTail[index]
		if argument == "--" {
			break
		}
		name, _, inline := strings.Cut(argument, "=")
		if _, exists := configured[name]; !exists || inline {
			continue
		}
		if index+1 >= len(callerTail) || callerTail[index+1] == "--" || strings.HasPrefix(callerTail[index+1], "-") {
			return fmt.Errorf("caller option %q requires a value", name)
		}
		index++
	}
	return nil
}

func activeLongOptionNames(arguments []string) map[string]struct{} {
	result := make(map[string]struct{})
	for _, argument := range arguments {
		if argument == "--" {
			break
		}
		if !strings.HasPrefix(argument, "--") {
			continue
		}
		name, _, _ := strings.Cut(argument, "=")
		result[name] = struct{}{}
	}
	return result
}

func validateOutputSelector(command sourcecatalog.Command, output tailoringbundle.Output, args []string) error {
	input, ok := outputInput(output)
	if !ok {
		return invalidInvocation("planned output stage does not declare one input format")
	}
	selectorFormats := make(map[string]map[string]struct{})
	for _, structured := range command.StructuredOutput {
		formats := selectorFormats[structured.SelectorFlag]
		if formats == nil {
			formats = make(map[string]struct{})
			selectorFormats[structured.SelectorFlag] = formats
		}
		formats[structured.Format] = struct{}{}
	}

	matched := 0
	for index := 0; index < len(args); index++ {
		argument := args[index]
		if argument == "--" {
			break
		}
		if !strings.HasPrefix(argument, "-") {
			continue
		}
		name, _, inline := strings.Cut(argument, "=")
		formats, isSelector := selectorFormats[name]
		if isSelector {
			if len(formats) != 1 {
				return invalidInvocation("structured-output selector %q does not identify one source format", name)
			}
			if _, wanted := formats[input]; !wanted {
				return invalidInvocation("structured-output selector %q conflicts with planned input format %q", name, input)
			}
			matched++
		}
		if !inline {
			for _, option := range command.Options {
				if option.Name == name && option.TakesValue && index+1 < len(args) {
					index++
					break
				}
			}
		}
	}
	if matched != 1 {
		return invalidInvocation("planned output format %q requires exactly one active cataloged selector before --; found %d", input, matched)
	}
	return nil
}

func validateOptions(value tailoringbundle.OptionSurface) error {
	if value.Default != tailoringbundle.SurfaceDefaultInherit && value.Default != tailoringbundle.SurfaceDefaultExclude {
		return fmt.Errorf("default is invalid")
	}
	if value.Include == nil || value.Exclude == nil || len(value.Include) > sourcecatalog.MaxOptions || len(value.Exclude) > sourcecatalog.MaxOptions || !sortedUnique(value.Include) || !sortedUnique(value.Exclude) {
		return fmt.Errorf("include and exclude must be explicit sorted unique lists")
	}
	if value.Default == tailoringbundle.SurfaceDefaultInherit && len(value.Include) != 0 {
		return fmt.Errorf("inherited options cannot include overrides")
	}
	if value.Default == tailoringbundle.SurfaceDefaultExclude && len(value.Exclude) != 0 {
		return fmt.Errorf("excluded-by-default options cannot exclude overrides")
	}
	seen := make(map[string]struct{}, len(value.Include)+len(value.Exclude))
	for _, values := range [][]string{value.Include, value.Exclude} {
		for _, option := range values {
			if !strings.HasPrefix(option, "--") || !validStableName(strings.TrimPrefix(option, "--")) {
				return fmt.Errorf("option %q is invalid", option)
			}
			if _, exists := seen[option]; exists {
				return fmt.Errorf("option %q is duplicated", option)
			}
			seen[option] = struct{}{}
		}
	}
	return nil
}

func validateOutput(value tailoringbundle.Output) error {
	switch value.Kind {
	case tailoringbundle.OutputKindProjection:
		if value.Projection == nil || value.Optimizer != nil {
			return fmt.Errorf("projection output union is incomplete or contradictory")
		}
		renames := make([]tailoring.Rename, len(value.Projection.Rename))
		for index, rename := range value.Projection.Rename {
			renames[index] = tailoring.Rename{From: rename.From, To: rename.To}
		}
		return (tailoring.OutputPlan{Input: tailoring.InputFormat(value.Projection.Input), Select: value.Projection.Select, Rename: renames, Render: tailoring.RenderFormat(value.Projection.Render)}).Validate()
	case tailoringbundle.OutputKindOptimizer:
		if value.Projection != nil || value.Optimizer == nil || !validStableName(value.Optimizer.Input) || !validNamespaced(value.Optimizer.Contract) || !value.Optimizer.AllowOriginalOutput {
			return fmt.Errorf("optimizer output contract is incomplete or contradictory")
		}
		return nil
	default:
		return fmt.Errorf("output kind is invalid")
	}
}

func resultModeFor(output *tailoringbundle.Output) ResultMode {
	if output == nil {
		return ResultModeSourceStreamPassthrough
	}
	switch output.Kind {
	case tailoringbundle.OutputKindProjection:
		return ResultModeTransformedJSON
	case tailoringbundle.OutputKindOptimizer:
		return ResultModeOriginalPreservingOptimizer
	default:
		return ""
	}
}

func cloneOptions(value tailoringbundle.OptionSurface) tailoringbundle.OptionSurface {
	return tailoringbundle.OptionSurface{Default: value.Default, Include: append([]string{}, value.Include...), Exclude: append([]string{}, value.Exclude...)}
}

func cloneOptionDefaults(values []tailoringbundle.OptionDefault) []tailoringbundle.OptionDefault {
	if values == nil {
		return nil
	}
	return append([]tailoringbundle.OptionDefault{}, values...)
}

func cloneOutput(value *tailoringbundle.Output) *tailoringbundle.Output {
	if value == nil {
		return nil
	}
	copy := *value
	if value.Projection != nil {
		projection := *value.Projection
		projection.Select = append([]string{}, value.Projection.Select...)
		projection.Rename = append([]tailoringbundle.Rename{}, value.Projection.Rename...)
		copy.Projection = &projection
	}
	if value.Optimizer != nil {
		optimizer := *value.Optimizer
		copy.Optimizer = &optimizer
	}
	return &copy
}

func cloneProcessor(value *tailoringbundle.ProcessorBinding) *tailoringbundle.ProcessorBinding {
	if value == nil {
		return nil
	}
	copy := *value
	copy.Observation.Probe.Argv = append([]string{}, value.Observation.Probe.Argv...)
	copy.Execution.Args = append([]string{}, value.Execution.Args...)
	return &copy
}

func outputInput(value tailoringbundle.Output) (string, bool) {
	switch value.Kind {
	case tailoringbundle.OutputKindProjection:
		if value.Projection != nil && value.Optimizer == nil {
			return value.Projection.Input, true
		}
	case tailoringbundle.OutputKindOptimizer:
		if value.Projection == nil && value.Optimizer != nil {
			return value.Optimizer.Input, true
		}
	}
	return "", false
}

func cloneCommandEntry(value tailoringbundle.CommandEntry) tailoringbundle.CommandEntry {
	result := value
	result.Command = append([]string{}, value.Command...)
	if value.Options != nil {
		copy := cloneOptions(*value.Options)
		result.Options = &copy
	}
	if value.Wrapper != nil {
		copy := *value.Wrapper
		copy.Before = append([]tailoringbundle.StageAction{}, value.Wrapper.Before...)
		copy.Invoke.OptionDefaults = cloneOptionDefaults(value.Wrapper.Invoke.OptionDefaults)
		copy.Invoke.AppendArgs = append([]string{}, value.Wrapper.Invoke.AppendArgs...)
		copy.Output = cloneOutput(value.Wrapper.Output)
		copy.After = append([]tailoringbundle.StageAction{}, value.Wrapper.After...)
		result.Wrapper = &copy
	}
	return result
}

func hasPrefix(values, prefix []string) bool {
	return len(values) >= len(prefix) && reflect.DeepEqual(values[:len(prefix)], prefix)
}

func validDigest(value string) bool {
	if len(value) != 64 {
		return false
	}
	for _, r := range value {
		if !((r >= '0' && r <= '9') || (r >= 'a' && r <= 'f')) {
			return false
		}
	}
	return true
}

func validStableName(value string) bool {
	if value == "" || len(value) > 128 {
		return false
	}
	for index, r := range value {
		if (r >= 'a' && r <= 'z') || (index > 0 && r >= '0' && r <= '9') || (index > 0 && (r == '-' || r == '_')) {
			continue
		}
		return false
	}
	return true
}

func validNamespaced(value string) bool {
	parts := strings.Split(value, ".")
	if len(parts) < 3 || len(value) > 128 {
		return false
	}
	for _, part := range parts {
		if !validStableName(part) {
			return false
		}
	}
	return true
}

func sortedUnique(values []string) bool {
	return sort.SliceIsSorted(values, func(i, j int) bool { return values[i] < values[j] }) && func() bool {
		for index := 1; index < len(values); index++ {
			if values[index] == values[index-1] {
				return false
			}
		}
		return true
	}()
}

func validateText(value string, limit int) error {
	if value == "" || len(value) > limit || !utf8.ValidString(value) {
		return fmt.Errorf("must be non-empty bounded UTF-8")
	}
	if strings.IndexFunc(value, func(r rune) bool {
		return unicode.IsControl(r) || unicode.Is(unicode.Cf, r) || r == '\u2028' || r == '\u2029'
	}) >= 0 {
		return fmt.Errorf("contains unsupported structural text")
	}
	return nil
}

func invalidInvocation(format string, args ...any) error {
	return fmt.Errorf("%w: %s", ErrInvalidInvocation, fmt.Sprintf(format, args...))
}

func invalidPlan(format string, args ...any) error {
	return fmt.Errorf("%w: %s", ErrInvalidPlan, fmt.Sprintf(format, args...))
}
