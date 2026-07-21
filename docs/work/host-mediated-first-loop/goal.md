# Work Goal: Host-mediated tailored invocation

- Status: Draft
- Retention: temporary
- Retention reason: None
- Governing contract: `docs/00_theses.md` through `docs/04_harness.md`
- Review/delete trigger: Delete after durable conclusions are promoted and the change completes
- Successor: The next highest-impact gap in the thesis coverage matrix
- Owner: Codex
- Target: First host-adapter iteration under the long-horizon thesis goal
- Related ADRs: ADR 0005 and ADR 0006; a new host-boundary ADR is expected

## Outcome

A maintainer can install one reviewed Atsura integration for a selected coding-agent host, bind it to an exact adopted bundle, and let an ordinary managed source-CLI attempt reach the existing deterministic surface, fresh-plan, and transform runtime without manually invoking `atr bundle execute`. An included GitHub CLI list command returns the same selected and renamed JSON result with one source attempt; a command absent from the tailored surface produces no plan and zero source attempts; an attempt outside the integration's declared scope remains explicitly not managed. Removing the integration preserves unrelated host configuration.

The host will be selected only after current primary-source research. The user outcome is host-mediated tailoring, not a Claude-specific core model.

## Why now

The direct maintainer path is release-quality, but Atsura has no host adapter. The current product therefore proves deterministic compilation and execution without yet proving its primary coding-agent experience. The user also required RTK, Claude Code hooks, and existing wrapper or policy tools to be checked from primary sources before Atsura chooses an overlapping mechanism.

## Non-goals

- Decide whether a source operation is authorized or safe.
- Add another source CLI, new wrapper stages, source refresh, raw execution, usage-history collection, or agent-generated proposals.
- Implement the RTK-backed default wrapper candidate selected for a later bounded iteration; this host slice may only consume an already compiled wrapper.
- Treat a host `allow`, `ask`, or `deny` value as core source-operation policy.
- Embed arbitrary shell, scripts, jq, RTK programs, plugins, or a runtime language model in a tailoring specification.
- Publish, tag, release, or create a pull request.

## Acceptance criteria

- [ ] Primary-source research compares current RTK behavior, relevant coding-agent host protocols, and representative CLI wrapper or policy tools; an ADR records reuse, integration, and non-adoption decisions before mechanism code is added.
- [ ] One exact maintainer setup outcome and one host scope are selected with a bounded human-handoff scorecard; public command names and persisted schema are not fixed before that decision.
- [ ] Host-neutral core states distinguish managed rewrite, command absent from surface, invalid invocation, interaction required, and not managed without importing vendor decision vocabulary.
- [ ] Installation, status/reconciliation, and removal have explicit Atsura-owned mutation contracts, preserve unrelated host state, and fail closed on malformed, ambiguous, drifted, or foreign-owned configuration.
- [ ] A credential- and provider-network-free production-composition fixture proves ordinary host attempt to tailored result, preview/execute plan parity, one exact source attempt on admitted success, zero attempts and no plan for surface absence, explicit not-managed behavior, and no hostile, secret, or raw-output leak.
- [ ] Exact host recovery metadata lets an agent select a valid next action without prose interpretation, and routine supported success requires no external parser or runtime model.
- [ ] The installed `atr` artifact replays the host-mediated journey on every claimed platform for which the selected host mechanism is supported; unsupported platform claims are explicit rather than skipped.
- [ ] Required focused tests and `task check`, `task security`, `task public:check`, and `task release:check` pass on the final clean committed tree.

## Governing documents

- Thesis: agents propose while the deterministic core compiles; host transports do not define core semantics.
- Product contract section: host adapter and current public artifact and transform workflow.
- Architecture or security invariant: source and host specifics stay in adapters; settings and host payloads are untrusted; Atsura-owned mutations cross one controlled boundary.
- Existing ADR: ADR 0005 corrects authorization-centered semantics; ADR 0006 bounds the first transform runtime.

## Completion definition

The work is complete when every acceptance criterion has executable evidence, the host choice and boundary are promoted into durable contracts and an ADR, all required profiles and installed-artifact checks pass, and this temporary packet is removed. Completion advances the long-horizon goal to the next largest thesis coverage gap; it is not a terminal product release.
