# Work Tasks: Release-quality transform journey

- Goal: [goal.md](goal.md)
- Plan: [plan.md](plan.md)

## Understand

- [x] Read governing theses, product, architecture, security, harness, and release sections.
- [x] Reproduce or observe current artifact evidence.
- [x] Record verified facts, constraints, unknowns, and thesis evidence.
- [x] Confirm the public outcome and non-goals.

## Decide

- [x] Compare native replay, public trust bypass, and emulation approaches.
- [x] Confirm no public-contract, role, reference-flow, or capability-ledger change.
- [x] Identify process, filesystem, trust-store, and output assets.
- [x] Accept composite trust evidence without a production bypass.
- [x] Confirm the design against the three parallel audits.

## Implement

- [x] Add the native GitHub-compatible source fixture and tests.
- [x] Complete the finite source-catalog, specification, and runtime-admission exact-help contract.
- [x] Add the exact-artifact journey orchestrator and tests.
- [x] Add native archive replay script and harness enforcement.
- [x] Add five-target CI and release workflow replay.
- [x] Update durable product, architecture, security, harness, release, and agent-readiness documents.

## Verify

- [x] Focused tests pass. Evidence: all changed Go packages pass on Go 1.26.5.
- [x] `task check` passes. Evidence: full tests, race tests, architecture, contract, and hygiene checks pass on the committed clean tree.
- [x] `task security` passes. Evidence: module verification, security guard, gosec, and vulnerability scan pass on the committed clean tree.
- [x] `task public:check` passes. Evidence: public guard and contract lint pass on the committed clean tree.
- [x] `task release:check` passes. Evidence: local release profile includes exact Darwin/arm64 archive replay.
- [x] Runtime-only behavior is observed on the native host. Evidence: extracted Darwin/arm64 archive reports five help contracts, 4 inspection attempts, 5 zero-attempt rejections, 9 total fixture attempts, matching plan digest, and absent canaries.
- [x] Routine-success external-processing count is zero. Evidence: the fixture has no provider transport and the journey uses no external interpreter or model.
- [x] Generated diff and repository status are understood. Evidence: this milestone began from a clean tree and all current paths belong to this work.

## Hand off

- [ ] Acceptance criteria have evidence.
- [ ] Durable decisions are promoted and the temporary packet is removed.
- [x] All implementation is committed by concern and the tree is clean.
- [ ] The milestone goal is completed and the far successor goal is created.
