package cli

import (
	"context"
	"encoding/json"

	"github.com/tasuku43/atsura/internal/app/processorinspect"
	"github.com/tasuku43/atsura/internal/domain/fault"
	"github.com/tasuku43/atsura/internal/domain/operation"
	"github.com/tasuku43/atsura/internal/domain/processorprocess"
)

const maxProcessorInspectionBytes = 64 * 1024

type processorInspectionDocument struct {
	SchemaVersion int                        `json:"schema_version"`
	Inspection    processorInspectionPayload `json:"inspection"`
}

type processorInspectionPayload struct {
	ObservationDigest        string                       `json:"observation_digest"`
	Observation              processorprocess.Observation `json:"observation"`
	ProcessorProcessAttempts int                          `json:"processor_process_attempts"`
}

func runProcessorInspect(ctx context.Context, c *CLI, _ CommandSpec, intent operation.Intent, inputs ParsedInputs) int {
	result, err := c.processors.Inspect(ctx, intent, inputs.One("--adapter"), inputs.One("--executable"))
	if err != nil {
		return c.fail(ctx, err)
	}
	output, err := renderProcessorInspection(result)
	if err != nil {
		return c.fail(ctx, err)
	}
	return c.emitResult(ctx, output)
}

func renderProcessorInspection(result processorinspect.Result) ([]byte, error) {
	if err := result.Observation.Validate(); err != nil {
		return nil, fault.Wrap(
			fault.KindContract,
			"invalid_processor_observation",
			"The processor inspection result is invalid.",
			false,
			err,
			fault.NextAction{Command: "help processor inspect", Reason: "Inspect the exact processor observation contract."},
		)
	}
	digest, err := result.Observation.Digest()
	if err != nil || digest != result.Digest || result.Observation.Probe.Attempts != 1 {
		return nil, fault.Wrap(
			fault.KindContract,
			"invalid_processor_observation",
			"The processor observation digest or attempt evidence is inconsistent.",
			false,
			err,
			fault.NextAction{Command: "help processor inspect", Reason: "Inspect the exact processor observation contract."},
		)
	}
	document := processorInspectionDocument{
		SchemaVersion: 1,
		Inspection: processorInspectionPayload{
			ObservationDigest:        result.Digest,
			Observation:              result.Observation,
			ProcessorProcessAttempts: result.Observation.Probe.Attempts,
		},
	}
	encoded, err := json.Marshal(document)
	if err != nil {
		return nil, fault.Wrap(fault.KindContract, "output_encoding_failed", "The processor observation JSON could not be encoded.", false, err)
	}
	if len(encoded)+1 > maxProcessorInspectionBytes {
		return nil, outputContractExceeded("The processor observation exceeds its declared byte limit.", "processor inspect")
	}
	return append(encoded, '\n'), nil
}
