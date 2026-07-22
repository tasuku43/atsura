// Package sourcecatalog defines vendor-neutral evidence about one installed
// source CLI. A catalog describes observed capability; it never grants policy.
package sourcecatalog

import (
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/tasuku43/atsura/internal/domain/sourceprocess"
)

const (
	SchemaVersion      = 2
	MaxCommands        = 512
	MaxCommandSegments = 12
	MaxOptions         = 128
	MaxTextBytes       = 4096
)

var (
	ErrInvalidCatalog     = errors.New("invalid source catalog")
	ErrUnsupportedAdapter = errors.New("unsupported source adapter")
	ErrUnsupportedVersion = errors.New("unsupported source version")
	ErrInspectionFailed   = errors.New("source inspection failed")
)

// Provenance states how strongly a source adapter can account for an entry.
type Provenance string

const (
	ProvenanceVerifiedBuiltin   Provenance = "verified_builtin"
	ProvenanceObservedExtension Provenance = "observed_extension"
	ProvenanceUnverifiedDynamic Provenance = "unverified_dynamic"
)

// Adapter identifies one bounded source-inspection contract. Kind is an
// opaque namespaced value; shared semantics never branch on a vendor name.
type Adapter struct {
	Kind            string `json:"kind"`
	ContractVersion int    `json:"contract_version"`
}

// Source is exact executable and version evidence used by the inspection.
type Source struct {
	RequestedExecutable string `json:"requested_executable"`
	ResolvedPath        string `json:"resolved_path"`
	SHA256              string `json:"sha256"`
	Size                int64  `json:"size"`
	Version             string `json:"version"`
}

// Option describes one observed long option without assigning policy meaning.
type Option struct {
	Name       string `json:"name"`
	TakesValue bool   `json:"takes_value"`
}

// StructuredOutput describes a source-native structured-output selector.
type StructuredOutput struct {
	Format       string   `json:"format"`
	SelectorFlag string   `json:"selector_flag"`
	Fields       []string `json:"fields"`
}

// Command is one source command or namespace observation.
type Command struct {
	Path             []string           `json:"path"`
	Summary          string             `json:"summary"`
	Provenance       Provenance         `json:"provenance"`
	Options          []Option           `json:"options"`
	StructuredOutput []StructuredOutput `json:"structured_output"`
}

// Probe records bounded inspection facts without persisting raw probe output.
type Probe struct {
	IDs      []string `json:"ids"`
	Attempts int      `json:"attempts"`
}

// Catalog is the vendor-neutral, provenance-bearing inspection result.
type Catalog struct {
	SchemaVersion int       `json:"schema_version"`
	Adapter       Adapter   `json:"adapter"`
	Source        Source    `json:"source"`
	Probe         Probe     `json:"probe"`
	Commands      []Command `json:"commands"`
}

// Validate rejects incomplete, ambiguous, unbounded, or non-canonical facts.
func (c Catalog) Validate() error {
	if c.SchemaVersion != SchemaVersion {
		return invalid("schema_version must be %d", SchemaVersion)
	}
	if !validNamespaced(c.Adapter.Kind) || c.Adapter.ContractVersion <= 0 {
		return invalid("adapter kind must be namespaced and contract_version positive")
	}
	if err := validateText(c.Source.RequestedExecutable, 256); err != nil {
		return invalid("requested executable: %v", err)
	}
	identity := sourceprocess.Identity{ResolvedPath: c.Source.ResolvedPath, SHA256: c.Source.SHA256, Size: c.Source.Size}
	if err := identity.Validate(); err != nil {
		return invalid("source identity: %v", err)
	}
	if filepath.Clean(c.Source.ResolvedPath) != c.Source.ResolvedPath {
		return invalid("source resolved path must be clean")
	}
	if err := validateText(c.Source.Version, 256); err != nil {
		return invalid("source version: %v", err)
	}
	if c.Probe.IDs == nil || len(c.Probe.IDs) == 0 || len(c.Probe.IDs) > 16 || c.Probe.Attempts != len(c.Probe.IDs) {
		return invalid("probe ids and attempts must describe one bounded attempt per id")
	}
	if !sortedUnique(c.Probe.IDs) {
		return invalid("probe ids must be sorted and unique")
	}
	for _, id := range c.Probe.IDs {
		if !validStableName(id) {
			return invalid("probe id %q is invalid", id)
		}
	}
	if c.Commands == nil || len(c.Commands) == 0 || len(c.Commands) > MaxCommands {
		return invalid("commands must be a non-empty bounded list")
	}
	previous := ""
	for index, command := range c.Commands {
		if err := command.validate(); err != nil {
			return invalid("command %d: %v", index, err)
		}
		key := strings.Join(command.Path, " ")
		if previous != "" && key <= previous {
			return invalid("command paths must be sorted and unique")
		}
		previous = key
	}
	return nil
}

func (c Command) validate() error {
	if len(c.Path) == 0 || len(c.Path) > MaxCommandSegments {
		return fmt.Errorf("path must be a non-empty bounded list")
	}
	for _, segment := range c.Path {
		if !validStableName(segment) {
			return fmt.Errorf("path segment %q is invalid", segment)
		}
	}
	if err := validateText(c.Summary, MaxTextBytes); err != nil {
		return fmt.Errorf("summary: %v", err)
	}
	switch c.Provenance {
	case ProvenanceVerifiedBuiltin, ProvenanceObservedExtension, ProvenanceUnverifiedDynamic:
	default:
		return fmt.Errorf("provenance %q is invalid", c.Provenance)
	}
	if c.Options == nil || len(c.Options) > MaxOptions {
		return fmt.Errorf("options must be an explicit bounded list")
	}
	previous := ""
	for _, option := range c.Options {
		if !strings.HasPrefix(option.Name, "--") || !validStableName(strings.TrimPrefix(option.Name, "--")) {
			return fmt.Errorf("option %q is invalid", option.Name)
		}
		if option.Name <= previous {
			return fmt.Errorf("options must be sorted and unique")
		}
		previous = option.Name
	}
	if c.StructuredOutput == nil || len(c.StructuredOutput) > 8 {
		return fmt.Errorf("structured_output must be an explicit bounded list")
	}
	previous = ""
	for _, output := range c.StructuredOutput {
		if !validStableName(output.Format) || !validSelectorFlag(output.SelectorFlag) {
			return fmt.Errorf("structured output selector is invalid")
		}
		key := output.Format + "\x00" + output.SelectorFlag
		if key <= previous || output.Fields == nil || len(output.Fields) > 256 || !sortedUnique(output.Fields) {
			return fmt.Errorf("structured output entries and fields must be sorted, unique, and bounded")
		}
		for _, field := range output.Fields {
			if err := validateText(field, 128); err != nil {
				return fmt.Errorf("structured output field: %v", err)
			}
		}
		previous = key
	}
	return nil
}

// CanonicalJSON returns the sole byte representation accepted for digesting.
// Callers must construct a validated, already sorted catalog.
func (c Catalog) CanonicalJSON() ([]byte, error) {
	if err := c.Validate(); err != nil {
		return nil, err
	}
	encoded, err := json.Marshal(c)
	if err != nil {
		return nil, fmt.Errorf("encode canonical catalog: %w", err)
	}
	return append(encoded, '\n'), nil
}

// Digest returns the lowercase SHA-256 of CanonicalJSON.
func (c Catalog) Digest() (string, error) {
	encoded, err := c.CanonicalJSON()
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%x", sha256.Sum256(encoded)), nil
}

// Sort prepares adapter-produced commands and nested observations for strict
// canonical validation. It does not repair invalid values.
func Sort(c Catalog) Catalog {
	result := c
	result.Probe.IDs = append(make([]string, 0, len(c.Probe.IDs)), c.Probe.IDs...)
	sort.Strings(result.Probe.IDs)
	result.Commands = append([]Command(nil), c.Commands...)
	for index := range result.Commands {
		command := &result.Commands[index]
		command.Path = append([]string(nil), command.Path...)
		command.Options = append(make([]Option, 0, len(command.Options)), command.Options...)
		sort.Slice(command.Options, func(i, j int) bool { return command.Options[i].Name < command.Options[j].Name })
		command.StructuredOutput = append(make([]StructuredOutput, 0, len(command.StructuredOutput)), command.StructuredOutput...)
		for outputIndex := range command.StructuredOutput {
			fields := append(make([]string, 0, len(command.StructuredOutput[outputIndex].Fields)), command.StructuredOutput[outputIndex].Fields...)
			sort.Strings(fields)
			command.StructuredOutput[outputIndex].Fields = fields
		}
		sort.Slice(command.StructuredOutput, func(i, j int) bool {
			left, right := command.StructuredOutput[i], command.StructuredOutput[j]
			return left.Format+"\x00"+left.SelectorFlag < right.Format+"\x00"+right.SelectorFlag
		})
	}
	sort.Slice(result.Commands, func(i, j int) bool {
		return strings.Join(result.Commands[i].Path, " ") < strings.Join(result.Commands[j].Path, " ")
	})
	return result
}

func validNamespaced(value string) bool {
	parts := strings.Split(value, ".")
	if len(parts) < 3 {
		return false
	}
	for _, part := range parts {
		if !validStableName(part) {
			return false
		}
	}
	return len(value) <= 128
}

func validSelectorFlag(value string) bool {
	name := ""
	switch {
	case strings.HasPrefix(value, "---"):
		return false
	case strings.HasPrefix(value, "--"):
		name = strings.TrimPrefix(value, "--")
	case strings.HasPrefix(value, "-"):
		name = strings.TrimPrefix(value, "-")
	default:
		return false
	}
	return !strings.ContainsRune(name, '=') && validStableName(name)
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

func sortedUnique(values []string) bool {
	for index := range values {
		if index > 0 && values[index] <= values[index-1] {
			return false
		}
	}
	return true
}

func invalid(format string, args ...any) error {
	return fmt.Errorf("%w: %s", ErrInvalidCatalog, fmt.Sprintf(format, args...))
}
