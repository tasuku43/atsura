# Work Goal: Execute one proven JSON-transform wrapper

- Status: Active
- Retention: temporary
- Retention reason: None
- Governing contract: `docs/00_theses.md` Thesis 4 and Thesis 6
- Review/delete trigger: Delete after durable conclusions are promoted and the change completes
- Successor: None
- Owner: Maintainer and implementation agent
- Target: First bundle-backed runtime slice
- Related ADRs: ADR 0005

## Outcome

A maintainer can preview and execute the same invocation through one adopted,
current bundle when the matched command has a transforming wrapper whose JSON
selector is proven by the source adapter. Execution rebuilds the plan, starts
the exact bundle-bound executable at most once without a shell, and returns the
complete declared typed JSON transformation. A post-start failure is
non-retryable and never exposes or falls back to raw source output.

## Why now

Zero-execution preview proves the surface and wrapper vocabulary but does not
test Atsura's differentiating runtime hypothesis: a reviewed wrapper can
substantially reshape source-native structured output deterministically.
Review found that the existing process runner is not bound to the bundle's
expected identity and that the catalog does not alone prove selector-value
encoding. Those gaps must close before source execution is public.

## Non-goals

- Identity-wrapper execution, argv-only transform execution, raw execution,
  source refresh, host adapters, hooks, or wrapper installation
- Before/after actions, interactive stdin, arbitrary shell/script/jq/RTK,
  external transformers, or a runtime language model
- Source-operation authorization or source effect inference
- Compatibility claims for an adapter/command/version without offline field
  evidence and an exact runtime selector contract
- Publication, push, tag, pull request, or release

## Acceptance criteria

- [ ] `atr bundle execute --bundle <path> -- <source-executable> <argv>` is a
  catalog-derived `RoleUtility` with `EffectExecute`, no mutation contract, and
  exact preview-compatible input grammar.
- [ ] Execute strictly loads an adopted/current bundle, rebuilds the same plan,
  proves runtime adapter compatibility, and starts no process on any failure
  before the controlled boundary.
- [ ] A plan-derived bound request compares expected path/hash/size before
  start, immediately before start, and after wait; the source starts at most
  once with exact argv, closed stdin, inherited cwd/environment, and no shell.
- [ ] A successful supported transform strictly parses bounded duplicate-free
  JSON, applies select/rename in declared order, preserves typed zero/false/
  null/empty/nested values, and emits one bounded schema-2 execution document.
- [ ] Every failure after one attempt is non-retryable, starts no second
  process, exposes no raw stdout/stderr, and never selects raw fallback.
- [ ] GitHub CLI runtime support is limited to exact commands whose four-probe
  offline adapter evidence includes accepted fields; synthetic fixtures remain
  the credential- and network-free canonical runtime corpus.
- [ ] Exact agent help describes discovery, inputs, nested output, failures,
  recovery, and a known-path discovery budget of one help call with zero
  undeclared parsing or provider notation.
- [ ] Focused tests, `task check`, `task security`, `task public:check`, and
  `task release:check` pass; the final tree is committed and clean.

## Governing documents

- Thesis: `docs/00_theses.md` Theses 3 through 6
- Product contract section: Wrapper execution plan, output transformation,
  output failure boundary, compatibility boundary
- Architecture or security invariant: four-layer dependency direction,
  controlled source-process boundary, fail-closed post-start uncertainty
- Existing ADR: `docs/decisions/0005-purpose-specific-surface-and-wrapper.md`

## Completion definition

The work is complete when every acceptance criterion has repository evidence,
durable contracts describe the runtime truth, all required gates pass on the
committed tree, and this temporary packet is removed.
