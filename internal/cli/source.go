package cli

import (
	"context"
	"encoding/json"

	"github.com/tasuku43/atsura/internal/app/sourceinspect"
	"github.com/tasuku43/atsura/internal/domain/fault"
	"github.com/tasuku43/atsura/internal/domain/operation"
	"github.com/tasuku43/atsura/internal/domain/sourcecatalog"
)

const maxSourceInspectionBytes = 1024 * 1024

type sourceInspectionDocument struct {
	SchemaVersion int                     `json:"schema_version"`
	Inspection    sourceInspectionPayload `json:"inspection"`
}

type sourceInspectionPayload struct {
	CatalogDigest         string                `json:"catalog_digest"`
	Catalog               sourcecatalog.Catalog `json:"catalog"`
	SourceProcessAttempts int                   `json:"source_process_attempts"`
}

func runSourceInspect(ctx context.Context, c *CLI, _ CommandSpec, intent operation.Intent, inputs ParsedInputs) int {
	result, err := c.sources.Inspect(ctx, intent, inputs.One("--adapter"), inputs.One("--executable"))
	if err != nil {
		return c.fail(ctx, err)
	}
	output, err := renderSourceInspection(result)
	if err != nil {
		return c.fail(ctx, err)
	}
	return c.emitResult(ctx, output)
}

func renderSourceInspection(result sourceinspect.Result) ([]byte, error) {
	document := sourceInspectionDocument{SchemaVersion: 1, Inspection: sourceInspectionPayload{
		CatalogDigest: result.Digest, Catalog: result.Catalog,
		SourceProcessAttempts: result.Catalog.Probe.Attempts,
	}}
	encoded, err := json.Marshal(document)
	if err != nil {
		return nil, fault.Wrap(fault.KindContract, "output_encoding_failed", "The source catalog JSON could not be encoded.", false, err)
	}
	if len(encoded)+1 > maxSourceInspectionBytes {
		return nil, outputContractExceeded("The source catalog exceeds its declared byte limit.", "source inspect")
	}
	return append(encoded, '\n'), nil
}
