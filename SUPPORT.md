# Support

Atsura is pre-release and maintained on a best-effort basis. Support currently
covers source inspection through the registered GitHub CLI adapter, schema-3
tailoring specification creation and validation, schema-2 bundle compilation,
exact-digest bundle status and interactive adoption, repository development,
zero-execution schema-2 wrapper-plan preview, the first narrow adapter-admitted
typed JSON transform runtime, host-neutral POSIX wrapper rendering/invocation,
and the verification harness.

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
- For wrapper problems, include the platform, contract version, rendered-source
  digest, and stable fault code. Do not attach confidential bundle content,
  source output, environment values, or a modified sourced function.

## Current boundary

The supported tailoring boundary is the artifact workflow:

```text
atr source inspect
atr spec init / spec validate
atr bundle build / bundle status / bundle trust
atr bundle preview --bundle <path> -- <source-executable> <argv>
atr bundle execute --bundle <path> -- <source-executable> <argv>
atr wrapper render --bundle <absolute-path> [--format text|json]
atr wrapper run --contract-version=2 --bundle=<absolute-path> \
  --bundle-digest=<sha256> --runtime-path=<absolute-atr> \
  --runtime-sha256=<sha256> --runtime-size=<bytes> -- <argv...>
```

Specification schema 4 composes command and option membership independently
from identity or transforming wrapper behavior. Bundle schema 3 binds the exact
source, catalog, normalized specification, and compiled surface. `bundle trust`
means user adoption of one exact purpose-specific bundle; it is not source
authorization.

`spec init` intentionally emits an identity-wrapper authoring baseline. Exact
agent help for `source inspect`, `spec init`, and `spec validate` describes the
catalog fields and finite schema-4 transform grammar needed to select observed
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
limited to schema-4 JSON transform wrappers for GitHub CLI adapter contract 2,
GitHub CLI major 2, and exact commands `issue list` and `pr list`. The adapter
must admit the complete argv and exact inline ordered `--json` selector before
one bounded source attempt. Competing `--jq`, `--template`, and `--web` modes,
unmodeled positional or option syntax, and unsupported wrapper/adapter/version
combinations fail before start. Successful stdout is still parsed and
transformed strictly; raw stdout, stderr, and unselected fields are not returned
or persisted.

On Linux and macOS, `wrapper render` produces a fixed POSIX function only when
the exact adopted bundle exposes one completely runtime-admitted command and
result mode. Contract 2 embeds the exact bundle digest and current absolute
`atr` identity. Root, included-namespace, and included-command final `--help`
views are compiled from the bundle and start no bound `atr`, source, or
processor; every other argv list is forwarded unchanged after the required
`--` to `wrapper run`. `wrapper run` rebuilds the same fresh plan and emits only
its declared compact JSON, exact bounded source-stream, or finite
original-preserving optimizer result. Activation and later modification of the
function are caller-owned. Windows returns the structured
`wrapper_platform_not_supported` fault and has no POSIX activation claim.

The runtime binding detects cooperative drift after the bound `atr` path starts;
it is not attestation or a sandbox against malicious replacement at that path.
Coding-agent hosts remain external callers and no vendor hook, settings,
permission, or process integration is part of support.

Source refresh, raw bypass, history, arbitrary external transformers,
additional runtime adapter contracts, persistent wrapper installation or
executable/PATH shims, and published release installation are not current
capabilities. The only external optimizer contract is the exact maintained
RTK v0.43.0 strict Go-test-pass tuple; it does not generalize to arbitrary RTK
programs or filters.
Coding-agent-host configuration, hooks, permissions, and
lifecycle are outside Atsura rather than deferred capabilities. Retired policy
schemas 1 and 2, bundle schema 1, legacy `plan preview`, and `run` are supported
only by explicit zero-execution migration diagnostics and are not automatically
converted.

Before requesting help, read [README.md](README.md), [AGENTS.md](AGENTS.md), and
the [documentation map](docs/README.md), then run the smallest relevant
verification profile.
