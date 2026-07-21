# Atsura Product Theses

These theses describe the intended Atsura experience and govern its first
release-quality vertical slice. They remain hypotheses tested through narrow
supported outcomes rather than claims that the complete vision already exists.

## North star

**Given an existing CLI and user-approved per-command YAML, Atsura gives a
coding agent a deterministic, inspectable CLI surface. Every attempted command
becomes either a reasoned rejection or a reviewable execution plan for the
exact source CLI, without reimplementing that CLI or requiring a language model
at runtime.**

The primary user is a maintainer of a coding-agent environment. That maintainer
wants to shape which parts of an existing CLI an agent can see, how accepted
commands behave, and how much output returns to the agent.

## Intended experience

```text
per-command YAML
  -> coding-agent hook intercepts an attempted command
  -> Atsura compiles source evidence + YAML + invocation into one plan
  -> preview renders the plan without side effects
     or
  -> execution revalidates and applies the plan through a wrapper
       -> built-in pre-processing
       -> exact source CLI invocation
       -> built-in post-processing and output transformation
```

An execution hook can reject a command that was attempted. Preventing the agent
from discovering a command in the first place also requires the eventual
agent-facing help or command-discovery integration to use the tailored surface.
Those are related outcomes, not the same mechanism.

## Thesis 1: The tailored surface is the product

YAML, hooks, catalogs, wrappers, and renderers are mechanisms. The user outcome
is a purpose-specific CLI surface for a coding agent.

That surface may:

- hide source commands or options from agent discovery;
- allow, require confirmation for, or reject an attempted operation;
- add, remove, or replace arguments and defaults;
- select a source CLI's structured-output mode;
- apply bounded processing before or after the source process;
- substantially reshape output rather than merely shorten it;
- explain every applied rule and transformation; and
- provide a separately selected raw route to the source CLI.

The initial product does not need every dimension, but a feature that only
shortens text does not validate the central thesis.

### Consequences

- Public tasks describe tailored user outcomes, not YAML mechanics.
- A hidden capability and a rejected invocation are represented distinctly.
- Raw execution is outside the tailored guarantee and is never an automatic
  fallback.

## Thesis 2: YAML is reviewed configuration, not executable code

The working product direction is per-command YAML. It declares differences
from an observed source CLI model and the processing required for that command.

The initial configuration uses typed Atsura actions whose meaning, inputs,
effects, and failure behavior are known to the deterministic core. It does not
embed arbitrary shell code.

### Consequences

- The exact YAML schema and file locations remain versioned product decisions.
- Repository-provided YAML and user-trusted YAML have distinct provenance.
- Unknown fields, unsupported actions, ambiguous matches, and invalid rule
  combinations fail before source execution.
- jq expressions, RTK invocation, generic external transformers, plugins, and
  shell scripts require later explicit trust and dependency decisions.

### Mechanical enforcement target

The first YAML slice must use bounded strict decoding, reject unknown
configuration, and prove that invalid or untrusted policy makes zero
source-process attempts.

## Thesis 3: One plan drives preview and execution

An execution plan is the typed result of compiling validated source evidence,
trusted YAML, the attempted invocation, and relevant environment facts.

A complete plan identifies at least:

- the source executable evidence;
- original and transformed argv;
- the matched rules and reasons;
- the allow, confirm, or reject decision;
- built-in pre-processing;
- source invocation;
- post-processing and output transformation; and
- raw-versus-tailored mode.

Preview and execution do not implement separate policy logic. Preview renders
the plan without side effects. Execution builds or revalidates the same plan
immediately before applying it. A previously displayed plan is not executable
authority when the YAML, source binary, catalog, or relevant environment has
changed.

### Mechanical enforcement target

Identical validated inputs must produce an identical typed plan. Preview and
execution tests must prove equivalent decisions and transformations, while
execution additionally proves immediate identity and configuration
revalidation.

## Thesis 4: Invocation and output transformation are separate stages

`invoke` determines which source executable and argv run. `output`
determines how successful source output is parsed, selected, aggregated,
renamed, and rendered for the agent.

Output transformation is a first-class product capability. It may replace the
source presentation with a substantially different compact structure, provided
the plan declares the transformation and the result does not invent facts.

The preferred first path is:

1. request a source CLI's structured output when reliably available;
2. parse through a bounded declared input format;
3. apply typed built-in selection, mapping, aggregation, and ordering actions;
4. render a declared agent-facing shape.

### Consequences

- Output processing is not hidden inside a generic pre/post shell command.
- Transform failure does not change argv, run another source command, retry the
  source command, or silently expose raw output.
- Source exit behavior and transform failure remain distinguishable.
- RTK-equivalent breadth is a research target, not a current compatibility
  claim.

### Mechanical enforcement target

Output fixtures must cover substantial reshaping, hostile source data, bounded
parsing, exact declared result shape, and transform failure after exactly one
source attempt.

## Thesis 5: Agents propose; the deterministic core enforces

A coding agent may study a user's purpose or usage evidence and propose YAML.
The proposal does not become trusted policy or authorization until a
user-controlled workflow accepts it.

Runtime rule matching, plan construction, confirmation requirements,
invocation rewriting, and output transformation are deterministic.

### Consequences

- Routine execution does not depend on model availability or prompt wording.
- Every decision and transformation is attributable to trusted YAML and source
  evidence.
- Source CLI authentication and authorization remain authoritative.
- Source binary or catalog drift invalidates a controlled plan instead of
  silently inheriting prior permission.

## Thesis 6: Release quality closes one supported outcome

Release quality is not the number of planned mechanisms. A supported outcome
is release-quality when a user can discover it, execute it within finite
bounds, interpret its complete machine result without an undeclared parser,
recover from every declared failure, and install the same reviewed artifacts
the project tested.

Every supported outcome therefore declares its inputs, effects, trust
selection, destinations, attempts, timeout, byte and complexity limits,
success shape, failures, compatibility, and explicit exclusions. Tests prove
zero side effects before authorization and the exact attempt count after the
controlled boundary. Full, security, public, and release gates must pass on one
committed tree before that tree is called release-quality.

### Current v0.1 boundary

The first release-quality outcome is deliberately read-only:

**A maintainer explicitly selects one reviewed schema-1 YAML file, runs one
JSON-producing source command through `atr run`, and receives the plan-declared
selected and renamed records after at most one direct no-shell source-process
attempt.**

Schema 1 requires `effect: read`. Atsura does not infer that effect from a
command name; the maintainer reviews and asserts it. Create/write, confirmation,
hooks, implicit policy activation, and raw execution require later outcomes
with their own enforceable contracts.

## First hypothesis and evidence sequence

The first vertical slice should provide this result:

**A maintainer can supply a small per-command YAML file and preview the
deterministic plan for one synthetic source invocation, including the decision,
exact argv, built-in output transformation, matched rules, and reasons, without
starting the source process.**

`atr plan preview` is the first executable test of this hypothesis. Its
schema-1 vocabulary deliberately supports only one exact executable and argv
prefix, an explicit read effect, allow or deny, appended argv, and a typed JSON
select/rename/compact render description.

Success evidence includes:

- identical inputs produce identical plan output;
- invalid or untrusted YAML produces no plan and no source attempt;
- invocation and output stages remain separate and inspectable;
- a nontrivial output reshape is fully described by typed built-in actions; and
- a maintainer can identify every change without reading implementation source.

`atr run` is the next evidence step. It must compile the same policy and
invocation, reject any non-read or deny plan before execution, resolve and
revalidate one source executable, perform at most one direct attempt, and apply
the declared transform only to bounded successful JSON. Nonzero exit, timeout,
cancellation, drift, overflow, or transform failure produces no raw success and
no automatic retry.

## Current non-goals

- Reimplementing a source CLI or its remote APIs.
- Hook installation, command interception, and command-surface hiding.
- Create/write source operations, confirmation, and mutation replay policy.
- Raw execution or automatic intact-output fallback.
- Requiring a language model for routine execution.
- Allowing arbitrary shell as the initial pre/post or output mechanism.
- Claiming RTK compatibility before primary-source research.
- Treating an agent proposal as user authorization.
- Treating the experimental preview command or schema as stable.
- Publishing or releasing Atsura.

## Open questions

- Which vendor source CLI should be supported first beyond the generic local
  JSON-producing read boundary?
- How should schema 1 evolve beyond explicit per-invocation file selection?
- How deeply should help or other command metadata be explored?
- How should Claude Code SessionStart and PreToolUse responsibilities differ?
- How should the tailored surface participate in agent command discovery?
- Should host integration use a shell function, PATH wrapper, hook input
  rewrite, or another mechanism?
- What are the exact semantics of confirm and mutation denial beyond v0.1's
  read-only allow/deny boundary?
- Which built-in pre-processing, post-processing, and output actions should
  follow v0.1 select/rename?
- When, if ever, should jq, RTK, external transformers, plugins, or scripts be
  admitted?
- When should behavior differ from v0.1's fail-closed transform, stderr, and
  source-failure contract?
- How, if at all, should usage history be collected?
- What stronger evidence should supplement v0.1 path resolution and SHA-256
  revalidation for plugins, interpreters, children, and version claims?

These questions require vertical-slice evidence or primary-source research.
