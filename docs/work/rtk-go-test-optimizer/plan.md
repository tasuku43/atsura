# Work Plan: Optimize passing Go test output with inspected RTK

- Status: Approved
- Goal: [goal.md](goal.md)
- Context: [context.md](context.md)
- Tasks: [tasks.md](tasks.md)

## Chosen approach

Implement ADR 0012 as one vertical slice: explicit RTK inspection, one finite
Go/RTK compatibility tuple, typed optimizer schemas, strict pre/post semantic
validation, isolated processor execution, shared wrapper application,
and native official-artifact evidence. Preserve conventional ineligible source
results before processor start; fail closed after processor authority begins.

## Alternatives considered

### Send all Go JSON output to RTK

Rejected because official-artifact hostile fixtures prove successful but
misleading skip, malformed, failure, and nondeterministic results.

### Implement only an internal Go summary

Rejected for this slice because it would not test the accepted external-
processor thesis or reuse RTK. Atsura computes the postcondition but RTK remains
the user-visible optimizer.

### Reuse the source process runner

Rejected because it closes stdin, inherits working directory/environment, owns
source-specific faults, and cannot account for source and processor attempts
separately. A processor-specific controlled boundary is required.

## Design

### Public contract

- `processor inspect` is a `RoleUtility`, `EffectExecute` command. It accepts one
  fixed adapter selector and one absolute executable path, produces no opaque
  references, and returns one complete strict JSON observation.
- Existing authoring/build commands accept explicit processor evidence. They do
  not discover, install, or start RTK and reject unused or incompatible input.
- Existing preview/status/trust surfaces include the bound processor identity,
  compatibility contract, reason, limits, and original-output allowance.
- The existing rendered POSIX wrapper and `wrapper run` path consume one fresh
  plan and return ordinary stdout/stderr/status. `bundle execute` remains a
  projection-only JSON evidence envelope and rejects this mode before source
  start. Optimizer disposition and attempt facts remain structured
  application/evidence facts rather than injected text.
- Faults distinguish inspection, compatibility, preflight, source,
  pre-processor preservation, processor execution, postcondition, drift,
  cancellation, delivery, and recovery without claiming retry safety after a
  source attempt.
- Delivery is complete and bounded; collection coverage and pagination are not
  applicable. No authentication is owned by Atsura.

### Layer changes

- Domain: add processor observation/request/result invariants; migrate the
  specification/bundle output stage to a discriminated projection-or-optimizer
  union; add processor-bound plan/result facts and attempt/disposition truth
  tables.
- Application: add processor inspection and one finite compatibility registry;
  extend spec initialization, bundle compilation, plan application, wrapper
  execution, status, and trust summaries through owned ports. Keep source
  runtime and processor compatibility separate.
- Infrastructure: add RTK version inspector, strict observation codec, frozen
  Go pass-event validator, and isolated stdin processor runner. Extend Go
  runtime only for exact `test -json` admission, without claiming runtime
  toolchain closure.
- CLI and catalog: register processor inspection, evidence inputs, schema/fault
  contracts, result modes, help, and presentation from the canonical catalog.

### Data and control flow

```text
explicit RTK path -> inspect once -> canonical observation
                                      |
Go catalog + observation -> spec init -> reviewed optimizer spec
                                      |
spec + catalog + observation -> bundle build -> preview/adopt -> fresh plan
                                      |
processor preflight -> exact go test -json once -> conventional result
                                      |
                 ineligible --------------------> preserved_before_processor
                                      |
                 eligible -> processor recheck -> isolated RTK once
                                      |
                 exact input -------------------> preserved_after_processor
                 smaller exact summary ---------> optimized delivery
                 any processor uncertainty -----> no-byte fault
```

### Error and cancellation behavior

- Invalid observations, unsupported tuples, incomplete surfaces, and missing or
  drifted processor preflight fail before source start.
- Conventional source completion can be status zero or nonzero. Ineligibility
  preserves exact bytes/status with zero processor attempts; uncertain source
  completion suppresses bytes and remains non-retryable.
- Eligible input is followed by one identity recheck and at most one processor
  start. Processor uncertainty/failure/postcondition mismatch suppresses all
  source and processor bytes and is non-retryable.
- Successful bytes are written stdout then stderr where applicable, followed by
  source/optimizer status. Any final write failure is non-retryable because the
  source may already have had effects.
- Cancellation before any process reports zero attempts; cancellation after
  source or processor start reports the truthful attempts and no replay advice.

### Security and public boundary

- The source and its output remain untrusted. RTK is also untrusted local code
  constrained by exact identity, argv, stdin, environment, cwd, attempts,
  timeout, bytes, and semantic postconditions.
- RTK receives no source stderr, credentials, agent-host configuration, project
  TOML, or arbitrary command. Temporary roots are private and removed; cleanup
  uncertainty fails closed.
- Native evidence downloads only pinned official public artifacts, verifies
  checksums before extraction, and never checks an RTK binary into the repo or
  Atsura archive.
- The isolated environment is a runtime invariant, not evidence that RTK starts
  no child process or performs no filesystem or network activity. Those claims
  require platform-specific external observer contracts and remain unasserted
  until such observers are implemented and validated. No OS sandbox claim is
  made.

### Evidence partition

- Installed journeys using the exact official RTK artifacts cover only cases
  reachable deterministically through `rtk pipe --filter=go-test`: optimized;
  `preserved_before_processor` for skip, failure, and other ineligible source
  results; projection-facade rejection; processor preflight drift; eligible
  post-source processor drift; and Windows optimizer non-support.
- Controlled application and infrastructure truth tables cover
  `preserved_after_processor` plus processor start, timeout, signal,
  cancellation, nonzero, stderr, overflow, post-run identity drift, unexpected
  stdout, and cleanup failure. They retain the one-attempt/no-byte/no-fallback
  requirements but are not described as official-artifact journeys.
- Archive checksum, extracted binary identity, version, platform, fixed argv,
  stdin, isolation, and semantic-result evidence remain mandatory. Source
  attempts are observed by the controlled source fixture; processor-attempt
  truth remains mandatory in application/infrastructure tests and becomes an
  installed-evidence claim only when an external observer proves it. External
  child-process, filesystem, and network observations are separate contracts
  and are not claimed until proven.

## Implementation slices

1. Accept ADR 0012, add this packet, freeze schemas/faults/provenance, and add
   failing domain/codec contract tests.
2. Implement processor observation, RTK inspection, strict codec, and finite
   processor compatibility registry.
3. Migrate specification, bundle, plan, trust/status/help, and authoring/build
   inputs to the typed optimizer union and exact identity binding.
4. Implement Go `-json` runtime admission, strict pass lifecycle validator,
   isolated processor execution, and shared wrapper result delivery.
5. Add hostile truth tables, presentation evidence, provenance manifest,
   capability/docs/harness propagation, native installed-artifact journeys,
   aggregate validation, and canonical gates.

Each concern is committed only after its focused tests pass. The repository is
kept buildable at commit boundaries; schema migrations are not split across a
publicly inconsistent commit.

## Verification

- Unit and contract tests: domain unions, observation codec, compatibility
  registry, Go lifecycle validator, processor runner, plan application, CLI,
  help, trust/status, and schema inventories.
- Negative side-effect tests: invalid evidence and drift are zero source calls;
  ineligible source is zero processor calls; processor failure leaks no bytes.
- Opaque-reference and pagination tests: not applicable because this utility
  produces no references and has complete noncollection output; graph lint must
  continue to pass.
- Structured output, hostile output, and recovery: BOM/CR/blank records,
  duplicates, unknown fields/actions, malformed and oversized records, terminal
  contradictions, non-beneficial summaries, stderr, statuses, signals,
  cancellation, write failure, and stable exact recovery commands.
- Agent readiness: root help discovers processor inspection and exact scoped
  help supplies inputs/faults in the existing discovery-round-trip budget;
  ordinary wrapper execution requires no undeclared parser or external semantic
  reconstruction.
- Human handoff: installation remains outside Atsura; exact path inspection and
  recovery instructions state that no credential or host configuration is
  requested.
- Installed-artifact observation: each claimed official RTK artifact is pinned
  and replays the deterministic reachable cases listed above under
  `atsura.processor.rtk_isolated.v2`. `preserved_after_processor` and processor
  failure/no-byte branches are verified by controlled application and
  infrastructure tests instead.
- External observation: add child-process, filesystem, or network absence
  claims only after an explicit observer contract is implemented and validated
  on the claimed platform; until then these facts remain unasserted.
- Required profiles: `task check`, `task security`, `task public:check`, and
  `task release:check` on one revision.
- Generated/artifact checks: clean generated diff, official checksum/binary
  identity, release archive member set without RTK, native semantic evidence,
  and aggregate comparison.

## Rollout and rollback

The new schemas are versioned and old artifacts are rejected rather than
reinterpreted. Existing users regenerate specifications and bundles. RTK is
selected only when explicitly observed; removing the processor observation or
choosing identity before compilation rolls back the default. Adopted bundles
remain content-addressed, so replacing RTK or changing the spec requires a new
bundle and adoption. No persisted secrets, RTK state, or host settings exist.

## Documentation promotion

- Mark the first RTK tuple implemented in theses and remove the corresponding
  unknown/deferred claims.
- Specify processor inspection, optimizer authoring/build/execution, trust,
  faults, and compatibility in the product contract.
- Record separate source/processor registries and controlled processor runtime
  in architecture.
- Record isolated environment, pre/postvalidation, no-byte processor faults,
  and provenance in security.
- Record exact hostile/native/artifact evidence and schemas in harness/release
  docs and the capability/provenance ledgers.
