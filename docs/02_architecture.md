# Architecture

Atsura keeps the foundry's four-layer dependency direction while assigning the
future source-inspection, policy, planning, and execution responsibilities to
explicit boundaries. This document describes intended ownership, not
implemented Atsura capabilities.

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

`tools/archlint` enforces this direction for production packages. The binary
entry point remains a thin composition handoff.

## Architectural principles

- Source-CLI facts, reviewed policy facts, and runtime observations remain
  distinguishable.
- The deterministic core receives bounded ports, not an unrestricted process,
  filesystem, or network executor.
- A coding-agent integration adapts an external workflow to Atsura use cases;
  it does not become the policy evaluator or authorization authority.
- Source execution is reached only through one infrastructure boundary after a
  complete execution plan is validated.
- No layer represents a transformed invocation as a shell program. Executable
  and argv are separate values.

## Layer responsibilities

### Domain

`internal/domain/` owns pure Atsura vocabulary and invariants. Expected future
types include:

- source executable identity and version evidence;
- source command and option descriptions;
- generated catalog provenance;
- trusted policy rules and their provenance;
- rule-match results;
- policy decisions and reasons;
- execution plans and transformed argv; and
- output-handling intent and policy rejection.

Domain code performs no source probing, file access, process launch, terminal
rendering, or agent communication. It rejects an incomplete plan, ambiguous
rule result, unknown controlling decision, or inconsistent source identity.

The exact catalog and policy schemas are intentionally unresolved.

### Application

`internal/app/` owns user tasks and deterministic orchestration. Future use
cases may:

- request bounded source-CLI inspection through a port;
- validate and assemble a command catalog from observed facts;
- request decoding of a configured policy, then apply domain validation;
- match rules and construct a complete preview or execution plan;
- require the observed executable identity to agree with catalog evidence;
- authorize one controlled process attempt only after plan validation;
- coordinate output transformation without changing the invocation's meaning;
  and
- expose task-specific ports used by coding-agent integrations.

Application code decides the task sequence and fail-closed behavior. It does
not parse provider-specific help text, read configuration files directly,
launch processes, or render output.

### Infrastructure

`internal/infra/` owns concrete external I/O behind application ports:

- resolving and inspecting a source executable;
- invoking bounded help or metadata commands selected by an inspection policy;
- decoding source-specific command descriptions into observed facts;
- reading a chosen configuration syntax and preserving source provenance;
- persisting generated catalogs or trusted policy only if later approved;
- launching the exact executable with an argv vector and bounded environment,
  time, stdout, and stderr;
- adapting source structured-output facilities;
- performing byte-level output capture or transformation mechanisms; and
- integrating with a shell, PATH wrapper, coding-agent hook, or other host
  surface if one is later selected.

Infrastructure reports observations and typed failures; it does not decide
which commands an agent should see, whether an operation is allowed, or what a
policy rule means.

Direct network integrations are not selected. Source CLIs retain their own
remote and credential behavior.

### CLI

`internal/cli/` is the composition and presentation boundary for `atr`. It
will own public command registration, typed argument parsing, human and agent
presentation, and wiring application use cases to infrastructure adapters.

The current inherited `doctor` and `sample` surface is scaffold evidence,
not the final Atsura command design. Public Atsura command paths and schemas
must not be inferred from the conceptual names in this document.

## Future responsibility map

| Concern | Semantic owner | I/O or presentation owner | Current status |
|---|---|---|---|
| Source CLI investigation | Application defines the bounded task and required evidence | Infrastructure resolves and probes the process | Not implemented; source and exploration depth unknown |
| Command catalog | Domain owns catalog values and invariants; application assembles and validates | Infrastructure may persist; CLI may present | Not implemented; schema unknown |
| Policy parsing and validation | Domain owns semantics; application owns validation workflow | Infrastructure decodes the selected file format | Not implemented; YAML is not selected |
| Rule matching | Domain pure evaluation | Application supplies validated catalog, policy, and invocation | Not implemented |
| Execution planning | Domain owns plan invariants; application constructs the plan | CLI presents preview | Recommended first slice |
| Process execution | Application authorizes one planned attempt | Infrastructure launches bounded executable plus argv | Explicitly excluded from bootstrap |
| Output transformation | Application owns task meaning and degradation policy | Infrastructure captures/transforms; CLI renders | Not implemented; exact boundary needs a slice |
| Coding-agent integration | Application exposes task use cases and trust decisions | Infrastructure/CLI adapts the chosen hook or wrapper | Not implemented; mechanism unknown |

## Proposed first vertical boundary

The smallest next slice should stop before process execution:

```text
synthetic modeled source invocation
  -> validated minimal policy
  -> pure rule match
  -> typed execution-plan preview or policy rejection
  -> explanation rendered by atr
```

This slice can test deterministic planning and explainability without choosing a
real source CLI, recursive discovery, configuration file format, wrapper, hook,
or output-transform mechanism. The implementation still requires the
`$add-capability` workflow and an accepted work packet; this bootstrap does
not add it.

## Unresolved architecture decisions

- How executable identity is represented across symlinks, PATH changes,
  replacement, plugins, and version drift.
- Which observations form a portable command catalog.
- Whether source-specific inspectors are adapters or generated specifications.
- The policy representation and precedence model.
- The host integration boundary and confirmation interaction.
- Whether output transformation is streaming, buffered, delegated to source
  structured output, or composed from those approaches.
- Whether RTK is a dependency, peer integration, or unrelated implementation.
