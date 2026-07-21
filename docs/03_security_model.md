# Security Model

This seed model defines the security direction for Atsura before it executes or
transforms a source command. It does not claim that the bootstrap implements a
secure wrapper, policy engine, or agent integration.

## Security objective

Atsura must not cause a source CLI operation that is broader than the exact
reviewed plan for the selected source executable. If the controlling policy,
source identity, or plan cannot be evaluated, Atsura does not execute the
operation. Output optimization must not change the source operation's meaning
or authorize a retry.

## Assets

- Integrity of the user's local system, files, repositories, and source-CLI
  accounts.
- Integrity of source executable selection, command modeling, policy
  evaluation, transformed argv, and execution-plan provenance.
- Confidentiality of credentials already managed by source CLIs.
- Confidentiality of command arguments and raw source stdout/stderr.
- Integrity and confidentiality of user-trusted policy and any future catalog
  or non-secret state.
- The user's ability to distinguish tailored execution, policy rejection, and
  explicit raw execution.

## Actors and assumptions

- A user may make mistakes and is the authority that can trust policy.
- A coding agent may propose configuration and invoke commands within its
  granted environment; that is not proof of human authorization.
- A repository contributor may provide useful or malicious configuration.
- A source CLI may change, return hostile text, load plugins, access remote
  systems, or have side effects not evident from a command name.
- An attacker may influence argv, environment, PATH, configuration, repository
  files, help output, executable replacement, plugin state, or source output.

A successful process launch, TTY presence, parent process, repository checkout,
or model-generated explanation is not proof that an operation is safe.

## Untrusted inputs

Atsura treats all of the following as untrusted data:

- command names and arguments;
- source executable paths and PATH resolution;
- source help, completion, schema, version, and error output;
- generated command catalogs;
- policy and configuration files;
- repository-provided integration files;
- source stdout and stderr; and
- coding-agent proposals and hook payloads.

Parsing or visible escaping does not turn external text into instructions.
Prompt-like text remains data.

## Trust boundaries

```text
untrusted invocation / repository / source observations
                         |
                         v
           bounded decoding and provenance
                         |
                         v
          user-trusted policy selection
                         |
                         v
        deterministic match and plan validation
                         |
              +----------+-----------+
              |                      |
              v                      v
       policy rejection        controlled process port
                                      |
                                      v
                              exact source executable
```

Coding-agent integrations sit outside the deterministic decision boundary.
They may request a preview or execution task, but cannot silently mark a
repository policy as user-trusted or bypass a rejection.

## Policy and configuration

- Arbitrary shell code is not an allowed default policy mechanism.
- A policy format must have bounded decoding, explicit schema/version behavior,
  and deterministic rule precedence before it can control execution.
- Unknown fields, ambiguous matches, missing decisions, or invalid provenance
  fail closed for controlled execution.
- Repository-provided policy and user-trusted policy are distinct sources. A
  repository checkout alone does not grant trust.
- The future precedence and approval mechanism must preserve which principal
  trusted each active policy; it must not copy a repository policy into a
  trusted store implicitly.
- An agent-generated policy remains a proposal until the user-controlled trust
  workflow accepts the exact reviewed bytes or semantic equivalent.

YAML is neither required nor ruled out by this bootstrap.

## Source executable identity and drift

A generated catalog and policy decision must eventually bind to evidence about
the source executable they describe. At minimum, the design must be able to
detect when the resolved executable or its reported version has changed.
Symlinks, PATH precedence, plugins, aliases, and self-updating CLIs may require
stronger evidence; the exact identity model is open.

A stale or mismatched identity invalidates controlled execution until the
source is reinspected and the resulting policy impact is reviewed. Atsura must
not silently regenerate a catalog and preserve old permission decisions as if
nothing changed.

## Process execution boundary

No source execution is implemented in this bootstrap. A future executor must:

- receive a validated executable and argv vector, never an interpolated shell
  program;
- resolve and revalidate the execution target immediately before launch;
- bound inherited environment, working directory, time, stdout, and stderr
  according to the task contract;
- keep credentials out of newly constructed argv, logs, diagnostics, catalogs,
  policies, and persisted history;
- perform zero attempts on policy rejection or invalid planning state; and
- classify cancellation and uncertain mutation outcomes without claiming a
  retry is safe.

Source CLI authentication and authorization remain in force. Atsura initially
does not acquire or store OAuth tokens, PATs, or provider credentials.

## Raw or passthrough execution

Raw execution is a possible explicit product route, not a recovery mechanism.
It must:

- be visibly selected by the caller;
- identify the exact executable being invoked;
- state that Atsura tailoring policy is bypassed;
- never be chosen automatically after policy parse failure, rejection, stale
  catalog evidence, or transform failure; and
- avoid claiming that Atsura approved the source operation.

Whether the raw route still applies generic process bounds is a later product
and security decision. It may never use shell interpretation merely to be
convenient.

## Output and data handling

- Atsura does not store authentication material or raw confidential source
  output.
- Usage history is not collected in the bootstrap and requires a new privacy,
  retention, and redaction decision before adoption.
- Output projection and transformation treat source text as untrusted and
  preserve the structural safety rules inherited by the repository.
- If output optimization is unavailable or fails, Atsura must not change argv,
  run a different command, repeat the source command, or report transformed
  output as complete.
- A later slice must choose between an explicit intact-output degradation and a
  typed failure. That choice must consider confidentiality and output bounds;
  this bootstrap does not select it.
- Source exit status, stdout/stderr ownership, ordering, truncation, and partial
  write behavior require explicit contracts before execution is supported.

## Network and credentials

Atsura selects no direct external API during bootstrap. Any network access is
performed by the source CLI under its own credential and destination policy.
Atsura must not capture those credentials from environment or configuration for
its own use.

If a future capability calls an external API directly, it requires a revised
thesis, authentication decision, destination policy, bounded call contract,
publishable fixtures, and the repository's external-API validation workflow
before implementation.

## Required evidence for the first slice

The recommended no-execution planning slice must prove:

- identical validated inputs yield an identical typed decision;
- invalid policy and rejection make zero source-process attempts;
- an allowed plan contains an exact executable/argv representation without a
  shell string;
- every decision names the matched rule and trusted policy provenance;
- repository-origin policy cannot be mistaken for user-trusted policy; and
- hostile command, argument, reason, and source-description text cannot break
  machine or terminal output structure.

## Open security decisions

- The exact source executable identity and version-drift model.
- Policy trust establishment, precedence, revocation, and confirmation.
- Exact allow, confirm, and deny semantics.
- Process environment and filesystem isolation.
- Raw execution bounds and user experience.
- Output fallback, redaction, size, and streaming behavior.
- History collection, retention, and privacy.
- Hook or wrapper authorization boundaries.
- RTK trust and dependency implications if it is considered.

Each must be decided by a concrete vertical slice or primary-source research,
not by bootstrap speculation.
