# Atsura Product Theses

These theses define Atsura's product direction. They are working hypotheses,
but they govern implementation until evidence justifies another revision. ADR
0005 corrects the authorization-centered interpretation introduced by ADR
0004, ADR 0006 accepts the first compatibility-admitted transform runtime, and
ADR 0007 prefers explicit RTK-backed optimizer defaults where Atsura maintains
an exact compatibility contract. ADR 0008 corrects the agent-host boundary:
coding-agent hosts consume an already generated host-neutral wrapper and are
not Atsura adapters. ADR 0010 admits plan-declared source-stream results for
finite identity and argv-only ordinary wrappers. ADR 0011 adds Go CLI as a
second source contract and replaces direct single-adapter composition with one
finite application compatibility registry. ADR 0012 admits the first exact
external output-processor tuple: strict pass-only `go test -json` through an
explicitly inspected RTK v0.43.0 artifact. ADR 0014 makes the ordinary wrapper
self-discoverable by compiling bounded tailored help from the exact bundle
into fixed wrapper material. ADR 0015 admits a non-empty, complete GitHub
contract-2 surface with one or both maintained commands while retaining
all-or-nothing runtime admission. ADR 0016 adds the first catalog-typed value-
option default with caller precedence, plan explanation, and exact-command
help disclosure. The vendor-neutral, compiled-bundle architecture remains
authoritative.

## North star

**Atsura is a deterministic framework for compiling an existing CLI into a
purpose-specific wrapper surface for coding agents. It changes which commands
and options exist in that surface and how those commands invoke and transform
the source CLI. It does not decide whether the user is authorized to perform
the source operation.**

The primary user is a maintainer of a coding-agent environment. The maintainer
wants a smaller, purpose-specific CLI whose shape and wrapper behavior can be
reviewed, reproduced, explained, and used without a runtime language model.

## Intended experience

```text
source-inspector adapter -> provenance-bearing command catalog
catalog + reviewed tailoring specification
  -> deterministic, content-addressed bundle
  -> purpose-specific command and option surface
  -> wrapper pipeline for every included command

adopted bundle + attempted source invocation
  -> resolve command in the tailored surface
  -> apply declared option defaults only where caller argv omitted them
  -> compile one wrapper execution plan
  -> preview the plan and its digest without starting the source
     or
  -> when the adapter admits the runtime contract, apply one identity-bound
     source invocation and its plan-declared result mode
       -> transformed_json: typed projection
       -> source_stream_passthrough: bounded exact source streams and status
       -> original_preserving_optimizer:
            -> ineligible conventional result: exact source streams and status
               with no processor attempt
            -> eligible result: one admitted processor attempt yielding either
               exact admitted input or independently validated optimized bytes

adopted bundle + explicit purpose binding
  -> `wrapper render` produces a deterministic host-neutral POSIX function
  -> caller-owned environment exposes it as the ordinary source command
  -> maintainer or coding agent invokes that ordinary command
  -> final `--help`: fixed bundle-derived tailored help without `atr`, source,
     or processor execution
  -> every other argv: fixed function calls `wrapper run` with its exact
     bundle/runtime closure
  -> wrapper run rebuilds and applies the same fresh plan
```

Source-CLI inspectors and finite output processors are adapters. Coding-agent
hosts are callers outside the product boundary. They may arrange command
resolution, but cannot create a second surface model, add wrapper semantics, or
turn Atsura into an authorization engine.

## Thesis 1: The tailored surface is a purpose-specific CLI

A command excluded from a tailored surface is absent from that CLI. It is not
an operation that Atsura found unsafe or refused to authorize. Resolution must
therefore use capability vocabulary such as `command_not_in_surface`, not
permission vocabulary such as `policy_rejected`.

Surface composition has an explicit default. An omitted command is inherited
or excluded because the specification says so, never because omission is
silently treated as denial. The same principle applies independently to the
options visible for an included command.

### Consequences

- Source catalog evidence is not a permission list.
- Hiding improves the agent-facing capability surface; it is not an OS sandbox.
- A caller may apply its own controls before invoking the wrapper, but those
  controls are neither interpreted nor preserved by Atsura.
- Source CLI, operating system, credential, and remote-provider authorization
  remain authoritative.

### Mechanical enforcement target

The specification model must represent `inherit` or `exclude` as an explicit
surface default, represent command membership separately from wrapper behavior,
return no execution plan for a command absent from the compiled surface, and
derive ordinary wrapper help only from included surface entries and their
effective options.

## Thesis 2: YAML is a tailoring specification, not an authorization policy

Reviewed YAML describes the deterministic difference between source CLI
evidence and one purpose-specific wrapper surface. It may describe:

- command and option inclusion or exclusion;
- identity wrappers that preserve source invocation and output;
- deterministic argv additions, removals, replacements, and defaults;
- selection of source-native structured output;
- typed processing before and after the source process;
- typed output transformation; and
- the reason for each explicit change.

The implemented schema supports exact appended argv and one finite default
operation for cataloged, tailored, value-taking long options. A configured
default applies only when its exact long option is absent before `--`; caller
inline, separated, explicit-empty, and repeated occurrences remain exact and
suppress insertion. Short aliases never suppress a long default, and matching
text after `--` remains positional. Missing defaults are inserted in declared
order immediately after the matched command path. The canonical
`--option=value` argv element, not only its value, must fit the 4096-byte source
argument bound. Removal, replacement,
boolean, short, root/global, positional, conditional, and environment-derived
defaults remain unsupported rather than being emulated with generic strings.

Unsupported actions remain explicit unknowns rather than generic strings or
embedded code. Schema 5 may select the one finite external-output-processor compatibility
contract maintained for strict passing Go test output. It never embeds an
executable path, arbitrary command, shell fragment, RTK program or argv,
plugin, or script. The authoring workflow materializes that default only from
an explicit compatible processor observation before review; neither the
compiler nor runtime discovers or inserts an ambient tool. Arbitrary shell,
arbitrary scripts, and a runtime language model are not part of the initial
specification.

### Consequences

- The product vocabulary is `tailoring specification`, `surface`, `wrapper`,
  and `pipeline`, not source-operation policy or permission.
- Source wrapper rules do not require allow/confirm/deny, read/create/write, or
  authorization target/impact declarations.
- Repository-provided specifications remain untrusted until a user adopts the
  exact compiled bundle.
- Unknown fields, actions, ambiguous matches, and invalid stage combinations
  fail before a source process starts.

### Mechanical enforcement target

Strict bounded decoding, versioned migration diagnostics, domain types, and
canonical round trips must reject the retired authorization schemas rather than
silently reinterpret them.

## Thesis 3: Surface membership and wrapper behavior are independent

Every source command is resolved along two independent dimensions:

1. whether it exists in the purpose-specific surface; and
2. if it exists, which wrapper pipeline applies.

An included command may use an identity wrapper or a transforming wrapper. An
excluded command has no wrapper and produces no execution plan. Changing a
wrapper never implicitly adds a command, and changing membership never invents
a transformation.

The wrapper pipeline keeps these ordered stages distinct:

```text
typed before actions
  -> deterministic invocation transformation
  -> exact identity-bound source invocation
  -> typed output transformation
  -> typed after actions
```

### Mechanical enforcement target

Domain validation must reject an excluded entry with a wrapper, an included
entry without a complete wrapper, an identity wrapper containing transforms,
and a transform wrapper containing no transformation.

## Thesis 4: One plan explains and applies one wrapper pipeline

A wrapper execution plan is not an authorization decision. It is the complete,
typed result of resolving an included command against source evidence, one
adopted bundle, and the attempted invocation. `bundle preview` constructs that
plan only after revalidating the bundle's exact user adoption and current
source path, SHA-256, and size. It starts no source process.

A complete plan identifies at least:

- the matched tailored command;
- bundle, catalog, specification, source, and adapter identity;
- the exact processor binding when the selected output stage requires one;
- original and transformed argv;
- every declared option default and the exact applied subset;
- the exact applied specification entry, or explicit `null` when the surface
  entry was inherited, plus its reason;
- before actions;
- the exact source invocation;
- the exact result mode;
- declared source-output input format;
- output transformation;
- after actions; and
- tailored mode and finite source-process bounds.

For an included command, successfully constructing the complete plan proves
that the wrapper pipeline is fully described. Execution additionally requires
an implemented runtime for the wrapper kind and adapter compatibility evidence
for any source-native output contract. For a command absent from the surface, plan
construction returns a surface-resolution failure. Execution reuses this plan
logic; an old preview is never runtime authority after bundle or source drift.

### Mechanical enforcement target

Identical validated inputs produce identical plans and the same canonical plan
digest. Resolution chooses the longest matching command path from the complete
catalog before checking command and option membership in the tailored surface.
When a matched command has cataloged descendants, unresolved non-dash data is
not guessed to be a child or a positional; an explicit `--` must disambiguate
positional intent.
For a defaulted option, detached validation recomputes caller presence before
the first `--`, the declaration-ordered applied subset, canonical
`--option=value` insertion immediately after the command path, and the final
argv before accepting a plan or digest.
Preview reports `source_process_attempts: 0`. `bundle execute` revalidates the
bundle and source identity, reuses this constructor, binds the plan's exact
path/hash/size to the process boundary, and starts at most the one source
attempt declared by the current wrapper contract. `wrapper run` reaches that
same constructor and process boundary after validating the render-produced
bundle digest and exact current `atr` path/hash/size. It derives the ordinary
source spelling from the strictly loaded bundle instead of accepting a second
caller-controlled spelling. Its success is the fresh plan's exact result
variant: either the compact JSON object or array, bounded source stdout/stderr
and conventional status, or an original-preserving optimizer result whose
disposition is `preserved_before_processor`,
`preserved_after_processor`, or `optimized`. None is the maintainer evidence
envelope returned by `bundle execute`; that direct evidence command rejects the
optimizer mode before source start.

## Thesis 5: The source CLI owns source-operation meaning and authorization

Atsura does not infer remote effects, safety, or authorization from command
names, help prose, or a maintainer-supplied read/write label. The source CLI
owns its domain semantics, authentication, authorization, destinations,
prompts, and downstream side effects.

Atsura still owns the safety of its own behavior. Starting an identity-bound
external process—whether the source CLI or an admitted output processor—is
declared honestly as execution, not disguised as an Atsura read. The source CLI
alone owns the downstream meaning of a source operation; a processor remains a
separate untrusted process with no authority to select or start the source.
Atsura-owned local mutations—such as bundle trust-store writes and any future
wrapper-artifact installation or replacement—continue to require explicit
effect, target, impact, central mutation invocation, and uncertain-outcome
handling.

### Mechanical enforcement target

`operation.EffectExecute` represents starting an identity-bound external source
or processor process and cannot carry an Atsura mutation target or impact.
`EffectCreate` and `EffectWrite` retain the existing mutation contracts for
Atsura-owned state. Unknown effects remain non-executable.

## Thesis 6: Output projection and optimization are first-class wrapper stages

Invocation transformation chooses the exact source executable and argv. Output
stages interpret successful source output and produce the declared agent-facing
result. They are separate from invocation and share no generic shell escape
hatch.

A typed projection and an original-preserving optimizer have different
contracts. A projection promises a declared shape and fails closed without
exposing its input. An optimizer may return conventional ineligible source
bytes as `preserved_before_processor`, byte-identical admitted input as
`preserved_after_processor`, or independently validated transformed bytes as
`optimized`, but only when the adopted plan explicitly permits original input
as agent-facing output. These are declared stage dispositions, not automatic
recovery from a failed projection or processor.

The preferred path is to request source-native structured output when the
adapter can verify it, parse it within declared bounds, apply typed built-ins,
and render a task-specific structure without inventing facts.

Preview verifies one active cataloged selector for the planned input format
before `--`. Runtime proceeds only when the exact source adapter kind, contract
version, source version, command, complete argv grammar, and selector value are
covered by maintained compatibility evidence. One application-owned finite
registry dispatches the plan and complete-surface proof by the exact adapter
kind already bound into the bundle; it does not inspect plugins, construct a
source request, or execute a process. GitHub CLI adapter contract 2 currently
admits only `issue list` and `pr list` after four fixed offline
version/reference/command-help probes. Successful stdout is still untrusted and
must satisfy the bounded JSON parser and typed transform.

Go CLI adapter contract 2 uses exactly three fixed offline probes: `go
version`, `go help`, and `go help test`. Its first runtime admits only plans
whose recorded inspection-time effective-toolchain observation is stable Go
1.26.x. It retains the exact no-argument identity-wrapper
`source_stream_passthrough` contract and additionally admits one transforming
wrapper that appends exact `-json` and selects
`original_preserving_optimizer`. Any caller-supplied option, package pattern,
positional marker, or test-binary argument is outside these contracts and fails
before source start. `go test` remains a source-owned `EffectExecute`: it may
compile and run repository code, consult credentials or configuration, resolve
modules, access networks, and mutate caller-owned files or caches. Atsura
neither classifies nor authorizes those effects.

`go version` may itself delegate, so `Source.Version` is the effective
toolchain observed under the inspection working directory and environment; it
is not the version of the direct launcher file. Runtime binds that direct
launcher's path/hash/size and exact argv but does not repeat the observation.
The same launcher may later select or download another toolchain from module
directives, `GOTOOLCHAIN`, `GOROOT`, or related ambient state without pre-start
detection. Atsura does not identify the selected toolchain or GOROOT tree.
Constraining that behavior requires an explicit environment/toolchain closure,
a successor ADR, and new native evidence rather than stronger wording around
contract 2.

Projection failure never changes argv, retries the source process, selects raw
mode, or silently exposes unreviewed raw output.

An identity or argv-only wrapper may instead declare
`source_stream_passthrough`. After one conventionally completed, identity-bound
source attempt, Atsura returns bounded stdout and stderr bytes without framing,
projection, UTF-8 interpretation, terminal-safety claim, or semantic-safety
claim, and returns the conventional source status only after both final writes
complete. This is explicit adopted wrapper behavior, not raw execution or
fallback: surface and option resolution, argv transformation, source identity,
compatibility admission, and fresh-plan validation still apply. Timing and
stdout/stderr interleaving are not preserved. Uncertain post-start outcomes
suppress both captured streams and never make replay safe.

The first implemented tuple is
`atsura.output.rtk_go_test_pass.v1`: source-catalog schema 2, Go CLI contract
2, an inspection-time stable Go 1.26.x observation, caller argv `go test`,
source argv `go test -json`, processor-observation schema 1, and an explicitly
inspected official RTK v0.43.0 artifact invoked as `pipe --filter=go-test`.
Specification schema 5 records only the typed contract and original-output
allowance. Bundle schema 4 and plan schema 6 bind the exact processor identity,
version, filter mapping, bounds, and reason. An installed tool never changes
the choice, and RTK is never the source executor, a runtime-selected fallback,
or a permission mechanism.

Atsura independently admits only one strict single-package pass lifecycle and
computes its exact shorter summary before processor start. Every conventional
but ineligible result—including skip, failure, malformed or unknown JSON,
empty output, source stderr, nonzero status, and a non-beneficial pass
summary—is returned byte-for-byte with its status as
`preserved_before_processor` and zero processor attempts. Eligible input is
passed once to RTK only after a second identity check. Valid RTK stdout may be
the exact admitted input (`preserved_after_processor`) or the independently
computed newline-free summary (`optimized`). Once processor authority begins,
failure, uncertainty, stderr, drift, or unexpected output is non-retryable,
publishes no source or processor bytes, and never falls back.

ADR 0009 still rejects the proposed `git log` / `git-log` tuple because a valid
commit message can collide with RTK's literal delimiter and produce a
successful but misleading association. RTK's advertised support list, exit
zero, and shorter output remain research evidence rather than compatibility
authority for any additional tuple.

## Thesis 7: Agents propose and invoke; the deterministic core compiles

A coding agent may propose a tailoring specification from a role, purpose, or
usage evidence. A user-controlled workflow adopts the exact compiled result.
Runtime surface resolution, plan construction, argv transformation, and output
processing are deterministic and attributable to the bundle.

The routine agent experience begins after a caller-owned environment has made
an Atsura-generated wrapper available under the ordinary source-command
spelling. The agent simply invokes that command. The wrapper receives argv,
revalidates its exact bundle, runtime, command, and source binding, constructs a
fresh plan, and applies the same runtime used by the direct maintainer gateway.

The first material form is a fixed POSIX function produced by `wrapper render`
on Linux and macOS. Contract 3 embeds the absolute current `atr` identity,
exact bundle digest, and one bounded help projection derived from the included
surface. A final exact `--help` selector for the root, an included namespace,
or an included command returns that fixed projection without starting `atr`,
the source, or a processor. Every other argv list invokes `wrapper run` with
structured JSON errors and is forwarded unchanged without `eval`, `sh -c`, or
specification-authored source. Rendering is
allowed only when the complete included surface is non-empty and every entry is
admitted by the one registry-selected source contract before any bytes are
rendered. GitHub CLI contract 2 permits one or both maintained `issue list` and
`pr list` commands and permits those entries to use different existing result
modes. Go CLI contract 2 remains exactly one no-argument `test` command, using
either its identity wrapper or the one processor-bound pass-only optimizer.
Windows returns a structured unsupported fault for POSIX rendering and has no
optimizer runtime claim; this contract makes no Windows activation claim.
Exact-command help discloses each configured default as a deterministic quoted
value. Root and namespace views retain command membership only.

Production Atsura has no coding-agent-host adapter. It never discovers,
inspects, starts, signals, or calls a host process, executable, service,
session, transcript, or API; decodes a hook payload or shell command string;
returns a host rewrite or permission decision; or owns host settings, hooks,
trust, or permission rules. Those responsibilities stay in the caller's
environment even if thin external glue makes the wrapper visible.

The host-neutral wrapper binding contains only Atsura product facts: the exact
adopted purpose bundle, source identity, wrapper contract, runtime identity,
ordinary command spelling, and bundle-derived tailored help. It contains no
coding-agent host, hook, model, session, or host-permission field. A generated
shell function's byte digest is reproducibility and review evidence; after
caller-owned activation Atsura does not claim to attest the in-memory function
bytes. Static help names its exact bundle digest and describes that rendered
artifact; it does not claim that later source, processor, adoption, or runtime
state is current.

Runtime binding is cooperative drift detection, not executable attestation.
The fixed function must start the bound `atr` path before honest runtime code
can hash itself. A mismatch prevents that honest runtime from starting the
source, but it cannot constrain malicious replacement code already executing at
the path.

Any coding-agent host may be an external consumer of this same argv contract.
Atsura does not maintain vendor compatibility fixtures or claim that it
installed the wrapper in a particular host. A downstream integration owns its
activation and compatibility evidence outside the product repository.

Output processing is orthogonal to the caller. A wrapper consumes the already
reviewed bundle and never detects RTK, selects a filter, or inserts an output
processor at invocation time.

## Thesis 8: Bundle trust adopts a surface and wrapper set

One canonical bundle binds source identity, adapter contract, catalog evidence,
normalized tailoring specification, compiled surface, and wrapper behavior.
Its digest is its identity. Trust is a user-local decision to adopt that exact
purpose-specific CLI, not a grant of permission to source operations.

A trust summary therefore describes included and excluded surface entries,
option changes, a distinct option-default count, identity and transforming
wrappers, argv changes, processing
stages, output transformations, source-stream result visibility, source
identity, exact processor bindings, original-output visibility, and bundle
digest. When source streams or pre-processor preserved bytes may be returned unprojected,
the controlling review warns that they may contain secrets, controls, malformed
text, or prompt-like content. It stores none of those bytes. The summary does
not count source permissions, decisions, or inferred effects.

Raw execution is a separate, explicit tailoring bypass. It revalidates the
bundle-bound source identity but applies no surface selection, argv transform,
or output transform. Raw is never automatic fallback, a recovery suggestion,
or part of the tailored agent surface.

Returning the exact admitted input from an adopted original-preserving optimizer
is not raw execution. It does not bypass surface resolution, source identity,
invocation transformation, or any preceding output stage, and the trust summary
must state that original stage input may remain visible.

## Release-quality target

Release quality closes one supported maintainer result rather than maximizing
mechanism count. A result is supported only when it is discoverable, bounded,
machine-interpretable without undeclared reconstruction, recoverable through
declared faults, and verified against the same artifacts users install.

The completed direct implementation-quality slice is:

**A maintainer can create a catalog-bound specification with an explicit
surface default, include one verified command with a typed JSON-transforming
wrapper, build and adopt the exact bundle, preview the resolved wrapper, and
execute the same fresh plan once to receive only the selected and renamed typed
JSON result.**

This slice tests the corrected surface/wrapper vocabulary at the controlled
runtime boundary without adding source authorization or a host dependency. It
reaches release quality only after the same scenario is replayed with the exact
artifacts on every platform for which runtime support will be claimed; archive
reproducibility alone is not that evidence.

The implemented host-neutral wrapper slice is:

**A maintainer can render one exact adopted purpose bundle as deterministic
POSIX function bytes, expose those bytes through an explicitly chosen caller-
owned command-resolution mechanism, and invoke the ordinary source-command
spelling to reach the same fresh plan and transform runtime as the direct
gateway. A missing, drifted, or mismatched bundle, runtime, source, surface, or
invocation starts no source process.**

The ordinary tailored-help extension is:

**From those same reviewed wrapper bytes, a maintainer or coding agent can use
the ordinary root, included namespace, or included exact-command spelling with
a final `--help` to discover only the bundle's included commands and effective
long options. The fixed help path names the exact bundle, starts no bound
`atr`, source, or processor process, does not execute or embed source help, and
leaves every non-help invocation on the existing fresh-plan path.**

The complete-surface extension is:

**A maintainer can render one same-source GitHub bundle containing one or both
maintained contract-2 commands, including a surface whose commands use
different existing result modes. Rendering succeeds only after every included
command, option surface, wrapper, and result mode is admitted; one unsupported
entry prevents all wrapper bytes rather than being omitted or deferred to
invocation. Go remains the exact singleton `test` surface.**

The catalog-typed option-default extension is:

**A maintainer can configure `gh pr list --limit` with one public, reviewed
default. Omission inserts the canonical value before caller tail argv; an
explicit caller value, including explicit empty, wins unchanged. Preview
records the declared and applied subsets, exact-command wrapper help discloses
the value, and neither path starts a source or processor. The full canonical
`--option=value` argv element is bounded to 4096 bytes.**

The bounded source-stream extension is:

**For one fully adapter-admitted identity or append-argv-only surface, that
ordinary wrapper applies the same fresh plan once and returns the conventionally
completed source stdout and stderr bytes unchanged, followed by the source
status only after complete final delivery. It never selects this visibility as
fallback, and uncertain execution exposes neither captured stream.**

The second-source proof is:

**A maintainer can obtain a stable Go 1.26.x effective-toolchain observation
through three fixed offline probes of one directly identified Go launcher,
adopt an exclude-by-default bundle containing only an identity-
wrapped `test` command, and invoke ordinary no-argument `go test` through the
same host-neutral wrapper, fresh-plan constructor, finite compatibility
registry, and source-process boundary. Any additional Go argv starts no source
process.**

The first external-output-processor proof is:

**A maintainer can explicitly inspect one official RTK v0.43.0 executable,
materialize the exact maintained Go pass optimizer into a reviewed schema-5
specification, build and adopt its processor-bound schema-4 bundle, and invoke
ordinary no-argument `go test` through the same host-neutral wrapper. A strict
eligible pass stream yields the independently validated shorter summary;
conventional ineligible results are preserved exactly before RTK starts; and a
processor fault after source execution publishes no bytes or fallback.**

The release-quality proof uses a generic caller-owned activation fixture and
the exact installed `atr` artifact. It verifies the wrapper bytes, binding,
argv, plan, attempt, and tailored result without importing a coding-agent host
protocol. Host independence follows from that host-neutral boundary; a
downstream vendor integration is responsible for proving its own activation.
The implementation does not become a release-quality platform claim until that
exact-artifact evidence and the required gates pass on the claimed targets.

## Current non-goals

- Deciding whether a user may perform a source operation.
- Replacing source CLI, OS, credential, or remote-provider authorization.
- Claiming that hidden commands are sandboxed or impossible to invoke elsewhere.
- Reimplementing source CLI domain semantics or remote APIs.
- Arbitrary shell, scripts, jq programs, RTK programs/argv, plugins, or
  unregistered external transformers in the initial specification. The one
  accepted RTK contract does not generalize this boundary.
- Unplanned or implicit raw/intact-output fallback. An adopted optimizer's
  declared `preserved_before_processor` or `preserved_after_processor` result
  is not fallback; a processor fault can produce neither.
- Requiring a language model for routine execution.
- Typed before/after actions in the initial runtime. `bundle execute` remains
  the direct JSON-transform evidence command; source-stream results belong to
  the finite ordinary-wrapper path in this slice.
- Source refresh or raw execution.
- Persisting, installing, replacing, selecting, or removing wrapper artifacts;
  executable/PATH shims and multi-profile activation remain later lifecycle
  work.
- Coding-agent host adapters, host hook decoding or rewriting, host settings or
  permission mutation, host process inspection, and vendor-specific lifecycle
  commands.
- Coding-agent-host compatibility, activation, installation, or enforcement
  claims.
- Boolean, short, root/global, positional, conditional, computed, or
  environment-derived option defaults.
- Using specification defaults for credentials or other secrets; their exact
  values are public in the specification, bundle, plan, help, and evidence.
- Publishing or releasing Atsura.

## Open questions

- Which argv removal/replacement actions and typed before/after actions should
  follow the first finite option-default vocabulary?
- How should an agent-facing option surface represent positional arguments and
  mutually dependent source options?
- How should the catalog and plan grammar model short options, root/global
  options, and command-specific positional arguments without guessing?
- Should invocation transforms be allowed to append option-looking arguments
  after an existing `--`, where the source will interpret them as positional?
- Which recorded inspection-time Go version observations beyond stable 1.26.x
  can enter a maintained source contract, and which evidence can revise that
  admission range without implying runtime toolchain closure?
- Should a future Go runtime close `GOTOOLCHAIN`, `GOROOT`, module toolchain
  selection, and the selected toolchain identity; if so, which explicit
  environment/toolchain artifact can prove that closure across platforms?
- Which Go test options, package patterns, positional markers, and test-binary
  arguments can be modeled without guessing across build, test, and package
  grammar?
- How should multiple purpose profiles select distinct wrappers for the same
  source command without ambient or host-specific state?
- Which executable-shim format, location, ownership, binding, atomic
  replacement, and recursion guard provide a reviewable persistent lifecycle
  without giving repository content authority?
- What stronger executable identity mechanism can close the remaining
  check-to-exec race on each supported platform?
- Which later source, version, command, RTK version, or filter—if any—can meet
  the same independent semantic-admission and native-evidence threshold as the
  first finite optimizer tuple?
- When, if ever, should jq, other external transformers, plugins, or scripts be
  admitted through a similarly finite contract?
- How, if at all, should usage evidence be collected without storing secrets or
  raw confidential output?

These questions require a vertical slice or primary-source research. They are
not authorization questions to be answered by adding allow/deny fields.
