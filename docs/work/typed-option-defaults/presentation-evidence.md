# Presentation Evidence: Tailored help discloses one option default

## Frozen semantic corpus

- Typed fixture path: `tools/sourcefixture/main.go` and schema-5
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

- [x] Exact help names the caller-visible option and configured value.
- [x] Help does not imply the default overrides explicit caller input.
- [x] Root and sibling namespace help do not leak a default onto another command.
- [x] Help succeeds while the bound runtime is non-executable and adds zero attempts.
- [x] Routine use requires no source-help inspection or external reconstruction.

## Reproducible comparison

| Evidence | Before | After |
|---|---:|---:|
| Caller tokens for routine limit | 1 option + 1 value | 0 |
| Exact help calls to learn behavior | source-dependent | 1 tailored help call |
| Source attempts for help | 0 | 0 |
| Duplicate-precedence assumptions | 1 | 0 |

- Golden generator: native package plus installed-artifact journey for one exact revision
- Tokenizer: not used; semantic eligibility decides this capability
- Platform/runtime facts: local Darwin/arm64 installed-artifact replay passed;
  fixed POSIX help targets Linux/Darwin, Windows remains structured unsupported,
  and the exact five-target replay is pending
- Invalidation rule: default grammar, bundle, help line, renderer, or evidence change

## Experiment outcome

- Outcome: implementation and local Darwin/arm64 installed-artifact replay
  passed; exact five-target native replay remains pending
- Eligible candidates: exact non-secret catalog-arity-typed defaults only
- Ineligible candidates: appended duplicates, hidden/selector options, shell or environment values
- Raw evidence retained at: none for the local temporary row; the strict
  schema-8 generator and validator remain, and exact native rows remain CI work
- Documented gates not implemented by the scorer: native artifact matrix

## Product compatibility decision

- Decision owner: Atsura product owner and maintainers
- Selected presentation: exact-command option line includes an unambiguous default value
- Compatibility rationale: ordinary behavior must be discoverable before execution
- Schema/version impact: wrapper contract 3 and evidence schema 8
- Rollout and rollback: rerender with the matching wrapper contract

## Security and execution boundary

- [x] Fixture values are synthetic, public, and non-secret.
- [x] Fixed help needs no runtime, source, processor, or network call.
- [x] Values remain bounded structural data and are never shell-evaluated.
- [x] Help makes no authorization, sandbox, or source-semantic claim.
