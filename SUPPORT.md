# Support

Atsura is pre-release and maintained on a best-effort basis. Support covers the
documented schema-1 `plan preview` and read-only local `run` outcome, repository
development, and the verification harness.

## Where to ask

- Use a GitHub issue for a reproducible bug or focused design proposal.
- Use a pull request for a reviewed change tied to a documented user outcome.
- Follow [SECURITY.md](SECURITY.md) for vulnerabilities or sensitive reports.

Do not place credentials, private URLs, personal data, confidential source-CLI
output, or embargoed details in issues, discussions, or pull requests.

## What to include

- Revision, operating system, architecture, and Go version.
- Exact command, expected result, actual result, and exit status.
- A minimal synthetic reproduction.
- Relevant bounded output with secrets and personal data removed.
- The smallest failing verification profile.

## Current boundary

The supported tailoring boundary requires an explicitly selected policy and a
local JSON-producing source executable. It supports exact prefix matching,
allow/deny, appended arguments, and built-in select/rename output. Source-help
inspection, vendor-specific compatibility, hooks, implicit policy activation,
mutations, confirmation, raw fallback, history, RTK or external transformers,
and published release installation are product proposals rather than existing
capabilities.

Before requesting help, read [README.md](README.md), [AGENTS.md](AGENTS.md), and
the [documentation map](docs/README.md), then run the smallest relevant
verification profile.
