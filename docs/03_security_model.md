# Security Model

This seed model covers the intended YAML-to-plan-to-wrapper boundary. It does
not claim that the current binary securely intercepts or executes a source CLI.

## Security objective

Atsura must not cause a source operation broader than the exact user-trusted
plan for the selected executable. Unknown configuration, untrusted policy,
source drift, ambiguous rules, incomplete plans, and unsupported actions fail
before source execution.

Output transformation must not become authority to change, repeat, or bypass
the source invocation.

## Assets

- Integrity of the user's local system, files, repositories, and source-CLI
  accounts.
- Integrity of command discovery, policy evaluation, confirmation, transformed
  argv, and plan provenance.
- Integrity of built-in processing and agent-facing result semantics.
- Confidentiality of source-CLI credentials, arguments, stdout, and stderr.
- Integrity and confidentiality of user-trusted YAML and future catalog state.
- The user's ability to distinguish preview, tailored execution, rejection,
  transform failure, and explicit raw execution.

## Actors and assumptions

- The user is the authority that may trust YAML and approve confirmation.
- A coding agent may propose YAML or invoke commands; neither action proves
  human authorization.
- Repository contributors may provide malicious configuration or hook content.
- Source CLIs may change, load plugins, emit hostile text, or have side effects
  not evident from names or help.
- Attackers may influence argv, environment, PATH, YAML, catalog files, hook
  payloads, executable replacement, stdout, and stderr.

A TTY, successful launch, repository checkout, hook installation, or
model-generated explanation is not proof that an operation is safe.

## Untrusted inputs

The following remain untrusted:

- command names and arguments;
- source executable paths and PATH resolution;
- source help, completion, schema, version, stdout, stderr, and errors;
- generated catalogs;
- all YAML before trust and semantic validation;
- repository-provided integrations;
- agent-generated proposals; and
- hook payloads and host metadata.

Strict parsing and visible escaping protect structure; they do not turn
external prose into instructions.

## Trust boundary

```text
untrusted hook request + source evidence + YAML
                    |
                    v
       bounded decoding and provenance
                    |
                    v
       user-trusted policy selection
                    |
                    v
    deterministic matching and plan validation
                    |
          +---------+---------+
          |                   |
          v                   v
       preview          rejection / confirm
          |
          v
execution-time revalidation
          |
          v
bounded wrapper: before -> source -> output -> after
```

Preview has no process or filesystem side effects. Execution cannot treat an
old preview as authority; it revalidates the trusted YAML, catalog/source
evidence, executable identity, and relevant environment immediately before the
wrapper begins.

## YAML policy boundary

- Per-command YAML is the selected configuration direction.
- Decoding is bounded and strict, with explicit schema-version behavior.
- Unknown fields, duplicate semantic keys, invalid types, unsupported actions,
  ambiguous matches, and invalid ordering fail closed.
- Repository YAML and user-trusted YAML are distinct provenance states. Merely
  opening a repository does not activate its policy.
- An agent proposal remains untrusted until a user-controlled workflow accepts
  the exact reviewed configuration or a defined semantic digest.
- Runtime evaluation never falls back to another configuration source after a
  present source fails validation.

The initial YAML contains no arbitrary shell. Each before, after, invocation,
and output action names a typed Atsura built-in with validated inputs and known
effects.

## Built-in action boundary

A built-in action must declare:

- a stable action kind and version;
- accepted typed inputs and finite bounds;
- whether it runs before invocation, changes argv, processes output, or runs
  after output;
- its filesystem, process, network, notification, access, and destructive
  effects;
- its output contract;
- cancellation and failure behavior; and
- whether failure occurs before or after the source attempt.

Unknown actions and stage-incompatible actions invalidate the plan. A built-in
does not receive an unrestricted shell, process executor, filesystem, or
network client.

jq expressions, RTK invocation, plugins, user scripts, and generic external
transformers are not initial built-ins. Admitting any of them requires a
separate product and threat-model decision covering executable identity,
configuration trust, data exposure, portability, time and size bounds, exit
semantics, dependency integrity, and recovery.

## Source identity and drift

Catalog and plan evidence must be bound to the source executable they describe.
At minimum, the design detects changes to the resolved executable or reported
version. PATH precedence, symlinks, replacement, plugins, aliases, and
self-updating CLIs may require stronger evidence.

A mismatch invalidates controlled execution. Atsura does not silently
regenerate a catalog and apply old permissions to new capabilities.

The executor resolves and revalidates the executable immediately before launch.

## Process execution boundary

A future executor must:

- accept an exact executable and argv vector, never an interpolated shell
  program;
- perform zero source attempts for invalid, rejected, unconfirmed, stale, or
  untrusted plans;
- use one caller context and bound working directory, inherited environment,
  time, stdout, and stderr;
- keep credentials out of newly constructed argv, YAML, plans, catalogs, logs,
  diagnostics, and history;
- distinguish a request not sent, confirmed result, source failure, transform
  failure, and unknown outcome; and
- never infer that repeating a source operation is safe from cancellation or
  output failure.

Source CLI authentication and authorization remain authoritative. Atsura does
not initially acquire or store OAuth tokens, PATs, or provider credentials.

## Output transformation boundary

Output can contain secrets, terminal controls, malformed structured data,
prompt-like prose, oversized collections, duplicate keys, deep nesting, and
values crafted to exploit a parser or coding agent.

The output pipeline must:

- declare the expected source format before parsing;
- enforce byte, nesting, item, field, and processing-time bounds;
- preserve source stdout and stderr as untrusted, separate channels;
- apply only plan-declared typed transformations;
- distinguish missing, null, empty, zero, false, malformed, and truncated values
  when they affect meaning;
- render only declared fields and structure;
- avoid inferring facts from labels, order, indentation, or nearby records; and
- retain source exit and transform status as separate facts.

If transformation fails after one source attempt, Atsura must not:

- retry or change the source invocation;
- run an alternative transformer automatically;
- report transformed success;
- expose raw output unless an explicit reviewed contract permits it; or
- switch to raw execution.

The exact policy for source nonzero exit, stderr, partial stdout, transform
failure, and optional intact-output fallback remains unresolved and must be
decided before execution support.

## Raw execution

Raw execution is a possible explicit route outside tailoring policy. It must be
selected visibly by the caller, identify the exact executable, and state that
Atsura tailoring is bypassed.

It is never automatic recovery for invalid YAML, rejection, missing
confirmation, source drift, built-in failure, or output-transform failure. It
never uses shell interpretation merely for convenience.

## Data, network, and credentials

- Atsura stores no authentication material or raw confidential source output.
- Usage history is not collected without a separate privacy, retention, and
  redaction decision.
- Direct external APIs are not selected. Network access remains the source
  CLI's responsibility under its own credential and destination policy.
- A future direct API or external transformer requires revised authentication,
  egress, bounded-call, fixture, and dependency contracts.

## Required evidence for the first slice

The first no-execution YAML-to-plan slice must prove:

- identical validated inputs yield an identical typed plan;
- invalid, unknown, ambiguous, or untrusted YAML yields no plan and no source
  attempt;
- executable and argv remain separate;
- every plan change names its trusted YAML rule and reason;
- invocation and output actions are separate ordered stages;
- a substantial output reshape is representable with typed built-ins; and
- hostile YAML values, source descriptions, and transform examples cannot break
  machine or terminal structure.

## Open security decisions

- YAML trust establishment, locations, precedence, revocation, and digesting.
- Exact allow, confirm, and deny authorization semantics.
- Executable identity across PATH, symlink, replacement, plugin, and version
  changes.
- Hook installation and command-discovery trust.
- Process environment, filesystem isolation, and output budgets.
- Built-in action effects and extension review.
- Source failure, stderr, partial output, transform failure, and raw-output
  interaction.
- Privacy and retention for any future history.
- Trust and dependency boundaries for jq, RTK, plugins, or external
  transformers.
