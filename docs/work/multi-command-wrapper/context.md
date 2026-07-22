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
- `githubcli.VerifySurface` rejects `len(bundle.Surface) != 1` before validating
  the single entry. This is the active blocker to one multi-command wrapper.
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

## Unknowns

- [ ] Whether evidence should add one dedicated `multi_command_wrapper` object
      or evolve the existing wrapper-case inventory without obscuring the fact
      that two calls share one exact bundle and rendered digest.
- [ ] Whether mixed admitted result modes need an additional bundle-level
      invariant beyond independent entry validation. Resolve through domain and
      runtime truth tables before changing production code.

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

The blocker is deterministic and requires no source process:

```sh
go test ./internal/infra/githubcli -run TestVerifySurfaceRejectsMixedAndPartialSurfaces
```

The current mixed-command fixture is expected to be rejected solely because
the surface contains two otherwise admitted entries.

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
