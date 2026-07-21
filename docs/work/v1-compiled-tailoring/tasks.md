# Work Tasks: Surface-and-wrapper correction

- Goal: [goal.md](goal.md)
- Plan: [plan.md](plan.md)

## Governance

- [x] Record the product owner's correction in thesis vocabulary.
- [x] Accept ADR 0005 and mark ADR 0004's source-authorization semantics
  superseded.
- [x] Align product, architecture, security, harness, `AGENTS.md`, and the
  capability Skill with purpose-specific surface and wrapper composition.
- [x] Replace the open-ended v1 authorization goal with this finite,
  mechanically evaluable correction goal.

## Specification and surface

- [x] Replace schema-2 policy types with schema-3 tailoring specification
  types and current vocabulary.
- [x] Enforce explicit surface default and independent command membership,
  option membership, and wrapper behavior.
- [x] Implement complete identity/transform wrapper validation with explicit
  empty before/after lists.
- [x] Implement deterministic normalization, canonical encoding, digest, and
  pure surface resolution.
- [x] Add full truth-table, catalog-binding, option-override, and
  `command_not_in_surface` tests.
- [x] Replace policy YAML with bounded strict schema-3 specification YAML and
  canonical round-trip fixtures.

## Bundle and adoption

- [x] Replace bundle schema 1 with schema 2 binding the normalized
  specification and derived surface/wrappers.
- [x] Recompute catalog, specification, surface, and bundle bindings on load.
- [x] Rename bundle build/status outputs and ports from policy to specification.
- [x] Replace permission/effect trust-summary counts with surface, option,
  wrapper, argv, stage, and output-transform facts.
- [x] Preserve exact-digest interactive user-local adoption and central
  mutation invocation for receipt changes.

## Effect and migration

- [x] Add stable `operation.EffectExecute` validation and encoding.
- [x] Declare source inspection as Execute and audit every mutation-only effect
  switch for explicit Execute handling.
- [x] Retire specification schemas 1 and 2 and bundle schema 1 without implicit
  conversion or trust migration.
- [x] Convert deprecated authorization command paths to catalog-declared
  migration diagnostics with exact recovery, zero persistence, and zero source
  attempts.
- [x] Remove authorization-centered source plan/runtime types, fields, tests,
  and capability claims.
- [x] Add no bundle runtime, raw, source refresh, or host integration in this
  milestone.

## Verification and handoff

- [x] Run focused domain, codec, application, CLI, catalog, and migration tests.
- [x] Run formatting, architecture, catalog/ledger, vet, and race checks through
  the repository interface.
- [x] Run `task check` and record the result.
- [x] Run `task security` and record the result.
- [x] Audit remaining policy/decision/effect/target/impact vocabulary and
  classify every intentional occurrence.
- [x] Review the complete diff against every goal acceptance item and preserve
  unrelated changes.
- [ ] Create the correction milestone commit without push, PR, tag, or release.
- [ ] Report exact verification evidence, remaining unknowns, and the next
  recommended minimal user outcome.
