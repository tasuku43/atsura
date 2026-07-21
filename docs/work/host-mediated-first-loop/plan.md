# Work Plan: Host-mediated tailored invocation

- Status: Proposed
- Goal: [goal.md](goal.md)
- Context: [context.md](context.md)
- Tasks: [tasks.md](tasks.md)

## Chosen approach

Run a research-first adapter selection. Current evidence makes Claude Code's
`SessionStart` PATH activation plus a synchronous `PreToolUse` health guard the
leading candidate. Complete a zero-cost tool-cycle and settings-lifecycle
prototype before accepting it. If it passes, implement one complete
ordinary-invocation outcome with vendor mechanics confined to infrastructure.
The healthy guard returns no host decision, the shell resolves a native shim
and supplies parsed argv, and the shim consumes only the exact adopted bundle.
Preserve the current surface, plan, and source execution authorities rather
than creating a host-owned parallel model.

## Alternatives considered

### Commit to Claude Code `PreToolUse` immediately

Rejected as the primary rewrite path. The host supplies an arbitrary shell
string rather than argv, and the documented `updatedInput` path is coupled to
`allow` or `ask`. Retain `PreToolUse` only as a health guard that makes broken
activation visible without changing healthy input or normal permission flow.

### Install a generic PATH wrapper first

Rejected as a standalone integration because activation failure silently
resolves the original CLI. Retain a native PATH shim behind a host lifecycle
adapter and synchronous health guard, subject to exact platform fixtures.

### Reuse RTK or another wrapper wholesale

Undecided until primary sources establish its current surface, transformation, raw-access, process, and host-integration contracts. Reuse is preferred when it preserves Atsura's deterministic bundle and trust boundaries; overlap alone is not a reason to reimplement.

## Design

### Public contract

Not yet accepted. Research must choose the exact maintainer setup outcome, public capability classification, role, inputs, outputs, effects, ownership target, recovery commands, platform support, and compatibility contract before implementation. No command path or persisted schema is reserved by this proposed plan.

### Layer changes

- Domain: only host-neutral states and invariants; no vendor payload or allow/ask/deny type.
- Application: smallest ports for translating an attempted invocation and for reconciling Atsura-owned integration state.
- Infrastructure: selected host payload codec, settings adapter, and transport mapping; all vendor fields remain here.
- CLI and catalog: composition, exact setup/status/removal commands if accepted, and catalog-derived help and recovery.

### Data and control flow

```text
untrusted host attempt
  -> bounded host adapter decode
  -> host-neutral invocation request
  -> exact adopted-bundle selection
  -> existing surface resolution and fresh plan
  -> core state: managed / absent / invalid / interaction / not-managed
  -> adapter-specific host response
  -> existing identity-bound source runtime only for managed execution
```

The host never supplies a trusted plan, receipt, executable identity, source permission, or arbitrary recovery argv.

### Error and cancellation behavior

All malformed, ambiguous, drifted, or unowned integration state fails before source start. Installation and removal use one central Atsura mutation boundary and preserve an uncertain post-write outcome as non-retryable with read-only reconciliation. Managed source execution keeps the current zero-attempt pre-start and one-attempt post-start classifications. Not-managed is not a fallback after a managed failure.

### Security and public boundary

Fixtures use synthetic host payloads, isolated settings, no credentials, no provider transport, and no shell evaluation. The selected external schema, dependency, and license are reviewed and pinned. Repository configuration cannot create a user adoption receipt or overwrite foreign settings.

## Implementation slices

1. Primary-source comparison, bounded protocol prototypes, and host-boundary ADR.
2. Host-neutral domain/application contract and negative tests.
3. Safe integration settings adapter with install/status/remove reconciliation.
4. Selected host transport adapter and ordinary invocation fixture.
5. CLI catalog, exact help/recovery, harness, installed-artifact evidence, and durable documentation.

## Verification

- Unit and contract tests: host-neutral state truth table, strict payload/schema handling, and integration ownership.
- Negative side-effect tests: malformed, ambiguous, unadopted, drifted, absent, and foreign-owned inputs produce zero source attempts and no unintended settings mutation.
- Opaque-reference and complete-pagination tests: determine after the accepted public contract; no reference or pagination should be invented for transport convenience.
- Structured output, hostile-output, and recovery tests: preserve existing selected JSON result and secret/raw canaries across host framing.
- Agent-readiness scenario and discovery-round-trip count: one documented setup path, then ordinary managed invocation without exploratory commands.
- Human-handoff scorecard: compare host scopes, edits, restarts, shell dependencies, OS support, consent, and uninstall certainty.
- Manual observation: selected host in an isolated project and user-config root.
- Required profiles: `task check`, `task security`, `task public:check`, and `task release:check`.
- Generated-diff or artifact checks: exact installed `atr` plus synthetic host fixture on every claimed compatible platform.

## Rollout and rollback

No integration is exposed until its ownership and removal contract is accepted. Rollback removes only the exact Atsura-owned fragment and leaves unrelated host settings byte-for-byte or semantically unchanged according to the accepted format contract. Persisted state disposition must be explicit before the first public install command.

## Documentation promotion

- Record the selected host contract and rejected alternatives in an ADR.
- Update theses only if primary evidence changes the host-neutral mapping or trust model.
- Update product, architecture, security, harness, release, and agent-readiness documents with the accepted setup and runtime boundary.
- Update `$add-capability` if the new host workflow reveals a general rule for future adapters.
