# Work Plan: Surface-and-wrapper correction

- Status: Accepted
- Goal: [goal.md](goal.md)
- Context: [context.md](context.md)
- Tasks: [tasks.md](tasks.md)

## Chosen approach

Correct the governing vocabulary first, then replace the serialized and domain
model as one compatibility break. Preserve only mechanisms whose semantics
remain valid: source evidence, strict codecs, canonical digests, exact adoption,
controlled Atsura-owned mutation, and bounded no-shell process infrastructure.

Do not adapt the superseded permission model into surface composition. Retired
documents and public paths become explicit migration diagnostics. Runtime, raw,
refresh, and host work remain stopped until the corrected pure model passes the
full and security gates.

## Delivery slices

1. **Governance correction.** Accept ADR 0005; align theses, product,
   architecture, security, harness, `AGENTS.md`, the capability Skill, and this
   finite work packet.
2. **Pure schema replacement.** Introduce tailoring specification schema 3 and
   bundle schema 2; model explicit command/option surface plus independent
   identity/transform wrappers; add canonical normalization, digest, surface
   derivation, and pure resolution tests.
3. **Codec and application migration.** Replace policy codecs and init/validate
   orchestration with strict specification equivalents; update bundle build,
   adoption summary, status, and exact output vocabulary.
4. **Effect and migration enforcement.** Add `EffectExecute`, move source
   inspection to it, audit mutation-only switches, and implement stable
   zero-attempt diagnostics for retired authorization schemas and commands.
5. **Remove invalid runtime work.** Delete or retire authorization-centered
   preview/explain/run additions. Add no bundle runtime, raw, refresh, or host
   feature in this milestone.
6. **Verify and commit.** Run focused tests, formatting/lint as needed,
   `task check`, and `task security`; audit remaining vocabulary; review the
   diff; create one intentional correction milestone commit; report remaining
   unknowns and recommend the next user outcome.

## Ordering rationale

The ADR and governing documents choose vocabulary that the types must enforce.
The new domain types then become the source of truth for codecs and application
tasks. Public catalog and migration behavior can be updated only after those
types stabilize. Runtime code is removed before gates so no passing test can
accidentally endorse the superseded authority model.

## Alternatives rejected

- **Rename authorization fields in place.** Rejected because allow/confirm/deny
  and read/create/write do not map to surface membership or wrapper behavior.
- **Keep schema versions and reinterpret bytes.** Rejected because identical
  serialized artifacts would change meaning silently.
- **Auto-convert legacy policy.** Rejected because conversion would invent
  command presence, wrapper completeness, and source-permission semantics.
- **Finish runtime while correcting schemas.** Rejected because it would bind
  execution to an unstable model and exceed the finite milestone.
- **Treat host deny as the core absent state.** Rejected because host values are
  adapter transport vocabulary and hiding is not sandboxing.

## Verification strategy

- Domain tests own specification, wrapper, surface, bundle, digest,
  `EffectExecute`, and resolution invariants.
- Infrastructure tests own bounded strict schema decoding and retired-version
  detection.
- Application tests own compilation/adoption ordering, mutation separation,
  and zero source attempts.
- CLI tests own catalog registration, exact output vocabulary, stable migration
  faults, and recovery commands.
- Repository searches own removal of live authorization-centered source-wrapper
  contracts.
- `task check` and `task security` decide milestone completion.

## Rollout and migration

This is a deliberate pre-release schema break. Specification schemas 1 and 2
and bundle schema 1 are read only far enough to identify them as retired. No
automatic conversion, implicit adoption, or trust migration occurs. Users are
directed to create a new schema-3 specification and build/adopt a schema-2
bundle.

Runtime, raw, and host integration resume in a successor packet only after the
corrected pure surface and wrapper slice is committed and its remaining
unknowns are explicitly selected.
