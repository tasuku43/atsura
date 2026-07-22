# ADR 0016: Add catalog-typed option defaults

- Status: Accepted
- Date: 2026-07-22
- Deciders: Repository maintainer and product owner
- Scope: Specification invocation vocabulary, wrapper plans, tailored help,
  runtime admission, and installed-artifact evidence
- Supersedes: None
- Superseded by: None

## Context

Atsura can expose or hide observed long options and can append fixed argv, but
it cannot yet express a value that applies only when the caller omits an
option. A maintainer who wants `gh pr list` to use a purpose-specific limit must
either require every caller to repeat it or append a fixed `--limit` that the
caller cannot cleanly override.

Treating fixed appended argv as a default would rely on undocumented duplicate-
option precedence in the source CLI. It would also make preview and tailored
help unable to explain whether caller input or configuration controlled the
effective invocation. The first default operation therefore needs its own
typed, deterministic contract.

The source catalog currently proves only an option's exact long name and
whether it takes a value. It does not prove numeric, enum, path, credential, or
other source-specific value semantics. The first operation must not invent
those types.

## Decision drivers

- An explicit caller value must win without relying on source duplicate-option
  precedence.
- Preview must explain both the declared default and whether it was applied.
- Tailored help must disclose the value that ordinary invocation may insert.
- Defaults may use only catalog and tailored-surface facts already reviewed in
  the exact bundle.
- Output-mode selection remains owned by typed output stages.
- Invocation remains no-shell, deterministic, bounded, and limited to one
  source attempt.
- The core contract remains independent from source vendors and coding-agent
  hosts.

## Considered options

### Treat `append_args` as defaults

This would place both the caller value and configured value in source argv and
delegate precedence to the source CLI. Different commands can use first-wins,
last-wins, merge, or rejection semantics, so the plan would not determine the
effective behavior.

### Append a configured value only after scanning caller argv

Conditional append avoids duplicates, but an existing caller `--` marker would
turn the configured option into positional data. Placing it after all caller
argv also makes the insertion point depend on positional grammar that the
catalog does not yet model completely.

### Insert a typed default immediately after the command path

The plan can first validate caller argv, detect exact option presence before
`--`, and insert one canonical long-option token only when absent. This keeps
the result independent from source duplicate precedence and gives preview a
complete explanation.

## Decision

Specification schema 5 adds an explicit `invoke.option_defaults` list. Each
entry has exactly `option` and `value` fields:

```yaml
invoke:
  option_defaults:
    - option: --limit
      value: "30"
  append_args:
    - --json=number,title,state
```

The list is explicit, bounded, ordered, and unique by option. Declaration order
is semantic and is preserved rather than sorted. A default option:

- is an exact catalog-observed long option on the matched command;
- has `takes_value: true`;
- is included in that command's tailored option surface;
- is not a cataloged structured-output selector;
- has one non-empty, structurally safe UTF-8 value whose canonical
  `--option=value` argv element is at most
  `sourceprocess.MaxArgumentBytes` (4096 bytes); and
- does not also occur by parsed active long-option name in `append_args`.

Append overlap recognizes both `--option=value` and separated `--option value`
forms before an exact `--` marker. Text after that marker is positional data
and does not overlap a default.

The combined number of default entries and appended arguments cannot exceed
the existing wrapper-argument bound. An identity wrapper has neither defaults
nor appended arguments. A transform wrapper may use defaults alone or combine
them with another admitted typed transform.

Plan construction validates the caller's option surface before applying a
default. Before the first caller `--`, `--option=value`, `--option value`,
`--option=`, and separated `--option` plus an explicit empty argv element all
count as explicit presence and suppress that option's default. Repeated valid
caller occurrences remain byte-for-byte and order-for-order exact and suppress
only the configured insertion; Atsura does not claim which repeated value the
source CLI selects. A short alias never suppresses a long-option default. An
invalid or value-less caller form remains invalid; presence does not repair it.
The same text after `--` is positional data and does not suppress a default.

The generic plan preserves a caller's explicit empty value. A finite source
adapter may reject that value before source start when its admitted runtime
grammar does not support it; it may not replace it with the configured default.

For every absent option, the plan inserts canonical `--option=value`
immediately after the matched command path. Applied defaults retain declaration
order, the remaining caller argv retains its exact order, and `append_args`
retains its existing exact order at the end. No environment expansion,
normalization, source-specific coercion, shell evaluation, or runtime discovery
occurs.

Plan schema 6 records the complete declared `option_defaults` list and an
explicit `applied_option_defaults` subset. An empty applied list beside a
non-empty declared list means valid caller input suppressed every default.
Detached plan validation re-derives the subset from original argv and
reconstructs the exact transformed argv; altered membership, order, or values
are invalid.

Generated-wrapper contract 3 includes each configured default in exact-command
tailored help. Value display is unambiguous and fixed in the rendered bytes.
The adoption summary exposes a distinct `OptionDefaultCount`, while exact
values remain reviewable in the canonical specification, bundle, plan, and
tailored help.

The schema changes are:

- specification 4 to 5;
- embedded bundle 3 to 4;
- wrapper plan 5 to 6;
- generated-wrapper contract 2 to 3; and
- installed-artifact evidence 7 to 8.

Source catalog 2, GitHub CLI contract 2, Go CLI contract 2, processor contracts,
the public command set, outer CLI result envelopes, agent-help schema 12, and
aggregate evidence schema 2 remain unchanged.

## Consequences

### Positive

- A purpose bundle can own one reviewable option default without preventing a
  caller override.
- Plan bytes explain exactly why a value was or was not inserted.
- The source process never receives duplicate configured/caller values for the
  same defaulted option.
- Static help tells an agent the ordinary behavior without starting Atsura, the
  source CLI, or a processor.
- The operation is reusable by any source adapter that can prove the same
  catalog arity and finite argv grammar.

### Negative

- Existing specifications, bundles, plans, and generated wrappers require
  explicit regeneration even when they want an empty default list.
- The first value type is only catalog arity plus an exact string. It does not
  validate numeric ranges, enums, paths, or semantic conflicts.
- Boolean flags, short options, root/global options, and positional defaults
  remain unsupported.
- Help and native evidence gain another compatibility surface and test matrix.

## Mechanical enforcement

- Strict schema tests require explicit default and append lists, preserved
  declaration order, uniqueness, catalog value arity, tailored visibility,
  non-selector ownership, the canonical-token byte bound, and no append
  overlap.
- Plan truth tables cover omission, inline and separated override, explicit
  empty override, repetition, malformed values, and the `--` boundary.
- Plan mutation tests reject changed applied subsets and transformed argv.
- Complete-surface admission rejects an invalid default on any entry before
  wrapper bytes or source/processor attempts.
- Contract-3 help tests bind exact default display into deterministic wrapper
  material.
- Installed-artifact evidence schema 8 records caller argv, source argv,
  declared defaults, and applied defaults for ordinary applied and overridden
  calls.
- Repository guards continue to reject arbitrary shell/script fields and
  coding-agent-host fields from production schemas.

## Security and public-boundary impact

Defaults are public, reviewable configuration. They are emitted in bundles,
plans, help, tests, and evidence and therefore are not a credential or secret
storage mechanism. Source authentication, authorization, and operation meaning
remain source-owned.

The configured value crosses only the existing plan-owned source-process
boundary as one exact argv element. The change adds no process, filesystem,
network, mutation, credential, host protocol, or third-party dependency. It
does not turn a tailored surface into an authorization policy or sandbox.

## Compatibility and migration

Specification schemas 1 through 4 and bundle schema 3 are retired for the new
binary. They fail before source execution with recovery that directs the user
to regenerate a schema-5 specification, rebuild, review and adopt the new
bundle, and render a contract-3 wrapper. Contract-2 wrapper invocation fails at
typed argument validation before bundle or source execution; current `wrapper
run` help exposes the only admitted contract, which is emitted by `wrapper
render`.

No persisted value is rewritten automatically. Trust is digest-bound, so a
rebuilt bundle requires fresh exact adoption. Rollback requires the matching
older binary, bundle, and wrapper contract; mixing generations is rejected.

## Validation

- focused domain, YAML, application, compatibility, help, renderer, CLI, and
  evidence tests with race detection where applicable
- `task check`
- `task security`
- `task public:check`
- `task release:check`
- exact five-target native artifact rows and aggregate evidence

## Reconsideration signals

Supersede this ADR before adding boolean or short-option defaults, semantic
value types, root/global options, positional arguments, conditional rules,
environment-derived values, or source-specific normalization. Do not emulate
those features with arbitrary appended strings or shell code.
