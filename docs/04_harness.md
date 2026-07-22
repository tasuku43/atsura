# Harness

The harness turns Atsura's product, architecture, and security claims into
repeatable checks. A capability is complete only when `task check` passes. The
current transform, host-neutral wrapper, and finite output-optimizer milestone
also requires `task security` because it admits attempted source and processor
invocations, observes every executable identity, exposes a canonical plan
contract, and can start one compatibility-admitted source process followed by
one compatibility-admitted processor process.

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
  CLI contract 2 executes exactly `go version`, `go help`, and `go help test`;
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

### Tailoring specification schema 4

Domain, codec, application, and CLI tests must prove:

- schema version 4 is required and catalog-bound;
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
- output projection and original-preserving optimization are a closed
  discriminated union;
- an optimizer contains only the accepted namespaced processor contract,
  declared input format, and explicit original-output allowance; it never
  contains a shell fragment, executable path, arbitrary argv, filter, plugin,
  or runtime-LLM action;
- the only admitted optimizer contract is
  `atsura.output.rtk_go_test_pass.v1`; every other RTK filter, processor,
  version, source tuple, and platform remains invalid; and
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

### Bundle schema 3 and adoption

Tests must prove:

- a canonical bundle binds exact source identity, adapter evidence, normalized
  catalog and digest, normalized schema-4 specification and digest, the
  derived surface with wrappers, and any exact processor observation required
  by those wrappers;
- canonical bytes exclude timestamps, machine/user identity, credentials,
  source output, and random values;
- every embedded digest and derived surface is recomputed on load;
- catalog, specification, surface, wrapper, source, processor, and bundle drift
  are distinguishable;
- alternate vendor-neutral adapter fixtures compile to the same shared bundle
  contract; and
- schema-1 and schema-2 bundles are rejected rather than reinterpreted.

Adoption tests must prove that only the full exact digest is accepted through a
controlling terminal; the receipt is user-local and content-bound; unrelated
receipts survive replacement; malformed or unsafe storage fails closed; and
the review summary counts surface and wrapper facts rather than source
permissions, decisions, or inferred effects. It exposes the exact processor
identity and whether original transformed-source output may remain visible.

Trust-store writes remain Atsura-owned create/write mutations and must pass
through the central mutation invoker with exact target and impact contracts.

### Wrapper plan contract

`bundle preview --bundle <path> -- <source-executable> <argv>` is the current
zero-execution plan boundary. Tests must prove:

- only a strictly loaded schema-3 bundle with an exact valid adoption receipt
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
- the schema-5 plan binds bundle/catalog/specification digests, source and
  adapter identity, any required processor identity and contract, matched
  command and surface origin, wrapper kind, reason, option surface,
  original/transformed argv, and ordered before/invoke/output/after stages plus
  exactly one result mode;
- the invocation stage declares closed stdin plus inherited working directory
  and environment modes without serializing ambient values;
- the invoke stage declares exactly one maximum attempt plus finite timeout,
  stdout, and stderr bounds, even though preview never crosses that boundary;
- `append_args` appear exactly at the end of transformed argv;
- an output transform requires exactly one active cataloged selector matching
  its input format before `--`; missing, duplicate, conflicting, or positional
  selectors fail plan construction;
- a projection output stage derives `transformed_json`; a complete identity or
  append-argv-only wrapper without an output stage derives
  `source_stream_passthrough`; the accepted optimizer stage derives
  `original_preserving_optimizer`; a missing, unknown, or contradictory mode
  fails;
- identical validated inputs produce identical canonical plan bytes and
  `plan_digest` values;
- the schema-2 preview envelope contains exactly `plan_digest`, `plan`, and
  `source_process_attempts`, with the attempt count always zero;
- exact schema-12 agent help publishes the versioned schema-5 `wrapper-plan`
  inventory,
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
- an `original_preserving_optimizer` plan is rejected before source or
  processor start because `bundle execute` retains its projection-only result
  envelope;
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

### Ordinary-wrapper runtime contract

`wrapper run` extends the shared plan application service without broadening
the direct `bundle execute` result envelope. Tests must prove:

- one finite application registry structurally satisfies both fresh-plan and
  whole-surface compatibility ports, dispatches the exact unchanged plan or
  bundle by adapter kind, preserves valid finite admission categories, and
  rejects nil, typed-nil, empty, unknown, duplicate, or misconfigured entries
  as `adapter_contract` without dispatch;
- the complete GitHub CLI adapter/version/command/surface/long-option grammar
  is admitted for every included entry before wrapper rendering and source
  start; one bundle may contain one or both maintained commands with independent
  existing result modes, and each later invocation uses only the selected
  command's plan without requiring a JSON selector for identity or append-argv-
  only wrappers;
- Go CLI contract 2 admits a recorded stable Go 1.26.x inspection observation
  and exact command `test`. Its identity branch permits one complete identity
  wrapper, `source_stream_passthrough`, no caller-visible option surface, and
  no argv element after `test`;
- every Go option, package pattern, `--` marker, test-binary argument, other
  command, non-identity wrapper, and catalog recording a version outside stable
  1.26.x fails before source start;
- tests distinguish direct-launcher path/hash/size from the possibly delegated
  `Source.Version` observation and do not claim that runtime repeats `go
  version`, detects a later effective-toolchain change, or binds a selected
  toolchain/GOROOT tree;
- preview and wrapper application rebuild byte-identical schema-5 plans and
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

### Original-preserving optimizer contract

ADR 0012 admits exactly one finite tuple:
`atsura.output.rtk_go_test_pass.v1`, source-catalog schema 2, Go CLI contract 2,
processor-observation schema 1, specification schema 4, bundle schema 3, and
plan schema 5. It binds exact caller argv `go test`, source argv
`go test -json`, an official RTK v0.43.0 artifact, and processor argv
`pipe --filter=go-test` on Linux and macOS amd64/arm64. Tests must prove:

- projection and original-preserving optimizer are disjoint typed contracts,
  and arbitrary processor paths, argv, filters, versions, source tuples,
  platforms, shell fragments, and runtime discovery are rejected;
- `processor inspect` accepts one explicit absolute path, verifies the exact
  maintained artifact identity for the current platform, runs `--version`
  once without a shell under host-neutral
  `atsura.processor.rtk_isolated.v2`, and emits one bounded canonical schema-1
  observation with no ambient discovery, coding-agent-host variable, or
  install; retired v1 observations are rejected;
- `spec init --processor` selects the optimizer only for the exact compatible
  Go catalog and explicit processor observation, while omission keeps the
  identity draft; `bundle build --processor` requires and binds that same
  observation, and status/adoption/preview expose the exact identity and
  original-output visibility;
- source and processor preflight completes before source start. Missing,
  incompatible, or drifted processor evidence therefore yields source zero and
  processor zero attempts;
- strict typed Go-test JSONL admission independently validates the single-
  package pass lifecycle and calculates the exact shorter summary before RTK
  starts. Failure, skip, malformed/unknown JSON, empty output, source stderr,
  nonzero status, and a non-beneficial valid summary return exact source
  stdout/stderr/status as `preserved_before_processor` after one source and
  zero processor attempts;
- eligible input alone reaches the exact identity-bound processor process port.
  It receives only the bounded admitted source stdout on stdin, starts at most
  once without a shell, and inherits neither caller credentials nor coding-
  agent-host state;
- successful processor output is either byte-identical admitted input as
  `preserved_after_processor` or the independently calculated newline-free
  summary as `optimized`, each after one source and one processor attempt;
- processor start/wait/timeout/cancellation/limit/status/stderr/identity or
  semantic-postcondition failure after source completion is non-retryable,
  publishes no source or processor bytes, and never selects original output as
  fallback;
- controlled application/infrastructure fixtures own every disposition,
  source/processor attempt combination, isolation classification, hostile
  source data, and arbitrary processor failure that a real official artifact
  cannot safely fabricate;
- process-runner cleanup tests pin the original root and owner marker across
  unlink/recreate, reject both early and late root replacement, preserve a
  replacement sentinel, and prove cleanup never recursively follows a replaced
  top-level pathname; and
- native release evidence replays the exact official artifact on all four
  claimed platforms, records Apache-2.0 provenance, verifies the supplied
  isolated environment/root contract, exact caller/source/processor argv,
  formats, modes, attempt ceilings, timeout and byte bounds, and proves the
  optimized and reachable pre-processor preservation journeys. It does not
  claim absence of child
  processes, outside-root filesystem access, or network attempts without a
  separately implemented and validated external observer. A release-quality
  native claim belongs only to an exact revision whose evidence schema 7 rows
  and aggregate schema 2 pass the required five-target workflow.

ADR 0009's rejected `git-log` tuple remains a negative fixture: executable
identity, status zero, empty stderr, and smaller output never substitute for
task-owned semantic validation.

### External-process effect and controlled boundaries

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

Processor-inspection and optimizer tests likewise use `EffectExecute` and bind
the exact processor identity, argv, environment contract, stdin ownership, and
attempt budget at their separate port. Execute does not turn the processor into
an Atsura-owned mutation or let it select or start the source.

Go runtime tests must also keep `go test` classified as source-owned
`EffectExecute`, not Read or an Atsura-owned mutation. The tests prove only
exact identity/argv, zero/one attempts, and failure classification; they do not
claim to contain or authorize repository code, module resolution, credential
use, network access, or caller-owned file/cache changes.

### Retired-schema migration

Fixtures for specification schemas 1 through 3 and bundle schemas 1 and 2 must
prove:

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
  envelope and publishes an exact typed `plan_result_modes` union. The variants
  are compact object-or-array JSON plus LF/empty stderr/status zero; exact
  bounded source stdout/stderr, no framing, buffered delivery, and conventional
  source status with no timing/interleaving claim; and the three exact
  original-preserving optimizer dispositions with their source/processor
  attempt contracts. Whole-catalog validation resolves the exact `bundle
  preview` `plan`/`wrapper-plan` schema-5 reference and rejects an incomplete
  or unknown result variant;
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
  defaults to raw sourceable text, and returns the exact schema-2 review
  envelope in JSON with zero source and processor attempts;
- POSIX rendering succeeds only on Linux and macOS; Windows returns exact
  structured `wrapper_platform_not_supported`, emits no wrapper bytes, and has
  no POSIX activation claim;
- the bundle's requested executable is used verbatim only when it is a portable
  POSIX Name outside the maintained reserved/fixed and implementation-specific
  function-name set; no basename or path normalization invents a command;
- the complete included surface is non-empty and every command, effective
  option surface, wrapper, and result mode is covered by its registry-selected
  verifier before bytes are rendered: GitHub CLI contract 2 admits one or both
  of `issue list` and `pr list`, including different existing result modes per
  command; Go CLI contract 2 remains exactly one identity-wrapped `test` or the
  exact RTK-bound `test -json` optimizer, with no caller-visible option surface;
- one unsupported GitHub entry rejects the complete render with zero wrapper
  bytes and zero source and processor attempts rather than silently omitting
  that entry; empty surfaces and an added Go command fail the same pre-render
  boundary;
- the wrapper binding contains only wrapper contract, bundle identity, runtime
  identity, source identity, ordinary command spelling, and the bounded help
  projection rederived from the included surface;
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
  `wrapper run` for every non-help argv list, includes the explicit `--`
  separator, accepts exact argv rather than a shell command string, and reaches
  the same plan constructor and source-execution boundary as the direct
  gateway;
- generated-wrapper contract 2 derives root, namespace, and exact-command
  views only from included surface entries and effective long options; final
  exact `--help` prints deterministic bundle-digested text with zero bound
  `atr`, source, and processor attempts, while excluded and unknown selectors
  expose no raw source help and retain existing fail-closed faults;
- spaces, empty values, Unicode, dash-prefixed values, literal metacharacters,
  and ordering survive wrapper forwarding without `eval`, `sh -c`, or shell
  reconstruction;
- caller-defined `command`, `return`, `test`, and `printf` functions or aliases
  cannot intercept tailored-help matching, rendering, or termination; cleanup
  is confined to the generated function's subshell and the caller's
  definitions remain intact;
- an existing exact same-name alias is removed before the function definition,
  alias-safe no-op fallback cannot intercept activation, and source-file tests
  prove the intended wrapper name is defined and selected under the declared
  standard-`unalias` POSIX activation precondition;
- missing adoption, runtime or bundle mismatch, source drift, absent surface
  command, invalid option, processor drift, or unsupported runtime starts zero
  source and processor processes;
- the runtime identity check is described and tested as cooperative drift
  detection after the bound `atr` path starts, not attestation or containment
  against malicious replacement at that path;
- admitted success starts the exact physical source once, conditionally starts
  the exact bound processor once, applies the same typed stages, emits only the
  plan-declared result, and never selects raw, another bundle, or original bytes
  after processor failure as fallback;
- wrapper success uses the exclusive `fresh_wrapper_plan` interpretation and
  presentation authority and emits no maintainer evidence envelope; exact
  scoped schema-12 help publishes all three typed result modes and points to the
  schema-5 `bundle preview` wrapper-plan governing the selected variant;
- the current renderer persists nothing and edits no activation configuration;
  any future persisted artifact lifecycle uses exact ownership, bounded
  regular-file paths, symlink/special-file rejection, atomic replacement,
  central mutation invocation, and read-only drift reconciliation without
  editing caller-owned activation configuration; and
- optimizer implementation conformance is owned by local controlled tests;
  release-quality optimizer status additionally requires exact installed-
  artifact replay with the official RTK artifact on every claimed Linux and
  macOS native target, while Windows replays existing commands and the exact
  unsupported-render result with zero source attempts and no processor
  artifact or evidence.

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

- Domain tests own specification, surface, wrapper, processor binding, bundle,
  digest, effect, full-catalog matching, option admission, plan, optimizer
  disposition, host-neutral wrapper-binding invariants, safe command names, and
  pure resolution invariants.
- Application tests own ordering, port calls, adoption assessment, current
  source/runtime identity assessment, whole-surface runtime admission, mutation
  invocation, wrapper/direct fresh-plan parity, zero-attempt rejection,
  bounded source/processor ordering and attempt counts, conventional-completion
  classification, optimizer admission/postconditions, and uncertain post-start
  fault suppression.
- Infrastructure tests own bounded strict codecs, executable identity, process
  limits, safe local persistence, source/output adapter mechanics, RTK identity,
  isolated processor execution and observation, fixed POSIX quoting and
  rendering, and bounded argv forwarding.
- CLI tests own catalog registration, typed argv, help, output schemas,
  migration recovery, stdout/stderr routing, and any generic wrapper lifecycle,
  invocation, output-authority, processor inspection, complete dual-stream
  writes, source-status propagation, optimizer disposition presentation, and
  mutation contracts.
- CLI integration fixtures own clean-state specification through bundle status,
  adoption and preview, plus one synthetic GitHub-compatible transform that
  runs through the production compatibility verifier, identity-bound process
  runner, parser, transformer, and result renderer without provider credentials
  or network effects. The recovery fixture additionally proves an exact
  bijection with every catalog-declared preview and execute help fault. It
  exercises every declared preview zero-attempt case and every execute
  pre-start and post-start phase, including both phases of identity change and
  unclassified outcomes, exact recovery actions, drift through the production
  identity reader, zero/one source-attempt accounting, and hostile-canary
  absence.
  Narrow controlled ports provide deterministic external-boundary
  observations; defensive request/encoding cases are invoked at their owning
  boundary. The execute encoding case corrupts the application result only
  after the production service and controlled process complete one attempt,
  while application/domain tests prove the undecorated result boundary.
  Infrastructure tests independently prove production file, trust, identity,
  and process fault emission, including native start, wait, limit,
  cancellation, timeout, and identity-race faults.
- Optimizer integration fixtures own the complete explicit
  processor-observation-to-specification-to-bundle-to-plan journey and all
  three dispositions. Controlled source and processor ports prove the exact
  source/processor attempt matrix, preflight ordering, semantic oracle, no-byte
  processor-failure rule, and no fallback without manufacturing behavior in an
  official RTK executable.
- Artifact-journey fixtures own execution of the exact `atr` file extracted
  from a release archive. They use a native credential- and provider-network-
  free GitHub-compatible source fixture, a direct Go launcher whose fixture
  observation is stable 1.26.x against a dependency-free synthetic module, an
  isolated user-config root, and bounded append-only attempt logs. Before
  source inspection they verify schema-12 root help and the catalog-derived
  exact scoped authoring/runtime contracts, including complete nested field
  inventories and complete ordered recovery signatures. The non-shipped
  harness may seed an exact
  receipt through the production trust-store adapter for each tested bundle;
  this proves receipt consumption, not human consent. Controlling-terminal
  full-digest confirmation remains separate required production-adapter
  evidence. Linux and macOS activate deterministic `wrapper render` bytes and
  invoke the maintained GitHub and Go ordinary-command surfaces through the
  extracted runtime. Windows verifies exact structured unsupported rendering
  with no sourced bytes, zero wrapper source attempts, and no processor
  artifact or evidence. The Go
  fixture sets `GOTOOLCHAIN=local`, disables download, and isolates module/cache
  roots; those are deterministic harness inputs, not production environment or
  effective-toolchain guarantees.
- Each native CI artifact row runs the full production source-runner and
  trust-store and processor-runner tests, the exact bundle-file fault mapping,
  and the complete CLI recovery matrix before packaging and replay. The release
  linter pins that exact step as well as the five base runner/target tuples.
- The native journey opens each candidate archive once for one bounded read and
  derives both the verified digest and extracted bytes from that value. Tests
  replace the pathname after the read and prove that later path contents cannot
  change the bytes selected for replay.
- Artifact-evidence aggregate schema 2 owns the exact five-target base set.
  Historical evidence schema 4 proves only the pre-optimizer GitHub and Go
  identity-wrapper journey. Schema 5 retains those base facts and adds Go CLI
  contract 2, processor-observation schema 1, the exact RTK identity and
  invocation, schema-3 bundle and schema-5 plan identities, exact caller/source/
  processor argv, formats, modes, v2 environment and bounds, separate source
  and processor-inspection evidence, disposition/status, source-fixture attempt
  counts, and leak booleans. It is optimizer-aware but predates static tailored
  help.
- Historical evidence schema 6 retains the complete schema-5 record and adds
  the first bounded `tailored_help` object for one transformed-PR wrapper.
  Current schema 7 adds exact `caller_argv` to every wrapper case. POSIX rows
  keep three ordered cases and three wrapper source attempts: transformed
  `pr list` and append-only `issue list` share one exact bundle and wrapper
  digest while keeping distinct caller argv and plan digests; identity remains
  independently bound. Their shared wrapper requires five ordered root,
  namespace, and exact-command help views plus hidden `api --help`
  `command_not_in_surface` and unknown `unknown --help` `invalid_invocation`
  faults with zero source and processor attempts. The GitHub fixture total
  remains 13. Windows requires `platform_not_supported`, empty wrapper cases,
  views, and fault lists, no tailored-help bundle or rendered-wrapper binding
  digests or wrapper contract, zero wrapper attempts, and 10 GitHub fixture
  attempts. Top-level journey identities remain required. Aggregate schema 2
  is unchanged and excludes the new per-case caller argv.
- The four Linux/macOS optimizer targets must prove `optimized` and reachable
  `preserved_before_processor`; Windows records no optimizer case and no
  processor evidence. Installed evidence does not claim processor-launch
  counts without an accepted external observer; controlled application and
  infrastructure tests own that attempt truth. Each native journey locally
  verifies the supplied RTK archive and extracted executable against the code-
  pinned manifest; the processor archive is never uploaded as an Atsura
  artifact. A dependent job pairs each document with the actual candidate
  Atsura archive, validates the exact pinned processor identity recorded by
  every applicable row, recomputes candidate hashes, strictly rejects missing,
  extra, duplicate, symlinked, malformed, or cross-revision inputs, and emits a
  path-free unattested digest index. No candidate rebuild or replay substitute
  is admitted. The inherited schema-5 optimizer shape keeps the outer
  `go_source` identity baseline and the nested `optimizer` bundle, plan,
  rendered-wrapper digest, cases, and faults separate.
- Architecture and public guards own dependency direction and secret-free
  repository state.

## Host-neutral wrapper and optimizer implementation gate

This implementation milestone is complete only when all of the following are
true on the same tree:

1. focused domain, codec, application, infrastructure, CLI, and migration tests
   pass;
2. schema-4 specification, schema-3 bundle, schema-5 plan, and schema-1
   processor-observation canonical fixtures pass;
3. surface/wrapper truth tables, the projection/optimizer discriminated union,
   and `EffectExecute` negative tests pass;
4. adopted/current bundle preview covers identity, projection, source-stream,
   and optimizer wrappers, stable plan digests, exact processor binding, and
   exactly zero process attempts;
5. compatibility-admitted GitHub CLI execution covers exact selector encoding,
   preview/execute plan parity, selected typed JSON, no raw-output leak, and one
   source attempt per admitted command;
6. Go CLI contract 2 inspection proves its exact three probes and recorded
   stable 1.26.x observation; its finite registry admits only exact no-argument
   `test` through either the identity surface or wrapper-owned `-json` optimizer
   surface;
7. `processor inspect`, `spec init --processor`, and `bundle build --processor`
   prove explicit evidence flow, exact RTK v0.43.0 identity, no ambient
   discovery, deterministic authoring default, and preflight rejection before
   source start;
8. strict Go-test event validation proves every accepted lifecycle fact and
   exact shorter-summary oracle before the processor starts;
9. application and infrastructure tests prove all three optimizer dispositions,
   separate zero/one source and processor attempt counts, isolated no-shell
   execution, post-source identity revalidation, no-byte processor faults, and
   no failure fallback;
10. `wrapper render` and `wrapper run` catalog, typed-argv, output-authority,
    fault, schema-2 review-envelope, and scoped-help contracts match the
    implementation; `wrapper run` publishes all three schema-12 result modes;
11. deterministic binding/render tests cover portable command names, POSIX
    quoting, contract-2 root/namespace/exact tailored help, exact
    bundle/runtime/source/processor closure, one- and two-command GitHub whole-
    surface admission, mixed existing result modes, retained singleton Go
    admission, hostile argv forwarding, and absence of coding-agent-host
    fields;
12. the production-composition recovery matrix is an exact bijection with every
    catalog-declared fault, covers each pre-start and post-start attempt phase,
    and leaks no raw or secret canary;
13. retired authorization schemas fail with zero process attempts and legacy
    `plan preview` remains migration-only;
14. exact scoped agent help publishes the finite source-catalog,
    processor-inspection, specification-authoring, planning, and runtime
    admission contracts;
15. `task check`, `task security`, and `task public:check` pass; and
16. repository search finds no live source-wrapper requirement for
    allow/confirm/deny, source read/create/write, source target/impact, arbitrary
    processors, or coding-agent-host adaptation outside explicit migration and
    superseded-history contexts.

Implementation acceptance alone is not a release-quality installed-artifact
claim. That claim additionally requires all of the following for the exact
candidate revision:

1. `task release:check` replays the platform-native exact Atsura archive;
2. CI provides native Linux amd64/arm64, Darwin amd64/arm64, and Windows amd64
   base rows, with the exact official RTK v0.43.0 artifact supplied only to the
   four optimizer-supported rows;
3. evidence schema 7 retains the schema-5 optimizer and historical schema-6
   tailored-help records, adds exact caller argv to each ordinary-wrapper case,
   binds transformed `pr list` and append-only `issue list` to one shared bundle
   and wrapper with distinct plans, and records all five POSIX help views plus
   the runtime-non-executable condition and zero-attempt fallthrough faults;
4. Windows proves structured unsupported rendering and an explicit empty
   `tailored_help: platform_not_supported` record without receiving a
   processor artifact or claiming POSIX activation or an optimizer;
5. aggregation recomputes every candidate Atsura archive hash, validates each
   applicable row's recorded processor identity against the code-pinned
   manifest, rejects missing or extra evidence, and the release publish job
   depends on that aggregate;
   and
6. every required native job succeeds. Emulation, cross-build metadata, local
   replay of one platform, or controlled synthetic processor tests cannot stand
   in for another platform's native evidence.

Historical predecessor evidence: a clean detached-worktree
`task release:check` and CI run 29910455312 passed the corresponding schema-6
six-condition set on 2026-07-22 for revision
`01c05a45e8b00f09d63d3c6551d3a5df393c41b5`. That run does not satisfy current
schema-7 condition 3 or the current six-condition set. A schema-7 candidate must
run both the local release gate and native workflow before making this claim.

Neither gate claims raw execution, richer argv transforms, persistent wrapper
installation or executable shims, Windows POSIX activation, arbitrary
transformer integration, support beyond an accepted source/processor contract,
executable attestation, source authorization, sandboxing, coding-agent-host
integration, or publication authorization.

## Evidence discipline

Record exact commands, fixture identities, attempt counts, and gate results in
the active work packet while the change is open. Promote durable decisions to
theses, architecture, security, an accepted ADR, types, and tests. Delete the
temporary packet after its evidence is represented by the final committed tree
and completion report.
