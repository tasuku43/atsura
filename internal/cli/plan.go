package cli

import (
	"context"
	"encoding/json"

	"github.com/tasuku43/atsura/internal/domain/fault"
	"github.com/tasuku43/atsura/internal/domain/operation"
	"github.com/tasuku43/atsura/internal/domain/tailoring"
)

const maxPlanPreviewBytes = 256 * 1024

type planDocument struct {
	SchemaVersion int         `json:"schema_version"`
	Plan          planPayload `json:"plan"`
}

type planPayload struct {
	Decision              string            `json:"decision"`
	Executable            bool              `json:"executable"`
	SourceExecutable      string            `json:"source_executable"`
	MatchedCommand        string            `json:"matched_command"`
	OriginalArgv          []string          `json:"original_argv"`
	TransformedArgv       []string          `json:"transformed_argv"`
	Reason                string            `json:"reason"`
	Output                planOutputPayload `json:"output"`
	SourceProcessAttempts int               `json:"source_process_attempts"`
}

type planOutputPayload struct {
	Input  string              `json:"input"`
	Select []string            `json:"select"`
	Rename []planRenamePayload `json:"rename"`
	Render string              `json:"render"`
}

type planRenamePayload struct {
	From string `json:"from"`
	To   string `json:"to"`
}

func runPlanPreview(ctx context.Context, c *CLI, _ CommandSpec, intent operation.Intent, inputs ParsedInputs) int {
	plan, err := c.plans.Preview(ctx, intent, inputs.One("--config"), inputs.Values("command"))
	if err != nil {
		return c.fail(ctx, err)
	}
	output, err := renderPlan(plan)
	if err != nil {
		return c.fail(ctx, err)
	}
	return c.emitResult(ctx, output)
}

func renderPlan(plan tailoring.Plan) ([]byte, error) {
	renames := make([]planRenamePayload, len(plan.Output.Rename))
	for index, value := range plan.Output.Rename {
		renames[index] = planRenamePayload{From: value.From, To: value.To}
	}
	document := planDocument{SchemaVersion: 1, Plan: planPayload{
		Decision: string(plan.Decision), Executable: plan.Executable,
		SourceExecutable: plan.SourceExecutable, MatchedCommand: plan.MatchedCommand,
		OriginalArgv:    append([]string(nil), plan.OriginalArgv...),
		TransformedArgv: append(make([]string, 0, len(plan.TransformedArgv)), plan.TransformedArgv...),
		Reason:          safeExternalText(plan.Reason),
		Output: planOutputPayload{
			Input: string(plan.Output.Input), Select: append([]string(nil), plan.Output.Select...),
			Rename: renames, Render: string(plan.Output.Render),
		},
		SourceProcessAttempts: plan.SourceProcessAttempts,
	}}
	encoded, err := json.Marshal(document)
	if err != nil {
		return nil, fault.Wrap(fault.KindContract, "output_encoding_failed", "The plan preview JSON could not be encoded.", false, err)
	}
	if len(encoded)+1 > maxPlanPreviewBytes {
		return nil, outputContractExceeded("The plan preview exceeds its declared byte limit.", "plan preview")
	}
	return append(encoded, '\n'), nil
}
