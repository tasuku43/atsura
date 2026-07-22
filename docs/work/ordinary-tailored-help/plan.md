# Work Plan: Ordinary tailored help

- Status: Active
- Goal: [goal.md](goal.md)
- Context: [context.md](context.md)
- Tasks: [tasks.md](tasks.md)

## Chosen approach

Derive one bounded typed help projection from the canonical bundle during
`wrapper render`, bind it into the generated-wrapper product closure, and have
the fixed POSIX renderer emit exact argument-count/segment comparisons for root,
namespace, and included exact-command `--help`. Each matched branch prints
only fixed single-quoted values through a constant `%s\\n` format and returns
without starting `atr`. Every non-help argv falls through unchanged to the
existing `wrapper run` invocation. The function body is an isolated subshell;
alias-safe cleanup removes caller-defined `command` and `return` functions
there before escaped builtin lookup, without modifying the caller's shell.

## Alternatives considered

### Route help through `wrapper run`

This reuses runtime bundle loading, but mixes a read-only presentation into an
`EffectExecute` command whose public output authority is exclusively the fresh
execution plan. It would either misdescribe help as a source plan or create a
larger dispatch union.

### Add a separate runtime `wrapper help` command

This preserves a read-only application boundary but requires shell-side argv
classification or a new dispatch façade and starts `atr` for information that
is already closed in the reviewed wrapper artifact. It remains a fallback if
the bounded static material contract proves insufficient.

## Design

### Public contract

No new user-authored configuration or source command is added. The existing
`tailoring.wrapper.materialize` capability gains a wrapper-owned final
`--help` grammar. It remains a `RoleUtility`; `wrapper render` is read-only and
`wrapper run` remains execute-only for non-help argv. The generated-wrapper
contract version changes because its fixed material semantics change. Help is
complete, non-paged text with zero source/processor attempts and no opaque
references or authentication requirement.

### Layer changes

- Domain: add a validated, detached help projection derived solely from a
  valid bundle and included option surface; bind it to the exact wrapper.
- Application: continue to verify adoption, identities, and whole-surface
  compatibility before giving the complete binding to the renderer.
- Infrastructure: render fixed POSIX argument comparisons and literal help
  lines; preserve the existing exact fallthrough invocation.
- CLI and catalog: describe the revised wrapper contract and help behavior;
  keep `wrapper run`'s fresh-plan output authority unchanged.

### Data and control flow

```text
validated adopted bundle
  -> bundle-derived semantic help projection
  -> exact wrapper binding + runtime identity
  -> fixed POSIX function and digest
  -> final --help selector: literal help, no bound atr/source/processor
  -> every other argv: existing atr wrapper run path
```

### Error and cancellation behavior

Invalid or unbounded help projections fail `wrapper render` before any wrapper
bytes are emitted. A selector not compiled into the wrapper falls through to
the existing fail-closed fresh-plan resolution, preserving
`command_not_in_surface` or `invalid_invocation` and zero source attempts.
Successful static help has no Atsura runtime cancellation boundary. POSIX may
implement its formatting utility outside the shell process. A final output
write failure returns the underlying shell print status and is not retried.

### Security and public boundary

No credentials, source help bytes, caller environment, host protocol, or new
process enters the model. Catalog summaries and specification reasons remain
untrusted data: structural controls are rejected by existing canonical input
validation and shell punctuation is always a single-quoted `%s` argument, not
code or a format string. Exact argument matching uses shell grammar rather
than reconstructed command strings.

## Implementation slices

1. ADR, contract, and failing domain/renderer tests
2. Typed help projection and wrapper-binding contract
3. Fixed POSIX presentation and exact forwarding regression tests
4. CLI/catalog, installed-artifact journey, and schema-6 evidence contracts
5. Durable documentation, all gates, native matrix, and packet cleanup

## Verification

- Unit and contract tests: help projection, wrapper binding, POSIX renderer,
  wrapper application, CLI catalog
- Negative side-effect tests: hidden/unknown selectors, hostile text,
  malformed binding, zero source/processor attempt logs
- Opaque-reference and pagination tests: not applicable; no references or
  collection pagination
- Structured output, hostile-output, and recovery tests: deterministic help
  golden, literal punctuation, existing structured fallthrough faults
- Agent-readiness scenario: one ordinary `--help` invocation discovers the
  complete finite command-path surface with zero external reconstruction
- Human-handoff scorecard: not applicable; no setup/authentication change
- Manual observation: source generated function in a private shell and invoke
  root/namespace/exact/hidden help
- Required profiles: `task check`, `task security`, `task public:check`,
  `task release:check`, then native artifact CI
- Generated-diff or artifact checks: exact wrapper bytes/digest and release
  evidence for every claimed POSIX target; Windows retains its unsupported
  POSIX-materialization contract

## Rollout and rollback

Newly rendered wrappers use the revised generated-wrapper contract. Existing
wrappers remain closed over their old exact `atr` runtime identity; replacing
that runtime causes existing drift protection to fail before source start.
Rollback is a code and contract-version revert followed by re-rendering; no
persisted wrapper installation or migrated state exists.

## Documentation promotion

- State ordinary tailored help as part of the purpose-specific CLI thesis.
- Define help as a compiled bundle/surface projection in product and
  architecture contracts.
- Record no-source-help and literal-rendering security boundaries.
- Accept an ADR for static compiled help versus runtime dispatch.
- Update harness/release evidence expectations for the generated-wrapper
  contract.
