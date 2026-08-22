# Readiness: `supplier-master-core-read`

## Summary

Implemented directly (all 5 planning phases + all 3 implementation units), not delegated to sub-agents — `sdd-spec` and `sdd-design` sub-agents both failed with API session-limit errors before producing output, extending the previously-recorded sdd-apply-only rate-limit pattern to planning phases as well. All work from `sdd-propose` onward was done in the orchestrator thread directly.

## What shipped

- `suppliercore/` (repo root): `doc.go`, `types.go`, `queries.go`, `errors.go`, `copy.go`, `reader.go` + `errors_test.go`, `copy_test.go`, `reader_test.go`, `external_test.go`. Public read-only contract: `Reader`/`ReadCapabilities` with 6 methods (`GetSupplier`, `SearchSuppliers`, `ListBranches`, `GetBranch`, `ListContacts`, `GetContact`), 5-category `Error`/`ErrorCode`, defensive cloning.
- `internal/bridge/suppliercore/`: `adapter.go`, `adapter_test.go`. `Adapter` implementing `public.ReadCapabilities` over a narrow `serviceReader` seam onto `internal/modules/suppliers/app.Service`, with `errors.Is`-based error mapping and pagination-asymmetry handling.

## Verified bug caught and fixed before shipping

Naively mirroring resourcecore's `Limit+1` bridge technique for `SearchSuppliers` would have broken the default page size: a caller passing `Limit: 0` (meaning "use the default") would have caused the bridge to request `Limit: 1` from the internal service, returning a 1-row page instead of the intended default-100 page. Fixed by resolving `effectiveLimit(q.Limit)` (bridge's own `defaultLimit = 100` constant) before deciding whether to over-fetch. Covered by an explicit regression test: `TestAdapter_SearchSuppliers_DefaultLimitDoesNotUnderFetch`.

Also confirmed and preserved as designed: Branch/Contact repositories already over-fetch by one row internally (`limitPlusOne` in `postgres/branch.go` and `postgres/contact.go`), so the bridge requests `effectiveLimit` unmodified for those two entities — an asymmetric-by-necessity, not accidental, difference from Supplier's request strategy. Covered by `TestAdapter_ListBranches_RequestsEffectiveLimitUnmodified` / `TestAdapter_ListContacts_RequestsEffectiveLimitUnmodified`.

## Test evidence

- `suppliercore`: 24 tests, all passing (`go test ./suppliercore/... -count=1 -race`).
- `internal/bridge/suppliercore`: 15 tests, all passing (`go test ./internal/bridge/suppliercore/... -count=1 -race`).
- `gofmt -l suppliercore/ internal/bridge/suppliercore/`: no output (clean).
- `go vet ./suppliercore/... ./internal/bridge/suppliercore/...`: clean.
- `go build ./suppliercore/... ./internal/... ./resourcecore/...`: clean.
- `go test $(go list ./... | grep -v /agent/) -count=1`: all green. (`agent/skills/golang-cli/assets/examples` fails on missing third-party deps — pre-existing, untracked, unrelated to this change; not part of the module's real code.)
- `golangci-lint`: not configured in this repository — not run.

## Scope discipline confirmed

- Zero changes to `internal/modules/suppliers/`, `resourcecore/`, migrations, or schema.
- No `Actor` field, no `Revision`/CAS field, no write method, no HardDelete anywhere in the new surface — confirmed by `TestReader_NoUngraduatedMethodExported` (reflection-based compiled-surface guard) and `TestAdapter_ImplementsReadCapabilities` (compile-time interface assertion).
- Branch-ownership invariant (`ensureBranchOwnership`) is enforced only by the internal service and observed, never re-implemented, by the bridge — confirmed by `TestAdapter_ListContacts_ForeignBranchYieldsValidation`.
- No PostgreSQL/pgx detail leaks through any public error — confirmed by `TestAdapter_GetSupplier_RawInternalErrorNeverLeaks`.

## Ready for sdd-verify
