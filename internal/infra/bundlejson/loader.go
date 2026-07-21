// Package bundlejson loads the exact JSON envelope emitted by bundle build.
package bundlejson

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/tasuku43/atsura/internal/domain/fault"
	"github.com/tasuku43/atsura/internal/domain/tailoringbundle"
	"github.com/tasuku43/atsura/internal/infra/localfile"
	"github.com/tasuku43/atsura/internal/infra/strictjson"
)

const maxBundleBytes = int64(2 * 1024 * 1024)

type Loader struct{}

func New() *Loader { return &Loader{} }

type document struct {
	SchemaVersion int     `json:"schema_version"`
	Build         payload `json:"build"`
}

type payload struct {
	BundleDigest string                 `json:"bundle_digest"`
	Bundle       tailoringbundle.Bundle `json:"bundle"`
}

func (l *Loader) Load(ctx context.Context, path string) (tailoringbundle.Bundle, string, error) {
	raw, err := localfile.Read(ctx, path, maxBundleBytes)
	if err != nil {
		return tailoringbundle.Bundle{}, "", fileFault(err)
	}
	var header struct {
		SchemaVersion int             `json:"schema_version"`
		Build         json.RawMessage `json:"build"`
	}
	if err := strictjson.Decode(raw, &header, 96); err != nil {
		return tailoringbundle.Bundle{}, "", fault.Wrap(fault.KindInvalidInput, "invalid_bundle_file", "The bundle build JSON is invalid.", false, err, helpAction())
	}
	if header.SchemaVersion == 1 {
		return tailoringbundle.Bundle{}, "", fault.New(fault.KindInvalidInput, "legacy_tailoring_schema", "Bundle build schema 1 used the retired authorization model and cannot be adopted as schema 2.", false, helpAction())
	}
	if header.SchemaVersion != 2 {
		return tailoringbundle.Bundle{}, "", fault.New(fault.KindInvalidInput, "invalid_bundle_file", "The bundle build JSON must use schema version 2.", false, helpAction())
	}
	var value document
	if err := strictjson.Decode(raw, &value, 96); err != nil {
		return tailoringbundle.Bundle{}, "", fault.Wrap(fault.KindInvalidInput, "invalid_bundle_file", "The bundle build JSON is invalid.", false, err, helpAction())
	}
	if err := value.Build.Bundle.Validate(); err != nil {
		return tailoringbundle.Bundle{}, "", fault.Wrap(fault.KindInvalidInput, "invalid_bundle_file", "The embedded tailoring bundle is invalid.", false, err, helpAction())
	}
	digest, err := value.Build.Bundle.Digest()
	if err != nil || digest != value.Build.BundleDigest {
		return tailoringbundle.Bundle{}, "", fault.Wrap(fault.KindRejected, "bundle_digest_mismatch", "The bundle digest does not match its canonical content.", false, err, helpAction())
	}
	return value.Build.Bundle, digest, nil
}

func fileFault(err error) error {
	switch {
	case errors.Is(err, localfile.ErrNotFound):
		return fault.Wrap(fault.KindNotFound, "bundle_file_not_found", "The bundle build JSON was not found.", false, err, helpAction())
	case errors.Is(err, localfile.ErrPermission):
		return fault.Wrap(fault.KindPermission, "bundle_file_permission_denied", "The bundle build JSON cannot be read.", false, err, helpAction())
	case errors.Is(err, localfile.ErrUnsafe):
		return fault.Wrap(fault.KindInvalidInput, "unsafe_bundle_file", "The bundle build JSON must be a stable regular file, not a symbolic link.", false, err, helpAction())
	case errors.Is(err, localfile.ErrTooLarge):
		return fault.Wrap(fault.KindInvalidInput, "bundle_file_too_large", "The bundle build JSON exceeds 2 MiB.", false, err, helpAction())
	default:
		return fault.Wrap(fault.KindUnavailable, "bundle_file_read_failed", "The bundle build JSON could not be read.", true, err, helpAction())
	}
}

func helpAction() fault.NextAction {
	return fault.NextAction{Command: "bundle build", Reason: "Build and select a valid canonical bundle document."}
}
