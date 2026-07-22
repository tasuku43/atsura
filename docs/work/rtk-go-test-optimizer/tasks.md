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

- [ ] Add failing processor domain, observation codec, and compatibility tests.
- [ ] Implement processor inspection and fixed RTK v0.43.0 observation.
- [ ] Implement the finite processor compatibility registry.
- [ ] Migrate specification, bundle, plan, result, trust, and schema contracts.
- [ ] Add source-catalog schema 2 single-dash selector evidence and exact Go
      contract-2 `test -json` admission without RTK policy in Go.
- [ ] Implement the frozen LF/record/field/action/lifecycle Go pass validator,
      strict-size eligibility, and summary oracle.
- [ ] Implement isolated exact-identity processor execution and cleanup.
- [ ] Extend shared plan application and wrapper delivery; keep bundle execute
      projection-only with a zero-source optimizer rejection.
- [ ] Register CLI inputs/faults/help and update capability/schema/provenance
      ledgers.
- [ ] Add hostile output, cancellation, drift, no-leak, and delivery tests.
- [ ] Add frozen presentation fixture, answer key, and before/after evidence.
- [ ] Add native installed-artifact journeys and semantic aggregate validation.
- [ ] Propagate durable conclusions through product, architecture, security,
      harness, release, and public-boundary docs.
- [ ] Commit each coherent concern after focused verification.

## Verify

- [ ] Focused tests pass. Evidence:
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
