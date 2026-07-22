# Atsura

A deterministic framework for tailoring existing CLIs to coding agents.

Atsura compiles bounded source-CLI evidence and a reviewed tailoring
specification into a purpose-specific command and option surface. Each included
command has either an identity wrapper or a finite deterministic transforming
wrapper. The source CLI remains authoritative for its own authentication,
authorization, operation semantics, and remote effects.

## Project status

The current milestone implements artifact compilation, adoption, deterministic
wrapper-plan inspection, one narrow bundle-backed transformation runtime, and a
host-neutral ordinary-command entry point:

```text
source inspect -> spec init/validate -> bundle build -> bundle status/trust
  -> bundle preview
  -> bundle execute
  -> wrapper render -> caller-owned POSIX activation -> ordinary source command
```

- Tailoring specification schema 3 independently declares command membership,
  option membership, and wrapper behavior.
- Bundle schema 2 binds exact source identity, catalog evidence, the normalized
  specification, and the compiled purpose-specific surface.
- `bundle trust` interactively records adoption of one exact bundle digest. It
  does not grant permission to run source operations.
- `bundle preview --bundle <path> -- <source-executable> <argv>` returns one
  deterministic schema-4 wrapper plan and digest with zero source-process
  attempts.
- `bundle execute` rebuilds that plan and, for a compatibility-admitted GitHub
  CLI `issue list` or `pr list` JSON transform, requires every observable
  executable identity to match the bundle and starts at most once before
  returning only selected/renamed typed JSON.
- `wrapper render` emits deterministic POSIX function bytes on Linux and macOS,
  bound to one exact adopted bundle and the current absolute `atr` identity.
  The fixed function forwards ordinary argv to `wrapper run`, which applies the
  same fresh plan. A `transformed_json` plan emits one compact JSON object or
  array; a `source_stream_passthrough` plan returns the conventionally completed
  source stdout, stderr, and exit status without changing their bytes. One
  finite compatibility registry admits the existing GitHub cases and exact no-
  argument `test` identity wrappers carrying a recorded stable Go 1.26.x
  inspection observation through that same path.
- The retired authorization-oriented policy schemas, legacy `plan preview`,
  and `run` have migration diagnostics only. They are not current tailoring
  capabilities.

Direct `bundle execute` support for identity and argv-only plans, source
refresh, raw bypass, source contracts beyond the current GitHub CLI and Go CLI
slices, output-processor contracts, persistent wrapper installation, and
executable shims remain unimplemented.
Coding-agent host adapters are outside the product boundary. Windows retains
the existing command surface but returns a structured unsupported fault for
POSIX wrapper rendering; no Windows POSIX activation support is claimed.

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

exact adopted bundle + current stable atr identity
  -> deterministic POSIX function + source digest
  -> caller-owned activation as the ordinary source command
  -> wrapper run verifies the bundle/runtime closure
  -> same fresh plan and source boundary
     -> transformed_json: one compact plan-declared JSON value
     -> source_stream_passthrough: exact bounded source streams + status
```

An excluded command is absent from the tailored surface; it is not denied or
classified as unsafe. Surface membership and wrapper transformation are
independent. A wrapper plan describes ordered stages and exact argv, not an
authorization decision. Hiding is a discoverability and composition
feature, not an OS sandbox.

## Try the installed artifact workflow

The current source adapters inspect GitHub CLI with four bounded offline probes.
The Go adapter uses exactly `go version`, `go help`, and `go help test`, and
requires the first probe to record stable Go 1.26.x.
Use one stable built or installed `atr` path for the entire
workflow; do not render a wrapper from `go run`, whose temporary executable may
disappear before the function is invoked. No public Atsura archive has been
released yet. From a source checkout, a stable local candidate can be built as:

```sh
mkdir -p /tmp/atsura-demo/bin
go build -o /tmp/atsura-demo/bin/atr ./cmd/atr
ATR=/tmp/atsura-demo/bin/atr
```

With GitHub CLI 2.x installed, inspect `gh` by its ordinary spelling so the
renderer can later use that exact spelling as the function name:

```sh
"$ATR" source inspect \
  --adapter github-cli \
  --executable gh > /tmp/atsura-catalog.json

"$ATR" spec init \
  --catalog /tmp/atsura-catalog.json \
  -- pr list > /tmp/atsura-spec.yaml
```

`spec init` creates an exclude-by-default specification containing one exact
verified command with inherited options and an identity wrapper. Review and
edit that file before validation and compilation. To exercise the current
runtime and make the complete exposed surface renderable, replace the generated
command's option surface and `wrapper` with this deliberately narrow built-in
JSON transform. Only `--limit` remains agent-visible; the generated `--json`
selector belongs to the invocation stage. Exact `spec init` and `spec validate`
agent help publish the finite schema-3 field inventory and authoring
constraints:

```yaml
options:
  default: exclude
  include: [--limit]
  exclude: []
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
"$ATR" spec validate \
  --catalog /tmp/atsura-catalog.json \
  --spec /tmp/atsura-spec.yaml

"$ATR" bundle build \
  --catalog /tmp/atsura-catalog.json \
  --spec /tmp/atsura-spec.yaml > /tmp/atsura-bundle.json

"$ATR" bundle status \
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
"$ATR" bundle trust --bundle /tmp/atsura-bundle.json
"$ATR" bundle status --bundle /tmp/atsura-bundle.json
"$ATR" bundle preview \
  --bundle /tmp/atsura-bundle.json \
  -- gh pr list --limit=2
"$ATR" bundle execute \
  --bundle /tmp/atsura-bundle.json \
  -- gh pr list --limit=2
```

On Linux or macOS, render and activate the same bundle as the ordinary command.
The JSON form is review metadata; the default text form is the exact sourceable
function. Activation is deliberately caller-owned:

```sh
"$ATR" wrapper render \
  --bundle /tmp/atsura-bundle.json \
  --format json

"$ATR" wrapper render \
  --bundle /tmp/atsura-bundle.json > /tmp/atsura-gh-wrapper.sh

. /tmp/atsura-gh-wrapper.sh
gh pr list --limit=2
unset -f gh
```

The fixed function invokes the absolute `atr` that rendered it, passes the
complete bundle/runtime closure to `wrapper run`, inserts the required `--`, and
forwards `"$@"` without `eval` or `sh -c`. Successful ordinary-command stdout
and status follow the fresh plan's required result mode. `transformed_json`
emits exactly one compact object or array plus LF, empty stderr, and status
zero. `source_stream_passthrough` emits the conventionally completed source
stdout and stderr bytes exactly and returns its status; it adds no framing or
projection and makes no timing or cross-stream interleaving claim. Neither mode
adds a `bundle execute` evidence envelope. Windows returns the structured
`wrapper_platform_not_supported` fault and does not claim POSIX activation.

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

`wrapper render` additionally rejects a bundle unless its complete included
surface contains exactly one command admitted by the registry-selected runtime
verifier. GitHub CLI contract 2 permits `issue list` or `pr list` under the
existing typed JSON, identity, or finite append-argv-only grammar. Go CLI
contract 1 permits only identity-wrapped `test` with no observed long-option or
structured-output surface. It derives the function name verbatim from the
bundle's requested executable, so an absolute source path or non-POSIX/reserved
name is not normalized into a wrapper name. The rendered source digest is
deterministic review evidence, not attestation after the caller sources or
changes the function.

`wrapper run` derives the source spelling from the strictly loaded bundle and
uses the same fresh-plan application service as direct execution. Its runtime
hash check is cooperative drift detection: the shell must start the bound
absolute `atr` path before honest runtime code can verify itself. A mismatch
prevents that honest runtime from starting the source, but Atsura does not claim
to sandbox malicious replacement code already executing at that path.

To exercise the second-source slice, run from a reviewed Go module where
inspection records stable Go 1.26.x. The identity draft is already the complete admitted specification;
direct `bundle execute` remains transform-only, so use preview and the ordinary
wrapper:

```sh
"$ATR" source inspect \
  --adapter go-cli \
  --executable go > /tmp/atsura-go-catalog.json
"$ATR" spec init \
  --catalog /tmp/atsura-go-catalog.json \
  -- test > /tmp/atsura-go-spec.yaml
"$ATR" spec validate \
  --catalog /tmp/atsura-go-catalog.json \
  --spec /tmp/atsura-go-spec.yaml
"$ATR" bundle build \
  --catalog /tmp/atsura-go-catalog.json \
  --spec /tmp/atsura-go-spec.yaml > /tmp/atsura-go-bundle.json
"$ATR" bundle trust --bundle /tmp/atsura-go-bundle.json
"$ATR" bundle preview --bundle /tmp/atsura-go-bundle.json -- go test
"$ATR" wrapper render \
  --bundle /tmp/atsura-go-bundle.json > /tmp/atsura-go-wrapper.sh

. /tmp/atsura-go-wrapper.sh
go test
unset -f go
```

The runtime accepts no argv after `test`: options, package patterns, `--`, and
test-binary arguments fail before source start. `go test` is source-owned
`EffectExecute`, not a read or permission decision. It may compile and run
repository code, use credentials or configuration, resolve modules, access
networks, and mutate caller-owned files or caches; Atsura does not sandbox or
authorize those effects.

For Go, path/hash/size identify the direct launcher file, while
`Source.Version` is the possibly delegated effective toolchain observed by
`go version` under the inspection working directory and environment. Runtime
does not repeat that probe or bind a selected/downloaded toolchain or GOROOT
tree. The same launcher may later select another toolchain from module state,
`GOTOOLCHAIN`, `GOROOT`, or related ambient inputs without pre-start detection.

Use `atr help <exact-command> --format agent` for the complete machine-readable
contract. Agent help currently uses schema version 10; object outputs may
publish a versioned nested JSON-pointer field inventory. Each output declares
whether the catalog or a freshly rebuilt wrapper plan governs its result.
`wrapper run` points to the exact `bundle preview` plan schema and publishes
the finite `plan_result_modes` byte, framing, projection, and status contract.

## Current decisions and open work

The current schema supports exact command include/exclude composition,
inherited or narrowed options, identity wrappers, appended argv, and typed JSON
select/rename/compact-output transformations. Before and after stage lists are
explicit but must remain empty; arbitrary shell, script, jq, plugin, RTK, and
runtime-LLM actions are invalid.

The following remain later research or vertical-slice decisions:

- source CLIs beyond the current GitHub and Go contracts;
- recorded Go version observations beyond 1.26.x, any option, package,
  positional-marker, or test-binary argument grammar, and any future explicit
  environment/toolchain closure for effective runtime selection;
- source refresh and command-discovery depth;
- direct `bundle execute` support for source-stream plans;
- persistent wrapper installation, replacement, removal, executable/PATH shims,
  and multi-profile command selection;
- raw tailoring bypass;
- output transformations beyond the schema-3 built-ins;
- usage-history collection; and
- jq, RTK, plugin, or external-transformer boundaries. The next RTK research
  candidate is pass-only `go test -json` with the fixed `go-test` filter; it is
  not implemented, registered, or generated as a default.

Current plan parsing is deliberately bounded. Source short options,
root/global options, and command-specific positional grammar are not completely
modeled. If a matched command has cataloged descendants, an unknown following
non-dash token is ambiguous rather than assumed positional; use an inner `--`
before positional data. `append_args` remain at the end even after an existing
`--`, and option-looking values there are positional. Preview requires one
active cataloged selector matching a planned structured input only for a
`transformed_json` plan. Direct `bundle execute` remains restricted to that
path and additionally requires the exact ordered selector. The ordinary
wrapper runtime separately admits the exact finite identity and append-only
source-stream grammars without inventing an output selector. Competing `--jq`,
`--template`, and `--web` output modes plus unmodeled positional or option
syntax fail before source start. These are compatibility limits, not inferred
behavior.

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
The native second-source artifact fixture also uses `GOTOOLCHAIN=local`,
isolated cache/module roots, and disabled downloads for deterministic evidence.
Those fixture settings do not change the production wrapper's inherited
environment or guarantee its effective Go toolchain.

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
