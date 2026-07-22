# Work Goal: Execute identity and argv-only ordinary wrappers

- Status: Active
- Retention: temporary
- Retention reason: None
- Governing contract: `docs/00_theses.md`, `docs/01_product_contract.md`,
  `docs/02_architecture.md`, and `docs/03_security_model.md`
- Review/delete trigger: Delete after durable conclusions are promoted and the change completes
- Successor: None
- Owner: Repository maintainer and product owner
- Target: Current tailoring iteration
- Related ADRs: ADR 0005, ADR 0006, ADR 0008, and the source-stream ADR created by this work

## Outcome

A maintainer can adopt a bundle whose admitted command uses either an identity
wrapper or an argv-only transform, render the existing host-neutral POSIX
function, and invoke the ordinary source command through it. Atsura rebuilds
one fresh plan, executes the exact source at most once, and returns the bounded
source stdout bytes, stderr bytes, and conventional exit status under an
explicit reviewed result mode. This remains a tailored execution path, not a
raw bypass.

## Why now

The specification, bundle, and plan already represent identity and argv-only
wrappers, but the runtime admits only a GitHub-specific JSON transform. That
gap makes the generic wrapper abstraction appear to require output projection
and blocks any later original-preserving processor such as RTK. Closing the
output-authority contract first tests the vendor-neutral core without adding a
new source CLI, external processor, coding-agent host, or lifecycle mechanism.

## Non-goals

- Raw or policy-bypassing source execution
- Arbitrary shell, script, jq, plugin, RTK, or runtime-LLM processing
- before/after executable actions or output transformation
- Timing preservation, streaming delivery, or stdout/stderr interleaving
- Persistent shim installation, multiple profiles, or source refresh
- Coding-agent hook parsing, rewriting, settings, process inspection, or host adapters
- Windows POSIX wrapper support
- A broader GitHub CLI grammar or a second source adapter

## Acceptance criteria

- [ ] Schema-4 fresh plans declare exactly one result mode:
      `transformed_json` or `source_stream_passthrough`.
- [ ] Identity and append-argv-only bundles are admitted only for the existing
      finite source adapter, command, surface, option, identity, adoption, and
      fresh-plan contracts.
- [ ] A conventional source completion returns captured stdout and stderr
      byte-for-byte within the existing 4 MiB and 256 KiB limits, adds no
      framing, and returns the source status only after both final writes succeed.
- [ ] A nonzero conventional source status is a source result, not an Atsura
      retry recommendation; signal, timeout, cancellation, overflow, identity,
      wait, or final-write uncertainty remains non-retryable and never exposes
      captured source bytes through an Atsura fault.
- [ ] One identity fixture and one append-argv-only fixture prove preview,
      direct plan application, and ordinary POSIX wrapper invocation use the
      same fresh plan, exact executable identity, exact transformed argv, and
      zero-or-one source attempt contract.
- [ ] Empty argv values, Unicode, dash-prefixed values, and `--` remain
      lossless wherever the finite catalog grammar admits them.
- [ ] Scoped human and agent help honestly describe the result variants,
      bounds, framing, exit behavior, and recovery without claiming terminal,
      UTF-8, semantic, timing, or interleaving safety for source streams.
- [ ] Linux and macOS exact-artifact journeys pass; Windows retains its
      structured unsupported result with zero source attempts.
- [ ] `task check`, `task security`, `task public:check`, and
      `task release:check` pass without weakening a gate.

## Governing documents

- Thesis: bounded ordinary wrappers, deterministic compiled plan, original-output visibility
- Product contract section: tailored CLI surface, execution plan, source-stream result mode
- Architecture or security invariant: one plan application boundary, one source process boundary, external text remains untrusted
- Existing ADR: ADR 0005, ADR 0006, and ADR 0008

## Completion definition

The work is complete when every acceptance criterion has executable evidence,
the source-stream authority and its security exception are promoted to durable
documents and an accepted ADR, the required profiles and native artifact
matrix pass, no temporary or sensitive artifact remains, and this temporary
packet is removed from the final tree.
