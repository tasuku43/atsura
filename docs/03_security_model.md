# Security Model

Atsura narrows and transforms the CLI surface presented to a coding agent. It
does not authorize source CLI operations and does not provide an operating
system sandbox. Security therefore depends on honest boundaries: strict
tailoring inputs, exact artifact adoption, identity-bound no-shell process
execution, controlled Atsura-owned mutations, bounded parsing, and explicit
failure when the core cannot evaluate its own contracts.

## Security objectives

Atsura must:

- never treat a command, argument, help response, catalog, specification,
  bundle, repository file, host payload, or source output as trusted merely
  because it exists;
- compile only complete, validated surface and wrapper definitions;
- start no source process when surface resolution or wrapper construction
  fails;
- never execute arbitrary shell, scripts, jq programs, plugins, or external
  transformers from the initial specification;
- bind adoption and execution to exact bundle and source identity;
- keep credentials and raw sensitive source output out of persistent state;
- preserve source invocation meaning when output optimization cannot be
  applied safely; and
- keep source execution separate from Atsura-owned mutation authority.

## Non-objectives

Atsura does not:

- determine whether a user or agent is authorized for a source operation;
- infer source safety or remote effects from command names or help prose;
- replace source authentication, authorization, prompts, operating-system
  controls, or provider-side policy;
- make an excluded command impossible to invoke through another binary, shell,
  or integration path;
- turn an adopted bundle into a permission grant; or
- claim that host `allow`, `ask`, or `deny` responses are universal security
  decisions.

## Trust boundaries

### Source executable and catalog evidence

The source executable is untrusted local code. Inspection may start it with
adapter-owned fixed argv, so inspection is an `EffectExecute` operation and
must use an exact identity-bound, no-shell, bounded process port. Help text,
option names, versions, extensions, and structured-output claims remain
untrusted evidence until validated by the named adapter contract.

Catalog membership proves observed structure, not permission or safety. Only
verified built-ins are eligible for the currently managed compiled surface;
observed extensions and unverified dynamic entries remain evidence.

### Tailoring specification

Repository-provided and user-provided specifications are untrusted input. The
schema is bounded, versioned, strict, and closed over a finite typed action
vocabulary. Unknown fields, aliases, duplicate entries, ambiguous option
overrides, catalog mismatches, invalid wrapper combinations, excessive input,
and retired authorization schemas fail before compilation or process start.

Arbitrary shell and arbitrary executable code are not valid specification
actions. Arguments remain argv elements and never become a shell program.

### Compiled bundle and adoption receipt

A bundle is self-consistent evidence, not trusted merely because its hashes
validate. Exact-digest adoption is a user-local decision to use the compiled
surface and wrapper set. It is not authorization for downstream source
operations.

Adoption summaries describe material surface and wrapper facts: included and
excluded entries, option changes, identity and transforming wrappers, argv and
output transformations, source identity, and bundle digest. They do not count
source permissions or inferred effects.

Changed source identity, catalog, specification, surface, wrapper, or bundle
content never inherits adoption. Repository-controlled paths cannot select or
replace receipts silently. Trust-store create/write operations remain
Atsura-owned mutations and cross the central mutation invoker.

### Host adapters

Host payloads, shell-like strings, settings files, working directories, and
environment values are untrusted. A future adapter may translate core states
into transport values such as `allow`, `ask`, or `deny`, but it cannot broaden
the compiled surface, authorize a source operation, or reinterpret absence as
a core permission decision.

Project-local host installation must use exact ownership markers, preserve
unrelated settings, and fail closed on malformed or conflicting state. Host
work is outside the current transform-runtime milestone.

## Surface composition is not isolation

The tailored surface is a capability and usability boundary for the agent. If
a command is absent, resolution returns `command_not_in_surface` and constructs
no wrapper plan. That result means only that Atsura's tailored surface does not
provide the command.

Hiding a command or option is not a sandbox, ACL, or operating-system deny.
Atsura documentation, faults, trust summaries, and host mappings must not imply
otherwise. Users who require containment must apply source, credential,
provider, host, and operating-system controls independently.

## Wrapper integrity

Surface membership and wrapper behavior are validated independently:

- an excluded command has no option surface and no wrapper;
- an included command has an explicit option surface and complete wrapper;
- an identity wrapper has no transformation;
- a transforming wrapper contains at least one supported typed transform; and
- unsupported before/after actions are invalid rather than ignored.

`bundle preview` binds the adopted bundle, bundle/catalog/specification
digests, exact source and adapter identity, matched command, original and
transformed argv, ordered stages, finite process bounds, and the exact applied
specification entry or `null` for inheritance. It does not carry a universal
permission decision. Preview revalidates current source path, SHA-256, and size,
returns a canonical plan digest, and reports `source_process_attempts: 0`.
Runtime revalidates again and rebuilds the plan rather than using an old
preview as authority.

## Source process execution

Starting the source is `operation.EffectExecute`: a source-owned process may
perform downstream work whose semantics Atsura does not classify. It is not an
Atsura read and carries no Atsura mutation target or impact.

The process boundary must:

- bind an exact resolved regular executable identity and argv vector;
- avoid a shell and ambient command reconstruction;
- declare finite attempts, time, stdout, and stderr limits;
- revalidate identity immediately before execution and assess it after the
  attempt where supported;
- start zero attempts on invalid surface, wrapper, adoption, drift, or identity
  state; and
- report every unknown post-start outcome as non-retryable rather than imply
  that replay is safe.

The source CLI remains responsible for credential prompts, source-specific
confirmation, authorization, destinations, and downstream effects.

`bundle execute` is the first bundle-backed source boundary. It supports only
adapter-admitted JSON transform wrappers, derives the request from the fresh
plan, and compares expected path/hash/size before start, immediately before
start, and after wait. Compatibility admission covers maintained command and
argv behavior; it does not trust stdout, which still passes through the strict
parser and typed transformer. `bundle preview` remains read-only. Inspection
probes remain separately bounded source execution.

## Atsura-owned mutations

Only Atsura state changes use the create/write mutation contract. Examples are
adoption receipt changes and future integration settings. Before the adapter
acts, these operations require explicit intent, exact target binding, complete
impact, and the central mutation invoker. Structured known outcomes survive
cancellation; unclassified results become non-retryable uncertain outcomes
with a read-only reconciliation action.

Those controls must never be used to claim knowledge about the downstream
effect of a source CLI command.

## Output transformation

Source output is untrusted and may contain control characters, prompt-like
text, malformed encodings, secrets, or very large structures. Typed parsers and
transformers use explicit format, depth, node, record, field, and byte bounds;
reject duplicate keys where semantics would be ambiguous; and preserve visible
projection rules at the CLI boundary.

If output optimization cannot be applied safely, Atsura must not change argv,
retry the source, select raw mode, invent a partial success, or silently expose
unreviewed raw output. The source attempt's meaning is preserved and the
transform failure is reported separately. Persistent state contains no raw
stdout, stderr, credentials, tokens, or transcripts.

## Raw execution

Future raw execution is an explicit tailoring bypass, not a permission bypass.
It revalidates bundle-bound source identity but applies no surface selection or
wrapper transformation. Raw is never automatic fallback, never a recovery
suggestion, and never part of the tailored agent surface. Raw is outside the
current transform-runtime milestone.

## Failure policy

Atsura fails before source process start when it cannot establish a contract it
owns, including:

- unsupported or retired specification/bundle schema;
- unknown or duplicate fields;
- catalog/specification/bundle digest mismatch;
- invalid surface membership or option override;
- missing, incomplete, or contradictory wrapper stages;
- command absent from the tailored surface;
- attempted option absent from the matched command's tailored option surface;
- missing adoption, source drift, or pre-start identity mismatch; or
- unknown core effect.

Retired authorization schemas are not auto-converted. Their allow/confirm/deny,
read/create/write, target, and impact values have no lossless meaning in the
surface/wrapper model. Migration diagnostics identify the retired schema and a
current recovery command, persist no state, and start zero source processes.

## Secrets and persistence

Atsura does not persist source credentials, environment snapshots, raw source
output, usage history, prompts, host transcripts, or agent reasoning. Canonical
catalogs and bundles contain only publishable structural evidence and exact
source identity facts required by their contracts. Diagnostic output must not
echo arbitrary secret-bearing environment values or unbounded hostile text.

## Release-artifact security evidence

The candidate archive and extracted `atr` are executable untrusted inputs to
the conformance harness. A fixed regular-member allowlist, link and traversal
rejection, byte bounds, and safe extraction prevent the archive from choosing
other filesystem targets; a digest and safe extraction do not make an
arbitrary executable trustworthy. Replay is limited to the reviewed candidate
from the same workflow, on an ephemeral matching native runner, using absolute
no-shell execution, an isolated working directory, isolated home and
configuration roots, a minimal credential-free environment, and finite time
and output bounds.

The installed-artifact conformance journey runs a provider-transport-free
source fixture with an allowlisted child environment, bounded attempt logs,
and unique stdout, stderr, and unselected-field canaries. Its synthetic
adoption receipt is written through the production trust-store adapter in that
isolated test root. This proves receipt consumption and exact-digest
enforcement; it is not a public trust bypass and is not evidence of user
consent.

The journey must show zero source attempts for pre-start rejection, exactly one
attempt for every admitted execution, non-retryable classification for every
post-start failure, and absence of canaries from stdout, stderr, saved state,
and bounded evidence. It may retain only digests, counters, target identity,
stable fault codes, and boolean leak checks. Native replay is required on each
claimed release target so an emulation or cross-build cannot silently replace
runtime security evidence.

Each native replay emits one bounded journey document. The aggregation tool
accepts exactly the five canonical evidence filenames and five matching
candidate archive filenames as regular non-symlink files, strictly binds each
target, observed host, archive name and recomputed SHA-256, tag-derived
version, full revision, command set, digests, counters, fault set, and leak
booleans, and rejects every extra or missing input. Its summary contains no
filesystem path, raw output, environment value, receipt, bundle digest, or
plan digest and labels itself `workflow_index_unattested`. A syntactically
valid document is not independently attested evidence: the workflow dependency
and GitHub artifact provenance must still show that the matching native matrix
job produced it.

## Known limitations

- Hiding commands and options does not prevent invocation outside Atsura.
- Artifact replay is not an OS or network sandbox. “Provider-network-free
  fixture” means the fixture implements no provider transport; it does not
  prove that a malicious candidate executable could not use runner networking.
- Local executable identity checks cannot provide operating-system sandboxing;
  portable path execution may retain a race between the final identity check
  and the operating system opening the file.
- Exact bundle adoption does not review or authorize every future downstream
  result of the source CLI.
- Source help can omit dynamic behavior or change through plugins and
  environment; adapter compatibility remains bounded evidence.
- Preview recognizes complete catalog command paths by longest prefix, but the
  catalog and plan grammar do not yet model short options, root/global options,
  or command-specific positional arguments completely. When a matched command
  has cataloged descendants, a non-dash token that is not a known child fails
  closed unless an inner `--` marks positional intent.
- `append_args` are appended exactly even after a positional-only `--`; preview
  exposes that result but does not prove that the source interprets it as an
  option.
- Preview requires exactly one active cataloged structured-output selector for
  the planned input format before `--`. Execute further requires an exact
  adapter compatibility contract; GitHub CLI contract 2 covers only `issue
  list` and `pr list`.
- The current GitHub runtime admits major 2 as a maintained range; one captured
  version is not evidence for every future 2.x release. Competing
  `--jq`/`--template`/`--web` output modes, unmodeled options, and positional
  arguments fail before source start.
- Successful nonempty stderr is rejected without exposing it because the first
  result schema has no reviewed stderr meaning.
- Identity/argv-only execution, before/after actions, richer argv transforms,
  raw, and host adapters remain unimplemented.

## Security claim for the current milestone

The transform-runtime milestone may claim that validated schema-3
specifications compile deterministically into schema-2 bundles; preview returns
one canonical plan with zero attempts; and execute rebuilds that plan, requires
exact adoption/current identity and adapter compatibility admission, requires
every observable executable identity to match the plan-bound path/hash/size,
starts at most once without a shell, and returns only the complete typed
selected JSON result. Pre-start contract failures start zero processes. Every post-start
failure is non-retryable and exposes no raw source output. The milestone does
not claim source-operation authorization, sandboxing, identity/raw execution,
or host integration.
