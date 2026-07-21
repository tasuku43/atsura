// Package catalogjson loads the exact JSON envelope emitted by source inspect.
package catalogjson

import (
	"context"
	"errors"

	"github.com/tasuku43/atsura/internal/domain/fault"
	"github.com/tasuku43/atsura/internal/domain/sourcecatalog"
	"github.com/tasuku43/atsura/internal/infra/localfile"
	"github.com/tasuku43/atsura/internal/infra/strictjson"
)

const maxCatalogBytes = int64(1024 * 1024)

type Loader struct{}

func New() *Loader { return &Loader{} }

type document struct {
	SchemaVersion int        `json:"schema_version"`
	Inspection    inspection `json:"inspection"`
}

type inspection struct {
	CatalogDigest         string                `json:"catalog_digest"`
	Catalog               sourcecatalog.Catalog `json:"catalog"`
	SourceProcessAttempts int                   `json:"source_process_attempts"`
}

func (l *Loader) Load(ctx context.Context, path string) (sourcecatalog.Catalog, error) {
	raw, err := localfile.Read(ctx, path, maxCatalogBytes)
	if err != nil {
		return sourcecatalog.Catalog{}, fileFault(err)
	}
	var value document
	if err := strictjson.Decode(raw, &value, 64); err != nil {
		return sourcecatalog.Catalog{}, fault.Wrap(fault.KindInvalidInput, "invalid_catalog_file", "The source inspection JSON is invalid.", false, err, helpAction())
	}
	if value.SchemaVersion != 1 || value.Inspection.SourceProcessAttempts != value.Inspection.Catalog.Probe.Attempts {
		return sourcecatalog.Catalog{}, fault.New(fault.KindInvalidInput, "invalid_catalog_file", "The source inspection JSON does not match schema 1.", false, helpAction())
	}
	if err := value.Inspection.Catalog.Validate(); err != nil {
		return sourcecatalog.Catalog{}, fault.Wrap(fault.KindInvalidInput, "invalid_catalog_file", "The source inspection catalog is invalid.", false, err, helpAction())
	}
	digest, err := value.Inspection.Catalog.Digest()
	if err != nil || digest != value.Inspection.CatalogDigest {
		return sourcecatalog.Catalog{}, fault.Wrap(fault.KindRejected, "catalog_digest_mismatch", "The source inspection catalog digest does not match its content.", false, err, helpAction())
	}
	return value.Inspection.Catalog, nil
}

func fileFault(err error) error {
	switch {
	case errors.Is(err, localfile.ErrNotFound):
		return fault.Wrap(fault.KindNotFound, "catalog_file_not_found", "The source inspection JSON was not found.", false, err, helpAction())
	case errors.Is(err, localfile.ErrPermission):
		return fault.Wrap(fault.KindPermission, "catalog_file_permission_denied", "The source inspection JSON cannot be read.", false, err, helpAction())
	case errors.Is(err, localfile.ErrUnsafe):
		return fault.Wrap(fault.KindInvalidInput, "unsafe_catalog_file", "The source inspection JSON must be a stable regular file, not a symbolic link.", false, err, helpAction())
	case errors.Is(err, localfile.ErrTooLarge):
		return fault.Wrap(fault.KindInvalidInput, "catalog_file_too_large", "The source inspection JSON exceeds 1 MiB.", false, err, helpAction())
	default:
		return fault.Wrap(fault.KindUnavailable, "catalog_file_read_failed", "The source inspection JSON could not be read.", true, err, helpAction())
	}
}

func helpAction() fault.NextAction {
	return fault.NextAction{Command: "source inspect", Reason: "Generate a fresh bounded source inspection JSON document."}
}
