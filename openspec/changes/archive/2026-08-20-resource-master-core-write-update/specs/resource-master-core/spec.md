# Delta for resource-master-core

## ADDED Requirements

### Requirement: Consumer-neutral public Update for catalog and resource

The system MUST provide `UpdateCatalog`/`UpdateResource` on the public `Writer` obtained through `WriteCapabilities`. `UpdateCatalog` MUST address the target by `Kind` + `ID`; `UpdateResource` MUST address the target by `ID`. Both requests MUST carry an `expectedRevision` compare-and-swap value and a caller-supplied `Actor`. Both methods MUST defensively copy the incoming request and the returned record. A successful update MUST return the record's newly persisted `Revision`.

#### Scenario: External consumer updates catalog and resource records under CAS

- GIVEN a Go package outside the internal import boundary holds a constructed `Writer` and the current revision of an existing catalog record and resource
- WHEN it calls `UpdateCatalog` with `Kind` + `ID` + `expectedRevision` and `UpdateResource` with `ID` + `expectedRevision`, each matching the record's current persisted revision
- THEN both calls compile and succeed using only public types
- AND each returned record carries a `Revision` greater than the `expectedRevision` supplied

#### Scenario: Caller mutation after the call has no effect

- GIVEN a caller holds the request passed to `UpdateCatalog` or `UpdateResource`
- WHEN it mutates that request or the returned record after the call
- THEN neither the accepted request nor the persisted, returned record changes

### Requirement: Update field completeness through the sole bridge

Every field `UpdateCatalog`/`UpdateResource` requests expose MUST reach the internal `UpdateRevision` command or record through `internal/bridge/resourcecore.Adapter`, the sole translation path for Update, or be omitted with a one-line rationale comment at the omission site. A public update field the internal domain cannot honor MUST be reported as a blocking `MISSING DOMAIN CRITERION`, never silently accepted and ignored.

#### Scenario: Every update field is mapped or documented

- GIVEN a public `UpdateCatalog` or `UpdateResource` request field
- WHEN `internal/bridge/resourcecore.Adapter` translates the request
- THEN the field's value reaches the internal `UpdateRevision` command or record unchanged, or the omission site carries a one-line rationale comment
- AND no field is accepted and silently dropped

### Requirement: Catalog stale-revision update returns CONFLICT, not INTERNAL

A catalog `UpdateCatalog` request whose `expectedRevision` no longer matches the persisted revision MUST return the stable `CONFLICT` category. The internal stale-revision sentinels `errApplicabilityStaleRevision` and `errCatalogStaleRevisionV2` MUST resolve to `domain.ErrRevisionConflict` so `core.Map` classifies them as `CONFLICT`; neither MUST surface to a caller as an opaque `INTERNAL` failure or expose PostgreSQL detail.

#### Scenario: Stale catalog revision conflicts, not crashes

- GIVEN a catalog record has been updated since the caller last read its revision
- WHEN `UpdateCatalog` is called with the caller's now-stale `expectedRevision`
- THEN the caller receives `CONFLICT`
- AND no field or revision of the persisted record changes
- AND the caller receives no `INTERNAL` category and no PostgreSQL detail

#### Scenario: Resource stale revision already conflicts correctly

- GIVEN a resource has been updated since the caller last read its revision
- WHEN `UpdateResource` is called with the caller's now-stale `expectedRevision`
- THEN the caller receives `CONFLICT`
- AND no field or revision of the persisted resource changes

### Requirement: Update-reachable error category coverage

`UpdateCatalog` MUST prove exactly 10 reachable and distinct public error categories: `INVALID_ARGUMENT`, `NOT_FOUND`, `DUPLICATE`, `INVALID_REFERENCE`, `VALIDATION`, `INVALID_CATALOG`, `UNAVAILABLE`, `INTERNAL`, `CONFLICT`, `IMMUTABLE_CODE`. `UpdateResource` MUST prove exactly 9 reachable and distinct public error categories: `INVALID_ARGUMENT`, `NOT_FOUND`, `DUPLICATE`, `INVALID_REFERENCE`, `VALIDATION`, `INTEGRITY`, `UNAVAILABLE`, `INTERNAL`, `CONFLICT`. `IDENTITY_CONFLICT`, `INVALID_LIFECYCLE`, `REACTIVATION_IMPOSSIBLE`, and `IN_USE` MUST NOT be claimed reachable from either Update operation (all four are Delete/Reactivate-only sentinels); `IMMUTABLE_CODE` MUST NOT be claimed reachable from `UpdateResource` (catalog-only concept); `INTEGRITY` MUST NOT be claimed reachable from `UpdateCatalog` — `domain.ErrResourceIntegrity` has no production call site anywhere in the catalog admin repository.

#### Scenario: Update reaches categories Create could not

- GIVEN catalog and resource update requests that individually trigger each of `UpdateCatalog`'s 10 and `UpdateResource`'s 9 reachable categories
- WHEN each is submitted through `UpdateCatalog` or `UpdateResource`
- THEN the caller receives the matching category and no two conditions collapse into the same one
- AND `IDENTITY_CONFLICT`, `INVALID_LIFECYCLE`, `REACTIVATION_IMPOSSIBLE`, and `IN_USE` are never claimed reachable from either operation

#### Scenario: Code change on a referenced catalog record is immutable

- GIVEN a catalog record's `code` field descriptor is `ImmutableOnceReferenced` and the record is currently referenced by at least one other catalog or resource relationship
- WHEN `UpdateCatalog` is called with a changed `code` value and a current `expectedRevision`
- THEN the caller receives `IMMUTABLE_CODE`
- AND the record's persisted `code` value remains unchanged

### Requirement: Actor attribution without persistence on Update

`UpdateCatalog` and `UpdateResource` requests MUST carry a caller-supplied `Actor` string, which the Core MUST NOT authenticate, authorize, or validate. The bridge MUST forward `Actor` to the internal diagnostic seam (`internal/core/diagnostics.go`) alongside the operation and key, reusing the same `core.WithActor`/`ActorFrom` mechanism built for Create. `Actor` MUST NOT be persisted with the updated record, introduce a new column or migration, or appear on any public DTO.

#### Scenario: Actor reaches diagnostics but never persistence on Update

- GIVEN an update request carries a caller-supplied `Actor`
- WHEN `UpdateCatalog` or `UpdateResource` is called, whether it succeeds or fails
- THEN the internal diagnostic seam records `Actor` alongside the operation and key
- AND no persisted column or public DTO carries the `Actor` value

### Requirement: Compiled surface limited to graduated Create and Update

Consistent with the existing "Separate read and per-operation write readiness" requirement, the compiled `WriteCapabilities` interface and `Writer` MUST declare only `CreateCatalog`, `CreateResource`, `UpdateCatalog`, and `UpdateResource` in and after this change. No method for `Deactivate`, `Reactivate`, or `HardDelete` MUST be exported, stubbed, or otherwise discoverable. `cmd/garfex/` and `internal/tui/` MUST have zero changed lines and zero new `resourcecore` references from this change.

#### Scenario: Only Create and Update are discoverable and drivers stay untouched

- GIVEN the compiled `resourcecore` package and the pre-change state of `cmd/garfex/` and `internal/tui/`
- WHEN this change is applied
- THEN only `CreateCatalog`, `CreateResource`, `UpdateCatalog`, and `UpdateResource` exist on `Writer`/`WriteCapabilities`, with no ungraduated stub
- AND `cmd/garfex/` and `internal/tui/` have zero changed lines and zero `resourcecore` references
