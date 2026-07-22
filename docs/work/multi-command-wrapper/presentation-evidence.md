# Presentation Evidence: One wrapper serves one complete multi-command surface

## Frozen semantic corpus

- Typed fixture path: `tools/sourcefixture/main.go` and the typed specification
  composition in `tools/artifactjourney/journey.go`
- Fixture SHA-256: bound through the exact inspected source identity, catalog
  digest, and compiled bundle digest for each native replay
- Presentation-independent answer-key path:
  `tools/artifactevidence/evidence.go`
- Answer-key SHA-256: parameterized through exact bundle, plan, rendered-source,
  stdout, and stderr digests
- Declared task and dimensions: discover one two-command included surface, then
  select either exact command and its independent option/result contract
- Interpretation-relevant cases: two sibling namespaces, two exact paths,
  different wrapper result modes, hidden cataloged path, unknown path
- Canonical references and exact next argv: no references; root help returns
  exact paths `issue list` and `pr list`

## Semantic eligibility

- [x] One root invocation returns both complete exact paths.
- [x] One exact help invocation returns only that command's option surface and
      wrapper reason.
- [x] Routine success requires zero joins, source inspection, exploratory calls,
      or provider-notation interpretation.
- [x] No canonical references, collection scope, pagination, or uncertainty are
      introduced.
- [x] Same-name, order, adjacency, and indentation do not imply that one
      command's wrapper behavior applies to the other.
- [x] Hidden and unknown recovery obey the executable argv grammar with zero
      source and processor attempts.

## Reproducible comparison

| Evidence | Before | After |
|---|---:|---:|
| Ordinary wrappers needed | 2 | 1 |
| Root help calls needed to discover both paths | 2 | 1 |
| Source attempts for discovery | 0 | 0 |
| External reconstruction steps | at least 1 merge | 0 |

- Golden generator or command: `scripts/package-release.sh` followed by
  `scripts/test-release-artifact.sh` on the same exact native revision
- Tokenizer: not used; semantic eligibility decides this capability
- Platform/runtime facts: POSIX wrapper on Linux and Darwin; Windows remains
  explicitly unsupported
- Invalidation rule: any bundle fixture, help projection, wrapper material, or
  evidence-schema change invalidates the recorded digests

## Experiment outcome

- Outcome: eligible on exact local Darwin/arm64 revision
  `a79a637d3067c86c72e77862ad06382f679d9d5c`; the required five-target
  schema-7 workflow remains pending
- Eligible candidates: one all-entry-admitted wrapper
- Failed or invalidated candidates: several same-name functions are ineligible
  because sourcing order would silently select only one surface
- Raw evidence retained at: digests and counters above; temporary artifact and
  raw native row were deleted after validation
- Documented gates not implemented by the scorer: native artifact matrix

## Product compatibility decision

- Decision owner: Atsura product owner and maintainers
- Selected presentation: one complete same-source bundle rendered as one
  ordinary command, as accepted in ADR 0015
- Compatibility rationale: one purpose bundle should appear as one ordinary CLI
- Schema/version impact: product schemas and wrapper contract remain unchanged;
  installed evidence advances from schema 6 to 7 for exact `caller_argv`
- Rollout and rollback: render the same bundle with a supporting runtime; use
  one-command bundles to roll back
- Relationship to experiment outcome: exact artifact behavior determines
  eligibility; byte counts are secondary

## Security and execution boundary

- [x] Fixtures and evidence are synthetic and public-safe.
- [x] No new artifact path or subprocess is needed for help.
- [x] Each ordinary execution retains the existing exact executable, argv,
      identity, timeout, output bounds, and one-attempt ceiling.
- [x] Network and live-model evaluation are unnecessary for deterministic gates.
