# Work Goal: Release-quality transform journey

- Status: Active
- Retention: temporary
- Retention reason: None
- Governing contract: `docs/00_theses.md` through `docs/06_release.md`
- Review/delete trigger: Delete after durable conclusions are promoted and the change completes
- Successor: A far, finite goal for the complete vendor-neutral wrapper pipeline
- Owner: Codex
- Target: First transform-runtime release-quality milestone
- Related ADRs: ADR 0005 and ADR 0006

## Outcome

A user can take the same packaged `atr` artifact claimed for their supported
platform through the documented, zero-state GitHub CLI transform journey. The
binary's exact help closes discovery and recovery, including the finite
catalog and schema-3 authoring vocabulary. The user reviews the generated
identity draft, deliberately edits it into the documented built-in transform,
validates it, builds it, and adopts the exact bundle interactively. Separate
composite conformance then proves that each packaged artifact consumes the
same exact-digest receipt, previews with zero source attempts, and executes one
compatibility-admitted source process to return only selected and renamed
typed JSON; synthetic receipt creation is not represented as human consent.

## Why now

The production composition is covered by an in-process fixture, and release
packaging proves deterministic archive contents. The repository does not yet
replay the native executable extracted from each claimed archive, so the
release-quality runtime claim is broader than its executable evidence.

## Non-goals

- Add a public trust bypass or weaken interactive adoption.
- Implement identity-wrapper execution, argv-only transforms, source refresh,
  raw execution, before/after actions, host adapters, or another source CLI.
- Add source-operation authorization semantics, arbitrary code, credentials,
  network access, provider SDKs, publication, or release creation.

## Acceptance criteria

- [x] The documented installed-`atr` journey is self-contained and agrees with exact command help, finite catalog/schema authoring metadata, and structured faults.
- [x] A credential- and network-free native fixture proves four inspection attempts, zero preview attempts, one execute attempt, stable plan identity, and selected/renamed JSON without an unselected canary.
- [ ] Every claimed release OS/architecture runs the exact `atr` executable extracted from its packaged archive in CI.
- [x] Exact-artifact conformance seeds only an isolated non-shipped receipt through the production store; existing production tests separately prove controlling-terminal full-digest consent and no release command exposes a bypass.
- [x] The release gate detects removal or weakening of native artifact replay.
- [ ] `task check`, `task security`, `task public:check`, and `task release:check` pass on one clean committed tree.

## Governing documents

- Thesis: deterministic evidence-to-bundle-to-plan core; source-owned semantics; no authorization or arbitrary-code model.
- Product contract section: current zero-state artifact workflow and transform execution boundary.
- Architecture or security invariant: controlled process boundary, exact source identity, fail-closed runtime admission, secret-free structured output.
- Existing ADR: ADR 0006 accepts only the adapter-proven typed JSON transform runtime.

## Completion definition

The work is complete when every acceptance criterion has executable evidence,
durable release and harness decisions are promoted, all required profiles pass,
the final tree is committed and clean, and this temporary packet is removed.
