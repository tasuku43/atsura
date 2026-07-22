# Work Context: Add the second source CLI runtime

This file records verified facts and unresolved questions. It does not make a
planned Go adapter or runtime a current product claim.

## Current behavior

- Production source inspection registers only `github-cli`; the shared catalog
  already represents adapter identity as the namespaced `Adapter.Kind` plus a
  positive contract version.
- The canonical specification, bundle, plan, wrapper binding, and dynamic
  wrapper result contain no GitHub CLI or coding-agent-host field.
- `wrapper run` already returns exact bounded source streams and conventional
  status for a finite identity or append-only GitHub CLI surface.
- The production composition injects one GitHub CLI verifier directly into
  plan application and wrapper rendering. A second adapter therefore needs a
  host-neutral compatibility registry, not a second executor or plan path.
- `sourcecatalog.Option` intentionally describes observed long options. Go's
  build and test flags use single-dash spellings, so this iteration can retain
  an explicit empty long-option inventory and reject every caller-supplied
  option before start without claiming those source flags do not exist.

## Relevant structure

- Entry point: `internal/cli/source.go`, `internal/cli/wrapper.go`
- Domain rule: `internal/domain/sourcecatalog`, `tailoringbundle`, and
  `tailoringplan`
- Application use case: `internal/app/sourceinspect`, `planapply`,
  `wrapperrender`, and `wrapperrun`
- Infrastructure boundary: `internal/infra/githubcli` and `sourceexec`
- CLI catalog or presentation: `internal/cli/catalog.go`
- Existing tests and harness checks: source adapter contracts, alternate
  synthetic catalog fixtures, wrapper journeys, `archlint`, `repoguard`, and
  `tools/artifactjourney`

## Constraints

- The Go adapter may only report evidence established by fixed bounded probes;
  it must not infer source-operation safety from help prose.
- Source execution remains exactly one identity-bound no-shell attempt with
  closed stdin and inherited working directory/environment.
- `go test` can compile and execute untrusted project code, write build caches,
  consult environment/configuration, resolve modules, access networks, and
  perform arbitrary downstream effects. Those remain source-owned semantics.
- The finite first runtime accepts no caller argument after `test`. This avoids
  inventing Go's single-dash, package-pattern, test-binary, and cross-flag
  grammar in the shared core.
- No new third-party Go dependency is needed. Go documentation and source are
  BSD-3-Clause; no upstream content or binary is copied into the repository.
- Documentation remains English and public fixtures contain synthetic data
  only.

## External facts

- Go command documentation, <https://pkg.go.dev/cmd/go>, checked 2026-07-22:
  Go 1.26.5 documents `go <command> [arguments]`, lists `test` as a built-in
  command, and describes `go test` as compiling and running package tests.
- Go `test2json` documentation, <https://pkg.go.dev/cmd/test2json>, checked
  2026-07-22: `go test -json` is the preferred producer of the documented
  newline-delimited `TestEvent` stream. This is evidence for the successor RTK
  iteration, not a contract implemented in this slice.
- RTK v0.43.0 release and source,
  <https://github.com/rtk-ai/rtk/releases/tag/v0.43.0> and
  <https://github.com/rtk-ai/rtk/blob/v0.43.0/src/cmds/system/pipe_cmd.rs>,
  checked 2026-07-22: the fixed pipe registry has no GitHub CLI filter and does
  include `go-test`. No RTK artifact is added by this slice.

## Unknowns

- [x] Which next source best enables a useful RTK tuple? Go CLI, because its
      documented test event stream has a fixed RTK filter and native hostile
      evidence showed material pass-result reduction.
- [x] Should the first Go runtime admit options or package arguments? No; each
      grammar family requires separate evidence and is rejected pre-start.
- [ ] Which exact status-zero Go test event lifecycle qualifies for RTK, and
      which ineligible conventional results are explicitly preserved before a
      processor attempt? Answer in the successor optimizer ADR and work packet.
- [ ] How should effective `GOFLAGS` and `GOENV` be closed or represented for a
      future `-json` transform? Answer with a bounded native prototype before
      accepting the RTK tuple.

## Thesis evidence

- Repeated design decision or point of agent confusion: source adapters and
  coding-agent hosts were previously conflated; a second source adapter tests
  the corrected orthogonality directly.
- User outcome or friction observed in the minimal slice: an identity draft is
  useful only when the ordinary wrapper can execute it without requiring JSON
  projection or vendor-specific activation.
- Code workaround or exception being considered: adding a Go branch inside the
  generic application service.
- Current thesis that resolves it, or proposed thesis revision: dispatch exact
  source contracts through a finite injected compatibility registry while
  retaining one shared plan and executor.
- Downstream impact: product compatibility matrix, architecture composition,
  security claims, source-inspection help, artifact journeys, and the
  add-capability Skill must name multi-source evidence without adding vendor
  fields to core schemas.

## Reproduction or observation

```sh
go version
go help
go help test
```

Observed on Darwin arm64 with Go 1.26.5:

- `go version` emitted `go version go1.26.5 darwin/arm64`.
- root help contained one bounded built-in command table including `test`.
- test help began with the exact no-shell usage
  `go test [build/test flags] [packages] [build/test flags & test binary flags]`.
- all three commands exited zero without stderr.

## Security and public-boundary notes

- Assets and side effects involved: source executable identity; three
  inspection attempts; one optional routine Go test process; caller-owned Go
  caches, module files, and test effects.
- Credentials or confidential data involved: none supplied by Atsura. The Go
  process inherits caller state exactly as the plan already declares.
- New dependencies, destinations, files, processes, or generated content: no
  dependency or destination; one new source adapter and native fixture. The
  release fixture uses a dependency-free synthetic module with module download
  disabled and isolated cache roots.
- External schema provenance, publication rights, and drift evidence: Go
  command/help behavior is pinned to adapter contract 1 and Go 1.26.x; probe
  grammar drift fails inspection.
- Output delivery, timeout, retry, and cancellation: exact source streams under
  existing 4 MiB/256 KiB and 30-second bounds; zero or one source attempt;
  unknown post-start outcomes and final write failures are non-retryable.
- Publication and licensing concerns: documentation links only; no Go or RTK
  bytes are redistributed.

## Glossary

- **Go CLI adapter contract 1:** three fixed offline probes plus the finite
  no-argument `go test` source-stream runtime grammar for Go 1.26.x.
- **No-argument runtime:** the transformed argv is exactly `test`; no option,
  package pattern, positional marker, or test-binary argument follows it.
- **Compatibility registry:** an application-owned finite dispatcher from the
  plan's namespaced source adapter contract to one injected verifier; it is not
  a source catalog, executor, or plugin registry.
