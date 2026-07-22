# Presentation Evidence: Ordinary tailored help

## Frozen semantic corpus

- Typed fixture: `internal/domain/wrapperbinding/help_test.go`, helper
  `compiledHelpBundle` and test
  `TestCompileHelpDerivesRootNamespaceExactAndCombinedViews`
- Presentation answer key: `internal/infra/posixwrapper/render_test.go`, test
  `TestRenderedFunctionPrintsExactRootNamespaceExactAndCombinedHelp`
- Installed-artifact answer key: `tools/artifactjourney/journey.go`, functions
  `verifyPackagedTailoredHelp` and `expectedTailoredHelp`; its independent
  validator is `tools/artifactevidence/evidence.go:validateTailoredHelp`
- Fixture SHA-256: parameterized, not hard-coded. Schema-6 evidence records the
  exact bundle digest and wrapper-source SHA-256 for each built target.
- Presentation SHA-256: schema 6 records SHA-256 for stdout and stderr of each
  root, namespace, and exact view. The validator independently reconstructs
  expected bytes from the recorded bundle digest.
- Declared task and dimensions: discover one exact bundle-derived surface at
  root, namespace, and exact-command selectors
- Interpretation-relevant cases: no root command, namespace-only view,
  exact command with zero or nonzero options, value-taking versus boolean
  option, hidden cataloged command, unknown selector
- Canonical references and exact next argv: no references. Root help yields
  exact included command paths; an exact selector then yields that command's
  effective long-option names and value arity. This slice makes no positional,
  global-option, default-value, dependency, or conflict claim.

## Semantic eligibility

- [x] One root ordinary invocation returns the complete included exact-command
      path surface; one exact ordinary invocation returns that command's
      effective long-option arity.
- [x] Routine success requires zero joins, parsers, source inspection, or
      exploratory calls.
- [x] No canonical references are produced or consumed.
- [x] Scope, pagination, and uncertain semantic states are not applicable.
- [x] Same-name, order, quoted-prose, raw-notation, and indentation canaries
      create no unsupported inference.
- [ ] Hidden and unknown recovery obey existing executable argv grammar in the
      installed artifact; the schema-6 journey and validator are implemented,
      but native execution remains pending.

## Reproducible comparison

| Evidence | Before | After |
|---|---:|---:|
| Golden path | ordinary `--help` fault | exact `/bin/sh` renderer golden and schema-6 target evidence |
| Golden SHA-256 | not applicable (no successful output) | recomputed from each recorded bundle digest |
| UTF-8 bytes | not applicable (no successful output) | exact bytes asserted by focused tests; target values pending native run |
| Task invocations | 1 unsuccessful | 1 successful |
| External reconstruction steps | at least 1 | 0 |

- Golden generator or command: focused POSIX wrapper renderer test plus
  `tools/artifactjourney`'s parameterized `expectedTailoredHelp`
- Tokenizer: not used; semantic eligibility is sufficient for this slice
- Platform/runtime facts: POSIX `/bin/sh`; wrapper rendering remains
  unsupported on Windows
- Invalidation rule: any bundle fixture, help model, renderer, or generated
  wrapper-contract change invalidates the recorded hashes

## Experiment outcome

- Outcome: Static bundle-derived help is semantically eligible and implemented;
  focused domain, renderer, application, CLI, journey, and evidence tests pass.
  Installed native artifact proof remains pending.
- Eligible candidates: static bundle-derived wrapper help
- Failed or invalidated candidates: runtime fresh-plan help is ineligible
  because help is not a source execution plan
- Raw evidence retained at: temporary work packet until promotion
- Documented gates not implemented by the scorer: native installed-artifact
  matrix. The complete local repository gates passed on 2026-07-22.

## Product compatibility decision

- Decision owner: Atsura product owner and maintainers
- Selected presentation: accepted ADR 0014, compiled ordinary tailored help
- Compatibility rationale: ordinary reduced CLI must describe its own exact
  surface without source execution
- Schema/version impact: generated-wrapper binding/material contract 2 and
  per-target installed-artifact evidence schema 6. Wrapper review envelope
  schema 2, nested `wrapper-contract` schema 1, agent-help schema 12, and
  aggregate evidence schema 2 remain unchanged.
- Rollout and rollback: render a wrapper under the accepted contract version;
  rollback requires re-rendering
- Relationship to experiment outcome: contract tests determine semantic
  eligibility; byte counts are secondary

## Security and execution boundary

- [x] Fixtures and evidence are synthetic and public-safe.
- [x] No new filesystem artifact path is consumed at help invocation time.
- [x] Successful help starts no bound `atr`, source, or processor and inherits
      no secret; no generic zero-OS-process claim is made.
- [x] Network and tools are disabled and unnecessary.
- [x] No live-model evaluation is part of deterministic gates.
