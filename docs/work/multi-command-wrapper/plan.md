# Work Plan: One wrapper serves one complete multi-command surface

- Status: Accepted
- Goal: [goal.md](goal.md)
- Context: [context.md](context.md)
- Tasks: [tasks.md](tasks.md)

## Chosen approach

Replace the GitHub CLI verifier's exact-one-entry condition with a non-empty,
bounded all-entry loop. Extract one pure entry verifier that applies the same
existing command, wrapper/output, fixed-argv, selector, and effective-option
proof independently to every canonical surface entry. Render only after the
whole loop succeeds. Keep Go CLI's distinct one-command contract unchanged.

Prove the user result with one two-command bundle and one rendered `gh`
function. The bundle will combine existing admitted behavior rather than add a
new transformation: `pr list` uses the current compact JSON projection and
`issue list` uses the current append-only source-stream path.

## Alternatives considered

### Generate one wrapper per included command

Rejected because both commands share the same ordinary source spelling `gh`.
Several generated functions would overwrite each other and would not represent
one purpose-specific CLI surface.

### Add typed option defaults first

Deferred. Defaults are valuable but require new specification, bundle, and plan
semantics for presence, insertion, `--`, duplicates, and explicit empty values.
All-entry admission closes a more fundamental surface gap without a schema or
trust-boundary change.

## Design

### Public contract

No public command is added. Existing `wrapper render` remains `RoleUtility`,
`EffectRead`, reference-free, complete-delivery output. Its accepted input
expands from one admitted GitHub command to one non-empty complete set of
admitted GitHub commands from the same bundle. `wrapper run` still resolves
exactly one command per invocation and returns that command's declared result
mode. Existing structured faults and recovery remain unchanged.

### Layer changes

- Domain: no schema change; add integration assertions only if needed to prove
  plan isolation across entries.
- Application: no registry or port change; it already delegates the complete
  bundle once.
- Infrastructure: GitHub CLI runtime verifier loops over all surface entries
  and rejects the entire surface on the first canonical invalid entry.
- CLI and catalog: keep command paths and output schemas unchanged; update
  descriptions only where they incorrectly promise exactly one command.

### Data and control flow

```text
exact adopted bundle with N included entries
  -> runtimecompat selects one source verifier
  -> verifier validates source contract once
  -> verifier validates entries 1..N without I/O
  -> wrapper binding compiles help from all N entries
  -> one deterministic function is rendered

ordinary invocation
  -> exact command resolution in the same bundle
  -> that entry's fresh plan only
  -> existing controlled source/output boundary
```

### Error and cancellation behavior

Empty surfaces and any unsupported command, option cardinality, exposed source
selector, wrapper/output union, or appended argv reject rendering with the
existing finite runtime-admission category. Validation performs no I/O and is
not cancellable. No partial material, fallback, retry, or source attempt is
allowed. Invocation-time behavior and cancellation are unchanged.

### Security and public boundary

The change widens only a pure accepted set inside an already finite GitHub CLI
contract. It adds no credential, filesystem, process, network, mutation,
provider SDK, arbitrary shell, or coding-agent-host boundary. Fixtures contain
only synthetic public values.

## Implementation slices

1. Accept ADR 0015 and add failing multi-entry truth-table tests.
2. Refactor GitHub complete-surface admission into one all-entry loop.
3. Add one CLI/application integration fixture for render and per-command plan
   isolation through the same bundle.
4. Extend installed-artifact evidence with one same-bundle/same-wrapper,
   two-command proof and strict mutation tests.
5. Promote documentation, run gates, commit by concern, push, and verify all
   native rows before deleting this packet.

## Verification

- Unit and contract tests: GitHub verifier accepts two independently admitted
  entries and rejects invalid first/middle/last entries.
- Negative side-effect tests: failed render reaches neither renderer nor source
  process; hidden and unknown invocations remain zero-attempt.
- Opaque-reference and complete-pagination tests: not applicable; no references
  or collections are added.
- Structured output, hostile-output, and recovery tests: reuse both current
  result modes with hostile argv/source bytes and existing fault mapping.
- Agent-readiness scenario and discovery-round-trip count: one root help call
  discovers both paths; one exact help call discovers each option surface; no
  undeclared parser or source inspection.
- Human-handoff scorecard: not applicable; setup and authentication do not
  change.
- Manual observation: source one generated function and invoke both exact paths.
- Required profiles: `task check`, `task security`, `task public:check`,
  `task release:check`, then exact five-target CI and aggregate.
- Generated-diff or artifact checks: deterministic wrapper digest, shared
  bundle digest for both calls, distinct plan digests, and schema-bound evidence.

## Rollout and rollback

There is no persisted state or new command. Older binaries continue to reject a
multi-command surface at render time; the new binary admits it only when every
entry passes the existing finite contract. Rollback means using separate
one-command bundles or an older runtime; no receipt migration is required.

## Documentation promotion

Promote the all-entry admission rule and exact native observation to theses,
product, architecture, security, harness, release, agent readiness, and ADR
0015. Keep typed defaults and persistent activation as explicit later gaps.
