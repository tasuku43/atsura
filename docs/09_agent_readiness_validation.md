# Agent Readiness Validation

This validation asks whether a coding agent can discover, execute, interpret, and recover from representative API-backed tasks with few CLI round trips. The scenarios are intentionally synthetic and public-safe. They model a project-collaboration CLI and a team-chat CLI without embedding a private roadmap, endpoint, account, or credential.

## What counts as a round trip

A round trip is one CLI invocation whose purpose is to learn what invocation should come next. The task invocation and authentication ceremony are counted separately, but the ceremony must also publish the human-handoff scorecard below. Parsing a declared JSON or TSV field is not an additional discovery round trip; scraping prose, guessing a URL, or probing variants is.

Track external processing separately from round trips. Extracting a declared
JSON or TSV field is direct consumption and has count zero. An undeclared
`jq`/`grep` join, custom parser, provider-notation interpretation, source
inspection, or exploratory API request is one external reconstruction step. A
supported task outcome has a routine-success external-processing budget of zero;
a deliberately raw export or low-level utility may publish a narrower contract.

The target is:

- unknown surface to one selected scoped task contract: at most two
  help-discovery invocations;
- known task path, with every required reference and other task input already
  held, to its executable contract: one scoped-help invocation;
- discover reference to read/write: no extra lookup or transformation invocation;
- classified failure to next corrective command: no prose interpretation or command guessing.
- supported outcome to semantic answer and canonical next reference: zero
  undeclared external reconstruction steps.

These are bounds on help discovery and selected-contract retrieval, not on the
whole workflow. They exclude authentication and task invocations, producer
discovery, and any later scoped-help request needed for the complete contract of
a workflow endpoint outside the initial selection. Grouped workflow endpoint
usage can form next argv without implying that the endpoint's complete contract
was selected.

For each setup/authentication candidate, record required environment variables or exports, fixed values re-entered, terminal-to-browser transfers, browser-to-terminal transfers, clipboard or OS-integration dependencies, discover-to-act trips that do not contribute to target selection, first-run commands, steady-state commands, and ceremonial inputs that add no target certainty. These values compare candidates; they are not a scalar optimization target. A handoff may be justified when it materially improves safety, explicit consent, or agent certainty.

Use this scorecard in the active work packet for every credible candidate:

| Measure | Candidate value | Safety/certainty reason |
|---|---:|---|
| Required environment variables or exports |  |  |
| Fixed values re-entered |  |  |
| Terminal-to-browser transfers |  |  |
| Browser-to-terminal transfers |  |  |
| Clipboard or OS-integration dependencies |  |  |
| Non-selecting discover-to-act trips |  |  |
| First-run commands |  |  |
| Steady-state commands |  |  |
| Ceremonial inputs with no target certainty |  |  |

## Contract-level validation method

For each derived command, verify all four stages.

| Stage | Evidence |
|---|---|
| Discover | Root `view: index` exposes path, namespace, summary, capability, outcome, effect, and role plus a machine-readable `scope_request`; selected `view: scope` declares the argv grammar, inputs, input sources, prerequisites, effect, output, authentication, errors, mutation facts, and workflow edges |
| Execute | Arguments are copied from declared fields or explicit configuration, and the resolved command, effect, and any declared authentication requirement validate before I/O. Source execution additionally binds exact executable identity, argv, and process bounds but has no mutation target or impact. An Atsura-owned create/write additionally validates its exclusive reference/fixed-target binding, mutation target, and impact. |
| Interpret | The result is bound to its declared task and every target, parent, or scope dimension that task actually carries before rendering; scoped empty collections retain scope and interpretation-relevant absent, empty, zero, false, and unresolved states stay distinct; machine output has declared fields/types/delivery/collection coverage; structural runes are visibly projected; scoped I/O metadata marks external text as untrusted data; opaque references retain exact values and their field-required kinds |
| Recover | Failure kind/code/retryability/next actions are structured; auth, permission, ambiguity, missing targets, rate limits, temporary failure, cancellation, and contract failure remain distinct |

## Semantic fixture and presentation evidence

A relationship-rich or otherwise interpretation-sensitive capability keeps one
presentation-independent typed fixture and a machine-readable answer key. The
fixture includes every request dimension the task carries, retains scope when a
scoped collection is empty, and includes interpretation-relevant
absence/empty/zero/false examples, unresolved facts, and canonical references.
Tests bind the answer key back to the typed fixture before using it to judge a
renderer.

Select negative canaries that apply to the capability, including tempting but
invalid inferences from equal display names, adjacent items, ordering, quoted
prose, raw provider notation, unknown or out-of-window parents, and indentation.
A renderer is eligible only when the semantic answer and exact next argv can be
obtained from one command with zero external reconstruction and every canonical
action reference remains complete.

For a significant default presentation change, generate before and after
goldens from the same typed fixture. Record the fixture and golden hashes, exact
byte counts, and any tokenizer name/version and token counts. These are
secondary efficiency evidence after semantic eligibility, not a substitute for
correctness. Keep failed, invalidated, and inconclusive evidence, and record a
product compatibility decision separately from any benchmark result. Live-model
evaluation is explicit and optional; it is not part of `task check`.

## Scenario A: project-collaboration CLI

### Outcome

Find a project by a human filter, obtain its canonical reference, and read its current summary.

### Expected path

1. The agent reads the compact root outcome index, chooses `commands[].path` or `commands[].namespace`, and applies the published `scope_request.invocation_template` without guessing help syntax.
2. Scoped help identifies a `discover` command, its filter input, authentication/scopes, complete or paged delivery, exhaustive/bounded/differential collection coverage, and its produced `project` reference field.
3. The agent runs discovery in a machine format and selects an exact `project_id`. Multiple candidates remain data, not a hidden choice by the later action.
4. Scoped help for the read action declares `--project-id` as consuming that reference kind.
5. The agent passes the exact emitted bytes into the read action. It does not parse a browser URL, normalize case, or call discovery again.
6. The result declares its delivery and collection coverage and names every
   stable output field.

### Recovery probes

- No credential: `authentication`, not `permission`; next action names the configured login/status command.
- Valid identity without scope: `permission`; retrying login is not claimed to fix it unless the derived flow can request additional scope.
- No matches: successful empty discovery or a documented `not_found`, never a fabricated reference.
- Multiple matches: discovery returns candidates or `ambiguous`; action is not attempted.
- Stale project ID: `not_found` with discovery as a next action.
- Page cursor loop or local bound: contract failure, no partial successful output.
- Rate limit: `rate_limited`, independent retryable metadata and bounded
  retry-after evidence; timing never authorizes a duplicate logical operation.

### Acceptance

An agent that knows only the desired outcome reaches one selected task contract
with at most two help-discovery invocations, then reuses the discovered
reference without transformation. Once the read path and all required inputs
are known, its complete contract takes one scoped-help invocation. Retrieving a
complete contract for an endpoint outside the initial selection is counted
separately. Every recovery probe selects its next action from structured
metadata.

## Scenario B: team-chat CLI

### Outcome

Find a room, inspect its metadata, then send one message to the explicitly selected room.

### Expected path

1. A scoped query identifies room discovery and declares the exact output field carrying the room reference.
2. The read action consumes the same room reference and makes no hidden name search.
3. The send action declares `create` or `write` according to the derived thesis, cardinality `one`, notification `yes`, access-change/destructive declarations, authentication/scopes, idempotency behavior, and stable result fields. Creating a new message binds the selected room reference as `parent_input` and has no `target_id_input`; changing an existing message binds the message reference as `target_id_input` and may bind the room as a distinct `parent_input`.
4. The application mutation invoker validates the runtime intent and applies the project's policy. The template does not assume whether that policy is human confirmation, dry-run, OS authentication, or a role check.
5. The infrastructure adapter performs one logical send. It retries transport only if the upstream operation is safe or uses one stable idempotency key.
6. The result returns the canonical message and room references needed by later reads or updates.

### Recovery probes

- Room name supplied where an ID is required: `invalid_input`; next action is room discovery.
- Room reference maps to multiple accounts/tenants: `ambiguous`; account-selection command is explicit.
- Missing send scope: `permission`; zero send attempts.
- Policy denial or missing impact dimension: `rejected` or `contract`; zero send attempts.
- Missing, extra, non-opaque, or reference-kind-mismatched mutation binding: catalog/contract rejection; zero send attempts.
- Timeout before execution: `canceled`/`unavailable`; zero or explicitly classified transport attempts.
- Timeout after an unknown upstream result: do not claim a safe retry unless idempotency proves it; provide a read/status action when available.
- Hostile room/message text: raw controls, format runes, line/paragraph separators, and delimiters cannot alter terminal or TSV/JSON structure. Existing backslashes remain distinguishable from projected controls. Printable JSON-looking or prompt-like prose remains present as untrusted data; the CLI makes no semantic prompt-injection-prevention claim.

### Acceptance

The agent never sends to a room selected implicitly by display name, can identify the exact input supplying the create parent or existing write target, can tell that sending has a notification side effect before executing it, and does not repeat an unsafe send after an uncertain failure. It treats every external text field as data rather than as a CLI-authored instruction.

## Runnable template probes

The synthetic sample flow is the executable minimum for these scenarios:

```sh
go run ./cmd/atr help --format agent
go run ./cmd/atr help sample --format agent
go run ./cmd/atr sample list --format json
go run ./cmd/atr sample read --id smp_2f4a6c8e0b1d --format json
go run ./cmd/atr --error-format json sample read --id smp_000000000000
```

The root agent contract must be schema version 10 with `view: index`, reveal the
`sample` namespace and both exact paths, and contain no input, output,
authentication, error, mutation, fixed-target, or workflow detail. Its
`scope_request` must identify the selector fields and exact invocation template.
Its two-invocation unknown-outcome bound means root index plus one selected
scoped contract; its one-invocation known-path bound assumes every required
reference and other task input is already held. Neither includes task execution
or later complete-contract retrieval for a workflow endpoint outside the
selected scope. The scoped contract must use `view: scope`, contain only the
relevant list/read commands, and represent the `sample` workflow as one
reference-kind group with unique `producers[]` and `consumers[]`. The producer
field plus consumer input and exact usage must provide the next argv without a
command-local duplicate edge. Its `invocation_grammar` must explain value and
boolean flag forms, equals-only dash-prefixed flag values, and the
positional-only marker. The complete global and selected command contracts
remain present, including fault-local recovery actions. Its `io_contract` must
publish `external_text_trust: untrusted_data`,
`external_text_projection: visible_escape`, and
`opaque_reference_policy: validated_exact_bytes`. The help catalog's
`CommandOutput.Fields` describes root `view: index` command entries; the
input-selected `view: scope` document is an independent variant under the same
schema version, with both views covered by dedicated exact-key contract tests.
The `id` selected from the list JSON is field extraction, not identifier
transformation: pass its exact string bytes to read. The final probe must fail as
`not_found`, use the dedicated exit status, write no success data to stdout, and
name `sample list` as the structured next action on stderr.

### Scoped-help footprint evidence

On the template catalog, the schema-4 (pre-schema-5) 2026-07-18 UTF-8
measurements were 1,517 bytes for root agent help, 5,359 bytes for exact
`sample read` help, and 8,359 bytes for the `sample` namespace. The 512-byte
limit continues to bound each root selection entry.

With schema 10, measured on 2026-07-22 for the plan-result-mode catalog, the
current root is 6,333 bytes, exact `sample read` help is 6,005 bytes, and the
`sample` namespace is 8,916 bytes. Exact artifact contracts remain scoped:
`source inspect` is 11,338 bytes, `spec init` is 10,651 bytes, `spec validate`
is 11,843 bytes, `bundle build` is 8,546 bytes, `bundle status` is 7,687 bytes,
`bundle trust` is 8,903 bytes, `bundle preview` is 16,887 bytes, `bundle execute`
is 15,571 bytes, `wrapper render` is 11,511 bytes, and `wrapper run` is 16,127
bytes. Preview's larger scoped contract
includes the versioned `wrapper-plan` JSON-pointer field/type inventory. The
root contains selection entries rather than those complete invocation and
failure contracts.

Schema 10 retains the fixed derived-scale regression with six selected
commands, 18 producer endpoints, 18 consumer endpoints, and 324 implicit same-
kind edges. The grouped document is 26,225 UTF-8 bytes; a pair-expanded
representation of the same facts is 169,699 bytes. The fixed corpus has a
65,536-byte whole-response budget. The test expands the groups in memory and
proves exact edge-set equality, so meeting the budget cannot delete producer
fields, consumer inputs, usage, invocation contracts, or fault recovery. This
is a regression bound for the named corpus, not a claim that an arbitrary
catalog can never exceed 64 KiB.

Validation must also cover:

- every list-emitted sample ID passed unchanged to read;
- URL, name, partial, uppercase, whitespace, and control-character variants rejected before repository access;
- catalog/output snapshots detecting field or semantic changes;
- root-versus-scoped agent-help shape snapshots and a per-command root-size growth bound;
- executable checks that each single-shape JSON result's schema version,
  envelope, and item keys equal its `CommandOutput` declaration;
- help checks that root `commands[]` keys equal the help
  `CommandOutput.Fields` declaration and that dedicated exact-key contracts fix
  both root `view: index` and input-selected `view: scope` variants;
- adversarial TSV/JSON/stderr fixtures containing ESC, actual newline, bidi and zero-width format runes, U+2028/U+2029, literal backslash escapes, JSON-looking fragments, and prompt-like printable text;
- exact opaque-ID round trips alongside hostile labels/content, proving presentation never rewrites identity;
- complete pagination or no result;
- typed not-found recovery pointing back to discovery;
- structured contract visibility for effect, prerequisites, fields, delivery,
  collection coverage, errors, and next actions.
- declared default formats, JSON envelopes/schema versions, stdout/stderr ownership, and the complete exit-code map;
- successful output emitted only after complete pagination, validation, bounding, and rendering;
- root help that never embeds complete command contracts, plus namespace/exact scoped help that does not force the agent to ingest unrelated detail.

The sample is not evidence that a real API adapter is secure. A derived CLI repeats the scenario with fake adapter fixtures, authentication failures, pagination, cancellation, policy denial, and upstream error mappings before enabling a real network integration.

## Scenario C: Atsura artifact compilation

### Outcome

Given one installed supported source CLI, a maintainer can produce exact
catalog evidence, create and review a schema-3 purpose-specific surface, and
compile one deterministic schema-2 bundle without starting a bundle-backed
source invocation.

### Runnable probe

From the repository root, with `gh` installed:

```sh
go run ./cmd/atr help --format agent
go run ./cmd/atr help spec --format agent
go run ./cmd/atr source inspect --adapter github-cli --executable gh > /tmp/atsura-catalog.json
go run ./cmd/atr spec init --catalog /tmp/atsura-catalog.json -- pr list > /tmp/atsura-spec.yaml
go run ./cmd/atr spec validate --catalog /tmp/atsura-catalog.json --spec /tmp/atsura-spec.yaml
go run ./cmd/atr bundle build --catalog /tmp/atsura-catalog.json --spec /tmp/atsura-spec.yaml > /tmp/atsura-bundle.json
go run ./cmd/atr bundle status --bundle /tmp/atsura-bundle.json
```

Root selection plus one `spec` namespace request meets the two-invocation
unknown-surface bound. The GitHub CLI source inspection reports exactly four
bounded offline probe attempts. The generated specification is exclude-by-default
and contains one exact included verified command with inherited options and an
identity wrapper. Validation returns the normalized specification, its digest,
and surface/wrapper counts. Repeating bundle build with identical catalog and
specification bytes produces the same bundle digest. Status starts no source
process and reports `not_adopted` before adoption.

The same public inspection contract also accepts `--adapter go-cli` when `go
version` records a stable Go 1.26.x effective-toolchain observation. It
performs exactly `go version`, `go help`, and `go help test`, emits the same
vendor-neutral catalog schema with all parsed root built-ins including `test`,
and persists no raw help. The runtime contract admits only `test`; this
alternate artifact path does not imply that arbitrary Go commands or arguments
are executable. Scenario G owns the one admitted no-argument runtime.

The static [schema-3 example](../examples/tailoring-spec.schema3.yaml) is
structural evidence only: its placeholder digest is deliberately not silently
rebound. `spec init` is the executable route to a catalog-bound draft.

### Recovery probes

- A schema-1 or schema-2 specification returns `invalid_input` /
  `legacy_tailoring_schema`, starts no source process, and points to exact
  `spec` help.
- A bundle-schema-1 document returns the same stable migration code and is not
  adopted or converted.
- Catalog digest mismatch, unverified command provenance, unknown fields,
  aliases, multiple YAML documents, unsupported option overrides, and an
  identity wrapper with transformations fail before bundle compilation.
- Deprecated `policy init`, `policy validate`, `plan preview`, and `run`
  invocations return only their migration diagnostic and never read the
  historical configuration or start the supplied source command.
- Source inspection timeout, wait failure, or cancellation after process start
  is non-retryable because the outcome is not inferred from an `execute`
  effect.

This scenario validates artifact inspection, specification composition, and
deterministic compilation. Scenario E separately owns wrapper-plan preview and
Scenario F owns the first transform runtime. This scenario does not validate
runtime output transformation, raw bypass, or the caller-owned activation
covered by Scenario G.

## Scenario D: Atsura bundle adoption

### Outcome

Given one current schema-2 bundle, a maintainer can review its exact source,
surface, wrapper, and digest summary on a controlling terminal and explicitly
adopt that purpose-specific CLI definition without granting source-operation
permission.

### Runnable probe

Continue from Scenario C in an interactive terminal:

```sh
go run ./cmd/atr help --format agent
go run ./cmd/atr help bundle trust --format agent
go run ./cmd/atr bundle status --bundle /tmp/atsura-bundle.json
go run ./cmd/atr bundle trust --bundle /tmp/atsura-bundle.json
go run ./cmd/atr bundle status --bundle /tmp/atsura-bundle.json
```

The root index plus exact scoped help meets the two-invocation unknown-surface
bound. Known-path discovery takes one scoped-help invocation. The first status
reports `not_adopted`. Trust displays the exact digest and compiled
surface/wrapper summary on the controlling terminal, counts wrappers whose
source streams may be returned without projection, emits the conditional
control/secret warning when that count is nonzero, and records only the digest
after explicit confirmation. The final status reports `adopted: true`. Status
and trust both report `source_process_attempts: 0`.

### Recovery probes

- Redirected stdin or the absence of a controlling terminal cannot create an
  adoption receipt.
- Source identity drift rejects adoption and points to `bundle status` without
  starting the source executable.
- Changed catalog, specification, surface, wrapper, or bundle bytes produce a
  different digest and do not inherit adoption.
- A malformed or mismatched bundle fails strict loading before confirmation.
- Output failure after confirmed adoption is non-retryable and points to status
  reconciliation rather than repeating the mutation.
- An old exact-digest receipt remains inert for a schema-2 bundle with a
  different digest; migration never copies or removes receipts automatically.

This scenario validates an Atsura-owned adoption-store mutation. Adoption is
not source authorization, command approval, runtime activation, or proof that
hidden commands are inaccessible through another route.

## Scenario E: Atsura zero-execution wrapper-plan preview

### Outcome

Given one adopted current schema-2 bundle, a maintainer can resolve an exact
attempted source invocation into its complete deterministic tailored wrapper
plan and digest without starting the source executable.

### Runnable probe

Continue from Scenarios C and D:

```sh
go run ./cmd/atr help bundle preview --format agent
go run ./cmd/atr bundle preview --bundle /tmp/atsura-bundle.json -- gh pr list
```

The scoped contract publishes the exact
`--bundle <path> -- <source-executable> <argv>` positional-only grammar. Preview
strictly loads the bundle, requires its exact digest to be adopted, and
revalidates the current source path, SHA-256, and size. It accepts only the
requested executable spelling or resolved path bound into the bundle.

Resolution selects the longest matching command prefix from the complete
embedded catalog before evaluating command membership and the matched command's
option surface. If the match has cataloged descendants, a following non-dash
token that does not complete a known child is ambiguous rather than assumed to
be positional; the caller must put an inner `--` before positional data. The
schema-2 JSON envelope contains `plan_digest`, `plan`, and
`source_process_attempts`. Exact schema-10 agent help declares the nested plan
as `wrapper-plan` version 4 and publishes its typed JSON-pointer inventory. The
plan binds:

- bundle, catalog, and specification digests;
- exact source path/hash/size/version and adapter kind/contract version;
- matched command and explicit or inherited surface origin;
- the exact schema-3 specification entry for an explicit match, or JSON `null`
  for an inherited match;
- reason, option surface, and wrapper kind;
- exactly one result mode, `transformed_json` or
  `source_stream_passthrough`;
- original and transformed argv;
- ordered before, invoke, output, and after stages; and
- closed stdin, inherited working-directory and environment modes, plus maximum
  attempts, timeout, stdout, and stderr bounds for a source invocation.

The plan digest is the SHA-256 identity of the canonical complete plan.
Repeating preview with identical validated evidence and argv returns the same
plan and digest. The runnable `spec init` example produces an explicit identity
wrapper entry whose plan declares `source_stream_passthrough`; fixture coverage
also exercises an inherited entry, an append-argv-only wrapper, and a typed
transforming wrapper whose plan declares `transformed_json`. Every success and
failure reports or proves
`source_process_attempts: 0`; no provider credential or network call is needed.

### Recovery probes

- A missing adoption receipt returns `bundle_not_adopted` and points to
  `bundle trust`; invalid adoption storage and current source drift point to
  status reconciliation.
- A different executable spelling than the bundle's requested or resolved path
  returns `source_executable_mismatch`.
- Argv without a cataloged command prefix returns `invalid_invocation`.
- A cataloged command outside the compiled surface returns
  `command_not_in_surface`; an observed option outside the matched option
  surface returns `option_not_in_surface`. Neither produces a plan.
- Unmodeled short options and ambiguous dash-prefixed separated values fail
  closed instead of being inferred.
- An unknown non-dash token immediately after a command with cataloged
  descendants fails as child-versus-positional ambiguity; `--` makes the
  positional intent explicit.
- Legacy `plan preview --config` remains a zero-execution
  `legacy_tailoring_schema` diagnostic; it does not dispatch to `bundle
  preview` or read the retired policy as authority.
- Changed bundle content, source path/hash/size, or malformed plan evidence
  fails before any source-process attempt.
- An output transform with no active cataloged selector, more than one, a
  conflicting input format, or a selector only after `--` fails before a plan.

### Current compatibility limits

The catalog and plan grammar do not yet model source short options,
root/global options, or command-specific positional arguments completely.
Positional values are preserved, but their source-specific dependencies are
not inferred. A command with cataloged descendants requires an inner `--`
before otherwise ambiguous positional data. `append_args` are appended to the
exact attempted argv even when
it already contains `--`; option-looking appended values then remain after the
positional-only marker rather than being silently relocated. Preview requires
one active cataloged selector matching the planned input format only for
`transformed_json`; a source-stream plan has no output selector. Preview alone
does not prove that a selector value encodes the requested select fields. The
GitHub CLI compatibility contract in Scenario F makes that narrower command-
and adapter-specific admission check before transformed execution.

### Acceptance

An agent that knows only the preview outcome reaches its scoped contract with
at most two help-discovery invocations; a known path takes one. It can identify
every plan field from the declared JSON contract, distinguish explicit from
inherited surface origin and both result modes without reconstructing policy,
and select every
recovery command from structured faults. Routine external processing and
source-process attempts are both zero. This acceptance proves plan inspection,
not runtime application, raw execution, or ordinary-command activation.

## Scenario F: Atsura compatibility-admitted JSON transform execution

### Outcome

Given one adopted schema-2 bundle containing a supported transform wrapper, a
maintainer can run one exact GitHub CLI `issue list` or `pr list` invocation and
receive only the configured selected and renamed typed JSON fields.

### Runnable probe

Start with the catalog from Scenario C. Replace the `spec init` identity
wrapper with the transform shown in
[the schema-3 example](../examples/tailoring-spec.schema3.yaml), preserving the
generated catalog digest, then validate, build, and adopt the resulting bundle.
The user invocation deliberately omits `--json`; Atsura's wrapper appends the
exact selector:

```sh
go run ./cmd/atr help bundle execute --format agent
go run ./cmd/atr bundle preview \
  --bundle /tmp/atsura-bundle.json \
  -- gh pr list --limit=1
go run ./cmd/atr bundle execute \
  --bundle /tmp/atsura-bundle.json \
  -- gh pr list --limit=1
```

The exact scoped help publishes the positional-only grammar and fixed schema-2
execution output. Execute strictly reloads the adopted bundle, reassesses the
current source identity, and rebuilds the same wrapper plan used by preview. It
then requires GitHub CLI adapter contract 2, source major 2, exact command
`issue list` or `pr list`, the complete maintained argv grammar, JSON output,
and exactly one inline
`--json=<ordered-select>` before any positional-only marker.

On success, `execution.plan_digest` equals preview's `plan_digest`,
`source_process_attempts` is one, `source.exit_code` is zero, and
`execution.output` preserves object-versus-array shape, final field order, and
selected/renamed typed records. For the example, the fields are
`["id","title","state"]`; raw stdout, stderr, and unselected source fields are
absent. Interpreting these declared fields requires no custom parser, join,
source inspection, provider-notation decoding, or external model call.

The runnable live probe uses the caller's GitHub CLI authentication and current
repository context. It is supporting observation only. The in-process
credential- and network-free synthetic GitHub-compatible fixture is the fast
production-composition gate. Scenario G separately proves the ordinary-command
contract, and Scenario H owns the exact packaged executable on every claimed
native platform.

That production-composition gate executes the complete recovery contract: all
27 preview faults at zero attempts, all 28 execute pre-start phase cases at
zero attempts, and all 15 execute post-start phase cases at one attempt. It
matches exact scoped-help kind, code, retryability, and next actions, uses the
production identity reader for real file drift, and rejects any raw or secret
canary in public output or saved state. Narrow controlled ports supply
deterministic boundary observations; infrastructure tests independently prove
production file, trust, identity, and process fault emission, including native
start, wait, limit, cancellation, timeout, and identity races. The defensive
execute encoding case corrupts the result only after the production service
and controlled process complete one attempt; production application/domain
tests prove the undecorated result boundary. Each five-target native CI row
runs the runner and recovery contracts before exact-archive replay.

### Recovery probes

- Unsupported adapter contract, source major version, command, identity
  wrapper, argv-only transform, output mode, or selector encoding fails with
  zero source attempts.
- Competing `--jq`, `--template`, or `--web` output modes, unmodeled options,
  and positional arguments fail before source start.
- Missing adoption, source drift, surface absence, option absence, and invalid
  invocation fail through the same pre-start contracts as preview.
- Source nonzero exit, timeout, output-limit failure, identity drift after
  start, malformed or duplicate-key JSON, missing selected fields, nonempty
  successful stderr, transform failure, cancellation after start, and final
  output failure are non-retryable after exactly one attempt.
- A failure never returns raw source output, retries with modified argv, drops
  the transform, or falls back to raw execution.
- Source authentication, authorization, and operation semantics remain owned by
  the source CLI; the transform runtime does not reinterpret them as an Atsura
  permission decision.

### Current compatibility limits

This direct `bundle execute` scenario does not cover identity-wrapper
execution, argv-only transforms, nonempty successful stderr, a source CLI
beyond an accepted runtime adapter, raw execution, arbitrary shell/jq/RTK/plugin
transformers, or caller-owned ordinary-command activation. Scenario G owns the
finite identity, append-only, nonempty-stderr, and activation results.
It does not claim that every GitHub CLI major-2 command is supported.
The accepted major-2 range is a maintained compatibility decision rather than
proof that one captured fixture predicts every future 2.x release.

### Acceptance

An agent that knows the exact path obtains its complete execution contract with
one scoped-help invocation; an unknown-path agent uses at most the root index
plus that scoped contract. Preview starts zero source processes, successful
execute starts exactly one, and both identify the same canonical plan. Routine
external interpretation count is zero, and every recovery action comes from a
structured fault rather than from raw source data.

## Scenario G: Host-neutral ordinary-command wrapper

### Outcome

Given one adopted runtime-admitted bundle and a stable installed or built
`atr`, a maintainer can render a deterministic POSIX function, activate it in a
caller-owned Linux or macOS shell, and invoke ordinary `gh` through any of the
three finite result cases: transformed JSON, identity source stream, or append-
argv-only source stream. The same product path admits ordinary no-argument `go
test` for one identity-wrapped bundle carrying a recorded stable Go 1.26.x
observation. No coding-agent-host
protocol or repository-source inspection is part of routine invocation.

### Runnable probe

Use one stable `atr` path. `go run` is not suitable for rendering because its
temporary executable may disappear before the generated function is invoked:

```sh
mkdir -p /tmp/atsura-demo/bin
go build -o /tmp/atsura-demo/bin/atr ./cmd/atr
ATR=/tmp/atsura-demo/bin/atr

"$ATR" help wrapper render --format agent
"$ATR" help wrapper run --format agent
```

Build the catalog and initial specification with requested executable spelling
`gh`, not an absolute source path. The renderer uses that spelling verbatim as
the function name:

```sh
"$ATR" source inspect \
  --adapter github-cli \
  --executable gh > /tmp/atsura-catalog.json

"$ATR" spec init \
  --catalog /tmp/atsura-catalog.json \
  -- pr list > /tmp/atsura-spec.yaml
```

Before validation, replace the generated command's inherited option surface
and identity wrapper with one deliberately narrow admitted transform. The
command must expose only `--limit`; generated `--json` remains an invocation
stage rather than an agent-visible option:

```yaml
options:
  default: exclude
  include: [--limit]
  exclude: []
wrapper:
  kind: transform
  before: []
  invoke:
    append_args: ["--json=number,title,state"]
  output:
    input: json
    select: [number, title, state]
    rename:
      - from: number
        to: id
    render: compact_json
  after: []
```

Then validate, build to an absolute bundle locator, adopt the exact digest in a
controlling terminal, and render the fixed function bytes:

```sh
"$ATR" spec validate \
  --catalog /tmp/atsura-catalog.json \
  --spec /tmp/atsura-spec.yaml

"$ATR" bundle build \
  --catalog /tmp/atsura-catalog.json \
  --spec /tmp/atsura-spec.yaml > /tmp/atsura-bundle.json

"$ATR" bundle trust --bundle /tmp/atsura-bundle.json
"$ATR" wrapper render \
  --bundle /tmp/atsura-bundle.json > /tmp/atsura-gh-wrapper.sh

. /tmp/atsura-gh-wrapper.sh
gh pr list --limit=1
unset -f gh
```

That is the `transformed_json` case. Repeat the build/adopt/render/activate
sequence with two separate schema-3 specifications:

- keep the generated identity wrapper to obtain
  `source_stream_passthrough`, then invoke ordinary `gh pr list --limit=1`; and
- use an output-less transform whose only action is
  `invoke.append_args: ["--limit=1"]`, then invoke ordinary `gh issue list`
  without supplying `--limit` at the call site.

Each bundle contains exactly one included command and only an option surface
covered by the maintained GitHub CLI runtime grammar. These are three reviewed
bundles and three ordinary invocations, not runtime mode selection or fallback.

For the nature-distinct second source, run from a reviewed Go module where the
inspection probe records stable Go 1.26.x. The generated draft is already the only admitted
identity wrapper, so no transform edit is needed:

```sh
"$ATR" source inspect \
  --adapter go-cli \
  --executable go > /tmp/atsura-go-catalog.json

"$ATR" spec init \
  --catalog /tmp/atsura-go-catalog.json \
  -- test > /tmp/atsura-go-spec.yaml

"$ATR" spec validate \
  --catalog /tmp/atsura-go-catalog.json \
  --spec /tmp/atsura-go-spec.yaml

"$ATR" bundle build \
  --catalog /tmp/atsura-go-catalog.json \
  --spec /tmp/atsura-go-spec.yaml > /tmp/atsura-go-bundle.json

"$ATR" bundle trust --bundle /tmp/atsura-go-bundle.json
"$ATR" bundle preview --bundle /tmp/atsura-go-bundle.json -- go test
"$ATR" wrapper render \
  --bundle /tmp/atsura-go-bundle.json > /tmp/atsura-go-wrapper.sh

. /tmp/atsura-go-wrapper.sh
go test
unset -f go
```

Inspection performs exactly three source attempts. Path/hash/size identify the
direct `go` launcher; `Source.Version` is the possibly delegated effective
toolchain observed by `go version` under the inspection working directory and
environment. Preview performs zero and
ordinary `go test` performs one. The finite registry selects Go CLI contract 1
from the bundle and plan adapter kind; it does not introduce a Go-specific
plan, executor, or result. `go test` remains source-owned `EffectExecute` and
may compile and run repository code, use credentials or configuration, resolve
modules, access networks, and mutate caller-owned files or caches. This probe
is an invocation contract, not a sandbox or authorization claim. Runtime does
not repeat the version probe, so the same launcher may select or download a
different toolchain because of module state, `GOTOOLCHAIN`, `GOROOT`, or related
ambient inputs without a pre-start rejection.

The generated function contains the complete `wrapper run` contract-version-1
closure and always inserts the explicit `--` separator before `"$@"`. Users do
not copy the bundle digest or runtime identity into a second command. On
transformed success, ordinary `gh` stdout is exactly one compact JSON object or
array plus LF and stderr is empty. The source JSON supplies container and value
types, while the fresh schema-4 plan governs selection, rename, order, compact
rendering, and `result_mode`. On a conventionally completed source-stream
case, stdout and stderr are the exact separately bounded source bytes and the
ordinary command returns the source status only after both final writes
complete. Atsura adds no LF, envelope, projection, or UTF-8 interpretation and
makes no timing or stdout/stderr-interleaving claim.

For each bundle, `wrapper render --format json` returns the same source in a
schema-1 review envelope with its SHA-256, command name, contract, bundle
locator/digest,
current `atr` path/hash/size, and zero source attempts. That source digest is
review evidence. Sourcing or modifying the function is caller-owned, so it is
not runtime attestation.

### Recovery probes

- A relative/unclean bundle path or a requested executable that is not a
  portable non-reserved POSIX Name returns `invalid_wrapper_binding` and no
  source bytes.
- Windows returns `wrapper_platform_not_supported`, empty success stdout, one
  structured fault on stderr, and zero wrapper source attempts. It does not
  claim POSIX activation.
- A mixed, multi-command, partially admitted, or otherwise unsupported complete
  surface returns
  `wrapper_runtime_not_supported` before rendering.
- A catalog recording a version outside stable Go 1.26.x, a non-`test` command,
  a non-identity wrapper, any option or package pattern, `--`, or a test-binary argument returns
  `wrapper_runtime_not_supported` before the Go test process starts. Expanding
  either the recorded source-version range or argv grammar requires new
  evidence. A later effective Go 1.27 selection by the same launcher is not
  detected by this contract and is not this recovery case.
- A nil, unknown, duplicate, or misconfigured compatibility verifier fails as
  `adapter_contract`; the registry never tries the other source adapter.
- A changed bundle digest, missing adoption, source drift, malformed closure,
  or honest current-`atr` path/hash/size mismatch starts zero source processes
  and points to render, status, or trust recovery as declared.
- Empty forwarded argv still reaches `wrapper run` after its explicit `--`, but
  cannot resolve a cataloged command and fails `invalid_invocation` at zero
  attempts.
- Spaces, empty values, Unicode, duplicate values, dash-prefixed values, and
  literal shell metacharacters remain separate argv elements; the fixed
  function uses no `eval` or `sh -c`.
- A conventional source nonzero status in `source_stream_passthrough` returns
  the exact bounded streams and the same status; it is not an Atsura fault or a
  replay recommendation.
- Signal/abnormal termination, timeout, cancellation, capture overflow, wait or
  identity uncertainty, and inconsistent process evidence are non-retryable,
  expose neither captured stream, and never select raw execution, another
  bundle, or ambient `gh` as fallback.
- A short stdout or stderr final write is non-retryable
  `execute_output_write_failed`. Already-written caller bytes cannot be
  retracted, the source status is not returned, and replay is not recommended.
- The function starts the bound absolute `atr` path before honest `wrapper run`
  code can verify that executable. Drift detection prevents that honest
  mismatched runtime from starting the source; it does not attest or sandbox
  malicious replacement code already executing at the path.

### Acceptance

An agent that knows only the ordinary-command outcome reaches `wrapper render`
and `wrapper run` through root plus one selected wrapper scope; a known path
takes one scoped-help invocation. After caller-owned activation, routine use is
the ordinary `gh` or exact no-argument `go test` invocation with zero external
reconstruction. Direct preview
and wrapper application use the same fresh plan and plan digest for identical
validated inputs; the generic fixture, not wrapper stdout, compares that
evidence. Each admitted case has exactly one successful source attempt, the
three-case GitHub fixture has three, the Go case has one, and every pre-start
rejection has zero. Vendor-specific activation remains downstream.

## Scenario H: Exact installed-artifact transform and wrapper journey

### Outcome

The same `atr` executable that would be published for a claimed target can be
extracted from its immutable archive and can close the finite GitHub CLI
transform journey on that target without a repository-built replacement
binary, provider credential, provider network call, or undeclared parser. On
Linux and macOS the same extracted executable must also render and serve the
transformed-JSON, identity, and append-argv-only ordinary-command POSIX cases.
Every native target must record stable Go 1.26.x inspection evidence through contract 1; Linux and
macOS must additionally serve one exact identity-wrapped no-argument `go test`,
while Windows proves the exact unsupported-render result at zero attempts for
both source bundles while retaining the GitHub transform journey.

### Automated probe

For a target-native host, the release harness runs:

```sh
scripts/test-release-artifact.sh \
  <tag> <revision> <goos> <goarch> <exact-archive>
```

The standard-library orchestrator safely extracts the archive, checks the host
tuple and embedded `atr <version> (<revision>)`, and uses that extracted path
for every public command. A native synthetic source supports only the exact
four GitHub CLI inspection probes and the admitted `issue list` and `pr list`
invocations. Its append-only JSONL log is outside public output.

The replay starts from an isolated user-config root. Before starting the source
fixture, packaged `atr` must return schema-10 root help plus exact `source
inspect`, `spec init`, `spec validate`, `bundle preview`, `bundle execute`,
`wrapper render`, and `wrapper run` scopes. It checks the complete
catalog/specification output-schema field
inventory and the exact ordered 27-fault preview and 41-fault execute recovery
signatures rather than recognizing prose markers alone. It then obtains the
catalog in four fixture attempts and, for
each admitted command, asks packaged `atr` for an identity draft, applies the
same documented finite transform edit with option default `exclude` and only
`--limit` included, validates and builds it, and observes
`not_adopted/current`. Pre-adoption preview and execute fail without another
fixture attempt. The non-shipped orchestrator then loads each exact bundle and
adds its digest through the production trust-store adapter. This step proves
receipt consumption only; it is not recorded as interactive human consent.
Production tests separately require a controlling terminal, display the full
authority summary, require the full digest, and reject other input.

After receipt seeding, status reports `adopted/current`, preview reports zero
source attempts, and compatibility conflict probes still start no source. Each
induced fault must match the applicable packaged help declaration for kind,
code, retryability, and exact next actions. Post-start fixture failures each
add exactly one attempt, are non-retryable, and expose none of the stdout,
stderr, or unselected-field canaries. Successful execute adds exactly one
attempt per command, returns fields `["id","title","state"]`, omits the
unselected canary, and has the same command-specific plan digest as preview.

Linux and macOS also build and adopt three wrapper bundles: the existing
transformed-JSON `pr list` case, one identity case, and one output-less fixed-
argv-append case. For each bundle they render JSON review material and raw
function text from the exact extracted `atr`, compare the exact source and
SHA-256, source that fixed material in an isolated generic POSIX shell, and
invoke ordinary `gh`. Every wrapper/preview schema-4 plan identity must match
in bounded fixture evidence and the append-only log must add exactly one source
attempt per case. The transformed result must equal its compact JSON value.
The other two must match the fixture's exact bounded stdout/stderr digests and
conventional status without storing either stream. Windows must instead
receive `wrapper_platform_not_supported`, no rendered source digest or case,
and zero wrapper source attempts.

Every target also uses packaged `atr` to obtain a stable Go 1.26.x observation
with `go version`, `go help`, and `go help test`. The harness
builds an exclude-by-default identity specification for only `test`, validates,
builds, seeds its exact receipt, and proves preview has zero attempts. Linux and
macOS render the ordinary `go` function from the extracted `atr`, first invoke
`go test extra`, and require `wrapper_runtime_not_supported` with zero Go
attempts and exit 12. They then invoke exact no-argument `go test` once in a dependency-free synthetic module with
`GOTOOLCHAIN=local`, isolated Go/cache roots, and downloads disabled. These are
fixture conditions, not production guarantees. Windows performs the inspection
but requires `wrapper_platform_not_supported`, no Go wrapper case, one zero-
attempt rejection, and zero Go wrapper source attempts. Isolation is fixture
discipline, not an Atsura sandbox claim.

The bounded journey document required for this source-stream candidate uses
evidence schema 4. Linux/macOS retain the GitHub `wrapper_outcome:
ordinary_command_verified`, an ordered three-entry `wrapper_cases` inventory,
and `wrapper_source_process_attempts: 3`. Case names occur in the fixed order
`transformed_json`, `identity`, `append_only`. Each case binds `name`,
`wrapper_kind`, `result_mode`, `bundle_digest`, `plan_digest`,
`wrapper_source_sha256`, `stdout_sha256`, `stderr_sha256`,
`source_exit_code`, and `source_process_attempts: 1`. Windows records
`wrapper_outcome: platform_not_supported`, an empty `wrapper_cases` inventory,
and zero attempts.

The required `go_source` object separately binds
`atsura.source.go_cli` contract 1, a recorded stable `go1.26.x` observation, three source-
inspection attempts, `commands_verified: ["test"]`, and exact catalog, bundle,
and plan digests. Linux/macOS record one `go_test_identity` case with identity
wrapper, `source_stream_passthrough`, a nonempty rendered-source digest, stdout/stderr digests,
status zero, one source attempt, and one preceding zero-attempt rejection.
Windows records
`platform_not_supported`, an empty case list, one zero-attempt rejection, and
zero Go wrapper source attempts. Together with the four GitHub inspection and
two direct success attempts plus induced failures, the fixed GitHub fixture-
attempt total remains 13 on Linux/macOS and 10 on Windows. These are acceptance
requirements, not a claim here that a gate run has passed.

### Platform acceptance

CI runs this probe natively on Linux amd64, Linux arm64, macOS amd64, macOS
arm64, and Windows amd64. The release workflow downloads the exact archive
uploaded by its build job and blocks publication until all five native replays
succeed. The four POSIX rows must close all three GitHub ordinary-command result
cases plus the exact Go identity case; the Windows row must close both exact
structured unsupported-render contracts and does not count as POSIX activation.
Each job uploads a bounded
document bound to its target, archive digest, and exact revision. A dependent
job accepts exactly those five
canonical documents, validates the complete fixed journey facts, and emits a
path-free unattested digest index after recomputing all five candidate archive
hashes. Cross-compilation, build metadata, aggregation of locally fabricated
JSON, emulation, and the current host's local replay are not substitutes for
the five native job results.

On 2026-07-22, the exact packaged Darwin/arm64 journey passed for revision
`b4ade8c`, including ordinary-command activation. That bounded observation does
not cover this later documentation tree, schema-4 source-stream plans, evidence
schema 4, the identity or append-argv-only GitHub cases, the Go inspection and
identity-wrapper case, Linux, macOS amd64, Windows, evidence aggregation,
publication, or the complete release matrix; the tagged revision must replay
every required row.

Exact scoped help is the public authoring contract: the source catalog exposes
command paths, provenance, option grammar, structured output selector, and
fields; schema-3 help exposes surface, option, wrapper, select, rename, and
render constraints; execute help exposes the finite runtime-admission matrix;
wrapper help exposes the renderer-produced closure, explicit `--` argv
boundary, platform matrix, static review envelope, and the exact fresh-plan
result-mode union.
The harness's deterministic YAML edit verifies those artifact contracts but
does not erase the user's deliberate configuration-authoring step.

This journey does not run RTK or validate an optimizer default. Pass-only `go
test -json` with RTK's fixed `go-test` filter is the next research candidate,
but skip-only, malformed, nonzero-status, and failure-order behavior remains
unresolved and outside evidence schema 4.

## Review record

Record the invocation transcript, number of discovery round trips, routine
external-processing count, selected output/reference fields, exact next argv,
and each recovery probe in the active work packet. If an agent needs prose
interpretation, source inspection, URL parsing, hidden filtering, a custom
join/parser, provider-notation decoding, an exploratory request, or an extra
command guess, treat that as product/thesis evidence rather than teaching the
agent a workaround.
