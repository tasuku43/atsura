# ADR 0008: Keep coding-agent hosts outside the wrapper boundary

- Status: Accepted
- Date: 2026-07-22
- Deciders: Repository maintainer and product owner
- Scope: Product boundary, wrapper materialization, coding-agent consumption,
  architecture, security, compatibility evidence, and release quality
- Supersedes: The coding-agent-host adapter portions of
  `docs/decisions/0005-purpose-specific-surface-and-wrapper.md`
- Historical note: ADR 0004 is already superseded in full by ADR 0005; this ADR
  confirms that its Claude Code hook and settings design is not revived
- Extended by: None

## Context

Atsura already has a host-neutral core that inspects a source CLI, compiles a
reviewed tailoring specification into an adopted bundle, constructs a complete
wrapper plan, and applies one bounded transform runtime through the direct
`atr` gateway.

Earlier designs treated coding-agent hooks as Atsura host adapters. They made
Atsura decode an attempted Bash command, translate core outcomes into vendor
permission values, install vendor settings, and rewrite the host tool input to
an internal helper. That placed Atsura above its actual product boundary.

The intended experience starts one layer lower. Before a coding agent invokes
a source command, an environment has made an Atsura-generated wrapper
available under the ordinary command spelling. The agent then invokes that
ordinary command. It does not ask Atsura to reinterpret or rewrite a host tool
request.

Conceptually:

```text
reviewed specification + source evidence
  -> adopted bundle
  -> deterministic wrapper implementation

external environment exposes that wrapper as the ordinary command
  -> coding agent or maintainer invokes the ordinary command
  -> wrapper constructs a fresh plan
  -> exact source execution and typed stages
```

Bounded Claude Code and Codex research reinforced the boundary correction.
Their hook request, rewrite, permission, trust, configuration, and failure
contracts differ. Modeling either protocol would make Atsura responsible for
an agent-host action that is unnecessary once the wrapper is already the
resolved command. Those observations remain research evidence about external
wiring, not product protocols or runtime dependencies.

## Decision drivers

- Preserve the normal source-command experience for maintainers and coding
  agents.
- Keep routine execution deterministic and independent of an LLM or agent-host
  process.
- Avoid parsing a general shell command or translating vendor permission
  semantics.
- Make one wrapper artifact usable by a direct shell and by different coding
  agents without recompiling the core schema for each vendor.
- Retain exact bundle adoption, fresh planning, source identity, no-shell
  source execution, bounded attempts, and typed transformations.
- Keep source-specific compatibility in source adapters and output-specific
  compatibility in output processors without inventing a coding-agent adapter
  layer.

## Decision

### Atsura ends at a host-neutral wrapper

Atsura owns:

- bounded source inspection through source adapters;
- strict tailoring specification validation;
- deterministic bundle compilation and exact adoption;
- purpose-specific command and option surfaces;
- complete wrapper-plan construction;
- deterministic wrapper materialization from an adopted bundle;
- fresh plan validation and application at each wrapper invocation;
- exact identity-bound source execution; and
- typed before, invocation, output, and after stages admitted by maintained
  compatibility contracts.

The generated wrapper is a product artifact. Its product input is an argv
invocation of the ordinary source-command spelling, not a Claude Code, Codex,
or other agent-host event. Its result is the same tailored result available
through the direct maintainer gateway.

A wrapper binding must identify the exact adopted purpose bundle, source
executable identity, wrapper contract, exact runtime identity, and ordinary
command spelling. Host name, hook event, permission value, settings path,
transcript, session, or model identity is not part of that binding.

### Activation belongs to the caller's environment

An external environment decides how the generated wrapper becomes the command
that a maintainer or coding agent resolves. A shell startup file, an
agent-provided hook, a container image, a development-environment launcher, or
another caller-owned mechanism may expose the wrapper. Atsura does not own,
inspect, edit, or attest that mechanism.

Atsura may generate deterministic, bounded wrapper material from a fixed
template. Configuration cannot supply arbitrary shell, a script body, or an
executable transport program. Whether the first supported materialization is a
shell function, an executable shim, or both is deliberately left to the next
vertical slice and its native evidence. That mechanism choice does not move
agent-host configuration into Atsura.

Activation is not an Atsura authorization boundary. A caller can still invoke
the physical source executable through another path. Surface reduction remains
a purpose and capability boundary, not an OS sandbox.

### Coding-agent hosts are consumers, not adapters

Production Atsura has no Claude Code or Codex adapter. It does not:

- discover, inspect, start, signal, or call an agent-host process, executable,
  service, session, transcript, or API;
- decode a host hook payload or a Bash command string;
- return `updatedInput`, `allow`, `ask`, `deny`, or another host decision;
- install, merge, reconcile, or remove host settings, permission rules, trust
  state, or hooks; or
- infer which host invoked a wrapper.

If a vendor-specific integration is useful, it is external glue whose only
product-facing responsibility is to expose the same generated wrapper. It is
not an Atsura capability, adapter contract, reference kind, lifecycle, or
shared schema.

The word `adapter` remains valid for source-CLI inspectors/runtime admission
and for finite output processors. It no longer denotes coding-agent hosts in
the Atsura architecture.

### Vendor conformance remains downstream

Repository conformance uses a generic caller-owned environment to expose and
invoke the host-neutral wrapper. It does not embed or simulate a vendor hook,
settings, permission, trust, or session protocol.

A downstream vendor integration may prove that it exposes the same wrapper,
but that evidence belongs to the integration and does not create an Atsura host
compatibility API. The public Atsura contract is complete at exact argv input
and tailored result output.

### Existing core and bypass contracts remain

`bundle preview` and `bundle execute` remain explicit maintainer and recovery
gateways. Wrapper invocation must reuse their application/domain authority and
fresh plan construction rather than create another compiler, registry, or
executor.

Raw execution remains a separate manual bypass using the exact bundle-bound
source identity. It is never automatic fallback and is not a substitute for a
broken wrapper or activation mechanism.

Source CLI authentication, authorization, prompts, destination selection, and
downstream effects remain source-owned. An agent host may apply its own
independent controls before calling the wrapper, but Atsura does not interpret
or preserve those controls by equivalence.

## Consequences

### Positive

- The product boundary matches the intended experience: the agent calls an
  already tailored command.
- One wrapper contract can serve maintainers, shells, containers, and coding
  agents without knowing which caller selected it.
- Vendor hook and permission changes cannot force changes to the core bundle,
  plan, or fault schemas.
- Atsura avoids general shell parsing and vendor settings mutation.
- The existing direct runtime remains reusable as the wrapper's lower-level
  execution authority.

### Negative

- Atsura cannot claim to have installed, enabled, or enforced a wrapper inside
  a particular coding-agent host.
- A user or external integration must arrange command resolution before the
  agent starts using the environment.
- Wrapper availability does not prevent direct invocation of the source
  executable through another path.
- Shell-function and executable-shim behavior must be tested separately where
  their command-resolution and portability semantics differ.

## Mechanical enforcement

Current boundary checks:

- Production packages reserve the exact path segments `agenthost`,
  `hostadapter`, `hostintegration`, `claudehook`, and `codexhook`; architecture
  lint rejects those package purposes without rejecting source/output adapters
  or source CLIs that happen to use vendor names.
- Default-catalog tests reject the exact retired Claude Code and Codex
  integration/hook routes and capability identifiers. They do not reserve the
  generic word `integration` for unrelated future outcomes.
- The capability ledger tracks a generic wrapper-materialization result, not
  vendor lifecycle or transport capabilities.

The wrapper implementation slice must additionally prove:

- catalog, specification, bundle, plan, wrapper binding, and result schemas
  contain no agent-host field;
- the generated wrapper and direct gateway use the same application/domain
  plan constructor and source-execution boundary;
- exact bundle/runtime/source binding, separate argv, no-shell
  source execution, zero/one source attempts, drift failure, and no automatic
  raw fallback through tests; deterministic shell bytes have a digest for
  review and release evidence but are not attested after caller-owned
  activation;
- a generic caller fixture compares rendered bytes, bundle, argv, plan, result,
  and attempt evidence using the exact installed artifact. Vendor-specific
  compatibility evidence remains downstream; and
- schema key-inventory tests introduced with the wrapper slice, dependency
  review, and code review enforce the broader absence of host executables,
  APIs, settings, hook payloads, and transcripts; the package-path lint is a
  deliberate structural tripwire rather than a semantic vocabulary scanner.

## Reconsideration signals

Reconsider this decision only if evidence shows that a supported user outcome
cannot be achieved by invoking a preconfigured ordinary command and that the
missing invariant belongs to CLI tailoring rather than to an agent host. A
vendor convenience feature alone is not sufficient; a revision must explain
why the responsibility is product-semantic, preserve a host-neutral core, and
include at least two independent consumers before introducing shared types.
