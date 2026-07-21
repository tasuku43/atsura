# Harness

The harness turns Atsura's product, architecture, and security claims into
repeatable checks. A capability is complete only when `task check` passes. The
current transform-runtime milestone also requires `task security` because it
admits an attempted source invocation, observes executable identity, exposes a
canonical plan contract, and can start one compatibility-admitted source process.

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

The transform-runtime work packet deliberately runs both profiles as broad
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
- source identity and probe evidence are preserved in the catalog;
- verified built-ins, observed extensions, and unverified dynamic entries stay
  distinct;
- alternate synthetic adapters satisfy shared contracts without GitHub- or
  host-specific fields; and
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
- arbitrary shell, script, jq, RTK, plugin, and runtime-LLM actions are
  rejected; and
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
  stages;
- the invocation stage declares closed stdin plus inherited working directory
  and environment modes without serializing ambient values;
- the invoke stage declares exactly one maximum attempt plus finite timeout,
  stdout, and stderr bounds, even though preview never crosses that boundary;
- `append_args` appear exactly at the end of transformed argv;
- an output transform requires exactly one active cataloged selector matching
  its input format before `--`; missing, duplicate, conflicting, or positional
  selectors fail plan construction;
- identical validated inputs produce identical canonical plan bytes and
  `plan_digest` values;
- the schema-2 preview envelope contains exactly `plan_digest`, `plan`, and
  `source_process_attempts`, with the attempt count always zero;
- exact schema-8 agent help publishes the versioned `wrapper-plan` inventory,
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
- retired `policy` vocabulary appears only in migration diagnostics or
  historical superseded documents;
- exact output schemas reject undeclared fields and preserve absent versus
  explicit empty where meaningful; and
- root agent help remains a bounded capability index, with details reachable
  by exact command or namespace help.

### Host mapping

Host integration is outside the current milestone. Its future conformance
tests must keep transport values separate from core meaning: host
`allow`/`ask`/`deny` may map `rewrite`, `not_managed`,
`command_not_in_surface`, `invalid_invocation`, or `interaction_required`, but
must never become a source-operation permission model.

### External text and secrets

Hostile fixtures must cover source help, catalog labels, YAML strings, source
output, paths, and host payloads containing controls, Unicode separators,
prompt-like text, and secret-shaped values. Visible rendering must preserve
terminal and JSON/TSV framing without filtering printable meaning. Persistent
fixtures must assert that credentials, raw stdout/stderr, environment
snapshots, transcripts, and agent reasoning are absent.

## Test ownership

- Domain tests own specification, surface, wrapper, bundle, digest, effect,
  full-catalog matching, option admission, plan, and pure resolution invariants.
- Application tests own ordering, port calls, adoption assessment, current
  source identity assessment, mutation invocation, zero-attempt rejection,
  one-attempt execution, and post-start fault classification.
- Infrastructure tests own bounded strict codecs, executable identity, process
  limits, safe local persistence, and adapter protocol mechanics.
- CLI tests own catalog registration, typed argv, help, output schemas,
  migration recovery, and stdout/stderr routing.
- CLI integration fixtures own clean-state specification through bundle status,
  adoption and preview, plus one synthetic GitHub-compatible transform that
  runs through the production compatibility verifier, identity-bound process
  runner, parser, transformer, and result renderer without provider credentials
  or network effects.
- Architecture and public guards own dependency direction and secret-free
  repository state.

## Transform-runtime milestone gate

This milestone is complete only when all of the following are true on the same
tree:

1. focused domain, codec, application, CLI, and migration tests pass;
2. schema-3 specification and schema-2 bundle canonical fixtures pass;
3. surface/wrapper truth-table and `EffectExecute` negative tests pass;
4. adopted/current bundle preview covers identity and transforming wrappers,
   explicit and inherited surface entries, longest-prefix matching, option
   absence, stable plan digests, and exactly zero source attempts;
5. compatibility-admitted GitHub CLI `issue list` and `pr list` transformation execution
   covers exact selector encoding, preview/execute plan-digest equality,
   selected/renamed typed JSON, no raw-output leak, and exactly one source
   attempt;
6. unsupported runtime combinations and every pre-start contract failure record
   zero attempts, while post-start failures record one and are non-retryable;
7. retired authorization schemas fail with zero source attempts and legacy
   `plan preview` remains migration-only;
8. `task check` passes;
9. `task security` passes; and
10. repository search finds no live source-wrapper requirement for
   allow/confirm/deny, source read/create/write, or source target/impact outside
   explicit migration and superseded-history contexts.

The gate does not claim identity-wrapper or argv-only-transform execution, raw
execution, host installation, arbitrary transformer integration, support for a
source CLI beyond an accepted adapter contract, or publication readiness.

## Evidence discipline

Record exact commands, fixture identities, attempt counts, and gate results in
the active work packet while the change is open. Promote durable decisions to
theses, architecture, security, an accepted ADR, types, and tests. Delete the
temporary packet after its evidence is represented by the final committed tree
and completion report.
