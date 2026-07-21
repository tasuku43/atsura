# Work Context: Release-quality local tailoring run

## Current behavior

- Commit `5a4c788` exposes deterministic `atr plan preview` with a strict 64 KiB schema-1 YAML loader and zero process boundary.
- Schema 1 currently declares executable, argv prefix, allow/deny, appended argv, JSON select/rename, and compact render, but no operation effect.
- The repository has generic operation effects, structured faults, catalog-derived help, complete output writes, architecture lint, and release packaging gates.
- The current release workflow builds five pure-Go platform tuples, but Atsura has made no project-specific release promise and this work will not publish one.

## Relevant structure

- Entry point: `cmd/atr/main.go`
- Domain rule: `internal/domain/tailoring` and new `internal/domain/sourceprocess`
- Application use case: new `internal/app/tailorrun`
- Infrastructure boundary: existing `internal/infra/tailoringyaml`, new process and JSON adapters
- CLI catalog or presentation: `internal/cli/catalog.go`, new run handler and renderer
- Existing tests and harness checks: catalog, JSON-schema, hostile-output, archlint, contractlint, security, public, and release profiles

## Constraints

- The public command is `EffectRead`; schema 1 accepts only the explicit `read` source effect.
- An explicit `--config` path is trust selection for that invocation only; no file is discovered or activated automatically.
- Executable and argv remain separate and are passed directly to `os/exec`; policy never supplies shell source.
- The process inherits the current directory and environment, receives EOF stdin, and gets one direct start attempt.
- Successful source stdout must be JSON object or array of objects; all failures suppress raw stdout.
- Output and stderr bounds are byte limits, not truncation promises; overflow is failure.
- The final implementation must compile and package on the inherited release matrix without claiming signing, notarization, or provenance.

## External facts

- No new third-party dependency is selected. Process control, SHA-256 identity evidence, JSON parsing, and rendering use the pinned Go standard library.
- `go.yaml.in/yaml/v3` v3.0.4 remains the sole runtime third-party module and stays confined to infrastructure.

## Unknowns

- [x] Supported effect for v0.1: explicit read only; mutations require a later contract.
- [x] Timeout and byte budgets: 30 seconds, 4 MiB stdout, 256 KiB stderr, one direct attempt.
- [x] Success shape: fixed execution envelope with result shape, ordered output fields, records, explanation, and attempt count.
- [x] Source failure policy: no retry, no raw fallback, no partial success.
- [x] Configuration trust: explicit path selection for one invocation, without persistence or repository activation.
- [ ] Exact executable fingerprint fields and cross-platform revalidation behavior; settle with adapter tests before implementation.
- [ ] Exact JSON complexity limits; settle with parser fixtures before implementation.

## Thesis evidence

- Repeated design decision or point of agent confusion: a plan is useful only if one controlled runtime applies exactly its argv and output stages.
- User outcome or friction observed in the minimal slice: preview demonstrates intent but cannot yet prove RTK-scale structural output replacement.
- Code workaround or exception being considered: treating arbitrary source commands as catalog `read` would bypass the operation-effect invariant.
- Current thesis that resolves it, or proposed thesis revision: require schema-1 `effect: read` and defer mutation execution.
- Downstream impact: theses, product, architecture, security, release quality definition, YAML schema, plan JSON, catalog, tests, README, SECURITY, SUPPORT, and agent-readiness evidence.

## Reproduction or observation

```sh
go run ./cmd/atr plan preview --config examples/plan-preview.yaml -- gh pr list --state open
```

Observed before this work: schema-1 plan JSON, exit 0, and
`source_process_attempts: 0`; no execution command exists.

## Security and public-boundary notes

- Assets and side effects involved: one user-selected YAML read and one declared read-only local source process that may perform source-owned network reads.
- Credentials or confidential data involved: Atsura stores none; inherited environment and caller argv may still reach the source process and raw captures remain memory-only.
- New dependencies, destinations, files, processes, or generated content: no dependency or fixed network destination; one exact local process; synthetic public examples only.
- External schema provenance, publication rights, and drift evidence: no external schema; Atsura owns schema 1.
- Output delivery, collection coverage, pagination, timeout, retry, idempotency, and cancellation facts: complete/exhaustive for the exact captured source result, no pagination, 30 seconds, one attempt, read-safe declaration, kill on timeout/cancel/overflow, no automatic retry.
- Publication and licensing concerns: standard-library implementation and synthetic fixtures; no release is published.

## Glossary

- Direct source-process attempt: one successful `Start` call by Atsura; children created by the source CLI are source-owned behavior.
- Explicit trust selection: passing one exact YAML path to `--config` for the current invocation; it creates no persisted trust.
- Execution envelope: fixed Atsura JSON metadata plus dynamically configured transformed records.
