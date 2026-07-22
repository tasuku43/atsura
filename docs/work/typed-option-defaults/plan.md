# Work Plan: Apply one reviewable option default only when caller input omits it

- Status: Accepted
- Goal: [goal.md](goal.md)
- Context: [context.md](context.md)
- Tasks: [tasks.md](tasks.md)

## Chosen approach

Add one generic `OptionDefault{Option, Value}` to the invocation vocabulary and
derive an explicit applied subset after caller argv validation. Insert applied
defaults in declaration order immediately after the matched command path,
retain caller-tail order, then retain the existing ordered append stage.

Demonstrate the result with `gh pr list --limit=30` in the same complete bundle
that retains `issue list` append-only behavior. Compile the value into exact
tailored help and prove applied plus overridden ordinary calls from one wrapper.

## Alternatives considered

### Reuse fixed appended argv

Rejected because caller/configured duplicates have source-specific precedence
and cannot produce a deterministic effective plan.

### Add general replacement rules first

Deferred because replacement needs a broader grammar for values, repetition,
positionals, absence, and conflicts. Omission-only defaults close one useful
outcome with a smaller finite contract.

## Design

### Public contract

No command, role, effect, reference, pagination, authentication, or side-effect
contract changes. Existing utility/read commands accept schema-5 content and
publish updated exact schema inventories. `wrapper run` continues to apply the
fresh plan selected by one exact adopted bundle.

Old specs, bundles, plans, and wrappers fail before source execution with
stable regeneration/render recovery. Exact command help discloses each default
value; it remains fixed artifact-local output with zero process attempts.

### Layer changes

- Domain: add strict default option and decision vocabulary, precedence,
  insertion, plan validation, help metadata, versions, and adoption counts.
- Application: reuse current build, preview, render, run, and registry paths;
  update complete-surface and zero-render assertions.
- Infrastructure: update strict YAML, finite GitHub/Go admission, fixed POSIX
  help rendering, strict bundle migration, and synthetic source fixture.
- CLI and catalog: update schema inventories, versioned help prose, stable
  retired-contract recovery, and wrapper review metadata where required.

### Data and control flow

```text
strict schema-5 default option
  -> catalog + surface + non-selector + append-overlap validation
  -> canonical bundle schema 4 and exact adoption
  -> validate caller argv and option surface
  -> retain declared defaults and derive the applied subset
  -> matched command + applied --option=value + caller tail + append_args
  -> canonical schema-6 plan and digest
  -> finite adapter admission
  -> existing one-attempt source/output boundary
```

Static help derives the same default values from the exact bundle before
contract-3 wrapper bytes are rendered.

### Error and cancellation behavior

Invalid schemas, defaults, overlap, option forms, plan mutation, old wrapper
contracts, or unsupported adapter grammar fail before source/processor start.
Planning is pure and not retryable. Ordinary timeout, cancellation, output
bounds, uncertain outcomes, and final delivery behavior remain unchanged. No
raw or original-command fallback is added.

### Security and public boundary

Default values are intentionally public and may appear in generated help and
evidence. They cannot be used for credential persistence. Structural validation
rejects controls and unbounded values; no shell interprets them. No new effect,
target, credential, destination, dependency, or coding-agent-host boundary is
introduced.

## Implementation slices

1. Accept ADR 0016 and add failing schema/domain/plan precedence tests.
2. Implement schema-5 bundle and schema-6 plan defaults plus migration.
3. Implement contract-3 help and complete-surface runtime admission.
4. Update application/CLI inventories, adoption summary, and recovery tests.
5. Add evidence schema 8 applied/override journeys and strict mutations.
6. Promote durable documentation, run all gates, push, and verify native rows.

## Verification

- Unit and contract tests: default validation, preserved declaration order,
  precedence, plan reconstruction, help, versions, migration, and runtime
  admission.
- Negative side-effect tests: invalid later entry, old wrapper, plan mutation,
  hidden/selector/append overlap, and malformed caller input all attempt zero.
- Opaque-reference and complete-pagination tests: not applicable; no references
  or collections are added.
- Structured output, hostile-output, and recovery tests: retain projection,
  source-stream, optimizer, hostile argv, and exact next-command contracts.
- Agent-readiness scenario and discovery-round-trip count: one root help and one
  exact-command help; zero source inspection or external reconstruction.
- Human-handoff scorecard: not applicable; setup/authentication do not change.
- Manual observation: source one function; invoke `gh pr list` with no limit and
  with an explicit limit; compare fresh plans and source fixture attempts.
- Required profiles: `task check`, `task security`, `task public:check`,
  `task release:check`, then exact five-target CI and aggregate.
- Generated-diff or artifact checks: schema-8 strict evidence, exact help hash,
  declared/applied defaults, caller/source argv, and candidate archive digests.

## Rollout and rollback

Schema generations are intentionally incompatible. Regenerate schema 5,
rebuild and review bundle 4, re-adopt its new digest, and render contract 3.
No file is rewritten automatically. Rollback uses a matching older binary,
bundle, and wrapper; mixed generations fail before source execution.

## Documentation promotion

Promote the finite default grammar and evidence into theses, product,
architecture, security, harness, release, readiness, CLI help, capability
ledger, and ADR 0016. Keep boolean/short/global/positional defaults, semantic
value types, removal, replacement, and before/after actions explicit gaps.
