# Work Tasks: Ordinary tailored help

- Goal: [goal.md](goal.md)
- Plan: [plan.md](plan.md)

## Understand

- [x] Read governing theses, product, architecture, and security sections.
- [x] Reproduce or observe current behavior.
- [x] Record verified facts and unknowns in `context.md`.
- [x] Record repeated decisions, friction, and potential thesis workarounds.
- [x] Confirm the public outcome and non-goals in `goal.md`.

## Decide

- [x] Compare credible approaches and record the selected design.
- [x] Resolve the typed semantic help model and explicit bounds.
- [x] Identify exact public-contract and compatibility version impact.
- [x] Classify the capability as the existing public wrapper utility.
- [x] Confirm there is no new effect, target, authentication, or host boundary.
- [x] Add and accept durable ADR 0014 before completing the mechanism.

## Implement

- [x] Add domain and renderer contract tests that failed before the mechanism.
- [x] Implement domain-owned help invariants and binding.
- [x] Implement fixed bounded POSIX help comparisons.
- [x] Update CLI catalog/help and the existing capability contract.
- [x] Add hostile-text, hidden/unknown, forwarding, and zero-attempt tests.
- [x] Prove caller-defined `command`, `return`, `test`, and `printf` functions
      or aliases cannot intercept tailored-help dispatch/termination and
      remain unchanged outside the generated function's subshell.
- [x] Prove an existing same-name alias is removed before parsing and selecting
      the generated ordinary function.
- [x] Add the schema-6 installed-artifact/native-platform evidence harness.
- [ ] Run and aggregate the native installed-artifact evidence.
- [x] Update durable documentation and harness contracts.
- [x] Complete `presentation-evidence.md` from the frozen code fixtures and
      parameterized artifact answer key.

## Verify

- [x] Focused tests pass. Evidence: `go test ./internal/domain/tailoringbundle
      ./internal/domain/wrapperbinding ./internal/infra/posixwrapper
      ./internal/app/wrapperrender ./internal/cli ./tools/artifactjourney
      ./tools/artifactevidence` passed on 2026-07-22.
- [x] `task check` passes. Evidence: local full gate passed on 2026-07-22.
- [x] `task security` passes. Evidence: local security gate passed on
      2026-07-22.
- [x] `task public:check` passes. Evidence: local public-boundary gate passed on
      2026-07-22.
- [x] `task release:check` passes. Evidence: local release-contract gate passed
      on 2026-07-22, including its current-host archive replay.
- [ ] Runtime-only behavior was observed on supported POSIX targets. Evidence:
- [x] One ordinary help invocation discovers the complete finite command-path
      surface with zero external processing. Evidence: schema-6 replay of the
      packaged `darwin/arm64` archive for revision
      `1232913ba6d8458f3cdd9dde872f8d11b70a5228` recorded the root view with
      zero source and processor attempts while the packaged `atr` was not
      executable.
- [x] Generated diff and repository status are understood. Evidence: scoped
      `git status --short`, implementation diff review, and packet whitespace
      check on 2026-07-22; no unrelated path was edited by this packet task.

## Hand off

- [ ] Acceptance criteria have evidence.
- [ ] Goal and plan status are complete only after all criteria pass.
- [ ] Durable decisions are promoted out of this packet.
- [ ] Temporary diagnostics and sensitive artifacts are removed.
- [ ] Follow-up work is explicit and does not block this goal.
- [ ] Temporary packet is removed after native evidence passes.
