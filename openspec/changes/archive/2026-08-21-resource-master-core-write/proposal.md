# Expose public WRITE on the Resource Master Core, starting with Create

## Intent

`resourcecore/` is READ-ONLY. Go forbids importing `internal/` from outside the module, so today **no external consumer can create anything** in the Resource Master: every write path is reachable only from `internal/tui`, an in-repo driving adapter. The maintainer has decided this repository becomes Core-only under strict hexagonal architecture — `internal/tui/` and `cmd/garfex/` are deleted once the Core's public contract is complete, and an external product (PI) takes over the interface role. Public WRITE is the hard blocker on that deletion.

This change opens that gate for the first operation only. The internal write authority already exists and is production-hardened (`catalogo.Service.Create`/`CreateV2`, `recursos.Service.Create`, CAS-aware V2 repositories). No new business behavior is invented here: this change delivers the *public mirror* plus its *sole bridge translation*, with the evidence that per-operation WRITE readiness demands.

## User outcome

An external Go consumer can import `resourcecore`, construct a writer through the module-owned bridge, and create both catalog records (all 11 registered kinds) and resources — receiving the persisted record, its durable identity, and its monotonic revision back, or a stable public GARFEX error that leaks no PostgreSQL detail. `Update`, `Deactivate`, `Reactivate`, and `HardDelete` remain unavailable and are not discoverable on the public surface.

## Binding decisions

| Area | Decision |
| --- | --- |
| Graduation model | Per-operation WRITE graduation is **inherited and binding** from the archived `stabilize-resource-master-core` spec ("Separate read and per-operation write readiness"), never superseded. It is not reopened here. |
| Contract shape | Design the complete eventual `Writer`/`WriteCapabilities` shape now — naming, request-DTO conventions, CAS argument placement, return projection — so later operations extend rather than reshape it. Record that shape in `design.md`. |
| Compiled surface | The compiled `WriteCapabilities` interface and `Writer` methods declare **only `CreateCatalog` and `CreateResource`** in this change. Ungraduated operations are designed on paper, not stubbed in code: `golang-hexagonal` forbids abstractions for hypothetical futures, and a stub that returns "unimplemented" makes an unsafe operation discoverable. |
| Interface ownership | `WriteCapabilities` is a Core-owned seam for the module's own bridge; external consumers depend on `*Writer` methods. Adding a method per later slice is an accepted, per-slice change to that seam, not a consumer break. |
| Sole bridge | `internal/bridge/resourcecore.Adapter` remains the only package permitted to translate `internal/domain` ↔ `resourcecore` types. New narrow `catalogWriter`/`resourceWriter` seams parallel the existing reader seams. |
| Internal authority | Zero changes to `internal/app/catalogo` and `internal/app/recursos` business logic, and no new composition wiring — `postgres.NewResourceRepository` already satisfies `ResourceRepositoryV2`, and `catalogo.Service.Create` is already V2-backed in production. |
| Field/query completeness | The `GARFEX_STRICT` field-completeness gate applies in the write direction: every public request field MUST reach the internal command/record, or be omitted with a one-line rationale. A public field the domain cannot honor is a BLOCKER (`MISSING DOMAIN CRITERION`), never silently ignored — the `ResourceQuery.TypeCode` precedent must not repeat on write. |
| DTO discipline | Write requests are package-owned, reuse existing public `Value`/`AttributeValue`/`ResourceScope`/`ApplicabilityRule`, and are defensively copied on entry so a caller cannot mutate an in-flight request. Results project through the existing public `CatalogRecord`/`Resource` DTOs, including `Revision`. |
| Error surface | No new `ErrorCode`. Create must prove exactly 9 reachable categories: `INVALID_ARGUMENT`, `NOT_FOUND`, `DUPLICATE`, `INVALID_REFERENCE`, `VALIDATION`, `INTEGRITY`, `INVALID_CATALOG`, `UNAVAILABLE`, `INTERNAL`. `CONFLICT`, `IDENTITY_CONFLICT`, `INVALID_LIFECYCLE`, `REACTIVATION_IMPOSSIBLE`, `IN_USE`, `IMMUTABLE_CODE` are unreachable from Create and stay unproven (design.md corrected this list from 5 to 6 — `INVALID_LIFECYCLE` was initially missed). |
| Resource hard delete | Does not exist by design and will never be proposed. `recursos.Service.Delete` is a compatibility alias for deactivation. |
| Driving adapters | `cmd/garfex/` and `internal/tui/` are untouched. They hold zero references to `resourcecore` today, and this change introduces none. |
| Actor attribution | Every public write request carries an `Actor` (caller identity string, e.g. "PI" or a downstream user/service id) supplied by the consumer. The Core does not authenticate, authorize, or validate it — that is entirely the consumer's responsibility. `Actor` is **not persisted** with the created record (no new column, no migration, no `internal/domain`/`internal/postgres` change): the bridge passes it into the existing internal diagnostic seam (`internal/core/diagnostics.go`) alongside the operation and key, the same place technical error causes are already recorded, so every write is attributable in diagnostics without becoming durable business data. |

## Capabilities

### New Capabilities

- None. Public write behavior belongs to the existing capability.

### Modified Capabilities

- `resource-master-core`: add normative requirements for a public WRITE contract, Create graduation for catalog and resource, write-direction defensive copying and field completeness, and the write-error category surface — extending the existing "Separate read and per-operation write readiness" requirement rather than replacing it.

## Scope

### In scope

- Public write request DTOs in `resourcecore/` plus defensive-copy helpers in `resourcecore/copy.go`.
- `resourcecore` `Writer` + `WriteCapabilities` with a Create-only compiled surface and a nil-capability `INVALID_ARGUMENT` guard, mirroring `NewReadOnly`.
- Bridge `catalogWriter`/`resourceWriter` seams and `Adapter.CreateCatalog`/`Adapter.CreateResource` delegating to the existing application services.
- Category-coverage evidence for the 9 Create-reachable error codes across catalog and resource, with no PostgreSQL detail in strings, types, or unwrap chains.
- External-package tests proving a consumer creates records without importing any `internal` package.
- A per-operation readiness record for Create, in the style of the archived `readiness.md` gate.
- Documentation of which write operations are public and which remain unavailable.

### Non-goals

- `Update`, `Deactivate`, `Reactivate`, and catalog `HardDelete` on the public surface — each is a separate, later, per-operation-gated change with its own readiness evidence.
- Any resource hard-delete capability.
- Any change to `cmd/garfex/`, `internal/tui/`, or their deletion (a later, separate change).
- Any change to internal business rules, validation, CAS semantics, publication equivalence, `identity-v1`, or canonical presentation.
- New error codes, new services, new repositories, or a second translation site.
- Transport, HTTP, MCP, or PI-specific concerns.
- Cross-process refresh or multi-writer coherence; the one-writer topology is unchanged.

## Affected areas

| Area | Impact | Description |
| --- | --- | --- |
| `resourcecore/types.go` (or new `write_types.go`) | New | Create request DTOs reusing existing public value types. |
| `resourcecore/writer.go` | New | `Writer`, `WriteCapabilities`, constructor, request-shape validation. |
| `resourcecore/copy.go` | Modified | Defensive copies for write requests. |
| `resourcecore/errors.go` | Unchanged | All 15 codes already exist; only new reachability tests. |
| `internal/bridge/resourcecore/adapter.go` | Modified | Write seams and two new delegating methods. |
| `internal/core/errors.go` | Unchanged | `Map` already covers all categories. |
| `internal/core/diagnostics.go` | Modified | Accept and record the caller-supplied `Actor` alongside operation/key/cause; still non-public, still no persistence. |
| `internal/app/*`, `internal/postgres/*` | Unchanged | Write authority already built and wired. |
| `cmd/garfex/`, `internal/tui/` | Unchanged | Confirmed zero coupling; must stay zero. |
| Tests | New/Modified | `resourcecore/writer_test.go`, extended `external_test.go`, extended `internal/bridge/resourcecore/adapter_test.go`. |

## Architecture impact

Dependency direction is unchanged: consumer → `resourcecore` (public contract) → `internal/bridge/resourcecore` (sole translation) → application services → domain → ports → PostgreSQL. The `Writer` is a translation and shape-validation boundary, not an authority — it must not validate business rules, resolve references, decide lifecycle, or construct identity. `resourcecore` remains free of any `internal` import.

The public surface stops being read-only, which is the architectural precondition for retiring the in-repo driving adapters. After this change the Core exposes read plus create; the remaining operations are what still keep PI from fully replacing `internal/tui`.

## Risks and mitigations

| Risk | Likelihood | Mitigation |
| --- | --- | --- |
| "The type exists" implies "every method is safe" | Med | Compile only graduated methods; ungraduated operations live in `design.md`, not in code. |
| Silent field loss on the write path | High | Apply the `GARFEX_STRICT` field-completeness gate field-by-field; mapped, or commented as intentionally unexposed. |
| Public request field the domain cannot honor | Med | Report `MISSING DOMAIN CRITERION` and drop the field rather than accept-and-ignore. |
| Slice exceeds the 400-line review budget | High | READ-ONLY precedent cost 1,348 lines; plan an ordered auto-chain up front (below). |
| Public-contract churn across later slices | Med | Fix the full contract shape now; later slices add methods without reshaping DTOs. |
| Unproven error categories reached in production | Low | Restrict graduation to the 9 Create-reachable categories and record which remain unproven. |
| Uncommitted in-flight work in the tree masks a broken baseline | Med | Confirm a clean `git status` and green `go test ./... -count=1` before the first slice. |
| Bridge grows into a second business authority | Low | Bridge methods translate and delegate only; any conditional business decision there is a BLOCKER. |

## Rollback boundaries

- Each auto-chain unit is independently revertible while preserving accepted earlier units.
- The READ-ONLY contract stays fully available if any write slice is reverted; reverting write touches no read path.
- Reverting write requires no database, migration, or composition rollback — nothing new is persisted or wired.
- If Create's evidence later regresses, withdraw only `CreateCatalog`/`CreateResource`; read and any other graduated operation remain available (archived spec, "Failing evidence withdraws only affected write readiness").
- No rollback may weaken CAS, dependency guards, atomicity, publication equivalence, `identity-v1`, or error safety to restore green.

## Success criteria

- [ ] An external Go package creates a catalog record and a resource through `resourcecore` with no `internal` import.
- [ ] `resourcecore` compiles with `CreateCatalog`/`CreateResource` only — no ungraduated write method is exported or stubbed.
- [ ] Every public write-request field is mapped at the bridge or carries a one-line rationale for its omission.
- [ ] All 9 Create-reachable error categories are proven reachable and distinct for both catalog and resource.
- [ ] No public error string, type, or unwrap chain exposes pgx, `PgError`, SQLSTATE, constraints, tables, columns, or PostgreSQL messages.
- [ ] Created records return their persisted `Revision` and, for resources, durable `identity-v1`.
- [ ] Every write request carries a caller-supplied `Actor`, recorded in the internal diagnostic seam alongside operation/key/cause, and never persisted as business data or exposed publicly.
- [ ] Write requests are defensively copied; mutating a caller-held request after the call changes nothing.
- [ ] `cmd/garfex/` and `internal/tui/` have zero changed lines and still zero `resourcecore` references.
- [ ] A Create readiness record documents the evidence and names the operations that remain unavailable.
- [ ] `gofmt -l .`, `go vet ./...`, `golangci-lint run ./...`, and `go test ./... -count=1` pass; CI-only race/build checks reported separately.
- [ ] Strict red-green-refactor evidence preserved for every slice.

## Delivery plan

Estimated 600–1,000+ authored lines even for this narrowest slice (anchor: the READ-ONLY bridge cost 1,348 lines for 7 methods). This exceeds the 400-line review budget and uses the project's configured `auto-chain`, mirroring the archived change's 6A/6B/6C split:

1. **Public write contract** — write DTOs, defensive copies, `Writer`/`WriteCapabilities` (Create-only), shape validation, unit tests, external-consumer compile/usage test.
2. **Catalog Create bridge** — `catalogWriter` seam, `Adapter.CreateCatalog`, field-completeness verification against `domain.CatalogRecord`, 9-category coverage for catalog.
3. **Resource Create bridge and readiness** — `resourceWriter` seam, `Adapter.CreateResource`, field completeness against `domain.CreateCommand`, 9-category coverage for resource, readiness record, documentation, full verification.

Each unit targets ≤400 changed lines, follows strict TDD, and must leave the tree green. No unit may bypass the application services to accelerate delivery.
