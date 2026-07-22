package wrapperbinding

import (
	"errors"
	"fmt"
	"reflect"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/tasuku43/atsura/internal/domain/sourcecatalog"
	"github.com/tasuku43/atsura/internal/domain/tailoringbundle"
)

const (
	// MaxCompiledHelpViews bounds root plus unique namespace/exact selectors.
	MaxCompiledHelpViews = 256
	// MaxCompiledHelpLines bounds the semantic lines emitted across all fixed
	// help branches, including repeated root/namespace projections.
	MaxCompiledHelpLines = 2048
	// MaxCompiledHelpLiteralBytes bounds the aggregate bytes supplied as fixed
	// printf arguments. The final generated source retains its separate 64 KiB
	// wrapperbinding.RenderedMaterial bound.
	MaxCompiledHelpLiteralBytes = 48 * 1024
)

var ErrInvalidCompiledHelp = errors.New("invalid compiled wrapper help")

// HelpOption is one caller-visible long option and its observed value arity.
type HelpOption struct {
	Name       string `json:"name"`
	TakesValue bool   `json:"takes_value"`
}

// HelpCommand is one included exact command projected from a compiled bundle.
// It stores semantic facts once; root and namespace views reference only full
// command paths so excluded command text cannot leak through presentation.
type HelpCommand struct {
	Path    []string     `json:"path"`
	Summary string       `json:"summary"`
	Reason  string       `json:"reason"`
	Options []HelpOption `json:"options"`
}

// CompiledHelp is the canonical semantic help snapshot embedded in one
// generated wrapper binding.
type CompiledHelp struct {
	Commands []HelpCommand `json:"commands"`
}

// HelpView is one root, namespace, exact, or combined exact-and-namespace
// projection. Selector is empty for root. Descendants are strict included
// descendants represented by their full exact paths.
type HelpView struct {
	Selector    []string
	Exact       *HelpCommand
	Descendants [][]string
}

// CompileHelp derives one detached help snapshot solely from the exact valid
// bundle. It never reads source help or consults a runtime registry.
func CompileHelp(bundle tailoringbundle.Bundle) (CompiledHelp, error) {
	if err := bundle.Validate(); err != nil {
		return CompiledHelp{}, invalidHelp("bundle: %v", err)
	}
	catalogCommands := make(map[string]sourcecatalog.Command, len(bundle.Catalog.Commands))
	for _, command := range bundle.Catalog.Commands {
		catalogCommands[helpPathKey(command.Path)] = command
	}

	help := CompiledHelp{Commands: make([]HelpCommand, 0, len(bundle.Surface))}
	for _, entry := range bundle.Surface {
		command, exists := catalogCommands[helpPathKey(entry.Command)]
		if !exists {
			return CompiledHelp{}, invalidHelp("included command %q is missing from the catalog", strings.Join(entry.Command, " "))
		}
		included, err := entry.Options.IncludedOptions(command)
		if err != nil {
			return CompiledHelp{}, invalidHelp("included command %q options: %v", strings.Join(entry.Command, " "), err)
		}
		options := make([]HelpOption, len(included))
		for index, option := range included {
			options[index] = HelpOption{Name: option.Name, TakesValue: option.TakesValue}
		}
		help.Commands = append(help.Commands, HelpCommand{
			Path:    cloneHelpStrings(entry.Command),
			Summary: command.Summary,
			Reason:  entry.Reason,
			Options: options,
		})
	}
	if err := help.Validate(); err != nil {
		return CompiledHelp{}, err
	}
	return help.Clone(), nil
}

// Validate rejects incomplete, non-canonical, structurally unsafe, or
// unbounded compiled help independently from the bundle that produced it.
func (h CompiledHelp) Validate() error {
	if h.Commands == nil || len(h.Commands) == 0 || len(h.Commands) > sourcecatalog.MaxCommands {
		return invalidHelp("commands must be a non-empty bounded list")
	}
	previous := ""
	for index, command := range h.Commands {
		if err := command.validate(); err != nil {
			return invalidHelp("command %d: %v", index, err)
		}
		key := helpPathKey(command.Path)
		if previous != "" && key <= previous {
			return invalidHelp("commands must be sorted and unique by path")
		}
		previous = key
	}
	views, err := h.viewsUnchecked()
	if err != nil {
		return err
	}
	return validateHelpBudget(views)
}

func (c HelpCommand) validate() error {
	if len(c.Path) == 0 || len(c.Path) > sourcecatalog.MaxCommandSegments {
		return fmt.Errorf("path must be a non-empty bounded list")
	}
	for _, segment := range c.Path {
		if !validHelpStableName(segment) {
			return fmt.Errorf("path segment %q is invalid", segment)
		}
	}
	if err := validateHelpText(c.Summary, sourcecatalog.MaxTextBytes); err != nil {
		return fmt.Errorf("summary: %v", err)
	}
	if err := validateHelpText(c.Reason, sourcecatalog.MaxTextBytes); err != nil {
		return fmt.Errorf("reason: %v", err)
	}
	if c.Options == nil || len(c.Options) > sourcecatalog.MaxOptions {
		return fmt.Errorf("options must be an explicit bounded list")
	}
	previous := ""
	for _, option := range c.Options {
		if !strings.HasPrefix(option.Name, "--") || !validHelpStableName(strings.TrimPrefix(option.Name, "--")) {
			return fmt.Errorf("option %q is invalid", option.Name)
		}
		if option.Name <= previous {
			return fmt.Errorf("options must be sorted and unique")
		}
		previous = option.Name
	}
	return nil
}

// Views returns detached root and selector projections. Root is first; every
// non-root selector follows canonical full-path order.
func (h CompiledHelp) Views() ([]HelpView, error) {
	if err := h.Validate(); err != nil {
		return nil, err
	}
	views, err := h.viewsUnchecked()
	if err != nil {
		return nil, err
	}
	return cloneHelpViews(views), nil
}

func (h CompiledHelp) viewsUnchecked() ([]HelpView, error) {
	selectors := map[string][]string{"": {}}
	for _, command := range h.Commands {
		for length := 1; length <= len(command.Path); length++ {
			selector := command.Path[:length]
			key := helpPathKey(selector)
			if _, exists := selectors[key]; exists {
				continue
			}
			if len(selectors) >= MaxCompiledHelpViews {
				return nil, invalidHelp("views exceed the %d-view bound", MaxCompiledHelpViews)
			}
			selectors[key] = cloneHelpStrings(selector)
		}
	}

	ordered := make([][]string, 0, len(selectors)-1)
	for key, selector := range selectors {
		if key != "" {
			ordered = append(ordered, selector)
		}
	}
	sort.Slice(ordered, func(i, j int) bool { return helpPathKey(ordered[i]) < helpPathKey(ordered[j]) })
	ordered = append([][]string{{}}, ordered...)

	views := make([]HelpView, 0, len(ordered))
	for _, selector := range ordered {
		view := HelpView{Selector: cloneHelpStrings(selector), Descendants: [][]string{}}
		for _, command := range h.Commands {
			switch {
			case helpPathsEqual(command.Path, selector):
				exact := cloneHelpCommand(command)
				view.Exact = &exact
			case helpPathHasPrefix(command.Path, selector):
				view.Descendants = append(view.Descendants, cloneHelpStrings(command.Path))
			}
		}
		if view.Exact == nil && len(view.Descendants) == 0 {
			return nil, invalidHelp("selector %q has neither an exact command nor descendants", strings.Join(selector, " "))
		}
		views = append(views, view)
	}
	return views, nil
}

// Clone deeply detaches paths and options.
func (h CompiledHelp) Clone() CompiledHelp {
	if h.Commands == nil {
		return CompiledHelp{}
	}
	result := CompiledHelp{Commands: make([]HelpCommand, len(h.Commands))}
	for index, command := range h.Commands {
		result.Commands[index] = cloneHelpCommand(command)
	}
	return result
}

// Equal compares canonical semantic values. Callers that accept untrusted
// values validate them before using equality as a bundle-binding decision.
func (h CompiledHelp) Equal(other CompiledHelp) bool {
	return reflect.DeepEqual(h, other)
}

func validateHelpBudget(views []HelpView) error {
	lines := 0
	bytes := 0
	add := func(line string) error {
		lines++
		bytes += len(line)
		if lines > MaxCompiledHelpLines {
			return invalidHelp("semantic lines exceed the %d-line bound", MaxCompiledHelpLines)
		}
		if bytes > MaxCompiledHelpLiteralBytes {
			return invalidHelp("literal payload exceeds the %d-byte bound", MaxCompiledHelpLiteralBytes)
		}
		return nil
	}
	for _, view := range views {
		if err := add("Atsura tailored help"); err != nil {
			return err
		}
		if err := add("Bundle digest: " + strings.Repeat("a", 64)); err != nil {
			return err
		}
		if len(view.Descendants) > 0 {
			if err := add("Commands:"); err != nil {
				return err
			}
			for _, path := range view.Descendants {
				if err := add("  " + strings.Join(path, " ")); err != nil {
					return err
				}
			}
		}
		if view.Exact != nil {
			if err := add("Command: " + strings.Join(view.Exact.Path, " ")); err != nil {
				return err
			}
			if err := add("Source summary: " + view.Exact.Summary); err != nil {
				return err
			}
			if err := add("Tailoring reason: " + view.Exact.Reason); err != nil {
				return err
			}
			if len(view.Exact.Options) > 0 {
				if err := add("Options:"); err != nil {
					return err
				}
				for _, option := range view.Exact.Options {
					line := "  " + option.Name + " (no value)"
					if option.TakesValue {
						line = "  " + option.Name + "=<value> (value required)"
					}
					if err := add(line); err != nil {
						return err
					}
				}
			}
		}
	}
	return nil
}

func cloneHelpCommand(value HelpCommand) HelpCommand {
	var options []HelpOption
	if value.Options != nil {
		options = append([]HelpOption{}, value.Options...)
	}
	return HelpCommand{
		Path:    cloneHelpStrings(value.Path),
		Summary: value.Summary,
		Reason:  value.Reason,
		Options: options,
	}
}

func cloneHelpViews(values []HelpView) []HelpView {
	result := make([]HelpView, len(values))
	for index, value := range values {
		result[index] = HelpView{Selector: cloneHelpStrings(value.Selector), Descendants: make([][]string, len(value.Descendants))}
		if value.Exact != nil {
			exact := cloneHelpCommand(*value.Exact)
			result[index].Exact = &exact
		}
		for descendantIndex, descendant := range value.Descendants {
			result[index].Descendants[descendantIndex] = cloneHelpStrings(descendant)
		}
	}
	return result
}

func cloneHelpStrings(values []string) []string {
	if values == nil {
		return nil
	}
	return append([]string{}, values...)
}

func helpPathKey(path []string) string { return strings.Join(path, "\x00") }

func helpPathsEqual(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range right {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}

func helpPathHasPrefix(path, prefix []string) bool {
	if len(path) <= len(prefix) {
		return false
	}
	for index := range prefix {
		if path[index] != prefix[index] {
			return false
		}
	}
	return true
}

func validHelpStableName(value string) bool {
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

func validateHelpText(value string, limit int) error {
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

func invalidHelp(format string, args ...any) error {
	return fmt.Errorf("%w: %s", ErrInvalidCompiledHelp, fmt.Sprintf(format, args...))
}
