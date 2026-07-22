package cli

import (
	"context"

	"github.com/tasuku43/atsura/internal/domain/fault"
	"github.com/tasuku43/atsura/internal/domain/operation"
)

func runPlanPreview(ctx context.Context, c *CLI, _ CommandSpec, _ operation.Intent, _ ParsedInputs) int {
	return c.fail(ctx, legacyTailoringSchemaFault("plan preview", "help spec validate"))
}

func runPolicyInit(ctx context.Context, c *CLI, _ CommandSpec, _ operation.Intent, _ ParsedInputs) int {
	return c.fail(ctx, legacyTailoringSchemaFault("policy init", "help spec init"))
}

func runPolicyValidate(ctx context.Context, c *CLI, _ CommandSpec, _ operation.Intent, _ ParsedInputs) int {
	return c.fail(ctx, legacyTailoringSchemaFault("policy validate", "help spec validate"))
}

func legacyTailoringSchemaFault(command, recovery string) error {
	return fault.New(
		fault.KindInvalidInput,
		"legacy_tailoring_schema",
		"The "+command+" command belongs to the retired authorization-policy schema and performs no source execution.",
		false,
		fault.NextAction{Command: recovery, Reason: "Create or validate a schema-4 tailoring specification; no automatic authorization-to-surface conversion is available."},
	)
}
