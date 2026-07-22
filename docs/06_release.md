# Release Model

## Current pre-release packaging decision

Atsura is not yet published or released. The repository identity is
`tasuku43/atsura`, the binary is `atr`, and MIT is the deliberate project
license. ADR 0005 supersedes the earlier v0.1 local-run product boundary, and
ADR 0006 accepts the narrow transform runtime. The
packaging mechanics below remain reviewed infrastructure, but they do not
authorize publication. The current runtime claim is intentionally narrower
than the artifact matrix: one compatibility-admitted typed JSON transform
boundary plus fixed host-neutral POSIX wrapper rendering and invocation on
Linux and macOS. Windows retains existing-command runtime evidence and exact
structured unsupported wrapper rendering; no Windows POSIX activation claim is
made.

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
with its generated POSIX function and no bundled provider SDK or credential
store. The selected source CLI is an external user dependency. A later
persisted wrapper lifecycle, executable shim, signing system, extra package
manager, or non-Go runtime dependency must revise the matrix and release
contract.

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
When a reviewed root `THIRD_PARTY_NOTICES` file exists, every archive contains
those exact bytes as well; absence means there is no notice entry, not an empty
placeholder. No other member is allowed. `scripts/package-release.sh` builds,
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
   installed-artifact transform and wrapper journey with no provider
   credentials or provider network. Linux and macOS activate the generated
   ordinary-command function; Windows verifies the exact structured
   unsupported-render contract and continues the existing command journey.
5. Aggregate exactly five bounded native evidence documents and verify their
   target, archive digest, version, revision, journey counts, fault set, plan
   identities, and leak checks.
6. Verify archive names, contents, version, commit, and native executable behavior.
7. Generate and verify `checksums.txt`.
8. Publish one GitHub Release only after the five native replays and their
   aggregate succeed.
9. For a stable tag, render the checksum-pinned Homebrew Formula and open a Formula update pull request.

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
runners. Preview must require the adopted current bundle, reproduce its
canonical plan digest, and report
`source_process_attempts: 0`. Execute must rebuild the same plan, report the
same digest, and report exactly one attempt only after compatibility succeeds.

The same candidate must expose exact `wrapper render` and `wrapper run` scoped
contracts. On Linux and macOS, release preparation renders deterministic POSIX
function bytes from one absolute adopted bundle, compares their SHA-256,
activates them in a generic caller-owned shell, invokes ordinary `gh` with exact
argv, and observes the same fresh plan plus one compact plan-declared JSON value
and one source attempt. On Windows, rendering must return the exact structured
`wrapper_platform_not_supported` fault, no source bytes or digest, and zero
wrapper source attempts. This is a platform boundary, not a skipped test.

The current executable runtime claim is limited to all of the following:

- a strict schema-3 transform wrapper with JSON input and compact JSON output;
- GitHub CLI adapter contract 2, GitHub CLI major 2, and a successful exact
  four-probe inspection;
- exact commands `issue list` and `pr list`;
- exactly one inline `--json=<fields>` selector before `--`, with its field
  order equal to the plan's `select` order;
- rejection of competing `--jq`, `--template`, and `--web` output modes plus
  unmodeled option or positional syntax;
- every observable executable identity matching the bound path/hash/size, no
  shell, closed stdin, inherited working directory and environment, one maximum
  attempt, 30-second timeout, 4 MiB stdout, and 256 KiB stderr limits;
- empty stderr on success, selected/renamed typed JSON only, and no persistence
  or public inclusion of raw stdout, raw stderr, or unselected fields; and
- non-retryable classification for every post-start or final-output failure.

The host-neutral wrapper claim is additionally limited to all of the following:

- `wrapper render --bundle <absolute-path> [--format text|json]` on Linux or
  macOS, with a portable non-reserved POSIX requested executable name;
- one complete included transform surface, either GitHub CLI `issue list` or
  `pr list`, whose exposed options all belong to the maintained runtime grammar;
- fixed Atsura-generated function source containing the exact bundle digest and
  current absolute `atr` path/hash/size, root structured errors, the public
  `wrapper run` contract version 1, an explicit `--`, and lossless `"$@"`;
- honest runtime validation of that closure, followed by the same fresh-plan
  application and exact source process boundary as `bundle execute`;
- successful stdout consisting of exactly one compact plan-declared JSON object
  or array plus LF, empty stderr, and no maintainer evidence envelope; and
- caller-owned activation, no persisted install/shim lifecycle, no raw fallback,
  no coding-agent-host protocol, and no claim that the runtime binding attests
  against malicious replacement of the bound executable.

The credential- and provider-network-free synthetic GitHub-compatible native
fixture runs through the exact archived `atr`, production composition,
verifier, runner, parser, transformer, and renderers and is the canonical
automated artifact gate. It first verifies schema-9 root help and seven exact
scoped authoring/runtime contracts, including `wrapper render` and `wrapper
run`, the complete nested catalog and
specification field inventories and the complete ordered 27-fault preview and
41-fault execute recovery signatures. Every induced fault must then equal its
packaged declaration. Its append-only log must prove four inspection attempts,
zero preview attempts, exactly one direct success attempt for each of `issue
list` and `pr list`, and either one ordinary-command wrapper attempt on
Linux/macOS or zero on Windows. The fixed total is therefore 11 fixture
attempts on a POSIX wrapper target and 10 on Windows; channel-specific canaries
prove that raw failure data and
unselected fields do not reach public output or isolated state. The
non-shipped harness seeds each exact receipt into its ephemeral user-config
root through the production trust-store adapter. This is explicitly
receipt-consumption evidence, not evidence that automation provided human
consent. Full-digest controlling-terminal success and redirected-input
rejection are proven separately by production-adapter and application tests.

Each bounded artifact-journey document uses evidence schema 2. It must record
`wrapper_outcome: ordinary_command_verified`, a valid
`wrapper_source_sha256`, and `wrapper_source_process_attempts: 1` on Linux and
macOS. Windows must instead record `wrapper_outcome:
platform_not_supported`, an empty wrapper-source digest, and zero wrapper source
attempts. The aggregator validates those target-specific alternatives rather
than treating unsupported POSIX activation as a skipped success.

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

Matrix artifacts use stable, target-unique names and explicit replacement of
the prior same-name artifact. Candidate archives, native journey evidence, and
aggregate summaries use disjoint prefixes. This avoids immutable-name
collisions on a rerun while strict revision, filename, target, and digest
checks still fail closed on any absent or stale input.

Current bounded observation: on 2026-07-22 the Darwin/arm64 exact packaged
journey passed for revision `b4ade8c`, including ordinary-command wrapper
activation. This is one implementation observation, not evidence for the later
documentation commit, the other four native rows, the aggregate, publication,
or a release-quality matrix claim. The exact revision selected for any tag must
replay all required rows again.

No public release has yet made the wrapper claim. A future candidate that
passes the complete gates may claim only the fixed Linux/macOS POSIX
materialization and Windows unsupported contract above. It still has no claim
for identity-wrapper execution, argv-only transforms, nonempty successful
source stderr, raw execution, arbitrary transformers, persistent wrapper
installation, executable/PATH shims, coding-agent-host activation, executable
attestation, or Windows POSIX activation. The retired legacy `plan
preview`/`run` slice is not runtime evidence.

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
