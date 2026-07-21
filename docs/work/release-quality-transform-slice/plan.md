# Work Plan: Release-quality transform journey

- Status: Accepted
- Goal: [goal.md](goal.md)
- Context: [context.md](context.md)
- Tasks: [tasks.md](tasks.md)

## Chosen approach

First make installed `atr` exact help sufficient to author the finite schema-3
transform and recover from validation/runtime admission faults without the
repository source. Then add a native, credential-free source fixture and an
internal artifact-journey orchestrator. The orchestrator invokes only the
`atr` executable extracted from the target archive, records exact source
attempt counts, validates every automatable public artifact step, and seeds an
isolated receipt through the same non-public trust-store adapter. Existing
tests separately prove that a real user can create that receipt only after
controlling-terminal confirmation. Run this composite evidence on all five
target-native GitHub-hosted runners, upload one bounded revision-bound evidence
document per target, pair the exact five-document set with the five candidate
archives, recompute their hashes, and enforce that dependency from the release
gate.

## Alternatives considered

### Add noninteractive public trust

Rejected because redirected input or repository automation could silently
adopt a changed bundle. It would alter a security-sensitive public mutation
solely for test convenience.

### Cross-compile and emulate non-host artifacts

Rejected because emulator behavior is not the claimed host runtime, adds a new
supply-chain boundary, and still leaves native filesystem/config semantics
unproven.

## Design

### Public contract

No command, flag, schema, effect, role, or reference flow is added. Existing
agent-help output contracts gain the nested source-catalog and schema-3
inventories needed for self-contained authoring and finite runtime recovery.
The existing read-only inspection/build/status/preview utilities, fixed-target
local trust mutation, and execute effect remain unchanged.

### Layer changes

- Domain: add the closed vendor-neutral runtime-admission category vocabulary
  shared by application and source adapters.
- Application: project finite runtime-admission categories into stable,
  secret-free public faults without retaining adapter causes.
- Infrastructure: classify runtime admission and make process start/wait
  recovery mechanically testable; extend the synthetic source fixture; use
  the existing exact trust-store adapter only inside the repository
  conformance tool.
- CLI and catalog: complete scoped help metadata without changing invocation
  argv, then replay and assert the catalog-derived interface.

### Data and control flow

The public journey uses exact help plus the generated YAML as the authoring
contract. The conformance tool creates an isolated directory and minimal child
environment, invokes packaged `atr` for exact help, inspection, and artifact
commands, validates bounded public outputs through narrow harness projections,
applies that same documented identity-to-transform edit, seeds one exact
receipt, then compares preview and execute evidence with the fixture's
append-only log. Tool-side editing and receipt seeding prove artifact plumbing
only, not user consent or an agent's unaided authoring behavior.

### Error and cancellation behavior

Every tool failure names the exact journey stage but never forwards captured
source stdout, stderr, or an internal cause. A failed step stops immediately.
The source fixture never retries and the orchestrator rejects any per-command
success count other than 0 preview/1 execute; the complete two-command journey
has four shared inspection attempts and ten fixture attempts including the
four induced post-start failures.

### Security and public boundary

The tool passes no credentials, calls no network API, and executes absolute
paths without a shell. Its receipt lives only below a temporary config root.
The fixture returns a synthetic unselected canary so output leakage is
detectable. No public trust bypass exists in a release binary.

## Implementation slices

1. Work packet and artifact replay contract.
2. Native fixture and artifact-journey tests.
3. Release script and five-target CI/release matrices.
4. Strict five-target evidence aggregation and workflow dependency checks.
5. Durable release/harness/agent-readiness documentation.
6. Focused verification, commits, all four gates, packet removal.

## Verification

- Unit and contract tests: `go test ./tools/artifactjourney ./tools/artifactevidence ./tools/sourcefixture`
- Negative side-effect tests: fixture log proves preview does not start source; canary proves projection.
- Opaque-reference and pagination tests: not applicable; current workflow produces no references or paged output.
- Structured output and recovery tests: journey validates both command results
  and compares every induced fault's kind, code, retryability, and exact next
  actions with the applicable packaged scoped-help declaration.
- Agent-readiness scenario and discovery-round-trip count: exact help plus documented one-pass workflow; no exploratory provider call.
- Manual observation: native host archive replay through `task release:check`.
- Required profiles: `task check`, `task security`, `task public:check`, `task release:check`.
- Generated-diff or artifact checks: two complete reproducible matrices plus exact native extracted-artifact replay.

## Rollout and rollback

There is no production state migration. A rollback removes only test tooling
and workflow evidence, so it must also retract the corresponding release claim.

## Documentation promotion

- Promote exact native artifact replay to `docs/04_harness.md` and `docs/06_release.md`.
- Record the composite interactive-adoption evidence and journey result in `docs/09_agent_readiness_validation.md`.
- Clarify the installed-artifact workflow in the product contract and keep
  test-only composition outside production-layer and trust claims in the
  architecture and security documents. No thesis or capability-ledger
  revision is required.
