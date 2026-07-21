# Product Contract

This contract defines Atsura's current vocabulary and supported product
boundaries. ADR 0005 supersedes the authorization-centered source-wrapper
semantics of ADR 0004 while retaining one canonical vendor-neutral bundle.

## Product statement

Atsura deterministically compiles an existing CLI and a reviewed tailoring
specification into a purpose-specific wrapper surface for coding agents. The
surface defines which commands and options exist and how included commands
invoke and transform the source CLI. The source CLI remains authoritative for
operation semantics, authentication, authorization, and remote effects.

Routine compilation and future execution require no language model.

## Primary user outcome

A maintainer can create and adopt a purpose-specific view of an existing CLI:

- irrelevant commands and options are absent from that surface;
- included commands can preserve source behavior through an identity wrapper;
- included commands can use a deterministic transforming wrapper;
- source-native structured output can be selected and reshaped with typed
  stages; and
- every explicit surface or wrapper change has an inspectable reason.

Atsura does not decide whether the maintainer or agent is permitted to perform
the downstream source operation. It does not make an excluded command
impossible to invoke through another route.

## Conceptual flow

```text
bounded source-inspector adapter -> generated catalog evidence
catalog + reviewed tailoring specification schema 3
        |
        v
canonical bundle schema 2 + exact-digest user adoption
        |
        +--> compiled command and option surface
        +--> identity or transforming wrapper per included command
        |
attempted source invocation
        |
        v
surface resolution
        +--> command absent: command_not_in_surface, no plan
        +--> command present: deterministic wrapper execution plan
```

Bundle-backed plan preview and source execution are paused during the schema
correction. The next vertical slice will implement pure surface resolution and
identity-wrapper preview before runtime execution resumes.

## Working vocabulary

### Source CLI

The existing executable being tailored. Its exact resolved identity, observed
version, relevant extensions, and command model are evidence. A PATH name,
command label, or help paragraph is not sufficient identity or a statement of
safety.

The source CLI owns its authentication, authorization, prompts, destinations,
remote effects, and domain semantics.

### Generated command catalog

A deterministic, provenance-bearing model of the observed source CLI. It may
describe command paths, options, argument evidence, source-native output modes,
adapter kind/version, probe facts, and executable identity.

Catalog evidence is neither a permission list nor a security sandbox. Entries
are classified as:

- `verified_builtin`: structure accepted by the named adapter contract;
- `observed_extension`: an extension observed but not covered by that built-in
  contract; or
- `unverified_dynamic`: structure the adapter could not validate.

Only verified built-ins are eligible for the currently compiled managed
surface. Other entries remain catalog evidence and are not described as denied.

### Tailoring specification

The strict catalog-bound YAML document that describes one purpose-specific
surface and wrapper set. The current schema version is 3.

Its initial normalized model is:

```yaml
schema_version: 3
catalog_digest: <sha256>
surface:
  default: exclude
commands:
  - command: [item, list]
    presence: include
    reason: Include compact item inventory for this purpose.
    options:
      default: inherit
      include: []
      exclude: []
    wrapper:
      kind: transform
      before: []
      invoke:
        append_args: ["--json=id,name"]
      output:
        input: json
        select: [id, name]
        rename: []
        render: compact_json
      after: []
```

The exact implemented constraints are:

- `surface.default` is required and is `inherit` or `exclude`;
- commands are exact, sorted, unique, verified catalog paths;
- `presence` is `include` or `exclude`;
- every explicit command entry has a bounded reason;
- an excluded entry has no options or wrapper;
- an included entry has an explicit option surface and wrapper;
- option default is `inherit` or `exclude`;
- under option `inherit`, only exact exclusions may be listed;
- under option `exclude`, only exact inclusions may be listed;
- option overrides are sorted, unique, disjoint, and catalog-observed;
- wrapper kind is `identity` or `transform`;
- an identity wrapper has empty before/invoke/after changes and no output
  transform;
- a transform wrapper contains at least one supported transform;
- before and after lists are explicit and currently must be empty because no
  built-in actions have yet been selected; and
- arbitrary shell, script, jq, plugin, RTK, or runtime-LLM actions are invalid.

Schema 3 contains no source allow/confirm/deny, source read/create/write,
authorization target, or authorization impact fields.

### Surface default and command membership

When `surface.default` is `inherit`, every otherwise unlisted verified built-in
catalog command is included with inherited options and an identity wrapper.
When it is `exclude`, every otherwise unlisted command is absent.

An explicit `include` entry adds or customizes one command. An explicit
`exclude` entry removes one command. These facts are surface composition, not
permission decisions.

### Option surface

The options presented for one included command. Option membership is separate
from command membership and wrapper behavior. The initial schema uses one
explicit inherit/exclude default plus exact include/exclude overrides. It does
not yet model every positional or cross-option dependency a source CLI may
have; those remain catalog evidence and future specification work.

### Identity wrapper

An included command whose source invocation and output are preserved by the
tailoring specification. Source identity validation and generic bounded
process framing still apply when runtime execution is implemented.

### Transforming wrapper

An included command with a typed deterministic pipeline. The initial accepted
transform vocabulary retains exact argv additions and structured JSON
select/rename/compact rendering. Removal, replacement, defaults, and typed
before/after actions remain planned vocabulary until implemented and tested.

### Wrapper execution plan

The future complete typed result of resolving an included command against one
adopted bundle and attempted invocation. A plan will contain:

- matched tailored command and surface binding;
- bundle, specification, catalog, and source identity;
- original and transformed argv;
- wrapper kind and ordered before/invoke/output/after stages;
- applied specification entry and reason;
- tailored or raw mode; and
- declared source-process attempt count.

Plan existence means the included wrapper pipeline is complete and applicable.
The plan does not contain a universal permission decision, source effect,
authorization target/impact, or confirmation requirement. A command absent
from the surface produces no plan.

### Raw execution

A future explicit tailoring bypass. Raw uses the bundle-bound source identity
but does not apply surface selection, argv changes, output transformation, or
other wrapper stages. It is never automatic fallback, a recovery action, a
host-generated rewrite, or a member of the tailored surface.

### Compiled tailoring bundle

Bundle schema 2 is one canonical JSON document binding:

- exact resolved source identity and observed version;
- source-adapter kind and contract version;
- normalized catalog and catalog digest;
- normalized schema-3 specification and specification digest; and
- the derived purpose-specific surface with its wrapper definitions.

The semantic content excludes time, machine, user, credential, random, and
captured source-output fields. Canonical bytes determine its SHA-256 identity.
Changed source, catalog, specification, surface, or bundle content never
inherits adoption automatically.

### Bundle trust and adoption

`bundle trust` remains the public name of the interactive local action, but its
meaning is adoption of one exact purpose-specific CLI bundle. It is not a grant
of source-operation permission.

The controlling-terminal summary identifies:

- bundle, catalog, and specification digests;
- exact source path, hash, and observed version;
- surface default;
- included and explicitly excluded command counts;
- identity and transforming wrapper counts;
- option override, argv transformation, before/after action, and output
  transformation counts.

It contains no source read/create/write or allow/confirm/deny counts. The
user-local receipt stores only the exact bundle digest. Repository content and
redirected stdin cannot create a receipt.

### Source-owned execution effect

`operation.EffectExecute` means an Atsura command starts an identity-bound
source process. It is a statement about the local boundary, not the downstream
source operation. Execute carries no Atsura mutation target or impact and does
not imply read-only, safe, authorized, idempotent, or retryable.

Any post-start unclassified outcome must be treated as non-retryable because
the source CLI may already have performed an operation. Exact identity,
separate argv, no shell, finite attempts, time, and bytes remain required.

Source inspection uses Execute because it starts the selected source executable
with adapter-owned fixed metadata argv. Preview, validation, build, and status
remain read effects because they do not start a caller-selected source task.

### Atsura-owned mutation

Create/write effects remain for state Atsura owns: bundle trust receipts,
future integration settings, and future Atsura configuration persistence.
These commands retain exact intent, target, impact, central mutation invocation,
complete-output handling, and non-retryable uncertain-outcome rules.

### Host adapter

A host adapter maps untrusted host messages to shared surface and wrapper
states. It may encode `command_not_in_surface` as the host protocol's `deny`, or
an interaction stage as `ask`, but those are transport values. Shared domain
types do not expose host allow/ask/deny vocabulary.

Hiding through a host changes routine discovery and invocation. It does not
replace OS, source CLI, credential, or provider authorization.

## Current public artifact workflow

The schema-correction milestone supports these artifact outcomes:

```text
atr source inspect --adapter github-cli --executable <path-or-name>
atr spec init --catalog <catalog.json> -- <source-command-path>
atr spec validate --catalog <catalog.json> --spec <spec.yaml>
atr bundle build --catalog <catalog.json> --spec <spec.yaml>
atr bundle status --bundle <bundle.json>
atr bundle trust --bundle <bundle.json>
```

`spec init` emits an exclude-by-default specification containing one included
verified command with inherited options and an identity wrapper. It does not
infer source safety or create adoption. Validation and build are read-only;
redirection of stdout is caller-selected filesystem behavior.

`bundle status` recomputes all canonical bindings, observes exact-digest
adoption, and compares the current source path/hash/size without starting the
source. `bundle trust` is the only Atsura-owned mutation in this workflow.

Source refresh, bundle plan/runtime, raw, and host-adapter commands are not
implemented in this correction.

## Migration contract

Authorization policy schemas 1 and 2, bundle schema 1, and their plan/run
semantics are retired experimental formats.

- Readers never interpret schema 1 or 2 as specification schema 3.
- Old bundle bytes never validate as bundle schema 2.
- Old trust receipts remain inert exact digests and are not copied or removed
  automatically.
- No automatic converter is selected because deny/hidden/confirm/effect/target
  cannot be mapped to surface membership and wrapper behavior without
  inventing user intent.
- Deprecated `policy init`, `policy validate`, `plan preview --config`, and
  `run --config` invocations return a stable migration diagnostic and start no
  source process.
- Recovery points to exact `spec` help. It never selects raw or silently builds
  or trusts replacement content.

## Output failure boundary

Source execution and output transformation are separate facts. When future
runtime resumes, transform failure after a source attempt must not retry or
change the invocation, claim transformed success, expose raw output without an
explicit output contract, or select raw mode.

The existing no-shell process adapter retains fixed identity, timeout, stdout,
stderr, and attempt bounds as implementation evidence. It is not a current
public bundle-runtime compatibility claim.

## Compatibility boundary

The stable project identity is `Atsura`, binary `atr`, and Go module
`github.com/tasuku43/atsura`.

Shared catalog, specification, bundle, surface, and future plan schemas contain
no GitHub- or Claude-specific fields. GitHub CLI 2.x remains the first source
inspection adapter. Claude Code remains a planned host adapter, not the core
product model. Real compatibility is limited to maintained adapter fixtures and
version ranges.

## Deliberately unsupported now

- Source refresh and catalog persistence.
- Bundle-backed plan preview or source execution.
- Raw execution.
- Hook installation or interception.
- Arbitrary shell, jq, scripts, plugins, RTK, or external transformers.
- Non-JSON, streaming, aggregate, filter, map, sort, or multi-source transforms.
- Usage-history collection or agent-generated automatic activation.
- Direct external APIs.
- Release or package distribution.
