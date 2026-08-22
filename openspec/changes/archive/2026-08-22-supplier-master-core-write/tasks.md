# Tasks: Public WRITE on the Supplier Master Core — Supplier Create

## Review Workload Forecast

| Field | Value |
|-------|-------|
| Estimated changed lines | 450-560 (combined) |
| 400-line budget risk | High |
| Chained PRs recommended | Yes |
| Suggested split | PR 1 (suppliercore package) → PR 2 (bridge seam + docs) |
| Delivery strategy | ask-on-risk |
| Chain strategy | pending |

Decision needed before apply: Resolved — user chose to split into 2 PRs, stacked-to-main.
Chained PRs recommended: Yes
Chain strategy: stacked-to-main
400-line budget risk: High

### Suggested Work Units

| Unit | Goal | PR | Focused test | Runtime harness | Rollback boundary |
|---|---|---|---|---|---|
| 1 | `SupplierWriteRequest`, `Writer`, `WriteCapabilities`, guards, external proof (~250-300 lines) | 1 | `go test ./suppliercore/... -run Writer` | N/A — library-only | Revert new `suppliercore/*` files; read surface untouched |
| 2 | Bridge `serviceWriter` seam, `Adapter.CreateSupplier`, errors, `doc.go` (~230-280 lines) | 2 | `go test ./internal/bridge/suppliercore/... -run Adapter.*Create` | N/A — library-only | Revert `adapter.go`/`adapter_test.go`/`doc.go`; PR 1 stands alone |

## Phase 1: Public write types & construction (PR 1)

- [x] 1.1 RED→GREEN: `writer_test.go::TestNewWriter_NilCapability_ReturnsInvalidArgument`; add `suppliercore/write_types.go` (`SupplierWriteRequest{Actor,TradeName,LegalName,TaxIdentifier,Website,Notes}`, no `Active`/`Revision`) and `suppliercore/writer.go` (`WriteCapabilities{CreateSupplier}`, `Writer`, `NewWriter` nil-guard) (R1, R2, R4)

## Phase 2: Shape validation & delegation (PR 1)

- [x] 2.1 RED→GREEN: `TestWriter_CreateSupplier_BlankActor_RejectsWithoutDelegating`; `validateSupplierWriteRequest` checks only `Actor` non-blank, no `TrimSpace` on the 5 detail fields (R2)
- [x] 2.2 RED→GREEN: `TestWriter_CreateSupplier_DelegatesAndClonesResult` (fake cap); `Writer.CreateSupplier` delegates and clones via `CloneSupplier` (R1, R6)
- [x] 2.3 RED→GREEN: `TestWriter_CreateSupplier_MutatingRequestAfterCall_NoLeak`; confirm value-typed request already prevents aliasing, no production change (R6)

## Phase 3: Compiled-surface guards (PR 1)

- [x] 3.1 RED→GREEN: `TestWriter_NoUngraduatedMethodExported` mirroring `resourcecore/writer_test.go:216`; confirm `WriteCapabilities.NumMethod()==1`, `Writer` exports only `CreateSupplier` (R5)
- [x] 3.2 RED→GREEN: `TestSupplierWriteRequest_NoReferenceTypedField` (reflect: no ptr/slice/map/chan/func field); confirm against `write_types.go`, no `CloneSupplierWriteRequest` (R4)
- [x] 3.3 RED→GREEN: `TestWriter_NoDeleteOrHardDeleteMethodExported` over `Reader`+`Writer` method sets; confirm no such method package-wide (R7)

## Phase 4: External proof & refactor (PR 1)

- [x] 4.1 RED→GREEN: `external_test.go` — add `fakeWriteCapabilities`, `TestExternalConsumer_CreatesSupplier`; confirm zero `internal` import, `go test ./suppliercore/...` green (R1)
- [x] 4.2 REFACTOR: align `writer.go` doc comments with `resourcecore/writer.go` convention; no behavior change

## Phase 5: Bridge seam widening (PR 2)

- [x] 5.1 RED→GREEN: `adapter_test.go` — `stubService` gains `createSupplier`; `adapter.go` adds `serviceWriter{CreateSupplier}`, combined `supplierService{serviceReader;serviceWriter}`, widens `NewAdapter`, asserts `public.WriteCapabilities` (R1)

## Phase 6: CreateSupplier delegation (PR 2)

- [x] 6.1 RED→GREEN: `TestAdapter_CreateSupplier_MapsAllFieldsAndSetsActor` (5/5 fields + `WithActor`); `Adapter.CreateSupplier` uses `core.WithActor(ctx,req.Actor)`, maps to `domain.SupplierDetails`, delegates, returns `mapSupplier(created)` (R1, R2, R4)

## Phase 7: Error taxonomy coverage (PR 2)

- [x] 7.1 RED→GREEN: `TestAdapter_CreateSupplier_EmptyContent_ReturnsValidation`, `TestAdapter_CreateSupplier_DuplicateTaxIdentifier_ReturnsConflict`, `TestAdapter_CreateSupplier_RawInternalErrorNeverLeaks_ReturnsInternal` (no SQLSTATE/constraint/table/column text); confirm all pass via existing `mapError`, no new branch (design decision 6) (R3, R6)

## Phase 8: Defensive copy & neutral error message (PR 2)

- [x] 8.1 RED→GREEN: `TestAdapter_CreateSupplier_MutatingRequestAfterCall_NoLeak`; confirm value semantics hold, no production change (R6)
- [x] 8.2 RED→GREEN: assert `mapError` default message reads `"supplier master operation failed"`; change the literal in `adapter.go` (R3)

## Phase 9: Docs & final verification (PR 2)

- [x] 9.1 `suppliercore/doc.go` — "READ only" → "read plus Supplier Create"; document `Actor` as diagnostic-only/future audit seed, inherited no-CAS race, permanent `HardDelete` absence, `NOT_FOUND` unreachable from Create (R2, R4, R7)
- [x] 9.2 Verify existing read-surface field tests (Actor/Revision absence) still pass unchanged (R7)
- [x] 9.3 Run `go test ./... -count=1`, `gofmt -l .`, `go vet ./...`, `golangci-lint run ./...`; confirm all green
