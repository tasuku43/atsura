// Package tailoringyaml loads one bounded, strict per-command YAML policy.
package tailoringyaml

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/tasuku43/atsura/internal/domain/fault"
	"github.com/tasuku43/atsura/internal/domain/tailoring"
	"go.yaml.in/yaml/v3"
)

const (
	maxConfigurationBytes = 64 * 1024
	maxYAMLNodes          = 2048
	maxYAMLDepth          = 32
)

// Loader reads local policy files without following symbolic links.
type Loader struct{}

// New creates the production YAML loader.
func New() *Loader { return &Loader{} }

type document struct {
	SchemaVersion int      `yaml:"schema_version"`
	Command       *command `yaml:"command"`
	Decision      string   `yaml:"decision"`
	Reason        string   `yaml:"reason"`
	Invoke        *invoke  `yaml:"invoke"`
	Output        *output  `yaml:"output"`
}

type command struct {
	Executable string   `yaml:"executable"`
	ArgsPrefix []string `yaml:"args_prefix"`
}

type invoke struct {
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

// Load implements planpreview.ConfigurationPort.
func (l *Loader) Load(ctx context.Context, path string) (tailoring.Policy, error) {
	if ctx == nil {
		return tailoring.Policy{}, fmt.Errorf("configuration context is nil")
	}
	if err := ctx.Err(); err != nil {
		return tailoring.Policy{}, err
	}
	directory, name := filepath.Split(path)
	if directory == "" {
		directory = "."
	}
	root, err := os.OpenRoot(directory)
	if err != nil {
		return tailoring.Policy{}, fileFault(err)
	}
	defer root.Close()
	info, err := root.Lstat(name)
	if err != nil {
		return tailoring.Policy{}, fileFault(err)
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
		return tailoring.Policy{}, configurationFault(fault.KindInvalidInput, "unsafe_plan_configuration", "The plan configuration must be a regular file and must not be a symbolic link.", false, nil)
	}
	if info.Size() > maxConfigurationBytes {
		return tailoring.Policy{}, configurationFault(fault.KindInvalidInput, "plan_configuration_too_large", "The plan configuration exceeds the 64 KiB limit.", false, nil)
	}

	file, err := root.Open(name)
	if err != nil {
		return tailoring.Policy{}, fileFault(err)
	}
	defer file.Close()
	openedInfo, err := file.Stat()
	if err != nil {
		return tailoring.Policy{}, configurationFault(fault.KindUnavailable, "plan_configuration_read_failed", "The plan configuration could not be read.", true, err)
	}
	if !openedInfo.Mode().IsRegular() || !os.SameFile(info, openedInfo) {
		return tailoring.Policy{}, configurationFault(fault.KindInvalidInput, "unsafe_plan_configuration", "The plan configuration changed while it was being opened.", false, nil)
	}
	raw, err := io.ReadAll(io.LimitReader(file, maxConfigurationBytes+1))
	if err != nil {
		return tailoring.Policy{}, configurationFault(fault.KindUnavailable, "plan_configuration_read_failed", "The plan configuration could not be read.", true, err)
	}
	if len(raw) > maxConfigurationBytes {
		return tailoring.Policy{}, configurationFault(fault.KindInvalidInput, "plan_configuration_too_large", "The plan configuration exceeds the 64 KiB limit.", false, nil)
	}
	if err := ctx.Err(); err != nil {
		return tailoring.Policy{}, err
	}
	parsed, err := decode(raw)
	if err != nil {
		return tailoring.Policy{}, configurationFault(fault.KindInvalidInput, "invalid_plan_configuration", "The plan configuration is invalid.", false, err)
	}
	return parsed, nil
}

func decode(raw []byte) (tailoring.Policy, error) {
	var root yaml.Node
	nodes := yaml.NewDecoder(bytes.NewReader(raw))
	if err := nodes.Decode(&root); err != nil {
		return tailoring.Policy{}, fmt.Errorf("decode YAML document: %w", err)
	}
	var extra yaml.Node
	if err := nodes.Decode(&extra); !errors.Is(err, io.EOF) {
		if err == nil {
			return tailoring.Policy{}, fmt.Errorf("exactly one YAML document is required")
		}
		return tailoring.Policy{}, fmt.Errorf("decode trailing YAML document: %w", err)
	}
	count := 0
	if err := validateNode(&root, 0, &count); err != nil {
		return tailoring.Policy{}, err
	}

	decoder := yaml.NewDecoder(bytes.NewReader(raw))
	decoder.KnownFields(true)
	var value document
	if err := decoder.Decode(&value); err != nil {
		return tailoring.Policy{}, fmt.Errorf("decode strict configuration: %w", err)
	}
	if err := decoder.Decode(&document{}); !errors.Is(err, io.EOF) {
		return tailoring.Policy{}, fmt.Errorf("exactly one YAML document is required")
	}
	if value.Command == nil || value.Invoke == nil || value.Output == nil {
		return tailoring.Policy{}, fmt.Errorf("command, invoke, and output mappings are required")
	}
	renames := make([]tailoring.Rename, len(value.Output.Rename))
	for index, item := range value.Output.Rename {
		renames[index] = tailoring.Rename{From: item.From, To: item.To}
	}
	policy := tailoring.Policy{
		SchemaVersion: value.SchemaVersion,
		Executable:    value.Command.Executable,
		ArgsPrefix:    append([]string(nil), value.Command.ArgsPrefix...),
		Decision:      tailoring.Decision(value.Decision),
		Reason:        value.Reason,
		AppendArgs:    append([]string(nil), value.Invoke.AppendArgs...),
		Output: tailoring.OutputPlan{
			Input:  tailoring.InputFormat(value.Output.Input),
			Select: append([]string(nil), value.Output.Select...),
			Rename: renames,
			Render: tailoring.RenderFormat(value.Output.Render),
		},
	}
	if err := policy.Validate(); err != nil {
		return tailoring.Policy{}, err
	}
	return policy, nil
}

func validateNode(node *yaml.Node, depth int, count *int) error {
	if node == nil {
		return nil
	}
	*count = *count + 1
	if *count > maxYAMLNodes || depth > maxYAMLDepth {
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
	case errors.Is(err, os.ErrNotExist):
		return configurationFault(fault.KindNotFound, "plan_configuration_not_found", "The plan configuration was not found.", false, err)
	case errors.Is(err, os.ErrPermission):
		return configurationFault(fault.KindPermission, "plan_configuration_permission_denied", "The plan configuration cannot be read with the current permissions.", false, err)
	default:
		return configurationFault(fault.KindUnavailable, "plan_configuration_read_failed", "The plan configuration could not be read.", true, err)
	}
}

func configurationFault(kind fault.Kind, code, message string, retryable bool, cause error) *fault.Error {
	return fault.Wrap(kind, code, message, retryable, cause, fault.NextAction{
		Command: "help plan preview", Reason: "Review the plan preview contract and correct the configuration.",
	})
}
