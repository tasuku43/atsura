package tailoring

import (
	"errors"
	"fmt"
	"reflect"
)

var ErrTransform = errors.New("source JSON does not satisfy output plan")

// ResultShape preserves whether the source returned one object or an array.
type ResultShape string

const (
	ResultShapeObject ResultShape = "object"
	ResultShapeArray  ResultShape = "array"
)

// OutputResult contains transformed typed records and their declared order.
type OutputResult struct {
	Shape   ResultShape
	Fields  []string
	Records []JSONValue
}

// Validate rejects presentation-inconsistent transformed results.
func (r OutputResult) Validate() error {
	if r.Shape != ResultShapeObject && r.Shape != ResultShapeArray {
		return fmt.Errorf("%w: result shape is invalid", ErrTransform)
	}
	if r.Fields == nil || r.Records == nil {
		return fmt.Errorf("%w: fields and records must be explicit", ErrTransform)
	}
	if r.Shape == ResultShapeObject && len(r.Records) != 1 {
		return fmt.Errorf("%w: object shape requires exactly one record", ErrTransform)
	}
	for index, record := range r.Records {
		if err := record.Validate(); err != nil || record.Kind != JSONObject {
			return fmt.Errorf("%w: record %d is invalid", ErrTransform, index)
		}
		names := make([]string, len(record.ObjectValue))
		for fieldIndex, field := range record.ObjectValue {
			names[fieldIndex] = field.Name
		}
		if !reflect.DeepEqual(names, r.Fields) {
			return fmt.Errorf("%w: record %d fields do not match declared order", ErrTransform, index)
		}
	}
	return nil
}

// TransformJSON applies the schema-1 select and rename plan to an object or
// array of objects without parsing bytes or performing I/O.
func TransformJSON(plan OutputPlan, source JSONValue) (OutputResult, error) {
	if err := plan.Validate(); err != nil {
		return OutputResult{}, err
	}
	if err := source.Validate(); err != nil {
		return OutputResult{}, err
	}
	fields := finalFieldNames(plan)
	result := OutputResult{Fields: fields, Records: []JSONValue{}}
	switch source.Kind {
	case JSONObject:
		result.Shape = ResultShapeObject
		record, err := transformRecord(plan, source)
		if err != nil {
			return OutputResult{}, err
		}
		result.Records = append(result.Records, record)
	case JSONArray:
		result.Shape = ResultShapeArray
		for index, item := range source.ArrayValue {
			if item.Kind != JSONObject {
				return OutputResult{}, fmt.Errorf("%w: array item %d is not an object", ErrTransform, index)
			}
			record, err := transformRecord(plan, item)
			if err != nil {
				return OutputResult{}, fmt.Errorf("array item %d: %w", index, err)
			}
			result.Records = append(result.Records, record)
		}
	default:
		return OutputResult{}, fmt.Errorf("%w: top-level value must be an object or array", ErrTransform)
	}
	if err := result.Validate(); err != nil {
		return OutputResult{}, err
	}
	return result, nil
}

func finalFieldNames(plan OutputPlan) []string {
	renames := make(map[string]string, len(plan.Rename))
	for _, rename := range plan.Rename {
		renames[rename.From] = rename.To
	}
	fields := make([]string, len(plan.Select))
	for index, field := range plan.Select {
		fields[index] = field
		if renamed, exists := renames[field]; exists {
			fields[index] = renamed
		}
	}
	return fields
}

func transformRecord(plan OutputPlan, source JSONValue) (JSONValue, error) {
	byName := make(map[string]JSONValue, len(source.ObjectValue))
	for _, field := range source.ObjectValue {
		byName[field.Name] = field.Value
	}
	names := finalFieldNames(plan)
	fields := make([]JSONField, len(plan.Select))
	for index, selected := range plan.Select {
		value, exists := byName[selected]
		if !exists {
			return JSONValue{}, fmt.Errorf("%w: selected field %q is missing", ErrTransform, selected)
		}
		fields[index] = JSONField{Name: names[index], Value: cloneJSONValue(value)}
	}
	return NewJSONObject(fields), nil
}
