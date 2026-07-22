# Work Goal: Apply one reviewable option default only when caller input omits it

- Status: Active
- Retention: temporary
- Retention reason: None
- Governing contract: `docs/00_theses.md` Thesis 2, Thesis 3, Thesis 4, and Thesis 7
- Review/delete trigger: Delete after durable conclusions are promoted and the change completes
- Successor: None
- Owner: Atsura maintainers
- Target: Next release-quality implementation iteration
- Related ADRs: ADR 0005, ADR 0006, ADR 0014, ADR 0015, and ADR 0016

## Outcome

A maintainer can set a non-secret default for an included, catalog-observed
value-taking long option in YAML. Preview and static tailored help explain it.
The ordinary generated wrapper inserts it exactly once when the caller omits
the option and preserves a caller's explicit value unchanged.

## Why now

The current invocation vocabulary can append fixed argv but cannot distinguish
a default from an unconditional argument. Requiring callers to repeat a
purpose-specific value weakens the tailored experience; appending the value
would rely on source-specific duplicate-option precedence. A catalog-arity-
typed default is the smallest deterministic argv operation that closes this
gap without implementing general replacement or positional grammar.

## Non-goals

- Boolean flags, short options, root/global options, or positional defaults
- Numeric, enum, path, credential, or other source-specific value semantics
- Argument deletion, replacement, conditional expressions, or cross-option rules
- Environment/config interpolation, arbitrary shell, script, jq, plugin, or LLM
- RTK changes, source discovery changes, host adapters, hooks, or persistent shims
- A published release or Windows POSIX wrapper activation

## Acceptance criteria

- [ ] Schema-5 YAML and bundle schema 4 represent explicit bounded typed
      defaults; missing, null, non-string, empty, hidden, valueless, selector,
      duplicate, overlapping, structurally unsafe, or legacy content fails
      closed with exact recovery.
- [ ] Plan schema 6 records declared and applied default lists, and rebuilds
      exact transformed argv for omission, inline/separated/empty override,
      repetition, short aliases, malformed input, and `--` cases without source
      execution.
- [ ] Contract-3 exact-command tailored help discloses the configured value;
      root and namespace membership remain unchanged and help starts zero
      runtime, source, and processor attempts.
- [ ] One complete GitHub bundle and generated `gh` wrapper apply `--limit=30`
      to ordinary `pr list`, preserve an explicit caller limit, retain the
      sibling append-only command, and use one source attempt per ordinary call.
- [ ] Any invalid default on any surface entry rejects the whole render before
      material, adoption changes, source attempts, or processor attempts.
- [ ] Existing Go identity/optimizer behavior, output authority, source
      authentication/authorization meaning, and vendor/host independence remain
      unchanged.
- [ ] Evidence schema 8 passes four POSIX ordinary cases, the Windows structured
      unsupported result, all five native rows, and aggregate schema 2 for one
      exact revision.
- [ ] `task check`, `task security`, `task public:check`, and
      `task release:check` pass without weakening a check.

## Governing documents

- Thesis: deterministic, explainable typed argv changes in an adopted bundle
- Product contract: tailoring specification, wrapper plan, tailored help, and
  transformed invocation
- Architecture or security invariant: pure planning before one controlled
  no-shell source-process boundary; configuration is public and non-secret
- Existing ADR: ADR 0016 fixes precedence, insertion, disclosure, and migration

## Completion definition

The work is complete when every acceptance criterion has exact tests and native
artifact evidence, durable contracts describe the finite default vocabulary,
all required profiles pass, and this temporary packet is removed.
