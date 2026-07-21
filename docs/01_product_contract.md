# Product Contract

This contract defines Atsura's current product vocabulary and intended user
experience. `atr plan preview` is executable today. ADR 0002 selects
`atr run` as the finite v0.1 release-quality boundary; it is supported only
after its implementation and all required gates land together.

## Product statement

Atsura tailors existing CLIs to coding agents. A user manages reviewed
per-command YAML, a coding-agent integration intercepts attempted commands, and
Atsura compiles each attempt into an inspectable execution plan. The same plan
logic drives no-side-effect preview and controlled wrapper execution.

Routine planning and enforcement are deterministic and do not require a
language model.

## Primary user outcome

A maintainer can give an agent a purpose-specific view of an existing CLI:

- irrelevant commands and options can be omitted from agent discovery;
- attempted operations can be allowed, confirmed, or rejected;
- accepted invocations can be rewritten deterministically;
- source-native structured output can be selected; and
- source output can be substantially transformed into a smaller,
  task-specific result.

The source CLI retains its own domain behavior, authentication, authorization,
and remote API implementation.

The current `doctor` and synthetic `sample` commands are inherited harness
examples. They are not evidence that this outcome is implemented.

## Conceptual flow

```text
user-approved per-command YAML
        +
attempted source command from an agent hook
        +
source executable and catalog evidence
        |
        v
deterministic execution plan
        |
        +--> preview: render only, no side effects
        |
        +--> execute: revalidate, then apply wrapper pipeline
                       before -> invoke -> output -> after
```

The execution and hook protocols remain undecided. The current preview path is:

```text
atr plan preview --config <path> -- <source-command> [args...]
```

It returns schema-1 JSON under `plan` and starts no source process. The selected
execution path is:

```text
atr run --config <path> -- <source-command> [args...]
```

Both paths load and compile the same explicitly selected file. Passing
`--config` trusts that exact file for the current invocation only; Atsura does
not discover, persist, inherit, or automatically activate policy.

## Working vocabulary

### Source CLI

The existing executable being tailored. Its resolved identity, observed
version, relevant plugins, and command model are evidence used to build a plan.
A command name found on `PATH` is not sufficient identity by itself.

### Generated command catalog

A deterministic, provenance-bearing model of the observed source CLI surface.
It may describe commands, options, argument shapes, source-native output modes,
and executable evidence.

The catalog is evidence, not permission. Regeneration cannot silently grant an
operation or erase a reviewed YAML rule.

### Per-command YAML

The user-facing configuration direction for tailoring one source command or
command family. YAML declares policy differences and processing actions rather
than executable shell text.

Schema 1 is deliberately narrow: one exact executable and argv prefix, required
`effect: read`, an `allow` or `deny` decision with a reason, appended argv, and
a typed JSON `select`/`rename`/`compact_json` output description. Create/write,
confirmation, matching inheritance, file locations, activation, and persistent
trust are unsupported.
Repository-provided YAML is not automatically user-trusted.

### Tailored CLI surface

The commands, options, defaults, decisions, and result shapes intentionally
exposed to one coding-agent purpose.

Hiding a command from discovery and rejecting an attempted invocation are
distinct guarantees. Execution interception can enforce rejection; hiding also
requires agent-facing help or command discovery to consume the tailored
surface.

### Execution plan

The complete typed result of compiling source evidence, trusted YAML, the
attempted invocation, and relevant environment facts.

A plan declares:

- source executable evidence;
- original and transformed argv;
- matched rules and reasons;
- decision and any confirmation requirement;
- ordered built-in processing actions;
- source invocation;
- output input format and transformation;
- agent-facing result shape; and
- tailored or raw mode.

A plan can be rendered for preview. Execution must revalidate its inputs rather
than treating an old preview as authority.

### Wrapper

The controlled runtime that applies a valid plan. The wrapper does not decide
policy independently. It executes the ordered stages and reports stage-specific
results or failures.

The v0.1 wrapper accepts only an allowed read plan. It resolves and fingerprints
one regular executable, revalidates it immediately before and after execution,
passes executable and argv directly without a shell, supplies EOF stdin,
inherits the caller's current directory and environment, and starts at most one
direct source process. Children created by the source CLI are source-owned
behavior and are not additional Atsura attempts.

### Invocation transformation

The exact change from the attempted executable and argv to the source
invocation. It is represented as executable plus argv, never as an
interpolated shell fragment. Every change is attributable to a YAML rule and
source evidence.

### Built-in processing action

A named, typed operation implemented and understood by Atsura. Its accepted
inputs, effect, output, and failure behavior are part of the plan contract.
Initial before, after, and output processing uses these actions instead of
arbitrary shell.

### Output pipeline

The stages that interpret and reshape source stdout for the coding agent:

```text
declared source output
  -> bounded parse
  -> select / map / rename / aggregate / order
  -> declared agent-facing render
```

`invoke` selects the source executable and argv. `output` transforms the
result. These responsibilities do not share a generic shell escape hatch.

The initial direction prefers source-native structured output and typed built-in
transformations. jq expressions, RTK, plugins, and generic external
transformers remain future decisions.

For v0.1, source stdout must be one JSON object or an array of JSON objects.
Every record must contain each selected field. Missing fields, duplicate object
keys at any depth, invalid JSON, non-object records, and excessive bytes or
complexity fail the transform. Null, empty string/array/object, zero, and false
remain distinct values. Output record fields follow YAML `select` order with
the declared renames.

Successful execution returns schema-1 JSON under `execution` with exactly:

- `decision`;
- `matched_command`;
- `reason`;
- `result_shape` (`object` or `array`);
- ordered `fields`;
- transformed `records`; and
- `source_process_attempts`, equal to one.

The execution envelope is complete and exhaustive for the exact bounded source
result captured by that invocation. It makes no claim that the source CLI
queried all provider history.

### Policy rejection

A pre-execution result stating that trusted YAML rejects the attempted command
or cannot be evaluated safely. It includes the matched rule or validation
reason and causes zero source-process attempts.

### Raw execution

An explicitly selected route that bypasses tailoring policy and invokes the
chosen source CLI. It is never selected automatically after rejection, stale
evidence, invalid YAML, or transform failure. Its exact user experience and
generic process bounds remain undecided.

## Output failure boundary

A source attempt and an output transform are separate facts. If transformation
fails after the source command ran, Atsura must not:

- repeat the source command;
- change its invocation;
- claim transformed success;
- expose raw output unless the reviewed contract explicitly permits it; or
- choose raw execution as recovery.

The v0.1 execution contract is:

- fixed timeout: 30 seconds;
- direct source-process attempts: at most one, with no Atsura retry;
- stdout limit: 4 MiB;
- stderr limit: 256 KiB;
- nonzero exit, timeout, cancellation, overflow, executable drift, and transform
  failure produce no success stdout and never expose raw source stdout;
- failed source stderr and private causes are not copied into public faults;
- bounded stderr from a successful source process is visibly escaped on Atsura
  stderr before the checked success write; and
- no failure selects raw execution, another transformer, changed argv, or a
  repeated source command.

## Deterministic core versus coding agent

A coding agent may propose YAML from a user's purpose or usage evidence. A
user-controlled workflow must trust the proposal before it can affect command
discovery or execution.

The deterministic core owns strict YAML decoding, rule matching, plan
construction, drift detection, confirmation requirements, invocation
transformation, and built-in output processing.

## Compatibility boundary

The stable project identity is `Atsura`, binary `atr`, and Go module
`github.com/tasuku43/atsura`.

The following are not yet stable:

- command paths and hook protocol;
- YAML schema beyond the v0.1 patch series and storage locations;
- catalog and plan schemas;
- built-in action vocabulary;
- output transformation contract;
- raw route; and
- source-CLI compatibility.

## Deliberately unsupported now

- A real source-CLI inspector or generated catalog.
- Policy activation, precedence, inheritance, or trusted repository loading.
- Hook installation or command interception.
- Create/write wrapper execution, confirmation, or mutation policy.
- Non-JSON, streaming, aggregate, map, filter, sort, or multi-source output transforms.
- Raw execution or automatic source-output fallback.
- Arbitrary shell, jq, external-transformer, plugin, or RTK execution.
- Usage-history collection and agent policy activation.
- Direct external API integrations.
- Release or package distribution.

Current specifications of RTK, Claude Code hooks, and comparable wrapper or
policy tools require later primary-source research.
