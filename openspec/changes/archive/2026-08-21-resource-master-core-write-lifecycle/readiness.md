# Readiness: resource-master-core-write-lifecycle (Deactivate/Reactivate graduation)

## Verdict

**Deactivate/Reactivate-WRITE is ready.** Unlike Update, this graduation makes zero `internal/postgres` changes — no new postgres wiring gap was found (grep-confirmed, `explore.md`), and `CONFLICT` reachability reuses the sentinel fix already shipped in Update (`casUpdateRevision`, verified independently during design review). There is consequently no live-PostgreSQL integration-evidence gap to disclose for this change specifically: every reachability claim, including the no-op/CAS-bypass asymmetry, is proven at the bridge-injection layer against in-repo fakes that mirror the internal behavior exactly, and that behavior itself is pre-existing, already-shipped, TUI-consumed code this change does not modify.

## What shipped

- `resourcecore.Writer.DeactivateCatalog`/`ReactivateCatalog`/`DeactivateResource`/`ReactivateResource` (+`WriteCapabilities` +4 methods, now 8 total).
- `internal/bridge/resourcecore.Adapter.DeactivateCatalog`/`ReactivateCatalog` with a confirm-read (`a.catalog.Get`) after a successful toggle; `Adapter.DeactivateResource`/`ReactivateResource` with no confirm-read (`domain.LifecycleResult.Resource` already complete). Zero new mapping functions on either side.
- Honest documentation (not correction) of the idempotent no-op / CAS-bypass asymmetry across the four internal lifecycle paths.

## Lifecycle-reachable error category coverage

### Catalog Deactivate / Reactivate — identical, 6 of 15

| Category | Reachable | Evidence |
| --- | --- | --- |
| `INVALID_ARGUMENT` | Yes | `Writer` shape gate; `setActiveRevision`'s `id == 0 \|\| expectedRevision == 0` guard |
| `NOT_FOUND` | Yes | `setActiveRevision`'s `s.repo.Get` absent-record path |
| `INVALID_CATALOG` | Yes | `domain.WrapInvalidCatalog(next.Validate())` after `ApplyCatalogMutation` |
| `CONFLICT` | Yes (not on the no-op path) | `casUpdateRevision`'s stale-revision branch inside `repoV2.SetActive` — reuses the sentinel fix already shipped in Update; no new postgres change needed |
| `UNAVAILABLE` | Yes | `prepareV2Write`'s unavailable sentinels; ctx cancellation |
| `INTERNAL` | Yes | Unclassified fallback |

Excluded, never claimed reachable: `DUPLICATE`, `INVALID_REFERENCE`, `VALIDATION` (APLICABILIDAD gate is `OpInsert`/`OpUpdate`-only), `INTEGRITY` (zero catalog-admin references), `IDENTITY_CONFLICT`/`REACTIVATION_IMPOSSIBLE` (resource-only), `IN_USE`/`IMMUTABLE_CODE` (Delete/Update-only).

### Resource Deactivate — 5 of 15

| Category | Reachable | Evidence |
| --- | --- | --- |
| `INVALID_ARGUMENT` | Yes | `Writer` shape gate; `id <= 0 \|\| expectedRevision == 0` guard |
| `NOT_FOUND` | Yes | `errors.Is(err, domain.ErrResourceNotFound)` pass-through |
| `CONFLICT` | Yes — **CAS-checked even on the no-op** | No app-level short-circuit before `v2.DeactivateRevision` |
| `UNAVAILABLE` | Yes | ctx cancellation |
| `INTERNAL` | Yes | Fallback wrap |

Excluded: `DUPLICATE`, `INVALID_REFERENCE` (Reactivate-only), `VALIDATION`, `INTEGRITY` (identity check gated off, empty on Deactivate), `IDENTITY_CONFLICT` (no such arm on this path).

### Resource Reactivate — 7 of 15 (first graduation reaching IDENTITY_CONFLICT/REACTIVATION_IMPOSSIBLE)

| Category | Reachable | Evidence |
| --- | --- | --- |
| `INVALID_ARGUMENT` | Yes | Same guard |
| `NOT_FOUND` | Yes | `finder.GetByID` absent, or the CAS call |
| `IDENTITY_CONFLICT` | **Yes — new** | `errors.Is(err, domain.ErrResourceIntegrity)` reclassified, or `reactivationCandidate`'s own identity check |
| `REACTIVATION_IMPOSSIBLE` | **Yes — new** | `domain.WrapReactivationImpossible` on `ErrResourceReference`, or on `domain.NewResource` failure |
| `CONFLICT` | Yes (not on the no-op path) | `errors.Is(err, domain.ErrResourceRevisionConflict)` pass-through |
| `UNAVAILABLE` | Yes | ctx cancellation |
| `INTERNAL` | Yes | Fallback |

Excluded: `DUPLICATE`, `INVALID_REFERENCE` (always reclassified to `REACTIVATION_IMPOSSIBLE`), `VALIDATION` (same reclassification), `INTEGRITY` (always reclassified to `IDENTITY_CONFLICT`).

## The no-op / CAS-bypass asymmetry — reused and documented, not fixed

Verified independently against source during exploration and design review (not just relayed):

| Path | Short-circuits before CAS check? |
|---|---|
| Catalog `setActiveRevision` (both ops) | **Yes** — `service.go:572` |
| Resource `DeactivateRevision` | **No** |
| Resource `ReactivateRevision` | **Yes** — `service.go:272-273` |

This is pre-existing, production-hardened `internal/app` behavior, already consumed by the TUI. It is reused as-is through the bridge, not corrected — correcting it would be a behavior change outside a bridge-only graduation's boundary.

**Three-test falsifiable proof pair**, all passing:
- `TestCatalogDeactivate_NoOp_StaleRevision_SilentSuccess` / `TestCatalogReactivate_NoOp_StaleRevision_SilentSuccess` — catalog silently succeeds on a stale `ExpectedRevision` when already in the desired state.
- `TestResourceReactivate_NoOp_StaleRevision_SilentSuccess` — same for resource Reactivate.
- `TestResourceDeactivate_SameCallShape_StaleRevision_Conflict` — the contrast: the identical call shape through resource Deactivate returns `CONFLICT`, not silent success.

## Confirm-read (catalog) / no-confirm-read (resource)

- Catalog: `TestCatalogLifecycle_ConfirmRead_ReturnsCompleteRecord` proves the confirm-read recovers a complete record (not the 4-field partial projection `SetActive` builds internally). `TestCatalogLifecycle_ConfirmReadFailure_SurfacesHonestlyUnreclassified` proves a confirm-read failure surfaces through the ordinary `mapError` path, unreclassified — matching `CreateResource`'s established precedent (`adapter.go:229-234`) exactly.
- Resource: `TestResourceLifecycle_NoConfirmRead_CallCountAssertion` proves zero `Get` calls across both operations, success and failure paths — `domain.LifecycleResult.Resource` is already complete.

## Actor attribution

`Actor` reaches `internal/core/diagnostics.go`'s `core.WithActor`/`ActorFrom` seam on both success and failure paths for all 4 operations (`TestCatalogLifecycle_ActorReachesDiagnosticSeam`, `TestResourceLifecycle_ActorReachesDiagnosticSeam`) — Create/Update's mechanism reused verbatim, zero redesign. Never persisted, no new column or migration, never on a public DTO.

## Compiled surface

`WriteCapabilities`/`Writer` compile exactly 8 methods: `CreateCatalog`, `CreateResource`, `UpdateCatalog`, `UpdateResource`, `DeactivateCatalog`, `ReactivateCatalog`, `DeactivateResource`, `ReactivateResource` (`TestWriter_NoUngraduatedMethodExported`, reflection-based). No `HardDelete` symbol, stub, or discoverable artifact.

## Zero-touch confirmation

- `git diff --stat -- cmd/garfex internal/tui internal/app internal/domain internal/postgres` — empty for all five.
- `rg -l resourcecore cmd/garfex internal/tui` — no matches.
- `internal/core/errors.go`'s `Map` — unchanged; `IdentityConflict`/`ReactivationImpossible` arms already existed, already correctly ordered.

## Full-suite proof

`go build ./...`, `gofmt -l .`, `go vet ./...`, `golangci-lint run` (0 issues on `resourcecore` and `internal/bridge/resourcecore`), and `go test ./... -count=1` all pass for every real project package. The only failure in the full-suite run is the pre-existing, unrelated `agent/skills/golang-cli/assets/examples` package (untracked, missing third-party dependencies, unconnected to this change — present before this change started).

## Changed-line summary

1029 insertions, 22 deletions across `resourcecore/{write_types,writer,writer_test,external_test}.go` and `internal/bridge/resourcecore/{adapter,adapter_test}.go` — 6 files, ~1051 total changed lines, split across the U1/U2/U3/U4 stacked chain per the review-budget forecast (some per-unit `size:exception` documentation expected at delivery, matching Update's own precedent for test-evidence-heavy units).
