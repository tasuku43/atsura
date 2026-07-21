# ADR 0004: Compile one trusted tailoring bundle for every adapter

- Status: Accepted
- Date: 2026-07-21
- Deciders: Repository maintainer and product owner
- Scope: Product, CLI, catalog inspection, policy trust, execution, host integration, security, compatibility, and release quality
- Supersedes: None
- Superseded by: None

## Context

The v0.1 `plan preview` and `run` slice proves deterministic matching, one
bounded source attempt, and typed JSON reshaping. It still makes the maintainer
select one YAML file for every invocation and gives a coding agent no tailored
discovery surface. Catalog generation, persistent trust, confirmation, raw
execution, drift handling, and host setup are only planned vocabulary.

Three interface concepts were reviewed:

1. require every caller to use an explicit Atsura gateway;
2. transparently rewrite source commands in one coding-agent hook; or
3. compile source evidence and reviewed policy into one versioned bundle which
   both gateways and host adapters consume.

Current Claude Code hooks can decide and update a Bash tool input before
execution, while post-use output replacement happens after effects. RTK shows
that transparent Bash rewriting is usable, but its product center is
command-specific output reduction. Source help is not a uniform trusted API:
Git may include aliases and PATH commands, kubectl discovers arbitrary plugin
executables, and local Docker help may query CLI plugins.

## Decision drivers

- Make preview, manual execution, and host interception apply identical facts.
- Keep host-specific protocols outside policy and execution semantics.
- Let a maintainer inspect, trust, update, diagnose, and remove the complete
  local installation.
- Bind permission to reviewed content rather than a moving repository path.
- Preserve an explicit gateway for testing and recovery without making it the
  routine agent syntax.
- Treat observed source capabilities as evidence, never authorization.
- Define a finite v1 compatibility claim rather than claim every possible CLI.

## Decision

Choose the compiled tailoring bundle concept.

### Canonical bundle

`atr bundle build` creates one deterministic, immutable JSON document from:

- the resolved source executable identity and exact observed version;
- inspector kind and inspector contract version;
- a provenance-bearing generated command catalog;
- normalized typed policy;
- the catalog and policy SHA-256 digests; and
- the resulting tailored agent surface.

The semantic document contains no build timestamp, hostname, username, random
identifier, credential, or source output. Its SHA-256 digest is its identity.
Preview, controlled execution, raw execution, status, and every host adapter
load and revalidate that same bundle. No adapter recompiles policy.

### Trust

A bundle stored in a repository is untrusted data. `atr bundle trust` shows the
source identity, visible capabilities, decisions, effects, mutation impacts,
and bundle digest, then requires an interactive terminal confirmation. It
writes a user-local trust receipt keyed by the exact bundle digest. An agent or
repository file cannot create that receipt through redirected stdin.

Changing the bundle bytes, source executable, observed version, catalog, or
policy invalidates the receipt. Trust does not transfer automatically to a
refreshed bundle. Trust receipts contain no credentials or source output and
are never repository state.

### Vendor-neutral core and adapter compatibility

Atsura's domain, policy, catalog, bundle, plan, and execution contracts do not
name a source-CLI vendor or a coding-agent vendor. Source inspection is a port
implemented by capability-specific adapters. Coding-agent integration is a
separate port implemented by host-protocol adapters. Neither adapter kind may
add policy semantics, broaden permission, or create a second bundle format.

An adapter is selected by a stable, namespaced kind and contract version in the
bundle. Unknown kinds fail closed. The core validates the adapter-produced
catalog through the same vendor-neutral domain invariants used for every
source. At least one synthetic alternate adapter must pass the core contract
suite so that GitHub- or Claude-shaped assumptions cannot enter shared types.

### First reference adapters and compatibility

The first real source inspector adapter is `github_cli_v1`. It accepts a resolved GitHub CLI
whose parsed major version is 2 and whose fixed probe outputs satisfy the
driver grammar. It invokes only a finite declared probe set under per-process
and aggregate time/byte/attempt bounds. The primary reference probe is
`gh help reference`; commands declaring native `--json fields` may receive a
bounded field-discovery probe. Inspection never invokes a provider task.

Catalog entries classify provenance as:

- `verified_builtin`: accepted by the named inspector grammar;
- `observed_extension`: alias, plugin, or other external capability observed
  but not granted built-in compatibility; or
- `unverified_dynamic`: structure or behavior the inspector cannot validate.

Only `verified_builtin` entries can become a controlled v1 policy rule.
Extensions and dynamic entries remain visible as evidence and fail closed.
The executable hash and exact version in each bundle are always exact even
though the driver accepts structurally valid GitHub CLI 2.x instances.

The required compatibility corpus is a synthetic GitHub CLI 2.72 fixture plus
additive, hostile, malformed, oversized, plugin, version-drift, and
executable-drift variants. A local installed `gh` inspection is a documented
manual smoke test, not a network or authentication gate.

### Public workflow

The v1 task surface is:

```text
atr source inspect
atr source refresh
atr policy init
atr policy validate
atr bundle build
atr bundle trust
atr bundle status
atr plan preview
atr plan explain
atr run
atr raw
atr integration claude-code install
atr integration claude-code status
atr integration claude-code remove
atr hook claude-code
```

Every state-changing local command is create-only or exact-owner-scoped. An
existing destination is never overwritten implicitly. Refresh produces a
comparison and a new catalog destination; it does not mutate a trusted bundle.

The explicit gateway is `atr run --bundle <path> -- <source argv>`. The routine
Claude Code experience keeps the original source command syntax. The adapter
uses `SessionStart` only to publish the tailored discovery surface and
`PreToolUse` to parse, decide, request confirmation, deny, or rewrite a simple
source invocation to the gateway. Unsupported compound shell syntax involving
a managed source fails closed. `PostToolUse` is not an authorization or output
secrecy boundary.

`atr raw --bundle <path> -- <source argv>` is a manual, explicit bypass. It
still binds and revalidates the bundle's source identity but applies no policy
or output transform. It is never emitted as a recovery action and is absent
from the tailored agent surface and automatic hook rewrite.

### Confirmation

Read plans may be allowed or denied. Create/write plans require complete target
and impact declarations and may be denied or require confirmation; they cannot
use unconditional `allow` in v1. Manual confirmation requires an interactive
terminal. Claude Code confirmation uses its `PreToolUse` ask decision and a
one-shot plan receipt bound to bundle digest, exact argv, source identity, and
effect. A receipt is consumed once and never turns an uncertain mutation
outcome into retry permission.

### Integration ownership

v1 installs only a project-local Claude Code integration in
`.claude/settings.local.json`. Atsura records and removes only the exact hook
entries it owns, preserves unrelated settings, refuses ambiguous or changed
owned entries, and offers status/recovery instead of broad replacement. The
fixed generated hook command contains no policy-authored shell and delegates
to `atr hook claude-code` on stdin JSON. Global installation and other coding
agents are later host adapters over the same vendor-neutral bundle and plan
contracts.

## Consequences

### Positive

- One reviewed artifact drives every execution path.
- Routine agents retain familiar source CLI syntax.
- Manual preview, execution, and diagnostics remain available without a host.
- Bundle and trust digests make drift and review precise.
- The host adapter can change without changing policy semantics.
- Source-specific inspection is honest about compatibility and extensions.

### Negative

- v1 does not claim tested compatibility for every CLI; it proves one real
  source adapter, one real host adapter, and vendor-neutral shared contracts.
- Interactive trust and confirmation add deliberate user ceremony.
- Simple-command parsing is narrower than arbitrary Bash.
- Repository-local Claude Code setup is the only supported host integration.
- Persisted trust and settings ownership add filesystem and migration risk.

## Mechanical enforcement

- Domain types validate canonical bundle identity, provenance classes,
  source/version/catalog/policy bindings, effects, impacts, and visibility.
- Shared contract fixtures include a synthetic non-GitHub inspector and a
  synthetic non-Claude host consumer so vendor-specific fields cannot leak
  into catalog, bundle, plan, or decision values.
- Inspector fixtures prove the exact probe budget and that malformed,
  extension, dynamic, and drift observations cannot become permission.
- Canonical encoding and round-trip tests prove identical inputs produce
  identical bundle bytes and digests.
- Application fake ports prove trust, deny, confirmation, drift, and invalid
  bundles cause zero source task attempts.
- Filesystem adapters use bounded regular-file operations, create-exclusive
  staging, exact ownership markers, identity revalidation, and rollback tests.
- Hook fixtures cover SessionStart and PreToolUse allow/ask/deny/defer/rewrite,
  compound-command rejection, hostile JSON, and unrelated settings retention.
- End-to-end fixtures cover zero state through inspect, policy, build, trust,
  install, use, drift, refresh, raw, and remove.
- Catalog and agent-readiness tests prove at most two help-discovery calls and
  zero undeclared reconstruction for each supported outcome.

## Compatibility and migration

The current `--config` schema-1 preview/run path remains available during the
v1 development series as an explicitly deprecated local utility. A v1 bundle
does not silently import or trust it. `policy init` may generate a schema-2
draft from a reviewed schema-1 file, but the generated draft must be validated,
built, and trusted as new content.

Bundle, catalog, policy, trust-receipt, and hook protocols are independently
schema-versioned. Unknown major schemas fail closed. Patch-compatible readers
reject unknown security-relevant fields rather than ignoring them.

## Sources reviewed

- Claude Code hooks: <https://code.claude.com/docs/en/hooks>
- RTK repository and integration behavior: <https://github.com/rtk-ai/rtk>
- GitHub CLI formatting: <https://cli.github.com/manual/gh_help_formatting>
- Git help enumeration: <https://git-scm.com/docs/git-help>
- kubectl plugins: <https://kubernetes.io/docs/tasks/extend-kubectl/kubectl-plugins/>

## Reconsideration signals

Create a superseding ADR before allowing a generic help parser to claim
verified compatibility, global host installation, arbitrary shell policy,
automatic raw fallback, non-interactive trust, or trust migration across
changed bundle content. A second source or host adapter does not require a
product thesis change when it satisfies the existing adapter conformance suite
and only extends the explicitly documented compatibility matrix.
