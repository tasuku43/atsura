# Atsura

A deterministic framework for tailoring existing CLIs to coding agents.

Atsura compiles bounded source-CLI evidence and a reviewed tailoring
specification into a purpose-specific command and option surface. Each included
command has either an identity wrapper or a finite deterministic transforming
wrapper. The source CLI remains authoritative for its own authentication,
authorization, operation semantics, and remote effects.

## Project status

The current milestone implements artifact compilation, adoption, deterministic
wrapper-plan inspection, finite source and processor compatibility admission,
and a host-neutral ordinary-command entry point:

```text
source inspect + optional processor inspect
  -> spec init/validate -> bundle build -> bundle status/trust
  -> bundle preview
  -> bundle execute
  -> wrapper render -> caller-owned POSIX activation -> ordinary source command
```

- Tailoring specification schema 5 independently declares command membership,
  option membership, catalog-typed value-option defaults, wrapper behavior,
  and a closed projection-or-optimizer output union.
- Bundle schema 4 binds exact source identity, catalog evidence, the normalized
  specification, the compiled purpose-specific surface, and any exact
  processor observation required by that surface.
- `bundle trust` interactively records adoption of one exact bundle digest. It
  does not grant permission to run source operations.
- `bundle preview --bundle <path> -- <source-executable> <argv>` returns one
  deterministic schema-6 wrapper plan and digest with zero source-process
  attempts.
- `bundle execute` rebuilds that plan and, for a compatibility-admitted GitHub
  CLI `issue list` or `pr list` JSON transform, requires every observable
  executable identity to match the bundle and starts at most once before
  returning only selected/renamed typed JSON.
- `wrapper render` emits deterministic POSIX function bytes on Linux and macOS,
  bound to one exact adopted bundle and the current absolute `atr` identity.
  Its JSON review envelope is schema 2 and reports both source and processor
  attempts as zero.
  Contract 3 compiles root, included-namespace, and included-command final
  `--help` views from that exact bundle. Those views start no bound `atr`,
  source, or processor; every other argv list is forwarded unchanged to
  `wrapper run`, which applies the
  same fresh plan. A `transformed_json` plan emits one compact JSON object or
  array; a `source_stream_passthrough` plan returns conventionally completed
  source stdout, stderr, and status without changing their bytes; and an
  `original_preserving_optimizer` plan exposes one of three exact dispositions.
  One finite source registry admits the existing GitHub cases and Go CLI
  contract 2. A separate processor registry admits only the exact
  `atsura.output.rtk_go_test_pass.v1` tuple using an explicitly inspected
  official RTK v0.43.0 artifact.
- The retired authorization-oriented policy schemas, legacy `plan preview`,
  and `run` have migration diagnostics only. They are not current tailoring
  capabilities.

Direct `bundle execute` support for identity and argv-only plans, source
refresh, raw bypass, source contracts beyond the current GitHub CLI and Go CLI
slices, additional output-processor contracts, persistent wrapper installation,
and executable shims remain unimplemented.
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

catalog + reviewed schema-5 specification
  -> deterministic compilation
  -> canonical schema-4 bundle
     + compiled command/option surface
     + identity or transforming wrapper per included command
     + exact processor binding when the wrapper requires one

exact bundle digest
  -> explicit user adoption
  -> current source path/hash/size validation
  -> deterministic wrapper plan + digest
     -> preview with zero source attempts
     -> source-adapter compatibility admission
        -> one bounded source process
        -> selected/renamed typed JSON result

explicit RTK observation + exact Go test optimizer specification
  -> source plan binds go test -json
  -> processor plan binds rtk pipe --filter=go-test
  -> strict pass-only admission before processor start
     -> preserved_before_processor: exact source streams/status, or
     -> preserved_after_processor: byte-identical admitted input, or
     -> optimized: independently validated shorter summary

exact adopted bundle + current stable atr identity
  -> deterministic POSIX function + source digest
  -> caller-owned activation as the ordinary source command
  -> wrapper run verifies the bundle/runtime closure
  -> same fresh plan and source boundary
     -> transformed_json: one compact plan-declared JSON value
     -> source_stream_passthrough: exact bounded source streams + status
     -> original_preserving_optimizer: one exact declared disposition
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
agent help publish the finite schema-5 field inventory and authoring
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
    option_defaults:
      - option: --limit
        value: "30"
    append_args: ["--json=number,title,state"]
  output:
    kind: projection
    projection:
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
[schema-5 example](examples/tailoring-spec.schema5.yaml) illustrates the strict
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
  -- gh pr list
"$ATR" bundle execute \
  --bundle /tmp/atsura-bundle.json \
  -- gh pr list
"$ATR" bundle preview \
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
gh --help
gh pr --help
gh pr list --help
gh pr list
gh pr list --limit=2
unset -f gh
```

Sourcing deliberately removes an existing `gh` alias before defining the
function so the ordinary name resolves to the wrapper. It does not edit shell
startup files; restore a prior alias yourself after `unset -f gh` if needed.
This activation expects `unalias` to be the standard shell utility rather than
a caller-defined function.

For this one-command walkthrough, the fixed function answers the three shown
help selectors from the exact bundle-derived surface and prints its full bundle
digest. A bundle containing both maintained GitHub commands instead answers
five selectors: root, `issue`, `issue list`, `pr`, and `pr list`. This artifact-
local help does not execute raw source help or claim that later source/runtime
state is current. Exact-command help discloses the configured `--limit`
default; root and namespace membership views are unchanged. For every non-help
argv list, the function invokes the
absolute `atr` that rendered it, passes the complete bundle/runtime closure to
`wrapper run`, inserts the required `--`, and forwards `"$@"` without `eval` or
`sh -c`. Successful ordinary-command stdout
and status follow the fresh plan's required result mode. `transformed_json`
emits exactly one compact object or array plus LF, empty stderr, and status
zero. `source_stream_passthrough` emits the conventionally completed source
stdout and stderr bytes exactly and returns its status; it adds no framing or
projection and makes no timing or cross-stream interleaving claim.
`original_preserving_optimizer` either preserves an ineligible conventional
result before the processor starts, accepts byte-identical admitted input after
the processor, or returns an independently validated shorter summary. None of
the three modes adds a `bundle execute` evidence envelope. Windows returns the
structured `wrapper_platform_not_supported` fault and does not claim POSIX
activation or optimizer support.

`bundle preview` requires the exact bundle digest to be adopted and the current
source path, SHA-256, and size to match its catalog evidence. It selects the
longest command prefix from the complete catalog, applies command and option
surface membership, and returns source/adapter identity, the exact or `null`
specification entry, original/transformed argv, ordered stages, finite
process bounds, and a canonical plan digest. It always reports
`source_process_attempts: 0` and does not transform output at preview time.
Plan schema 6 also records the complete declared `option_defaults` list and
the exact applied subset. In this walkthrough, omitted `--limit` inserts
`--limit=30` immediately after `pr list`; explicit `--limit=2` remains exact
and suppresses the default.

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
surface is non-empty and every entry is admitted by the registry-selected
runtime verifier. GitHub CLI contract 2 permits one or both of `issue list` and
`pr list` under the existing typed JSON, identity, or finite append-argv-only
grammar; different admitted entries may retain different result modes. Go CLI
contract 2 remains a singleton surface containing identity-wrapped `test` or
the exact processor-bound `test -json` optimizer, both with no caller-visible
option surface. It derives
the function name verbatim from the bundle's requested executable, so an
absolute source path or non-POSIX/reserved name is not normalized into a
wrapper name. The rendered source digest is deterministic review evidence, not
attestation after the caller sources or changes the function.

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

On a supported Linux or macOS amd64/arm64 host, the exact RTK-backed optimizer
is selected only from an explicit observation of an official RTK v0.43.0
artifact. Atsura neither discovers nor installs RTK:

```sh
"$ATR" processor inspect \
  --adapter rtk \
  --executable /absolute/path/to/rtk > /tmp/atsura-rtk-observation.json
"$ATR" spec init \
  --catalog /tmp/atsura-go-catalog.json \
  --processor /tmp/atsura-rtk-observation.json \
  -- test > /tmp/atsura-go-optimized-spec.yaml
"$ATR" spec validate \
  --catalog /tmp/atsura-go-catalog.json \
  --spec /tmp/atsura-go-optimized-spec.yaml
"$ATR" bundle build \
  --catalog /tmp/atsura-go-catalog.json \
  --spec /tmp/atsura-go-optimized-spec.yaml \
  --processor /tmp/atsura-rtk-observation.json \
  > /tmp/atsura-go-optimized-bundle.json
"$ATR" bundle trust --bundle /tmp/atsura-go-optimized-bundle.json
"$ATR" wrapper render \
  --bundle /tmp/atsura-go-optimized-bundle.json \
  > /tmp/atsura-go-optimized-wrapper.sh

. /tmp/atsura-go-optimized-wrapper.sh
go test
unset -f go
```

Atsura starts `go test -json` itself. Only stdout that passes its strict
single-package pass validator is sent to `rtk pipe --filter=go-test`. Failures,
skips, malformed or non-beneficial results are returned exactly before RTK
starts. Once RTK starts, any processor failure is non-retryable, returns no
source or processor bytes, and never falls back to the original stream.

Use `atr help <exact-command> --format agent` for the complete machine-readable
contract. Agent help currently uses schema version 12; object outputs may
publish a versioned nested JSON-pointer field inventory. Each output declares
whether the catalog or a freshly rebuilt wrapper plan governs its result.
`wrapper run` points to the exact `bundle preview` plan schema and publishes
the finite `plan_result_modes` byte, framing, projection, and status contract.

## Current decisions and open work

The current schema supports exact command include/exclude composition,
inherited or narrowed options, identity wrappers, catalog-typed value-option
defaults, appended argv, and typed JSON select/rename/compact-output
transformations. It also supports one finite
original-preserving optimizer contract: Go CLI contract 2 plus an explicitly
inspected official RTK v0.43.0 artifact and the fixed `go-test` filter. Before
and after stage lists are explicit but must remain empty; arbitrary shell,
script, jq, plugin, RTK path/argv, unregistered processor, and runtime-LLM
actions are invalid.

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
- output transformations beyond the schema-5 projection and exact admitted
  optimizer;
- usage-history collection; and
- jq, plugin, or additional external-transformer contracts.

The optimizer implementation and controlled conformance tests are distinct
from release evidence. Historical installed-artifact schema 4 predates the
optimizer, schema 5 adds optimizer evidence, schema 6 adds the first static
tailored-help proof, and schema 7 adds exact caller argv for the multi-command
wrapper. Current evidence schema 8 retains those proofs and adds exact source
argv plus declared/applied option defaults for omitted and caller-overridden
`pr list --limit` cases. The POSIX default-applied, override, and append-only
cases share one bundle and rendered wrapper while retaining distinct caller
argv and plan digests; identity remains independent. Windows retains empty
wrapper/help inventories, zero wrapper attempts, and the structured unsupported
result. Aggregate schema 2 is unchanged. The required five-target schema-8
native replay is pending for this worktree. On 2026-07-22, CI run
[29914651542](https://github.com/tasuku43/atsura/actions/runs/29914651542)
passed all five historical schema-7 rows and aggregate schema 2 for revision
`8dd5b251b9bdd93120ceb5e8b2d3cb0caf24c927`. That is release-quality
implementation evidence for this exact revision, not publication,
independent executable attestation, or evidence for a later revision.

Current plan parsing is deliberately bounded. Source short options,
root/global options, and command-specific positional grammar are not completely
modeled. If a matched command has cataloged descendants, an unknown following
non-dash token is ambiguous rather than assumed positional; use an inner `--`
before positional data. Before the first `--`, exact inline, separated, and
explicit-empty occurrences of a configured long option suppress its default;
repeated caller occurrences remain exact, a short alias never suppresses the
long default, and matching text after `--` is positional. Missing defaults are
inserted in declaration order immediately after the matched command path.
Each non-empty structurally safe UTF-8 value is accepted only when its full
canonical `--option=value` argv element is at most 4096 bytes.
Defaults and appended arguments together are limited to 64 entries.
`append_args` remain at the end even after an existing `--`, and option-looking
values there are positional. Preview requires one
active cataloged selector matching a planned structured input for projection
and optimizer plans. Direct `bundle execute` remains restricted to projection
and additionally requires the exact ordered selector. The ordinary wrapper
runtime separately admits the exact finite identity and append-only
source-stream grammars plus the wrapper-owned `go test -json` optimizer; it
never invents another selector. Competing `--jq`, `--template`, and `--web`
output modes plus unmodeled positional or option syntax fail before source
start. These are compatibility limits, not inferred behavior.

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
option defaults are public in specification, bundle, plan, tailored help, and
evidence and therefore must never contain credentials. Atsura does not acquire
or store provider credentials or persist source output. Source
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
