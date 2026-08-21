# Explore: resource-master-core-write-update

## Goal

Second per-operation WRITE graduation on `resourcecore` (after Create):
expose `Update` for both catalog and resource records through the public
`Writer`/`WriteCapabilities` contract, following the exact same pattern
established by `resource-master-core-write` (archived
`2026-08-21-resource-master-core-write`).

## Baseline: shape carries over unchanged

The archived `resource-master-core-write` design.md's "complete eventual
shape" table for Update needs zero reshaping. Verified field-by-field
against current source:

- `internal/app/catalogo/service.go:525` —
  `func (s *Service) UpdateRevision(ctx context.Context, kind domain.CatalogKindCode, rec domain.CatalogRecord, expectedRevision uint64) (domain.CatalogRecord, error)`
- `internal/app/recursos/service.go:300` —
  `func (s *Service) UpdateRevision(ctx context.Context, command domain.UpdateCommand, expectedRevision uint64) (domain.Resource, error)`
- `internal/domain/resource_types.go:204` —
  `UpdateCommand` = `CreateCommand` shape + `ID int64` (confirmed:
  `ID`, `Scope`, `NaturalUnit`, `Attributes`).

Both `UpdateRevision` methods take an explicit `expectedRevision uint64`
(optimistic-concurrency / CAS parameter) — this is new surface area versus
Create, and is the direct cause of the CONFLICT finding below.

## New finding: catalog CONFLICT is currently dead code (verified)

Catalog CAS stale-revision conflicts misclassify as `INTERNAL` instead of
`CONFLICT`. Root cause, confirmed directly in source:

- `internal/postgres/catalog_admin_repository_v2.go:20` —
  `errApplicabilityStaleRevision = errors.New("applicability revision conflict")`,
  comment: "stand-in for future domain.ErrRevisionConflict".
- `internal/postgres/catalog_admin_repository_v2.go:196` —
  `errCatalogStaleRevisionV2 = errors.New("catalog revision conflict")`,
  comment: "stand-in for ... errApplicabilityStaleRevision" (same family).
- `internal/domain/core_errors.go:58` — `ErrRevisionConflict = ErrResourceRevisionConflict`
  (an alias, added at stage 5) already exists — but neither postgres
  sentinel was ever rewired to it. Both remain private
  `errors.New(...)` values with no `errors.Is` relationship to any domain
  sentinel, so `core.Map`/the bridge's `mapError` cannot classify them as
  CONFLICT — they fall through to INTERNAL.

Contrast with resources, which already work correctly:
`internal/postgres/resource_repository_crud.go:396,467,496` return
`domain.ErrResourceRevisionConflict` directly on stale CAS — no gap there.

**This is in-scope, not a tangent**: CONFLICT is Update's headline new
capability (Create has no revision to conflict on). Shipping catalog Update
without this two-line fix means the bridge's own readiness evidence would
have to document catalog CONFLICT as "not reachable" — while the sentinel
comments explicitly say the domain-level fix was expected to already exist.
Recommended fix: reassign `errApplicabilityStaleRevision` and
`errCatalogStaleRevisionV2` to `domain.ErrRevisionConflict` (or wrap/alias
them the same way `resource_repository_crud.go` does). Zero existing test
breakage expected — existing postgres tests assert against the unexported
sentinels directly, not through `core.Map`.

## Error-category reach

- **Catalog Update**: 11 categories reachable, including `NOT_FOUND`
  (reverses Create's finding — Create's catalog NOT_FOUND was proven
  unreachable via `insertLocked`; Update's candidate-fetch-before-update
  path can genuinely miss) and `IMMUTABLE_CODE` (new to Update — via
  `ImmutableOnceReferenced` on the `code` field descriptor,
  `internal/domain/catalog_kind.go:44,128`: "editable until at [referenced,
  then frozen]").
- **Resource Update**: 9 categories reachable, including `CONFLICT` (works
  today, confirmed above — no postgres fix needed on the resource side).

## Identity resolution

- Catalog: by `Kind` + `ID` — consistent with the existing public read
  contract (`CatalogKey`).
- Resource: by `ID` (a stable BIGSERIAL PK) — genuinely stable, but only
  obtainable today as a byproduct of a prior `GetResource`/`SearchResources`
  call; there is no direct by-ID lookup in the public read contract. Update
  requests will carry `ID` as returned by a prior read, same as internal
  callers do today.

## Actor attribution

Reuses the `core.WithActor`/`ActorFrom` mechanism built for Create — no new
design needed; `Writer.UpdateCatalog`/`UpdateResource` requests carry the
same `Actor string` field pattern as `CatalogWriteRequest`/
`ResourceWriteRequest`, validated non-blank, never persisted.

## Size estimate

~900-1100 authored lines across a 3-unit auto-chain (public contract types +
validation, bridge translation + actor wiring, postgres CONFLICT-wiring fix
+ readiness). Smaller per-operation than Create since inverse mappers and
shape validators are largely reusable, not rewritten.

## Approaches considered

1. **(Recommended, adopted)** Follow the archived design as-is for Update's
   shape, plus include the CONFLICT-wiring fix as an explicitly-scoped unit
   with its own justification in the proposal.
2. Ship Update without touching postgres, documenting catalog CONFLICT as
   "not proven" in readiness — rejected: CONFLICT is Update's headline new
   capability: shipping it silently broken is worse than a small,
   low-risk, two-sentinel reassignment that existing tests already exercise
   indirectly.

## Next recommended phase

`sdd-propose`
