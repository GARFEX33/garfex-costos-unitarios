# Catalog Source of Truth

PostgreSQL is the canonical runtime source for catalog *structure* — Clase, Familia, Tipo, Característica, Conjunto de Opciones, Opción, Unidad, Política de Unidad, Aplicabilidad/Identidad, and Presentación. This deliberately **supersedes `recursos-maestro`'s D10 decision** ("Code is the referential-integrity join key only, never rendered — the Go literal stays the runtime source of truth for names/order/aliases/keywords").

## Boundary at a glance

| Concern | Before (D10) | After (this ADR) |
| --- | --- | --- |
| Catalog structure at boot | `domain.NewResourceCatalog()` Go literal | `postgres.LoadResourceCatalog` hydrates `domain.ResourceCatalog` from PostgreSQL |
| PostgreSQL's role for structure | Referential integrity only; Code columns joined by repositories/tests, never rendered | Canonical source; every field (names, order, aliases, keywords) is authoritative |
| Extending the catalog | Requires a Go code change, a build, and a deploy | Staff-authored from the app (`Configuración`); no code change |
| Boot failure surface | Invalid catalog only | DB unreachable / catálogo vacío / catálogo inválido — three distinct Spanish diagnostics |
| Resource *instances* (Material/Mano de obra/Equipo-Herramienta records) | PostgreSQL-backed (`recursos-maestro`) | Unchanged — already canonical before this change |
| `domain.ResourceCatalog` Go type | The in-memory shape `Validate()` and the query API operate on | Unchanged — same type, same validation role |

```text
before (D10):  NewResourceCatalog() [Go literal, runtime source] ──> ResourceCatalog ──> Validate()

after:         PostgreSQL [canonical] ──LoadResourceCatalog──> ResourceCatalog ──> Validate()
                     ^
                     └── staff writes via Configuración (internal/app/catalogo, validate-before-persist)
```

## What changed

PostgreSQL becomes canonical for catalog *structure*: the ~10 administrable concepts named above. `postgres.LoadResourceCatalog` hydrates the existing `domain.ResourceCatalog` at boot, from PostgreSQL, inside one read-only transaction. Admin writes flow through `internal/app/catalogo.Service`, which validates a proposed mutation against `ResourceCatalog.Validate()` **before** persisting anything, then refreshes the in-process snapshot only on success.

`NewResourceCatalog()` is renamed `SeedResourceCatalog()` and demoted to a non-production seed/fixture: it continues to back unit-test fixtures (many tests already depend on it) and an optional dev command, but `cmd/garfex/main.go` never calls it. The canonical initial catalog data is authored as SQL migrations — this was already true as of `migrations/000002_resource_master.up.sql`, which independently seeds content matching `NewResourceCatalog()`'s literal. This change therefore does not require a re-seed migration, only the schema gaps (`active`/audit columns, ordering, multi-rule support, and `garfex_app` write grants) that let that already-seeded data become writable and staff-administrable.

## What did not change

- **`domain.ResourceCatalog` the Go type is unchanged.** It is still what `Validate()` and the query API (`FamiliesFor`, `TypesFor`, `OptionsFor`, `NaturalUnitsFor`, ...) operate on. What changed is *where the values inside it come from* — PostgreSQL via `LoadResourceCatalog`, loaded fresh at boot — not its shape or its role in validation.
- **Resource instances are unaffected.** Actual Material/Mano de obra/Equipo-Herramienta records (`recursos`, `resource_attribute_values`) were already PostgreSQL-backed before this change (`recursos-maestro`). This ADR is scoped to catalog *structure* only — it does not reopen or change how resource instances are stored, searched, or identified.
- **Boot still fails fast.** Crash-fast on startup with no degraded mode and no Go-literal fallback remains the contract; the failure surface simply gains a distinct "catálogo de recursos vacío: ejecuta las migraciones" case, since an empty database is now a different failure mode from a structurally invalid one.

## Rollout

Reversible in one line: `SeedResourceCatalog()` is retained, so reverting `cmd/garfex/main.go`'s boot wiring to call it directly restores pre-ADR behavior exactly. Every schema migration in this change ships a `.down.sql`.

## Current scope

This ADR documents the D10 reversal and its exact boundary. `migrations/000003_catalog_admin.{up,down}.sql` (the schema gaps: `active`/audit columns, class aliases/keywords, display-order columns, the `resource_attribute_rules` table, and `garfex_app` write grants) ships alongside it in the same PR. The loader (`postgres.LoadResourceCatalog`), the descriptor-driven admin engine, and the `Configuración` TUI wiring described above land in later PRs of this change.
