// Package specyaml decodes one bounded strict schema-3 tailoring
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
	AppendArgs []string `yaml:"append_args"`
}

type output struct {
	Input  string   `yaml:"input"`
	Select []string `yaml:"select"`
	Rename []rename `yaml:"rename"`
	Render string   `yaml:"render"`
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
				"Authorization-centered tailoring schemas 1 and 2 are retired and cannot be converted automatically.",
				false,
				err,
				fault.NextAction{Command: "help spec init", Reason: "Create a schema-3 surface and wrapper specification from the catalog."},
			)
		}
		return tailoringbundle.Specification{}, fault.Wrap(fault.KindInvalidInput, "invalid_specification_yaml", "The schema-3 tailoring specification YAML is invalid.", false, err, helpAction())
	}
	return parsed, nil
}

// Encode renders one normalized specification as deterministic schema-3 YAML.
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
			converted.Wrapper = encodeWrapper(*source.Wrapper)
		}
		value.Commands[index] = converted
	}
	encoded, err := yaml.Marshal(value)
	if err != nil {
		return nil, fmt.Errorf("encode schema-3 tailoring specification YAML: %w", err)
	}
	return encoded, nil
}

func encodeWrapper(source tailoringbundle.Wrapper) *wrapper {
	converted := &wrapper{
		Kind: string(source.Kind), Before: make([]stageAction, len(source.Before)),
		Invoke: invocation{AppendArgs: append(make([]string, 0, len(source.Invoke.AppendArgs)), source.Invoke.AppendArgs...)},
		After:  make([]stageAction, len(source.After)),
	}
	for index, action := range source.Before {
		converted.Before[index] = stageAction{Kind: action.Kind}
	}
	for index, action := range source.After {
		converted.After[index] = stageAction{Kind: action.Kind}
	}
	if source.Output != nil {
		renames := make([]rename, len(source.Output.Rename))
		for index, item := range source.Output.Rename {
			renames[index] = rename{From: item.From, To: item.To}
		}
		converted.Output = &output{
			Input: source.Output.Input, Select: append(make([]string, 0, len(source.Output.Select)), source.Output.Select...),
			Rename: renames, Render: source.Output.Render,
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
	if header.SchemaVersion == 1 || header.SchemaVersion == 2 {
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
			Invoke: tailoringbundle.Invocation{AppendArgs: append(make([]string, 0, len(source.Wrapper.Invoke.AppendArgs)), source.Wrapper.Invoke.AppendArgs...)},
			After:  after,
		}
		if source.Wrapper.Output != nil {
			renames := make([]tailoringbundle.Rename, len(source.Wrapper.Output.Rename))
			for index, item := range source.Wrapper.Output.Rename {
				renames[index] = tailoringbundle.Rename{From: item.From, To: item.To}
			}
			converted.Output = &tailoringbundle.Output{
				Input:  source.Wrapper.Output.Input,
				Select: append(make([]string, 0, len(source.Wrapper.Output.Select)), source.Wrapper.Output.Select...),
				Rename: renames,
				Render: source.Wrapper.Output.Render,
			}
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
		return fault.Wrap(fault.KindNotFound, "specification_file_not_found", "The schema-3 tailoring specification file was not found.", false, err, helpAction())
	case errors.Is(err, localfile.ErrPermission):
		return fault.Wrap(fault.KindPermission, "specification_file_permission_denied", "The schema-3 tailoring specification file cannot be read.", false, err, helpAction())
	case errors.Is(err, localfile.ErrUnsafe):
		return fault.Wrap(fault.KindInvalidInput, "unsafe_specification_file", "The schema-3 tailoring specification must be a stable regular file, not a symbolic link.", false, err, helpAction())
	case errors.Is(err, localfile.ErrTooLarge):
		return fault.Wrap(fault.KindInvalidInput, "specification_file_too_large", "The schema-3 tailoring specification exceeds 256 KiB.", false, err, helpAction())
	default:
		return fault.Wrap(fault.KindUnavailable, "specification_file_read_failed", "The schema-3 tailoring specification could not be read.", true, err, helpAction())
	}
}

func helpAction() fault.NextAction {
	return fault.NextAction{Command: "help spec validate", Reason: "Review the schema-3 tailoring specification contract and file path."}
}
