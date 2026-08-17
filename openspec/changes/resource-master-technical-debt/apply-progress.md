# Apply Progress: Resource Master Technical Debt

## Slice

- Change: `resource-master-technical-debt`
- Work unit: `pr1-resource-integrity-audit`
- Mode: Strict TDD
- Boundary: PR 1 / task 1.1 only; stacked-to-main target `main`, no predecessor
- Review budget: 312 authored changed lines; no size exception

## Completed Task

- [x] 1.1 Foundation audit and integrity migration

## TDD Cycle Evidence

| Task | Test file | Layer | Safety net | RED | GREEN | TRIANGULATE | REFACTOR |
|---|---|---|---|---|---|---|---|
| 1.1 | `internal/postgres/resource_repository_integration_test.go` | PostgreSQL integration | ⚠️ Original pre-edit baseline was not captured; correction assertions were written before the migration correction | ✅ Frozen candidate failed when its v1-only constraint rejected a legacy-key resource created after up | ✅ Corrected audit-only migration passed on PostgreSQL | ✅ Collision with inactive lifecycle row, overlapping applicability, legacy mapping, post-up legacy writer, and rollback | ✅ `gofmt`, vet, lint, and full tests remained green |

## Work Unit Evidence

| Evidence | Result |
|---|---|
| Focused test | `GARFEX_ADMIN_TEST_DSN=... go test ./internal/postgres -run '^TestResourceIntegrityMigrationIntegration$' -count=1` — PASS, 2 named admission subtests plus rollback flow, 0.342s |
| Runtime harness | Isolated integration database `garfex_resource_integrity_pr1_20260816` owned by `garfex_admin`; migrations 000001–000004 applied, migration admission and rollback exercised. Existing PostgreSQL volumes were not touched or destroyed. |
| Rollback boundary | Revert `migrations/000005_resource_integrity.up.sql`, `migrations/000005_resource_integrity.down.sql`, and the focused integration fixtures; no unrelated Resource, Supplier, TUI, or `.atl` files are involved. |

## Implementation Notes

- The up migration records legacy-to-v1 identity mappings, computes UTF-8 byte-length-prefixed canonical components, rejects canonical collisions, rejects mixed encodings, and blocks overlapping or invalid attribute applicability without rewriting keys or adding an encoding constraint.
- The down migration removes only the mapping table and helper function; it never restores or changes resource identity keys.
- The migration intentionally does not choose winners for collisions or applicability ambiguity; admission fails instead.
- PR2 owns the atomic writer transition, admitted key rewrite, and v1-only constraint.

## Verification

- `gofmt -l .` — PASS (no output)
- `go vet ./...` — PASS
- `golangci-lint run ./...` — PASS (`0 issues`)
- `go test ./internal/postgres -count=1` with temporary admin/app DSNs — PASS
- `go test ./... -count=1` without database DSNs — PASS
- A database-enabled `go test ./...` was also attempted; it exposed the repository's existing cross-package shared-database cleanup race in the TUI E2E test, while the focused PostgreSQL package run passed. This remains a verification risk for parallel DB-enabled full-suite execution.

## Remaining Tasks

- [ ] 2.1 through 9.1 remain pending and are out of scope for this slice.

## Next Recommendation

- PR 1 must undergo independent verification, native review, and delivery to `main` before PR 2 begins.
- Whole-change final SDD verification is not ready because tasks 2.1–9.1 remain pending.
