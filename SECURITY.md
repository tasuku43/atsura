# Security Policy

Atsura is pre-release. Its current tailoring boundary can execute one
explicitly configured read-only source command and transform bounded successful
JSON. The exact controls and limitations are recorded in
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
  policies, hook payloads, and source output are untrusted.
- Repository-provided policy is not automatically user-trusted.
- A controlling policy or source-identity decision that cannot be evaluated
  must produce no source-process attempt.
- Arbitrary shell code is not an accepted default policy mechanism.
- Process execution uses an exact executable and argv vector without shell
  interpolation, with one direct attempt and fixed time and byte bounds.
- Output optimization failure must not change argv, run a different command,
  repeat execution, or select raw execution automatically.
- Atsura acquires and stores no OAuth token, PAT, provider credential, usage
  history, or raw confidential source output. A source process still inherits
  the caller's environment.
- A future raw route must be explicit and must state that Atsura policy is
  bypassed.

The inherited harness checks architecture, repository hygiene, secrets,
contracts, and public boundaries. Those checks bound the current synthetic
local-run outcome; they do not prove a particular vendor source CLI, future
hook, mutation policy, or extension mechanism is secure.

## Out of scope

Atsura cannot protect a compromised developer machine, maintainer account,
source CLI, provider account, CI platform, or external service. It cannot infer
that an agent proposal represents human authorization or that a source command
is safe from its name or help text.
