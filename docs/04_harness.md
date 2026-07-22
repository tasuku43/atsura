# Harness

The harness turns Atsura's product, architecture, and security claims into
repeatable checks. A capability is complete only when `task check` passes. The
current transform and host-neutral wrapper milestone also requires `task
security` because it admits an attempted source invocation, observes source and
runtime executable identity, exposes a canonical plan contract, and can start
one compatibility-admitted source process.

Do not weaken a check to preserve the superseded authorization-centered model.
Update the governing contract and its mechanical claim together.

## Required commands

```sh
task check:fast
task check
task security
task public:check
task release:check
```

The underlying stable interface is:

```sh
./scripts/check.sh fast
./scripts/check.sh full
./scripts/check.sh security
./scripts/check.sh public
./scripts/check.sh release
```

`task check` is the implementation completion gate. Publication additionally
requires `task public:check`; release additionally requires
`task release:check`. Public and release profiles are not implementation
completion gates for the current non-publication milestone unless its changes
affect their tracked fixtures. They remain required evidence before any
publication or release.

The wrapper work packet deliberately runs both profiles as broad
regression evidence because it changes public documentation and release claims.
Passing them does not replace the exact-artifact runtime replay required before
a first tag.

## Executable claims

### Repository identity and layering

Tests and lint must prove:

- `.harness/project.json` remains the identity source of truth;
- `cmd/atr` and `internal/cli` remain the public composition root;
- domain has no outward dependency, application does not import
  infrastructure, and adapter packages do not define product policy; and
- YAML and other third-party codecs remain confined to the approved layer.

### `tools/bootstrap`

Bootstrap tests preserve the foundry identity recorded in
`projectconfig.Defaults` as protected provenance while previewing and applying
only the derived identity supplied through the project manifest. They cover
transactional replacement, path renames, rollback, Go formatting, and residue
cleanup. Product-direction changes must not reinterpret those protected
defaults or bypass the bootstrap transaction.

### Source catalog evidence

Contract tests must prove:

- source adapters declare namespaced kind, contract version, exact probe argv,
  and finite attempts/time/bytes; adapter contracts and tests define and
  enforce their accepted source-version range;
- GitHub CLI contract 2 executes exactly four fixed offline probes, while Go
  CLI contract 1 executes exactly `go version`, `go help`, and `go help test`;
- Go inspection accepts a stable Go 1.26.x effective-toolchain observation
  from `go version` under the probe working directory/environment, requires the
  bounded root command table and no-argument test-help anchors, emits exact
  `test` catalog evidence, and rejects version/help drift without persisting raw
  probe output;
- source identity and probe evidence are preserved in the catalog;
- verified built-ins, observed extensions, and unverified dynamic entries stay
  distinct;
- alternate synthetic source adapters satisfy shared contracts without
  GitHub-specific or coding-agent-host fields; and
- catalog evidence is never interpreted as allow/deny or source safety.

Inspection command catalog entries declare `EffectExecute` because their
bounded probes start a source-owned process.

### Tailoring specification schema 3

Domain, codec, application, and CLI tests must prove:

- schema version 3 is required and catalog-bound;
- `surface.default` is explicitly `inherit` or `exclude`;
- commands are exact, sorted, unique, verified catalog paths with bounded
  reasons;
- presence is explicitly `include` or `exclude`;
- included entries have an explicit option surface and complete wrapper;
- excluded entries have neither options nor wrapper;
- option defaults and overrides are exact, sorted, unique, disjoint, and
  catalog-observed;
- identity wrappers contain no transformations;
- transform wrappers contain at least one supported transformation;
- before/after lists are explicit and reject unsupported actions;
- the current schema rejects every shell, script, jq, RTK, plugin, external-
  processor, and runtime-LLM action; acceptance of a future finite RTK adapter
  does not make arbitrary executable names or argv valid; and
- bounded strict decoding rejects aliases, excessive depth/nodes/bytes,
  duplicate or unknown fields, and trailing documents.

Canonical round-trip fixtures must produce stable normalized bytes and digest.

### Surface composition and resolution

Pure domain tests own the complete truth table:

| Default/entry | Expected surface result |
|---|---|
| `inherit`, no entry, verified built-in | Included with inherited options and identity wrapper |
| `exclude`, no entry | Absent |
| explicit `include` | Included exactly as declared |
| explicit `exclude` | Absent with no wrapper |

Negative tests must prove that an excluded entry with a wrapper, an included
entry without a complete wrapper, and a wrapper-only or membership-only
shortcut are invalid. Resolving an absent command returns
`command_not_in_surface`, produces no plan, and starts zero source processes.

### Bundle schema 2 and adoption

Tests must prove:

- a canonical bundle binds exact source identity, adapter evidence, normalized
  catalog and digest, normalized schema-3 specification and digest, and the
  derived surface with wrappers;
- canonical bytes exclude timestamps, machine/user identity, credentials,
  source output, and random values;
- every embedded digest and derived surface is recomputed on load;
- catalog, specification, surface, wrapper, source, and bundle drift are
  distinguishable;
- alternate vendor-neutral adapter fixtures compile to the same shared bundle
  contract; and
- schema-1 bundles are rejected rather than reinterpreted.

Adoption tests must prove that only the full exact digest is accepted through a
controlling terminal; the receipt is user-local and content-bound; unrelated
receipts survive replacement; malformed or unsafe storage fails closed; and
the review summary counts surface and wrapper facts rather than source
permissions, decisions, or inferred effects.

Trust-store writes remain Atsura-owned create/write mutations and must pass
through the central mutation invoker with exact target and impact contracts.

### Wrapper plan contract

`bundle preview --bundle <path> -- <source-executable> <argv>` is the current
zero-execution plan boundary. Tests must prove:

- only a strictly loaded schema-2 bundle with an exact valid adoption receipt
  is admitted;
- current source path, SHA-256, and size are observed and must equal the
  bundle-bound identity before plan construction;
- the supplied executable is exactly the bundle's requested spelling or
  resolved path;
- matching selects the longest command prefix from the complete embedded
  catalog before command and option membership are evaluated;
- when the match has cataloged descendants, a following non-dash token that
  does not complete a known child fails as ambiguous; an explicit inner `--`
  is required before positional data;
- an absent command returns `command_not_in_surface`, an absent observed option
  returns `option_not_in_surface`, and both produce no plan;
- an explicit surface match includes the exact specification entry, while an
  inherited match encodes `specification_entry: null`;
- the plan binds bundle/catalog/specification digests, source and adapter
  identity, matched command and surface origin, wrapper kind, reason, option
  surface, original/transformed argv, and ordered before/invoke/output/after
  stages plus exactly one schema-4 result mode;
- the invocation stage declares closed stdin plus inherited working directory
  and environment modes without serializing ambient values;
- the invoke stage declares exactly one maximum attempt plus finite timeout,
  stdout, and stderr bounds, even though preview never crosses that boundary;
- `append_args` appear exactly at the end of transformed argv;
- an output transform requires exactly one active cataloged selector matching
  its input format before `--`; missing, duplicate, conflicting, or positional
  selectors fail plan construction;
- an output stage derives `transformed_json`; a complete identity or append-
  argv-only wrapper without an output stage derives
  `source_stream_passthrough`; a missing, unknown, or contradictory mode fails;
- identical validated inputs produce identical canonical plan bytes and
  `plan_digest` values;
- the schema-2 preview envelope contains exactly `plan_digest`, `plan`, and
  `source_process_attempts`, with the attempt count always zero;
- exact schema-10 agent help publishes the versioned `wrapper-plan` inventory,
  including nested JSON-pointer paths, scalar/object/array types, array element
  types, requiredness, and nullable object states; and
- wrapper stages contain no allow/confirm/deny or source
  read/create/write/target/impact fields.

Tests must retain fail-closed evidence for the current grammar boundary:
unmodeled short options fail, root/global and command-specific positional
grammar are not inferred, ambiguous descendant-versus-positional tokens require
an inner `--`, and values after `--` are positional. Appended option-looking
argv after `--` is never silently moved; when it is the required output
selector, plan construction fails because it is positional. Tests prove the
active selector flag and declared input format, not that the selector value
encodes the plan's requested select/rename fields. That encoding requires a
source-adapter runtime fixture.

Runtime uses this same plan constructor and tests exact plan-digest equality
with preview for the same admitted input. It rebuilds the plan and may not
consume an old preview as authority.

### Transform runtime contract

`bundle execute --bundle <path> -- <source-executable> <argv>` is the first
public source-runtime boundary. Tests must prove:

- the same strict bundle, adoption, current-identity, surface, option, and plan
  checks as preview complete before source start;
- the current GitHub CLI runtime adapter accepts only its exact contract,
  compatible major version, `issue list` or `pr list`, JSON output mode, and
  one selector whose ordered value exactly equals the plan's selected fields;
- unsupported adapters, versions, commands, identity wrappers, argv-only
  transforms, missing or mismatched selectors, and unmodeled invocation forms
  fail with zero source-process attempts;
- execution is bound to the plan's exact resolved path, SHA-256, and size and
  revalidates that identity before start and after wait;
- the source process starts at most once with exact argv, no shell, closed
  stdin, inherited working directory and environment, and finite timeout and
  output bounds;
- successful nonempty source stderr, nonzero exit, malformed or duplicate-key
  JSON, missing selected fields, limit failures, cancellation after start,
  identity drift, transform failure, and final output failure are
  non-retryable and never expose raw source output;
- successful output contains only the fixed execution envelope, selected and
  renamed typed JSON fields, plan and bundle digests, matched command,
  transformation metadata, exit code, and the exact attempt count of one;
- a missing selected field fails, while explicit empty, zero, false, null,
  lexical number values, object versus array shape, field order, nested JSON
  types, and visible projection of structural external text remain distinct;
- secret-shaped canaries in unselected fields and source stderr do not appear
  in success output, faults, persisted bundles, receipts, or diagnostics.

### Source-stream ordinary-wrapper runtime contract

`wrapper run` extends the shared plan application service without broadening
the direct `bundle execute` result envelope. Tests must prove:

- one finite application registry structurally satisfies both fresh-plan and
  whole-surface compatibility ports, dispatches the exact unchanged plan or
  bundle by adapter kind, preserves valid finite admission categories, and
  rejects nil, typed-nil, empty, unknown, duplicate, or misconfigured entries
  as `adapter_contract` without dispatch;
- the complete GitHub CLI adapter/version/command/surface/long-option grammar
  is admitted before source start for one identity or append-argv-only wrapper,
  without requiring a JSON selector;
- Go CLI contract 1 admits only a recorded stable Go 1.26.x inspection
  observation, exact command `test`, one complete identity wrapper,
  `source_stream_passthrough`, no observed long-
  option or structured-output surface, and no argv element after `test`;
- every Go option, package pattern, `--` marker, test-binary argument, other
  command, non-identity wrapper, and catalog recording a version outside stable
  1.26.x fails before source start;
- tests distinguish direct-launcher path/hash/size from the possibly delegated
  `Source.Version` observation and do not claim that runtime repeats `go
  version`, detects a later effective-toolchain change, or binds a selected
  toolchain/GOROOT tree;
- preview and wrapper application rebuild byte-identical schema-4 plans and
  plan digests for `source_stream_passthrough`;
- source stdout and stderr are returned byte-for-byte within 4 MiB and 256 KiB,
  including empty streams, NUL, non-UTF-8, control-looking, and prompt-like
  bytes, with no projection, redaction, envelope, or added LF;
- status zero with nonempty stderr and conventional nonzero status with both
  streams are valid source results; nonzero status is returned unchanged and
  receives no Atsura retry advice;
- signal or abnormal termination, timeout, cancellation, stdout/stderr
  overflow, wait uncertainty, identity uncertainty, and inconsistent result
  evidence remain one-attempt non-retryable faults and suppress both streams;
- stdout and stderr final writers are tested separately for short/error writes;
  `execute_output_write_failed` does not return source status or recommend
  replay and documents that already-written caller output cannot be retracted;
- successful final delivery performs one complete stdout write, then one
  complete stderr write, then returns source status; tests and help make no
  timing or cross-stream-interleaving claim;
- empty argv elements, spaces, Unicode, dash-prefixed values, `--`, repeated
  options, literal shell metacharacters, and order survive the admitted argv
  grammar without shell reconstruction;
- no source-stream byte appears in a fault, bundle, trust receipt, evidence
  document, log, or transcript; evidence stores only fixed digests; and
- trust-summary derivation counts source-stream-result wrappers and the
  controlling terminal emits a conditional warning without persisting bytes.

### Deferred original-preserving optimizer contract

`tailoring.output.optimize` remains deferred. Schema 3 and the current runtime
continue to reject RTK and arbitrary external-processor actions. ADR 0007
selects a future contract, while ADR 0009 rejects the ambiguous `v0.43.0`
`git-log` candidate. ADR 0011 identifies pass-only `go test -json` plus the
fixed RTK `go-test` filter as the next candidate, but does not accept it as a
tuple. Neither the current transform-runtime milestone nor its passing gates
claim an optimizer, an RTK authoring default, or RTK runtime compatibility.

An implementing slice may change that ledger state only after tests prove:

- projection and original-preserving optimizer are disjoint typed contracts;
  projection never preserves failed input, while successful valid processor
  stdout byte-equal to admitted input is `preserved` and different valid stdout
  is `optimized`; preservation is valid only when the adopted plan explicitly
  permits the original stage input as agent-facing output, and the disposition
  makes no claim about RTK's internal branch;
- the preferred backend, exact processor contract, filter, and reason are
  materialized by the authoring workflow into canonical specification, bundle,
  plan, and trust-summary facts; an explicit authoring override remains
  reviewable, and compilation and execution never discover or insert ambient
  RTK implicitly;
- source adapter, source version, command contract, source argv, processor path,
  SHA-256, size, version, adapter contract, filter, and claimed platform form
  one exact compatibility tuple; the trust summary exposes the processor kind,
  version and identity, contract/filter mapping, and original-output visibility;
- missing or drifted processor identity at preflight fails before either
  process starts; after admitted source success, Atsura revalidates processor
  identity before start, and a change then is non-retryable with one source
  attempt and zero processor attempts;
- Atsura starts the source at most once and starts the processor at most once
  only after admitted source success; source failure produces one source
  attempt and zero processor attempts, while every post-source processor
  failure is non-retryable and leaks no source bytes, processor bytes, or
  processor stderr;
- the processor uses no shell, receives only the bounded admitted stage input,
  and runs with finite time and byte limits in isolated working and
  configuration roots; native fixtures observe the exact invocation's file
  effects and attempted network I/O within declared platform capabilities,
  without claiming an OS/network sandbox, and tests retain the portable
  processor check-to-exec race as a stated limitation; and
- every claimed platform replays an exact native RTK artifact and records
  Apache-2.0 provenance, license, notice, dependency, and SBOM evidence.
- each filter fixture covers its literal delimiters, grouping keys, truncation
  boundaries, and association rules with hostile valid source data; exit zero,
  empty stderr, and smaller output never substitute for semantic validation.
- the Go candidate additionally resolves skip-only classification, malformed-
  line omission, nonzero-status preservation, and deterministic failure
  ordering before any pass-only source result becomes eligible.

### Source execution effect and process boundary

Operation tests must prove:

- `EffectExecute` validates and has stable JSON/text encoding;
- unknown effects fail closed;
- Execute is not treated as Read, Create, or Write;
- Execute carries no Atsura mutation contract; and
- every mutation-only switch handles Execute explicitly rather than relying on
  `effect != read`.

Existing source-inspection process tests retain exact executable/argv,
no-shell, identity revalidation, time/byte limits, declared attempt counts, and
non-retryable post-start uncertainty. `bundle preview` is `EffectRead` and
starts no source process. `bundle execute` is `EffectExecute`, has no Atsura
mutation contract, and starts at most one source process after compatibility
and identity checks succeed.

Go runtime tests must also keep `go test` classified as source-owned
`EffectExecute`, not Read or an Atsura-owned mutation. The tests prove only
exact identity/argv, zero/one attempts, and failure classification; they do not
claim to contain or authorize repository code, module resolution, credential
use, network access, or caller-owned file/cache changes.

### Retired-schema migration

Fixtures for specification schemas 1 and 2 and bundle schema 1 must prove:

- retired documents are rejected before adoption or source start;
- no allow/confirm/deny, source read/create/write, target, or impact value is
  silently converted;
- deprecated public paths return the stable migration fault and an exact
  catalog-declared recovery command;
- migration diagnostics persist no trust or configuration state; and
- every migration path starts zero source processes.

### CLI catalog and output

Catalog tests must prove:

- every current and migration-only public command is registered once in
  `cli.Catalog`;
- help, routing, typed parsing, capabilities, effects, outputs, faults, and
  recovery paths derive from that catalog;
- current output uses `specification`, `surface`, and `wrapper` vocabulary;
- every command selects exactly one authority for result interpretation and
  presentation; `wrapper run` is explicitly `fresh_wrapper_plan`
  authoritative, while `wrapper render` and other current commands retain
  their catalog-static field, envelope, and schema-version contracts, with
  help's tested root-index/scoped-contract variants kept explicit;
- `fresh_wrapper_plan` authority has no static result fields or maintainer
  envelope and publishes an exact typed `plan_result_modes` union. One variant
  is compact object-or-array JSON plus LF/empty stderr/status zero; the other is
  exact bounded source stdout/stderr, no framing, buffered delivery, and
  conventional source status with no timing/interleaving claim. Whole-catalog
  validation resolves the exact `bundle preview` `plan`/`wrapper-plan`
  schema-4 reference and rejects an incomplete or unknown result variant;
- retired `policy` vocabulary appears only in migration diagnostics or
  historical superseded documents;
- exact output schemas reject undeclared fields and preserve absent versus
  explicit empty where meaningful; and
- root agent help remains a bounded capability index, with details reachable
  by exact command or namespace help.

### Host-neutral wrapper contract

`wrapper render` and `wrapper run` are the public entry points for
`tailoring.wrapper.materialize`. Their implementation is complete only when the
catalog, capability status, runtime, security contracts, generic caller journey,
and required gates agree on the same tree. Documentation of the command
contract is not evidence that those gates or native journeys passed.

The slice must prove:

- one exact adopted purpose bundle produces deterministic wrapper material and
  a canonical rendered-byte digest;
- `wrapper render --bundle <absolute-path> [--format text|json]` is read-only,
  defaults to raw sourceable text, and returns the exact schema-1 review
  envelope in JSON without a source attempt;
- POSIX rendering succeeds only on Linux and macOS; Windows returns exact
  structured `wrapper_platform_not_supported`, emits no wrapper bytes, and has
  no POSIX activation claim;
- the bundle's requested executable is used verbatim only when it is a portable
  non-reserved POSIX Name; no basename or path normalization invents a command;
- the complete included surface contains exactly one command and result mode
  covered by its registry-selected verifier before bytes are rendered: GitHub
  CLI contract 2 admits finite `issue list` / `pr list` transform, identity, or
  append-only surfaces; Go CLI contract 1 admits only identity-wrapped `test`
  with no observed long-option or structured-output surface;
- the wrapper binding contains only wrapper contract, bundle identity, runtime
  identity, source identity, and ordinary command spelling;
- no bundle, binding, plan, result, help, or capability field names a
  coding-agent host, hook event, permission value, settings path, session,
  transcript, or model;
- `tools/archlint` reserves the exact production package-path segments
  `agenthost`, `hostadapter`, `hostintegration`, `claudehook`, and `codexhook`
  for the out-of-product responsibility while allowing source/output adapters
  and source vendor names;
- default-catalog tests reject the exact retired Claude Code and Codex
  integration/hook routes and capability identifiers without reserving a
  generic `integration` namespace;
- the generated function passes the complete render-produced closure to
  `wrapper run`, includes the explicit `--` separator, accepts exact argv rather
  than a shell command string, and reaches the same plan constructor and
  source-execution boundary as the direct gateway;
- spaces, empty values, Unicode, dash-prefixed values, literal metacharacters,
  and ordering survive wrapper forwarding without `eval`, `sh -c`, or shell
  reconstruction;
- missing adoption, runtime or bundle mismatch, source drift, absent surface
  command, invalid option, or unsupported runtime
  starts zero source processes;
- the runtime identity check is described and tested as cooperative drift
  detection after the bound `atr` path starts, not attestation or containment
  against malicious replacement at that path;
- admitted success starts the exact physical source once, applies the same
  typed stages, emits only the plan-declared result, and never selects raw or
  another bundle as fallback;
- wrapper success uses the exclusive `fresh_wrapper_plan` interpretation and
  presentation authority and emits no maintainer evidence envelope; exact
  scoped schema-10 help publishes both typed result modes and points to the
  `bundle preview` wrapper-plan schema governing the selected variant;
- the current renderer persists nothing and edits no activation configuration;
  any future persisted artifact lifecycle uses exact ownership, bounded
  regular-file paths, symlink/special-file rejection, atomic replacement,
  central mutation invocation, and read-only drift reconciliation without
  editing caller-owned activation configuration; and
- the exact installed release artifact replays the ordinary-command journey on
  every claimed Linux and macOS native target, while Windows replays existing
  commands and the exact unsupported-render result.

Consumer conformance uses a non-shipped generic caller-owned activation fixture
that invokes the exact installed `atr` artifact. It compares rendered bytes and
digest, bundle, ordinary argv, transformed argv, plan, result, and zero/one-
attempt evidence without importing any coding-agent-host protocol. Downstream
vendor integrations own their own activation and compatibility tests outside
Atsura. Failure to activate a wrapper is outside Atsura's fail-closed claim
because the product boundary begins only after the wrapper was selected.

### External text and secrets

Hostile fixtures must cover source help, catalog labels, YAML strings, source
output, paths, and wrapper bindings containing controls, Unicode separators,
prompt-like text, and secret-shaped values. Visible rendering must preserve
terminal and JSON/TSV framing without filtering printable meaning. Persistent
fixtures must assert that credentials, raw stdout/stderr, environment
snapshots, transcripts, and agent reasoning are absent.

For an adopted `source_stream_passthrough` result, hostile byte fixtures instead
prove exact unprojected delivery after conventional completion and suppression
after uncertainty. They must not decode the streams as text or place their raw
bytes in persisted or structured evidence.

## Test ownership

- Domain tests own specification, surface, wrapper, bundle, digest, effect,
  full-catalog matching, option admission, plan, host-neutral wrapper-binding
  invariants, safe command names, and pure resolution invariants.
- Application tests own ordering, port calls, adoption assessment, current
  source/runtime identity assessment, whole-surface runtime admission, mutation
  invocation, wrapper/direct fresh-plan parity, zero-attempt rejection,
  one-attempt execution, conventional-completion classification, and uncertain
  post-start fault suppression.
- Infrastructure tests own bounded strict codecs, executable identity, process
  limits, safe local persistence, source/output adapter mechanics, fixed POSIX
  quoting and rendering, and bounded argv forwarding.
- CLI tests own catalog registration, typed argv, help, output schemas,
  migration recovery, stdout/stderr routing, and any generic wrapper lifecycle,
  invocation, output-authority, complete dual-stream writes, source-status
  propagation, and mutation contracts.
- CLI integration fixtures own clean-state specification through bundle status,
  adoption and preview, plus one synthetic GitHub-compatible transform that
  runs through the production compatibility verifier, identity-bound process
  runner, parser, transformer, and result renderer without provider credentials
  or network effects. The recovery fixture additionally proves an exact
  bijection with all 27 preview help faults and all 41 execute help faults. It
  exercises 27 preview zero-attempt cases plus 28 execute pre-start and 15
  execute post-start cases, including both phases of identity change and
  unclassified outcomes, exact recovery actions, drift through the production
  identity reader, zero/one attempt accounting, and hostile-canary absence.
  Narrow controlled ports provide deterministic external-boundary
  observations; defensive request/encoding cases are invoked at their owning
  boundary. The execute encoding case corrupts the application result only
  after the production service and controlled process complete one attempt,
  while application/domain tests prove the undecorated result boundary.
  Infrastructure tests independently prove production file, trust, identity,
  and process fault emission, including native start, wait, limit,
  cancellation, timeout, and identity-race faults.
- Artifact-journey fixtures own execution of the exact `atr` file extracted
  from a release archive. They use a native credential- and provider-network-
  free GitHub-compatible source fixture, a direct Go launcher whose fixture
  observation is stable 1.26.x against a dependency-free synthetic module, an isolated user-
  config root, and bounded append-only attempt logs. Before source inspection
  they verify schema-10 root help and seven exact
  scoped authoring/runtime contracts, including complete nested field
  inventories and the complete ordered 27-fault preview and 41-fault execute
  recovery signatures. The non-shipped harness may seed an exact
  receipt through the production trust-store adapter for each tested bundle;
  this proves receipt consumption, not human consent. Controlling-terminal
  full-digest confirmation remains separate required production-adapter
  evidence. The journey verifies eight help documents: the root index plus
  seven exact scoped public contracts. Linux and macOS activate deterministic
  `wrapper render` bytes and invoke transformed-JSON, identity, and append-argv-
  only GitHub ordinary commands plus one exact identity-wrapped no-argument Go
  `test` command through the extracted runtime. The raw-byte cases prove exact
  stream digests, conventional status, hostile argv, and three GitHub plus one
  Go wrapper source attempts. Windows verifies the exact structured unsupported-
  render result for both source bundles without sourcing bytes or starting a
  wrapper source process. The Go fixture sets `GOTOOLCHAIN=local`, disables
  download, and isolates module/cache roots; those are deterministic harness
  inputs, not production environment or effective-toolchain guarantees.
- Each native CI artifact row runs the full production source-runner and
  trust-store tests, the exact bundle-file fault mapping, and the complete CLI
  recovery matrix before packaging and replay. The release linter pins that
  exact step as well as the five runner/target tuples.
- Artifact-evidence aggregation owns the exact five-target set. Evidence schema
  4 distinguishes `ordinary_command_verified` on Linux/macOS from
  `platform_not_supported` on Windows and binds an ordered `wrapper_cases`
  inventory for GitHub CLI. Each POSIX case records wrapper kind, result mode, bundle and plan
  digests, rendered-source digest, stdout/stderr digests, source status, and one
  source attempt; Windows records an empty case list and zero attempts. Its
  required `go_source` record separately binds adapter contract 1, a stable Go
  1.26.x inspection observation, three inspection attempts, exact command `test`, catalog/bundle/plan
  digests, and one zero-attempt rejection on every target. POSIX additionally
  records exit 12 / `wrapper_runtime_not_supported` for `go test extra`, then
  one identity-wrapper case with a nonempty rendered-wrapper digest and one
  attempt; Windows records the empty
  unsupported case set with zero wrapper attempts. Each
  native job uploads one bounded document containing target and
  observed host,
  revision, archive, command, bundle, and command-specific plan identities,
  fixed attempt/fault counts, and leak booleans. A dependent job pairs those
  documents with the five actual candidate archives, recomputes their hashes,
  strictly rejects missing, extra, duplicate, symlinked, malformed, or
  cross-revision inputs,
  and emits a path-free unattested digest index. The release linter pins the
  aggregator to one exact verifier command plus an allowlist of checkout, Go
  setup, artifact download, and artifact upload actions; no candidate rebuild
  or replay step is admitted.
- Architecture and public guards own dependency direction and secret-free
  repository state.

## Host-neutral wrapper and source-stream milestone gate

This milestone is complete only when all of the following are true on the same
tree:

1. focused domain, codec, application, CLI, and migration tests pass;
2. schema-3 specification and schema-2 bundle canonical fixtures pass;
3. surface/wrapper truth-table and `EffectExecute` negative tests pass;
4. adopted/current bundle preview covers identity and transforming wrappers,
   explicit and inherited surface entries, longest-prefix matching, option
   absence, exact schema-4 result modes, stable plan digests, and exactly zero
   source attempts;
5. compatibility-admitted GitHub CLI `issue list` and `pr list` transformation execution
   covers exact selector encoding, preview/execute plan-digest equality,
   selected/renamed typed JSON, no raw-output leak, and exactly one source
   attempt per command;
6. Go CLI contract 1 inspection proves the exact three probes and a recorded
   stable 1.26.x observation; finite-registry, plan, and whole-surface truth tables admit
   only identity-wrapped exact no-argument `test` and reject every additional
   argv or misconfigured adapter before source start;
7. `wrapper render` and `wrapper run` catalog, typed-argv, output-authority,
   fault, and scoped-help contracts match the implementation; `wrapper run`
   requires the explicit `--` separator and publishes the exact transformed-
   JSON/source-stream result union;
8. deterministic binding/render tests cover portable command-name eligibility,
   POSIX quoting, exact bundle and runtime closure, whole-surface runtime
   admission, hostile argv forwarding, and no host fields;
9. application and CLI tests prove direct preview/wrapper plan-digest parity,
   bundle/runtime/source drift rejection at zero source attempts, exact one-
   attempt transformed-JSON, identity, and append-argv-only success, source
   stream/status fidelity, uncertain-stream suppression, final-write faults,
   no maintainer envelope, and no raw fallback;
10. the production-composition recovery matrix covers all 27 preview faults,
   all 28 execute pre-start phase cases at zero attempts, and all 15 execute
   post-start phase cases at one non-retryable attempt, with exact scoped-help
   parity and no raw or secret canary leak;
11. retired authorization schemas fail with zero source attempts and legacy
   `plan preview` remains migration-only;
12. exact scoped agent help publishes the finite source-catalog,
   specification-authoring, and runtime-admission contracts used by the
   installed-artifact journey;
13. `task release:check` replays the platform-native exact archive, and CI defines
   required native jobs for Linux amd64/arm64, macOS amd64/arm64, and Windows
   amd64; a dependent aggregation job verifies the exact five evidence
   documents and five candidate archive hashes, and the release publish job
   depends on that aggregate. Linux/macOS rows activate the ordinary POSIX
   GitHub commands for all three result cases plus one exact Go `test` identity
   case, while Windows proves structured unsupported rendering for both source
   bundles with zero wrapper source attempts;
14. evidence schema 4 records the ordered GitHub wrapper cases and the separate
   Go adapter/recorded-version/three-probe/catalog/bundle/plan and platform outcome,
   including one zero-attempt Go rejection on every target, followed by one
   POSIX Go success attempt or the Windows empty unsupported case set,
   without storing source bytes or claiming caller attestation;
15. `task check` passes;
16. `task security` passes;
17. `task public:check` passes; and
18. repository search finds no live source-wrapper requirement for
   allow/confirm/deny, source read/create/write, or source target/impact outside
   explicit migration and superseded-history contexts.

Local `task release:check` proves archive mechanics, workflow structure, and
the current platform's native replay. It cannot stand in for the other four native
jobs; an exact commit has complete platform evidence only after the required CI
matrix succeeds. Emulation and cross-build metadata do not count as native
runtime evidence.

The gate does not claim original-preserving optimization, external-processor
execution, raw execution, richer argv transforms, persistent wrapper
installation or executable shims, Windows POSIX activation, arbitrary
transformer integration, support for a source CLI beyond an accepted adapter
contract, executable attestation, or publication authorization.

## Evidence discipline

Record exact commands, fixture identities, attempt counts, and gate results in
the active work packet while the change is open. Promote durable decisions to
theses, architecture, security, an accepted ADR, types, and tests. Delete the
temporary packet after its evidence is represented by the final committed tree
and completion report.
