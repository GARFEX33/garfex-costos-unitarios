# Delta for resource-master-core

## ADDED Requirements

### Requirement: Consumer-neutral public Deactivate/Reactivate for catalog and resource

The system MUST provide `DeactivateCatalog`/`ReactivateCatalog`/`DeactivateResource`/`ReactivateResource` on the public `Writer` obtained through `WriteCapabilities`. `DeactivateCatalog`/`ReactivateCatalog` MUST address the target by `Kind` + `ID`; `DeactivateResource`/`ReactivateResource` MUST address the target by `ID`. All four requests MUST carry an `ExpectedRevision` compare-and-swap value and a caller-supplied `Actor`, and MUST NOT carry any content field (`Values`, `Rules`, `Scope`, `Attributes`, or similar) — these operations toggle lifecycle state only. All four methods MUST defensively copy the incoming request and the returned record; a caller mutating either after the call MUST NOT observe any change to the accepted request or the persisted, returned record.

#### Scenario: External consumer deactivates and reactivates a catalog record and a resource under CAS

- GIVEN a Go package outside the internal import boundary holds a constructed `Writer` and the current revision of an existing, active catalog record and an existing, active resource
- WHEN it calls `DeactivateCatalog` with `Kind` + `ID` + `ExpectedRevision` matching the catalog record's current persisted revision, and `DeactivateResource` with `ID` + `ExpectedRevision` matching the resource's current persisted revision
- THEN both calls compile and succeed using only public types
- AND each returned record reports `Active == false`

#### Scenario: External consumer reactivates a previously deactivated catalog record and resource

- GIVEN a catalog record and a resource that are both currently inactive, and the caller holds each one's current persisted revision
- WHEN it calls `ReactivateCatalog` and `ReactivateResource` with matching `ExpectedRevision` values
- THEN both calls succeed using only public types
- AND each returned record reports `Active == true`

#### Scenario: No content field exists on a lifecycle request

- GIVEN the public `CatalogLifecycleRequest` and `ResourceLifecycleRequest` types
- WHEN their fields are inspected
- THEN neither type declares `Values`, `Rules`, `Scope`, `Attributes`, or any other content field
- AND both declare only an identity (`Kind`+`ID` or `ID`), `ExpectedRevision`, and `Actor`

### Requirement: Lifecycle field completeness through the sole bridge, including the catalog confirm-read

Every field `DeactivateCatalog`/`ReactivateCatalog`/`DeactivateResource`/`ReactivateResource` requests expose MUST reach the internal `DeactivateRevision`/`ReactivateRevision` command through `internal/bridge/resourcecore.Adapter`, the sole translation path for these four operations, or be omitted with a one-line rationale comment at the omission site. Because the internal catalog `DeactivateRevision`/`ReactivateRevision` methods return only `error` — no record — the bridge MUST issue a confirm-read (`a.catalog.Get`) immediately after a successful catalog toggle to obtain the complete record it returns publicly. A failure of that confirm-read MUST pass through the ordinary error mapping unchanged — never reclassified — matching the precedent already established for `CreateResource`'s confirm-read (`internal/bridge/resourcecore/adapter.go:229-234`). The resource side of both operations MUST NOT perform a confirm-read; the internal `domain.LifecycleResult.Resource` it already returns is the complete, persisted resource.

#### Scenario: Every lifecycle field is mapped or documented

- GIVEN a public `CatalogLifecycleRequest` or `ResourceLifecycleRequest` field
- WHEN `internal/bridge/resourcecore.Adapter` translates the request
- THEN the field's value reaches the internal `DeactivateRevision`/`ReactivateRevision` command unchanged, or the omission site carries a one-line rationale comment
- AND no field is accepted and silently dropped

#### Scenario: Catalog confirm-read returns the complete record after a successful toggle

- GIVEN a catalog record whose lifecycle toggle (`DeactivateCatalog` or `ReactivateCatalog`) is about to succeed
- WHEN the internal `DeactivateRevision`/`ReactivateRevision` call returns successfully with no record
- THEN the bridge issues a confirm-read by `Kind`+`ID` before returning
- AND the caller receives the complete persisted record, not the 4-field (`Kind`/`ID`/`Revision`/`Active`) partial projection the internal postgres `SetActive` path produces internally

#### Scenario: Catalog confirm-read failure surfaces honestly, unreclassified

- GIVEN a catalog lifecycle toggle (`DeactivateCatalog` or `ReactivateCatalog`) whose underlying state change has already committed durably
- WHEN the bridge's follow-up confirm-read fails (for example, a transient connection error)
- THEN the caller receives the confirm-read's own mapped error category, unmodified by any lifecycle-specific reclassification
- AND the caller is never told the toggle itself failed when the persisted state has, in fact, already changed

#### Scenario: Resource lifecycle toggles need no confirm-read

- GIVEN a resource lifecycle toggle (`DeactivateResource` or `ReactivateResource`) that succeeds
- WHEN the bridge translates the internal `domain.LifecycleResult`
- THEN the bridge issues no additional read call
- AND the returned `Resource` is built directly from `domain.LifecycleResult.Resource`

### Requirement: Idempotent no-op / CAS-bypass asymmetry, reused and documented

`DeactivateCatalog`, `ReactivateCatalog`, and `ReactivateResource` MUST short-circuit to a no-op success — returning the current persisted record unchanged, with no `CONFLICT` — when the target is already in the requested active state, even if the caller's `ExpectedRevision` no longer matches the persisted revision; the short-circuit MUST occur before any revision comparison. `DeactivateResource` MUST NOT apply this bypass: it MUST enforce `ExpectedRevision` strictly and return `CONFLICT` on a stale value even when the resource is already inactive. This asymmetry is pre-existing, production-hardened internal behavior (`internal/app/catalogo/service.go`'s `setActiveRevision`; `internal/app/recursos/service.go`'s `ReactivateRevision`/`DeactivateRevision`), already relied on by the TUI; this change reuses it as-is through the bridge and documents it as an observed caller-facing contract, not a defect this graduation corrects.

#### Scenario: Catalog Deactivate bypasses a stale revision when already deactivated

- GIVEN a catalog record that is already `Active == false`, currently at revision 5
- WHEN `DeactivateCatalog` is called with `ExpectedRevision` 3 (stale)
- THEN the call succeeds
- AND the caller receives the current persisted record at revision 5, `Active == false`, with no `CONFLICT`

#### Scenario: Catalog Reactivate bypasses a stale revision when already active

- GIVEN a catalog record that is already `Active == true`, currently at revision 5
- WHEN `ReactivateCatalog` is called with `ExpectedRevision` 3 (stale)
- THEN the call succeeds
- AND the caller receives the current persisted record at revision 5, `Active == true`, with no `CONFLICT`

#### Scenario: Resource Reactivate bypasses a stale revision when already active

- GIVEN a resource that is already `Active == true`, currently at revision 5
- WHEN `ReactivateResource` is called with `ExpectedRevision` 3 (stale)
- THEN the call succeeds
- AND the caller receives the current persisted resource at revision 5, `Active == true`, with no `CONFLICT`

#### Scenario: Resource Deactivate enforces CAS strictly even when already inactive

- GIVEN a resource that is already `Active == false`, currently at revision 5
- WHEN `DeactivateResource` is called with `ExpectedRevision` 3 (stale)
- THEN the caller receives `CONFLICT`
- AND the persisted resource's revision and `Active` value remain unchanged

### Requirement: Lifecycle-reachable error category coverage

`DeactivateCatalog` and `ReactivateCatalog` MUST each prove exactly the same 6 reachable and distinct public error categories: `INVALID_ARGUMENT`, `NOT_FOUND`, `INVALID_CATALOG`, `CONFLICT` (not reachable on the no-op path — see the no-op/CAS-bypass requirement), `UNAVAILABLE`, `INTERNAL`. `DeactivateResource` MUST prove exactly 5 reachable and distinct public error categories: `INVALID_ARGUMENT`, `NOT_FOUND`, `CONFLICT` (CAS-checked even on the no-op path), `UNAVAILABLE`, `INTERNAL`. `ReactivateResource` MUST prove exactly 7 reachable and distinct public error categories: `INVALID_ARGUMENT`, `NOT_FOUND`, `IDENTITY_CONFLICT`, `REACTIVATION_IMPOSSIBLE`, `CONFLICT` (not reachable on the no-op path), `UNAVAILABLE`, `INTERNAL`. `DUPLICATE`, `INVALID_REFERENCE`, `VALIDATION`, `INTEGRITY`, `IN_USE`, `IMMUTABLE_CODE`, and `INVALID_LIFECYCLE` MUST NOT be claimed reachable from any of the four lifecycle operations — all are Create/Update/Delete-only sentinels or, for `INTEGRITY`/`IDENTITY_CONFLICT` specifically, unreachable from `DeactivateResource` because its identity check is gated on a non-empty `expectedIdentityKey`, which Deactivate never sets.

#### Scenario: Catalog Deactivate and Reactivate reach an identical 6-category set

- GIVEN catalog lifecycle requests that individually trigger each of `INVALID_ARGUMENT`, `NOT_FOUND`, `INVALID_CATALOG`, `CONFLICT`, `UNAVAILABLE`, and `INTERNAL`
- WHEN each is submitted through `DeactivateCatalog` and, separately, through `ReactivateCatalog`
- THEN both operations return the matching category for every trigger
- AND no category outside this set of 6 is ever returned by either operation

#### Scenario: Resource Reactivate is the first operation to reach IDENTITY_CONFLICT and REACTIVATION_IMPOSSIBLE

- GIVEN a resource reactivation request whose target identity now collides with another resource's identity, and a separate request whose rebuilt candidate fails domain reconstruction
- WHEN each is submitted through `ReactivateResource`
- THEN the identity-collision request returns `IDENTITY_CONFLICT`
- AND the reconstruction-failure request returns `REACTIVATION_IMPOSSIBLE`
- AND neither category has been reachable from any previously graduated `resourcecore` write operation (`CreateCatalog`, `CreateResource`, `UpdateCatalog`, `UpdateResource`, `DeactivateCatalog`, `ReactivateCatalog`, `DeactivateResource`)

#### Scenario: Resource Deactivate never returns IDENTITY_CONFLICT

- GIVEN any `DeactivateResource` request, valid or invalid
- WHEN it is submitted, regardless of outcome
- THEN the caller never receives `IDENTITY_CONFLICT`, `REACTIVATION_IMPOSSIBLE`, `DUPLICATE`, `INVALID_REFERENCE`, `VALIDATION`, or `INTEGRITY`

### Requirement: Actor attribution without persistence on Deactivate/Reactivate

`DeactivateCatalog`, `ReactivateCatalog`, `DeactivateResource`, and `ReactivateResource` requests MUST carry a caller-supplied `Actor` string, which the Core MUST NOT authenticate, authorize, or validate. The bridge MUST forward `Actor` to the internal diagnostic seam (`internal/core/diagnostics.go`) alongside the operation and key, reusing the same `core.WithActor`/`ActorFrom` mechanism built for Create and extended for Update. `Actor` MUST NOT be persisted with the toggled record, introduce a new column or migration, or appear on any public DTO beyond the request itself.

#### Scenario: Actor reaches diagnostics but never persistence on Deactivate/Reactivate

- GIVEN a lifecycle request carries a caller-supplied `Actor`
- WHEN `DeactivateCatalog`, `ReactivateCatalog`, `DeactivateResource`, or `ReactivateResource` is called, whether it succeeds or fails
- THEN the internal diagnostic seam records `Actor` alongside the operation and key
- AND no persisted column or public DTO other than the originating request carries the `Actor` value

### Requirement: Compiled surface extended to Create, Update, Deactivate, and Reactivate

The compiled `WriteCapabilities` interface and `Writer` MUST declare exactly `CreateCatalog`, `CreateResource`, `UpdateCatalog`, `UpdateResource`, `DeactivateCatalog`, `ReactivateCatalog`, `DeactivateResource`, and `ReactivateResource` — 8 methods — in and after this change. No method for `HardDelete` MUST be exported, stubbed, or otherwise discoverable. `cmd/garfex/` and `internal/tui/` MUST have zero changed lines and zero new `resourcecore` references from this change. This requirement supersedes and obsoletes the prior "Compiled surface limited to graduated Create and Update" requirement (archived `2026-08-20-resource-master-core-write-update`), which becomes a false claim once this change lands and MUST be removed from the merged spec at archive time.

#### Scenario: Create, Update, Deactivate, and Reactivate are discoverable; HardDelete and drivers stay untouched

- GIVEN the compiled `resourcecore` package and the pre-change state of `cmd/garfex/` and `internal/tui/`
- WHEN this change is applied
- THEN exactly `CreateCatalog`, `CreateResource`, `UpdateCatalog`, `UpdateResource`, `DeactivateCatalog`, `ReactivateCatalog`, `DeactivateResource`, and `ReactivateResource` exist on `Writer`/`WriteCapabilities`, with no `HardDelete` stub
- AND `cmd/garfex/` and `internal/tui/` have zero changed lines and zero new `resourcecore` references
