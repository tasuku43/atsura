# Work Plan: Wrapper-plan preview

- Status: Accepted
- Goal: [goal.md](goal.md)
- Context: [context.md](context.md)
- Tasks: [tasks.md](tasks.md)

## Chosen approach

Add a pure `tailoringplan` domain constructor and a small application use case
that establishes bundle/adoption/source preconditions before calling it. Expose
the result through a new `bundle preview` catalog command. Keep bundle loading,
adoption lookup, executable identity reads, parsing, rendering, and composition
at their existing layer boundaries.

## Delivery slices

1. Define the typed plan, attempt input, deterministic command/option
   resolution, validation, canonical encoding, and digest in domain.
2. Add application orchestration with narrow bundle, adoption, and identity
   ports and stable zero-attempt faults.
3. Compose existing infrastructure ports and expose catalog-derived CLI/help/
   JSON output at `bundle preview`.
4. Promote the implemented boundary into governing docs, capability ledger,
   README, and agent-readiness evidence.
5. Run focused/full/security checks, audit runtime absence and vocabulary,
   commit the milestone, and remove this temporary packet.

## Alternatives rejected

- Reusing `plan preview`: its schema and command shape are an explicit retired
  authorization migration boundary.
- Previewing an unadopted bundle: the thesis defines a plan against one adopted
  bundle, and this would blur build review with selected runtime state.
- Starting the source to prove the plan: that would make preview an Execute
  operation and skip the intended pure validation slice.
- Treating every non-command token as opaque: this would let excluded or
  unknown options bypass the tailored option surface.
- Modeling source permission or remote effect: source semantics remain
  source-owned and are not required to apply a wrapper.

## Verification strategy

- Domain owns plan completeness, argv grammar, surface/option resolution,
  canonical digest, and negative states.
- Application owns precondition order, state mapping, port calls, and no source
  execution.
- CLI owns exact public grammar, catalog metadata, output schema, rendering,
  and recovery.
- Full/security gates and repository search own architectural and vocabulary
  regression.
