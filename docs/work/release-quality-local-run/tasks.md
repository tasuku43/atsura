# Work Tasks: Release-quality local tailoring run

- Goal: [goal.md](goal.md)
- Plan: [plan.md](plan.md)

## Understand

- [x] Read governing theses, product, architecture, security, harness, public, release, and add-capability Skill.
- [x] Observe and checkpoint-commit the current preview MVP.
- [x] Record verified facts, constraints, risks, and unknowns.
- [x] Confirm the finite public outcome and explicit non-goals.

## Decide

- [x] Choose an explicit read-only v0.1 source effect rather than arbitrary mutation execution.
- [x] Choose one local `run` command sharing loader/compiler semantics with preview.
- [x] Choose fixed timeout, byte bounds, one-attempt behavior, and no raw fallback.
- [x] Choose strict object/array JSON to typed values and a fixed execution envelope.
- [x] Classify `tailoring.execute` as public utility/read with no references.
- [x] Record the accepted durable boundary in ADR 0002 and governing docs.

## Implement

- [ ] Add failing domain, application, infrastructure, CLI, and catalog tests.
- [ ] Add the required read effect to schema-1 policy and plan output.
- [ ] Implement source-process request, identity, and result invariants.
- [ ] Implement typed JSON values and pure select/rename transformation.
- [ ] Implement one-attempt bounded process execution and identity revalidation.
- [ ] Implement strict bounded duplicate-aware JSON parsing.
- [ ] Implement the application run use case and stable fault ordering.
- [ ] Register `atr run`, render fixed JSON, and escape successful source stderr.
- [ ] Update the capability ledger, examples, README, SECURITY, SUPPORT, and agent-readiness evidence.
- [ ] Add claims-to-checks and release-quality enforcement documentation.

## Verify

- [ ] Focused tests pass. Evidence:
- [ ] `task check` passes. Evidence:
- [ ] `task security` passes. Evidence:
- [ ] `task public:check` passes. Evidence:
- [ ] `task release:check` passes. Evidence:
- [ ] Synthetic runtime fixture succeeds with one direct attempt. Evidence:
- [ ] Root-to-scoped discovery takes at most two help invocations. Evidence:
- [ ] Routine-success external-processing count is zero. Evidence:
- [ ] Final committed tree is clean and publication did not occur. Evidence:

## Hand off

- [ ] Acceptance criteria have evidence.
- [ ] Durable decisions are promoted and the temporary work packet is removed.
- [ ] Follow-up work is explicit and does not block v0.1 local run.
- [ ] Commits separate preview, contract, and implementation boundaries.
- [ ] Handoff summarizes outcome, checks, limitations, and next research slice.
