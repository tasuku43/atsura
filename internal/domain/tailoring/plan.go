// Package tailoring defines the pure typed output-transformation vocabulary
// used by schema-3 wrappers without performing source I/O.
package tailoring

import (
	"errors"
	"fmt"
)

const maxOutputFields = 64

var ErrInvalidOutputPlan = errors.New("invalid tailoring output plan")

// InputFormat describes the structured source output expected by a wrapper.
type InputFormat string

const InputJSON InputFormat = "json"

// RenderFormat describes the built-in agent-facing representation.
type RenderFormat string

const RenderCompactJSON RenderFormat = "compact_json"

// Rename changes one selected source field name in the transformed shape.
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

func (o OutputPlan) validate() error {
	if o.Input != InputJSON {
		return invalidOutputPlan("output input must be json")
	}
	if o.Select == nil || len(o.Select) == 0 || len(o.Select) > maxOutputFields {
		return invalidOutputPlan("output select must be a non-empty bounded list")
	}
	selected := make(map[string]struct{}, len(o.Select))
	for _, field := range o.Select {
		if err := validateFieldName(field); err != nil {
			return invalidOutputPlan("output select field %q: %v", field, err)
		}
		if _, duplicate := selected[field]; duplicate {
			return invalidOutputPlan("output select field %q is duplicated", field)
		}
		selected[field] = struct{}{}
	}
	if o.Rename == nil || len(o.Rename) > maxOutputFields {
		return invalidOutputPlan("output rename must be an explicit bounded list")
	}
	renamedFrom := make(map[string]struct{}, len(o.Rename))
	finalNames := make(map[string]struct{}, len(o.Select))
	for _, field := range o.Select {
		finalNames[field] = struct{}{}
	}
	for _, rename := range o.Rename {
		if err := validateFieldName(rename.From); err != nil {
			return invalidOutputPlan("output rename source %q: %v", rename.From, err)
		}
		if err := validateFieldName(rename.To); err != nil {
			return invalidOutputPlan("output rename target %q: %v", rename.To, err)
		}
		if _, exists := selected[rename.From]; !exists {
			return invalidOutputPlan("output rename source %q is not selected", rename.From)
		}
		if _, duplicate := renamedFrom[rename.From]; duplicate {
			return invalidOutputPlan("output rename source %q is duplicated", rename.From)
		}
		delete(finalNames, rename.From)
		if _, collision := finalNames[rename.To]; collision {
			return invalidOutputPlan("output rename target %q collides with another result field", rename.To)
		}
		finalNames[rename.To] = struct{}{}
		renamedFrom[rename.From] = struct{}{}
	}
	if o.Render != RenderCompactJSON {
		return invalidOutputPlan("output render must be compact_json")
	}
	return nil
}

// Validate proves that an output plan contains only the supported typed
// transformation vocabulary.
func (o OutputPlan) Validate() error { return o.validate() }

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

func invalidOutputPlan(format string, args ...any) error {
	return fmt.Errorf("%w: %s", ErrInvalidOutputPlan, fmt.Sprintf(format, args...))
}
