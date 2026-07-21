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
- [x] Add strict per-target evidence upload and five-target aggregation.
- [x] Update durable product, architecture, security, harness, release, and agent-readiness documents.

## Verify

- [x] Focused tests pass on the final implementation. Evidence: race-enabled
  artifact-journey, artifact-evidence, and production CLI tests plus the
  source fixture pass on Go 1.26.5.
- [x] `task check` passes on the final committed clean tree.
- [x] `task security` passes on the final committed clean tree.
- [x] `task public:check` passes on the final committed clean tree.
- [x] `task release:check` passes on the final committed clean tree.
- [x] Runtime-only behavior is observed on the native host. Evidence: the exact
  Darwin/arm64 archive reports 6 help contracts, 4 inspection attempts, 7
  zero-attempt rejections, 10 total fixture attempts, per-command
  preview/execute digest parity, and absent canaries.
- [x] Routine-success external-processing count is zero. Evidence: the fixture has no provider transport and the journey uses no external interpreter or model.
- [x] Generated diff and final repository status are understood. Evidence: the
  milestone began clean, all commits are scoped to this work, and the four
  profiles leave no tracked or untracked repository change.

## Hand off

- [ ] Acceptance criteria have evidence.
- [ ] Durable decisions are promoted and the temporary packet is removed.
- [x] All implementation is committed by concern and the tree is clean.
- [ ] The milestone goal is completed and the far successor goal is created.
