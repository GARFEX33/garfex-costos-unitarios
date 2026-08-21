# Resource Master Source of Truth

This guide records the decisions reviewers and operators must use when tracing Resource Master writes, reads, lifecycle, and search. Supplier Master is a separate bounded context.

## Decisions first

| Concern | Authoritative decision | Concrete boundary |
|---|---|---|
| Construction | The application owns canonical Resource construction and validation. The consumer submits intent; it does not establish validity. | `internal/app/recursos/service.go`, `internal/domain/resource_validation.go` |
| Identity | A v1 key is derived from class, family, type, and identity-participating canonical attributes. Natural unit, display data, and non-identity attributes do not change it. | `internal/domain/resource_validation.go`, `internal/domain/resource_types.go` |
| Persistence | PostgreSQL is the source of truth for Resource instances and attribute values. The repository is reached through the domain port. | `internal/domain/resource_types.go`, `internal/postgres/resource_repository_crud.go` |
| Lifecycle | Deactivation is non-destructive. `Deactivate` and `Reactivate` are explicit transitions; historical rows remain readable. | `internal/app/recursos/service.go`, `internal/postgres/resource_repository_crud.go` |
| Search | Active resources are the default. Inactive discovery is explicit. Search pages are bounded, stable, filter-preserving, and set-hydrated. | `internal/domain/resource_types.go`, `internal/postgres/resource_repository_search.go` |
| Supplier | Supplier data and behavior are not Resource Master authority. Do not infer a shared model or workflow. | `internal/modules/suppliers/` |

## Authority boundaries

```text
Consumer intent
  -> internal/app/recursos.Service
  -> internal/domain.NewResource / ResourceRepository port
  -> PostgreSQL recursos + resource_attribute_values
```

- `Service.Create` and `Service.Update` load the current `CatalogAuthority`, call `domain.NewResource`, and only then invoke the repository. The caller-provided update ID is the stable storage identity; the v1 `IdentityKey` is re-derived.
- `domain.HydrateResource` accepts repository snapshots only when the stored identity is v1 and the required resource fields are present. `SeedResourceCatalog` is fixture/seed data, not a replacement for persisted Resource rows.
- Any consumer reaches Resource Master only through application contracts for create, update, get, lifecycle, and `SearchPage` — never by importing `internal/domain` or `internal/postgres` directly.

## Write and read flow

| Flow | Steps | Failure boundary |
|---|---|---|
| Create/update | Consumer intent → application catalog validation → canonical domain Resource → repository transaction → PostgreSQL | Invalid or forged input fails before persistence; scoped target/cardinality failures roll back the transaction. |
| Get | Class + v1 identity → PostgreSQL base row → shared effective-value loader → domain hydration | Inactive catalog references do not hide a historical Resource; effective-attribute ambiguity or effective-value metadata/decode/required-value integrity failures return `ErrResourceIntegrity`. Malformed base snapshots may return domain validation errors. |
| SearchPage | Normalized criteria → bounded base/filter query → one set-based effective-value query → canonical page | No partial page is returned when an effective winner is ambiguous or effective-value metadata, decode, or required-value integrity is inconsistent. Malformed base snapshots may return domain validation errors. |

## Identity and effective values

### v1 identity

`NewResource` canonicalizes the class/family/type scope and accepted values, sorts identity attributes by code, and prefixes the result with `v1|`. Only attributes marked `IdentityParticipates` contribute. On update, the application preserves the database ID while deriving the new key from the submitted canonical intent.

### Type over family

For each `(resource_id, definition code)`, PostgreSQL computes one effective value:

| Candidate | Scope rank | Result |
|---|---:|---|
| Family-wide attribute (`type_id IS NULL`) | 0 | Fallback when no type-specific candidate exists. |
| Type-specific attribute | 1 | Overrides the family-wide value and hides it from Get, Search, filters, and rule hydration. |

The shared CTE and loader live in `internal/postgres/resource_repository_attributes.go` and are used by both Get and Search. An ambiguous effective winner, effective-value metadata mismatch, decode failure, or missing required value is an integrity failure—not a reason to silently select one row.

## Lifecycle and pagination

### Lifecycle checklist

- [ ] Default search uses `LifecycleScopeActive`.
- [ ] Inactive discovery explicitly uses `LifecycleScopeInactive`.
- [ ] `Deactivate` changes `recursos.active` without deleting the row, identity, or attributes.
- [ ] `Reactivate` rechecks current class/family/type/unit/policy eligibility and the expected identity.
- [ ] A failed reactivation leaves the Resource inactive.

### Page guarantees

- `SearchCriteria.Normalize` supplies a default size of 50 when `Limit` is zero, rejects `Limit > 50` with `ErrInvalidSearch` rather than capping it, rejects negative bounds, and validates lifecycle scope.
- PostgreSQL fetches `limit + 1` rows and reports `HasNext`; `HasPrevious` is true only when the normalized offset is positive.
- Ordering is `identity_key ASC, id ASC`, so equal display values have a stable tie-breaker.
- `ResourcePage.Criteria` retains text, class, family, attribute filters, lifecycle scope, limit, and offset. Paging forward/backward changes only the offset and retains that context.
- A boundary request is a no-op. A navigation failure retains the current page and selection context.

## Integrity failures

| Symptom | Expected result | Where to inspect |
|---|---|---|
| Invalid/forged write | Validation error; repository is not called. | `internal/app/recursos/service.go`, domain tests |
| Zero or multiple `(class, family, type, definition)` targets | `ErrResourceIntegrity`; transaction rolls back. | `internal/postgres/resource_repository_crud.go` |
| Ambiguous effective winner, effective-value metadata/decode/required-value failure | `ErrResourceIntegrity`; Get/SearchPage returns no partial Resource/page. Malformed base snapshots may instead return a domain validation error. | `internal/postgres/resource_repository_attributes.go`, `resource_repository_search.go` |

## Operator path: verify, troubleshoot, rollback

1. Confirm the branch and clean scope: `git status --short --branch` and `git diff --stat`.
2. Run static checks: `gofmt -l .`, `go vet ./...`, and `golangci-lint run ./...`.
3. Run focused tests, then the full suite: `go test ./internal/domain ./internal/app/recursos ./internal/postgres -count=1` and `go test ./... -count=1`.
4. For PostgreSQL evidence, treat `sh scripts/db/integration_test.sh` only as the isolated harness/migration-lifecycle check: its database has no host port, so the script neither exposes a host DSN nor runs Resource tests. Resource-focused evidence comes from the recorded isolated CI run (migrations 000001–000007, with separate app/admin DSNs), or an equivalently isolated database; with `GARFEX_TEST_DSN` and `GARFEX_ADMIN_TEST_DSN` pointed there, run `go test ./internal/postgres -run 'TestResourceRepository(LifecycleIntegration|AttributeCardinalityIntegration|SearchSetHydrationIntegration|Integration)$' -count=1`. Never point these tests at the normal `garfex_pgdata` volume.

Rollback for this documentation slice is limited to the changed comments and this guide. Reverting it does not alter Resource behavior, schema, migrations, PostgreSQL data, Supplier code, `.atl`, or pagination/lifecycle implementation. For a runtime integrity incident, preserve the failing evidence, stop writes if necessary, and roll back the owning code slice—not by deleting catalog or Resource rows.

## Public read-only contract (`resourcecore`)

A consumer-neutral, importable public Go package, `resourcecore`, exposes a **read-only** view of this authority: active classes, catalog descriptors, catalog list/get, resource search/get, and canonical presentation. It is a translation boundary, not a second authority — every read still goes through `internal/app/catalogo.Service` and `internal/app/recursos.Service` above via a module-owned bridge (`internal/bridge/resourcecore.Adapter`), and it changes nothing about the write/read decisions in this document. There is no public `Create`, `Update`, `Deactivate`, `Reactivate`, `Delete`, or `Publish`; public WRITE is a distinct, later, per-operation change. Errors are one of fifteen stable `resourcecore.ErrorCode` categories with no PostgreSQL or driver detail exposed. Exactly one process is the authoritative writer; a `resourcecore.Reader` reflects a coherent snapshot as of its last read and requires an explicit reload/restart to observe a write made by another process. See [Resource Master Core — Public Read Contract](resource-master-core.md) for the complete ownership, error-reachability, identity, freshness, and migration-8-compatibility reference.

## Supplier separation

Supplier Master remains under `internal/modules/suppliers/` with its own domain, application, and PostgreSQL packages. This guide intentionally does not define Supplier identity, lifecycle, catalog authority, search, pagination, or persistence semantics. Future master catalogs must establish their own authority boundary rather than extending Resource comments or contracts by implication.
