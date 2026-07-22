# Work Goal: Add the second source CLI runtime

- Status: Active
- Retention: temporary
- Retention reason: None
- Governing contract: `docs/00_theses.md` theses 1, 4, 5, 7, and 8
- Review/delete trigger: Delete after durable conclusions are promoted and the change completes
- Successor: The finite RTK `go-test` optimizer work packet
- Owner: Atsura maintainers
- Target: The next release-quality implementation iteration
- Related ADRs: ADR 0006, ADR 0008, ADR 0010, and ADR 0011

## Outcome

A maintainer can inspect an installed Go 1.26 CLI, compile and adopt a bundle
whose only included command is an identity-wrapped `go test`, render the same
host-neutral ordinary-command wrapper used for GitHub CLI, and invoke `go
test` with no additional arguments. Atsura rebuilds one fresh plan, starts the
exact bundle-bound Go executable at most once without a shell, and returns the
conventionally completed stdout, stderr, and status exactly. This proves that
the shared catalog, specification, bundle, plan, wrapper, and execution core
does not depend on GitHub CLI or a coding-agent host.

## Why now

Primary-source and native RTK research found no safe, useful RTK pipe tuple for
the current GitHub CLI adapter. The strongest candidate is the official Go
`test2json` stream produced by `go test -json`, but implementing an RTK runtime
and a second source adapter in one iteration would combine two independently
reviewable compatibility boundaries. Closing Go CLI as a release-quality
source first makes the following optimizer iteration smaller and supplies the
second nature-distinct source required by the product goal.

## Non-goals

- RTK inspection, execution, packaging, or an RTK authoring default.
- Adding `-json`, any Go build/test flag, a package argument, or a test-binary
  argument to the admitted runtime grammar.
- Output projection, optimization, filtering, or interpretation of Go test
  output.
- Delegating source execution to another wrapper or coding-agent host.
- Persistent wrapper installation, Windows POSIX activation, raw execution,
  source refresh, or multiple purpose profiles.
- Claiming that `go test` is read-only, hermetic, network-free, or safe to
  replay.

## Acceptance criteria

- [ ] `atr source inspect --adapter=go-cli --executable <path-or-name>` performs
      exactly the adapter's three fixed offline probes, accepts only Go 1.26.x,
      and emits one canonical vendor-neutral catalog containing the observed
      built-in root commands.
- [ ] The generated catalog, schema-3 specification, schema-2 bundle,
      schema-4 plan, and wrapper binding contain no Go-specific field and the
      existing GitHub CLI journey remains byte-compatible.
- [ ] `spec init`, `spec validate`, `bundle build`, `bundle status`, `bundle
      trust`, and `bundle preview` close an exclude-by-default, single-command
      identity surface for `go test` without starting a routine source task.
- [ ] `wrapper render` and `wrapper run` accept that complete surface on Linux
      and macOS; exact no-argument `go test` starts once and returns exact
      stdout, stderr, and conventional status. Any added source argument or
      option fails before source start.
- [ ] Unsupported adapter/version/help evidence, missing adoption, source or
      runtime drift, incompatible surface, and invalid invocation have exact
      structured recovery and zero source attempts.
- [ ] Root agent help plus one exact scoped-help request is sufficient to find
      and understand source inspection. Routine wrapper success requires zero
      undeclared external-processing steps.
- [ ] Native installed-artifact evidence exercises Go inspection on every
      release target and the ordinary Go wrapper on every claimed POSIX target;
      Windows retains the exact structured unsupported-render result.
- [ ] `task check`, `task security`, `task public:check`, and `task
      release:check` pass on the same tree, and the required native CI matrix
      succeeds for the pushed revision.

## Governing documents

- Thesis: `docs/00_theses.md`, especially source-adapter independence and the
  one-plan wrapper contract.
- Product contract section: Source CLI, generated command catalog, source-
  stream passthrough, and compatibility boundary.
- Architecture or security invariant: four-layer dependency direction,
  identity-bound `EffectExecute`, and no coding-agent-host adapter.
- Existing ADR: ADR 0006, ADR 0008, ADR 0010; ADR 0011 selects this slice.

## Completion definition

The work is complete when every acceptance criterion has direct evidence,
ADR 0011 and governing documents describe the implemented boundary, the exact
installed artifacts pass all required native and repository gates, temporary
diagnostics contain no source bytes, and this packet is removed from the final
tree.
