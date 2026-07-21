# Atsura Product Theses

This document is the first decision source when an Atsura design is ambiguous.
These are seed hypotheses: they are strong enough to choose the first vertical
slice, but they must change when implementation and user evidence contradict
them.

## North star

**A maintainer can turn an existing CLI into a smaller, purpose-specific
interface for coding agents without reimplementing the source CLI and without
placing a language model in the routine execution path.**

The primary users are maintainers of coding-agent environments who already rely
on capable CLIs but need each agent or task to see and exercise only the useful
part of those CLIs. Their problem is not merely verbose output. A source CLI may
expose too many commands and options, unsafe defaults, ambiguous behavior, and
more result data than the agent needs.

## Thesis 1: Tailoring changes capability, behavior, and presentation

For Atsura, tailoring a CLI is the reviewed application of a small policy
difference to a discovered source command model. Depending on later validated
product decisions, that difference may:

- hide commands or options from an agent-facing surface;
- classify an operation as allowed, confirmation-required, or rejected;
- change arguments or defaults deterministically;
- request a source CLI's own structured-output mode;
- select only the information required by the agent's task;
- explain the applied policy and its reason; and
- offer an explicit path that invokes the source CLI without applying the
  tailoring policy.

This list is a product hypothesis, not a public command set or configuration
schema. The first slice need not implement every dimension.

### Consequences

- Atsura models a source CLI rather than recreating its business behavior.
- An output-shortening feature alone does not validate the central hypothesis.
- A policy decision and its transformed invocation must be inspectable before
  execution can be considered trustworthy.

### Enforcement status

These consequences are aspirational until the first Atsura-specific domain
types and contract tests replace the inherited scaffold examples.

## Thesis 2: Routine execution is deterministic

The validated catalog, trusted policy, invocation, and source-binary identity
must produce the same execution decision without asking a language model.

A coding agent may study a user's purpose or prior work and propose a policy.
It may also explain trade-offs during configuration. The deterministic Atsura
core owns parsing validated inputs, matching rules, producing an execution
plan, and enforcing the selected decision. Agent output is a proposal until a
user-controlled trust step accepts it.

### Consequences

- Policy proposals and runtime policy enforcement are separate tasks.
- Routine execution cannot depend on model availability, prompt wording, or
  probabilistic classification.
- Every applied rule needs stable provenance and a reason suitable for preview
  and diagnostics.
- Configuration that embeds arbitrary shell code is outside the initial design.

### Mechanical enforcement target

The first policy-bearing slice must include repeatability fixtures proving that
identical validated inputs produce an identical plan and that rejected or
invalid policy makes zero source-process attempts.

## Thesis 3: Preserve the source CLI's meaning and safety boundary

Atsura must not silently broaden a source operation, invent support the source
CLI does not have, or treat presentation optimization as permission to change
the operation. Argument rewriting is valid only when the resulting invocation
has an explicit, reviewable meaning.

### Consequences

- Source executable identity and observed version or equivalent evidence are
  part of the catalog and plan context.
- A stale catalog or an unevaluable controlling rule fails closed instead of
  falling through to an unintended operation.
- Policy rejection happens before source-process execution.
- Failure to optimize output must not trigger a different command or an
  implicit second execution.
- Any raw or passthrough route is explicit, is never selected as recovery from
  policy failure, and makes clear that Atsura policy was bypassed.
- Source CLI authentication and authorization remain authoritative; Atsura
  does not claim that its own policy makes an upstream operation safe.

### Mechanical enforcement target

Future execution tests must prove exact argv construction without shell
interpretation, stale-binary rejection, zero attempts on policy failure, and no
automatic raw fallback.

## Thesis 4: Discover once, apply small reviewed differences

The working hypothesis is that a source CLI's command structure can be
discovered by a bounded deterministic program, represented as a generated
catalog, and tailored through a smaller policy than a hand-maintained wrapper
or reimplementation.

This hypothesis is not yet proven. Source CLIs differ in help behavior,
structured metadata, plugins, dynamic commands, aliases, environment-dependent
surfaces, and versioning. Atsura must test one narrow slice before generalizing
the discovery mechanism.

### Consequences

- Generated facts and reviewed policy facts remain distinguishable.
- Regeneration cannot silently grant a capability or weaken an existing
  decision.
- Catalog provenance and compatibility with the resolved source binary are
  product facts, not cache implementation details.
- Current specifications of comparable tools must be researched from primary
  sources before Atsura claims an overlapping capability is needed.

### Mechanical enforcement target

A later catalog slice must bind generated output to its source evidence, reject
unclassified drift, and keep generation deterministic. No such catalog is
implemented by this bootstrap.

## First hypothesis to test

The recommended first user result is:

**Before any source command runs, a maintainer can preview how one small,
trusted policy treats one modeled source invocation and receive a deterministic
decision, the exact planned argv when applicable, and the reason for the
decision.**

The first experiment should use a synthetic source executable or fixture. It
does not select the first supported real CLI, a policy file syntax, recursive
help discovery, or an integration mechanism. This slice tests whether Atsura's
policy vocabulary, plan boundary, and explanation are useful before process
execution and broad discovery create additional variables.

Success evidence should include:

- the same inputs produce the same preview bytes or equivalent typed plan;
- an allowed invocation remains semantically traceable to the modeled source
  command;
- a rejected or invalid decision makes zero source-process attempts; and
- a maintainer can identify which rule caused the result without inspecting
  implementation source.

## Current non-goals

- Reimplementing the source CLI or its remote APIs.
- Implementing source-help exploration, catalog generation, a policy language,
  command execution, output transformation, agent hooks, usage-history
  collection, agent-generated policy, RTK integration, or distribution during
  this bootstrap.
- Requiring a language model for normal command execution.
- Allowing arbitrary shell code as the default policy extension mechanism.
- Claiming universal compatibility with every CLI shape.
- Treating a coding agent's proposal as user authorization.
- Finalizing a stable public Atsura command or configuration contract before a
  vertical slice supplies evidence.

## Open questions

The following remain deliberately unresolved:

- Which source CLI should be tested first?
- Should the eventual configuration use YAML or another representation?
- How deeply should help or other command metadata be explored?
- How should Claude Code SessionStart and PreToolUse responsibilities differ?
- Should integration use a shell function, PATH wrapper, hook input rewrite, or
  another mechanism?
- What are the exact semantics of allow, confirm, and deny?
- How, if at all, should usage history be collected?
- Should Atsura use RTK internally, integrate with it, or remain independent?
- Which source-CLI structured-output facilities are reliable enough to use?
- What evidence establishes executable identity across path, replacement,
  plugins, and version changes?

These questions are inputs to primary-source research and the first vertical
slice, not bootstrap decisions.
