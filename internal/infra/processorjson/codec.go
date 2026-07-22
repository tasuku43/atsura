// Package processorjson strictly encodes and loads schema-1 processor
// observations.
package processorjson

import (
	"context"
	"errors"
	"fmt"

	"github.com/tasuku43/atsura/internal/domain/fault"
	"github.com/tasuku43/atsura/internal/domain/processorprocess"
	"github.com/tasuku43/atsura/internal/infra/localfile"
	"github.com/tasuku43/atsura/internal/infra/strictjson"
)

const MaxObservationBytes = int64(64 * 1024)

// Encode returns the canonical compact LF-terminated observation bytes.
func Encode(observation processorprocess.Observation) ([]byte, error) {
	return observation.CanonicalJSON()
}

// Decode rejects duplicate and unknown fields, trailing values, excessive
// depth or bytes, and every invalid domain observation.
func Decode(raw []byte) (processorprocess.Observation, error) {
	if len(raw) == 0 || int64(len(raw)) > MaxObservationBytes {
		return processorprocess.Observation{}, fmt.Errorf("%w: observation JSON must be non-empty and at most %d bytes", processorprocess.ErrInvalidObservation, MaxObservationBytes)
	}
	var observation processorprocess.Observation
	if err := strictjson.Decode(raw, &observation, 8); err != nil {
		return processorprocess.Observation{}, fmt.Errorf("%w: %v", processorprocess.ErrInvalidObservation, err)
	}
	if err := observation.Validate(); err != nil {
		return processorprocess.Observation{}, fmt.Errorf("%w: %v", processorprocess.ErrInvalidObservation, err)
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
