# Work Context: Host-mediated tailored invocation

This file records verified facts and unresolved questions. A desired host design is not current behavior.

## Current behavior

- The final direct-path milestone is committed at `7426a99b7c3c6f66cf7ceb6b2691f138d66d7b2b`; GitHub Actions run `29870606690` passed the implementation and boundary gates, five native exact-artifact journeys, and the dependent aggregate on that exact revision.
- `atr bundle preview` and `atr bundle execute` expose the current direct maintainer path. There is no installed host adapter, host settings mutation, or ordinary coding-agent command interception path.
- The shared theses already define host adapters as translations of core states rather than source-operation authorization. Claude Code is named only as a possible adapter, not as a core dependency.
- GitHub CLI adapter contract 2 admits only `issue list` and `pr list` with the finite JSON transform runtime. That bounded source contract remains the source fixture for this iteration.

## Relevant structure

- Entry point: `cmd/atr/main.go`
- Domain rule: `internal/domain/tailoringbundle`, `tailoringplan`, `runtimeadmission`, and `operation`
- Application use case: `planpreview`, `bundleexecute`, `bundleauthority`, and `execution`
- Infrastructure boundary: no host package exists; source boundaries are `githubcli`, `sourceexec`, and `sourcejson`
- CLI catalog or presentation: `internal/cli/catalog.go`, command handlers, and exact agent help
- Existing tests and harness checks: production recovery matrix, artifact journey, five-target CI aggregate, and the four canonical gates

## Constraints

- The product remains vendor-neutral. A host adapter may contain vendor fields, but shared domain, bundle, specification, plan, and execution result schemas may not.
- Source CLI semantics, authentication, authorization, prompts, and downstream effects remain source-owned.
- Host input, working directory, environment, shell-like text, and settings are untrusted. No host payload may become an authentication binding or bundle-adoption receipt.
- Repository-provided configuration cannot silently install, select, or adopt an integration.
- Routine execution remains deterministic and does not require a language model, arbitrary shell evaluation, or an external parser.
- The selected mechanism must state its platform and shell coverage exactly; unsupported paths fail explicitly.

## External facts

Research baseline: 2026-07-22. Current web documentation without a stable
version is treated as a moving host contract; RTK claims below are pinned to
the stable `v0.43.0` tag rather than its newer prerelease or default branch.

### Claude Code

- The current [hooks reference](https://code.claude.com/docs/en/hooks) says
  `SessionStart` can persist environment changes for later Bash calls through
  `CLAUDE_ENV_FILE`, but it cannot block session startup. Command hooks with an
  `args` field use exec form and do not invoke a shell.
- `PreToolUse` receives a Bash `command` as one shell string and can replace the
  complete tool input through `updatedInput`. Its `allow`, `ask`, `deny`, and
  `defer` values are host permission transport; matching hooks run in parallel,
  and the documented `if` filter is best-effort rather than an authorization
  boundary. A deterministic Atsura rewrite cannot rely on that filter to parse
  or prove arbitrary shell syntax.
- `PostToolUse` can replace what Claude receives only after the source tool has
  run; the original result may already have reached host telemetry. It cannot
  implement Atsura's pre-start surface or zero-attempt boundary.
- Hooks can live in user, project, local, managed, or plugin scope. The current
  [plugin reference](https://code.claude.com/docs/en/plugins-reference) provides
  host-owned install and uninstall commands, a plugin-local hook file, and
  session-local `--plugin-dir`. The docs do not promise that a plugin `bin/`
  entry shadows an existing source executable, so precedence remains a runtime
  observation rather than a contract.
- Anthropic's [security guidance](https://code.claude.com/docs/en/security)
  treats hooks and plugins as arbitrary user-privileged code. An Atsura adapter
  must therefore install only deterministic generated content, use exact paths,
  and not inherit the host's general shell-hook power into the bundle schema.

#### Bounded runtime observation

An isolated 2026-07-22 probe used the official npm distribution of Claude Code
`2.1.217` on `darwin/arm64`. The platform binary SHA-256 was
`5840c777fd47115e9ca276e165563c6e121e7c7e2b4d86598e0025f8cc37de56`;
Apple code-signature verification identified Anthropic PBC and passed. The
probe removed all real authentication environment variables, isolated `HOME`,
`CLAUDE_CONFIG_DIR`, npm cache, and temporary files, disabled nonessential
traffic and update paths, and ended with `Not logged in`, zero API duration,
zero tokens, and zero cost.

Before that unauthenticated exit, an exec-form user-settings `SessionStart`
hook ran and appended one fixed export to the host-provided
`CLAUDE_ENV_FILE`. A strictly validated local plugin loaded with `--plugin-dir`
also ran its exec-form `SessionStart` hook and resolved
`${CLAUDE_PLUGIN_ROOT}` to the supplied plugin directory. This confirms hook
loading and environment-file write timing for one exact host artifact without
claiming a successful Bash tool cycle.

Still unverified: whether the appended environment reaches later Bash calls or
command hooks, `PreToolUse.updatedInput` behavior, matching-hook interaction,
persistent plugin installation/removal, settings precedence, interactive UI,
other Claude versions, and other platforms.

#### First-host scorecard

| Candidate | Ordinary invocation | Native argv | Broken activation | Host permission semantics | Initial assessment |
|---|---|---|---|---|---|
| Claude `SessionStart` plus PATH only | Complete after activation | Preserved | Silent fallthrough | Preserved | Reject alone |
| Claude `PreToolUse.updatedInput` | Complete for parsed rewrites | Shell string must be interpreted | Can block | `allow` or `ask` is coupled to the documented rewrite | Fallback only |
| Claude `SessionStart` PATH plus `PreToolUse` health guard | Complete after activation | Preserved | Guard can deny while unhealthy | Original command and normal flow preserved while healthy | Leading prototype |
| Gemini `BeforeTool` plus policy | Complete for host shell tool | Shell string must be interpreted | Non-blocking hook errors need separate policy | Vendor policy is central | Later host candidate |
| Host-independent PATH activation | Complete only in launched environment | Preserved | Silent fallthrough | No host vocabulary | Direct-path complement |

The leading hybrid does not rewrite a healthy tool input. `SessionStart` makes a
native shim directory visible; a synchronous exec-form `PreToolUse` hook checks
only integration health and returns no decision while healthy. On missing or
drifted activation it uses host `deny` as an adapter transport failure, not a
source-operation judgment. A conservative implementation may block all Bash
tool calls while the explicitly installed integration is unhealthy, avoiding
the need to parse arbitrary shell text in the guard.

This candidate remains provisional until a zero-cost tool-cycle probe proves
that the environment and guard observe one coherent session. Absolute source
paths, disabled hooks, or an uninstalled plugin remain routes outside the
tailored surface; the adapter must describe them as out of scope rather than
claiming sandbox isolation.

### RTK

- Stable [RTK v0.43.0](https://github.com/rtk-ai/rtk/releases/tag/v0.43.0)
  is an Apache-2.0 Rust binary focused on reducing coding-agent CLI output. Its
  [architecture](https://github.com/rtk-ai/rtk/blob/v0.43.0/docs/contributing/ARCHITECTURE.md)
  uses host rewrite adapters, command-specific filters, source structured-output
  flags, and a finite TOML output pipeline. This is strong evidence for a thin
  host adapter and a first-class typed output stage.
- The tagged implementation has 76 static rewrite rules and 63 built-in TOML
  filters; its public claim of 100+ commands is a product claim, not independent
  coverage evidence. Unsupported commands and parser/filter failures fall back
  to raw output, and a never-worse guard may choose raw when filtering is larger.
  Those choices fit output optimization but cannot implement Atsura surface
  absence or control-policy failure.
- `rtk pipe` is a useful future adapter seam, but its current interface accepts
  one bounded stdin blob and does not carry separate stderr, the source exit
  status, source identity, or Atsura plan identity. `rtk run` invokes a shell.
  Neither is a production replacement for Atsura's controlled source boundary.
- RTK's current tracking code can retain full original/rewritten command strings
  and canonical project paths in local SQLite for 90 days, and its failure tee
  can retain raw failure output. Atsura cannot enable those behaviors implicitly
  because its current contract does not persist commands containing secrets or
  raw confidential output.
- Decision: preserve an independent Atsura catalog, surface, plan, identity, and
  execution core. Reconsider an optional, explicitly pinned RTK output adapter
  only after a bounded prototype can preserve source status, no-secret storage,
  plan attribution, limits, and non-fallback semantics.

### Representative adjacent tools

- [Gemini CLI's policy engine](https://geminicli.com/docs/reference/policy-engine/)
  and [hooks](https://geminicli.com/docs/hooks/reference/) already provide
  vendor-specific tool filtering, allow/deny/ask decisions, input replacement,
  and output replacement. Its shell tool and command hooks still execute shell
  strings. Atsura should translate its host-neutral states through a thin Gemini
  adapter later, not recreate Gemini permission semantics in the core.
- [just](https://just.systems/man/en/) can present a manually authored small CLI,
  parameters, dependencies, private recipes, and dry-run text, but recipes are
  shell or arbitrary interpreter programs and provide no inspected catalog,
  adopted bundle, exact source identity, or canonical plan digest.
- [Nushell custom commands](https://www.nushell.sh/book/custom_commands.html)
  and [externs](https://www.nushell.sh/book/externs.html) demonstrate a powerful
  typed wrapper and structured pipeline, but as a general programming language
  they are deliberately broader than Atsura's finite declarative core.
- [jc](https://kellyjonbrazil.github.io/jc/) demonstrates broad conversion of
  existing command output into JSON. Human-output parsing remains version and
  locale sensitive, and local parsers are arbitrary Python modules; it is a
  possible versioned source-output adapter, not a policy runtime.

### Research consequence

Adjacent products already solve host-specific permission hooks, manual shell
facades, general wrapper languages, and broad output parsing. Atsura's distinct
integration remains the evidence-to-adoption-to-surface-to-canonical-plan chain,
exact source identity, deterministic typed execution, host-neutral adapters, and
an explicit identity-bound raw bypass. The first host slice must reuse host
installation and lifecycle facilities where they have a sufficient contract,
while retaining Atsura's runtime and trust authorities.

### Product direction after research

The product owner prefers RTK-supported commands to use an RTK-backed default
wrapper configuration. Here, **default** means a compile-time candidate or
authoring default that is explicit in the reviewed specification, canonical
bundle, and complete plan. It never means runtime detection, implicit process
insertion, or fallback after another stage fails.

The current RTK interface is not yet sufficient to claim that default. A later
bounded RTK iteration must decide the exact supported subset and prove RTK
binary/version identity, filter identity, separate source status and stderr,
finite I/O, disabled command tracking and raw-output tee, no project-filter
inheritance, and no raw fallback. Commands outside that proven subset receive
an explicit built-in or identity candidate instead of silently using RTK.

This host iteration deliberately reuses the current built-in JSON transform so
host-transport evidence is not confounded with a new external output stage.
The selected host adapter may consume only the exact adopted wrapper plan; it
must not detect RTK support or choose a backend at invocation time.

## Unknowns

- [ ] Which current coding-agent host offers the smallest complete path for interception, rewrite, recovery, and safe installation? The Claude `SessionStart` PATH plus `PreToolUse` health-guard hybrid is the leading candidate; one isolated lifecycle probe passed, but a later tool cycle and persistent lifecycle remain unverified.
- [x] For Claude Code specifically, what are the current and distinct roles of `SessionStart`, `PreToolUse`, and other hook events?
- [x] Can a host replace exact tool input, or does Atsura need a PATH wrapper, shell function, or another adapter-owned transport? Claude can replace Bash input, but a PATH shim preserves native argv without interpreting an arbitrary shell string and is the leading transport.
- [ ] Which settings scope can Atsura own without overwriting repository or user configuration it did not create?
- [x] How do RTK and existing wrappers already shorten output, rewrite commands, preserve raw access, or integrate with agent hosts?
- [ ] Which host response values represent transport decisions, and which core state must produce each value without importing authorization semantics?
- [ ] Which supported platforms can run the selected host mechanism with native installed-artifact evidence?
- [ ] Can a generated Claude plugin and `SessionStart` activation fail visibly enough despite `SessionStart` being non-blocking, or must a second host guard close that usability gap?
- [ ] Does Claude's plugin `bin/` directory reliably precede an existing source binary on every claimed platform, and what happens after plugin reload or concurrent-version update?
- [ ] Does a `CLAUDE_ENV_FILE` export reach both later Bash commands and later command hooks in the same session on each claimed platform?

## Long-horizon coverage baseline

| Axis | Proven now | Remaining target |
|---|---|---|
| Source adapters | GitHub CLI contract 2 | At least one materially different source CLI |
| Host adapters | None | Two independent host paths plus the direct path |
| Surface states | Include, exclude, option surface, ambiguous invocation | Ordinary host discovery and routing |
| Wrapper runtime | JSON transform with appended argv | Identity, argv-only, broader argv grammar, typed before/after |
| Output | Select, rename, compact JSON | RTK-backed default candidate for its proven subset, plus finite built-ins where RTK cannot satisfy the plan contract |
| Lifecycle | Build, exact adoption, status, drift rejection | Profiles, refresh, material diff, re-adoption, explicit raw |
| Agent role | Exact help can guide a maintainer or agent | Untrusted candidate proposal with no implicit activation |
| Orthogonality | Vendor-neutral shared types, one source fixture | Two source by two host fixture matrix with no vendor leakage |

## Thesis evidence

- Repeated design decision or point of agent confusion: a host protocol's permission vocabulary is easy to mistake for Atsura source-operation policy.
- User outcome or friction observed in the minimal slice: the current transform succeeds only when a maintainer explicitly calls `atr bundle execute`; the coding agent does not encounter the tailored CLI in its ordinary workflow.
- Code workaround or exception being considered: directly embedding Claude hook payloads in core types or installing a shell snippet before deciding the host boundary.
- Current thesis that resolves it, or proposed thesis revision: host transports translate core states and cannot create a second surface or wrapper model.
- Downstream product, architecture, security, Skill, catalog, and harness impact: expected across all of them once research chooses the first host contract.

## Reproduction or observation

```sh
atr bundle execute --bundle <bundle.json> -- gh pr list --limit=1
```

Expected and observed direct-path behavior is already covered by exact-artifact fixtures. No equivalent ordinary host-mediated invocation exists yet.

The isolated Claude lifecycle probe used its official `2.1.217` native package,
a synthetic settings/plugin tree, empty authentication variables, `--tools ''`,
`--no-session-persistence`, and JSON output. Both hook variants wrote only a
synthetic marker; the host made no model request and all temporary state was
removed after observation.

## Security and public-boundary notes

- Assets and side effects involved: selected host settings, Atsura ownership markers, bundle selection metadata, and child process attempts.
- Credentials or confidential data involved: host payloads and source output may contain confidential text; no credential is needed by the planned fixtures and none may be persisted.
- New dependencies, destinations, files, processes, or generated content: unknown until primary-source research selects a mechanism; provider-network-free fixtures remain required.
- External schema provenance, publication rights, and drift evidence: official host schemas must be version-bound and represented by synthetic publishable fixtures.
- Output delivery, collection coverage, pagination, timeout, retry, idempotency, and cancellation facts: the current source attempt remains at most one; host installation and removal require idempotent reconciliation and uncertain-outcome handling.
- Publication and licensing concerns: dependencies and copied schema examples require license and provenance review before adoption.

## Glossary

- **Host-mediated invocation:** an ordinary coding-agent attempt translated by an adapter into an Atsura core state and, when managed, the existing deterministic runtime.
- **Not managed:** the selected integration deliberately does not own the attempted invocation; it is distinct from surface absence and source authorization.
- **Ownership marker:** exact metadata proving which bounded host configuration fragment Atsura may reconcile or remove.
