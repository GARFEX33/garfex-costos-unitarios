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
