# ADR 0009: Reject the ambiguous RTK git-log tuple

- Status: Accepted
- Date: 2026-07-22
- Deciders: Repository maintainer and product owner
- Scope: First RTK compatibility tuple, semantic validation, optimizer rollout
- Extends: `docs/decisions/0007-prefer-explicit-rtk-optimizer-defaults.md`
- Supersedes: Only ADR 0007's recommended first `git log` / `git-log` tuple
- Superseded by: None

## Context

ADR 0007 accepted the direction of using an exact RTK artifact only as a
post-source output processor and proposed Git `log` plus
`rtk pipe --filter=git-log` as the first tuple to investigate. Current
primary-source review confirms that RTK `v0.43.0` is the latest stable release
and that the narrow `pipe` path can be isolated from source execution,
tracking, tee, project filters, and telemetry. It also found that the proposed
filter does not preserve commit association for all valid Git data.

The `v0.43.0` implementation splits input on the literal text `---END---`.
Git commit messages may contain that text. A synthetic commit body containing:

```text
before
---END---
after
```

produced exit zero and empty stderr while moving `after` into output that can
be read as a different block. This is not merely deliberate information
reduction. It can change which commit a reader associates with text, and the
successful processor result contains no metadata from which Atsura can detect
the ambiguity.

Primary sources reviewed:

- [RTK v0.43.0 release](https://github.com/rtk-ai/rtk/releases/tag/v0.43.0)
- [RTK pipe implementation](https://github.com/rtk-ai/rtk/blob/v0.43.0/src/cmds/system/pipe_cmd.rs)
- [RTK git-log formatter and filter](https://github.com/rtk-ai/rtk/blob/v0.43.0/src/cmds/git/git.rs)
- [RTK never-worse guard](https://github.com/rtk-ai/rtk/blob/v0.43.0/src/core/guard.rs)

The experiment used a synthetic temporary repository and an official Darwin
arm64 artifact whose archive checksum matched the release checksum. No RTK
bytes, source, or output are stored in this repository.

## Decision drivers

- An optimizer may reduce information, but it must not silently invent or
  reassign semantic relationships.
- Exit zero and empty stderr are not sufficient compatibility evidence.
- External processors remain untrusted even when their identity is exact.
- One unsafe tuple must not invalidate the narrower RTK processor direction.
- The next core slice should not be shaped around a rejected filter grammar.

## Decision

The Git `log` plus RTK `git-log` tuple is not an Atsura compatibility contract.
Atsura must not generate it as an authoring default, admit it at runtime, or
describe it as the first supported optimizer.

ADR 0007 remains authoritative for the broader direction:

- Atsura starts the exact source itself;
- RTK, if admitted later, receives only bounded successful stage input through
  one exact `rtk pipe --filter=<finite-name>` invocation;
- the reviewed bundle binds exact processor identity and compatibility facts;
- original input is visible only through an explicitly adopted
  original-preserving result mode; and
- ambient detection, source delegation, arbitrary RTK argv, project filters,
  tracking, tee, and automatic fallback remain prohibited.

Before selecting another RTK tuple, Atsura must first implement and verify its
own plan-declared original-output authority. A future tuple then needs hostile
semantic fixtures that exercise every delimiter, grouping key, truncation
boundary, and association rule on which the filter relies. A candidate is
eligible only when Atsura can validate its input preconditions without guessing
and can distinguish an invalid semantic result from a valid optimization.

The `git-log` tuple may be reconsidered only after one of these is proven:

- upstream RTK accepts a collision-resistant structured input contract;
- an Atsura-owned pre/post normalization has an injective, bounded, and tested
  mapping that restores exact associations; or
- a different source format and filter contract has no ambiguous delimiter.

Merely excluding the demonstrated string from a repository, trusting commit
authors, or checking processor exit status is insufficient.

## Consequences

### Positive

- Atsura does not publish a successful-but-misleading optimizer result.
- The RTK direction remains finite and evidence-driven rather than becoming a
  marketing-support allowlist.
- Original-output authority can be designed from Atsura's product semantics
  before any external processor is introduced.

### Negative

- There is no accepted first RTK compatibility tuple yet.
- A second source adapter and external-processor runtime remain deferred.
- Upstream change, additional normalization, or another filter investigation
  is required before RTK becomes a public authoring default.

## Mechanical enforcement

- `tailoring.output.optimize` remains deferred in the capability ledger.
- Schema 3 continues to reject RTK and external-processor actions.
- No runtime registry contains a `git-log` tuple.
- Future processor fixtures must include hostile semantic-association cases,
  not only byte bounds, exit status, and size reduction.
- A future accepted tuple requires a successor ADR or an explicit extension to
  this decision with exact source, processor, version, platform, and fixture
  evidence.

## Reconsideration signals

Reconsider only when primary-source and native-artifact evidence closes the
delimiter and semantic-association problem. RTK adding more supported commands,
the filter usually producing shorter output, or a successful happy-path fixture
does not satisfy this condition.
