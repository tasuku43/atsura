# Security Policy

Atsura is pre-release. Its current tailoring boundary can execute one exact
bundle-backed source invocation and transform bounded successful JSON. Atsura
does not classify that source operation as read-only; the source CLI remains
authoritative for its authentication, authorization, semantics, and effects.
The exact controls and limitations are recorded in
[the Security Model](docs/03_security_model.md).

## Supported versions

| Version | Security support |
|---|---|
| `main` | Supported for repository and pre-release development fixes |
| Releases | No Atsura release exists |

This table must be revised before the first release.

## Report a vulnerability

Use GitHub's private vulnerability reporting flow from this repository's
**Security** tab. If that flow is unavailable, email
[task.teckac@gmail.com](mailto:task.teckac@gmail.com).

Do not include vulnerability details, credentials, private URLs, raw
confidential CLI output, or personal data in a public issue. A public issue may
ask how to contact maintainers but must contain no sensitive evidence.

Include, when possible:

- affected revision and platform;
- preconditions and impact;
- a minimal synthetic reproduction;
- whether secrets, source-CLI accounts, or user data may be affected; and
- any known mitigation.

## Current security principles

- Commands, arguments, executable resolution, help output, generated catalogs,
  tailoring specifications, bundles, wrapper bindings, and source output are
  untrusted.
- Repository-provided configuration is not automatically user-adopted.
- A controlling bundle, specification, or source-identity decision that cannot
  be evaluated must produce no source-process attempt.
- A tailoring specification does not accept arbitrary shell code.
- Process execution uses an exact executable and argv vector without shell
  interpolation, with one direct attempt and fixed time and byte bounds.
- Output optimization failure must not change argv, run a different command,
  repeat execution, or select raw execution automatically.
- Atsura acquires and stores no OAuth token, PAT, provider credential, usage
  history, or raw confidential source output. A source process still inherits
  the caller's environment.
- A future raw route must be explicit and must state that tailoring is bypassed.

The inherited harness checks architecture, repository hygiene, secrets,
contracts, and public boundaries. Those checks bound the current synthetic
local-run outcome; they do not prove a particular vendor source CLI, external
activation mechanism, mutation capability, or extension mechanism is secure.

Coding-agent hosts and their hooks, settings, permissions, sessions, and tool
request protocols are outside Atsura. External consumers may expose an
Atsura-generated wrapper, but Atsura does not parse or rewrite their requests
or manage their configuration.

## Out of scope

Atsura cannot protect a compromised developer machine, maintainer account,
source CLI, provider account, CI platform, or external service. It cannot infer
that an agent proposal represents human authorization or that a source command
is safe from its name or help text.
