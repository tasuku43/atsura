# Security Model

Atsura narrows and transforms the CLI surface presented to a coding agent. It
does not authorize source CLI operations and does not provide an operating
system sandbox. Security therefore depends on honest boundaries: strict
tailoring inputs, exact artifact adoption, identity-bound no-shell process
execution, controlled Atsura-owned mutations, bounded parsing, and explicit
failure when the core cannot evaluate its own contracts.
ADR 0016 adds one finite value-option-default transformation without adding a
process, shell, secret, host, or vendor trust boundary. ADR 0017 adds one
private user-local managed executable-shim boundary with exact ownership,
non-replacing publication, read-only reconciliation, and exact removal; it
does not make caller activation or the source operation trusted.

## Security objectives

Atsura must:

- never treat a command, argument, help response, catalog, specification,
  bundle, wrapper binding, repository file, caller environment, or source
  output as trusted merely because it exists;
- compile only complete, validated surface and wrapper definitions;
- start no source process when surface resolution or wrapper construction
  fails;
- never execute arbitrary shell, scripts, jq programs, plugins, or external
  transformers from the initial specification;
- bind adoption and execution to exact bundle and source identity;
- keep credentials and raw sensitive source output out of persistent state;
- preserve source invocation meaning when output optimization cannot be
  applied safely; and
- keep source execution separate from Atsura-owned mutation authority.

## Non-objectives

Atsura does not:

- determine whether a user or agent is authorized for a source operation;
- infer source safety or remote effects from command names or help prose;
- replace source authentication, authorization, prompts, operating-system
  controls, or provider-side policy;
- make an excluded command impossible to invoke through another binary, shell,
  or command-resolution path;
- turn an adopted bundle into a permission grant; or
- interpret or preserve a coding-agent host's permission, trust, or sandbox
  decisions.

## Trust boundaries

### Source executable and catalog evidence

The source executable is untrusted local code. Inspection may start it with
adapter-owned fixed argv, so inspection is an `EffectExecute` operation and
must use an exact identity-bound, no-shell, bounded process port. Help text,
option names, versions, extensions, and structured-output claims remain
untrusted evidence until validated by the named adapter contract.

Catalog membership proves observed structure, not permission or safety. Only
verified built-ins are eligible for the currently managed compiled surface;
observed extensions and unverified dynamic entries remain evidence.

The two current inspection contracts are separately bounded. `github-cli`
contract 2 performs four fixed offline probes. `go-cli` contract 2 performs
exactly `go version`, `go help`, and `go help test`, admits only stable Go
1.26.x effective-toolchain evidence observed under the inspection working
directory and environment, and stores no raw help or environment value. Neither probe set
makes the inspected executable, its help, or a later source invocation trusted.

For Go, path/hash/size identify the direct launcher file, while
`Source.Version` is the possibly delegated effective toolchain observed by
`go version`. Runtime revalidates the launcher identity but does not repeat the
version observation. The same launcher may later select or download another
toolchain because of module state, working directory, `GOTOOLCHAIN`, `GOROOT`,
or related ambient inputs; contract 2 neither detects nor pre-start rejects that
change and does not identify the selected toolchain or GOROOT tree.

### Tailoring specification

Repository-provided and user-provided specifications are untrusted input. The
schema is bounded, versioned, strict, and closed over a finite typed action
vocabulary. Unknown fields, aliases, duplicate entries, ambiguous option
overrides, catalog mismatches, invalid wrapper combinations, excessive input,
and retired authorization schemas fail before compilation or process start.

Arbitrary shell and arbitrary executable code are not valid specification
actions. Arguments remain argv elements and never become a shell program.
Each `invoke.option_defaults` entry must name one exact included cataloged long
option that takes a value, avoid structured-output selectors and active
`append_args` options, and carry one non-empty structurally safe UTF-8
value whose full canonical `--option=value` argv element is no larger than
`sourceprocess.MaxArgumentBytes` (4096 bytes). The ordered list is public
artifact data propagated to the bundle, plan, exact-command help, and release
evidence; it must never hold credentials or other secrets.

### Compiled bundle and adoption receipt

A bundle is self-consistent evidence, not trusted merely because its hashes
validate. Exact-digest adoption is a user-local decision to use the compiled
surface and wrapper set. It is not authorization for downstream source
operations.

Adoption summaries describe material surface and wrapper facts: included and
excluded entries, option changes, identity and transforming wrappers, argv and
output transformations, source-stream visibility, source identity, exact
processor bindings, original-output visibility, and bundle digest. When any
included wrapper may return unprojected source streams or pre-processor
preserved bytes, the
controlling terminal warns that those bytes may contain secrets, controls,
malformed text, or prompt-like content. No source byte enters the receipt. The
summary does not count source permissions or inferred effects.

Changed source identity, catalog, specification, surface, wrapper, or bundle
content never inherits adoption. Repository-controlled paths cannot select or
replace receipts silently. Trust-store create/write operations remain
Atsura-owned mutations and cross the central mutation invoker.

The trust-store adapter rejects symbolic links and non-directory/non-regular
shapes, confines creation and replacement to one verified directory, and
revalidates directory identity before replacement. On Unix it additionally
requires owner-only directory and file modes. Windows `FileMode` permission
bits do not represent the directory or file ACL, so the portable adapter makes
no ACL-ownership claim there; it retains the shape and confinement checks and
relies on the user configuration directory's inherited operating-system ACL.
A stronger Windows ACL guarantee requires a separate platform-specific policy
and implementation.

### Wrapper bindings and external activation

A generated wrapper and its binding are untrusted until the exact bundle,
adoption, wrapper contract, runtime identity, command spelling, and source
identity validate. A path or familiar command name is only a locator.
Repository content, ambient `PATH`, a shell environment, or a coding-agent host
cannot create adoption, select another bundle, or replace the physical source
authority.

The current wrapper accepts argv, not a shell command string or agent-host
payload. On Linux and macOS, `wrapper render` emits one fixed POSIX function and
an optional schema-2 review envelope. Contract 3 compiles bounded root,
namespace, and exact-command final-`--help` views from the bundle's included
surface. Those fixed branches print only constant-format single-quoted data,
name the exact bundle digest, and start no bound `atr`, source, or processor.
Exact-command help discloses configured option defaults; root and namespace
help remain bounded indexes.
The function body is a subshell: alias-safe POSIX special-builtin cleanup
removes inherited `command` and `return` functions only in that isolated body
and fails before runtime start if cleanup is unavailable. Escaped builtin
lookup then bypasses caller-defined `test` and `printf` functions without
modifying the caller's shell.
The generated preamble removes an exact same-name alias before defining the
wrapper; otherwise alias expansion can rename or bypass a function. This is an
explicit in-memory effect of sourcing, not a persisted activation edit. The
preamble assumes `unalias` has its standard POSIX meaning; a caller-defined
`unalias` function is outside the supported activation environment and cannot
be bypassed and preserved portably at top-level parse time.
Every non-help argv list is forwarded losslessly to `wrapper run`; the
specification cannot inject shell source, and the template uses neither
`eval` nor `sh -c`. The source command spelling is derived verbatim from the
bundle's requested executable, must be a portable POSIX Name outside the
maintained reserved/fixed and implementation-specific function-name set, and
is never derived from a path or basename. `wrapper run` invokes only the exact
physical source path already bound into the adopted bundle, so ambient command
resolution cannot recurse into the wrapper. Windows returns a structured
unsupported fault for POSIX rendering and receives no POSIX activation claim.

Once the bound `atr` has started, honest `wrapper run` code revalidates the
wrapper contract, its own path/hash/size, exact expected bundle digest, current
adoption, source identity, and bundle-derived command spelling and builds a
fresh plan. Missing, malformed, drifted, or mismatched state starts zero source
processes. The wrapper never selects a different bundle or raw execution as
fallback. Because the shell must start the bound runtime path before that code
can fingerprint itself, this check detects cooperative drift; it is not
attestation, a sandbox, or protection against malicious code that replaced the
`atr` executable at that path.

Static tailored help describes the exact reviewed wrapper artifact rather than
current executability. It does not revalidate later adoption, source,
processor, or runtime state and therefore must not claim readiness,
authorization, containment, or attestation. Non-help execution retains every
existing current-state check. An excluded or unknown help-shaped selector has
no compiled branch and reaches existing fail-closed resolution without source
help or a source attempt. POSIX may implement the formatting utility outside
the shell process, so this is not a generic zero-subprocess guarantee.

For generated shell material, the rendered-byte digest proves deterministic
generation and lets reviewers and release fixtures compare exact output. Once
a caller sources or otherwise changes that function, Atsura cannot attest its
in-memory bytes; activation integrity remains caller-owned. ADR 0017's
persisted executable shim adds private-store ownership and material drift
checks before an honest managed shim can
reach the bound runtime. It still cannot attest code that a same-user attacker
has replaced and already caused the operating system to execute.

`wrapper render` still writes only to stdout and does not persist or install
the function. `wrapper install`, `wrapper status`, and `wrapper remove` form a
separate Atsura-owned executable-shim lifecycle on Linux/macOS amd64/arm64.
The lifecycle uses only one fixed private platform-configuration-root store,
canonical immutable manifests, fixed executable material, effective-user and
mode checks, create-exclusive staging, non-replacing atomic publication,
bounded enumeration, exact hard-link identity, and read-only reconciliation.
It never edits caller-owned `PATH`, shell startup, coding-agent settings,
hooks, trust, or permission rules.

The operating-system user is the local store's security principal. Owner-only
modes, effective-UID checks, link counts, pinned directory identities, and the
store lock protect against accidental drift, foreign shapes, and conflicting
cooperating Atsura processes. They do not isolate mutually hostile processes
running as the same user: such a process can modify that user's files or
ignore Atsura's advisory lock. Atsura detects the bounded passive drift it can
observe and refuses unknown state; it does not claim containment against a
deliberate same-UID racer.

Publication never falls back to a rename that can replace an entry created
after an absence check. Supported Linux targets require
`renameat2(RENAME_NOREPLACE)` and supported Darwin targets require
`renameatx_np(RENAME_EXCL)` between pinned directory descriptors. If the
kernel or filesystem cannot provide the exclusive primitive, installation
fails without publishing a weaker claim. The first lifecycle has no replace
operation.

Crash residue is not ownership evidence. Status validates strictly shaped,
bounded staging residue but performs no cleanup. A later mutation may remove
only the exact nonrecursive files and directory whose identities it inspected;
unknown names, nested content, hard links, replacements, or over-capacity
state are preserved and rejected. Active removal first deletes and syncs the
exact bin hard link, so a crash can leave a valid inactive record. It then
exclusively quarantines and syncs the record before bounded cleanup, so a later
crash leaves no public record and only validated residue. Any error after a
mutation may have begun is non-retryable and points only to read-only status.

External activation is outside Atsura's security claim. A shell, container,
coding-agent hook, or other launcher may expose the wrapper, but Atsura neither
inspects nor attests that setup. If activation is absent or bypassed, the source
command may resolve elsewhere; this is not an Atsura fail-closed guarantee.
Surface composition remains a capability boundary rather than a sandbox.

Production Atsura contains no coding-agent-host protocol, hook payload,
permission mapping, settings lifecycle, host process client, session, or
transcript handling. Vendor-specific activation and conformance remain
downstream responsibilities, not product input, persisted product state, test
fixture authority, or a public compatibility schema.

## Surface composition is not isolation

The tailored surface is a capability and usability boundary for the agent. If
a command is absent, resolution returns `command_not_in_surface` and constructs
no wrapper plan. That result means only that Atsura's tailored surface does not
provide the command.

Hiding a command or option is not a sandbox, ACL, or operating-system deny.
Atsura documentation, faults, trust summaries, and caller fixtures must not
imply otherwise. Users who require containment must apply source, credential,
provider, host, and operating-system controls independently.

## Wrapper integrity

Surface membership and wrapper behavior are validated independently:

- an excluded command has no option surface and no wrapper;
- an included command has an explicit option surface and complete wrapper;
- an identity wrapper has no transformation;
- a transforming wrapper contains at least one supported typed transform; and
- unsupported before/after actions are invalid rather than ignored.

`bundle preview` binds the adopted bundle, bundle/catalog/specification
digests, exact source and adapter identity, matched command, original and
transformed argv, the complete declared option-default list and exact applied
subset, ordered stages, finite process bounds, and the exact applied
specification entry or `null` for inheritance. It does not carry a universal
permission decision. Preview revalidates current source path, SHA-256, and size,
returns a canonical plan digest, and reports `source_process_attempts: 0`.
Runtime revalidates again and rebuilds the plan rather than using an old
preview as authority.

Caller precedence is exact and no-shell: inline, separated, explicit-empty,
and repeated occurrences of the same long option before the first `--`
suppress its default. A short alias does not, and the same spelling after `--`
is positional data. Missing defaults are inserted as `--option=value` in
declaration order immediately after the matched command path; the caller tail
then remains byte-for-byte ordered and `append_args` remain last. Detached plan
validation recomputes both the applied subset and transformed argv so tampered
plans fail before process start.

`wrapper render` additionally requires the complete included surface to be a
non-empty same-source set whose every command, exposed option, wrapper, and
result mode is admitted by the registry-selected source contract before
exposing ordinary-command material. GitHub CLI contract 2 may admit one or both
maintained commands with different existing result modes; Go CLI contract 2
remains the exact singleton `test` surface. One invalid entry rejects the whole
render rather than being hidden or deferred. Its binding closes the exact
bundle digest with the current `atr` path/hash/size, requested command spelling,
and a rederived bounded help projection. `wrapper run` accepts only
that complete closure plus argv after the explicit `--` separator, derives the
source spelling from the loaded bundle, and rebuilds the same plan. Success is
one compact plan-declared JSON object or array plus LF, an explicitly adopted
bounded source-stream result, or one exact original-preserving optimizer
disposition. None has a maintainer evidence envelope, and raw-byte modes are
not raw execution because every tailored check and argv transformation still
applies.

One finite application compatibility registry serves both the fresh-plan and
complete-surface checks. It dispatches only by the exact namespaced adapter kind
already present in validated evidence. Missing, unknown, duplicate, nil, or
misconfigured verifiers fail as `adapter_contract`; the registry never probes,
starts a source, selects another adapter, or falls back to a weaker result mode.
Whole-surface iteration is pure validation over the already loaded canonical
bundle. It adds no source or processor I/O, no ambient discovery, and no coding-
agent-host boundary. Mixed result modes remain command-local plan facts and
cannot authorize fallback or cross-command output substitution.

## Source process execution

Starting the source is `operation.EffectExecute`: a source-owned process may
perform downstream work whose semantics Atsura does not classify. It is not an
Atsura read and carries no Atsura mutation target or impact.

The process boundary must:

- bind an exact resolved regular executable identity and argv vector;
- avoid a shell and ambient command reconstruction;
- declare finite attempts, time, stdout, and stderr limits;
- revalidate identity immediately before execution and assess it after the
  attempt where supported;
- start zero attempts on invalid surface, wrapper, adoption, drift, or identity
  state; and
- report every unknown post-start outcome as non-retryable rather than imply
  that replay is safe.

The source CLI remains responsible for credential prompts, source-specific
confirmation, authorization, destinations, and downstream effects.

`bundle execute` is the first bundle-backed source boundary. It supports only
adapter-admitted JSON transform wrappers, derives the request from the fresh
plan, and compares expected path/hash/size before start, immediately before
start, and after wait. Compatibility admission covers maintained command and
argv behavior; it does not trust stdout, which still passes through the strict
parser and typed transformer. `bundle preview` remains read-only. Inspection
probes remain separately bounded source execution.

Go CLI contract 2 adds exact no-argument `go test` through either an identity
wrapper and `source_stream_passthrough` or the exact append-`-json` transforming
wrapper and `original_preserving_optimizer`. Every caller-supplied option,
package pattern, `--` marker, or test-binary argument fails before start. An
admitted Go test remains
source-owned `EffectExecute`: it may compile and run untrusted repository code,
read credentials or configuration, resolve modules, access networks, and
mutate caller-owned files or caches. Atsura does not classify, authorize, or
sandbox those effects.

`wrapper run` is a second public façade over that same source boundary, not a
second executor. Runtime/bundle closure validation, adoption, source identity,
surface and option resolution, fresh planning, compatibility admission, and
process bounds all complete before an honest runtime starts the source. It
forwards separate argv and returns the plan-authoritative result variant.

## Atsura-owned mutations

Only Atsura state changes use the create/write mutation contract. Examples are
adoption receipt changes and managed wrapper-artifact installation or removal.
Before infrastructure acts, these operations require explicit
intent, exact target binding, complete
impact, and the central mutation invoker. Structured known outcomes survive
cancellation; unclassified results become non-retryable uncertain outcomes
with a read-only reconciliation action.

Those controls must never be used to claim knowledge about the downstream
effect of a source CLI command.

## Output projection and optimization

Source output is untrusted and may contain control characters, prompt-like
text, malformed encodings, secrets, or very large structures. Typed parsers and
transformers use explicit format, depth, node, record, field, and byte bounds;
reject duplicate keys where semantics would be ambiguous; and preserve visible
projection rules at the CLI boundary.

Visible projection governs output that Atsura interprets or presents as its own
terminal, TSV, or JSON structure. A plan-declared
`source_stream_passthrough` result is a deliberate adopted exception: after a
conventionally completed identity-bound invocation, Atsura may return bounded
source stdout and stderr bytes without projection, framing, UTF-8, terminal-
safety, prompt-safety, or semantic-safety claims. This mode never bypasses
surface resolution, invocation transformation, source identity, compatibility
admission, or fresh-plan validation.

The source-stream path buffers each stream independently within the existing
4 MiB and 256 KiB limits. A conventional nonzero status and successful nonempty
stderr are source results, not Atsura faults. Signal or abnormal termination,
timeout, cancellation, overflow, wait uncertainty, identity uncertainty, or
inconsistent process evidence suppresses both captured streams. The CLI writes
complete stdout once and complete stderr once, then returns the source status;
it does not preserve timing or cross-stream interleaving. Those two writes are
not atomic. A short or failed final write may leave partial caller-visible
bytes, returns non-retryable `execute_output_write_failed`, does not return the
source status, and never recommends replay.

Atsura never persists source-stream bytes or copies them into faults, trust
records, evidence documents, logs, transcripts, or structured diagnostics.
Process uncertainty suppresses captured bytes before delivery; a delivery
failure cannot retract bytes already written.

A typed projection may receive source output only behind its declared parser and
must fail closed without exposing its input when it cannot produce the adopted
shape. It must not change argv, retry the source, select raw mode, invent a
partial success, or silently expose unreviewed bytes. The source attempt and
projection failure are reported separately.

An original-preserving optimizer has a narrower and explicit exception. A
conventional but ineligible source result may expose its exact transformed
stdout, stderr, and status as `preserved_before_processor` only when the plan
permits that stage input; it starts no processor. For eligible input, successful
processor stdout equal to the admitted input is
`preserved_after_processor`; the only different accepted output is the
independently calculated `optimized` summary. Preservation is a success
disposition, not failure recovery, and it cannot cross a confidentiality-
selecting projection boundary in the unsafe direction. Trust output states
that original stage input may remain visible. Persistent state still contains
no raw stdout, stderr, credentials, tokens, or transcripts.

### External output processors

An external output processor is untrusted executable code. Atsura pins and
revalidates its exact path, SHA-256, size, observed version, compatibility
contract, and argv. It starts the processor without a shell, in an isolated
working directory and minimal environment, with closed noninteractive stdin
except for the bounded stage input and with finite time and output limits.

The processor receives no separately supplied credential material, source
stderr, environment snapshot, caller payload, transcript, or authority to launch
the source. Its admitted stage input is still untrusted source output and may
itself contain secrets; original-output visibility is therefore a reviewed plan
fact. Atsura disables telemetry, tee, project-filter lookup, and tracking
defensively where the processor supports those controls. For each claimed
platform, native evidence must replay the exact artifact with this isolated
environment and root contract. Isolation inputs alone do not prove that no
child process, outside-root filesystem access, or network attempt occurred;
Atsura makes no such absence claim until a platform-specific external observer
contract is implemented and validated. Portable processor identity checks
retain a check-to-exec race. A source failure starts no processor. A processor
failure after source start is non-retryable and exposes neither processor
stderr nor failed intermediate output.

RTK itself has been observed checking ambient Claude configuration during
startup. Atsura does not model that host or set a host-specific redirect. The
host-neutral `atsura.processor.rtk_isolated.v2` contract rejects the entire
ambient environment and supplies fresh private generic home, XDG, temporary,
state, and application-data roots plus finite RTK-owned controls. An ambient
host variable is therefore not inherited, and default host configuration is
outside the isolated home. Retired v1 evidence fails compatibility checks.

The isolated root and its owner marker remain pinned by open handles for the
entire processor lifetime. Cleanup performs at most 4,096 root-relative
top-level removals, keeps every recursive traversal anchored to that held root,
revalidates the top-level identity, and performs only a nonrecursive final
directory removal. Observed marker or root replacement fails cleanup, and
additions that keep the held root nonempty beyond the bound also fail. The
nonrecursive final operation ensures an unresolved name race cannot recursively
delete replacement contents. A same-user racer may still cause residue or the
removal of an unrelated empty top-level directory; this is cleanup containment,
not an OS sandbox.

Successful processor status is not semantic validation. The one admitted tuple
is `atsura.output.rtk_go_test_pass.v1`: source-catalog schema 2, Go contract 2,
an inspection-time stable Go 1.26.x observation, exact no-argument caller
invocation, transformed `go test -json`, processor-observation schema 1, and an
official RTK v0.43.0 artifact invoked as `pipe --filter=go-test`. Atsura's
strict single-package pass lifecycle validator and exact summary oracle, not
RTK status, decide eligibility and the accepted postcondition. Skip, failure,
malformed or unknown JSON, empty output, source stderr, nonzero status, and a
non-beneficial summary remain exact pre-processor preservation cases.

ADR 0009 records why RTK `git-log` is rejected. Future tuples must exercise
hostile delimiters, grouping keys, truncation boundaries, and association rules
and reject results whose task-owned relationships cannot be validated
independently of presentation.

Recovery conformance covers every exact scoped-help declaration rather than a
selected sample, including separate source and processor attempt phases and
hostile canaries at output and persistence boundaries.
Narrow controlled ports provide deterministic boundary observations, while
infrastructure tests prove that the production file, trust, identity, and
process adapters emit them. Native runner tests specifically own
start/wait/limit/cancellation/timeout/identity classifications. Defensive
encoding or request faults are tested at their owning boundary without adding
a production fixture escape hatch. Execute encoding conformance consumes a
result corrupted at the CLI-to-application seam only after the production
service and controlled process complete exactly one attempt; the production
application and domain tests independently prove valid undecorated output.

## Raw execution

Future raw execution is an explicit tailoring bypass, not a permission bypass.
It revalidates bundle-bound source identity but applies no surface selection or
wrapper transformation. Raw is never automatic fallback, never a recovery
suggestion, and never part of the tailored agent surface. Raw is outside the
current transform-runtime milestone.

A `preserved_before_processor` or byte-identical
`preserved_after_processor` result from an adopted optimizer is not raw
execution: surface resolution, invocation transformation, exact source
identity, source execution, and all preceding stages still apply. Either is
invalid unless the plan explicitly permits original stage input as output, and
neither may be selected after a processor fault.

Likewise, `source_stream_passthrough` is not raw execution. It preserves the
result of a fully resolved and possibly argv-transformed tailored invocation;
raw execution would bypass those surface and wrapper semantics and remains
unimplemented.

## Failure policy

Atsura fails before source process start when it cannot establish a contract it
owns, including:

- unsupported or retired specification/bundle schema;
- unknown or duplicate fields;
- catalog/specification/bundle digest mismatch;
- invalid surface membership or option override;
- missing, incomplete, or contradictory wrapper stages;
- command absent from the tailored surface;
- attempted option absent from the matched command's tailored option surface;
- missing adoption, source drift, or pre-start identity mismatch;
- a missing, incompatible, drifted, or otherwise unverifiable processor bound
  by an optimizer plan;
- missing, unknown, duplicate, nil, or misconfigured runtime compatibility
  verifier;
- malformed wrapper binding, expected bundle mismatch, honest runtime drift,
  unsupported POSIX platform, or a surface not completely covered by one
  maintained runtime contract; or
- unknown core effect.

Retired authorization schemas are not auto-converted. Their allow/confirm/deny,
read/create/write, target, and impact values have no lossless meaning in the
surface/wrapper model. Migration diagnostics identify the retired schema and a
current recovery command, persist no state, and start zero source processes.

Once an eligible source result has been produced, any processor start, wait,
limit, status, stderr, post-run identity, or semantic-postcondition failure is
non-retryable. Atsura publishes neither the admitted source bytes nor processor
bytes and never selects original output as an automatic failure fallback.

## Secrets and persistence

Atsura does not persist source credentials, environment snapshots, raw source
output, usage history, prompts, caller transcripts, or agent reasoning. Canonical
catalogs and bundles contain only publishable structural evidence and exact
source identity facts required by their contracts. Diagnostic output must not
echo arbitrary secret-bearing environment values or unbounded hostile text.

Managed-shim manifests persist the already reviewed wrapper binding, exact
bundle locator/digest, runtime/source identities, compiled tailored help,
material digest, size, and opaque reference. They persist no credential,
environment snapshot, source/processor output, usage history, host session, or
agent transcript. The executable shim is fixed template material derived from
that binding; configuration cannot contribute shell source.

## Release-artifact security evidence

The candidate archive and extracted `atr` are executable untrusted inputs to
the conformance harness. A fixed regular-member allowlist, link and traversal
rejection, byte bounds, and safe extraction prevent the archive from choosing
other filesystem targets; a digest and safe extraction do not make an
arbitrary executable trustworthy. Replay is limited to the reviewed candidate
from the same workflow, on an ephemeral matching native runner, using absolute
no-shell execution, an isolated working directory, isolated home and
configuration roots, a minimal credential-free environment, and finite time
and output bounds.

The installed-artifact conformance journey runs a provider-transport-free
source fixture with an allowlisted child environment, bounded attempt logs,
and unique stdout, stderr, and unselected-field canaries. Its synthetic
adoption receipt is written through the production trust-store adapter in that
isolated test root. This proves receipt consumption and exact-digest
enforcement; it is not a public trust bypass and is not evidence of user
consent.

The journey must verify the complete ordered preview and execute recovery
signatures from packaged help, show zero source attempts for each induced
pre-start rejection, exactly one attempt for every induced admitted execution,
non-retryable classification for every induced post-start failure, and absence
of canaries from stdout, stderr, saved state, and bounded evidence. Complete
phase coverage belongs to the production-composition fixture above; exact
artifact replay proves that the packaged interface retains that complete help
contract and that its portable induced subset behaves identically. It may
retain only digests, counters, target identity, stable fault codes, and boolean
leak checks. Native replay is required on each claimed release target so an
emulation or cross-build cannot silently replace runtime security evidence.

For the host-neutral slice, packaged help must also expose exact `wrapper
render` and `wrapper run` contracts. On Linux and macOS the native journey must
compare deterministic rendered bytes and digest, activate those bytes in a
generic caller-owned POSIX shell, invoke the ordinary command, and retain only
the plan-declared result. On Windows it must obtain the exact structured
`wrapper_platform_not_supported` result with zero wrapper source attempts, no
processor evidence, and no rendered digest; that is regression evidence, not
POSIX activation or optimizer support.

The native journey opens each candidate release archive once for one bounded
read, then derives both its digest and extracted bytes from that same in-memory
value. A pathname replacement cannot make validation cover bytes different
from those executed by the journey.

Historical bounded journey evidence schema 4 predates the optimizer and binds
only Go adapter contract 1 and the identity-wrapper journey. It is insufficient
to support a release-quality claim for the accepted contract. Schema 5 binds
Go adapter contract 2, processor-observation schema 1, the exact
`atsura.output.rtk_go_test_pass.v1` identity and invocation, processor
path/hash/size/version, catalog/specification/bundle/plan digests, exact caller/
source/processor argv, formats, process modes, v2 isolation and bounds, both
source fixture attempt counts and processor-inspection evidence, result
disposition, status, and the same bounded leak checks. It predates static
tailored help.

Historical schema 6 retains that optimizer-aware proof and adds the first
bounded `tailored_help` record for one transformed-PR wrapper. Historical
schema 7 adds exact `caller_argv` to every wrapper case and binds the transformed
`pr list` and append-only `issue list` calls to one exact bundle and one rendered
wrapper while requiring separate plan digests. A POSIX row proves five exact
root/namespace/command help views while the bound `atr` is non-executable, then
proves hidden `api --help` remains `command_not_in_surface` and unknown
`unknown --help` remains `invalid_invocation`, all without source or processor
attempts. It retains three wrapper source attempts and 13 total GitHub fixture
attempts. A Windows row records the explicit unsupported outcome with empty
wrapper cases, help views, and faults, no tailored-help bundle or rendered-
wrapper binding digests or wrapper contract, zero wrapper attempts, and 10
GitHub fixture attempts. Top-level journey identities remain required.
Aggregate schema 2 is unchanged and does not carry per-case caller argv.

Historical evidence schema 8 succeeds that record. It binds specification schema
5, bundle schema 4, plan schema 6, generated-wrapper contract 3, exact source
argv, the declared option defaults, and the exact applied subset. POSIX rows
require ordered `default_applied`, `default_overridden`, `append_only`, and
`identity` cases, four wrapper source attempts, and 14 GitHub fixture attempts.
Windows retains zero wrapper attempts and 10 GitHub fixture attempts. The Go
and RTK contracts do not change. CI run
[29920148480](https://github.com/tasuku43/atsura/actions/runs/29920148480)
passed these five native schema-8 rows, the canonical full/security/public
gates, and aggregate schema 2 on 2026-07-22 for revision
`99fbd0e97489b1f3b7a68e2617fa4056b2c12a1d`.

Current evidence schema 9 retains schema 8 and adds the managed-shim lifecycle.
Each Linux/Darwin amd64/arm64 native row installs exact GitHub and Go artifacts
through the packaged `atr`, inspects canonical manifest and fixed material,
selects ordinary commands using only the reported bin directory, proves static
help has zero source/processor attempts, executes each source once through the
fresh plan, passes both status references unchanged to removal, and observes an
explicit empty final inventory. It also proves unknown-reference, tamper, and
foreign/symlink/special collision faults leave the fixture-owned filesystem
unchanged and start no source or processor. Collision status is exercised while
another valid owned artifact exists, and its fault output must not expose that
artifact's reference. Windows invokes install/status/remove, receives only
structured unsupported faults, and leaves the store absent. Evidence retains
only digests, references, public argv, counters, and booleans—not source or
processor streams, paths, environment values, or secrets.

Installed evidence does not claim processor-launch counts without an accepted
external observer; controlled application and infrastructure tests own that
attempt truth. On Linux amd64, Linux arm64, Darwin amd64, and Darwin arm64,
native replay must prove an `optimized` strict pass and the reachable
`preserved_before_processor` path through the packaged wrapper. Windows must
continue to prove structured unsupported behavior with zero source attempts,
no processor evidence, and no optimizer claim. Controlled application and
infrastructure tests, rather than the official-artifact journey, own synthetic
`preserved_after_processor` and arbitrary processor-failure branches. No source
or processor stream may enter the evidence document. These are required
evidence conditions; the bounded observation below records the exact revision
that satisfied them. The inherited schema-5 optimizer shape preserves the
identity-wrapper baseline in the outer `go_source` fields and confines the
optimizer's distinct bundle, plan, rendered-wrapper digest, cases, and faults
to the nested `optimizer` object.

The native Go fixture fixes `GOTOOLCHAIN=local`, disables download, and isolates
module/cache roots so that one replay is deterministic. Those are harness-owned
conditions, not production wrapper guarantees. Production inherits the
caller's environment and working directory; constraining effective toolchain
selection would require a new explicit environment/toolchain closure and
native security evidence.

Each native replay emits one bounded journey document. The aggregation tool
accepts exactly the five canonical evidence filenames and five matching
candidate archive filenames as regular non-symlink files, strictly binds each
target, observed host, archive name and recomputed SHA-256, tag-derived
version, full revision, command set, digests, counters, fault set, and leak
booleans, and rejects every extra or missing input. Its summary contains no
filesystem path, raw output, environment value, receipt, bundle digest, or
plan digest and labels itself `workflow_index_unattested`. A syntactically
valid document is not independently attested evidence: the workflow dependency
and GitHub artifact provenance must still show that the matching native matrix
job produced it.

On 2026-07-22, CI run 29910455312 supplied that workflow provenance and passed
all five historical schema-6 rows plus aggregate schema 2 for revision
`01c05a45e8b00f09d63d3c6551d3a5df393c41b5`. The observation does not attest
the executables, authorize publication, prove schema-7 multi-command behavior,
or carry forward to another revision.
CI run 29914651542 then supplied the same workflow provenance and passed all
five historical schema-7 rows plus aggregate schema 2 on 2026-07-22 for revision
`8dd5b251b9bdd93120ceb5e8b2d3cb0caf24c927`. That observation establishes the
bounded multi-command behavior for this revision; it does not independently
attest the executables, authorize publication, or carry forward to another
revision.

## Known limitations

- Hiding commands and options does not prevent invocation outside Atsura.
- Artifact replay is not an OS or network sandbox. “Provider-network-free
  fixture” means the fixture implements no provider transport; it does not
  prove that a malicious candidate executable could not use runner networking.
- Local executable identity checks cannot provide operating-system sandboxing;
  portable path execution may retain a race between the final identity check
  and the operating system opening the file.
- The generated function must start its bound `atr` path before honest
  `wrapper run` code can verify that executable's hash. A mismatch prevents the
  honest runtime from starting the source, but the binding does not attest or
  constrain malicious replacement code already executing at that path.
- Exact bundle adoption does not review or authorize every future downstream
  result of the source CLI.
- Source help can omit dynamic behavior or change through plugins and
  environment; adapter compatibility remains bounded evidence.
- Preview recognizes complete catalog command paths by longest prefix, but the
  catalog and plan grammar do not yet model short options, root/global options,
  or command-specific positional arguments completely. When a matched command
  has cataloged descendants, a non-dash token that is not a known child fails
  closed unless an inner `--` marks positional intent.
- `append_args` are appended exactly even after a positional-only `--`; preview
  exposes that result but does not prove that the source interprets it as an
  option.
- Option-default presence recognizes only the exact long option before the
  first `--`; short aliases intentionally do not suppress a default. The finite
  source compatibility contract, not generic parsing, decides whether each
  resulting invocation is executable.
- Preview requires exactly one active cataloged structured-output selector for
  the planned input format before `--`. Execute further requires an exact
  adapter compatibility contract; GitHub CLI contract 2 covers only `issue
  list` and `pr list`.
- The current GitHub runtime admits major 2 as a maintained range; one captured
  version is not evidence for every future 2.x release. Competing
  `--jq`/`--template`/`--web` output modes, unmodeled options, and positional
  arguments fail before source start.
- The Go runtime admits only catalogs whose recorded inspection observation is
  stable 1.26.x, and only exact no-argument `test`. A catalog recording another
  version, every option, package pattern, positional marker, and test-binary
  argument remain outside the contract. A later effective Go 1.27 selection by
  the same launcher is not detected; ambient toolchain selection and downstream
  test behavior are source-owned, not evidence that Atsura has modeled or
  contained them.
- Successful nonempty stderr is rejected without exposure by
  `transformed_json`, but is returned exactly when the adopted plan declares
  `source_stream_passthrough`.
- POSIX wrapper rendering and caller-owned function activation are limited to
  Linux and macOS. Managed executable shims additionally claim only amd64 and
  arm64. Windows has structured unsupported behavior only. Atsura can persist
  the fixed shim in its private store and report its bin path, but does not
  edit or attest activation state, replace an existing artifact, or provide a
  Windows shim.
- Managed-store checks assume the operating-system user is the security
  principal. They detect passive drift and serialize cooperating Atsura
  processes; they do not contain a malicious process running as the same UID.
- Before/after actions, richer argv transforms, additional optimizer tuples,
  and raw execution remain unimplemented. The only external processor contract
  is the finite RTK Go-test pass tuple accepted by ADR 0012; arbitrary paths,
  argv, filters, versions, platforms, and processor-defined fallback semantics
  remain outside the product. ADR 0008 excludes coding-agent-host adapters from
  the product boundary.

## Security claim for the current milestone

The runtime milestone may claim that validated schema-5 specifications compile
deterministically into schema-4 bundles; preview returns one canonical schema-6
plan with zero attempts; and application rebuilds that plan, requires exact
adoption/current identity and finite-registry source and processor
compatibility, requires every observable executable identity to match the
plan-bound path/hash/size, and starts each declared process at most once without
a shell. `bundle execute` returns only the complete typed selected JSON result
and rejects an optimizer plan before source start. An admitted ordinary wrapper
returns a projection, plan-declared bounded source streams and conventional
status, or one of the three original-preserving optimizer dispositions.

Go adapter contract 2 adds the recorded stable Go 1.26.x inspection
observation, exact no-argument `test`, exact wrapper-owned `-json` transform,
and the finite RTK Go-test pass optimizer. It does not classify the effects of
test execution or freeze the runtime-selected toolchain. RTK receives only
strictly admitted source stdout and has no authority to select or start Go.
Pre-start contract failures start zero processes. Every uncertain post-start
source or processor failure is non-retryable and exposes no captured source or
processor output. The milestone does not claim source-operation authorization,
sandboxing, raw execution, arbitrary external processors, or coding-agent-host
integration.

The host-neutral wrapper implementation adds a narrower conditional claim:
`wrapper render` emits only fixed POSIX source for one completely admitted
surface on Linux or macOS, and an honestly executing bound `wrapper run`
revalidates the bundle/runtime/source/processor closure before reaching the
same fresh plan and controlled process boundaries. Success emits one compact
plan-declared JSON value, exact bounded source streams and conventional status,
or the declared optimizer result; failure never selects raw, another bundle,
or original bytes after processor authority. This becomes a release-quality
optimizer claim only after the required full/security/public/release gates and
the current installed-artifact native evidence pass. Schema 8 passed in CI run
29920148480 on 2026-07-22 for exact revision
`99fbd0e97489b1f3b7a68e2617fa4056b2c12a1d`; every later candidate must
repeat the matrix.

The managed-shim implementation adds a further conditional claim: on the four
Linux/Darwin amd64/arm64 targets, install publishes only exact fixed material
without replacement inside the private store, status returns references only
after complete bounded ownership validation, and remove consumes one reference
unchanged and deletes only the revalidated owned artifact. Management starts no
source or processor. Unknown, tampered, foreign, symlinked, special, crash-
residue, or uncertain state never becomes deletion authority, and caller-owned
activation remains outside Atsura.

The claim does not include executable attestation, same-UID containment,
caller activation integrity, Windows POSIX/managed-shim/optimizer support,
source authorization, sandboxing, replacement, automatic update, or multiple
purpose profiles.
