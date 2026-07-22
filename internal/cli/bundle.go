package cli

import (
	"context"
	"encoding/json"

	"github.com/tasuku43/atsura/internal/app/bundleauthority"
	"github.com/tasuku43/atsura/internal/domain/bundletrust"
	"github.com/tasuku43/atsura/internal/domain/fault"
	"github.com/tasuku43/atsura/internal/domain/operation"
	"github.com/tasuku43/atsura/internal/domain/tailoring"
	"github.com/tasuku43/atsura/internal/domain/tailoringbundle"
	"github.com/tasuku43/atsura/internal/domain/tailoringplan"
	"github.com/tasuku43/atsura/internal/infra/specyaml"
)

const maxBundleOutputBytes = 2 * 1024 * 1024

type specificationValidationDocument struct {
	SchemaVersion int                            `json:"schema_version"`
	Validation    specificationValidationPayload `json:"validation"`
}

type specificationValidationPayload struct {
	Valid                 bool                          `json:"valid"`
	CatalogDigest         string                        `json:"catalog_digest"`
	SpecificationDigest   string                        `json:"specification_digest"`
	CommandCount          int                           `json:"command_count"`
	IncludedCount         int                           `json:"included_count"`
	ExcludedCount         int                           `json:"excluded_count"`
	IdentityWrapperCount  int                           `json:"identity_wrapper_count"`
	TransformWrapperCount int                           `json:"transform_wrapper_count"`
	Specification         tailoringbundle.Specification `json:"specification"`
}

type bundleBuildDocument struct {
	SchemaVersion int                `json:"schema_version"`
	Build         bundleBuildPayload `json:"build"`
}

type bundleBuildPayload struct {
	BundleDigest string                 `json:"bundle_digest"`
	Bundle       tailoringbundle.Bundle `json:"bundle"`
}

type bundleStatusDocument struct {
	SchemaVersion int                 `json:"schema_version"`
	Status        bundleStatusPayload `json:"status"`
}

type bundleStatusPayload struct {
	BundleDigest             string                  `json:"bundle_digest"`
	CatalogDigest            string                  `json:"catalog_digest"`
	SpecificationDigest      string                  `json:"specification_digest"`
	Adoption                 bundletrust.State       `json:"adoption"`
	Source                   bundletrust.SourceState `json:"source"`
	Adopted                  bool                    `json:"adopted"`
	SourcePath               string                  `json:"source_path"`
	SourceSHA256             string                  `json:"source_sha256"`
	SourceVersion            string                  `json:"source_version"`
	Processors               []bundleProcessorStatus `json:"processors"`
	SourceProcessAttempts    int                     `json:"source_process_attempts"`
	ProcessorProcessAttempts int                     `json:"processor_process_attempts"`
}

type bundleTrustDocument struct {
	SchemaVersion int                `json:"schema_version"`
	Trust         bundleTrustPayload `json:"trust"`
}

type bundleTrustPayload struct {
	BundleDigest             string                  `json:"bundle_digest"`
	Adopted                  bool                    `json:"adopted"`
	AlreadyAdopted           bool                    `json:"already_adopted"`
	Source                   bundletrust.SourceState `json:"source"`
	Processors               []bundleProcessorStatus `json:"processors"`
	SourceProcessAttempts    int                     `json:"source_process_attempts"`
	ProcessorProcessAttempts int                     `json:"processor_process_attempts"`
}

type bundleProcessorStatus struct {
	Contract     string                     `json:"contract"`
	AdapterKind  string                     `json:"adapter_kind"`
	Version      string                     `json:"version"`
	ResolvedPath string                     `json:"resolved_path"`
	SHA256       string                     `json:"sha256"`
	Size         int64                      `json:"size"`
	State        bundletrust.ProcessorState `json:"state"`
}

type bundlePreviewDocument struct {
	SchemaVersion int                  `json:"schema_version"`
	Preview       bundlePreviewPayload `json:"preview"`
}

type bundlePreviewPayload struct {
	PlanDigest            string             `json:"plan_digest"`
	Plan                  tailoringplan.Plan `json:"plan"`
	SourceProcessAttempts int                `json:"source_process_attempts"`
}

type bundleExecutionDocument struct {
	SchemaVersion int                    `json:"schema_version"`
	Execution     bundleExecutionPayload `json:"execution"`
}

type bundleExecutionPayload struct {
	BundleDigest          string                      `json:"bundle_digest"`
	PlanDigest            string                      `json:"plan_digest"`
	MatchedCommand        []string                    `json:"matched_command"`
	WrapperKind           tailoringbundle.WrapperKind `json:"wrapper_kind"`
	Output                bundleExecutionOutput       `json:"output"`
	Source                bundleExecutionSource       `json:"source"`
	SourceProcessAttempts int                         `json:"source_process_attempts"`
}

type bundleExecutionOutput struct {
	Render  tailoring.RenderFormat `json:"render"`
	Shape   tailoring.ResultShape  `json:"shape"`
	Fields  []string               `json:"fields"`
	Records []tailoring.JSONValue  `json:"records"`
}

type bundleExecutionSource struct {
	ExitCode int `json:"exit_code"`
}

func runSpecValidate(ctx context.Context, c *CLI, _ CommandSpec, intent operation.Intent, inputs ParsedInputs) int {
	result, err := c.bundles.ValidateSpecification(ctx, intent, inputs.One("--catalog"), inputs.One("--spec"))
	if err != nil {
		return c.fail(ctx, err)
	}
	document := specificationValidationDocument{SchemaVersion: 2, Validation: specificationValidationPayload{
		Valid: true, CatalogDigest: result.Specification.CatalogDigest, SpecificationDigest: result.SpecificationDigest,
		CommandCount: result.CommandCount, IncludedCount: result.IncludedCount, ExcludedCount: result.ExcludedCount,
		IdentityWrapperCount: result.IdentityWrapperCount, TransformWrapperCount: result.TransformWrapperCount,
		Specification: result.Specification,
	}}
	return c.emitJSONDocument(ctx, document, "spec validate")
}

func runSpecInit(ctx context.Context, c *CLI, _ CommandSpec, intent operation.Intent, inputs ParsedInputs) int {
	specification, err := c.drafts.Init(ctx, intent, inputs.One("--catalog"), inputs.Values("command"), inputs.Values("--processor")...)
	if err != nil {
		return c.fail(ctx, err)
	}
	encoded, err := specyaml.Encode(specification)
	if err != nil {
		return c.fail(ctx, fault.Wrap(fault.KindContract, "output_encoding_failed", "The schema-5 YAML draft could not be encoded.", false, err))
	}
	if len(encoded) > 256*1024 {
		return c.fail(ctx, outputContractExceeded("The schema-5 YAML draft exceeds 256 KiB.", "spec init"))
	}
	return c.emitResult(ctx, encoded)
}

func runBundleBuild(ctx context.Context, c *CLI, _ CommandSpec, intent operation.Intent, inputs ParsedInputs) int {
	result, err := c.bundles.Build(ctx, intent, inputs.One("--catalog"), inputs.One("--spec"), inputs.Values("--processor")...)
	if err != nil {
		return c.fail(ctx, err)
	}
	document := bundleBuildDocument{SchemaVersion: 2, Build: bundleBuildPayload{BundleDigest: result.BundleDigest, Bundle: result.Bundle}}
	return c.emitJSONDocument(ctx, document, "bundle build")
}

func runBundleStatus(ctx context.Context, c *CLI, _ CommandSpec, intent operation.Intent, inputs ParsedInputs) int {
	result, err := c.authority.Status(ctx, intent, inputs.One("--bundle"))
	if err != nil {
		return c.fail(ctx, err)
	}
	document := bundleStatusDocument{SchemaVersion: 3, Status: bundleStatusPayload{
		BundleDigest: result.BundleDigest, CatalogDigest: result.CatalogDigest, SpecificationDigest: result.SpecificationDigest,
		Adoption: result.Adoption, Source: result.Source, Adopted: result.Adopted, SourcePath: result.SourcePath,
		SourceSHA256: result.SourceSHA256, SourceVersion: result.SourceVersion, Processors: projectBundleProcessorStatuses(result.Processors),
		SourceProcessAttempts: result.SourceProcessAttempts, ProcessorProcessAttempts: result.ProcessorProcessAttempts,
	}}
	return c.emitJSONDocument(ctx, document, "bundle status")
}

func runBundleTrust(ctx context.Context, c *CLI, spec CommandSpec, intent operation.Intent, inputs ParsedInputs) int {
	if spec.Agent.FixedTarget == nil || spec.Agent.Mutation == nil {
		return c.fail(ctx, fault.New(fault.KindContract, "invalid_mutation_contract", "The bundle adoption mutation contract is incomplete.", false))
	}
	intent.Target = operation.TargetRef{Kind: spec.Agent.FixedTarget.Kind, ID: spec.Agent.FixedTarget.ID}
	intent.Impact = spec.Agent.Mutation.Impact
	result, err := c.authority.Trust(ctx, intent, inputs.One("--bundle"))
	if err != nil {
		return c.fail(ctx, err)
	}
	document := bundleTrustDocument{SchemaVersion: 3, Trust: bundleTrustPayload{
		BundleDigest: result.BundleDigest, Adopted: result.Adopted, AlreadyAdopted: result.AlreadyAdopted,
		Source: result.Source, Processors: projectBundleProcessorStatuses(result.Processors),
		SourceProcessAttempts: result.SourceProcessAttempts, ProcessorProcessAttempts: result.ProcessorProcessAttempts,
	}}
	return c.emitJSONDocument(ctx, document, "bundle trust")
}

func projectBundleProcessorStatuses(statuses []bundleauthority.ProcessorStatus) []bundleProcessorStatus {
	result := make([]bundleProcessorStatus, len(statuses))
	for index, status := range statuses {
		result[index] = bundleProcessorStatus{
			Contract: status.Contract, AdapterKind: status.AdapterKind, Version: status.Version,
			ResolvedPath: status.ResolvedPath, SHA256: status.SHA256, Size: status.Size, State: status.State,
		}
	}
	return result
}

func runBundlePreview(ctx context.Context, c *CLI, _ CommandSpec, intent operation.Intent, inputs ParsedInputs) int {
	result, err := c.previews.Preview(ctx, intent, inputs.One("--bundle"), tailoringplan.Attempt{
		Executable: inputs.One("source-executable"),
		Args:       inputs.Values("argv"),
	})
	if err != nil {
		return c.fail(ctx, err)
	}
	document := bundlePreviewDocument{SchemaVersion: 2, Preview: bundlePreviewPayload{PlanDigest: result.PlanDigest, Plan: result.Plan, SourceProcessAttempts: result.SourceProcessAttempts}}
	return c.emitJSONDocument(ctx, document, "bundle preview")
}

func runBundleExecute(ctx context.Context, c *CLI, _ CommandSpec, intent operation.Intent, inputs ParsedInputs) int {
	result, err := c.executions.Execute(ctx, intent, inputs.One("--bundle"), tailoringplan.Attempt{
		Executable: inputs.One("source-executable"),
		Args:       inputs.Values("argv"),
	})
	if err != nil {
		return c.fail(ctx, err)
	}
	if validationErr := validateTransformedJSONEnvelope(result); validationErr != nil {
		return c.fail(ctx, fault.Wrap(
			fault.KindContract,
			"output_contract_exceeded",
			"Bundle execution returned a result outside its transformed-JSON contract.",
			false,
			validationErr,
			fault.NextAction{Command: "bundle preview", Reason: "Inspect the fresh transformed-JSON plan; the source was not retried."},
		))
	}
	transformed := result.TransformedJSON
	if validationErr := transformed.Output.Validate(); validationErr != nil {
		return c.fail(ctx, fault.Wrap(
			fault.KindContract,
			"output_encoding_failed",
			"The canonical transformed-JSON output could not be encoded.",
			false,
			validationErr,
			fault.NextAction{Command: "bundle preview", Reason: "Repair deterministic schema-2 execution JSON; the source was not retried."},
		))
	}
	records := make([]tailoring.JSONValue, len(transformed.Output.Records))
	for index := range transformed.Output.Records {
		records[index] = projectExternalJSON(transformed.Output.Records[index])
	}
	document := bundleExecutionDocument{SchemaVersion: 2, Execution: bundleExecutionPayload{
		BundleDigest: result.BundleDigest, PlanDigest: result.PlanDigest,
		MatchedCommand: append([]string{}, result.MatchedCommand...), WrapperKind: result.WrapperKind,
		Output: bundleExecutionOutput{Render: transformed.Render, Shape: transformed.Output.Shape, Fields: append([]string{}, transformed.Output.Fields...), Records: records},
		Source: bundleExecutionSource{ExitCode: transformed.ExitCode}, SourceProcessAttempts: result.SourceProcessAttempts,
	}}
	return c.emitJSONDocument(ctx, document, "bundle execute")
}

func projectExternalJSON(value tailoring.JSONValue) tailoring.JSONValue {
	switch value.Kind {
	case tailoring.JSONNull:
		return tailoring.NewJSONNull()
	case tailoring.JSONBool:
		return tailoring.NewJSONBool(value.BoolValue)
	case tailoring.JSONNumber:
		return tailoring.NewJSONNumber(value.NumberValue)
	case tailoring.JSONString:
		return tailoring.NewJSONString(safeExternalText(value.StringValue))
	case tailoring.JSONArray:
		items := make([]tailoring.JSONValue, len(value.ArrayValue))
		for index := range value.ArrayValue {
			items[index] = projectExternalJSON(value.ArrayValue[index])
		}
		return tailoring.NewJSONArray(items)
	case tailoring.JSONObject:
		fields := make([]tailoring.JSONField, len(value.ObjectValue))
		for index := range value.ObjectValue {
			fields[index] = tailoring.JSONField{Name: safeExternalText(value.ObjectValue[index].Name), Value: projectExternalJSON(value.ObjectValue[index].Value)}
		}
		return tailoring.NewJSONObject(fields)
	default:
		return value
	}
}

func (c *CLI) emitJSONDocument(ctx context.Context, document any, command string) int {
	encoded, err := json.Marshal(document)
	if err != nil {
		return c.fail(ctx, fault.Wrap(fault.KindContract, "output_encoding_failed", "The canonical JSON output could not be encoded.", false, err))
	}
	if len(encoded)+1 > maxBundleOutputBytes {
		return c.fail(ctx, outputContractExceeded("The canonical JSON output exceeds its 2 MiB limit.", command))
	}
	return c.emitResult(ctx, append(encoded, '\n'))
}
