# Exploration: Graduate `HardDelete` as resourcecore's fourth WRITE capability

## Current State

- `resourcecore/writer.go:12` — `WriteCapabilities` has exactly 8 methods (Create/Update/Deactivate/Reactivate × Catalog/Resource). Confirmed no `HardDelete*` anywhere in `resourcecore`.
- `resourcecore/writer_test.go:211-233` (`TestWriter_NoUngraduatedMethodExported`) is a reflection-based exhaustive method-list assertion that currently fails-fast with "no HardDelete stub allowed" — verified directly, this is the compiled-surface guard from the prior change and will need explicit updating.
- **Catalog side is fully built end-to-end already:**
  - `internal/app/catalogo/service.go:607-630` — `Service.HardDeleteRevision(ctx, kind, id, expectedRevision) error` already exists, CAS-aware, production code (not a stub).
  - It delegates to `buildV2DeleteCandidate` (lines 446-483), the exact "conservative hard delete" guard chain: reject active target (`ErrInvalidLifecycle`) → reject any non-zero `Dependents` count → reject any `ReferencedByResources` hit → validate candidate → then `repoV2.Delete`.
  - `domain.CatalogAdminRepositoryV2.Delete` (`internal/domain/catalog_admin_v2.go:63-66`) and its Postgres implementation `internal/postgres/catalog_admin_repository_v2.go:1104` perform the real CAS `DELETE ... WHERE id=$1 AND revision=$2`.
  - Production-wired: `cmd/garfex/main.go:66` calls `WithCatalogAdminRepositoryV2(postgres.NewCatalogAdminRepositoryV2(pool))` — this is live, not dormant.
  - `internal/bridge/resourcecore/adapter.go`'s `catalogWriter` interface (lines 29-34, verified directly) only declares `Create/UpdateRevision/DeactivateRevision/ReactivateRevision` — **the bridge itself has no HardDelete method yet**. That's the only missing link for the catalog side.
- **Resource side has zero implementation, by design, not oversight:**
  - `internal/app/recursos/service.go:153-161` (verified directly) — `Service.Delete` is explicitly documented as "a compatibility alias for Deactivate. It never physically removes a resource."
  - `domain.ResourceRepositoryV2` (`internal/domain/resource_repository_v2.go:25-44`, verified directly) declares only `DeactivateRevision/ReactivateRevision/UpdateRevision` — no `Delete` method in the interface at all.
  - No production SQL hard-deletes the `recursos` table; every `DELETE FROM public.recursos` hit in the codebase is test-fixture cleanup, not application code.
  - The original stabilization design explicitly frames this as deferred, not missing: `openspec/changes/archive/2026-08-20-stabilize-resource-master-core/design.md:334` — "Resource update, deactivate, reactivate, and **any future resource hard delete** use identical CAS principles..." — future tense, no guard-chain design was ever written for it.
  - The merged spec (`openspec/specs/resource-master-core/spec.md`) has "Requirement: Complete catalog lifecycle" and "Requirement: Conservative guarded hard delete" — both scoped explicitly to "each of the 11 registered catalog kinds." No equivalent resource-hard-delete requirement exists anywhere.
- `resourcecore/errors.go` already declares `InvalidLifecycle` and `InUse` (`resourcecore/errors_test.go:9` lists all 15 categories), and `internal/core/errors.go:80-85` already maps `domain.ErrInvalidLifecycle`→`InvalidLifecycle` and `domain.ErrCatalogInUse`→`InUse`. No new error-mapping code is needed — but per the spec's own "reachable error category coverage" pattern (used for every prior op), `HardDeleteCatalog` would be the **first** graduated operation to actually prove both categories reachable.
- Request shape: `CatalogLifecycleRequest` (`resourcecore/write_types.go:47-52`, Actor/Kind/ID/ExpectedRevision, no content fields) is an exact structural match for `HardDeleteRevision`'s parameters — no new request type is needed for the catalog operation. `validateCatalogLifecycleRequest` (`writer.go:141-152`) already enforces non-zero `ExpectedRevision` — strict CAS, no idempotent no-op bypass (unlike Deactivate/Reactivate), which is correct: deletion has no "already in target state" concept.
- Return shape mismatch to flag: `HardDeleteRevision` and `CatalogAdminRepositoryV2.Delete` both return **only `error`** — the interface doc explicitly states "the returned CatalogWriteResult.Record is always nil." Every other graduated `Writer` method returns `(Record, error)`. A public `HardDeleteCatalog` would break that pattern by returning only `error`, which is a real design decision for `sdd-design`, not a mechanical copy.

## Key Questions Answered

1. **Does a hard-delete mechanism already exist at the service/bridge layer?** Yes for catalog (`catalogo.Service.HardDeleteRevision`, production-wired). No for resource — no domain interface method, no repository implementation, no service method beyond the deliberately-named `Delete` alias that only deactivates.
2. **Right request shape?** Reuse `CatalogLifecycleRequest` as-is for the catalog operation — no new type needed. Strict CAS (non-zero `ExpectedRevision` required, no idempotent bypass), unlike Deactivate/Reactivate's idempotent-no-op handling — correct because deletion has no "already in target state."
3. **Safety/irreversibility guardrails specific to HardDelete?** Already implemented at the service layer: reject if target is active (`ErrInvalidLifecycle`), reject if `Dependents` count is non-zero, reject if `ReferencedByResources` is non-empty (`ErrCatalogInUse`/`InUse`). These did not exist for Deactivate/Reactivate because those are reversible; HardDelete is not.
4. **Does completing HardDelete finish the public write contract?** No — only if scoped to catalog. Resource hard-delete has no implementation at any layer and was explicitly deferred as future work in the original stabilization design, not omitted by accident. Completing catalog-only HardDelete finishes catalog+resource parity on Create/Update/Deactivate/Reactivate, plus catalog-only HardDelete, leaving resource HardDelete as a documented, deliberate gap.

## Affected Areas

- `resourcecore/writer.go` — add `HardDeleteCatalog` to `WriteCapabilities` + `Writer`, decide return shape, validation (can reuse `validateCatalogLifecycleRequest`).
- `resourcecore/write_types.go` — reuse `CatalogLifecycleRequest` as-is; no new type needed for catalog-only scope.
- `internal/bridge/resourcecore/adapter.go` — extend `catalogWriter` interface with a `HardDeleteRevision`-shaped method and add `Adapter.HardDeleteCatalog`.
- `resourcecore/writer_test.go:211-233` — update the exhaustive compiled-surface assertion (currently rejects any `HardDelete*` symbol).
- `resourcecore/errors_test.go` / new integration tests — prove `INVALID_LIFECYCLE` and `IN_USE` reachable for the first time via this public surface.
- `openspec/specs/resource-master-core/spec.md` — the existing "Requirement: Compiled surface extended to Create, Update, Deactivate, and Reactivate" explicitly states "No method for HardDelete MUST be exported" — this requirement must be MODIFIED, plus a new requirement added mirroring the Deactivate/Reactivate section's shape (field completeness, actor attribution, error coverage).
- **Not needed for a catalog-only scope**: `internal/app/catalogo/service.go`, `internal/postgres/catalog_admin_repository_v2.go` (both already complete), `cmd/garfex/`, `internal/tui/` (precedent from the lifecycle change requires zero changed lines there).
- **Only needed if resource hard-delete is included**: `internal/domain/resource_repository_v2.go` (new `Delete` method), a new Postgres CAS delete for `recursos`, `internal/app/recursos/service.go` (new `HardDeleteRevision`, with an as-yet-undesigned guard chain), plus bridge/Writer wiring.

## Approaches

### 1. Graduate `HardDeleteCatalog` only

The only hard-delete surface with a complete, production-wired internal implementation.

- **Pros**: minimal new code (one bridge method, one `Writer` method, updated tests, spec delta); reuses every existing guardrail and error-mapping proof; stays well under the 400-line review budget; matches the exact one-operation-per-change cadence of the three prior graduations.
- **Cons**: leaves the public contract asymmetric (no `HardDeleteResource`); the change name doesn't disambiguate scope, so the proposal must state this explicitly to avoid reviewer confusion.
- **Effort**: Low.

### 2. Graduate both `HardDeleteCatalog` and `HardDeleteResource` in one change

Build the resource side from scratch (domain interface method, Postgres CAS delete, service guard chain, bridge, Writer).

- **Pros**: closes the full symmetry gap in one shot.
- **Cons**: no existing design or guard-chain ever specified for resource hard-delete; "what does referenced/in-use mean for a resource" is an open question since nothing in this codebase currently holds a foreign key into `recursos` beyond its own owned attribute-value rows (cascade-deleted); this is genuinely new design work, not graduation of existing evidence; very likely exceeds the 400-line PR budget and mixes two risk profiles (proven vs. unproven) in one review.
- **Effort**: High.

### 3. Graduate `HardDeleteCatalog` now; explicitly defer `HardDeleteResource` (recommended)

Functionally identical to Approach 1, but the proposal states the deferral as a deliberate, evidenced decision (citing the design doc's own "future resource hard delete" language) rather than leaving it implicit.

- **Pros**: same low effort as Approach 1, plus removes the naming-ambiguity risk by making the scope boundary a first-class, defensible statement in the proposal/spec.
- **Cons**: none beyond Approach 1's.
- **Effort**: Low.

## Recommendation

Approach 3. Graduate **`HardDeleteCatalog` only** in this change. It is the sole hard-delete surface with a complete, evidenced, production-wired internal implementation — everything downstream of the bridge already exists and is exercised. `HardDeleteResource` has no domain interface method, no repository implementation, and no design-level guard chain anywhere in the codebase; the original stabilization design explicitly named it a "future" concern, not an omission. Building it now would require inventing new guard semantics with zero existing acceptance evidence to graduate — a different, larger, and currently unscoped piece of work that deserves its own exploration/design cycle if/when a real product need for physical resource deletion emerges. The `sdd-propose`/`sdd-spec` phases should explicitly record this boundary so "the write contract is complete" is understood precisely as catalog+resource parity on Create/Update/Deactivate/Reactivate, plus catalog-only HardDelete — with resource HardDelete named as a known, deliberate gap.

## Risks

- The public return shape for `HardDeleteCatalog` breaks the pattern of the other 7 `Writer` methods (which all return a record) since the internal implementation returns only `error` — this needs an explicit decision in `sdd-design`, not a mechanical copy of the Deactivate/Reactivate shape.
- `resourcecore/writer_test.go`'s exhaustive reflection-based method-list assertion will fail immediately after adding the method and must be updated as part of the same PR.
- This is the first operation to reach `INVALID_LIFECYCLE` and `IN_USE` publicly; new "reachable error category coverage" scenarios and integration tests are needed, following the exact pattern of the three prior spec sections.
- Naming ambiguity: the change name `resource-master-core-write-harddelete` doesn't distinguish catalog vs. resource scope — without an explicit proposal-level scope statement, a reviewer could assume full parity is being delivered.
- If resource hard-delete is ever required later, its guard-chain design is completely unstarted (no known "in use" concept for a resource today) and will need its own exploration before it can be safely built.

## Ready for Proposal

Yes — scope: `HardDeleteCatalog` only, with `HardDeleteResource` explicitly named as an out-of-scope, deliberate gap in the proposal.
