# Work Plan: v1 compiled tailoring

- Status: Accepted
- Goal: [goal.md](goal.md)
- Context: [context.md](context.md)
- Tasks: [tasks.md](tasks.md)

## Chosen approach

Implement ADR 0004 through vertically testable commits. Establish canonical
catalog and bundle values first, then persistent trust, execution semantics,
and finally the host adapter. Every later layer consumes the same bundle rather
than introducing a parallel registry or policy compiler.

## Delivery slices

1. Accept ADR, theses, product/architecture/security/harness consequences, and
   the finite compatibility corpus.
2. Implement vendor-neutral source identity and adapter contracts, bounded
   GitHub CLI inspection, provenance-aware catalog, canonical encoding,
   inspect/refresh commands, and alternate-adapter conformance fixtures.
3. Implement schema-2 policy init/validation, normalized policy, bundle build,
   content digest, interactive trust receipt, and status/drift diagnostics.
4. Migrate preview/run to bundle consumption; add explain, explicit raw, richer
   typed transform actions, and controlled read/mutation outcomes.
5. Implement a vendor-neutral host decision contract plus project-local Claude
   Code install/status/remove, strict hook input parser, SessionStart surface,
   PreToolUse decisions/rewrite, and one-shot confirmation.
6. Replay complete E2E and agent-readiness journeys, update public/release
   documentation, pass all profiles, remove this packet, and commit the final
   clean tree.

## Alternatives retained as adapters

- The explicit gateway remains the verification and recovery interface.
- Transparent Claude rewriting is the primary routine experience.
- Neither owns bundle compilation or can broaden policy.

## Verification strategy

- Domain tests own finite state and canonical digests.
- Application fakes own ordering, admission, and attempt counts.
- Infrastructure fixtures own process and filesystem boundaries.
- CLI snapshots own discovery, exact argv, stdout/stderr, and recovery.
- E2E fixtures own zero-state through removal.
- Full, security, public, and release profiles decide completion.

## Rollout and migration

Keep schema-1 explicit config commands available but deprecated during v1
development. Introduce no implicit migration or trust. All new persistent
formats are versioned and create-only. Publication remains outside this work.
