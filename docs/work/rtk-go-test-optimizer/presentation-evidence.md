# Presentation Evidence: RTK Go test pass optimizer

This record will be completed from checked-in synthetic typed fixtures before
the optimizer default is promoted. It compares the admitted Go event stream and
the exact independently validated RTK summary; ineligible inputs are rejected
before measurement rather than scored as candidate presentations.

## Frozen semantic corpus

- Typed fixture path: To be added with the implementation
- Fixture SHA-256: Pending
- Presentation-independent answer-key path: To be added with the implementation
- Answer-key SHA-256: Pending
- Declared task and each applicable target, parent, or scope dimension: One
  no-argument current-package Go test result; no Atsura reference target,
  parent, or scope
- Interpretation-relevant absent, empty, zero, false, unresolved, and bounded
  cases: passing output present/absent and elapsed present/zero; failure, skip,
  malformed, unknown, empty, and overflow cases are semantic eligibility
  canaries rather than optimization candidates
- Canonical references and exact next argv: no opaque references; source argv
  is exact `go test -json`

## Semantic eligibility

- [ ] The pass count and package count are independently available from one
      admitted typed source result.
- [ ] Routine success requires zero undeclared joins, parsers, provider-notation
      interpretation, source inspection, or exploratory calls.
- [x] No canonical reference is produced or consumed.
- [x] The task does not return a scoped collection.
- [ ] The answer key explicitly records that per-test output, names, timing, and
      ordering are omitted from the optimized summary.
- [ ] Skip, failure, malformed, unknown-action, identity-conflict, and terminal-
      order canaries cannot be reported as an eligible optimized result.
- [x] Recovery does not construct unchecked source argv.

## Reproducible comparison

| Evidence | Before | After |
|---|---:|---:|
| Golden path | Pending | Pending |
| Golden SHA-256 | Pending | Pending |
| UTF-8 bytes | Pending | Pending |
| Tokens | Pending | Pending |
| Task invocations | 1 | 1 |
| External reconstruction steps | 0 | 0 |

- Golden generator or command: Pending checked-in deterministic generator
- Tokenizer name and exact version: Pending repository-pinned tokenizer
- Tokenizer configuration: Pending
- Platform/runtime facts that affect the measurement: the fixture uses the
  frozen admitted event schema associated with a Go 1.26.x inspection
  observation; it does not prove the runtime-selected toolchain version; exact
  synthetic bytes are platform-independent
- Invalidation rule: regenerate and review when the fixture, answer key,
  validator, RTK compatibility contract, renderer, or tokenizer changes

Byte and token measurements remain secondary to semantic eligibility.

## Experiment outcome

- Outcome: `inconclusive` until implementation evidence is recorded
- Eligible candidates: pending strict pass-only source and exact summary
- Failed or invalidated candidates and reasons: skip, failure, malformed,
  unknown-action, and multi-package failure are ineligible by ADR 0012
- Raw evidence retained at: pending repository-rooted public-safe fixture paths
- Documented gates not implemented by the scorer: runtime identity, isolation,
  attempts, no-byte faults, file/network observation, and final delivery

## Product compatibility decision

- Decision owner: Repository maintainer and product owner
- Selected presentation: exact RTK v0.43.0 `go-test` summary for the admitted
  strict pass-only tuple; exact source result before processor otherwise
- Compatibility rationale: ADR 0012
- Schema/version impact: pending implementation inventory
- Rollout and rollback: explicit processor observation and reviewed bundle;
  choose/regenerate identity specification to roll back
- Relationship to the experiment outcome: the accepted direction remains
  conditional on completing semantic and reproducible evidence

## Security and execution boundary

- [x] Fixtures and evidence will be synthetic and public-safe.
- [ ] Artifact paths are repository-rooted regular files reached without
      symbolic links.
- [ ] Every subprocess has a purpose-bound executable/argv, finite timeout,
      bounded output, private temporary storage, and no inherited secrets.
- [ ] Network is disabled or bounded and documented for native evidence; the
      deterministic local gate does not fetch artifacts.
- [x] No live-model evaluation is required; static fixture/schema checks remain
      deterministic.
