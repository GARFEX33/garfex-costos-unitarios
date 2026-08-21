# Design: Public WRITE on the Resource Master Core — Update

## Decision summary

`resourcecore.Writer` and `WriteCapabilities` gain exactly two methods, `UpdateCatalog` and `UpdateResource`, mirroring `CreateCatalog`/`CreateResource` byte-for-byte in mechanism: package-owned DTOs reusing the existing public `Value`/`AttributeValue`/`ApplicabilityRule` types, shape validation only, defensive copy on entry and exit, sole-bridge translation, and `Actor` carried via the already-built `core.WithActor`/`ActorFrom` context-metadata mechanism — no new actor design. The two new request types add an identity field (`ID`, plus `Kind` for catalog) and `ExpectedRevision uint64` (CAS), exactly the shape the archived Create design already fixed on paper in its "complete eventual shape" table.

`internal/bridge/resourcecore.Adapter` widens `catalogWriter`/`resourceWriter` by exactly one method each — `UpdateRevision` — bound to `catalogo.Service.UpdateRevision` and `recursos.Service.UpdateRevision`, the already-built, production-hardened internal authority. **Unlike Create, Update needs no create-confirm read**: both `UpdateRevision` methods return the real, persisted record/resource — `(domain.CatalogRecord, error)` and `(domain.Resource, error)` — directly, with `ID`/`Revision` populated by the same transaction that performed the CAS write. Create's confirm-read gap was caused specifically by `internal/postgres.resourceRepository.Create(ctx, resource) error` returning no identity at all; `domain.ResourceRepositoryV2.UpdateRevision(ctx, resource, expectedRevision) (domain.Resource, error)` has no such gap, so this asymmetry does not carry over to Update.

This change also fixes catalog `CONFLICT`: `internal/postgres/catalog_admin_repository_v2.go`'s `errApplicabilityStaleRevision` and `errCatalogStaleRevisionV2` are two private `errors.New(...)` sentinels with no `errors.Is` relationship to `domain.ErrRevisionConflict`, so a stale catalog CAS write currently falls through `core.Map`'s `default` arm to `INTERNAL` instead of the already-coded `CONFLICT` arm (`internal/core/errors.go:72-73`). The fix is a two-line direct reassignment, not a new abstraction: `internal/core.Map` and `internal/domain` are unchanged.

`internal/core.Map` needs no change — the `Conflict` and `ImmutableCode` arms already exist and are already correctly ordered (`internal/core/errors.go:72-73,82-83`), added at a prior stage for exactly this eventual Update graduation.

## Goals and non-goals

### Goals

- Make an external Go consumer able to update an existing catalog record (any of the 11 kinds) and an existing resource under optimistic concurrency (`ExpectedRevision`), with no `internal` import.
- Make catalog `CONFLICT` genuinely reachable in production, not merely correctly mapped if injected.
- Prove field completeness for both update requests, reusing Create's inverse mappers without duplicating them.
- Prove catalog Update's and resource Update's error-category reach with the same seam-injection, precedence, and no-leakage discipline as Create.
- Keep `internal/app/*`, `internal/domain/*`, `cmd/garfex/`, and `internal/tui/` at zero changed lines; `internal/postgres/*` changes exactly two lines plus their comment.

### Non-goals

- `Deactivate`, `Reactivate`, or catalog `HardDelete` — separate later per-operation changes.
- Any read-contract change, including a by-ID resource lookup (`ResourceUpdateRequest.ID` is supplied by the caller from a prior read, same as internal callers already do).
- New `ErrorCode` values — all fifteen already exist.
- Any `Actor`-carriage redesign — Create's mechanism is reused as-is.
- Reshaping `CatalogWriteRequest`/`ResourceWriteRequest` (Create) — those stay untouched; Update is purely additive.

## Architecture and dependency direction

```text
external consumer
    |
    v
resourcecore.Writer
  (shape validation, defensive copy, Actor required — unchanged mechanism, +2 methods)
    |
    v
resourcecore.WriteCapabilities   (+2 methods: UpdateCatalog, UpdateResource)
    ^
    |
    +---- internal/bridge/resourcecore.Adapter (sole bridge, unchanged mechanism) ----+
              |                                                                        |
              |  core.WithActor(ctx) — reused as-is                                    |
              v                                                                        v
      internal/app/catalogo.Service.UpdateRevision              internal/app/recursos.Service.UpdateRevision
              |                                                                        |
              +-----------------------------> internal/domain <-----------------------+
                                                    |
                                                    v
                                     internal/postgres  (2-line CONFLICT-wiring fix)
```

Nothing above `Adapter` or below `internal/domain` changes shape. `resourcecore` still imports no `internal` package.

## Public write contract

### Compiled types (this change)

```go
// resourcecore/write_types.go — additive; CatalogWriteRequest/ResourceWriteRequest untouched.

// CatalogUpdateRequest replaces one existing catalog record of any
// registered kind under optimistic concurrency.
type CatalogUpdateRequest struct {
    Actor            string
    Kind             KindCode
    ID               int64
    ExpectedRevision uint64
    Active           bool
    Values           map[string]Value
    Rules            []ApplicabilityRule // APLICABILIDAD aggregate; nil and empty differ
}

// ResourceUpdateRequest replaces one existing resource under optimistic
// concurrency. ID is a stable BIGSERIAL primary key, obtainable today only
// as a byproduct of a prior GetResource/SearchResources call — there is no
// direct by-ID lookup in the public read contract, and adding one is
// explicitly out of scope.
type ResourceUpdateRequest struct {
    Actor            string
    ID               int64
    ExpectedRevision uint64
    Scope            ResourceScope
    NaturalUnit      string
    Attributes       []AttributeValue
}
```

```go
// resourcecore/writer.go

type WriteCapabilities interface {
    CreateCatalog(context.Context, CatalogWriteRequest) (CatalogRecord, error)
    CreateResource(context.Context, ResourceWriteRequest) (Resource, error)
    UpdateCatalog(context.Context, CatalogUpdateRequest) (CatalogRecord, error)
    UpdateResource(context.Context, ResourceUpdateRequest) (Resource, error)
}

func (w *Writer) UpdateCatalog(ctx context.Context, req CatalogUpdateRequest) (CatalogRecord, error) {
    if err := validateCatalogUpdateRequest(req); err != nil {
        return CatalogRecord{}, err
    }
    rec, err := w.cap.UpdateCatalog(ctx, CloneCatalogUpdateRequest(req))
    if err != nil {
        return CatalogRecord{}, err
    }
    return CloneCatalogRecord(rec), nil
}

func (w *Writer) UpdateResource(ctx context.Context, req ResourceUpdateRequest) (Resource, error) {
    if err := validateResourceUpdateRequest(req); err != nil {
        return Resource{}, err
    }
    res, err := w.cap.UpdateResource(ctx, CloneResourceUpdateRequest(req))
    if err != nil {
        return Resource{}, err
    }
    return CloneResource(res), nil
}
```

`resourcecore/copy.go` gains `CloneCatalogUpdateRequest`/`CloneResourceUpdateRequest`, built on the same `CloneValue`/`CloneStringSlice` primitives as the Create clones, preserving nil-versus-empty for `Values`/`Rules`/`Attributes` identically.

### Shape validation owned by `Writer` (never business validation)

`validateCatalogUpdateRequest` and `validateResourceUpdateRequest` reuse `validateValueShape`/`isKnownAttributeMode`/`isKnownKind` verbatim (zero duplication) and add exactly the CAS/identity checks `UpdateRevision`'s own internal guard already enforces server-side — the shape gate simply moves the same rejection to the boundary instead of paying a round trip for it:

- Blank `Actor` → `INVALID_ARGUMENT` (unchanged rule).
- `ID == 0` → `INVALID_ARGUMENT` — mirrors `catalogo.Service.UpdateRevision`'s `rec.ID == 0` guard (service.go:526) and `recursos.Service.UpdateRevision`'s `command.ID <= 0` guard (service.go:301).
- `ExpectedRevision == 0` → `INVALID_ARGUMENT` — mirrors both services' `expectedRevision == 0` guard. CAS is mandatory; there is no "unconditional update" request shape.
- Catalog: unknown `Kind`, empty `Values`, and per-`Value`/per-`Rule` shape — identical to `validateCatalogWriteRequest`. Update is a full-replacement write (`UpdateRevision` takes a complete `domain.CatalogRecord`, not a patch), so the non-empty-`Values` rule that governs Create governs Update for the same reason.
- Resource: blank `Scope.ClassCode`/`FamilyCode`/`TypeCode`/`NaturalUnit`, and per-`AttributeValue` shape — identical to `validateResourceWriteRequest`, for the same full-replacement reason (`domain.UpdateCommand` carries the complete `Scope`/`NaturalUnit`/`Attributes`, exactly `CreateCommand`'s shape plus `ID`).

No business rule — reference existence, uniqueness, immutability, revision currency — is pre-judged at the boundary; all of it belongs to the Core, unchanged from Create's discipline.

## Bridge translation

### Seam widening

```go
// internal/bridge/resourcecore/adapter.go

type catalogWriter interface {
    Create(ctx context.Context, kind domain.CatalogKindCode, rec domain.CatalogRecord) (domain.CatalogRecord, error)
    UpdateRevision(ctx context.Context, kind domain.CatalogKindCode, rec domain.CatalogRecord, expectedRevision uint64) (domain.CatalogRecord, error)
}

type resourceWriter interface {
    Create(ctx context.Context, command domain.CreateCommand) (domain.Resource, error)
    UpdateRevision(ctx context.Context, command domain.UpdateCommand, expectedRevision uint64) (domain.Resource, error)
}
```

`*catalogo.Service` and `*recursos.Service` already satisfy the widened ports — both `UpdateRevision` methods already exist in production (`internal/app/catalogo/service.go:525`, `internal/app/recursos/service.go:300`). Only in-repo test fakes gain one method each, adapted in the same slice, per the compatibility invariant. `internal/tui/catalog_admin.go`'s existing `catalogRecordUpdater` interface (`UpdateRevision(ctx, kind, rec, expectedRevision) (domain.CatalogRecord, error)`) already proves this exact method signature is a real, already-consumed production seam — the bridge is not inventing a new call shape.

### `Adapter.UpdateCatalog`/`Adapter.UpdateResource`

```go
// UpdateCatalog updates one existing catalog record under optimistic
// concurrency. Reuses the exact inverse value/rule mappers Create built;
// no new mapping code. Actor travels only as request-scoped context
// metadata, exactly as in CreateCatalog.
func (a *Adapter) UpdateCatalog(ctx context.Context, req public.CatalogUpdateRequest) (public.CatalogRecord, error) {
    kind := domain.CatalogKindCode(req.Kind)
    rec := domain.CatalogRecord{
        ID:     req.ID,
        Active: req.Active,
        Values: a.toDomainCatalogValues(kind, req.Values),
        Rules:  toDomainCatalogRules(req.Rules),
    }
    updated, err := a.catalog.UpdateRevision(core.WithActor(ctx, req.Actor), kind, rec, req.ExpectedRevision)
    if err != nil {
        return public.CatalogRecord{}, mapError(err)
    }
    return a.mapCatalogRecord(kind, updated), nil
}

// UpdateResource updates one existing resource under optimistic
// concurrency. No create-confirm read: ResourceRepositoryV2.UpdateRevision
// returns the real persisted Resource directly (unlike the legacy
// ResourceRepository.Create this bridge's CreateResource must work around).
func (a *Adapter) UpdateResource(ctx context.Context, req public.ResourceUpdateRequest) (public.Resource, error) {
    attrs, err := toDomainResourceAttributes(req.Attributes)
    if err != nil {
        return public.Resource{}, err
    }
    command := domain.UpdateCommand{
        ID:          req.ID,
        Scope:       domain.ResourceScope{ClassCode: req.Scope.ClassCode, FamilyCode: req.Scope.FamilyCode, TypeCode: req.Scope.TypeCode},
        NaturalUnit: req.NaturalUnit,
        Attributes:  attrs,
    }
    updated, err := a.resources.UpdateRevision(core.WithActor(ctx, req.Actor), command, req.ExpectedRevision)
    if err != nil {
        return public.Resource{}, mapError(err)
    }
    return mapResource(updated), nil
}
```

Both bodies reuse, unmodified: `a.toDomainCatalogValues`, `toDomainCatalogRules`, `toDomainResourceAttributes`, `a.mapCatalogRecord`, `mapResource`, `mapError`, and `core.WithActor`. Zero new mapping logic is written — the field-completeness and round-trip proofs Create's design already established (`domain.CatalogRecord`'s six fields, `domain.CreateCommand`'s three fields extended by `UpdateCommand`'s fourth field `ID`) carry over verbatim; only `ID` and `ExpectedRevision` are new field-completeness rows, both trivially mapped (identity passthrough, no sub-mapping).

### No create-confirm read — the resolved asymmetry

Create needed a confirm-read because `internal/postgres.resourceRepository.Create(ctx, resource) error` (`resource_repository_crud.go:22`) returns only `error` — no identity, no revision — forcing `recursos.Service.Create` to return a canonically-built `domain.Resource` with `ID == 0` and `Revision == 0`. Returning those zeros publicly would have been accept-and-ignore on the result side, so `Adapter.CreateResource` performed one extra `Get` by `identity-v1` after a successful `Create`.

`domain.ResourceRepositoryV2.UpdateRevision(ctx, resource, expectedRevision) (Resource, error)` (`internal/domain/resource_repository_v2.go:43`) has no such gap: it returns the real, persisted `domain.Resource` directly, and `recursos.Service.UpdateRevision` returns that value verbatim on success (`service.go:314-321`, `return result, nil`). Symmetrically, `catalogo.Service.UpdateRevision` returns `*result.Record` — the repository's own committed, ID/Revision-populated record (`service.go:546-550`), the exact mirror of how `insertLocked` already avoids a confirm-read on the catalog side. **Neither Update path needs a confirm read**; adding one would be a needless extra round trip with no missing data to recover.

### Field-completeness — no new table needed

`CatalogUpdateRequest` carries every `CatalogWriteRequest` field (`Actor`, `Kind`, `Active`, `Values`, `Rules`) plus `ID`/`ExpectedRevision`; `ResourceUpdateRequest` carries every `ResourceWriteRequest` field (`Actor`, `Scope`, `NaturalUnit`, `Attributes`) plus `ID`/`ExpectedRevision`. Create's design already proved the shared fields complete against `domain.CatalogRecord` (6 fields) and `domain.CreateCommand` (3 fields); `domain.UpdateCommand` is `CreateCommand`'s shape plus `ID` (`resource_types.go:204-209`, verified in explore.md), so only `ID`/`ExpectedRevision` need a fresh completeness row each:

| Public field | Internal destination | Verdict |
| --- | --- | --- |
| `CatalogUpdateRequest.ID` | `catalogo.Service.UpdateRevision`'s `rec.ID` | Mapped |
| `CatalogUpdateRequest.ExpectedRevision` | `UpdateRevision`'s `expectedRevision` argument | Mapped |
| `ResourceUpdateRequest.ID` | `domain.UpdateCommand.ID` | Mapped |
| `ResourceUpdateRequest.ExpectedRevision` | `recursos.Service.UpdateRevision`'s `expectedRevision` argument | Mapped |

Every other field's mapping is identical to Create's already-proven table — a table-driven inverse round-trip test still runs per unit (per the discoverability gate), but it exercises the same mapping functions, not new ones.

## CONFLICT-wiring fix

`internal/postgres/catalog_admin_repository_v2.go` — two sentinels, currently private `errors.New(...)` values with no `errors.Is` relationship to `domain.ErrRevisionConflict`, are reassigned to alias it directly.

**Line 20 — before:**

```go
errApplicabilityStaleRevision    = errors.New("applicability revision conflict") // stand-in for future domain.ErrRevisionConflict
```

**Line 20 — after:**

```go
errApplicabilityStaleRevision    = domain.ErrRevisionConflict
```

**Lines 193-196 — before:**

```go
// errCatalogStaleRevisionV2 is a local, unexported sentinel — stand-in for
// the future domain.ErrRevisionConflict (stage 5), same rationale as 3E's
// errApplicabilityStaleRevision.
var errCatalogStaleRevisionV2 = errors.New("catalog revision conflict")
```

**Lines 193-196 — after:**

```go
// errCatalogStaleRevisionV2 aliases domain.ErrRevisionConflict directly
// (stage-5 fix, resource-master-core-write-update): the "future" the prior
// comment named has arrived. Kept as a local name — not deleted — because
// casUpdateRevision and its 20+ call sites/tests below still read most
// clearly with the local, purpose-scoped identifier.
var errCatalogStaleRevisionV2 = domain.ErrRevisionConflict
```

### Decision: direct reassignment, not `fmt.Errorf`-wrapping

**Verified first, before deciding.** Every existing reference to either sentinel across `internal/postgres` was enumerated by grep: both are compared exclusively via `errors.Is(err, errApplicabilityStaleRevision)` / `errors.Is(err, errCatalogStaleRevisionV2)` in `catalog_admin_repository_v2_integration_test.go` (14 call sites, lines 111, 407, 414, 603, 746, 753, 1027, 1037, 1047). None compares with `==`, none asserts `.Error()` string content, and no other package imports either identifier (both are unexported). Non-test production code returns each sentinel directly (`return 0, errApplicabilityStaleRevision`; `return 0, errCatalogStaleRevisionV2`), never wrapped.

Given that, **direct reassignment is strictly sufficient and strictly simpler than wrapping**: after `var errCatalogStaleRevisionV2 = domain.ErrRevisionConflict`, the two identifiers are the *same value*. Every existing `errors.Is(err, errCatalogStaleRevisionV2)` assertion still passes — `err` is that exact value, compared to itself — and `errors.Is(err, domain.ErrRevisionConflict)` (what `core.Map`'s `Conflict` arm and this change's bridge/postgres readiness tests need) is now *also* true, because it is the identical comparison. A `fmt.Errorf("%w: ...", domain.ErrRevisionConflict)`-style wrap would satisfy both checks too, but it would add a new allocation and a new message string that no test or design requirement needs, and it would keep two logically-identical-but-distinct error identities alive for no reason — a needless layer the strict-shape discipline elsewhere in this codebase (e.g. "never coerce, never wrap without a reason") argues against. Zero existing test breakage is therefore not just expected but structurally guaranteed by this reassignment, not merely likely.

## Error-category reachability

Both tables use the same method as the archived Create readiness record: a category counts as reachable only when a genuine, cited, production code path can return the underlying domain sentinel — not merely "the bridge's `mapError` classifies it correctly if injected." Every "Yes" row cites the exact source line; every "No" row cites why no such path exists for *this* operation specifically (a sentinel raised only by Deactivate/Reactivate/Delete does not make Update reach it).

### Catalog Update — 10 of 15 reachable (revises explore.md's estimate of 11)

| Category | Reachable | Evidence |
| --- | --- | --- |
| `INVALID_ARGUMENT` | Yes | `Writer` shape gate; `catalogo.ErrInvalidArgument` (`= core.ErrInvalidArgument`) on `rec.ID == 0 \|\| expectedRevision == 0` (`service.go:526-528`) |
| `NOT_FOUND` | **Yes — new, reverses Create's finding** | `buildV2UpdateCandidate`'s `s.repo.Get(ctx, kind, rec.ID)` (`service.go:409-412`) when the record is absent; `domain.ApplyCatalogMutation(...OpUpdate...)`'s `mutateSlice` `idx < 0` branch (`catalog_mutation.go:494-498`); `casUpdateRevision`'s same-transaction absent-row lookup (`catalog_admin_repository_v2.go:216-220`) — three independent, genuinely reachable paths, unlike Create's blind insert |
| `DUPLICATE` | Yes | `mapCatalogWriteError`'s unique-constraint (`23505`) classification, reached from every kind's update-row helper (e.g. `catalog_admin_repository_v2.go:121,214`) — the same postgres constraint mapping Insert already uses, now proven reachable from an update path too |
| `INVALID_REFERENCE` | Yes | Same `mapCatalogWriteError` FK-violation (`23503`) classification; also `validateApplicabilityRecord`'s `domain.ErrResourceReference` for APLICABILIDAD's family/type/characteristic checks (`catalog_mutation.go:392,397`), reached via `buildV2UpdateCandidate`'s `ApplyCatalogMutation(OpUpdate, ...)` call (`m.Op == OpInsert \|\| m.Op == OpUpdate` guard, `catalog_mutation.go:146`) |
| `VALIDATION` | **Yes** | `validateApplicabilityRecord`'s `domain.ErrResourceValidation` on `Rules == nil` (`catalog_mutation.go:380-381`), reached the same `OpUpdate` path as above — genuinely catalog-reachable, not resource-only despite the sentinel's name |
| `INTEGRITY` | **No** | `domain.ErrResourceIntegrity` has zero references anywhere in `internal/postgres/catalog_admin_*.go` (grep-verified); it is a resource-repository-only sentinel |
| `IDENTITY_CONFLICT` | No | `domain.ErrIdentityConflict` is raised only inside resource reactivation (`internal/app/recursos/service.go:288-289`) — no catalog path constructs it |
| `INVALID_LIFECYCLE` | No | Raised only by `buildV2DeleteCandidate`'s active-record guard (`service.go:456-458`) — Delete-only, not reachable from `UpdateRevision` |
| `REACTIVATION_IMPOSSIBLE` | No | `domain.WrapReactivationImpossible` — Reactivate-only |
| `INVALID_CATALOG` | Yes | `domain.WrapInvalidCatalog(next.Validate())` at the end of `buildV2UpdateCandidate` (`service.go:439-443`) — the exact mirror of `insertLocked`'s use on Create |
| `IN_USE` | No | `domain.ErrCatalogInUse` is raised only by `buildV2DeleteCandidate`'s dependency/reference guards (`service.go:465,473`) — Delete-only |
| `IMMUTABLE_CODE` | **Yes — new** | `buildV2UpdateCandidate`'s referenced-code-rename guard: `domain.ErrCodeImmutable` when an `ImmutableOnceReferenced` field (`code`) changes on a record already referenced by a resource (`service.go:420-434`) |
| `CONFLICT` | **Yes — new, this change's fix** | `casUpdateRevision`'s present-but-different-revision branch (`catalog_admin_repository_v2.go:224`) and `updateApplicabilityAggregateV2`'s parallel APLICABILIDAD path (line 131) — both newly aliased to `domain.ErrRevisionConflict` above |
| `UNAVAILABLE` | Yes | `catalogo.ErrCatalogWriterUnavailable` / `ErrCatalogAdminRepositoryV2Unavailable` from `prepareV2Write`; ctx `Canceled`/`DeadlineExceeded` |
| `INTERNAL` | Yes | `errCatalogCandidateMismatchV2` (reload/compare mismatch — deliberately carries no `errors.Is` relationship to any domain sentinel, unlike the two CONFLICT sentinels this change fixes) and any other unclassified failure |

**Discrepancy with explore.md, stated honestly.** explore.md estimated 11 catalog-reachable categories. Direct source tracing (not repeated here from memory, re-verified this session) finds 10: `VALIDATION` and the second `INVALID_REFERENCE` source both flow through the same `validateApplicabilityRecord` call already counted once each, and `INTEGRITY` has no catalog-admin call site at all — grep-confirmed empty across `internal/postgres/catalog_admin_*.go`. This narrows, rather than weakens, the claimed evidence, matching the archived Create design's own precedent of catching and correctly re-scoping an estimate (that design's catalog `NOT_FOUND`, reversed here). The readiness record for this change must carry the proven 10, not the estimated 11.

### Resource Update — 9 of 15 reachable (matches explore.md exactly)

| Category | Reachable | Evidence |
| --- | --- | --- |
| `INVALID_ARGUMENT` | Yes | `Writer` shape gate; `recursos.ErrInvalidArgument` on `command.ID <= 0 \|\| expectedRevision == 0` (`service.go:301-303`) |
| `NOT_FOUND` | Yes | `domain.ErrResourceNotFound`, explicitly passed through unwrapped (`service.go:316-317`) |
| `DUPLICATE` | Yes | `domain.ErrDuplicateResource`, explicitly passed through (`service.go:316-317`) |
| `INVALID_REFERENCE` | Yes | `domain.ErrResourceReference`, explicitly passed through (`service.go:316-317`) |
| `VALIDATION` | Yes | `domain.NewResource`'s validation failure, returned directly before any repository call (`service.go:305-308`) |
| `INTEGRITY` | Yes | `domain.ErrResourceIntegrity`, explicitly passed through (`service.go:316-317`) |
| `IDENTITY_CONFLICT` | No | Constructed only inside `ReactivateRevision`'s error-classification switch (`service.go:288-289`) — Reactivate-only, `UpdateRevision` never rewraps `ErrResourceIntegrity` into it |
| `INVALID_LIFECYCLE` | No | No resource Update path raises it |
| `REACTIVATION_IMPOSSIBLE` | No | Reactivate-only |
| `INVALID_CATALOG` | No | Catalog-only sentinel |
| `IN_USE` | No | Catalog-Delete-only sentinel |
| `IMMUTABLE_CODE` | No | Catalog-only concept — resources have no "código" field |
| `CONFLICT` | Yes | `domain.ErrResourceRevisionConflict`, explicitly passed through (`service.go:316-317`) — **already correctly wired today**; confirmed by `resource_repository_crud.go:396,467,496` returning it directly on stale CAS. No postgres fix needed on the resource side |
| `UNAVAILABLE` | Yes | ctx `Canceled`/`DeadlineExceeded` (`core.Map`'s generic arm); no resource-specific unavailable sentinel exists, but the ctx-based arm still applies to any call made with a cancellable ctx |
| `INTERNAL` | Yes | `fmt.Errorf("update resource %d: %w", ...)` fallback (`service.go:319`) for any error outside the five explicitly listed — no `errors.Is` relationship to any domain sentinel, falls to `default` |

Evidence shape for both tables (mirroring Create): per-entry-point table-driven tests inject each sentinel family into the widened `catalogWriter`/`resourceWriter` seam and assert the exact public code, distinctness from every other reachable category, and no leakage (`Error()`, `%v`, `%+v`, concrete type, `errors.Unwrap`, recursive chain) of pgx, `PgError`, SQLSTATE, constraints, tables, columns, or server text.

## Files and change surfaces

| Surface | Action | Description |
| --- | --- | --- |
| `resourcecore/write_types.go` | Modify | `CatalogUpdateRequest`, `ResourceUpdateRequest` (additive; Create's two types untouched) |
| `resourcecore/writer.go` | Modify | `UpdateCatalog`, `UpdateResource`, `WriteCapabilities` +2 methods, `validateCatalogUpdateRequest`, `validateResourceUpdateRequest` (reusing `validateValueShape`/`isKnownAttributeMode`/`isKnownKind`) |
| `resourcecore/copy.go` | Modify | `CloneCatalogUpdateRequest`, `CloneResourceUpdateRequest` |
| `resourcecore/writer_test.go` | Modify | Update shape gate, ID/ExpectedRevision-zero rejection, defensive copy, no ungraduated symbol beyond the now-4-method surface |
| `resourcecore/external_test.go` | Modify | External consumer updates both, using only public types |
| `internal/bridge/resourcecore/adapter.go` | Modify | `catalogWriter`/`resourceWriter` +1 method each, `Adapter.UpdateCatalog`, `Adapter.UpdateResource` (zero new mapping functions — reuses Create's) |
| `internal/bridge/resourcecore/adapter_test.go` | Modify | Fakes gain `UpdateRevision`; field-completeness for `ID`/`ExpectedRevision`; catalog 10-category and resource 9-category injected tables |
| `internal/postgres/catalog_admin_repository_v2.go` | Modify | Lines 20 and 193-196 — sentinel reassignment, exactly as specified above |
| `internal/postgres/catalog_admin_repository_v2_integration_test.go` | Unchanged | All 14 `errors.Is` assertions against the two sentinels continue to pass unmodified (structurally guaranteed, see decision above) |
| `internal/core/*` | Unchanged | `Conflict`/`ImmutableCode` arms already exist and are already correctly ordered |
| `internal/app/*`, `internal/domain/*` | Unchanged | Write authority already built and wired |
| `cmd/garfex/`, `internal/tui/` | Unchanged | Zero `resourcecore` references before and after |
| `openspec/changes/resource-master-core-write-update/readiness.md` | Create | Update readiness record: 10 catalog / 9 resource reachable categories, CONFLICT-fix verification, zero-test-breakage confirmation |

## Compatibility invariant

Unchanged from Create's binding decision, restated for Update: after every unit, `go test ./... -count=1` builds every current adapter, service, composition root, TUI call site, test double, and external test. A unit never adds a public method for an ungraduated operation, never adds a public request field an operation ignores, never changes `core.Record`'s, `core.WithActor`'s, or `DiagnosticSink`'s signature, never changes `internal/app`, `internal/domain`, `cmd/garfex`, or `internal/tui`, and never widens a bridge seam without adapting every in-repo fake in that same unit. `internal/postgres` changes are limited to exactly the two-line CONFLICT fix — no other postgres file changes. The READ contract and Create's compiled WRITE surface stay byte-compatible.

## Task-unit breakdown (auto-chain)

Three units, matching explore.md's ~900-1100-line estimate and the 400-line review-budget risk the proposal already flags as High.

| Unit | Start state | End state and compatibility proof | Required focused evidence | Forecast |
| --- | --- | --- | --- | --- |
| **U1 — public Update contract** | Create-WRITE shipped (archived) | Add `CatalogUpdateRequest`, `ResourceUpdateRequest`, their clones, `Writer.UpdateCatalog`/`UpdateResource`, shape validation reusing `validateValueShape`/`isKnownKind`/`isKnownAttributeMode`. No bridge change — `WriteCapabilities` grows two methods with nothing implementing them in production yet, so no composition is affected. `Reader` and Create's compiled surface untouched. | `go test ./resourcecore -run 'TestWriter\|TestExternalWrite\|TestWriteRequestCopy' -count=1`; ID/ExpectedRevision-zero → `INVALID_ARGUMENT` table; caller-mutation-after-call table extended to the two new request types; reflection assertion that the compiled surface is now exactly 4 methods, no 5th ungraduated one | ≤400 |
| **U2 — bridge Update translation (catalog + resource)** | U1 | Widen `catalogWriter`/`resourceWriter` with `UpdateRevision`, add `Adapter.UpdateCatalog`/`Adapter.UpdateResource` reusing every existing inverse mapper unmodified, adapt bridge fakes in-slice. Both catalog and resource in one unit because neither needs the postgres fix to be tested at this layer — the seam is injected, not real postgres, so `CONFLICT` reachability is provable via injection here even before U3 lands. | `go test ./internal/bridge/resourcecore -run 'TestWriteBridge\|TestCatalogUpdate\|TestResourceUpdate' -count=1`; field-completeness table for `ID`/`ExpectedRevision` against both `domain.CatalogRecord` and `domain.UpdateCommand`; catalog 10-category and resource 9-category injected tables with distinctness/no-leakage assertions; explicit assertion that `Adapter.UpdateResource` performs no confirm-read call (mock call-count proof) | ≤400 |
| **U3 — CONFLICT-wiring fix + readiness** | U2 | Reassign `errApplicabilityStaleRevision`/`errCatalogStaleRevisionV2` to `domain.ErrRevisionConflict`; run the full existing `catalog_admin_repository_v2_integration_test.go` suite to confirm zero breakage; write the Update readiness record (10/9 category tables, CONFLICT proof, zero-touch confirmation for `cmd/garfex`/`internal/tui`). Full-suite verification. | `go test ./internal/postgres -run 'TestCatalogAdminRepositoryV2\|TestApplicability' -count=1` (existing tests, unmodified, must stay green) plus the full suite `go test ./... -count=1`; `rg -l resourcecore cmd/garfex internal/tui` / `git diff --stat -- cmd/garfex internal/tui` both empty | ≤400 |

Review-guard forecast: **Decision needed before apply: No** (the project's configured `auto-chain` already covers it). **Chained PRs recommended: Yes.** **400-line budget risk: High**, per the proposal — Create's precedent cost 1,348 lines for 7 methods; Update is smaller (2 methods, reused mappers) but the two full reachability tables and their injected tests are still substantial per-unit evidence.

Each unit follows strict red-green-refactor with RED, GREEN, TRIANGULATE, and REFACTOR evidence, and leaves the tree green. Focused tests run first, then `gofmt -l .`, `go vet ./...`, `golangci-lint run ./...`, `go test ./... -count=1`. Per the `GARFEX_STRICT` focused-test discoverability gate, every new test function name contains a literal alternative from its unit's `-run` pattern as a substring, and each focused command must select at least one test in every package the unit touched.

## Invariants and failure modes

| Invariant / failure | Required behavior |
| --- | --- |
| `ID == 0` (either request) | `INVALID_ARGUMENT`; no capability call |
| `ExpectedRevision == 0` (either request) | `INVALID_ARGUMENT`; no capability call — CAS is mandatory, there is no unconditional Update |
| Blank or whitespace `Actor` | `INVALID_ARGUMENT`; no capability call |
| `Actor` supplied | Every downstream diagnostic under that ctx is attributed, via the unchanged Create-built mechanism; nothing is persisted |
| Stale `ExpectedRevision` (catalog) | `CONFLICT`, not `INTERNAL` — the headline fix of this change |
| Stale `ExpectedRevision` (resource) | `CONFLICT` — already worked before this change; unaffected |
| Referenced `code` field changed on a catalog record | `IMMUTABLE_CODE`; never silently accepted |
| Catalog record absent at the given `ID` | `NOT_FOUND` — genuinely reachable for Update, unlike Create |
| `Value.Kind` mismatches the descriptor field kind | `INVALID_ARGUMENT`; never coerced (unchanged from Create) |
| Public field the domain cannot honor | `MISSING DOMAIN CRITERION` BLOCKER: drop the field from the contract; never accept-and-ignore |
| Update succeeds | Returns the persisted projection with the new `Revision`; no confirm-read round trip performed |
| Any internal error | One of the 10 (catalog) or 9 (resource) proven categories; no pgx/SQLSTATE/constraint/table/column/server text in string, type, or unwrap chain; no public `Unwrap` |
| Bridge encounters a business decision | BLOCKER — move it to the Core (unchanged discipline) |
| Ungraduated operation (`Deactivate`/`Reactivate`/`HardDelete`) | Not exported, not stubbed, not discoverable at runtime |

## Threat matrix

N/A — no routing, shell command, subprocess, VCS/PR automation, executable-file classification, or process-integration boundary. This change adds in-process Go types, one in-process translation extension, and a two-line sentinel reassignment.

## Migration / rollout

No migration. No schema change, no new column, no composition wiring. `Actor` is never persisted (unchanged). The CONFLICT fix changes only which in-process `error` value two functions return — no SQL, no schema, no data touched. Rolling back `U1`/`U2` reverts `UpdateCatalog`/`UpdateResource` and their bridge translation only, leaving Create and Read unaffected. Rolling back `U3` restores the prior `INTERNAL` misclassification for catalog `CONFLICT` — no persisted data or migration is touched by either the fix or its rollback, exactly as the proposal's rollback plan states.

## Alternatives rejected

| Alternative | Why rejected |
| --- | --- |
| Give `Adapter.UpdateResource` a confirm-read symmetric with `CreateResource` | `ResourceRepositoryV2.UpdateRevision` already returns the real persisted `Resource` — a confirm-read would be a needless extra round trip solving a gap that does not exist on this path |
| Wrap the CONFLICT sentinels with `fmt.Errorf("%w: ...", domain.ErrRevisionConflict)` instead of direct reassignment | Every existing reference uses `errors.Is`, never `.Error()` string content or `==`; wrapping satisfies the same checks as direct reassignment but adds an allocation and a second, now-redundant error identity for no proven need |
| Ship catalog Update without the postgres fix, documenting `CONFLICT` as "not reachable" | `CONFLICT` is Update's headline new capability (Create has no revision to conflict on); the sentinel comments already said the domain-level fix was expected to exist; the fix is two lines and structurally cannot break any existing `errors.Is`-based test |
| One `CatalogWriteRequest`/`ResourceWriteRequest` covering both Create and Update, with `ID`/`ExpectedRevision` that Create ignores | Reproduces the `ResourceQuery.TypeCode` accept-and-ignore precedent the strict profile forbids — the exact alternative Create's own design already rejected for the same reason |
| Report explore.md's estimated 11 catalog-reachable categories without re-verifying | Violates "verify technical claims before stating them"; direct source tracing found 10, cited above, and the design records the corrected count rather than repeating an unverified estimate |
| Add a public by-ID resource read to remove the "identity from a prior read" friction | Explicitly out of scope per the proposal; Update's identity model matches how every existing internal caller already obtains a resource `ID` |

## Open questions

- [ ] U2's RED test must prove `Adapter.UpdateResource` never calls `resources.Get` — the create-confirm-read pattern's *absence* is itself a claim this design makes and a later reviewer should be able to falsify via a call-count assertion on the fake, not by reading this document.
- [ ] `ImmutableOnceReferenced` is defined generically per catalog kind (`internal/domain/catalog_kind.go:44,128`), currently only on each kind's `code` field. U2's `IMMUTABLE_CODE` evidence should confirm this via the registry, not hardcode one kind, in case a future kind adds a second immutable field.
