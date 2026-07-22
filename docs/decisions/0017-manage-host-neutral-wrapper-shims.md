# ADR 0017: Manage host-neutral wrapper shims

- Status: Accepted
- Date: 2026-07-22
- Deciders: Repository maintainer and product owner
- Scope: Persistent POSIX wrapper artifact lifecycle
- Supersedes: None
- Superseded by: None

## Context

The existing renderer produces deterministic POSIX function bytes and the
runtime consumes them without a vendor protocol, but every caller must still
materialize and activate those bytes. Atsura needs one durable ordinary-command
artifact before multiple profiles or host-specific consumers can be evaluated.

Writing arbitrary caller paths or shell content would expand authority beyond
the reviewed wrapper contract. Replacing an existing command would also require
an ownership and recovery contract not proven by the first create/remove slice.

## Decision

Atsura will manage fixed-template executable shims only inside one private,
user-local store selected by the platform configuration root. The store exposes
a `bin` directory for caller-owned command resolution. Atsura reports that path
but never edits `PATH`, startup files, hooks, or vendor settings.

`wrapper install --bundle <path>` is a fixed-target create. It reuses the exact
adopted-bundle, source, processor, runtime, and complete-surface checks already
owned by wrapper materialization, renders a POSIX executable that reaches only
the existing `wrapper run` contract, and publishes at most one active artifact
per ordinary command. An existing different artifact or foreign file is a
conflict; this version performs no replacement.

As a fixed-target act, install produces and consumes no opaque reference.
`wrapper status` performs bounded read-only discovery of owned records and
collisions and is the sole producer of an opaque artifact reference bound to
immutable manifest and shim bytes. `wrapper remove --artifact <ref>` consumes
that reference unchanged and removes only a structurally valid record whose
active shim still matches the recorded artifact. Unknown, tampered, symlinked,
special, multiply matched, or uncertain state is never deleted.

Store mutation uses a private opened root, create-exclusive staging, identity
revalidation, atomic publication where the platform contract supports it, and
directory synchronization on POSIX. Post-action uncertainty is non-retryable
and recovers only through status. Install and remove start no source or
processor. Ordinary shim invocation retains the existing fresh-plan and
zero/one process-attempt rules.

The first implementation is Linux/Darwin only. Windows returns a structured
unsupported result and creates no store state.

## Consequences

- Routine consumers can select one stable directory and call ordinary `gh` or
  `go` without embedding an Atsura or vendor protocol.
- Atsura gains a narrowly confined local filesystem mutation and must maintain
  exact ownership, write-phase, and output-failure tests.
- Activation absence or bypass remains caller-owned and is not fail closed.
- Replacement, multi-profile selection, automatic updates, raw execution, and
  host integration remain separate future decisions.

## Mechanical enforcement

- Catalog mutation contracts distinguish fixed-target create from exact
  reference-bound remove and expose status as the reference producer.
- Domain and store tests bound references, records, enumeration, bytes, names,
  modes, and every symlink/special/collision case.
- Renderer tests prove deterministic bytes, exact argv forwarding, no `eval`,
  no `sh -c`, and no ambient source lookup.
- Native artifact journeys prove install, ordinary help/execution, status, and
  remove on each claimed POSIX target; Windows proves zero mutation attempts.
