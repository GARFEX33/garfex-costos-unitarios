# Supplier Master Core — Read Contract

## ADDED Requirements

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

### Requirement: No Actor, Revision, or HardDelete on the read surface
The system SHALL NOT expose an `Actor` field, a `Revision`/CAS field, or any delete operation anywhere in `suppliercore`'s read-only surface — these remain explicitly out of scope for this change.

#### Scenario: No Actor or Revision field exists on any public type
- **GIVEN** the `Supplier`, `Branch`, `Contact`, and every query/page type in `suppliercore`
- **WHEN** their fields are inspected
- **THEN** none carries an `Actor` or `Revision` field

#### Scenario: No delete or hard-delete method is exported
- **GIVEN** the compiled `suppliercore` package
- **WHEN** its exported symbols are inspected
- **THEN** no `Delete`, `HardDelete`, or equivalent method exists — mirroring the internal module, which has no such capability for Supplier, Branch, or Contact
