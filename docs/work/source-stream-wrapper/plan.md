# Work Plan: Execute identity and argv-only ordinary wrappers

- Status: Accepted
- Goal: [goal.md](goal.md)
- Context: [context.md](context.md)
- Tasks: [tasks.md](tasks.md)

## Chosen approach

Extend the existing fresh wrapper plan with one explicit result-mode union.
`transformed_json` retains the current typed parser and transformer path.
`source_stream_passthrough` admits only identity or argv-only wrappers, runs the
same bound source request through the same process port, and returns a typed
application result containing bounded bytes and the conventional source status.
The CLI writes complete stdout once and complete stderr once, then returns the
source status. No alternate raw route or executor is introduced.

## Alternatives considered

### Separate raw command

Rejected because it would duplicate bundle loading, adoption, identity,
surface, fresh-plan, and process authority and could become an automatic escape
hatch around failed tailoring.

### Force every wrapper through a JSON or text projector

Rejected because identity and argv-only tailoring must preserve the source
result without inventing a transform, and a future finite optimizer needs an
explicit original-preserving result contract.

### Stream source output directly

Deferred because it would expose bytes before exit, identity, timeout, bounds,
and wait outcomes are known and would require a different uncertainty contract.

## Design

### Public contract

`wrapper render` remains `RoleUtility` / `EffectRead`; `wrapper run` remains
`RoleUtility` / `EffectExecute`. Neither produces or consumes an opaque
reference, models source permission, or adds an Atsura mutation target.

Fresh plan schema 4 names exactly one result mode. The structured help contract
describes both variants:

- `transformed_json`: current compact JSON object/array plus LF, empty stderr,
  and Atsura success status.
- `source_stream_passthrough`: exact bounded source stdout bytes, exact bounded
  source stderr bytes, no framing, and the conventional source status after
  both final writes.

The source-stream variant makes no terminal, UTF-8, prompt, semantic, timing,
or cross-stream-order safety claim. A plan fault remains an Atsura structured
fault; source bytes are never placed inside that fault.

### Layer changes

- Domain: add result mode and schema-4 coherence rules; separate executable
  validation from argv-element validation so empty argv elements remain valid.
- Application: branch the single `planapply` service after one admitted fresh
  plan; recognize only a validated conventional source completion for the
  source-stream variant.
- Infrastructure: extend the existing GitHub compatibility proof to finite
  identity and append-argv-only surfaces; retain the same source executor.
- CLI and catalog: present the result union, write buffered streams in a fixed
  stdout-then-stderr sequence, return source status, and document partial final
  write uncertainty.

### Data and control flow

```text
ordinary function
  -> atr wrapper run (exact bundle/runtime binding)
  -> load + exact adoption + current source identity
  -> rebuild and validate schema-4 fresh plan
  -> source-adapter compatibility proof
  -> one identity-bound source process attempt
  -> transformed_json parser/transform OR source_stream_passthrough result
  -> one complete stdout write, one complete stderr write, final status
```

### Error and cancellation behavior

Pre-start faults remain structured and make zero attempts. After source start,
signal termination, timeout, cancellation, bound overflow, wait uncertainty,
or identity drift is a sanitized non-retryable Atsura fault with both captured
streams suppressed. A normally exited nonzero source is a successful
source-stream application result and carries no Atsura retry advice. A short or
failed final write becomes `execute_output_write_failed`, is non-retryable, may
have produced partial external output, and never recommends replay.

### Security and public boundary

The user reviews and adopts original-output visibility before execution.
Unprojected bytes remain untrusted and caller-visible only on a conventional
completion. They are not logged, persisted, decoded, or copied into error
messages. The current process, path, hash, size, timeout, output limits, and
check-to-exec-race disclosure remain unchanged. No credentials, provider API,
host settings, new dependency, or external content enter the repository.

## Implementation slices

1. Governing ADR, thesis/security exception, and failing schema/result tests
2. Domain result mode and conventional-completion validation
3. Shared application branch and finite adapter surface proof
4. CLI result writer, help/catalog/trust contracts, and focused integration tests
5. Exact ordinary-wrapper artifact journey, durable documentation, and gates

## Verification

- Unit and contract tests: sourceprocess, tailoringplan, planapply, githubcli,
  wrapperrender, wrapperrun, and CLI
- Negative side-effect tests: every pre-start fault is zero attempts; every
  uncertain post-start fault is one attempt with suppressed bytes
- Opaque-reference and pagination tests: not applicable; no references or collection
- Structured/hostile output: arbitrary bounded bytes, successful stderr,
  nonzero status, final write faults, and transformed-JSON regression
- Agent readiness: exact `help wrapper run` and `help wrapper render` remain
  within existing discovery budgets
- Human handoff: not applicable; no authentication or setup flow
- Manual observation: packaged ordinary identity and argv-only commands on
  current POSIX host
- Required profiles: full, security, public, and release
- Artifact checks: Linux amd64/arm64, Darwin amd64/arm64, and Windows amd64
  structured-unsupported evidence

## Rollout and rollback

The plan schema changes from 3 to 4; older detached preview documents remain
non-authoritative and are never executed. Existing schema-2 bundles and
binding-contract-1 functions rebuild the new plan and require no migration.
Rollback is a code rollback before release; no persistent data is created.

## Documentation promotion

- Explicit source-stream result authority and bounded identity definition
- Invariant-12 exception for adopted plan-declared unprojected source bytes
- Conventional nonzero status and dual-writer uncertainty
- Shared runtime branch and finite source-adapter responsibility
- Harness/native evidence requirements and add-capability guidance
