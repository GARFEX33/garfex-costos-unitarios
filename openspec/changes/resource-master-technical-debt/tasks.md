# Tasks: Resource Master Technical Debt Remediation

## Review Workload Forecast

| Field | Value |
|---|---|
| Estimated changed lines | 650–950 total; each PR target ≤350 authored lines |
| 400-line budget risk | High |
| Chained PRs recommended | Yes |
| Delivery strategy | ask-on-risk, resolved to chained PRs |
| Execution mode | auto |
| Chain strategy | stacked-to-main |
| Size exception | No |

Decision needed before apply: No
Chained PRs recommended: Yes
Chain strategy: stacked-to-main
400-line budget risk: High

### Ordered stacked-to-main PR chain

Every PR targets `main`; PR N starts only after PR N-1 merges. Each stays under 400 changed lines. Supplier Master, `.atl/`, and unrelated worktree changes are excluded from every PR.

| PR | Owns | Target / main dependency | Acceptance evidence; runtime harness | Rollback boundary |
|---|---|---|---|---|
| 1 | 1.1 | `main`; none | Audit/admission without key rewrites or an encoding constraint, collisions, applicability, mappings; `go test ./internal/postgres -run ResourceIntegrity`; migration admission | Revert migration only |
| 2 | 2.1 | `main`; PR 1 merged | Forgery, canonical equivalence, delimiters, rejected calls, atomic writer transition, admitted key rewrite, and v1 constraint; `go test ./internal/domain ./internal/app/recursos`; N/A—in-process | Revert domain/app/migration files |
| 3 | 3.1 | `main`; PR 2 merged | Active-chain and historical reads; `go test ./internal/domain ./internal/postgres -run Active`; DB history | Revert catalog/CRUD activity |
| 4 | 4.1 | `main`; PR 3 merged | Zero/one/multiple targets and rollback; `go test ./internal/postgres -run 'Attribute|Cardinality'`; DB transaction | Revert scoped CRUD |
| 5 | 5.1 | `main`; PR 4 merged | Create/edit/duplicate, failure, cancellation parity; `go test ./internal/tui -run ResourceEditor`; N/A—characterization | Revert editor extraction |
| 6 | 6.1 | `main`; PR 5 merged | Defaults, visible state, idempotency, discovery, failed reactivation; `go test ./internal/app/recursos ./internal/tui`; DB lifecycle | Revert lifecycle wiring |
| 7 | 7.1 | `main`; PR 6 merged | Query count 1/10/50, filters, hydration failures; `go test ./internal/postgres -run Search`; DB query-count | Revert set-based search |
| 8 | 8.1 | `main`; PR 7 merged | Stable order, boundaries, retained filters/selection; `go test ./internal/tui -run 'Page|Navigation'`; N/A—TUI tests | Revert page/dispatch |
| 9 | 9.1 | `main`; PR 8 merged | Docs/comments and full gate; `go test ./... -count=1`; `go run ./cmd/garfex` smoke | Revert docs/comments |

## Foundation-first release gates and work units

- [x] 1.1 **PR 1 / foundation gate**: Test audit fixtures in `internal/postgres/resource_repository_integration_test.go`; add `migrations/<next>_resource_integrity.{up,down}.sql`; record legacy-to-v1 mappings and block identity/applicability ambiguity without rewriting keys or enforcing v1 (debts 2, 3, 8).

- [x] 2.1 **PR 2**: Test and enforce forged-input rejection, controlled construction, canonical identity, and stable update IDs in `internal/domain/*` and `internal/app/recursos/service.go`; atomically switch writers, rewrite admitted legacy keys, and add the v1 constraint only after writers emit v1 (debts 1, 7, 8).
- [x] 3.1 **PR 3**: Test and enforce active-chain eligibility with historical reads in `internal/postgres/catalog_loader.go` and `resource_repository_crud.go` (debt 4); release gate before refactors.
- [x] 4.1 **PR 4**: Test exact class/family/type/definition resolution and atomic cardinality in `internal/postgres/resource_repository_crud.go` (debts 2, 3).
- [ ] 5.1 **PR 5**: Characterize then extract state, transitions, persistence mapping, and presentation across `internal/tui/resource_editor*.go`, preserving behavior (debt 10).
- [ ] 6.1 **PR 6**: Add explicit `Deactivate`/`Reactivate`, inactive discovery, active-only defaults, and visible state in app, CRUD, and TUI dispatch (debts 5, 6).
- [ ] 7.1 **PR 7**: Replace per-row reads with bounded set hydration in `internal/postgres/resource_repository_search.go`; prove query-count and parity scenarios (debt 9).
- [ ] 8.1 **PR 8**: Add `SearchCriteria`/`ResourcePage` navigation and TUI filter/selection preservation in domain, app, repository ports, and TUI (debt 12).
- [ ] 9.1 **PR 9 / final gate**: Correct Resource Master comments and `docs/architecture/catalog-source-of-truth.md`; run full verification and smoke (debt 11).

### Current apply slice

PR 4 / checkbox 4.1 is complete in this apply run after PR 3 merged; PR 5+ remain ordered follow-ups and are out of scope.
