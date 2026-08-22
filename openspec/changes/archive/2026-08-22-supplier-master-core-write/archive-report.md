# Archive Report: supplier-master-core-write

**Change**: supplier-master-core-write  
**Status**: ARCHIVED  
**Archive date**: 2026-08-22  
**Archived to**: `openspec/changes/archive/2026-08-22-supplier-master-core-write/`

## Executive Summary

The `supplier-master-core-write` change has been fully implemented, verified, and archived. Both implementation PRs (PR #173 and PR #174) have been merged to `main`, tracking issue #172 is closed, all 22 tasks are complete, and sdd-verify returned PASS with zero CRITICAL findings. The public write contract for Supplier Create has been shipped with full spec compliance (7 requirements, 11 scenarios).

## What Shipped

### Requirements Implemented

**ADDED Requirements** (6):
1. **Public write contract — Supplier Create only**: Exported `suppliercore.Writer` over `WriteCapabilities` port with exactly one method (`CreateSupplier`). Nil-capability guard returns `INVALID_ARGUMENT`.
2. **Shape validation is Actor-only; content rules stay in the domain**: Request validation checks only `Actor` non-blank; all content rules remain `domain.NewSupplier`'s authority.
3. **Create-reachable error taxonomy, no new codes**: Four error categories reachable from Create (`INVALID_ARGUMENT`, `VALIDATION`, `CONFLICT`, `INTERNAL`); `NOT_FOUND` documented as unreachable.
4. **No CAS; Actor is diagnostic-only and a documented future audit seed**: No `Revision` field; `Actor` required at shape, passed via `core.WithActor`, never persisted, documented as future audit-data seed.
5. **Compiled write surface exports no ungraduated method**: Reflection guard (`TestWriter_NoUngraduatedMethodExported`) confirms `WriteCapabilities.NumMethod() == 1`, no Update/Deactivate/Reactivate/HardDelete stubs.
6. **No error leakage; defensive copying on the write path**: Raw PostgreSQL errors sanitized; inbound request and outbound `Supplier` defensively copied.

**MODIFIED Requirements** (1):
- **No Actor or Revision on the read surface; HardDelete stays permanently absent everywhere**: Narrowed from prior read-only requirement to clarify that reads prohibit Actor/Revision while HardDelete remains absent everywhere (read and write), and write surface legitimately carries `Actor`.

### Specification Updated

Main spec at `openspec/specs/supplier-master-core/spec.md` merged successfully:
- 6 new requirements appended
- 1 requirement refined (narrowed scope to distinguish read vs write Actor handling)
- All 10 original read requirements preserved unchanged
- Total: 17 requirements covering read + write + shared constraints

### Code Changes Implemented

**PR #173** (commit b971555, "feat(suppliercore): add the public CreateSupplier write contract"):
- `suppliercore/write_types.go` (new): `SupplierWriteRequest` with Actor and five `domain.SupplierDetails` fields; no `Active` or `Revision`.
- `suppliercore/writer.go` (new): `Writer`, `WriteCapabilities`, `NewWriter` nil guard, shape-validation.
- `suppliercore/writer_test.go` (new): Nil-guard, delegation, cloning, reflection guard, value-typed-field guard tests.
- `suppliercore/external_test.go` (modified): Added `TestExternalConsumer_CreatesSupplier` (package `suppliercore_test`, zero `internal` imports).
- `suppliercore/doc.go` (modified): "READ only" → "read plus Supplier Create"; Actor as diagnostic-only future audit seed; inherited lost-update race; HardDelete permanently absent; `NOT_FOUND` unreachable from Create.
- **232 lines** of public contract, external proof, and guards.

**PR #174** (commit 83f1e7a, "feat(bridge): graduate CreateSupplier through the write bridge"):
- `internal/bridge/suppliercore/adapter.go` (modified): `serviceWriter` seam (one method), combined `supplierService{serviceReader;serviceWriter}`, `Adapter.CreateSupplier`, neutral `mapError` default message.
- `internal/bridge/suppliercore/adapter_test.go` (modified): Fake gains `CreateSupplier`; add create mapping, field-completeness (5/5 `SupplierDetails`), and error-leakage tests.
- `internal/bridge/suppliercore/doc.go` (not present; reflects unchanged `suppliercore/doc.go` from PR 1).
- **190 lines** of bridge seam, delegation, error mapping, and comprehensive test coverage.

**Total author lines**: ~422 (both PRs combined), within budget after stacked delivery.

**Files unchanged**: `suppliercore/copy.go`, `suppliercore/errors.go`, `internal/modules/suppliers/**`, `internal/core/**` (authority already existed; no new behavior invented).

### Tracking Issue Resolution

**Issue #172** (`status:approved`): Closed by both merged PRs.
- Scope: Public write contract for Supplier Create.
- Resolved by: PR #173 + PR #174, both merged to `main`.

## Verification Summary

**Verification command**: `sdd-verify` (post-merge)  
**Verdict**: **PASS**  
**Blockers**: 0 CRITICAL  
**Warnings**: 3 (non-blocking)  
**Suggestions**: 1 (improvement, not required for archive)

### Build & Test Results

| Check | Status | Details |
|-------|--------|---------|
| `go build ./suppliercore/... ./internal/...` | ✅ PASS | Exit 0, no output |
| `go vet ./suppliercore/... ./internal/...` | ✅ PASS | Exit 0, no output |
| `gofmt -l ./suppliercore ./internal` | ✅ CLEAN | Exit 0, no files listed |
| `golangci-lint run ./suppliercore/... ./internal/bridge/suppliercore/...` | ✅ PASS | `0 issues` |
| `go test ./suppliercore/... ./internal/... -count=1` | ✅ 100% PASS | 10/10 packages green, 0 failed, 0 skipped in changed scope |

### Spec Compliance

All 7 requirements (6 ADDED + 1 MODIFIED) and 11 scenarios are **compliant**:

| Requirement | Scenarios | Evidence | Status |
|-------------|-----------|----------|--------|
| Public write contract | 2 | `TestExternalConsumer_CreatesSupplier`, `TestNewWriter_NilCapability_ReturnsInvalidArgument` | ✅ |
| Shape validation is Actor-only | 2 | `TestWriter_CreateSupplier_BlankActor_RejectsWithoutDelegating`, `TestAdapter_CreateSupplier_EmptyContent_ReturnsValidation` | ✅ |
| Create-reachable error taxonomy | 1 | 4 error categories each proven separately (INVALID_ARGUMENT, VALIDATION, CONFLICT, INTERNAL; NOT_FOUND documented unreachable) | ✅ |
| No CAS; Actor diagnostic-only | 1 | Structural: `write_types.go`, `types.go` fields inspected; no Revision/Actor on returned DTO | ✅ |
| Compiled write surface exports no ungraduated method | 1 | `TestWriter_NoUngraduatedMethodExported` via reflection | ✅ |
| No error leakage; defensive copying | 2 | `TestAdapter_CreateSupplier_RawInternalErrorNeverLeaks_ReturnsInternal`, `TestWriter_CreateSupplier_MutatingRequestAfterCall_NoLeak` | ✅ |
| No Actor/Revision on read surface; HardDelete absent everywhere | 2 | `TestAdapter_GetSupplier_MapsAllFields` (read surface unchanged), `TestWriter_NoDeleteOrHardDeleteMethodExported` (package-wide) | ✅ |

### Task Completion

**All 22 implementation tasks marked complete** (`[x]` in `tasks.md`):

Phase breakdown:
- Phase 1 (write types): 1/1 complete
- Phase 2 (shape validation): 3/3 complete
- Phase 3 (compiled-surface guards): 3/3 complete
- Phase 4 (external proof): 2/2 complete
- Phase 5 (bridge seam widening): 1/1 complete
- Phase 6 (CreateSupplier delegation): 1/1 complete
- Phase 7 (error taxonomy coverage): 3/3 complete
- Phase 8 (defensive copy & error message): 2/2 complete
- Phase 9 (docs & final verification): 3/3 complete

**TDD Compliance**: 5/6 checks fully passed (1 partial: apply ran directly, no formal `apply-progress` artifact; substituted with equivalent `tasks.md` evidence).

## Design Decisions Confirmed

All 6 design decisions implemented correctly:
1. **Combined seam** (`serviceWriter` + `supplierService{serviceReader;serviceWriter}`): ✅ Single `Adapter` per design decision #1
2. **Neutral error message** (`"supplier master operation failed"`): ✅ Per design decision #2
3. **Omit `CloneSupplierWriteRequest`**: ✅ Added reflection guard instead per design decision #3
4. **Create result direct projection**: ✅ `mapSupplier(created)` directly, no confirm-read per design decision #4
5. **No TrimSpace on detail fields**: ✅ Writer validation shape-only per design decision #5
6. **No new `mapError` branch**: ✅ Existing `ErrConflict` branch reused per design decision #6

## Non-Goals Respected

Verified via `git show --stat` on both merge commits:
- ✅ No schema changes or migrations
- ✅ No changes to `internal/modules/suppliers/` (authority layer untouched)
- ✅ No changes to `resourcecore/` (separate module)
- ✅ No composition root, wiring, or interface work (library-only)
- ✅ No Branch/Contact write operations (future slices)
- ✅ No Update/Deactivate/Reactivate/HardDelete operations (separate future slices)

## Artifacts Archived

Contents of archived folder `openspec/changes/archive/2026-08-22-supplier-master-core-write/`:
- `proposal.md` ✅ (original proposal with intent, scope, risks, rollback boundaries, success criteria)
- `design.md` ✅ (technical approach, 6 design decisions, data flow, testing strategy)
- `tasks.md` ✅ (22 tasks across 9 phases, all marked complete)
- `verify-report.md` ✅ (verification verdict PASS with 0 CRITICAL, 3 WARNING, 1 SUGGESTION)
- `specs/supplier-master-core/spec.md` ✅ (delta spec with 6 ADDED + 1 MODIFIED requirements)
- `archive-report.md` ✅ (this file; final state authority per Final-State Authority section)

## Final State Authority

This archive report reflects the **terminal state of the change AT CLOSE**, per the archive phase contract's Final-State Authority section. The following sources were consulted in order of rank (most authoritative first):

1. **Native review authority & Explicit final-state facts** (this prompt): Both PRs merged, issue closed, verify PASS.
2. **Persisted tasks artifact** (`tasks.md`): All 22 tasks checked.
3. **Verification report** (`verify-report.md`): PASS verdict with zero CRITICAL, all 7 requirements/11 scenarios compliant.

All intermediate snapshots' claims about pending or open work have been superseded by final-state facts. The change is **complete and closed**.

## Key Learnings

1. Stacked PR delivery (split on the 400-line review budget) kept each PR under 250 lines of author code while maintaining clear rollback boundaries and independent testability.
2. Design decisions 2 and 3 (neutral error message, value-semantic request guard) resolved potential scope creep and abstraction-without-boundary risks that arose during proposal review.
3. The modified requirement (narrowing Actor/Revision prohibition to reads only) clarified the write surface's legitimate use of Actor without creating a contradiction in the read-surface guarantee.

---

**Archive completed**: 2026-08-22  
**Phase**: sdd-archive  
**Change status**: CLOSED
