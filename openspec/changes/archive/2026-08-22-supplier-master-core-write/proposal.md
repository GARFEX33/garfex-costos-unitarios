# Expose public WRITE on the Supplier Master Core, starting with Supplier Create

| Session setting | Value |
| --- | --- |
| execution_mode | auto |
| artifact_store | hybrid |
| delivery_strategy | ask-on-risk |
| review_budget_lines | 400 |

## Intent

`suppliercore/` shipped read-only (archived `supplier-master-core-read`). Go forbids importing `internal/` from outside the module, so **no external consumer can create a supplier today**. This is the first Writer slice of a per-operation graduation series that mirrors the archived resourcecore precedent (`resource-master-core-write` → `-update` → `-lifecycle`). It wraps the existing authority `internal/modules/suppliers/app.Service.CreateSupplier`; no new business behavior is invented.

## User outcome

An external Go consumer builds a `suppliercore.Writer` over `internal/bridge/suppliercore.Adapter`, creates one supplier, and receives the persisted `suppliercore.Supplier` or a stable public `Error` with no PostgreSQL detail. No other write operation is exported or discoverable.

## Binding decisions (settled — not reopened here)

| Area | Decision |
| --- | --- |
| Slice boundary | Supplier + Create only. Branch/Contact writes and Update/Lifecycle are separate later slices. |
| HardDelete | Never proposed for this module: `internal/modules/suppliers` has **no** hard-delete capability for Supplier, Branch, or Contact. There is nothing to wrap, so this is an absence, not deferred work. |
| Actor | `SupplierWriteRequest.Actor string` is added, mirroring resourcecore, and shape-validated as non-blank. The bridge passes it through the existing module-neutral `internal/core.WithActor(ctx, actor)`; it is diagnostic-only and never persisted **in this slice**, but is explicitly documented as a foreseeable audit-data seed — a later change may persist it, and that would be an extension of this field's intent, not a surprise scope change. |
| CAS / Revision | Not added. No entity carries a revision and no internal method accepts `ExpectedRevision`. The public contract therefore inherits the same lost-update race the internal `UpdateSupplier`/`Set*Active` paths already have; this must be documented for the later Update/Lifecycle slices. |
| Errors | Reuse the existing five-category `suppliercore.Error`/`ErrorCode`. No new code. Create reaches four: `INVALID_ARGUMENT` (shape), `VALIDATION` (`domain.NewSupplier` requires trade name, legal name, or tax identifier), `CONFLICT` (`domain.ErrTaxIdentifierConflict`), `INTERNAL`. `NOT_FOUND` is unreachable from Create and stays read-only-proven. |
| Bridge | Extend the existing `internal/bridge/suppliercore.Adapter` with a narrow `serviceWriter` seam beside `serviceReader`; one `Adapter` implements both `ReadCapabilities` and `WriteCapabilities`. It translates and delegates only — re-implementing `CreateSupplier`'s validation or the meaningful-identifier rule is a BLOCKER (guardrail established by `resource-master-core-write-harddelete`). |
| Compiled surface | `WriteCapabilities` declares exactly `CreateSupplier`. A reflection guard `TestWriter_NoUngraduatedMethodExported` replicates `resourcecore/writer_test.go:216`, asserting the exact method count/names and that `Writer` exports no ungraduated method. No stubs. |
| Request shape | `Active` is **not** a request field: `domain.NewSupplier` always creates active. Request carries `Actor` plus the five `domain.SupplierDetails` fields; the result projects through the existing public `Supplier` DTO. |
| Composition | Library-only. Nothing in this repo constructs a `suppliercore.Writer`; no wiring, migration, or interface work is included. |

## Capabilities

### New Capabilities

- None. Public write behavior extends the existing capability.

### Modified Capabilities

- `supplier-master-core`: add normative requirements for a public write contract, Supplier Create graduation, `Actor` on the write surface (the existing "No Actor, Revision, or HardDelete on the **read** surface" requirement is narrowed, not contradicted), write-direction shape validation and defensive copying, the four Create-reachable error categories, and the permanent absence of HardDelete.

## Scope

### In scope

- `suppliercore/write_types.go`: `SupplierWriteRequest` (+ clone helper mirroring `CloneSupplier`).
- `suppliercore/writer.go`: `Writer`, `WriteCapabilities`, `NewWriter` with a nil-capability `INVALID_ARGUMENT` guard mirroring `NewReadOnly`, and shape-only validation.
- `internal/bridge/suppliercore`: `serviceWriter` seam + `Adapter.CreateSupplier`, with field-by-field completeness against `domain.SupplierDetails`.
- Error-category coverage for the four Create-reachable codes, with no driver leakage.
- External-package test proving a consumer creates a supplier without importing `internal`.
- Reflection guard for the compiled write surface; `doc.go` updated from "READ only" to "read plus Supplier Create".

### Non-goals

- Branch and Contact writes (later slices).
- `UpdateSupplier`, `DeactivateSupplier`, `ReactivateSupplier` (later slices).
- Any hard delete, ever — no internal capability exists to wrap.
- Adding `Revision`/CAS to any entity, or fixing the internal lost-update race.
- New error codes, new services, a second translation site, or changes to internal business rules.
- Any composition root, transport, or consumer wiring.

## Affected areas

| Area | Impact | Description |
| --- | --- | --- |
| `suppliercore/write_types.go` | New | Create request DTO. |
| `suppliercore/writer.go` | New | `Writer`, `WriteCapabilities`, shape validation. |
| `suppliercore/copy.go` | Modified | Defensive copy for the write request. |
| `suppliercore/doc.go` | Modified | Shipped-contract and Actor sections. |
| `suppliercore/errors.go` | Unchanged | Five codes already exist; only reachability tests. |
| `internal/bridge/suppliercore/adapter.go` | Modified | `serviceWriter` seam, `CreateSupplier`, write-safe error mapping. |
| `internal/modules/suppliers/**` | Unchanged | Authority already exists. |
| `internal/core/diagnostics.go` | Unchanged | `WithActor` already exists and is module-neutral. |
| Tests | New/Modified | `writer_test.go`, `external_test.go`, `bridge/suppliercore/adapter_test.go`. |

## Architecture impact

Dependency direction is unchanged: consumer → `suppliercore` → `internal/bridge/suppliercore` → `app.Service` → domain → PostgreSQL. The `Writer` is a translation and shape-validation boundary, never an authority. `suppliercore` keeps zero `internal` imports.

## Open questions

Resolved with user (2026-08-21):
1. **Empty-content Create**: delegate to the domain. Shape validation stays "Actor is non-blank" only; a request with no trade name, legal name, or tax identifier reaches `domain.NewSupplier` and returns `VALIDATION`, not `INVALID_ARGUMENT` at the boundary. Single authority for the business rule.
2. **Actor semantics**: diagnostic-only and unpersisted in this slice, but explicitly documented as a foreseeable audit-data seed for a later change (see Actor decision above) — not a silent scope surprise if persisted later.
3. **Duplicate tax identifier**: plain `CONFLICT` on `ErrTaxIdentifierConflict`. No find-or-create/upsert affordance — that would be new business behavior, out of scope for a translation boundary.

Still open, for `sdd-design` to resolve (implementation detail, not product-facing):
4. `mapError`'s default branch returns the literal message `"supplier master read failed"` (`adapter.go:290`). A write crossing it would report a read failure. Choose in design: make the message operation-neutral (touches the existing read behavior string) or add a sibling `mapWriteError`. Either is safe against the spec, which constrains only leakage.
5. `SupplierWriteRequest` is all value-typed fields, so a `CloneSupplierWriteRequest` is a by-value no-op. Adopt it for symmetry with `CloneSupplier` (same shape) or omit it as an abstraction with no aliasing to prevent — a `golang-hexagonal` "no abstraction without a real boundary" call.

## Risks and mitigations

| Risk | Likelihood | Mitigation |
| --- | --- | --- |
| Bridge re-implements the meaningful-identifier rule | Med | Bridge translates and delegates only; any conditional business decision is a BLOCKER. |
| "Writer exists" read as "all writes are safe" | Med | Compile only `CreateSupplier`; reflection guard + `doc.go` state what is unavailable. |
| Silent field loss on the write path | Med | `GARFEX_STRICT` field-completeness gate against `domain.SupplierDetails`, field by field. |
| Actor becomes implied audit/authorization | Med | Documented as diagnostic-only and unpersisted in this slice, with explicit note that it is a foreseeable audit-data seed for a future change — not authorization, and not a silent scope surprise if persisted later. |
| No-CAS contract mistaken for a safe concurrent-write surface | Med | `doc.go` states the inherited lost-update race explicitly. |
| Slice exceeds the 400-line budget | Med | Forecast below; `ask-on-risk` decides chaining before apply. |

## Rollback boundaries

- Reverting this change removes only the write surface; the read contract and its tests stay fully available.
- No migration, schema, wiring, or internal service rollback is involved — nothing new is persisted or composed.
- If Create's evidence regresses, withdraw `CreateSupplier` alone.
- No rollback may weaken error sanitization or the compiled-surface guard to restore green.

## Success criteria

- [ ] An external Go package creates a supplier through `suppliercore` with no `internal` import.
- [ ] `suppliercore` compiles with exactly one write method; the reflection guard fails if any other is exported or stubbed.
- [ ] All five `domain.SupplierDetails` fields reach the internal call, or carry a one-line omission rationale.
- [ ] `INVALID_ARGUMENT`, `VALIDATION`, `CONFLICT`, and `INTERNAL` are each proven reachable from Create; `NOT_FOUND` is documented as unreachable.
- [ ] No public error string, type, or unwrap chain exposes pgx, SQLSTATE, constraint, table, or column detail.
- [ ] `Actor` is required, passed via `core.WithActor`, never persisted, and absent from every returned DTO.
- [ ] `doc.go` states the graduated operation, the unavailable ones, the permanent absence of HardDelete, and the no-CAS race.
- [ ] `gofmt -l .`, `go vet ./...`, `golangci-lint run ./...`, `go test ./... -count=1` pass; CI-only race/build reported separately.
- [ ] Strict red-green-refactor evidence preserved.

## Delivery forecast

Estimated 300–450 authored lines (one entity, one operation; the read slice covered seven methods across three entities). This sits at the edge of the 400-line budget. Under `ask-on-risk`, `sdd-tasks` must forecast explicitly and, if the estimate crosses 400, propose a two-unit chain: (1) public write contract + guard tests, (2) bridge seam + error coverage + docs.
