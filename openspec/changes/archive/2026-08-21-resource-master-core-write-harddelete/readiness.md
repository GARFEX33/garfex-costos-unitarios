# Readiness: resource-master-core-write-harddelete

## Verdict

**Ready.** `HardDeleteCatalog` graduates as the 9th public `WriteCapabilities`/`Writer` method. `HardDeleteResource` remains absent, unstubbed, and undiscoverable — a deliberate, user-confirmed, documented gap, not addressed by this change.

## Units completed

- **U1 — Public HardDeleteCatalog contract.** `resourcecore/writer.go` (`WriteCapabilities` 8→9 methods, `Writer.HardDeleteCatalog`), `resourcecore/write_types.go` (doc comment widened), `resourcecore/writer_test.go` (`TestWriter_NoUngraduatedMethodExported` updated to 9 methods; `TestWriter_HardDeleteCatalog_ShapeValidation` + `TestWriter_HardDeleteCatalog_ShapeValidation_NoCapabilityCallOnInvalid` added), `resourcecore/external_test.go` (`externalFakeWriteCapabilities.HardDeleteCatalog` + `TestExternalWrite_ConsumerHardDeletesInactiveUnreferencedCatalog` added).
- **U2 — Bridge translation, no confirm-read, 8-category table, non-duplication proof.** `internal/bridge/resourcecore/adapter.go` (`catalogWriter` +`HardDeleteRevision`, `Adapter.HardDeleteCatalog` — 2-line pass-through), `internal/bridge/resourcecore/adapter_test.go` (fake `hardDeleteRevision`/`hardDeleteCalls`/`listCalls` added; 8 new test functions, see below).
- **U3 — This readiness record.** No source change.

## 8-category error reachability

| Category | Evidence | First-ever reachable |
| --- | --- | --- |
| `INVALID_ARGUMENT` | `TestHardDeleteCatalog_EightCategoryTable_DistinctAndNoLeakage/invalid_argument`; also `Writer` shape gate (`TestWriter_HardDeleteCatalog_ShapeValidation`) | No |
| `NOT_FOUND` | `TestHardDeleteCatalog_EightCategoryTable_DistinctAndNoLeakage/not_found` | No |
| `INVALID_CATALOG` | `TestHardDeleteCatalog_EightCategoryTable_DistinctAndNoLeakage/invalid_catalog` | No |
| `INVALID_LIFECYCLE` | `TestHardDeleteCatalog_EightCategoryTable_DistinctAndNoLeakage/invalid_lifecycle` | **Yes** |
| `IN_USE` | `TestHardDeleteCatalog_EightCategoryTable_DistinctAndNoLeakage/in_use` | **Yes** |
| `CONFLICT` | `TestHardDeleteCatalog_EightCategoryTable_DistinctAndNoLeakage/conflict`; also `TestHardDeleteCatalog_StaleExpectedRevision_AlwaysConflict_NoBypass` | No |
| `UNAVAILABLE` | `TestHardDeleteCatalog_EightCategoryTable_DistinctAndNoLeakage/unavailable` | No |
| `INTERNAL` | `TestHardDeleteCatalog_EightCategoryTable_DistinctAndNoLeakage/internal`, no pgx/SQLSTATE/constraint/table/column/server-text leakage asserted | No |

Explicit exclusion list, each proven to still map to its own distinct category rather than being silently collapsed (`TestHardDeleteCatalog_EightCategoryTable_ExcludesUnreachable`): `DUPLICATE`, `INVALID_REFERENCE`, `VALIDATION`, `INTEGRITY`, `IDENTITY_CONFLICT`, `REACTIVATION_IMPOSSIBLE`, `IMMUTABLE_CODE`.

Evidence pointers into the internal guard chain (design.md, verified against `internal/app/catalogo/service.go`): `buildV2DeleteCandidate` (446-483) — active-target rejection (456-458) → `INVALID_LIFECYCLE`; `Dependents`/`ReferencedByResources` rejection (463-474) → `IN_USE`; `next.Validate()` (479-481) → `INVALID_CATALOG`. `internal/core/errors.go` required zero changes — `ErrInvalidLifecycle`→`InvalidLifecycle` (80-81) and `ErrCatalogInUse`→`InUse` (84-85) were already mapped.

## Guard-chain non-duplication proof

`TestHardDeleteCatalog_NoGuardChainDuplication_OneSeamCallZeroReads`: a fake `catalogWriter` returns success from `HardDeleteRevision` while its `Get`/`List` projections would report the target as active; the bridge still succeeds, with exactly one seam call (`hardDeleteCalls == 1`) and zero `Get`/`List` calls (`getCalls == 0`, `listCalls == 0`). Proves the bridge cannot have re-implemented the lifecycle/dependents/reference checks — it structurally cannot evaluate `Dependents`/`ReferencedByResources` at all, since neither is on any seam the `Adapter` holds.

## No-confirm-read proof

`TestHardDeleteCatalog_NoConfirmReadAfterSuccess`: after a successful fake `HardDeleteRevision` call, `getCalls == 0`. Confirms the operative contrast with `DeactivateCatalog`/`ReactivateCatalog`, which both issue a confirm-read — `HardDeleteRevision` returns only `error`, and the deleted record no longer exists to read back.

## Strict-CAS-no-bypass proof

`TestHardDeleteCatalog_StaleExpectedRevision_AlwaysConflict_NoBypass`: a stale `ExpectedRevision` yields `CONFLICT` unconditionally. Contrast with the lifecycle precedent: `TestCatalogDeactivate_NoOp_StaleRevision_SilentSuccess`/`TestCatalogReactivate_NoOp_StaleRevision_SilentSuccess` (pre-existing) prove `DeactivateCatalog`/`ReactivateCatalog` silently succeed on a stale revision when the target is already in the requested state. `HardDeleteCatalog` has no such idempotent target — a deleted record is no longer addressable by `Kind`+`ID`.

## Actor attribution

`TestHardDeleteCatalog_ActorReachesDiagnosticSeam`: `Actor` reaches the internal diagnostic seam via `core.WithActor`/`core.ActorFrom`, on the failure path (mirrors the exact mechanism used by Create/Update/Deactivate/Reactivate). `Actor` is not persisted and appears on no public DTO beyond the originating request.

## Zero-touch confirmation

- `rg -l resourcecore cmd/garfex internal/tui` — no matches.
- `git diff --stat -- cmd/garfex internal/tui` — no changes.
- `internal/app/*`, `internal/domain/*`, `internal/postgres/*` — zero changed lines (confirmed via `git status`/`git diff --stat` across the whole chain).
- `internal/core/errors.go` — zero changed lines.

## Verification commands and results

- `go test ./resourcecore -run 'TestWriter|TestExternalWrite' -count=1` — PASS (all subtests).
- `go test ./internal/bridge/resourcecore -run 'TestHardDeleteCatalog|TestWriteBridge_CatalogWriter_HardDeleteRevisionFakeAdapted' -count=1` — PASS (all subtests).
- `go test ./... -count=1` — PASS across every package except the pre-existing, unrelated `agent/skills/golang-cli/assets/examples` (missing third-party deps for unrelated CLI-skill example code; not touched by this change, same disclosed gap noted by the three prior graduations' verify passes).
- `gofmt -l .` — clean (no output).
- `go vet ./...` — clean except the same pre-existing unrelated package.
- `golangci-lint run ./...` — clean except the same pre-existing unrelated package (5 typecheck issues, all missing-dependency errors in that one package).

## Disclosed evidence gap

Live-PostgreSQL integration evidence for `INVALID_LIFECYCLE`/`IN_USE` reachability was not executed in this sandboxed apply session — matching the disclosed gap in the three prior graduations. Bridge-injection tests (`TestHardDeleteCatalog_EightCategoryTable_DistinctAndNoLeakage`) are the fallback evidence, exercising `mapError`/`core.Map` against injected `domain.ErrInvalidLifecycle`/`domain.ErrCatalogInUse` directly.

## Compiled surface (G1 criterion g)

`WriteCapabilities`/`Writer` now declare exactly: `CreateCatalog`, `CreateResource`, `UpdateCatalog`, `UpdateResource`, `DeactivateCatalog`, `ReactivateCatalog`, `DeactivateResource`, `ReactivateResource`, `HardDeleteCatalog` — 9 methods, confirmed by `TestWriter_NoUngraduatedMethodExported`'s reflection assertion (`NumMethod() == 9`). No `HardDeleteResource` stub, symbol, or discoverable artifact exists anywhere in the repository.

## Scope boundary (carried from proposal, user-confirmed)

This change completes the **catalog** write contract only. `HardDeleteResource` remains a deliberate, documented, permanently out-of-scope gap unless a future change explicitly scopes it with its own exploration — no domain interface, repository implementation, or guard-chain design exists for it today.
