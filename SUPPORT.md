# Support

Atsura is pre-release and maintained on a best-effort basis. Source-CLI
tailoring is not implemented yet, so support currently covers repository
bootstrap, the inherited executable scaffold, documented product decisions, and
the verification harness.

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

The project does not yet support a real source CLI, policy file, wrapper, hook,
output transformation, history integration, RTK integration, or release
installation. Requests in those areas are product proposals, not support
requests for an existing capability.

Before requesting help, read [README.md](README.md), [AGENTS.md](AGENTS.md), and
the [documentation map](docs/README.md), then run the smallest relevant
verification profile.
