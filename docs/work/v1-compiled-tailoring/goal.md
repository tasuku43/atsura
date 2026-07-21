# Work Goal: Complete the v1 compiled tailoring experience

- Status: Active
- Retention: temporary
- Retention reason: None
- Governing contract: docs/00_theses.md
- Review/delete trigger: Delete after all v1 conclusions are promoted and the final committed tree satisfies every acceptance item
- Successor: None
- Owner: Repository maintainer
- Target: Atsura v1 finite ideal state
- Related ADRs: docs/decisions/0004-v1-compiled-tailoring-bundle.md

## Outcome

A maintainer can take a supported installed source CLI from zero Atsura state
to an explicitly trusted, version-bound tailored surface, use it manually or
through a supported coding-agent adapter, and diagnose, refresh, bypass
explicitly, or remove every part without runtime LLM policy or arbitrary
policy shell. GitHub CLI 2.x and project-local Claude Code are the first real
compatibility adapters, not concepts embedded in the core contracts.

## Why now

The v0.1 slice proves deterministic local execution but leaves YAML selection,
source knowledge, trust, and agent integration manual. The product owner chose
the compiled bundle concept and delegated subsequent product decisions so the
complete north-star workflow can now be tested rather than extended through
unconnected utilities.

## Non-goals

- Claiming tested compatibility for every CLI, GitHub CLI major version 3, or
  global host setup in v1.
- Executing arbitrary policy shell, jq, RTK, plugins, or external transformers.
- Inferring source effects, targets, or safety from command names or help prose.
- Automatically trusting repository state, migrating trust across digests, or
  selecting raw execution as fallback.
- Persisting credentials, source output, usage history, or agent transcripts.
- Publishing, pushing, tagging, opening a pull request, or creating a release.

## Acceptance criteria

- [ ] ADR 0004 and governing documents define the selected C concept, finite compatibility corpus, public state machine, trust boundary, and exclusions without unresolved implementation-critical choices.
- [ ] Core catalog, policy, bundle, plan, and decision schemas contain no GitHub- or Claude-specific fields; conformance fixtures exercise an alternate synthetic source and host adapter.
- [ ] `source inspect` and `source refresh` deterministically produce provenance-bearing catalogs for validated GitHub CLI 2.x reference fixtures under declared probe limits and classify extensions/dynamic evidence without granting permission.
- [ ] `policy init` and `policy validate` create and validate strict schema-2 deny-by-default policy with visibility, effect, decision, target/impact, argv, and typed transform contracts.
- [x] `bundle build`, `bundle trust`, and `bundle status` produce canonical content-addressed bundles, require interactive user-local trust, detect every source/catalog/policy/bundle drift, and never overwrite unrelated state.
- [ ] `plan preview`, `plan explain`, and `run` consume the same trusted bundle; deny, untrusted, unconfirmed, mismatch, and drift paths make zero source task attempts.
- [ ] Read execution and confirmed mutation execution use exact argv, one controlled attempt, bounded process/output behavior, typed transformations, and non-retryable uncertain mutation outcomes.
- [ ] `raw` is an explicit manual source-identity-bound bypass, is never fallback or recovery, and is absent from the tailored agent surface.
- [ ] Claude Code install/status/remove is project-local, reversible, exact-owner-scoped, preserves unrelated settings, and uses SessionStart for discovery plus PreToolUse for enforcement and rewriting.
- [ ] Hook fixtures prove simple-command allow/confirm/deny/rewrite, managed compound-command fail-closed behavior, hostile input isolation, and zero bypass through direct hook-only commands.
- [ ] The complete clean setup, routine run, deny, confirmation, drift, refresh, raw, update, and uninstall journeys pass fixture-driven end-to-end tests with declared attempt counts and no persisted secret/raw output.
- [ ] Every public command is catalog-discoverable; unknown outcomes need at most root plus one scoped help call; routine success needs zero undeclared external reconstruction.
- [ ] Migration, supported platforms, known limitations, README, SECURITY, SUPPORT, and release preparation describe the actual v1 behavior and no broader claim.
- [ ] Focused tests and `task check`, `task security`, `task public:check`, and `task release:check` pass on the same final committed tree.
- [ ] Durable conclusions are promoted, this temporary packet is removed, milestone commits exist, Git status is clean, and no publication action occurred.

## Governing documents

- Thesis: `docs/00_theses.md`, all theses and the v1 target
- Product contract section: `docs/01_product_contract.md`, v1 compiled tailoring workflow
- Architecture or security invariant: `docs/02_architecture.md` and `docs/03_security_model.md`
- Existing ADR: `docs/decisions/0004-v1-compiled-tailoring-bundle.md`

## Completion definition

The goal is complete only when every acceptance checkbox has direct evidence,
all durable decisions are enforced by types/tests/contracts, all required gates
pass on the final commit, the temporary packet is absent, Git status is clean,
and the full user objective—not merely one intermediate slice—survives a
requirement-by-requirement audit.
