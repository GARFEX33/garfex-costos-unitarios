# Exploration: resource-master-core-write

## Current State

**`resourcecore/` is READ-ONLY today — confirmed by inspection, not assumption.** `resourcecore/reader.go` defines `ReadCapabilities` (7 methods: `ActiveClasses`, `CatalogDescriptors`, `ListCatalog`, `GetCatalog`, `SearchResources`, `GetResource`, `DescribeResource`) and `Reader`, which validates and delegates to it. No `Create`/`Update`/`Deactivate`/`Reactivate`/`Delete` symbol exists anywhere in `resourcecore/`. `internal/bridge/resourcecore/adapter.go`'s `Adapter` implements exactly those 7 methods against narrow `catalogReader`/`resourceReader` seams — no write seam exists.

All 15 `resourcecore.ErrorCode` categories are already defined in `resourcecore/errors.go` and mirrored in `internal/core/errors.go`'s `Map`. Only the READ-reachable subset (`INVALID_ARGUMENT`, `NOT_FOUND`, `INTEGRITY`, `INVALID_CATALOG`, `UNAVAILABLE`, `INTERNAL` — confirmed by `readiness.md`'s explicit reachability table) is proven; the 9 write-outcome categories (`DUPLICATE`, `INVALID_REFERENCE`, `VALIDATION`, `IDENTITY_CONFLICT`, `INVALID_LIFECYCLE`, `REACTIVATION_IMPOSSIBLE`, `IN_USE`, `IMMUTABLE_CODE`, `CONFLICT`) have no public-boundary proof yet.

The internal write authority is fully built and already production-active — this is the key finding:
- `internal/app/catalogo.Service` has `Create` (now V2-backed when `repoV2` is wired), `CreateV2`, `UpdateRevision`, `DeactivateRevision`, `ReactivateRevision`, `HardDeleteRevision` (`internal/app/catalogo/service.go:525-630`), all CAS-checked via `expectedRevision`, routing through `domain.CatalogAdminRepositoryV2` (`internal/domain/catalog_admin_v2.go`) and its PostgreSQL implementation (`internal/postgres/catalog_admin_repository_v2.go`).
- **Production composition already wires this**: `cmd/garfex/main.go:64-67`'s `newCatalogService` calls `.WithCatalogAdminRepositoryV2(postgres.NewCatalogAdminRepositoryV2(pool))`. `catalogo.Service.Create` is already V2-backed in production; `Update/Deactivate/Reactivate/Delete` (the legacy, non-`*Revision` methods) stay on the legacy candidate-publish path per an explicit comment (main.go:56-58), but their `*Revision` siblings always use `repoV2` directly and are already called from `internal/tui/catalog_admin.go` — i.e., the TUI is today's only consumer of this internal write authority.
- `internal/app/recursos.Service` has `Create`, `Update`, `Deactivate`, `Reactivate` (legacy) plus `UpdateRevision`, `DeactivateRevision`, `ReactivateRevision` (`internal/app/recursos/service.go:229-322`), type-asserting `s.repo.(domain.ResourceRepositoryV2)`. `postgres.NewResourceRepository(pool)` (used unconditionally in `main.go`'s `newPostgresInfra`) already satisfies it — **no separate composition wiring needed for resources**. Resources have **no hard-delete** operation at all (by design — `Service.Delete` is a compatibility alias for `Deactivate` and never physically removes a resource); only catalog has `HardDeleteRevision`.

The prior change (`stabilize-resource-master-core`, archived at `openspec/changes/archive/2026-08-20-stabilize-resource-master-core/`) explicitly and deliberately stopped before public WRITE. Its `design.md` §"READ and per-operation WRITE readiness gates" and `specs/resource-master-core/spec.md` §"Requirement: Separate read and per-operation write readiness" both state the gate this new change must satisfy, and `readiness.md` (the P6 gate record) closes with: *"A future, separate change would need its own readiness evaluation per operation before exposing any WRITE method"* — this is that change.

## Affected Areas

- `resourcecore/types.go` — new write-request DTOs (`CatalogWriteRequest`, `ResourceWriteRequest` or similar), reusing existing public `Value`, `AttributeValue`, `ResourceScope`, `ApplicabilityRule` types already defined for read.
- `resourcecore/errors.go` — no change needed; all 15 codes already exist. Only new tests are needed to prove the 9 write-only codes are reachable and non-colliding.
- `resourcecore/copy.go` — defensive-copy helpers for new write DTOs (same discipline as `CloneCatalogRecord`/`CloneResource`).
- `internal/bridge/resourcecore/adapter.go` — new `catalogWriter`/`resourceWriter` narrow seams (parallel to existing `catalogReader`/`resourceReader`) and new `Adapter` methods delegating to `catalogo.Service`'s and `recursos.Service`'s already-built `*Revision`/`Create*` methods; this is the **only** package allowed to do this translation per `.agents/skills/golang-hexagonal/references/garfex-strict-profile.md`'s "Sole bridge" rule.
- `internal/core/errors.go` — no change expected; `Map` already covers all 15 categories from internal domain sentinels.
- `cmd/garfex/main.go`, `internal/tui/` — **confirmed NOT affected**. Zero references to `resourcecore` in either package today; the public surface is entirely decoupled from the TUI/CLI composition root, matching the maintainer's scope boundary that those packages stay untouched pending their separate deletion.
- New/extended tests carry the bulk of the line count: `resourcecore/writer_test.go` (new), `resourcecore/external_test.go` (extend), `internal/bridge/resourcecore/adapter_test.go` (extend) — each graduating write operation needs category-coverage evidence per the readiness-gate table.

## Approaches Considered

1. **Ship all 5 operations × both domains at once.** Rejected: directly contradicts the archived spec's binding requirement ("Requirement: Separate read and per-operation write readiness" / "One write operation does not unlock another," `spec.md:305-321`). Also a review-budget disaster: the READ-ONLY bridge alone already cost 1,348 lines against a 400-line budget; a full WRITE surface proving up to 14 categories × 5 catalog ops × 4 resource ops is plausibly several thousand lines in one shot.
2. **Per-operation graduation, starting with Create, re-deriving the interface shape each slice.** Matches the spec's requirement and keeps each slice small, but risks interface churn across the chain.
3. **Per-operation graduation, but design the complete `Writer`/`WriteCapabilities` contract up front (all 5×2 operations), then implement/graduate incrementally, starting with Create.** Same delivery cadence as #2, avoids public-contract churn; downstream consumer (PI) can code against a stable shape even before every operation graduates.

## Recommendation

**Approach 3.** Per-operation graduation is not a preference — it is the accepted, binding constraint inherited from the archived spec, never superseded. The open design choice is only whether the interface/DTO shape is fixed once (3) or re-derived per slice (2); fixing it once avoids public-contract churn while still respecting per-operation *availability* gating.

Suggested first slice: `Writer.CreateCatalog` + `CreateResource`, backed by new bridge write seams delegating to `catalogo.Service.Create`/`CreateV2` and `recursos.Service.Create` — both already production-hardened internally. Category coverage needed for Create specifically: `INVALID_ARGUMENT`, `NOT_FOUND`, `DUPLICATE`, `INVALID_REFERENCE`, `VALIDATION`, `INTEGRITY`, `INVALID_CATALOG`, `UNAVAILABLE`, `INTERNAL` — 9 categories, not the full 14 (Create has no CAS/`CONFLICT`, no `IDENTITY_CONFLICT`/`REACTIVATION_IMPOSSIBLE` (reactivate-only), no `IN_USE`/`IMMUTABLE_CODE` (update/delete-only)).

## Composition/TUI Coupling Risk

**Confirmed: none.** Zero matches for `resourcecore` in `cmd/garfex/` or `internal/tui/`. Exposing WRITE requires no change to either — they remain fully untouched. The internal V2 write authority this change would expose is already production-wired independently of the TUI; the TUI is merely one of two current *internal* callers of that authority, not a dependency of the new public surface.

## Size/Complexity Estimate

Using the archived change's own receipts as an anchor: the READ-ONLY bridge (7 methods, no lifecycle/CAS categories, 37 tests) cost **1,348 lines** against the 400-line budget (`size:exception`). A single WRITE operation graduation (Create, both domains) needs new DTOs, a `Writer`/`WriteCapabilities` pair (likely 80-150 lines mirroring `reader.go`'s 155 lines), defensive-copy additions, 2 new bridge methods plus 2 new narrow writer seams, and category-coverage test evidence for ~9 categories across catalog + resource — realistically **600-1,000+ authored lines** even for the narrowest slice. This will very likely need `auto-chain` (already this project's configured `delivery_strategy`), split at minimum into: (a) public DTOs + `Writer` skeleton, (b) bridge `Create` methods + catalog category evidence, (c) resource category evidence — mirroring the archived change's own 6A/6B/6C stage-splitting pattern.

## Constraints From the Archived Design

- `design.md` §"Rollout" step 7: *"Consider public WRITE later, per operation, only after a separate readiness review proves every reachable stable outcome and retains all Core, CAS, atomicity, and equivalence evidence."* — this change is exactly that readiness review, per operation.
- `design.md` §"Alternatives rejected": *"Expose public WRITE with the initial Reader: violates independent read readiness and makes incomplete operations discoverable"* — per-operation gating is deliberate, not an oversight.
- `spec.md`'s three scenarios under "Separate read and per-operation write readiness" are directly reusable as acceptance scenarios for this change's own spec: "Read-only graduates before write" (already true), "One write operation does not unlock another" (governs delivery order), "Failing evidence withdraws only affected write readiness" (governs regression handling — disable only the affected operation).
- `design.md` §"Definitive application-service APIs" already sketches the target internal signature shape (`expected uint64`, revision-conflict semantics, idempotent no-op on already-current lifecycle state, reactivation identity-mismatch vs. impossible-reactivation distinction) — the internal contract is already delivered; this change's job is purely the *public* mirror plus bridge translation, not new internal behavior.
- No public DTO/`Writer` shape was pre-designed by the archived change — `design.md` states plainly "Public WRITE is not part of this stabilization chain" and never sketches a public write type. This change starts that design from a blank public-API slate, informed by the existing read-side `resourcecore` DTO conventions and the internal `domain.CreateCommand`/`UpdateCommand`/`CatalogRecord`+`expectedRevision` shapes already built.

## Risks

- Per-operation graduation means the public `Writer` may exist with some methods effectively unusable in production until later slices graduate — needs explicit design-level guardrails (e.g., omitting ungraduated methods from the interface until their own slice) so "the type exists" never implies "every method is safe to call."
- Sizing risk: even the narrowest slice (Create only) may exceed the 400-line budget based on the READ-ONLY precedent, requiring `auto-chain` planning at `sdd-tasks` time.
- Uncommitted `internal/tui`/`internal/postgres` changes may exist in the working tree from unrelated in-flight work — worth a clean `git status`/`go build ./...` check before this change starts.

## Ready for Proposal

Yes. Scope should be framed as "design the full public `Writer` contract; implement/graduate `Create` (catalog + resource) as the first operation," with `Update`/`Deactivate`/`Reactivate`/`HardDelete` explicitly named as separate, later, per-operation-gated follow-up changes — mirroring the archived change's own accepted delivery pattern.
