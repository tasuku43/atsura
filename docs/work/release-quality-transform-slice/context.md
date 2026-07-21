# Work Context: Release-quality transform journey

## Current behavior

- `internal/cli/bundle_test.go` exercises production composition with the Go
  test executable as a synthetic GitHub CLI and proves a matching preview
  digest, one source attempt, and no unselected canary in output.
- `internal/cli/bundle_recovery_test.go` composes the production services and
  adapters into an exact-help recovery matrix covering 27 preview zero-attempt,
  28 execute pre-start, and 15 execute post-start phase cases. It uses the
  production identity reader for real file drift and narrow controlled ports
  for deterministic boundary observations. The defensive execute encoder case
  corrupts the result only after a production service/process attempt;
  undecorated result and adapter emission are proven independently by
  application/domain/infrastructure tests.
- `scripts/lint-release.sh` builds the five supported archives twice, verifies
  their contents and build metadata, and compares byte-for-byte digests.
- The working tree defines native replay for all five artifacts plus strict
  evidence/archive aggregation. Each native row also runs the production
  source-runner and complete recovery contracts, but those workflow changes do
  not yet have GitHub-hosted run evidence.
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
- Read-only GitHub inspection on 2026-07-22 found durable implementation head
  `c6df2ba7bf3d97697be067f3bacf420f83e2b0c7` absent from GitHub, with zero
  Actions runs for that SHA. Local `main` was 35 commits ahead of remote
  `main` at `10bf45d1a4d1e13a93ed917f5e25c799f4698ff2`; existing remote green checks
  cover an older workflow without the native artifact matrix and are not
  milestone evidence. Subsequent temporary-packet evidence commits are also
  intentionally local under the same no-push constraint.
- GitHub-hosted evidence cannot be created for a Git object the service does
  not possess. Under this goal's no-push constraint, workflow structure and
  current-host replay can be completed locally, but the five native results
  remain external evidence rather than something a local aggregate may infer.
- The pinned `actions/upload-artifact` contract, checked 2026-07-22, makes
  artifacts immutable and requires `overwrite: true` to delete and recreate a
  prior same-name artifact. The workflow therefore uses stable target-unique
  names with explicit replacement and disjoint archive/evidence/summary
  prefixes rather than binding downloads to only the latest run attempt.
- The first exact-SHA native run, GitHub Actions run `29869300072`, proved the
  four Linux and macOS rows and both general gates, then exposed a production
  Windows defect before packaging: `trustfile.ensurePrivateDirectory` treated
  synthesized Windows `FileMode` permission bits as POSIX ACL evidence and
  rejected every newly created trust directory. The aggregate correctly
  skipped, and no Windows or aggregate artifact was emitted. The fix retains
  shape, confinement, and identity checks on Windows while applying the
  owner-only mode check only where Unix mode bits are meaningful.

## Unknowns

- [ ] Whether native package construction exposes an OS-specific archive issue not visible to cross-build metadata checks; answer through the five-runner matrix.
- [ ] Whether the Windows trust-store and source-process path behavior agrees
  with the existing pure-Go contract; answer through native runner/recovery
  tests and exact-artifact replay.

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
- Output delivery and process facts: complete bounded local JSON; four shared
  inspect attempts, zero preview attempts, and exactly one execute attempt per
  admitted command; no retry.
- Publication and licensing concerns: none beyond existing MIT archive inputs.

## Glossary

- **Exact artifact replay:** executing the native `atr` file extracted from the
  archive produced by `scripts/package-release.sh` for that same target.
- **Conformance fixture:** a repository-owned native executable that implements
  only the finite synthetic GitHub CLI evidence and list invocation needed by
  the current accepted adapter contract.
