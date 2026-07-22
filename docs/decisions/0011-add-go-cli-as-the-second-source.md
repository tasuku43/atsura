# ADR 0011: Add Go CLI as the second source runtime

- Status: Accepted
- Date: 2026-07-22
- Deciders: Repository maintainer and product owner
- Scope: Source inspection, runtime compatibility dispatch, ordinary wrapper
  admission, multi-source conformance, and the enabling path to RTK
- Extends: ADR 0006, ADR 0008, ADR 0010
- Supersedes: None
- Superseded by: None

## Context

The first release-quality runtime supports GitHub CLI `issue list` and `pr
list`. Its shared catalogs, specifications, bundles, plans, wrapper bindings,
and results are vendor-neutral, but production composition still injects one
GitHub CLI verifier directly. A second nature-distinct source is needed to
prove that the architecture is genuinely adapter-based rather than merely
generic-looking.

Primary-source and native RTK v0.43.0 research found no fixed pipe filter for
the existing GitHub CLI contract. The apparent Git `status` candidate is not
acceptable: RTK's own complete implementation performs an additional plain
status observation because porcelain alone omits rebase, merge, and
cherry-pick state, while its pipe-only byte reduction is negligible. The
strongest useful RTK candidate is `go test -json` followed by the fixed
`go-test` filter. Go's official `test2json` stream gives a structured contract
that can be validated independently, and native pass fixtures reduced thousands
of bytes to one summary line.

The RTK candidate is not yet an accepted runtime tuple. Native hostile evidence
also found skip-only misclassification, silent malformed-line omission,
nonzero-status loss, and nondeterministic failure ordering. Adding the Go
source boundary first keeps source compatibility separate from processor
semantics and lets the next iteration define explicit pre-processor
preservation rather than hiding it as fallback.

## Decision drivers

- Prove shared core schemas and execution are independent of one source vendor.
- Preserve exactly one fresh plan and source-process boundary.
- Choose a second source with a documented path to meaningful output
  optimization.
- Avoid inventing Go's option, package-pattern, and test-binary grammar.
- Keep RTK identity, execution, licensing, and result semantics out of this
  source-only iteration.

## Decision

### Go CLI inspection contract 1

`source inspect` adds the public selector `go-cli`. The adapter performs exactly
three fixed no-shell probes against one identity-bound executable:

```text
go version
go help
go help test
```

It accepts a stable Go 1.26.x effective-toolchain observation from `go version`
under the inspection working directory and environment, requires the documented
root help grammar and test usage anchors, and emits a normal source-catalog
schema-1 document. Root built-in command names and summaries come only from the
bounded command table. The catalog contains no Go-specific field. Probe output,
environment values, credentials, and working-directory data are not persisted.

### First Go runtime grammar

The finite runtime accepts only:

- adapter kind `atsura.source.go_cli`, contract version 1;
- recorded inspection observation `source.version: go1.26.x`;
- a complete included surface containing exactly command `test`;
- an identity wrapper with no before, append, output, or after transform;
- plan result mode `source_stream_passthrough`; and
- no argv element after `test`.

Every option, package argument, `--` marker, and test-binary argument is outside
contract 1 and fails before source start. This is an intentional first surface,
not a claim that the Go command lacks those capabilities.

Successful application reuses ADR 0010 unchanged: one conventional completion
may have status zero or nonzero and nonempty stderr, exact bounded streams are
written stdout then stderr, and the source status is returned only after both
writes complete. Uncertain completion suppresses captured bytes and never
advertises replay as safe.

### Direct launcher and effective toolchain boundary

Executable path, SHA-256, and size identify the direct `go` launcher file.
`go version` may itself delegate, so `Source.Version` identifies only the
effective toolchain observed under the inspection working directory and
environment. Runtime revalidates the direct launcher identity and exact argv;
it does not repeat `go version`, freeze module/environment state, or identify a
selected or downloaded toolchain or GOROOT tree.

The same launcher may therefore select Go 1.27 or another toolchain at wrapper
runtime because of the working directory, module `go`/`toolchain` directives,
`GOTOOLCHAIN`, `GOROOT`, or related ambient state, without contract-1 pre-start
detection. That is source-owned downstream behavior, not an accepted Go 1.27
compatibility claim. Constraining it requires an explicit
environment/toolchain closure, a successor ADR, and new platform evidence.

### Compatibility registry

Application owns one finite compatibility registry implementing the existing
plan and complete-surface ports. It dispatches by the exact namespaced adapter
kind already bound into a validated plan or bundle and delegates to one
injected verifier. The registry does not inspect PATH, load plugins, construct
source requests, execute a process, or add a second surface or fault registry.

GitHub CLI and Go CLI remain infrastructure adapters. Shared domain,
application result, CLI output, bundle, binding, and plan schemas gain no
vendor discriminator beyond the existing opaque adapter kind and contract
version.

### Source-owned effects

`go test` remains `EffectExecute`. It may compile and run untrusted repository
code, read credentials or configuration, resolve modules, access networks, and
mutate caller-owned files or caches. Effective toolchain selection and download
are also source-owned. Atsura neither classifies nor authorizes those effects.
Exact direct-launcher identity, separate argv, no shell, finite bounds,
and non-retryable uncertainty describe only Atsura's process boundary.

## Alternatives considered

### Implement Go and RTK together

Rejected. Processor observation, exact RTK identity, isolated execution,
semantic validation, original preservation, schema migration, provenance,
SBOM, and native processor evidence form a separate public capability.

### Add source-specific switches to plan application

Rejected. It would make the application layer accumulate vendor policy and
would leave wrapper rendering with a competing dispatch mechanism.

### Admit all documented Go test flags now

Rejected. Go combines build flags, test flags, package patterns, environment
defaults, and test-binary flags. Help observation alone is not a complete
executable grammar contract.

## Consequences

### Positive

- Two materially different source CLIs exercise one canonical tailoring core.
- An identity draft for `go test` is executable through the ordinary wrapper.
- The next RTK slice can focus on processor and result authority rather than
  simultaneously proving a new source boundary.
- No new dependency, schema version, trust state, or host adapter is added.

### Negative

- The initial Go surface accepts only a no-argument current-package test.
- A recorded inspection observation outside Go 1.26.x needs new admission
  evidence; a later effective-toolchain change is currently unobserved.
- Ordinary Go test execution can have broad source-owned effects outside
  Atsura's containment and authorization claims.
- RTK remains deferred after this decision.

## Mechanical enforcement

- Inspector tests bind the exact three probe argv, byte/time limits, identity,
  Go 1.26 version grammar, root command table, and test usage anchors.
- Runtime truth tables reject every adapter/version/command/wrapper/result/argv
  variation before source start.
- Registry tests prove exact dispatch, unknown/missing/duplicate rejection,
  and no GitHub or Go field in shared canonical artifacts.
- CLI help and catalog tests publish both adapter selectors and adapter-specific
  bounded attempt facts without creating a second inspection command.
- Exact installed-artifact journeys exercise Go inspection on every native
  target and ordinary no-argument Go test wrappers on claimed POSIX targets.
  They set `GOTOOLCHAIN=local`, disable download, and isolate module/cache roots
  as deterministic fixture inputs, not production guarantees. Every target
  records one zero-attempt rejection; POSIX requires `go test extra` to return
  `wrapper_runtime_not_supported` / exit 12, then records one admitted Go test
  attempt and a nonempty rendered-wrapper digest, while Windows retains the
  empty unsupported wrapper case set.
- Completion requires `task check`, `task security`, `task public:check`,
  `task release:check`, and the required native CI matrix on one revision.

## Reconsideration signals

Create a successor ADR before admitting Go flags or package arguments,
accepting a recorded version observation outside Go 1.26.x, treating Go test as
read-only, adding any effective-toolchain guarantee, closing or injecting
working-directory/environment/module/toolchain state into the plan, adding a
second source executor, or combining pre-processor preservation with ADR
0010's plain source-stream mode.
