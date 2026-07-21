package tailoring

import (
	"errors"
	"reflect"
	"testing"
)

func sourceRecord(number string, active bool, note JSONValue) JSONValue {
	return NewJSONObject([]JSONField{
		{Name: "number", Value: NewJSONNumber(number)},
		{Name: "title", Value: NewJSONString("Example")},
		{Name: "state", Value: NewJSONString("OPEN")},
		{Name: "active", Value: NewJSONBool(active)},
		{Name: "note", Value: note},
	})
}

func TestTransformJSONPreservesShapeOrderAndTypedStates(t *testing.T) {
	plan := validPolicy().Output
	plan.Select = []string{"number", "active", "note"}
	plan.Rename = []Rename{{From: "number", To: "id"}}
	source := NewJSONArray([]JSONValue{
		sourceRecord("0", false, NewJSONNull()),
		sourceRecord("2", true, NewJSONString("")),
	})

	result, err := TransformJSON(plan, source)
	if err != nil {
		t.Fatal(err)
	}
	if result.Shape != ResultShapeArray || !reflect.DeepEqual(result.Fields, []string{"id", "active", "note"}) || len(result.Records) != 2 {
		t.Fatalf("result = %+v", result)
	}
	if result.Records[0].ObjectValue[0].Value.NumberValue != "0" || result.Records[0].ObjectValue[1].Value.BoolValue || result.Records[0].ObjectValue[2].Value.Kind != JSONNull {
		t.Fatalf("first record lost typed state: %+v", result.Records[0])
	}
	if result.Records[1].ObjectValue[2].Value.Kind != JSONString || result.Records[1].ObjectValue[2].Value.StringValue != "" {
		t.Fatalf("explicit empty string was lost: %+v", result.Records[1])
	}
}

func TestTransformJSONSupportsObjectAndEmptyArray(t *testing.T) {
	plan := validPolicy().Output
	object, err := TransformJSON(plan, sourceRecord("1", false, NewJSONNull()))
	if err != nil || object.Shape != ResultShapeObject || len(object.Records) != 1 {
		t.Fatalf("object = %+v, error = %v", object, err)
	}
	empty, err := TransformJSON(plan, NewJSONArray([]JSONValue{}))
	if err != nil || empty.Shape != ResultShapeArray || empty.Records == nil || len(empty.Records) != 0 {
		t.Fatalf("empty = %+v, error = %v", empty, err)
	}
}

func TestTransformJSONRejectsMissingFieldsAndWrongShapes(t *testing.T) {
	plan := validPolicy().Output
	tests := []JSONValue{
		NewJSONObject([]JSONField{{Name: "number", Value: NewJSONNumber("1")}}),
		NewJSONArray([]JSONValue{NewJSONString("not an object")}),
		NewJSONString("not a record"),
	}
	for _, source := range tests {
		if _, err := TransformJSON(plan, source); !errors.Is(err, ErrTransform) {
			t.Errorf("TransformJSON(%+v) error = %v", source, err)
		}
	}
}

func TestJSONValueValidationRejectsDuplicatesMalformedNumbersAndComplexity(t *testing.T) {
	values := []JSONValue{
		NewJSONObject([]JSONField{{Name: "same", Value: NewJSONNull()}, {Name: "same", Value: NewJSONNull()}}),
		NewJSONNumber("01"),
		{Kind: JSONArray},
		NewJSONArray(make([]JSONValue, MaxJSONArrayItems+1)),
	}
	for _, value := range values {
		if err := value.Validate(); !errors.Is(err, ErrInvalidJSONValue) {
			t.Errorf("Validate(%+v) error = %v", value, err)
		}
	}
}

func TestTransformJSONReturnsDetachedRecords(t *testing.T) {
	plan := validPolicy().Output
	source := sourceRecord("1", false, NewJSONNull())
	result, err := TransformJSON(plan, source)
	if err != nil {
		t.Fatal(err)
	}
	source.ObjectValue[0].Value.NumberValue = "99"
	if result.Records[0].ObjectValue[0].Value.NumberValue != "1" {
		t.Fatalf("result aliases source: %+v", result)
	}
}
