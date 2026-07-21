# Work Goal: Preview one adopted wrapper plan

- Status: Active
- Retention: temporary
- Retention reason: None
- Governing contract: docs/00_theses.md and docs/decisions/0005-purpose-specific-surface-and-wrapper.md
- Review/delete trigger: Delete after durable plan-preview conclusions are promoted, gates pass, and the milestone is committed
- Successor: A separately scoped bundle runtime packet that reuses the exact plan constructor
- Owner: Repository maintainer
- Target: Finite zero-execution wrapper-plan preview milestone
- Related ADRs: docs/decisions/0005-purpose-specific-surface-and-wrapper.md

## Outcome

A maintainer can pass one adopted schema-2 bundle and one attempted source
invocation to `atr bundle preview` and receive the complete deterministic
tailored wrapper plan, including exact source identity, matched command,
surface/option binding, original and transformed argv, ordered typed stages,
reason, mode, digest, and declared runtime attempt count. Preview starts zero
source processes. A command or option outside the tailored surface produces no
plan and an exact structured recovery.

## Why now

ADR 0005 corrected the product from source authorization to purpose-specific
surface and wrapper composition. The pure bundle model and exact-digest
adoption now pass full and security gates, but a maintainer cannot yet inspect
how one attempted invocation resolves. This is the smallest end-to-end result
that tests the corrected thesis before runtime execution resumes.

## Non-goals

- Starting the source process or transforming live output.
- Raw execution, source refresh, hook installation, or any host adapter.
- Source allow/confirm/deny, read/create/write inference, target, or impact.
- New wrapper actions, argv replacement/defaulting, or new output transforms.
- Accepting unadopted bundles, stale source identity, unknown commands, hidden
  options, unmodeled short options, or implicit fallback.
- Reinterpreting or removing the migration-only legacy `plan preview` path.
- Publication, push, pull request, tag, or release.

## Acceptance criteria

- [ ] `bundle preview` is catalog-discoverable as a read-only utility with
  public capability `tailoring.preview`, exact positional-only argv grammar,
  schema-2 JSON output, and complete declared faults/recoveries.
- [ ] One pure plan constructor validates the bundle and attempted invocation,
  resolves the longest exact catalog command prefix, enforces tailored option
  membership, appends wrapper argv deterministically, and emits a stable plan
  digest.
- [ ] The plan contains no authorization decision or inferred source mutation
  facts and preserves explicit empty before/after/output states.
- [ ] Application orchestration requires exact-digest adoption and current
  source path/hash/size before plan construction; every failure and preview
  itself starts zero source processes.
- [ ] Excluded commands return `command_not_in_surface`; catalog-unknown or
  ambiguous argv returns `invalid_invocation`; excluded or unmodeled options
  fail without constructing a plan.
- [ ] Legacy authorization preview remains a separate zero-execution migration
  diagnostic, and current plan output uses schema version 2.
- [ ] Product, architecture, security, harness, README, capability ledger, and
  agent-readiness scenario describe the implemented preview boundary honestly.
- [ ] Focused tests, `task check`, and `task security` pass on one final tree;
  the runtime executor is still absent.
- [ ] The milestone is committed intentionally, the temporary packet is
  removed, the worktree is clean, and no publication action occurs.

## Governing documents

- Thesis: `docs/00_theses.md`, Thesis 4
- Product contract: `docs/01_product_contract.md`, Wrapper execution plan
- Architecture: `docs/02_architecture.md`, pure plan constructor
- Security: `docs/03_security_model.md`, Wrapper integrity
- Harness: `docs/04_harness.md`, Wrapper plan contract
- Existing ADR: `docs/decisions/0005-purpose-specific-surface-and-wrapper.md`

## Completion definition

The goal is complete only when every acceptance item has direct executable
evidence, preview and the future runtime boundary share the exported pure plan
constructor by construction, all zero-attempt negative paths are tested, full
and security gates pass, the milestone commit succeeds, and this temporary
packet is removed from the final tree.
