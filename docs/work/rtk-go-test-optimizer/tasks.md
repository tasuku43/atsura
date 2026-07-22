# Work Tasks: Optimize passing Go test output with inspected RTK

- Goal: [goal.md](goal.md)
- Plan: [plan.md](plan.md)

## Understand

- [x] Read governing theses, product, architecture, security, and harness.
- [x] Reproduce official RTK v0.43.0 behavior with pass, skip, failure,
      malformed, unknown-action, invalid-UTF-8, overflow, and real Go fixtures.
- [x] Record verified facts and unknowns in `context.md`.
- [x] Record repeated projection/optimizer/source/host confusion as thesis
      evidence.
- [x] Confirm the public outcome and non-goals in `goal.md`.

## Decide

- [x] Compare general RTK, internal formatter, and separate controlled processor
      approaches; select the strict pass-only tuple.
- [x] Identify schema-version, command/help, trust, result, artifact, and
      compatibility impact.
- [x] Classify processor inspection as utility/execute with no references.
- [x] Classify the optimizer/default as a public finite capability.
- [x] Identify source, processor, output, temporary-root, identity, and
      provenance assets and trust changes.
- [x] Decide complete bounded delivery, zero/one attempts, fail-closed
      cancellation, and non-retryability after source start.
- [x] Accept ADR 0012 for the durable trade-off.
- [x] Confirm no thesis exception is required: the slice implements the accepted
      original-preserving optimizer and external-host boundary.
- [x] Product owner previously approved RTK-preferred defaults and authorized
      implementation, commits, and main push.

## Implement

- [x] Add processor domain, observation codec, and compatibility tests.
      Evidence: commits `5ddc29f`, `59881f8`, and `d6b58c5`.
- [x] Implement processor inspection and fixed RTK v0.43.0 observation.
      Evidence: commits `5ddc29f` and `3fdadaf`.
- [x] Implement the finite processor compatibility registry.
      Evidence: commit `59881f8`.
- [x] Migrate specification, bundle, plan, result, trust, and schema contracts.
      Evidence: commits `abac36d`, `a190995`, and `240cf9a`.
- [x] Add source-catalog schema 2 single-dash selector evidence and exact Go
      contract-2 `test -json` admission without RTK policy in Go.
      Evidence: commit `abac36d`.
- [x] Implement the frozen LF/record/field/action/lifecycle Go pass validator,
      strict-size eligibility, and summary oracle.
      Evidence: commits `cea7f1c` and `d6b58c5`.
- [x] Implement isolated exact-identity processor execution and cleanup.
      Evidence: commit `5ddc29f`.
- [ ] Extend shared plan application and wrapper delivery; keep bundle execute
      projection-only with a zero-source optimizer rejection. Shared plan
      application and the two application facades are complete in commits
      `a36a0d0` and `cdb006d`; production CLI delivery remains in progress.
- [ ] Register CLI inputs/faults/help and update capability/schema/provenance
      ledgers.
- [x] Add controlled hostile-output, cancellation, drift, no-leak, valid
      `preserved_after_processor`, and processor-failure truth tables.
      Evidence: commits `5ddc29f`, `cea7f1c`, and `a36a0d0`. The controlled
      `preserved_after_processor` and processor one-attempt failure/no-byte
      branches are not official-artifact journey claims.
- [x] Add the frozen presentation fixture, independent answer key, and exact
      byte/hash comparison without an unsupported token claim. Evidence: commit
      `06e572c` and `presentation-evidence.md`.
- [x] Pin exact official RTK artifact and dependency provenance without
      redistributing RTK. Evidence: commit `5a1c9a0`.
- [ ] Add native installed-artifact journeys and semantic aggregate validation
      for only deterministic reachable official-RTK cases: optimized;
      `preserved_before_processor` skip/fail/ineligible; projection-facade
      rejection; preflight drift; eligible post-source drift; and Windows
      unsupported.
- [x] Remove unsupported child-process/filesystem/network absence claims from
      the evidence contract. External observer contracts remain an explicit
      unknown and must precede any future claim.
- [ ] Propagate durable conclusions through product, architecture, security,
      harness, release, and public-boundary docs.
- [ ] Commit each coherent concern after focused verification.

## Verify

- [x] Focused tests pass. Evidence: `go test ./internal/infra/gotestjson
      ./internal/app/planapply ./internal/infra/processorexec` on 2026-07-22.
- [ ] `task check` passes. Evidence:
- [ ] `task security` passes. Evidence:
- [ ] `task public:check` passes. Evidence:
- [ ] `task release:check` passes. Evidence:
- [ ] Runtime-only behavior is observed on four claimed optimizer platforms;
      Windows non-support is proved before source start. Evidence:
- [ ] Agent readiness meets the existing discovery-round-trip budget. Evidence:
- [ ] Routine successful wrapper execution requires zero undeclared external
      semantic processing; RTK is the declared adopted output processor.
      Evidence:
- [ ] Processor setup has a human-handoff scorecard with exact path, no secret,
      and no automatic installation claims. Evidence:
- [ ] Generated diff and repository status are understood. Evidence:

## Hand off

- [ ] Every acceptance criterion has evidence.
- [ ] Durable decisions are promoted out of this packet.
- [ ] Temporary binaries, roots, diagnostics, and sensitive artifacts are
      removed.
- [ ] Follow-up tuples and installation UX remain explicit nonblocking work.
- [ ] Delete this temporary packet from the final tree after completion.
- [ ] Handoff summary explains outcome, contract, checks, native evidence, and
      remaining risks.
