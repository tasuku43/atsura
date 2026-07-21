// Package strictjson decodes bounded JSON artifacts with duplicate and unknown
// field rejection.
package strictjson

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
)

type tokenFrame struct {
	object    bool
	expectKey bool
	keys      map[string]struct{}
}

// Decode rejects duplicate object keys, unknown destination fields, trailing
// values, and structures deeper than maxDepth.
func Decode(raw []byte, destination any, maxDepth int) error {
	if destination == nil || maxDepth <= 0 {
		return fmt.Errorf("strict JSON destination and depth are required")
	}
	if err := validateTokens(raw, maxDepth); err != nil {
		return err
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(destination); err != nil {
		return fmt.Errorf("decode strict JSON: %w", err)
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		return fmt.Errorf("strict JSON requires exactly one value")
	}
	return nil
}

func validateTokens(raw []byte, maxDepth int) error {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	stack := make([]tokenFrame, 0, maxDepth)
	for {
		token, err := decoder.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("scan strict JSON: %w", err)
		}
		switch value := token.(type) {
		case json.Delim:
			switch value {
			case '{':
				stack = append(stack, tokenFrame{object: true, expectKey: true, keys: map[string]struct{}{}})
			case '[':
				stack = append(stack, tokenFrame{})
			case '}', ']':
				if len(stack) == 0 {
					return fmt.Errorf("strict JSON delimiter is unbalanced")
				}
				stack = stack[:len(stack)-1]
				markValueConsumed(stack)
			}
			if len(stack) > maxDepth {
				return fmt.Errorf("strict JSON exceeds depth %d", maxDepth)
			}
		case string:
			if len(stack) > 0 && stack[len(stack)-1].object && stack[len(stack)-1].expectKey {
				frame := &stack[len(stack)-1]
				if _, exists := frame.keys[value]; exists {
					return fmt.Errorf("strict JSON contains duplicate key %q", value)
				}
				frame.keys[value] = struct{}{}
				frame.expectKey = false
			} else {
				markValueConsumed(stack)
			}
		default:
			markValueConsumed(stack)
		}
	}
	return nil
}

func markValueConsumed(stack []tokenFrame) {
	if len(stack) > 0 && stack[len(stack)-1].object && !stack[len(stack)-1].expectKey {
		stack[len(stack)-1].expectKey = true
	}
}
