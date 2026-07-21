# Work Tasks: Execute one proven JSON-transform wrapper

- Goal: [goal.md](goal.md)
- Plan: [plan.md](plan.md)

## Understand and decide

- [x] Read governing theses, product, architecture, security, harness, ADR 0005,
  add-capability Skill, current process/plan/transform code, and adapter fixture.
- [x] Observe current GitHub 2.72.0 offline help and primary formatting docs.
- [x] Record verified identity-binding and selector-proof gaps.
- [x] Close the outcome to transform-only bundle execution and list non-goals.
- [x] Classify `bundle execute` as public utility/execute with no references,
  mutation, authentication contract, or source authorization vocabulary.

## Implement

- [ ] Add explicit plan process framing and a plan-derived bound request.
- [ ] Enforce expected identity before, immediately before, and after one run.
- [ ] Add adapter runtime proof and phase-aware application execution.
- [ ] Add canonical bounded typed JSON output encoding.
- [ ] Extend GitHub offline inspection with exact field evidence and maintained
  runtime selector proof for a finite command set.
- [ ] Register `bundle execute`, output schema, faults, help, capability, and
  composition without altering legacy migration-only `run`.
- [ ] Add zero/one-attempt, hostile-output, cancellation, digest-equality,
  catalog, output, and readiness tests.
- [ ] Promote durable runtime decisions and compatibility limits.

## Verify and hand off

- [ ] Focused tests pass. Evidence:
- [ ] `task check` passes. Evidence:
- [ ] `task security` passes. Evidence:
- [ ] `task public:check` passes. Evidence:
- [ ] `task release:check` passes. Evidence:
- [ ] Runtime behavior observed locally without persisting source output.
- [ ] Known-path discovery uses one help call and routine success needs zero
  external interpretation steps.
- [ ] Final diff/status are understood, commits are concern-scoped, no push or
  publication occurred, and the temporary packet is removed.
