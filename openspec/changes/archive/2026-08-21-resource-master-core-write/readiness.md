# Create-WRITE Readiness — `resource-master-core-write`

Parent gate **G1**, recorded after W4. Scope: recalculate and record readiness for the single graduated operation, `Create` (catalog + resource). `Update`, `Deactivate`, `Reactivate`, and catalog `HardDelete` are explicitly **not** applied or planned by this change; each remains a separate, later, per-operation-gated change with its own readiness evidence, per the archived `stabilize-resource-master-core` spec's "Failing evidence withdraws only affected write readiness."

## Verdict

**Create-WRITE is ready**, with one precisely scoped, honestly documented gap: catalog Create's `NOT_FOUND` category is not end-to-end reachable (see below) — this narrows, rather than weakens, the claimed evidence.

## External-package usage

`resourcecore/external_test.go`'s `TestExternalWrite_ConsumerConstructsWriterUsingPublicTypesOnly` constructs a `Writer` via `NewWriter` using a hand-written `WriteCapabilities` fake and only public types, then calls both `CreateCatalog` and `CreateResource` successfully — proving external construction and use with no `internal` import (spec: "Consumer-neutral public Create for catalog and resource").

## Field and query completeness

- **Catalog** (`W2`): `domain.CatalogRecord` has exactly six fields (`Kind`, `ID`, `Revision`, `Active`, `Values`, `Rules`); every one is mapped or explicitly documented as intentionally absent from a create request (`ID`/`Revision` are persistence-assigned). `Kind` maps to the `Create` *argument*, not `rec.Kind` — the real `catalogo.Service.Create` assigns `rec.Kind = kind` internally, confirmed by reading the production code, not assumed (`TestWriteBridge_CatalogCreate_FieldCompleteness`). Value sub-mapping proven for all 7 field kinds (`TestCatalogCreate_ValueMapping_InverseRoundTrip`); rule sub-mapping proven for all 6 `ApplicabilityRule` fields (`TestCatalogCreate_RuleMapping_AllSixFieldsMapped`). `domain.CatalogRef.Label` receives no public source by design (repository-resolved presentation, not a mutation key) — documented at the omission site, not silently dropped.
- **Resource** (`W3`): `domain.CreateCommand` has exactly three fields (`Scope`, `NaturalUnit`, `Attributes`); all three mapped and proven (`TestResourceCreate_FieldCompleteness_AgainstCreateCommand`). Attribute sub-mapping proven for 7 mappable `Value.Kind`s and correctly rejects the 4 unmappable ones (`CODE`, `ENUM`, `STRING_LIST`, `REFERENCE`) as `INVALID_ARGUMENT` without coercion (`TestResourceCreate_AttributeMapping_InverseRoundTripSevenKinds`, `TestResourceCreate_AttributeMapping_RejectsFourUnmappableKinds`).

No public write field is silently accepted and ignored anywhere in this change — the `ResourceQuery.TypeCode` precedent from the READ-ONLY delivery was not repeated.

## Actor attribution without persistence

`resourcecore.CatalogWriteRequest.Actor` and `ResourceWriteRequest.Actor` are required (`Writer`'s shape gate rejects a blank actor with `INVALID_ARGUMENT` before any capability call). The bridge converts `Actor` into request-scoped context metadata via `internal/core.WithActor(ctx, req.Actor)` — never a business-function parameter, never persisted, never exposed on a public DTO. `core.ActorFrom(ctx)` and `core.NewDiagnosticRecord` make the attribution available to any diagnostic emitted under that ctx, including the three existing `core.Record` call sites inside `catalogo.Service` (`service.go:375,380,390`) — with zero change to `internal/app/catalogo` itself, confirmed by reading the byte-identical `core.Record`/`DiagnosticSink` signatures. **Known, documented gap**: three `internal/postgres` call sites (`catalog_admin_repository.go:227,248`, `resource_repository_codec.go:115`) still call `core.Record` with `context.Background()`, so diagnostics emitted from those specific sites do not carry the actor. Fixing this would require touching `internal/postgres`, which this change's binding decisions forbid — recorded here as an INFO-level follow-up for a future change, not silently ignored.

## Create-reachable error category coverage

Both `Adapter.CreateCatalog` and `Adapter.CreateResource` were independently table-tested against 9 injected sentinel families (`TestCatalogCreate_NineCategoryTable_DistinctAndNoLeakage`, `TestResourceCreate_NineCategoryTable_DistinctAndNoLeakage`), each asserting the exact public code, distinctness across all 9, and no leakage of `Error()`/`%v`/`%+v`/concrete type/`errors.Unwrap`/SQLSTATE/constraint/table/column/server text.

| Category | Catalog Create | Resource Create |
|---|---|---|
| `INVALID_ARGUMENT` | Reachable (`Writer` shape gate; `catalogo.ErrInvalidArgument`) | Reachable (`Writer` shape gate; bridge attribute-mapping rejection) |
| `NOT_FOUND` | **Not end-to-end reachable** — see below | Reachable (create-confirm-read `domain.ErrResourceNotFound`) |
| `DUPLICATE` | Reachable (`domain.ErrCatalogDuplicate`) | Reachable (`domain.ErrDuplicateResource`) |
| `INVALID_REFERENCE` | Reachable (`domain.ErrCatalogReference`) | Reachable (`domain.ErrResourceReference`) |
| `VALIDATION` | Reachable (`domain.ErrResourceValidation`) | Reachable (`domain.ErrResourceValidation` from `domain.NewResource`) |
| `INTEGRITY` | Reachable (`domain.ErrResourceIntegrity`) | Reachable (`domain.ErrResourceIntegrity`) |
| `INVALID_CATALOG` | Reachable (`domain.WrapInvalidCatalog(next.Validate())` in `insertLocked`) | Reachable (same wrapper, injected via the seam) |
| `UNAVAILABLE` | Reachable (`catalogo.ErrCatalogWriterUnavailable`, `ErrCatalogAdminRepositoryV2Unavailable`; ctx cancel/deadline) | Reachable (`core.ErrUnavailable`; ctx cancel/deadline) |
| `INTERNAL` | Reachable (unclassified/driver errors) | Reachable (unclassified/driver errors) |

**Catalog `NOT_FOUND` — resolved open question**: read `internal/app/catalogo/service.go`'s `insertLocked` in full. It performs `domain.ApplyCatalogMutation(...OpInsert...)` → `next.Validate()` → `s.repoV2.Insert(ctx, rec)`, with no lookup-by-ID call anywhere in the create path — unlike resource create, which needs a confirm-read because the repository's `Create` returns no identity. There is no code path in catalog `Create`/`insertLocked` that can produce `domain.ErrCatalogRecordNotFound`. **Catalog `NOT_FOUND` is therefore named not-reachable-for-catalog-Create with this reviewed, operation-specific reason**, per the archived spec's allowance for a reviewed non-reachability declaration rather than letting an unreachable category silently fall through to `INTERNAL`. The bridge-level table test still proves `mapError` correctly classifies `ErrCatalogRecordNotFound` *if* injected — the finding is that production `Create` never triggers it, not that the mapping is untested.

The six categories genuinely unreachable from Create for both domains — `CONFLICT`, `IDENTITY_CONFLICT`, `INVALID_LIFECYCLE`, `REACTIVATION_IMPOSSIBLE`, `IN_USE`, `IMMUTABLE_CODE` — stay unproven, as designed; none is claimed reachable anywhere in this change's evidence.

## Defensive copies

`resourcecore.Writer.CreateCatalog`/`CreateResource` deep-copy the incoming request (`CloneCatalogWriteRequest`/`CloneResourceWriteRequest`, preserving nil-vs-non-nil-empty `Rules`) before calling the capability, and deep-copy the returned record (`CloneCatalogRecord`/`CloneResource`) before returning it. `TestWriteRequestCopy_CallerMutationAfterCall_NoEffect` proves a caller mutating either side after the call changes nothing in flight or already-submitted.

## Compiled surface limited to graduated Create

`TestWriter_NoUngraduatedMethodExported` (reflection over both `WriteCapabilities` and `Writer`) proves exactly two methods exist — `CreateCatalog`, `CreateResource` — with no `Update`/`Deactivate`/`Reactivate`/`HardDelete` symbol, stub, or "unimplemented" branch anywhere in `resourcecore`.

**Zero-touch confirmation for the driving adapters** (textual reference scan, run at W4):

```
$ rg -l "resourcecore" cmd/garfex/*.go internal/tui/*.go
(no output)

$ git diff --stat -- cmd/garfex internal/tui
(no output)
```

Zero `resourcecore` references and zero changed lines in `cmd/garfex/` and `internal/tui/`, confirmed directly, not assumed — matching this change's binding decision that those packages stay untouched pending their own, separate, later deletion.

## Focused receipts

| Unit | Command | Result |
|---|---|---|
| W1 | `go test ./resourcecore -run 'TestWriter\|TestExternalWrite\|TestWriteRequestCopy' -count=1` | Pass |
| W2 | `go test ./internal/core ./internal/bridge/resourcecore -run 'TestWriteBridge\|TestActor\|TestCatalogCreate' -count=1` | Pass (one test-authoring correction made and re-verified within the same GREEN pass; see `apply-progress.md`) |
| W3 | `go test ./internal/bridge/resourcecore ./resourcecore -run 'TestWriteBridge\|TestResourceCreate\|TestExternalWrite' -count=1` | Pass, first attempt |
| W4 (this gate) | `rg -l resourcecore cmd/garfex internal/tui` / `git diff --stat -- cmd/garfex internal/tui` | Both empty, confirming zero-touch |

## Full-suite proof

`go test ./... -count=1`, re-run after every unit (W1, W2, W3, and again for this gate): every package this change touches is green — `resourcecore`, `internal/bridge/resourcecore`, `internal/core`, and (unaffected, re-confirmed) `internal/domain`, `internal/app/catalogo`, `internal/app/recursos`, `internal/postgres`, `cmd/garfex`, `internal/tui`. One failure persists across all four runs, pre-existing and outside this change's scope: `agent/skills/golang-cli/assets/examples` (untracked skill scaffold missing example dependencies, first observed in `stabilize-resource-master-core`, not in git, never reaches real CI).

## vet / lint / build

- `go vet ./resourcecore/... ./internal/bridge/resourcecore/... ./internal/core/...` — clean, every unit.
- `golangci-lint run ./resourcecore/... ./internal/bridge/resourcecore/... ./internal/core/...` — `0 issues.`, every unit.
- `gofmt -l` — clean, every unit.
- Local `go build`/`docker build`/race are CI-only per `openspec/config.yaml`'s testing policy and were not run locally, as instructed by `tasks.md`'s global contract (no migration or PostgreSQL SQL in this change; bridge-layer evidence uses in-repo fakes).

## Deviations

1. **Catalog Create `NOT_FOUND` not end-to-end reachable** — see above. Not a defect; a correctly scoped finding that narrows the claimed evidence rather than overclaiming it.
2. **`internal/postgres`'s three `context.Background()` diagnostic sites lose actor attribution** — documented INFO-level gap, out of this change's forbidden-to-touch scope.
3. **Redundant `AttributeValue.UnitCode` (top-level) left unvalidated and unread** on the write path — only the nested `AttributeValue.Value.UnitCode` is validated (W1) and mapped (W3), consistent with design.md's shape-gate list, which speaks only of a `Value`'s `UnitCode`.
4. **One test-authoring bug caught and fixed within W2's own GREEN pass** (not a production defect) — see `apply-progress.md`'s W2 entry for detail.

## Freshness and topology

Unaffected by this change: no migration, no schema change, no composition wiring, one authoritative writer topology unchanged. The one behavioral addition — `Adapter.CreateResource`'s create-confirm read — is non-transactional and safe only under the already-approved one-writer topology (no concurrent writer can intervene between the internal `Create` and the confirm `Get`); this assumption is inherited from the archived change's writer-topology decision, not newly introduced.

## Rollback notes

Reverting Create-WRITE readiness removes only `resourcecore/write_types.go`, `resourcecore/writer.go`, the `CloneCatalogWriteRequest`/`CloneResourceWriteRequest` additions to `resourcecore/copy.go`, the write-related additions to `resourcecore/writer_test.go`/`external_test.go`, the additive `internal/core/diagnostics.go` symbols, and the `catalogWriter`/`catalogPort`/`resourceWriter`/`resourcePort`/`CreateCatalog`/`CreateResource`/inverse-mapper additions in `internal/bridge/resourcecore/adapter.go` (plus their tests). The READ-ONLY contract is untouched and stays fully available; no database, migration, or composition rollback is needed, since nothing new was persisted or wired.

## What this gate does not do

Per its exact scope, this gate does not apply, plan, or evaluate `Update`, `Deactivate`, `Reactivate`, or catalog `HardDelete`. Each remains a separate, later, per-operation-gated change with its own readiness evaluation, per the archived spec's "Requirement: Separate read and per-operation write readiness." This gate also does not evaluate or authorize deleting `cmd/garfex/`/`internal/tui/` — it only confirms the technical precondition (public WRITE existing) is now partially satisfied for `Create`; the deletion itself waits for every operation this change's successors will graduate.
