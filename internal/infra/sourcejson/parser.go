// Package sourcejson strictly parses bounded source JSON into typed domain
// values without deciding which fields a wrapper transformation selects.
package sourcejson

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"unicode/utf8"

	"github.com/tasuku43/atsura/internal/domain/tailoring"
)

const MaxInputBytes = 4 * 1024 * 1024

// Parser is the production duplicate-aware JSON adapter.
type Parser struct{}

// New creates a source JSON parser.
func New() *Parser { return &Parser{} }

// Parse converts exactly one JSON value into a validated typed tree.
func (p *Parser) Parse(ctx context.Context, input []byte) (tailoring.JSONValue, error) {
	if ctx == nil {
		return tailoring.JSONValue{}, fmt.Errorf("source JSON context is nil")
	}
	if err := ctx.Err(); err != nil {
		return tailoring.JSONValue{}, err
	}
	if len(input) == 0 || len(input) > MaxInputBytes || !utf8.Valid(input) {
		return tailoring.JSONValue{}, fmt.Errorf("source JSON must be non-empty valid UTF-8 within %d bytes", MaxInputBytes)
	}
	decoder := json.NewDecoder(bytes.NewReader(input))
	decoder.UseNumber()
	nodes := 0
	value, err := parseValue(ctx, decoder, 0, &nodes)
	if err != nil {
		return tailoring.JSONValue{}, err
	}
	if _, err := decoder.Token(); !errors.Is(err, io.EOF) {
		if err == nil {
			return tailoring.JSONValue{}, fmt.Errorf("source JSON contains more than one value")
		}
		return tailoring.JSONValue{}, fmt.Errorf("source JSON trailing data: %w", err)
	}
	if err := value.Validate(); err != nil {
		return tailoring.JSONValue{}, fmt.Errorf("source JSON domain value: %w", err)
	}
	return value, nil
}

func parseValue(ctx context.Context, decoder *json.Decoder, depth int, nodes *int) (tailoring.JSONValue, error) {
	if err := ctx.Err(); err != nil {
		return tailoring.JSONValue{}, err
	}
	*nodes = *nodes + 1
	if depth > tailoring.MaxJSONDepth || *nodes > tailoring.MaxJSONNodes {
		return tailoring.JSONValue{}, fmt.Errorf("source JSON exceeds its complexity limit")
	}
	token, err := decoder.Token()
	if err != nil {
		return tailoring.JSONValue{}, fmt.Errorf("read source JSON value: %w", err)
	}
	switch value := token.(type) {
	case nil:
		return tailoring.NewJSONNull(), nil
	case bool:
		return tailoring.NewJSONBool(value), nil
	case string:
		return tailoring.NewJSONString(value), nil
	case json.Number:
		return tailoring.NewJSONNumber(value.String()), nil
	case json.Delim:
		switch value {
		case '{':
			return parseObject(ctx, decoder, depth, nodes)
		case '[':
			return parseArray(ctx, decoder, depth, nodes)
		default:
			return tailoring.JSONValue{}, fmt.Errorf("unexpected source JSON delimiter %q", value)
		}
	default:
		return tailoring.JSONValue{}, fmt.Errorf("unsupported source JSON token %T", token)
	}
}

func parseObject(ctx context.Context, decoder *json.Decoder, depth int, nodes *int) (tailoring.JSONValue, error) {
	fields := make([]tailoring.JSONField, 0)
	seen := make(map[string]struct{})
	for decoder.More() {
		nameToken, err := decoder.Token()
		if err != nil {
			return tailoring.JSONValue{}, fmt.Errorf("read source JSON object field: %w", err)
		}
		name, ok := nameToken.(string)
		if !ok {
			return tailoring.JSONValue{}, fmt.Errorf("source JSON object field name is not a string")
		}
		if _, duplicate := seen[name]; duplicate {
			return tailoring.JSONValue{}, fmt.Errorf("source JSON object field %q is duplicated", name)
		}
		if len(fields) >= tailoring.MaxJSONObjectFields {
			return tailoring.JSONValue{}, fmt.Errorf("source JSON object exceeds its field limit")
		}
		child, err := parseValue(ctx, decoder, depth+1, nodes)
		if err != nil {
			return tailoring.JSONValue{}, fmt.Errorf("source JSON object field %q: %w", name, err)
		}
		seen[name] = struct{}{}
		fields = append(fields, tailoring.JSONField{Name: name, Value: child})
	}
	closing, err := decoder.Token()
	if err != nil || closing != json.Delim('}') {
		return tailoring.JSONValue{}, fmt.Errorf("source JSON object is not closed")
	}
	return tailoring.NewJSONObject(fields), nil
}

func parseArray(ctx context.Context, decoder *json.Decoder, depth int, nodes *int) (tailoring.JSONValue, error) {
	items := make([]tailoring.JSONValue, 0)
	for decoder.More() {
		if len(items) >= tailoring.MaxJSONArrayItems {
			return tailoring.JSONValue{}, fmt.Errorf("source JSON array exceeds its item limit")
		}
		child, err := parseValue(ctx, decoder, depth+1, nodes)
		if err != nil {
			return tailoring.JSONValue{}, fmt.Errorf("source JSON array item %d: %w", len(items), err)
		}
		items = append(items, child)
	}
	closing, err := decoder.Token()
	if err != nil || closing != json.Delim(']') {
		return tailoring.JSONValue{}, fmt.Errorf("source JSON array is not closed")
	}
	return tailoring.NewJSONArray(items), nil
}
