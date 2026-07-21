# Atsura

A deterministic framework for tailoring existing CLIs to coding agents.

Atsura compiles bounded source-CLI evidence and a reviewed tailoring
specification into a purpose-specific command and option surface. Each included
command has either an identity wrapper or a finite deterministic transforming
wrapper. The source CLI remains authoritative for its own authentication,
authorization, operation semantics, and remote effects.

## Project status

The current schema-correction milestone implements the artifact workflow, not
bundle-backed source execution:

```text
source inspect -> spec init/validate -> bundle build -> bundle status/trust
```

- Tailoring specification schema 3 independently declares command membership,
  option membership, and wrapper behavior.
- Bundle schema 2 binds exact source identity, catalog evidence, the normalized
  specification, and the compiled purpose-specific surface.
- `bundle trust` interactively records adoption of one exact bundle digest. It
  does not grant permission to run source operations.
- The retired authorization-oriented policy schemas, `plan preview`, and `run`
  have migration diagnostics only. They are not current tailoring capabilities.

Source refresh, bundle runtime execution, raw bypass, and host adapters remain
paused until pure surface resolution and identity-wrapper planning are
implemented and validated.

The `atr` binary also contains the foundry's `doctor` and synthetic `sample`
commands as executable architecture and harness examples. They are not
source-CLI tailoring features.

Project identity:

- Product: `Atsura`
- Binary: `atr`
- Go module: `github.com/tasuku43/atsura`
- License: MIT
- Documentation locale: English

## Product model

```text
installed source CLI
  -> bounded adapter inspection
  -> provenance-bearing command catalog

catalog + reviewed schema-3 specification
  -> deterministic compilation
  -> canonical schema-2 bundle
     + compiled command/option surface
     + identity or transforming wrapper per included command

exact bundle digest
  -> explicit user adoption
```

An excluded command is absent from the tailored surface; it is not denied or
classified as unsafe. Surface membership and wrapper transformation are
independent. A future wrapper plan will describe ordered stages and exact argv,
not an authorization decision. Hiding is a discoverability and composition
feature, not an OS sandbox.

## Try the artifact workflow

The first source adapter inspects an installed GitHub CLI using two bounded
offline probes. From the repository root, with `gh` installed:

```sh
go run ./cmd/atr source inspect \
  --adapter github-cli \
  --executable gh > /tmp/atsura-catalog.json

go run ./cmd/atr spec init \
  --catalog /tmp/atsura-catalog.json \
  -- pr list > /tmp/atsura-spec.yaml

go run ./cmd/atr spec validate \
  --catalog /tmp/atsura-catalog.json \
  --spec /tmp/atsura-spec.yaml

go run ./cmd/atr bundle build \
  --catalog /tmp/atsura-catalog.json \
  --spec /tmp/atsura-spec.yaml > /tmp/atsura-bundle.json

go run ./cmd/atr bundle status \
  --bundle /tmp/atsura-bundle.json
```

`spec init` creates an exclude-by-default specification containing one exact
verified command with inherited options and an identity wrapper. Review and
edit that file before validation and compilation. The static
[schema-3 example](examples/tailoring-spec.schema3.yaml) illustrates the strict
shape, but its placeholder catalog digest must be replaced by the digest from
the exact inspected catalog; `spec init` is the reliable way to produce a bound
draft.

To adopt the compiled surface, run this in an interactive terminal and confirm
the exact digest after reviewing the source, surface, and wrapper summary:

```sh
go run ./cmd/atr bundle trust --bundle /tmp/atsura-bundle.json
go run ./cmd/atr bundle status --bundle /tmp/atsura-bundle.json
```

Use `atr help <exact-command> --format agent` for the complete machine-readable
contract. Agent help currently uses schema version 7.

## Current decisions and open work

The current schema supports exact command include/exclude composition,
inherited or narrowed options, identity wrappers, appended argv, and typed JSON
select/rename/compact-output transformations. Before and after stage lists are
explicit but must remain empty; arbitrary shell, script, jq, plugin, RTK, and
runtime-LLM actions are invalid.

The following remain later research or vertical-slice decisions:

- additional source CLIs and adapter compatibility;
- source refresh and command-discovery depth;
- pure surface resolution and identity-wrapper plan output;
- bundle runtime and exact post-start failure behavior;
- Claude Code and other host-adapter responsibilities;
- wrapper installation or hook integration mechanisms;
- raw tailoring bypass;
- output transformations beyond the schema-3 built-ins;
- usage-history collection; and
- jq, RTK, plugin, or external-transformer boundaries.

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

Commands, arguments, help output, source output, generated catalogs,
specifications, bundles, and hook payloads are untrusted. Repository-provided
configuration is not implicitly user-adopted. Specification processing is
limited to typed Atsura built-ins rather than arbitrary executable code. Atsura
does not acquire or store provider credentials or persist source output. Source
inspection starts the selected executable without a shell using adapter-owned,
bounded offline probes; the source process inherits the caller's environment.

No Atsura release has been published. See [Release Model](docs/06_release.md)
for the reviewed packaging boundary and remaining first-tag controls.

For contributions and help, see [CONTRIBUTING.md](CONTRIBUTING.md),
[SUPPORT.md](SUPPORT.md), and [SECURITY.md](SECURITY.md).

## License

Atsura is licensed under the [MIT License](LICENSE).
