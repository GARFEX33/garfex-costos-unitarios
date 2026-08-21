# Delta for resource-master-core

## ADDED Requirements

### Requirement: Consumer-neutral public HardDeleteCatalog

The system MUST provide `HardDeleteCatalog` on the public `Writer` obtained through `WriteCapabilities`, reusing `CatalogLifecycleRequest` (`Kind`+`ID`+`ExpectedRevision`+`Actor`) unchanged — no new request type, no content field (`Values`/`Rules`/`Scope`/`Attributes`). `HardDeleteCatalog` MUST defensively copy the incoming request; a caller mutating it after the call MUST NOT observe any change to the accepted request.

#### Scenario: External consumer hard-deletes an inactive, unreferenced catalog record

- GIVEN a Go package outside the internal import boundary holds a constructed `Writer` and the current revision of an existing, inactive catalog record with zero dependents and zero resource references
- WHEN it calls `HardDeleteCatalog` with `Kind`+`ID`+`ExpectedRevision` matching the record's current persisted revision, using only the reused `CatalogLifecycleRequest` fields
- THEN the call compiles and succeeds using only public types
- AND the record no longer exists in any subsequent read

### Requirement: HardDelete field completeness through the sole bridge, no confirm-read

Every field `HardDeleteCatalog` requests expose MUST reach the internal `HardDeleteRevision` command through `internal/bridge/resourcecore.Adapter`, the sole translation path for this operation, or be omitted with a one-line rationale comment at the omission site. Because `HardDeleteRevision` returns only `error` and the deleted record no longer exists to read back, the bridge MUST NOT issue any confirm-read after a successful delete.

#### Scenario: Every field is mapped, and no confirm-read follows success

- GIVEN a public `CatalogLifecycleRequest` field on a `HardDeleteCatalog` call that is about to succeed
- WHEN `internal/bridge/resourcecore.Adapter` translates the request and the internal `HardDeleteRevision` call returns successfully
- THEN the field's value reached `HardDeleteRevision` unchanged, or its omission site carries a one-line rationale comment
- AND the bridge issues no follow-up read of the now-deleted record

### Requirement: Bridge delegates to Core authority — no guard-chain re-implementation

The public exposure of `HardDeleteCatalog` MUST NOT weaken, duplicate, or bypass the internal service's existing guard chain (`buildV2DeleteCandidate`: active-target rejection, dependents rejection, resource-reference rejection). The bridge MUST delegate every guard decision to `catalogo.Service.HardDeleteRevision` and MUST NOT independently re-evaluate `Active`, `Dependents`, or `ReferencedByResources` before or after calling it.

#### Scenario: A guard-chain rejection surfaces unchanged through the bridge, with no second check

- GIVEN a catalog record that is active, or has a non-zero `Dependents` count, or is referenced by at least one resource
- WHEN `HardDeleteCatalog` is called against it
- THEN the call fails with the exact category `catalogo.Service.HardDeleteRevision`'s own guard chain produced, not an inline bridge-side re-evaluation of the same condition
- AND the record remains persisted, unchanged, at its prior revision

### Requirement: Strict CAS on HardDeleteCatalog — no idempotent no-op bypass

`HardDeleteCatalog` MUST enforce `ExpectedRevision` strictly: a non-zero value is required, and a stale value MUST return `CONFLICT` regardless of the target's current state. Deletion MUST NOT apply the no-op bypass `DeactivateCatalog`/`ReactivateCatalog` apply for an already-matching state — there is no "already deleted" target to short-circuit to, since a deleted record is no longer addressable by `Kind`+`ID`.

#### Scenario: A stale ExpectedRevision yields CONFLICT

- GIVEN an inactive, unreferenced catalog record currently at revision 5
- WHEN `HardDeleteCatalog` is called with `ExpectedRevision` 3 (stale)
- THEN the caller receives `CONFLICT`
- AND the record remains persisted, unchanged, at revision 5

### Requirement: HardDelete-reachable error category coverage

`HardDeleteCatalog` MUST prove exactly 8 reachable and distinct public error categories: `INVALID_ARGUMENT`, `NOT_FOUND`, `INVALID_CATALOG`, `INVALID_LIFECYCLE`, `IN_USE`, `CONFLICT`, `UNAVAILABLE`, `INTERNAL`. `INVALID_LIFECYCLE` and `IN_USE` MUST be proven reachable through the public `resourcecore` surface for the first time by this change. `DUPLICATE`, `INVALID_REFERENCE`, `VALIDATION`, `INTEGRITY`, `IDENTITY_CONFLICT`, `REACTIVATION_IMPOSSIBLE`, and `IMMUTABLE_CODE` MUST NOT be claimed reachable from `HardDeleteCatalog` — all are Create/Update/resource-lifecycle-only sentinels.

#### Scenario: Active-target and in-use rejections are the first-ever reachable proof, within the declared set of 8

- GIVEN one catalog record that is currently active, and a separate inactive record with a non-zero `Dependents` count or at least one resource reference
- WHEN each is submitted through `HardDeleteCatalog`
- THEN the active-target request returns `INVALID_LIFECYCLE` and the dependents/reference request returns `IN_USE`
- AND neither category has been reachable from any previously graduated `resourcecore` write operation
- AND no category outside the declared set of 8 is ever returned by `HardDeleteCatalog`

### Requirement: Actor attribution without persistence on HardDeleteCatalog

`HardDeleteCatalog` requests MUST carry a caller-supplied `Actor` string, which the Core MUST NOT authenticate, authorize, or validate. The bridge MUST forward `Actor` to the internal diagnostic seam (`internal/core/diagnostics.go`) alongside the operation and key, reusing the same `core.WithActor`/`ActorFrom` mechanism built for Create and extended for Update, Deactivate, and Reactivate. `Actor` MUST NOT be persisted, introduce a new column or migration, or appear on any public DTO beyond the request itself.

#### Scenario: Actor reaches diagnostics but never persistence on HardDeleteCatalog

- GIVEN a `HardDeleteCatalog` request carries a caller-supplied `Actor`
- WHEN the call is made, whether it succeeds or fails
- THEN the internal diagnostic seam records `Actor` alongside the operation and key
- AND no persisted column or public DTO other than the originating request carries the `Actor` value

## MODIFIED Requirements

### Requirement: Compiled surface extended to Create, Update, Deactivate, Reactivate, and HardDeleteCatalog

The compiled `WriteCapabilities` interface and `Writer` MUST declare exactly `CreateCatalog`, `CreateResource`, `UpdateCatalog`, `UpdateResource`, `DeactivateCatalog`, `ReactivateCatalog`, `DeactivateResource`, `ReactivateResource`, and `HardDeleteCatalog` — 9 methods — in and after this change. `HardDeleteResource` MUST NOT be exported, stubbed, or otherwise discoverable; it remains a deliberate, documented gap, not addressed by this change. `cmd/garfex/` and `internal/tui/` MUST have zero changed lines and zero new `resourcecore` references from this change.
(Previously: forbade any `HardDelete*` symbol on an 8-method surface; now permits `HardDeleteCatalog` specifically on a 9-method surface while still forbidding `HardDeleteResource`.)

#### Scenario: Create, Update, Deactivate, Reactivate, and HardDeleteCatalog are discoverable; HardDeleteResource and drivers stay untouched

- GIVEN the compiled `resourcecore` package and the pre-change state of `cmd/garfex/` and `internal/tui/`
- WHEN this change is applied
- THEN exactly `CreateCatalog`, `CreateResource`, `UpdateCatalog`, `UpdateResource`, `DeactivateCatalog`, `ReactivateCatalog`, `DeactivateResource`, `ReactivateResource`, and `HardDeleteCatalog` exist on `Writer`/`WriteCapabilities`, with no `HardDeleteResource` stub
- AND `cmd/garfex/` and `internal/tui/` have zero changed lines and zero new `resourcecore` references
