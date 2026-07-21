package sourcejson

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/tasuku43/atsura/internal/domain/tailoring"
)

func TestParsePreservesJSONTypesAndLexicalNumbers(t *testing.T) {
	input := []byte(`[{"zero":0,"fraction":1.20e+3,"false":false,"null":null,"empty":"","array":[],"object":{}}]`)
	value, err := New().Parse(context.Background(), input)
	if err != nil {
		t.Fatal(err)
	}
	if value.Kind != tailoring.JSONArray || len(value.ArrayValue) != 1 {
		t.Fatalf("value = %+v", value)
	}
	fields := value.ArrayValue[0].ObjectValue
	if fields[0].Value.NumberValue != "0" || fields[1].Value.NumberValue != "1.20e+3" || fields[2].Value.BoolValue || fields[3].Value.Kind != tailoring.JSONNull {
		t.Fatalf("typed values = %+v", fields)
	}
	if fields[5].Value.ArrayValue == nil || fields[6].Value.ObjectValue == nil {
		t.Fatalf("explicit empty values were lost: %+v", fields)
	}
}

func TestParseRejectsDuplicateMalformedTrailingAndInvalidUTF8(t *testing.T) {
	tests := [][]byte{
		[]byte(`{"a":1,"\u0061":2}`),
		[]byte(`{"nested":{"x":1,"x":2}}`),
		[]byte(`{"missing":`),
		[]byte(`{} {}`),
		{0xff},
	}
	for _, input := range tests {
		if _, err := New().Parse(context.Background(), input); err == nil {
			t.Errorf("Parse(%q) succeeded", input)
		}
	}
}

func TestParseRejectsDepthItemsAndByteLimits(t *testing.T) {
	deep := strings.Repeat("[", tailoring.MaxJSONDepth+2) + strings.Repeat("]", tailoring.MaxJSONDepth+2)
	items := "[" + strings.Repeat("null,", tailoring.MaxJSONArrayItems) + "null]"
	oversized := strings.Repeat(" ", MaxInputBytes) + "0"
	for _, input := range []string{deep, items, oversized} {
		if _, err := New().Parse(context.Background(), []byte(input)); err == nil {
			t.Fatal("oversized or deep source JSON succeeded")
		}
	}
}

func TestParseHonorsCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := New().Parse(ctx, []byte(`{}`)); !errors.Is(err, context.Canceled) {
		t.Fatalf("Parse() error = %v", err)
	}
}
