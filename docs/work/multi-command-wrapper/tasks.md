# Work Tasks: One wrapper serves one complete multi-command surface

- Goal: [goal.md](goal.md)
- Plan: [plan.md](plan.md)

## Understand

- [x] Read governing theses, product, architecture, security, and harness.
- [x] Reproduce the exact-one-entry admission boundary by code and test review.
- [x] Record verified facts and unknowns in `context.md`.
- [x] Record the single-command friction as thesis evidence.
- [x] Confirm the public outcome and non-goals in `goal.md`.

## Decide

- [x] Compare multi-command admission, typed defaults, and persistent shims.
- [x] Identify public-contract and compatibility impact.
- [x] Keep the existing utility role, reference flow, and read effect.
- [x] Classify this as an expansion of the existing public wrapper capability.
- [x] Confirm there is no new I/O, credential, mutation, or host boundary.
- [x] Add and accept ADR 0015 before completing the mechanism.
- [x] Confirm no thesis exception is required; the change removes a temporary
      implementation limit in favor of the current thesis.
- [x] Product owner authorized Codex to select the recommended direction.

## Implement

- [x] Add failing complete-surface truth-table tests.
- [x] Implement all-entry GitHub runtime admission.
- [x] Add same-bundle per-command plan/render integration tests.
- [x] Confirm no cataloged CLI description retains an exact-one-command claim.
- [x] Add strict installed-artifact multi-command evidence and mutation tests.
- [x] Update durable documentation and harness contracts.
- [x] Complete `presentation-evidence.md` from one frozen two-command fixture.

## Verify

- [x] Focused tests pass. Evidence: related packages pass with `-race -count=1`.
- [x] `task check` passes. Evidence: exact revision
      `a79a637d3067c86c72e77862ad06382f679d9d5c`.
- [x] `task security` passes. Evidence: exact revision
      `a79a637d3067c86c72e77862ad06382f679d9d5c`; no
      vulnerabilities, security guard OK.
- [x] `task public:check` passes. Evidence: exact revision
      `a79a637d3067c86c72e77862ad06382f679d9d5c`; public guard and contract
      lint OK.
- [x] `task release:check` passes. Evidence: exact revision
      `a79a637d3067c86c72e77862ad06382f679d9d5c`; release lint OK.
- [ ] Runtime-only behavior was observed on every claimed native target.
- [x] One ordinary root help call exposes the complete two-command surface and
      each exact call closes its outcome without external reconstruction.
- [x] Routine success requires zero external processor attempts for both
      commands; each command starts exactly one source attempt.
- [x] Generated diff and repository status are understood. Evidence: all
      product changes are split by concern; the exact local replay left a clean
      worktree and removed its temporary archive.

## Hand off

- [ ] Acceptance criteria have evidence.
- [ ] Durable decisions are promoted out of this packet.
- [ ] Temporary diagnostics and sensitive artifacts are removed.
- [ ] Follow-up work is explicit and does not block this goal.
- [ ] Temporary packet is removed after native evidence passes.
