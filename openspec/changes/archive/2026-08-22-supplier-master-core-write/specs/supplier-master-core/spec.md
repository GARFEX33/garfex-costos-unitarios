# Delta for Supplier Master Core

## ADDED Requirements

### Requirement: Public write contract — Supplier Create only
The system SHALL expose a public, `internal`-free `Writer` over a `WriteCapabilities` port in `suppliercore`, declaring exactly one method, `CreateSupplier`. `NewWriter(nil)` SHALL return `INVALID_ARGUMENT`, never a nil-pointer panic.

#### Scenario: External package creates a supplier
- GIVEN a Go test outside this module's `internal/` tree
- WHEN it imports `suppliercore`, builds a `Writer` via `NewWriter`, and calls `CreateSupplier`
- THEN it compiles, runs, and returns the persisted `Supplier` with no `internal` import

#### Scenario: Nil capability rejected at construction
- GIVEN a nil `WriteCapabilities`
- WHEN `NewWriter(nil)` is called
- THEN it returns a nil `*Writer` and `Code() == INVALID_ARGUMENT`

### Requirement: Shape validation is Actor-only; content rules stay in the domain
`SupplierWriteRequest` validation SHALL check only that `Actor` is non-blank before delegating. The five `domain.SupplierDetails` fields are optional at the shape layer; their combined-content rule (at least one of trade name, legal name, tax identifier) SHALL remain `domain.NewSupplier`'s sole authority. The request SHALL NOT carry `Active`; created suppliers are always active.

#### Scenario: Blank Actor rejected before delegation
- GIVEN `Actor == ""` or all-whitespace
- WHEN `CreateSupplier` is called
- THEN it returns `INVALID_ARGUMENT` without invoking the underlying capability

#### Scenario: Empty content reaches the domain, not a boundary rejection
- GIVEN a non-blank `Actor` and no trade name, legal name, or tax identifier
- WHEN `CreateSupplier` is called
- THEN it delegates, and the domain's error surfaces as `Code() == VALIDATION`, not `INVALID_ARGUMENT`

### Requirement: Create-reachable error taxonomy, no new codes
The existing five-category `Error`/`ErrorCode` SHALL be reused. `CreateSupplier` SHALL make exactly four reachable — `INVALID_ARGUMENT`, `VALIDATION`, `CONFLICT` (duplicate tax identifier, plain conflict, no upsert), `INTERNAL` — and `NOT_FOUND` SHALL be unreachable from Create.

#### Scenario: Each Create-reachable category is proven
- GIVEN the four applicable categories
- WHEN a scenario is built per category (blank Actor, empty content, duplicate tax identifier, unclassified failure)
- THEN each produces the matching `Code()`, and no test proves `NOT_FOUND` from Create

### Requirement: No CAS; Actor is diagnostic-only and a documented future audit seed
No `Revision`/CAS field SHALL be added to `SupplierWriteRequest` or any entity; the write surface SHALL inherit the internal service's existing lost-update race, disclosed in `doc.go`. `Actor` SHALL be required, passed via `internal/core.WithActor(ctx, actor)`, and SHALL NOT be persisted or returned on any DTO in this slice; `doc.go` SHALL document it as a foreseeable audit-data seed for a future change, not authorization.

#### Scenario: No Revision field and no Actor leak
- GIVEN `SupplierWriteRequest` and a successful `CreateSupplier` call
- WHEN the request type and the returned `Supplier` are inspected
- THEN no `Revision`/`ExpectedRevision` field exists, and the returned `Supplier` carries no `Actor` field

### Requirement: Compiled write surface exports no ungraduated method
`WriteCapabilities` SHALL declare exactly one method, `CreateSupplier`; `Writer`'s exported method set SHALL contain no other method, per a reflection guard mirroring `resourcecore/writer_test.go:216`.

#### Scenario: Reflection guard fails on any ungraduated method
- GIVEN the compiled `WriteCapabilities` and `Writer` types
- WHEN their method sets are enumerated via `reflect`
- THEN `WriteCapabilities.NumMethod() == 1` (`CreateSupplier`), and every exported `Writer` method is in `{CreateSupplier}` — no Update/Deactivate/Reactivate/HardDelete stub

### Requirement: No error leakage; defensive copying on the write path
`CreateSupplier` SHALL NOT leak pgx/SQLSTATE/constraint/table/column detail through any public error, and SHALL defensively copy the inbound request and outbound `Supplier` so post-call mutation never affects a later read.

#### Scenario: Raw PostgreSQL error never reaches the public surface
- GIVEN an internal create fails with a raw, unwrapped `PgError`-shaped error
- WHEN it crosses the bridge into `suppliercore`
- THEN the public `Error` has `Code() == INTERNAL` with no SQLSTATE/constraint/table/column text

#### Scenario: Mutating the request after the call does not leak
- GIVEN a `SupplierWriteRequest` passed to `CreateSupplier`
- WHEN the caller mutates the request after the call returns
- THEN a subsequent `GetSupplier` for the created record reflects the original request values

## MODIFIED Requirements

### Requirement: No Actor or Revision on the read surface; HardDelete stays permanently absent everywhere
The system SHALL NOT expose an `Actor` field or a `Revision`/CAS field anywhere in `suppliercore`'s read-only surface (Supplier, Branch, Contact query/page/record types); this stays explicitly out of scope for reads and does not extend to the write surface, where `SupplierWriteRequest` legitimately carries `Actor` under its own requirement. No delete or hard-delete operation exists anywhere in `suppliercore`, read or write, because the internal module has no such capability for Supplier, Branch, or Contact — an absence, not deferred work.
(Previously: "No Actor, Revision, or HardDelete on the read surface" — narrowed to scope the Actor/Revision prohibition to reads only; HardDelete's absence is restated as package-wide.)

#### Scenario: No Actor or Revision field exists on any read-surface type
- GIVEN `Supplier`, `Branch`, `Contact`, and every read query/page type
- WHEN their fields are inspected
- THEN none carries an `Actor` or `Revision` field

#### Scenario: No delete or hard-delete method is exported anywhere in the package
- GIVEN the compiled `suppliercore` package, including both read and write capabilities
- WHEN its exported symbols are inspected
- THEN no `Delete`, `HardDelete`, or equivalent method exists, mirroring the internal module
