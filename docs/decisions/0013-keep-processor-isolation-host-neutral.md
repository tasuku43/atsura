# ADR 0013: Keep processor isolation host-neutral

- Status: Accepted
- Date: 2026-07-22
- Deciders: Repository maintainer and product owner
- Scope: External-processor environment, coding-agent-host boundary,
  compatibility, security, and release evidence
- Supersedes: The `CLAUDE_CONFIG_DIR` environment clause of ADR 0012
- Superseded by: None

## Context

ADR 0008 keeps coding-agent hosts outside Atsura. ADR 0012 nevertheless put a
Claude-specific environment variable into the first RTK process environment
after observing that RTK checks Claude hook state during startup. That stopped
one observed warning, but it made the lower processor boundary name and model a
coding-agent host.

The process runner already rejects the caller's ambient environment and gives
RTK fresh private `HOME`, XDG, temporary, state, and application-data roots.
Consequently, an ambient `CLAUDE_CONFIG_DIR` is not inherited and the isolated
home cannot expose the caller's default host configuration. No host-specific
redirect is needed to close the observed path.

## Decision drivers

- Atsura is called by maintainers and agent environments; it does not operate
  or configure those environments.
- Processor isolation must deny ambient state by a generic boundary rather
  than encode every host that a processor happens to recognize.
- An environment-contract identifier must change when its exact environment
  changes so an old observation or bundle cannot silently acquire new
  semantics.
- RTK-specific controls remain legitimate output-processor compatibility facts;
  coding-agent-host controls do not.

## Considered options

### Keep the Claude-specific redirect

This preserves the original observation literally, but leaks host concerns
into a lower reusable process boundary and requires similar exceptions for
every future host recognized by a processor.

### Use only generic isolation and processor-owned controls

Scrub all ambient variables, provide private generic roots, and pass only the
finite RTK controls needed by the accepted processor tuple. Reject the tuple if
that boundary cannot contain a future host-specific interaction.

### Inherit the caller environment

This would expose credentials, configuration, telemetry state, and
nondeterministic behavior to an untrusted processor and is incompatible with
the security model.

## Decision

Adopt generic isolation and processor-owned controls. The exact environment is
identified as `atsura.processor.rtk_isolated.v2`. It contains fresh private
generic roots, finite locale/time variables, OS-required Windows variables,
and the accepted `RTK_*` controls. It contains no Claude, Codex, or other
coding-agent-host variable, and inherits no caller credential or host setting.

`atsura.processor.rtk_isolated.v1` is retired. Processor observations, bundles,
and execution requests carrying v1 fail compatibility validation; they must be
re-inspected and rebuilt under v2.

## Consequences

### Positive

- The process boundary now matches the product thesis and ADR 0008.
- One generic denial rule covers current and future caller-owned host state.
- Old observations cannot be mistaken for evidence of the revised environment.

### Negative

- Existing local processor observations and derived bundles must be rebuilt.
- A future RTK release that consults host state outside the isolated generic
  roots cannot be admitted merely by adding another host variable.

### Risks and mitigations

- RTK may add a new path that bypasses the generic roots. Native inspection and
  runtime evidence require empty stderr and bounded deterministic output; a
  regression fails the tuple and triggers compatibility review.
- Windows may require additional OS variables. Only variables required to
  start the native process may be added, with native tests and without agent-
  host semantics.

## Mechanical enforcement

- `processorprocess.EnvironmentRTKIsolatedV2` is the only accepted request and
  compatibility value; explicit negative tests reject v1.
- The process-runner test compares the complete POSIX environment-key set and
  proves ambient `CLAUDE_CONFIG_DIR`, `PATH`, and a secret canary are absent.
- Architecture and catalog tests continue to reject coding-agent-host fields,
  commands, and adapters from the core surface.
- Native artifact replay inspects and runs the official RTK artifact under v2.

## Compatibility and migration

There is no command-shape change. Processor-observation and bundle schemas keep
their versions because the environment value is itself versioned. Users must
run processor inspection again, regenerate the specification default if used,
rebuild the bundle, review it, and adopt the new exact digest.

## Security and public-boundary impact

The change removes a host-specific state name from production execution. It
adds no credential, network destination, executable, or persisted data. RTK
remains an untrusted pinned processor with the same identity, argv, time, byte,
and no-shell bounds.

## Validation

- `go test -race ./internal/domain/processorprocess ./internal/app/processorcompat ./internal/infra/processorexec ./internal/infra/rtkprocessor`
- `task security`
- native exact-artifact replay on every claimed target

## Reconsideration signals

Supersede this decision only if a verified processor cannot be safely admitted
through generic isolation and the product thesis is explicitly revised to own
the additional boundary. A processor's incidental awareness of an agent host
is not sufficient.
