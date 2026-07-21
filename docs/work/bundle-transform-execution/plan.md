# Work Plan: Execute one proven JSON-transform wrapper

- Status: Accepted
- Goal: [goal.md](goal.md)
- Context: [context.md](context.md)
- Tasks: [tasks.md](tasks.md)

## Chosen approach

Add the smallest transform-only runtime. It resolves fresh authority exactly as
preview does, requires an adapter-owned compatibility proof, converts the plan
to an identity-bound process request, starts that request once, strictly parses
and transforms successful JSON, and emits a fixed execution envelope. Identity
and raw output remain outside this slice because they have different output
meaning and safety contracts.

## Alternatives considered

### Execute every wrapper and frame arbitrary output

This broadens the slice but cannot honestly preserve identity-wrapper output
inside a new stable JSON envelope. It also forces a successful-stderr policy
before the differentiating structured transform is validated.

### Treat any cataloged `--json` flag as runtime proof

This assumes a vendor-specific value grammar in the shared core and would let
an empty field inventory act as an undeclared wildcard. Runtime instead selects
an exact adapter contract and fails with zero attempts when proof is absent.

## Design

### Public contract

`bundle execute` is `RoleUtility`, `EffectExecute`, capability
`tailoring.execute`, complete JSON delivery, no collection claim, no references,
authentication contract, fixed target, or mutation contract. Inputs mirror
preview. Output schema 2 contains bundle and plan digests, matched command,
wrapper kind, render/shape/fields/records, source exit code, and attempts=1.
Only a transform wrapper with a typed output stage and empty successful stderr
is supported. Failures declare exact next actions and never suggest raw.

### Layer changes

- Domain: explicit process framing in plan schema 3, detachable output plan,
  bound source request and validation, canonical typed JSON encoding.
- Application: fresh bundle/adoption/identity/plan orchestration, adapter proof
  port, phase-aware one-attempt execution, parse and transform.
- Infrastructure: identity-bound no-shell runner, strict source parser, GitHub
  runtime proof, four-probe offline field evidence.
- CLI and catalog: command registration, composition, fixed schema-2 document,
  nested agent-help schema, declared faults, capability promotion.

### Data and control flow

```text
typed CLI inputs
  -> strict bundle load -> exact adoption -> current identity
  -> same pure plan builder -> adapter runtime proof
  -> plan-derived bound request -> no-shell process once
  -> bounded JSON parser -> pure select/rename -> complete JSON output
```

### Error and cancellation behavior

All authority, plan, compatibility, and request failures prove zero attempts.
After calling the process port, a valid structured process fault is preserved;
an unknown or inconsistent result collapses to non-retryable
`unclassified_source_execution_outcome`. Parser, transform, context, encoding,
and output-write failures after one attempt are non-retryable. Raw output is
not included in messages or recovery.

### Security and public boundary

The source executable, argv, environment, and output are untrusted. Exact
identity and finite bounds cross one process port. No new dependency, network
client, persisted output, credential type, or arbitrary code action is added.
GitHub probes are fixed offline help/version calls; canonical runtime gates use
synthetic output rather than credentials or live provider data.

## Implementation slices

1. Work packet, plan framing, bound request, and runner contract
2. Runtime proof, application orchestration, typed output encoding
3. CLI command, catalog/help/fault/output contracts, compatibility fixture
4. Durable docs, readiness/release evidence, gates, packet removal

## Verification

- Unit/contract: domain, application, infrastructure, CLI, catalog, schema
- Negative side effects: zero calls before compatibility; max one after start
- Structured/hostile output: duplicate/malformed/deep/large/missing fields and
  typed null/false/zero/empty/nested values
- Agent readiness: known path one help call; preview and execute digests equal;
  no external processing for routine success
- Manual: real GitHub 2.72.0 offline inspection; optional authenticated local
  execution is observation, not canonical gate
- Required: `task check`, `task security`, `task public:check`,
  `task release:check`

## Rollout and rollback

This adds one command and bumps the experimental wrapper-plan schema from 2 to
3. Specification schema 3 and bundle schema 2 do not change. Older preview
consumers must request current agent help. Removing the command and returning
the capability to deferred is a state-free rollback; bundles and trust receipts
remain valid because runtime authority is rebuilt from them.

## Documentation promotion

- Runtime milestone and adapter proof in docs 00 through 04
- Agent journey and compatibility limitations in docs 09 and README
- Release evidence boundary in docs 06
- Plan schema 3 process framing and runtime output schema 2 in product contract
