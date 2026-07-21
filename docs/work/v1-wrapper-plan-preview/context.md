# Work Context: Wrapper-plan preview

## Verified foundation

- Repository profile is `ready`; the tree was clean at `d53ec73` before this
  packet.
- Specification schema 3 and bundle schema 2 already bind a validated catalog,
  source identity, compiled surface, option membership, and complete
  identity/transform wrappers.
- `tailoringbundle.Bundle.Resolve` already distinguishes included surface
  entries from `ErrCommandNotInSurface` for an exact command path.
- `bundlejson.Loader` strictly loads and revalidates every bundle binding.
- The user-local store exposes exact-digest adoption state without mutation.
- `sourceexec.Runner.Identify` verifies the current regular executable without
  starting it.
- ADR 0005 and the theses already define the minimum complete plan fields and
  require preview and runtime to share one pure constructor.
- Capability `tailoring.preview` is deferred specifically for this slice.
- The legacy `plan preview --config` catalog path is migration-only and must
  remain distinguishable from the new bundle-backed preview.

## Fixed decisions for this milestone

- Public command: `bundle preview`.
- Input grammar: `--bundle <path> -- <source-executable> <argv...>`.
- The attempted executable must exactly equal the bundle's requested
  executable or resolved path; execution always plans the resolved path.
- Command resolution uses the longest exact catalog command-path prefix at the
  start of source argv. No match is `invalid_invocation`.
- The matched command must be in the compiled surface or the result is
  `command_not_in_surface` with no plan.
- Only observed long options are modeled. A surfaced option is allowed; a
  cataloged but excluded option is `option_not_in_surface`; unknown long or
  unmodeled short options are `invalid_invocation`. Values after `--` are
  positional data, not options.
- A value-taking long option accepts `--name=value` or a non-dash next token;
  dash-prefixed values require equals form. A flag without a value rejects an
  equals payload.
- Wrapper-appended argv is specification-owned and independent from the
  agent-visible option surface.
- Public plan schema version is 2 because schema-1 preview represented the
  retired authorization model.
- Plan mode is `tailored`; raw remains future scope.
- The plan declares one potential runtime source attempt while preview reports
  zero actual source attempts.
- Adoption and current source identity are application preconditions evaluated
  before pure plan construction.

## Risks and unknowns

- The catalog does not model positional arguments, short options, root/global
  options, or every source parser convention. This slice preserves ordinary
  positional values but fails closed on unmodeled dash-prefixed options.
- Some source CLIs permit options before subcommands. This initial grammar
  requires the command path first so selection is deterministic from current
  catalog evidence.
- Accepting both requested and resolved executable spellings is deterministic
  but deliberately does not add basename or PATH alias inference.
- Runtime must re-run adoption, source identity, bundle, and plan construction;
  a preview document is evidence for review, never execution authority.
- Plan fields and digest are pre-release but must not reuse retired schema-1
  meaning.

## Evidence required

- Pure fixture and answer key for identity and transforming wrappers.
- Stable canonical plan bytes and digest across repeated construction.
- Longest-prefix, unknown command, excluded command, option membership,
  option-value form, positional-only, and hostile argv tests.
- Application ordering tests proving unadopted/drifted bundles do not invoke
  the constructor and no path starts a source process.
- CLI argv-to-JSON, structured error, output-write, help, catalog, and exact-key
  contract tests.
- Agent-readiness transcript showing root plus one scoped help call, direct
  plan consumption, and zero external processing/source attempts.
- Focused, full, and security gate results from the final tree.
