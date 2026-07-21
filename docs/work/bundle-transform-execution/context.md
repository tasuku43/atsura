# Work Context: Execute one proven JSON-transform wrapper

## Current behavior

- `bundle preview` strictly loads an adopted/current schema-2 bundle, calls
  `tailoringplan.Build`, and returns a canonical plan digest with zero process
  attempts.
- `tailoring.TransformJSON` and `sourcejson.Parser` already provide bounded,
  duplicate-aware typed JSON selection and rename semantics.
- `sourceexec.Runner.Run` fingerprints an executable twice before start and
  once after wait, but uses its first observation as authority rather than the
  bundle identity. Replacement between application identity observation and
  runner observation can therefore start different bytes.
- GitHub CLI inspector contract 1 uses only `version` and `help reference`.
  It records `--json` with an empty field list, so schema-3 output selection
  cannot be validated for an inspected GitHub catalog.
- Local GitHub CLI 2.72.0 command help for `issue list` and `pr list` publishes
  accepted JSON fields. Official `gh help formatting` documents comma-separated
  `--json` fields and omission of the value as field discovery.

## Relevant structure

- Entry point: `internal/cli/bundle.go` and catalog-derived routing
- Domain rule: `internal/domain/tailoringplan`, `tailoring`, `sourceprocess`
- Application use case: new `internal/app/bundleexecute`; preview remains
  independent public behavior over shared plan resolution
- Infrastructure boundary: `sourceexec`, `sourcejson`, source runtime verifier
- CLI catalog or presentation: `internal/cli/catalog.go`, nested output schemas
- Existing tests and harness checks: plan/transform/parser/process contracts,
  catalog lints, output schema contracts, agent readiness scenarios

## Constraints

- Surface composition is not authorization; execution carries no source
  allow/confirm/deny, inferred source effect, target, or impact.
- Preview documents are explanations, never runtime authority.
- Source attempts are zero or one. Only proven zero-attempt transient failures
  may be retryable; every post-start outcome is non-retryable.
- No raw source bytes enter bundle, plan, trust state, public faults, or files.
- Identity and raw wrapper output need separate public presentation contracts
  and are not smuggled into this transform slice.
- Current plan process framing is closed stdin with inherited cwd/environment;
  modes, not ambient values, must become explicit plan facts.
- Shared runtime semantics remain vendor-neutral. Adapter kind/version chooses
  a bounded infrastructure proof rather than introducing vendor fields into
  the bundle or plan.

## External facts

- GitHub CLI manual, `gh help formatting`, https://cli.github.com/manual/gh_help_formatting,
  checked 2026-07-21: `--json` requires comma-separated field names and omitting
  the value reports possible fields.
- GitHub CLI manual, `gh pr list`, https://cli.github.com/manual/gh_pr_list,
  checked 2026-07-21: the command supports `--json <fields>` and publishes a
  finite JSON-field inventory.
- Local `/opt/homebrew/bin/gh` 2.72.0, observed 2026-07-21: `issue list --help`
  and `pr list --help` emit finite `JSON FIELDS` sections without provider I/O.

## Unknowns

- [ ] The portable check-to-exec race cannot be closed completely without a
  platform-specific execute-open mechanism; retain it as a known limitation.
- [ ] Identity wrapper and successful source stderr presentation require later
  evidence; this slice rejects both before execution.
- [ ] Runtime support for further GitHub commands or other source adapters
  requires their own maintained offline evidence and runtime verifier.

## Thesis evidence

- Repeated design decision or point of agent confusion: a cataloged selector
  flag was repeatedly mistaken for proof of its value encoding.
- User outcome or friction observed in the minimal slice: preview explains the
  intended RTK-like reshape but cannot yet demonstrate the result.
- Code workaround or exception being considered: executing `plan.Stages.Invoke`
  directly or treating an empty field inventory as wildcard.
- Current thesis that resolves it: Thesis 4 requires fresh complete planning;
  Thesis 6 prefers adapter-verified structured output and no raw fallback.
- Downstream impact: plan schema, bound process contract, application runtime,
  source adapter fixture, catalog/help/output tests, docs 00 through 09.

## Security and public-boundary notes

- Assets and side effects involved: one untrusted local source process and its
  source-owned downstream behavior.
- Credentials or confidential data involved: inherited source environment may
  contain credentials; values are never captured or persisted.
- New dependencies, destinations, files, processes, or generated content: no
  dependency or destination; one exact local process; bounded in-memory output.
- Output delivery: complete JSON document, not paged; coverage is not
  applicable because Atsura transforms the source result it receives.
- Timeout/retry/cancellation: 30 seconds, one attempt, no post-start retry,
  post-start cancellation is non-retryable.
- Publication: synthetic fixtures only; no live provider result is committed.

## Glossary

- Runtime proof: adapter-owned validation that one plan's exact selector value
  requests the declared structured format and fields on stdout.
- Bound request: process request plus exact bundle-derived executable identity.
- Post-start: any state after the process port reports or may have made one
  source attempt.
