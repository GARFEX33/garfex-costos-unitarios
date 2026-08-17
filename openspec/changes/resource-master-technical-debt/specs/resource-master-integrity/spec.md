# Resource Master Integrity Specification

## Purpose

Define authoritative resource invariants, active catalog eligibility, scoped attribute persistence, and transactional write guarantees without changing the dependency direction.

## Requirements

### Requirement: Application-authoritative canonical resources

The system MUST preserve `TUI -> application -> domain repository port -> PostgreSQL`. The application/domain boundary MUST validate or reconstruct every create and update candidate from the active catalog before invoking the repository. Callers MUST NOT establish validity by supplying a forged identity or bypassing the domain port.

#### Scenario: Valid candidate reaches persistence

- GIVEN a candidate whose scope, values, relations, and unit are valid
- WHEN the application processes a create or update
- THEN it derives canonical state and calls only the domain repository port
- AND PostgreSQL persists the canonical candidate.

#### Scenario: Forged or invalid candidate is rejected

- GIVEN a caller supplies an invalid value or a caller-controlled `IdentityKey`
- WHEN the application processes the candidate
- THEN it returns a validation error before the repository is called
- AND no PostgreSQL write occurs.

### Requirement: Derived canonical identity

The system MUST derive identity from canonical class, family, type, and identity-participating technical attributes. Natural unit, display data, and non-identity attributes MUST NOT alter identity unless explicitly classified as identity-participating. Updates MUST preserve the stored resource identifier while re-deriving identity.

#### Scenario: Equivalent input produces one identity

- GIVEN equivalent values with different accepted presentation forms or order
- WHEN the candidate is canonicalized
- THEN the same canonical identity key is derived deterministically.

#### Scenario: Non-canonical identity is not trusted

- GIVEN a supplied key differs from the key derived from canonical values
- WHEN create or update is requested
- THEN the supplied key is ignored or rejected
- AND persistence receives only the derived key.

### Requirement: Scoped exact attribute persistence

The repository MUST resolve each expected attribute using class, family, type, and attribute identity. Each expected attribute write MUST affect exactly one row. A zero-row, multi-row, missing-definition, or ambiguous match MUST fail the transaction.

#### Scenario: Scoped attributes persist exactly once

- GIVEN family-wide and type-specific definitions resolve uniquely for the resource scope
- WHEN a resource with its canonical attributes is saved
- THEN each expected attribute is written once
- AND no attribute from another class, family, or type is selected.

#### Scenario: Cardinality failure rolls back

- GIVEN any expected attribute resolves to zero or multiple write targets
- WHEN the repository saves the resource
- THEN it returns an integrity error
- AND the resource row and all attribute changes are rolled back.

### Requirement: Active catalog eligibility with historical read compatibility

Catalog activity MUST be evaluated consistently across parent and child records. New creates, updates, and reactivations MUST require currently eligible active catalog data, while historical resources MUST remain readable when their stored references are inactive; historical readability MUST NOT grant write eligibility.

#### Scenario: Inactive catalog data blocks new writes but permits history

- GIVEN a stored resource references a catalog element that is now inactive
- WHEN the resource is read and when an equivalent new write is attempted
- THEN the historical resource remains displayable
- AND the new write is rejected as ineligible.
