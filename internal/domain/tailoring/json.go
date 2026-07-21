package tailoring

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"unicode/utf8"
)

const (
	MaxJSONDepth        = 32
	MaxJSONNodes        = 100_000
	MaxJSONObjectFields = 256
	MaxJSONArrayItems   = 10_000
)

var (
	ErrInvalidJSONValue = errors.New("invalid typed JSON value")
	jsonNumberPattern   = regexp.MustCompile(`^-?(0|[1-9][0-9]*)(\.[0-9]+)?([eE][+-]?[0-9]+)?$`)
)

// JSONKind is the finite semantic type of one parsed source JSON value.
type JSONKind string

const (
	JSONNull   JSONKind = "null"
	JSONBool   JSONKind = "bool"
	JSONNumber JSONKind = "number"
	JSONString JSONKind = "string"
	JSONArray  JSONKind = "array"
	JSONObject JSONKind = "object"
)

// JSONField preserves exact object field order and value semantics.
type JSONField struct {
	Name  string
	Value JSONValue
}

// JSONValue is a presentation-independent, validated JSON tree. Exactly one
// value member is active according to Kind.
type JSONValue struct {
	Kind        JSONKind
	BoolValue   bool
	NumberValue string
	StringValue string
	ArrayValue  []JSONValue
	ObjectValue []JSONField
}

func NewJSONNull() JSONValue               { return JSONValue{Kind: JSONNull} }
func NewJSONBool(value bool) JSONValue     { return JSONValue{Kind: JSONBool, BoolValue: value} }
func NewJSONNumber(value string) JSONValue { return JSONValue{Kind: JSONNumber, NumberValue: value} }
func NewJSONString(value string) JSONValue { return JSONValue{Kind: JSONString, StringValue: value} }
func NewJSONArray(value []JSONValue) JSONValue {
	return JSONValue{Kind: JSONArray, ArrayValue: append([]JSONValue{}, value...)}
}
func NewJSONObject(value []JSONField) JSONValue {
	return JSONValue{Kind: JSONObject, ObjectValue: append([]JSONField{}, value...)}
}

// Validate proves type exclusivity, UTF-8, duplicate-free objects, and finite
// complexity before application or presentation consumes the value.
func (v JSONValue) Validate() error {
	nodes := 0
	return v.validate(0, &nodes)
}

// MarshalJSON emits the validated semantic value as compact JSON while
// preserving lexical numbers and the declared order of object fields.
func (v JSONValue) MarshalJSON() ([]byte, error) {
	if err := v.Validate(); err != nil {
		return nil, err
	}
	var output bytes.Buffer
	if err := v.writeJSON(&output); err != nil {
		return nil, err
	}
	return output.Bytes(), nil
}

func (v JSONValue) writeJSON(output *bytes.Buffer) error {
	switch v.Kind {
	case JSONNull:
		output.WriteString("null")
	case JSONBool:
		if v.BoolValue {
			output.WriteString("true")
		} else {
			output.WriteString("false")
		}
	case JSONNumber:
		output.WriteString(v.NumberValue)
	case JSONString:
		encoded, err := json.Marshal(v.StringValue)
		if err != nil {
			return err
		}
		output.Write(encoded)
	case JSONArray:
		output.WriteByte('[')
		for index := range v.ArrayValue {
			if index > 0 {
				output.WriteByte(',')
			}
			if err := v.ArrayValue[index].writeJSON(output); err != nil {
				return err
			}
		}
		output.WriteByte(']')
	case JSONObject:
		output.WriteByte('{')
		for index := range v.ObjectValue {
			if index > 0 {
				output.WriteByte(',')
			}
			name, err := json.Marshal(v.ObjectValue[index].Name)
			if err != nil {
				return err
			}
			output.Write(name)
			output.WriteByte(':')
			if err := v.ObjectValue[index].Value.writeJSON(output); err != nil {
				return err
			}
		}
		output.WriteByte('}')
	default:
		return fmt.Errorf("%w: kind %q is unsupported", ErrInvalidJSONValue, v.Kind)
	}
	return nil
}

func (v JSONValue) validate(depth int, nodes *int) error {
	*nodes = *nodes + 1
	if depth > MaxJSONDepth || *nodes > MaxJSONNodes {
		return fmt.Errorf("%w: complexity limit exceeded", ErrInvalidJSONValue)
	}
	scalarInactive := func() bool {
		return v.NumberValue == "" && v.StringValue == "" && v.ArrayValue == nil && v.ObjectValue == nil
	}
	switch v.Kind {
	case JSONNull:
		if v.BoolValue || !scalarInactive() {
			return fmt.Errorf("%w: null carries another value", ErrInvalidJSONValue)
		}
	case JSONBool:
		if v.NumberValue != "" || v.StringValue != "" || v.ArrayValue != nil || v.ObjectValue != nil {
			return fmt.Errorf("%w: bool carries another value", ErrInvalidJSONValue)
		}
	case JSONNumber:
		if !jsonNumberPattern.MatchString(v.NumberValue) || v.BoolValue || v.StringValue != "" || v.ArrayValue != nil || v.ObjectValue != nil {
			return fmt.Errorf("%w: number is malformed or carries another value", ErrInvalidJSONValue)
		}
	case JSONString:
		if !utf8.ValidString(v.StringValue) || v.BoolValue || v.NumberValue != "" || v.ArrayValue != nil || v.ObjectValue != nil {
			return fmt.Errorf("%w: string is invalid or carries another value", ErrInvalidJSONValue)
		}
	case JSONArray:
		if v.ArrayValue == nil || len(v.ArrayValue) > MaxJSONArrayItems || v.BoolValue || v.NumberValue != "" || v.StringValue != "" || v.ObjectValue != nil {
			return fmt.Errorf("%w: array is missing, oversized, or carries another value", ErrInvalidJSONValue)
		}
		for index := range v.ArrayValue {
			if err := v.ArrayValue[index].validate(depth+1, nodes); err != nil {
				return fmt.Errorf("array item %d: %w", index, err)
			}
		}
	case JSONObject:
		if v.ObjectValue == nil || len(v.ObjectValue) > MaxJSONObjectFields || v.BoolValue || v.NumberValue != "" || v.StringValue != "" || v.ArrayValue != nil {
			return fmt.Errorf("%w: object is missing, oversized, or carries another value", ErrInvalidJSONValue)
		}
		seen := make(map[string]struct{}, len(v.ObjectValue))
		for index := range v.ObjectValue {
			field := v.ObjectValue[index]
			if !utf8.ValidString(field.Name) {
				return fmt.Errorf("%w: object field %d has invalid UTF-8", ErrInvalidJSONValue, index)
			}
			if _, duplicate := seen[field.Name]; duplicate {
				return fmt.Errorf("%w: duplicate object field %q", ErrInvalidJSONValue, field.Name)
			}
			seen[field.Name] = struct{}{}
			if err := field.Value.validate(depth+1, nodes); err != nil {
				return fmt.Errorf("object field %q: %w", field.Name, err)
			}
		}
	default:
		return fmt.Errorf("%w: kind %q is unsupported", ErrInvalidJSONValue, v.Kind)
	}
	return nil
}

func cloneJSONValue(value JSONValue) JSONValue {
	clone := value
	clone.ArrayValue = make([]JSONValue, len(value.ArrayValue))
	for index := range value.ArrayValue {
		clone.ArrayValue[index] = cloneJSONValue(value.ArrayValue[index])
	}
	clone.ObjectValue = make([]JSONField, len(value.ObjectValue))
	for index := range value.ObjectValue {
		clone.ObjectValue[index] = JSONField{Name: value.ObjectValue[index].Name, Value: cloneJSONValue(value.ObjectValue[index].Value)}
	}
	if value.ArrayValue == nil {
		clone.ArrayValue = nil
	}
	if value.ObjectValue == nil {
		clone.ObjectValue = nil
	}
	return clone
}
