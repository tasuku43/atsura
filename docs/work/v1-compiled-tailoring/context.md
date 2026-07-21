# Work Context: Surface-and-wrapper correction

## Direction correction

The product owner clarified that Atsura is not an authorization or policy
enforcement engine. It compiles an existing CLI into a purpose-specific command
and option surface plus deterministic wrapper pipelines for coding agents.

The corrected distinctions are:

- command and option membership describe what exists in the tailored surface;
- wrapper behavior independently describes how an included command invokes and
  transforms the source;
- an absent command is `command_not_in_surface`, not denied permission;
- source CLI owns source-operation semantics, authentication, authorization,
  prompts, destinations, and downstream effects;
- adoption means selecting one exact surface/wrapper bundle, not granting
  permission;
- `EffectExecute` describes starting a source-owned process;
- create/write target and impact contracts apply only to Atsura-owned
  mutations; and
- host allow/ask/deny values are protocol mapping only.

ADR 0005 records this correction and supersedes ADR 0004's
authorization-centered source-wrapper semantics while retaining the
vendor-neutral, content-addressed bundle architecture.

## Verified reusable foundation

The repository already contains reusable, direction-compatible foundations:

- the four-layer dependency structure and architecture lint;
- catalog-derived routing, typed argv, help, output, faults, and capability
  metadata;
- exact source executable identity and provenance-bearing catalog values;
- bounded no-shell source inspection with adapter-owned fixed probes;
- canonical JSON and digest infrastructure;
- strict bounded regular-file and YAML/JSON decoding infrastructure;
- exact-digest user-local trust receipts and drift status;
- the central mutation invoker for Atsura-owned trust-store changes;
- bounded typed JSON parsing and select/rename transformation primitives; and
- synthetic alternate-source fixtures.

These mechanisms may be retained only after their public and type-level
semantics agree with ADR 0005.

## Invalidated evidence

Previous work and passing gates for the following behavior do not satisfy this
goal and must not be carried forward as current product truth:

- schema-2 `policy` rules requiring visibility, allow/confirm/deny,
  read/create/write, source authorization target, or impact;
- deny-by-default as an implicit surface model;
- trust summaries that count permission decisions or inferred effects;
- bundle plans whose existence or applicability depends on a permission
  decision;
- preview/run compatibility that projects legacy authorization values into a
  new model; and
- host enforcement designs that treat Claude Code allow/ask/deny as core
  authorization semantics.

The uncommitted authorization-centered bundle-plan/runtime slice is paused. It
must be removed or reduced to explicit migration behavior before this
correction is committed. Its earlier `task check` or `task security` results are
historical evidence only.

## Fixed decisions for this milestone

- Tailoring specification schema version is 3.
- Compiled bundle schema version is 2.
- Surface default is explicit `inherit` or `exclude`.
- Explicit command presence is `include` or `exclude`.
- Included commands have an explicit option surface and one complete wrapper.
- Excluded commands have neither options nor wrapper.
- Initial wrappers are `identity` or `transform`.
- Identity wrappers contain no transformations.
- Transform wrappers require exact appended argv and/or the existing typed
  structured-output transform.
- Before and after lists are explicit and empty until typed actions are chosen.
- Unlisted verified built-ins inherit an identity wrapper only when the surface
  default is `inherit`.
- Observed extensions and unverified dynamic commands do not enter the managed
  surface implicitly.
- Adoption remains exact bundle-digest, user-local, and interactive.
- Specification schemas 1 and 2 and bundle schema 1 are retired without
  automatic conversion or trust migration.
- Deprecated authorization paths return stable migration diagnostics and start
  zero source processes.
- Source inspection is `EffectExecute` because it starts the source executable.
- Runtime, raw, refresh, and host adapters are outside this milestone.

## Constraints and risks

- Current code and tests use `policy`, `decision`, `effect`, `target`, and
  `impact` names across domain, application, CLI, fixtures, capability metadata,
  and docs. A partial rename would preserve contradictory contracts.
- Schema versions must advance rather than reinterpret existing serialized
  bytes under old numbers.
- Authorization policy values have no lossless automatic mapping to surface
  membership and wrapper behavior.
- Migration-only public commands still need catalog-derived routing, faults,
  recovery, effect, and output metadata.
- A surface default of `inherit` must not silently include unverified dynamic
  or observed extension evidence.
- Adoption summaries must remain materially reviewable after permission counts
  are removed.
- Any switch that treats every non-read effect as mutation could accidentally
  apply mutation contracts to `EffectExecute`.
- Repository searches must distinguish live contracts from superseded ADR and
  intentional migration fixture vocabulary.
- Hiding must never be documented as a security sandbox.

## Evidence required for completion

- Canonical schema-3 specification bytes/digest and strict codec fixtures.
- Surface resolution truth-table tests including inherited, explicit included,
  explicit excluded, invalid excluded-wrapper, invalid incomplete included,
  and unverified-default cases.
- Canonical schema-2 bundle bytes/digest and recomputed derived surface.
- Adoption summary fixtures containing surface/option/wrapper/transform counts
  and no source permission/effect counts.
- `EffectExecute` encoding/validation and mutation-separation tests.
- Retired specification/bundle and deprecated command fixtures with exact fault,
  recovery, zero persistence, and zero source attempts.
- Focused test commands and results.
- Final `task check` and `task security` results from the same tree.
- A final repository vocabulary search with every remaining authorization term
  explained as superseded history, migration, source-owned semantics, or an
  Atsura-owned mutation contract.

## Remaining unknowns after this milestone

- The first supported runtime source command and end-to-end maintainer journey.
- Richer argv replacement/default operations.
- The first non-empty typed before and after actions.
- Exact bundle-backed wrapper-plan and runtime command shapes.
- Raw execution's public command shape.
- The first host adapter and its protocol mapping.
- Whether jq, RTK, plugins, or another external transformer port is justified.
- Named profiles, multiple adopted bundles, and selection precedence.
- Additional source adapters and their compatibility ranges.

## Verification evidence

Verified on 2026-07-21 with the repository-required Go 1.26.5 selected through
mise:

- focused domain, application, codec, infrastructure, and CLI tests: pass;
- `go test ./...`: pass after restoring the harness bootstrap provenance
  contract in `docs/04_harness.md`;
- `task check:fast`: pass (`repoguard`, `archlint`, `contractlint`, all Go
  tests);
- `task check`: pass, including the race run;
- `task security`: pass (`go mod verify`, security guard, vulnerability scan);
- `git diff --check`: pass; and
- schema-7 agent-help byte measurements match the readiness record: root 4,755,
  exact `sample read` 5,845, `sample` namespace 8,734, exact `spec validate`
  8,967, `bundle build` 8,386, `bundle status` 7,527, and `bundle trust` 8,743.

The first unqualified `task check:fast` attempt stopped at preflight because
the interactive shell selected Go 1.26.3 while `go.mod` requires 1.26.5. The
gate was not weakened; the already installed required toolchain was selected
and every gate was rerun.

The final vocabulary audit classified remaining authorization terms as one of:
superseded ADR history, explicit legacy-schema migration diagnostics,
source-owned authentication/authorization statements, future host transport
mapping, negative tests that prohibit old fields, or the generic mutation
policy for Atsura-owned adoption-store writes. No live tailored-source wrapper
contains a permission decision, inferred source read/create/write effect, or
source target/impact contract.
