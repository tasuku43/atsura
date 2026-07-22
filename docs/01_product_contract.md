# Product Contract

This contract defines Atsura's current vocabulary and supported product
boundaries. ADR 0005 supersedes the authorization-centered source-wrapper
semantics of ADR 0004; ADR 0006 adds the first compatibility-admitted runtime;
ADR 0007 prefers explicit RTK-backed optimizer defaults for exact maintained
compatibility contracts; and ADR 0008 keeps coding-agent hosts outside Atsura's
wrapper boundary. One canonical vendor-neutral bundle remains the authority.

## Product statement

Atsura deterministically compiles an existing CLI and a reviewed tailoring
specification into a purpose-specific wrapper surface for coding agents. The
surface defines which commands and options exist and how included commands
invoke and transform the source CLI. The source CLI remains authoritative for
operation semantics, authentication, authorization, and remote effects.

Routine compilation and supported execution require no language model.

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

adopted bundle + explicit purpose binding
        |
        v
host-neutral wrapper materialization
        |
caller-owned command resolution exposes the ordinary source command
        +--> maintainer invocation
        +--> coding-agent invocation
        |
        v
same fresh plan and bundle execution path
```

`bundle preview` implements the zero-execution branch of this flow. `bundle
execute` implements the first transform-only runtime branch. Both require the
exact schema-2 bundle to be user-adopted, revalidate current source path,
SHA-256, and size, and rebuild the same deterministic schema-3 plan. Execute
additionally requires an accepted adapter JSON selector contract and starts the
identity-checked resolved path at most once.

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
- arbitrary shell, script, jq, plugin, RTK program/argv, or runtime-LLM actions
  are invalid. The accepted finite output-processor direction is not implemented
  by schema 3.

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
process framing still apply. Identity-wrapper execution is not implemented by
the initial transform-only runtime.

### Transforming wrapper

An included command with a typed deterministic pipeline. The initial accepted
transform vocabulary retains exact argv additions and structured JSON
select/rename/compact rendering. Removal, replacement, defaults, and typed
before/after actions remain planned vocabulary until implemented and tested.

### Typed output projection

An output stage that promises one declared agent-facing shape. The current JSON
select/rename/compact stage is a projection. If its parser, input, selected
fields, or renderer cannot satisfy the declared contract, execution fails and
does not expose the projection input.

### Original-preserving output optimizer

A planned output stage that may produce either a smaller `optimized` result or
the byte-identical admitted stage input as `preserved`. Preservation is valid
only when the reviewed specification, bundle, and plan explicitly state that
the original stage input is allowed agent-facing output. An optimizer does not
make a confidentiality or complete-information claim about its optimized
result. Because RTK does not report which internal branch produced its bytes,
Atsura derives the disposition only from the admitted stage boundary: valid
processor stdout equal byte-for-byte to the admitted input is `preserved`; any
different valid stdout is `optimized`. This labels observable bytes, not RTK's
internal parser or fallback path.

An RTK-backed optimizer is the authoring default when Atsura's maintained
registry proves an exact source adapter/version/command and RTK version/filter
contract. The generated specification contains that choice explicitly before
review; a user or proposing agent may replace it explicitly before compilation.
RTK never selects or starts the source CLI in this boundary. The current schema
and runtime do not implement this concept.

### Materialized authoring default

A preferred wrapper choice written into the tailoring specification before the
user reviews and compiles it. It is ordinary canonical input after generation.
Installed tools, caller messages, and runtime discovery never add or replace a
stage implicitly. Outside the proven RTK matrix no RTK candidate is generated.
A built-in or identity alternative is offered only when its own contract is
supported; otherwise authoring reports that no maintained default exists.

### Wrapper execution plan

The complete typed result of resolving an included command against one adopted
bundle and attempted invocation. `bundle preview` constructs this plan without
starting the source. It contains:

- matched tailored command and explicit or inherited surface binding;
- bundle, specification, and catalog digests;
- exact source identity and source-adapter kind/contract version;
- original and transformed argv;
- wrapper kind and ordered before/invoke/output/after stages;
- the exact applied specification entry, or JSON `null` for an inherited
  surface entry, and the effective reason;
- tailored mode; and
- closed stdin, inherited working-directory and environment modes, and finite
  source-process attempts, timeout, stdout, and stderr bounds.

Plan existence means the included wrapper pipeline is structurally complete.
Preview does not apply it. Execute rebuilds rather than consumes that plan and
applies it only when the current runtime and adapter compatibility checks cover
the wrapper.
The plan does not contain a universal permission decision, source effect,
authorization target/impact, or confirmation requirement. A command absent
from the surface produces no plan. The preview envelope is schema version 2 and
contains `plan_digest`, `plan`, and `source_process_attempts`; the last field is
always zero. Exact schema-8 agent help publishes the `wrapper-plan` schema
version and a typed JSON-pointer inventory for every nested plan field.

Resolution first chooses the longest matching command path from the complete
embedded catalog. It then evaluates command membership and validates each
observed long option against the matched command's tailored option surface.
If that command has cataloged descendants and its next token is non-dash but
does not complete a cataloged descendant, preview cannot distinguish an
unobserved child from positional data and fails `invalid_invocation`. A caller
must place `--` before intended positional data in that ambiguous case.
Only after those steps does it append the wrapper's `append_args` and validate
the bounded no-shell invocation recorded in the plan. A plan with an output
transform additionally requires exactly one active cataloged structured-output
selector for the declared input format before any `--` marker.

### Raw execution

A future explicit tailoring bypass. Raw uses the bundle-bound source identity
but does not apply surface selection, argv changes, output transformation, or
other wrapper stages. It is never automatic fallback, a recovery action, a
caller-generated bypass, or a member of the tailored surface.

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

A future optimizer trust summary additionally identifies the processor kind,
version and exact identity, namespaced compatibility contract and filter
mapping, and whether original stage input may remain agent-visible.

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
future materialized wrapper artifacts or bindings, and future Atsura
configuration persistence. These commands retain exact intent, target binding,
impact, central mutation invocation, complete-output handling, and
non-retryable uncertain-outcome rules.

### Host-neutral wrapper materialization

A materialized wrapper is the product artifact that lets a caller use the
ordinary source-command spelling while applying one adopted purpose bundle. It
accepts argv, not an agent-host event or a shell command string, and delegates
to the same surface resolution, fresh planning, source execution, and result
contracts as the direct gateway.

The wrapper binding identifies only product facts:

- exact adopted purpose bundle;
- exact source executable identity;
- generated wrapper contract and exact runtime identity;
- ordinary command spelling; and
- recursion-prevention information that ensures the wrapper reaches the bound
  physical source rather than itself.

The binding contains no host name, hook event, settings path, permission value,
session, transcript, or model identity. Repository content and ambient host
state cannot manufacture adoption or replace the binding.

Materialization may eventually create Atsura-owned local state. Any such write
uses the normal create/write mutation contracts, atomic ownership, drift
reporting, and read-only reconciliation. The exact first lifecycle and whether
its artifact is a generated shell function, an executable shim, or both remain
part of the next implementation slice rather than this current public runtime.

### External activation and coding-agent consumers

The caller's environment decides how the generated wrapper wins ordinary
command resolution. Shell startup, a coding-agent hook, a container image, or
another external launcher may expose it. Atsura does not inspect, edit,
install, reconcile, or attest that environment and does not infer which caller
invoked the wrapper.

Coding-agent hosts are therefore not Atsura adapters. Production Atsura does
not decode their hook payloads, rewrite tool input, return permission decisions,
or own their settings and trust state. A downstream integration consumes the
same host-neutral argv contract and owns its vendor-specific compatibility
evidence outside Atsura.

This boundary preserves ordinary invocation but does not provide containment.
A caller may bypass the wrapper by resolving the physical source executable
through another route. Source, credential, provider, host, and operating-system
controls remain independent.

RTK is also independent of the caller. The wrapper consumes an already
compiled output stage and never detects RTK, selects a filter, or inserts an
output processor at invocation time.

## Current public artifact and transform workflow

The current milestone supports these artifact and runtime outcomes:

```text
atr source inspect --adapter github-cli --executable <path-or-name>
atr spec init --catalog <catalog.json> -- <source-command-path>
atr spec validate --catalog <catalog.json> --spec <spec.yaml>
atr bundle build --catalog <catalog.json> --spec <spec.yaml>
atr bundle status --bundle <bundle.json>
atr bundle trust --bundle <bundle.json>
atr bundle preview --bundle <bundle.json> -- <source-executable> <argv>
atr bundle execute --bundle <bundle.json> -- <source-executable> <argv>
```

`spec init` emits an exclude-by-default specification containing one included
verified command with inherited options and an identity wrapper. It does not
infer source safety or create adoption. Validation and build are read-only;
redirection of stdout is caller-selected filesystem behavior. The identity
draft is an authoring baseline, not an executable transform. For the current
runtime, the user deliberately changes its wrapper to the finite schema-3 JSON
transform, selecting only fields observed together in the inspected command's
structured-output evidence and declaring any collision-free rename. Exact
`source inspect`, `spec init`, and `spec validate` agent help publish the
versioned nested catalog and specification inventories needed to make that
edit without repository-source inspection.

`bundle status` recomputes all canonical bindings, observes exact-digest
adoption, and compares the current source path/hash/size without starting the
source. `bundle trust` is the only Atsura-owned mutation in this workflow.

`bundle preview` is a read-only, JSON-only utility. It admits only the exact
requested executable spelling or resolved path recorded in an adopted current
bundle, resolves one cataloged attempted invocation, and returns the complete
schema-3 tailored plan inside a schema-2 preview envelope plus its canonical
SHA-256 plan digest. It reads current
source identity but reports `source_process_attempts: 0` and performs no output
transformation.

`bundle execute` repeats those authority and plan steps rather than accepting a
preview document. It supports only a transforming wrapper with a typed JSON
output stage whose exact command and argv are admitted by a maintained source
adapter compatibility contract. Successful stdout is then validated by the
bounded parser and typed transform. It starts the identity-checked resolved
path with exact argv, closed stdin, inherited working directory and
environment, no shell, finite output/time bounds, and at most one attempt.
Success is schema-2 JSON containing bundle/plan identity,
matched command, transform result, source exit code, and attempts=1. Raw source
stdout/stderr and unselected fields are absent. Identity wrappers, argv-only
transforms, nonempty successful stderr, source refresh, and raw execution are
not implemented by this direct runtime slice. Host-neutral wrapper
materialization is the next consumer-facing entry point around this same path,
not another executor.

The current compatibility admission is also available in exact `bundle
execute` help: GitHub CLI adapter contract 2 and major 2, `issue list` or `pr
list`, a transform JSON output stage, one exact inline ordered selector,
maintained long-option grammar, no competing output mode, and empty stderr on
success. Live execution inherits the caller's source-CLI authentication plus
repository context from the inherited working directory or an admitted
command-specific `--repo` option. Atsura neither obtains those credentials nor
turns a source-owned failure into replay permission.

## Accepted next wrapper result

ADR 0008 fixes the next product slice without broadening the completed direct
runtime claim. A maintainer materializes one exact adopted purpose bundle as a
host-neutral wrapper, explicitly exposes that wrapper through a caller-owned
command-resolution mechanism, and invokes the ordinary source-command spelling
to reach the same fresh plan and execution path as `bundle execute`.

The first wrapper contract must fail before source start on missing adoption,
bundle or source drift, runtime or binding mismatch, or unsupported invocation.
A deterministic generated shell form records its byte digest as review and
release evidence, but invocation does not claim to attest a sourced function's
in-memory bytes. A future persisted executable artifact must add explicit
artifact ownership and drift validation before receiving that stronger claim.

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
- Deprecated `policy init`, `policy validate`, legacy `plan preview --config`, and
  `run --config` invocations return a stable migration diagnostic and start no
  source process.
- Recovery points to exact `spec` help. It never selects raw or silently builds
  or trusts replacement content.

## Output failure boundary

Source execution and output processing are separate facts. A projection failure
after the one source attempt does not retry or change the invocation, claim
transformed success, expose its input, or select raw mode. Cancellation,
timeout, output overflow, malformed JSON, missing fields, projection failure,
and final output-write failure after start are all non-retryable.

A future original-preserving optimizer may emit byte-identical `preserved`
output only as an adopted success disposition, never as recovery from another
stage. Processor failure remains non-retryable after source start and exposes
neither processor stderr nor failed intermediate output.

The no-shell process adapter compares the plan-bound path/hash/size before
start, immediately before start, and after wait. A portable race remains
between the final check and the operating system opening the executable.

Exact scoped help publishes the complete recovery inventory: 27 zero-attempt
faults for `bundle preview` and 41 fault codes for `bundle execute`. Execute
has 28 pre-start and 15 post-start phase cases because
`source_identity_changed` and `unclassified_source_execution_outcome` can
arise on either side of process start. Their one public signature remains
conservative enough for both phases. A declared recovery is part of the
contract only when conformance proves its exact kind, retryability, next
action, attempt phase, and secret-free output; a defensive invariant fault
that valid typed input cannot naturally reach is exercised at its owning
boundary rather than by corrupting production behavior.

## Compatibility boundary

The stable project identity is `Atsura`, binary `atr`, and Go module
`github.com/tasuku43/atsura`.

Shared catalog, specification, bundle, surface, wrapper-binding, and plan
schemas contain no GitHub-, Claude-, Codex-, or RTK-specific transport fields.
GitHub CLI 2.x remains
the first source adapter. Inspection contract 2 uses four fixed offline probes
and exposes runtime field/selector evidence only for `issue list` and `pr
list`. Its maintained runtime accepts GitHub CLI major 2, but one captured
version does not prove every future 2.x release. Coding-agent host protocols are
outside production compatibility. Consumer fixtures may record exact external
conditions needed to invoke the shared wrapper, but those conditions neither
enter product schemas nor become Atsura support claims.

The current preview grammar is intentionally narrower than arbitrary source
CLI grammar. Catalog evidence does not yet model short options, root/global
options, or command-specific positional arguments completely. Preview accepts
positional data, but rejects unmodeled dash-prefixed options and cannot prove
all positional dependencies. A command with cataloged descendants requires an
inner `--` before otherwise ambiguous positional data. `append_args` are
appended to the attempted argv exactly, including after an existing `--`; in
that case the source would treat
option-looking appended values as positional, and an output selector appended
there is not active. Preview proves the presence and format of one active
cataloged selector. Execute additionally requires an accepted exact selector
encoding and finite argv grammar; unknown adapters, older contracts,
unsupported commands, competing `--jq`/`--template`/`--web` output modes,
unmodeled positional or option syntax, separated selector values, duplicates,
ordering differences, or selectors after `--` fail before source start.

## Deliberately unsupported now

- Source refresh and catalog persistence.
- Identity-wrapper and argv-only-transform execution.
- Raw execution.
- Coding-agent host adapters, vendor hook protocols, host settings or permission
  mutation, and vendor-specific integration lifecycle commands.
- Any claim that Atsura installs, enables, or enforces wrapper activation in a
  coding-agent host.
- The accepted RTK-backed optimizer stage and its authoring default; both await
  an exact external-processor compatibility slice.
- Arbitrary shell, jq programs, scripts, plugins, RTK programs/argv, or
  unregistered external transformers.
- Non-JSON, streaming, aggregate, filter, map, sort, or multi-source transforms.
- Usage-history collection or agent-generated automatic activation.
- Direct external APIs.
- Public release or package-manager distribution.
