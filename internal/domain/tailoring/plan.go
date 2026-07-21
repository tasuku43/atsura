// Package tailoring defines the pure policy and execution-plan vocabulary used
// to tailor one modeled source command without performing source I/O.
package tailoring

import (
	"errors"
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"
)

const (
	maxArgumentBytes = 4096
	maxArguments     = 64
	maxReasonBytes   = 4096
	maxOutputFields  = 64
)

var (
	ErrInvalidPolicy     = errors.New("invalid tailoring policy")
	ErrInvalidInvocation = errors.New("invalid attempted invocation")
	ErrNoMatch           = errors.New("tailoring policy does not match the attempted invocation")
)

// Decision states whether a matched source invocation may produce executable
// transformed argv. Confirmation is deliberately outside the first slice.
type Decision string

const (
	DecisionAllow Decision = "allow"
	DecisionDeny  Decision = "deny"
)

// InputFormat describes the structured source output expected by a plan.
type InputFormat string

const InputJSON InputFormat = "json"

// RenderFormat describes the built-in agent-facing representation.
type RenderFormat string

const RenderCompactJSON RenderFormat = "compact_json"

// Rename changes one selected source field name in the planned output shape.
type Rename struct {
	From string
	To   string
}

// OutputPlan is the first typed built-in output transformation vocabulary.
type OutputPlan struct {
	Input  InputFormat
	Select []string
	Rename []Rename
	Render RenderFormat
}

// Policy is one validated per-command policy decoded from schema-1 YAML.
type Policy struct {
	SchemaVersion int
	Executable    string
	ArgsPrefix    []string
	Decision      Decision
	Reason        string
	AppendArgs    []string
	Output        OutputPlan
}

// Invocation is the exact executable and argv sequence attempted by a caller.
// Argv includes the executable as its first element.
type Invocation struct {
	Argv []string
}

// Plan is a detached deterministic preview. SourceProcessAttempts is always
// zero because this package has no execution boundary.
type Plan struct {
	Decision              Decision
	Executable            bool
	SourceExecutable      string
	MatchedCommand        string
	OriginalArgv          []string
	TransformedArgv       []string
	Reason                string
	Output                OutputPlan
	SourceProcessAttempts int
}

// Validate proves that a policy is complete, bounded, and internally
// consistent before it can participate in matching or plan construction.
func (p Policy) Validate() error {
	if p.SchemaVersion != 1 {
		return invalidPolicy("schema_version must be 1")
	}
	if err := validateArgument(p.Executable); err != nil {
		return invalidPolicy("command executable: %v", err)
	}
	if p.ArgsPrefix == nil || len(p.ArgsPrefix) > maxArguments {
		return invalidPolicy("command args_prefix must be an explicit bounded list")
	}
	for index, argument := range p.ArgsPrefix {
		if err := validateArgument(argument); err != nil {
			return invalidPolicy("command args_prefix item %d: %v", index, err)
		}
	}
	if p.Decision != DecisionAllow && p.Decision != DecisionDeny {
		return invalidPolicy("decision must be allow or deny")
	}
	if p.Reason == "" || len(p.Reason) > maxReasonBytes || !utf8.ValidString(p.Reason) {
		return invalidPolicy("reason must be non-empty bounded UTF-8")
	}
	if p.AppendArgs == nil || len(p.AppendArgs) > maxArguments {
		return invalidPolicy("invoke append_args must be an explicit bounded list")
	}
	for index, argument := range p.AppendArgs {
		if err := validateArgument(argument); err != nil {
			return invalidPolicy("invoke append_args item %d: %v", index, err)
		}
	}
	if err := p.Output.validate(); err != nil {
		return err
	}
	return nil
}

func (o OutputPlan) validate() error {
	if o.Input != InputJSON {
		return invalidPolicy("output input must be json")
	}
	if o.Select == nil || len(o.Select) == 0 || len(o.Select) > maxOutputFields {
		return invalidPolicy("output select must be a non-empty bounded list")
	}
	selected := make(map[string]struct{}, len(o.Select))
	for _, field := range o.Select {
		if err := validateFieldName(field); err != nil {
			return invalidPolicy("output select field %q: %v", field, err)
		}
		if _, duplicate := selected[field]; duplicate {
			return invalidPolicy("output select field %q is duplicated", field)
		}
		selected[field] = struct{}{}
	}
	if o.Rename == nil || len(o.Rename) > maxOutputFields {
		return invalidPolicy("output rename must be an explicit bounded list")
	}
	renamedFrom := make(map[string]struct{}, len(o.Rename))
	finalNames := make(map[string]struct{}, len(o.Select))
	for _, field := range o.Select {
		finalNames[field] = struct{}{}
	}
	for _, rename := range o.Rename {
		if err := validateFieldName(rename.From); err != nil {
			return invalidPolicy("output rename source %q: %v", rename.From, err)
		}
		if err := validateFieldName(rename.To); err != nil {
			return invalidPolicy("output rename target %q: %v", rename.To, err)
		}
		if _, exists := selected[rename.From]; !exists {
			return invalidPolicy("output rename source %q is not selected", rename.From)
		}
		if _, duplicate := renamedFrom[rename.From]; duplicate {
			return invalidPolicy("output rename source %q is duplicated", rename.From)
		}
		delete(finalNames, rename.From)
		if _, collision := finalNames[rename.To]; collision {
			return invalidPolicy("output rename target %q collides with another result field", rename.To)
		}
		finalNames[rename.To] = struct{}{}
		renamedFrom[rename.From] = struct{}{}
	}
	if o.Render != RenderCompactJSON {
		return invalidPolicy("output render must be compact_json")
	}
	return nil
}

// Compile matches one attempted invocation and returns a detached preview.
// It performs no source discovery, process execution, or output transformation.
func Compile(policy Policy, invocation Invocation) (Plan, error) {
	if err := policy.Validate(); err != nil {
		return Plan{}, err
	}
	if err := invocation.validate(); err != nil {
		return Plan{}, err
	}
	if invocation.Argv[0] != policy.Executable || len(invocation.Argv)-1 < len(policy.ArgsPrefix) {
		return Plan{}, ErrNoMatch
	}
	for index, expected := range policy.ArgsPrefix {
		if invocation.Argv[index+1] != expected {
			return Plan{}, ErrNoMatch
		}
	}

	original := append([]string(nil), invocation.Argv...)
	transformed := make([]string, 0)
	executable := policy.Decision == DecisionAllow
	if executable {
		transformed = append(append([]string(nil), invocation.Argv...), policy.AppendArgs...)
	}
	matched := append([]string{policy.Executable}, policy.ArgsPrefix...)
	return Plan{
		Decision:              policy.Decision,
		Executable:            executable,
		SourceExecutable:      policy.Executable,
		MatchedCommand:        strings.Join(matched, " "),
		OriginalArgv:          original,
		TransformedArgv:       transformed,
		Reason:                policy.Reason,
		Output:                cloneOutputPlan(policy.Output),
		SourceProcessAttempts: 0,
	}, nil
}

func (i Invocation) validate() error {
	if i.Argv == nil || len(i.Argv) == 0 || len(i.Argv) > maxArguments {
		return fmt.Errorf("%w: command must contain a bounded executable and argv", ErrInvalidInvocation)
	}
	for index, argument := range i.Argv {
		if err := validateArgument(argument); err != nil {
			return fmt.Errorf("%w: argv item %d: %v", ErrInvalidInvocation, index, err)
		}
	}
	return nil
}

func validateArgument(value string) error {
	if value == "" || len(value) > maxArgumentBytes || !utf8.ValidString(value) {
		return fmt.Errorf("must be non-empty bounded UTF-8")
	}
	if strings.IndexFunc(value, func(r rune) bool {
		return unicode.IsControl(r) || unicode.Is(unicode.Cf, r) || r == '\u2028' || r == '\u2029'
	}) >= 0 {
		return fmt.Errorf("contains unsupported structural text")
	}
	return nil
}

func validateFieldName(value string) error {
	if value == "" || len(value) > 128 {
		return fmt.Errorf("must be non-empty and at most 128 bytes")
	}
	for index, r := range value {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || r == '_' ||
			(index > 0 && r >= '0' && r <= '9') || (index > 0 && (r == '-' || r == '.')) {
			continue
		}
		return fmt.Errorf("must use ASCII letters, digits, underscore, dot, or hyphen")
	}
	return nil
}

func cloneOutputPlan(value OutputPlan) OutputPlan {
	return OutputPlan{
		Input:  value.Input,
		Select: append([]string(nil), value.Select...),
		Rename: append([]Rename(nil), value.Rename...),
		Render: value.Render,
	}
}

func invalidPolicy(format string, args ...any) error {
	return fmt.Errorf("%w: %s", ErrInvalidPolicy, fmt.Sprintf(format, args...))
}
