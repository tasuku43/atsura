# Release Model

## Current pre-release packaging decision

Atsura is not yet published or released. The repository identity is
`tasuku43/atsura`, the binary is `atr`, and MIT is the deliberate project
license. ADR 0005 supersedes the earlier v0.1 local-run product boundary, ADR
0006 accepts the narrow transform runtime, and ADR 0010 admits finite plan-
declared source-stream results for identity and append-argv-only ordinary
wrappers. ADR 0011 adds Go CLI as the nature-distinct second source and ADR
0012 advances it to contract 2 while admitting the exact
`atsura.output.rtk_go_test_pass.v1` processor tuple. The packaging mechanics
below remain reviewed infrastructure, but they do not authorize publication.
ADR 0016 adds one finite catalog-typed value-option default and advances the
current artifacts to specification schema 5, bundle schema 4, plan schema 6,
and generated-wrapper contract 3 without changing the Go, RTK, host, vendor,
or process boundaries.
ADR 0017 adds one private user-local persistent POSIX executable-shim lifecycle
whose caller selects the reported command-resolution directory; Atsura does not
edit `PATH`, startup files, hooks, or vendor settings.
The current implemented runtime is intentionally narrower than the artifact
matrix: one GitHub compatibility-admitted typed JSON transform boundary; one
complete two-command GitHub wrapper combining default-applied and caller-
overridden transformed `pr list` with append-argv-only `issue list`; one
separate identity case; exact no-argument Go `test`
identity; and the processor-bound strict pass-only `go test -json` optimizer
through an explicitly inspected official RTK v0.43.0 artifact. Linux and macOS
have the host-neutral POSIX renderer and managed shim install/status/remove.
Windows retains existing-command runtime evidence and exact structured
unsupported wrapper rendering and managed-shim operations, with no store,
artifact-reference, POSIX activation, or optimizer claim.

Implementation acceptance and release-quality evidence are separate.
Installed-artifact schema 9 and aggregate schema 2 are the current acceptance
mechanism. Schema 9 retains the historical schema-8 optimizer, static-help,
per-case caller-argv, exact source-argv, option-default, and generated-wrapper
contract-3 record and adds the persistent-shim lifecycle. They do not by
themselves establish a release-quality optimizer, managed-shim, tailored-help,
multi-command, or platform claim: that claim belongs only to the workflow
result for one exact revision after all five required native rows and their
aggregate pass. Historical observations do not carry this schema-9 claim
forward to another tree. CI run 29910455312 supplied the historical schema-6 evidence on
2026-07-22 for revision
`01c05a45e8b00f09d63d3c6551d3a5df393c41b5`. No release was created, and
that historical run did not establish schema-7 behavior. CI run 29914651542
then passed all five historical schema-7 rows and aggregate schema 2 on 2026-07-22
for revision
`8dd5b251b9bdd93120ceb5e8b2d3cb0caf24c927`. That is release-quality
implementation evidence for this exact revision's schema-7 contract; no release
was created. CI run
[29920148480](https://github.com/tasuku43/atsura/actions/runs/29920148480)
then passed all five historical schema-8 rows, the canonical
full/security/public
gates, and aggregate schema 2 on 2026-07-22 for revision
`99fbd0e97489b1f3b7a68e2617fa4056b2c12a1d`. That is release-quality
implementation evidence for the option-default contract on that exact
revision; no release was created, and every later candidate must repeat the
matrix.

The current first-release packaging decisions are:

- retain the inherited Linux amd64/arm64, macOS amd64/arm64, and Windows amd64
  pure-Go artifact matrix;
- treat command and schema compatibility as patch-stable within one v0.x minor
  series; a later v0 minor may break only with explicit migration notes;
- permit prerelease tags, created only by the repository owner after the full
  manual public/release review;
- publish, when separately authorized, through GitHub Releases; stable versions
  may update the repository Homebrew Formula, while prereleases do not;
- maintain no additional package manager for the first release;
- claim checksums and reproducible archives, but no code signing, notarization,
  SBOM, or externally verifiable provenance;
- use a GitHub `release` environment for the publish job and require the
  repository owner to configure its human reviewer protection before the first
  tag;
- withdraw a vulnerable or broken release visibly, remove or supersede package
  metadata as appropriate, and publish a new version rather than replacing
  immutable artifacts; and
- require the repository owner to perform and record the final public-boundary
  review.

These decisions fit the current pure-Go artifact and transform-runtime binary
with its generated POSIX function, fixed executable shim, private user-local
store, and no bundled provider SDK or credential store. The selected source CLI
is an external user dependency. Replacement, automatic update, multi-profile
selection, a signing system, an extra package manager, or a non-Go runtime
dependency must revise the matrix and release contract.

The base template defines byte-for-byte reproducible archives within a pinned pure-Go build contract and a public, reproducible-enough overall release path without private package infrastructure. A derived project must review supported platforms, artifact signing, provenance, package managers, and compatibility promises before its first release.

Passing `task release:check` proves artifact mechanics for a commit. It does not
prove that GitHub environment protection is configured and does not create a
release. That external setting and the human review remain first-tag blockers.

## Version contract

Release tags use Semantic Versioning with a leading `v`:

```text
vMAJOR.MINOR.PATCH
vMAJOR.MINOR.PATCH-PRERELEASE
```

Examples are `v1.2.3` and `v1.2.3-rc.1`. Stable tags may update stable package-manager metadata. Prerelease tags publish prerelease artifacts but do not replace the stable Homebrew Formula.

`go run ./tools/releaseversion <tag>` is the single validator used by local packaging and the release workflow. It implements SemVer 2.0 identifier rules, including rejection of leading zeroes in numeric prerelease identifiers. The repository release policy excludes SemVer build metadata even though the SemVer grammar permits it: tags with equal precedence must not identify different immutable artifact sets. Use a new patch or prerelease version instead.

The binary reports the embedded version and commit as:

```text
atr <version> (<commit>)
```

Release checks verify that the displayed values match the tag and source revision.

`task build` is deliberately unversioned and retains the compiled `dev` default. It does not interpolate `git describe`, a tag name, or another repository-controlled string into a shell command. Only `scripts/package-release.sh` injects version and revision metadata, after the shared release-tag validator and full lowercase commit-SHA validation succeed.

## Default platform matrix

The template builds with `CGO_ENABLED=0` for:

| Operating system | Architecture | Archive |
|---|---|---|
| Linux | `amd64` | `.tar.gz` |
| Linux | `arm64` | `.tar.gz` |
| macOS | `amd64` | `.tar.gz` |
| macOS | `arm64` | `.tar.gz` |
| Windows | `amd64` | `.zip` |

Archive names and bytes are deterministic for identical source, tag, full revision, target, and exact Go toolchain:

```text
<binary>_<tag>_<os>_<arch>.tar.gz
<binary>_<tag>_windows_amd64.zip
```

The tag retains its leading `v`, and architectures keep Go-native names such as `amd64` and `arm64`.

A derived project may change this matrix only after updating supported-platform documentation, packaging checks, installation instructions, and package-manager metadata together.

## Published release contents

Each tag publishes:

- one archive for every supported platform tuple;
- `checksums.txt` containing cryptographic checksums for all archives;
- generated release notes or reviewed notes describing user-visible changes;
- prerelease metadata when the tag contains a prerelease suffix.

Every archive contains the intended binary and the repository `LICENSE` bytes.
The reviewed root `THIRD_PARTY_NOTICES` contains the license and NOTICE material
for the exact `go.yaml.in/yaml/v3 v3.0.4` linked dependency, and every archive
contains those exact bytes. The release gate binds that notice review to the
exact module version and required upstream copyright/license lines. No other
member is allowed. `scripts/package-release.sh` builds,
reopens, and verifies each artifact; `task release:check` validates the same
packaging contract over the full matrix.

The packaging command is create-only. It stages and inspects an archive in a temporary directory on the output filesystem, then publishes it with an atomic no-overwrite hard link. It refuses to overwrite an archive that already exists or appears during the build.

Archive creation uses the Go standard library rather than host-specific `tar`, `gzip`, or `zip` creation flags. Inputs are explicit path/name/mode triples and must remain regular files with the same opened identity. Entry basenames are safe, at most 100 bytes for portable USTAR, and unique both byte-for-byte and after case folding; archive order is locale-independent bytewise order. The executable is regular mode `0755`; `LICENSE` and an optional `THIRD_PARTY_NOTICES` are regular mode `0644`. Entries use a fixed UTC modification time, empty user and group names, and numeric user and group IDs of zero where the format carries them. Gzip and ZIP headers use the same canonical time and contain no build-host identity. Output creation is no-overwrite and failed construction removes only the path that invocation created. An independent verifier reopens the completed archive and reviewed inputs, then checks the exact order and member set, modes, raw canonical headers, byte content, zero-filled tar padding, exactly two tar terminator blocks, and absence of extra uncompressed or gzip-member data before the archive is published to the output directory. The packaging boundary forces module mode; fixes `GOAMD64` or `GOARM64` at the portable baseline; sets `GOFIPS140=off`; ignores ambient Go workspace, toolchain, experiment, and flag configuration; and disables implicit Go VCS stamping because the reviewed full revision is already embedded explicitly. Repository release inputs—including the license and optional notice, production and tool source, packaging and Formula policy, workflow configuration, and project metadata—must be regular files and are content-fingerprinted through the final release check; dependency modules are verified around each archive pass; and local filesystem module replacements are rejected because their source would sit outside the public release input boundary. The exact Go version in `go.mod`, `-trimpath`, the source bytes, tag, revision, target, and verified module graph are part of the reproducibility input. This contract does not promise equal bytes across different Go versions or establish who performed a build.

## Executable release profile

`task release:check` performs two independent complete local matrix package passes, not a host-only approximation. It compares each corresponding digest, then reuses the primary five archives for every remaining check. The profile:

1. requires the exact Go toolchain selected by `go.mod`;
2. independently builds Linux `amd64` and `arm64`, macOS `amd64` and `arm64`, and Windows `amd64` twice with `CGO_ENABLED=0` and separate Go build caches;
3. fingerprints release inputs before and after each pass and after the remaining release checks, reports source drift separately, and proves byte-for-byte reproducibility by comparing every corresponding archive digest across the two output directories;
4. independently reopens each primary archive and verifies the exact executable,
   license, optional-notice member set, canonical metadata, and reviewed bytes;
5. extracts every primary archive, compares every supporting file byte-for-byte,
   and checks the executable's Go module, `GOOS`, and `GOARCH` build metadata;
6. creates `checksums.txt`, proves it has a one-to-one correspondence with all five primary archives, and recomputes every digest;
7. positively renders a stable Homebrew Formula from the real macOS archive checksums and verifies its URLs, digests, version, class, and placeholder removal;
8. runs `ruby -c` against the rendered Formula; and
9. exercises the isolated-tap ownership test for Formula audit cleanup.

The profile requires `tar`, `unzip`, either `sha256sum` or `shasum`, ShellCheck `0.9.0` or newer, and Ruby. The canonical `release` profile preflights these system commands before tests or matrix builds; the release lint still validates ShellCheck's compatibility floor. Archive creation itself has no host `zip` dependency. ShellCheck covers every publishable `.sh` file rather than a hand-maintained subset. It is a system prerequisite with an explicit compatibility floor, not an exact repository pin: the floor accepts the `0.9.0` analyzer supplied by the documented Linux runner and newer compatible analyzers such as `0.11.x`. A missing or older ShellCheck, or a missing Ruby executable, is a failed release check rather than a skipped check. A developer without these tools must use the documented CI release gate and treat its result as required evidence before tagging.

The workflow runs `full`, `security`, `release`, and `public` explicitly in the Ubuntu preflight. The later macOS Formula job is deliberately narrower: it renders the checksum-pinned Formula, runs `ruby -c`, and performs the real Homebrew strict audit. It does not repeat `check.sh release`, because that would rebuild the complete five-target verification matrix on a different host and would incorrectly make Formula publication depend on Linux preflight tools such as ShellCheck being installed on the macOS runner. The Formula job consumes only artifacts produced after the preflight and build jobs succeed.

## Release workflow

The release workflow follows this order:

1. Validate tag syntax and resolve its exact source revision.
2. Run source, security, release, and public-boundary gates required by policy.
3. Build the complete pure-Go platform matrix from that revision.
4. Download each build artifact on its matching native runner and replay the
   installed-artifact transform and wrapper journey with synthetic source
   fixtures and no provider credentials. Linux and macOS activate generated
   ordinary-command functions for the four GitHub cases, one exact identity-
   wrapped no-argument Go `test`, and the finite RTK-backed optimizer cases.
   On each POSIX row the transformed-PR wrapper also serves exact root,
   namespace, and command `--help`, including exact-command default disclosure,
   while its bound `atr` is non-executable,
   then proves the hidden and unknown help-shaped fallthrough faults without a
   source or processor attempt.
   Each optimizer row receives one separately verified official RTK v0.43.0
   archive and materializes the optimizer only through explicit processor
   inspection. Each candidate archive is opened once for a bounded read; its
   digest and extracted bytes derive from that same immutable in-memory value.
   Windows receives no RTK artifact and verifies exact structured unsupported
   rendering plus an explicit empty unsupported tailored-help proof without
   claiming POSIX activation or an optimizer.
   Each POSIX row also installs the exact GitHub and Go bundles, obtains their
   opaque artifact references only from status, places the reported `bin`
   directory first in fixture-owned `PATH`, and invokes ordinary `gh`/`go`
   help and normal commands. Their bundle/runtime binding, fresh plan, argv,
   result, and attempt evidence must equal the existing `wrapper run` cases.
   Exact-reference removal must delete only the selected owned artifacts and a
   final status must be explicitly empty. Tamper, foreign regular-file,
   symlink, special-file, and unknown-reference probes must fail closed with
   unchanged fixture-owned filesystem state and zero source/processor attempts.
   Windows records structured unsupported install/status/remove with zero
   store/source/processor attempts and no artifact references.
5. Aggregate exactly five bounded native evidence documents with their matching
   candidate Atsura archives. Recompute every candidate digest and verify each
   applicable row's recorded processor provenance against the code-pinned
   manifest, together with target, version, revision, journey counts, fault
   set, plan identities, dispositions, and leak checks. Processor archives are
   verified locally by the native rows and are not uploaded as Atsura release
   artifacts.
6. Verify archive names, contents, version, commit, and native executable behavior.
7. Generate and verify `checksums.txt`.
8. Publish one GitHub Release only after the five native replays and their
   aggregate succeed.
9. For a stable tag, render the checksum-pinned Homebrew Formula and open a Formula update pull request.

Steps 4 and 5 are implemented by evidence schema 9, aggregate schema 2, and the
native workflow. Their presence is not an attestation for a moving worktree.
Publication remains blocked until every required native row and its aggregate
succeed for the exact candidate revision.

The workflow uses a public GitHub Release path. It must not embed private asset URLs, personal access tokens, authorization headers, or organization-specific package infrastructure in Formula content.

Publication is create-only. If a GitHub Release already exists for the tag, the workflow fails without uploading or replacing any asset. It never uses `--clobber`. Correct a failed or incorrect release with a new version and an explicit incident or withdrawal decision; do not silently rewrite published evidence.

Workflow checkouts disable persisted Git credentials. The Formula pull-request action receives only its explicitly scoped workflow token, while source checkout does not leave that token in Git configuration.

Every matrix build checkout and Formula generation step is bound to `needs.preflight.outputs.revision`, not an implicit event checkout or the moving `main` branch. The workflow renders and strictly audits the Formula from that exact release revision and its project metadata into runner-owned temporary storage. Only after audit succeeds does it check out current `main`, copy the already-audited Formula into `Formula/`, and open the reviewable pull request. Changes to identity, templates, or generation scripts on `main` therefore cannot race the tagged artifacts and checksums used as generation input.

## Homebrew contract

Stable releases support macOS `arm64` and `amd64` through a generated Formula. The Formula:

- selects the archive matching the user's CPU;
- uses the public release URL;
- pins the exact checksum;
- installs the binary without cloning source or requiring a Go toolchain;
- contains no unreplaced template value or private authentication behavior.

`scripts/render-formula.sh` renders the project Formula from the reviewed template. Formula changes are proposed through a pull request so the generated diff and release references are visible before merge.

`scripts/audit-formula.sh` creates a collision-resistant temporary tap name from `mktemp`, verifies that name is not already installed, and records ownership only after `brew tap-new` succeeds. Its exit trap removes only that owned tap. It never pre-emptively untaps a fixed name, so an existing user tap is outside its cleanup authority. The release profile tests this property with a fake Homebrew boundary on both audit success and audit failure.

Prereleases do not update stable Formula metadata.

## Release preparation

Create a temporary work packet for a release and record:

- target version and rationale;
- included changes and compatibility impact;
- security fixes and disclosure coordination;
- migration or deprecation notes;
- required profiles and their results;
- clean-environment installation evidence;
- artifact and checksum verification;
- public-boundary review.

After rollout, promote stable procedure and policy into this document or an ADR.
Delete the ordinary packet from the final tree. Retain it as
`Retention: evidence` only when it contains unique manual rollout, incident, or
external-system observations that cannot be reconstructed from the immutable
Release, workflow run, tests, and commit; state its governing contract and
review/delete trigger in `goal.md`.

Before tagging, run:

```sh
task check
task security
task release:check
task public:check
```

Then review the exact commit that will receive the tag. A clean local run does not authorize tagging a different revision.

Before a first release is approved, release preparation must replay the current
public artifact, adoption-receipt consumption, `bundle preview`, and `bundle
execute` scenarios against the exact release artifacts on matching native
runners. Preview must require the adopted current schema-4 bundle, reproduce
its canonical schema-6 plan digest, report `source_process_attempts: 0`, and
start no processor. Execute must rebuild the same plan and report exactly one
source attempt only after compatibility succeeds. Direct `bundle execute` remains the GitHub
projection-evidence command and must reject an optimizer plan before either
process starts.

The same candidate must expose exact `wrapper render` and `wrapper run` scoped
contracts. On Linux and macOS, release preparation renders deterministic POSIX
function bytes from the shared multi-command and separate identity bundles,
compares each SHA-256, activates each in a generic caller-owned shell, and
invokes ordinary `gh` for default-applied and caller-overridden transformed
JSON, append-argv-only, and identity cases. Each invocation
must rebuild the same schema-6 plan as preview and add exactly one source
attempt. Both transformed cases emit one compact plan-declared JSON value. The
other two return exact bounded source streams and conventional status with no
added framing or projection. On Windows, POSIX rendering must return
the exact structured `wrapper_platform_not_supported` fault, no source bytes or
digest, and zero wrapper source attempts. This is a platform boundary, not a
skipped test.

The same candidate must obtain a stable Go 1.26.x effective-toolchain
observation with
exactly `go version`, `go help`, and `go help test`, then build and adopt one
exclude-by-default bundle containing only identity-wrapped `test`. Linux and
macOS render the ordinary wrapper, first invoke `go test extra`, and require
`wrapper_runtime_not_supported`, exit 12, and zero Go attempts. They then invoke no-
argument `go test` once in the dependency-free synthetic module, bind the
preview and wrapper plan digest, and retain only stdout/stderr digests plus
conventional status. Windows performs the same
three-probe inspection but must record the exact unsupported render with one
zero-attempt rejection, no rendered case, and zero Go wrapper source attempts.
This fixture sets `GOTOOLCHAIN=local`, isolates module/cache roots, and disables
downloads as test discipline; those are not production wrapper guarantees and
do not sandbox the Go process or narrow source-owned effects.

The four Linux/macOS native rows must also use an explicitly supplied official
RTK v0.43.0 archive from the pinned processor manifest. They verify the
archive's target, size, and SHA-256 before extracting one regular executable,
run packaged `atr processor inspect`, and carry that schema-1 observation
through `spec init --processor`, `bundle build --processor`, adoption,
schema-6 preview, rendering, and ordinary no-argument `go test`. The exact
contract is `atsura.output.rtk_go_test_pass.v1`: source-catalog schema 2, Go CLI
contract 2, source argv `go test -json`, and processor argv
`pipe --filter=go-test` under host-neutral
`atsura.processor.rtk_isolated.v2`. Native replay must bind the exact caller,
source, and processor argv, formats, modes, one-attempt ceilings, timeouts, and
byte bounds, then prove an independently validated `optimized` strict pass and
a reachable `preserved_before_processor` result. Retired v1 processor evidence
is incompatible.
Windows receives no processor artifact and records the structured unsupported
renderer result with zero source attempts and no processor evidence. Controlled
application and infrastructure tests, not a manufactured official-artifact
journey, own `preserved_after_processor` and arbitrary processor-failure
branches.

Processor evidence is limited to the facts the harness actually observes:
artifact and executable identity, fixed invocation, bounded input/output,
status, disposition, source-fixture attempt counts, and the processor-inspection
attempt. Without a separate external observer, the native journey does not
claim processor-launch counts or that RTK created no child process, accessed no
filesystem path, or performed no network activity.

The current executable runtime claim is limited to all of the following:

- a strict schema-5 tailoring specification containing a transform wrapper
  with JSON input and compact JSON output;
- GitHub CLI adapter contract 2, GitHub CLI major 2, and a successful exact
  four-probe inspection;
- exact commands `issue list` and `pr list`;
- exactly one inline `--json=<fields>` selector before `--`, with its field
  order equal to the plan's `select` order;
- one finite `pr list --limit` value-option default, inserted as
  `--limit=<value>` immediately after the command path only when the exact long
  option is absent before the first `--`; valid non-empty inline, separated,
  and repeated caller forms win. At the generic plan boundary an explicit
  empty value suppresses the configured default, but this finite GitHub runtime
  rejects both inline and separated empty values before source start. Short
  aliases are unmodeled, and matching positional text after `--` does not
  suppress the default; neither form is admitted by this runtime tuple. The
  full canonical argv element must fit `sourceprocess.MaxArgumentBytes` (4096
  bytes);
- default values are public in the specification, bundle, plan, exact-command
  help, and evidence; they are not credential storage and never become shell
  source;
- rejection of competing `--jq`, `--template`, and `--web` output modes plus
  unmodeled option or positional syntax;
- every observable executable identity matching the bound path/hash/size, no
  shell, closed stdin, inherited working directory and environment, one maximum
  attempt, 30-second timeout, 4 MiB stdout, and 256 KiB stderr limits;
- empty stderr on success, selected/renamed typed JSON only, and no persistence
  or public inclusion of raw stdout, raw stderr, or unselected fields; and
- non-retryable classification for every post-start or final-output failure.

The second-source executable claim is separately limited to all of the
following:

- Go CLI adapter contract 2 and a successful exact three-probe inspection whose
  recorded effective-toolchain observation is stable Go 1.26.x;
- one complete included surface containing only exact command `test`, no
  observed long-option or structured-output grammar, and an identity wrapper;
- exact source argv `test`, with every option, package pattern, `--` marker,
  test-binary argument, and other command rejected before source start;
- `source_stream_passthrough` through the same finite compatibility registry,
  fresh schema-6 plan, identity-bound no-shell process port, byte bounds, and
  conventional-completion contract as the existing ordinary wrapper; and
- `EffectExecute` with no claim that Go test is read-only, authorized, or
  contained; repository code, module resolution, credentials, network access,
  caller-owned mutations, and effective toolchain selection remain source-owned.

For this contract, path/hash/size identify the direct `go` launcher, while
`Source.Version` is the possibly delegated effective toolchain observed by
`go version` under inspection conditions. Production runtime does not repeat
that observation or bind the selected/downloaded toolchain or GOROOT tree. A
later different selection by the same launcher is not a pre-start rejection.
Any future release claim that constrains it requires an explicit
environment/toolchain closure, successor ADR, and native matrix evidence.

The host-neutral wrapper claim is additionally limited to all of the following:

- `wrapper render --bundle <absolute-path> [--format text|json]` on Linux or
  macOS, with a portable POSIX requested executable name outside the maintained
  reserved/fixed and implementation-specific function-name set;
- one complete included surface selected by the shared finite compatibility
  registry: either GitHub CLI `issue list` / `pr list` under contract 2, or
  exact Go `test` under contract 2 with either the identity wrapper or the
  processor-bound finite optimizer, with every exposed option and concrete
  invocation inside that adapter's maintained grammar;
- exactly one schema-6 result mode: `transformed_json` for the existing typed
  projection, `source_stream_passthrough` for a complete identity or fixed-
  argv-append-only wrapper without an output stage, or
  `original_preserving_optimizer` for the exact RTK-backed Go tuple;
- fixed Atsura-generated function source containing the exact bundle digest and
  current absolute `atr` path/hash/size, root structured errors, the public
  `wrapper run` contract version 3, bounded bundle-derived root/namespace/exact
  final-`--help` material, an explicit `--`, and lossless non-help `"$@"`;
- static tailored-help success starts no bound `atr`, source, or processor and
  names the full bundle digest; exact-command help discloses configured option
  defaults while root and namespace indexes remain unchanged. It describes the
  exact rendered artifact rather than current executability, readiness,
  authorization, or attestation;
- honest runtime validation of that closure, followed by the same fresh-plan
  application and exact source process boundary as `bundle execute`, plus the
  separately identity-bound processor boundary only for the optimizer mode;
- successful transformed output consisting of exactly one compact plan-
  declared JSON object or array plus LF and empty stderr, or a conventionally
  completed source-stream result consisting of exact bounded stdout, stderr,
  and source status with no framing, projection, timing, or cross-stream-
  interleaving claim; or one of the optimizer dispositions
  `preserved_before_processor`, `preserved_after_processor`, or `optimized`;
  none has a maintainer evidence envelope; and
- caller-owned activation for rendered functions, no raw fallback, no coding-
  agent-host protocol, and no claim that the runtime binding attests against
  malicious replacement of the bound executable.

The persistent host-neutral shim claim is narrower still:

- `wrapper install --bundle <absolute-path>` creates at most one exact
  fixed-template executable per ordinary command in the private platform-
  configuration-root store. It reports the store's `bin` directory, produces
  no opaque reference, starts no source or processor, and never replaces a
  different artifact or foreign path;
- `wrapper status` performs bounded read-only ownership validation and is the
  sole producer of an opaque artifact reference bound to immutable manifest
  and shim material;
- `wrapper remove --artifact <reference>` consumes that status reference
  unchanged and deletes only the exact revalidated owned artifact. Unknown,
  tampered, foreign, symlinked, special, multiply matched, or uncertain state
  is never deleted; and
- activation remains caller-owned. Atsura neither edits nor claims containment
  of `PATH`, shell startup files, hooks, host settings, source behavior, or
  downstream authorization. Windows creates no store and returns structured
  unsupported for all three lifecycle commands.

For `source_stream_passthrough`, a conventional nonzero status remains a source
result rather than an Atsura fault. Signal termination, timeout, cancellation,
capture overflow, wait uncertainty, identity uncertainty, and inconsistent
process evidence are non-retryable and expose neither captured stream. Final
delivery writes complete stdout, then complete stderr, and only then returns
the source status. A final writer failure may leave partial caller-visible
bytes, returns non-retryable `execute_output_write_failed`, and never recommends
replay. No source-stream byte is persisted or copied into a fault or evidence
document.

For `original_preserving_optimizer`, Atsura independently admits only the
strict single-package pass lifecycle and calculates its exact shorter summary
before processor start. Conventional ineligible results—including skip,
failure, malformed or unknown JSON, empty output, source stderr, nonzero
status, and a non-beneficial summary—return exact source bytes and status as
`preserved_before_processor` after one source and zero processor attempts.
Eligible input reaches the bound RTK process once after a second identity
check. Successful output may be only the byte-identical admitted input as
`preserved_after_processor` or the independently calculated newline-free
summary as `optimized`. Once processor authority begins, every processor
failure is non-retryable, exposes no source or processor bytes, and never
selects original output as fallback.

The credential- and provider-network-free synthetic GitHub-compatible native
fixture runs through the exact archived `atr`, production composition,
verifier, runner, parser, transformer, and renderers and is the canonical
automated artifact gate. It first verifies schema-12 root help
and the exact scoped authoring/runtime contracts, including `processor
inspect`, `wrapper render`, and `wrapper run`, the complete nested catalog and
specification field inventories and the complete ordered 27-fault preview and
41-fault execute recovery signatures. Every induced fault must then equal its
packaged declaration. Its append-only log must prove four inspection attempts,
zero preview attempts, exactly one direct success attempt for each of `issue
list` and `pr list`, and either four ordinary-command wrapper attempts on
Linux/macOS or zero on Windows. The fixed total is therefore 14 fixture
attempts on a POSIX wrapper target and 10 on Windows; channel-specific canaries
prove that raw failure data and
unselected fields do not reach public output or isolated state. The
non-shipped harness seeds each exact receipt into its ephemeral user-config
root through the production trust-store adapter. This is explicitly
receipt-consumption evidence, not evidence that automation provided human
consent. Full-digest controlling-terminal success and redirected-input
rejection are proven separately by production-adapter and application tests.

The second-source portion uses a directly identified Go launcher, requires its
fixture observation to be stable Go 1.26.x, and runs a dependency-free
synthetic module with `GOTOOLCHAIN=local`, isolated cache/config roots, and
downloads disabled. Its separate bounded evidence proves three inspection attempts,
command `test`, zero preview attempts, and one zero-attempt rejection on every
target. Linux/macOS then record one ordinary Go wrapper source attempt; Windows
records the unsupported-render outcome and zero wrapper attempts. These counts
do not change the GitHub fixture totals above and do not claim that the Go
process is sandboxed.

The existing bounded artifact-journey schema 4 is the pre-optimizer baseline.
On Linux and macOS its GitHub section records `wrapper_outcome:
ordinary_command_verified`, an ordered three-entry `wrapper_cases` inventory,
and `wrapper_source_process_attempts: 3`. The entries represent transformed-
JSON, identity, and append-argv-only ordinary invocations in the fixed order
`transformed_json`, `identity`, `append_only`. Each entry binds its
`name`, `wrapper_kind`, `result_mode`, `bundle_digest`, `plan_digest`,
`wrapper_source_sha256`, `stdout_sha256`, `stderr_sha256`,
`source_exit_code`, and `source_process_attempts: 1` without storing either
stream. Windows instead records
`wrapper_outcome: platform_not_supported`, an empty `wrapper_cases` inventory,
and zero wrapper source attempts.

That baseline additionally requires `go_source` with exact
`adapter_kind: atsura.source.go_cli`, the historical contract version 1, a
recorded stable `go1.26.x` observation,
three inspection attempts, `commands_verified: ["test"]`, and separate catalog,
bundle, and plan digests. Linux/macOS record
`wrapper_outcome: ordinary_command_verified`, one `go_test_identity` case,
identity wrapper, `source_stream_passthrough`, nonempty rendered-wrapper and stdout digests, empty
stderr digest, status zero, one source attempt, and one preceding zero-attempt
`go test extra` rejection. Windows records
`wrapper_outcome: platform_not_supported`, an empty case list, one zero-attempt
rejection, and zero Go wrapper source attempts. The fixed GitHub fixture-attempt
total remains 13 on a POSIX target and 10 on Windows. The aggregator validates
both source-specific platform alternatives without treating unsupported POSIX
activation as a skipped success.

Historical evidence schema 4 cannot support the accepted optimizer or current
Go contract 2 release claim. Schema 5 retains the base GitHub and Go identity
facts while binding Go contract 2, processor-observation schema 1, the exact
processor artifact and executable identity, fixed RTK invocation, schema-3
bundle and schema-5 plan digests, exact caller/source/processor argv, formats,
process modes, v2 isolation and bounds, source-fixture attempt counts,
processor-inspection evidence, result disposition and status, and bounded leak
checks. It is optimizer-aware but predates static tailored help.

Historical schema 6 retains the complete schema-5 proof and adds one bounded
`tailored_help` object for the transformed-PR wrapper. Historical schema 7 keeps
that object and adds exact `caller_argv` to every `wrapper_cases` entry. Each
POSIX row orders the cases as `transformed_json`, `append_only`, and `identity`.
The first two use caller argv
`["pr","list","--limit=1"]` and
`["issue","list","--search=append value","--label=one","--label=two"]`,
bind the same complete bundle and rendered-wrapper digest, and require distinct
schema-5 plan digests. The identity case retains its separate bundle, wrapper,
plan, and hostile caller argv. The shared wrapper records the five exact views
`["--help"]`, `["issue","--help"]`,
`["issue","list","--help"]`, `["pr","--help"]`, and
`["pr","list","--help"]` while the extracted `atr` is temporarily non-
executable. It then restores that runtime and records `["api","--help"]` as
`command_not_in_surface` and `["unknown","--help"]` as
`invalid_invocation`, both with zero source and processor attempts. The three
ordinary calls add one source attempt each and retain the POSIX GitHub fixture
total of 13. Windows records `platform_not_supported`, empty wrapper cases,
help views, and fallthrough inventories, no tailored-help bundle or rendered-
wrapper binding digests or wrapper contract, zero wrapper attempts, and 10
GitHub fixture attempts. Top-level journey identities remain required. Its Go
case is also empty; POSIX Go identity evidence records caller argv `["test"]`.

The four POSIX rows must also record `optimized` and reachable
`preserved_before_processor`; Windows records no optimizer case or processor
evidence. Without an accepted external observer, installed evidence does not
claim processor-launch counts; controlled application and infrastructure tests
own that truth. Schema 7 and unchanged aggregate schema 2 implement that
historical proof mechanism; aggregate schema 2 intentionally does not project
the per-case caller argv. The workflow result for one exact candidate revision owns
whether the required five-target evidence exists. The inherited schema-5 optimizer
shape keeps the identity case in the outer `go_source` wrapper fields and
records the optimizer's separate bundle, plan, rendered-wrapper digest, cases,
and faults in the nested `optimizer` object.

Historical evidence schema 8 advances the production artifacts to specification
schema 5, bundle schema 4, plan schema 6, and generated-wrapper contract 3. It
records exact source argv plus complete declared and applied option-default
lists. POSIX rows order four cases as `default_applied`,
`default_overridden`, `append_only`, and `identity`; the first three share one
two-command bundle and rendered wrapper while retaining distinct plans. The
first caller omits `--limit`, the second supplies `--limit=2`, and both exact
source argv prove the precedence result. POSIX totals become four wrapper
source attempts and 14 GitHub fixture attempts. Windows remains zero wrapper
attempts and 10 fixture attempts. The Go/RTK record and aggregate schema 2 are
unchanged. CI run 29920148480 passed the exact five-target schema-8 result on
2026-07-22 for revision
`99fbd0e97489b1f3b7a68e2617fa4056b2c12a1d`.

Current evidence schema 9 retains that complete schema-8 record and adds one
required persistent `wrapper_lifecycle` record. A POSIX record binds shim
contract version 1, a digest of the reported `bin` directory, caller-owned
PATH-first selection, status-produced opaque references, immutable material
digests, the existing GitHub and Go bundle/plan/argv/result identities,
ordinary help and execution outcomes, exact-reference removals, explicit final
empty status, store/source/processor counters, hostile fault codes, and
unchanged-filesystem observations. A Windows record carries no bin, material,
bundle, plan, or reference claim; it records empty path-command, status-
snapshot, and artifact collections, three structured unsupported install/
status/remove faults, and zero store/source/processor attempts. Evidence stores
no raw filesystem path, stdout/stderr stream,
environment snapshot, credential, or secret, and adds no host, vendor, hook,
settings, or activation field.

Aggregate schema 2 remains unchanged. It validates exactly five strict schema-9
rows and their candidate archive bindings, then emits only the path-free
`workflow_index_unattested` digest index. Per-row paths, opaque artifact
references, material digests, and bundle/plan identities do not enter the
aggregate. This is the current acceptance mechanism. It becomes evidence for a
candidate only when that exact revision passes all five native rows and their
aggregate.

The credential-free in-process production-composition fixture supplies the
complete phase evidence that a portable exact-archive journey cannot safely
force: 27 preview zero-attempt cases, 28 execute pre-start cases, and 15
execute post-start cases across the 41 execute codes. It uses the production
identity reader for actual file drift and narrow controlled ports for
deterministic boundary observations. Infrastructure tests prove production
file, trust, identity, and process fault emission, including native
start/wait/identity races. Defensive request and encoding faults are exercised
at their owning boundary without a shipped fixture mode. The execute encoding
case corrupts the result only after the production service and controlled
process complete one real attempt; production application/domain tests prove
the undecorated result. Every native CI artifact row runs these contracts and
the production source-runner tests before archive replay, and
`lint-release.sh` pins the exact commands.

The accepted major-2 range is a maintained compatibility decision, not a claim
that one fixture proves every future 2.x release. A live GitHub CLI observation
is optional supporting evidence and must not persist account, repository,
pull-request, or raw source data. Packaging metadata and cross-compilation do
not prove runtime behavior. The release workflow blocks publication on exact-
archive native replay for Linux amd64/arm64, macOS amd64/arm64, and Windows
amd64. Each replay uploads one bounded JSON document, and a separate job
requires the exact five-document, five-candidate-archive, one-revision set,
recomputes every archive digest, and emits an explicitly unattested index
before publication. That job does not rebuild or rerun a binary. If a native
runner is unavailable, the release fails or the platform claim must be
revised; emulation is not substituted.

Recorded stable Go 1.26.x inspection evidence is likewise an explicit contract,
not evidence for every future patch or for the toolchain selected during a
later wrapper invocation. A catalog recording another version, options,
package patterns, `--`, and test-binary arguments remain outside the first
runtime. The same launcher selecting Go 1.27 later is not detected by contract
2. Ambient Go configuration and toolchain selection remain source-owned. The
pass-only `go test -json` / RTK `go-test` optimizer is now the one accepted
external-processor implementation tuple. Current evidence schema 9 and
aggregate schema 2 can carry that optimizer claim together with the contract-2
multi-command tailored-help, option-default, and managed-shim claim, but only a
successful five-target native run for one exact candidate revision can advance
the implementation to a release-quality platform claim.

Matrix artifacts use stable, target-unique names and explicit replacement of
the prior same-name artifact. Candidate archives, native journey evidence, and
aggregate summaries use disjoint prefixes. This avoids immutable-name
collisions on a rerun while strict revision, filename, target, and digest
checks still fail closed on any absent or stale input.

Historical bounded observation: on 2026-07-22 CI run 29910455312 passed the exact
Linux amd64/arm64, Darwin amd64/arm64, and Windows amd64 schema-6 journeys,
full/security/public gates, and aggregate schema 2 for revision
`01c05a45e8b00f09d63d3c6551d3a5df393c41b5`. This establishes the implemented
optimizer, one-command tailored-help, and platform contracts for that revision
only. It is not schema-7 multi-command evidence, publication, attestation, or
evidence for another commit or tag; the exact revision selected for any release
must replay all required rows again.

Historical bounded schema-7 observation: on 2026-07-22 CI run 29914651542
passed the exact Linux amd64/arm64, Darwin amd64/arm64, and Windows amd64
schema-7 journeys,
full/security/public gates, and aggregate schema 2 for revision
`8dd5b251b9bdd93120ceb5e8b2d3cb0caf24c927`. This establishes the implemented
optimizer, multi-command tailored-help, and platform contracts for that
revision only. It is not publication, independent executable attestation, or
evidence for another commit or tag; the exact revision selected for any release
must replay all required rows again.

Historical bounded schema-8 observation: CI run
[29920148480](https://github.com/tasuku43/atsura/actions/runs/29920148480)
passed the exact Linux amd64/arm64, Darwin amd64/arm64, and Windows amd64
schema-8 journeys, canonical full/security/public gates, and aggregate schema
2 on 2026-07-22 for revision
`99fbd0e97489b1f3b7a68e2617fa4056b2c12a1d`. This establishes the
option-default implementation claim for that revision only. It is not
publication, independent executable attestation, or evidence for another
commit or tag.

Schema 9 is the implemented acceptance mechanism for an exact candidate. A
bounded observation belongs only to the revision whose five native workflows
and dependent aggregate passed; it must not be inferred from the historical
schema-8 run or carried forward to a later revision.

No public release has yet made the wrapper claim. A future candidate that
passes the complete gates may claim only the fixed Linux/macOS POSIX
materialization and managed-shim lifecycle, the exact finite source/processor
tuples, and the Windows unsupported contract above. It still has no claim for
identity or argv transforms beyond the exact admitted cases, live stream timing
or interleaving, signal-status passthrough, raw execution, arbitrary
transformers, artifact replacement or automatic update, multi-profile
selection, Atsura-owned `PATH` or shell-startup edits, coding-agent-host
activation, executable attestation, RTK child-process/filesystem/network
absence, Windows shim support, or Windows POSIX activation.
The retired legacy `plan preview`/`run` slice is not runtime evidence.

## Failure and recovery

- If preflight or any matrix build fails, publish nothing.
- If the artifact set is incomplete, do not create a partial stable release.
- If a checksum or Formula reference is wrong, correct it through a reviewed replacement release or metadata change; do not silently mutate evidence.
- If the tag already has a GitHub Release, stop. Do not overwrite its assets or rerun publication as an update operation.
- If sensitive content reaches a release, stop distribution, revoke affected credentials, preserve incident evidence, and follow the security response process.
- Do not reuse a stable version for different source or artifacts.

## Signing and provenance

The base template does not claim code signing, notarization, or externally verifiable build provenance. Checksums detect accidental corruption after a trusted release is selected; they do not establish who produced it.

A derived project that needs stronger guarantees must document and test:

- signing identity and key protection;
- verification instructions;
- provenance format and builder trust;
- rotation and revocation;
- platform-specific installation consequences.

Absence of these controls must remain visible in the security model and release notes rather than being implied away.

## Future release decisions

Revisit the accepted first-release choices when evidence requires changing:

1. supported and tested platforms;
2. the compatibility-stability threshold;
3. prerelease authorization;
4. maintained package managers;
5. signing, notarization, SBOM, or attestation requirements;
6. withdrawal procedure;
7. workflow permissions or protected environments; or
8. the manual public-boundary reviewer.

Record durable trade-offs in an ADR and update `task release:check` so the chosen contract is executable.
