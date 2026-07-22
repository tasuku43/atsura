# Work Context: Ordinary tailored help

## Pre-change behavior

- `internal/infra/posixwrapper/render.go` emits one fixed function that
  forwards every `"$@"` value to `atr wrapper run`.
- `internal/app/wrapperrun/service.go` sends every forwarded argv list to the
  shared fresh-plan application path.
- `internal/domain/tailoringplan/plan.go` requires argv to begin with a
  cataloged command path and treats long options through the command option
  grammar. Root `--help` has no command path; namespace and exact-command help
  do not produce a tailored presentation.
- The bundle already contains canonical source summaries, the compiled
  included surface, exact option membership, tailoring reasons, wrapper
  behavior, and exact source/processor identities. No new inspection or
  registry is needed to derive help.
- At the slice start, complete-surface runtime admission permits one included
  command, so the first generated help corpus is naturally bounded even though
  the domain contract must still enforce its own explicit limits.

## Implemented contract

- `wrapperbinding.CompileHelp` derives one detached semantic snapshot from the
  exact valid bundle. It stores each included exact command once with its
  source summary, tailoring reason, and effective long-option arity; root,
  namespace, exact, and combined exact/namespace views are derived from that
  canonical snapshot.
- Generated-wrapper contract 2 recognizes only a final exact `--help` for the
  root or a compiled selector. It emits fixed literal lines before the runtime
  fallthrough and therefore does not start the bound `atr`, source, or
  processor. Every nonmatching argv remains unchanged.
- The generated function body is a subshell. It removes inherited `command`
  and `return` functions only inside that body through alias-safe POSIX
  special-builtin lookup, fails before runtime start if cleanup is unavailable,
  and then uses escaped `command` lookup for fixed matching and formatting.
- Generated material removes an exact same-name alias before defining the
  ordinary function. This in-memory effect is part of caller-owned sourcing
  and prevents the alias from renaming or bypassing the wrapper.
- Supported activation assumes `unalias` retains its standard POSIX meaning;
  a caller-defined `unalias` function cannot be bypassed and preserved by a
  portable top-level preamble.
- The semantic snapshot is limited to 256 total views, 2,048 emitted semantic
  lines, and 48 KiB of literal payload. The independently validated rendered
  material remains limited to 64 KiB.
- An exact command that is also a namespace has one view: included descendant
  paths are listed first and the exact command's own summary, reason, and
  options follow. There is no competing or ambiguous second view.
- The wrapper review envelope remains schema 2 and its nested
  `wrapper-contract` shape remains schema 1. Installed-artifact evidence
  advances to schema 6; aggregate evidence remains schema 2.

## Relevant structure

- Entry point: `atr wrapper render`; generated ordinary POSIX function
- Domain rule: `sourcecatalog.Catalog`, `tailoringbundle.Bundle`, and
  `wrapperbinding.Binding`
- Application use case: `internal/app/wrapperrender`
- Infrastructure boundary: `internal/infra/posixwrapper`
- CLI catalog or presentation: `internal/cli/catalog.go` and
  `internal/cli/wrapper.go`
- Existing tests and harness checks: wrapper render/run unit and integration
  tests, `tools/artifactjourney`, release evidence aggregation, and full gates

## Constraints

- Help is a projection of the exact adopted compiled surface, not source help,
  an authorization decision, or an execution plan.
- The source catalog and bundle remain the only semantic source of truth.
- Generated code stays fixed product code: no specification-authored shell,
  `eval`, `sh -c`, arbitrary format string, or runtime LLM.
- Only a final exact `--help` token is wrapper-owned in this slice.
- Non-help argv must continue byte-for-byte to the existing `wrapper run`
  boundary.
- Help must remain host-neutral and must not name Claude, Codex, hooks,
  sessions, models, or host permission concepts.
- The generated wrapper source remains bounded, NUL-free UTF-8 and
  content-digested.

## External facts

None. This slice changes only repository-owned contracts and uses the POSIX
shell grammar already accepted by the wrapper renderer.

## Resolved questions

- [x] The binding carries a typed semantic projection; rendered shell lines
      remain infrastructure-owned presentation.
- [x] The generated-wrapper contract advances from 1 to 2. The review-envelope
      schemas do not change because their field inventories are unchanged.
- [x] Root and namespace views list full included exact paths without
      truncation; exact views state the source summary, tailoring reason, and
      effective long options with value arity. Explicit domain and final
      material bounds reject oversized surfaces.
- [x] An exact command that is also a namespace receives one combined view.

No unresolved question remains inside this bounded slice. Vendor aliases,
short options, positional/global grammar, installation, and richer argv
transforms remain explicit non-goals rather than hidden decisions.

## Thesis evidence

- Repeated design decision or point of agent confusion: command rejection was
  repeatedly described as making a command invisible even though the ordinary
  wrapper could not positively display the reduced surface.
- User outcome or friction observed in the minimal slice: an agent must know
  Atsura's bundle tooling or guess source commands instead of asking the
  ordinary tailored CLI what exists.
- Code workaround or exception being considered: routing help through the
  fresh execution plan would falsely make presentation an execute result.
- Current thesis that resolves it, or proposed thesis revision: compile a
  bounded surface projection into generated wrapper material; keep help
  orthogonal to source execution and caller/host integration.
- Downstream impact: theses, product, architecture, security, ADR, wrapper
  binding/rendering, catalog help, release journey, evidence schemas if their
  shape changes, and harness contract tests.

## Reproduction or observation

The pre-change gap was established by inspection and the existing tests at the
base revision: rendered material had no help branch and every argv reached
`wrapper run`; no live source or network call was required.

The implemented focused contract was verified with:

```sh
go test ./internal/domain/tailoringbundle ./internal/domain/wrapperbinding \
  ./internal/infra/posixwrapper ./internal/app/wrapperrender ./internal/cli \
  ./tools/artifactjourney ./tools/artifactevidence
```

This passed on 2026-07-22. `task check`, `task security`,
`task public:check`, and `task release:check` also passed on that date. Native
installed-artifact execution remains pending because `task release:check`
validates the release contract and workflow rather than replaying a local
archive.

## Security and public-boundary notes

- Assets and side effects involved: read-only canonical bundle input and
  generated POSIX bytes; successful help starts no bound `atr`, source, or
  processor and performs no filesystem mutation. POSIX may implement its
  formatting utility in a separate process.
- Credentials or confidential data involved: none. Fixtures remain synthetic.
- New dependencies, destinations, files, processes, or generated content: no
  dependency or network destination; only bounded generated wrapper content.
- External schema provenance, publication rights, and drift evidence: source
  summaries are existing bounded inspection evidence and remain untrusted
  visible data.
- Output delivery: one fixed `printf` call emits the complete line set; an OS
  write failure may leave caller-visible partial bytes and returns the shell's
  conventional nonzero status. Pagination, retry, idempotency, and external
  API cancellation are not applicable.
- Publication and licensing concerns: none beyond existing repository-owned
  code and synthetic fixtures.

## Glossary

- **Tailored help**: a deterministic presentation derived only from an exact
  bundle's included surface; never raw source help.
- **Help selector**: zero or more stable command-path segments immediately
  followed by the wrapper-owned final `--help` token.
- **Ordinary spelling**: the source command name used to invoke the generated
  wrapper, such as `gh` or `go`.
