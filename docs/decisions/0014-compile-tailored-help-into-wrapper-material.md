# ADR 0014: Compile tailored help into wrapper material

- Status: Accepted
- Date: 2026-07-22
- Deciders: Repository maintainer and product owner
- Scope: Purpose-specific surface discovery, generated-wrapper material,
  output authority, shell rendering, and installed-artifact evidence
- Supersedes: None
- Superseded by: None

## Context

The generated POSIX function currently forwards every argv list to
`wrapper run`. Runtime resolution correctly rejects commands and options that
are absent from the compiled surface, but the ordinary command spelling cannot
positively describe that reduced surface. Root `--help` has no cataloged
command prefix, namespace help has no tailored presentation, and exact-command
`--help` is an execution-shaped attempted option rather than wrapper help.

This leaves the central product phrase "purpose-specific CLI" incomplete. A
caller can enforce the surface through the wrapper, but a maintainer or coding
agent must inspect Atsura artifacts or guess source syntax to discover it.

Help is not a source execution plan. Routing it through `wrapper run` would
mix read-only presentation with an `EffectExecute` command whose successful
output is exclusively governed by a fresh wrapper plan. Starting a second
runtime route for facts already closed into the rendered artifact would add
another public command and invocation-time failure boundary.

## Decision drivers

- The ordinary tailored command should describe its own exact surface.
- The bundle and compiled surface must remain the only semantic source of
  truth; source help must not become invocation-time authority.
- Help must not start `atr`, the source CLI, or an output processor or
  masquerade as a wrapper execution plan.
- The generated shell remains fixed product code with exact argv matching,
  literal data rendering, and no `eval`, `sh -c`, or authored shell.
- Non-help argv must continue byte-for-byte to the existing fresh-plan path.
- The first grammar must not infer vendor help aliases, root/global options,
  short options, or positional syntax.

## Considered options

### Add help as a `wrapper run` result mode

This minimizes generated shell changes, but a help result has no source plan.
It would weaken the exclusive `fresh_wrapper_plan` output authority or require
a broader dispatch union on an execute command.

### Add a public read-only `wrapper help` route

This gives help its own application service and current-state validation, but
the generated function must still classify help-shaped argv before choosing
that route. It also starts `atr` and reopens the bundle for a semantic
projection already fixed by the reviewed wrapper bytes.

### Compile bounded help into the generated wrapper

The renderer receives a typed projection derived from the exact valid bundle,
emits only fixed exact-argv branches, and keeps all other argv on the existing
runtime path. The wrapper digest then reviews the surface presentation and its
dispatch together.

## Decision

Compile tailored help into generated POSIX wrapper contract 2.

The domain derives a versioned, bounded semantic help projection from the
canonical bundle. It contains included exact commands, derived namespace
views, bounded source summaries, tailoring reasons, and effective included
long options with explicit value arity. It contains no raw source help, source
output, permission judgment, host field, or executable action.

The fixed renderer recognizes only these first-slice forms:

```text
<ordinary-command> --help
<ordinary-command> <included-namespace> --help
<ordinary-command> <included-exact-command> --help
```

`--help` must be the final exact argv element. Selector segments are exact
stable command-path elements. The renderer uses argument-count-specific fixed
POSIX patterns and constant `%s\n` formatting with each displayed value
single-quoted as data. It does not join or reparse a shell command string.
The function body runs in a subshell. Before matching, alias-safe POSIX
special-builtin syntax removes inherited `command` and `return` functions only
inside that subshell; failure exits before the bound runtime. Subsequent
`\command test` and `\command printf` calls therefore cannot be redirected by
caller-defined `command`, `return`, `test`, or `printf` functions and aliases,
while the caller's functions remain unchanged after invocation.
Before defining the ordinary function, the generated material removes an
existing alias with that exact command name. Alias expansion otherwise changes
the parsed function definition and also wins later command resolution. This is
an explicit in-memory consequence of caller-owned sourcing; the material does
not edit shell startup files or attempt to restore an alias. This preamble
requires `unalias` to retain its standard POSIX meaning. A caller-defined
`unalias` function is outside the supported activation environment because no
portable top-level mechanism can both bypass and preserve it before parsing
the following function definition.

A matching view returns complete deterministic text directly from the shell
function. The output names the full bundle digest so it describes one exact
rendered artifact, not current source readiness. It starts no bound `atr`,
source, or processor process. POSIX does not require the formatting utility to
be implemented in-process, so this is not a generic zero-OS-process claim.

Every nonmatching argv list, including excluded and unknown help-shaped
selectors, falls through unchanged to contract-2 `wrapper run`. Existing
resolution then returns `command_not_in_surface` for an excluded cataloged
command or `invalid_invocation` for an unresolvable shape before source start.
There is no source-help fallback.

The source-catalog, specification, bundle, plan, processor-observation, agent
help, wrapper review-envelope, and evidence aggregate output shapes do not
change. The wrapper binding/material contract changes from 1 to 2. The
schema-2 wrapper review envelope reports contract version 2; its nested
`wrapper-contract` field inventory remains schema 1 because its shape stays
`version` plus `shell`. Installed artifact evidence advances separately to
record the new proof. `wrapper run` remains exclusively fresh-plan-authoritative
for the non-help path.

## Consequences

### Positive

- The ordinary wrapper becomes a self-discoverable purpose-specific CLI.
- Help success has a mechanically enforced zero-`atr`, zero-source, and
  zero-processor boundary.
- Surface discovery cannot expose excluded source help or acquire ambient
  plugin behavior after rendering.
- The generated bytes and digest review both help and execution routing.
- No coding-agent host, vendor hook, PATH lifecycle, or runtime LLM enters the
  product model.

### Negative

- The fixed POSIX template becomes larger and owns a small exact help dispatch
  grammar.
- Help describes the exact rendered bundle even if later source, processor, or
  adoption state changes. Execution still revalidates all of that state; help
  must not claim current executability.
- A larger future surface may exceed the bounded generated-material contract
  and will require an explicit successor material form rather than truncation.
- `-h`, source-specific `help` subcommands, no-argument aliases, global
  options, and positional syntax remain unsupported.

## Mechanical enforcement

- A pure domain compiler and validator rederive every help view and effective
  option from `tailoringbundle.Bundle`; the binding validates that projection
  against the same bundle.
- Contract tests cover root, namespace, exact command, exact command that is
  also a namespace, option inherit/exclude, stable ordering, and explicit
  bounds.
- POSIX tests execute the generated function in `/bin/sh`, compare exact help
  bytes, prove punctuation remains literal, prove caller-defined
  `command`/`return`/`test`/`printf` functions and aliases cannot intercept
  fixed dispatch, and
  prove every non-help argv is forwarded unchanged.
- Hidden and unknown selectors produce no static help and retain existing
  structured fail-closed runtime faults with zero source attempts.
- Installed-artifact journeys source the exact generated bytes and prove root,
  namespace, and exact help without changing the source-fixture attempt log.
- Documentation, catalog help, wrapper contract, and release evidence agree on
  contract version 2.

## Security and public-boundary impact

The change adds no credential, persisted state, network destination,
third-party dependency, source process, processor process, or host protocol.
Catalog summaries and specification reasons remain untrusted printable data;
their existing validators reject controls, format characters, and Unicode line
separators, while fixed single-quoted `%s` rendering prevents shell or format
interpretation. Printable prompt-like meaning is preserved rather than
filtered.

Static help is not executable attestation, source authorization, containment,
or current-state reconciliation. Caller-owned activation and any later
modification of the function remain outside Atsura's integrity claim.

## Compatibility and migration

New wrappers render as contract 2 and call `wrapper run` with
`--contract-version=2`. Contract-1 runtime invocations are rejected rather than
reinterpreted. An old wrapper remains closed over the old exact `atr` identity;
users obtain the new behavior by rendering a new wrapper. No persisted wrapper
state is migrated because Atsura still does not install wrappers.

## Validation

- focused domain, application, POSIX renderer, CLI, and artifact-journey tests
- `task check`
- `task security`
- `task public:check`
- `task release:check`
- native exact-artifact matrix on every claimed target

## Reconsideration signals

Supersede this ADR when a validated multi-command surface cannot fit the
bounded fixed material, another portable material form is accepted, or user
evidence shows that current-state help validation is more important than
artifact-local zero-`atr` discovery. Do not route help through a source plan as
a local workaround.
