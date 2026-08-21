# Explore: resource-master-core-write-lifecycle

## Goal

Third per-operation WRITE graduation on `resourcecore`: `Deactivate` and
`Reactivate` together (a deliberate pairing — they share the same internal
lifecycle-toggle mechanism). `HardDelete` is explicitly OUT of scope,
deferred to a later, separate, most-destructive-last change.

## Internal authority — confirmed by direct read

Catalog (`internal/app/catalogo/service.go`):
- `setActiveRevision` (private helper, line 556) — shared by both ops.
- `DeactivateRevision` (line 598) / `ReactivateRevision` (line 603) —
  `func(ctx, kind, id int64, expectedRevision uint64) error`. Both return
  **only `error`**, no record — unlike catalog `UpdateRevision` which
  returns `(domain.CatalogRecord, error)`.

Resource (`internal/app/recursos/service.go`):
- `DeactivateRevision` (line 233) — its own distinct method, type-asserts
  `domain.ResourceRepositoryV2`, calls `v2.DeactivateRevision` directly, no
  app-level no-op short-circuit.
- `ReactivateRevision` (line 255) — loads via `finder.GetByID`,
  short-circuits `if current.Active { return LifecycleResult{Resource:
  current}, nil }`, otherwise rebuilds via `reactivationCandidate` (line
  209) and calls `v2.ReactivateRevision`.
- Both return `domain.LifecycleResult{Resource, Changed}`
  (`resource_types.go:47-50`) — the real, complete, persisted resource.

## Request/result shape

Mirrors Update's identity+CAS pattern (`Kind`+`ID` catalog, `ID` resource,
`ExpectedRevision uint64`), but with **no content fields** — Deactivate/
Reactivate never touch Values/Rules/Scope/Attributes. Scalar-only requests
need **no `Clone*` function at all** (Go value semantics already give full
immutability) — a genuine simplification versus Create/Update.

## Idempotent no-op behavior — NOT uniform (independently verified)

| Path | Short-circuits before CAS check? | Consequence |
|---|---|---|
| Catalog `setActiveRevision` (both ops) | **Yes** — `service.go:572`: `if current.Active == active { return nil }`, before `expectedRevision` is ever compared | A stale `expectedRevision` on an already-in-desired-state record silently succeeds instead of returning `CONFLICT`. |
| Resource `DeactivateRevision` | **No** — delegates straight to `v2.DeactivateRevision` | Postgres checks `currentRevision != expectedRevision` → `ErrResourceRevisionConflict` before checking `currentActive == active`. CAS strictly enforced even on the no-op. |
| Resource `ReactivateRevision` | **Yes** — `service.go:272-273`: `if current.Active { return LifecycleResult{Resource: current}, nil }`, before `reactivationCandidate`/CAS | Same looseness as catalog. |

Verified directly against source (not just relayed): all three citations
read byte-for-byte as described. This is **pre-existing production
behavior** in `internal/app`, already consumed by the TUI — not introduced
by this graduation. Recommendation: document as reused-as-is in
readiness.md, do not silently fix inside a bridge-only graduation (fixing
it would change already-shipped, TUI-consumed behavior).

**Public no-op contract implication**: a no-op should still return the
current persisted record (Active + unmodified Revision), not an error.

## Central design decision: catalog confirm-read, for a different reason than Create's

- `domain.CatalogAdminRepositoryV2.SetActive` (`catalog_admin_v2.go:61`)
  does return `(domain.CatalogWriteResult, error)` with a real `Record
  *CatalogRecord` — the port itself is not the gap.
- But `catalog_admin_repository_v2.go:1100`'s postgres implementation
  builds that record as `domain.CatalogRecord{Kind, ID, Revision, Active}`
  — **only 4 fields; Values/Rules always empty**. A deliberately minimal
  CAS-confirmation projection.
- `catalogo.Service.setActiveRevision` (`service.go:588-594`) uses only
  `result.Catalog` and **discards `result.Record` entirely** —
  `DeactivateRevision`/`ReactivateRevision` return only `error`.

Even widening `catalogo.Service`'s signatures would still yield a partial
record unless postgres's `SetActive` also does a full read-back (a much
larger internal change), and would break `internal/tui/catalog_admin.go`'s
`catalogDeactivator`/`catalogReactivator` interfaces — violating the
zero-internal-touch invariant both prior graduations preserved.

Resource side needs **no confirm-read** — already returns the real,
complete `domain.LifecycleResult{Resource, Changed}` directly.

## Error-category reachability — traced from source

`domain.ErrInvalidLifecycle`/`domain.ErrCatalogInUse` are raised only
inside `Delete`/`buildV2DeleteCandidate` — confirmed unreachable from
Deactivate/Reactivate, correctly deferred to `HardDelete`.

### Catalog Deactivate / Reactivate — identical reachable set, 6 of 15

`INVALID_ARGUMENT`, `NOT_FOUND`, `INVALID_CATALOG`, `CONFLICT` (not on the
no-op path), `UNAVAILABLE`, `INTERNAL`. Not reachable: `DUPLICATE`,
`INVALID_REFERENCE`, `VALIDATION` (the APLICABILIDAD-reference/validation
gate is `OpInsert`/`OpUpdate`-only, excluding `OpDeactivate`/`OpReactivate`
— `catalog_mutation.go:146`), `INTEGRITY` (zero references in
`catalog_admin_*.go`, unchanged from Update's finding), `IDENTITY_CONFLICT`/
`REACTIVATION_IMPOSSIBLE` (resource-only), `IN_USE`/`IMMUTABLE_CODE`
(Delete/Update-only).

### Resource Deactivate — 5 of 15

`INVALID_ARGUMENT`, `NOT_FOUND`, `CONFLICT` (CAS-checked even on no-op),
`UNAVAILABLE`, `INTERNAL`. Not reachable: `DUPLICATE`, `INVALID_REFERENCE`
(only raised for `active==true`/Reactivate), `VALIDATION`, `INTEGRITY`
(identity check gated `expectedIdentityKey != ""`, empty on Deactivate),
`IDENTITY_CONFLICT` (no such classification arm on this path).

### Resource Reactivate — 7 of 15 (first graduation reaching IDENTITY_CONFLICT/REACTIVATION_IMPOSSIBLE)

`INVALID_ARGUMENT`, `NOT_FOUND`, `IDENTITY_CONFLICT` (new — via
`ErrResourceIntegrity`→reclassified, or `reactivationCandidate`'s own
identity check), `REACTIVATION_IMPOSSIBLE` (new — via `ErrResourceReference`
reclassified, or `domain.NewResource` failure wrapped unconditionally),
`CONFLICT` (not on no-op path), `UNAVAILABLE`, `INTERNAL`. Not reachable:
`DUPLICATE`, `INVALID_REFERENCE` (always reclassified to
`REACTIVATION_IMPOSSIBLE`), `VALIDATION` (always reclassified the same
way), `INTEGRITY` (always reclassified to `IDENTITY_CONFLICT`).

## No new postgres wiring gap

`core.Map` already has correctly-ordered arms for every category above —
zero `core.Map` changes needed. Grepped `internal/postgres` for the
"stand-in for future domain.Err…" pattern that caused Update's CONFLICT
bug — zero matches. No analogous bug exists for this graduation.

## Actor attribution

Purely reused, as expected — `core.WithActor`/`ActorFrom`, identical
mechanism, no new design.

## Approaches considered

1. **(Recommended)** Bridge confirm-read for catalog (`a.catalog.Get` after
   a successful `error`-only call), direct pass-through for resource
   (already returns the complete record). Zero `internal/app`/
   `internal/domain`/`internal/tui` changes.
2. Widen `catalogo.Service`'s signatures to return a record — rejected:
   still partial without a much larger postgres change, and breaks
   `internal/tui`'s existing interfaces.
3. Public contract returns no record for lifecycle ops (error/void only) —
   rejected: inconsistent with every other graduated write returning the
   persisted projection.

## Size/complexity

Structurally simpler per-method mapping than Update (no Clone* needed),
offset by 3 distinct reachability tables (vs Update's 2) each needing their
own injected tests. Estimate: 650-950 lines. Recommended 4-unit auto-chain:
U1 (public contract), U2 (catalog bridge, confirm-read, 6-category table),
U3 (resource bridge, 5- and 7-category tables), U4 (readiness + full-suite
verification). 400-line budget risk: Medium-High.

## Risks

- The idempotent no-op CAS-bypass asymmetry must be documented, not
  silently fixed — out of scope for a bridge-only graduation.
- Catalog confirm-read adds a round trip (acceptable, worth flagging).
- Live-PostgreSQL integration evidence gap likely recurs in a sandboxed
  apply session, same as Update's disclosed gap.

## Next recommended phase

`sdd-propose`
