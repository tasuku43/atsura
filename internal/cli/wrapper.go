package cli

import (
	"context"
	"fmt"

	"github.com/tasuku43/atsura/internal/domain/fault"
	"github.com/tasuku43/atsura/internal/domain/operation"
	"github.com/tasuku43/atsura/internal/domain/tailoring"
	"github.com/tasuku43/atsura/internal/domain/wrapperbinding"
)

type wrapperRenderDocument struct {
	SchemaVersion int                  `json:"schema_version"`
	Wrapper       wrapperRenderPayload `json:"wrapper"`
}

type wrapperRenderPayload struct {
	Source                string                         `json:"source"`
	SourceSHA256          string                         `json:"source_sha256"`
	Command               string                         `json:"command"`
	Contract              wrapperRenderContract          `json:"contract"`
	Bundle                wrapperRenderBundle            `json:"bundle"`
	Runtime               wrapperbinding.RuntimeIdentity `json:"runtime"`
	SourceProcessAttempts int                            `json:"source_process_attempts"`
}

type wrapperRenderContract struct {
	Version int    `json:"version"`
	Shell   string `json:"shell"`
}

type wrapperRenderBundle struct {
	Locator string `json:"locator"`
	Digest  string `json:"digest"`
}

func runWrapperRender(ctx context.Context, c *CLI, _ CommandSpec, intent operation.Intent, inputs ParsedInputs) int {
	result, err := c.wrapperRenders.Render(ctx, intent, inputs.One("--bundle"))
	if err != nil {
		return c.fail(ctx, err)
	}
	if err := result.Binding.Validate(); err != nil {
		return c.fail(ctx, fault.Wrap(
			fault.KindContract,
			"output_contract_exceeded",
			"The wrapper renderer returned an invalid product binding.",
			false,
			err,
			fault.NextAction{Command: "help wrapper render", Reason: "Reduce the bounded generated wrapper output."},
		))
	}
	if err := result.Material.Validate(); err != nil {
		return c.fail(ctx, fault.Wrap(
			fault.KindContract,
			"output_contract_exceeded",
			"The wrapper renderer returned material outside its bounded zero-execution contract.",
			false,
			err,
			fault.NextAction{Command: "help wrapper render", Reason: "Reduce the bounded generated wrapper output."},
		))
	}
	if result.SourceProcessAttempts != 0 {
		return c.fail(ctx, fault.Wrap(
			fault.KindContract,
			"output_contract_exceeded",
			"The wrapper renderer returned material outside its bounded zero-execution contract.",
			false,
			fmt.Errorf("source_process_attempts is %d", result.SourceProcessAttempts),
			fault.NextAction{Command: "help wrapper render", Reason: "Reduce the bounded generated wrapper output."},
		))
	}

	if inputs.One("--format") == "text" {
		return c.emitResult(ctx, result.Material.Clone().Source)
	}
	document := wrapperRenderDocument{SchemaVersion: 1, Wrapper: wrapperRenderPayload{
		Source: string(result.Material.Source), SourceSHA256: result.Material.SHA256,
		Command:  result.Binding.CommandName,
		Contract: wrapperRenderContract{Version: result.Binding.ContractVersion, Shell: "posix"},
		Bundle:   wrapperRenderBundle{Locator: result.Binding.BundleLocator, Digest: result.Binding.BundleDigest},
		Runtime:  result.Binding.Runtime, SourceProcessAttempts: result.SourceProcessAttempts,
	}}
	return c.emitJSONDocument(ctx, document, "wrapper render")
}

func runWrapperRun(ctx context.Context, c *CLI, _ CommandSpec, intent operation.Intent, inputs ParsedInputs) int {
	if !inputs.PositionalOnlyMarkerUsed() {
		return c.failUsage(
			ctx,
			"invalid_arguments",
			"wrapper run requires the explicit -- boundary before forwarded argv, including an empty argv list.",
			"help wrapper run",
			"Use only the complete render-produced binding flags and forward argv after --.",
		)
	}
	contractVersion, _ := inputs.Integer("--contract-version")
	runtimeSize, _ := inputs.Integer("--runtime-size")
	result, err := c.wrapperRuns.Execute(ctx, intent, wrapperbinding.RuntimeInvocation{
		ContractVersion: int(contractVersion),
		BundleLocator:   inputs.One("--bundle"),
		BundleDigest:    inputs.One("--bundle-digest"),
		Runtime: wrapperbinding.RuntimeIdentity{
			ResolvedPath: inputs.One("--runtime-path"),
			SHA256:       inputs.One("--runtime-sha256"),
			Size:         runtimeSize,
		},
	}, inputs.Values("argv"))
	if err != nil {
		return c.fail(ctx, err)
	}
	if err := validateWrapperPlanResult(result.Output, result.Render, result.SourceProcessAttempts); err != nil {
		return c.fail(ctx, fault.Wrap(
			fault.KindContract,
			"output_contract_exceeded",
			"The fresh wrapper plan returned a result outside its declared compact JSON contract.",
			false,
			err,
			fault.NextAction{Command: "bundle preview", Reason: "Reduce the bounded transformed result; the source was not retried."},
		))
	}
	encoded, err := encodeWrapperPlanResult(result.Output, result.Render, result.SourceProcessAttempts)
	if err != nil {
		return c.fail(ctx, fault.Wrap(
			fault.KindContract,
			"output_encoding_failed",
			"The fresh wrapper plan result could not be encoded as one compact JSON value.",
			false,
			err,
			fault.NextAction{Command: "bundle preview", Reason: "Repair deterministic compact wrapper JSON; the source was not retried."},
		))
	}
	if len(encoded)+1 > maxBundleOutputBytes {
		return c.fail(ctx, fault.New(
			fault.KindContract,
			"output_contract_exceeded",
			"The plan-declared wrapper result exceeds its 2 MiB output limit.",
			false,
			fault.NextAction{Command: "bundle preview", Reason: "Reduce the bounded transformed result; the source was not retried."},
		))
	}
	return c.emitResult(ctx, append(encoded, '\n'))
}

func encodeWrapperPlanResult(output tailoring.OutputResult, render tailoring.RenderFormat, attempts int) ([]byte, error) {
	if err := validateWrapperPlanResult(output, render, attempts); err != nil {
		return nil, err
	}
	records := make([]tailoring.JSONValue, len(output.Records))
	for index := range output.Records {
		records[index] = projectExternalJSON(output.Records[index])
	}
	if output.Shape == tailoring.ResultShapeObject {
		return records[0].MarshalJSON()
	}
	return tailoring.NewJSONArray(records).MarshalJSON()
}

func validateWrapperPlanResult(output tailoring.OutputResult, render tailoring.RenderFormat, attempts int) error {
	if err := output.Validate(); err != nil {
		return err
	}
	if render != tailoring.RenderCompactJSON || attempts != 1 {
		return tailoring.ErrTransform
	}
	return nil
}
