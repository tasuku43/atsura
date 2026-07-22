# Work Plan: Add the second source CLI runtime

- Status: Accepted
- Goal: [goal.md](goal.md)
- Context: [context.md](context.md)
- Tasks: [tasks.md](tasks.md)

## Chosen approach

Add `go-cli` as a second bounded source-inspection adapter and Go CLI contract
1 as a second implementation of the existing runtime-compatibility port. The
adapter performs exactly `version`, root `help`, and `help test` probes, accepts
Go 1.26.x, and emits a normal vendor-neutral catalog. A small application
registry dispatches plan and complete-surface verification by the existing
namespaced adapter kind. The first Go runtime accepts only one identity-wrapped
`test` command and no caller arguments, then reuses the exact bundle, adoption,
fresh-plan, source-process, source-stream, wrapper, and presentation boundaries
already used by GitHub CLI.

## Alternatives considered

### Add RTK and Go in one iteration

Rejected for this iteration. It combines source inspection, source runtime,
processor observation, processor execution, optimizer semantics, schema
migration, third-party provenance, and native processor packaging. Closing the
source adapter first leaves one independently testable compatibility boundary.

### Use Git status as the first RTK tuple

Rejected by primary-source evidence. RTK itself performs an additional plain
status call because porcelain output omits rebase, merge, and cherry-pick
state. Atsura's one-source-attempt contract cannot reproduce that correction,
and the pipe transform saves at most about one token.

### Put Go-specific branches in plan application and wrapper rendering

Rejected. Source-specific compatibility belongs behind the existing port. A
finite injected registry preserves one shared application path and makes the
absence of source-vendor fields mechanically testable.

## Design

### Public contract

`source inspect` remains the existing `RoleUtility`, `EffectExecute`, public
capability `tailoring.catalog.inspect`; its `--adapter` values become
`github-cli|go-cli`. The Go adapter output uses the existing complete JSON
catalog envelope and reports exactly three source attempts. It produces or
consumes no opaque references.

Existing `spec`, `bundle`, and `wrapper` commands remain the public workflow.
The one new runtime compatibility statement is: a Go CLI contract-1, Go 1.26.x
bundle whose complete included surface is one identity-wrapped `test` command
with no observed long options can render; `wrapper run ... -- test` returns the
plan-declared source streams and status. Any additional argv is unsupported and
starts no source process.

### Layer changes

- Domain: no source-specific type. Add only tests proving two adapter kinds
  round-trip through existing catalogs, bundles, plans, and bindings without a
  vendor field.
- Application: add a finite compatibility registry implementing the existing
  runtime and whole-surface ports.
- Infrastructure: add `internal/infra/gocli` inspector and runtime verifier;
  reuse the existing identity-bound source-process adapter.
- CLI and catalog: register `go-cli`, compose both compatibility verifiers,
  update exact help and attempt descriptions, and retain one source execution
  and result renderer.

### Data and control flow

```text
go-cli selection
  -> fixed go version / help / help test probes
  -> vendor-neutral source catalog
  -> existing specification / bundle / adoption / plan flow
  -> compatibility registry selects exact Go contract from plan evidence
  -> existing sourceexec RunBound once
  -> existing source_stream_passthrough result and final writers
```

### Error and cancellation behavior

Malformed version/help evidence or identity drift rejects inspection after its
bounded observed attempt count. Runtime adapter/version/command/surface/argv
mismatch is `wrapper_runtime_not_supported` before source start. Process start,
wait, timeout, overflow, cancellation, identity uncertainty, conventional
completion, and final writes retain ADR 0010's exact behavior. No error
recommends replay after a Go test attempt.

### Security and public boundary

The Go executable and tested repository are untrusted. `go test` may compile
and execute arbitrary project code, mutate caller-owned caches or files, use
credentials, and access networks. Atsura does not classify, constrain, or
authorize those source-owned effects. The native fixture is dependency-free,
credential-free, disables module/toolchain download, and isolates cache roots;
this is fixture evidence, not a sandbox claim. No Go or RTK artifact ships in
the Atsura archive.

## Implementation slices

1. Accepted ADR, work packet, and failing inspector/registry contracts.
2. Go inspector and runtime verifier with hostile and zero-attempt tests.
3. CLI composition, catalog/help, and full direct workflow tests.
4. Exact installed-artifact Go inspection and POSIX ordinary-wrapper journey.
5. Governing documentation, Skill/harness enforcement, all gates, and packet
   removal.

## Verification

- Unit and contract tests: Go version/help parsing, exact probes/identity,
  runtime plan and whole-surface truth tables, registry dispatch.
- Negative side-effect tests: unsupported version, malformed help, drift,
  extra option/argument, wrong wrapper/result mode, and unknown adapter produce
  zero routine source attempts.
- Structured output, hostile-output, and recovery tests: controls and
  secret-shaped Go help remain bounded/unpersisted; arbitrary binary-like
  routine streams retain ADR 0010's exact behavior.
- Agent-readiness scenario: root help plus exact `source inspect` scope; direct
  catalog-to-spec and bundle-to-wrapper values pass unchanged.
- Manual observation: real Go 1.26.5 inspection and no-argument test wrapper on
  the development platform.
- Required profiles: `task check:fast`, `task check`, `task security`, `task
  public:check`, `task release:check`, and pushed native CI matrix.
- Generated-diff or artifact checks: exact archive journey/evidence schema and
  no vendor fields in shared serialized contracts.

## Rollout and rollback

No persistent schema changes or new Atsura state are introduced. Removing the
new adapter before publication means removing its CLI allowed value,
composition registration, verifier, fixtures, and compatibility claim; existing
GitHub CLI bundles and receipts are byte-compatible. Already adopted Go bundle
digests become inert if the implementation is rolled back, and no automatic
receipt cleanup occurs.

## Documentation promotion

- ADR 0011: Go CLI contract 1 and the reason it precedes RTK execution.
- Theses and product contract: two source adapters, exact first Go grammar, and
  retained host independence.
- Architecture and security: compatibility registry and Go source-owned test
  effects.
- Harness, release, readiness, and add-capability Skill: native multi-source
  evidence and exact Go journey.
