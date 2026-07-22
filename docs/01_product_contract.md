# Product Contract

This contract defines Atsura's current vocabulary and supported product
boundaries. ADR 0005 supersedes the authorization-centered source-wrapper
semantics of ADR 0004; ADR 0006 adds the first compatibility-admitted runtime;
ADR 0007 prefers explicit RTK-backed optimizer defaults for exact maintained
compatibility contracts; and ADR 0008 keeps coding-agent hosts outside Atsura's
wrapper boundary. ADR 0010 adds plan-declared source-stream results for finite
identity and argv-only ordinary wrappers. ADR 0011 introduces Go as a second
source and one finite application compatibility registry shared by plan
application and whole-surface rendering. ADR 0012 advances the current Go
source to contract 2 and admits the first exact external processor contract,
`atsura.output.rtk_go_test_pass.v1`. ADR 0014 compiles the included surface
into bounded ordinary-command help without turning help into a source
execution plan. ADR 0015 admits complete non-empty GitHub contract-2 surfaces
containing one or both maintained commands without changing the bundle or
wrapper contract. One canonical vendor-neutral bundle remains the authority.
ADR 0016 adds one finite catalog-typed value-option default with deterministic
caller precedence, schema-6 plan explanation, contract-3 help disclosure, and
no new process or host boundary.

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
  stages;
- the exact maintained Go-test tuple can use an explicitly inspected RTK
  artifact as a reviewable optimizer default without giving RTK source-
  execution authority; and
- every explicit surface or wrapper change has an inspectable reason.

Atsura does not decide whether the maintainer or agent is permitted to perform
the downstream source operation. It does not make an excluded command
impossible to invoke through another route.

## Conceptual flow

```text
bounded source-inspector adapter -> generated catalog evidence
catalog + reviewed tailoring specification schema 5
        |
        v
canonical bundle schema 4 + exact-digest user adoption
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

explicit `processor inspect` schema-1 observation
        +--> optional authoring evidence for the exact maintained optimizer
        +--> exact processor binding in a schema-4 bundle and schema-6 plan

adopted bundle + explicit purpose binding
        |
        v
`atr wrapper render --bundle <absolute-path>`
        |
        +--> fixed POSIX function bytes (Linux/macOS)
        +--> optional schema-2 JSON review envelope
        |
caller-owned command resolution exposes the ordinary source command
        +--> maintainer invocation
        +--> coding-agent invocation
        |
        v
generated function -> `atr wrapper run` -> same fresh plan and execution path
```

`bundle preview` implements the zero-execution branch of this flow. `bundle
execute` implements the first transform-only runtime branch. Both require the
exact schema-4 bundle to be user-adopted, revalidate current source and any
bound processor path, SHA-256, and size, and rebuild the same deterministic
schema-6 plan. Execute
additionally requires an accepted adapter JSON selector contract and starts the
identity-checked resolved path at most once. `wrapper render` closes the exact
bundle digest with the current absolute `atr` path/hash/size in deterministic
function bytes. `wrapper run` verifies that closure once the bound `atr` starts,
derives the source command spelling from the strictly loaded bundle, and
applies the same fresh plan while returning exactly its declared result mode:
either a compact JSON value, bounded source streams and conventional status,
or the result of the one finite original-preserving optimizer.

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
surface and wrapper set. The current schema version is 5.

Its initial normalized model is:

```yaml
schema_version: 5
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
        option_defaults:
          - option: --limit
            value: "30"
        append_args: ["--json=id,name"]
      output:
        kind: projection
        projection:
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
- each option-surface default is `inherit` or `exclude`;
- under option `inherit`, only exact exclusions may be listed;
- under option `exclude`, only exact inclusions may be listed;
- option overrides are sorted, unique, disjoint, and catalog-observed;
- wrapper kind is `identity` or `transform`;
- an identity wrapper has empty before/invoke/after changes and no output
  transform;
- a transform wrapper contains at least one supported transform;
- `invoke.option_defaults` and `invoke.append_args` are explicit lists whose
  combined length is at most 64;
- each option default is an exact included cataloged long option with
  `takes_value: true`, is not a structured-output selector, has one non-empty
  structurally safe UTF-8 value whose canonical `--option=value` argv element
  is at most `sourceprocess.MaxArgumentBytes` (4096 bytes), retains declaration
  order, is unique, and does not overlap an active option name in
  `append_args`;
- an output stage is a complete discriminated `projection` or `optimizer`
  union and cannot contain both;
- the optimizer form contains a catalog-observed input, one namespaced
  compatibility contract, and explicit original-output allowance, but no
  executable path or processor argv;
- before and after lists are explicit and currently must be empty because no
  built-in actions have yet been selected; and
- arbitrary shell, script, jq, plugin, RTK program/argv, unregistered
  processor, or runtime-LLM actions are invalid. Schema 5 admits only the exact
  maintained optimizer contract.

Schema 5 contains no source allow/confirm/deny, source read/create/write,
authorization target, or authorization impact fields.
Option-default values are public artifact data carried through the
specification, bundle, plan, exact-command help, and release evidence. They are
not a credential or secret-storage mechanism.

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
process framing still apply. The finite ordinary-wrapper runtime executes this
form through `source_stream_passthrough`; the direct `bundle execute` evidence
command remains transform-only.

### Transforming wrapper

An included command with a typed deterministic pipeline. The initial accepted
transform vocabulary retains exact argv additions, catalog-typed value-option
defaults, and structured JSON select/rename/compact rendering. A default is
wrapper behavior, not option membership: its option must remain in the
effective tailored surface so a caller can override it. Removal, replacement,
boolean/short/root/global/positional defaults, and typed before/after actions
remain planned vocabulary until implemented and tested.

Before the first caller `--`, `--option=value`, `--option value`,
`--option=`, and a separated option followed by an explicit empty argv element
all suppress that option's default. Repeated valid occurrences remain exact
and suppress only the configured insertion; Atsura does not select a repeated
source value. A short alias does not suppress a long default, and identical
text after `--` is positional. Each missing default becomes one canonical
`--option=value` token, in declaration order, immediately after the matched
command path. Caller tail argv remains exact, followed by unchanged
`append_args` at the end.

### Typed output projection

An output stage that promises one declared agent-facing shape. The current JSON
select/rename/compact stage is a projection. If its parser, input, selected
fields, or renderer cannot satisfy the declared contract, execution fails and
does not expose the projection input.

### Source-stream passthrough

A plan-declared result mode for an identity or append-argv-only wrapper whose
finite source-adapter contract is fully admitted. It returns the conventionally
completed source stdout and stderr bytes unchanged within the 4 MiB and 256 KiB
limits, adds no framing or projection, and returns the source status after both
final writes complete. It makes no UTF-8, terminal-safety, semantic-safety,
timing, or cross-stream-interleaving claim.

This is not raw execution: the tailored surface, option grammar, argv
transformation, exact adoption, source identity, compatibility proof, and fresh
plan all remain in force. It is never selected as fallback. Timeout,
cancellation, signal termination, overflow, wait uncertainty, identity
uncertainty, or inconsistent process evidence suppresses both captured streams
and produces a non-retryable Atsura fault.

### Original-preserving output optimizer

A finite output stage that may produce either independently validated
`optimized` bytes or the byte-identical admitted stage input. Preservation is valid
only when the reviewed specification, bundle, and plan explicitly state that
the original stage input is allowed agent-facing output. An optimizer does not
make a confidentiality or complete-information claim about its optimized
result. Because RTK does not report which internal branch produced its bytes,
Atsura derives the disposition only from observable stage boundaries. An
ineligible conventional source result is `preserved_before_processor` and
starts no processor. Successful processor stdout equal byte-for-byte to the
admitted input is `preserved_after_processor`; the only other accepted output
is the independently calculated shorter summary, reported as `optimized`.
These labels do not infer RTK's internal parser or fallback path.

The first exact contract is `atsura.output.rtk_go_test_pass.v1`: source-catalog
schema 2, Go CLI contract 2 with a recorded stable Go 1.26.x observation,
caller argv `go test`, transformed source argv `go test -json`, one explicit
processor-observation schema-1 document for an official RTK v0.43.0 artifact,
and fixed processor argv `pipe --filter=go-test`. `spec init --processor`
materializes this optimizer for that tuple; without the explicit observation
it retains the identity draft. `bundle build --processor` requires the same
compatible observation and binds the exact identity. RTK receives only the
bounded stdout that passes Atsura's strict pass-only lifecycle validator and
never selects or starts the source CLI.

Skip, failure, malformed or unknown JSON, empty output, source stderr, nonzero
status, and non-beneficial valid output are conventional but ineligible and
are preserved exactly before RTK starts. A processor failure after an eligible
source result is non-retryable, returns no source or processor bytes, and never
falls back. ADR 0009's rejected `git-log` tuple remains evidence that identity,
exit zero, empty stderr, and shorter output do not establish semantic
compatibility for another tuple.

### Materialized authoring default

A preferred wrapper choice written into the tailoring specification before the
user reviews and compiles it. It is ordinary canonical input after generation.
Installed tools, caller messages, and runtime discovery never add or replace a
stage implicitly. Outside the proven RTK matrix no optimizer default is
generated.
A built-in or identity alternative is offered only when its own contract is
supported; otherwise authoring reports that no maintained default exists.

### Wrapper execution plan

The complete typed result of resolving an included command against one adopted
bundle and attempted invocation. `bundle preview` constructs this plan without
starting the source. It contains:

- matched tailored command and explicit or inherited surface binding;
- bundle, specification, and catalog digests;
- exact source identity and source-adapter kind/contract version;
- the exact processor binding when the output stage requires one;
- original and transformed argv;
- the complete declared option-default list and exact applied subset;
- wrapper kind and ordered before/invoke/output/after stages;
- exactly one result mode: `transformed_json`,
  `source_stream_passthrough`, or `original_preserving_optimizer`;
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
always zero. Exact schema-12 agent help publishes the schema-6 `wrapper-plan`
version and a typed JSON-pointer inventory for every nested plan field.

Resolution first chooses the longest matching command path from the complete
embedded catalog. It then evaluates command membership and validates each
observed long option against the matched command's tailored option surface.
If that command has cataloged descendants and its next token is non-dash but
does not complete a cataloged descendant, preview cannot distinguish an
unobserved child from positional data and fails `invalid_invocation`. A caller
must place `--` before intended positional data in that ambiguous case.
After those steps it derives exact caller presence, inserts missing
`option_defaults` immediately after the matched command path, preserves caller
tail argv, appends the wrapper's `append_args`, and validates the bounded no-
shell invocation recorded in the plan. Inline, separated, explicit-empty, and
repeated exact long-option occurrences before the first `--` suppress a
default. Short aliases do not suppress it, and the same spelling after `--` is
positional data. Defaults are emitted in declaration order as
`--option=value`; caller repetitions and tail bytes retain their order and
spelling. Detached plan validation recomputes the applied subset and exact
transformed argv. A plan with an output
transform additionally requires exactly one active cataloged structured-output
selector for the declared input format before any `--` marker.

### Raw execution

A future explicit tailoring bypass. Raw uses the bundle-bound source identity
but does not apply surface selection, argv changes, output transformation, or
other wrapper stages. It is never automatic fallback, a recovery action, a
caller-generated bypass, or a member of the tailored surface.

### Compiled tailoring bundle

Bundle schema 4 is one canonical JSON document binding:

- exact resolved source identity and observed version;
- source-adapter kind and contract version;
- normalized catalog and catalog digest;
- normalized schema-5 specification and specification digest;
- an explicit bounded processor-binding list, empty when no optimizer is used;
  and
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
- option override, option-default, argv transformation, before/after action,
  and output transformation counts;
- the count of included wrappers whose conventional source streams may be
  returned without projection, with a conditional control/secret warning; and
- every processor kind, version, exact identity, namespaced compatibility
  contract, fixed filter mapping, and original-output visibility fact.

It contains no source read/create/write or allow/confirm/deny counts. The
user-local receipt stores only the exact bundle digest. Repository content and
redirected stdin cannot create a receipt.

Status and trust use schema-3 envelopes and report zero processor attempts;
they observe and compare processor identities but never start a processor.

### External-process execution effect

`operation.EffectExecute` means an Atsura command starts an identity-bound
external process: either a source CLI or an admitted output processor. It is a
statement about the local process boundary, not the downstream meaning of a
source operation. Execute carries no Atsura mutation target or impact and does
not imply read-only, safe, authorized, idempotent, or retryable.

Any post-start unclassified outcome must be treated as non-retryable because
the source CLI may already have performed an operation. Exact identity,
separate argv, no shell, finite attempts, time, and bytes remain required.

Source inspection uses Execute because it starts the selected source executable
with adapter-owned fixed metadata argv. Preview, validation, build, and status
remain read effects because they do not start a caller-selected source task.
Ordinary no-argument `go test` also remains Execute: tests may run untrusted
repository code, use credentials or configuration, resolve modules, access
networks, and mutate caller-owned files or caches. Its inclusion in a tailored
surface is not a read-only or authorization claim.

Processor inspection and optimizer application also use Execute. Their
identity, argv, environment, attempts, and failures are tracked at a processor
boundary distinct from source execution; a processor receives no authority to
select, authorize, or start the source CLI.

### Atsura-owned mutation

Create/write effects remain for state Atsura owns: bundle trust receipts,
future materialized wrapper artifacts or bindings, and future Atsura
configuration persistence. These commands retain exact intent, target binding,
impact, central mutation invocation, complete-output handling, and
non-retryable uncertain-outcome rules.

### Host-neutral wrapper materialization

A materialized wrapper is the product output that lets a caller use the
ordinary source-command spelling while applying one adopted purpose bundle.
The first public material form is a deterministic POSIX function rendered by:

```text
atr wrapper render --bundle <absolute-path> [--format text|json]
```

Text is the default and contains the exact sourceable function bytes. JSON is a
schema-2 `wrapper` review envelope containing `source`, `source_sha256`,
`command`, the POSIX contract version, exact bundle locator/digest, exact
current `atr` path/hash/size, `source_process_attempts: 0`, and
`processor_process_attempts: 0`. Rendering is an `EffectRead` utility and never
starts the source CLI or processor.

The renderer accepts only Linux or macOS, an absolute clean bundle locator, and
a requested executable that is verbatim one portable POSIX Name outside the
maintained reserved/fixed and implementation-specific function-name set.
It does not derive a basename from a path. One finite application registry
selects the whole-surface verifier by the bundle's exact adapter kind. The
complete included surface must be non-empty and every entry must be admitted in
full before any bytes are rendered. GitHub CLI contract 2 permits one or both
of `issue list` and `pr list`, each with its maintained transform, identity, or
append-only grammar. Those entries may use different existing result modes.
Go CLI contract 2 remains exactly one `test` command using either the identity-
wrapped source-stream surface or the exact processor-bound `test -json`
optimizer surface, with no caller-visible option grammar. Empty, partially
admitted, or otherwise unsupported surfaces produce no wrapper bytes; Atsura
never renders only the valid subset. On Windows, POSIX rendering returns the
structured `wrapper_platform_not_supported` fault; no Windows POSIX activation
contract is claimed.

This is one same-source surface, not composition across adapters. The canonical
bundle already binds one source identity and adapter contract. Whole-surface
admission validates each included command, effective option surface, wrapper,
and result mode through that exact registry-selected verifier. It performs no
source or processor probe, starts no process, and does not add a host adapter or
another I/O boundary. Each invocation still selects exactly one command and
uses that command's fresh plan as its exclusive result authority.

The fixed function invokes the exact absolute `atr` that rendered it:

```text
atr wrapper run \
  --contract-version=3 \
  --bundle=<absolute-path> \
  --bundle-digest=<sha256> \
  --runtime-path=<absolute-path> \
  --runtime-sha256=<sha256> \
  --runtime-size=<positive-integer> \
  -- <argv...>
```

These flags are a renderer-produced closure, not an authoring interface for a
second wrapper. `wrapper run` accepts exact argv, not an agent-host event or a
shell command string, and delegates to the same bundle loading, adoption,
surface resolution, fresh planning, compatibility admission, source execution,
and typed transformation used by the direct gateway. The source command
spelling is absent from the flags and is derived from the one strictly loaded
bundle.

The wrapper binding identifies only product facts:

- exact adopted purpose bundle;
- exact source executable identity;
- generated wrapper contract and exact runtime identity;
- ordinary command spelling; and
- the bounded tailored-help projection derived from the exact included
  surface; and
- the bundle-bound physical source path that execution uses instead of
  resolving the ordinary wrapper name.

The binding contains no host name, hook event, settings path, permission value,
session, transcript, or model identity. Repository content and ambient host
state cannot manufacture adoption or replace the binding.

Runtime identity is a cooperative drift contract, not executable attestation.
The shell must start the bound `atr` path before honest `wrapper run` code can
fingerprint itself. An unchanged honest runtime detects a binding mismatch and
starts no source process, but Atsura does not claim to constrain malicious code
that has already replaced the executable at that path.

The current renderer writes no Atsura-owned artifact and edits no activation
configuration; stdout redirection or sourcing is caller-owned behavior. A
future persisted wrapper or executable shim would create Atsura-owned local
state and must add normal create/write mutation contracts, bounded ownership,
atomic replacement, drift reporting, read-only reconciliation, and a separate
platform contract.

The generated source removes any existing alias with the exact ordinary
command name immediately before defining the function. This prevents alias
expansion from renaming or bypassing the wrapper. It is an in-memory effect of
the caller's explicit sourcing action, not an edit to shell startup or host
configuration; a caller that needs the old alias must restore it after removing
the function. Supported activation requires the standard POSIX `unalias`
utility not to be shadowed by a caller function; activation integrity beyond
that precondition remains caller-owned.

### Ordinary tailored help

Generated-wrapper contract 3 makes the rendered artifact self-describing. It
recognizes only a final exact `--help` after zero or more included command-path
segments. The root view lists every included exact command path; a namespace
view lists only its included descendants; an exact-command view shows the
bounded source summary, tailoring reason, and only the effective included long
options with explicit value arity. When an exact command is also a namespace,
one view contains both its exact-command facts and included descendants.
For a value-taking option with a configured default, only the exact-command
view adds the deterministic quoted disclosure
`--option=<value> (value required; default when omitted: "...")`. Root and
namespace membership remain unchanged.

The help model is derived from the canonical bundle during `wrapper render`.
The fixed function prints it through constant-format, single-quoted POSIX data
and includes the full bundle digest. It executes no raw source help and starts
no bound `atr`, source, or processor process. POSIX may implement its formatting
utility externally, so this is not a generic zero-OS-process claim. The output
describes the exact rendered artifact, not current source readiness or
authorization. Structural controls, format characters, and Unicode line
separators are invalid upstream; printable prompt-like meaning remains visible
untrusted text.

`-h`, source-specific `help` subcommands, no-argument aliases, root/global
options, positionals, and help-like values after `--` are not tailored-help
grammar in this slice. A selector without a compiled view is forwarded
unchanged to `wrapper run`; an excluded cataloged command therefore retains
`command_not_in_surface`, an unknown shape retains `invalid_invocation`, and
neither starts the source or falls back to source help.

### Wrapper result authority

Every command output declares exactly one authority for interpreting and
presenting a successful result. `catalog` authority keeps the static contract:
declared logical fields and, when JSON is supported, one named envelope and
positive result schema version. `help` remains the deliberate selector-
dependent exception: its catalog fields describe the root index while scoped
help projects the selected catalog contract. `wrapper render` is catalog-
authoritative; its text bytes and JSON review envelope describe the same
binding. `wrapper run` is the first public command with fresh-plan authority.
Agent help schema 12 publishes that discriminator without reinterpreting an
older machine contract.

`wrapper run` uses the exclusive
`fresh_wrapper_plan` authority. It is a finite union with no catalog-static
result fields or maintainer envelope. `transformed_json` emits one admitted
compact object or array plus LF on stdout, empty stderr, and status zero.
`source_stream_passthrough` emits exact bounded source stdout and stderr bytes
without framing and returns the conventional source status. Its complete
buffered delivery does not preserve timing or cross-stream order.
`original_preserving_optimizer` publishes the three exact dispositions and
their source/processor attempt contracts. `preserved_before_processor` returns
the exact conventional transformed-source streams and status after one source
and zero processor attempts. `preserved_after_processor` returns the exact
admitted input after one attempt of each. `optimized` returns the independently
validated newline-free summary after one attempt of each. Once processor
authority begins, a fault publishes no dynamic result and never falls back.
The agent contract's typed `plan_result_modes` publishes all three alternatives, and
`plan_schema` points to the exact `plan` field on `bundle preview`, including
the `wrapper-plan` schema ID and version. It describes the governing plan, not
a schema that either dynamic result must match, and avoids a second plan
registry. Whole-catalog validation resolves that reference and rejects drift.
Runtime rebuilds and validates the plan for the current invocation; an old
preview document is never input or authority.

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

## Current public artifact, transform, and wrapper workflow

The current milestone exposes these artifact and runtime outcomes, with release
quality still conditional on the required gates and exact-artifact evidence:

```text
atr source inspect --adapter github-cli|go-cli --executable <path-or-name>
atr processor inspect --adapter rtk --executable <absolute-path>
atr spec init --catalog <catalog.json> [--processor <processor.json>] -- <source-command-path>
atr spec validate --catalog <catalog.json> --spec <spec.yaml>
atr bundle build --catalog <catalog.json> --spec <spec.yaml> [--processor <processor.json>]
atr bundle status --bundle <bundle.json>
atr bundle trust --bundle <bundle.json>
atr bundle preview --bundle <bundle.json> -- <source-executable> <argv>
atr bundle execute --bundle <bundle.json> -- <source-executable> <argv>
atr wrapper render --bundle <absolute-bundle.json> [--format text|json]
atr wrapper run --contract-version=3 --bundle=<absolute-bundle.json> \
  --bundle-digest=<sha256> --runtime-path=<absolute-atr> \
  --runtime-sha256=<sha256> --runtime-size=<bytes> -- <argv...>
```

`source inspect` selects one bounded adapter explicitly. `github-cli` contract
2 performs four fixed offline probes; `go-cli` contract 2 performs exactly
`go version`, `go help`, and `go help test`. Both produce the same vendor-neutral
catalog schema 2 and retain only validated structural evidence, source
identity, and attempt counts. Go contract 2 additionally records exact
`go_test_jsonl` output selected by the single-dash `-json` token.

`processor inspect` accepts only one explicit absolute path. It verifies the
official artifact identity for the current maintained Linux/Darwin platform,
runs exactly one isolated no-shell `rtk --version` probe, and returns canonical
processor-observation schema 1. It does not discover, download, install, or
configure RTK. The observation binds host-neutral environment contract
`atsura.processor.rtk_isolated.v2`; retired v1 observations are incompatible
and must not build or run a bundle.

`spec init` emits an exclude-by-default specification containing one included
verified command with inherited options and an identity wrapper. It does not
infer source safety or create adoption. Validation and build are read-only;
redirection of stdout is caller-selected filesystem behavior. The identity
draft is an executable ordinary-wrapper baseline only when its complete surface
and invocation fit the finite source-stream compatibility contract. For exact
Go `test`, supplying a compatible processor observation materializes the
reviewable optimizer default; without that explicit evidence the draft remains
identity. A user may instead deliberately change a draft to the finite schema-5
JSON projection, selecting
only fields observed together in the inspected command's structured-output
evidence and declaring any collision-free rename. Exact
`source inspect`, `spec init`, and `spec validate` agent help publish the
versioned nested catalog and specification inventories needed to make that
edit without repository-source inspection.

`bundle build` requires that same observation exactly when the specification
selects the optimizer, rejects unused or incompatible evidence, and binds the
processor identity into bundle schema 4. `bundle status` recomputes all
canonical bindings, observes exact-digest adoption, and compares current source
and processor identity without starting either process. `bundle trust` is the
only Atsura-owned mutation in this workflow.

`bundle preview` is a read-only, JSON-only utility. It admits only the exact
requested executable spelling or resolved path recorded in an adopted current
bundle, resolves one cataloged attempted invocation, and returns the complete
schema-6 tailored plan inside a schema-2 preview envelope plus its canonical
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
transforms, original-preserving optimizers, nonempty successful stderr, source
refresh, and raw execution are not implemented by this direct evidence command;
an optimizer is rejected before source start.

`wrapper render` is the zero-source-attempt producer of the complete function
and binding. Its contract-3 fixed function answers compiled root, namespace,
and exact-command final-`--help` selectors without starting the bound `atr`,
source, or processor. Exact-command help discloses each configured finite
value-option default with the same bounded formatter used by compilation;
root and namespace help remain indexes and do not repeat those defaults. Every
other argv list is forwarded unchanged to
`wrapper run` with root structured JSON errors. `wrapper run` first validates the closure against the
honestly executing current `atr` identity and expected bundle digest, then
reuses the shared fresh-plan application boundary. Success follows one of three
plan result modes: compact JSON plus LF and empty stderr; exact bounded source
stdout/stderr and conventional status; or the finite original-preserving
optimizer with one of its three declared dispositions. A conventional nonzero
status in source-stream or pre-processor preservation is a source result, not
an Atsura fault. Pre-start failures start zero source processes; uncertain
post-start, processor, and final-output failures are non-retryable and expose
no captured source or processor bytes through a fault. The generated function
never selects raw execution, another bundle, an ambient source executable, or
a runtime-discovered processor.

The current compatibility admission is also available in exact `bundle
execute` help: GitHub CLI adapter contract 2 and major 2, `issue list` or `pr
list`, a transform JSON output stage, one exact inline ordered selector,
maintained long-option grammar, no competing output mode, and empty stderr on
success. The finite GitHub `pr list` admission also accepts its compiled
`--limit` value-option default. Atsura inserts it as `--limit=<value>` only
when the caller did not supply `--limit` before the first positional-only `--`;
the adapter validates both defaulted and caller-overridden transformed argv.
Live execution inherits the caller's source-CLI authentication plus
repository context from the inherited working directory or an admitted
command-specific `--repo` option. Atsura neither obtains those credentials nor
turns a source-owned failure into replay permission.

Ordinary-wrapper admission is broader only through the shared finite registry,
not through a second plan or executor. It dispatches by the exact adapter kind
already bound into the fresh plan or bundle. Go CLI contract 2 accepts stable
Go 1.26.x effective-toolchain evidence observed by `go version` during
inspection and exact command `test`. It admits either one identity wrapper with
no output stage and `source_stream_passthrough`, or the exact append-`-json`
transform plus processor-bound `original_preserving_optimizer`. Every
caller-supplied Go option, package pattern, `--` marker, and test-binary
argument is rejected before source start. Direct `bundle execute` remains the
GitHub JSON-transform evidence command.

`go version` may itself delegate, so the recorded version is not the version of
the directly identified launcher file. The plan carries that inspection-time
observation; runtime revalidates only the direct launcher's path, SHA-256, size,
and exact argv, and does not repeat the version probe. The same launcher can
later select or download a different
effective toolchain from module, working-directory, `GOTOOLCHAIN`, `GOROOT`, or
related ambient state without pre-start detection. Atsura does not identify a
selected toolchain or GOROOT tree. A future constraint needs an explicit
environment/toolchain closure, a successor ADR, and platform evidence.

## Host-neutral wrapper result

ADR 0008 fixes this product slice without broadening the direct runtime's source
compatibility. A maintainer renders one exact adopted purpose bundle as a host-
neutral POSIX function, explicitly exposes that function through a caller-owned
command-resolution mechanism, and invokes the ordinary source-command spelling
to reach the shared fresh plan and execution path. The JSON-transform variant
matches `bundle execute`; the source-stream variant is currently exposed only
through the ordinary-wrapper path. The processor-bound Go variant is likewise
an ordinary-wrapper result; `bundle execute` remains projection-only.

The wrapper contract fails before source start on missing adoption,
bundle or source drift, runtime or binding mismatch, or unsupported invocation.
A deterministic generated shell form records its byte digest as review and
release evidence, but invocation does not claim to attest a sourced function's
in-memory bytes. Linux and macOS are the POSIX rendering and activation targets;
Windows retains existing-command regression coverage and a structured
unsupported result, not a POSIX activation claim. A future persisted executable
artifact must add explicit artifact ownership and drift validation before
receiving that stronger claim.

## Migration contract

Authorization-policy schemas 1 and 2, tailoring-specification schemas 3 and 4,
bundle schemas 1 through 3, generated-wrapper contract 2, and their plan/run
semantics are retired experimental formats.

- Readers never interpret specification schemas 1 through 4 as schema 5.
- Old bundle bytes never validate as bundle schema 4.
- Contract-2 generated-wrapper invocations are rejected; maintainers render a
  fresh contract-3 function from the adopted current bundle.
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

The finite original-preserving optimizer returns exact transformed source
bytes and status as `preserved_before_processor` only after a conventional
ineligible result and with zero processor attempts. After one admitted
processor attempt, byte-identical output is `preserved_after_processor` and
the exact independently calculated shorter summary is `optimized`. Processor
failure remains non-retryable after source start, exposes neither processor
stderr nor failed intermediate output, and never selects preservation as
fallback.

For `source_stream_passthrough`, one bounded conventional completion is itself
the declared result. Zero or nonzero source status and successful nonempty
stderr are preserved. Abnormal termination, timeout, cancellation, overflow,
wait or identity uncertainty, and inconsistent process evidence suppress both
captured streams. The CLI completes stdout once and stderr once, in that order,
then returns the source status. Those writes are not atomic; a short or failed
write returns non-retryable `execute_output_write_failed`, may leave partial
caller-visible bytes, does not return the source status, and never recommends
replay.

The no-shell process adapter compares the plan-bound path/hash/size before
start, immediately before start, and after wait. A portable race remains
between the final check and the operating system opening the executable.

Exact scoped help publishes the complete catalog-derived recovery inventory.
Conformance proves a bijection with every declared code, kind, retryability,
next action, source/processor attempt phase, and secret-free output. A
defensive invariant fault that valid typed input cannot naturally reach is
exercised at its owning boundary rather than by corrupting production
behavior.

## Compatibility boundary

The stable project identity is `Atsura`, binary `atr`, and Go module
`github.com/tasuku43/atsura`.

Shared catalog, specification, bundle, surface, wrapper-binding, and plan
schemas contain no GitHub-, Claude-, Codex-, or RTK-specific transport fields.
GitHub CLI 2.x remains the first source adapter. Inspection contract 2 uses four
fixed offline probes and exposes runtime field/selector evidence only for
`issue list` and `pr list`. Go CLI is the second source. Inspection contract 2
uses three fixed offline probes and the maintained runtime accepts stable Go
1.26.x recorded inspection observations only for exact no-argument `test` with
an identity wrapper or the exact append-`-json` optimizer wrapper. One finite
application registry routes both source contracts through the same plan, binding,
process, and result schemas. An unknown, absent, duplicate, nil, or otherwise
misconfigured verifier fails as `adapter_contract`; it cannot create a fallback
or partial registry.

A separate finite processor registry contains only
`atsura.output.rtk_go_test_pass.v1` for an official RTK v0.43.0 artifact on
Linux amd64/arm64 or Darwin amd64/arm64. It maps the namespaced contract to the
fixed `pipe --filter=go-test` invocation and never owns source selection.
Windows remains outside the optimizer matrix. No registry consults ambient
`PATH`, host settings, plugins, or project configuration.

The maintained GitHub major-2 and recorded Go 1.26.x ranges are explicit
compatibility decisions, not proof that one captured observation predicts
every later patch or minor release. A catalog whose recorded Go observation is
outside 1.26.x, Go options, package arguments, positional markers, and test-
binary arguments require new evidence and contract revisions. A later effective
Go 1.27 selection by the same launcher is not detected by contract 2; closing
that gap is a separate environment/toolchain decision. Coding-agent
host protocols are outside production compatibility. Consumer fixtures may
record exact external conditions needed to invoke the shared wrapper, but those
conditions neither enter product schemas nor become Atsura support claims.

The first wrapper renderer is intentionally platform- and surface-bounded. It
produces fixed POSIX function source only on Linux and macOS, derives the
ordinary command (`gh` or `go`) verbatim from the bundle's requested
executable, and requires one non-empty complete runtime-admitted surface. The
finite GitHub contract permits one or both maintained commands and their
existing JSON-transform, identity, and append-argv-only source-stream modes,
including different modes per command. The Go contract remains the exact
singleton `test` identity source-stream surface or exact Go pass optimizer
surface. Windows supports the existing portable commands but not POSIX wrapper
rendering, activation, or the optimizer.

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
- Raw execution.
- Persistent wrapper installation, replacement, removal, executable/PATH shims,
  and multi-profile wrapper selection.
- Coding-agent host adapters, vendor hook protocols, host settings or permission
  mutation, and vendor-specific integration lifecycle commands.
- Any claim that Atsura installs, enables, or enforces wrapper activation in a
  coding-agent host.
- Any RTK source, version, command, package count, processor version, or filter
  outside `atsura.output.rtk_go_test_pass.v1`.
- Arbitrary shell, jq programs, scripts, plugins, RTK programs/argv, or
  unregistered external transformers.
- Streaming, aggregate, filter, map, sort, or multi-source transforms. Plan-
  declared source streams are result delivery, not a text transformation.
- Usage-history collection or agent-generated automatic activation.
- Direct external APIs.
- Public release or package-manager distribution.
