# Design: Public WRITE on the Resource Master Core — Deactivate/Reactivate

## Decision summary

`resourcecore.Writer` and `WriteCapabilities` gain exactly four methods — `DeactivateCatalog`, `ReactivateCatalog`, `DeactivateResource`, `ReactivateResource` — mirroring Create/Update's mechanism byte-for-byte: package-owned DTOs, shape validation only, sole-bridge translation, `Actor` carried via the already-built `core.WithActor`/`ActorFrom` context-metadata mechanism. `WriteCapabilities` reaches 8 methods total (Create ×2, Update ×2, Deactivate ×2, Reactivate ×2).

Unlike Create/Update, the two new request DTOs (`CatalogLifecycleRequest`, `ResourceLifecycleRequest`) carry **no content fields** — only identity + `ExpectedRevision` + `Actor`. Scalar-only requests need **no new `Clone*` function**: Go value semantics already give full immutability, confirmed against `resourcecore/copy.go`'s existing pattern (every `Clone*` there exists specifically to deep-copy a map or slice field; these two requests have neither).

The structural novelty is asymmetric, confirmed by direct read: `catalogo.Service.DeactivateRevision`/`ReactivateRevision` (`internal/app/catalogo/service.go:598,603`) both delegate to `setActiveRevision` (line 556) and return **only `error`** — no record. `Adapter.DeactivateCatalog`/`ReactivateCatalog` therefore issue a confirm-read (`a.catalog.Get`) after a successful call, the exact precedent `Adapter.CreateResource` already established for a return-shape gap (`adapter.go:234`, cited below). `recursos.Service.DeactivateRevision`/`ReactivateRevision` (`internal/app/recursos/service.go:233,255`) both already return the complete `(domain.LifecycleResult, error)` — `Adapter.DeactivateResource`/`ReactivateResource` need **no confirm-read**, mapping `LifecycleResult.Resource` directly through the existing `mapResource` (`adapter.go:513`), zero new mapping code either way.

`internal/core.Map` needs no change: `IdentityConflict` and `ReactivationImpossible` arms already exist and are already correctly ordered (`internal/core/errors.go:74-77`), added at a prior stage for exactly this eventual graduation.

## Goals and non-goals

### Goals

- Let an external Go consumer deactivate/reactivate a catalog record (any of the 11 kinds) and a resource under optimistic concurrency (`ExpectedRevision`), no `internal` import.
- Prove catalog Deactivate/Reactivate's identical 6-category reach, resource Deactivate's 5-category reach, and resource Reactivate's 7-category reach — the first graduation reaching `IDENTITY_CONFLICT`/`REACTIVATION_IMPOSSIBLE`.
- Document, not fix, the idempotent no-op / CAS-bypass asymmetry (catalog both ops + resource Reactivate short-circuit before the CAS compare; resource Deactivate does not).
- Keep `internal/app/*`, `internal/domain/*`, `cmd/garfex/`, `internal/tui/` at zero changed lines.

### Non-goals

- `HardDelete` — separate, later, most-destructive-last change.
- Any read-contract change (no `Changed` field, no "include inactive" filter).
- New `ErrorCode` values — all 15 already exist.
- Fixing the no-op/CAS-bypass asymmetry — reused-as-is, pre-existing `internal/app` behavior the TUI already depends on.

## Architecture and dependency direction

```text
external consumer
    |
    v
resourcecore.Writer
  (shape validation only, Actor required — unchanged mechanism, +4 methods)
    |
    v
resourcecore.WriteCapabilities   (+4: DeactivateCatalog, ReactivateCatalog, DeactivateResource, ReactivateResource)
    ^
    |
    +---- internal/bridge/resourcecore.Adapter (sole bridge, unchanged mechanism) ----+
              |                                                                        |
              |  core.WithActor(ctx) — reused as-is                                    |
              v                                                                        v
   catalogo.Service.DeactivateRevision/ReactivateRevision      recursos.Service.DeactivateRevision/ReactivateRevision
       (error only) -> confirm-read a.catalog.Get                  (LifecycleResult, error) -> mapResource directly
              |                                                                        |
              +-----------------------------> internal/domain <-----------------------+
                                                    |
                                                    v
                                             internal/postgres  (unchanged)
```

Nothing above `Adapter` or below `internal/domain` changes shape. `resourcecore` still imports no `internal` package.

## Public write contract

```go
// resourcecore/write_types.go — additive; all prior request types untouched.

// CatalogLifecycleRequest deactivates or reactivates one existing catalog
// record of any registered kind under optimistic concurrency. No content
// fields — Deactivate/Reactivate never touch Values/Rules.
type CatalogLifecycleRequest struct {
    Actor            string
    Kind             KindCode
    ID               int64
    ExpectedRevision uint64
}

// ResourceLifecycleRequest deactivates or reactivates one existing resource
// under optimistic concurrency. No content fields.
type ResourceLifecycleRequest struct {
    Actor            string
    ID               int64
    ExpectedRevision uint64
}
```

```go
// resourcecore/writer.go

type WriteCapabilities interface {
    CreateCatalog(context.Context, CatalogWriteRequest) (CatalogRecord, error)
    CreateResource(context.Context, ResourceWriteRequest) (Resource, error)
    UpdateCatalog(context.Context, CatalogUpdateRequest) (CatalogRecord, error)
    UpdateResource(context.Context, ResourceUpdateRequest) (Resource, error)
    DeactivateCatalog(context.Context, CatalogLifecycleRequest) (CatalogRecord, error)
    ReactivateCatalog(context.Context, CatalogLifecycleRequest) (CatalogRecord, error)
    DeactivateResource(context.Context, ResourceLifecycleRequest) (Resource, error)
    ReactivateResource(context.Context, ResourceLifecycleRequest) (Resource, error)
}

func (w *Writer) DeactivateCatalog(ctx context.Context, req CatalogLifecycleRequest) (CatalogRecord, error) {
    if err := validateCatalogLifecycleRequest(req); err != nil {
        return CatalogRecord{}, err
    }
    rec, err := w.cap.DeactivateCatalog(ctx, req) // scalar-only: no Clone* needed
    if err != nil {
        return CatalogRecord{}, err
    }
    return CloneCatalogRecord(rec), nil
}
// ReactivateCatalog, DeactivateResource, ReactivateResource follow the same
// three-line shape (validate, delegate, clone-on-return only where the
// return type itself owns a map/slice — CatalogRecord/Resource, not the
// scalar-only request).
```

`resourcecore/copy.go`: **no new `Clone*` function**. `CatalogLifecycleRequest`/`ResourceLifecycleRequest` have zero map/slice fields, so passing `req` by value already gives the caller-mutation-after-call guarantee every existing `Clone*` exists to provide for a reference-typed field. The returned `CatalogRecord`/`Resource` still go through the existing `CloneCatalogRecord`/`CloneResource` on the way out — unchanged, reused as-is.

### Shape validation owned by `Writer`

```go
func validateCatalogLifecycleRequest(req CatalogLifecycleRequest) error {
    if strings.TrimSpace(req.Actor) == "" {
        return NewError(InvalidArgument, "actor is required")
    }
    if req.ID == 0 {
        return NewError(InvalidArgument, "id is required")
    }
    if req.ExpectedRevision == 0 {
        return NewError(InvalidArgument, "expected revision is required")
    }
    return nil
}
// validateResourceLifecycleRequest: identical minus Kind (resource has none).
```

No unknown-`Kind` check and no `Values`/`Rules` shape loop — there is no content payload to validate; `isKnownKind` is deliberately not reused here since an invalid `Kind` on a lifecycle request surfaces as `NOT_FOUND` from the internal `Get` inside `setActiveRevision` (service.go:568), not a boundary shape rejection — mirroring how Update's boundary never pre-validates reference existence.

## Bridge translation

### Seam widening

```go
// internal/bridge/resourcecore/adapter.go

type catalogWriter interface {
    Create(ctx context.Context, kind domain.CatalogKindCode, rec domain.CatalogRecord) (domain.CatalogRecord, error)
    UpdateRevision(ctx context.Context, kind domain.CatalogKindCode, rec domain.CatalogRecord, expectedRevision uint64) (domain.CatalogRecord, error)
    DeactivateRevision(ctx context.Context, kind domain.CatalogKindCode, id int64, expectedRevision uint64) error
    ReactivateRevision(ctx context.Context, kind domain.CatalogKindCode, id int64, expectedRevision uint64) error
}

type resourceWriter interface {
    Create(ctx context.Context, command domain.CreateCommand) (domain.Resource, error)
    UpdateRevision(ctx context.Context, command domain.UpdateCommand, expectedRevision uint64) (domain.Resource, error)
    DeactivateRevision(ctx context.Context, id int64, expectedRevision uint64) (domain.LifecycleResult, error)
    ReactivateRevision(ctx context.Context, id int64, expectedRevision uint64) (domain.LifecycleResult, error)
}
```

Both signatures are not invented: `internal/tui/catalog_admin.go`'s `catalogDeactivator`/`catalogReactivator` interfaces (lines 75-81) already require exactly `DeactivateRevision(ctx, kind, id, expectedRevision) error` / `ReactivateRevision(ctx, kind, id, expectedRevision) error`, consumed today by the catalog-admin TUI flow. `internal/tui/resource_editor.go`'s `resourceLifecycle` interface (lines 131-134) already requires exactly `DeactivateRevision(ctx, id, expectedRevision) (domain.LifecycleResult, error)` / `ReactivateRevision(ctx, id, expectedRevision) (domain.LifecycleResult, error)`, consumed by the resource-editor TUI flow. `*catalogo.Service` and `*recursos.Service` already satisfy both widened ports in production. Only in-repo test fakes gain two methods each, adapted in the same slice.

### `Adapter.DeactivateCatalog`/`ReactivateCatalog` — confirm-read, same precedent as Create

```go
// DeactivateCatalog/ReactivateCatalog toggle one catalog record's lifecycle
// state under optimistic concurrency. DeactivateRevision/ReactivateRevision
// return only error — no record, because postgres's SetActive builds a
// deliberately partial 4-field domain.CatalogRecord projection even at the
// domain-port level (catalog_admin_repository_v2.go:1100). A confirm-read
// recovers the complete public record, the same gap-recovery shape
// CreateResource already established at adapter.go:234 for a different
// underlying gap (resourceRepository.Create returning only error).
func (a *Adapter) DeactivateCatalog(ctx context.Context, req public.CatalogLifecycleRequest) (public.CatalogRecord, error) {
    kind := domain.CatalogKindCode(req.Kind)
    if err := a.catalog.DeactivateRevision(core.WithActor(ctx, req.Actor), kind, req.ID, req.ExpectedRevision); err != nil {
        return public.CatalogRecord{}, mapError(err)
    }
    rec, err := a.catalog.Get(ctx, kind, req.ID)
    if err != nil {
        return public.CatalogRecord{}, mapError(err) // confirm-read failure: unreclassified, passed straight through
    }
    return a.mapCatalogRecord(kind, rec), nil
}
// ReactivateCatalog is byte-identical except it calls a.catalog.ReactivateRevision.
```

Confirm-read failure is **not** given a distinct error category: it flows through the same `mapError` any other `a.catalog.Get` failure would use — exactly `CreateResource`'s established discipline of reporting the second call's own outcome honestly rather than inventing a new classification for "wrote but couldn't read back" (see proposal question 1: assumption used = report honestly, no retry).

### `Adapter.DeactivateResource`/`ReactivateResource` — no confirm-read

```go
// DeactivateResource/ReactivateResource toggle one resource's lifecycle
// state under optimistic concurrency. No confirm-read: LifecycleResult
// already carries the real, persisted Resource.
func (a *Adapter) DeactivateResource(ctx context.Context, req public.ResourceLifecycleRequest) (public.Resource, error) {
    result, err := a.resources.DeactivateRevision(core.WithActor(ctx, req.Actor), req.ID, req.ExpectedRevision)
    if err != nil {
        return public.Resource{}, mapError(err)
    }
    return mapResource(result.Resource), nil
}
// ReactivateResource is byte-identical except it calls a.resources.ReactivateRevision.
```

Zero new mapping functions on either bridge path — `mapCatalogRecord`, `mapResource`, `mapError`, `core.WithActor` are all reused verbatim.

## Error-category reachability

Same method as Create/Update's archived readiness records: a category counts as reachable only when a cited production code path can return the underlying domain sentinel for *this exact operation*, not merely "mapError classifies it correctly if injected."

### Catalog Deactivate / Reactivate — identical, 6 of 15

| Category | Reachable | Evidence | Test approach |
| --- | --- | --- | --- |
| `INVALID_ARGUMENT` | Yes | `Writer` shape gate (`ID`/`ExpectedRevision` zero, blank `Actor`); `setActiveRevision`'s own `id == 0 \|\| expectedRevision == 0` guard (`service.go:557-558`) as defense-in-depth | Table test: zero `ID`, zero `ExpectedRevision`, blank `Actor`, each → `INVALID_ARGUMENT`, no bridge call |
| `NOT_FOUND` | Yes | `setActiveRevision`'s `s.repo.Get(ctx, kind, id)` (`service.go:568`) when absent | Inject `domain.ErrCatalogNotFound`-equivalent from the fake `catalogWriter`; assert `NOT_FOUND` |
| `INVALID_CATALOG` | Yes | `domain.WrapInvalidCatalog(next.Validate())` after `ApplyCatalogMutation` (`service.go:584-586`) | Inject the wrapped sentinel from the fake; assert `INVALID_CATALOG` |
| `CONFLICT` | Yes (not on the no-op path) | `casUpdateRevision`-equivalent stale-revision branch inside `repoV2.SetActive`, classified by `classifyV2WriteError` (`service.go:588-590`) | Inject `domain.ErrRevisionConflict` from the fake on a record already in the *opposite* state; assert `CONFLICT` |
| `UNAVAILABLE` | Yes | `ErrCatalogWriterUnavailable`/`ErrCatalogAdminRepositoryV2Unavailable` from `prepareV2Write` (`service.go:564`); ctx cancellation | Inject the unavailable sentinel and a canceled ctx separately; assert `UNAVAILABLE` both times |
| `INTERNAL` | Yes | Any unclassified fake-injected error, no `errors.Is` relationship to a mapped sentinel | Inject a bare `errors.New(...)`; assert `INTERNAL`, no leakage |

Not reachable, same as explore.md: `DUPLICATE`, `INVALID_REFERENCE`, `VALIDATION` (APLICABILIDAD gate is `OpInsert`/`OpUpdate`-only, `catalog_mutation.go:146`), `INTEGRITY` (zero catalog-admin references), `IDENTITY_CONFLICT`/`REACTIVATION_IMPOSSIBLE` (resource-only), `IN_USE`/`IMMUTABLE_CODE` (Delete/Update-only).

### Resource Deactivate — 5 of 15

| Category | Reachable | Evidence | Test approach |
| --- | --- | --- | --- |
| `INVALID_ARGUMENT` | Yes | `Writer` shape gate; `recursos.Service.DeactivateRevision`'s `id <= 0 \|\| expectedRevision == 0` guard (`service.go:234`) | Table test: zero `ID`/`ExpectedRevision`, blank `Actor` |
| `NOT_FOUND` | Yes | `errors.Is(err, domain.ErrResourceNotFound)` pass-through (`service.go:243-245`) | Inject the sentinel from the fake `resourceWriter`; assert `NOT_FOUND` |
| `CONFLICT` | Yes — **CAS-checked even on the no-op** | `errors.Is(err, domain.ErrResourceRevisionConflict)` pass-through (`service.go:243-245`); no app-level short-circuit exists before `v2.DeactivateRevision` is called | Inject `domain.ErrResourceRevisionConflict` on an already-inactive record with a stale `ExpectedRevision`; assert `CONFLICT`, not silent success — this is the asymmetry proof (see below) |
| `UNAVAILABLE` | Yes | Ctx `Canceled`/`DeadlineExceeded` via `core.Map`'s generic arm | Canceled-ctx call; assert `UNAVAILABLE` |
| `INTERNAL` | Yes | `fmt.Errorf("deactivate resource %d: %w", ...)` fallback (`service.go:246`) for any error outside the two explicitly listed | Inject a bare error; assert `INTERNAL`, no leakage |

Not reachable: `DUPLICATE`, `INVALID_REFERENCE` (`active==true`/Reactivate-only), `VALIDATION`, `INTEGRITY` (identity check gated `expectedIdentityKey != ""`, empty on Deactivate), `IDENTITY_CONFLICT` (no such arm on this path).

### Resource Reactivate — 7 of 15 (first to reach IDENTITY_CONFLICT/REACTIVATION_IMPOSSIBLE)

| Category | Reachable | Evidence | Test approach |
| --- | --- | --- | --- |
| `INVALID_ARGUMENT` | Yes | `Writer` shape gate; `id <= 0 \|\| expectedRevision == 0` guard (`service.go:256`) | Table test |
| `NOT_FOUND` | Yes | `errors.Is(err, domain.ErrResourceNotFound)` on the `finder.GetByID` load (`service.go:267-269`) or the CAS `v2.ReactivateRevision` call (`service.go:286-287`) | Inject at either point via the fake; assert `NOT_FOUND` |
| `IDENTITY_CONFLICT` | **Yes — new** | `errors.Is(err, domain.ErrResourceIntegrity)` reclassified (`service.go:288-289`), or `reactivationCandidate`'s own `candidate.IdentityKey != current.IdentityKey` check (`service.go:223-225`) | Two injected paths: (a) fake `GetByID` returns a resource whose rebuilt identity mismatches; (b) fake `ReactivateRevision` returns `domain.ErrResourceIntegrity` |
| `REACTIVATION_IMPOSSIBLE` | **Yes — new** | `domain.WrapReactivationImpossible` on `errors.Is(err, domain.ErrResourceReference)` (`service.go:290-291`), or on `reactivationCandidate`'s `domain.NewResource` failure (`service.go:220-222`) | Two injected paths: (a) fake `GetByID` returns attributes that fail `domain.NewResource` against the current authority; (b) fake `ReactivateRevision` returns `domain.ErrResourceReference` |
| `CONFLICT` | Yes (not on the no-op path) | `errors.Is(err, domain.ErrResourceRevisionConflict)` pass-through (`service.go:286-287`) | Inject on an already-inactive record; assert `CONFLICT` |
| `UNAVAILABLE` | Yes | Ctx cancellation, generic `core.Map` arm | Canceled-ctx call |
| `INTERNAL` | Yes | `fmt.Errorf("reactivate resource %d: %w", ...)` fallback (`service.go:293`) | Inject a bare error |

Not reachable: `DUPLICATE`, `INVALID_REFERENCE` (always reclassified to `REACTIVATION_IMPOSSIBLE`), `VALIDATION` (same reclassification), `INTEGRITY` (always reclassified to `IDENTITY_CONFLICT`).

### Testing the no-op/CAS-bypass asymmetry directly

The proposal's headline behavioral claim needs its own explicit test pair, not just incidental coverage inside the tables above:

- **Catalog Deactivate or Reactivate, no-op case**: call with a record already in the desired `Active` state and a deliberately stale `ExpectedRevision`. Expected: **succeeds silently**, returns the current persisted record unchanged — because `setActiveRevision`'s `if current.Active == active { return nil }` (`service.go:572-574`) runs before `expectedRevision` is ever compared.
- **Resource Reactivate, no-op case**: call with an already-active resource and a stale `ExpectedRevision`. Expected: **succeeds silently**, returns the current resource — `service.go:272-273`'s `if current.Active { return domain.LifecycleResult{Resource: current}, nil }` runs before any CAS call.
- **Resource Deactivate, same call shape**: call with an already-inactive resource and a stale `ExpectedRevision`. Expected: **`CONFLICT`**, not silent success — `DeactivateRevision` has no app-level short-circuit; it delegates straight to `v2.DeactivateRevision`, which checks `currentRevision != expectedRevision` before `currentActive == active` at the postgres layer. This one injected/integration test pair is the proof artifact for the entire documented asymmetry — its assertion is the falsifiable claim, not the readiness prose alone.

All three assertions run at the bridge-injection layer using the fakes in U2/U3 (mirroring the internal behavior exactly, since the bridge adds no logic of its own) and are additionally exercised, gap permitting, against a real `internal/app` service in an integration test — the same disclosed live-PostgreSQL evidence gap the proposal already flags for a sandboxed apply session.

## Files and change surfaces

| Surface | Action | Description |
| --- | --- | --- |
| `resourcecore/write_types.go` | Modify | `CatalogLifecycleRequest`, `ResourceLifecycleRequest` (additive) |
| `resourcecore/writer.go` | Modify | 4 new `Writer` methods, `WriteCapabilities` +4, `validateCatalogLifecycleRequest`, `validateResourceLifecycleRequest` |
| `resourcecore/copy.go` | Unchanged | Scalar-only requests need no new `Clone*`; existing `CloneCatalogRecord`/`CloneResource` reused on return |
| `resourcecore/writer_test.go` | Modify | Shape gate, ID/ExpectedRevision-zero rejection, reflection assertion the compiled surface is exactly 8 methods |
| `resourcecore/external_test.go` | Modify | External consumer exercises all four using only public types |
| `internal/bridge/resourcecore/adapter.go` | Modify | `catalogWriter`/`resourceWriter` +2 methods each, 4 `Adapter` methods (zero new mapping functions) |
| `internal/bridge/resourcecore/adapter_test.go` | Modify | Fakes gain 2 methods each; catalog shared 6-category table; resource Deactivate 5-category and Reactivate 7-category tables; no-op/CAS-bypass asymmetry proof pair |
| `internal/app/*`, `internal/domain/*`, `internal/postgres/*` | Unchanged | Internal authority already built and production-hardened |
| `cmd/garfex/`, `internal/tui/` | Unchanged | Zero `resourcecore` references before and after |
| `openspec/changes/resource-master-core-write-lifecycle/readiness.md` | Create | 6/5/7-category readiness record, confirm-read failure-mode note, asymmetry documentation |

## Compatibility invariant

Unchanged from Create/Update's binding decision: after every unit, `go test ./... -count=1` builds every current adapter, service, composition root, TUI call site, test double, and external test. A unit never adds a public method for an ungraduated operation (`HardDelete`), never changes `core.Record`'s/`core.WithActor`'s/`DiagnosticSink`'s signature, never touches `internal/app`, `internal/domain`, `cmd/garfex`, or `internal/tui`, and never widens a bridge seam without adapting every in-repo fake in the same unit. The READ contract and the compiled Create+Update WRITE surface stay byte-compatible.

## Task-unit breakdown (auto-chain)

Four units, matching explore.md's 650-950-line estimate and Medium-High 400-line budget risk. Three reachability tables (vs Update's two) justify splitting the resource side from the catalog side, unlike Update where one bridge unit covered both operations because there was only one table each.

| Unit | Start state | End state and compatibility proof | Required focused evidence | Forecast |
| --- | --- | --- | --- | --- |
| **U1 — public lifecycle contract** | Update graduation shipped (archived) | Add `CatalogLifecycleRequest`, `ResourceLifecycleRequest`, `Writer.DeactivateCatalog`/`ReactivateCatalog`/`DeactivateResource`/`ReactivateResource`, shape validation. No bridge change — `WriteCapabilities` grows to 8 methods with nothing implementing the new 4 in production yet. No `Clone*` addition (documented decision, not an oversight). | `go test ./resourcecore -run 'TestWriter\|TestExternalWrite\|TestLifecycleRequest' -count=1`; ID/ExpectedRevision-zero → `INVALID_ARGUMENT` table for all 4 methods; reflection assertion the compiled surface is exactly 8 methods, no `HardDelete` stub | ≤250 |
| **U2 — catalog bridge + confirm-read + shared 6-category table** | U1 | Widen `catalogWriter` with `DeactivateRevision`/`ReactivateRevision`, add `Adapter.DeactivateCatalog`/`ReactivateCatalog` with the confirm-read, adapt fakes. Both catalog ops in one unit because they share one table and one confirm-read shape. | `go test ./internal/bridge/resourcecore -run 'TestCatalogLifecycle\|TestCatalogDeactivate\|TestCatalogReactivate' -count=1`; shared 6-category injected table with distinctness/no-leakage assertions; confirm-read failure-mode test (post-success `Get` error passed through unreclassified); catalog no-op/stale-revision proof (silent success) | ≤300 |
| **U3 — resource bridge, both ops, 5- and 7-category tables** | U2 | Widen `resourceWriter` with `DeactivateRevision`/`ReactivateRevision`, add `Adapter.DeactivateResource`/`ReactivateResource`, no confirm-read, adapt fakes. Both resource ops in one unit — no confirm-read shape to isolate, and `IDENTITY_CONFLICT`/`REACTIVATION_IMPOSSIBLE` only exist on the Reactivate side, best proven alongside Deactivate's 5-category table for direct contrast. | `go test ./internal/bridge/resourcecore -run 'TestResourceLifecycle\|TestResourceDeactivate\|TestResourceReactivate' -count=1`; 5-category and 7-category injected tables; explicit mock call-count assertion that neither method calls `resources.Get`/`GetByID` beyond what the internal service itself already performs; resource Deactivate-vs-Reactivate no-op/CAS-bypass contrast pair (the asymmetry's core proof) | ≤300 |
| **U4 — readiness + full-suite verification** | U3 | Write the readiness record (6/5/7-category tables, confirm-read note, asymmetry documentation); no source change beyond the readiness doc. | `go test ./... -count=1`; `rg -l resourcecore cmd/garfex internal/tui` / `git diff --stat -- cmd/garfex internal/tui` both empty; `gofmt -l .`, `go vet ./...`, `golangci-lint run ./...` all clean | ≤100 |

Review-guard forecast: **Decision needed before apply: No** (project's configured `auto-chain` already covers it). **Chained PRs recommended: Yes.** **400-line budget risk: Medium-High**, per explore.md's 650-950-line estimate against 4 units.

Each unit follows strict red-green-refactor with RED, GREEN, TRIANGULATE, and REFACTOR evidence, leaves the tree green. Focused tests run first, then `gofmt -l .`, `go vet ./...`, `golangci-lint run ./...`, `go test ./... -count=1`. Per the `GARFEX_STRICT` focused-test discoverability gate, every new test function name contains a literal alternative from its unit's `-run` pattern as a substring, and each focused command selects at least one test in every package the unit touched.

## Invariants and failure modes

| Invariant / failure | Required behavior |
| --- | --- |
| `ID == 0` (either request) | `INVALID_ARGUMENT`; no capability call |
| `ExpectedRevision == 0` (either request) | `INVALID_ARGUMENT`; no capability call — CAS is mandatory |
| Blank or whitespace `Actor` | `INVALID_ARGUMENT`; no capability call |
| Catalog no-op (already in desired state), stale `ExpectedRevision` | Silent success, returns current record — documented, reused-as-is |
| Resource Reactivate no-op, stale `ExpectedRevision` | Silent success, returns current resource — documented, reused-as-is |
| Resource Deactivate, already-inactive, stale `ExpectedRevision` | `CONFLICT` — no app-level short-circuit; documented asymmetry vs. the two rows above |
| Catalog confirm-read fails after a committed write | Reported honestly via `mapError`, not silently swallowed, no retry — matches `CreateResource`'s established discipline |
| Any internal error | One of the 6 (catalog)/5 (resource Deactivate)/7 (resource Reactivate) proven categories; no pgx/SQLSTATE/constraint/table/column/server text in string, type, or unwrap chain |
| Ungraduated operation (`HardDelete`) | Not exported, not stubbed, not discoverable at runtime |

## Threat matrix

N/A — no routing, shell command, subprocess, VCS/PR automation, executable-file classification, or process-integration boundary. This change adds in-process Go types and one in-process translation extension.

## Migration / rollout

No migration. No schema change, no new column, no composition wiring. `Actor` is never persisted (unchanged). Rolling back any unit reverts `DeactivateCatalog`/`ReactivateCatalog`/`DeactivateResource`/`ReactivateResource` and their bridge translation only, leaving Create, Update, and Read unaffected — no persisted data or internal behavior touched by either the change or its rollback, exactly as the proposal's rollback plan states.

## Alternatives rejected

| Alternative | Why rejected |
| --- | --- |
| Widen `catalogo.Service`'s signatures to return a record instead of confirm-reading | Postgres's `SetActive` builds a deliberately partial 4-field projection even at the domain-port level; widening the service signature would still yield a partial record without a much larger postgres change, and would break `internal/tui/catalog_admin.go`'s `catalogDeactivator`/`catalogReactivator` interfaces |
| Public contract returns no record for lifecycle ops (error/void only) | Inconsistent with every other graduated write returning the persisted projection |
| Add a `Clone*` function for the two lifecycle requests, matching Create/Update's pattern mechanically | Both requests are scalar-only (no map/slice field); Go value semantics already give the caller-mutation-after-call guarantee, and an empty pass-through `Clone*` would be dead code with nothing to copy |
| Fix the idempotent no-op / CAS-bypass asymmetry inside the bridge | Pre-existing, TUI-consumed `internal/app` production behavior; a bridge-only graduation's boundary explicitly excludes correcting shipped internal behavior |
| Give the confirm-read failure its own distinct `ErrorCode` | No such precedent exists in Create/Update's bridge; the confirm-read's own `Get` failure is reported through the same `mapError` path any other `Get` failure would use |

## Open questions

None — all four proposal question-round assumptions (confirm-read failure honesty, no `Changed` field, unchanged `Actor` optionality, unchanged read-side visibility) are load-bearing design decisions already reflected above, pending the proposal's own explicit confirmation gate before `sdd-tasks`.
