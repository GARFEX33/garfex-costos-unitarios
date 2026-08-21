# Delta for resource-master-core

## ADDED Requirements

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

### Requirement: Compiled surface limited to graduated Create

Consistent with the existing "Separate read and per-operation write readiness" requirement, the compiled `WriteCapabilities` interface and `Writer` MUST declare only `CreateCatalog` and `CreateResource` in this change. No method for `Update`, `Deactivate`, `Reactivate`, or `HardDelete` MUST be exported, stubbed, or otherwise discoverable. `cmd/garfex/` and `internal/tui/` MUST have zero changed lines and zero new `resourcecore` references from this change.

#### Scenario: Only Create is discoverable and drivers stay untouched

- GIVEN the compiled `resourcecore` package and the pre-change state of `cmd/garfex/` and `internal/tui/`
- WHEN this change is applied
- THEN only `CreateCatalog` and `CreateResource` exist on `Writer`/`WriteCapabilities`, with no ungraduated stub
- AND `cmd/garfex/` and `internal/tui/` have zero changed lines and zero `resourcecore` references
