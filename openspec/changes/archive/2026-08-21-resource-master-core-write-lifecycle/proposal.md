# Proposal: Expose public WRITE Deactivate/Reactivate on the Resource Master Core

## Intent

Third per-operation WRITE graduation on `resourcecore`, after `Create`
(shipped, archived `2026-08-21-resource-master-core-write`, issue #148) and
`Update` (shipped, archived `2026-08-20-resource-master-core-write-update`,
issue #152). External consumers (PI) need to toggle a catalog record's or
resource's lifecycle state — take it out of active use or bring it back —
under the same optimistic-concurrency contract already proven for Create and
Update. `Deactivate` and `Reactivate` are graduated together because they
share one internal mechanism (`setActiveRevision` on the catalog side,
symmetric `*Revision` methods on the resource side); splitting them would
duplicate the same bridge and readiness work twice for no product benefit.
`HardDelete` stays out — it is the only lifecycle operation that destroys
data instead of toggling a flag, and is deliberately graduated last, on its
own.

Internal authority already exists and is production-hardened
(`catalogo.Service.DeactivateRevision`/`ReactivateRevision`,
`recursos.Service.DeactivateRevision`/`ReactivateRevision`, all consumed by
the TUI today); this change is the public mirror and its sole bridge
translation. No internal behavior changes.

## Scope

### In Scope
- `resourcecore.Writer.DeactivateCatalog`/`ReactivateCatalog`/
  `DeactivateResource`/`ReactivateResource` (4 methods) + `WriteCapabilities`
  gains exactly these four (Create, Update untouched).
- Request DTOs: `CatalogLifecycleRequest` (`Kind`+`ID`) and
  `ResourceLifecycleRequest` (`ID`), both carrying `ExpectedRevision uint64`
  (CAS) and `Actor string`. Scalar-only — no `Clone*` function needed; Go
  value semantics already give full immutability.
- Bridge translation for all 4 methods in `internal/bridge/resourcecore.Adapter`:
  - Catalog side (both ops): a confirm-read (`a.catalog.Get`) after a
    successful call, because the internal `DeactivateRevision`/
    `ReactivateRevision` return only `error` — no record — and postgres's
    `SetActive` builds a deliberately partial 4-field projection
    (`Kind`/`ID`/`Revision`/`Active`; `Values`/`Rules` always empty) even at
    the domain-port level, so widening the internal signature would not by
    itself solve completeness.
  - Resource side (both ops): direct pass-through, no confirm-read — already
    returns the complete, persisted `domain.LifecycleResult{Resource,
    Changed}`.
- Honest documentation (readiness.md + spec) of the idempotent no-op /
  CAS-bypass asymmetry: catalog Deactivate+Reactivate and resource
  Reactivate silently succeed on a stale `ExpectedRevision` when the record
  is already in the desired state (short-circuit happens before the CAS
  compare); resource Deactivate enforces CAS strictly even on the no-op
  (postgres checks revision before active-state). Reused as-is — fixing it
  would change already-shipped `internal/app` behavior the TUI depends on,
  which is outside a bridge-only graduation's boundary.
- Readiness evidence across 3 distinct reachability tables: catalog shared
  (Deactivate+Reactivate identical) 6 of 15 categories, resource Deactivate
  5 of 15, resource Reactivate 7 of 15 — the first graduation to reach
  `IDENTITY_CONFLICT` and `REACTIVATION_IMPOSSIBLE`.

### Out of Scope
- `HardDelete` graduation (separate, later, final change — deliberately
  most-destructive-last).
- Any `internal/tui`/`cmd/garfex` change.
- Any read-contract change, including surfacing a `Changed`/no-op-signal
  field or an "include inactive" read filter.
- Fixing the idempotent no-op / CAS-bypass asymmetry — documented as
  reused-as-is (see In Scope), not corrected.
- New `ErrorCode` values (all 15 already exist).

## Capabilities

### New Capabilities
None.

### Modified Capabilities
- `resource-master-core`: extend "Separate read and per-operation write
  readiness" with Deactivate/Reactivate graduation for catalog and
  resource; the lifecycle request shape (`CatalogLifecycleRequest`/
  `ResourceLifecycleRequest`, no content fields); the catalog confirm-read
  design decision; the documented no-op/CAS-bypass asymmetry; and the three
  reachability tables (catalog shared 6, resource Deactivate 5, resource
  Reactivate 7 — first to prove `IDENTITY_CONFLICT`/
  `REACTIVATION_IMPOSSIBLE`).

## Approach

Mirror Create/Update's pattern: package-owned DTOs, defensive copy on entry
(trivial here — scalar fields only), `Writer` does shape validation only (no
business rules), sole-bridge translation, `Actor` via
`core.WithActor`/`ActorFrom` — never persisted, never validated, consistent
with Create/Update. The one structural novelty versus Update is the catalog
confirm-read: `DeactivateRevision`/`ReactivateRevision` return only `error`,
so the bridge issues `a.catalog.Get` immediately after a successful call to
obtain a complete record to return publicly, accepting one extra round trip.
Resource side needs no such step. No `core.Map` change — every reachable
category for all three tables already has a correctly-ordered arm; no
analogous "stand-in sentinel" bug (the one Update's CONFLICT fix addressed)
exists on these paths.

## Affected Areas

| Area | Impact | Description |
|------|--------|--------------|
| `resourcecore/write_types.go` | Modified | `CatalogLifecycleRequest`/`ResourceLifecycleRequest` DTOs |
| `resourcecore/writer.go` | Modified | `DeactivateCatalog`/`ReactivateCatalog`/`DeactivateResource`/`ReactivateResource`, `WriteCapabilities` +4 methods |
| `resourcecore/copy.go` | Modified | Defensive copies for lifecycle requests (no `Clone*` needed — scalar-only) |
| `internal/bridge/resourcecore/adapter.go` | Modified | 4 delegating methods; catalog confirm-read after `DeactivateRevision`/`ReactivateRevision` |
| `cmd/garfex/`, `internal/tui/` | Unchanged | Zero coupling preserved |
| Tests | New/Modified | `writer_test.go`, `adapter_test.go`, 3 reachability-table suites (catalog shared, resource Deactivate, resource Reactivate) |

## Risks

| Risk | Likelihood | Mitigation |
|------|------------|------------|
| Catalog confirm-read fails after the underlying `SetActive` already committed (state changed durably, but the follow-up `Get` errors) | Low | Document as a known limitation in readiness.md; report the confirm-read failure as the public result rather than silently swallowing it — needs explicit product sign-off (see question round below) |
| Consumers misread the no-op/CAS-bypass asymmetry as a bug rather than documented, reused-as-is behavior | Med | Explicit readiness.md + spec scenario coverage for all 3 paths; proposal and spec both state the asymmetry is intentionally unfixed |
| Confirm-read adds a network round trip to every catalog Deactivate/Reactivate call | Low | Accepted cost; flagged, no perf budget defined for this graduation |
| Slice exceeds 400-line review budget | Medium-High | 4-unit auto-chain per explore.md estimate (~650-950 lines): U1 public contract, U2 catalog bridge, U3 resource bridge, U4 readiness + full-suite verification |
| Live-PostgreSQL integration evidence gap in a sandboxed apply session | Med | Same disclosed gap as Update; documented, not silently skipped |

## Rollback Plan

Revert `DeactivateCatalog`/`ReactivateCatalog`/`DeactivateResource`/
`ReactivateResource` and their bridge methods independently; Create, Update,
and Read stay unaffected. No persisted data, migration, or internal
`internal/app`/`internal/domain` behavior is touched by either the change or
its rollback.

## Dependencies

- `resource-master-core-write` (Create), merged to main, archived
  `2026-08-21-resource-master-core-write` (issue #148, PRs #149-151).
- `resource-master-core-write-update` (Update), merged to main, archived
  `2026-08-20-resource-master-core-write-update` (issue #152, PRs #153-155).

## Success Criteria

- [ ] External consumer deactivates and reactivates a catalog record and a
      resource via `resourcecore` with `ExpectedRevision` CAS, no `internal`
      import.
- [ ] `WriteCapabilities` compiles Create + Update + Deactivate + Reactivate
      only — no `HardDelete` stub or export.
- [ ] Catalog confirm-read returns a complete record on success; its failure
      mode is explicit and tested, not silently swallowed.
- [ ] Every public lifecycle request field maps to its internal destination
      or carries a one-line omission rationale.
- [ ] Catalog proves 6 categories (Deactivate+Reactivate identical), resource
      Deactivate proves 5, resource Reactivate proves 7 — including
      `IDENTITY_CONFLICT`/`REACTIVATION_IMPOSSIBLE` for the first time.
- [ ] The no-op/CAS-bypass asymmetry is documented in readiness.md and the
      spec for all 3 paths, explicitly marked reused-as-is, not fixed.
- [ ] No PostgreSQL detail leaks through any error string or type.
- [ ] `cmd/garfex/`, `internal/tui/` have zero changed lines.

## Proposal question round

These are product decisions this proposal had to assume to stay unblocked.
Please confirm, correct, or ask for a second round.

1. **Confirm-read failure after a committed catalog write.** If the
   catalog `Get` confirm-read fails right after `SetActive` already
   committed the state change durably (e.g. a transient connection blip),
   should `resourcecore` report that as a failed operation (`INTERNAL`/
   `UNAVAILABLE`, even though the toggle actually succeeded), or attempt a
   best-effort retry before failing? **Assumption used**: report the
   confirm-read failure honestly as the operation's result and document it
   as a known limitation — do not silently mask a real state change behind
   an apparent failure, and do not add retry logic (no such precedent
   exists in Create/Update's bridge).
2. **No-op visibility to the caller.** Should the public contract expose
   whether a Deactivate/Reactivate call actually changed anything (mirroring
   internal `LifecycleResult.Changed`), or is returning the current
   persisted record silently sufficient, consistent with how the internal
   `internal/app` layer already behaves? **Assumption used**: no new
   `Changed` field — return the record only, matching Update's DTO shape
   and keeping this graduation's public surface minimal; the underlying
   asymmetry is documented but not newly exposed to callers.
3. **Actor requirement.** Should `Actor` be mandatory (non-empty) for
   lifecycle operations specifically, given they are consequential
   state-changes, or does it stay optional/unauthenticated exactly as
   Create/Update treat it? **Assumption used**: unchanged precedent —
   optional, unauthenticated, forwarded to diagnostics only, never
   persisted or validated.
4. **Read-side visibility of deactivated records.** Confirm no new "include
   inactive" read filter or visibility change is needed for this
   graduation — deactivated catalog/resource records keep being visible
   through whatever the existing Read capability already returns.
   **Assumption used**: unchanged; any read-contract change is explicitly
   out of scope here.

If these assumptions hold, no changes to this proposal are needed and it is
ready for `sdd-spec`/`sdd-design`.
