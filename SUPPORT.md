# Support

Atsura is pre-release and maintained on a best-effort basis. Support currently
covers source inspection through the registered GitHub CLI adapter, schema-3
tailoring specification creation and validation, schema-2 bundle compilation,
exact-digest bundle status and interactive adoption, repository development,
zero-execution schema-2 wrapper-plan preview, the first narrow adapter-admitted
typed JSON transform runtime, and the verification harness.

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
- For artifact problems, the schema version and digest fields without
  confidential catalog or source content.
- For runtime problems, adapter contract, source version, matched command,
  plan digest, attempt count, and fault code; never attach raw source output.

## Current boundary

The supported tailoring boundary is the artifact workflow:

```text
atr source inspect
atr spec init / spec validate
atr bundle build / bundle status / bundle trust
atr bundle preview --bundle <path> -- <source-executable> <argv>
atr bundle execute --bundle <path> -- <source-executable> <argv>
```

Specification schema 3 composes command and option membership independently
from identity or transforming wrapper behavior. Bundle schema 2 binds the exact
source, catalog, normalized specification, and compiled surface. `bundle trust`
means user adoption of one exact purpose-specific bundle; it is not source
authorization.

`spec init` intentionally emits an identity-wrapper authoring baseline. Exact
agent help for `source inspect`, `spec init`, and `spec validate` describes the
catalog fields and finite schema-3 transform grammar needed to select observed
JSON fields and declare collision-free renames. The installed `atr` workflow
does not require a source checkout, although editing the reviewed YAML remains
an explicit user or agent configuration action.

`bundle preview` requires that exact adoption and current source path/hash/size,
then returns the complete deterministic plan and digest with
`source_process_attempts: 0`. The current grammar does not completely model
source short options, root/global options, or command-specific positional
arguments. A command with cataloged descendants requires an inner `--` before
otherwise ambiguous positional data; appended argv after `--` stays after that
marker. Preview proves an active selector and planned input format but does not
apply it.

`bundle execute` independently rebuilds that plan. Current runtime support is
limited to schema-3 JSON transform wrappers for GitHub CLI adapter contract 2,
GitHub CLI major 2, and exact commands `issue list` and `pr list`. The adapter
must admit the complete argv and exact inline ordered `--json` selector before
one bounded source attempt. Competing `--jq`, `--template`, and `--web` modes,
unmodeled positional or option syntax, and unsupported wrapper/adapter/version
combinations fail before start. Successful stdout is still parsed and
transformed strictly; raw stdout, stderr, and unselected fields are not returned
or persisted.

Identity-wrapper and argv-only-transform execution, nonempty successful stderr,
source refresh, raw bypass, history, RTK or external transformers, additional
runtime adapter contracts, and published release installation are not current
capabilities. Coding-agent-host configuration, hooks, permissions, and
lifecycle are outside Atsura rather than deferred capabilities. Retired policy
schemas 1 and 2, bundle schema 1, legacy `plan preview`, and `run` are supported
only by explicit zero-execution migration diagnostics and are not automatically
converted.

Before requesting help, read [README.md](README.md), [AGENTS.md](AGENTS.md), and
the [documentation map](docs/README.md), then run the smallest relevant
verification profile.
