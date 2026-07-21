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
	"unicode"
	"unicode/utf8"

	"github.com/tasuku43/atsura/internal/domain/sourcecatalog"
	"github.com/tasuku43/atsura/internal/domain/sourceprocess"
	"github.com/tasuku43/atsura/internal/domain/tailoring"
	"github.com/tasuku43/atsura/internal/domain/tailoringbundle"
)

const SchemaVersion = 2

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
	Executable       string   `json:"executable"`
	Args             []string `json:"args"`
	AppendedArgs     []string `json:"appended_args"`
	MaxAttempts      int      `json:"max_attempts"`
	TimeoutMillis    int64    `json:"timeout_millis"`
	StdoutLimitBytes int      `json:"stdout_limit_bytes"`
	StderrLimitBytes int      `json:"stderr_limit_bytes"`
}

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

// Plan is the canonical complete result shared by preview and future runtime.
type Plan struct {
	SchemaVersion       int                           `json:"schema_version"`
	Mode                Mode                          `json:"mode"`
	BundleDigest        string                        `json:"bundle_digest"`
	CatalogDigest       string                        `json:"catalog_digest"`
	SpecificationDigest string                        `json:"specification_digest"`
	Source              SourceIdentity                `json:"source"`
	MatchedCommand      []string                      `json:"matched_command"`
	SurfaceOrigin       SurfaceOrigin                 `json:"surface_origin"`
	SpecificationEntry  *tailoringbundle.CommandEntry `json:"specification_entry"`
	Reason              string                        `json:"reason"`
	Options             tailoringbundle.OptionSurface `json:"options"`
	WrapperKind         tailoringbundle.WrapperKind   `json:"wrapper_kind"`
	OriginalArgv        []string                      `json:"original_argv"`
	TransformedArgv     []string                      `json:"transformed_argv"`
	Stages              Stages                        `json:"stages"`
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

	transformedArgs := append([]string{}, attempt.Args...)
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
		Stages: Stages{
			Order:  []StageKind{StageBefore, StageInvoke, StageOutput, StageAfter},
			Before: append([]tailoringbundle.StageAction{}, entry.Wrapper.Before...),
			Invoke: Invocation{
				Executable:       bundle.Catalog.Source.ResolvedPath,
				Args:             append([]string{}, transformedArgs...),
				AppendedArgs:     append([]string{}, entry.Wrapper.Invoke.AppendArgs...),
				MaxAttempts:      1,
				TimeoutMillis:    sourceprocess.MaxTimeout.Milliseconds(),
				StdoutLimitBytes: sourceprocess.MaxStdoutBytes,
				StderrLimitBytes: sourceprocess.MaxStderrBytes,
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
	if p.Stages.Before == nil || p.Stages.After == nil || p.Stages.Invoke.Args == nil || p.Stages.Invoke.AppendedArgs == nil {
		return invalidPlan("stage lists must be explicit")
	}
	wantArgs := append([]string{}, p.OriginalArgv[1:]...)
	wantArgs = append(wantArgs, p.Stages.Invoke.AppendedArgs...)
	if !reflect.DeepEqual(wantArgs, p.Stages.Invoke.Args) {
		return invalidPlan("invoke args are not the exact original args plus appended args")
	}
	if p.Stages.Invoke.MaxAttempts != 1 || p.Stages.Invoke.TimeoutMillis != sourceprocess.MaxTimeout.Milliseconds() || p.Stages.Invoke.StdoutLimitBytes != sourceprocess.MaxStdoutBytes || p.Stages.Invoke.StderrLimitBytes != sourceprocess.MaxStderrBytes {
		return invalidPlan("source process bounds are invalid")
	}
	switch p.WrapperKind {
	case tailoringbundle.WrapperIdentity:
		if len(p.Stages.Before) != 0 || len(p.Stages.After) != 0 || len(p.Stages.Invoke.AppendedArgs) != 0 || p.Stages.Output != nil {
			return invalidPlan("identity wrapper contains a transformation")
		}
	case tailoringbundle.WrapperTransform:
		if len(p.Stages.Before) != 0 || len(p.Stages.After) != 0 || (len(p.Stages.Invoke.AppendedArgs) == 0 && p.Stages.Output == nil) {
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
			Invoke: tailoringbundle.Invocation{AppendArgs: append([]string{}, p.Stages.Invoke.AppendedArgs...)},
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

func validateOutputSelector(command sourcecatalog.Command, output tailoringbundle.Output, args []string) error {
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
		if !strings.HasPrefix(argument, "--") {
			continue
		}
		name, _, inline := strings.Cut(argument, "=")
		formats, isSelector := selectorFormats[name]
		if isSelector {
			if len(formats) != 1 {
				return invalidInvocation("structured-output selector %q does not identify one source format", name)
			}
			if _, wanted := formats[output.Input]; !wanted {
				return invalidInvocation("structured-output selector %q conflicts with planned input format %q", name, output.Input)
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
		return invalidInvocation("planned output format %q requires exactly one active cataloged selector before --; found %d", output.Input, matched)
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
	renames := make([]tailoring.Rename, len(value.Rename))
	for index, rename := range value.Rename {
		renames[index] = tailoring.Rename{From: rename.From, To: rename.To}
	}
	return (tailoring.OutputPlan{Input: tailoring.InputFormat(value.Input), Select: value.Select, Rename: renames, Render: tailoring.RenderFormat(value.Render)}).Validate()
}

func cloneOptions(value tailoringbundle.OptionSurface) tailoringbundle.OptionSurface {
	return tailoringbundle.OptionSurface{Default: value.Default, Include: append([]string{}, value.Include...), Exclude: append([]string{}, value.Exclude...)}
}

func cloneOutput(value *tailoringbundle.Output) *tailoringbundle.Output {
	if value == nil {
		return nil
	}
	copy := *value
	copy.Select = append([]string{}, value.Select...)
	copy.Rename = append([]tailoringbundle.Rename{}, value.Rename...)
	return &copy
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
