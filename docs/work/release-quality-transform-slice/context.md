# Work Context: Release-quality transform journey

## Current behavior

- `internal/cli/bundle_test.go` exercises production composition with the Go
  test executable as a synthetic GitHub CLI and proves a matching preview
  digest, one source attempt, and no unselected canary in output.
- `scripts/lint-release.sh` builds the five supported archives twice, verifies
  their contents and build metadata, and compares byte-for-byte digests.
- `.github/workflows/release.yml` currently cross-builds all five artifacts on
  one Linux amd64 runner and does not execute the extracted non-host binaries.
- `bundle trust` requires a controlling terminal through the production
  confirmation adapter. Existing application, CLI, and infrastructure tests
  cover that interaction; automation must not add a public confirmation bypass.

## Relevant structure

- Entry point: `cmd/atr/main.go`
- Domain rule: `internal/domain/tailoringbundle`, `tailoringplan`, and `bundletrust`
- Application use case: `sourceinspect`, `specinit`, `bundlebuild`,
  `bundleauthority`, `planpreview`, and `bundleexecute`
- Infrastructure boundary: `githubcli`, `sourceexec`, `sourcejson`, `specyaml`,
  `bundlejson`, `terminalconfirm`, and `trustfile`
- CLI catalog or presentation: `internal/cli/catalog.go` and `README.md`
- Existing tests and harness checks: `internal/cli/bundle_test.go`,
  `scripts/lint-release.sh`, and `.github/workflows/ci.yml`

## Constraints

- The conformance fixture must use no credentials, network, shell evaluation,
  hidden source operation, or provider SDK.
- The exact public archive executable, not a separately rebuilt `atr`, must be
  the runtime under test.
- All five existing pure-Go targets remain claimed until a durable release
  decision changes the matrix.
- The public CLI and schema do not expand in this work.

## External facts

- GitHub Docs, “GitHub-hosted runners reference,” checked 2026-07-22:
  standard public-repository runner labels exist for Linux x64 and arm64,
  macOS Intel and arm64, and Windows x64. The selected labels are recorded in
  the workflow rather than inferred from `-latest` architecture.

## Unknowns

- [ ] Whether native package construction exposes an OS-specific archive issue not visible to cross-build metadata checks; answer through the five-runner matrix.
- [ ] Whether the Windows trust-store and source-process path behavior agrees with the existing pure-Go contract; answer through exact-artifact replay.

## Thesis evidence

- Repeated design decision or point of agent confusion: unit and in-process integration evidence cannot establish that the downloaded artifact is the tested program.
- User outcome or friction observed in the minimal slice: the documented workflow is runnable, but release readiness still requires manual per-platform artifact execution.
- Code workaround or exception being considered: a public noninteractive trust flag would simplify CI but would weaken the adoption boundary.
- Current thesis that resolves it: test-only orchestration may seed the same user-local receipt store while production trust remains interactive and independently tested.
- Downstream impact: release workflow, harness enforcement, release documentation, and agent-readiness evidence; no public command or capability-ledger change.

## Reproduction or observation

```sh
env -u GOROOT PATH=<go-1.26.5-bin>:$PATH task release:check
```

Before this change, the gate inspects every archive but executes only the host
binary's `version` command during packaging.

## Security and public-boundary notes

- Assets and side effects involved: temporary catalog, specification, bundle,
  fixture log, and user-config root under an isolated temporary directory.
- Credentials or confidential data involved: none; child processes receive a
  minimal environment and the source fixture has no network implementation.
- New dependencies, destinations, files, processes, or generated content: no
  dependency or network destination; one native fixture process and the exact
  packaged `atr` process.
- External schema provenance, publication rights, and drift evidence: the
  GitHub-compatible help is synthetic and minimal; it is not copied provider
  documentation.
- Output delivery and process facts: complete bounded local JSON; four inspect
  attempts, zero preview attempts, exactly one execute attempt; no retry.
- Publication and licensing concerns: none beyond existing MIT archive inputs.

## Glossary

- **Exact artifact replay:** executing the native `atr` file extracted from the
  archive produced by `scripts/package-release.sh` for that same target.
- **Conformance fixture:** a repository-owned native executable that implements
  only the finite synthetic GitHub CLI evidence and list invocation needed by
  the current accepted adapter contract.
