# Work Goal: Release-quality local tailoring run

- Status: Active
- Retention: temporary
- Retention reason: None
- Governing contract: docs/00_theses.md
- Review/delete trigger: Delete after durable conclusions are promoted and the final committed tree passes every required gate
- Successor: None
- Owner: Repository maintainer
- Target: v0.1 local-tailoring quality boundary
- Related ADRs: docs/decisions/0002-v0.1-local-run-boundary.md

## Outcome

A maintainer can explicitly select one reviewed schema-1 YAML file and use
`atr run` to execute one declared read-only source command without a shell,
then receive a bounded, deterministic JSON result whose fields were selected
and renamed by the same plan that `atr plan preview` explains.

## Why now

Plan preview validates vocabulary but not Atsura's central wrapper hypothesis.
The next release-quality slice must prove one complete plan-to-process-to-output
path before hooks, discovery, confirmation, or extension mechanisms add more
variables.

## Non-goals

- Tagging, publishing, pushing, opening a pull request, or creating a release.
- Source-help exploration, generated command catalogs, or selecting a first supported vendor CLI.
- Hooks, PATH wrappers, shell functions, command hiding, or host input rewriting.
- Create/write source effects, confirmation, raw execution, or automatic fallback.
- YAML inheritance, implicit repository activation, persistent trust, or agent-authored policy activation.
- Arbitrary shell, jq, RTK, plugins, scripts as policy actions, or external transformers.
- Usage history, agent proposals, authentication storage, or direct external APIs.

## Acceptance criteria

- [ ] `atr run --config <path> -- <source-command> [args...]` is catalog-discoverable as a read-only utility.
- [ ] Schema 1 explicitly requires `effect: read`; create, write, confirm, and unknown effects fail before source execution.
- [ ] Preview and run use the same strict loader and pure plan compiler for decision, match, argv, reason, and output actions.
- [ ] Deny, mismatch, invalid YAML, unsafe config, unresolved or changed executable, and canceled preflight produce zero source-process attempts.
- [ ] Allow starts at most one direct source process, without a shell or stdin, under a 30-second timeout, 4 MiB stdout bound, and 256 KiB stderr bound.
- [ ] Successful object or array JSON is strictly parsed, rejects duplicates and excessive complexity, preserves null/empty/zero/false, and applies ordered select/rename/compact rendering.
- [ ] Nonzero exit, timeout, cancellation, output overflow, malformed JSON, missing fields, and transform failure never retry or expose raw stdout as success.
- [ ] Success uses fixed schema-1 JSON fields and needs zero undeclared external reconstruction; source stderr is bounded and structurally escaped on stderr.
- [ ] Exact human/scoped-agent help and structured recovery are executable without command guessing.
- [ ] The documented synthetic fixture succeeds locally and reports exactly one direct source-process attempt.
- [ ] Focused tests, `task check`, `task security`, `task public:check`, and `task release:check` all exit 0 on the same final commit.
- [ ] The work packet is removed, the implementation is committed, Git status is clean, and no publication action occurred.

## Governing documents

- Thesis: `docs/00_theses.md`, Theses 2 through 5
- Product contract section: `docs/01_product_contract.md`, wrapper and output failure boundary
- Architecture or security invariant: `docs/02_architecture.md` and `docs/03_security_model.md`
- Existing ADR: `docs/decisions/0002-v0.1-local-run-boundary.md`

## Completion definition

The goal is complete only when every acceptance item is mechanically or
manually evidenced, durable decisions are promoted, required gates pass on the
final committed tree, `git status --short` is empty, and no tag, push, pull
request, Formula update, artifact publication, or GitHub Release exists from
this work.
