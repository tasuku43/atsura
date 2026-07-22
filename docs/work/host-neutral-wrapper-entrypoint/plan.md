# Work Plan: Host-neutral ordinary-command wrapper

- Status: Proposed
- Goal: [goal.md](goal.md)
- Context: [context.md](context.md)
- Tasks: [tasks.md](tasks.md)

## Chosen approach

Add one host-neutral wrapper runtime plus one deterministic POSIX-function
renderer. The runtime receives a render-produced binding and forwarded argv; it
derives the requested source executable from the bundle and reuses a shared
application-level plan-application component. The renderer resolves and binds
the current exact `atr` runtime identity, absolute bundle path and digest, and
an eligible ordinary command name. It does not ask the user to select an `atr`
binary or copy a digest from another command.

This separates product semantics from activation. The caller may source or
evaluate the fixed generated function in its own environment. Atsura neither
edits that environment nor knows what process selected the function.

Wrapper success renders only the plan-declared tailored value. The existing
`bundle execute` command continues to render the complete maintainer evidence
envelope.

## Alternatives considered

### Parse and rewrite agent-host tool requests

Rejected by ADR 0008. It places vendor hooks, permission semantics, settings,
and shell-command parsing above Atsura's wrapper boundary.

### Install an executable PATH shim first

Deferred. A shim can provide broader process-level command resolution but adds
artifact ownership, platform executable format, PATH ordering, source/shim
identity, and atomic lifecycle questions before the underlying generic wrapper
entry point is proven.

### Require users to hand-write a function around `bundle execute`

Rejected as the supported result. It exposes the maintainer envelope, lacks an
expected-digest closure, and makes callers reconstruct quoting and output
semantics outside the public contract.

## Design

### Public contract still to freeze

The product outcomes are a deterministic POSIX-function renderer and its
host-neutral execution entry point. Exact command paths, capability identifiers,
binding flags, and output contracts remain unfrozen until tests resolve:

- how plan-authoritative dynamic JSON is represented honestly in
  `cli.CommandOutput` and scoped help;
- how the renderer reports raw function source plus its digest without making
  a sourced function's bytes runtime authority;
- how the current `atr` executable path/hash/size and wrapper contract version
  are embedded and revalidated without a caller-supplied `--atr`;
- how an absolute bundle path and renderer-computed digest form the closure;
- which exact POSIX `Name`, reserved-word, and special-builtin grammar defines
  an eligible ordinary command; and
- how a bundle is rejected at render time when any included wrapper lacks a
  maintained runtime compatibility contract.

The renderer is `RoleUtility` and `EffectRead`; the execution entry point is
`RoleUtility` and `EffectExecute`. Neither produces or consumes an opaque
reference, performs pagination, authenticates to a provider, or mutates local
state.

### Layer changes

- Domain: add only generic wrapper-output authority or binding vocabulary that
  cannot be represented by existing plan and bundle types. Do not add host or
  shell protocol types to the wrapper plan.
- Application: extract one shared plan-application component used by the direct
  execute and wrapper façades; validate expected bundle and runtime identities
  immediately after strict load and before source identity assessment.
- Infrastructure: add only a pure/fixed POSIX renderer if it cannot live as
  presentation. Reuse bundle, trust, source identity, process, adapter, parser,
  and transformer implementations.
- CLI and catalog: register render/run once, declare dynamic wrapper-output
  authority explicitly, and preserve structured faults on stderr.

### Data and control flow

```text
render inputs
  -> strict bundle load + computed digest + adoption/current checks
  -> all-surface runtime coverage + exact current atr identity
  -> eligible POSIX command name + fixed quoted function template
  -> caller-owned activation

ordinary command argv
  -> wrapper entry point + exact bundle/runtime closure
  -> strict load + expected bundle/runtime identity + adoption/current checks
  -> bundle-derived source executable spelling + forwarded argv
  -> existing fresh plan constructor
  -> exact compatibility admission
  -> exact physical source, no shell, at most one attempt
  -> typed transformation
  -> plan-authoritative tailored stdout
```

### Error and cancellation behavior

Every pre-start contract failure retains its exact structured fault and zero
attempts. Run reuses the existing non-retryable post-start classifications.
Render starts no source process. A renderer fault does not output partial shell
source. No error path emits raw source output, switches bundle, retries, or
selects raw.

### Security and public boundary

Function source is deterministic product output, not specification-authored
code. Paths and digests are quoted by one tested POSIX single-quote algorithm;
function names satisfy a finite identifier grammar. The generated body uses
only a fixed absolute `atr` invocation, structured JSON errors, and `"$@"`.
The rendered function-byte digest is reproducibility evidence, not attestation
after activation.
Production code contains no agent-host dependency or protocol. A generic
caller fixture emits bounded evidence only.

## Implementation slices

1. Correct governing contracts, retire host-specific capability design, and
   commit the boundary change.
2. Freeze output authority, command-name eligibility, runtime binding, platform
   matrix, exact public paths, then add failing contract tests.
3. Refactor the existing execute application path and add wrapper run.
4. Add the fixed POSIX function renderer and ordinary-command integration test.
5. Add architecture/security guards and exact installed-artifact replay.
6. Run full gates, promote learning, remove this temporary packet, commit, and
   push each coherent concern.

## Verification

- Unit and contract tests: bundle/runtime digest binding, safe name/quoting,
  exact argv, direct/wrapper plan parity, dynamic output authority, and full
  included-surface runtime coverage.
- Negative side-effect tests: adoption/drift/bundle-or-runtime-digest/surface/
  option/runtime rejection at zero attempts; post-start failures at one.
- Structured output and hostile-output tests: plan-declared value only, no
  envelope/canary/stderr leakage, exact JSON shape.
- Agent-readiness scenario: discover render/run through scoped help and invoke
  the ordinary command with no repository-source inspection.
- Manual observation: activate the generated function in a clean POSIX shell
  against a credential-free source fixture on Linux and macOS; Windows keeps
  existing-command regression evidence but receives no POSIX activation claim.
- Required profiles: `task check`, `task security`, `task public:check`, and
  `task release:check`.
- Generated/artifact checks: deterministic function bytes and exact archive
  replay on every claimed platform.

## Rollout and rollback

The slice adds no persistent wrapper state and edits no caller configuration.
Rollback removes the two catalog commands and renderer/runtime contracts; no
wrapper registry or host settings require migration. Existing bundles,
receipts, preview, and execute remain compatible.

## Documentation promotion

Promote the chosen command names, output authority, digest closure, safe
function grammar, supported platforms, installed-artifact evidence, and any
rejected mechanism into theses, product, architecture, security, Skill, harness,
agent-readiness, and a successor ADR when durable.
