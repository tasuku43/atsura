# Product Contract

This document records Atsura's current product vocabulary and boundaries. The
concepts are hypotheses for the first vertical slice. They do not yet define
stable public commands, flags, output schemas, configuration keys, or file
formats.

## Product statement

Atsura is a deterministic framework for tailoring existing CLIs to coding
agents. It discovers or consumes a model of a source CLI, applies a small
reviewed policy difference, and produces an inspectable decision about the CLI
surface or invocation. Routine enforcement does not require a language model.

The initial product focus is the agent-visible capability and behavior of a
CLI, not only shorter output.

## Primary user and outcome

The primary user is a maintainer who configures coding-agent environments and
wants an existing CLI to expose a purpose-appropriate, explainable surface.

The first recommended outcome is a no-execution preview: for one modeled source
invocation and one small trusted policy, the maintainer receives a deterministic
decision, the exact planned argv when applicable, and the reason that selected
the decision.

No Atsura-specific end-user command currently promises that outcome. The
inherited `doctor` and synthetic `sample` commands remain repository harness
examples while the first Atsura slice is designed; they are not evidence that
source-CLI tailoring exists.

## Working vocabulary

### Source CLI

The existing executable whose interface Atsura may tailor. Its own
authentication, authorization, domain behavior, and compatibility contract
remain authoritative. A source CLI is not merely a command name from `PATH`;
the relevant executable identity and version evidence must eventually be
explicit and verified.

Open questions include the first source CLI, plugin and alias treatment,
version detection, and whether a source exposes reliable structured metadata.

### Generated command catalog

A deterministic, provenance-bearing model of the source CLI surface observed
by an inspection process. It may eventually describe commands, options,
argument shapes, relevant output modes, and source evidence.

The catalog is generated evidence, not permission. Regeneration must not grant
an operation or erase a reviewed policy decision. Its schema, exploration
depth, persistence, and compatibility rules are not decided.

### Policy

A reviewed set of differences between the source catalog and an agent-specific
surface or invocation. Candidate policy dimensions include visibility,
allow/confirm/reject decisions, argument/default changes, structured-output
selection, output selection, and explanatory reasons.

The policy language and storage format are undecided. Policy must be bounded,
deterministically evaluated, and unable to execute arbitrary shell code by
default. Repository-provided policy and user-trusted policy require distinct
provenance and trust treatment.

### Tailored CLI surface

The commands, options, defaults, and behavioral choices intentionally exposed
to one agent role or purpose after applying policy to the source catalog.

The surface is a derived view, not a fork or reimplementation of the source
CLI. Whether the first product renders help, supplies a wrapper, rewrites hook
input, or uses another integration remains open.

### Execution plan

The complete typed decision produced before process execution. A future plan is
expected to identify the source executable evidence, source command, applied
rule and reason, decision class, transformed invocation when applicable, and
output handling intent.

A plan is inspectable product output and a security boundary. Missing,
ambiguous, stale, or inconsistent controlling facts invalidate it.

### Transformed invocation

The exact executable and argv vector derived from a valid plan. It is not a
shell fragment. Any added, removed, or changed argument must be attributable to
the source model and an applied policy rule.

No invocation transformation or process execution is implemented by this
bootstrap.

### Passthrough or raw execution

An explicit route that invokes the selected source CLI without applying Atsura
tailoring policy. It exists as a product hypothesis because users may need the
original interface.

It must never be an implicit fallback from policy rejection, policy parse
failure, stale catalog evidence, or transformation failure. The UI must make
the bypass and selected executable unambiguous. The integration mechanism and
exact raw contract remain undecided.

### Policy rejection

A pre-execution result stating that a controlling policy does not permit the
invocation or cannot be evaluated safely. It includes a stable reason and
causes zero source-process attempts. Rejection does not automatically offer raw
execution as recovery.

The exact allow, confirmation, and denial model remains open. Until that model
exists, an unevaluable controlling decision fails closed.

## Product boundaries

### Deterministic core versus coding agent

The deterministic core owns validated input decoding, rule matching, plan
construction, stale-evidence detection, and enforcement. A coding agent may
research a user's purpose or usage evidence and propose policy, but proposals
do not become active policy or authorization without a user-controlled trust
step.

### Source semantics

Atsura may narrow a surface or deliberately transform an invocation, but must
not claim that a transformed command means the same thing without reviewable
evidence. Source exit behavior and side effects are not inferred from labels.
Output optimization cannot authorize a different invocation or an automatic
retry.

### Compatibility

The project identity is now `Atsura`, binary `atr`, and Go module
`github.com/tasuku43/atsura`. No Atsura-specific command path, policy syntax,
catalog schema, persisted location, transformed-invocation rule, or hook
contract is stable yet.

The first vertical slice must name its compatibility boundary and executable
tests before it is described as supported.

## Deliberately unsupported in this bootstrap

- Source-CLI help exploration or command-catalog generation.
- A YAML or other policy schema.
- Source command wrapping or execution.
- Output transformation.
- Claude Code hooks or another agent integration.
- Usage-history storage or analysis.
- Agent-generated policy activation.
- RTK integration.
- Release or package distribution.
- Direct external API integrations and Atsura-owned credentials.

## Research boundary

Current behavior of RTK, Claude Code hooks, CLI wrappers, and policy tools must
be verified from primary sources in later work. This contract makes no claim
about their present capabilities or gaps.
