# ADR 0006: Admit only adapter-proven typed transform execution

- Status: Accepted
- Date: 2026-07-21
- Deciders: Repository maintainer and product owner
- Scope: Bundle runtime, adapter compatibility, source process, output
  transformation, faults, tests, and release evidence
- Extends: docs/decisions/0005-purpose-specific-surface-and-wrapper.md
- Extended by: docs/decisions/0007-prefer-explicit-rtk-optimizer-defaults.md
- Supersedes: None
- Superseded by: None

## Context

ADR 0005 established the purpose-specific surface, wrapper pipeline, canonical
plan, and zero-execution preview. The next product question is whether one
reviewed transform can be applied end to end without turning preview output
into authority, embedding arbitrary executable configuration, or moving
source-operation authorization into Atsura.

A generic plan can prove its own structure, but it cannot by itself prove how a
particular source CLI encodes a structured-output selector or which other argv
would change stdout. Source output also remains untrusted after a compatible
invocation starts. Compatibility admission and output validation are therefore
separate responsibilities.

## Decision drivers

- Demonstrate an RTK-scale output reshape through finite typed built-ins.
- Keep the domain and application runtime vendor-neutral.
- Fail before source start when exact adapter behavior is not maintained.
- Rebuild the plan at execution time instead of trusting preview output.
- Bind execution to adopted source identity and one bounded process attempt.
- Never expose raw source output as failure recovery.
- Make implementation quality hermetic while keeping release-artifact evidence
  an explicit first-tag requirement.

## Decision

### Public boundary

`bundle execute --bundle <path> -- <source-executable> <argv>` is an
`EffectExecute`, `RoleUtility` command. It carries no Atsura mutation target,
impact, or source permission decision.

Execute strictly loads the bundle, requires adoption of its exact digest,
observes current source identity, and calls the same plan constructor as
preview. It never accepts a preview document as input or authority.

### Vendor-neutral core and adapter proof

Application code owns a generic compatibility port whose input is the complete
wrapper plan. Infrastructure adapters implement that port for exact source
contracts. Shared catalog, bundle, plan, execution result, and fault types
contain no GitHub-specific field.

The first implementation is GitHub CLI adapter contract 2. It admits only:

- a successfully inspected GitHub CLI major-2 executable;
- exact command `issue list` or `pr list`;
- a transform wrapper with JSON input and compact JSON rendering;
- exactly one inline `--json=<ordered-select>` before `--`; and
- only the maintained finite option grammar for those commands.

The verifier rejects `--jq`, `--template`, `--web`, positional arguments, an
unknown option, an uncovered option encoding, and any selector mismatch before
the process starts. The major-2 range is a maintained compatibility contract,
not a claim that one captured version proves every future 2.x release.

Passing compatibility admits one attempt; it does not prove that stdout will be
truthful. Successful stdout is still strictly parsed, bounded, and transformed.

### Identity-bound process

The plan declares closed stdin, inherited working directory and environment,
no shell, one maximum attempt, a 30-second timeout, a 4 MiB stdout limit, and a
256 KiB stderr limit. The runtime compares every observable executable
identity with the bundle-bound resolved path, SHA-256, and size before start,
immediately before start, and after wait.

A portable check-to-exec race remains between the last userspace identity check
and the operating system opening the executable. This ADR does not claim that
the race is eliminated.

### Transform and result

The initial runtime requires empty stderr on successful source exit. Stdout
must contain one bounded JSON object or array of objects. Duplicate keys,
trailing values, excessive structure, malformed UTF-8, missing selected fields,
or an invalid transformation fail.

Selection order and JSON types are preserved; configured renames are applied;
explicit empty, zero, false, null, lexical numbers, nested values, and
object-versus-array shape remain distinguishable. A missing selected field is a
contract failure rather than an absent result value.

The fixed schema-2 result contains bundle and plan digests, matched command,
wrapper kind, render/shape/field metadata, transformed records, source exit
code, and exact attempt count. Raw stdout, raw stderr, and unselected fields are
never returned or persisted. Structural external text receives the shared
visible projection before JSON presentation.

### Failure and retry

Every failure after source start is non-retryable, including nonzero exit,
timeout, cancellation, identity drift, output limits, wait uncertainty,
successful stderr, parse/transform failure, and final output failure. Runtime
does not retry, alter argv, drop the transform, or fall back to raw output.

The application boundary maps recognized process fault codes to reviewed
messages and never republishes a process adapter's message. Unknown or
inconsistent results collapse to `unclassified_source_execution_outcome`.

### Evidence and release

The canonical implementation gate is a credential- and network-free synthetic
GitHub-compatible plan and source process that runs through the production
compatibility verifier, identity-bound process runner, parser, transformer, and
CLI output boundary. Unit and negative contract fixtures remain responsible for
each individual race, limit, grammar, and failure class.

This is implementation-quality evidence, not proof that every cross-compiled
archive behaves on its target platform. Before a first release claims this
runtime on a platform, the exact release artifact must replay the documented
preview/execute scenario there. Packaging reproducibility alone is not runtime
compatibility evidence.

## Deferred

- Identity-wrapper and argv-only-transform execution.
- Nonempty successful source stderr.
- A source adapter beyond an accepted compatibility contract.
- Source refresh, raw execution, and host integration.
- Arbitrary shell, jq, RTK program/argv, plugin, script, or runtime-LLM
  transformers. ADR 0007 accepts only a future finite identity-bound RTK
  optimizer contract and does not change this milestone's implemented schema.
- A stronger platform-specific executable handle that closes the remaining
  check-to-exec race.

## Consequences

### Positive

- One user-visible typed output reshape now runs end to end.
- The runtime core remains vendor-neutral and deterministic.
- Preview and execute share one plan identity without making preview authority.
- Unsupported argv and post-start uncertainty fail closed without raw fallback.

### Negative

- Runtime compatibility is intentionally much narrower than catalog discovery.
- GitHub CLI major-2 compatibility requires ongoing fixture maintenance.
- Identity, argv-only, raw, and host experiences remain incomplete.
- First-release runtime claims require platform observations beyond the
  automated packaging profile.

## Mechanical enforcement

- Domain tests require a complete schema-3 plan and plan-derived bound request.
- GitHub adapter tests cover accepted commands/options and reject competing
  output modes, positional data, unknown options, and selector mismatch.
- Source-process tests cover exact argv, no shell, identity observations,
  bounds, and zero/one attempts.
- Application tests cover ordering, preview parity, sanitized faults,
  non-retryable post-start results, strict parsing, and no raw fallback.
- CLI integration runs the production verifier and runner against a synthetic
  self-process, checks plan-digest parity and exactly one attempt, and proves an
  unselected canary is absent.
- `task check` and `task security` decide implementation completion. Public and
  release profiles provide repository and packaging evidence but do not waive
  the exact-artifact runtime replay required before a first tag.

## Reconsideration signals

Create a successor ADR before broadening the accepted source-version range,
adding a source adapter without executable compatibility fixtures, supporting
arbitrary executable transforms, exposing raw output on failure, weakening
identity/adoption checks, or claiming runtime support from packaging alone.
