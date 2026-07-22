# Presentation Evidence: Tailored help discloses one option default

## Frozen semantic corpus

- Typed fixture path: planned `tools/sourcefixture/main.go` and schema-5
  composition in `tools/artifactjourney/journey.go`
- Presentation-independent answer key: schema-8 evidence validator
- Declared task and dimensions: invoke `pr list` with an omitted or explicit
  `--limit` under one adopted bundle
- Interpretation-relevant cases: default applied, inline/separated/empty and
  repeated override, short alias, `--` boundary, sibling command without the
  default
- Canonical references and exact next argv: no references; exact help selector
  is `pr list --help`

## Semantic eligibility

- [ ] Exact help names the caller-visible option and configured value.
- [ ] Help does not imply the default overrides explicit caller input.
- [ ] Root and sibling namespace help do not leak a default onto another command.
- [ ] Help succeeds while the bound runtime is non-executable and adds zero attempts.
- [ ] Routine use requires no source-help inspection or external reconstruction.

## Reproducible comparison

| Evidence | Before | After |
|---|---:|---:|
| Caller tokens for routine limit | 1 option + 1 value | 0 |
| Exact help calls to learn behavior | source-dependent | 1 tailored help call |
| Source attempts for help | 0 | 0 |
| Duplicate-precedence assumptions | 1 | 0 |

- Golden generator: native package plus installed-artifact journey for one exact revision
- Tokenizer: not used; semantic eligibility decides this capability
- Platform/runtime facts: fixed POSIX help on Linux/Darwin; Windows remains unsupported
- Invalidation rule: default grammar, bundle, help line, renderer, or evidence change

## Experiment outcome

- Outcome: pending implementation and exact native replay
- Eligible candidates: exact non-secret catalog-arity-typed defaults only
- Ineligible candidates: appended duplicates, hidden/selector options, shell or environment values
- Raw evidence retained at: bounded schema-8 rows after implementation
- Documented gates not implemented by the scorer: native artifact matrix

## Product compatibility decision

- Decision owner: Atsura product owner and maintainers
- Selected presentation: exact-command option line includes an unambiguous default value
- Compatibility rationale: ordinary behavior must be discoverable before execution
- Schema/version impact: wrapper contract 3 and evidence schema 8
- Rollout and rollback: rerender with the matching wrapper contract

## Security and execution boundary

- [ ] Fixture values are synthetic, public, and non-secret.
- [ ] Fixed help needs no runtime, source, processor, or network call.
- [ ] Values remain bounded structural data and are never shell-evaluated.
- [ ] Help makes no authorization, sandbox, or source-semantic claim.
