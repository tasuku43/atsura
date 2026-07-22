# Work Context: Optimize passing Go test output with inspected RTK

This file records verified facts and unresolved questions. Desired behavior is
kept in `goal.md`, `plan.md`, and ADR 0012.

## Current behavior

- The implementation now contains the typed original-preserving optimizer
  result, processor observation/compatibility contracts, strict Go pass-event
  admission, and isolated processor execution boundary.
- Go CLI contract 2 records the exact `go_test_jsonl` selector `-json`
  separately from the empty caller option surface and admits only the reviewed
  no-argument `go test` tuple.
- Application truth tables distinguish `preserved_before_processor`,
  `preserved_after_processor`, and `optimized`, enforce truthful source and
  processor attempts, and suppress all intermediate bytes on processor faults.
- The finite application runtime registries keep source and processor
  compatibility separate; production contains no coding-agent host process,
  settings, hook-input rewrite, or vendor-specific core schema.
- Processor inspection, explicit authoring evidence, bundle identity binding,
  wrapper plan application, and projection-facade rejection have focused
  implementation tests. Native installed-artifact evidence and canonical gates
  are not yet completion evidence for this packet.
- `main` at `146201a` was the last recorded pre-iteration revision with local
  full, security, public, and release gates plus GitHub Actions run
  `29891859068`; that result does not validate the current optimizer change.

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
  two-package failure order varied across repeated processes.
- A prior isolated pass-only invocation did not report a file write or network
  attempt, but it did not use a reviewed external child-process/filesystem/
  network observer contract. It is research context only and is not acceptance
  evidence for absence of those effects.
- For the admitted pass fixture, Atsura independently requires a strictly
  smaller 31-byte summary before starting RTK. Official RTK v0.43.0
  deterministically emits that same summary for the admitted grammar.
  Consequently, the fixed artifact has no deterministic
  `preserved_after_processor` fixture; that postcondition remains a required
  controlled application truth-table case.
- The fixed official executable and argv expose no deterministic input that
  produces processor start, timeout, signal, cancellation, nonzero, stderr,
  overflow, post-run identity drift, unexpected-output, or cleanup faults.
  Those mandatory one-attempt/no-byte branches are controlled application/
  infrastructure tests, not installed official-artifact cases. Preflight and
  eligible post-source drift remain deterministic installed-wrapper cases
  because the journey controls replacement between the relevant identity
  checks.
- The checked-in pass fixture is 1,273 bytes with SHA-256
  `a876a23b60dad0984d822f98c2ed5a94f82e368e985bdd19e5bd5bb90a733885`.
  Its independent answer key has SHA-256
  `060e4e2ee88ced24bc53d5916f953588c252165134250c2f24c7fd5d0ab67a95`.
  The exact newline-free 31-byte summary has SHA-256
  `a4f3dee01192dc3d1e710a3301d7f9f35bf7e7f14135b4a96ce398dc3af043b4`.

## Unknowns

- [ ] Exact remaining version increments for agent help, capability ledger, and
      native evidence after tracing all generated consumers.
- [ ] Whether Linux amd64/arm64 and Darwin amd64/arm64 can run the pinned
      official RTK artifact under the exact isolation contract; answer with
      native CI. Windows is explicitly outside this optimizer runtime matrix.
- [ ] Whether a platform needs a minimal OS-specific environment variable not
      present in the portable base; answer with isolated native fixtures and
      document only observed additions.
- [ ] Platform-specific external observer contracts for child-process,
      filesystem, and network activity. No absence claim is accepted until the
      observer, event grammar, bounds, and failure semantics are proved.
- [ ] A vendor-neutral tokenizer contract. Token counts are intentionally not
      asserted for this evidence; exact byte count and hash are authoritative.
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
