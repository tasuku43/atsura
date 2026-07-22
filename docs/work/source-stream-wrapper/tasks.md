# Work Tasks: Execute identity and argv-only ordinary wrappers

- Goal: [goal.md](goal.md)
- Plan: [plan.md](plan.md)

## Understand

- [x] Read governing theses, product, architecture, security, harness, and Skill.
- [x] Observe the current JSON-transform-only runtime and wrapper surface proof.
- [x] Record verified facts, constraints, unknowns, and thesis evidence.
- [x] Confirm the public outcome and non-goals.

## Decide

- [x] Reject a separate raw executor and direct streaming for this slice.
- [x] Classify wrapper render/run as unchanged utility read/execute operations.
- [x] Define source-stream output, nonzero status, retry, and uncertainty boundaries.
- [ ] Accept the source-stream ADR and propagate its governing consequences.
- [ ] Add failing contract tests for result modes and conventional completion.

## Implement

- [ ] Add schema-4 result-mode invariants and lossless empty argv validation.
- [ ] Extend the shared plan application result without duplicating execution.
- [ ] Admit finite identity and append-argv-only GitHub CLI surfaces.
- [ ] Add the CLI dual-stream final writer and source status mapping.
- [ ] Update scoped help, agent help, trust summary, schemas, and fixtures.
- [ ] Extend exact-artifact evidence for ordinary identity and argv-only wrappers.
- [ ] Preserve all transformed-JSON and Windows-unsupported regressions.
- [ ] Promote durable documentation and remove this packet.

## Verify

- [ ] Focused tests pass. Evidence:
- [ ] `task check` passes. Evidence:
- [ ] `task security` passes. Evidence:
- [ ] `task public:check` passes. Evidence:
- [ ] `task release:check` passes. Evidence:
- [ ] Native ordinary-wrapper behavior passes on Linux and macOS. Evidence:
- [ ] Windows structured unsupported remains zero-attempt. Evidence:
- [ ] Agent help discovery budget remains within the declared limit. Evidence:
- [ ] Routine-success external-processing count remains zero. Evidence:
- [ ] Generated diff and repository status are understood. Evidence:

## Hand off

- [ ] Acceptance criteria have evidence.
- [ ] Durable decisions are promoted and this temporary packet is removed.
- [ ] Follow-up work is explicit and non-blocking.
- [ ] Commit and push each coherent concern to `main`.
