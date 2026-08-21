# Resource Master Core — Public Read Contract

This guide records the decisions reviewers and operators must use when tracing the public `resourcecore` package: what it exposes, what it deliberately does not, and how it stays consistent with `internal/app/catalogo`, `internal/app/recursos`, and PostgreSQL as the only business authorities. It complements [Resource Master Source of Truth](catalog-source-of-truth.md), which covers the internal write/read authority this package delegates to.

## Decisions first

| Concern | Authoritative decision | Concrete boundary |
|---|---|---|
| Ownership | `resourcecore` is a translation boundary, not a second authority. It owns its DTOs and safe errors only; every read delegates to `internal/app/catalogo.Service` and `internal/app/recursos.Service` through a module-owned bridge. | `resourcecore/reader.go`, `internal/bridge/resourcecore/adapter.go` |
| Shipped operations | READ only: active classes, catalog descriptors, catalog list/get, resource search/get, canonical presentation. No `Create`, `Update`, `Deactivate`, `Reactivate`, `Delete`, `Publish`, or `Reload` exists on `Reader`. | `resourcecore/reader.go` |
| Errors | Every failure is a `resourcecore.Error` with one of fifteen stable `ErrorCode` values, mapped from Core through `internal/core.Map`. No `Unwrap`, no `Cause`, no PostgreSQL/driver detail crosses the boundary. | `resourcecore/errors.go`, `internal/core/errors.go` |
| Identity | `Resource.IdentityV1` is durable business identity. Catalog `CatalogRecord.ID`/`CatalogKey.ID` is an opaque, hash-derived reference only. | `resourcecore/types.go`, `internal/domain/resource_validation.go` |
| Freshness | Exactly one authoritative writer process. A `Reader` reflects a coherent snapshot as of its capability's last read; another process needs an explicit reload/restart to observe a write. There is no `Reload` on `Reader`. | `resourcecore/reader.go` |
| Migration 8 | Additive `revision BIGINT NOT NULL DEFAULT 1 CHECK (revision > 0)` on `recursos` and all 11 catalog parent tables, backfilled to 1. Projected as `Revision uint64` wherever it exists; never derived from `updated_at`, `xmin`, or a hash ID. | `migrations/000008_resource_revisions.up.sql` |

## Authority boundaries

```text
external consumer
  -> resourcecore.Reader                        (DTO validation, copy, error translation only)
  -> resourcecore.ReadCapabilities               (public, narrow service-shaped seam)
  -> internal/bridge/resourcecore.Adapter        (module-owned; the only implementation shipped)
  -> internal/app/catalogo.Service / internal/app/recursos.Service
  -> internal/domain -> internal/postgres
```

- `Reader` never imports `internal`; it is constructed with any `ReadCapabilities` implementation, but the only implementation shipped and integration-tested is `internal/bridge/resourcecore.Adapter`.
- The adapter maps every public query to a domain call, translates errors through `internal/core.Map`, and defensively copies data at both edges — it holds no business rule of its own (lifecycle, presentation, dependency, or validation policy stays in `internal/app/catalogo` and `internal/app/recursos`).
- `cmd/garfex/main.go`, the TUI, and the shipped CLI do not construct or wire a `Reader`. There is no concrete consumer yet; a future composition root may instantiate the bridge when one exists.

## Lifecycle semantics through READ

All 11 registered catalog kinds (`CLASE`, `FAMILIA`, `TIPO`, `CARACTERISTICA`, `CONJUNTO_OPCIONES`, `OPCION`, `RELACION_OPCIONES`, `UNIDAD`, `POLITICA_UNIDAD`, `APLICABILIDAD`, `PRESENTACION`) are lifecycle-capable in Core and project uniformly through `CatalogRecord{Active, Revision, Values}`; `APLICABILIDAD` additionally reports its complete ordered `Rules []ApplicabilityRule`. Deactivated records remain readable under an explicit `INACTIVE`/`ALL` lifecycle scope — this package does not distinguish "inactive" from "deleted" on its own; deactivation retains the record, hard delete (not yet public) removes it from every lifecycle-scoped read.

## Errors

| Public category | Reachable from today's READ surface? |
|---|---|
| `INVALID_ARGUMENT` | Yes — request-shape validation (e.g. unknown kind, unsupported `ScopeAll`/`TypeCode` filter on resource search). |
| `NOT_FOUND` | Yes — absent catalog record or resource. |
| `INTEGRITY` | Yes — persisted structure/cardinality inconsistency surfaced while reading. |
| `INVALID_CATALOG` | Yes — the loaded catalog itself is structurally invalid. |
| `UNAVAILABLE` | Yes — context cancellation/deadline or a classified temporary outage. |
| `INTERNAL` | Yes — unclassified unexpected failure. |
| `DUPLICATE`, `INVALID_REFERENCE`, `VALIDATION`, `IDENTITY_CONFLICT`, `INVALID_LIFECYCLE`, `REACTIVATION_IMPOSSIBLE`, `IN_USE`, `IMMUTABLE_CODE`, `CONFLICT` | No — these describe write outcomes. The `ErrorCode` enum already defines all fifteen so a later WRITE change never renumbers a category, but nothing on the shipped read path can currently produce them. |

`resourcecore.Error` never implements `Unwrap`, never exposes a `Cause`, and its `Error()`/`%v`/`%+v` never contain a PostgreSQL message, SQLSTATE, constraint, table, column, or driver type. `internal/core.Map` performs one centralized, exhaustive translation from Core sentinels; the adapter never infers a category from `Error()`, a substring, or a PostgreSQL message.

## Writer topology and freshness

Exactly one process is the designated authoritative writer for Resource Master mutations; this package neither supports nor claims safe independent multi-process writers. A `Reader` reflects a coherent snapshot as of its backing capability's most recent read — there is no `LISTEN`/`NOTIFY`, polling, shared cache, or automatic cross-process refresh. A process other than the writer needs an explicit reconstruct/reload of its bridge, or a restart, to observe a write made elsewhere. `Reader` intentionally has no `Reload` method: publication control is not a consumer capability.

## Migration 8 compatibility

`migrations/000008_resource_revisions` adds an additive, backfilled `revision BIGINT NOT NULL DEFAULT 1 CHECK (revision > 0)` column to `recursos` and the 11 catalog parent tables. It changes no `identity-v1` value and rewrites no existing row. A process running before this package or this migration existed continues to compile and read/write unaffected — the column is additive-only and no trigger derives it from `updated_at`, `xmin`, or a hash-derived catalog ID.

## Absence of public WRITE

There is no `Create`, `Update`, `Deactivate`, `Reactivate`, `Delete`, or `Publish` anywhere in `resourcecore`. Public WRITE is a distinct, later, per-operation change: each operation graduates independently only after it separately proves lifecycle parity across all 11 kinds, atomic `APLICABILIDAD` behavior, revision-based compare-and-swap, guarded hard-delete policy where applicable, commit-before-single-publication behavior, and persistence/authority equivalence, plus the required PostgreSQL and race evidence. Shipping READ never implies any WRITE readiness.

## Operator path: verify

1. Focused: `go test ./resourcecore ./internal/bridge/resourcecore -count=1`.
2. Style: `gofmt -l resourcecore`.
3. Full suite: `go test ./... -count=1`.

Rollback for this documentation slice is limited to `resourcecore/doc.go`, this file, and the Resource Master sections of [catalog-source-of-truth.md](catalog-source-of-truth.md). Reverting it does not alter `resourcecore` behavior, `internal/bridge/resourcecore` behavior, schema, migrations, or PostgreSQL data.
