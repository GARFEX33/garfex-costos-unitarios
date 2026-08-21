# Proposal: Expose public WRITE Update on the Resource Master Core

## Intent

Second per-operation WRITE graduation on `resourcecore`, after `Create`
(shipped, archived `2026-08-21-resource-master-core-write`). `Update` is
Create's CAS-bearing counterpart: external consumers (PI) need to modify an
existing catalog record or resource under optimistic concurrency and get a
stable `CONFLICT` on a stale write instead of a leaked `INTERNAL` error.
Internal authority already exists and is production-hardened
(`catalogo.Service.UpdateRevision`, `recursos.Service.UpdateRevision`); this
change is the public mirror, its sole bridge translation, and a two-line
postgres fix that makes catalog `CONFLICT` actually reachable.

## Scope

### In Scope
- `resourcecore.Writer.UpdateCatalog`/`UpdateResource` + `WriteCapabilities`
  gains exactly these two methods (Create untouched).
- Request DTOs carrying `ID`/`Kind` + `expectedRevision` (CAS) + `Actor`,
  mirroring `UpdateRevision`/`UpdateCommand` exactly.
- Bridge translation in `internal/bridge/resourcecore.Adapter` (sole
  bridge), reusing Create's `Actor` context-metadata wiring as-is.
- CONFLICT-wiring fix in `internal/postgres/catalog_admin_repository_v2.go`:
  reassign `errApplicabilityStaleRevision` (line 20) and
  `errCatalogStaleRevisionV2` (line 196) to `domain.ErrRevisionConflict`.
  In scope because `CONFLICT` is Update's headline new capability (Create
  has no revision to conflict on), the sentinel comments already say this
  wiring was expected to exist, and coverage confirms zero existing-test
  breakage.
- Readiness evidence: catalog Update's 11 reachable categories (incl. the
  newly-fixed `CONFLICT`, `NOT_FOUND`, `IMMUTABLE_CODE`), resource Update's 9.

### Out of Scope
- `Deactivate`/`Reactivate`/`HardDelete` graduation (separate later changes).
- Any `internal/tui`/`cmd/garfex` change.
- Any read-contract change, including a by-ID resource lookup.
- New `ErrorCode` values (all 15 already exist).

## Capabilities

### New Capabilities
None.

### Modified Capabilities
- `resource-master-core`: extend "Separate read and per-operation write
  readiness" with Update graduation for catalog and resource, the CAS
  (`expectedRevision`) request shape, and Update's error-category surface —
  including `CONFLICT` and `IMMUTABLE_CODE` newly proven, and catalog
  `NOT_FOUND` reversing from Create's "unreachable" finding.

## Approach

Mirror Create's pattern exactly: package-owned DTOs reusing existing public
value types, defensive copy on entry, `Writer` does shape validation only
(no business rules), sole-bridge translation, `Actor` via
`core.WithActor`/`ActorFrom` — never persisted. Resource-side `CONFLICT`
already works (`resource_repository_crud.go` returns
`domain.ErrResourceRevisionConflict` directly); only catalog needs the
sentinel fix.

## Affected Areas

| Area | Impact | Description |
|------|--------|--------------|
| `resourcecore/write_types.go` | Modified | Update request DTOs (ID/Kind + expectedRevision + Actor) |
| `resourcecore/writer.go` | Modified | `UpdateCatalog`/`UpdateResource`, `WriteCapabilities` +2 methods |
| `resourcecore/copy.go` | Modified | Defensive copies for update requests |
| `internal/bridge/resourcecore/adapter.go` | Modified | Delegating methods, field-completeness checks |
| `internal/postgres/catalog_admin_repository_v2.go` | Modified | CONFLICT sentinel reassignment (2 lines) |
| `cmd/garfex/`, `internal/tui/` | Unchanged | Zero coupling preserved |
| Tests | New/Modified | `writer_test.go`, `adapter_test.go`, CONFLICT coverage in postgres tests |

## Risks

| Risk | Likelihood | Mitigation |
|------|------------|------------|
| Sentinel reassignment changes observable error identity elsewhere | Low | Verified only postgres-internal `errors.Is` checks reference the unexported sentinels; no cross-package coupling |
| Resource Update's ID-only identity surprises consumers without a prior read | Med | Documented as existing internal pattern; by-ID read is an explicit non-goal here |
| Slice exceeds 400-line review budget | High | 3-unit auto-chain per explore.md estimate (~900-1100 lines) |
| Unproven error categories reached in production | Low | Restrict graduation to the proven-reachable categories only |

## Rollback Plan

Revert `UpdateCatalog`/`UpdateResource` and the sentinel reassignment
independently; Create and Read stay unaffected. The sentinel revert only
restores prior `INTERNAL` misclassification — no persisted data or migration
is touched by either the fix or its rollback.

## Dependencies

- `resource-master-core-write` (Create), merged to main, archived
  `2026-08-21-resource-master-core-write` (issue #148, PRs #149-151).

## Success Criteria

- [ ] External consumer updates a catalog record and a resource via
      `resourcecore` with `expectedRevision` CAS, no `internal` import.
- [ ] `WriteCapabilities` compiles Create + Update only.
- [ ] A stale catalog revision returns `CONFLICT`, not `INTERNAL` (tested).
- [ ] Every public Update field maps to its internal destination or carries
      a one-line omission rationale.
- [ ] Catalog Update proves 11 categories, resource Update proves 9.
- [ ] No PostgreSQL detail leaks through any error string or type.
- [ ] `cmd/garfex/`, `internal/tui/` have zero changed lines.
