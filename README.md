# Atsura

A deterministic framework for tailoring existing CLIs to coding agents.

Atsura's working hypothesis is that a maintainer can inspect an existing CLI,
apply a small reviewed policy difference, and give a coding agent a narrower,
purpose-specific command surface without reimplementing the source CLI. The
routine decision path is intended to be deterministic and not require a
language model.

## Project status

Atsura is identity-bootstrapped but does not yet implement source-CLI
inspection, policy evaluation, command wrapping, execution, or output
transformation. Public Atsura commands and configuration schemas are not stable.

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
structured output, select task-relevant information, and explain the applied
policy. An explicit raw route may preserve access to the original CLI, but must
never be an automatic fallback from policy rejection.

The recommended first slice is intentionally smaller: preview one modeled
source invocation against one small trusted policy and return a deterministic
decision, exact planned argv when applicable, and the matched reason without
executing the source process.

The following decisions remain open and require later research or a vertical
slice:

- first source CLI;
- policy representation, including whether YAML is appropriate;
- command-discovery depth;
- Claude Code hook responsibilities;
- wrapper or hook integration mechanism;
- exact allow, confirm, and deny semantics;
- usage-history collection;
- RTK reuse or integration; and
- output transformation and fallback behavior.

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

Commands, arguments, help output, source output, generated catalogs, and policy
files are treated as untrusted. Repository-provided configuration is not
implicitly user-trusted. Atsura does not currently acquire or store provider
credentials or raw confidential source output.

No release is created or promised by the bootstrap. See [Release Model](docs/06_release.md)
for the inherited packaging foundation and decisions still required before an
Atsura release.

For contributions and help, see [CONTRIBUTING.md](CONTRIBUTING.md),
[SUPPORT.md](SUPPORT.md), and [SECURITY.md](SECURITY.md).

## License

Atsura is licensed under the [MIT License](LICENSE).
