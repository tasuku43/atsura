# ADR 0010: Admit plan-declared source-stream results

- Status: Accepted
- Date: 2026-07-22
- Deciders: Repository maintainer and product owner
- Scope: Identity and argv-only ordinary wrappers, plan output authority,
  process completion, final stream delivery, trust, and release evidence
- Extends: `docs/decisions/0005-purpose-specific-surface-and-wrapper.md`,
  `docs/decisions/0006-adapter-proven-transform-runtime.md`, and
  `docs/decisions/0008-keep-coding-agent-hosts-outside-atsura.md`
- Supersedes: ADR 0006's deferral of identity-wrapper, argv-only-transform,
  and successful nonempty-stderr execution only
- Superseded by: None

## Context

The compiled model already distinguishes identity wrappers from transforming
wrappers and can represent a transform whose only current action is to append
fixed argv. The fresh-plan constructor preserves those facts, the source
process boundary can capture both streams under fixed limits, and the
host-neutral POSIX function forwards ordinary command argv losslessly.

Runtime admission nevertheless requires a typed JSON output stage. This leaves
an identity specification produced by `spec init` non-executable through its
ordinary wrapper and makes output projection an accidental requirement of the
generic wrapper framework. Adding a second raw command would close the symptom
by bypassing the reviewed surface and plan rather than completing them.

An original-preserving optimizer also requires a prior answer to a more basic
question: when does the adopted plan intentionally make source bytes visible?
ADR 0009 therefore defers an RTK tuple until Atsura owns an explicit
original-output authority.

## Decision drivers

- Make identity and argv-only tailoring executable without inventing a
  transformation.
- Keep one bundle, fresh-plan constructor, compatibility boundary, process
  port, and attempt counter.
- Distinguish reviewed source-stream visibility from raw execution and fallback.
- Preserve source stdout, stderr, and conventional status without falsely
  claiming streaming or terminal safety.
- Suppress source bytes whenever process completion is uncertain.
- Keep coding-agent hosts and their rewrite or permission protocols outside
  Atsura.

## Decision

### Plan result mode

Wrapper-plan schema 4 declares exactly one `result_mode`:

- `transformed_json` requires the existing typed JSON output stage and retains
  its compact-JSON, LF-framed, empty-stderr success contract.
- `source_stream_passthrough` requires no output stage. In this slice it admits
  only a complete identity wrapper or a transform whose sole action is fixed
  argv append under the maintained source-adapter grammar.

The second mode is tailored execution. Surface and option resolution,
invocation transformation, exact bundle adoption, current source identity,
adapter compatibility, fresh-plan construction, process bounds, and the
zero-or-one-attempt contract all remain mandatory. It is not raw execution,
an escape hatch, or a fallback selected after another stage fails.

Schema-2 bundle bytes and wrapper-binding contract 1 do not change. A generated
function contains no plan copy; `wrapper run` rebuilds schema 4 after validating
the bound runtime and bundle. Detached schema-3 preview documents remain
non-authoritative and are never executed.

### Source completion

Both result modes use the existing identity-bound, no-shell source-process
port with closed stdin, inherited working directory and environment, one
maximum attempt, a 30-second timeout, a 4 MiB stdout limit, and a 256 KiB stderr
limit.

For `source_stream_passthrough`, one result is a conventional completion only
when all of these facts agree:

- exactly one process attempt occurred;
- the returned executable identity equals the plan-bound identity;
- stdout and stderr are within their declared bounds;
- the process has a non-negative conventional exit status; and
- the port either returned success with status zero or returned the recognized
  source-nonzero outcome with the same nonzero status.

A conventional nonzero source status is a source result, not an Atsura fault,
permission decision, or retry recommendation. A successful source may have
nonempty stderr in this mode. Signal termination, timeout, cancellation,
capture overflow, wait uncertainty, inconsistent port evidence, or identity
uncertainty remains a sanitized non-retryable Atsura fault. Atsura exposes
neither captured stream on such an outcome.

Executable validation and argv-element validation are separate. The
executable remains a nonempty absolute clean path at the bound request; an argv
element may be empty because an empty string is a valid process argument. The
existing list, byte, UTF-8, and structural-text bounds otherwise remain.

### Public stream delivery

The application returns a typed union, not already-written bytes. For a
source-stream result, the CLI:

1. writes the complete captured stdout byte slice once;
2. writes the complete captured stderr byte slice once; and
3. only after both writes complete, returns the conventional source status.

No byte, newline, JSON envelope, visible projection, or diagnostic is added to
either source stream. Delivery is buffered rather than live. Atsura makes no
claim about source timing or stdout/stderr interleaving. If a caller merges the
two destinations, the fixed final write order is stdout then stderr rather
than the source process's original interleaving.

A short or failed final write becomes `execute_output_write_failed`, is
non-retryable, and never recommends replay. Some caller-visible bytes may
already have been written; two output writers cannot provide one atomic commit.
If the diagnostic writer itself has failed, a structured fault may be
unobservable. The implementation must not report the source status after an
incomplete final write.

### Output authority and trust

`wrapper run` retains `fresh_wrapper_plan` as its sole command-level output
authority. Scoped help publishes both result variants rather than describing
the command as JSON-only. The source-stream variant declares exact bytes, no
framing, the fixed bounds, complete buffered delivery, and conventional status.

Trust review records how many included entries expose source streams directly,
in addition to identity/transform, argv, and output-stage counts. This warns
that source output may contain credentials, terminal controls, malformed text,
or prompt-like content before adoption.

`bundle execute` remains the maintainer-facing transformed-JSON evidence
envelope in this slice. The ordinary generated-wrapper path is the first
public source-stream consumer; it still shares the underlying plan application
service with direct application tests.

### Visible-projection exception

Visible projection governs output that Atsura interprets or presents as its own
terminal, TSV, or JSON structure. An explicitly adopted, plan-declared
`source_stream_passthrough` result intentionally returns unprojected source
bytes and therefore makes no terminal-safety, UTF-8, prompt-safety, or semantic-
safety claim. This exception does not allow Atsura to copy those bytes into a
fault, log, trust record, bundle, transcript, or other persisted state.

## Consequences

### Positive

- The generated identity draft becomes a viable ordinary-wrapper baseline.
- Invocation tailoring and output tailoring are independently executable.
- Future finite output processors can build on an explicit original-output
  authority rather than inventing raw fallback semantics.
- The vendor-neutral core gains capability without any coding-agent host or
  external-processor dependency.

### Negative

- Callers that adopt this result mode are responsible for source-byte terminal
  and confidentiality risks.
- Buffered delivery does not preserve streaming latency or cross-stream order.
- Final delivery across stdout and stderr cannot be atomic.
- The plan and help schemas require a version increment and compatibility
  updates even though bundle and generated-function bytes need not change.

## Mechanical enforcement

- Plan validation derives and checks exactly one schema-4 result mode.
- The maintained source adapter proves the complete identity or append-only
  surface and each concrete runtime argv before source start.
- Conventional-completion validation rejects inconsistent error, status,
  attempt, identity, or stream-bound facts.
- Application tests prove zero attempts for pre-start faults, one attempt for
  conventional and uncertain post-start outcomes, and no captured bytes in
  faults.
- CLI tests prove byte-exact stdout/stderr, nonzero status, no framing, fixed
  final-write order, and non-retryable partial-write failure.
- Trust tests prove the source-stream visibility count and confirmation text.
- Exact installed-artifact journeys invoke one identity wrapper and one
  append-argv-only wrapper as ordinary commands on Linux and macOS. Windows
  retains structured unsupported rendering with zero attempts.
- Existing transformed-JSON journeys remain unchanged regressions.
- Completion requires `task check`, `task security`, `task public:check`, and
  `task release:check`.

## Deferred

- Live streaming and source stdout/stderr interleaving preservation
- Signal-status passthrough or stronger platform-specific process handles
- before/after actions and transforms other than fixed argv append
- raw execution, source refresh, and persistent wrapper installation
- RTK or another external processor
- a second source adapter or broader source option grammar
- Windows POSIX wrapper materialization
- coding-agent host adapters, hook decoding, input rewriting, permission
  mapping, settings, process inspection, or activation lifecycle

## Reconsideration signals

Create a successor ADR before streaming bytes prior to verified completion,
exposing captured bytes after uncertain outcomes, treating an unknown process
error as a conventional source result, adding a fallback from transformed
output to source streams, or moving caller-host behavior into Atsura.
