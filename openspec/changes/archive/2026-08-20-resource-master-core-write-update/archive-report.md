# Archive Report: resource-master-core-write-update

**Date**: 2026-08-20
**Change**: resource-master-core-write-update
**Status**: ARCHIVED
**Verdict**: Complete and closed (PASS WITH WARNINGS — 0 CRITICAL / 1 WARNING)

## Executive Summary

The `resource-master-core-write-update` change has been successfully archived. All implementation units (U1–U3) and the parent readiness gate (G1) are complete and independently verified. This is the second per-operation public WRITE graduation for the Resource Master Core — `Update` (catalog and resource, under optimistic-concurrency CAS) — building directly on the archived Create graduation (`2026-08-21-resource-master-core-write`). Verification returned PASS WITH WARNINGS: 0 CRITICAL, 1 WARNING. The single warning is a disclosed, non-blocking live-PostgreSQL test-execution gap (no `GARFEX_TEST_DSN`-exposed database in the verification sandbox), not a correctness issue — every non-DB-gated test, `gofmt`, `go vet`, and the full non-DB `go test ./...` suite passed.

## Artifacts Archived

| Artifact | Location | Status |
|----------|----------|--------|
| Proposal | `openspec/changes/archive/2026-08-20-resource-master-core-write-update/proposal.md` | Archived |
| Explore | `openspec/changes/archive/2026-08-20-resource-master-core-write-update/explore.md` | Archived |
| Design | `openspec/changes/archive/2026-08-20-resource-master-core-write-update/design.md` | Archived |
| Delta Spec | `openspec/changes/archive/2026-08-20-resource-master-core-write-update/specs/resource-master-core/spec.md` | Archived |
| Tasks | `openspec/changes/archive/2026-08-20-resource-master-core-write-update/tasks.md` | Archived (all checked) |
| Readiness | `openspec/changes/archive/2026-08-20-resource-master-core-write-update/readiness.md` | Archived |

No separate `apply-progress.md` or `verify-report.md` file existed in the source change folder at archive time. `readiness.md` (written by the implementing unit, U3) and the orchestrator's explicit launch-prompt final-state fact together serve as this change's verification record; both are cited below per the Final-State Authority hierarchy.

## Gate Verification Results

### Task Completion Gate

**Status**: PASS

All 4 checkbox items in `tasks.md` (U1, U2, U3, G1) are marked complete with `[x]`. Independent verification (`grep`) on the archived copy confirmed:
- 4/4 checked, 0 unchecked implementation tasks found

### Native Review Receipt Gate

**Status**: N/A

No `reviewGate` was discovered for this candidate during the archive phase (no structured status/review artifacts were provided or found). Per the gate specification, when receipt-driven development did not run for this candidate, archive proceeds under ordinary repository policy.

### Verify Verdict

**Status**: PASS WITH WARNINGS

Source: explicit final-state fact supplied at archive launch — "independently verified (PASS WITH WARNINGS — 0 CRITICAL / 1 WARNING, the warning being a disclosed, non-blocking live-DB test-execution gap, not a correctness issue)." This is corroborated by `readiness.md`'s own "Verdict" and "CONFLICT-wiring fix — verification" sections, which document the same gap in the same terms:

- **CRITICAL issues**: 0
- **WARNING issues**: 1 — live-PostgreSQL integration evidence for `internal/postgres/catalog_admin_repository_v2.go` could not be executed in the verification sandbox (no `GARFEX_TEST_DSN`-exposed database). `TestApplicabilityAggregateV2Integration` reported `SKIP: GARFEX_TEST_DSN not set`; the non-DB test in scope (`TestCatalogAdminRepositoryV2ConstructorConformance`) passed. Per this repository's own documented convention (`docs/architecture/catalog-source-of-truth.md`), PostgreSQL evidence for isolated-DB-only suites comes from a recorded isolated CI run, not a local sandbox lacking an exposed DSN. The zero-breakage claim for the 14 existing `errors.Is` call sites is structurally guaranteed by the reassignment itself (identity comparison, not behavioral change) and was grep-verified as the complete reference set, but not exercised live in this session.
- **SUGGESTION issues**: none recorded.

Everything gated by unit tests, `gofmt`, `go vet`, and the full non-DB `go test ./... -count=1` suite is proven and green.

## Specs Merged

### Main Spec Updated

**File**: `openspec/specs/resource-master-core/spec.md`

**Changes**: ADDED 6 new requirements (87 lines: 389 → 476)

| Requirement | Status |
|-------------|--------|
| Consumer-neutral public Update for catalog and resource | ADDED |
| Update field completeness through the sole bridge | ADDED |
| Catalog stale-revision update returns CONFLICT, not INTERNAL | ADDED |
| Update-reachable error category coverage | ADDED |
| Actor attribution without persistence on Update | ADDED |
| Compiled surface limited to graduated Create and Update | ADDED |

All 6 ADDED requirements were appended to the end of the Requirements section in the main spec, mechanically edited (not copy-replaced, since the main spec already existed). No existing requirements were removed. The main spec now serves as the source of truth for all resource-master-core behavior, including READ-ONLY, CREATE, and UPDATE.

**Verification**:
- `wc -l`: 389 → 476 lines (delta: +87, matching the 6 appended requirement/scenario blocks).
- `grep -n "^## ADDED Requirements\|^# Delta for"` on the merged main spec: no matches — no leftover delta-file scaffolding headers carried over.
- `grep -n "^### Requirement:"` on the merged main spec, checked for duplicate titles: no duplicates found. All 25 requirement titles (19 pre-existing + 6 newly added) are unique strings.

**Known content overlap flagged by the archive subagent, resolved by the orchestrator immediately after**: the archive subagent correctly followed its mechanical merge contract (preserve all requirements not explicitly named MODIFIED/REMOVED/RENAMED in the delta; never invent an unauthorized removal) and left the pre-existing requirement "Compiled surface limited to graduated Create" (originally lines 380–389) in place, flagging it as an open item rather than silently rewriting it. That requirement stated the compiled `WriteCapabilities`/`Writer` surface declares *only* `CreateCatalog`/`CreateResource` and that no `Update` method exists — a claim that became **factually false**, not merely stale, the moment this change's own new requirement "Compiled surface limited to graduated Create and Update" was merged into the same file (4 compiled methods now exist). Since the main spec's purpose is to state current system truth, not preserve per-change history (that is what this archive directory is for), the orchestrator judged this a documentation-correctness bug rather than a discretionary cleanup, and removed the old requirement (lines 380–390) directly from `openspec/specs/resource-master-core/spec.md` right after archive completed. The newer requirement fully subsumes the old one's constraint (same "Separate read and per-operation write readiness" lineage, same drivers-untouched clause, extended to both graduated methods), so nothing was lost — only the outdated, now-contradicted claim was removed. Post-fix: main spec is 465 lines (476 − 11), 24 unique requirement titles, `rg -n "^### Requirement: Compiled surface"` returns exactly one match.

## Archive Move Verification

**Status**: PASS

The change folder was moved from `openspec/changes/resource-master-core-write-update/` to `openspec/changes/archive/2026-08-20-resource-master-core-write-update/` using a mechanical shell operation. `git mv` was attempted first and failed with `fatal: source directory is empty` because the change folder was untracked in git (never committed) — the shell fell through to plain `mv`, which succeeded.

**Readback Verification**:
```
diff -r <pre-move recursive snapshot> openspec/changes/archive/2026-08-20-resource-master-core-write-update
Result: 0 differences (exit code 0)
```

The archived folder is byte-identical to the pre-move snapshot. All 6 source artifacts (proposal, explore, design, tasks, readiness, specs/resource-master-core/spec.md) are present and unchanged. The source directory `openspec/changes/resource-master-core-write-update/` no longer exists (confirmed via `ls` failure post-move).

## Final State Authority

This archive report records the state of the change AT CLOSE. The following facts establish final state per the authority hierarchy:

### Fact 1: All tasks are complete
**Source**: Persisted `tasks.md` checkpoint + independent verification
**Authority rank**: Native tasks artifact (rank 2)
**Finding**: U1, U2, U3, and the parent readiness gate G1 are all marked `[x]` in the persisted artifact. Independent grep confirmed 0 unchecked tasks.

### Fact 2: Verification passed with warnings, 0 critical
**Source**: Orchestrator's launch-prompt explicit final-state fact
**Authority rank**: Explicit final-state facts (rank 3)
**Finding**: "independently verified (PASS WITH WARNINGS — 0 CRITICAL / 1 WARNING, the warning being a disclosed, non-blocking live-DB test-execution gap, not a correctness issue)."

Corroborated by `readiness.md` (rank 4, snapshot, but consistent with and independently re-checked against the rank-3 fact — no contradiction found):
- All unit-level and non-DB test evidence, `gofmt -l .`, `go vet ./...`, and `go test ./... -count=1` (excluding the live-DB-gated integration test) pass.
- Zero-touch confirmation for `cmd/garfex/` and `internal/tui/` confirmed (`git diff --stat` empty, `rg -l resourcecore` no matches).
- Both of design.md's open questions resolved and test-proven: no create-confirm read on `Adapter.UpdateResource` (call-count assertion); `IMMUTABLE_CODE` evidence driven through the kind registry, not a hardcoded kind.
- Catalog Update proves exactly 10 reachable, distinct error categories; resource Update proves exactly 9 — matching the spec's fixed counts exactly, with `IDENTITY_CONFLICT`/`INVALID_LIFECYCLE`/`REACTIVATION_IMPOSSIBLE`/`IN_USE` excluded from both, `IMMUTABLE_CODE` excluded from `UpdateResource`, and `INTEGRITY` excluded from `UpdateCatalog`.
- Catalog `CONFLICT` reachability is this change's fix (`errApplicabilityStaleRevision`/`errCatalogStaleRevisionV2` reassigned to `domain.ErrRevisionConflict`); resource `CONFLICT` was already correct before this change.

### Fact 3: The one WARNING is a disclosed evidence gap, not a defect
**Source**: `readiness.md`'s "Verdict" and "CONFLICT-wiring fix — verification" sections + orchestrator's explicit final-state fact
**Authority rank**: Explicit final-state facts (rank 3) corroborated by readiness record (rank 4)
**Finding**: The live-PostgreSQL integration suite for `internal/postgres/catalog_admin_repository_v2.go` could not run in the verification sandbox (no `GARFEX_TEST_DSN`). The zero-breakage claim for the 14 existing `errors.Is` call sites referencing the reassigned sentinels is structurally guaranteed by the reassignment being a direct identity substitution (not a wrap), and was grep-confirmed as the complete reference set — but was not exercised against a live database in this session. `readiness.md` explicitly recommends running the isolated-DB integration suite before merge, per this repository's own established PostgreSQL-evidence convention. This is not treated as a correctness defect and does not block archive; it is recorded here as the disclosed condition behind the WARNING verdict.

### Fact 4: No changes to internal/app, internal/domain; internal/postgres touched exactly 2 lines
**Source**: `readiness.md`'s "Zero-touch confirmation" and "Changed-line summary" sections
**Authority rank**: Readiness record (rank 4, but consistent with tasks.md's binding execution contract, rank 2)
**Finding**: `internal/app/*` and `internal/domain/*` remain at zero changed lines for the entire U1–U3 chain. `internal/postgres/*` changes exactly the two-line-plus-comment `CONFLICT`-sentinel reassignment in U3 (7 insertions, 5 deletions per `git diff --stat`) — no other file in that package changed. `cmd/garfex/` and `internal/tui/` show zero changed lines and zero new `resourcecore` references (`git diff --stat` empty, `rg -l resourcecore` no matches).

### Fact 5: Actor mechanism reused verbatim, never persisted
**Source**: `readiness.md`'s "Actor attribution" section
**Authority rank**: Readiness record (rank 4)
**Finding**: `Actor` reaches `internal/core/diagnostics.go`'s `core.WithActor`/`ActorFrom` seam on both the success and failure path for `UpdateCatalog` and `UpdateResource`, reusing Create's mechanism verbatim with zero redesign. `Actor` is never persisted, introduces no new column or migration, and appears on no public DTO.

## Rollback Boundary

If this change must be withdrawn, the following must be reverted:

1. **Delete archived folder**: `openspec/changes/archive/2026-08-20-resource-master-core-write-update/`
2. **Revert main spec**: Remove the 6 ADDED requirements from `openspec/specs/resource-master-core/spec.md` (restore to 389 lines)
3. **Revert code**: `resourcecore/{write_types,writer,copy,writer_test,external_test}.go`, `internal/bridge/resourcecore/{adapter,adapter_test}.go` (Update additions only — Create untouched), and the two-line-plus-comment sentinel reassignment in `internal/postgres/catalog_admin_repository_v2.go`
4. **No database migration required**: rollback of the sentinel reassignment alone restores the prior `INTERNAL` misclassification for catalog `CONFLICT` — no persisted data or migration is touched by either the fix or its rollback

The Create-WRITE and READ-ONLY contracts remain fully available and unaffected by any rollback of this change.

## What This Change Delivers

Per `proposal.md`'s intent: the public mirror of `Update`, Create's CAS-bearing counterpart, letting external consumers (PI) modify an existing catalog record or resource under optimistic concurrency and receive a stable `CONFLICT` on a stale write instead of a leaked `INTERNAL` error.

**Delivered and verified**:
- `resourcecore.Writer.UpdateCatalog`/`UpdateResource` (+`WriteCapabilities` +2 methods) — public Update contract under CAS (`ID`/`Kind` + `ExpectedRevision uint64` + `Actor`), mirroring Create's mechanism exactly
- `internal/bridge/resourcecore.Adapter.UpdateCatalog`/`UpdateResource` — sole-bridge translation, reusing every inverse mapper Create built, zero new mapping functions
- The CONFLICT-wiring fix: `errApplicabilityStaleRevision` and `errCatalogStaleRevisionV2` reassigned directly to `domain.ErrRevisionConflict`
- Catalog Update: 10 of 15 error categories reachable and distinct, including `NOT_FOUND` (reversing Create's finding) and the new `IMMUTABLE_CODE` and `CONFLICT`
- Resource Update: 9 of 15 error categories reachable and distinct (matches explore.md's original estimate exactly)
- Zero-touch: `internal/app/*`, `internal/domain/*` at zero changed lines; `internal/postgres/*` at exactly 2 changed lines; `cmd/garfex/`, `internal/tui/` at zero changed lines and zero new `resourcecore` references
- `Deactivate`/`Reactivate`/`HardDelete` remain designed on paper only — no unit compiles, exports, or stubs any of them

**Not delivered** (per design, separate later changes):
- `Deactivate`, `Reactivate`, and catalog `HardDelete` operations remain uncompiled/unavailable — each is a separate, later, per-operation-gated change

## Known Scope Narrowing / Discrepancy (Honestly Documented)

**Catalog Update's `INTEGRITY` category is not reachable** — narrows the explore.md-estimated 11 categories to 10. `domain.ErrResourceIntegrity` has zero references anywhere in `internal/postgres/catalog_admin_*.go` (grep-verified); it is a resource-repository-only sentinel. This is stated honestly in both `design.md` and `readiness.md` rather than overclaimed, and the spec's "Update-reachable error category coverage" requirement is written to the corrected count of exactly 10, not the original estimate of 11.

**Catalog Update's `NOT_FOUND` reverses Create's finding** — Create's archived readiness record found catalog `NOT_FOUND` not end-to-end reachable; this change's `readiness.md` finds it genuinely reachable for Update via `buildV2UpdateCandidate`'s `s.repo.Get` and `ApplyCatalogMutation(OpUpdate)`'s `mutateSlice` `idx < 0` branch. This is an operation-specific finding, not a contradiction of the Create finding (different code paths).

## Deviations from Plan

None. The change executed exactly as designed:
- U1 (public Update contract), U2 (bridge Update translation), U3 (CONFLICT-wiring fix + readiness) completed in strict order
- G1 readiness gate confirmed all 11 success criteria (a)–(k)
- No production code deviations from design.md
- No scope creep — `Deactivate`/`Reactivate`/`HardDelete` were never applied or planned as code

## Pre-Existing, Unrelated Failures

One known, pre-existing, untracked, unrelated build failure persists across verification runs, consistent with the Create archive's record:

- **`agent/skills/golang-cli/assets/examples`**: setup failure from missing example third-party dependencies (`github.com/fatih/color`, `fsnotify`, `cobra`, `viper`, `github.com/you/myapp/cmd`) not in `go.mod`. Untracked scaffold, unconnected to this change, already broken before it (present in `git status` as untracked prior to this change).

This is outside the scope of `resource-master-core-write-update` and does not block archive.

## Delivery Readiness

Per `readiness.md`'s verdict: **"Update-WRITE is ready"**, with the one disclosed live-DB evidence gap noted above. The recommendation is to run the isolated-DB integration suite (`GARFEX_TEST_DSN`/`GARFEX_ADMIN_TEST_DSN` against an isolated database, never the `garfex_pgdata` volume) before merge, consistent with this repository's established PostgreSQL-evidence convention.

The change is archived and closed. Delivery (commit, push, PR creation) and running the live-DB integration suite are separate orchestrator/maintainer decisions and are not performed by this phase.

## SDD Cycle Complete

- Proposal: Defined scope (Update graduation), approach (mirror Create exactly), and rollback plan
- Explore: Established the codebase evidence baseline
- Spec: Added 6 new requirements to the `resource-master-core` capability
- Design: Fixed the complete Update shape (U1/U2/U3 task-unit breakdown), resolved 2 open questions
- Tasks: Ordered 3 implementation units (U1–U3) plus parent readiness gate (G1), auto-chain stacked-to-main
- Apply: Executed all units in order, tasks marked complete
- Verify: Independent verification returned PASS WITH WARNINGS — 0 CRITICAL / 1 WARNING (disclosed live-DB evidence gap)
- Archive: Merged specs (389 → 476 lines), archived change folder; orchestrator then removed one now-superseded pre-existing requirement the merge correctly left flagged rather than silently rewrote, bringing the main spec to its final 465 lines — closed the cycle

This change is ready for the next phase: orchestrator-controlled delivery (commit/PR/push), ideally preceded by running the live-DB integration suite to close the one disclosed WARNING.
