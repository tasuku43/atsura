# Work Context: One wrapper serves one complete multi-command surface

## Current behavior

- `tailoringbundle.Bundle.Surface` is a bounded ordered list and can contain
  several included commands from one exact source catalog.
- `wrapperbinding.CompileHelp` already derives root, namespace, exact, and
  combined views from every included surface entry.
- `tailoringplan.Build` resolves one longest catalog command for each attempted
  invocation and then selects only that exact surface entry's wrapper.
- `githubcli.VerifyRuntime` already admits both `issue list` and `pr list` under
  the maintained GitHub CLI contract 2 grammar.
- `githubcli.VerifySurface` now rejects an empty surface and independently
  validates every canonical entry before wrapper material can be rendered.
- Installed-artifact evidence schema 7 records exact caller argv and proves the
  first two ordinary cases share one bundle, rendered source, and actual sourced
  file while retaining distinct preview-derived plans and result modes.
- Go CLI admission intentionally remains a one-command `test` surface and is
  outside this GitHub-specific compatibility slice.

## Relevant structure

- Entry point: `atr wrapper render` and generated ordinary `gh`
- Domain rule: `tailoringbundle.Bundle`, `tailoringplan.Build`, and
  `wrapperbinding.CompileHelp`
- Application use case: `internal/app/wrapperrender` plus
  `internal/app/runtimecompat`
- Infrastructure boundary: `internal/infra/githubcli.RuntimeVerifier` and
  `internal/infra/posixwrapper`
- CLI catalog or presentation: existing `wrapper render`, `wrapper run`, and
  compiled tailored help contracts
- Existing tests and harness checks: GitHub runtime truth tables, wrapper
  render/run tests, artifact journey, strict evidence aggregation, and CI

## Constraints

- The bundle remains one source identity and one adopted purpose; commands do
  not select different source executables or processors dynamically.
- Admission must cover the complete included surface before rendering. It may
  not validate only the first command or defer a known unsupported entry until
  an agent invokes it.
- Each invocation still produces one fresh plan from the exact adopted bundle
  and current source identity. The rendered function carries no routing policy
  beyond compiled help and unchanged argv forwarding.
- The implementation adds no I/O, credential, host, network, or mutation
  boundary and no third-party dependency.
- Repository documentation remains English and fixtures remain synthetic.

## External facts

None. This iteration changes only repository-owned compatibility and evidence
contracts. It does not rely on a new claim about GitHub CLI or another tool.

## Resolved unknowns

- [x] Evidence evolves the existing ordered `wrapper_cases` inventory rather
      than adding a parallel object. Schema 7 adds exact `caller_argv`; the first
      two cases must share bundle and wrapper identities and use distinct plans.
- [x] Mixed admitted result modes need no new bundle schema invariant. Complete-
      surface admission validates every entry, and each fresh invocation plan
      remains the exclusive authority for its selected command.

## Thesis evidence

- Repeated design decision or point of agent confusion: the product describes
  a purpose-specific CLI surface while installed evidence uses one command per
  wrapper artifact.
- User outcome or friction observed in the minimal slice: a user must source
  separate `gh` functions to tailor `issue list` and `pr list`, which is not a
  coherent ordinary CLI experience.
- Code workaround or exception being considered: generating several wrappers
  from one source command would create competing ordinary spellings and evade
  complete-surface validation.
- Current thesis that resolves it: one bundle and one ordinary command expose
  one bounded surface; every included entry has independent complete behavior.
- Downstream impact: product, architecture, security, ADR, GitHub compatibility
  tests, wrapper integration, artifact journey, evidence validation, and
  release/readiness documentation.

## Reproduction or observation

Focused race tests and the exact installed-artifact replay pass:

```sh
go test -race -count=1 ./internal/app/wrapperrender \
  ./internal/domain/tailoringbundle ./internal/domain/tailoringplan \
  ./internal/infra/githubcli ./tools/artifactjourney \
  ./tools/artifactevidence ./tools/sourcefixture

bash scripts/package-release.sh v0.0.0-rc.1 \
  a79a637d3067c86c72e77862ad06382f679d9d5c darwin arm64 <temp-dir>
bash scripts/test-release-artifact.sh v0.0.0-rc.1 \
  a79a637d3067c86c72e77862ad06382f679d9d5c darwin arm64 <archive>
```

The native Darwin/arm64 row emitted schema 7 with shared bundle digest
`c07e18d653ad53e4897371a9cce0177ecb504c6575c827459cd5c4e1a85e8602`,
shared rendered-source digest
`a11ecb790aa747d2e729234012e4b25fdd76d6599b65d8106c2b464c98419308`,
distinct PR/issue plan digests, three ordinary source attempts, 13 fixture
attempts, and zero source or processor attempts for all five help views and two
fallthrough faults. The temporary archive and raw row were removed after the
bounded replay. Cross-platform workflow evidence remains pending.

## Security and public-boundary notes

- Assets and side effects involved: validated in-memory bundle and generated
  wrapper bytes; ordinary execution retains the existing one-process boundary.
- Credentials or confidential data involved: none in fixtures or evidence.
- New dependencies, destinations, files, processes, or generated content: no
  dependency or destination; only a larger bounded generated help projection.
- External schema provenance, publication rights, and drift evidence: existing
  GitHub CLI contract 2 synthetic fixtures only.
- Output delivery, collection coverage, pagination, timeout, retry,
  idempotency, and cancellation facts: unchanged per selected command and
  result mode; no cross-command aggregation occurs.
- Publication and licensing concerns: none beyond the existing release model.

## Glossary

- **Multi-command surface**: two or more included exact command paths from one
  source catalog, one purpose bundle, and one ordinary command spelling.
- **All-entry admission**: pure validation of every included surface entry
  before a wrapper artifact may be produced.
