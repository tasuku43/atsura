package cli

import (
	"context"

	"github.com/tasuku43/atsura/internal/domain/fault"
	"github.com/tasuku43/atsura/internal/domain/operation"
)

// emitResult performs exactly one checked write after a command has rendered
// and validated its complete output in memory. It resolves the effect from the
// catalog-bound dispatch context so handlers cannot substitute a weaker output
// contract.
func (c *CLI) emitResult(ctx context.Context, output []byte) int {
	path, bound := boundCommandPath(ctx)
	if !bound {
		return c.fail(ctx, fault.New(
			fault.KindContract,
			"missing_context",
			"The command output context is missing.",
			false,
			fault.NextAction{Command: "help", Reason: "Retry through the context-aware CLI entry point."},
		))
	}
	command, found := c.catalog.Lookup(path)
	if !found {
		return c.fail(ctx, fault.New(
			fault.KindContract,
			"invalid_catalog",
			"The command output contract is missing from the catalog.",
			false,
			fault.NextAction{Command: "help", Reason: "Repair the catalog before emitting output."},
		))
	}
	switch command.Effect {
	case operation.EffectRead:
		if err := ctx.Err(); err != nil {
			return c.fail(ctx, err)
		}
		return c.emitComplete(ctx, output)
	case operation.EffectExecute:
		return c.emitExecuteResult(ctx, command, output)
	case operation.EffectCreate, operation.EffectWrite:
		return c.emitMutationResult(ctx, command, output)
	default:
		return c.fail(ctx, fault.New(
			fault.KindContract,
			"invalid_catalog",
			"The command output effect is invalid.",
			false,
			fault.NextAction{Command: "help " + command.Path, Reason: "Declare the command effect before emitting output."},
		))
	}
}

// emitExecuteResult never represents a failed final write as replay-safe. The
// source process has already started, and Atsura does not know the downstream
// source operation's semantics or outcome.
func (c *CLI) emitExecuteResult(ctx context.Context, command CommandSpec, output []byte) int {
	recovery, ok := executeOutputWriteRecovery(command)
	if !ok {
		return c.fail(ctx, fault.New(
			fault.KindContract,
			"invalid_catalog",
			"The source-execution output contract is invalid.",
			false,
			fault.NextAction{Command: "help " + command.Path, Reason: "Declare non-retryable source-execution output recovery."},
		))
	}
	if _, err := writeOnce(c.Out, output); err != nil {
		return c.failExecuteOutputWrite(ctx, "The source process completed, but its Atsura output could not be written completely.", err, recovery)
	}
	return ExitOK
}

// emitSourceStreamResult performs the plan-declared buffered two-stream
// delivery. The writes cannot be atomic: stdout is completed once before
// stderr is attempted, and the conventional source status is returned only
// after both writes complete.
func (c *CLI) emitSourceStreamResult(ctx context.Context, command CommandSpec, stdout, stderr []byte, sourceStatus int) int {
	recovery, ok := executeOutputWriteRecovery(command)
	if !ok || sourceStatus < 0 {
		return c.fail(ctx, fault.New(
			fault.KindContract,
			"invalid_catalog",
			"The source-stream output contract is invalid.",
			false,
			fault.NextAction{Command: "help " + command.Path, Reason: "Declare bounded source-stream result and non-retryable final-write recovery."},
		))
	}
	if _, err := writeOnce(c.Out, stdout); err != nil {
		return c.failExecuteOutputWrite(ctx, "The source completed, but its plan-declared stdout could not be written completely; partial caller-visible output may already exist.", err, recovery)
	}
	if _, err := writeOnce(c.Err, stderr); err != nil {
		return c.failExecuteOutputWrite(ctx, "The source completed, but its plan-declared stderr could not be written completely; partial caller-visible output may already exist.", err, recovery)
	}
	return sourceStatus
}

func executeOutputWriteRecovery(command CommandSpec) ([]fault.NextAction, bool) {
	if command.Effect != operation.EffectExecute {
		return nil, false
	}
	for _, declared := range command.Agent.Errors {
		if declared.Code == "execute_output_write_failed" && declared.Kind == fault.KindInternal && !declared.Retryable && len(declared.NextActions) > 0 {
			return cloneSlice(declared.NextActions), true
		}
	}
	return nil, false
}

func (c *CLI) failExecuteOutputWrite(ctx context.Context, message string, err error, recovery []fault.NextAction) int {
	return c.fail(ctx, fault.Wrap(
		fault.KindInternal,
		"execute_output_write_failed",
		message,
		false,
		err,
		recovery...,
	))
}

// emitMutationResult writes a result after a mutation action has returned
// confirmed success. It deliberately does not reclassify that success as a
// retryable cancellation if the context becomes done after the action. A write
// failure is also non-retryable: the mutation already succeeded, so its catalog
// recovery must reconcile through a read rather than repeat the mutation.
func (c *CLI) emitMutationResult(ctx context.Context, command CommandSpec, output []byte) int {
	var recovery []fault.NextAction
	for _, declared := range command.Agent.Errors {
		if declared.Code == "mutation_output_write_failed" &&
			declared.Kind == fault.KindInternal && !declared.Retryable {
			recovery = cloneSlice(declared.NextActions)
			break
		}
	}
	if !isMutationEffect(command.Effect) || len(recovery) == 0 {
		return c.fail(ctx, fault.New(
			fault.KindContract,
			"invalid_catalog",
			"The mutation output contract is invalid.",
			false,
			fault.NextAction{Command: "help " + command.Path, Reason: "Declare non-retryable mutation output recovery through a read-only command."},
		))
	}
	if _, err := writeOnce(c.Out, output); err != nil {
		return c.fail(ctx, fault.Wrap(
			fault.KindInternal,
			"mutation_output_write_failed",
			"The mutation succeeded, but its output could not be written completely.",
			false,
			err,
			recovery...,
		))
	}
	return ExitOK
}

// emitComplete performs the common complete-or-no-success write. Callers must
// decide whether cancellation is still authoritative before reaching it.
func (c *CLI) emitComplete(ctx context.Context, output []byte) int {
	if _, err := writeOnce(c.Out, output); err != nil {
		return c.fail(ctx, fault.Wrap(
			fault.KindInternal,
			"output_write_failed",
			"The command output could not be written completely.",
			true,
			err,
			fault.NextAction{Command: invocationCommandPath(ctx), Reason: "Retry with a writable output stream."},
		))
	}
	return ExitOK
}
