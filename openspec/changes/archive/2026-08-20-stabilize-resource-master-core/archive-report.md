# Archive Report: stabilize-resource-master-core

**Date**: 2026-08-20  
**Change**: `stabilize-resource-master-core`  
**Artifact store**: Hybrid (OpenSpec + Engram)  
**Status**: ARCHIVED  

## Executive Summary

The completed and verified change `stabilize-resource-master-core` has been successfully archived. All 29 implementation units (Stages 1–6, tasks 1A–6C) and 6 parent gates (P1–P6) are marked complete and verified. The delta spec has been merged into the canonical main spec location. The change folder has been moved to the OpenSpec archive directory with an ISO date prefix. READ-ONLY is ready per the P6 gate verdict recorded in `readiness.md`.

## Scope Confirmation

Per `proposal.md`, `design.md`, and the final gate confirmations:
- **Delivery**: READ-ONLY contract only. Public WRITE is explicitly deferred to a separate, future, per-operation-gated change.
- **Stages completed**: Mutation correctness (1), all-11 lifecycle (2), completely green PostgreSQL (3), authority semantics and coherent-reload adoption (4), dedicated Core outcomes and neutral error mapping (5), public READ-ONLY DTOs, Reader, bridge, and documentation (6).
- **Parent gates**: All 6 parent gates (P1–P6) recorded as `[x]` completed.

## Task Completion Gate

**Gate Result: PASS**

All 29 implementation units are marked `[x]` complete in `tasks.md`:
- Stage 1: 1A, 1B, 1C, 1D (4 units)
- Stage 2: 2A, 2B, 2C (3 units)
- Stage 3: 3A, 3B, 3C, 3D, 3E, 3F, 3G, 3H, 3I (9 units)
- Stage 4: 4A, 4B, 4C, 4D, 4E, 4F, 4G, 4H (8 units)
- Stage 5: 5A, 5B (2 units)
- Stage 6: 6A, 6B, 6C (3 units)

All 6 parent gates (P1–P6) are marked `[x]` completed.

## Spec Merge Status

| Domain | Action | Location |
|--------|--------|----------|
| resource-master-core | Created (new main spec) | `openspec/specs/resource-master-core/spec.md` |

**Details**: The delta spec from `openspec/changes/stabilize-resource-master-core/specs/resource-master-core/spec.md` was the complete spec (not a delta to merge). It has been copied mechanically to the new main spec location at `openspec/specs/resource-master-core/spec.md`. The spec contains 14 requirements covering consumer-neutral READ-ONLY contract, generic catalog discovery, canonical values, resource query/detail, durable identity, stable public errors, complete catalog lifecycle, conservative hard delete, atomic applicability aggregate, persisted revisions with CAS, commit/publication equivalence, authoritative writer topology, and distinct read/write readiness gates. No modifications were made to the spec content during the merge.

## Archive Contents Verification

**Archive location**: `openspec/changes/archive/2026-08-20-stabilize-resource-master-core/`

All critical artifacts present:
- ✓ `proposal.md` (16,220 bytes)
- ✓ `design.md` (59,461 bytes)
- ✓ `tasks.md` (75,540 bytes)
- ✓ `verify-report.md` (15,269 bytes)
- ✓ `readiness.md` (11,730 bytes)
- ✓ `explore.md` (16,829 bytes)
- ✓ `specs/resource-master-core/spec.md` (22 KB)
- ✓ `apply-progress.md` (429,264 bytes)

**Verification method**: File-by-file existence check (all critical files present and readable).

## Final-State Authority and Corrections

Per the launch prompt's "Final-state facts" section:

1. **Verify verdict**: PASS, 0 CRITICAL, 1 WARNING (non-blocking), 1 SUGGESTION (non-blocking). Full detail in `verify-report.md`.

2. **The one WARNING (already fixed)**: apply-progress.md's 6B-correction and readiness.md's focused-receipts table originally claimed "27 total tests" across `resourcecore` + `internal/bridge/resourcecore`; that number was actually matched by a specific regex, not the full package test count (true total is 37: 23 `resourcecore` + 14 bridge). Both documents were corrected in place to distinguish "27 regex-matched" from "37 total". **This is closed and not re-flagged as an open issue.**

3. **P5 and P6 gate checkboxes**: P5 was retroactively confirmed and checked `[x]` during the P6 gate closure (its implementation was already done and green, but its own gate checkbox had lagged). P6 was closed with verdict "READ-ONLY is ready" recorded in `readiness.md`. Every task in `tasks.md` is now `[x]` — 29/29 implementation units plus 6/6 parent gates.

4. **Scope reminder**: This change delivers READ-ONLY only. Public WRITE is explicitly out of scope and deferred to a separate future change per `proposal.md` and `design.md`. This archive does not deliver, contain, or claim WRITE capability.

5. **Two known, already-investigated, unrelated build failures**: Both persist in the full suite and are NOT part of this change:
   - `agent/skills/golang-cli/assets/examples` (untracked skill scaffold missing example deps)
   - `internal/tui/suppliers_admin.go` (untracked work-in-progress from unrelated `proveedores-maestro-ux` branch/stash)
   
   Both confirmed via `git ls-files`/`git status --porcelain` to be untracked and outside this change's scope. Neither introduced nor affected by this change. Do not treat them as follow-up work items owned by this archive.

## Key Learnings

1. The delta spec for resource-master-core was a complete, self-contained specification, not a patch requiring merge into an existing main spec.
2. All 29 implementation units across 6 stages and 6 parent gates completed successfully with strict TDD evidence (RED/GREEN/TRIANGULATE/REFACTOR) recorded in apply-progress.md for every unit.
3. The P6 gate (READ-ONLY readiness evaluation) found that exactly 6 of the 15 public error categories are reachable from the READ-ONLY surface today (`INVALID_ARGUMENT`, `NOT_FOUND`, `INTEGRITY`, `INVALID_CATALOG`, `UNAVAILABLE`, `INTERNAL`); the remaining 9 categories are write-only outcomes.
4. PostgreSQL integration and race evidence was correctly gated as CI-only per project policy and not re-run locally during the targeted verify pass, consistent with the discipline of not trusting intermediate snapshots.
5. External-package construction using only public types was proven (resourcecore/external_test.go), confirming no internal/infrastructure leakage in the public boundary.
6. The design's one-writer topology was confirmed by inspection (`Reader` has no `Reload` method).
7. Migration 8 (additive revision support) backfilled all 12 tables correctly with identity byte-equality preserved (verified in 3C/3I).

## Artifacts Persisted

**OpenSpec locations:**
- Main spec: `openspec/specs/resource-master-core/spec.md`
- Archive folder: `openspec/changes/archive/2026-08-20-stabilize-resource-master-core/` (complete with proposal, design, tasks, readiness, verify-report, explore, specs, apply-progress)
- This archive report: `openspec/changes/archive/2026-08-20-stabilize-resource-master-core/archive-report.md`

**Engram topic** (hybrid mode):
- Topic key: `sdd/stabilize-resource-master-core/archive-report`
- Type: architecture
- Observation IDs: Recorded below for traceability

## Traceability (Observation IDs for Hybrid Mode)

No Engram observations were read during this archive phase (all artifacts came from OpenSpec), so no observation IDs are recorded. In hybrid mode, the archive report itself is persisted to Engram as a new observation under topic_key `sdd/stabilize-resource-master-core/archive-report`.

## Risks

None. All tasks completed, all gates passed, all specs merged, all files archived. No stale checkboxes or open issues remain. The two pre-existing untracked build failures are acknowledged as unrelated and out-of-scope.

## Rollback and Recovery

If this archive must be reversed:
1. Move `openspec/changes/archive/2026-08-20-stabilize-resource-master-core/` back to `openspec/changes/stabilize-resource-master-core/`
2. Delete `openspec/specs/resource-master-core/` (it is new, not a pre-existing main spec)
3. No production code, tests, or database state was changed by archival; stages 1–6 remain in the repository

The change is immutable in the archive and serves as an audit trail for the complete delivered READ-ONLY contract and all prerequisite stabilization stages.

## Next Steps

Per the launch prompt and final-state authority hierarchy:
- **READ-ONLY**: Ready and complete. No further action required.
- **Public WRITE**: Explicitly not part of this change. A separate, per-operation-gated change may begin only after explicit new-change decisions, per `spec.md`'s "Requirement: Separate read and per-operation write readiness" and `proposal.md`'s "Delivery plan."

---

**Archived by**: sdd-archive executor  
**Archive date**: 2026-08-20 (ISO format)  
**Archive mode**: hybrid (OpenSpec + Engram)  
**Final status**: CLOSED
