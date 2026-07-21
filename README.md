# Atsura

A deterministic framework for tailoring existing CLIs to coding agents.

Atsura's working hypothesis is that a maintainer can manage per-command YAML
for an existing CLI and give a coding agent a narrower, purpose-specific
surface without reimplementing that CLI. A coding-agent hook intercepts an
attempted command, Atsura compiles it into one inspectable execution plan, and
the same plan logic drives preview or controlled wrapper execution. Routine
planning and enforcement are deterministic and do not require a language model.

## Project status

Atsura has one release-quality local tailoring outcome. `atr run` strictly
loads one explicitly selected schema-1 policy, admits only an `allow` decision
with `effect: read`, starts one bounded source process without a shell, and
returns selected and renamed records from successful JSON output. `atr plan
preview` compiles the same policy and invocation with zero source-process
attempts.

This v0.1 boundary does not inspect source help, install hooks, hide commands,
activate repository policy implicitly, execute mutations, or provide raw
fallback. The command and schema remain pre-release interfaces.

The current `atr` binary still contains the foundry's `doctor` and synthetic
`sample` commands as executable architecture and harness examples. They are
not source-CLI tailoring features.

Project identity:

- Product: `Atsura`
- Binary: `atr`
- Go module: `github.com/tasuku43/atsura`
- License: MIT
- Documentation locale: English

## Product direction

A future tailored surface may narrow visible commands and options, classify
operations, deterministically change arguments or defaults, use a source CLI's
structured output, apply built-in processing around execution, substantially
reshape output, and explain every change. An explicit raw route may preserve
access to the original CLI, but must never be an automatic fallback from policy
or transformation failure.

Conceptually:

```text
per-command YAML + attempted command + source evidence
  -> deterministic plan
  -> preview
     or
  -> wrapper: built-in before -> source CLI -> built-in output/after
```

## Try the local slice

The repository includes an example policy. Previewing it does not require
`gh` to be installed because Atsura makes zero source-process attempts:

```sh
go run ./cmd/atr plan preview \
  --config examples/plan-preview.yaml \
  -- gh pr list --state open
```

The JSON result shows the allow/deny decision, original and transformed argv,
the matched command and reason, a typed output reshape, and
`source_process_attempts: 0`. Use `atr help plan preview` for the complete
machine-readable command contract.

Run the same kind of plan against the repository's synthetic JSON source:

```sh
go run ./cmd/atr run \
  --config examples/run-local.yaml \
  -- go run ./tools/sourcefixture --limit=2
```

The result contains only `id`, `title`, and `state`, reports
`source_process_attempts: 1`, and needs no provider account. Run this from the
repository root with the exact Go version declared in `go.mod`.

The following decisions remain open and require later research or a vertical
slice:

- first source CLI;
- future YAML schema evolution, locations, inheritance, and trust workflow;
- command-discovery depth;
- Claude Code hook responsibilities;
- wrapper or hook integration mechanism;
- exact allow, confirm, and deny semantics;
- output-transform vocabulary beyond schema-1 select and rename;
- usage-history collection;
- jq, RTK, plugin, or external-transformer boundaries; and
- behavior beyond v0.1's fail-closed source and transform boundary.

See [Project Theses](docs/00_theses.md), [Product Contract](docs/01_product_contract.md),
[Architecture](docs/02_architecture.md), and [Security Model](docs/03_security_model.md).

## Repository layout

```text
cmd/atr/                 executable entry point
internal/domain/         pure vocabulary and invariants
internal/app/            deterministic user-task orchestration and ports
internal/infra/          bounded process, filesystem, and other adapters
internal/cli/            public catalog, parsing, presentation, composition
docs/                    durable product and engineering decisions
docs/work/               temporary active-change packets
tools/                   repository-aware validation tools
scripts/check.sh         canonical verification interface
```

The four-layer dependency direction and repository invariants are defined in
[AGENTS.md](AGENTS.md). New user-visible capabilities must follow the
[`$add-capability` workflow](.agents/skills/add-capability/SKILL.md).

## Development

Install the exact Go version declared by `go.mod`, then run:

```sh
go run ./cmd/atr --help
task check:fast
```

The canonical verification profiles are:

```sh
./scripts/check.sh fast
./scripts/check.sh full
./scripts/check.sh security
./scripts/check.sh release
./scripts/check.sh public
```

The gate sets `GOTOOLCHAIN=local`; the `go` binary selected by `PATH` must
belong to the exact required installation.

## Safety and maturity

Commands, arguments, help output, source output, generated catalogs, YAML, and
hook payloads are treated as untrusted. Repository-provided configuration is
not implicitly user-trusted. Initial YAML processing is limited to typed
Atsura built-ins rather than arbitrary shell. Atsura does not acquire or store
provider credentials or persist source output. The source process inherits the
caller's environment, so its own credential handling remains authoritative.

No Atsura release has been published. See [Release Model](docs/06_release.md)
for the reviewed v0.1 packaging boundary and the remaining first-tag controls.

For contributions and help, see [CONTRIBUTING.md](CONTRIBUTING.md),
[SUPPORT.md](SUPPORT.md), and [SECURITY.md](SECURITY.md).

## License

Atsura is licensed under the [MIT License](LICENSE).
