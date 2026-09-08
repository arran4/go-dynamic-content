# Agent Guidance

## Preserve composability

This library is intentionally built around composing behaviour through functions, closures, helper types, and functional options. Keep the primary `contentImpl` type focused on coordinating cached content and configured operations; do not turn it into the owner of every behavioural policy.

State that belongs to a generator or validator should normally be captured by the generator/validator itself, either in a closure or in a dedicated helper type whose method is passed to `WithGenerator` / `WithValidator`.

Existing examples of the intended architecture include:

- `helpers.DynamicGenerator`, which owns mutable generator-selection state and exposes `Generate` for composition with `WithGenerator`;
- `helpers.TimeExpiry`, which captures validator state in returned closures;
- `helpers.RetryGenerator`, which composes generator behaviour by returning a wrapped generator function.

Prefer extending that model over adding policy-specific fields to `contentImpl`.

### Primary-type boundary

Avoid adding fields such as generation-in-progress, validation-in-progress, retry state, expiry state, callback state, or similar behavioural state directly to `contentImpl` when that state can be encapsulated in a composed function/type.

In particular, issue-specific fixes must not casually make `contentImpl` responsible for global generation/validation state merely because doing so is convenient for synchronization. If coordination state is genuinely required at the cache/store boundary, isolate it behind a small internal abstraction and explain why it cannot live in the composed generator/validator layer.

`contentImpl` should remain primarily responsible for:

- the selected `Store`;
- configured generator/validator/callback functions;
- lazy/eager orchestration;
- minimal synchronization needed to keep cache state coherent.

## Re-entrancy and concurrency fixes

For fixes such as issue #16:

- Never execute arbitrary caller-supplied generators, validators, or callbacks while `contentImpl.mu` is held.
- User-supplied code must be able to re-enter the public `Content` API without permanently deadlocking.
- Preserve controlled single-flight/concurrent loading behaviour; do not introduce uncontrolled duplicate generation.
- Preserve coherent state transitions: invalidation must be committed before `onInvalidate`; completed generation state must be committed before `onGenerate` observes it.
- Preserve existing public error/nil semantics unless the task explicitly changes them.
- Do not use goroutine-ID tricks, re-entrant mutex hacks, or other hidden caller-identity mechanisms.
- Prefer composable wrappers/closures/types for generator- or validator-specific in-flight/re-entry state.
- If stale-result/version coordination is required, keep it in the narrowest internal abstraction that owns cache-commit coordination rather than expanding `contentImpl` into a general state machine.

Regression tests for concurrency/re-entry must be bounded and deterministic. Include direct re-entry cases, not only re-entry through separate goroutines, and run `go test ./...`, `go test -race ./...`, and `go vet ./...` for concurrency-related changes.

## Review rule

A change can be functionally correct and still be architecturally wrong. Treat loss of composability or movement of behaviour-specific state into `contentImpl` as a review blocker unless the task explicitly requires that architectural change and the reason is documented.