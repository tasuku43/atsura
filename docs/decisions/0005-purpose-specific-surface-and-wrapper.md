# ADR 0005: Compile a purpose-specific surface and wrapper pipeline

- Status: Accepted
- Date: 2026-07-21
- Deciders: Repository maintainer and product owner
- Scope: Product semantics, tailoring schema, bundle, plan, operation effects, trust, host adapters, migration, and release quality
- Supersedes: docs/decisions/0002-v0.1-local-run-boundary.md and docs/decisions/0004-v1-compiled-tailoring-bundle.md
- Superseded by: None

## Context

ADR 0004 correctly selected one deterministic bundle shared by gateways and
host adapters, but it modeled source commands through allow/confirm/deny,
read/create/write, target, and impact. That pulled Atsura toward an
authorization engine.

That model is not honest for an arbitrary source CLI. Help and command names do
not prove remote effect or safety, and Atsura does not reimplement source
semantics. More importantly, excluding a command from a purpose-specific CLI
is surface composition, not permission denial.

The product owner corrected the north star: Atsura compiles an existing CLI
into a purpose-specific command and option surface plus deterministic wrapper
pipelines. Source CLI, OS, credentials, and remote provider remain the
authorities for source-operation authentication, authorization, and semantics.

## Decision drivers

- Represent what exists in a tailored CLI without claiming sandboxing.
- Keep surface membership independent from wrapper behavior.
- Preserve deterministic catalog, canonical bundle, exact-digest adoption,
  vendor-neutral adapters, bounded I/O, and typed transformations.
- Describe source-process launch honestly without inventing a remote effect.
- Keep the stronger mutation boundary for state Atsura itself owns.
- Avoid silently changing the meaning of already emitted schemas and receipts.

## Decision

### Product semantics

Atsura is a purpose-specific CLI surface and wrapper compiler. It does not
decide whether a user is authorized to perform a source operation.

An excluded command is absent from the compiled surface. Surface resolution
uses `command_not_in_surface` vocabulary and constructs no execution plan.
Absence is not `deny`, `policy_rejected`, or `permission_denied`.

### Tailoring specification schema 3

The authorization-oriented policy schema 2 is retired. Its fields are not
reinterpreted.

The replacement is a catalog-bound tailoring specification with:

- an explicit command-surface default: `inherit` or `exclude`;
- exact command entries whose presence is `include` or `exclude`;
- an independent option-surface default plus exact include/exclude overrides;
- an included command's explicit `identity` or `transform` wrapper;
- a reason for every explicit command entry;
- finite typed invocation and output transformations; and
- explicit before and after stage collections, initially empty until a finite
  built-in action is accepted.

`inherit` includes otherwise unlisted verified built-in catalog commands with
identity wrappers and inherited options. `exclude` omits otherwise unlisted
commands. Catalog entries without verified built-in provenance remain evidence
but are not inherited into the managed surface.

An excluded entry has no wrapper or option surface. An included entry has both.
An identity wrapper contains no transformations. A transform wrapper contains
at least one supported transformation. Membership never follows implicitly
from wrapper content.

### Bundle schema 2

The canonical bundle architecture remains. Bundle schema 2 binds:

- exact source executable identity and observed version;
- source-adapter kind and contract version;
- the provenance-bearing catalog and its digest;
- the normalized schema-3 tailoring specification and its digest; and
- the derived purpose-specific surface and wrapper definitions.

Canonical bytes exclude ambient and secret data. Digest identity, strict
loading, source drift detection, and vendor-neutral conformance remain.

### Wrapper execution plan

A future plan describes one resolved wrapper pipeline, not authorization. It
contains the matched tailored command, bundle/source identity, original and
transformed argv, before actions, exact source invocation, output input and
transformation, after actions, applied specification and reason, and
tailored/raw mode.

An included command with a complete plan is applicable. A command outside the
surface has no plan. Confirmation is not a universal source permission state;
if evidence later requires interaction, it must be a typed wrapper stage or a
host interaction request with its own contract.

### Source-owned execution and Atsura-owned mutation

`operation.EffectExecute` means that Atsura starts an identity-bound,
caller-selected source process whose downstream semantics are source-owned. It
does not mean read, safe, allowed, or idempotent. Execute carries no Atsura
mutation target or impact and is never valid as a mutation reconciliation
action.

`EffectCreate` and `EffectWrite` remain reserved for mutations Atsura owns,
including bundle trust-store changes and integration configuration. Those
commands retain exact target binding, impact, central mutation invocation,
complete-output handling, and uncertain-outcome rules. Unknown effects remain
invalid.

### Bundle trust is adoption

`bundle trust` records that a user adopted the exact source identity, surface,
wrapper set, and bundle digest as a purpose-specific CLI. It grants no source
permission.

The terminal summary shows surface default, included/excluded command changes,
option overrides, identity/transform wrapper counts, argv transformations,
processing stages, output transformations, source identity, and digest. It
does not show source effect or allow/confirm/deny counts.

### Raw and host adapters

Raw is a tailoring bypass. It revalidates bundle-bound source identity but
applies no surface selection or wrapper transformation. It is explicit,
manual, absent from the tailored surface, never an automatic fallback, and
never a recovery suggestion.

A host adapter translates core states into host protocol vocabulary. For
example, Claude Code may require an `allow`, `ask`, or `deny` transport value,
but its inputs are core states such as `rewrite`, `not_managed`,
`command_not_in_surface`, `invalid_invocation`, `interaction_required`, or
`wrapper_plan_available`. The transport does not create source authorization
semantics.

### Migration and implementation pause

Policy schemas 1 and 2, bundle schema 1, and authorization-centered plan output
are retired. Readers return an explicit migration diagnostic; they do not
silently convert, activate, or trust old content. No old trust receipt applies
to a schema-2 bundle because its digest and semantics differ.

An automatic converter is not selected. In particular, hidden/deny and
confirm/create/write rules cannot be mapped without inventing surface or source
meaning. A maintainer creates and reviews a new schema-3 specification.

Source refresh, bundle runtime execution, raw, and host integration remain
paused until the schema-3 surface and wrapper model passes its gates.

## Consequences

### Positive

- The product vocabulary matches the maintainer's purpose-specific CLI task.
- Hiding no longer masquerades as authorization or security isolation.
- Source CLI remains authoritative for its own semantics and access control.
- Identity wrappers and transforming wrappers coexist without changing surface
  membership semantics.
- Atsura-owned mutations retain their established safety boundary.
- Vendor-neutral bundle and adapter architecture survives the correction.

### Negative

- Existing experimental specifications and bundles require deliberate
  recreation and adoption.
- The previous preview/run slice is retired before its replacement runtime is
  implemented.
- Surface and option composition add schema and validation complexity.
- `EffectExecute` requires catalog and invariant audits that cannot treat every
  non-read command as a mutation.

## Mechanical enforcement

- Domain tests reject authorization fields in schema 3 and validate independent
  presence, option surface, wrapper kind, pipeline stages, and derived surface.
- Canonical and codec tests reject schemas 1 and 2 with stable migration
  diagnostics and require bundle schema 2.
- Operation and catalog tests treat Execute as a finite non-mutation effect,
  reject target/impact on Execute, and keep mutation recovery read-only.
- Whole-repository contract tests ensure public source-wrapper catalog entries
  contain no permission decision or inferred source effect.
- Trust-summary tests assert surface/wrapper counts and prohibit permission or
  source-effect counts.
- Future host fixtures must test transport mapping separately from core state.
- Focused, full, and security profiles decide completion of this correction.

## Reconsideration signals

Create a successor ADR before adding a universal source permission taxonomy,
claiming sandbox isolation, auto-converting retired authorization rules,
allowing arbitrary executable specification code, weakening exact-digest
adoption, or treating a host's protocol decision as core product semantics.
