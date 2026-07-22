# ADR 0012: Admit the RTK Go test pass optimizer

- Status: Accepted
- Date: 2026-07-22
- Deciders: Repository maintainer and product owner
- Scope: First external output processor, source-catalog selector vocabulary,
  Go runtime successor contract, authoring default, output-result authority,
  processor isolation, provenance, and native compatibility evidence
- Extends: ADR 0007, ADR 0009, ADR 0010, and ADR 0011
- Supersedes: None
- Superseded by: None

## Context

ADR 0007 accepts explicit RTK-backed optimizer defaults only for exact tuples
whose semantics Atsura can validate independently. ADR 0009 rejected the
proposed Git `log` tuple after a valid commit message produced a misleading
successful result. ADR 0011 then established Go CLI as a separate source
runtime and identified no-argument `go test -json` plus RTK's fixed `go-test`
filter as the next candidate without admitting it.

Primary-source review and bounded execution of the official RTK `v0.43.0`
artifacts found both a useful result and unsafe general behavior. A real Go
1.26.5 passing package was reduced to one summary line, but RTK also:

- reports a skip-only stream as `Go test: No tests found`;
- silently ignores malformed JSON lines and unknown actions;
- returns status zero for filtered source failures and cannot represent the
  source status or stderr at its pipe boundary;
- produces nondeterministic multi-package failure ordering through hash-map
  iteration; and
- uses an approximate token-count guard rather than a semantic equivalence
  check when deciding whether its output is never worse.

RTK startup also attempts telemetry unless explicitly disabled and checks
Claude hook state after parsing. An ambient Claude configuration can therefore
produce processor stderr and a hook-warning marker even though coding-agent
hosts are outside Atsura. The processor must run in an isolated environment;
host-specific behavior is neither useful nor permitted at this boundary.

These facts reject RTK as a general Go-test interpreter. They do permit one
strict pass-only tuple when Atsura owns admission before the processor,
requires a strictly smaller expected summary before starting it, and validates
the processor result without relying on RTK's internal fallback.

Primary evidence checked on 2026-07-22:

- [RTK v0.43.0 release](https://github.com/rtk-ai/rtk/releases/tag/v0.43.0),
  commit `5a7880d404db8364d602f2ecdc41dd790f64013f`;
- [official RTK checksums](https://github.com/rtk-ai/rtk/releases/download/v0.43.0/checksums.txt);
- [RTK pipe implementation](https://github.com/rtk-ai/rtk/blob/5a7880d404db8364d602f2ecdc41dd790f64013f/src/cmds/system/pipe_cmd.rs);
- [RTK Go filter](https://github.com/rtk-ai/rtk/blob/5a7880d404db8364d602f2ecdc41dd790f64013f/src/cmds/go/go_cmd.rs);
- [RTK telemetry startup](https://github.com/rtk-ai/rtk/blob/5a7880d404db8364d602f2ecdc41dd790f64013f/src/core/telemetry.rs);
- [RTK hook warning behavior](https://github.com/rtk-ai/rtk/blob/5a7880d404db8364d602f2ecdc41dd790f64013f/src/hooks/hook_check.rs);
- [RTK Apache-2.0 license](https://github.com/rtk-ai/rtk/blob/5a7880d404db8364d602f2ecdc41dd790f64013f/LICENSE); and
- [Go test2json output contract](https://go.dev/cmd/test2json/).

## Decision drivers

- Let a user obtain a materially smaller successful `go test` result without
  trusting RTK to interpret failures, skips, malformed streams, or source
  status.
- Prefer RTK by default only after a maintainer explicitly supplies and reviews
  one exact inspected processor artifact.
- Keep source selection and execution entirely inside Atsura; RTK receives
  bounded stdout through stdin and never calls Go.
- Preserve every conventional result that is ineligible for the optimizer
  before any processor attempt, including exact source stdout, stderr, and
  status.
- Make missing, drifted, failed, or semantically surprising processor behavior
  fail closed without publishing intermediate bytes.
- Keep coding-agent hosts as external callers and prevent ambient host state,
  configuration, credentials, tracking, or telemetry from entering the
  processor boundary.
- Bind support to exact source, processor, platform, version, and artifact
  evidence rather than RTK's advertised command list.

## Decision

### Exact compatibility tuple

Atsura admits one initial optimizer compatibility contract:

```text
source adapter:       atsura.source.go_cli contract 2
catalog evidence:     schema 2 go_test_jsonl selected by exact -json
recorded toolchain:   stable Go 1.26.x inspection observation
user argv:            go test
source argv:          go test -json
source attempts:      at most 1
result mode:          original_preserving_optimizer
optimizer contract:   atsura.output.rtk_go_test_pass.v1
processor:            exact inspected RTK v0.43.0 artifact
platforms:            linux/amd64, linux/arm64, darwin/amd64, darwin/arm64
version probe:        rtk --version
processor argv:       rtk pipe --filter=go-test
processor stdin:      bounded admitted source stdout
processor attempts:   at most 1
shell:                never
```

Source-catalog schema 2 generalizes a structured-output selector from a long
option name to one exact flag token. GitHub CLI continues to use `--json`;
Go contract 2 records `go_test_jsonl` selected by exact single-dash `-json` and
the bounded `TestEvent` field inventory. The caller-visible Go option surface
remains empty, so the user cannot supply `-json`; the reviewed wrapper alone
appends the active cataloged selector. Preview requires exactly one matching
selector before any positional-only marker.

The Go source adapter admits that exact append transform but contains no RTK
identity, filter, result, or isolation policy. A separate finite processor-
compatibility registry owns the namespaced contract and maps it to the fixed
processor invocation. Neither registry consults ambient `PATH`, plugins,
project files, or coding-agent host state.

No package patterns, Go flags, test-binary flags, alternate Go versions,
alternate RTK versions, alternate filters, arbitrary processor argv, or
arbitrary external processors are accepted by this decision.

The recorded Go 1.26.x version remains an inspection-time effective-toolchain
observation under ADR 0011. Runtime binds and revalidates only the direct
launcher file; it does not rerun `go version`, freeze `GOTOOLCHAIN`, `GOROOT`,
module directives, environment, working directory, or a delegated toolchain.
An unchanged launcher may therefore select another runtime toolchain. The
tuple's runtime authority is the direct-launcher identity, exact argv, and
strict event-content admission, not a claim that the selected runtime
toolchain is still Go 1.26.x.

### Explicit observation and authoring default

`processor inspect` accepts one explicit absolute RTK executable path, verifies
that its hash and size match the maintained official artifact for the current
claimed platform, performs the exact `--version` probe once without a shell,
and emits a strict canonical observation containing adapter contract, platform,
absolute path, SHA-256, size, exact version, probe argv, environment-contract
identifier, and attempt count. The probe uses the same isolated RTK environment
as output processing because telemetry and hook-state checks run during RTK
startup even for `--version`. Inspection does not install, download, discover,
or configure RTK.

When `spec init` receives a compatible processor observation alongside the
exact Go catalog and no-argument `test` selection, it materializes the
namespaced optimizer contract into the specification as the default. Without
that explicit observation, the existing identity wrapper remains the draft;
there is no ambient or runtime default. A user may edit the draft back to an
identity choice before compilation.

`bundle build` requires the same compatible processor observation for an
optimizer specification, rejects missing, extra, stale, or incompatible
processor evidence, and binds the exact identity into the bundle. The
specification contains a typed optimizer contract and original-output
allowance, not an executable path, RTK argv, shell fragment, or vendor-host
field. Preview, status, trust summary, adoption, and the plan expose the
processor identity and reason before execution. The trust summary also warns
that an ineligible conventional result exposes the exact transformed
`go test -json` stream, not ordinary human `go test` text; that stream may
contain untrusted controls, prompt-like text, paths, or secrets emitted by the
source test.

The initial optimizer is executable through a rendered POSIX wrapper and its
single shared `wrapper run` application path. Running that rendered wrapper is
also the direct maintainer path and requires no coding-agent host. The
`bundle execute` command retains its existing projection-only structured
evidence envelope and rejects the optimizer before source start; this decision
does not redefine its output or UTF-8 contract.

### Pre-processor result admission

The plan declares that original source output is allowed for this optimizer.
After exact source and processor preflight, Atsura starts the source once. A
conventional source completion is eligible for RTK only when all of these facts
hold:

1. status is zero and stderr is empty;
2. stdout is nonempty valid UTF-8, at most 4 MiB, has no UTF-8 BOM or literal
   carriage return, and consists solely of nonempty LF-terminated records with
   exactly one final LF and no blank record;
3. there are at most 65,536 records, each at most 256 KiB before its LF, and
   each record is one strict non-nested JSON object with no duplicate or
   unknown field;
4. every object uses only the frozen scalar `Time`, `Action`, `Package`, `Test`,
   `Elapsed`, `Output`, and `FailedBuild` field table and its required/optional
   combinations; strings are bounded by their record and aggregate limits;
5. every action is one of the frozen admitted `start`, `run`, `output`, `pause`,
   `cont`, or `pass` actions; no `fail`, `skip`, `bench`, build action, or
   unknown action exists;
6. the stream contains exactly one nonempty package identity, begins with its
   sole package-level `start`, contains no event after its sole final package-
   level `pass`, and contains no `Test` on either package terminal;
7. every nonempty test identity begins with exactly one `run`; `output` is
   allowed only after `run` and before its terminal; `pause` changes running to
   paused; `cont` changes paused to running; and exactly one test-level `pass`
   ends the lifecycle with no later event;
8. package-level `output` occurs only after package start and before package
   pass, every event repeats the exact package identity, at least one test-level
   pass exists, and no lifecycle, identity, or terminal fact conflicts; and
9. the exact newline-free summary independently calculated from these typed
   facts is strictly shorter in bytes than the admitted source stdout.

An otherwise conventional result that fails any eligibility condition is not a
processor fault. Atsura starts zero processor processes and returns the exact
transformed source stdout, stderr, and status with disposition
`preserved_before_processor`. This includes ordinary test failure, build
failure, skip-only output, malformed or unknown JSON, empty output, source
stderr, nonzero source status, or a valid pass stream whose summary would not
be shorter. These bytes are raw `go test -json` output and may expose all
source-emitted content. This is plan-authorized preservation before processor
start, not fallback after a processor failure.

An uncertain source-process outcome, timeout, signal, cancellation, identity
drift, or stream overflow remains a non-retryable source fault under ADR 0010;
captured bytes are not published and no processor starts.

### Processor execution and postcondition

For eligible input only, Atsura revalidates the processor identity and starts
the exact processor once. It passes only admitted stdout on stdin. Both
inspection and processing use environment contract
`atsura.processor.rtk_isolated.v1`: a fresh private root; created empty working,
user-state, temporary, XDG config/data/cache, and platform application-data
directories; `RTK_TELEMETRY_DISABLED=1`, `RTK_TEE=0`, `RTK_NO_TOML=1`, and an
`RTK_DB_PATH` inside that root; and a `CLAUDE_CONFIG_DIR` pointing to a child
that is deliberately not created. Windows variables are not part of the
initial runtime matrix. Apart from the finite locale/timezone and OS-required
variables recorded by the implementation, ambient environment and credentials
are not inherited. The executable path is absolute and no shell or `PATH`
lookup is used.

Successful processor completion requires status zero, empty stderr, bounded
stdout, unchanged post-run processor identity, and one of exactly two stdout
values:

- the byte-identical admitted input, reported as
  `preserved_after_processor`; or
- the exact newline-free summary independently computed from the admitted
  typed events, strictly shorter than the input, reported as `optimized`:

```text
Go test: <test-pass-count> passed in 1 packages
```

The optimized summary deliberately omits passing test names, output, elapsed
time, event ordering, and package identity. That
loss is the reviewed meaning of this optimizer, not evidence that all source
information was preserved. Atsura does not infer RTK's internal parsing,
panic, recovery, or never-worse branch from either disposition.

A processor start failure, timeout, signal, cancellation, nonzero status,
nonempty stderr, output overflow, identity drift, or any other stdout is a
non-retryable processor fault. Because the source has already run, Atsura does
not automatically fall back to its input and publishes no source stdout,
source stderr, partial processor output, or processor stderr. Attempt counts
remain explicit: preflight failure is source zero/processor zero; eligible
post-source identity failure is source one/processor zero; processor failure is
source one/processor one.

### Distribution and host boundary

RTK remains a user-supplied external executable. Atsura release archives do not
bundle or install it. `.harness/processors.json` is the strict versioned
dependency/provenance manifest for this contract. It records the exact official
release and checksums URLs and digests, extracted binary identity, four claimed
platform tuples, version, upstream commit, Apache-2.0 license URL/digest,
upstream NOTICE absence, distribution status, and the SBOM review outcome
`not_provided_external_dependency`. The manifest is evidence and an input to a
future SBOM; it is not represented as a standards-compliant SBOM. This
explicitly refines ADR 0007's SBOM-evidence requirement for a separable,
non-redistributed executable. Any future bundling requires a separate license,
notice, dependency, standards-compliant SBOM, integrity, and update decision.

The optimizer runtime matrix is exactly Linux amd64, Linux arm64, Darwin amd64,
and Darwin arm64. Windows remains a supported base Atsura build target but has
no rendered wrapper and no optimizer compatibility claim in this decision,
even though upstream publishes an RTK Windows artifact.

No Claude, Codex, or other coding-agent adapter is added. Such hosts may invoke
an already rendered wrapper, but the core, processor adapter, bundle, plan, and
runtime never inspect a host process, rewrite a host request, edit host
settings, or select behavior from a host vendor.

## Alternatives considered

### Pass every Go JSON stream to RTK

Rejected. Skip-only, malformed, unknown, failure, and multi-package failure
fixtures demonstrate successful but incomplete, misleading, or nondeterministic
results. RTK's status and never-worse guard cannot repair source semantics.

### Fall back to source bytes after a processor fault

Rejected. Once an eligible source result has entered a processor, a processor
fault is an uncertain post-source outcome. Publishing the input as if the
processor had never failed would conceal the adopted stage failure and make
runtime behavior depend on an unreviewed fallback. Preservation must be chosen
before processor start or returned as a valid postcondition.

### Reimplement the Go summary inside Atsura

Rejected for this first integration. Atsura necessarily computes the exact
expected summary to validate RTK, but the user-visible optimized bytes still
come from the inspected RTK artifact. This tests the external-processor
boundary and avoids presenting an unreviewed internal formatter as a broader
RTK replacement.

### Discover or download RTK automatically

Rejected. Ambient discovery and installation would move dependency choice,
network I/O, identity, and update behavior outside the reviewed specification
and bundle. Explicit inspection is a small, attributable handoff.

## Consequences

### Positive

- The frozen passing no-argument Go fixture gains the first RTK-scale output
  reduction while failed, skipped, malformed, non-beneficial, and otherwise
  ineligible results retain their exact transformed-source meaning.
- The original-preserving optimizer becomes an executable, reviewable result
  contract rather than a projection fallback.
- The first external processor proves a finite adapter boundary without giving
  it source-execution or host-integration authority.
- RTK's useful filter work is reused without accepting its advertised support
  list or internal best-effort behavior as compatibility proof.

### Negative

- A passing Go test can involve two local processes and duplicate semantic
  validation.
- Users must install RTK separately and supply its exact path for inspection
  and bundle compilation.
- The first contract supports only one recorded Go and exact RTK version
  family, one command, one package, one filter, and four POSIX targets.
- Every claimed optimizer platform needs pinned artifact and isolation evidence
  for this exact tuple; Windows has no optimizer wrapper contract.

### Risks and mitigations

- RTK may change at the same version or the executable may be replaced. Bundle
  and plan bind path, size, hash, version, and platform; runtime validates
  identity before source, before processor start, and after processor exit.
- RTK may access ambient host, configuration, credentials, or networks.
  Execution receives a minimal isolated environment. Absence of child-process,
  filesystem, or network activity is not inferred from that environment:
  installed evidence may make such a claim only after a platform-specific
  external observer contract records it. No such observer contract is accepted
  by this decision yet, and no OS sandbox claim is made.
- Go event grammar or runtime toolchain selection may evolve after inspection.
  Strict recorded-version admission plus field, action, lifecycle, identity,
  and exact-output checks make new behavior ineligible or fail closed without
  claiming runtime toolchain closure.
- Writing a completed result may fail after source effects. Delivery failures
  remain non-retryable and never imply replay safety.

## Mechanical enforcement

- Domain schema truth tables distinguish projection, source-stream,
  `preserved_before_processor`, `preserved_after_processor`, and `optimized`
  results and reject every ambiguous union.
- Source-catalog schema 2 tests admit exact single- and double-dash structured-
  output selector flags, keep them separate from the caller option surface, and
  require exactly one planned selector before source start.
- One finite processor registry is the only namespaced source/processor tuple
  authority; source adapters contain no RTK policy and application code imports
  no RTK infrastructure package.
- Strict processor-observation codecs reject unknown fields, duplicates,
  trailing values, path aliases, incompatible versions, and unused evidence.
- Go lifecycle fixtures freeze the exact LF/record/field/action/state table and
  cover BOM, CR, blank/unterminated/oversized records, pass, output,
  pause/continue, skip, failure, build events, malformed JSON,
  duplicate/unknown fields, unknown actions, empty streams, conflicting
  identities, summary-not-smaller, and terminal-order violations.
- Application truth tables prove source/processor attempt counts, exact
  pre-processor preservation, valid optimized/preserved postconditions,
  no-byte processor faults, cancellation, final-write behavior, and no retry
  claim after source start.
- Infrastructure tests bind exact executable identity, argv, stdin, isolated
  cwd/environment, byte/time limits, no shell, pre/post identity checks, and
  cleanup behavior.
- Catalog, help, trust-summary, capability-ledger, schema-inventory, and
  compatibility tests make processor inspection and the default discoverable
  without exposing arbitrary processor execution.
- Installed-artifact journeys replay the four claimed official RTK `v0.43.0`
  native artifacts and verify archive and binary identities. They exercise only
  deterministic cases reachable through the fixed admitted invocation:
  `optimized`; `preserved_before_processor` for skip, failure, and other
  ineligible source results; projection-facade rejection before source start;
  processor preflight drift; eligible post-source processor drift; and Windows
  optimizer non-support before source start.
- `preserved_after_processor` and the one-processor-attempt failure/no-byte
  branches remain mandatory application and infrastructure truth-table tests.
  The exact admitted RTK artifact and fixed argv have no deterministic fixture
  that produces those branches, so installed-artifact journeys do not claim to
  exercise them. This evidence boundary does not weaken the runtime
  postcondition, no-fallback rule, no-byte fault rule, attempt accounting, or
  processor identity checks.
- Native child-process, filesystem, or network claims require an explicit
  external observer contract. Until one is implemented and validated for a
  platform, installed evidence records no such absence claim.
- Shared schemas and architecture lint reject RTK-specific fields outside the
  finite processor adapter/manifest and reject coding-agent host fields
  everywhere in the core.
- Completion requires `task check`, `task security`, `task public:check`,
  `task release:check`, and the claimed native CI matrix on one revision.

## Compatibility and migration

The source catalog, specification, bundle, plan, agent-help, capability-ledger,
and artifact-evidence schemas receive explicit version changes; Go inspection
contract 2 replaces contract 1 for new catalogs. Older schemas remain rejected
rather than guessed. Existing identity and strict-projection bundles retain
their semantics after regeneration. No persisted credential, host
configuration, or RTK state is migrated. Rolling back means regenerating and
adopting an identity specification; it never requires uninstalling or mutating
RTK.

## Security and public-boundary impact

The new asset is a user-selected local RTK executable and its identity
observation. The new effect is one optional post-source local process for an
eligible result. No authentication or credential is required by Atsura; the
processor environment excludes ambient secrets. Processor input may contain
untrusted repository test output and is neither logged nor persisted by
Atsura. A pre-processor preserved transformed stream may still expose that
untrusted source content because the reviewed plan explicitly allows it.
Processor stderr and failed intermediate bytes are suppressed. Native
evidence uses synthetic public fixtures and official release artifacts.

RTK is not redistributed, so its license text is not added as an archive member
by this decision. The pinned dependency/provenance manifest records
Apache-2.0, official source/release locations, NOTICE absence, and the explicit
external-dependency SBOM review status. Any redistribution or vendoring
requires a successor review.

## Validation

- Focused domain, application, infrastructure, CLI, hostile-output, and schema
  tests described above.
- Same-fixture presentation evidence showing the exact pass-only semantic loss
  and byte comparison, with ineligible fixtures excluded rather than scored as
  successful optimizations. Token counts are not asserted without an accepted
  vendor-neutral tokenizer contract.
- Installed `atr` wrapper journeys with exact official RTK artifacts on Linux
  amd64/arm64 and Darwin amd64/arm64 cover only the deterministic reachable
  cases listed under mechanical enforcement; the Windows journey proves the
  optimizer remains unavailable before source start. Controlled application
  and infrastructure truth tables separately prove
  `preserved_after_processor` and processor one-attempt failure/no-byte
  behavior.
- `task check`, `task security`, `task public:check`, and
  `task release:check`.

## Reconsideration signals

Create a successor ADR before admitting another source command, Go version,
package count, RTK version/filter, processor implementation, arbitrary
processor, automatic install/discovery, processor-failure fallback, relaxed Go
event grammar, host-derived behavior, or RTK redistribution. A new RTK release,
artifact replacement, changed telemetry/config behavior, changed Go output
schema, or native evidence mismatch invalidates the tuple until requalified.
