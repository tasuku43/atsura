# ADR 0007: Prefer explicit RTK-backed optimizer defaults

- Status: Accepted
- Date: 2026-07-22
- Deciders: Repository maintainer and product owner
- Scope: Output-stage vocabulary, authoring defaults, external processor
  identity, bundle trust, failure policy, and compatibility evidence
- Extends: docs/decisions/0005-purpose-specific-surface-and-wrapper.md and
  docs/decisions/0006-adapter-proven-transform-runtime.md
- Supersedes: None
- Superseded in part by:
  `docs/decisions/0009-reject-ambiguous-rtk-git-log-tuple.md`, for the proposed
  first Git `log` / `git-log` compatibility tuple only

## Context

The first runtime proves a strict typed JSON projection implemented inside
Atsura. The product owner wants commands already supported by RTK to use RTK by
default rather than duplicating its large command-filter catalog.

Primary-source review of stable RTK `v0.43.0` found useful deterministic output
filters and a narrow `rtk pipe --filter=<name>` boundary. It also found an
important semantic mismatch: RTK deliberately returns the original bytes when
a parser degrades, a filter panics, or its never-worse guard prefers the input.
That is appropriate for best-effort output optimization but cannot satisfy a
strict projection contract that promises selected fields and no raw fallback.

RTK's ordinary command paths also resolve and start source CLIs themselves,
may make more than one source call, and use RTK-owned tracking, tee, and
configuration behavior. Delegating source execution would give up Atsura's
exact source identity, argv, attempt, status, stderr, and plan boundaries.

## Decision drivers

- Reuse RTK's maintained filters wherever Atsura can prove compatibility.
- Keep authoring defaults reviewable and attributable to an exact bundle.
- Preserve Atsura's identity-bound, no-shell, one-source-attempt boundary.
- Distinguish an output projection from an optimization that may preserve its
  admitted input.
- Do not let RTK implicitly persist command strings, raw output, project paths,
  tracking data, or telemetry as a consequence of using the processor.
- Expand from one exact compatibility fixture rather than treating RTK's
  marketing support list as a stable machine contract.

## Decision

### Two output-stage meanings

A strict typed projection and an original-preserving optimizer are different
stage contracts.

- A projection promises a declared output shape. It fails closed and never
  exposes its input when that promise cannot be satisfied.
- An optimizer promises a bounded attempt to produce a smaller agent-facing
  representation. It may return either `optimized` output or the byte-identical
  admitted input as `preserved`, but only when the reviewed plan explicitly
  states that the original stage input is itself allowed agent-facing output.

`preserved` is not raw execution. Surface resolution, invocation transforms,
source identity checks, source execution, and every preceding stage still
apply. It is also not a general claim that RTK preserves all output semantics;
RTK intentionally removes information when it optimizes.

RTK exposes no disposition metadata at this boundary. Atsura therefore labels
only the observable byte result: valid processor stdout equal byte-for-byte to
the admitted input is `preserved`; any different valid stdout is `optimized`.
The label does not infer whether RTK parsed, recovered, panicked, or applied its
never-worse guard internally.

### RTK-preferred authoring default

When Atsura's maintained compatibility registry proves an exact mapping from a
source adapter/version/command contract to an RTK processor contract, the
authoring workflow materializes an explicit RTK-backed optimizer as the default
wrapper choice instead of reimplementing the same optimization. A user or
proposing agent may explicitly replace that choice before compilation.

The default is materialized into the tailoring specification before
compilation. The compiler and runtime never detect an installed RTK binary and
insert it implicitly. The user reviews and adopts the exact resulting bundle.
Outside Atsura's proven RTK compatibility matrix no RTK default is generated.
A built-in or identity alternative is offered only when Atsura maintains its
own explicit contract; otherwise authoring reports no maintained default.

The current schema and runtime do not yet implement this stage. Until the
bounded slice below is accepted, RTK actions remain invalid and the capability
ledger reports the default as deferred.

### Controlled processor boundary

Atsura starts the exact source executable once and admits its status, stderr,
and bounded stdout before starting RTK. The initial RTK path is only:

```text
exact successful source stdout
  -> exact RTK executable
  -> pipe --filter=<one compatibility-bound name>
  -> optimized bytes or byte-identical preserved input
```

RTK never receives authority to select or start the source CLI. `rtk run`,
`rtk proxy`, command-specific RTK source wrappers, auto-detected pipe filters,
project TOML filters, and arbitrary RTK argv are outside this contract.

The specification binds a namespaced compatibility-contract identifier, not an
executable path or argv. Infrastructure maps that finite identifier to fixed
processor argv. The bundle and plan then bind sufficient identity and
compatibility evidence for the explicitly observed RTK path, SHA-256, size,
version, adapter contract, filter, input contract, original-output allowance,
source and processor attempt limits, time and byte limits, isolated
environment, and reason. Bundle build must receive this observation through an
explicit reviewed input boundary; it never discovers RTK from ambient `PATH`.
Exact schema fields remain a responsibility of the implementing vertical slice.

Missing or drifted RTK identity at preflight fails before the source starts.
After successful source execution, Atsura revalidates the processor before
start; a missing or changed processor at that phase is non-retryable with one
source attempt and zero processor attempts. A source failure starts no
processor. Any processor start, timeout, nonzero status, stderr, post-start
drift, overflow, cancellation, or malformed result is non-retryable and exposes
neither failed intermediate bytes nor RTK stderr.

### No implicit RTK state

The processor runs without a shell, with closed stdin except for its bounded
stage input, an isolated working directory and configuration roots, and a
minimal explicit environment. At minimum it disables telemetry, tee, and TOML
lookup defensively and uses an exact filter. For every claimed platform,
compatibility evidence records that the exact native artifact and invocation
were observed to create no tracking database, tee file, hook marker, telemetry
marker, or other file outside isolated temporary roots, and to attempt no
network I/O within the harness's declared observation scope.

Environment flags are defense in depth, not the proof. The native exact-RTK
fixture observes filesystem and network behavior because other RTK command
paths do not consistently honor one global tracking disable. This evidence is
not an OS or network sandbox, and the portable processor identity checks retain
a check-to-exec race between the last observation and process open.

### Historical first compatibility proposal

This ADR originally recommended a second source adapter for Git with one exact
`git log` invocation contract and `rtk pipe --filter=git-log`. ADR 0009 rejects
that tuple after a hostile valid commit body demonstrated an ambiguous literal
delimiter and a successful but misleading association. The broader controlled
processor and explicit-default decisions remain accepted.

No replacement first tuple is accepted. `git diff` remains deferred because its
current filter may emit an RTK-specific recovery hint, and `git status` remains
deferred because RTK's complete status path performs additional source
observations that `rtk pipe` alone does not own.

## Alternatives considered

### Treat RTK as a strict projection backend

Rejected for the general `v0.43.0` pipe contract. It provides no metadata that
distinguishes parser degradation, panic recovery, never-worse selection, and a
legitimate unchanged transformation. Individual filters may later add a
stronger postcondition, but that proof is filter-specific.

### Delegate source execution to RTK

Rejected. It would move source selection, attempts, status, stderr, tracking,
and fallback outside Atsura's canonical plan and controlled process boundary.

### Vendor or port RTK filters now

Rejected for the first proof. RTK has no Go library API, its filter behavior is
fast-moving, and copying it would recreate the maintenance burden this decision
is intended to avoid. A subprocess adapter is the narrower experiment.

### Keep RTK merely optional

Rejected as the product direction. Where Atsura proves the exact compatibility
contract, the authoring default should use RTK. Optional manual selection
remains possible, but it should not force every user to rediscover the preferred
backend.

## Consequences

### Positive

- Atsura can reuse RTK breadth without adopting its source-execution model.
- Defaults remain explicit, reviewable, content-addressed, and deterministic.
- Projection confidentiality and best-effort optimization no longer share an
  ambiguous failure contract.
- The first external processor has a finite identity and no-state test surface.

### Negative

- RTK's full advertised command set is not immediately eligible.
- One managed source command may involve a second local process after the one
  source attempt.
- Original output may remain agent-visible for an adopted optimizer stage.
- Each RTK version/filter/source tuple needs native compatibility evidence and
  license/provenance maintenance.

## Mechanical enforcement target

The implementing slice must add:

- domain truth tables separating projection from optimizer behavior;
- canonical specification, bundle, plan, trust-summary, and result facts;
- tests proving the authoring default is materialized, an explicit override is
  reviewable, and compiler/runtime never insert RTK implicitly;
- exact source and processor identity, argv, attempt, time, byte, stdin,
  working-directory, environment, preflight drift, post-source revalidation,
  and portable check-to-exec race tests;
- source-failure/processor-zero and source-one/processor-one accounting;
- `optimized` versus byte-identical `preserved` results derived by comparing
  successful valid processor stdout with admitted input, with preservation
  invalid unless the plan explicitly permits original output and no claim about
  RTK's internal branch;
- non-retryable processor failure with no source, processor, or stderr leak;
- hostile configuration, bounded native file/network observation, and no-secret
  persistence fixtures without a sandbox claim;
- exact native RTK artifact replay on every claimed platform;
- Apache-2.0 provenance, license, notice, dependency, and SBOM review; and
- the canonical repository gates required by the affected public and release
  contracts.

## Reconsideration signals

Create a successor ADR before allowing an arbitrary external processor,
delegating source execution to RTK, auto-detecting a filter at runtime,
silently preserving input for a strict projection, enabling RTK project
configuration or persistence, treating RTK's advertised support list as
compatibility proof, or claiming that optimized output preserves all source
information.
