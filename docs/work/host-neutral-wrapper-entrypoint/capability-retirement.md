# Capability Retirement: Remove coding-agent-host adapter plans

The four retired entries were deferred design placeholders. They were never
public commands, implemented transports, persisted formats, or release claims.

## Decision and evidence

- Capability IDs: `integration.claude-code.lifecycle`,
  `integration.claude-code.transport`, `integration.codex.lifecycle`, and
  `integration.codex.transport`
- Previous status and public commands: all `deferred`; no public commands
- New status: removed from the product capability ledger
- Superseding capability or ADR: ADR 0008 and deferred
  `tailoring.wrapper.materialize`
- Evidence: the product owner clarified that a coding agent calls an already
  exposed wrapper and agent-specific hook/settings concerns belong outside
  Atsura; repository audit found no production implementation
- Last version or revision that supported the old surface: none

## Public contract removal

- [x] No command path, help entry, example, or dispatch binding existed.
- [x] No produced/consumed reference edge existed.
- [x] No fault or recovery action existed.
- [x] The capability ledger removes all four vendor entries and records only
      the generic deferred wrapper result.
- [x] No machine schema or released compatibility claim changes.
- [x] Repository search and catalog tests prove the four retired paths and
      capability identifiers have no product route.

## Implementation and dependency removal

- [x] No application port, domain variant, infrastructure adapter, composition
      wiring, or feature-specific production policy existed.
- [x] No provider SDK, protocol library, generated file, environment variable,
      or CI/release step was added.
- [x] No dormant transport, hidden flag, or legacy environment value exists.
- [x] Theses, product, architecture, security, Skill, harness, README, SUPPORT,
      and active work packet describe the corrected product boundary.

## Persisted state

| State | Secret? | Disposition | Recovery and evidence |
|---|---|---|---|
| None; the deferred designs never wrote state | No | Not applicable | Production and repository audit |

- [x] No cleanup action is required.
- [x] No dependency is retained for legacy state.
- [x] No credential or settings content was persisted or committed.

## Verification

- Focused negative tests: existing catalog/capability contract tests,
  `TestDefaultCatalogContainsNoRetiredCodingAgentHostSurface`, and
  `TestFindViolationsRejectsExplicitCodingAgentHostPackages`
- Catalog/capability/schema checks: `contractlint` and `task check:fast`
- Dependency and import diff: no production Go dependency delta
- Persisted-state migration or cleanup tests: not applicable; no state existed
- Required gate: full design commit uses `task check`; implementation closure
  additionally uses security, public, and release profiles
- Rollback or reintroduction policy: a successor thesis and ADR must prove a
  product-semantic need; vendor convenience alone cannot restore these entries
