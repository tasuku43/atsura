# Plan: Persistent wrapper shims

1. Accept ADR 0017 and add the three catalog contracts without promoting incomplete reachability.
2. Add pure artifact/reference/manifest invariants and a fixed executable-shim renderer.
3. Add application install/status/remove use cases through the central mutation invoker.
4. Add a confined POSIX store with bounded discovery, create-exclusive publication, exact-reference removal, and typed uncertain outcomes.
5. Wire CLI JSON/help/recovery contracts and prove ordinary invocation plus hostile filesystem cases.
6. Reuse the current installed-artifact journey where possible, run all gates, promote only learned decisions, remove this packet, commit, and push.

## Chosen boundary

The first lifecycle supports create, bounded status discovery, and exact remove.
It does not replace an existing command. Caller-owned `PATH` selection remains
outside Atsura. The installed shim reaches only `wrapper run`; it never invokes
the source by ambient name and never falls back to raw execution.

## Main risks

- Filesystem races or foreign-file deletion: confine operations to one opened private root and revalidate identities before mutation.
- Partial install/remove: classify the post-action outcome as non-retryable and make `wrapper status` the only recovery.
- Script injection or recursion: render a fixed template from validated binding values and invoke the exact bound `atr` path.
