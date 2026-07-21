# Work Tasks: Host-mediated tailored invocation

- Goal: [goal.md](goal.md)
- Plan: [plan.md](plan.md)

## Understand

- [x] Read governing theses, product, architecture, security, and harness documents.
- [x] Observe that the release-quality direct path has no host-mediated equivalent.
- [x] Record current facts, constraints, long-horizon coverage, and unknowns.
- [x] Confirm the public outcome and non-goals.
- [x] Complete current primary-source research for RTK, coding-agent hooks, and representative wrappers or policy tools.
- [x] Record the product-owner direction that compatible RTK stages should be the compile-time default candidate, never a runtime implicit fallback.
- [x] Observe one exact Claude Code artifact loading an exec-form `SessionStart` hook and local plugin before a zero-cost unauthenticated exit.

## Decide

- [x] Compare credible host transports and record a human-handoff scorecard.
- [ ] Select exactly one first host, scope, and ordinary-invocation outcome.
- [ ] Identify public-contract and compatibility impact without prematurely fixing command names.
- [ ] Classify utility/discover/act roles and any opaque reference flow.
- [ ] Classify every capability as public, internal, deferred, or excluded.
- [ ] Identify effects, targets, settings assets, and trust-boundary changes.
- [ ] Accept a host-boundary ADR and propagate any thesis revision.

## Implement

- [ ] Add failing host-neutral state and zero-side-effect contract tests.
- [ ] Implement domain invariants and application-owned ports.
- [ ] Implement bounded host payload and safe settings adapters.
- [ ] Register accepted setup, status, removal, and runtime commands in `cli.Catalog` if the selected design requires them.
- [ ] Update the capability ledger and publishable schema fixtures.
- [ ] Add exact recovery, hostile-output, ownership, idempotence, cancellation, and zero/one source-attempt tests.
- [ ] Add installed-artifact and native-platform harness enforcement.
- [ ] Update durable documentation and the general capability Skill when evidence requires it.

## Verify

- [ ] Focused tests pass. Evidence:
- [ ] `task check` passes. Evidence:
- [ ] `task security` passes. Evidence:
- [ ] `task public:check` passes. Evidence:
- [ ] `task release:check` passes. Evidence:
- [ ] Runtime-only host behavior is observed on every claimed compatible platform. Evidence:
- [ ] The host-mediated readiness scenario meets its discovery and setup budget. Evidence:
- [ ] Routine supported success has external-processing count zero. Evidence:
- [ ] The human-handoff scorecard justifies each retained setup step. Evidence:
- [ ] Generated diff and repository status are understood. Evidence:

## Hand off

- [ ] Acceptance criteria have evidence.
- [ ] Durable decisions are promoted and the temporary packet is removed.
- [ ] Implementation is committed by concern and the final tree is clean.
- [ ] The long-horizon coverage baseline is updated and the next maximum gap is selected.
