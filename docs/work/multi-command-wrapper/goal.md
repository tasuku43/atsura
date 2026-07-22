# Work Goal: One wrapper serves one complete multi-command surface

- Status: Active
- Retention: temporary
- Retention reason: None
- Governing contract: `docs/00_theses.md` Thesis 3, Thesis 4, and Thesis 7
- Review/delete trigger: Delete after durable conclusions are promoted and the change completes
- Successor: None
- Owner: Atsura maintainers
- Target: Next release-quality implementation iteration
- Related ADRs: ADR 0010, ADR 0014, and accepted ADR 0015

## Outcome

A maintainer can adopt one exact bundle whose included GitHub CLI surface
contains both `issue list` and `pr list`, render one host-neutral ordinary `gh`
wrapper, discover both paths through that wrapper's compiled help, and invoke
either path through its own reviewed option surface and wrapper behavior. The
complete surface is admitted before any wrapper bytes are produced; one
unsupported entry rejects the whole render without a source attempt.

## Why now

The canonical catalog, specification, bundle, plan, and contract-2 help model
already represented a bounded multi-command surface. Before this slice, runtime
compatibility rejected every surface whose included-entry count was not exactly
one, so Atsura's purpose-specific CLI hypothesis was demonstrated only as
separate one-command wrappers. Removing that local limit through all-entry
validation is the smallest step from a command demo to one useful tailored CLI.

## Non-goals

- New source commands, source adapters, option grammar, or output processors
- Typed option defaults, argument removal/replacement, or before/after actions
- Cross-command rules, aliases, fallback, or rediscovery at invocation time
- Persistent wrapper installation, PATH shims, shell startup edits, or any
  coding-agent-host integration
- Windows POSIX activation or a published release

## Acceptance criteria

- [x] One valid two-command bundle renders one deterministic POSIX function
      whose root and namespace help expose exactly both included paths.
- [x] From the same rendered function, ordinary `gh pr list` closes the existing
      transformed-JSON outcome and ordinary `gh issue list` closes the existing
      source-stream outcome, with one source attempt per call.
- [x] Surface admission validates every included command, option surface,
      wrapper, selector, and argv addition; any invalid later entry rejects the
      entire render before bytes, adoption changes, or source/processor starts.
- [x] Hidden cataloged and unknown paths remain fail-closed with zero source and
      processor attempts; no command inherits another entry's wrapper behavior.
- [x] Specification schema 4, bundle schema 3, plan schema 5, wrapper contract 2,
      and aggregate evidence schema 2 remain unchanged. Installed-artifact
      evidence advances only if its document shape changes.
- [ ] Linux and Darwin native artifacts prove the same two-command wrapper;
      Windows proves the unchanged structured unsupported result; all five
      native rows and their aggregate pass for one exact revision.
- [x] `task check`, `task security`, `task public:check`, and
      `task release:check` pass without weakening a check.

## Governing documents

- Thesis: one purpose-specific surface; membership independent from complete
  per-command wrapper behavior; host-neutral ordinary invocation
- Product contract: tailored CLI surface, wrapper pipeline, ordinary tailored
  help, and complete-surface runtime admission
- Architecture or security invariant: pure all-entry admission before the
  controlled process boundary; no partial wrapper materialization
- Existing ADR: ADR 0010 source-stream result modes and ADR 0014 static help

## Completion definition

The work is complete when every acceptance criterion has exact test and native
artifact evidence, durable contracts and ADR 0015 describe the multi-command
boundary, all required profiles pass, and this temporary packet is removed.
