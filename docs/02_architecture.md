# Architecture

Atsura uses the foundry's four-layer dependency direction to keep YAML
configuration, deterministic planning, process execution, and output
transformation from collapsing into an unrestricted wrapper.

This document assigns intended responsibilities. The current binary does not
yet implement the Atsura pipeline.

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

`tools/archlint` enforces this direction.

## Runtime flow

```text
coding-agent hook adapter
  -> attempted command
  -> application planning use case
       -> validated source/catalog evidence
       -> strictly decoded trusted YAML
       -> pure rule matching
       -> typed execution plan
  -> preview renderer
     or
  -> application execution use case
       -> immediate plan-input revalidation
       -> infrastructure wrapper
            -> built-in before actions
            -> exact source process
            -> bounded output capture
            -> built-in output pipeline
            -> built-in after actions
       -> CLI result or typed failure
```

Preview and execution share plan construction. Execution adds revalidation and
side effects; it does not reimplement policy logic.

## Architectural principles

- Source observations, catalog facts, trusted YAML, runtime facts, and agent
  proposals remain distinct values.
- Executable and argv are separate typed values; no plan contains a shell
  program.
- The plan is immutable input to the wrapper. The wrapper cannot broaden it.
- Invocation transformation and output transformation are independent stages.
- Initial pre/post and output actions come from a finite built-in registry.
- Coding-agent adapters request Atsura tasks but cannot trust policy or bypass a
  rejection.
- All process and filesystem I/O crosses bounded infrastructure ports.

## Layer responsibilities

### Domain

`internal/domain/` owns pure vocabulary and invariants:

- source executable identity and version evidence;
- source commands, options, and output capabilities;
- catalog provenance;
- per-command policy rules and trust provenance;
- rule-match results and reasons;
- allow, confirm, and reject decisions;
- original and transformed invocations;
- typed built-in action specifications;
- ordered execution plans;
- declared source-output input formats;
- output selection, mapping, aggregation, ordering, and result shapes; and
- stage-specific failures.

Domain validation rejects incomplete identity, ambiguous matches, unknown
actions, invalid action ordering, a shell fragment in place of argv, and a plan
whose decision conflicts with its stages.

Domain performs no YAML decoding, source probing, process launch, byte parsing,
terminal rendering, or hook communication.

### Application

`internal/app/` owns deterministic user-task orchestration:

- obtain bounded source and catalog evidence through ports;
- request YAML decoding and validate trust provenance;
- match rules and construct one complete plan;
- return that plan for preview without side effects;
- revalidate configuration, catalog, executable, and relevant environment
  immediately before execution;
- apply confirmation policy;
- authorize exactly one source attempt when the plan permits it;
- coordinate stage-specific failure handling; and
- return task-owned semantic results rather than process DTOs.

Application owns whether output transformation applies to a source result and
how a transform failure is classified. It never launches a process, invokes jq
or RTK, parses arbitrary source bytes directly, or renders user output.

### Infrastructure

`internal/infra/` owns concrete I/O behind narrow ports:

- resolve and identify source executables;
- perform bounded source help or metadata probes selected by an inspection
  task;
- decode strict YAML into syntax DTOs while preserving file provenance;
- persist catalogs or trusted configuration only if later approved;
- adapt Claude Code or another host hook protocol;
- execute the exact plan executable with its argv vector and bounded working
  directory, environment, time, stdout, and stderr;
- parse declared source formats such as JSON through bounded decoders;
- apply byte-level mechanisms required by typed built-in transformations; and
- later adapt a specifically approved external transformer behind its own
  contract.

Infrastructure reports observations and typed failures. It does not decide
which source capability is visible, allowed, or confirmed, and it does not
interpret a generic shell string from YAML.

### CLI

`internal/cli/` is the composition and presentation boundary for `atr`. It
will own:

- public command registration and typed arguments;
- plan preview presentation;
- tailored result and stage-specific failure rendering;
- human and agent help derived from product contracts;
- composition of application use cases and infrastructure adapters; and
- any CLI-facing installation or status workflow for host integrations.

The inherited `doctor` and `sample` commands remain scaffold evidence. No
public command name for planning, execution, or hook installation is selected
by this document.

## Responsibility map

| Concern | Semantic owner | I/O or presentation owner | Current decision |
|---|---|---|---|
| Source CLI investigation | Application task and domain evidence | Infrastructure probe | Source and exploration depth unresolved |
| Command catalog | Domain values; application assembly | Infrastructure persistence; CLI view | Generated evidence, never permission |
| YAML decoding | Domain policy semantics; application trust validation | Infrastructure strict syntax decoder | YAML selected; schema and paths unresolved |
| Rule matching | Domain pure evaluation | Application supplies validated inputs | Deterministic |
| Plan construction | Domain invariants; application compiler | CLI preview | One plan logic for preview and execution |
| Hook interception | Application task boundary | Infrastructure host adapter | Claude Code or similar; exact protocol unresolved |
| Command discovery hiding | Domain tailored surface | CLI/host integration | Distinct from execution rejection |
| Process execution | Application authorizes one attempt | Infrastructure process adapter | No shell interpolation |
| Output transformation | Domain typed actions; application result policy | Infrastructure parse mechanics; CLI render | First-class built-in pipeline |
| External transformer | Future reviewed port | Future infrastructure adapter | jq, RTK, plugins, and scripts excluded initially |

## First vertical boundary

The recommended first slice stops before source execution:

```text
synthetic source evidence
  + small per-command YAML fixture
  + attempted invocation
  -> strict decode
  -> rule match
  -> typed plan with invocation and output stages
  -> preview
```

The fixture should describe a substantial built-in output reshape so the plan
model is not accidentally limited to argv rewriting or line shortening.

Implementation requires `$add-capability`, a work packet, catalog decisions,
and contract tests. This thesis change does not implement the slice.

## Unresolved architecture decisions

- Exact YAML schema, matching, inheritance, locations, and trust workflow.
- Executable identity across PATH changes, symlinks, replacement, plugins, and
  version drift.
- Portable catalog observations and source-specific inspectors.
- Claude Code hook responsibilities and agent discovery integration.
- Confirmation interaction and authorization reuse.
- Built-in action vocabulary and extension compatibility.
- Buffered versus streaming transforms and output budgets.
- Source nonzero exit, stderr, partial output, and transform-failure ordering.
- Whether a future jq, RTK, plugin, or external-transformer port is justified.
