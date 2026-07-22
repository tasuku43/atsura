# Work Goal: Optimize passing Go test output with inspected RTK

- Status: Active
- Retention: temporary
- Retention reason: None
- Governing contract: ADR 0012
- Review/delete trigger: Delete after durable conclusions are promoted and the change completes
- Successor: None
- Owner: Repository maintainer and implementation agent
- Target: First release-quality external output processor iteration
- Related ADRs: ADR 0007, ADR 0009, ADR 0010, ADR 0011, ADR 0012

## Outcome

A maintainer can explicitly inspect an RTK v0.43.0 executable, generate and
review a Go `test` specification whose default is the exact maintained
optimizer tuple, build and adopt a processor-bound bundle, and run a rendered
host-neutral POSIX wrapper directly or from any caller. The frozen strict
pass-only fixture is reduced to the independently validated RTK summary; every
conventional ineligible result is returned exactly before RTK starts; every
uncertain source or processor outcome fails closed with truthful attempt counts
and no intermediate-byte leak.

## Why now

The second source runtime is complete and native evidence proves no-argument Go
test wrappers. RTK v0.43.0 research now provides enough primary and hostile
evidence to accept one narrow pass-only tuple while rejecting unsafe skip,
failure, malformed, and nondeterministic behavior. This is the largest current
gap between the accepted optimizer thesis and executable product behavior.

Because this changes default agent-facing presentation, this packet includes
[presentation evidence](presentation-evidence.md).

## Non-goals

- Additional Go flags, packages, test-binary arguments, toolchain versions,
  RTK versions, RTK filters, source CLIs, or optimizer tuples
- Arbitrary processors, RTK argv, shell, jq, scripts, plugins, runtime LLMs, or
  automatic PATH discovery/download/install/update
- Delegating source execution, permission decisions, or retry policy to RTK
- Claude, Codex, or other coding-agent process inspection, settings mutation,
  hook input rewriting, or vendor-specific core behavior
- RTK redistribution in Atsura release archives
- General proof that RTK preserves Go test semantics or all source information
- Optimizer execution through the projection-only `bundle execute` envelope
- Runtime proof that the inspection-time Go 1.26.x observation remains the
  toolchain selected later by an unchanged launcher
- Release, tag, or publication creation

## Acceptance criteria

- [ ] `atr processor inspect --adapter rtk --executable <absolute-path>` performs
      exactly one no-shell `--version` probe and emits a strict identity-bound
      observation only for the maintained RTK v0.43.0 platform artifact.
- [ ] Source-catalog schema 2 and Go inspection contract 2 record the exact
      `go_test_jsonl` single-dash `-json` selector separately from the empty
      caller option surface; preview requires that one active planned selector.
- [ ] A recorded Go 1.26.x `test` catalog plus an explicit compatible processor
      observation materializes the maintained optimizer default; absent or
      incompatible evidence never inserts RTK implicitly, and an identity
      choice remains reviewable.
- [ ] Specification, bundle, plan, trust/status output, and agent help expose one
      unambiguous `atsura.output.rtk_go_test_pass.v1` optimizer contract, exact
      processor identity, original-output allowance, fixed bounds, reason, and
      schema versions without arbitrary executable/argv or coding-agent-host
      fields in core schemas.
- [ ] The frozen pass fixture executed through a rendered host-neutral POSIX
      wrapper produces the exact, strictly smaller, newline-free
      `Go test: N passed in 1 packages` only after strict Go JSON admission and
      reports one source/one processor attempt internally; `bundle execute`
      rejects this result mode before source start.
- [ ] Skip, fail, build failure, malformed/unknown JSON, empty output, source
      stderr, conventional source nonzero, and valid-but-not-smaller pass output
      complete as exact transformed `go test -json` stdout/stderr/status with
      `preserved_before_processor` and one source/zero processor attempts.
- [ ] Valid processor output distinguishes `preserved_after_processor` from
      `optimized`; the former is byte-identical to admitted input, while the
      latter equals the independent summary and is strictly shorter. This is a
      required controlled application/infrastructure truth table, not an
      official-artifact case for the fixed RTK invocation.
- [ ] Missing/drifted processor preflight starts neither process; eligible
      post-source processor drift starts one source/zero processor; processor
      start, timeout, signal, cancellation, nonzero, stderr, overflow, identity
      drift, or unexpected stdout starts one source/one processor and exposes no
      source or processor bytes. All are non-retryable once source may have run.
      The one-processor-attempt branches are required controlled
      application/infrastructure tests and are not attributed to an unreachable
      official-RTK fixture.
- [ ] Inspection and processor execution use fixed argv/stdin, no shell or PATH
      lookup, and exact environment contract `atsura.processor.rtk_isolated.v1`,
      including disabled telemetry/tee/TOML, isolated state/temp/data roots, and
      a `CLAUDE_CONFIG_DIR` child that is deliberately not created.
- [ ] Official RTK v0.43.0 archive checksum, extracted binary hash/size, version,
      platform tuple, commit, release URL, and Apache-2.0 provenance are pinned;
      RTK is not present in Atsura release archives.
- [ ] Installed-artifact evidence verifies exact official RTK identity and
      provenance, then replays only deterministic reachable cases on every
      claimed optimizer platform (Linux amd64/arm64 and Darwin amd64/arm64):
      optimized; `preserved_before_processor` for skip, failure, and other
      ineligible results; projection-facade rejection; preflight drift; and
      eligible post-source drift. It proves Windows does not claim the optimizer
      and compares semantic results rather than aggregate file existence.
- [ ] Installed evidence makes no child-process, filesystem, or network absence
      claim unless a platform-specific external observer contract has first
      been implemented and validated. Isolation remains mandatory even when
      those observations are not yet available.
- [x] Presentation evidence uses one typed pass fixture and answer key, records
      the deliberate information loss and exact 1,273-to-31 byte comparison and
      hashes, excludes semantically ineligible cases, and makes no token claim
      without an accepted vendor-neutral tokenizer contract.
- [ ] Capability ledger and durable product, architecture, security, harness,
      release, and public-boundary documents describe only the implemented
      finite contract and retain RTK/host unknowns outside it.
- [ ] `.harness/processors.json` pins the four official artifacts, dependency
      and Apache-2.0 provenance, NOTICE absence, external distribution status,
      and explicit non-SBOM review outcome; RTK remains outside release archives.
- [ ] Focused tests and `task check`, `task security`, `task public:check`, and
      `task release:check` pass on the same revision; required native CI is
      green; repository status and pre-existing changes are understood.

## Governing documents

- Thesis: `docs/00_theses.md`, especially output-stage authority, explicit RTK
  defaults, deterministic runtime, and vendor-host independence
- Product contract section: original-preserving output optimizer, compilation,
  plan, execution, trust summary, and deferred surface
- Architecture or security invariant: four-layer dependency direction, one
  controlled source/processor boundary, exact identity, fail-closed output, and
  isolated secret-free external execution
- Existing ADR: ADR 0007, ADR 0009, ADR 0010, ADR 0011; accepted successor ADR
  0012

## Completion definition

The work is complete only when every acceptance criterion has repository or CI
evidence, durable decisions are promoted, all required gates pass on one
revision, no temporary RTK binary or sensitive observation remains, and this
temporary packet is removed from the final tree.
