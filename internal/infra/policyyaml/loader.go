// Package policyyaml decodes one bounded strict schema-2 policy file.
package policyyaml

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"

	"github.com/tasuku43/atsura/internal/domain/fault"
	"github.com/tasuku43/atsura/internal/domain/operation"
	"github.com/tasuku43/atsura/internal/domain/tailoringbundle"
	"github.com/tasuku43/atsura/internal/infra/localfile"
	"go.yaml.in/yaml/v3"
)

const (
	maxPolicyBytes = int64(256 * 1024)
	maxNodes       = 8192
	maxDepth       = 32
)

type Loader struct{}

func New() *Loader { return &Loader{} }

type document struct {
	SchemaVersion int    `yaml:"schema_version"`
	CatalogDigest string `yaml:"catalog_digest"`
	Rules         []rule `yaml:"rules"`
}

type rule struct {
	Command    []string `yaml:"command"`
	Visibility string   `yaml:"visibility"`
	Effect     string   `yaml:"effect"`
	Decision   string   `yaml:"decision"`
	Reason     string   `yaml:"reason"`
	AppendArgs []string `yaml:"append_args"`
	Target     *target  `yaml:"target,omitempty"`
	Impact     *impact  `yaml:"impact,omitempty"`
	Output     *output  `yaml:"output,omitempty"`
}

type target struct {
	Kind          string `yaml:"kind"`
	ArgumentIndex *int   `yaml:"argument_index,omitempty"`
	Flag          string `yaml:"flag,omitempty"`
}

type impact struct {
	Cardinality  string `yaml:"cardinality"`
	Notification string `yaml:"notification"`
	AccessChange string `yaml:"access_change"`
	Destructive  string `yaml:"destructive"`
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

func (l *Loader) Load(ctx context.Context, path string) (tailoringbundle.Policy, error) {
	raw, err := localfile.Read(ctx, path, maxPolicyBytes)
	if err != nil {
		return tailoringbundle.Policy{}, fileFault(err)
	}
	parsed, err := decode(raw)
	if err != nil {
		return tailoringbundle.Policy{}, fault.Wrap(fault.KindInvalidInput, "invalid_policy_yaml", "The schema-2 policy YAML is invalid.", false, err, helpAction())
	}
	return parsed, nil
}

// Encode renders one already normalized policy as deterministic schema-2 YAML.
func Encode(policy tailoringbundle.Policy) ([]byte, error) {
	value := document{SchemaVersion: policy.SchemaVersion, CatalogDigest: policy.CatalogDigest, Rules: make([]rule, len(policy.Rules))}
	for index, source := range policy.Rules {
		converted := rule{
			Command:    append(make([]string, 0, len(source.Command)), source.Command...),
			Visibility: string(source.Visibility), Effect: source.Effect.String(), Decision: string(source.Decision),
			Reason: source.Reason, AppendArgs: append(make([]string, 0, len(source.AppendArgs)), source.AppendArgs...),
		}
		if source.Target != nil {
			converted.Target = &target{Kind: source.Target.Kind, ArgumentIndex: source.Target.ArgumentIndex, Flag: source.Target.Flag}
		}
		if source.Impact != nil {
			converted.Impact = &impact{Cardinality: source.Impact.Cardinality.String(), Notification: source.Impact.Notification.String(), AccessChange: source.Impact.AccessChange.String(), Destructive: source.Impact.Destructive.String()}
		}
		if source.Output != nil {
			renames := make([]rename, len(source.Output.Rename))
			for renameIndex, item := range source.Output.Rename {
				renames[renameIndex] = rename{From: item.From, To: item.To}
			}
			converted.Output = &output{Input: source.Output.Input, Select: append(make([]string, 0, len(source.Output.Select)), source.Output.Select...), Rename: renames, Render: source.Output.Render}
		}
		value.Rules[index] = converted
	}
	encoded, err := yaml.Marshal(value)
	if err != nil {
		return nil, fmt.Errorf("encode schema-2 policy YAML: %w", err)
	}
	return encoded, nil
}

func decode(raw []byte) (tailoringbundle.Policy, error) {
	var root yaml.Node
	nodes := yaml.NewDecoder(bytes.NewReader(raw))
	if err := nodes.Decode(&root); err != nil {
		return tailoringbundle.Policy{}, fmt.Errorf("decode YAML document: %w", err)
	}
	var extra yaml.Node
	if err := nodes.Decode(&extra); !errors.Is(err, io.EOF) {
		return tailoringbundle.Policy{}, fmt.Errorf("exactly one YAML document is required")
	}
	count := 0
	if err := validateNode(&root, 0, &count); err != nil {
		return tailoringbundle.Policy{}, err
	}
	decoder := yaml.NewDecoder(bytes.NewReader(raw))
	decoder.KnownFields(true)
	var value document
	if err := decoder.Decode(&value); err != nil {
		return tailoringbundle.Policy{}, fmt.Errorf("decode strict policy: %w", err)
	}
	if err := decoder.Decode(&document{}); !errors.Is(err, io.EOF) {
		return tailoringbundle.Policy{}, fmt.Errorf("exactly one YAML document is required")
	}
	result := tailoringbundle.Policy{SchemaVersion: value.SchemaVersion, CatalogDigest: value.CatalogDigest, Rules: make([]tailoringbundle.Rule, len(value.Rules))}
	for index, source := range value.Rules {
		converted, err := convertRule(source)
		if err != nil {
			return tailoringbundle.Policy{}, fmt.Errorf("rule %d: %w", index, err)
		}
		result.Rules[index] = converted
	}
	return tailoringbundle.SortPolicy(result), nil
}

func convertRule(source rule) (tailoringbundle.Rule, error) {
	effect := operation.EffectUnknown
	if err := effect.UnmarshalText([]byte(source.Effect)); err != nil {
		return tailoringbundle.Rule{}, err
	}
	result := tailoringbundle.Rule{
		Command: append(make([]string, 0, len(source.Command)), source.Command...), Visibility: tailoringbundle.Visibility(source.Visibility),
		Effect: effect, Decision: tailoringbundle.Decision(source.Decision), Reason: source.Reason,
		AppendArgs: append(make([]string, 0, len(source.AppendArgs)), source.AppendArgs...),
	}
	if source.Target != nil {
		result.Target = &tailoringbundle.TargetBinding{Kind: source.Target.Kind, ArgumentIndex: source.Target.ArgumentIndex, Flag: source.Target.Flag}
	}
	if source.Impact != nil {
		converted := operation.Impact{}
		if err := converted.Cardinality.UnmarshalText([]byte(source.Impact.Cardinality)); err != nil {
			return tailoringbundle.Rule{}, err
		}
		if err := converted.Notification.UnmarshalText([]byte(source.Impact.Notification)); err != nil {
			return tailoringbundle.Rule{}, err
		}
		if err := converted.AccessChange.UnmarshalText([]byte(source.Impact.AccessChange)); err != nil {
			return tailoringbundle.Rule{}, err
		}
		if err := converted.Destructive.UnmarshalText([]byte(source.Impact.Destructive)); err != nil {
			return tailoringbundle.Rule{}, err
		}
		result.Impact = &converted
	}
	if source.Output != nil {
		renames := make([]tailoringbundle.Rename, len(source.Output.Rename))
		for index, item := range source.Output.Rename {
			renames[index] = tailoringbundle.Rename{From: item.From, To: item.To}
		}
		result.Output = &tailoringbundle.Output{Input: source.Output.Input, Select: append(make([]string, 0, len(source.Output.Select)), source.Output.Select...), Rename: renames, Render: source.Output.Render}
	}
	return result, nil
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
		return fault.Wrap(fault.KindNotFound, "policy_file_not_found", "The schema-2 policy file was not found.", false, err, helpAction())
	case errors.Is(err, localfile.ErrPermission):
		return fault.Wrap(fault.KindPermission, "policy_file_permission_denied", "The schema-2 policy file cannot be read.", false, err, helpAction())
	case errors.Is(err, localfile.ErrUnsafe):
		return fault.Wrap(fault.KindInvalidInput, "unsafe_policy_file", "The schema-2 policy must be a stable regular file, not a symbolic link.", false, err, helpAction())
	case errors.Is(err, localfile.ErrTooLarge):
		return fault.Wrap(fault.KindInvalidInput, "policy_file_too_large", "The schema-2 policy exceeds 256 KiB.", false, err, helpAction())
	default:
		return fault.Wrap(fault.KindUnavailable, "policy_file_read_failed", "The schema-2 policy could not be read.", true, err, helpAction())
	}
}

func helpAction() fault.NextAction {
	return fault.NextAction{Command: "help policy validate", Reason: "Review the schema-2 policy contract and file path."}
}
