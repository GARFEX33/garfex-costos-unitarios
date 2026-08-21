# READ-ONLY Readiness — `stabilize-resource-master-core`

Parent gate **P6**, recorded after 6C. Scope: recalculate and record READ-ONLY readiness only. Public WRITE is explicitly **not** applied or planned by this change; it remains a separate, later, per-operation-gated decision (`proposal.md` §"Delivery order", `design.md` §"READ and per-operation WRITE readiness gates").

## Verdict

**READ-ONLY is ready.** Every scenario in `specs/resource-master-core/spec.md` that governs the read-only surface is satisfied by the shipped `resourcecore` package and its `internal/bridge/resourcecore` adapter, with focused and full-suite evidence recorded below. No WRITE symbol exists anywhere in the public package.

## External-package usage

- `resourcecore/external_test.go` (package `resourcecore_test`) constructs a `Reader` via `NewReadOnly` using a hand-written service-shaped fake and only public types — `TestExternal_ConstructsReaderWithOnlyPublicTypes`.
- `TestExternal_NoInternalImports` independently asserts the external test file itself imports no `internal/...` path.
- The only implementation shipped and integration-tested is the module-owned `internal/bridge/resourcecore.Adapter`, proven against the real `internal/app/catalogo.Service`-shaped and `internal/app/recursos.Service`-shaped seams in `internal/bridge/resourcecore/adapter_test.go`. `go test ./resourcecore ./internal/bridge/resourcecore -count=1` runs and passes all 37 top-level test functions across both packages (23 in `resourcecore`, 14 in `internal/bridge/resourcecore`).
- No unused `Reader` is wired into `cmd/garfex/main.go`, the TUI, or the shipped CLI — confirmed by `design.md`'s explicit non-goal and by the 6B/6C receipts recording no such file was touched.

## Projections

All 11 registered kinds (`CLASE`, `FAMILIA`, `TIPO`, `CARACTERISTICA`, `CONJUNTO_OPCIONES`, `OPCION`, `RELACION_OPCIONES`, `UNIDAD`, `POLITICA_UNIDAD`, `APLICABILIDAD`, `PRESENTACION`) project through the same generic `CatalogDescriptor`/`CatalogRecord` DTOs — no per-kind public struct. `mapCatalogDescriptor` preserves `Fields`, `RefKind`, `RefScopedBy`, `EnumValues`, and `IdentityFields` (`TestAdapter_CatalogDescriptorsIncludesRefScopedByAndEnumValues`, fixed in the 6B correction). `mapCatalogRecord` preserves `Active`, `Revision`, `Values`, and `APLICABILIDAD`'s `Rules` (`TestAdapter_CatalogRecordIncludesRules`, fixed in the 6B correction). Resource projection preserves `ID`, `IdentityV1`, `Scope{ClassCode,FamilyCode,TypeCode}`, `NaturalUnit`, `Active`, `Revision`, and typed `Attributes` (`TestAdapter_ResourceValueProjection`, `TestAdapter_GetResource`).

## Copies

The defensive-copy invariant (`design.md` §"Defensive-copy invariant") is proven at both edges: `resourcecore/copy.go`'s `CloneCatalogRecord`, `CloneCatalogDescriptor`, and `CloneResource` deep-copy every mutable collection including `Rules`, `RefScopedBy`, and `EnumValues`; `Reader` calls these clone helpers on every return path (`reader.go`); caller-side mutation after a call cannot alter accepted or returned state (`TestAdapter_DeepCopyIncomingQuery`, `TestReader_*CopiesResult` family in `reader_test.go`).

## Values

Canonical base-10 numeric strings (no exponent, no redundant zeroes, negative zero normalized) are produced by `resourcecore/values.go` and covered by `resourcecore/values_test.go`. `NOT_APPLICABLE` is its own `ValueKind`, distinct from empty/zero/null (`TestAdapter_ResourceValueProjection`'s `text` case, `values_test.go`).

## Queries

`CatalogQuery`/`ResourceQuery` carry explicit lifecycle scope, text/filter fields, `Limit`, `Offset`; pages normalize `Limit`/`Offset` and report `HasPrevious`/`HasNext` (`TestAdapter_ListCatalogPagination`, `TestAdapter_SearchResources`). `ResourceQuery.Scope == ScopeAll` and any non-empty `ResourceQuery.TypeCode` are explicitly rejected with `INVALID_ARGUMENT` rather than silently ignored — the second of these was a 6B correction (`TestAdapter_SearchResourcesAllScopeNotSupported`, `TestAdapter_SearchResourcesTypeCodeNotSupported`); `TypeCode` filtering has no support anywhere in `domain.SearchCriteria` today, so the public contract does not promise it.

## Canonical presentation

`DescribeResource` delegates to `internal/app/recursos.Service.Describe` and never reconstructs presentation independently (`TestAdapter_DescribeResourceDelegates` asserts the adapter's fetched resource, not a re-derived one, reaches `Describe`).

## Safe read errors

All 15 `resourcecore.ErrorCode` categories are defined (`errors.go`), matching `design.md`'s complete public error model exactly. `resourcecore.Error` has no `Unwrap`, no `Cause`, and formats only its stable message (`TestAdapter_MapsErrorsThroughNeutralBoundary` asserts `errors.Unwrap(publicErr) == nil` for every case). Only a subset is reachable from the shipped READ surface today, verified rather than assumed (see `docs/architecture/resource-master-core.md`'s error-reachability table):

| Reachable today | Not reachable from READ (write-only outcomes) |
|---|---|
| `INVALID_ARGUMENT`, `NOT_FOUND`, `INTEGRITY`, `INVALID_CATALOG`, `UNAVAILABLE`, `INTERNAL` | `DUPLICATE`, `INVALID_REFERENCE`, `VALIDATION`, `IDENTITY_CONFLICT`, `INVALID_LIFECYCLE`, `REACTIVATION_IMPOSSIBLE`, `IN_USE`, `IMMUTABLE_CODE`, `CONFLICT` |

`internal/core.Map` performs one centralized, exhaustive translation (verified present and used by the adapter); no leakage of SQLSTATE, constraint, table, column, or driver type crosses the boundary in any adapter test.

## API absence of WRITE

Grepped `resourcecore/reader.go` and `resourcecore/types.go`: `ReadCapabilities` and `Reader` expose exactly `ActiveClasses`, `CatalogDescriptors`, `ListCatalog`, `GetCatalog`, `SearchResources`, `GetResource`, `DescribeResource`. No `Create`, `Update`, `Deactivate`, `Reactivate`, `Delete`, `Publish`, or `Reload` exists on any public type, matching `spec.md`'s "Read-only authority cannot mutate state" scenario.

## Focused receipts

| Unit | Command | Result |
|---|---|---|
| 6A | `go test ./resourcecore -run 'Test(Canonical\|Value\|NotApplicable\|Query\|Page\|Defensive\|PublicError)' -count=1` | Pass |
| 6B (original) | `go test ./resourcecore ./internal/bridge/resourcecore -count=1` | Pass |
| 6B (correction) | `go test ./internal/bridge/resourcecore -run 'TestAdapter_(SearchResourcesTypeCodeNotSupported\|CatalogDescriptorsIncludesRefScopedByAndEnumValues\|CatalogRecordIncludesRules)' -v -count=1` | RED→GREEN, all pass |
| 6B (corrected focused check) | `go test ./resourcecore ./internal/bridge/resourcecore -run 'Test(Adapter\|Reader\|External\|Bridge\|Read\|Description\|Error\|API)' -count=1` | Pass, 27/27 regex-matched tests (previously discovered 0 bridge tests) |
| P6 (unfiltered, this gate) | `go test ./resourcecore ./internal/bridge/resourcecore -count=1` | Pass, all 37 top-level test functions across both packages (23 `resourcecore`, 14 `internal/bridge/resourcecore`) |
| 6C | `gofmt -l resourcecore` / `go vet ./resourcecore ./internal/bridge/resourcecore` / `go test ./resourcecore ./internal/bridge/resourcecore -count=1` | Clean / Clean / Pass |
| P6 (this gate) | `golangci-lint run ./resourcecore/... ./internal/bridge/resourcecore/...` | `0 issues.` |

## Full-suite proof

`go test ./... -count=1`, run at the end of 6B, the 6B correction, 6C, and again for this P6 gate: every package this change touches is green — `resourcecore`, `internal/bridge/resourcecore`, `internal/domain`, `internal/app/catalogo`, `internal/app/recursos`, `internal/postgres`, `internal/core`, `internal/config`, `migrations`. Two failures persist across all four runs, both pre-existing, untracked, and outside this change's scope (confirmed via `git ls-files` — neither file is tracked, neither belongs to `stabilize-resource-master-core`):

1. `agent/skills/golang-cli/assets/examples` — skill scaffold missing example dependencies (`github.com/fatih/color`, `github.com/fsnotify/fsnotify`, `github.com/spf13/cobra`, `github.com/spf13/viper`, `github.com/you/myapp/cmd`), first observed at 6A.
2. `internal/tui` / `cmd/garfex` — build failure on untracked `internal/tui/suppliers_admin.go` (undefined `ChildLifecycleFrame`, `supplierKeyDelta`, `supplierPrintableText`, `SupplierModel`), work-in-progress from an unrelated `proveedores-maestro-ux` branch/stash, first observed at 6A.

Neither failure touches, imports, or is imported by `resourcecore` or `internal/bridge/resourcecore`.

## vet / lint / race / build CI status

- `go vet ./resourcecore ./internal/bridge/resourcecore` — clean (6C, reconfirmed at P6).
- `golangci-lint run ./resourcecore/... ./internal/bridge/resourcecore/...` — `0 issues.` (P6).
- `gofmt -l resourcecore` — clean (6C, reconfirmed at P6).
- Race (`go test ./... -race -count=1`) and full local build (`go build ./...`) are CI-only per project policy (`design.md` §"TDD and ordered auto-chain": "CI-only build and race checks are reported separately") and were not run locally for this gate; no local race or build receipt is claimed here.

## Deviations

1. **Public `Reference` gained `Code string`** (6B) — `design.md`'s sketch shows `Reference{Kind, ID}`; domain catalog references are natural-code-based, so `Code` was added to preserve them across the boundary per the spec's "preserve its public values, references" requirement. `ID` remains for future ID-bearing references. No spec scenario regressed.
2. **`ResourceQuery.Scope == ScopeAll` rejected** (6B) — `domain.SearchCriteria` supports only active/inactive resource lifecycle scope; catalog `ScopeAll` is supported and unaffected.
3. **`ResourceQuery.TypeCode` rejected when non-empty** (6B correction) — `domain.SearchCriteria` has no `TypeCode` field anywhere in the stack; rather than silently drop the filter (the original defect), the adapter now returns `INVALID_ARGUMENT`. Extending Core to support it is out of this change's scope and would require a design amendment.
4. **6B authored delta exceeded its 400-line forecast** (1,348 vs. 400) — recorded as `size:exception` in the original 6B receipt; not revisited by this gate, which only recalculates READ *behavioral* readiness, not review-budget bookkeeping.

## Freshness limits

Exactly one process is the authoritative writer (`design.md` §"Writer topology and freshness"). `Reader` has no `Reload` method — confirmed absent by inspection of `reader.go`'s full method set. A process other than the writer must explicitly reconstruct its bridge or restart to observe a write made elsewhere; no LISTEN/NOTIFY, polling, or live cross-process coherence is implemented, matching `spec.md`'s "Another process remains unchanged" scenario exactly.

## Rollback notes

READ-ONLY readiness can be withdrawn without touching write-path code: revert the `resourcecore/`, `internal/bridge/resourcecore/`, and the two 6C documentation files independently of Stages 1–5, since no production write composition or TUI file was ever touched by Stage 6 (`design.md` §"Rollback boundaries": "READ-ONLY façade slices can remain available if later lifecycle or WRITE slices are rolled back" — the converse also holds here, nothing downstream of Stage 6 depends on it).

## What this gate does not do

Per its exact scope, this gate does not apply, plan, or evaluate public WRITE. Stages 1–5 (mutation correctness, all-11 lifecycle, PostgreSQL, authority equivalence, neutral errors) were separately gated at P1–P5 and are prerequisites already satisfied, not re-verified here. A future, separate change would need its own readiness evaluation per operation before exposing any WRITE method, per `spec.md`'s "Requirement: Separate read and per-operation write readiness".
