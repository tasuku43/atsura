# Architecture

Atsura keeps source inspection, surface composition, wrapper planning, source
execution, output processing, and presentation in separate layers. ADR 0005 supersedes the
authorization-centered source-wrapper model from ADR 0004: the core compiles a
purpose-specific command and option surface plus deterministic wrapper
pipelines. It does not decide whether a source operation is permitted. ADR
0006 adds the first compatibility-admitted transform runtime without making its
GitHub CLI evidence part of the shared model. ADR 0007 accepts explicit
RTK-backed optimizer defaults as a future finite processor contract without
delegating source execution to RTK. ADR 0008 keeps coding-agent hosts outside
Atsura: they consume an already generated host-neutral wrapper rather than
enter through a host protocol adapter.

The current runtime milestone extends strict schema-3 specification loading,
schema-2 bundle compilation/adoption, and pure surface resolution through one
complete bundle-backed wrapper plan into one compatibility-admitted JSON
transform execution. It also exposes that same path through a deterministic
Linux/macOS POSIX function rendered from an exact bundle/runtime closure.
Identity/raw execution and persistent wrapper installation remain unimplemented.

## Dependency direction

```text
internal/cli  ------> internal/app
      |                    |
      |                    v
      +------------> internal/domain <------ internal/infra

internal/domain does not depend on app, infra, or cli.
internal/app does not depend on infra or cli.
internal/infra does not depend on app or cli.
```

`tools/archlint` enforces this direction. Source-specific adapters remain
outside the shared domain vocabulary; coding-agent-host protocols remain
outside production packages entirely.

## Artifact flow

```text
bounded source-inspector adapter
  -> provenance-bearing command catalog

catalog + strict tailoring specification schema 3
  -> normalized specification
  -> pure command/option surface composition
  -> complete identity or transforming wrapper for each included command
  -> canonical bundle schema 2
  -> exact-digest user adoption

adopted bundle + attempted invocation
  -> revalidate adoption and current source path/hash/size
  -> longest command-prefix match over the complete catalog
  -> fail closed on child-versus-positional ambiguity unless `--` is explicit
  -> pure surface and option resolution
       -> absent command: command_not_in_surface, no wrapper plan
       -> included command: one complete schema-3 wrapper plan and digest
  -> preview: zero source-process attempts
     or
  -> compatibility-admitted transform runtime
       -> plan-derived expected path/hash/size and exact argv
       -> one no-shell source attempt
       -> bounded JSON parse -> pure select/rename -> fixed result envelope

adopted bundle + explicit purpose binding
  -> `wrapper render`: deterministic POSIX function + review digest
  -> caller-owned command resolution exposes the ordinary source command
  -> fixed function invokes the bound absolute `atr` and forwards exact argv
  -> `wrapper run`: revalidate bundle/runtime/source binding
  -> same fresh plan constructor and compatibility-admitted execution path
  -> one compact plan-authoritative JSON value, not a maintainer envelope
```

Surface membership and wrapper behavior are independent inputs to compilation.
An excluded command has no wrapper. An included command has an explicit option
surface and exactly one complete wrapper. A wrapper change cannot add a
command, and a membership change cannot invent a transformation.

Preview and supported execution share one pure plan constructor:

```text
typed before stages
  -> deterministic argv transformation
  -> exact identity-bound source invocation
  -> typed output transformation
  -> typed after stages
```

The previewed plan binds bundle/catalog/specification digests, exact source and
adapter identity, the matched command, explicit or inherited surface origin,
the exact specification entry or `null`, original and transformed argv,
ordered stages, and finite process bounds. Its canonical bytes determine the
plan digest. It contains no universal allow/confirm/deny decision, inferred
source read/create/write effect, or source-operation target and impact.

## Architectural principles

- Source observations, catalog facts, tailoring specifications, compiled
  bundles, adoption receipts, runtime observations, and agent proposals are
  distinct values.
- The catalog is evidence about a source CLI, not a permission list.
- The specification independently defines surface membership, option
  membership, and wrapper behavior.
- Executable and argv are separate typed values; no specification or plan
  contains a shell program.
- Invocation and output transformations are independent typed stages.
- The finite built-in stage registry is the only initial transformation
  vocabulary. Unknown actions fail during validation.
- One canonical bundle is the compilation product consumed by the direct
  gateway and every materialized wrapper.
- Adoption binds the exact bundle digest. It does not grant source-operation
  permission.
- Source execution and Atsura-owned mutation cross different controlled
  boundaries.
- Source adapters and output processors are orthogonal vendor-neutral ports.
  They cannot broaden a surface or define core wrapper semantics.
- A wrapper binding contains only the exact adopted purpose bundle, wrapper
  contract, runtime identity, command spelling, and source identity. It
  contains no host protocol or configuration state.
- Caller-owned activation is outside Atsura and cannot create adoption or
  change wrapper meaning.
- Output processors remain orthogonal to the caller. A wrapper consumes an
  already compiled stage and never selects RTK or another processor.

## Layer responsibilities

### Domain

`internal/domain/` owns pure values and invariants for:

- exact source executable identity, version, adapter contract, and catalog
  provenance;
- source commands, options, argument evidence, and structured-output
  capabilities;
- schema-3 tailoring specifications;
- explicit `inherit` or `exclude` surface defaults;
- command `include` or `exclude` membership;
- included-command option surfaces;
- identity and transforming wrappers;
- deterministic invocation and typed output transformations;
- canonical schema-2 bundles, digests, and drift validation;
- pure surface resolution and `command_not_in_surface`;
- ordered schema-3 wrapper execution plans and canonical plan digests;
- a host-neutral wrapper binding containing an exact adopted purpose
  bundle, wrapper contract, runtime identity, source identity, and ordinary
  command spelling;
- finite vendor-neutral runtime-admission diagnostic categories; and
- operation effects, including `EffectExecute` for starting a source-owned
  process and create/write contracts for Atsura-owned state only.

Domain validation enforces this truth table:

| Surface membership | Wrapper | Valid result |
|---|---|---|
| Excluded | None | Command is absent; no plan can exist |
| Excluded | Present | Invalid specification or bundle |
| Included | Complete identity wrapper | Valid unchanged wrapper pipeline |
| Included | Complete transforming wrapper | Valid typed wrapper pipeline |
| Included | Missing or incomplete | Invalid specification or bundle |

The domain never decodes YAML or JSON bytes, probes a source executable,
launches a process, mutates trust state, renders terminal output, or speaks a
coding-agent-host protocol.

### Application

`internal/app/` owns task orchestration through narrow ports:

- request bounded source inspection and validate the returned catalog;
- request strict specification and bundle decoding;
- validate catalog/specification bindings before compilation;
- compile and resolve the purpose-specific surface;
- build canonical bundles without ambient values;
- assess exact-digest adoption and source drift;
- require exact bundle adoption, revalidate current source path/hash/size, and
  construct one pure wrapper plan without starting the source;
- coordinate Atsura-owned trust-store changes through the central mutation
  invoker;
- render one adopted bundle/runtime closure as deterministic POSIX function
  material through a narrow pure renderer port, and apply the render-produced
  runtime closure through the same fresh-plan application service as direct
  execution;
- apply a supported transform plan through an identity-bound source-process
  port, a vendor-neutral compatibility port, strict source-format parser, and
  pure transformer;
- for a future optimizer, require exact external-processor identity and
  compatibility before source start, then coordinate at most one processor
  attempt only after an admitted successful source result.

Application code receives typed observations. It does not parse vendor help,
YAML, arbitrary source bytes, shell syntax, or coding-agent-host payloads. It
does not infer source-operation meaning or authorization.

A source launch declares `EffectExecute`. The application binds exact source
identity and argv, requires finite attempts/time/bytes, and treats every
unknown post-start outcome as non-retryable. It attaches no Atsura mutation
target or impact to the downstream source operation.

### Infrastructure

`internal/infra/` owns concrete I/O behind narrow ports:

- resolve and identify source executables;
- perform finite adapter-selected help or metadata probes;
- strictly decode bounded schema-3 YAML and schema-2 JSON;
- reject duplicate or unknown fields and retired schema versions;
- read and persist exact-digest adoption receipts safely;
- observe the current path/hash/size identity used by zero-execution preview;
- execute an exact plan-bound executable plus argv vector without a shell under
  declared time and byte bounds;
- admit only command and argv combinations covered by the exact source-adapter
  compatibility contract before a source attempt;
- parse declared source formats through bounded decoders;
- run a future exact output processor with bounded stdin/stdout/stderr, an
  isolated environment and working directory, no shell, and separately counted
  attempts without giving it source-execution authority; and
- identify the current `atr` executable and render a fixed bounded POSIX
  function without accepting configuration-authored code; and
- in a future persisted lifecycle, own bounded artifact encoding, identity,
  and atomic replacement behind separate mutation ports.

Each source adapter owns its probe grammar, accepted version range, runtime
argv contract, attempt and byte budgets, and conversion into the shared
catalog. Compatibility admission does not make stdout trusted; the format
parser and transformer still validate every successful result. Production
infrastructure contains no coding-agent-host protocol codec, settings store,
permission mapper, process client, or lifecycle adapter.

Infrastructure reports observations and typed failures. It does not decide
which command is included, which wrapper applies, or whether the source CLI
will authorize its downstream operation.

### Host-neutral wrapper boundary

The implemented first vertical slice introduces one generated wrapper binding,
not a coding-agent integration. `wrapper render` derives a fixed POSIX function
from an exact adopted bundle and current `atr` identity. `wrapper run` accepts
ordinary invocation argv and enters the same shared fresh-plan application
service as `bundle execute`; neither command accepts a host hook document or a
shell command string.

The selected command-resolution material is a fixed generated shell function
on Linux and macOS. It contains only Atsura's template, the exact bundle and
runtime closure, structured-error selection, and lossless `"$@"` forwarding.
The tailoring specification cannot contribute shell source. The ordinary
command name is derived verbatim from the bundle's requested executable and
must be one portable non-reserved POSIX Name. The runtime derives the same
spelling from the strictly loaded bundle and reaches its bound physical source
path rather than resolving the wrapper through ambient `PATH`.

If materialization persists local artifacts, application owns the task and
infrastructure owns bounded atomic file operations. The lifecycle exposes exact
ownership and drift, preserves unrelated state, and routes create/write through
the central mutation boundary. Caller-owned shell or agent settings remain
outside that lifecycle.

At invocation, honest `wrapper run` code revalidates its exact runtime identity,
the expected bundle digest and adoption, source identity, and command spelling
before fresh plan construction. Failure starts no source process. Success uses
the existing compatibility admission, no-shell source process, typed transform,
and a plan-authoritative compact JSON renderer. It cannot select raw mode or
another bundle as fallback. The shell necessarily starts the bound `atr` path
before that program can fingerprint itself, so this is cooperative drift
detection rather than attestation or containment against malicious executable
replacement. A generated shell function's digest is deterministic artifact
evidence, not runtime attestation of the sourced function bytes.

Rendering rejects Windows with a structured unsupported fault and requires the
complete included surface to contain exactly one runtime-admitted transforming
GitHub CLI list command, including every exposed option. Windows remains a
regression target for existing commands but receives no POSIX activation claim.

The repository conformance fixture owns only a generic caller environment. A
vendor integration and its host-specific evidence live downstream and consume
the same wrapper argv contract without adding a production Atsura path.

### External output processors

An output processor is orthogonal to source adapters and wrapper consumers. Shared domain
types describe a projection or original-preserving optimizer contract; they do
not contain RTK command lines, host fields, or arbitrary executable
configuration. The specification selects one namespaced, versioned
compatibility-contract identifier. Infrastructure translates that finite
identifier into fixed processor argv, while bundle construction receives an
explicit processor-identity observation rather than searching ambient `PATH`.

The first intended RTK boundary is `pipe` with one explicit filter after Atsura
has started the exact source once. RTK receives only the bounded admitted stage
input and never resolves or starts the source CLI. Source and processor
identity, attempt, status, stderr, timeout, and byte evidence remain distinct.
Missing or drifted processor identity at preflight is checked before source
start. After admitted source success, identity is revalidated before processor
start; a change at that phase is non-retryable with one source attempt and zero
processor attempts. A processor failure after start is non-retryable.

The processor runs with isolated configuration roots and a minimal environment.
Compatibility fixtures, not environment flags alone, record that each exact
native artifact and invocation read no project filter, created no
tracking/tee/telemetry state outside temporary roots, and attempted no network
I/O within the platform harness's declared observation scope. This is bounded
compatibility evidence, not an OS or network sandbox; portable processor
identity checks retain a check-to-exec race. A wrapper consumes the already
compiled stage and never selects RTK at invocation time.

No concrete processor tuple currently occupies that registry. ADR 0009 rejects
the proposed RTK `v0.43.0` `git-log` tuple because its literal block delimiter
collides with valid Git commit text. A future adapter must validate every
semantic delimiter, grouping key, and association precondition through hostile
fixtures before an exit-zero processor result can become plan output.

### CLI

`internal/cli/` is the composition and presentation boundary. It owns:

- catalog-derived public command registration and typed argv parsing;
- specification validation and bundle-build presentation;
- adoption and drift status presentation;
- host-neutral `wrapper render` and `wrapper run` presentation, including the
  static review envelope and fresh-plan-authoritative tailored value;
- stable migration diagnostics for retired policy and bundle schemas;
- schema-3 wrapper-plan and schema-2 tailored-result rendering; and
- composition of application tasks with source and output infrastructure
  adapters.

The current CLI composition injects the GitHub CLI contract-2 verifier into the
generic execution application service. A later source adapter supplies another
implementation of the same compatibility port; it does not add provider fields
to the plan or result.

The credential-free recovery conformance fixture composes the production CLI,
application services, bundle codec, trust store, source identity reader,
GitHub runtime verifier, JSON parser, transformer, and renderer. Narrow owning
ports provide deterministic boundary observations; infrastructure tests
independently prove that the production file, trust, identity, and process
adapters emit them. The fixture directly exercises the generic presentation
encoder for the defensive preview failure. For the corresponding execute
failure, the CLI-to-application seam supplies an invalid typed result that is
corrupted only after the production service and controlled process complete
exactly one attempt; application and domain tests separately prove that the
undecorated service returns validated output. Real source-file drift uses the
production identity reader, while the process runner's own tests induce native
start, wait, limit, cancellation, timeout, and pre/post identity races. No
fixture mode or test branch exists in the shipped composition.

The wrapper entry point does not add identity/raw execution or a persisted
installation lifecycle. ADR 0008 keeps caller activation outside Atsura.
Retired authorization command paths remain only as
catalog-declared migration diagnostics and start zero source processes.

## Controlled side-effect boundaries

### Source-owned process execution

Starting a source executable is `operation.EffectExecute`. The process port
requires every observable identity to match the plan-bound path/hash/size, is
argv-vector-only and no-shell, and is bounded by explicit attempts,
time, stdout, and stderr limits. The source CLI remains responsible for its
prompts, credentials, authorization, remote destinations, and downstream
effects. A post-start unknown outcome cannot be reported as safe to retry.

Source inspection also starts a source-owned process and therefore uses
`EffectExecute`, even when its fixed adapter probes are observational in
purpose.

### Atsura-owned mutation

Trust receipts and any future wrapper artifacts or bindings are Atsura state.
Their create/write tasks retain explicit intent, exact target binding, impact,
central mutation invocation, and structured uncertain-outcome handling. Those
contracts must not be projected onto source CLI commands or caller-owned
activation settings.

## Bundle adoption and drift

The canonical bundle binds source identity, adapter evidence, catalog,
schema-3 specification, and the derived surface with wrapper definitions. A
receipt adopts exactly one digest. Status recomputes every embedded digest and
checks current source identity without starting a routine source task.

A repository path, familiar command name, or previous bundle receipt is not
authority for changed content. Any catalog, specification, surface, wrapper,
source, or bundle change requires a new digest and explicit adoption.

## Raw and caller boundaries

Raw execution, when implemented, will be an explicit tailoring bypass using
the same bundle-bound source identity. It will not apply surface selection or
wrapper transforms and will never be automatic fallback or a recovery hint.

A caller-owned environment may expose a generated wrapper through shell or
agent-host mechanisms, but those mechanisms remain outside Atsura's surface,
plan, execution, fault, and lifecycle boundaries. Atsura cannot claim that a
missing activation is fail closed or that a hidden command is sandboxed. Its
guarantees begin only after the generated wrapper has actually been selected.

## Release-artifact conformance boundary

The release harness owns a credential-free source fixture and
`tools/artifactjourney`, a non-shipped test-only composition root. It owns safe
archive extraction, bounded public-output projections, the deterministic
identity-draft edit, isolated process composition, and conformance evidence.
It exercises the public `atr` binary from an exact native release archive, but
it is not a production adapter and adds no fixture-only branch to the CLI,
application, domain, or infrastructure layers.

Before source inspection, the artifact journey also requires the complete
ordered 27-fault preview and 41-fault execute signatures from packaged scoped
help plus exact `wrapper render` and `wrapper run` contracts. It induces only
the fixed portable subset needed for archive replay; complete zero/one-attempt
phase coverage belongs to the production-composition fixture above.

Each of the five native CI artifact rows also runs the production source
runner tests, exact bundle-file mapping test, and complete CLI recovery matrix
before packaging and replay. This distinguishes portable compilation from
native classification evidence without putting test behavior into `atr`.

The test composition root invokes public commands for the user-visible path.
Its one deliberate internal composition is direct use of the production
`trustfile` adapter against a fresh isolated configuration root, solely to
prove exact-receipt consumption without representing the receipt as human
consent. Release archive member allowlists prove that neither the journey
runner nor source fixture ships, and architecture checks prevent production
`cmd` or `internal` packages from importing harness tools.

`tools/artifactevidence` is the separate test-only aggregation boundary. It
does not execute or rebuild a candidate. It strictly consumes exactly one
bounded journey document and the corresponding candidate archive from each
canonical native target, binds all five to one tag and revision, recomputes
each archive SHA-256, rejects missing, extra, duplicated, symlinked, or
contract-incomplete inputs, and emits only a bounded digest index explicitly
marked as unattested. The GitHub workflow's matrix dependency and artifact
service establish which native jobs supplied those documents; JSON
aggregation alone is not proof of native execution.

Each claimed release target must replay its bounded native journey. Linux and
macOS rows render, activate, and invoke the ordinary POSIX function; the
Windows row exercises the existing command journey and exact structured
unsupported rendering result, not POSIX activation. Cross-compilation proves
that an artifact can be built; it does not prove that the target can execute
it. Platform evidence therefore belongs to the CI and release harness, while
the deterministic catalog, specification, bundle, plan, and execution
contracts remain owned by their production layers above.

## Current milestone boundary

The completed finite transform-runtime milestone is:

```text
strict schema-3 specification
  + validated catalog
  -> pure surface and wrapper compilation
  -> canonical schema-2 bundle
  -> exact-digest adoption/status
  -> current source path/hash/size observation
  -> longest full-catalog command match
  -> included/absent command and option resolution
  -> complete schema-3 wrapper plan + digest
  -> preview: source_process_attempts: 0
     or
  -> exact adapter compatibility admission
  -> plan-derived bound request
  -> one source attempt
  -> bounded typed JSON transform
  -> schema-2 execution result with source_process_attempts: 1

retired authorization schema or command
  -> explicit migration diagnostic
  -> zero source-process attempts
```

The host-neutral wrapper slice implemented around that runtime is:

```text
exact adopted purpose bundle
  -> `wrapper render`: deterministic POSIX function and binding
  -> caller-owned environment exposes ordinary source-command spelling

ordinary argv invocation
  -> fixed absolute `atr` -> `wrapper run`
  -> honest bundle/runtime/source/command binding revalidation
  -> fresh plan through the same application/domain constructor
  -> same bundle execution path
  -> compact plan-declared JSON object or array
```

This implementation does not itself establish release-quality evidence. The
full gates and exact installed-artifact POSIX journey on every claimed Linux
and macOS target remain the completion decision. Windows exercises existing
commands and structured unsupported behavior only.

Identity/argv-only plan application, original-preserving optimizers, external
processor execution, raw execution, source refresh, and coding-agent-host
integration remain outside these milestones.

## Unresolved architecture decisions

- Which argv replacement/default operations and typed before/after actions join
  exact append arguments and structured output transformation.
- How catalog and plan grammar should model short options, root/global options,
  and command-specific positional arguments beyond the current explicit `--`
  disambiguation rule.
- Whether `append_args` may follow an existing positional-only `--`, and how a
  wrapper should express any required insertion point.
- Which source adapters and commands should gain a maintained runtime contract
  after GitHub CLI contract 2 `issue list` and `pr list`.
- Whether named profiles or multiple adopted bundles are needed and how they
  are selected.
- Executable identity evidence beyond exact path, bytes, observed version, and
  adapter contract.
- Streaming and output budgets beyond the current bounded buffered process
  boundary.
- Which executable-shim format, artifact location, ownership, atomic
  replacement, and recursion guard close a persistent wrapper lifecycle.
- How multiple purpose profiles select wrappers for one ordinary command
  without ambient or coding-agent-host state.
- Which source/RTK tuple should replace the rejected ambiguous `git-log`
  candidate and prove the first original-preserving optimizer without an
  unvalidated delimiter or association rule.
- Which explicit processor-observation input and storage boundary should bind an
  exact RTK artifact at bundle build without consulting ambient `PATH`.
- Whether jq, plugins, scripts, or other external processors ever justify a
  similarly finite port.
- The exact raw-execution public contract after wrapper runtime is validated.
