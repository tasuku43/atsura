# Presentation Evidence: RTK Go test pass optimizer

This record compares the checked-in admitted Go event stream with the exact
independently validated RTK summary. Ineligible inputs are rejected before
measurement rather than scored as candidate presentations. Installed native
artifact evidence remains separate and incomplete.

## Frozen semantic corpus

- Typed fixture path: `internal/infra/gotestjson/testdata/pass.jsonl`
- Fixture SHA-256:
  `a876a23b60dad0984d822f98c2ed5a94f82e368e985bdd19e5bd5bb90a733885`
- Presentation-independent answer-key path:
  `internal/infra/gotestjson/testdata/pass.answer.json`
- Answer-key SHA-256:
  `060e4e2ee88ced24bc53d5916f953588c252165134250c2f24c7fd5d0ab67a95`
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

- [x] The pass count and package count are independently available from one
      admitted typed source result.
- [x] Routine success requires zero undeclared joins, parsers, provider-notation
      interpretation, source inspection, or exploratory calls.
- [x] No canonical reference is produced or consumed.
- [x] The task does not return a scoped collection.
- [x] The answer key explicitly records that per-test output, names, timing, and
      ordering are omitted from the optimized summary.
- [x] Skip, failure, malformed, unknown-action, identity-conflict, and terminal-
      order canaries cannot be reported as an eligible optimized result.
- [x] Recovery does not construct unchecked source argv.

## Reproducible comparison

| Evidence | Before | After |
|---|---:|---:|
| Golden source | `pass.jsonl` | answer-key `summary` bytes |
| Container SHA-256 | `a876a23b60dad0984d822f98c2ed5a94f82e368e985bdd19e5bd5bb90a733885` | answer key: `060e4e2ee88ced24bc53d5916f953588c252165134250c2f24c7fd5d0ab67a95` |
| Compared-byte SHA-256 | `a876a23b60dad0984d822f98c2ed5a94f82e368e985bdd19e5bd5bb90a733885` | `a4f3dee01192dc3d1e710a3301d7f9f35bf7e7f14135b4a96ce398dc3af043b4` |
| UTF-8 bytes | 1,273 | 31 |
| Terminal LF | exactly one | none |
| Tokens | not asserted | not asserted |
| Task invocations | 1 | 1 |
| External reconstruction steps | 0 | 0 |

- Exact after bytes: `Go test: 2 passed in 1 packages` with no trailing LF
- Reproduction commands:

  ```sh
  shasum -a 256 internal/infra/gotestjson/testdata/pass.jsonl \
    internal/infra/gotestjson/testdata/pass.answer.json
  wc -c internal/infra/gotestjson/testdata/pass.jsonl
  printf '%s' 'Go test: 2 passed in 1 packages' | wc -c
  printf '%s' 'Go test: 2 passed in 1 packages' | shasum -a 256
  ```

- Golden generator: none; the synthetic fixture and independent answer key are
  reviewed checked-in inputs.
- Tokenizer contract: none. Atsura has not accepted a vendor-neutral tokenizer,
  so this record intentionally makes no token-count or token-reduction claim.
- Platform/runtime facts that affect the measurement: the fixture uses the
  frozen admitted event schema associated with a Go 1.26.x inspection
  observation; it does not prove the runtime-selected toolchain version; exact
  synthetic bytes are platform-independent
- Invalidation rule: regenerate and review when the fixture, answer key,
  validator, RTK compatibility contract, or renderer changes

The exact byte measurement remains secondary to semantic eligibility. Token
measurement is outside this evidence contract.

## Experiment outcome

- Outcome: semantic eligibility and exact byte reduction are recorded; native
  installed-artifact validation remains pending
- Eligible candidates: the checked-in strict pass-only source and its exact
  independent summary
- Failed or invalidated candidates and reasons: skip, failure, malformed,
  unknown-action, and multi-package failure are ineligible by ADR 0012
- Raw evidence retained at:
  `internal/infra/gotestjson/testdata/pass.jsonl` and
  `internal/infra/gotestjson/testdata/pass.answer.json`
- Documented gates not implemented by this presentation comparison: runtime
  identity, isolation, attempt accounting, no-byte faults, and final delivery.
  Child-process, filesystem, and network absence are not asserted because no
  external observer contract has been accepted.

## Evidence boundary

- The exact official RTK artifact deterministically emits the 31-byte summary
  for this admitted fixture; that is the installed `optimized` case.
- `preserved_before_processor` skip/failure/ineligible cases do not start RTK
  and remain deterministic installed-wrapper cases.
- `preserved_after_processor` remains a required controlled application truth-
  table case. Atsura starts RTK only when the expected summary is strictly
  smaller, and the fixed official RTK invocation deterministically emits that
  summary, so no official-artifact fixture reaches byte-identical passthrough.
- Processor one-attempt failure/no-byte branches remain required controlled
  application/infrastructure tests. They are not presented as deterministic
  official-artifact cases, and the runtime no-fallback/no-byte requirements are
  unchanged.

## Product compatibility decision

- Decision owner: Repository maintainer and product owner
- Selected presentation: exact RTK v0.43.0 `go-test` summary for the admitted
  strict pass-only tuple; exact source result before processor otherwise
- Compatibility rationale: ADR 0012
- Schema/version impact: versioned specification, bundle, plan, result, and
  processor-evidence contracts carry the finite optimizer tuple; no arbitrary
  executable/argv or coding-agent-host field is added
- Rollout and rollback: explicit processor observation and reviewed bundle;
  choose/regenerate identity specification to roll back
- Relationship to the experiment outcome: semantic and reproducible byte
  evidence is complete for the frozen fixture; native installed-artifact and
  repository-gate evidence remain completion conditions

## Security and execution boundary

- [x] Fixtures and evidence are synthetic and public-safe.
- [x] Fixture and answer-key paths are repository-rooted regular files reached
      without symbolic links.
- [x] Controlled process-boundary tests require a purpose-bound
      executable/argv, finite timeout, bounded output, private temporary
      storage, and no inherited secrets.
- [ ] External observer contracts establish any claimed child-process,
      filesystem, or network activity bounds for native evidence. Until then,
      no absence claim is made.
- [x] The deterministic local fixture/schema gate does not fetch artifacts or
      use a network.
- [x] No live-model evaluation is required; static fixture/schema checks remain
      deterministic.
