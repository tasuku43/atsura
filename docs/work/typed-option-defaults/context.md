# Work Context: Apply one reviewable option default only when caller input omits it

## Current behavior

- Specification schema 4 has only `invoke.append_args`; it requires an explicit
  list and appends every element after all caller argv.
- `tailoringplan.Build` validates the caller's cataloged long options and then
  appends configured argv. It preserves explicit empty inline values at the
  generic plan boundary.
- Current plan schema 5 records original argv, transformed argv, and appended
  args but has no default decision.
- Contract-2 tailored help shows effective option names and value arity, not an
  invocation default.
- GitHub CLI contract 2 admits value-taking `--limit` for `pr list` and can
  prove both transformed-JSON and source-stream result modes.
- The complete-surface verifier already rejects any invalid later entry before
  rendering.

## Relevant structure

- Entry point: existing `spec init`, `spec validate`, `bundle build`, `bundle
  preview`, `wrapper render`, and ordinary generated `gh`
- Domain rule: `tailoringbundle.Invocation`, `tailoringplan.Build`, and
  `wrapperbinding.CompileHelp`
- Application use case: existing bundle build/preview, wrapper render/run, and
  runtime compatibility registry
- Infrastructure boundary: strict YAML, GitHub/Go compatibility verifiers,
  fixed POSIX renderer, and existing source-process adapter
- CLI catalog or presentation: existing schema inventories and wrapper help
- Existing tests and harness checks: domain truth tables, strict loaders,
  complete-surface tests, artifact journey, evidence aggregation, and CI

## Constraints

- A default cannot rely on the source CLI's duplicate-option precedence.
- Source catalog 2 proves only exact long-option name and value arity.
- Output selectors remain owned by output stages and cannot be defaults.
- Configured values are visible in YAML, bundle, plan, help, tests, and evidence
  and must not contain credentials or secrets.
- No source or processor attempt may occur during validation, preview, render,
  or static help.
- Runtime remains one identity-bound no-shell source attempt with existing
  output bounds and cancellation semantics.
- Documentation remains English and fixtures remain synthetic.

## External facts

None. This iteration relies only on repository-owned catalog contract 2 and the
existing synthetic GitHub compatibility fixture. It makes no new claim about a
live provider or coding-agent host.

## Resolved unknowns

- [x] Use `invoke.option_defaults: [{option, value}]`, not magic `append_args`.
- [x] Support only included cataloged value-taking long options with non-empty
      exact string values.
- [x] Insert canonical `--option=value` after the matched command path only when
      no caller occurrence exists before `--`.
- [x] Plan records explicit declared and applied default lists.
- [x] Preserve repeated caller occurrences without claiming source precedence;
      short aliases and positional text after `--` do not suppress a default.
- [x] Bump spec, bundle, plan, wrapper, and evidence contracts; do not infer
      backward compatibility or auto-adopt rebuilt content.

## Thesis evidence

- Repeated design decision or point of agent confusion: fixed appended argv is
  repeatedly mistaken for a caller-overridable default.
- User outcome or friction observed in the minimal slice: the caller must repeat
  `--limit` even though the adopted purpose already knows the routine value.
- Code workaround or exception being considered: duplicate configured and caller
  options would delegate policy to undocumented source precedence.
- Current thesis that resolves it: deterministic argv defaults are a distinct
  typed invocation operation, visible in preview and help.
- Downstream impact: product, architecture, security, schema inventories,
  adoption summary, help, runtime admission, evidence, release, and readiness.

## Reproduction or observation

```sh
atr bundle preview --bundle <bundle> -- gh pr list
atr bundle preview --bundle <bundle> -- gh pr list --limit=2
```

Current schema 4 can only encode an unconditional appended value. The first
command cannot produce a plan-owned default decision, and the second cannot
prove caller precedence without a new operation.

## Security and public-boundary notes

- Assets and side effects involved: public configuration and generated wrapper
  bytes; ordinary execution retains the existing source-process boundary.
- Credentials or confidential data involved: none; defaults are explicitly not
  a credential storage contract.
- New dependencies, destinations, files, processes, or generated content: no
  dependency, destination, process, or host integration; only versioned fields
  and changed deterministic wrapper/help bytes.
- External schema provenance, publication rights, and drift evidence: existing
  repository-owned schemas and synthetic fixtures only.
- Output delivery, collection coverage, pagination, timeout, retry,
  idempotency, and cancellation facts: unchanged; default planning performs no
  I/O and ordinary execution still permits one source attempt.
- Publication and licensing concerns: none beyond the existing release model.

## Glossary

- **Declared default**: one reviewed option/value pair in the bundle.
- **Applied default**: a declared default inserted because valid caller argv
  omitted the option before `--`.
- **Caller override**: any valid inline, separated, or explicit-empty long-option
  occurrence before `--`; Atsura preserves every occurrence and inserts no
  configured value.
