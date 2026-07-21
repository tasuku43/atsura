# Atsura Product Theses

These theses define Atsura's product direction. They are working hypotheses,
but they govern implementation until evidence justifies another revision. ADR
0005 corrects the authorization-centered interpretation introduced by ADR
0004 while retaining its vendor-neutral, compiled-bundle architecture.

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
  -> compile one wrapper execution plan
  -> preview the plan and its digest without starting the source
     or
  -> future: apply typed stages around one identity-bound source invocation
```

Source-CLI inspectors and coding-agent hosts are adapters. They can extend
tested compatibility or translate a host protocol, but cannot create a second
surface model, add wrapper semantics, or turn Atsura into an authorization
engine.

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
- A host may encode surface absence as its protocol's `deny` response, but that
  is transport mapping rather than a core authorization decision.
- Source CLI, operating system, credential, and remote-provider authorization
  remain authoritative.

### Mechanical enforcement target

The specification model must represent `inherit` or `exclude` as an explicit
surface default, represent command membership separately from wrapper behavior,
and return no execution plan for a command absent from the compiled surface.

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

The implemented schema may support these dimensions incrementally. Unsupported
actions remain explicit unknowns rather than generic strings or embedded code.
Arbitrary shell, arbitrary scripts, external transformers, and a runtime
language model are not part of the initial specification.

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
- original and transformed argv;
- the exact applied specification entry, or explicit `null` when the surface
  entry was inherited, plus its reason;
- before actions;
- the exact source invocation;
- declared source-output input format;
- output transformation;
- after actions; and
- tailored mode and finite source-process bounds.

For an included command, successfully constructing the complete plan proves
that the wrapper pipeline is fully described; it does not claim that current
Atsura can execute it. For a command absent from the surface, plan construction
returns a surface-resolution failure. Future execution must reuse this plan
logic; an old preview is never runtime authority after bundle or source drift.

### Mechanical enforcement target

Identical validated inputs produce identical plans and the same canonical plan
digest. Resolution chooses the longest matching command path from the complete
catalog before checking command and option membership in the tailored surface.
When a matched command has cataloged descendants, unresolved non-dash data is
not guessed to be a child or a positional; an explicit `--` must disambiguate
positional intent.
Preview reports `source_process_attempts: 0`. Future execution must revalidate
bundle and source identity, reuse this constructor, and start at most the
number of source attempts declared by the wrapper contract.

## Thesis 5: The source CLI owns source-operation meaning and authorization

Atsura does not infer remote effects, safety, or authorization from command
names, help prose, or a maintainer-supplied read/write label. The source CLI
owns its domain semantics, authentication, authorization, destinations,
prompts, and downstream side effects.

Atsura still owns the safety of its own behavior. Starting an identity-bound
source process is declared honestly as source-owned execution, not disguised
as an Atsura read. Atsura-owned local mutations—such as bundle trust-store
writes and integration installation or removal—continue to require explicit
effect, target, impact, central mutation invocation, and uncertain-outcome
handling.

### Mechanical enforcement target

`operation.EffectExecute` represents starting a source-owned process and cannot
carry an Atsura mutation target or impact. `EffectCreate` and `EffectWrite`
retain the existing mutation contracts for Atsura-owned state. Unknown effects
remain non-executable.

## Thesis 6: Output transformation is a first-class wrapper stage

Invocation transformation chooses the exact source executable and argv. Output
transformation interprets successful source output and produces the declared
agent-facing result. They are separate stages and share no generic shell escape
hatch.

The preferred path is to request source-native structured output when the
adapter can verify it, parse it within declared bounds, apply typed built-ins,
and render a task-specific structure without inventing facts.

Current preview verifies one active cataloged selector for the planned input
format before `--`; it does not yet prove that the selector value encodes the
requested select/rename fields for a running source adapter.

Transform failure never changes argv, retries the source process, selects raw
mode, or silently exposes unreviewed raw output. RTK-equivalent breadth remains
a research target rather than a present compatibility claim.

## Thesis 7: Agents propose; the deterministic core compiles

A coding agent may propose a tailoring specification from a role, purpose, or
usage evidence. A user-controlled workflow adopts the exact compiled result.
Runtime surface resolution, plan construction, argv transformation, and output
processing are deterministic and attributable to the bundle.

Host transports do not define core semantics. A Claude Code adapter may need
to emit `allow`, `ask`, or `deny`, but it translates core states such as
`rewrite`, `not_managed`, `command_not_in_surface`, `invalid_invocation`, or
`interaction_required`. It does not decide source-operation permission.

## Thesis 8: Bundle trust adopts a surface and wrapper set

One canonical bundle binds source identity, adapter contract, catalog evidence,
normalized tailoring specification, compiled surface, and wrapper behavior.
Its digest is its identity. Trust is a user-local decision to adopt that exact
purpose-specific CLI, not a grant of permission to source operations.

A trust summary therefore describes included and excluded surface entries,
option changes, identity and transforming wrappers, argv changes, processing
stages, output transformations, source identity, and bundle digest. It does not
count source permissions, decisions, or inferred effects.

Raw execution is a separate, explicit tailoring bypass. It revalidates the
bundle-bound source identity but applies no surface selection, argv transform,
or output transform. Raw is never automatic fallback, a recovery suggestion,
or part of the tailored agent surface.

## Release-quality hypothesis

Release quality closes one supported maintainer result rather than maximizing
mechanism count. A result is supported only when it is discoverable, bounded,
machine-interpretable without undeclared reconstruction, recoverable through
declared faults, and verified against the same artifacts users install.

The current minimal evidence slice is:

**A maintainer can create a catalog-bound specification with an explicit
surface default, include one verified command with either an identity or typed
transforming wrapper, build and adopt the exact bundle, and preview the
resolved wrapper pipeline without starting the source process.**

This slice tests the corrected vocabulary before source runtime or host
integration resumes.

## Current non-goals

- Deciding whether a user may perform a source operation.
- Replacing source CLI, OS, credential, or remote-provider authorization.
- Claiming that hidden commands are sandboxed or impossible to invoke elsewhere.
- Reimplementing source CLI domain semantics or remote APIs.
- Arbitrary shell, scripts, jq, RTK, plugins, or external transformers in the
  initial specification.
- Automatic raw or intact-output fallback.
- Requiring a language model for routine execution.
- Implementing source refresh, bundle runtime execution, raw, or host adapters
  during the zero-execution preview milestone.
- Publishing or releasing Atsura.

## Open questions

- Which argv replacement/default actions and typed before/after actions form
  the first finite wrapper vocabulary?
- How should an agent-facing option surface represent positional arguments and
  mutually dependent source options?
- How should the catalog and plan grammar model short options, root/global
  options, and command-specific positional arguments without guessing?
- Should invocation transforms be allowed to append option-looking arguments
  after an existing `--`, where the source will interpret them as positional?
- Which source adapters can prove the structured-output selector encoding used
  by a wrapper rather than merely record it as a specification value?
- Which source and host adapters should follow the first compatibility fixtures?
- What stronger executable identity mechanism can close the remaining
  check-to-exec race on each supported platform?
- When, if ever, should jq, RTK, external transformers, plugins, or scripts be
  admitted?
- How, if at all, should usage evidence be collected without storing secrets or
  raw confidential output?

These questions require a vertical slice or primary-source research. They are
not authorization questions to be answered by adding allow/deny fields.
