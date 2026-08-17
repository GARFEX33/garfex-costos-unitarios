# Design: Resource Master Technical Debt Remediation

## Technical Approach

Use reversible, foundations-first slices and preserve `TUI -> application -> domain repository port -> PostgreSQL`. Commands carry intent; the domain constructs canonical aggregates. Supplier Master remains out of scope.

## Architecture Decisions

| Decision | Alternatives | Choice and rationale |
|---|---|---|
| Write API | Accept `domain.Resource`; adapter construction | Add intent-only commands. `recursos.Service` reads `CatalogAuthority` and constructs domain state before persistence, centralizing authority. |
| Aggregate boundary | Public fields; database-only validation | Use private invariant state and accessors. Constructors create resources; `RehydrateResource(catalog,snapshot)` verifies persisted structure and identity without current-write eligibility. |
| Identity | Trust caller/stored keys; raw concatenation | On writes derive `v1` plus `<UTF-8-byte-length>:<value>` components: class, family, type, then attribute-code/type/value triples sorted by canonical code. Length-prefixing prevents delimiter collisions. PR1 audits legacy keys and records the mapping only; PR2 atomically changes writers, rewrites admitted keys, and adds the v1 constraint. |
| Activity | Filter inactive everywhere; permit historical edits | Hydrate every catalog level's activity. Writes require the applicable chain active; `Get` remains historical without granting write eligibility. |
| Lifecycle | Boolean `SetActive`; replacement create | Add `Deactivate`/`Reactivate` contracts returning `{Resource, Changed}`. Repetition is a no-op. Transactional reactivation revalidates eligibility and identity; rejection remains inactive. |
| Attribute writes | Family-only lookup; precedence | Resolve class, family, exact type, definition, and `(type_id IS NULL OR type_id = resource.type_id)`. Each write must resolve and insert once or return `ErrResourceIntegrity` and roll back; overlap is invalid. |
| Search/page | Per-row `Get`; raw slices | Select `limit+1` bases by `(identity_key ASC,id ASC)`, then hydrate IDs with one attribute query. Duplicate attributes, missing bases, or decode failures fail the page. Query count is page-size independent. |

## Data Flow

```text
TUI command -> recursos.Service -> domain constructor -> ResourceRepository -> PostgreSQL transaction
TUI page <- recursos.Service <- domain hydration <- ResourceRepository <- two set-based queries
```

## Interfaces / Contracts

```go
Create(ctx, CreateCommand{Scope, NaturalUnit, Attributes}) (domain.Resource, error)
Update(ctx, UpdateCommand{ID, Scope, NaturalUnit, Attributes}) (domain.Resource, error)
Deactivate(ctx, id) (LifecycleResult, error)
Reactivate(ctx, id) (LifecycleResult, error)
Search(ctx, SearchCriteria) (ResourcePage, error)

SearchCriteria{Text, ClassCode, LifecycleScope, Filters, Limit, Offset}
ResourcePage{Criteria, Resources, Limit, Offset, Order, HasNext, HasPrevious}
```

Commands cannot carry `IdentityKey`, `Active`, or snapshots. Every repository snapshot and hydrated `Resource` MUST contain caller-immutable active state; search rows and detail MUST render `Active` or `Inactive`.

`Text` is outer-trimmed, returned normalized, and matched case-insensitively within identity key or family code/name; empty matches all. Empty `ClassCode` means all; otherwise match exactly. Zero `LifecycleScope` means `Active`; valid scopes are `Active` and `Inactive`. `Filters` are ANDed exact canonical matches. Non-positive `Limit` becomes 50; negative `Offset` becomes zero. Return normalized values and `Order=IdentityKeyASCThenIDASC`.

Fetch `Limit+1`; exclude the extra row and set `HasNext` iff it exists. `HasPrevious` is `Offset > 0`. Next uses `Offset+Limit`; previous uses `max(0, Offset-Limit)`. Boundary requests preserve page/selection without searching. Successful transitions preserve criteria/order and select the first row, or none; selection never crosses pages. Detail return restores that page and selection.

## File Changes

| File | Action | Description |
|---|---|---|
| `internal/domain/resource_types.go`, `resource_validation.go`, `resource_catalog_query.go` | Modify | Encapsulation, hydration, lifecycle/page contracts, active-chain policy. |
| `internal/app/recursos/service.go` | Modify | Authoritative writes, lifecycle, page API. |
| `internal/postgres/catalog_loader.go`, `resource_repository_crud.go`, `resource_repository_search.go` | Modify | Activity hydration, exact scoped writes, lifecycle, set-based pages. |
| `internal/tui/resource_editor*.go`, `resources_workspace_dispatch.go` | Modify/Create | Adapt commands; separately extract state, transitions, persistence mapping, and presentation. |
| `migrations/<next>_resource_integrity.{up,down}.sql` | Create | Audit/admission gate and legacy-to-v1 mapping foundation; PR1 does not rewrite keys or enforce an encoding. |
| Resource docs/comments and adjacent `*_test.go` files | Modify | Settled contracts and evidence. |

## Testing Strategy

| Layer | Evidence |
|---|---|
| Domain/application | Table-driven invalid/inactive chains, forged and delimiter-bearing identity, canonical equivalence, update ID, active hydration, idempotent lifecycle; rejected writes never call repositories. |
| PostgreSQL integration | Audit/collision fixtures; zero/one/multiple targets; rollback; historical active state; reactivation rejection; page parity, order, boundaries, and constant query count for 1/10/50 rows. |
| TUI | Characterize create/edit/duplicate parity before extraction; separately test visible lifecycle state, retained filters, page selection, and boundaries. |

## Threat Matrix

N/A — no routing, shell, subprocess, VCS/PR automation, executable classification, or process-integration boundary changes.

## Migration / Rollout

Audit identity drift, legacy-key recomputation, canonical collisions, attribute applicability, and lifecycle conflicts first; any ambiguity blocks rollout. PR1 records admitted legacy-to-v1 mappings without rewriting keys or constraining encoding, so current legacy writers remain valid. PR2 atomically changes writers, rewrites admitted keys, and adds the v1 constraint. Then deploy applicability constraints, canonical writes, editor extraction, lifecycle, set-based search, pagination, and documentation. Modify only change-owned files; leave unrelated worktree state untouched. Review sizing remains for the tasks phase under `ask-on-risk`.

## Open Questions

None.
