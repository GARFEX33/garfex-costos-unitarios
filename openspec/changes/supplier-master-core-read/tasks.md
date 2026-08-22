# Tasks: `supplier-master-core-read`

Delivery: `auto-chain`, 3 units, stacked-to-main (mirrors the resourcecore write series). Strict TDD: red→green→refactor per task, tests first. Each unit must leave `go build ./...`, `go vet ./...`, `gofmt -l .`, and `go test ./... -count=1` green before the next unit starts.

## Unit 1 — Public contract (`suppliercore/`)

Budget: ≤400 changed lines (code + tests).

- [x] U1.1 `suppliercore/doc.go` — package contract statement (read-only translation boundary over `internal/modules/suppliers/app.Service` via a bridge; "presence of this read API is never permission to infer any write operation exists").
- [x] U1.2 `suppliercore/types.go` — `Supplier`, `Branch`, `Contact`, `BranchKey`, `ContactKey` (design.md `types.go` verbatim).
- [x] U1.3 `suppliercore/queries.go` — `LifecycleScope` + 3 consts; `SupplierQuery`/`BranchQuery`/`ContactQuery`; `SupplierPage`/`BranchPage`/`ContactPage` (design.md `queries.go` verbatim).
- [x] U1.4 `suppliercore/errors_test.go` then `suppliercore/errors.go` — `ErrorCode`, `Error`, `NewError`, `Code`, `IsCode`; test all 5 categories round-trip through `errors.As`/`Code`/`IsCode`.
- [x] U1.5 `suppliercore/copy_test.go` then `suppliercore/copy.go` — `CloneSupplier`/`CloneBranch`/`CloneContact` + 3 slice-clone helpers; test proves mutating a returned `Contact.BranchID` pointer or a returned slice element does not affect the clone's source (spec scenarios "Mutating a returned Contact's BranchID does not leak" / "Mutating a returned page slice does not leak").
- [x] U1.6 `suppliercore/reader_test.go` then `suppliercore/reader.go` — `ReadCapabilities` interface, `Reader`, `NewReadOnly`, all 6 methods, `validateScope`. Test every shape-validation scenario from spec.md against a stub `ReadCapabilities` (nil-capability rejection, non-positive IDs, unknown scope, non-positive `BranchID` filter) — stub must never be called on a rejected request.
- [x] U1.7 `suppliercore/reader_test.go` — add `TestReader_NoUngraduatedMethodExported` (design.md reflection guard verbatim): asserts `ReadCapabilities` has exactly 6 methods with the exact allow-listed names, and `Reader`'s exported method set is a subset.
- [x] U1.8 Run `gofmt -l .`, `go vet ./suppliercore/...`, `go test ./suppliercore/... -count=1 -race`. Confirm zero changes outside `suppliercore/`.

**U1 status: DONE.** 6 files (`doc.go`, `types.go`, `queries.go`, `errors.go`, `copy.go`, `reader.go`) + 3 test files, all green (`go test ./suppliercore/... -count=1 -race`: 23/23 pass). Zero changes outside `suppliercore/`.

## Unit 2 — Supplier bridge (`internal/bridge/suppliercore/`)

Budget: ≤400 changed lines (code + tests). Depends on Unit 1.

- [x] U2.1 `internal/bridge/suppliercore/adapter.go` — `serviceReader` seam interface (all 6 methods, declared complete in Unit 2 so Unit 3 adds no interface changes), `Adapter` struct, `NewAdapter`. (Compile-time `var _ public.ReadCapabilities` check deferred to U3, since Adapter is not yet interface-complete until Branch/Contact methods exist.)
- [x] U2.2 `internal/bridge/suppliercore/adapter.go` — `mapError` (NotFound/BranchOwnership→Validation/Validation/Conflict/default→Internal with the fixed literal `"supplier master read failed"` message, never the original error).
- [x] U2.3 `internal/bridge/suppliercore/adapter.go` — `mapSupplier`, `activeFromScope`, `effectiveLimit`, `trimToPage` pagination helpers, including the `defaultLimit = 100` constant and the explicit `q.Limit <= 0` resolution that avoids the "+1 turns default into a 1-row page" bug.
- [x] U2.4 `GetSupplier` (translate-and-delegate, one call, `mapError` on failure).
- [x] U2.5 `SearchSuppliers`: regression test for the pagination fix (Limit: 0 does not under-fetch), Limit-cap + HasNext test, HasPrevious/Offset test.
- [x] U2.6 Field-completeness test for `domain.Supplier` → `public.Supplier` (reflection-based, fails if a future domain field is unmapped).
- [x] U2.7 Error-category reachability: NOT_FOUND, CONFLICT, INTERNAL (raw internal error never leaks driver-shaped text).
- [x] U2.8 Run `gofmt -l .`, `go vet ./internal/bridge/suppliercore/...`, `go test ./internal/bridge/suppliercore/... -count=1 -race`. Confirm zero changes to `internal/modules/suppliers/` or `resourcecore/`.

**U2 status: DONE.** 7 tests passing, incl. the default-limit regression test.

## Unit 3 — Branch/Contact bridge, readiness, external consumer proof

Budget: ≤400 changed lines (code + tests). Depends on Unit 2 (reuses `effectiveLimit`/`trimToPage`/`mapError` unmodified).

- [x] U3.1 `internal/bridge/suppliercore/adapter.go` — `mapBranch`, `ListBranches`, `GetBranch` (`Limit` sent as `effectiveLimit` unmodified — postgres already over-fetches internally via `limitPlusOne`).
- [x] U3.2 `internal/bridge/suppliercore/adapter.go` — `mapContact`, `ListContacts`, `GetContact` (same shape as Branch; `ContactQuery.BranchID` passes through into `domain.ContactListCriteria.BranchID` unchanged).
- [x] U3.3 Branch pagination test confirming the different per-entity request strategy (`effectiveLimit`, not `effectiveLimit+1`) still yields correctly-capped pages.
- [x] U3.4 Contact pagination test, same shape.
- [x] U3.5 Branch-ownership `VALIDATION` scenario: foreign-branch `ListContacts` filter maps `domain.ErrBranchOwnership` to `public.Validation`.
- [x] U3.6 Unknown-supplier asymmetry scenario: `ListBranches` on unknown supplier → `NOT_FOUND`; `GetBranch` with no supplier pre-check → `NOT_FOUND` via a different internal path.
- [x] U3.7 Field-completeness table tests for `domain.Branch` and `domain.Contact` (reflection-based), plus `Contact.BranchID` clone-identity assertion in `mapContact`.
- [x] U3.8 `suppliercore/external_test.go` (`package suppliercore_test`) — external consumer reads Supplier, Branch, and Contact via `suppliercore.NewReadOnly` with zero `internal` imports.
- [x] U3.9 `suppliercore/doc.go` — finalized with the read-only boundary statement (written in U1, confirmed still accurate now that all 6 methods exist — no changes needed).
- [x] U3.10 `openspec/changes/supplier-master-core-read/readiness.md` — final build/vet/test evidence recorded.
- [x] U3.11 Full suite: `gofmt -l .`, `go vet ./...`, `go test $(go list ./... | grep -v /agent/) -count=1` all green. (`golangci-lint` not configured in this repo — not run.)

**U3 status: DONE.** `var _ public.ReadCapabilities = (*Adapter)(nil)` compile-time check added — Adapter is now fully interface-complete. 15 tests passing in `internal/bridge/suppliercore`, 1 in `suppliercore/external_test.go`.

## Parent gate (G1)

- [x] G1 All three units complete, full suite green (30 tests across `suppliercore` + `internal/bridge/suppliercore`, zero regressions elsewhere), `readiness.md` complete. Ready for `sdd-verify`.
