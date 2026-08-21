# Archive Report: resource-master-core-write-harddelete

**Change**: resource-master-core-write-harddelete  
**Archived**: 2026-08-21  
**Artifact store**: OpenSpec (hybrid mode with Engram)  
**Status**: ✅ CLOSED — ready for production inclusion

## Final Verdict

**PASS WITH WARNINGS**

- **CRITICAL findings**: 0
- **WARNING findings**: 3 (all non-blocking, all pre-existing pattern gaps, not regressions specific to this change)
- **SUGGESTION findings**: 1 (pre-existing untracked scaffold in unrelated package)
- **Requirements covered**: 7/7 by passing tests
- **Scenarios covered**: 7/7 by passing tests
- **Tasks complete**: 4/4 (U1, U2, U3, G1) — all `[x]`

Per `verify-report.md`, verdict is **PASS WITH WARNINGS** with 0 CRITICAL blocking the change. All three warnings are precedent-matching pattern gaps, not regressions this session introduced. Archive proceeds unconditionally.

## Compiled Surface

The public `Writer`/`WriteCapabilities` interface now exposes exactly **9 methods**:

1. `CreateCatalog`
2. `CreateResource`
3. `UpdateCatalog`
4. `UpdateResource`
5. `DeactivateCatalog`
6. `ReactivateCatalog`
7. `DeactivateResource`
8. `ReactivateResource`
9. `HardDeleteCatalog` ← **NEW in this change**

`HardDeleteResource` remains a **deliberate, documented gap**, not addressed by this graduation or any unit in the plan. No stub, no exported symbol, no discoverable artifact exists for `HardDeleteResource`.

## Scope Boundary

**In scope**: `HardDeleteCatalog` operation only, for the catalog side of Resource Master.  
**Out of scope**: `HardDeleteResource` — deferred to a separate, future, per-operation-gated change with its own readiness evidence.

This completes the **catalog write contract** (create, update, deactivate, reactivate, hard delete). The **resource write contract** remains open (resource-side create, update, deactivate, reactivate are already public; resource-side hard delete is still future work).

## Specification Merge

### Delta Spec Changes Applied

**File**: `openspec/specs/resource-master-core/spec.md`

**ADDED Requirements** (6 new):
1. Consumer-neutral public HardDeleteCatalog
2. HardDelete field completeness through the sole bridge, no confirm-read
3. Bridge delegates to Core authority — no guard-chain re-implementation
4. Strict CAS on HardDeleteCatalog — no idempotent no-op bypass
5. HardDelete-reachable error category coverage
6. Actor attribution without persistence on HardDeleteCatalog

**MODIFIED Requirement** (1):
- Requirement: "Compiled surface extended to Create, Update, Deactivate, and Reactivate" (8 methods)
  - **→** Requirement: "Compiled surface extended to Create, Update, Deactivate, Reactivate, and HardDeleteCatalog" (9 methods)
  - Updated scenario to include `HardDeleteCatalog` in the discoverable set
  - Added explicit note that `HardDeleteResource` remains a deliberate gap

The previous 8-method "Compiled surface" requirement is superseded by this change's modified 9-method version and removed from the merged spec per the delta's explicit notation.

## Verification Evidence

Per `verify-report.md` (independent fresh-context re-derivation, not trusting implementer claims):

### Test Coverage
- All 7 spec requirements have passing covering tests
- 11 named tests executed (3 in U1, 8 in U2)
- Every named test re-run independently and confirmed passing
- Full-suite `go test ./... -count=1` passes except for one pre-existing unrelated package (disclosed gap across 3 prior graduations)

### Correctness Proofs
- ✅ Error-only return shape (`HardDeleteCatalog(ctx, req) error`)
- ✅ Guard-chain non-duplication: structural (capability starvation) + behavioral (falsifiable call-count test)
- ✅ Strict CAS, no idempotent bypass: confirmed via source and test
- ✅ 8-category error table: distinct, no leakage, exclusion list confirmed
- ✅ 9-method compiled surface: reflection test confirms exactly 9 methods, `HardDeleteResource` confirmed absent everywhere in repo
- ✅ Zero-touch guarantee: `cmd/garfex/`, `internal/tui/`, `internal/app/*`, `internal/domain/*`, `internal/postgres/*`, `internal/core/errors.go` all unchanged
- ✅ Defensive copy of `CatalogLifecycleRequest` by construction (scalar-only fields)

### Build & Test Evidence
```text
Build: ✅ PASS (go build ./resourcecore/... ./internal/bridge/resourcecore/...)
Tests: ✅ PASS (go test ./resourcecore/... ./internal/bridge/resourcecore/... -count=1)
gofmt: ✅ PASS (no formatting issues)
go vet: ✅ PASS (except pre-existing unrelated package)
golangci-lint: ✅ PASS (5 issues in pre-existing unrelated package only)
```

### TDD Compliance
- RED: 11 named test functions confirmed present and verifying actual behavior
- GREEN: All 11 tests pass
- TRIANGULATE: Coverage across 3+ independent failure modes per requirement, 8 distinct error categories, 7 exclusion cases
- REFACTOR: No tautologies, no ghost loops, genuine production-code-exercising assertions using actual call-count tracking and ctx-propagated values

## Archive Contents

✅ All SDD artifacts successfully archived to `openspec/changes/archive/2026-08-21-resource-master-core-write-harddelete/`:

- `proposal.md` — change rationale, risk analysis, delivery strategy (auto-chain, chained PRs)
- `explore.md` — design exploration notes
- `design.md` — Decision 1 (error-only return) + Decision 2 (three mechanisms: capability starvation, pass-through body, falsifiable non-duplication proof)
- `tasks.md` — 4 ordered units (U1: public contract, U2: bridge translation, U3: readiness record, G1: parent gate) with strict RED/GREEN/TRIANGULATE/REFACTOR evidence embedded per-unit; all `[x]` checked
- `readiness.md` — comprehensive readiness record documenting the 8-category error table, guard-chain non-duplication proof, no-confirm-read proof, strict-CAS-no-bypass contrast with lifecycle precedent
- `specs/resource-master-core/spec.md` — delta spec with 6 ADDED + 1 MODIFIED requirement
- `verify-report.md` — independent fresh-context verification: PASS WITH WARNINGS, 0 CRITICAL, 7/7 scenarios passing

## Key Learnings

1. The four-graduation series (Create, Update, Deactivate/Reactivate, HardDeleteCatalog) demonstrates strict per-operation readiness gates enforced independently across all 11 catalog kinds without cross-operation coupling.

2. The guard-chain non-duplication requirement is proven falsifiable using a three-part mechanism: capability starvation (bridge port has no `Dependents` method), pass-through implementation (body has zero business-rule branches), and behavioral assertion (call-count test with a fake that would report `Active: true` still yields zero reads).

3. This change family's convention of omitting `apply-progress.md` in favor of embedding strict RED/GREEN/TRIANGULATE/REFACTOR evidence directly in per-unit `tasks.md` sections is an established pattern matched identically by the two immediately-preceding archived graduations in this series.

4. Strict CAS with no idempotent bypass is the operative contrast between hard delete and the lifecycle toggle precedent — a deleted record has no "already deleted" state to short-circuit to, making every stale `ExpectedRevision` an unconditional `CONFLICT`.

5. `resourcecore.NewWriter` has no production composition root yet, so the 8-method-to-9-method `WriteCapabilities` interface widening never breaks bridge compilation before the bridge adapter itself is updated in the same change.

## Next Steps

- ✅ Archive complete
- ✅ Main spec merged with 6 ADDED + 1 MODIFIED requirement
- ✅ Catalog write contract closed (create, update, deactivate, reactivate, hard delete all public and ready)
- ⏳ Resource write contract remains open (resource-side hard delete is a separate future change)
- ⏳ Next change: resource-master-core-write-harddelete-resource (if planned) or other tracked work
