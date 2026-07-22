# Atsura

A deterministic framework for tailoring existing CLIs to coding agents.

Atsura compiles bounded source-CLI evidence and a reviewed tailoring
specification into a purpose-specific command and option surface. Each included
command has either an identity wrapper or a finite deterministic transforming
wrapper. The source CLI remains authoritative for its own authentication,
authorization, operation semantics, and remote effects.

## Project status

The current milestone implements artifact compilation, adoption, deterministic
wrapper-plan inspection, and one narrow bundle-backed transformation runtime:

```text
source inspect -> spec init/validate -> bundle build -> bundle status/trust
  -> bundle preview
  -> bundle execute
```

- Tailoring specification schema 3 independently declares command membership,
  option membership, and wrapper behavior.
- Bundle schema 2 binds exact source identity, catalog evidence, the normalized
  specification, and the compiled purpose-specific surface.
- `bundle trust` interactively records adoption of one exact bundle digest. It
  does not grant permission to run source operations.
- `bundle preview --bundle <path> -- <source-executable> <argv>` returns one
  deterministic schema-3 wrapper plan and digest with zero source-process
  attempts.
- `bundle execute` rebuilds that plan and, for a compatibility-admitted GitHub
  CLI `issue list` or `pr list` JSON transform, requires every observable
  executable identity to match the bundle and starts at most once before
  returning only selected/renamed typed JSON.
- The retired authorization-oriented policy schemas, legacy `plan preview`,
  and `run` have migration diagnostics only. They are not current tailoring
  capabilities.

Identity-wrapper execution, argv-only transforms, successful nonempty source
stderr, source refresh, raw bypass, additional source/output adapter contracts,
and host-neutral wrapper materialization remain unimplemented. Coding-agent
host adapters are outside the product boundary.

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
  -> current source path/hash/size validation
  -> deterministic wrapper plan + digest
     -> preview with zero source attempts
     -> adapter compatibility admission
        -> one bounded source process
        -> selected/renamed typed JSON result
```

An excluded command is absent from the tailored surface; it is not denied or
classified as unsafe. Surface membership and wrapper transformation are
independent. A wrapper plan describes ordered stages and exact argv, not an
authorization decision. Hiding is a discoverability and composition
feature, not an OS sandbox.

## Try the installed artifact workflow

The first source adapter inspects an installed GitHub CLI using four bounded
offline probes. Extract the archive for your platform, make `atr` available on
your command path, and start with `atr help source inspect --format agent`.
No public Atsura archive has been released yet; this workflow currently applies
to a locally packaged candidate and to a future reviewed release. With GitHub
CLI 2.x installed, using such an archive requires no Atsura source checkout:

```sh
atr source inspect \
  --adapter github-cli \
  --executable gh > /tmp/atsura-catalog.json

atr spec init \
  --catalog /tmp/atsura-catalog.json \
  -- pr list > /tmp/atsura-spec.yaml
```

`spec init` creates an exclude-by-default specification containing one exact
verified command with inherited options and an identity wrapper. Review and
edit that file before validation and compilation. To exercise the current
runtime, replace the generated command's `wrapper` with this built-in JSON
transform. Exact `spec init` and `spec validate` agent help publish the finite
schema-3 field inventory and authoring constraints; this edit is deliberate
configuration authoring, not generated source code:

```yaml
wrapper:
  kind: transform
  before: []
  invoke:
    append_args: ["--json=number,title,state"]
  output:
    input: json
    select: [number, title, state]
    rename:
      - from: number
        to: id
    render: compact_json
  after: []
```

After editing the generated specification, validate and compile those exact
bytes:

```sh
atr spec validate \
  --catalog /tmp/atsura-catalog.json \
  --spec /tmp/atsura-spec.yaml

atr bundle build \
  --catalog /tmp/atsura-catalog.json \
  --spec /tmp/atsura-spec.yaml > /tmp/atsura-bundle.json

atr bundle status \
  --bundle /tmp/atsura-bundle.json
```

The static
[schema-3 example](examples/tailoring-spec.schema3.yaml) illustrates the strict
shape, but its placeholder catalog digest must be replaced by the digest from
the exact inspected catalog; `spec init` is the reliable way to produce a bound
draft.

To adopt the compiled surface, run this in an interactive terminal and confirm
the exact digest after reviewing the source, surface, and wrapper summary:

```sh
atr bundle trust --bundle /tmp/atsura-bundle.json
atr bundle status --bundle /tmp/atsura-bundle.json
atr bundle preview \
  --bundle /tmp/atsura-bundle.json \
  -- gh pr list --limit=2
atr bundle execute \
  --bundle /tmp/atsura-bundle.json \
  -- gh pr list --limit=2
```

`bundle preview` requires the exact bundle digest to be adopted and the current
source path, SHA-256, and size to match its catalog evidence. It selects the
longest command prefix from the complete catalog, applies command and option
surface membership, and returns source/adapter identity, the exact or `null`
specification entry, original/transformed argv, ordered stages, finite
process bounds, and a canonical plan digest. It always reports
`source_process_attempts: 0` and does not transform output at preview time.

`bundle execute` independently rebuilds the plan rather than trusting preview
output, verifies GitHub CLI adapter contract 2, the complete supported argv,
and the exact ordered `--json` selector, then starts the identity-checked
resolved path once without a shell. Successful stdout is still strictly parsed
and transformed. Its fixed
schema-2 result contains the bundle and plan digests, matched command,
transformation shape and fields, selected records, exit code, and
`source_process_attempts: 1`. Raw stdout, stderr, and unselected fields are not
returned or persisted. This live probe uses the source CLI's own authentication
plus repository context from the inherited working directory or an admitted
command-specific `--repo` option. Atsura does not obtain, store, or diagnose
GitHub credentials. A source-owned post-start failure is non-retryable from
Atsura's perspective; resolve it with the source CLI before choosing to execute
again. The credential- and provider-network-free synthetic fixture is the
canonical automated evidence.

Use `atr help <exact-command> --format agent` for the complete machine-readable
contract. Agent help currently uses schema version 8; object outputs may publish
a versioned nested JSON-pointer field inventory.

## Current decisions and open work

The current schema supports exact command include/exclude composition,
inherited or narrowed options, identity wrappers, appended argv, and typed JSON
select/rename/compact-output transformations. Before and after stage lists are
explicit but must remain empty; arbitrary shell, script, jq, plugin, RTK, and
runtime-LLM actions are invalid.

The following remain later research or vertical-slice decisions:

- additional source CLIs and adapter compatibility;
- source refresh and command-discovery depth;
- execution of identity wrappers, argv-only transforms, and nonempty successful
  stderr;
- host-neutral wrapper materialization and its artifact/runtime contract;
- fixture evidence that caller-owned environments can expose the same wrapper;
- raw tailoring bypass;
- output transformations beyond the schema-3 built-ins;
- usage-history collection; and
- jq, RTK, plugin, or external-transformer boundaries.

Current plan parsing is deliberately bounded. Source short options,
root/global options, and command-specific positional grammar are not completely
modeled. If a matched command has cataloged descendants, an unknown following
non-dash token is ambiguous rather than assumed positional; use an inner `--`
before positional data. `append_args` remain at the end even after an existing
`--`, and option-looking values there are positional. Preview requires one
active cataloged selector matching a planned structured input. Execute
additionally requires the current adapter contract to admit the exact ordered
selector and complete argv. Competing `--jq`, `--template`, and `--web` output
modes plus unmodeled positional or option syntax fail before source start.
These are compatibility limits, not inferred behavior.

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
specifications, bundles, and wrapper bindings are untrusted. Repository-provided
configuration is not implicitly user-adopted. Specification processing is
limited to typed Atsura built-ins rather than arbitrary executable code. Atsura
does not acquire or store provider credentials or persist source output. Source
inspection starts the selected executable without a shell using adapter-owned,
bounded offline probes; the source process inherits the caller's environment.
Coding-agent hook payloads, settings, permissions, and lifecycle remain outside
Atsura. External environments may expose and call the host-neutral wrapper;
their host-specific mechanics remain external.

No Atsura release has been published. See [Release Model](docs/06_release.md)
for the reviewed packaging boundary and remaining first-tag controls.

For contributions and help, see [CONTRIBUTING.md](CONTRIBUTING.md),
[SUPPORT.md](SUPPORT.md), and [SECURITY.md](SECURITY.md).

## License

Atsura is licensed under the [MIT License](LICENSE).
