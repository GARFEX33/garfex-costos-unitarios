# Proposal: Resource Master Technical Debt Remediation

## Intent

Remediate the user-approved 12-item Resource Master debt backlog before any further master-catalog expansion. Preserve the dependency direction `TUI -> application -> domain repository port -> PostgreSQL`; strengthen contracts within those boundaries rather than bypassing them.

## Goals

- Make the application/core boundary authoritative for valid, canonical resources.
- Make catalog activity and attribute persistence deterministic, scoped, and atomic.
- Deliver explicit lifecycle, bounded search loading, complete pagination, maintainable editor structure, and accurate documentation.

## Scope

### In Scope

1. Application-boundary validation. 2. Class/family/type/attribute persistence scope. 3. Exact write cardinality. 4. Consistent active catalog semantics. 5. Deactivate terminology. 6. Inactive discovery and reactivation. 7. Controlled aggregate construction. 8. Derived canonical identity. 9. N+1 search removal. 10. Behavior-preserving editor decomposition. 11. Documentation/comment correction. 12. Complete pagination.

### Non-Goals

- Supplier Master design or implementation, new master catalogs, or architectural shortcuts.
- Prompt, transition, ordering, canonical-value, or persistence-call changes during editor decomposition.
- Modifying, overwriting, or including unrelated existing modified/untracked TUI, Supplier Master, registry, or migration files.

## Capabilities

### New Capabilities

- `resource-master-integrity`: Canonical invariants, active catalog eligibility, scoped persistence, and exact transactional cardinality.
- `resource-master-lifecycle`: Visible deactivation, inactive discovery, and explicit reactivation with active-only defaults.
- `resource-master-search-navigation`: Set-based hydration, stable ordering, page boundaries, and next/previous navigation.
- `resource-master-editor-maintainability`: Characterized, behavior-preserving responsibility extraction and settled documentation.

### Modified Capabilities

None; no main OpenSpec capabilities exist.

## Staged Outcomes and Release Gates

1. Characterization and data audit establish a clean baseline.
2. Invariant ownership, active semantics, and persistence cardinality pass application/integration evidence; these are prerequisites and release gates before refactor, lifecycle, performance, or UX work.
3. Editor responsibilities are extracted without behavioral drift.
4. Lifecycle is explicit end to end; search becomes set-based before pagination.
5. Pagination and documentation ship only after stable behavior and ordering evidence.

## Success Criteria

- Invalid or forged state never reaches persistence; every expected attribute write resolves exactly once or rolls back.
- Historical reads and new-write eligibility follow the accepted active policy.
- Deactivate/discover/reactivate works end to end; query count is page-size independent; pagination preserves filters and selection.
- Architecture direction remains intact and unrelated worktree changes remain untouched.

## Risks and Rollback Boundaries

Persisted anomalies may block enforcement; active filtering may hide history; API tightening and set-based loading may alter callers, ordering, or hydration. Gate each stage with parity evidence. Revert code stages independently; isolate additive data repair/enforcement migrations and do not deploy dependent code before migration admission. Editor and migration units likely pressure the 400-line review budget under `ask-on-risk`; tasks must assess delivery shape later, without this proposal making the final forecast.
