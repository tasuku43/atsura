package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"

	"github.com/tasuku43/atsura/internal/app/tailorrun"
	"github.com/tasuku43/atsura/internal/domain/fault"
	"github.com/tasuku43/atsura/internal/domain/operation"
	"github.com/tasuku43/atsura/internal/domain/tailoring"
)

const maxRunOutputBytes = 4 * 1024 * 1024

type runDocument struct {
	SchemaVersion int                 `json:"schema_version"`
	Execution     runExecutionPayload `json:"execution"`
}

type runExecutionPayload struct {
	Decision              string          `json:"decision"`
	MatchedCommand        string          `json:"matched_command"`
	Reason                string          `json:"reason"`
	ResultShape           string          `json:"result_shape"`
	Fields                []string        `json:"fields"`
	Records               json.RawMessage `json:"records"`
	SourceProcessAttempts int             `json:"source_process_attempts"`
}

func runTailored(ctx context.Context, c *CLI, _ CommandSpec, intent operation.Intent, inputs ParsedInputs) int {
	result, err := c.runs.Run(ctx, intent, inputs.One("--config"), inputs.Values("command"))
	if err != nil {
		return c.fail(ctx, err)
	}
	output, err := renderRunResult(result)
	if err != nil {
		return c.fail(ctx, err)
	}
	if len(result.SourceStderr) > 0 {
		warning := []byte("source_stderr: " + safeExternalBytes(result.SourceStderr) + "\n")
		if _, err := writeOnce(c.Err, warning); err != nil {
			return c.fail(ctx, fault.Wrap(
				fault.KindInternal,
				"source_stderr_write_failed",
				"The successful source stderr could not be written.",
				true,
				err,
				fault.NextAction{Command: "run", Reason: "Retry the declared read after selecting a writable stderr stream."},
			))
		}
	}
	return c.emitResult(ctx, output)
}

func renderRunResult(result tailorrun.Result) ([]byte, error) {
	if err := result.Validate(); err != nil {
		return nil, fault.Wrap(fault.KindContract, "invalid_run_result", "The tailored run result is invalid.", false, err)
	}
	records, err := renderJSONRecords(result.Output.Records)
	if err != nil {
		return nil, fault.Wrap(fault.KindContract, "output_encoding_failed", "The tailored run JSON could not be encoded.", false, err)
	}
	document := runDocument{SchemaVersion: 1, Execution: runExecutionPayload{
		Decision: string(result.Plan.Decision), MatchedCommand: result.Plan.MatchedCommand,
		Reason: safeExternalText(result.Plan.Reason), ResultShape: string(result.Output.Shape),
		Fields: append([]string{}, result.Output.Fields...), Records: records,
		SourceProcessAttempts: result.SourceProcessAttempts,
	}}
	encoded, err := json.Marshal(document)
	if err != nil {
		return nil, fault.Wrap(fault.KindContract, "output_encoding_failed", "The tailored run JSON could not be encoded.", false, err)
	}
	if len(encoded)+1 > maxRunOutputBytes {
		return nil, outputContractExceeded("The tailored run output exceeds the 4 MiB limit.", "run")
	}
	return append(encoded, '\n'), nil
}

func renderJSONRecords(records []tailoring.JSONValue) (json.RawMessage, error) {
	var output bytes.Buffer
	output.WriteByte('[')
	for index, record := range records {
		if index > 0 {
			output.WriteByte(',')
		}
		if err := renderJSONValue(&output, record); err != nil {
			return nil, err
		}
	}
	output.WriteByte(']')
	return json.RawMessage(append([]byte{}, output.Bytes()...)), nil
}

func renderJSONValue(output *bytes.Buffer, value tailoring.JSONValue) error {
	if err := value.Validate(); err != nil {
		return err
	}
	switch value.Kind {
	case tailoring.JSONNull:
		output.WriteString("null")
	case tailoring.JSONBool:
		if value.BoolValue {
			output.WriteString("true")
		} else {
			output.WriteString("false")
		}
	case tailoring.JSONNumber:
		output.WriteString(value.NumberValue)
	case tailoring.JSONString:
		encoded, _ := json.Marshal(value.StringValue)
		output.Write(encoded)
	case tailoring.JSONArray:
		output.WriteByte('[')
		for index, item := range value.ArrayValue {
			if index > 0 {
				output.WriteByte(',')
			}
			if err := renderJSONValue(output, item); err != nil {
				return err
			}
		}
		output.WriteByte(']')
	case tailoring.JSONObject:
		output.WriteByte('{')
		for index, field := range value.ObjectValue {
			if index > 0 {
				output.WriteByte(',')
			}
			name, _ := json.Marshal(field.Name)
			output.Write(name)
			output.WriteByte(':')
			if err := renderJSONValue(output, field.Value); err != nil {
				return err
			}
		}
		output.WriteByte('}')
	default:
		return fmt.Errorf("unsupported typed JSON kind %q", value.Kind)
	}
	return nil
}
