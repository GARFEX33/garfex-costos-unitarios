# Supplier Master Core Specification

## Purpose

Define a consumer-neutral, read-first Supplier Master contract (Supplier, Branch, Contact) and the correctness conditions each read operation must satisfy — the same role resource-master-core plays for the Resource Master. Write operations are a distinct, later, per-operation-gated series.

## Requirements

### Requirement: Public read-only contract package
The system SHALL expose a public, `internal`-free Go package (`suppliercore`, repo root) providing read access to the Supplier Master (Supplier, Branch, Contact), so any external consumer can compile against it without importing `internal/modules/suppliers`.

#### Scenario: External package reads all three entities
- **GIVEN** a Go test file in a package outside this module's `internal/` tree
- **WHEN** it imports `suppliercore`, constructs a `Reader` via `NewReadOnly`, and calls `GetSupplier`, `ListBranches`, and `ListContacts`
- **THEN** it compiles and runs without importing any `internal/...` package

#### Scenario: No write method is reachable
- **GIVEN** the compiled `suppliercore.Reader` and `suppliercore.ReadCapabilities` types
- **WHEN** their exported method sets are inspected via reflection
- **THEN** the only methods present are `GetSupplier`, `SearchSuppliers`, `ListBranches`, `GetBranch`, `ListContacts`, `GetContact` — no Create, Update, Deactivate, Reactivate, or Delete method exists on either type

### Requirement: Constructor rejects a nil capability
The system SHALL reject construction of a `Reader` backed by a nil `ReadCapabilities` with `INVALID_ARGUMENT`, never a nil-pointer panic on first use.

#### Scenario: Nil capability rejected at construction
- **GIVEN** a nil `ReadCapabilities` value
- **WHEN** `NewReadOnly(nil)` is called
- **THEN** it returns a nil `*Reader` and an error whose `Code()` is `INVALID_ARGUMENT`

### Requirement: Request-shape validation precedes delegation
The system SHALL validate the shape of every read request (positive IDs, a known `LifecycleScope` value) before delegating to the underlying capability, rejecting shape violations with `INVALID_ARGUMENT` without ever reaching the internal service.

#### Scenario: Non-positive supplier ID rejected
- **GIVEN** a `Reader` backed by a capability that would panic or error loudly if called
- **WHEN** `GetSupplier(ctx, 0)` or `GetSupplier(ctx, -1)` is called
- **THEN** it returns `INVALID_ARGUMENT` and the underlying capability is never invoked

#### Scenario: Non-positive branch or contact ID rejected
- **GIVEN** a `BranchKey{SupplierID: 1, BranchID: 0}` or `ContactKey{SupplierID: 1, ContactID: -5}`
- **WHEN** `GetBranch` or `GetContact` is called with that key
- **THEN** it returns `INVALID_ARGUMENT` without delegating

#### Scenario: Unknown lifecycle scope rejected
- **GIVEN** a `SupplierQuery{Scope: "BOGUS"}`
- **WHEN** `SearchSuppliers` is called
- **THEN** it returns `INVALID_ARGUMENT` without delegating

#### Scenario: Non-positive BranchID filter on a contact query rejected
- **GIVEN** a `ContactQuery{SupplierID: 1, BranchID: &zero}` where `zero == 0`
- **WHEN** `ListContacts` is called
- **THEN** it returns `INVALID_ARGUMENT` without delegating; a nil `BranchID` (no filter) and a positive `BranchID` both pass shape validation

### Requirement: Parent-scoped Branch and Contact reads
The system SHALL require every Branch and Contact request to carry its owning `SupplierID` explicitly, since both are Supplier-scoped children with no independent identity.

#### Scenario: Branch key always carries SupplierID
- **GIVEN** the `BranchKey` and `BranchQuery` types
- **WHEN** their fields are inspected
- **THEN** both carry a `SupplierID int64` field, and `GetBranch`/`ListBranches` cannot be called without one

#### Scenario: Contact key always carries SupplierID, optionally BranchID
- **GIVEN** the `ContactKey` and `ContactQuery` types
- **WHEN** their fields are inspected
- **THEN** both carry `SupplierID int64`; `ContactQuery` additionally carries an optional `BranchID *int64` filter, and `Contact.BranchID *int64` is present on the returned record

### Requirement: Cross-entity ownership is enforced, not re-implemented
The system SHALL surface a `VALIDATION` error when a contact read is scoped to a branch that does not belong to the given supplier, by delegating the check to the internal service — the public package and bridge SHALL NOT re-implement this rule themselves.

#### Scenario: Listing contacts under a foreign branch fails validation
- **GIVEN** supplier A and a branch that belongs to supplier B
- **WHEN** `ListContacts` is called with `ContactQuery{SupplierID: A, BranchID: &brancheOfB}`
- **THEN** it returns an error whose `Code()` is `VALIDATION`, produced by the internal service's ownership check surfacing through the bridge — not by any ownership logic inside `suppliercore` or its bridge

### Requirement: Unknown-supplier reads are honestly asymmetric
The system SHALL preserve the existing internal asymmetry rather than normalize it: listing a supplier's children confirms the supplier exists, while getting one child by ID does not.

#### Scenario: Listing branches or contacts under an unknown supplier returns NOT_FOUND
- **GIVEN** a `SupplierID` that does not exist
- **WHEN** `ListBranches` or `ListContacts` is called with that `SupplierID`
- **THEN** it returns an error whose `Code()` is `NOT_FOUND`

#### Scenario: Getting one branch or contact by ID does not confirm the supplier
- **GIVEN** a `BranchKey`/`ContactKey` with a shape-valid but nonexistent `SupplierID`, and a `BranchID`/`ContactID` that also does not exist
- **WHEN** `GetBranch` or `GetContact` is called
- **THEN** it returns `NOT_FOUND` for the branch/contact lookup itself (ID-shape validation only, no supplier-existence pre-check) — this asymmetry is preserved as-is, not normalized

### Requirement: Defensive copying across the boundary
The system SHALL return only defensively copied values from every read method, so mutating a returned record, slice, or the `Contact.BranchID` pointer never affects a subsequent read.

#### Scenario: Mutating a returned Contact's BranchID does not leak
- **GIVEN** a `Contact` returned by `GetContact` with a non-nil `BranchID`
- **WHEN** the caller mutates the pointed-to value directly (`*contact.BranchID = 999`)
- **THEN** a subsequent `GetContact` call for the same contact returns the original, unmutated `BranchID` value

#### Scenario: Mutating a returned page slice does not leak
- **GIVEN** a `SupplierPage`, `BranchPage`, or `ContactPage` returned by a list/search method
- **WHEN** the caller appends to or mutates an element of the returned slice
- **THEN** a subsequent call for the same query returns an unaffected result

### Requirement: Page results never exceed the requested limit
The system SHALL cap every page's record count at the effective `Limit`, and correctly report `HasNext`, even though the internal Branch/Contact repository fetches one extra untrimmed row internally while the internal Supplier repository does not.

#### Scenario: Supplier search page respects Limit and reports HasNext
- **GIVEN** more matching suppliers exist than the requested `Limit`
- **WHEN** `SearchSuppliers` is called with that `Limit`
- **THEN** the returned `SupplierPage.Suppliers` has exactly `Limit` entries and `HasNext` is `true`

#### Scenario: Branch and contact list pages respect Limit and report HasNext
- **GIVEN** more branches (or contacts) exist under a supplier than the requested `Limit`
- **WHEN** `ListBranches` (or `ListContacts`) is called with that `Limit`
- **THEN** the returned page has exactly `Limit` entries and `HasNext` is `true`, identical in shape to the Supplier case despite the different internal fetch strategy

#### Scenario: A zero or negative Limit falls back to the documented default
- **GIVEN** a query with `Limit <= 0`
- **WHEN** any list/search method is called
- **THEN** the effective page size is the package's documented default (matching the internal repository's existing default), never a one-row page

### Requirement: Lifecycle scope is honestly reachable for every entity
The system SHALL support `ACTIVE`, `INACTIVE`, and `ALL` lifecycle scope filtering identically across Supplier, Branch, and Contact reads.

#### Scenario: Each scope value returns the matching population
- **GIVEN** a supplier with at least one active and one inactive branch
- **WHEN** `ListBranches` is called once per `LifecycleScope` value (`ACTIVE`, `INACTIVE`, `ALL`, and the empty/default value)
- **THEN** each call returns exactly the branches matching that scope (empty scope behaves as `ACTIVE`, matching resourcecore's own convention)

### Requirement: Small, purpose-built error taxonomy with no driver leakage
The system SHALL expose exactly five error categories (`NOT_FOUND`, `VALIDATION`, `CONFLICT`, `INVALID_ARGUMENT`, `INTERNAL`), each reachable, and SHALL NOT leak PostgreSQL/pgx detail (SQLSTATE, constraint names, table/column names, driver error types) through any public error.

#### Scenario: Every declared error category is reachable
- **GIVEN** the five declared `ErrorCode` values
- **WHEN** a read scenario is constructed for each — unknown record (`NOT_FOUND`), foreign-branch contact filter (`VALIDATION`), a capability returning a conflict-classified internal error (`CONFLICT`), malformed request shape (`INVALID_ARGUMENT`), and an unclassified internal failure (`INTERNAL`)
- **THEN** each scenario produces a public `Error` whose `Code()` matches the expected category

#### Scenario: A raw PostgreSQL-shaped error never reaches the public surface
- **GIVEN** an internal read fails with a raw, unwrapped pgx/`PgError`-shaped error (reproducing the verified gap where the internal `wrapRead` helper does not sanitize, unlike the write-path's `mapWriteError`)
- **WHEN** that error crosses the bridge into `suppliercore`
- **THEN** the resulting public `Error` has code `INTERNAL` and its message contains no SQLSTATE code, constraint name, table name, or column name

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
