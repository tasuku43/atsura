# Work Context: Host-neutral wrapper entry point

This file records verified facts and unresolved questions. Desired behavior is
not current behavior until tests prove it.

## Current behavior

- `atr bundle preview` strictly loads an adopted bundle, revalidates current
  source path/hash/size, and returns a canonical fresh plan with zero source
  attempts.
- `atr bundle execute` repeats the same authority checks, admits one finite
  GitHub CLI JSON contract, starts the exact physical source at most once
  without a shell, and returns a schema-2 maintainer evidence envelope.
- There is no public wrapper entry point or generated ordinary-command wrapper.
- Production `cmd/` and `internal/` contain no Claude Code, Codex, hook,
  settings, or host-permission implementation.
- The discarded host-specific direction changed only documentation, the Skill,
  and the capability ledger; no production Go implementation was discarded.

## Relevant structure

- Entry point: `cmd/atr` composes `internal/cli`.
- Domain rule: `internal/domain/tailoringplan` builds the complete plan and
  binds source execution to an absolute resolved path.
- Application use case: `internal/app/bundleexecute` owns adoption, drift,
  fresh planning, compatibility admission, one process attempt, and typed
  transformation.
- Infrastructure boundary: `internal/infra/sourceexec` invokes the plan-bound
  path and argv directly without ambient PATH lookup or a shell.
- CLI catalog or presentation: `internal/cli/catalog.go` and
  `internal/cli/bundle.go` expose the current maintainer envelope.
- Existing tests and harness checks: direct plan/runtime/recovery coverage plus
  installed-artifact replay in `tools/artifactjourney`.

## Constraints

- The coding-agent host is outside the product boundary. The generated wrapper
  accepts argv, not a host event or shell command string.
- The source executable remains untrusted and source-owned; wrapper execution
  retains exact identity, no-shell, finite time/bytes, and zero/one attempts.
- A generated shell function may use only a fixed template and exact quoted
  product bindings. The YAML specification cannot contain shell source.
- External activation can fail or be bypassed. Atsura makes no fail-closed claim
  until the wrapper is actually selected.
- The direct gateway remains available for evidence and recovery. Wrapper
  success must not emit its maintainer metadata envelope.
- Windows is not a claimed wrapper-function platform in this slice. Existing
  portable core and release checks remain green unless a separate platform
  decision changes them.

## External facts

- Bounded Claude Code `2.1.217` and Codex hook probes showed that hook rewrite,
  permission, trust, settings, and failure semantics are vendor-specific. They
  are evidence for excluding those protocols from Atsura, not positive wrapper
  compatibility evidence.
- The previous Claude plugin PATH probe appended its `bin/` directory after the
  existing PATH. That rejects that exact activation candidate; it does not
  establish the POSIX function contract in this goal.
- These probes create no Atsura compatibility obligation. Any positive vendor
  activation evidence belongs to a downstream integration.

## Unknowns

- [ ] Which exact public command names and input spellings best express render
      versus run without promising installation?
- [ ] How should catalog output metadata represent plan-authoritative dynamic
      wrapper output instead of a catalog-static envelope?
- [ ] Which fixed POSIX quoting algorithm and command-name grammar are both
      portable and mechanically round-trippable?
- [ ] Which existing executable-identity components should the renderer use to
      bind the currently running absolute `atr` path/hash/size?
- [ ] Which installed-artifact journey proves function activation without
      treating the generated function as arbitrary trusted shell?
- [ ] What evidence should select a later executable shim or multi-profile
      materialization lifecycle?

## Thesis evidence

- Repeated design decision or point of agent confusion: treating agent-host
  request rewrite as the wrapper boundary repeatedly pulled vendor permission,
  settings, and shell grammar into product design.
- User outcome or friction observed in the minimal slice: the user expects the
  agent to call an already rewritten function under the ordinary command name.
- Code workaround or exception being considered: emitting the current
  `bundle execute` evidence envelope from a wrapper would preserve plumbing but
  fail the tailored CLI output experience.
- Current thesis that resolves it: ADR 0008 and Thesis 7 put agent hosts outside
  Atsura and make the host-neutral wrapper the product artifact.
- Downstream impact: product output authority, CLI catalog, generic wrapper
  entry point, fixed shell rendering, harness boundary guards, and one generic
  caller-owned activation fixture.

## Reproduction or observation

```sh
task check:fast
go test ./internal/domain/tailoringplan ./internal/app/bundleexecute \
  ./internal/app/planpreview ./internal/infra/sourceexec ./internal/cli
```

The focused runtime packages passed before implementation began. Exact command
output and final gate evidence belong in `tasks.md`.

## Security and public-boundary notes

- Assets and side effects involved: adopted bundle receipt, generated wrapper
  text, exact source process, and tailored stdout.
- Credentials or confidential data involved: inherited source environment may
  contain credentials; Atsura does not persist it or add credential material.
- New dependencies, destinations, files, processes, or generated content: no
  dependency or network destination is planned; generated POSIX function text
  is fixed-template output.
- Publication concerns: no vendor configuration or transcript enters the
  repository; generic fixture evidence remains bounded and synthetic.

## Glossary

- **wrapper entry point**: host-neutral Atsura argv boundary used by generated
  wrappers.
- **wrapper function**: fixed generated POSIX function that forwards `"$@"` to
  the wrapper entry point.
- **activation**: caller-owned act of making the generated wrapper win command
  resolution.
- **caller fixture**: non-shipped generic environment that activates and invokes
  the generated wrapper without a coding-agent-host protocol.
