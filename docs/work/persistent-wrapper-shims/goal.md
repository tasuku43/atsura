# Work Goal: Persist one host-neutral wrapper shim

- Status: Active
- Retention: temporary
- Retention reason: Implementation coordination only
- Governing contract: ADR 0017
- Review/delete trigger: Delete after the capability is release-quality and durable conclusions are promoted
- Successor: None
- Owner: Atsura maintainers
- Target: Current iteration
- Related ADRs: 0005, 0008, 0014, 0017

## Outcome

A maintainer can install one exact adopted POSIX wrapper as an Atsura-owned
executable shim, add the reported directory to command resolution through
caller-owned configuration, invoke the ordinary `gh` or `go` spelling through
the existing fresh-plan runtime, inspect installed artifacts, and remove only
the exact artifact reference returned by Atsura.

## Why now

`wrapper render` proves the runtime contract, but routine use still requires a
caller to materialize and source function bytes for every environment. A
managed shim closes that daily-use gap without adding a coding-agent host.

## Non-goals

- Editing `PATH`, shell startup files, hooks, or vendor settings
- Replacement, automatic update, multi-profile selection, or Windows support
- Raw execution, source refresh, new transforms, or arbitrary shell content

## Acceptance criteria

- [ ] `wrapper install --bundle <path>` creates only a fixed-template shim in the user-local Atsura store and emits one opaque artifact reference.
- [ ] `wrapper status` reports bounded owned records and foreign collisions without starting a source or processor.
- [ ] The reported `bin` directory can be placed on `PATH` by the caller; ordinary help and execution match contract-3 `wrapper run` behavior.
- [ ] `wrapper remove --artifact <ref>` removes only the exact validated owned artifact; tamper, symlink, special-file, collision, and uncertain outcomes fail closed.
- [ ] Linux and Darwin installed artifacts pass; Windows returns structured unsupported behavior with zero store/source/processor attempts.
- [ ] `task check`, `task security`, `task public:check`, and `task release:check` pass.

## Governing documents

- Thesis: host-neutral ordinary-command wrapper and caller-owned activation
- Product contract: generated wrapper, bundle adoption, raw/caller boundaries
- Architecture/security: one controlled local mutation boundary; no shell fragments or ambient source discovery
- Existing ADR: 0005, 0008, 0014

## Completion definition

The installed artifact reproduces the user outcome on every claimed target,
all gates pass, learned durable decisions are promoted, and this packet is
removed.
