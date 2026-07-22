# ADR 0015: Admit complete multi-command wrapper surfaces

- Status: Accepted
- Date: 2026-07-22
- Deciders: Repository maintainer and product owner
- Scope: Whole-surface runtime admission, host-neutral wrapper rendering, and
  ordinary-command dispatch
- Supersedes: None
- Superseded by: None

## Context

The canonical specification, bundle, tailored surface, wrapper plan, and
generated-wrapper contract already represent more than one included command.
Contract-2 tailored help also derives root, namespace, and exact-command views
from every included surface entry. The first whole-surface verifier nevertheless
required exactly one included command before `wrapper render` could emit bytes.

That restriction was useful while proving one end-to-end wrapper, but it is not
a product boundary. A purpose-specific GitHub CLI surface may need both of the
finite commands already covered by GitHub CLI contract 2. Requiring separate
bundles and separate ordinary `gh` functions for `issue list` and `pr list`
would make one purpose surface impossible to expose under one ordinary source
command and would conflict with the bundle's existing multi-command model.

Removing the count check alone would be unsafe. Rendering must not succeed for
a surface where only one entry has a maintained runtime contract. The wrapper
would advertise the unsupported entry in static help and defer an incomplete
compatibility decision until invocation.

## Decision drivers

- One reviewed bundle should materialize its complete purpose-specific surface,
  not an undocumented subset selected by the renderer.
- Every command and exposed option must have maintained runtime admission before
  any wrapper bytes are emitted.
- A bundle remains bound to one exact source identity and adapter contract.
- Each command keeps its own existing plan-declared result mode; a surface need
  not force all entries into one output mode.
- The change must reuse the existing registry, plan constructor, process ports,
  wrapper binding, and contract-2 tailored help.
- Go CLI contract 2 must not be broadened beyond exact no-argument `test`.
- No coding-agent-host adapter, host protocol, runtime discovery, or new I/O
  boundary belongs in this change.

## Considered options

### Keep one command per bundle

This preserves the first implementation limit, but it makes a single ordinary
`gh` wrapper unable to represent both maintained GitHub outcomes. Selecting
between two wrappers with the same function name would require an ambient
profile mechanism that Atsura does not yet own.

### Render the admitted subset

The renderer could omit unsupported entries and emit bytes for the remainder.
That would create a second derived surface, make the rendered help disagree with
the adopted bundle, and turn compatibility admission into implicit tailoring.

### Admit every entry as one complete surface

The registry-selected source verifier can validate each included command under
the same bundle-bound adapter contract. Rendering succeeds only if the surface
is non-empty and every entry is admitted; otherwise it emits no wrapper bytes.

## Decision

`wrapper render` admits a non-empty complete included surface rather than
requiring exactly one command.

For GitHub CLI contract 2, the surface may contain one or both of the exact
maintained commands `issue list` and `pr list`. Every included entry must
independently satisfy its existing command, effective-option, wrapper, argv, and
result-mode contract before rendering. Unsupported commands, partial option
grammars, or unsupported wrapper forms fail the complete-surface check. Atsura
does not silently drop, rewrite, or downgrade an entry.

The two GitHub entries may use different already-supported result modes. For
example, one may use `transformed_json` while the other uses
`source_stream_passthrough`. The selected command's fresh plan remains the sole
authority for its result. Mixed modes do not add fallback, cross-command
dispatch, result coercion, or shared processor authority.

Go CLI contract 2 remains a singleton whole-surface contract: exactly the
cataloged `test` command, using either its identity source-stream wrapper or the
exact admitted `test -json` optimizer wrapper, with no caller-visible option
grammar. This ADR does not infer support for any additional Go command or argv.

The canonical specification remains schema 4, bundle schema 3, plan schema 5,
generated-wrapper contract 2, and wrapper review envelope schema 2. The wrapper
binding shape and rendered dispatch grammar do not change. Contract 2 already
carries the bounded help projection for the complete included surface, and non-
help argv already reaches the shared fresh-plan path where the longest catalog
command prefix selects one command. Installed-artifact evidence may advance
separately only if recording the new journey changes that document's shape.

Whole-surface admission remains a pure application/domain compatibility check
selected by the bundle's exact adapter kind. It performs no source or processor
probe, starts no process, consults no ambient executable, and creates no host-
specific state. The existing source and processor process ports remain the only
runtime execution boundaries.

## Consequences

### Positive

- One adopted GitHub bundle can expose both maintained commands under one
  ordinary `gh` wrapper.
- Static tailored help and runtime dispatch describe the same complete surface.
- All-or-nothing admission prevents a rendered wrapper from advertising a
  command that lacks a maintained runtime contract.
- Existing per-command output semantics remain explicit and independently
  reviewable.
- No new schema, wrapper version, process boundary, or coding-agent-host
  integration is introduced.

### Negative

- Whole-surface verification now has a collection invariant and must report one
  invalid entry without rendering otherwise-valid siblings.
- Mixed-mode surfaces increase the test matrix because each selected command
  must retain its own plan and output authority.
- The accepted multi-command range is still intentionally narrow: only the two
  commands already maintained by GitHub CLI contract 2.

## Mechanical enforcement

- Registry and GitHub adapter contract tests cover canonical one-command and
  two-command surfaces and mixed existing result modes.
- Negative tests prove empty, duplicate, unsupported, partially admitted, and
  invalid-option surfaces emit no wrapper bytes and start zero source and
  processor attempts.
- Go adapter tests retain the exact singleton `test` contract and reject an
  added command before rendering.
- Binding and renderer tests prove deterministic contract-2 bytes and complete
  root/namespace/exact tailored-help views for both GitHub commands.
- Invocation tests select each command from the same rendered binding, rebuild
  its byte-identical fresh plan, and preserve its own result authority.
- Repository guards continue to reject coding-agent-host fields, adapters, and
  protocols from production Atsura.

## Security and public-boundary impact

The change broadens only which already-admitted entries may coexist in one
same-source bundle. It adds no credential, persisted state, network destination,
source probe, processor probe, shell evaluation, arbitrary executable, or host
protocol. Every entry is validated before any wrapper material is emitted, and
every later invocation retains adoption, runtime, source, surface, option,
fresh-plan, and process-boundary checks.

Hiding remains surface composition rather than authorization. A multi-command
wrapper is not a sandbox and does not claim that the source operations share
permissions, remote effects, or safety semantics.

## Compatibility and migration

Existing one-command wrappers and bundles remain valid. New multi-command
GitHub wrappers still use generated-wrapper contract 2 and require no persisted
state migration. Contract-2 consumers already forward exact argv and therefore
need no host-specific change. Go and Windows support claims are unchanged.

## Validation

- focused whole-surface registry, GitHub adapter, Go adapter, binding, renderer,
  plan, and CLI tests
- `task check`
- `task security`
- `task public:check`
- `task release:check`

## Reconsideration signals

Supersede this ADR before admitting another source command, source-adapter
contract, cross-source surface, dynamic plugin, result mode, or unbounded
collection. Do not implement partial rendering or ambient profile selection as
a local workaround.
