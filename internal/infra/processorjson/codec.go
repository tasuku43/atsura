// Package processorjson strictly encodes and loads the schema-1 processor
// inspection document emitted by the public CLI.
package processorjson

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/tasuku43/atsura/internal/domain/fault"
	"github.com/tasuku43/atsura/internal/domain/processorprocess"
	"github.com/tasuku43/atsura/internal/infra/localfile"
	"github.com/tasuku43/atsura/internal/infra/strictjson"
)

const MaxObservationBytes = int64(64 * 1024)

const documentSchemaVersion = 1

type document struct {
	SchemaVersion int               `json:"schema_version"`
	Inspection    inspectionPayload `json:"inspection"`
}

type inspectionPayload struct {
	ObservationDigest        string                       `json:"observation_digest"`
	Observation              processorprocess.Observation `json:"observation"`
	ProcessorProcessAttempts int                          `json:"processor_process_attempts"`
}

// Encode returns the canonical compact LF-terminated public inspection
// document. The attempt count is derived from the validated observation.
func Encode(observation processorprocess.Observation) ([]byte, error) {
	digest, err := observation.Digest()
	if err != nil {
		return nil, err
	}
	encoded, err := json.Marshal(document{
		SchemaVersion: documentSchemaVersion,
		Inspection: inspectionPayload{
			ObservationDigest: digest, Observation: observation,
			ProcessorProcessAttempts: observation.Probe.Attempts,
		},
	})
	if err != nil {
		return nil, fmt.Errorf("encode canonical processor inspection document: %w", err)
	}
	return append(encoded, '\n'), nil
}

// Decode rejects duplicate and unknown fields, trailing values, excessive
// depth or bytes, digest/attempt mismatches, and every invalid observation.
func Decode(raw []byte) (processorprocess.Observation, error) {
	if len(raw) == 0 || int64(len(raw)) > MaxObservationBytes {
		return processorprocess.Observation{}, fmt.Errorf("%w: observation JSON must be non-empty and at most %d bytes", processorprocess.ErrInvalidObservation, MaxObservationBytes)
	}
	var value document
	if err := strictjson.Decode(raw, &value, 12); err != nil {
		return processorprocess.Observation{}, fmt.Errorf("%w: %v", processorprocess.ErrInvalidObservation, err)
	}
	if value.SchemaVersion != documentSchemaVersion {
		return processorprocess.Observation{}, fmt.Errorf("%w: inspection document schema_version must be %d", processorprocess.ErrInvalidObservation, documentSchemaVersion)
	}
	observation := value.Inspection.Observation
	if err := observation.Validate(); err != nil {
		return processorprocess.Observation{}, fmt.Errorf("%w: %v", processorprocess.ErrInvalidObservation, err)
	}
	digest, err := observation.Digest()
	if err != nil || value.Inspection.ObservationDigest != digest {
		return processorprocess.Observation{}, fmt.Errorf("%w: observation digest is invalid or mismatched", processorprocess.ErrInvalidObservation)
	}
	if value.Inspection.ProcessorProcessAttempts != 1 || value.Inspection.ProcessorProcessAttempts != observation.Probe.Attempts {
		return processorprocess.Observation{}, fmt.Errorf("%w: processor process attempts are invalid or mismatched", processorprocess.ErrInvalidObservation)
	}
	return observation, nil
}

// Loader reads one explicitly selected stable regular observation file.
type Loader struct{}

// New creates a strict processor-observation loader.
func New() *Loader { return &Loader{} }

// Load reads and validates one observation without discovering a processor or
// starting any process.
func (l *Loader) Load(ctx context.Context, path string) (processorprocess.Observation, error) {
	raw, err := localfile.Read(ctx, path, MaxObservationBytes)
	if err != nil {
		return processorprocess.Observation{}, fileFault(err)
	}
	observation, err := Decode(raw)
	if err != nil {
		return processorprocess.Observation{}, fault.Wrap(fault.KindInvalidInput, "invalid_processor_observation_file", "The processor observation JSON is invalid.", false, err, helpAction())
	}
	return observation, nil
}

func fileFault(err error) error {
	switch {
	case errors.Is(err, localfile.ErrNotFound):
		return fault.Wrap(fault.KindNotFound, "processor_observation_file_not_found", "The processor observation JSON was not found.", false, err, helpAction())
	case errors.Is(err, localfile.ErrPermission):
		return fault.Wrap(fault.KindPermission, "processor_observation_file_permission_denied", "The processor observation JSON cannot be read.", false, err, helpAction())
	case errors.Is(err, localfile.ErrUnsafe):
		return fault.Wrap(fault.KindInvalidInput, "unsafe_processor_observation_file", "The processor observation JSON must be a stable regular file, not a symbolic link.", false, err, helpAction())
	case errors.Is(err, localfile.ErrTooLarge):
		return fault.Wrap(fault.KindInvalidInput, "processor_observation_file_too_large", "The processor observation JSON exceeds 64 KiB.", false, err, helpAction())
	default:
		return fault.Wrap(fault.KindUnavailable, "processor_observation_file_read_failed", "The processor observation JSON could not be read.", true, err, helpAction())
	}
}

func helpAction() fault.NextAction {
	return fault.NextAction{Command: "processor inspect", Reason: "Generate a fresh exact processor observation JSON document."}
}
