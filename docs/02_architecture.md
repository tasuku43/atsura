# Architecture

Atsura keeps source inspection, surface composition, wrapper planning, source
execution, and presentation in separate layers. ADR 0005 supersedes the
authorization-centered source-wrapper model from ADR 0004: the core compiles a
purpose-specific command and option surface plus deterministic wrapper
pipelines. It does not decide whether a source operation is permitted.

The current zero-execution preview milestone extends strict schema-3
specification loading, schema-2 bundle compilation and adoption, and pure
surface resolution through one complete bundle-backed wrapper plan. Source
runtime, raw execution, and host adapters remain unimplemented.

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

`tools/archlint` enforces this direction. Source-specific and host-specific
adapters remain outside the shared domain vocabulary.

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
  -> zero source-process attempts
```

Surface membership and wrapper behavior are independent inputs to compilation.
An excluded command has no wrapper. An included command has an explicit option
surface and exactly one complete wrapper. A wrapper change cannot add a
command, and a membership change cannot invent a transformation.

Current preview owns one pure plan constructor that future execution must reuse:

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
- One canonical bundle is the compilation product consumed by every future
  gateway or host adapter.
- Adoption binds the exact bundle digest. It does not grant source-operation
  permission.
- Source execution and Atsura-owned mutation cross different controlled
  boundaries.
- Source and host adapters are orthogonal vendor-neutral ports. They cannot
  broaden a surface or define core wrapper semantics.

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
- ordered schema-3 wrapper execution plans and canonical plan digests; and
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
host protocol.

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
  invoker; and
- later apply that same complete wrapper plan through an
  identity-bound source-process port.

Application code receives typed observations. It does not parse vendor help,
YAML, arbitrary source bytes, shell syntax, or host payloads. It does not infer
source-operation meaning or authorization.

When runtime resumes, a source launch declares `EffectExecute`. The application
will bind exact source identity and argv, require finite attempts/time/bytes,
and treat an unknown post-start outcome as non-retryable. It will not attach an
Atsura mutation target or impact to the downstream source operation.

### Infrastructure

`internal/infra/` owns concrete I/O behind narrow ports:

- resolve and identify source executables;
- perform finite adapter-selected help or metadata probes;
- strictly decode bounded schema-3 YAML and schema-2 JSON;
- reject duplicate or unknown fields and retired schema versions;
- read and persist exact-digest adoption receipts safely;
- observe the current path/hash/size identity used by zero-execution preview;
- execute a future exact executable plus argv vector without a shell under
  declared time and byte bounds;
- parse declared source formats through bounded decoders; and
- translate a future host protocol without changing core surface or wrapper
  meaning.

Each source adapter owns its probe grammar, compatible versions, attempt and
byte budgets, and conversion into the shared catalog. Each host adapter owns
only protocol decoding, protocol response mapping, and exact-owner settings
persistence. A host `allow`, `ask`, or `deny` response is transport vocabulary,
not a core permission state.

Infrastructure reports observations and typed failures. It does not decide
which command is included, which wrapper applies, or whether the source CLI
will authorize its downstream operation.

### CLI

`internal/cli/` is the composition and presentation boundary. It owns:

- catalog-derived public command registration and typed argv parsing;
- specification validation and bundle-build presentation;
- adoption and drift status presentation;
- stable migration diagnostics for retired policy and bundle schemas;
- schema-2 wrapper-plan and future tailored-result rendering; and
- composition of application tasks with infrastructure adapters.

The preview milestone does not add bundle execution, raw execution, or host
installation commands. Retired authorization command paths may remain only as
catalog-declared migration diagnostics and must start zero source processes.

## Controlled side-effect boundaries

### Source-owned process execution

Starting a source executable is `operation.EffectExecute`. The process port is
identity-bound, argv-vector-only, no-shell, and bounded by explicit attempts,
time, stdout, and stderr limits. The source CLI remains responsible for its
prompts, credentials, authorization, remote destinations, and downstream
effects. A post-start unknown outcome cannot be reported as safe to retry.

Source inspection also starts a source-owned process and therefore uses
`EffectExecute`, even when its fixed adapter probes are observational in
purpose.

### Atsura-owned mutation

Trust receipt and future integration-setting changes are Atsura state. Their
create/write tasks retain explicit intent, exact target binding, impact,
central mutation invocation, and structured uncertain-outcome handling. Those
contracts must not be projected onto source CLI commands.

## Bundle adoption and drift

The canonical bundle binds source identity, adapter evidence, catalog,
schema-3 specification, and the derived surface with wrapper definitions. A
receipt adopts exactly one digest. Status recomputes every embedded digest and
checks current source identity without starting a routine source task.

A repository path, familiar command name, or previous bundle receipt is not
authority for changed content. Any catalog, specification, surface, wrapper,
source, or bundle change requires a new digest and explicit adoption.

## Future raw and host boundaries

Raw execution, when implemented, will be an explicit tailoring bypass using
the same bundle-bound source identity. It will not apply surface selection or
wrapper transforms and will never be automatic fallback or a recovery hint.

A host adapter, when implemented, will map core outcomes such as `rewrite`,
`not_managed`, `command_not_in_surface`, `invalid_invocation`, and
`interaction_required` into its transport. It cannot turn host `deny` into a
core authorization judgment or claim that a hidden command is sandboxed.

## Current milestone boundary

The finite zero-execution preview milestone is:

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
  -> source_process_attempts: 0

retired authorization schema or command
  -> explicit migration diagnostic
  -> zero source-process attempts
```

Runtime plan application, raw execution, source refresh, and host integration
are deliberately outside this milestone.

## Unresolved architecture decisions

- Which argv replacement/default operations and typed before/after actions join
  exact append arguments and structured output transformation.
- How catalog and plan grammar should model short options, root/global options,
  and command-specific positional arguments beyond the current explicit `--`
  disambiguation rule.
- Whether `append_args` may follow an existing positional-only `--`, and how a
  wrapper should express any required insertion point.
- How each source adapter proves its structured-output selector encoding before
  runtime applies that transform; current preview proves only one active
  cataloged selector and its declared input format.
- Whether named profiles or multiple adopted bundles are needed and how they
  are selected.
- Executable identity evidence beyond exact path, bytes, observed version, and
  adapter contract.
- Streaming and output budgets beyond the current bounded buffered process
  boundary.
- Further source and host adapters and their individual compatibility ranges.
- Whether a future jq, RTK, plugin, or external-transformer port is justified.
- The exact raw and host-adapter public contracts after wrapper runtime is
  validated.
