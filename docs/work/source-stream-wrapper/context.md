# Work Context: Execute identity and argv-only ordinary wrappers

This file records verified facts and unresolved questions. Desired behavior is
kept in `goal.md` and `plan.md` until it is implemented.

## Current behavior

- `tailoringbundle` and schema 3 can represent an identity wrapper and a
  transform whose only action is `invoke.append_args`.
- `tailoringplan` schema 3 constructs complete identity and argv-only plans,
  but does not name the plan result mode.
- `planapply.Apply` rejects a plan without a typed JSON output stage, so the
  generated ordinary wrapper cannot yet execute identity or argv-only entries.
- `wrapper render` admits only the complete GitHub JSON-transform surface
  proven by the current `githubcli` compatibility adapter.
- `sourceprocess` already provides no-shell, closed-stdin, inherited cwd/env,
  one-attempt, timeout, output-bound, and executable-identity checks.
- `sourceprocess.Result` can carry bounded stdout, stderr, and a conventional
  nonzero exit code, but the application currently classifies every returned
  process error as an Atsura fault.
- `sourceprocess.Request.Validate` currently rejects empty argv elements even
  though argv transport and generated POSIX forwarding otherwise preserve them.
- The released wrapper binding schema and POSIX function need not encode a
  result mode because `wrapper run` rebuilds the authoritative fresh plan.
- Windows wrapper rendering is intentionally unsupported; Linux and macOS are
  the current ordinary-function targets.

## Relevant structure

- Entry point: `atr wrapper render` and the generated function invoking
  `atr wrapper run`
- Domain rule: `internal/domain/tailoringplan`,
  `internal/domain/sourceprocess`, and `internal/domain/tailoringbundle`
- Application use case: shared `internal/app/planapply`, used by
  `internal/app/wrapperrun`
- Infrastructure boundary: `internal/infra/githubcli`,
  `internal/infra/sourceexec`, and `internal/infra/posixwrapper`
- CLI catalog or presentation: `internal/cli/catalog.go`, wrapper handlers,
  help schema, and trust summaries
- Existing tests and harness checks: focused package tests, CLI contract tests,
  `tools/sourcefixture`, `tools/artifactjourney`, artifact evidence, and the
  five-target GitHub Actions matrix

## Constraints

- The path remains adopted-bundle execution; it cannot skip surface, option,
  source identity, runtime identity, or fresh-plan checks.
- Atsura must not infer source authorization or convert source status into a
  coding-agent permission decision.
- Source bytes are untrusted and may be non-UTF-8 or terminal-active. Exact
  visibility must be an explicit plan result mode, never a fallback.
- Captured bytes must not be persisted, copied into structured faults, or
  exposed after an uncertain process outcome.
- Final stdout/stderr delivery cannot be atomic across two writers. Any write
  error may leave partial caller-visible output and cannot safely recommend replay.
- No new third-party dependency or external network destination is required.
- Existing transformed-JSON behavior and its strict no-raw-fallback contract
  must remain unchanged.

## External facts

No external schema or provider behavior is required for this core slice. ADR
0009 records why RTK integration waits for this original-output authority.

## Unknowns

- [ ] Whether a later public API should offer streaming rather than bounded
      final delivery; this slice deliberately does not decide it.
- [ ] Whether a future platform can preserve signal semantics safely; this
      slice treats non-conventional termination as uncertain.
- [ ] Whether source-specific adapters beyond the existing finite GitHub CLI
      contract can share a generic surface-proof vocabulary.

## Thesis evidence

- Repeated design decision or point of agent confusion: coding-agent hooks and
  input rewriting were repeatedly mistaken for Atsura responsibilities.
- User outcome or friction observed in the minimal slice: an ordinary command
  wrapper exists, but it works only when Atsura also transforms JSON output.
- Code workaround or exception being considered: a second raw executor would
  bypass the reviewed plan and duplicate process authority.
- Current thesis that resolves it, or proposed thesis revision: the plan must
  declare source-stream visibility explicitly, and the shared plan application
  boundary remains authoritative.
- Downstream impact: theses, product, architecture, security, AGENTS invariant
  12, add-capability Skill, catalog/help, trust summary, plan schema, runtime,
  fixtures, artifact evidence, and harness documentation.

## Reproduction or observation

```sh
go test ./internal/domain/tailoringplan ./internal/app/planapply \
  ./internal/app/wrapperrender ./internal/app/wrapperrun
```

Current focused tests pass, but their success contract covers only the typed
JSON transform runtime.

## Security and public-boundary notes

- Assets and side effects involved: one exact local source executable and its
  bounded stdout/stderr; no new files or network endpoints.
- Credentials or confidential data involved: source output may contain
  secrets; Atsura does not inspect, persist, quote, or embed it in faults.
- New dependencies, destinations, files, processes, or generated content: no
  dependency; the existing source process boundary remains the only process.
- External schema provenance, publication rights, and drift evidence: not applicable.
- Output delivery and retry facts: complete buffered delivery within fixed
  bounds; no pagination; 30-second timeout; no retry; conventional nonzero
  status is returned only after complete final writes.
- Publication and licensing concerns: none beyond existing repository policy.

## Glossary

- `source_stream_passthrough`: a reviewed plan result mode that returns bounded
  source bytes and conventional status after all tailoring checks. It is not a
  raw or policy-bypassing execution route.
- `conventional completion`: one started process that produced a non-negative
  operating-system exit status and retained the expected executable identity.
- `final write`: Atsura's post-process delivery of complete buffered stdout or
  stderr to the caller-owned output boundary.
