// Package tailoringbundle defines the canonical vendor-neutral tailoring
// specification, compiled surface, and wrapper vocabulary.
package tailoringbundle

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
	"github.com/tasuku43/atsura/internal/domain/tailoring"
)

const (
	SpecificationSchemaVersion = 3
	BundleSchemaVersion        = 2
	MaxCommandEntries          = 256
	MaxWrapperArguments        = 64
)

var (
	ErrInvalidSpecification = errors.New("invalid tailoring specification")
	ErrInvalidBundle        = errors.New("invalid tailoring bundle")
	ErrCommandNotInSurface  = errors.New("command not in tailored surface")
)

// SurfaceDefault determines membership for verified catalog commands without
// an explicit command entry.
type SurfaceDefault string

const (
	SurfaceDefaultInherit SurfaceDefault = "inherit"
	SurfaceDefaultExclude SurfaceDefault = "exclude"
)

// Presence changes membership for one exact verified catalog command.
type Presence string

const (
	PresenceInclude Presence = "include"
	PresenceExclude Presence = "exclude"
)

// WrapperKind distinguishes a source-preserving wrapper from one with at
// least one declared transformation.
type WrapperKind string

const (
	WrapperIdentity  WrapperKind = "identity"
	WrapperTransform WrapperKind = "transform"
)

// Surface declares the membership default for otherwise unlisted commands.
type Surface struct {
	Default SurfaceDefault `json:"default"`
}

// OptionSurface declares option membership independently from command
// membership and wrapper behavior.
type OptionSurface struct {
	Default SurfaceDefault `json:"default"`
	Include []string       `json:"include"`
	Exclude []string       `json:"exclude"`
}

// StageAction reserves a typed stage boundary. Schema 3 requires before and
// after to be explicit empty lists until a built-in action is accepted.
type StageAction struct {
	Kind string `json:"kind"`
}

// Invocation is the normalized source-argv transformation vocabulary.
type Invocation struct {
	AppendArgs []string `json:"append_args"`
}

// Rename changes one selected source field in the agent-facing result.
type Rename struct {
	From string `json:"from"`
	To   string `json:"to"`
}

// Output is the normalized built-in structured transformation contract.
type Output struct {
	Input  string   `json:"input"`
	Select []string `json:"select"`
	Rename []Rename `json:"rename"`
	Render string   `json:"render"`
}

// Wrapper describes the ordered typed stages applied to an included command.
type Wrapper struct {
	Kind   WrapperKind   `json:"kind"`
	Before []StageAction `json:"before"`
	Invoke Invocation    `json:"invoke"`
	Output *Output       `json:"output,omitempty"`
	After  []StageAction `json:"after"`
}

// CommandEntry independently declares membership, option surface, and wrapper
// behavior for one exact verified catalog command.
type CommandEntry struct {
	Command  []string       `json:"command"`
	Presence Presence       `json:"presence"`
	Reason   string         `json:"reason"`
	Options  *OptionSurface `json:"options,omitempty"`
	Wrapper  *Wrapper       `json:"wrapper,omitempty"`
}

// Specification is normalized schema-3 content bound to one exact catalog.
type Specification struct {
	SchemaVersion int            `json:"schema_version"`
	CatalogDigest string         `json:"catalog_digest"`
	Surface       Surface        `json:"surface"`
	Commands      []CommandEntry `json:"commands"`
}

// SurfaceEntry is one included command in the compiled purpose-specific CLI.
// Excluded commands are absent instead of carrying a denial decision.
type SurfaceEntry struct {
	Command []string      `json:"command"`
	Reason  string        `json:"reason"`
	Options OptionSurface `json:"options"`
	Wrapper Wrapper       `json:"wrapper"`
}

// Bundle is the canonical compiled surface and wrapper artifact.
type Bundle struct {
	SchemaVersion       int                   `json:"schema_version"`
	CatalogDigest       string                `json:"catalog_digest"`
	Catalog             sourcecatalog.Catalog `json:"catalog"`
	SpecificationDigest string                `json:"specification_digest"`
	Specification       Specification         `json:"specification"`
	Surface             []SurfaceEntry        `json:"surface"`
}

// Compile validates and binds catalog, specification, and tailored surface.
func Compile(catalog sourcecatalog.Catalog, specification Specification) (Bundle, error) {
	catalogDigest, err := catalog.Digest()
	if err != nil {
		return Bundle{}, invalidBundle("catalog: %v", err)
	}
	if err := specification.Validate(catalog); err != nil {
		return Bundle{}, err
	}
	if specification.CatalogDigest != catalogDigest {
		return Bundle{}, invalidSpecification("catalog digest does not match the supplied catalog")
	}
	specificationDigest, err := specification.Digest(catalog)
	if err != nil {
		return Bundle{}, err
	}
	return Bundle{
		SchemaVersion:       BundleSchemaVersion,
		CatalogDigest:       catalogDigest,
		Catalog:             catalog,
		SpecificationDigest: specificationDigest,
		Specification:       specification,
		Surface:             deriveSurface(catalog, specification),
	}, nil
}

// Validate proves every digest and derived value rather than trusting stored
// bundle metadata.
func (b Bundle) Validate() error {
	if b.SchemaVersion != BundleSchemaVersion {
		return invalidBundle("schema_version must be %d", BundleSchemaVersion)
	}
	catalogDigest, err := b.Catalog.Digest()
	if err != nil || catalogDigest != b.CatalogDigest {
		return invalidBundle("catalog digest is invalid or mismatched")
	}
	if err := b.Specification.Validate(b.Catalog); err != nil {
		return invalidBundle("specification: %v", err)
	}
	specificationDigest, err := b.Specification.Digest(b.Catalog)
	if err != nil || specificationDigest != b.SpecificationDigest {
		return invalidBundle("specification digest is invalid or mismatched")
	}
	if b.Specification.CatalogDigest != b.CatalogDigest {
		return invalidBundle("specification is bound to a different catalog")
	}
	if !reflect.DeepEqual(b.Surface, deriveSurface(b.Catalog, b.Specification)) {
		return invalidBundle("tailored surface is not the specification-derived projection")
	}
	return nil
}

// Resolve returns one detached included surface entry. Absence is a surface
// fact, not an authorization decision.
func (b Bundle) Resolve(command []string) (SurfaceEntry, error) {
	if err := b.Validate(); err != nil {
		return SurfaceEntry{}, err
	}
	key := strings.Join(command, " ")
	for _, entry := range b.Surface {
		if strings.Join(entry.Command, " ") == key {
			return cloneSurfaceEntry(entry), nil
		}
	}
	return SurfaceEntry{}, fmt.Errorf("%w: %q", ErrCommandNotInSurface, key)
}

// Validate rejects ambiguous, unbounded, unsupported, or non-canonical
// specification content.
func (s Specification) Validate(catalog sourcecatalog.Catalog) error {
	if s.SchemaVersion != SpecificationSchemaVersion {
		return invalidSpecification("schema_version must be %d", SpecificationSchemaVersion)
	}
	if len(s.CatalogDigest) != 64 || s.Commands == nil || len(s.Commands) > MaxCommandEntries {
		return invalidSpecification("catalog digest and an explicit bounded commands list are required")
	}
	if !validSurfaceDefault(s.Surface.Default) {
		return invalidSpecification("surface default must be inherit or exclude")
	}
	wantedDigest, err := catalog.Digest()
	if err != nil || wantedDigest != s.CatalogDigest {
		return invalidSpecification("catalog digest does not match the validated catalog")
	}
	commands := make(map[string]sourcecatalog.Command, len(catalog.Commands))
	for _, command := range catalog.Commands {
		commands[strings.Join(command.Path, " ")] = command
	}
	previous := ""
	for index, entry := range s.Commands {
		if err := entry.validate(commands); err != nil {
			return invalidSpecification("command %d: %v", index, err)
		}
		key := strings.Join(entry.Command, " ")
		if previous != "" && key <= previous {
			return invalidSpecification("commands must be sorted and unique by command")
		}
		previous = key
	}
	return nil
}

func (e CommandEntry) validate(commands map[string]sourcecatalog.Command) error {
	if len(e.Command) == 0 || len(e.Command) > sourcecatalog.MaxCommandSegments {
		return fmt.Errorf("command must be a non-empty bounded path")
	}
	key := strings.Join(e.Command, " ")
	command, exists := commands[key]
	if !exists || command.Provenance != sourcecatalog.ProvenanceVerifiedBuiltin {
		return fmt.Errorf("command %q is not verified catalog evidence", key)
	}
	if err := validateText(e.Reason, 4096); err != nil {
		return fmt.Errorf("reason: %v", err)
	}
	switch e.Presence {
	case PresenceExclude:
		if e.Options != nil || e.Wrapper != nil {
			return fmt.Errorf("excluded commands must not declare options or a wrapper")
		}
	case PresenceInclude:
		if e.Options == nil || e.Wrapper == nil {
			return fmt.Errorf("included commands require explicit options and a wrapper")
		}
		if err := e.Options.validate(command); err != nil {
			return err
		}
		if err := e.Wrapper.validate(command); err != nil {
			return err
		}
	default:
		return fmt.Errorf("presence must be include or exclude")
	}
	return nil
}

func (o OptionSurface) validate(command sourcecatalog.Command) error {
	if !validSurfaceDefault(o.Default) {
		return fmt.Errorf("option default must be inherit or exclude")
	}
	if o.Include == nil || o.Exclude == nil || len(o.Include) > sourcecatalog.MaxOptions || len(o.Exclude) > sourcecatalog.MaxOptions {
		return fmt.Errorf("option include and exclude must be explicit bounded lists")
	}
	if !sortedUnique(o.Include) || !sortedUnique(o.Exclude) {
		return fmt.Errorf("option overrides must be sorted and unique")
	}
	if o.Default == SurfaceDefaultInherit && len(o.Include) != 0 {
		return fmt.Errorf("inherited options may only declare exclusions")
	}
	if o.Default == SurfaceDefaultExclude && len(o.Exclude) != 0 {
		return fmt.Errorf("excluded-by-default options may only declare inclusions")
	}
	observed := make(map[string]struct{}, len(command.Options))
	for _, option := range command.Options {
		observed[option.Name] = struct{}{}
	}
	seen := make(map[string]struct{}, len(o.Include)+len(o.Exclude))
	for _, values := range [][]string{o.Include, o.Exclude} {
		for _, option := range values {
			if _, exists := observed[option]; !exists {
				return fmt.Errorf("option %q is not observed for command", option)
			}
			if _, duplicate := seen[option]; duplicate {
				return fmt.Errorf("option %q is both included and excluded", option)
			}
			seen[option] = struct{}{}
		}
	}
	return nil
}

func (w Wrapper) validate(command sourcecatalog.Command) error {
	if w.Before == nil || w.After == nil || w.Invoke.AppendArgs == nil {
		return fmt.Errorf("wrapper before, invoke append_args, and after must be explicit lists")
	}
	if len(w.Before) != 0 || len(w.After) != 0 {
		return fmt.Errorf("schema 3 does not support before or after actions")
	}
	if len(w.Invoke.AppendArgs) > MaxWrapperArguments {
		return fmt.Errorf("invoke append_args exceeds its bound")
	}
	for _, argument := range w.Invoke.AppendArgs {
		if err := validateArgument(argument); err != nil {
			return fmt.Errorf("append argument: %v", err)
		}
	}
	switch w.Kind {
	case WrapperIdentity:
		if len(w.Invoke.AppendArgs) != 0 || w.Output != nil {
			return fmt.Errorf("identity wrapper must not transform invocation or output")
		}
	case WrapperTransform:
		if len(w.Invoke.AppendArgs) == 0 && w.Output == nil {
			return fmt.Errorf("transform wrapper requires at least one supported transform")
		}
		if w.Output != nil {
			if err := w.Output.validate(command); err != nil {
				return err
			}
		}
	default:
		return fmt.Errorf("wrapper kind must be identity or transform")
	}
	return nil
}

func (o Output) validate(command sourcecatalog.Command) error {
	renames := make([]tailoring.Rename, len(o.Rename))
	for index, rename := range o.Rename {
		renames[index] = tailoring.Rename{From: rename.From, To: rename.To}
	}
	plan := tailoring.OutputPlan{Input: tailoring.InputFormat(o.Input), Select: o.Select, Rename: renames, Render: tailoring.RenderFormat(o.Render)}
	if err := plan.Validate(); err != nil {
		return fmt.Errorf("output: %v", err)
	}
	formatObserved := false
	for _, output := range command.StructuredOutput {
		if output.Format != o.Input {
			continue
		}
		formatObserved = true
		observed := make(map[string]struct{}, len(output.Fields))
		for _, field := range output.Fields {
			observed[field] = struct{}{}
		}
		allObserved := true
		for _, field := range o.Select {
			if _, exists := observed[field]; !exists {
				allObserved = false
				break
			}
		}
		if allObserved {
			return nil
		}
	}
	if !formatObserved {
		return fmt.Errorf("output requests %s not observed for command", o.Input)
	}
	return fmt.Errorf("one or more selected output fields were not observed together for command")
}

// CanonicalJSON encodes the normalized specification only after catalog-bound
// validation.
func (s Specification) CanonicalJSON(catalog sourcecatalog.Catalog) ([]byte, error) {
	if err := s.Validate(catalog); err != nil {
		return nil, err
	}
	encoded, err := json.Marshal(s)
	if err != nil {
		return nil, fmt.Errorf("encode canonical specification: %w", err)
	}
	return append(encoded, '\n'), nil
}

func (s Specification) Digest(catalog sourcecatalog.Catalog) (string, error) {
	encoded, err := s.CanonicalJSON(catalog)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%x", sha256.Sum256(encoded)), nil
}

func (b Bundle) CanonicalJSON() ([]byte, error) {
	if err := b.Validate(); err != nil {
		return nil, err
	}
	encoded, err := json.Marshal(b)
	if err != nil {
		return nil, fmt.Errorf("encode canonical bundle: %w", err)
	}
	return append(encoded, '\n'), nil
}

func (b Bundle) Digest() (string, error) {
	encoded, err := b.CanonicalJSON()
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%x", sha256.Sum256(encoded)), nil
}

// SortSpecification detaches and canonicalizes command and option-set order.
// Ordered transformations such as append_args, select, and rename retain
// their declared order.
func SortSpecification(s Specification) Specification {
	result := s
	result.Commands = cloneCommandEntries(s.Commands)
	for index := range result.Commands {
		entry := &result.Commands[index]
		entry.Command = cloneStrings(entry.Command)
		if entry.Options != nil {
			copy := *entry.Options
			copy.Include = cloneStrings(copy.Include)
			copy.Exclude = cloneStrings(copy.Exclude)
			sort.Strings(copy.Include)
			sort.Strings(copy.Exclude)
			entry.Options = &copy
		}
		if entry.Wrapper != nil {
			copy := cloneWrapper(*entry.Wrapper)
			entry.Wrapper = &copy
		}
	}
	sort.Slice(result.Commands, func(i, j int) bool {
		return strings.Join(result.Commands[i].Command, " ") < strings.Join(result.Commands[j].Command, " ")
	})
	return result
}

func deriveSurface(catalog sourcecatalog.Catalog, specification Specification) []SurfaceEntry {
	explicit := make(map[string]CommandEntry, len(specification.Commands))
	for _, entry := range specification.Commands {
		explicit[strings.Join(entry.Command, " ")] = entry
	}
	result := make([]SurfaceEntry, 0, len(catalog.Commands))
	for _, command := range catalog.Commands {
		if command.Provenance != sourcecatalog.ProvenanceVerifiedBuiltin {
			continue
		}
		entry, exists := explicit[strings.Join(command.Path, " ")]
		if exists {
			if entry.Presence == PresenceInclude {
				result = append(result, SurfaceEntry{Command: append([]string(nil), entry.Command...), Reason: entry.Reason, Options: cloneOptionSurface(*entry.Options), Wrapper: cloneWrapper(*entry.Wrapper)})
			}
			continue
		}
		if specification.Surface.Default == SurfaceDefaultInherit {
			result = append(result, SurfaceEntry{
				Command: append([]string(nil), command.Path...),
				Reason:  "Inherited from the source catalog by surface.default.",
				Options: OptionSurface{Default: SurfaceDefaultInherit, Include: []string{}, Exclude: []string{}},
				Wrapper: Wrapper{Kind: WrapperIdentity, Before: []StageAction{}, Invoke: Invocation{AppendArgs: []string{}}, After: []StageAction{}},
			})
		}
	}
	return result
}

func cloneSurfaceEntry(entry SurfaceEntry) SurfaceEntry {
	return SurfaceEntry{Command: append([]string(nil), entry.Command...), Reason: entry.Reason, Options: cloneOptionSurface(entry.Options), Wrapper: cloneWrapper(entry.Wrapper)}
}

func cloneOptionSurface(value OptionSurface) OptionSurface {
	return OptionSurface{Default: value.Default, Include: cloneStrings(value.Include), Exclude: cloneStrings(value.Exclude)}
}

func cloneWrapper(value Wrapper) Wrapper {
	result := value
	result.Before = cloneStageActions(value.Before)
	result.Invoke.AppendArgs = cloneStrings(value.Invoke.AppendArgs)
	result.After = cloneStageActions(value.After)
	if value.Output != nil {
		copy := *value.Output
		copy.Select = cloneStrings(value.Output.Select)
		copy.Rename = cloneRenames(value.Output.Rename)
		result.Output = &copy
	}
	return result
}

func cloneCommandEntries(values []CommandEntry) []CommandEntry {
	if values == nil {
		return nil
	}
	return append([]CommandEntry{}, values...)
}

func cloneStrings(values []string) []string {
	if values == nil {
		return nil
	}
	return append([]string{}, values...)
}

func cloneStageActions(values []StageAction) []StageAction {
	if values == nil {
		return nil
	}
	return append([]StageAction{}, values...)
}

func cloneRenames(values []Rename) []Rename {
	if values == nil {
		return nil
	}
	return append([]Rename{}, values...)
}

func validSurfaceDefault(value SurfaceDefault) bool {
	return value == SurfaceDefaultInherit || value == SurfaceDefaultExclude
}

func sortedUnique(values []string) bool {
	for index := range values {
		if index > 0 && values[index] <= values[index-1] {
			return false
		}
	}
	return true
}

func validateText(value string, limit int) error {
	if value == "" || len(value) > limit || !utf8.ValidString(value) {
		return fmt.Errorf("must be non-empty bounded UTF-8")
	}
	return validateStructuralText(value)
}

func validateArgument(value string) error {
	if len(value) > 4096 || !utf8.ValidString(value) {
		return fmt.Errorf("must be bounded UTF-8")
	}
	return validateStructuralText(value)
}

func validateStructuralText(value string) error {
	if strings.IndexFunc(value, func(r rune) bool {
		return unicode.IsControl(r) || unicode.Is(unicode.Cf, r) || r == '\u2028' || r == '\u2029'
	}) >= 0 {
		return fmt.Errorf("contains unsupported structural text")
	}
	return nil
}

func invalidSpecification(format string, args ...any) error {
	return fmt.Errorf("%w: %s", ErrInvalidSpecification, fmt.Sprintf(format, args...))
}

func invalidBundle(format string, args ...any) error {
	return fmt.Errorf("%w: %s", ErrInvalidBundle, fmt.Sprintf(format, args...))
}
