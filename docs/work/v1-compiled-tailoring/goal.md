# Work Goal: Correct the compiled tailoring model

- Status: Active
- Retention: temporary
- Retention reason: None
- Governing contract: docs/00_theses.md and docs/decisions/0005-purpose-specific-surface-and-wrapper.md
- Review/delete trigger: Delete after the correction conclusions are promoted, all acceptance evidence is in the final committed tree, and the completion report records the gates
- Successor: A separately scoped wrapper-preview/runtime packet after this correction passes
- Owner: Repository maintainer
- Target: Finite surface-and-wrapper correction milestone
- Related ADRs: docs/decisions/0005-purpose-specific-surface-and-wrapper.md; superseded docs/decisions/0002-v0.1-local-run-boundary.md and docs/decisions/0004-v1-compiled-tailoring-bundle.md

## Outcome

A maintainer can express one purpose-specific command and option surface as a
strict schema-3 tailoring specification, compile it deterministically with a
validated source catalog into a canonical schema-2 bundle, and adopt and
inspect that exact bundle. The pure compiler resolves whether a command is
present and which complete identity or transforming wrapper applies, without
starting a bundle runtime source process.

The public and internal contracts describe source execution honestly as
`EffectExecute`, reserve create/write mutation contracts for Atsura-owned
state, and reject retired authorization schemas with explicit migration
diagnostics.

## Why now

The authorization-centered interpretation of ADR 0004 conflated three
different concerns: purpose-specific surface composition, wrapper behavior,
and source-operation permission. The product owner corrected that direction.
Continuing runtime, raw, or host work on the old model would make the wrong
vocabulary and trust boundary harder to remove.

This finite milestone corrects the governing model and its mechanical schema
before runtime work resumes.

## Non-goals

- Starting a source process from a compiled bundle.
- Implementing wrapper runtime, raw execution, source refresh, or a host
  adapter.
- Deciding whether a source operation is allowed, requires confirmation, or is
  safe.
- Inferring source read/create/write effects, authorization targets, or impacts.
- Claiming that an absent command or option is sandboxed.
- Selecting richer argv replacement/default actions or non-empty typed
  before/after actions.
- Executing arbitrary shell, scripts, jq, RTK, plugins, external transformers,
  or a runtime language model.
- Auto-converting retired policy or bundle schemas or carrying trust across
  schema/digest changes.
- Publishing, pushing, opening a pull request, tagging, or releasing.

## Acceptance criteria

- [x] `docs/00_theses.md`, product, architecture, security, harness,
  `AGENTS.md`, the capability Skill, and accepted ADR 0005 consistently define
  purpose-specific surface composition and wrapper pipelines rather than
  source authorization.
- [x] Schema-3 tailoring specification types and a bounded strict YAML codec
  require explicit surface default, command presence, included-command option
  surface, complete identity/transform wrapper, and bounded reasons.
- [x] Pure domain tests cover every membership/wrapper truth-table state,
  verified-catalog binding, deterministic normalization/digest, option
  overrides, and `command_not_in_surface` with no plan.
- [x] Bundle schema 2 canonically binds source identity, adapter evidence,
  catalog/digest, normalized specification/digest, and the derived surface with
  wrapper definitions; load recomputes all semantic bindings.
- [x] Specification init/validate and bundle build/status/trust use current
  `specification`, `surface`, and `wrapper` vocabulary. Adoption summaries
  contain no permission decision or inferred source effect counts.
- [x] Specification schemas 1 and 2, bundle schema 1, and deprecated
  authorization command paths produce explicit stable migration diagnostics,
  perform no implicit conversion or trust migration, persist no new state, and
  start zero source processes.
- [x] `operation.EffectExecute` is stable and source-inspection catalog and
  intent contracts use it; mutation-only code handles Execute explicitly and
  create/write target/impact contracts remain limited to Atsura-owned state.
- [x] The authorization-centered bundle-plan/runtime additions are removed or
  retired and no runtime/raw/host implementation is added in this milestone.
- [x] Focused tests, `task check`, and `task security` pass on the same final
  tree without weakened checks.
- [x] A requirement audit and repository search find no live source-wrapper
  allow/confirm/deny, source read/create/write, target/impact, policy-as-
  permission, trust-as-grant, or hiding-as-sandbox contract outside explicitly
  superseded history and migration diagnostics.
- [ ] The correction is captured in an intentional milestone commit, unrelated
  user changes are preserved, no publication action occurs, and the final
  completion report records exact verification results and remaining unknowns.

## Governing documents

- Product theses: `docs/00_theses.md`
- Product contract: `docs/01_product_contract.md`
- Architecture: `docs/02_architecture.md`
- Security model: `docs/03_security_model.md`
- Harness: `docs/04_harness.md`
- Accepted decision: `docs/decisions/0005-purpose-specific-surface-and-wrapper.md`

## Completion definition

The goal is complete only when every acceptance checkbox has direct evidence,
the schema and public vocabulary implement ADR 0005, focused/full/security
verification passes on the same tree, the milestone commit succeeds, and no
runtime, raw, host, or publication scope has been included. A passing test for
the superseded authorization model does not count as evidence.
