# Expose a public READ contract for the Supplier Master Core

## Intent

`internal/modules/suppliers` is the authoritative Supplier Master (Supplier, Branch, Contact), and Go forbids importing `internal/` from outside the module — so **no external consumer can read a supplier, branch, or contact at all today**. `resourcecore` already solved this exact problem for the Resource Master; the Supplier Master has no equivalent.

This change delivers the first slice of that graduation: a read-only public package mirroring `resourcecore`'s Reader pattern, covering all three entities at once. Reads carry no concurrency-control, attribution, or authorization ambiguity, so this slice ships the DTO and error vocabulary that the later Writer series will extend rather than reshape.

No new business behavior is invented. The internal `suppliers/app.Service` remains the sole authority; this change delivers the public mirror plus its sole bridge translation.

## User outcome

An external Go consumer imports `suppliercore`, constructs a Reader through the module-owned bridge, and reads the complete Supplier Master — get/search Supplier, list/get Branch, list/get Contact (supplier-scoped, optionally branch-filtered) — receiving package-owned DTOs and stable public errors that leak no PostgreSQL detail. No write operation is exported or discoverable.

## Binding decisions

| Area | Decision |
| --- | --- |
| Package | New `suppliercore/` at repo root (mirrors `resourcecore/`'s location and role). Sole bridge: new `internal/bridge/suppliercore.Adapter`. |
| Compiled surface | `ReadCapabilities` declares exactly 6 methods: `GetSupplier`, `SearchSuppliers`, `ListBranches`, `GetBranch`, `ListContacts`, `GetContact`. `Reader` exports only those. A reflection-based surface guard test (the `resourcecore/writer_test.go:216` pattern) makes any ungraduated method a compile-visible, provable gap. |
| Boundary role | `Reader` validates request **shape** only (positive IDs, known lifecycle scope, non-nil capability), delegates, then defensively clones every returned value. It is never a second business authority. |
| DTO ownership | Package-owned `Supplier`/`Branch`/`Contact`, keys, queries, and pages. Never aliases an internal domain type. `Contact.BranchID *int64` is deep-copied on every crossing. |
| Parent scoping | Branch and Contact are `SupplierID`-scoped (no `resourcecore` analogue): every key, query, and record DTO carries `SupplierID` explicitly. |
| Error surface | Small, purpose-built `Error`/`ErrorCode`, **not** `resourcecore`'s fixed-15 taxonomy. Proposed set (5): `NOT_FOUND`, `VALIDATION`, `CONFLICT` (mirroring `domain.ErrNotFound`/`ErrValidation`/`ErrConflict`), plus `INVALID_ARGUMENT` for boundary shape rejection and `INTERNAL` as the unmappable-cause fallback. |
| Error mapping | Internal errors are `fmt.Errorf("%s: %w", op, err)`-wrapped, so the bridge classifies with `errors.Is` against the domain sentinels. Verified gap: `postgres.wrapRead` only wraps — it does **not** sanitize, unlike the write path's `mapWriteError` — so a raw pgx error can reach the bridge on a read. Mapping to `INTERNAL` with no driver detail is normative, not incidental. |
| Lifecycle scope | Public `LifecycleScope` (`ACTIVE`/`INACTIVE`/`ALL`) maps to the internal `Active *bool` (nil = all). All three values are honestly reachable for all three entities. |
| CAS / `Revision` | **Settled: none.** No `Revision` field exists on Supplier, Branch, or Contact, and none will be added. The public contract reflects current internal reality. Not an open risk. |
| `Actor` | Write-side concept only. Absent from this read slice. A later Writer change will introduce it (diagnostic-only attribution, `resourcecore`'s convention), including the internal capability it needs. |
| Composition root | Verified: no `cmd/` exists (removed in `c38aacf`), and `resourcecore.NewAdapter`/`NewWriter` have zero non-test callers. `suppliercore` ships as a library-only contract nothing in this repo constructs — the same, expected position `resourcecore` occupies. Not a defect and not in scope to fix. |

## Correction to the briefed scope (verified, needs acknowledgement)

`ensureBranchOwnership` was framed as write-only. **It is not.** `app/contact.go:41` calls it inside `ListContacts` whenever `criteria.BranchID` is set, so a branch-filtered contact read on a foreign branch returns `ErrBranchOwnership` (which unwraps to `ErrValidation`).

This does not change the non-goal — the public package and bridge still must not re-implement the invariant. It changes two things: the read path **must** surface `VALIDATION` as a reachable category, and `Reader.ListContacts` shape validation may check only that `BranchID` is positive, never that it belongs to the supplier.

Also verified: `GetBranch`/`GetContact` validate ID shape only and do **not** confirm the supplier exists, while `ListBranches`/`ListContacts` call `GetSupplier` first and so return `NOT_FOUND` for an unknown supplier. The public contract mirrors this asymmetry rather than normalizing it.

## Capabilities

### New Capabilities

- `supplier-master-core`: the public, consumer-neutral read contract for the Supplier Master — Reader surface, DTO ownership and defensive copying, request-shape validation, supplier-scoped child reads, lifecycle scope projection, pagination semantics, and the public error category surface.

### Modified Capabilities

- None. `resource-master-core` is untouched.

## Scope

### In scope

- `suppliercore/`: `doc.go` (contract statement), `types.go` (Supplier/Branch/Contact DTOs, keys), `queries.go` (queries, pages, `LifecycleScope`), `errors.go` (`Error`/`ErrorCode`, `IsCode`), `copy.go` (clone helpers), `reader.go` (`ReadCapabilities`, `Reader`, `NewReadOnly` with an `INVALID_ARGUMENT` nil guard, shape validation).
- `internal/bridge/suppliercore/adapter.go`: narrow `supplierReader`/`branchReader`/`contactReader` seams over `suppliers/app.Service`, translation only, plus `errors.Is`-based error mapping.
- Pagination derivation at the bridge (see Architecture impact).
- Field-completeness evidence: every internal read field is projected or carries a one-line rationale for omission.
- Reachability evidence for every declared error category.
- External-package tests proving a consumer reads all three entities with no `internal` import.
- A reflection-based compiled-surface guard asserting the read-only surface.

### Non-goals

- **Any** write operation on the public surface (Create, Update, Deactivate, Reactivate) — a separate later series, Supplier-first, mirroring `resourcecore`'s sequencing. Ungraduated operations are not stubbed.
- `Actor` on any public type.
- `Revision`/CAS on any entity, public or internal.
- HardDelete — no internal capability exists for any of the three entities; it will never be proposed.
- Re-implementing `ensureBranchOwnership`, or any other business rule, in `suppliercore` or the bridge.
- Any change to `internal/modules/suppliers` business logic, `domain.Repository`, or SQL.
- Removing the existing direct `*app.Service` coupling held by `internal/app/catalogo` and `internal/app/recursos`. The bridge adds a second, parallel path; it replaces nothing.
- Normalizing the internal `AddBranch`/`AddContact` vs `CreateSupplier` naming inconsistency (write-side, and not touched here).
- Any composition root, transport, HTTP, MCP, or consumer-specific concern.

## Architecture impact

Dependency direction is unchanged and additive: consumer → `suppliercore` (public contract) → `internal/bridge/suppliercore` (sole translation) → `suppliers/app.Service` → domain → ports → PostgreSQL. `suppliercore` imports no `internal` package.

**Pagination (verified asymmetry, must be handled explicitly).** `postgres` uses `limitPlusOne` for `ListBranches`/`ListContacts` but plain `limit` for `SearchSuppliers`, and nothing in the postgres or app layer trims the extra row — so branch/contact list reads currently return up to `Limit+1` records to their callers. The bridge therefore derives `HasNext` differently per entity without touching internal code: it requests `Limit+1` for suppliers (the `resourcecore` adapter's existing technique) and `Limit` for branches/contacts, then trims to `Limit` in both cases. The internal `limit(0) → defaultLimit` behavior must also be documented in the public contract rather than silently inherited.

## Risks and mitigations

| Risk | Likelihood | Mitigation |
| --- | --- | --- |
| Raw pgx/SQLSTATE detail leaks through the unsanitized `wrapRead` read path | High | Normative requirement: no public error string, type, or unwrap chain exposes driver detail; prove it with a read-path test injecting a pgx-shaped error. |
| Untrimmed `Limit+1` rows leak into the public page | High | Bridge trims per entity and asserts exact page length; test both entities at an exact page boundary. |
| "The package exists" read as "writes are coming/safe" | Med | `doc.go` states the read-only boundary explicitly; the reflection surface guard makes the compiled surface provable. |
| Public DTO field silently dropped | Med | Field-completeness gate per entity: mapped, or omitted with a written rationale. |
| Error taxonomy reshaped by the later Writer series | Med | Fix `Error`/`ErrorCode` shape now; the Writer adds categories additively, never renumbers or redefines. |
| `Contact.BranchID` pointer aliased across the boundary | Med | Deep-copy in every clone helper; test that mutating a returned pointer changes nothing. |
| Slice exceeds the 400-line review budget | High | Auto-chained delivery plan below. |
| Bridge grows into a second business authority | Low | Bridge methods translate and delegate only; any conditional business decision there is a BLOCKER. |

## Rollback boundaries

- `suppliercore/` and `internal/bridge/suppliercore/` are entirely new; reverting deletes two new packages and touches nothing existing.
- Zero changes to `internal/modules/suppliers`, `resourcecore`, migrations, or schema — no database, migration, or composition rollback is possible or needed.
- Nothing constructs the Reader in production today, so a revert has no runtime blast radius.
- Each auto-chain unit is independently revertible while preserving accepted earlier units.
- No rollback may weaken error sanitization or defensive copying to restore green.

## Delivery plan

Forecast 700–1,000 authored lines. Anchor: the `resourcecore` READ contract cost 1,348 lines for 7 methods, but most of that was the polymorphic `Value`/descriptor machinery, which three flat entity structs do not need. Still over the 400-line budget, so `auto-chain` (project default) with three ordered units:

1. **Public contract** — DTOs, keys, queries, pages, `Error`/`ErrorCode`, clone helpers, `Reader`/`ReadCapabilities` (6 methods), shape validation, unit tests, reflection surface guard.
2. **Supplier bridge** — `supplierReader` seam, `Adapter.GetSupplier`/`SearchSuppliers`, `errors.Is` mapping, `Limit+1` page derivation, field completeness vs `domain.Supplier`, error-category coverage.
3. **Branch/Contact bridge and readiness** — `branchReader`/`contactReader` seams, four methods, branch/contact page trimming, `VALIDATION` coverage for the branch-ownership read path, external-consumer test, `doc.go` and readiness record, full verification.

Each unit targets ≤400 changed lines, follows strict TDD, and must leave the tree green.

## Success criteria

- [ ] An external Go package reads a supplier, its branches, and its contacts through `suppliercore` with no `internal` import.
- [ ] `suppliercore` compiles with exactly the 6 read methods; no write method is exported or stubbed, proven by the reflection surface guard.
- [ ] Every declared error category is proven reachable and distinct, including `VALIDATION` via the branch-ownership read path.
- [ ] No public error string, type, or unwrap chain exposes pgx, `PgError`, SQLSTATE, constraints, tables, columns, or PostgreSQL messages.
- [ ] Pages contain at most `Limit` records for all three entities, with correct `HasNext`/`HasPrevious` at an exact page boundary.
- [ ] Mutating any returned DTO, slice, or `BranchID` pointer changes nothing observable on a subsequent read.
- [ ] Every internal read field is projected into a public DTO or carries a written omission rationale.
- [ ] `internal/modules/suppliers` and `resourcecore` have zero changed lines.
- [ ] `gofmt -l .`, `go vet ./...`, `golangci-lint run ./...`, and `go test ./... -count=1` pass; CI-only race/build checks reported separately.
- [ ] Strict red-green-refactor evidence preserved for every unit.

## Proposal question round

The executor could not ask interactively. These are the assumptions that need user review before `sdd-spec`/`sdd-design`:

1. **Branch-ownership on the read path** — the briefed premise that `ensureBranchOwnership` is write-only is factually wrong (`app/contact.go:41`, inside `ListContacts`). The proposal assumes: keep the non-goal (never re-implement it), but treat `VALIDATION` as a reachable read-path category. Confirm.
2. **Error taxonomy size** — the decided set was Validation/NotFound/Conflict. The proposal adds `INVALID_ARGUMENT` (boundary shape rejection and nil capability, which never reach the internal service) and `INTERNAL` (unmappable cause), for 5 total. Is 5 acceptable, or should shape rejection reuse `VALIDATION`?
3. **Pagination shape** — the proposal exposes `SupplierPage`/`BranchPage`/`ContactPage` with `HasNext`/`HasPrevious`, mirroring `resourcecore`, and absorbs the verified `limitPlusOne` asymmetry at the bridge. The simpler alternative is returning plain slices and no page metadata. Pages are the recommendation; confirm the extra bridge logic is wanted.
4. **`limit(0) → defaultLimit` (100)** — should the public contract document and inherit this default, or reject `Limit <= 0` as `INVALID_ARGUMENT` at the boundary?
5. **Package name** — `suppliercore` at repo root, matching `resourcecore`. Confirm, or name it otherwise.
