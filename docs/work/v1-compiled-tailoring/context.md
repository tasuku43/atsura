# Work Context: v1 compiled tailoring

## Verified starting state

- Commit `523ef63` is clean and passes the v0.1 full, security, public, and
  release gates.
- `atr plan preview` and `atr run` share strict schema-1 YAML compilation.
- Current execution is read-only, explicit-config, one-attempt, no-shell, and
  JSON select/rename only.
- The repository already provides typed effects, mutation impact vocabulary,
  catalog-derived help, structured faults, architecture lint, release
  packaging, and synthetic source-process fixtures.
- Local observations include GitHub CLI 2.72.0, Git 2.50.1, kubectl 1.33.0,
  and Docker 28.1.1. GitHub CLI is the first real source compatibility adapter,
  not the product model.

## Primary-source observations

- Claude Code `PreToolUse` supports allow, deny, ask, defer, and updated input;
  `SessionStart` can add refreshed context; post-use output replacement occurs
  after the tool effect.
- RTK transparently rewrites supported Bash commands but centers on output
  filtering/compression and cannot intercept non-Bash built-in tools.
- GitHub CLI 2.72.0 emits a 79,022-byte offline `gh help reference`, marks
  commands supporting `--json fields`, and can list allowed JSON fields without
  a provider task.
- Git help may include aliases and PATH external commands. kubectl plugins are
  arbitrary PATH executables. Local Docker help attempted plugin metadata
  discovery. Help and plugin observations are untrusted evidence.

## Fixed v1 decisions

- Concept C: one compiled bundle is canonical; gateway and Claude adapter are
  consumers.
- Source and coding-agent integration are independent vendor-neutral ports;
  shared schemas contain no adapter-specific fields.
- First real source adapter: capability-validated GitHub CLI major version 2.
- First real host adapter: project-local Claude Code hooks.
- A synthetic alternate source and host consumer must pass conformance tests.
- Bundle: deterministic JSON without ambient or temporal fields; digest is
  identity.
- Policy: YAML schema 2, deny-by-default, typed built-ins, no arbitrary shell.
- Trust: exact bundle digest, user-local receipt, interactive terminal only.
- Host: project-local Claude Code settings only.
- Enforcement: SessionStart discovery, PreToolUse admission/rewrite,
  PostToolUse never authorizes.
- Raw: explicit manual identity-bound bypass, never fallback or hook surface.
- Persistence: no credentials, source output, history, or transcript.

## Constraints and risks

- Inspection itself starts an untrusted local executable; probes need exact
  argv, identity checks, aggregate budgets, and no provider task.
- Help does not prove operation effect or semantic safety; policy owns both.
- Bundle paths may be repository-controlled; trust must bind content, not path.
- Existing Claude settings may be concurrently changed or malformed; ownership
  and replacement must fail closed.
- Claude Bash input is a shell string. v1 supports a strict simple-command
  grammar and rejects managed compound syntax rather than interpreting general
  shell.
- Confirmation must not become a reusable mutation permit or replay signal.
- Cross-platform packaging does not imply every integration feature has the
  same filesystem durability or shell host; limitations must be explicit and
  tested where supported.

## Evidence to capture

- Probe argv, count, byte/time bounds, fixture version, catalog digest, and
  provenance classification.
- Canonical catalog/policy/bundle bytes and digest round trips.
- Trust and integration filesystem identities before/after create/update/remove.
- Zero/one source task attempts for every pre/post boundary failure.
- Agent help discovery counts and routine external-processing count.
- Setup/authentication scorecard and complete E2E transcript.
- Gate outputs and final clean commit.

## Iteration evidence

- The first vendor-neutral catalog contract validates namespaced adapter kinds,
  exact executable identity/version, three provenance classes, sorted command
  and option evidence, structured-output selectors, finite probe facts,
  canonical JSON, and SHA-256 identity.
- An alternate synthetic adapter passes the application conformance test with
  no GitHub or Claude field in shared values.
- The GitHub CLI adapter requests exactly `version` and `help reference`, each
  with a five-second process bound, 64 KiB stderr bound, and 64/256 KiB stdout
  bound respectively; it requires unchanged executable identity.
- A local offline smoke against GitHub CLI 2.72.0 produced 169 command entries,
  31 JSON-capable entries, two source attempts, and a valid canonical digest.
- `task check:fast` passed after the public `source inspect` catalog contract
  and capability ledger entry were added.
- The pure schema-2 model is catalog-digest-bound and deny-by-default, rejects
  unverified catalog commands, restricts reads to allow/deny, restricts
  mutations to confirm/deny with complete target and impact, and requires typed
  output for executable rules.
- Canonical bundle tests recompute catalog and policy digests, derive the
  visible surface, detect catalog/policy/surface drift, exclude ambient fields,
  and pass with an alternate synthetic adapter catalog.
- `policy validate` and `bundle build` now consume explicit regular files
  through bounded strict codecs. The source inspection wrapper is revalidated
  against its embedded canonical catalog digest before policy evaluation.
- CLI and application tests prove validate/build use the same catalog and
  normalized policy, reject digest mismatch before producing a bundle, emit
  exact catalog-declared JSON shapes, and create no persisted trust state.
