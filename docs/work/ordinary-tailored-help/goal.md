# Work Goal: Ordinary tailored help

- Status: Active
- Retention: temporary
- Retention reason: None
- Governing contract: Project theses 1, 2, 5, 7, and 8
- Review/delete trigger: Delete after durable conclusions are promoted and the change completes
- Successor: None
- Owner: Atsura maintainers
- Target: Current implementation iteration
- Related ADRs: 0005, 0006, 0008, 0010, 0011, 0012, and accepted 0014

## Outcome

After a caller sources one generated POSIX wrapper, a maintainer or coding
agent can use the ordinary source-command spelling with a final `--help` to
discover the bundle's purpose-specific root, namespace, and exact-command
surface. The deterministic output contains only included command paths and
in-surface long options, is compiled from the exact adopted bundle into the
reviewed wrapper bytes, and starts neither the source CLI nor an output
processor.

## Why now

At the slice start, the wrapper enforced command and option membership but
forwarded every argv to `wrapper run`. Consequently, ordinary root and
namespace help could not display the tailored surface. Three independent
repository audits ranked this as a larger thesis gap than additional argv
transforms or persistent wrapper installation: a reduced CLI that cannot
describe itself under its ordinary name is enforceable but not yet
self-discoverable.

## Non-goals

- Do not execute or embed raw source help.
- Do not infer vendor `help` subcommands, `-h`, root/global options,
  positional grammar, option dependencies, or source usage syntax.
- Do not add persistent installation, PATH mutation, profile selection,
  coding-agent-host adapters, argv defaults/replacement, raw execution, or
  source refresh.
- Do not change the fresh execution-plan authority or treat help as a source
  execution plan.
- Do not claim that excluded source capabilities are sandboxed outside the
  generated wrapper.

## Acceptance criteria

- [x] `<source> --help` deterministically lists only included exact command
      paths from the compiled surface.
- [x] `<source> <namespace> --help` deterministically lists only included
      descendants of that namespace.
- [x] `<source> <included-command> --help` displays the exact command,
      bounded source summary, tailoring reason, and only its in-surface long
      options with explicit value arity.
- [ ] An excluded cataloged command remains fail-closed as
      `command_not_in_surface`; syntactically unknown help remains
      `invalid_invocation`; neither path reveals source help.
- [x] Every tailored-help success makes zero source and zero processor
      attempts and does not require the bound `atr` process to start.
- [x] Help bytes, branch order, quoting, size, and wrapper digest are
      deterministic; hostile shell punctuation remains literal and structural
      text is rejected by the governing bundle/catalog validation.
- [ ] A generic caller-owned POSIX activation journey proves root, namespace,
      and exact help from the installed `atr` artifact without a coding-agent
      host or vendor-host fields.
- [ ] `task check`, `task security`, `task public:check`, and
      `task release:check` pass, followed by the claimed native artifact CI
      matrix.

## Governing documents

- Thesis: purpose-specific discoverable surface; deterministic compilation;
  source-owned meaning; host-neutral ordinary wrapper
- Product contract section: tailored CLI surface and host-neutral wrapper
  materialization
- Architecture or security invariant: catalog/bundle source of truth; no
  source attempt for compiled help; external text remains untrusted
- Existing ADR: 0005, 0006, 0008, 0010, 0011, 0012, and accepted 0014

## Completion definition

The work is complete when every acceptance criterion has exact evidence,
durable decisions are promoted to numbered documentation and an accepted ADR,
required local and native-artifact profiles pass, no temporary diagnostics or
sensitive artifacts remain, and this temporary packet is removed.
