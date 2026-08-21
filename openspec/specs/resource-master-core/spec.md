# Resource Master Core Specification

## Purpose

Define a consumer-neutral, read-first Resource Master contract and the correctness conditions that each catalog or resource write operation must satisfy before it becomes publicly available.

## Requirements

### Requirement: Consumer-neutral read-only Go contract

The system MUST provide an importable public Go contract for Resource Master read operations whose public types are owned by that contract. A consumer MUST be able to construct and use the read-only contract without importing an `internal` package, naming an internal service or repository type, implementing an infrastructure port, or receiving a database, repository, mutable snapshot, or publication control through the public API. The contract MUST delegate authoritative resource and catalog behavior rather than establish a second business authority.

#### Scenario: External consumer uses the read contract

- GIVEN a Go package outside the module's internal import boundary
- WHEN it constructs the Resource Master read contract and performs supported reads
- THEN it compiles and operates using only public module packages and public contract types
- AND no public signature requires or returns an internal or infrastructure-specific type

#### Scenario: Read-only authority cannot mutate state

- GIVEN a consumer has only the read-only contract
- WHEN its available methods and returned values are inspected
- THEN no resource or catalog mutation, repository access, snapshot publication, or mutable authority control is available

### Requirement: Generic catalog discovery and projection

The read-only contract MUST expose the active catalog classes and the descriptor-driven catalog structure. It MUST support descriptor and record projections for all registered kinds: `CLASE`, `FAMILIA`, `TIPO`, `CARACTERISTICA`, `CONJUNTO_OPCIONES`, `OPCION`, `RELACION_OPCIONES`, `UNIDAD`, `POLITICA_UNIDAD`, `APLICABILIDAD`, and `PRESENTACION`. Each projection MUST preserve its kind code, active state, public values, references, and lossless typed-value meaning. Catalog reads MUST support explicit lifecycle filtering and, where records are listed, deterministic filtering and pagination metadata.

#### Scenario: Consumer traverses active catalog structure

- GIVEN the authority contains active and inactive classes and related catalog records
- WHEN a consumer requests active classes and follows the returned catalog descriptors
- THEN only active classes are returned at the first step
- AND every reachable record identifies one of the 11 registered kinds and preserves its public values and references

#### Scenario: Consumer explicitly includes inactive records

- GIVEN a catalog kind contains active and inactive records
- WHEN a consumer requests that kind with an explicit lifecycle scope that includes inactive records
- THEN records in both states are eligible for the result
- AND each returned record reports its active state

### Requirement: Canonical values and defensive copies

Every numeric value exposed by the public contract MUST use a canonical, lossless base-10 string: no exponent notation, no redundant leading or trailing fractional zeroes, no leading plus sign, and zero represented as `0`. Public values MUST retain enough type information to distinguish supported scalar and collection value kinds, including `NOT_APPLICABLE`. Every mutable map, slice, nested rule collection, option collection, and byte-like value accepted or returned at the public boundary MUST be defensively copied.

#### Scenario: Numeric values have one public representation

- GIVEN equivalent stored numeric values with differing scale or textual representation
- WHEN they are projected through the public contract
- THEN each is returned as the same canonical numeric string
- AND its typed-value meaning is preserved

#### Scenario: Not-applicable remains distinct

- GIVEN an attribute value is `NOT_APPLICABLE`
- WHEN the resource or catalog record is projected
- THEN the public value identifies `NOT_APPLICABLE` distinctly from null, an empty string, and numeric zero

#### Scenario: Caller mutation cannot alter contract state

- GIVEN a caller has supplied or received a DTO containing nested mutable collections
- WHEN the caller mutates those collections after the API call
- THEN previously accepted state, authority state, and other returned DTOs remain unchanged

### Requirement: Resource query, detail, and canonical presentation

The read-only contract MUST support deterministic resource listing and search with explicit lifecycle scope, filters, and pagination. A resource detail MUST include its stable resource ID, durable `identity-v1`, scope, natural unit, active state, and complete lossless typed attributes. Canonical presentation MUST be the presentation returned by the authoritative resource service and MUST NOT be reconstructed independently at the public boundary.

#### Scenario: Search, filter, and page resources

- GIVEN resources span multiple classes, scopes, natural units, and lifecycle states
- WHEN a consumer searches with explicit filters, lifecycle scope, page size, and page position
- THEN only matching resources are returned in deterministic order
- AND the response reports sufficient pagination metadata to request the next page without duplicates or omissions

#### Scenario: Read complete resource detail

- GIVEN a resource has typed attributes, scope, natural unit, identity, and lifecycle state
- WHEN the consumer requests its detail
- THEN all those values are returned losslessly
- AND the public DTO does not expose the underlying domain object

#### Scenario: Obtain canonical presentation

- GIVEN the authoritative resource service defines a canonical presentation for a resource
- WHEN the consumer requests that resource's presentation
- THEN the contract returns exactly that authoritative presentation

### Requirement: Durable and opaque identity semantics

The system MUST preserve resource `identity-v1` as the canonical durable resource business identity across migrations and compatible updates. Hash-derived catalog IDs MUST be treated as opaque references only; the public contract MUST NOT claim that they are durable business identities, collision-free, or stable after a natural-code change.

#### Scenario: Resource identity survives compatible persistence change

- GIVEN an existing resource has an `identity-v1` value
- WHEN revision support is migrated, backfilled, or used by later compatible writes
- THEN the resource retains the same `identity-v1` value and identity semantics

#### Scenario: Catalog natural code changes

- GIVEN a catalog kind uses a hash-derived ID
- WHEN an allowed update changes the record's natural code
- THEN consumers are not promised that the previous ID remains valid or equal to the updated ID
- AND the record's supported natural identity fields remain available through its projection

### Requirement: Stable public GARFEX errors

Every public failure MUST map deterministically from an existing authoritative internal outcome or a dedicated Core sentinel to one stable GARFEX category suitable for programmatic identity. The public categories MUST distinguish at minimum `INVALID_ARGUMENT`, `NOT_FOUND`, `DUPLICATE`, `INVALID_REFERENCE`, `VALIDATION`, `INTEGRITY`, `IDENTITY_CONFLICT`, `INVALID_LIFECYCLE`, `REACTIVATION_IMPOSSIBLE`, `INVALID_CATALOG`, `IN_USE`, `IMMUTABLE_CODE`, `CONFLICT`, `UNAVAILABLE`, and `INTERNAL`. `IDENTITY_CONFLICT` MUST remain distinct from both generic `DUPLICATE` and `INTEGRITY`; `CONFLICT` MUST identify a stale concurrent revision rather than those identity or integrity failures. When the Core can determine identity conflict, invalid lifecycle, impossible reactivation, or invalid catalog semantics, it MUST NOT collapse that outcome into an opaque generic error. Public error values, messages, types, formatting, causes, and unwrap chains MUST NOT expose or permit inspection of technical causes or PostgreSQL details, including driver types, `PgError`, SQLSTATE values, constraints, tables, columns, server messages, or infrastructure wrapping.

#### Scenario: Required public categories map deterministically

- GIVEN authoritative outcomes or dedicated Core sentinels exist for invalid input, absence, duplication, invalid reference, validation, integrity, identity conflict, invalid lifecycle, impossible reactivation, invalid catalog, dependency use, immutable code, stale revision, service unavailability, and unexpected failure
- WHEN each failure crosses the public boundary
- THEN it maps respectively to `INVALID_ARGUMENT`, `NOT_FOUND`, `DUPLICATE`, `INVALID_REFERENCE`, `VALIDATION`, `INTEGRITY`, `IDENTITY_CONFLICT`, `INVALID_LIFECYCLE`, `REACTIVATION_IMPOSSIBLE`, `INVALID_CATALOG`, `IN_USE`, `IMMUTABLE_CODE`, `CONFLICT`, `UNAVAILABLE`, and `INTERNAL`
- AND repeated translation of the same internal outcome or Core sentinel produces the same public category

#### Scenario: Distinct Core semantics are not collapsed

- GIVEN the Core can determine that a request has an identity conflict, violates its current lifecycle, cannot reactivate the target, or uses an invalid catalog value
- WHEN the failure crosses the public boundary
- THEN the caller receives respectively `IDENTITY_CONFLICT`, `INVALID_LIFECYCLE`, `REACTIVATION_IMPOSSIBLE`, or `INVALID_CATALOG`
- AND none of those outcomes is replaced by an opaque generic error
- AND `IDENTITY_CONFLICT` is not reported as `DUPLICATE` or `INTEGRITY`

#### Scenario: Stale revision remains a concurrency conflict

- GIVEN an authoritative write rejects a stale expected revision
- WHEN the failure crosses the public boundary
- THEN the caller receives `CONFLICT`
- AND the category is distinct from `IDENTITY_CONFLICT`, `DUPLICATE`, and `INTEGRITY`

#### Scenario: Unexpected database failure remains opaque

- GIVEN a technical failure contains a PostgreSQL message, SQLSTATE, constraint name, and infrastructure-wrapped cause
- WHEN it is translated to a public error
- THEN the caller receives only the applicable `UNAVAILABLE` or `INTERNAL` GARFEX error
- AND formatting, type inspection, cause access, and recursive unwrapping reveal none of the technical cause or PostgreSQL detail

### Requirement: Complete catalog lifecycle

Each of the 11 registered catalog kinds MUST support create, update, deactivate, reactivate, and guarded hard delete through the authoritative catalog boundary. Successful create and update MUST preserve every supported field, active state, reference, option set, map, slice, nested value, and definition relationship without aliasing or silent loss.

#### Scenario: Every kind completes every lifecycle operation

- GIVEN a valid record for each of the 11 registered catalog kinds
- WHEN create, update, deactivate, reactivate, and guarded hard delete are exercised for each kind under valid preconditions
- THEN every operation produces the requested authoritative state
- AND no kind is treated as a snapshot no-op or as lacking lifecycle support

#### Scenario: Mutation materialization is lossless

- GIVEN a catalog mutation contains active state, references, option collections, maps, slices, or nested values supported by its kind
- WHEN the mutation is accepted and later read from authority and persistence
- THEN all supported values are equivalent to the accepted mutation
- AND later caller changes to the original input cannot alter either result

### Requirement: Conservative guarded hard delete

A catalog hard delete MUST be authorized only when the target exists, is inactive, and has no known dependency or retained relationship. Every active, inactive, historical, blocking, or non-blocking relationship MUST block hard delete by default, including catalog-to-catalog and resource-to-catalog relationships. Consumer preflight MUST NOT substitute for the authoritative guard, and a dependency introduced during a delete race MUST still prevent physical deletion.

#### Scenario: Active target cannot be hard deleted

- GIVEN a catalog record is active and otherwise has no dependencies
- WHEN hard delete is requested with the current revision
- THEN the operation is rejected
- AND persistence and published authority remain unchanged

#### Scenario: Any dependency blocks deletion

- GIVEN an inactive catalog record has at least one active, inactive, historical, or nominally non-blocking relationship
- WHEN hard delete is requested
- THEN the operation returns the stable in-use category
- AND the record and relationship remain persisted and published

#### Scenario: Dependency-free inactive target is deleted

- GIVEN a catalog record is inactive, dependency-free, and addressed with its current revision
- WHEN hard delete is requested
- THEN the record is removed from persistence
- AND the subsequently published authority no longer contains it

#### Scenario: Concurrent dependency wins the race

- GIVEN preflight observes an inactive dependency-free target
- WHEN a dependency is committed before the guarded delete commits
- THEN physical deletion is rejected
- AND no deletion snapshot is published

### Requirement: Atomic applicability aggregate

An `APLICABILIDAD` record, its active state, definition relationships, and all nested attribute rules MUST form one atomic mutation aggregate for create and update. Accepted mutations MUST materialize those values losslessly in persistence and published authority. An incomplete, invalid, catalog-invalid, snapshot-invalid, or lossy aggregate MUST be rejected as a whole, with neither partial persistence nor publication.

#### Scenario: Valid applicability is materialized completely

- GIVEN a valid applicability contains active state, definition relationships, and nested rules
- WHEN its create or update succeeds
- THEN persistence contains the complete aggregate
- AND the published authority contains an equivalent complete aggregate

#### Scenario: Invalid nested rule rejects the aggregate

- GIVEN one nested applicability rule is missing required data or references an invalid catalog value
- WHEN create or update is requested
- THEN the entire operation is rejected with a stable public category
- AND neither the applicability record nor any of its nested rules is persisted or published

#### Scenario: Lossy candidate is rejected

- GIVEN candidate materialization would omit or alter active state, a definition relationship, or a nested rule
- WHEN the candidate is validated
- THEN the mutation is rejected before publication
- AND the prior persistence and authority snapshot remain unchanged

### Requirement: Persisted monotonic revision and compare-and-swap

Every mutable resource and catalog record MUST have a persisted revision that increases monotonically after each successful write affecting that record and is never derived from a hash catalog ID. Every public update, lifecycle transition, and hard-delete request MUST compare its expected revision with the current persisted revision as part of the authoritative write. A stale expected revision MUST return the stable conflict category without persistence or publication.

#### Scenario: Successful update advances revision

- GIVEN a resource or catalog record has revision `R`
- WHEN an update with expected revision `R` succeeds
- THEN the resulting persisted and public record has a revision greater than `R`

#### Scenario: Stale update conflicts

- GIVEN another write has advanced a record beyond revision `R`
- WHEN an update is requested with expected revision `R`
- THEN the operation returns conflict
- AND no field, revision, or published snapshot changes because of the stale request

#### Scenario: Stale lifecycle or delete conflicts

- GIVEN a resource or catalog record has advanced beyond the caller's expected revision
- WHEN the caller requests a supported deactivate, reactivate, or hard-delete operation with the stale revision
- THEN the operation returns conflict
- AND neither persistence nor publication changes because of that request

### Requirement: Commit and publication equivalence

For every successful catalog mutation, persistence after commit MUST be equivalent to the published `CatalogAuthority` snapshot after normalization. Publication MUST occur exactly once and only after the successful commit. Validation failures, guard failures, conflicts, and persistence failures MUST publish nothing; rejected or rolled-back operations MUST leave both persistence and the previously published snapshot unchanged.

#### Scenario: Successful mutation publishes committed state once

- GIVEN a valid catalog create, update, lifecycle transition, or hard delete
- WHEN the authoritative mutation commits successfully
- THEN exactly one publication occurs after commit
- AND the published snapshot is equivalent to an independent coherent reload of persistence

#### Scenario: Pre-commit failure has no effects

- GIVEN a catalog mutation fails validation, a guard, revision comparison, or persistence before commit
- WHEN the operation returns
- THEN no mutation from that operation remains persisted
- AND no snapshot is published

#### Scenario: Coherent reload is the equivalence oracle

- GIVEN any of the 11 kinds has completed a successful lifecycle mutation
- WHEN persistence is independently reloaded as one coherent snapshot
- THEN the normalized reload equals the currently published authority, including active states, references, option sets, definitions, and nested applicability rules

### Requirement: Authoritative writer topology and freshness limits

This version MUST guarantee exactly one authoritative writer process for Resource Master mutations. It MUST NOT claim safe independent multi-process writers, automatic cross-process refresh, notification, polling, or live coherence. A read-only process MUST receive a coherent snapshot at construction or explicit reload, but consumers MUST be informed that a separate process requires explicit reload or restart to observe later writes.

#### Scenario: Single writer updates its authority

- GIVEN the designated authoritative writer process commits a mutation
- WHEN publication completes
- THEN reads using that process's published authority observe the committed state

#### Scenario: Another process remains unchanged

- GIVEN a second process loaded a coherent catalog before the writer committed a mutation
- WHEN the writer publishes its new in-process authority
- THEN no guarantee states that the second process observes the mutation automatically
- AND an explicit reload or restart is required for a freshness guarantee

### Requirement: PostgreSQL compatibility and exit evidence

Release evidence for WRITE readiness MUST include additive revision migration and backfill verification, persistence compatibility for all 11 catalog kinds and resources, application-role access verification, deterministic isolated fixtures, stale-write and guarded-delete integration tests, authority-versus-reload equivalence tests, and race-enabled Go test results. Fixtures MUST preserve seeded business data and clean up only data they own. Migration, fixture, and integration behavior MUST preserve existing `identity-v1` values and MUST NOT substitute hash-derived catalog IDs for durable identity.

#### Scenario: Revision migration preserves existing identities

- GIVEN a PostgreSQL database containing existing resources and all supported catalog shapes
- WHEN the revision migration and backfill are applied
- THEN every mutable record has a valid initial persisted revision
- AND every existing resource retains its original `identity-v1`

#### Scenario: Integration fixtures prove complete persistence behavior

- GIVEN isolated fixtures for all 11 kinds, applicability rules, dependencies, and resources
- WHEN lifecycle, CAS, atomic rejection, and coherent-reload integration tests run
- THEN they verify successful and rejected outcomes without altering unrelated seeded business rows

#### Scenario: Race evidence is required for WRITE exit

- GIVEN a candidate release exposes one or more WRITE operations
- WHEN its readiness evidence is reviewed
- THEN race-enabled tests and PostgreSQL integration tests for those operations have passed
- AND no test obtains success by weakening `identity-v1`, dependency guards, CAS, atomicity, or publication equivalence

### Requirement: Separate read and per-operation write readiness

READ-ONLY readiness MUST be evaluated independently from WRITE readiness. The read-only contract MAY be declared ready when external-package construction and usage, all read projections, defensive copies, identity semantics, query behavior, canonical presentation, and safe read errors pass their required tests; incomplete WRITE work MUST NOT prevent or silently expand that read-only release. Each WRITE operation—create, update, deactivate, reactivate, and hard delete—MUST remain unavailable until that operation independently proves applicable lifecycle parity across all 11 kinds, atomic applicability behavior, stable safe errors, current-revision CAS, guarded-delete policy where applicable, commit-before-single-publication behavior, persistence/authority equivalence, and required PostgreSQL and race evidence.

#### Scenario: Read-only graduates before write

- GIVEN all read-only acceptance evidence passes and one or more write criteria remain incomplete
- WHEN readiness is evaluated
- THEN the read-only contract may be released
- AND no incomplete write operation is publicly available

#### Scenario: One write operation does not unlock another

- GIVEN create satisfies every WRITE readiness criterion but update or hard delete does not
- WHEN public WRITE availability is evaluated
- THEN create may be exposed only if its own prerequisites are complete
- AND update and hard delete remain unavailable

#### Scenario: Failing evidence withdraws only affected write readiness

- GIVEN an exposed write operation no longer satisfies revision, atomicity, error-safety, guard, or equivalence evidence
- WHEN readiness is reevaluated
- THEN that operation MUST be disabled or withdrawn
- AND the accepted read-only contract and independently ready operations remain available

### Requirement: Consumer-neutral public Create for catalog and resource

The system MUST provide a public `Writer` obtained through `WriteCapabilities`, exposing `CreateCatalog` for all 11 registered catalog kinds and `CreateResource`. Both methods MUST defensively copy the incoming request and the returned record. A successful create MUST return the persisted `Revision` and, for resources, the durable `identity-v1`.

#### Scenario: External consumer creates catalog and resource records

- GIVEN a Go package outside the internal import boundary holds a constructed `Writer`
- WHEN it calls `CreateCatalog` for a registered kind and `CreateResource`
- THEN both calls compile and succeed using only public types
- AND each returned record carries its persisted `Revision`, and the resource its `identity-v1`

#### Scenario: Caller mutation after the call has no effect

- GIVEN a caller holds the request passed to `CreateCatalog` or `CreateResource`
- WHEN it mutates that request or the returned record after the call
- THEN neither the accepted request nor the persisted, returned record changes

### Requirement: Write-direction field and query completeness

Every field a public write request exposes MUST reach the internal command or record at the bridge, or be omitted with a one-line rationale comment at the omission site. A public write field the internal domain cannot honor MUST be reported as a blocking `MISSING DOMAIN CRITERION`, never silently accepted and ignored.

#### Scenario: Every write field is mapped or documented

- GIVEN a public write request field
- WHEN the bridge translates the request
- THEN the field's value reaches the internal command or record unchanged, or the omission site carries a one-line rationale comment
- AND no field is accepted and silently dropped

### Requirement: Actor attribution without persistence

Every public write request MUST carry a caller-supplied `Actor` string, which the Core MUST NOT authenticate, authorize, or validate. The bridge MUST forward `Actor` to the internal diagnostic seam (`internal/core/diagnostics.go`) alongside the operation and key. `Actor` MUST NOT be persisted with the created record, introduce a new column or migration, or appear on any public DTO.

#### Scenario: Actor reaches diagnostics but never persistence

- GIVEN a write request carries a caller-supplied `Actor`
- WHEN `CreateCatalog` or `CreateResource` is called, whether it succeeds or fails
- THEN the internal diagnostic seam records `Actor` alongside the operation and key
- AND no persisted column or public DTO carries the `Actor` value

### Requirement: Create-reachable error category coverage

`CreateCatalog` and `CreateResource` MUST prove exactly 9 reachable and distinct public error categories: `INVALID_ARGUMENT`, `NOT_FOUND`, `DUPLICATE`, `INVALID_REFERENCE`, `VALIDATION`, `INTEGRITY`, `INVALID_CATALOG`, `UNAVAILABLE`, `INTERNAL`. `CONFLICT`, `IDENTITY_CONFLICT`, `REACTIVATION_IMPOSSIBLE`, `IN_USE`, and `IMMUTABLE_CODE` MUST NOT be claimed reachable from Create.

#### Scenario: Nine categories reachable, five stay unproven

- GIVEN catalog and resource create requests that individually trigger each of the 9 Create-reachable categories
- WHEN each is submitted through `CreateCatalog` or `CreateResource`
- THEN the caller receives the matching category and no two conditions collapse into the same one
- AND `CONFLICT`, `IDENTITY_CONFLICT`, `REACTIVATION_IMPOSSIBLE`, `IN_USE`, and `IMMUTABLE_CODE` are never claimed reachable from Create

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

### Requirement: Compiled surface extended to Create, Update, Deactivate, Reactivate, and HardDeleteCatalog

The compiled `WriteCapabilities` interface and `Writer` MUST declare exactly `CreateCatalog`, `CreateResource`, `UpdateCatalog`, `UpdateResource`, `DeactivateCatalog`, `ReactivateCatalog`, `DeactivateResource`, `ReactivateResource`, and `HardDeleteCatalog` — 9 methods — in and after this change. `HardDeleteResource` MUST NOT be exported, stubbed, or otherwise discoverable; it remains a deliberate, documented gap, not addressed by this change. `cmd/garfex/` and `internal/tui/` MUST have zero changed lines and zero new `resourcecore` references from this change.

#### Scenario: Create, Update, Deactivate, Reactivate, and HardDeleteCatalog are discoverable; HardDeleteResource and drivers stay untouched

- GIVEN the compiled `resourcecore` package and the pre-change state of `cmd/garfex/` and `internal/tui/`
- WHEN this change is applied
- THEN exactly `CreateCatalog`, `CreateResource`, `UpdateCatalog`, `UpdateResource`, `DeactivateCatalog`, `ReactivateCatalog`, `DeactivateResource`, `ReactivateResource`, and `HardDeleteCatalog` exist on `Writer`/`WriteCapabilities`, with no `HardDeleteResource` stub
- AND `cmd/garfex/` and `internal/tui/` have zero changed lines and zero new `resourcecore` references
