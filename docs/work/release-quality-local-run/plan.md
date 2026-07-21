# Work Plan: Release-quality local tailoring run

- Status: Accepted
- Goal: [goal.md](goal.md)
- Context: [context.md](context.md)
- Tasks: [tasks.md](tasks.md)

## Chosen approach

Add one read-only `atr run` capability that reloads and compiles the exact
schema-1 policy, rejects anything but an allowed read plan, resolves and
fingerprints one executable, performs one bounded no-shell process attempt,
strictly parses successful JSON into typed domain values, applies the plan's
select/rename operation, and renders one fixed execution envelope.

## Alternatives considered

### Execute arbitrary allow/deny source operations

Rejected for v0.1 because a single catalog effect cannot honestly represent
unknown create/write impact, target binding, confirmation, or replay risk.

### Transform an offline JSON fixture only

Useful as a unit boundary but insufficient as the supported outcome because it
does not test executable identity, process limits, one-attempt behavior, or the
wrapper sequence in Thesis 3.

### Add hook interception with execution

Deferred because host trust, installation, discovery hiding, and command rewrite
protocols are independent risks. The local run command gives a deterministic
adapter target for later hook research.

## Design

### Public contract

- Outcome: execute one explicitly configured read-only source command and receive its tailored JSON records.
- Command: `atr run --config <path> -- <source-command> [args...]`.
- Capability: `tailoring.execute`, public.
- Role/effect: utility/read; no opaque references and no mutation contract.
- Input: required single `--config`, required repeatable source command after `--`.
- Output: JSON only, schema 1, fixed `execution` object with decision, matched command, reason, result shape, ordered fields, records, and direct source-process attempt count.
- Delivery/coverage: complete/exhaustive for the exact bounded source result observed by this invocation.
- Authentication: none owned by Atsura; the source process may use its inherited environment and own credentials.
- Stable failures: invalid arguments/config/invocation; policy denied or unmatched; executable missing/unsafe/changed; process start/nonzero/timeout/cancel; stdout/stderr limits; JSON parse/shape/field/complexity; output encoding/write; internal contract.
- Recovery: exact `help run`, `plan preview`, or `run` paths only; no argument-bearing guessed recovery.

### Layer changes

- Domain: explicit read effect in policy/plan, source-process request/result/identity invariants, typed JSON values, pure select/rename transformation.
- Application: load/compile/reject ordering, one process port call, result validation, parse port call only after exit 0, transform, and typed execution result.
- Infrastructure: strict config loader update, executable resolution/fingerprint/revalidation, bounded process adapter, bounded duplicate-aware JSON parser.
- CLI and catalog: `run` registration, composition, fixed JSON renderer, visibly escaped successful source stderr, capability ledger and contract probes.

### Data and control flow

```text
catalog argv
  -> strict explicit YAML load
  -> pure Compile(policy, invocation)
  -> reject deny/non-read/mismatch
  -> resolve + fingerprint executable
  -> revalidate + direct Start once
  -> bounded stdout/stderr + Wait + identity revalidation
  -> parse successful JSON into typed values
  -> pure select/rename transform
  -> render fixed execution envelope
  -> checked stderr warning write, then checked stdout success write
```

### Error and cancellation behavior

Preflight faults have zero attempts. A successful `Start` fixes attempts at one
for every later outcome. Caller cancellation, the 30-second timeout, or either
byte bound cancels the child; Atsura never retries. Nonzero exit suppresses
stdout and source stderr from the public fault. Transform failure occurs after
one attempt, suppresses raw output, and points to preview/help rather than a raw
route. Since v0.1 accepts only a user-declared read effect, cancellation and
write-stream recovery may be retryable, but Atsura itself performs no retry.

### Security and public boundary

The source executable is resolved once, required to be a regular executable,
hashed with SHA-256, and revalidated before and after the attempt. This detects
ordinary PATH, symlink-target, and replacement drift but cannot make a
compromised kernel/filesystem or source-created child process trustworthy.
Captured bytes stay in memory and are not logged or persisted. Invalid or
failed output never crosses stdout. Fixtures contain only synthetic data.

## Implementation slices

1. Commit preview checkpoint; accept ADR and durable release-quality contract.
2. Add failing domain/application/adapter/catalog tests.
3. Implement domain plan/effect, typed JSON, and transformation.
4. Implement process and JSON infrastructure adapters plus application run use case.
5. Add CLI command, fixed output, examples, support/security docs, and agent-readiness evidence.
6. Pass every gate, run the synthetic fixture, remove this packet, and commit the clean final tree.

## Verification

- Unit and contract tests: policy effect, plan equivalence, JSON value invariants, select/rename, catalog and output schema.
- Negative side-effect tests: zero calls for all policy/preflight failures and one call for every post-start failure.
- Opaque-reference and complete-pagination tests: not applicable; no references or pagination.
- Structured output, hostile-output, and recovery tests: duplicate/malformed/deep/large JSON, missing fields, nonzero/timeout/cancel, hostile stderr, short writers.
- Agent-readiness scenario and discovery-round-trip count: root index plus exact `run` scope, at most two; known path one.
- Human-handoff scorecard for setup/authentication candidates: no Atsura setup or authentication; inherited source authentication remains source-owned.
- Manual observation: compile and invoke the synthetic source fixture through `atr run`.
- Required profiles: focused tests, full, security, public, release.
- Generated-diff or artifact checks: `go mod tidy`, release matrix reproducibility, final clean Git status.

## Rollout and rollback

Schema 1 and public commands are still pre-release. This change deliberately
adds required `effect: read`, updates the example, and freezes the supported
v0.1 boundary for a future prerelease. No state is persisted or migrated.
Rollback removes `run`, the execute capability, adapters, effect field, and
examples without cleanup of external state because this work creates none.

## Documentation promotion

- Theses: first executed hypothesis and read-only v0.1 boundary.
- Product: exact run outcome, schema, output, and failure semantics.
- Architecture/security: process, identity, byte, timeout, JSON, stderr, and trust boundaries.
- Harness: executable release-quality definition and tests.
- Release: v0.x compatibility, matrix, ownership, package, signing, and withdrawal decisions without publication.
- README, SECURITY, SUPPORT, and agent-readiness: current supported behavior and limitations.
