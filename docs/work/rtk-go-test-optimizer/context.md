# Work Context: Optimize passing Go test output with inspected RTK

This file records verified facts and unresolved questions. Desired behavior is
kept in `goal.md`, `plan.md`, and ADR 0012.

## Current behavior

- Atsura currently supports strict JSON projection and exact source-stream
  passthrough results; no processor result exists in specification schema 3,
  bundle schema 2, or plan schema 4.
- Go CLI contract 1 inspects `go version`, `go help`, and `go help test`, then
  admits only identity no-argument `go test` with source-stream passthrough.
- Source-catalog schema 1 models structured selectors as double-dash flags, so
  exact Go `-json` evidence requires a versioned selector-vocabulary change.
- The finite application runtime registry dispatches source compatibility by
  namespaced adapter kind; production contains no coding-agent host process,
  settings, hook-input rewrite, or vendor-specific core schema.
- `main` at `146201a` passed local full, security, public, and release gates and
  GitHub Actions run `29891859068`, including the five-target native aggregate.
- The capability ledger reports original-preserving optimization and RTK
  authoring defaults as deferred.

## Relevant structure

- Entry point: `cmd/atr` through `internal/cli`
- Domain rule: `internal/domain/tailoringbundle`, `tailoringplan`,
  `sourceprocess`, `operation`, and a new processor contract
- Application use case: source/processor inspection, `specinit`, `bundlebuild`,
  `tailoringplan.Build`, `planapply`, `bundleexecute`, `wrapperrun`, and
  runtime/processor compatibility registries
- Infrastructure boundary: existing `gocli` and `sourceexec`; new strict Go
  event validator, RTK inspector, processor observation codec, and isolated
  processor runner
- CLI catalog or presentation: source/spec/bundle/wrapper commands, a new
  processor inspection utility, derived help, trust/status summaries, and
  structured faults
- Existing tests and harness checks: domain/application/infrastructure/CLI
  tests, architecture/public lint, artifact journey/evidence, release archive
  checks, and native CI matrix

## Constraints

- RTK receives only source stdout after Atsura starts the exact source once.
- No runtime detection, automatic install, arbitrary executable selection,
  arbitrary argv, shell, project filter, agent-host state, or credential
  inheritance is allowed.
- Conventional ineligible source results are preserved before processor start;
  processor faults never trigger raw fallback or disclose intermediate bytes.
- The user supplies and reviews exact processor evidence; the specification
  binds only a finite compatibility contract and the bundle/plan bind identity.
- Go 1.26.x is an inspection-time effective-toolchain observation, not a runtime
  closure; the unchanged launcher can select another toolchain under ambient
  working-directory, module, and environment state.
- RTK is a user-supplied Apache-2.0 executable and is not bundled in Atsura;
  its strict dependency/provenance manifest is evidence, not an SBOM.
- Repository documents remain English and public-safe; fixtures are synthetic.
- Commit and push are authorized; tag, release, PR, and publication are not.

## External facts

- RTK v0.43.0 release, official GitHub release, checked 2026-07-22: latest
  stable release published 2026-06-28 at commit
  `5a7880d404db8364d602f2ecdc41dd790f64013f`; official archives and checksums
  exist for claimed native targets.
- RTK pipe and Go filter source at that commit, checked 2026-07-22: malformed
  JSON and unknown actions can be ignored; the pipe does not receive source
  status/stderr; the never-worse guard compares approximate token counts; the
  release profile aborts on panic.
- RTK main, telemetry, and hook-warning source at that commit, checked
  2026-07-22: telemetry precedes CLI parsing unless disabled, and an ambient
  Claude configuration can produce warning stderr and a marker file.
- Go `test2json` documentation, checked 2026-07-22: output is newline-separated
  `TestEvent` JSON with documented actions; package streams can interlace for
  multi-package invocation, while this tuple admits exactly one package.
- Official macOS arm64 RTK artifact, observed 2026-07-22: exact `--version`
  emits `rtk 0.43.0`; a real Go 1.26.5 passing package emitted
  `Go test: 28 passed in 1 packages` with no trailing newline.
- Hostile official-artifact observations, 2026-07-22: skip-only was
  misclassified, malformed lines were omitted, failure status was lost, and
  two-package failure order varied across repeated processes. An isolated
  pass-only pipe invocation required no file write or observed network attempt.

## Unknowns

- [ ] Exact public processor-observation schema names and fault codes; decide in
      contract tests before implementation.
- [ ] Exact version increments for agent help, capability ledger, and native
      evidence after tracing all generated consumers.
- [ ] Whether Linux amd64/arm64 and Darwin amd64/arm64 can run the pinned
      official RTK artifact under the exact isolation contract; answer with
      native CI. Windows is explicitly outside this optimizer runtime matrix.
- [ ] Whether a platform needs a minimal OS-specific environment variable not
      present in the portable base; answer with isolated native fixtures and
      document only observed additions.
- [ ] Exact token comparison for the frozen pass fixture; answer after the
      typed fixture and independent answer key are checked in.
- [ ] The linked `go.yaml.in/yaml/v3` module carries an upstream Apache NOTICE,
      while the current Atsura archive has no `THIRD_PARTY_NOTICES`. This
      pre-existing release-compliance gap is independent of RTK and needs its
      own reviewed notice fix before publication.
- [ ] Later source/RTK tuples, versions, filters, auto-install UX, RTK internal
      reuse, and coding-agent adapters remain outside this iteration.

## Thesis evidence

- Repeated design decision or point of agent confusion: output optimization was
  repeatedly conflated with strict projection, source execution, raw fallback,
  or host request rewriting.
- User outcome or friction observed in the minimal slice: users want RTK-scale
  output reduction by default where it is already supported, while Atsura must
  stay below and independent of coding-agent hosts.
- Code workaround or exception being considered: invoking RTK directly from the
  Go adapter or reusing the source runner would mix source and processor policy,
  inherit ambient state, and lose stdin/fault/attempt distinctions.
- Current thesis that resolves it: typed original-preserving optimizer,
  explicit materialized default, exact processor identity, and separate finite
  processor compatibility/execution boundaries.
- Downstream impact: product contract, architecture, security, add-capability
  workflow expectations, catalog/help, capability ledger, schema versions,
  artifact evidence, and release/public gates.

## Reproduction or observation

```sh
go test -json | env -i \
  HOME=/var/empty/atsura-rtk \
  XDG_CONFIG_HOME=/var/empty/atsura-rtk/config \
  XDG_DATA_HOME=/var/empty/atsura-rtk/data \
  XDG_CACHE_HOME=/var/empty/atsura-rtk/cache \
  CLAUDE_CONFIG_DIR=/var/empty/atsura-rtk/claude \
  RTK_TELEMETRY_DISABLED=1 RTK_NO_TOML=1 \
  /absolute/path/to/rtk pipe --filter=go-test
```

On macOS arm64 with Go 1.26.5 and official RTK v0.43.0, a passing synthetic
package produced one newline-free summary. This command records research only;
production must additionally bind identity, isolate all roots, enforce limits,
validate input/output, and avoid relying on shell pipelines.

## Security and public-boundary notes

- Assets and side effects involved: untrusted source executable/output, exact
  user-supplied RTK executable, two local processes, temporary isolated roots,
  stdout/stderr delivery, and source-owned test effects.
- Credentials or confidential data involved: source commands may observe
  ambient credentials; Atsura does not persist them, and the processor receives
  no ambient credential environment or source stderr.
- New dependencies, destinations, files, processes, or generated content: no Go
  library dependency; optional local RTK process; temporary isolated roots;
  official GitHub release downloads only in pinned native evidence.
- External schema provenance, publication rights, and drift evidence: Go event
  schema is authoritative upstream documentation; RTK is Apache-2.0 and pinned
  to release, commit, archive checksum, binary identity, and source behavior.
- Output delivery and retry: complete bounded streams, at most one source and
  one processor attempt, non-retryable after source start, exact final write
  ordering, and no intermediate leak on uncertain/processor faults.
- Publication and licensing concerns: RTK is not redistributed. Any future
  vendoring or archive inclusion needs a separate license/notice/SBOM review.

## Glossary

- **Processor observation:** canonical identity and version evidence from one
  explicit executable path; it grants no execution authority by itself.
- **Compatibility tuple:** finite mapping of source adapter/version/command and
  transformed argv to one processor contract/version/filter and result grammar.
- **Eligible input:** conventional source result that passes the strict
  pass-only Go event validator and may be sent to RTK.
- **`preserved_before_processor`:** exact conventional transformed source
  result returned because it is ineligible; the processor was not attempted.
- **`preserved_after_processor`:** valid processor stdout byte-identical to its
  admitted input after one processor attempt.
- **Optimized:** valid processor stdout equal to Atsura's independently computed
  exact summary and different from its admitted input.
