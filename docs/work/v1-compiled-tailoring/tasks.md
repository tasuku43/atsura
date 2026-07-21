# Work Tasks: v1 compiled tailoring

- Goal: [goal.md](goal.md)
- Plan: [plan.md](plan.md)

## Decide and document

- [x] Review three interface concepts and select compiled tailoring bundle.
- [x] Research current Claude Code hooks, RTK, GitHub CLI formatting, and dynamic CLI extension behavior from primary sources.
- [x] Select GitHub CLI 2.x capability validation as the first real source adapter without embedding it in core schemas.
- [x] Require vendor-neutral source and host ports plus alternate-adapter conformance fixtures.
- [ ] Accept ADR 0004 and propagate v1 consequences through docs 00 through 04.
- [ ] Define exact schemas, public commands, faults, budgets, and compatibility fixtures.

## Catalog and bundle

- [x] Implement vendor-neutral canonical catalog values and alternate-adapter application conformance.
- [x] Implement bounded GitHub CLI 2.x reference inspection and public `source inspect` output.
- [x] Add failing domain and adapter tests for vendor-neutral identity, provenance, catalog, canonical encoding, adapter conformance, and probe budgets.
- [ ] Implement `source refresh` comparison and persistence after the completed GitHub CLI inspection task.
- [ ] Register the remaining source refresh command and capability contract.
- [ ] Implement schema-2 policy init/validate and normalized policy.
- [ ] Implement canonical bundle build, digest, trust receipt, and status/drift.

## Execution and policy

- [ ] Make preview, explain, run, and raw consume the same bundle.
- [ ] Implement visibility, allow/confirm/deny, target/impact, and mutation uncertainty contracts.
- [ ] Implement richer typed transforms without arbitrary external execution.
- [ ] Prove raw is explicit, identity-bound, absent from hook surface, and never recovery.

## Claude Code integration

- [ ] Implement exact-owner project-local install/status/remove.
- [ ] Implement strict simple-command parsing and managed compound fail-closed behavior.
- [ ] Implement SessionStart surface projection and PreToolUse allow/ask/deny/defer/rewrite.
- [ ] Implement one-shot bundle/plan/argv-bound confirmation.

## Verify and finish

- [ ] Pass focused tests and complete fixture E2E journeys.
- [ ] Replay agent readiness and record the setup/authentication scorecard.
- [ ] Update README, SECURITY, SUPPORT, compatibility, migration, and release docs.
- [ ] Pass `task check`, `task security`, `task public:check`, and `task release:check` on one final commit.
- [ ] Promote evidence, remove this packet, confirm clean Git state, and report no publication.
