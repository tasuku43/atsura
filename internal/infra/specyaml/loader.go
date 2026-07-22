// Package specyaml decodes one bounded strict schema-5 tailoring
// specification file.
package specyaml

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"

	"github.com/tasuku43/atsura/internal/domain/fault"
	"github.com/tasuku43/atsura/internal/domain/tailoringbundle"
	"github.com/tasuku43/atsura/internal/infra/localfile"
	"go.yaml.in/yaml/v3"
)

const (
	maxSpecificationBytes = int64(256 * 1024)
	maxNodes              = 8192
	maxDepth              = 32
)

var ErrLegacyTailoringSchema = errors.New("legacy tailoring schema requires migration")

type Loader struct{}

func New() *Loader { return &Loader{} }

type document struct {
	SchemaVersion int       `yaml:"schema_version"`
	CatalogDigest string    `yaml:"catalog_digest"`
	Surface       surface   `yaml:"surface"`
	Commands      []command `yaml:"commands"`
}

type surface struct {
	Default string `yaml:"default"`
}

type command struct {
	Command  []string       `yaml:"command"`
	Presence string         `yaml:"presence"`
	Reason   string         `yaml:"reason"`
	Options  *optionSurface `yaml:"options,omitempty"`
	Wrapper  *wrapper       `yaml:"wrapper,omitempty"`
}

type optionSurface struct {
	Default string   `yaml:"default"`
	Include []string `yaml:"include"`
	Exclude []string `yaml:"exclude"`
}

type wrapper struct {
	Kind   string        `yaml:"kind"`
	Before []stageAction `yaml:"before"`
	Invoke invocation    `yaml:"invoke"`
	Output *output       `yaml:"output,omitempty"`
	After  []stageAction `yaml:"after"`
}

type stageAction struct {
	Kind string `yaml:"kind"`
}

type invocation struct {
	OptionDefaults []optionDefault `yaml:"option_defaults"`
	AppendArgs     []string        `yaml:"append_args"`
}

type optionDefault struct {
	Option strictString `yaml:"option"`
	Value  strictString `yaml:"value"`
}

// strictString prevents yaml.v3 from coercing booleans or numbers into the
// public string schema. A missing value remains empty and is rejected by the
// explicit option-default validation after decoding.
type strictString string

func (s *strictString) UnmarshalYAML(node *yaml.Node) error {
	if node == nil || node.Kind != yaml.ScalarNode || node.Tag != "!!str" {
		return fmt.Errorf("option default fields must be YAML strings")
	}
	*s = strictString(node.Value)
	return nil
}

func (s strictString) MarshalYAML() (any, error) {
	return string(s), nil
}

type output struct {
	Kind       string      `yaml:"kind"`
	Projection *projection `yaml:"projection,omitempty"`
	Optimizer  *optimizer  `yaml:"optimizer,omitempty"`
}

type projection struct {
	Input  string   `yaml:"input"`
	Select []string `yaml:"select"`
	Rename []rename `yaml:"rename"`
	Render string   `yaml:"render"`
}

type optimizer struct {
	Input               string `yaml:"input"`
	Contract            string `yaml:"contract"`
	AllowOriginalOutput bool   `yaml:"allow_original_output"`
}

type rename struct {
	From string `yaml:"from"`
	To   string `yaml:"to"`
}

func (l *Loader) Load(ctx context.Context, path string) (tailoringbundle.Specification, error) {
	raw, err := localfile.Read(ctx, path, maxSpecificationBytes)
	if err != nil {
		return tailoringbundle.Specification{}, fileFault(err)
	}
	parsed, err := decode(raw)
	if err != nil {
		if errors.Is(err, ErrLegacyTailoringSchema) {
			return tailoringbundle.Specification{}, fault.Wrap(
				fault.KindInvalidInput,
				"legacy_tailoring_schema",
				"Earlier tailoring schemas 1 through 4 are retired and cannot be converted automatically.",
				false,
				err,
				fault.NextAction{Command: "help spec init", Reason: "Create a schema-5 surface and wrapper specification from the catalog."},
			)
		}
		return tailoringbundle.Specification{}, fault.Wrap(fault.KindInvalidInput, "invalid_specification_yaml", "The schema-5 tailoring specification YAML is invalid.", false, err, helpAction())
	}
	return parsed, nil
}

// Encode renders one normalized specification as deterministic schema-5 YAML.
func Encode(specification tailoringbundle.Specification) ([]byte, error) {
	value := document{
		SchemaVersion: specification.SchemaVersion,
		CatalogDigest: specification.CatalogDigest,
		Surface:       surface{Default: string(specification.Surface.Default)},
		Commands:      make([]command, len(specification.Commands)),
	}
	for index, source := range specification.Commands {
		converted := command{
			Command: append(make([]string, 0, len(source.Command)), source.Command...), Presence: string(source.Presence), Reason: source.Reason,
		}
		if source.Options != nil {
			converted.Options = &optionSurface{
				Default: string(source.Options.Default),
				Include: append(make([]string, 0, len(source.Options.Include)), source.Options.Include...),
				Exclude: append(make([]string, 0, len(source.Options.Exclude)), source.Options.Exclude...),
			}
		}
		if source.Wrapper != nil {
			if source.Wrapper.Invoke.OptionDefaults == nil {
				return nil, fmt.Errorf("encode schema-5 tailoring specification YAML: invoke option_defaults must be an explicit list")
			}
			converted.Wrapper = encodeWrapper(*source.Wrapper)
		}
		value.Commands[index] = converted
	}
	encoded, err := yaml.Marshal(value)
	if err != nil {
		return nil, fmt.Errorf("encode schema-5 tailoring specification YAML: %w", err)
	}
	return encoded, nil
}

func encodeWrapper(source tailoringbundle.Wrapper) *wrapper {
	converted := &wrapper{
		Kind: string(source.Kind), Before: make([]stageAction, len(source.Before)),
		Invoke: invocation{
			OptionDefaults: make([]optionDefault, len(source.Invoke.OptionDefaults)),
			AppendArgs:     append(make([]string, 0, len(source.Invoke.AppendArgs)), source.Invoke.AppendArgs...),
		},
		After: make([]stageAction, len(source.After)),
	}
	for index, item := range source.Invoke.OptionDefaults {
		converted.Invoke.OptionDefaults[index] = optionDefault{Option: strictString(item.Option), Value: strictString(item.Value)}
	}
	for index, action := range source.Before {
		converted.Before[index] = stageAction{Kind: action.Kind}
	}
	for index, action := range source.After {
		converted.After[index] = stageAction{Kind: action.Kind}
	}
	if source.Output != nil {
		converted.Output = &output{Kind: string(source.Output.Kind)}
		if source.Output.Projection != nil {
			renames := make([]rename, len(source.Output.Projection.Rename))
			for index, item := range source.Output.Projection.Rename {
				renames[index] = rename{From: item.From, To: item.To}
			}
			converted.Output.Projection = &projection{
				Input: source.Output.Projection.Input, Select: append(make([]string, 0, len(source.Output.Projection.Select)), source.Output.Projection.Select...),
				Rename: renames, Render: source.Output.Projection.Render,
			}
		}
		if source.Output.Optimizer != nil {
			converted.Output.Optimizer = &optimizer{
				Input: source.Output.Optimizer.Input, Contract: source.Output.Optimizer.Contract,
				AllowOriginalOutput: source.Output.Optimizer.AllowOriginalOutput,
			}
		}
	}
	return converted
}

func decode(raw []byte) (tailoringbundle.Specification, error) {
	var root yaml.Node
	nodes := yaml.NewDecoder(bytes.NewReader(raw))
	if err := nodes.Decode(&root); err != nil {
		return tailoringbundle.Specification{}, fmt.Errorf("decode YAML document: %w", err)
	}
	var extra yaml.Node
	if err := nodes.Decode(&extra); !errors.Is(err, io.EOF) {
		return tailoringbundle.Specification{}, fmt.Errorf("exactly one YAML document is required")
	}
	count := 0
	if err := validateNode(&root, 0, &count); err != nil {
		return tailoringbundle.Specification{}, err
	}
	var header struct {
		SchemaVersion int `yaml:"schema_version"`
	}
	if err := root.Decode(&header); err != nil {
		return tailoringbundle.Specification{}, fmt.Errorf("decode schema header: %w", err)
	}
	if header.SchemaVersion >= 1 && header.SchemaVersion < tailoringbundle.SpecificationSchemaVersion {
		return tailoringbundle.Specification{}, fmt.Errorf("%w: schema_version %d", ErrLegacyTailoringSchema, header.SchemaVersion)
	}
	decoder := yaml.NewDecoder(bytes.NewReader(raw))
	decoder.KnownFields(true)
	var value document
	if err := decoder.Decode(&value); err != nil {
		return tailoringbundle.Specification{}, fmt.Errorf("decode strict tailoring specification: %w", err)
	}
	if err := decoder.Decode(&document{}); !errors.Is(err, io.EOF) {
		return tailoringbundle.Specification{}, fmt.Errorf("exactly one YAML document is required")
	}
	for index, entry := range value.Commands {
		if entry.Wrapper != nil && entry.Wrapper.Invoke.OptionDefaults == nil {
			return tailoringbundle.Specification{}, fmt.Errorf("commands[%d].wrapper.invoke.option_defaults must be an explicit list", index)
		}
		if entry.Wrapper != nil {
			for defaultIndex, item := range entry.Wrapper.Invoke.OptionDefaults {
				if item.Option == "" || item.Value == "" {
					return tailoringbundle.Specification{}, fmt.Errorf("commands[%d].wrapper.invoke.option_defaults[%d] requires non-empty option and value strings", index, defaultIndex)
				}
			}
		}
	}
	result := tailoringbundle.Specification{
		SchemaVersion: value.SchemaVersion,
		CatalogDigest: value.CatalogDigest,
		Surface:       tailoringbundle.Surface{Default: tailoringbundle.SurfaceDefault(value.Surface.Default)},
		Commands:      make([]tailoringbundle.CommandEntry, len(value.Commands)),
	}
	for index, source := range value.Commands {
		result.Commands[index] = convertCommand(source)
	}
	return tailoringbundle.SortSpecification(result), nil
}

func convertCommand(source command) tailoringbundle.CommandEntry {
	result := tailoringbundle.CommandEntry{
		Command: append(make([]string, 0, len(source.Command)), source.Command...), Presence: tailoringbundle.Presence(source.Presence), Reason: source.Reason,
	}
	if source.Options != nil {
		result.Options = &tailoringbundle.OptionSurface{
			Default: tailoringbundle.SurfaceDefault(source.Options.Default),
			Include: append(make([]string, 0, len(source.Options.Include)), source.Options.Include...),
			Exclude: append(make([]string, 0, len(source.Options.Exclude)), source.Options.Exclude...),
		}
	}
	if source.Wrapper != nil {
		before := make([]tailoringbundle.StageAction, len(source.Wrapper.Before))
		for index, action := range source.Wrapper.Before {
			before[index] = tailoringbundle.StageAction{Kind: action.Kind}
		}
		after := make([]tailoringbundle.StageAction, len(source.Wrapper.After))
		for index, action := range source.Wrapper.After {
			after[index] = tailoringbundle.StageAction{Kind: action.Kind}
		}
		converted := tailoringbundle.Wrapper{
			Kind: tailoringbundle.WrapperKind(source.Wrapper.Kind), Before: before,
			Invoke: tailoringbundle.Invocation{
				OptionDefaults: make([]tailoringbundle.OptionDefault, len(source.Wrapper.Invoke.OptionDefaults)),
				AppendArgs:     append(make([]string, 0, len(source.Wrapper.Invoke.AppendArgs)), source.Wrapper.Invoke.AppendArgs...),
			},
			After: after,
		}
		for index, item := range source.Wrapper.Invoke.OptionDefaults {
			converted.Invoke.OptionDefaults[index] = tailoringbundle.OptionDefault{Option: string(item.Option), Value: string(item.Value)}
		}
		if source.Wrapper.Output != nil {
			output := &tailoringbundle.Output{Kind: tailoringbundle.OutputKind(source.Wrapper.Output.Kind)}
			if source.Wrapper.Output.Projection != nil {
				renames := make([]tailoringbundle.Rename, len(source.Wrapper.Output.Projection.Rename))
				for index, item := range source.Wrapper.Output.Projection.Rename {
					renames[index] = tailoringbundle.Rename{From: item.From, To: item.To}
				}
				output.Projection = &tailoringbundle.Projection{
					Input:  source.Wrapper.Output.Projection.Input,
					Select: append(make([]string, 0, len(source.Wrapper.Output.Projection.Select)), source.Wrapper.Output.Projection.Select...),
					Rename: renames,
					Render: source.Wrapper.Output.Projection.Render,
				}
			}
			if source.Wrapper.Output.Optimizer != nil {
				output.Optimizer = &tailoringbundle.Optimizer{
					Input: source.Wrapper.Output.Optimizer.Input, Contract: source.Wrapper.Output.Optimizer.Contract,
					AllowOriginalOutput: source.Wrapper.Output.Optimizer.AllowOriginalOutput,
				}
			}
			converted.Output = output
		}
		result.Wrapper = &converted
	}
	return result
}

func validateNode(node *yaml.Node, depth int, count *int) error {
	if node == nil {
		return nil
	}
	(*count)++
	if *count > maxNodes || depth > maxDepth {
		return fmt.Errorf("YAML structure exceeds its complexity limit")
	}
	if node.Kind == yaml.AliasNode {
		return fmt.Errorf("YAML aliases are not supported")
	}
	for _, child := range node.Content {
		if err := validateNode(child, depth+1, count); err != nil {
			return err
		}
	}
	return nil
}

func fileFault(err error) error {
	switch {
	case errors.Is(err, localfile.ErrNotFound):
		return fault.Wrap(fault.KindNotFound, "specification_file_not_found", "The schema-5 tailoring specification file was not found.", false, err, helpAction())
	case errors.Is(err, localfile.ErrPermission):
		return fault.Wrap(fault.KindPermission, "specification_file_permission_denied", "The schema-5 tailoring specification file cannot be read.", false, err, helpAction())
	case errors.Is(err, localfile.ErrUnsafe):
		return fault.Wrap(fault.KindInvalidInput, "unsafe_specification_file", "The schema-5 tailoring specification must be a stable regular file, not a symbolic link.", false, err, helpAction())
	case errors.Is(err, localfile.ErrTooLarge):
		return fault.Wrap(fault.KindInvalidInput, "specification_file_too_large", "The schema-5 tailoring specification exceeds 256 KiB.", false, err, helpAction())
	default:
		return fault.Wrap(fault.KindUnavailable, "specification_file_read_failed", "The schema-5 tailoring specification could not be read.", true, err, helpAction())
	}
}

func helpAction() fault.NextAction {
	return fault.NextAction{Command: "help spec validate", Reason: "Review the schema-5 tailoring specification contract and file path."}
}
