package cli

import (
	"context"
	"fmt"

	"github.com/tasuku43/atsura/internal/app/planapply"
	"github.com/tasuku43/atsura/internal/app/wrappershimcmd"
	"github.com/tasuku43/atsura/internal/domain/fault"
	"github.com/tasuku43/atsura/internal/domain/operation"
	"github.com/tasuku43/atsura/internal/domain/tailoring"
	"github.com/tasuku43/atsura/internal/domain/tailoringbundle"
	"github.com/tasuku43/atsura/internal/domain/tailoringplan"
	"github.com/tasuku43/atsura/internal/domain/wrapperbinding"
	"github.com/tasuku43/atsura/internal/domain/wrappershim"
)

type wrapperInstallDocument struct {
	SchemaVersion int                   `json:"schema_version"`
	Installation  wrapperInstallPayload `json:"installation"`
}

type wrapperInstallPayload struct {
	Command                  string `json:"command"`
	Path                     string `json:"path"`
	BinPath                  string `json:"bin_path"`
	AlreadyInstalled         bool   `json:"already_installed"`
	SourceProcessAttempts    int    `json:"source_process_attempts"`
	ProcessorProcessAttempts int    `json:"processor_process_attempts"`
}

type wrapperStatusDocument struct {
	SchemaVersion int                      `json:"schema_version"`
	Artifacts     []wrapperArtifactPayload `json:"artifacts"`
}

type wrapperArtifactPayload struct {
	Reference      string `json:"reference"`
	Command        string `json:"command"`
	State          string `json:"state"`
	Path           string `json:"path"`
	MaterialSHA256 string `json:"material_sha256"`
}

type wrapperRemoveDocument struct {
	SchemaVersion int                  `json:"schema_version"`
	Removal       wrapperRemovePayload `json:"removal"`
}

type wrapperRemovePayload struct {
	Command                  string `json:"command"`
	Path                     string `json:"path"`
	Removed                  bool   `json:"removed"`
	SourceProcessAttempts    int    `json:"source_process_attempts"`
	ProcessorProcessAttempts int    `json:"processor_process_attempts"`
}

type wrapperRenderDocument struct {
	SchemaVersion int                  `json:"schema_version"`
	Wrapper       wrapperRenderPayload `json:"wrapper"`
}

type wrapperRenderPayload struct {
	Source                   string                         `json:"source"`
	SourceSHA256             string                         `json:"source_sha256"`
	Command                  string                         `json:"command"`
	Contract                 wrapperRenderContract          `json:"contract"`
	Bundle                   wrapperRenderBundle            `json:"bundle"`
	Runtime                  wrapperbinding.RuntimeIdentity `json:"runtime"`
	SourceProcessAttempts    int                            `json:"source_process_attempts"`
	ProcessorProcessAttempts int                            `json:"processor_process_attempts"`
}

type wrapperRenderContract struct {
	Version int    `json:"version"`
	Shell   string `json:"shell"`
}

type wrapperRenderBundle struct {
	Locator string `json:"locator"`
	Digest  string `json:"digest"`
}

func runWrapperInstall(ctx context.Context, c *CLI, spec CommandSpec, intent operation.Intent, inputs ParsedInputs) int {
	if c.wrapperShims == nil {
		return c.fail(ctx, wrapperShimUnavailable(wrappershimcmd.InstallCommand))
	}
	if spec.Agent.FixedTarget == nil || spec.Agent.Mutation == nil {
		return c.fail(ctx, fault.New(fault.KindContract, "invalid_mutation_contract", "The wrapper artifact installation mutation contract is incomplete.", false))
	}
	intent.Target = operation.TargetRef{Kind: spec.Agent.FixedTarget.Kind, ParentID: spec.Agent.FixedTarget.ID}
	intent.Impact = spec.Agent.Mutation.Impact
	result, err := c.wrapperShims.Install(ctx, intent, inputs.One("--bundle"))
	if err != nil {
		return c.fail(ctx, err)
	}
	if result.CommandName == "" || result.Path == "" || result.BinPath == "" ||
		result.SourceProcessAttempts != 0 || result.ProcessorProcessAttempts != 0 {
		return c.fail(ctx, fault.New(
			fault.KindContract,
			"output_contract_exceeded",
			"Wrapper installation returned an incomplete or process-attempting artifact result.",
			false,
			fault.NextAction{Command: "wrapper status", Reason: "Reconcile the managed artifact without repeating installation."},
		))
	}
	document := wrapperInstallDocument{SchemaVersion: 1, Installation: wrapperInstallPayload{
		Command: result.CommandName, Path: result.Path, BinPath: result.BinPath,
		AlreadyInstalled: result.AlreadyInstalled, SourceProcessAttempts: result.SourceProcessAttempts,
		ProcessorProcessAttempts: result.ProcessorProcessAttempts,
	}}
	return c.emitJSONDocument(ctx, document, wrappershimcmd.InstallCommand)
}

func runWrapperStatus(ctx context.Context, c *CLI, _ CommandSpec, intent operation.Intent, _ ParsedInputs) int {
	if c.wrapperShims == nil {
		return c.fail(ctx, wrapperShimUnavailable(wrappershimcmd.StatusCommand))
	}
	result, err := c.wrapperShims.Status(ctx, intent)
	if err != nil {
		return c.fail(ctx, err)
	}
	artifacts := make([]wrapperArtifactPayload, len(result.Artifacts))
	for index, artifact := range result.Artifacts {
		record := wrappershim.Record{
			CommandName: artifact.CommandName, State: artifact.State,
			Reference: artifact.Reference, MaterialSHA256: artifact.MaterialSHA256,
		}
		if err := record.Validate(); err != nil ||
			(record.State != wrappershim.StateOwnedActive && record.State != wrappershim.StateOwnedInactive) || artifact.Path == "" {
			return c.fail(ctx, fault.New(
				fault.KindContract,
				"output_contract_exceeded",
				"Wrapper status returned an incomplete artifact record.",
				false,
				fault.NextAction{Command: "help wrapper status", Reason: "Repair the bounded managed-artifact result."},
			))
		}
		artifacts[index] = wrapperArtifactPayload{
			Reference: artifact.Reference.String(), Command: artifact.CommandName, State: string(artifact.State),
			Path: artifact.Path, MaterialSHA256: artifact.MaterialSHA256,
		}
	}
	return c.emitJSONDocument(ctx, wrapperStatusDocument{SchemaVersion: 1, Artifacts: artifacts}, wrappershimcmd.StatusCommand)
}

func runWrapperRemove(ctx context.Context, c *CLI, spec CommandSpec, intent operation.Intent, inputs ParsedInputs) int {
	if c.wrapperShims == nil {
		return c.fail(ctx, wrapperShimUnavailable(wrappershimcmd.RemoveCommand))
	}
	if spec.Agent.Mutation == nil || spec.Agent.Mutation.TargetIDInput != "--artifact" {
		return c.fail(ctx, fault.New(fault.KindContract, "invalid_mutation_contract", "The wrapper artifact removal mutation contract is incomplete.", false))
	}
	artifact := inputs.One("--artifact")
	intent.Target = operation.TargetRef{Kind: spec.Agent.Mutation.TargetKind, ID: artifact}
	intent.Impact = spec.Agent.Mutation.Impact
	result, err := c.wrapperShims.Remove(ctx, intent, artifact)
	if err != nil {
		return c.fail(ctx, err)
	}
	if result.CommandName == "" || result.Path == "" || !result.Removed ||
		result.SourceProcessAttempts != 0 || result.ProcessorProcessAttempts != 0 {
		return c.fail(ctx, fault.New(
			fault.KindContract,
			"output_contract_exceeded",
			"Wrapper removal returned an incomplete or process-attempting artifact result.",
			false,
			fault.NextAction{Command: "wrapper status", Reason: "Reconcile the managed artifact without repeating removal."},
		))
	}
	document := wrapperRemoveDocument{SchemaVersion: 1, Removal: wrapperRemovePayload{
		Command: result.CommandName, Path: result.Path, Removed: result.Removed,
		SourceProcessAttempts: result.SourceProcessAttempts, ProcessorProcessAttempts: result.ProcessorProcessAttempts,
	}}
	return c.emitJSONDocument(ctx, document, wrappershimcmd.RemoveCommand)
}

func wrapperShimUnavailable(command string) error {
	return fault.New(
		fault.KindInternal,
		"internal_error",
		"The managed wrapper artifact service is not configured.",
		false,
		fault.NextAction{Command: "wrapper status", Reason: "Inspect managed artifact composition without starting a source process."},
	)
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
	if result.SourceProcessAttempts != 0 || result.ProcessorProcessAttempts != 0 {
		return c.fail(ctx, fault.Wrap(
			fault.KindContract,
			"output_contract_exceeded",
			"The wrapper renderer returned material outside its bounded zero-execution contract.",
			false,
			fmt.Errorf("source_process_attempts is %d and processor_process_attempts is %d", result.SourceProcessAttempts, result.ProcessorProcessAttempts),
			fault.NextAction{Command: "help wrapper render", Reason: "Reduce the bounded generated wrapper output."},
		))
	}

	if inputs.One("--format") == "text" {
		return c.emitResult(ctx, result.Material.Clone().Source)
	}
	document := wrapperRenderDocument{SchemaVersion: 2, Wrapper: wrapperRenderPayload{
		Source: string(result.Material.Source), SourceSHA256: result.Material.SHA256,
		Command:  result.Binding.CommandName,
		Contract: wrapperRenderContract{Version: result.Binding.ContractVersion, Shell: "posix"},
		Bundle:   wrapperRenderBundle{Locator: result.Binding.BundleLocator, Digest: result.Binding.BundleDigest},
		Runtime:  result.Binding.Runtime, SourceProcessAttempts: result.SourceProcessAttempts,
		ProcessorProcessAttempts: result.ProcessorProcessAttempts,
	}}
	return c.emitJSONDocument(ctx, document, "wrapper render")
}

func runWrapperRun(ctx context.Context, c *CLI, spec CommandSpec, intent operation.Intent, inputs ParsedInputs) int {
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
	switch result.ResultMode {
	case tailoringplan.ResultModeSourceStreamPassthrough:
		if err := result.Validate(); err != nil {
			return c.fail(ctx, invalidWrapperPlanResult(err))
		}
		return c.emitBufferedPlanResult(ctx, spec, result.SourceStream.Stdout, result.SourceStream.Stderr, result.SourceStream.ExitCode)
	case tailoringplan.ResultModeOriginalPreservingOptimizer:
		if err := result.Validate(); err != nil {
			return c.fail(ctx, invalidWrapperPlanResult(err))
		}
		return c.emitBufferedPlanResult(ctx, spec, result.Optimizer.Stdout, result.Optimizer.Stderr, result.Optimizer.ExitCode)
	case tailoringplan.ResultModeTransformedJSON:
		if err := validateTransformedJSONEnvelope(result); err != nil {
			return c.fail(ctx, invalidWrapperPlanResult(err))
		}
	default:
		return c.fail(ctx, fault.New(
			fault.KindContract,
			"output_contract_exceeded",
			"The fresh wrapper plan returned an unknown result mode.",
			false,
			wrapperResultRecovery(),
		))
	}

	encoded, err := encodeWrapperPlanResult(result.TransformedJSON.Output, result.TransformedJSON.Render, result.SourceProcessAttempts)
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
			wrapperResultRecovery(),
		))
	}
	return c.emitResult(ctx, append(encoded, '\n'))
}

func invalidWrapperPlanResult(err error) error {
	return fault.Wrap(
		fault.KindContract,
		"output_contract_exceeded",
		"The fresh wrapper plan returned a result outside its declared result-mode contract.",
		false,
		err,
		wrapperResultRecovery(),
	)
}

func wrapperResultRecovery() fault.NextAction {
	return fault.NextAction{Command: "bundle preview", Reason: "Inspect the bounded fresh-plan result; the source was not retried."}
}

func validateTransformedJSONEnvelope(result planapply.Result) error {
	if result.SourceProcessAttempts != 1 || result.ResultMode != tailoringplan.ResultModeTransformedJSON ||
		result.TransformedJSON == nil || result.SourceStream != nil || result.WrapperKind != tailoringbundle.WrapperTransform {
		return fmt.Errorf("transformed JSON result envelope is incomplete or contradictory")
	}
	if result.TransformedJSON.ExitCode != 0 || result.TransformedJSON.Render != tailoring.RenderCompactJSON {
		return fmt.Errorf("transformed JSON result framing is invalid")
	}
	return nil
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
