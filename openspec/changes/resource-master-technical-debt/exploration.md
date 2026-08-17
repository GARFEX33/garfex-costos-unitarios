# Exploration: Resource Master Technical Debt Remediation

## Current State

Resource Master currently follows the sound dependency direction that this remediation must preserve:

`Bubble Tea TUI -> internal/app/recursos.Service -> domain.ResourceRepository port -> internal/postgres adapter -> PostgreSQL`

Composition remains in `cmd/garfex/main.go`; the domain and application packages do not import the TUI or PostgreSQL. The primary risk is therefore not dependency inversion but incomplete ownership of invariants and inconsistent behavior across the existing boundaries.

The approved backlog is confirmed by the checked-out code:

- `domain.NewResource` validates catalog scope and derives canonical identity, but `recursos.Service.Create` and `Update` accept a publicly constructible `domain.Resource` and trust the caller to have used that function. A non-TUI adapter can bypass validation and provide its own `IdentityKey`.
- The TUI editor calls `domain.NewResource`, so the strongest guarantee currently lives in an adapter workflow rather than at the application/core boundary.
- Attribute writes use `INSERT ... SELECT` scoped only by family code and attribute definition code. They do not explicitly resolve class and type, and `Exec` does not verify `RowsAffected()`. Zero inserted rows can therefore commit silently; family-wide and type-specific definitions can also be resolved ambiguously.
- The schema already models class, family, type, and attribute relationships, but the write path does not use the complete scope. Existing foreign keys prevent some cross-family corruption, but they do not prove that a selected resource attribute applies to the resource's type.
- PostgreSQL defines active flags across catalog records after migration `000003`, while `LoadResourceCatalog` loads some flags, omits others, and does not filter inactive children. Domain validation helpers such as `hasFamily`, `hasType`, `hasUnit`, `resourceAttributes`, and option resolution do not consistently require active state.
- Resource search is active-only, while `Get` can return inactive resources and `Resource` exposes no active state. `Service.Delete` calls `SetActive(false)` but names the operation as deletion. There is no Resource Master reactivation use case, inactive discovery mode, or state presentation.
- An inactive resource still occupies the database unique key `(class_id, identity_key)`. Creating a replacement is not a safe substitute for reactivation.
- PostgreSQL search first loads matching keys and then calls `Get` once per result, producing one base query plus two queries per resource. Pagination primitives exist (`Limit`, `Offset`), but the TUI always requests eleven rows, displays ten, and tells the user to refine instead of navigating pages.
- `internal/tui/resource_editor.go` is 1,215 lines and combines editor state, adapter ports, create/edit/duplicate workflows, canonical candidate construction, persistence error mapping, and presentation helpers.

No OpenSpec project configuration or main specs directory exists in this checkout. This exploration therefore records repository evidence without inferring project-specific OpenSpec rules.

## Affected Areas

- `internal/domain/resource_types.go` — public aggregate shape, lifecycle state visibility, search criteria/result contracts, and repository port.
- `internal/domain/resource_validation.go` — canonical construction, catalog-scoped invariant enforcement, and identity derivation.
- `internal/domain/resource_canonical.go` — controlled-option validity and identity canonicalization; active options are currently accepted.
- `internal/domain/resource_catalog_query.go` — active catalog discovery used by navigation and editors.
- `internal/app/recursos/service.go` — application boundary for create/update, lifecycle naming and reactivation, search, and error translation.
- `internal/app/recursos/service_test.go` — adapter-independent proof that every caller receives the same guarantees.
- `internal/postgres/resource_repository_crud.go` — fully scoped attribute resolution, exact insert-count enforcement, active-state loading, and lifecycle persistence.
- `internal/postgres/resource_repository_search.go` — active/inactive criteria, N+1 removal, and page loading.
- `internal/postgres/catalog_loader.go` — catalog active-flag parity and filtering/hydration policy.
- `internal/postgres/resource_repository_integration_test.go` — transactional persistence, scoped attributes, lifecycle, and query-result evidence.
- `internal/postgres/catalog_loader_integration_test.go` — authoritative active/inactive catalog behavior against PostgreSQL.
- `internal/tui/resource_editor.go` — current adapter-side validation and oversized mixed responsibilities.
- `internal/tui/resources_workspace_dispatch.go` — delete terminology, state visibility, inactive discovery, reactivation, and page navigation.
- `internal/tui/resource_editor_test.go` — characterization coverage required before responsibility extraction.
- `internal/tui/resources_workspace_adapter_test.go` — lifecycle and pagination interaction contracts.
- `cmd/garfex/main.go` and `cmd/garfex/main_test.go` — composition updates while preserving port direction.
- `migrations/000002_resource_master.up.sql` and `migrations/000003_catalog_admin.up.sql` — current integrity and active-state baseline.
- A future additive migration — potentially required for data repair and database-level type/scope enforcement; exact DDL is deliberately deferred to design.
- Resource Master architecture documentation and stale comments — must be updated only after behavior and names are settled.

## Concern Classification

| Concern | Backlog items | Nature | Primary proof |
|---|---:|---|---|
| Invariant ownership | 1, 7, 8 | Core/application correctness | Application tests with invalid, non-canonical, and forged caller inputs |
| Persistence integrity | 2, 3 | PostgreSQL correctness and atomicity | Integration tests for zero, one, and ambiguous matches plus rollback |
| Active catalog semantics | 4 | Cross-layer invariant and persistence parity | Loader/domain/application integration matrix for every active flag |
| Lifecycle capability | 5, 6 | Explicit application capability and UX | Deactivate/discover/reactivate round trip with visible state |
| Performance | 9 | Repository read-path efficiency | Bounded query-count evidence independent of page size |
| Refactor-only | 10 | Behavior-preserving maintainability | Characterization tests unchanged before and after file extraction |
| Documentation/UX | 11, 12 | Terminology, guidance, and navigation | Interaction tests, TUI smoke evidence, and documentation review |

## Sequencing Constraints and Safe Work Units

1. **Baseline characterization and data audit**
   - Freeze current create/edit/duplicate/search/detail behavior with focused tests before changing boundaries.
   - Audit persisted resources for identity mismatches, duplicate or type-inapplicable attribute rows, missing expected attributes, and inactive rows that occupy identity keys.
   - This is evidence work only and should not change behavior.

2. **Invariant boundary and canonical identity** — items 1, 7, and 8
   - Make the application/core boundary reconstruct or validate canonical state before repository calls, including updates while preserving stable persistence identity.
   - Do not rely on a comment saying callers used `NewResource`.
   - Keep TUI validation for immediate feedback if useful, but treat it as convenience rather than authority.
   - This slice must precede new adapters or master-catalog reuse.

3. **Active catalog semantics** — item 4
   - Establish one explicit meaning for active/inactive across loader, domain queries, validation, catalog administration, and resource writes.
   - Parent/child semantics must be resolved together; filtering only top-level records can leave dangling active children or invalidate historical resources.
   - This slice should precede Resource lifecycle UX because reactivation validity depends on current catalog availability.

4. **Persistence scope and exact cardinality** — items 2 and 3
   - Resolve each attribute using Class + Family + Type + Attribute and require exactly one inserted row per expected value.
   - Keep create/update transactional so any mismatch rolls back the resource row and all attribute replacement work.
   - If the data audit finds historical ambiguity, repair and enforcement must be an independently reversible migration unit.

5. **Behavior-preserving editor decomposition** — item 10
   - Perform only after invariant behavior has stable characterization tests and before adding more editor responsibilities.
   - Move cohesive responsibilities without changing prompts, state transitions, ordering, canonical values, or persistence calls.
   - Do not combine this mechanical change with lifecycle or pagination behavior.

6. **Explicit lifecycle capability** — items 5 and 6
   - Rename the application and TUI concepts from delete to deactivate while retaining soft-deactivation persistence.
   - Add inactive discovery, visible state, and an explicit reactivation path as one end-to-end capability slice.
   - Preserve active-only default search behavior for existing callers unless they deliberately request another lifecycle scope.

7. **Set-based search loading** — item 9
   - Remove page-size-dependent query growth while preserving search filters, ordering, canonical resource reconstruction, and active-only defaults.
   - Keep this repository-focused slice separate from TUI navigation so performance regressions can be isolated.

8. **Complete pagination** — item 12
   - Add next/previous navigation only after the repository can efficiently load a page.
   - Define stable ordering and page-boundary evidence before exposing navigation; offset alone does not communicate total or next-page state.

9. **Documentation and stale comments** — item 11
   - Update terminology, identity scope, lifecycle, query behavior, and architecture only after the corresponding contracts are accepted.
   - Documentation must continue to show TUI -> application -> domain port -> PostgreSQL, never TUI-to-database shortcuts.

Each numbered unit should be reviewable and reversible on its own. The editor split and likely migration work may exceed the 400-line review budget even without broad behavior changes. With `ask-on-risk`, the tasks phase should flag those units for an explicit chained-PR decision before apply rather than silently creating an oversized review.

## Migration and Backward-Compatibility Implications

- **Stored identity:** Valid existing identity keys should remain byte-compatible if they already equal canonical class + family + type + identity attributes. Before enforcing derived identity, compare stored keys with re-derived keys and define remediation for mismatches; do not silently rewrite identities that other records may reference.
- **Attribute data:** Existing foreign keys are not sufficient evidence of type applicability. A migration may need to detect and repair duplicate, missing, or wrong-type attribute rows before adding stronger constraints. Migration admission must fail on unresolved ambiguity rather than choosing a row arbitrarily.
- **Active flags:** A loader policy change can alter runtime catalog contents immediately after deployment. Historical inactive resources may reference now-inactive catalog elements, so read/display compatibility must be distinguished from eligibility for new writes.
- **Lifecycle:** Renaming `Delete` to `Deactivate` changes Go interfaces and test fakes. A temporary compatibility wrapper may reduce rollout risk, but keeping misleading language indefinitely would preserve the debt. Existing active-only search behavior should remain the default.
- **Aggregate/API shape:** Restricting direct construction or changing create/update command shapes affects TUI adapters, tests, fakes, and any future adapter. The transition should be atomic across the application boundary; PostgreSQL must continue implementing a domain-owned port.
- **Search contract:** Returning lifecycle state, pagination metadata, or a page object can be a source-compatible break for interfaces and fakes. If introduced, transition application and TUI contracts in one bounded slice while retaining deterministic ordering.
- **Rollback:** Data migrations should be additive and reversible where possible. Code that depends on a new constraint must not deploy before the data audit and migration have succeeded.

## Tests and Evidence by Slice

| Slice | Required evidence |
|---|---|
| Baseline | Characterization tests for TUI create/edit/duplicate prompts and transitions; fixture-backed audit query with explicit clean/failing results |
| Invariants | Table-driven application tests proving invalid family/type/unit/attribute/value/relations and forged `IdentityKey` never reach the repository; success tests proving canonical state does |
| Active semantics | Domain tests for every active parent/child combination; PostgreSQL loader integration tests covering all active columns introduced by `000002`/`000003`; write rejection versus historical read acceptance |
| Attribute persistence | Integration fixtures with reused family codes across classes, family-wide plus type-specific attributes, missing definitions, and quantity units; assert exactly expected rows and full transaction rollback |
| Editor refactor | Existing TUI tests unchanged; focused state-transition characterization; `gofmt`, `go vet`, lint, and full Go tests with no golden/prompt drift |
| Lifecycle | Application tests for idempotent or explicitly rejected transitions; repository integration round trip active -> inactive -> discover -> reactivate; TUI tests for language, state visibility, cancellation, and errors |
| Search performance | Integration-level query counter or trace proving query count is bounded for 1, 10, and 50 results; equality of hydrated resources before/after optimization |
| Pagination | Repository boundary tests for limit/offset/order and end pages; TUI tests for next, previous, query retention, class filter retention, and selection after navigation |
| Docs/UX | Manual Bubble Tea smoke through search, detail, deactivate, inactive discovery, reactivation, and page navigation; review comments/docs against actual symbols and behavior |

Repository validation for implementation slices should follow project policy: `gofmt -l .`, `go vet ./...`, compatible `golangci-lint run ./...`, and `go test ./... -count=1`. CI remains responsible for race tests and builds; local exploration performs no build and no implementation tests.

## Approaches

1. **Foundation-first staged remediation** — establish invariant ownership and catalog/persistence integrity before lifecycle, performance, refactoring, and UX.
   - Pros: Prevents later capabilities from building on invalid state; makes data migration gates explicit; preserves architectural direction.
   - Cons: User-visible lifecycle and pagination improvements arrive later; several coordinated work units are required.
   - Effort: High

2. **Independent vertical backlog slices** — remediate each numbered item end to end in priority order, including its UI and persistence changes.
   - Pros: Produces visible outcomes sooner; each item has a direct acceptance narrative.
   - Cons: Cross-cutting contracts such as active state, aggregate construction, and search result shape would be changed repeatedly; higher conflict and migration risk.
   - Effort: High

3. **Large Resource Master rewrite** — replace aggregate, service, repository, and TUI contracts in one program increment.
   - Pros: Fewer temporary compatibility seams.
   - Cons: Exceeds the review budget, weakens rollback, mixes behavior change with refactoring, and makes regression attribution difficult.
   - Effort: Very High

## Recommendation

Advance to proposal using the foundation-first staged remediation approach, with each stage delivered as a vertical proof through the existing TUI -> application -> domain port -> PostgreSQL direction. Treat invariant ownership, active semantics, and persistence cardinality as release gates before any further master catalog work. Keep the editor decomposition strictly behavior-preserving and isolate migrations, lifecycle capability, search performance, pagination, and documentation into separate work units.

This recommendation is sequencing guidance, not a final technical design. The proposal/design phases still need to resolve command/aggregate API shape, exact active-state policy for historical reads, database enforcement strategy, lifecycle transition semantics, and pagination result shape.

## Risks

- Persisted identity or attribute anomalies may require a blocking repair migration before stronger enforcement can be enabled.
- Active filtering can make historical resources impossible to display or reactivate unless read compatibility is explicitly separated from write eligibility.
- Reactivation can collide with lifecycle expectations because inactive rows retain their unique identity keys.
- Tightening aggregate construction can create a broad compile-time blast radius across TUI adapters, repository hydration, tests, and fakes.
- A set-based search rewrite can change ordering or hydration semantics unless parity evidence is captured first.
- Editor decomposition can look low-risk while producing a large, difficult review; behavior preservation and the 400-line budget must be enforced.
- The current worktree contains unrelated in-progress TUI and Supplier Master files. Future remediation must avoid mixing or overwriting those changes, and this exploration does not design Supplier Master.

## Ready for Proposal

Yes. The priority order, dependency map, migration questions, evidence expectations, and safe work-unit boundaries are sufficient for a remediation proposal. The proposal should keep Supplier Master and all further master catalog expansion out of scope until the high-priority gates are complete.
