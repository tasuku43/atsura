package cli

import (
	"context"

	"github.com/tasuku43/atsura/internal/domain/operation"
)

func runRun(ctx context.Context, c *CLI, _ CommandSpec, _ operation.Intent, _ ParsedInputs) int {
	return c.fail(ctx, legacyTailoringSchemaFault("run", "help spec validate"))
}
