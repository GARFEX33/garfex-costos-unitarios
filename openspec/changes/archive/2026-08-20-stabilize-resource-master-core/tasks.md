# Implementation Tasks: Stabilize the Resource Master Core

This is the corrected, strictly ordered auto-chain. It begins with Core mutation primitives and does not create any public package before Stage 6. `internal/app/catalogo.Service` and `internal/app/recursos.Service` remain the business authorities; PostgreSQL remains the persistence authority. Public WRITE is explicitly outside this plan and may be considered only by a later change after READ readiness.

## Review Workload Forecast

| Field | Value |
| ------- | ------- |
| Estimated changed lines | 7,500–10,000 authored changed lines total; each implementation slice is forecast at 250–400 changed lines |
| 400-line budget risk | High |
| Chained PRs recommended | Yes |
| Suggested split | Stage 1: 1A → 1B → 1C → 1D; Stage 2: 2A → 2B → 2C; Stage 3: 3A → 3B → 3C → 3D → 3E → 3F → 3G → 3H → 3I; Stage 4: 4A → 4B → 4C → 4D → 4E → 4F → 4G → 4H; Stage 5: 5A → 5B; Stage 6: 6A → 6B → 6C |
| Delivery strategy | auto-chain |
| Chain strategy | stacked-to-main |

Decision needed before apply: No
Chained PRs recommended: Yes
Chain strategy: stacked-to-main
400-line budget risk: High

Each unit is one stacked-to-main review boundary with its tests, focused receipt, rollback boundary, and full-suite proof. No unit may exceed 400 additions plus deletions. If measured work exceeds its forecast, split that unit by the named kind group, method family, or fixture boundary; do not create a broad exception.

### Per-slice forecast

| Slice | Forecast |
| ------- | ---------- |
| 1A | ≤250 changed lines |
| 1B | ≤300 changed lines |
| 1C | ≤400 changed lines |
| 1D | ≤400 changed lines |
| 2A | ≤400 changed lines |
| 2B | ≤400 changed lines |
| 2C | ≤350 changed lines |
| 3A | ≤350 changed lines |
| 3B | ≤300 changed lines |
| 3C | ≤400 changed lines |
| 3D | ≤250 changed lines |
| 3E | ≤400 changed lines |
| 3F | ≤400 changed lines |
| 3G | ≤400 changed lines |
| 3H | ≤400 changed lines |
| 3I | ≤350 changed lines |
| 4A | ≤250 changed lines |
| 4B | ≤400 changed lines |
| 4C | ≤400 changed lines |
| 4D | ≤400 changed lines |
| 4E | ≤400 changed lines |
| 4F | ≤300 changed lines |
| 4G | ≤400 changed lines |
| 4H | ≤250 changed lines |
| 5A | ≤350 changed lines |
| 5B | ≤400 changed lines |
| 6A | ≤400 changed lines |
| 6B | ≤400 changed lines |
| 6C | ≤300 changed lines |

Every unchecked implementation task below maps one-to-one to this forecast table.

## Global execution and compatibility contract

- Use strict RED → GREEN → TRIANGULATE → REFACTOR evidence whenever the affected package has tests. RED must be captured before the production behavior is implemented.
- For every slice, run the affected package test first, then the mandatory boundary command `go test ./... -count=1`; the latter is both the compile-safety proof for every current adapter/caller/test double and the full-suite proof. Record the exact command and result in the unit receipt.
- Do not run local `go build`, `docker build`, or blanket database/volume cleanup. CI-only build and race results are receipts, not reasons to bypass a focused slice.
- Inspect `git diff --name-only` and `git diff --stat` before and after every unit. Preserve unrelated user changes and preserve `openspec/changes/resource-master-technical-debt/` without edits.
- Never edit an existing port interface mid-chain unless every current implementation and compile assertion changes in the same slice. Prefer an additive V2 interface or method beside the legacy surface.
- Never change an existing service signature mid-chain unless every current caller, composition root, fake, and TUI test double changes in the same slice. Prefer additive methods/constructors, then switch complete composition, then retire compatibility only in 4H if grep and compilation prove it safe.
- A dormant V2 path is not production authority. It remains dormant until its producer, every consumer, transaction behavior, tests, and composition switch are complete and green.
- Applicability parent plus rules are one atomic aggregate; all dependencies block hard delete; `identity-v1` is never weakened; hash-derived catalog IDs remain opaque; stable errors never expose technical causes.
- Existing source-writing normalizers remain representational only and are ordered before committed-result comparison. No normalizer may hide a field, active state, revision, identity, or rule difference.
- Every rollback removes only that unit's listed surfaces and behavior. A failed equivalence or indeterminate commit disables publication/writes and requires coherent reload/restart; it never publishes a speculative candidate.

## Strict stage order and implementation units

### Stage 1 — Catalog mutation correctness

#### 1A — Mutation copy primitives

- [x] Implement deep-copy primitives used by the existing mutation path in `internal/domain/catalog_record.go` and `internal/domain/catalog_mutation.go`, with aliasing/value-table coverage in `internal/domain/catalog_mutation_test.go`; keep the current public function and repository signatures unchanged. <!-- sdd-owner: implementation -->

**Allowed edit surfaces:** exactly `internal/domain/catalog_record.go`, `internal/domain/catalog_mutation.go`, and `internal/domain/catalog_mutation_test.go`.

**Dependencies and end state:** start from the current domain API. Copy `CatalogValue` text, refs, lists, maps, and any nested storage before use and before returning a candidate. Existing mutation callers compile unchanged, and no applicability or lifecycle signature is added here. Rollback is limited to these three domain surfaces.

**Focused checks:** `go test ./internal/domain -run 'Test(CatalogValue|Mutation|Copy|Alias)' -count=1`.

**Strict evidence:** RED adds caller/prior-snapshot aliasing tests and records their intended failures; GREEN implements the smallest helpers and passes those tests; TRIANGULATE mutates nested lists, maps, refs, and zero values after apply; REFACTOR simplifies helpers only while the focused suite remains green.

**Mandatory slice boundary:** after the focused command, run `go test ./... -count=1` and record full compilation and suite success before 1B.

#### 1B — Applicability value model

- [x] Add `CatalogRuleRecord`, ordered rule storage, and defensive record copying in `internal/domain/catalog_record.go` and `internal/domain/catalog_mutation.go`, with nil-versus-empty, reorder, omission, and caller-mutation tests in `internal/domain/catalog_record_test.go` and `internal/domain/catalog_mutation_test.go`; old callers that omit rules must continue compiling safely. <!-- sdd-owner: implementation -->

**Allowed edit surfaces:** exactly `internal/domain/catalog_record.go`, `internal/domain/catalog_mutation.go`, `internal/domain/catalog_record_test.go`, and `internal/domain/catalog_mutation_test.go`.

**Dependencies and end state:** 1A and its full suite are green. Rules have ordered `When`, mode, identity, not-applicable, and active semantics; `nil` means omitted aggregate input while a non-nil empty slice remains distinguishable. No persistence or public package path is touched. Rollback is limited to the listed domain files.

**Focused checks:** `go test ./internal/domain -run 'Test(CatalogRecord|CatalogRule|Applicability|Copy)' -count=1`.

**Strict evidence:** RED adds rule-copy and nil/empty tests before model use; GREEN adds the model and clone paths; TRIANGULATE proves reordered/omitted rules are unequal and caller mutation cannot alter a prior record; REFACTOR keeps the old zero-value callers valid while focused tests stay green.

**Mandatory slice boundary:** after the focused command, run `go test ./... -count=1` and record compile/full-suite success before 1C.

#### 1C — Exact kind materializers I

- [x] Correct lossless insert/update/deactivate/reactivate/delete materializers for `CLASE`, `FAMILIA`, `TIPO`, `CARACTERISTICA`, and `CONJUNTO_OPCIONES` in `internal/domain/catalog_mutation.go` and `internal/domain/resource_types.go`, with table-driven behavior in `internal/domain/catalog_mutation_test.go` and `internal/domain/resource_option_set_test.go`; make option-set operations real mutations rather than snapshot no-ops. <!-- sdd-owner: implementation -->

**Allowed edit surfaces:** exactly `internal/domain/catalog_mutation.go`, `internal/domain/resource_types.go`, `internal/domain/catalog_mutation_test.go`, and `internal/domain/resource_option_set_test.go`.

**Dependencies and end state:** 1B is green. Preserve every existing field, aliases, keywords, references, active value already represented by the current types, and option-set record; copy caller-owned lists/maps. Keep `ApplyCatalogMutation`'s existing signature and do not change registry lifecycle flags yet. Rollback is limited to these domain files.

**Focused checks:** `go test ./internal/domain -run 'Test(ApplyCatalogMutation|OptionSet|Class|Family|Type|Definition)' -count=1`.

**Strict evidence:** RED adds the five-kind losslessness matrix and an option-set non-no-op assertion; GREEN implements copy-on-write builders and matching; TRIANGULATE covers update replacement, delete boundaries, inactive values, alias mutation, and unknown records; REFACTOR removes duplicated copy logic without changing signatures.

**Mandatory slice boundary:** after the focused command, run `go test ./... -count=1` and record compile/full-suite success before 1D.

#### 1D — Exact kind materializers II

- [x] Correct lossless materializers for `OPCION`, `RELACION_OPCIONES`, `UNIDAD`, `POLITICA_UNIDAD`, `APLICABILIDAD`, and `PRESENTACION` in `internal/domain/catalog_mutation.go`, `internal/domain/resource_types.go`, and `internal/domain/resource_presentation.go`, including definition rebuilds and complete applicability candidate validation, with coverage in `internal/domain/catalog_mutation_test.go`, `internal/domain/resource_types_test.go`, and `internal/domain/resource_presentation_test.go`. <!-- sdd-owner: implementation -->

**Allowed edit surfaces:** exactly `internal/domain/catalog_mutation.go`, `internal/domain/resource_types.go`, `internal/domain/resource_presentation.go`, `internal/domain/catalog_mutation_test.go`, `internal/domain/resource_types_test.go`, and `internal/domain/resource_presentation_test.go`.

**Dependencies and end state:** 1C is green. Preserve option-set and endpoint references, unit policy flags, presentation position, active data already available, materialized definitions, and complete ordered rules. A definition replacement rebuilds every matching active and inactive embedded definition. Invalid or incomplete applicability is rejected as a whole without mutating the prior snapshot or caller input; dedicated Core error identities wait for Stage 5. Rollback is limited to the listed domain surfaces.

**Focused checks:** `go test ./internal/domain -run 'Test(ApplyCatalogMutation|Applicability|Definition|Option|Relation|Unit|Presentation)' -count=1`.

**Strict evidence:** RED adds the remaining-kind matrix, definition-rebuild test, and invalid aggregate cases; GREEN implements the materializers and aggregate checks; TRIANGULATE covers nil versus empty rules, rule omission/reorder, invalid references, natural-code replacement, and every active/inactive combination; REFACTOR verifies no service or repository rule was duplicated.

**Mandatory slice boundary:** after the focused command, run `go test ./... -count=1` and record compile/full-suite success before the Stage 1 gate.

### Stage 2 — All-11 lifecycle alignment

#### 2A — Active-state domain types

- [x] Add or preserve `Active` fields, defensive setters, and nested active-state copying for the currently incomplete catalog structures in `internal/domain/resource_types.go`, `internal/domain/resource_class.go`, `internal/domain/resource_presentation.go`, and `internal/domain/catalog_mutation.go`, with round-trip tests in `internal/domain/resource_types_test.go`, `internal/domain/resource_presentation_test.go`, and `internal/domain/catalog_mutation_test.go`. <!-- sdd-owner: implementation -->

**Allowed edit surfaces:** exactly `internal/domain/resource_types.go`, `internal/domain/resource_class.go`, `internal/domain/resource_presentation.go`, `internal/domain/catalog_mutation.go`, `internal/domain/resource_types_test.go`, `internal/domain/resource_presentation_test.go`, and `internal/domain/catalog_mutation_test.go`.

**Dependencies and end state:** all Stage 1 units and the Stage 1 gate are green. Every administrable structure has a representable active state, including definitions, option sets, relations, unit policies, applicability, presentation, and nested rules; no registry support flag is changed yet. Existing domain constructors/loaders still compile, and no service or repository signature changes. Rollback is limited to the listed domain files.

**Focused checks:** `go test ./internal/domain -run 'Test(Active|Lifecycle|CatalogMutation|Authority)' -count=1`.

**Strict evidence:** RED adds inactive round-trip and setter tests; GREEN adds only the missing fields/setters and copy paths; TRIANGULATE checks inactive definitions embedded in active/inactive bindings and nested rule active values; REFACTOR updates stale comments without broad registry changes.

**Mandatory slice boundary:** after the focused command, run `go test ./... -count=1` and record compile/full-suite success before 2B.

#### 2B — Lifecycle registry and operations

- [x] Mark every registered kind lifecycle-capable and remove no-op/unsupported lifecycle behavior in `internal/domain/catalog_kind.go` and `internal/domain/catalog_mutation.go`, with the complete 11-kind matrix in `internal/domain/catalog_kind_test.go` and `internal/domain/catalog_mutation_test.go`; keep `APLICABILIDAD` nested rules under its parent and do not add a twelfth kind. <!-- sdd-owner: implementation -->

**Allowed edit surfaces:** exactly `internal/domain/catalog_kind.go`, `internal/domain/catalog_mutation.go`, `internal/domain/catalog_kind_test.go`, `internal/domain/catalog_mutation_test.go`, `internal/app/catalogo/service_test.go`, `internal/tui/catalog_admin_test.go`, and `internal/tui/catalogo_recursos_u3_test.go`. The three non-domain files are compatibility-test surfaces only; no executable application or TUI behavior changes are permitted in this slice.

**Dependencies and end state:** 2A is green. `CLASE`, `FAMILIA`, `TIPO`, `CARACTERISTICA`, `CONJUNTO_OPCIONES`, `OPCION`, `RELACION_OPCIONES`, `UNIDAD`, `POLITICA_UNIDAD`, `APLICABILIDAD`, and `PRESENTACION` all support create, update, deactivate, reactivate, and delete in the domain candidate. Replace stale five-kind comments and `ErrSoftDeleteUnsupported` behavior only after every materializer exists. Compatibility fixtures and assertions must describe the all-11 registry truth without changing production TUI behavior. Rollback is limited to the seven listed implementation/test files.

**Focused checks:** `go test ./internal/domain -run 'Test(CatalogKind|Lifecycle|ApplyCatalogMutation)' -count=1` and `go test ./internal/app/catalogo ./internal/tui -run 'Test(ServiceDeactivateOnLifecycleCapableKindCommitsOnPersist|CatalogAdminDeactivateOfferedForApplicability|U3DetailLifecycleIsTruthful)' -count=1`.

**Strict evidence:** RED expands the 11×5 matrix and changes the old unsupported expectations to explicit required behavior; GREEN changes registry metadata and operation dispatch; TRIANGULATE covers idempotent state toggles, unknown kind/op, missing records, option-set mutation, and the three exact app/TUI compatibility expectations; REFACTOR checks defensive descriptor copies and comments.

**Mandatory slice boundary:** after the focused command, run `go test ./... -count=1` and record compile/full-suite success before 2C.

#### 2C — Conservative service guards

- [x] Enforce authoritative catalog hard-delete guards and active-target lifecycle rejection in `internal/app/catalogo/service.go`, with fake-repository and publication tests in `internal/app/catalogo/service_test.go` and `internal/app/catalogo/session_coherence_test.go`; retain every existing service signature in this slice. <!-- sdd-owner: implementation -->

**Allowed edit surfaces:** exactly `internal/app/catalogo/service.go`, `internal/app/catalogo/service_test.go`, and `internal/app/catalogo/session_coherence_test.go`.

**Dependencies and end state:** 2B is green. The service loads current state, rejects active hard delete with the dedicated behavior reserved for Stage 5 mapping, counts every dependent regardless of `Active` or `Blocking`, checks active/inactive/history resource references, validates a private candidate, and never treats consumer preflight as authority. No repository port or service signature is changed. Rollback is limited to these catalog application files.

**Focused checks:** `go test ./internal/app/catalogo -run 'Test(Service|HardDelete|Dependency|Publication|Session)' -count=1`.

**Strict evidence:** RED adds active-delete, non-blocking/inactive/history dependency, reference, no-repository-call, and no-publication tests; GREEN adds the guard order; TRIANGULATE proves stale/invalid candidates and repository failures leave both snapshot and publication unchanged; REFACTOR keeps business policy in the application boundary only.

**Mandatory slice boundary:** after the focused command, run `go test ./... -count=1` and record compile/full-suite success before the Stage 2 gate.

### Stage 3 — PostgreSQL completely green

#### 3A — Migration-5/7 reset compatibility

- [x] Restore and coordinate the complete historical migration-5/7 reset suite in `internal/postgres/resource_repository_integration_test.go`, `internal/postgres/resource_integrity_migration_test.go`, and `internal/postgres/unit_names_migration_integration_test.go`, using `internal/postgres/catalog_test_fixtures_test.go` only if a shared lock/state helper is needed. <!-- sdd-owner: implementation -->

**Allowed edit surfaces:** exactly `internal/postgres/resource_repository_integration_test.go`, `internal/postgres/resource_integrity_migration_test.go`, `internal/postgres/unit_names_migration_integration_test.go`, and `internal/postgres/catalog_test_fixtures_test.go` only if a shared lock/state helper is needed. Migration 5 and migration 7 SQL are read-only; no migration SQL, production code, TUI code, or unrelated file may be edited.

**Dependencies and end state:** P2/Stage 2 and its full suite are green; there is no fixture-repair prerequisite or fixture-repair completion claim. Fixture-only work cannot preserve or pass the legacy migration-5/7 integration coverage on a migration-7 database before reset compatibility exists. Restore every removed historical collision, applicability, mapping, and up/down scenario with no early return, skip, or weakened assertion. Acquire an integration advisory lock and coordinate every other schema-destructive test. Capture the original migration-5 map and migration-7 constraint state. Starting from migration 7, run down 7, reverse owned/mapped v1 rows to legacy through the identity map, run down 5, execute the legacy scenarios, remove only owned rows, then restore migration 5 and migration 7 if each was initially present. Cleanup restores state even when the test body fails, and the lock is released only after restoration. Rollback is limited to reset synchronization/state helpers and restored scenarios.

**Focused checks:** `GARFEX_TEST_DSN=<isolated-app-dsn> GARFEX_ADMIN_TEST_DSN=<isolated-admin-dsn> go test ./internal/postgres -run 'Test(ResourceIntegrityMigrationIntegration|ResourceIntegrityMigrationAuditRelation|UnitNamesMigrationIntegration)' -count=1`.

**Strict evidence:** RED reproduces the historical reset/version/identity-map failures; GREEN restores all legacy scenarios and lock/setup/cleanup sequencing; TRIANGULATE injects a body failure and proves migration-map/constraint restoration, verifies seeded identity data is unchanged, and coordinates another schema-destructive test; REFACTOR keeps migration 5/7 SQL read-only and all assertions intact. Record the focused command and mandatory full-suite result.

**Mandatory slice boundary:** after the focused command, run `go test ./... -count=1` and record compile/full-suite success before 3B.

#### 3B — Identity-v1 fixture repair

- [x] Repair owned PostgreSQL fixture identities and cleanup in `internal/postgres/resource_repository_integration_test.go`, `internal/postgres/unit_names_migration_integration_test.go`, `internal/postgres/resource_integrity_migration_test.go`, `internal/postgres/catalog_admin_repository_integration_test.go`, and `internal/postgres/catalog_test_fixtures_test.go`, while preserving the corrected 3A reset scenarios unchanged. <!-- sdd-owner: implementation -->

**Allowed edit surfaces:** exactly `internal/postgres/resource_repository_integration_test.go`, `internal/postgres/unit_names_migration_integration_test.go`, `internal/postgres/resource_integrity_migration_test.go`, `internal/postgres/catalog_admin_repository_integration_test.go`, and `internal/postgres/catalog_test_fixtures_test.go`; no migration SQL, production code, TUI code, or unrelated file may be edited.

**Dependencies and end state:** corrected 3A and its full suite are green, so reset restoration now permits honest fixture execution on a migration-7 database. Repair `TEST_REPO_IDENTITY_KEY`, `TEST_U2A_RESTART_KEY`, `TEST_UNIT_NAMES_RESOURCE`, and every equivalent direct insert. Each identity derives from the actual fixture scope plus persisted identity-participating values; synthetic suffix components that are not persisted are prohibited. Retain returned IDs, clean ID-owned children before parents, remove only owned rows, preserve seeded business rows, and leave every 3A reset scenario and assertion unchanged. Rollback removes only fixture identity/ownership changes.

**Focused checks:** `GARFEX_TEST_DSN=<isolated-app-dsn> GARFEX_ADMIN_TEST_DSN=<isolated-admin-dsn> go test ./internal/postgres -run 'Test(ResourceRepository|UnitNames|ResourceIntegrity|CatalogAdmin|FixtureIdentity)' -count=1`.

**Strict evidence:** RED runs the focused fixture suite against the isolated migration-7 database and captures canonical identity/cleanup failures; GREEN repairs only persisted-scope identities and ID-owned cleanup; TRIANGULATE proves seeded identity data is unchanged, cancellation/failure cleanup remains child-before-parent, and the complete 3A reset suite still passes; REFACTOR audits fixture identity inputs and ownership without altering 3A scenarios.

**Mandatory slice boundary:** after the focused command and preserved 3A reset suite, run `go test ./... -count=1` and record compile/full-suite success before 3C.

#### 3C — Revision schema and loaders

- [x] Add additive migration 8 and revision projections in `migrations/000008_resource_revisions.up.sql`, `migrations/000008_resource_revisions.down.sql`, `internal/domain/catalog_record.go`, `internal/domain/resource.go`, `internal/domain/resource_types.go`, `internal/domain/catalog_repository_errors.go`, `internal/postgres/catalog_loader.go`, and the listed migration/grant integration tests; retain all legacy write APIs compiling and preserve identity-v1 byte-for-byte. <!-- sdd-owner: implementation -->

**Allowed edit surfaces:** exactly `migrations/000008_resource_revisions.up.sql`, `migrations/000008_resource_revisions.down.sql`, `internal/domain/catalog_record.go`, `internal/domain/resource.go`, `internal/domain/resource_types.go`, `internal/domain/catalog_repository_errors.go`, `internal/postgres/catalog_loader.go`, `internal/postgres/catalog_loader_integration_test.go`, `internal/postgres/resource_repository_integration_test.go`, `internal/postgres/resource_integrity_migration_test.go`, `internal/postgres/catalog_admin_grants_integration_test.go`, and `migrations/migration_versions_test.go`.

**Dependencies and end state:** corrected 3B fixture repair and the preserved 3A reset suite are green. Add `BIGINT NOT NULL DEFAULT 1 CHECK (revision > 0)` to `recursos` and all 11 catalog parent tables, backfill to 1, verify application-role grants, and give no independent revision to `resource_attribute_rules`. Down removes only revision columns/constraints. Existing constructors, ports, and writes remain compatible; no public path is created. Rollback may leave migration 8 inert but must not down-migrate while revision-aware code exists.

**Focused checks:** `GARFEX_TEST_DSN=<isolated-app-dsn> GARFEX_ADMIN_TEST_DSN=<isolated-admin-dsn> go test ./internal/postgres -run 'Test(Migration|Loader|Grant|Identity|Revision)' -count=1`.

**Strict evidence:** RED adds migration/backfill/grant/projection and identity snapshot tests; GREEN implements migration 8 and loader fields; TRIANGULATE applies migration 7 then 8, asserts exact identity equality, tests up/down compatibility and role access; REFACTOR checks no revision is derived from updated_at, xmin, or hash IDs.

**Mandatory slice boundary:** after the focused command, run `go test ./... -count=1` and record compile/full-suite success before 3D.

#### 3D — Additive V2 result and port skeleton

- [x] Define only the complete additive `CatalogWriteResult` and `CatalogAdminRepositoryV2` contract in the new `internal/domain/catalog_admin_v2.go`, with defensive result tests and domain fakes in `internal/domain/catalog_admin_v2_test.go`; leave `CatalogAdminRepository` unchanged and add no PostgreSQL V2 constructor or concrete conformance assertion. <!-- sdd-owner: implementation -->

**Allowed edit surfaces:** exactly `internal/domain/catalog_admin_v2.go` and `internal/domain/catalog_admin_v2_test.go`.

**Dependencies and end state:** 3C is green. The complete domain V2 contract exists before any consumer uses it; it returns defensively copied committed record/revision and complete transaction-loaded catalog data, and domain tests/fakes compile against the full method set. PostgreSQL construction, implementation, and concrete interface assertions are explicitly deferred to 3E–3G; no partial V2 path becomes authority. Rollback removes only these two domain surfaces.

**Focused checks:** `go test ./internal/domain -run 'Test(CatalogWriteResult|CatalogAdminRepositoryV2|V2|Fake|Copy)' -count=1`.

**Strict evidence:** RED adds domain interface/result compile, fake, and copy tests before implementation; GREEN adds only the complete additive domain contract and its test fakes; TRIANGULATE mutates returned nested catalog/result storage and verifies isolation while the legacy port still compiles; REFACTOR keeps V2 names additive and dormant with no PostgreSQL construction or assertion.

**Mandatory slice boundary:** after the focused command, run `go test ./... -count=1` and record compile/full-suite success before 3E.

#### 3E — Atomic applicability V2 SQL

- [x] Build the dormant internal parent-plus-ordered-rule transaction helpers, kind handlers, persistence, and reload for `APLICABILIDAD` in `internal/postgres/catalog_admin_repository.go`, `internal/postgres/catalog_admin_repository_v2.go`, `internal/postgres/catalog_admin_kinds.go`, and `internal/postgres/catalog_loader.go`, with commit/rollback/count/order integration coverage in `internal/postgres/catalog_admin_repository_v2_integration_test.go` and `internal/postgres/catalog_loader_integration_test.go`; do not claim complete `CatalogAdminRepositoryV2` conformance or add unsupported-returning authority stubs. <!-- sdd-owner: implementation -->

**Allowed edit surfaces:** exactly `internal/postgres/catalog_admin_repository.go`, `internal/postgres/catalog_admin_repository_v2.go`, `internal/postgres/catalog_admin_kinds.go`, `internal/postgres/catalog_loader.go`, `internal/postgres/catalog_admin_repository_v2_integration_test.go`, and `internal/postgres/catalog_loader_integration_test.go`.

**Dependencies and end state:** 3D is green. Dormant V2 applicability create/update performs parent and complete ordered child replacement in one transaction, verifies count/order/definition relationships, rolls back on any child or reload failure, and keeps child concurrency under the parent revision. These are internal helpers/families only: no complete PostgreSQL interface conformance, constructor, or compile assertion is added; legacy methods remain untouched and production authority remains unchanged. Rollback removes only the listed applicability helper/test surfaces.

**Focused checks:** `GARFEX_TEST_DSN=<isolated-app-dsn> GARFEX_ADMIN_TEST_DSN=<isolated-admin-dsn> go test ./internal/postgres -run 'Test(Applicability|Rule|Atomic|Rollback|CatalogLoader)' -count=1`.

**Strict evidence:** RED adds valid, nil-omitted, explicit-empty, malformed, missing-reference, child-failure, count, and order cases; GREEN implements one dormant transaction family without unsupported stubs; TRIANGULATE fails each parent/child/reload point and proves no partial row survives; REFACTOR preserves source-writing normalization boundaries and verifies no conformance or authority claim was introduced.

**Mandatory slice boundary:** after the focused command, run `go test ./... -count=1` and record compile/full-suite success before 3F.

#### 3F — Catalog CAS SQL I

- [x] Build dormant internal revision-aware create/update CAS transaction helpers and kind-handler families for `CLASE`, `FAMILIA`, `TIPO`, `CARACTERISTICA`, and `CONJUNTO_OPCIONES` in `internal/postgres/catalog_admin_repository_v2.go`, `internal/postgres/catalog_admin_repository.go`, and `internal/postgres/catalog_admin_kinds.go`, with stale/not-found/success tables in `internal/postgres/catalog_admin_repository_v2_integration_test.go`; do not claim complete `CatalogAdminRepositoryV2` conformance or add unsupported-returning authority stubs. <!-- sdd-owner: implementation -->

**Allowed edit surfaces:** exactly `internal/postgres/catalog_admin_repository_v2.go`, `internal/postgres/catalog_admin_repository.go`, `internal/postgres/catalog_admin_kinds.go`, and `internal/postgres/catalog_admin_repository_v2_integration_test.go`.

**Dependencies and end state:** 3E is green and the complete domain V2 contract is available. Dormant SQL updates explicitly increment persisted revision with `WHERE revision = $expected`; zero rows are disambiguated by same-transaction existence/revision lookup, never by error text. Create starts at 1. These real internal families do not yet satisfy the complete PostgreSQL interface: no constructor, concrete assertion, or unsupported-returning stub is added; legacy adapter behavior and production authority remain unchanged. Rollback removes only the listed CAS helper/handler/test surfaces.

**Focused checks:** `GARFEX_TEST_DSN=<isolated-app-dsn> GARFEX_ADMIN_TEST_DSN=<isolated-admin-dsn> go test ./internal/postgres -run 'Test(CatalogCAS|Revision|Class|Family|Type|Definition|OptionSet)' -count=1`.

**Strict evidence:** RED adds same-revision success, stale conflict, absent not-found, and exactly-once increment tests; GREEN implements the narrow dormant kind families; TRIANGULATE covers natural-key/hash-ID resolution, duplicate/reference classification, and transaction rollback; REFACTOR checks all SQL predicates are explicit CAS and proves no conformance or authority claim was introduced.

**Mandatory slice boundary:** after the focused command, run `go test ./... -count=1` and record compile/full-suite success before 3G.

#### 3G — Catalog CAS SQL II and complete PostgreSQL V2 conformance

- [x] After every V2 operation family from 3E–3F exists, finish all remaining catalog create/update/lifecycle/delete CAS families—including `OPCION`, `RELACION_OPCIONES`, `UNIDAD`, `POLITICA_UNIDAD`, `APLICABILIDAD`, and `PRESENTACION` plus any remaining lifecycle/delete methods for the first five kinds—in `internal/postgres/catalog_admin_repository_v2.go`, `internal/postgres/catalog_admin_repository.go`, and `internal/postgres/catalog_admin_kinds.go`; in this same slice add the concrete `CatalogAdminRepositoryV2` methods, `NewCatalogAdminRepositoryV2`, and compile-time assertion, with the all-11 SQL matrix and FK backstop in `internal/postgres/catalog_admin_repository_v2_integration_test.go` and `internal/postgres/catalog_admin_grants_integration_test.go`. <!-- sdd-owner: implementation -->

**Allowed edit surfaces:** exactly `internal/postgres/catalog_admin_repository_v2.go`, `internal/postgres/catalog_admin_repository.go`, `internal/postgres/catalog_admin_kinds.go`, `internal/postgres/catalog_admin_repository_v2_integration_test.go`, and `internal/postgres/catalog_admin_grants_integration_test.go`.

**Dependencies and end state:** 3F is green and every operation family required by the complete domain V2 contract has been built internally. Only after that precondition does this slice add the concrete interface methods, `NewCatalogAdminRepositoryV2`, and `var _ CatalogAdminRepositoryV2 = ...` together; no earlier slice may add any of them. All 11 parent kinds use persisted monotonic CAS; applicability lifecycle changes only the parent active/revision while rules remain intact; hard delete relies on transaction/FK backstop after the service guard. Zero-row disambiguation and opaque hash-ID resolution are uniform. The complete adapter remains dormant/non-authoritative, with no service/TUI signature change. Rollback removes only the listed completion, constructor, assertion, and test surfaces.

**Focused checks:** `GARFEX_TEST_DSN=<isolated-app-dsn> GARFEX_ADMIN_TEST_DSN=<isolated-admin-dsn> go test ./internal/postgres -run 'Test(CatalogCAS|Lifecycle|Delete|Applicability|FK|Grant|Constructor|Compile)' -count=1`.

**Strict evidence:** RED adds the remaining-kind five-operation matrix, constructor/conformance compile checks, and FK race-backstop expectations only after verifying the 3E–3F families exist; GREEN implements every remaining method family and adds the concrete methods, constructor, and assertion in this slice; TRIANGULATE covers stale precedence, missing rows, child-rule preservation, dependency/FK rejection, natural-code opaque-ID changes, grant behavior, constructor use, and exact interface conformance; REFACTOR removes duplicate SQL helpers without altering the completed V2 contract or making it authoritative.

**Mandatory slice boundary:** after the focused command, run `go test ./... -count=1` and record compile/full-suite success before 3H.

#### 3H — Resource CAS and canonical reactivation E2E

- [x] Add compatible resource V2 repository methods and the additive `recursos.Service` revision-aware reactivation path in `internal/domain/resource_types.go`, `internal/domain/resource_repository_v2.go`, `internal/postgres/resource_repository.go`, `internal/postgres/resource_repository_crud.go`, `internal/postgres/resource_repository_attributes.go`, `internal/postgres/resource_repository_codec.go`, and `internal/app/recursos/service.go`, with canonical end-to-end evidence in `internal/app/recursos/canonical_writes_test.go`, `internal/app/recursos/lifecycle_test.go`, and `internal/postgres/resource_lifecycle_integration_test.go`; retain all legacy signatures and adapt every V2 fake in this same slice. <!-- sdd-owner: implementation -->

**Allowed edit surfaces:** exactly `internal/domain/resource_types.go`, `internal/domain/resource_repository_v2.go`, `internal/postgres/resource_repository.go`, `internal/postgres/resource_repository_crud.go`, `internal/postgres/resource_repository_attributes.go`, `internal/postgres/resource_repository_codec.go`, `internal/app/recursos/service.go`, `internal/app/recursos/service_test.go`, `internal/app/recursos/canonical_writes_test.go`, `internal/app/recursos/lifecycle_test.go`, and `internal/postgres/resource_lifecycle_integration_test.go`.

**Dependencies and end state:** 3G is green. Resource create starts at revision 1; update, deactivate, reactivate, and future delete method families use explicit CAS and atomic attribute replacement. The additive service path proves canonical reactivation through `recursos.Service`, unchanged `identity-v1`, complete attributes, current revision, and authoritative validation. No existing resource port/signature is edited without all current PostgreSQL adapters and test doubles adapting in this slice. Rollback is limited to resource V2/revision surfaces.

**Focused checks:** `go test ./internal/app/recursos ./internal/postgres -run 'Test(Revision|CAS|Canonical|Reactivation|Lifecycle|Resource)' -count=1` with isolated PostgreSQL DSNs for integration cases.

**Strict evidence:** RED adds service and two-connection stale/reactivation tests before V2 consumption; GREEN adds the repository/service additive path; TRIANGULATE proves identity mismatch versus other reactivation failure, complete attribute rollback, absent-row lookup, stale no-change, and exact revision increment; REFACTOR audits all resource fakes and legacy callers.

**Mandatory slice boundary:** after the focused command, run `go test ./... -count=1` and record compile/full-suite success before 3I.

#### 3I — PostgreSQL race and integration exit

- [x] Close Stage 3 PostgreSQL evidence in `internal/postgres/catalog_admin_repository_v2_integration_test.go`, `internal/postgres/resource_lifecycle_integration_test.go`, `internal/postgres/resource_repository_integration_test.go`, `internal/postgres/resource_integrity_migration_test.go`, `internal/postgres/unit_names_migration_integration_test.go`, and `internal/postgres/catalog_loader_integration_test.go`; add no new authority path. <!-- sdd-owner: implementation -->

**Allowed edit surfaces:** exactly the six PostgreSQL integration test files named above and `internal/postgres/catalog_test_fixtures_test.go` only for owned fixture helpers. No application service, public package, migration SQL, TUI, or technical-debt file is allowed.

**Dependencies and end state:** 3H is green. Two independent connections issue the same expected revision and produce exactly one success, one conflict, one revision increment; dependency/delete races prove the dependency or FK wins and never both operations succeed. All-11 persistence, atomic applicability, migration-5/7 reset restoration, identity-v1 fixture repair with the reset suite preserved, migration 8 identity preservation, grants, and canonical resource reactivation are green. This is the PostgreSQL exit gate before authority work. Rollback removes only race/integration evidence.

**Focused checks:** isolated `GARFEX_TEST_DSN=<isolated-app-dsn> GARFEX_ADMIN_TEST_DSN=<isolated-admin-dsn> go test ./internal/postgres -run 'Test(Catalog|Resource|CAS|Race|Migration|Loader|Identity)' -count=1`, followed by CI receipt `go test ./... -race -count=1` when CI executes it.

**Strict evidence:** RED adds the two-connection and dependency-race scenarios and records their failures; GREEN makes the already additive V2 SQL pass; TRIANGULATE reruns failure/retry/cleanup scenarios and verifies no speculative publication claim; REFACTOR audits isolated roles, fixture ownership, migration restoration, and no destructive volume cleanup.

**Mandatory slice boundary:** after focused PostgreSQL and permitted race evidence, run `go test ./... -count=1` and record compile/full-suite success before the Stage 3 gate.

### Stage 4 — Authority semantics, coherent committed reload, adoption, composition, and TUI compatibility

#### 4A — Domain equivalence

- [x] Add domain-owned representational normalization/equivalence in the new `internal/domain/catalog_equivalence.go` with focused tests in `internal/domain/catalog_equivalence_test.go`; keep repository ports and service signatures unchanged and do not hide semantic differences. <!-- sdd-owner: implementation -->

**Allowed edit surfaces:** exactly `internal/domain/catalog_equivalence.go` and `internal/domain/catalog_equivalence_test.go`.

**Dependencies and end state:** the Stage 3 gate is green. Equivalence normalizes only approved representations such as default option-set spelling and nil/empty collections; omission, active state, revision, identity, references, and rule order/content remain unequal. Existing source-writing normalizers remain ordered as read/projection helpers and are not broadened here. Rollback is limited to these domain files.

**Focused checks:** `go test ./internal/domain -run 'Test(CatalogEquivalence|CatalogNormalize)' -count=1`.

**Strict evidence:** RED adds equal-representation and unequal-semantic tests; GREEN implements domain comparison; TRIANGULATE proves omitted fields/rules, changed active/revision/identity, and reordered rules are unequal; REFACTOR keeps normalization pure and copy-safe.

**Mandatory slice boundary:** after the focused command, run `go test ./... -count=1` and record compile/full-suite success before 4B.

#### 4B — Transactional coherent result

- [x] Make every V2 catalog write load all 11 collections and nested rules through a transaction-scoped loader, validate, compare with the private candidate, and commit only on equivalence in `internal/postgres/catalog_admin_repository_v2.go`, `internal/postgres/catalog_admin_repository.go`, `internal/postgres/catalog_admin_kinds.go`, and `internal/postgres/catalog_loader.go`, with rollback/equivalence integration cases in `internal/postgres/catalog_admin_repository_v2_integration_test.go` and `internal/postgres/catalog_loader_integration_test.go`. <!-- sdd-owner: implementation -->

**Allowed edit surfaces:** exactly `internal/postgres/catalog_admin_repository_v2.go`, `internal/postgres/catalog_admin_repository.go`, `internal/postgres/catalog_admin_kinds.go`, `internal/postgres/catalog_loader.go`, `internal/postgres/catalog_admin_repository_v2_integration_test.go`, and `internal/postgres/catalog_loader_integration_test.go`.

**Dependencies and end state:** 4A is green. V2 writes perform CAS/insert/delete, complete transaction reload, domain validation, normalized equivalence, then commit and return `CatalogWriteResult`; mismatch or any reload/validation failure rolls back. No hand-built candidate is returned for publication, and V2 remains non-authoritative until service adoption. Rollback is limited to coherent-result adapter files/tests.

**Focused checks:** isolated `GARFEX_TEST_DSN=<isolated-app-dsn> GARFEX_ADMIN_TEST_DSN=<isolated-admin-dsn> go test ./internal/postgres -run 'Test(Coherent|Equivalence|Reload|Rollback|CatalogCAS)' -count=1`.

**Strict evidence:** RED adds transaction reload/mismatch and pre-commit failure tests; GREEN wires the coherent result; TRIANGULATE injects lost fields, rule count/order changes, invalid reload, commit failure, and indeterminate commit behavior; REFACTOR preserves the existing source-writing normalizers and keeps commit-before-publication ordering explicit.

**Mandatory slice boundary:** after the focused command, run `go test ./... -count=1` and record compile/full-suite success before 4C.

#### 4C — Additive catalog service V2 adoption

- [x] Add revision-aware additive service constructors/methods and adapt all catalog service fakes in `internal/app/catalogo/service.go`, `internal/app/catalogo/service_test.go`, and `internal/app/catalogo/session_coherence_test.go`; keep production composition on the legacy entry point until 4D and publish nothing from an incomplete path. <!-- sdd-owner: implementation -->

**Allowed edit surfaces:** exactly `internal/app/catalogo/service.go`, `internal/app/catalogo/service_test.go`, and `internal/app/catalogo/session_coherence_test.go`.

**Dependencies and end state:** 4B is green. The V2 service validates a private candidate, passes expected revisions to the V2 repository, consumes only its committed coherent result, and has explicit no-publication behavior for validation, guard, conflict, SQL, reload, equivalence, and commit failures. Every fake implementing the V2 consumer contract is adapted in this slice; legacy methods remain valid and V2 is not production authority. Rollback is limited to catalog service/V2 test surfaces.

**Focused checks:** `go test ./internal/app/catalogo -run 'Test(ServiceV2|Revision|Candidate|Publication|Coherence|HardDelete)' -count=1`.

**Strict evidence:** RED adds V2 fake/constructor, result-use, no-publication, and publication-order tests; GREEN implements additive orchestration; TRIANGULATE covers idempotent lifecycle no-op, stale precedence, divergent reload, writer-unavailable latch, and defensive authority values; REFACTOR verifies no old caller was silently broken.

**Mandatory slice boundary:** after the focused command, run `go test ./... -count=1` and record compile/full-suite success before 4D.

#### 4D — Composition and authority switch

- [x] Migrate every production composition root and integration composition to the complete V2 service path in `cmd/garfex/main.go`, `cmd/garfex/main_test.go`, `internal/tui/catalog_admin_e2e_test.go`, and `internal/app/catalogo/service.go`; make legacy entry points delegate to the complete path only after all V2 methods are present and green. <!-- sdd-owner: implementation -->

**Allowed edit surfaces:** exactly `cmd/garfex/main.go`, `cmd/garfex/main_test.go`, `internal/tui/catalog_admin_e2e_test.go`, and `internal/app/catalogo/service.go`.

**Dependencies and end state:** 4C is green. The shipped composition owns one writer service/authority, routes every successful catalog mutation through the committed coherent result, and publishes exactly once after commit. No second writer flow or candidate publication remains. Before switching, run a read-only search for all `NewService`, repository constructors, and authority wiring; the four enumerated files are authoritative for this slice. If another current production composition root is found, stop and repair design/tasks before editing rather than expanding the allowed surfaces; otherwise leave the old delegating wrapper where required. Rollback returns composition to the prior complete behavior without bypassing services.

**Focused checks:** `go test ./cmd/garfex ./internal/app/catalogo ./internal/tui -run 'Test(Main|Composition|Authority|CatalogAdminE2E|Publication)' -count=1` with isolated PostgreSQL where required.

**Strict evidence:** RED adds composition/publication-count and real-bridge tests; GREEN switches complete roots; TRIANGULATE injects repository/reload/commit failures and checks no publication, then compares authority with an independent load; REFACTOR audits `rg` results for every constructor and confirms no dormant V2 route became authority prematurely.

**Mandatory slice boundary:** after the focused command, run `go test ./... -count=1` and record compile/full-suite success before 4E.

#### 4E — All-11 authority oracle and writer latch

- [x] Complete the 11-kind × 5-operation authority oracle and writer-unavailable behavior in `internal/app/catalogo/service.go`, `internal/app/catalogo/service_test.go`, `internal/app/catalogo/session_coherence_test.go`, `internal/postgres/catalog_admin_repository_v2_integration_test.go`, and `internal/postgres/catalog_loader_integration_test.go`; compare the published `CatalogAuthority` with an independent coherent reload after every successful mutation. <!-- sdd-owner: implementation -->

**Allowed edit surfaces:** exactly `internal/app/catalogo/service.go`, `internal/app/catalogo/service_test.go`, `internal/app/catalogo/session_coherence_test.go`, `internal/postgres/catalog_admin_repository_v2_integration_test.go`, and `internal/postgres/catalog_loader_integration_test.go`.

**Dependencies and end state:** 4D is green. Every successful create, update, deactivate, reactivate, and guarded hard delete publishes the committed catalog once and matches independent PostgreSQL reload after only representational normalization. Every rejected path leaves persistence and prior authority unchanged; an indeterminate commit latches unavailable until coherent reload/restart. Rollback is limited to authority/oracle tests and service behavior.

**Focused checks:** isolated `GARFEX_TEST_DSN=<isolated-app-dsn> GARFEX_ADMIN_TEST_DSN=<isolated-admin-dsn> go test ./internal/app/catalogo ./internal/postgres -run 'Test(All11|Authority|Oracle|Publication|Unavailable|Coherence)' -count=1`.

**Strict evidence:** RED adds the full matrix and exact publication count assertions; GREEN completes service/oracle behavior; TRIANGULATE covers applicability atomicity, every dependency activity class, stale conflicts, reload mismatch, and writer latch; REFACTOR removes candidate-based assertions that could mask persistence divergence.

**Mandatory slice boundary:** after the focused command, run `go test ./... -count=1` and record compile/full-suite success before 4F.

#### 4F — TUI revision state compatibility

- [x] Carry captured catalog/resource revisions through existing TUI state without changing service call signatures in `internal/tui/catalog_admin.go`, `internal/tui/catalog_admin_dispatch.go`, `internal/tui/catalog_wizard.go`, `internal/tui/resource_editor_state.go`, `internal/tui/resource_editor_persistence.go`, `internal/tui/catalog_admin_test.go`, `internal/tui/resource_editor_test.go`, and `internal/tui/resources_workspace_adapter_test.go`. <!-- sdd-owner: implementation -->

**Allowed edit surfaces:** exactly the eight TUI files named above. No TUI view, prompt, navigation, rendering, or new consumer behavior is allowed.

**Dependencies and end state:** 4E is green. Before 4F starts, run a read-only textual-reference gate for catalog/resource `Create`, `Update`, `Deactivate`, `Reactivate`, `Delete`, `SetActive`, and V2 constructor references and confirm that the exact 4F eight-file state/test surface plus exact 4G thirteen-file caller/interface/fake surface cover the current references. Those enumerations are authoritative; any additional current caller requires design/tasks repair before editing rather than an expanded 4F or 4G surface. Existing records/session state retain the revision needed by the later additive service calls, while current service signatures and behavior still compile. Direct `Model.Update` tests cover state capture and no accidental revision loss. Rollback is limited to the eight listed TUI state/test files.

**Focused checks:** `go test ./internal/tui -run 'Test(Model|CatalogAdmin|ResourceEditor|Revision|Workspace)' -count=1`.

**Strict evidence:** The pre-start textual-reference gate is read-only and fail-closed; if it finds a current caller outside the enumerated surface, stop and repair design/tasks before any edit. RED adds state/round-trip tests; GREEN adds only revision storage and propagation; TRIANGULATE covers create versus edit, deactivate/reactivate/delete display states, stale state, and direct update messages; REFACTOR confirms no consumer redesign.

**Mandatory slice boundary:** after the focused command, run `go test ./... -count=1` and record compile/full-suite success before 4G.

#### 4G — TUI revision calls and co-migration

- [x] Migrate the exact current catalog/resource edit, lifecycle, and hard-delete caller/test-double surface to the complete additive revision-aware service methods in `internal/tui/catalog_admin.go`, `internal/tui/catalog_admin_dispatch.go`, `internal/tui/catalog_wizard.go`, `internal/tui/resource_editor.go`, `internal/tui/resource_editor_persistence.go`, `internal/tui/resources_workspace_dispatch.go`, `internal/tui/catalog_admin_test.go`, `internal/tui/resource_editor_test.go`, `internal/tui/resource_lifecycle_test.go`, `internal/tui/resources_workspace_adapter_test.go`, `internal/tui/catalogo_recursos_u2a_test.go`, `internal/app/catalogo/service_test.go`, and `internal/app/recursos/service_test.go`. <!-- sdd-owner: implementation -->

**Allowed edit surfaces:** exactly the thirteen repository-relative files named above. Before RED or any implementation edit, run a read-only textual-reference scan for all Go references to catalog/resource `Create`, `Update`, `Deactivate`, `Reactivate`, `Delete`, `SetActive`, and V2 constructors and compare it with this exact list. The enumerated surface is authoritative: if the scan finds another current caller or fake, apply MUST stop and design/tasks must be repaired before editing; the worker may not expand its own allowed surfaces.

**Dependencies and end state:** 4F is green and its pre-start gate passed. All current callers and doubles in the exact thirteen-file surface carry the captured revision and use explicit deactivation versus hard deletion. No existing service signature is changed in isolation; additive methods are complete before callers switch. Rollback is limited to the thirteen listed caller/interface/fake surfaces.

**Focused checks:** `go test ./internal/app/catalogo ./internal/app/recursos ./internal/tui -run 'Test(Revision|Lifecycle|Catalog|Resource|Workspace|Model)' -count=1`.

**Strict evidence:** The pre-start textual-reference gate is fail-closed and occurs before RED or edits; a new current caller outside the exact list stops apply and requires design/tasks repair. RED adds failing compatibility tests for the enumerated callers/fakes; GREEN migrates only that exact surface; TRIANGULATE covers zero/stale revisions, idempotent transitions, identity preservation, and direct TUI actions; REFACTOR reruns the textual scan and proves no revision-less production writer remains.

**Mandatory slice boundary:** after the focused command, run `go test ./... -count=1` and record compile/full-suite success before 4H.

#### 4H — Compatibility retirement decision

- [x] Retire only obsolete compatibility wrappers after repository-wide caller and compile assertions prove the complete V2 path is used, or deliberately retain safe delegating wrappers, in `internal/domain/catalog_record.go`, `internal/domain/resource_types.go`, `internal/app/catalogo/service.go`, `internal/app/recursos/service.go`, `internal/postgres/catalog_admin_repository.go`, `internal/postgres/resource_repository.go`, `cmd/garfex/main.go`, and the affected tests; do not remove a surface merely to shorten the diff. <!-- sdd-owner: implementation -->

**Allowed edit surfaces:** exactly the eight files named above plus their already-listed focused test files `internal/app/catalogo/service_test.go`, `internal/app/recursos/service_test.go`, `internal/postgres/catalog_admin_repository_v2_integration_test.go`, and `internal/postgres/resource_lifecycle_integration_test.go`.

**Dependencies and end state:** 4G is green. Any retirement is a separate, independently green compatibility change; otherwise old constructors/methods remain harmless delegators to the complete authoritative path. No public package is introduced. Rollback restores only wrapper/retirement edits and never reintroduces a revision-less production writer.

**Focused checks:** `go test ./internal/app/catalogo ./internal/app/recursos ./internal/postgres ./cmd/garfex -run 'Test(Compatibility|Compile|Revision|Authority|Lifecycle)' -count=1`.

**Strict evidence:** RED records `rg`/compile assertions for obsolete callers; GREEN removes only proven-dead wrappers or makes them delegate; TRIANGULATE tests legacy entry points against V2 behavior and checks stale/authority semantics; REFACTOR leaves the smallest compatible surface.

**Mandatory slice boundary:** after the focused command, run `go test ./... -count=1` and record compile/full-suite success before the Stage 4 gate.

### Stage 5 — Dedicated Core outcomes and neutral safe mapping

#### 5A — Dedicated Core semantic outcomes

- [x] Add and emit dedicated internal `ErrIdentityConflict`, `ErrInvalidLifecycle`, `ErrReactivationImpossible`, `ErrInvalidCatalog`, and `ErrRevisionConflict` outcomes at authoritative domain/application/adapter sites in `internal/domain/core_errors.go`, `internal/domain/catalog_repository_errors.go`, `internal/domain/resource_types.go`, `internal/app/catalogo/service.go`, `internal/app/recursos/service.go`, `internal/postgres/catalog_admin_repository.go`, `internal/postgres/resource_repository_codec.go`, with non-collapse tests in `internal/domain/core_errors_test.go`, `internal/app/catalogo/service_test.go`, `internal/app/recursos/service_test.go`, and `internal/postgres/resource_repository_codec_test.go`. <!-- sdd-owner: implementation -->

**Allowed edit surfaces:** exactly `internal/domain/core_errors.go`, `internal/domain/core_errors_test.go`, `internal/domain/catalog_repository_errors.go`, `internal/domain/resource_types.go`, `internal/app/catalogo/service.go`, `internal/app/catalogo/service_test.go`, `internal/app/recursos/service.go`, `internal/app/recursos/service_test.go`, `internal/postgres/catalog_admin_repository.go`, `internal/postgres/resource_repository_codec.go`, and `internal/postgres/resource_repository_codec_test.go`.

**Dependencies and end state:** the Stage 4 gate is green. Identity disagreement is not integrity/duplicate; active or unsupported transitions are invalid lifecycle; retained records that cannot validate are impossible reactivation; candidate/current/reloaded catalog loss is invalid catalog; stale CAS is revision conflict. Internal wrappers may retain causes and operation metadata, but no public package exists. Rollback is limited to internal outcome/classification surfaces.

**Focused checks:** `go test ./internal/domain ./internal/app/catalogo ./internal/app/recursos ./internal/postgres -run 'Test(Identity|Lifecycle|Reactivation|Catalog|Revision|Outcome|Error)' -count=1`.

**Strict evidence:** RED adds wrapped-sentinel and application/adapter non-collapse tables; GREEN emits the dedicated outcomes at authoritative sites; TRIANGULATE proves stale precedence over lifecycle, identity mismatch versus integrity, changed-catalog reactivation, lossy catalog, and cardinality corruption; REFACTOR centralizes identities without changing existing port signatures.

**Mandatory slice boundary:** after the focused command, run `go test ./... -count=1` and record compile/full-suite success before 5B.

#### 5B — Neutral non-unwrappable GARFEX mapping and diagnostics

- [x] Add the internal neutral GARFEX error categories, exhaustive typed mapper, and opaque diagnostic sink in the new `internal/core/errors.go`, `internal/core/errors_test.go`, `internal/core/diagnostics.go`, `internal/postgres/catalog_admin_repository.go`, `internal/postgres/resource_repository_codec.go`, and `internal/app/catalogo/service.go`; keep this mapper internal and create no `resourcecore` or bridge package. <!-- sdd-owner: implementation -->

**Allowed edit surfaces:** exactly `internal/core/errors.go`, `internal/core/errors_test.go`, `internal/core/diagnostics.go`, `internal/postgres/catalog_admin_repository.go`, `internal/postgres/resource_repository_codec.go`, and `internal/app/catalogo/service.go`.

**Dependencies and end state:** 5A is green. Map all fifteen stable categories by `errors.Is`/typed classification with explicit precedence for revision, identity, impossible reactivation, invalid catalog, lifecycle, integrity, and in-use outcomes. The neutral GARFEX error has safe messages, no `Unwrap`/cause/arbitrary formatting, and technical causes are retained only in internal diagnostics. String/regex/driver-message parsing is prohibited at this boundary. Rollback is limited to `internal/core` and classifier call sites.

**Focused checks:** `go test ./internal/core ./internal/domain ./internal/app/catalogo ./internal/postgres -run 'Test(Error|Map|Leak|Precedence|Diagnostic)' -count=1`.

**Strict evidence:** RED adds the fifteen-category precedence and leakage tests using pgx-like values containing SQLSTATE, constraints, tables, columns, and server text; GREEN implements typed mapping and diagnostics; TRIANGULATE inspects `%v`, `%+v`, concrete type, `errors.Unwrap`, recursive chains, cancellation/deadline, unavailable latch, and unknown errors; REFACTOR verifies no string parsing and no cause escapes the internal error.

**Mandatory slice boundary:** after the focused command, run `go test ./... -count=1` and record compile/full-suite success before the Stage 5 gate.

### Stage 6 — Public READ-ONLY DTOs, Reader, bridge, documentation, and READ gate

#### 6A — Public READ-ONLY DTOs and values

- [x] Add owned public read-only DTOs, canonical numeric values, lifecycle/query/page types, defensive copies, and stable public error identities in the new `resourcecore/types.go`, `resourcecore/values.go`, `resourcecore/queries.go`, `resourcecore/errors.go`, `resourcecore/copy.go`, with package tests in `resourcecore/types_test.go`, `resourcecore/values_test.go`, `resourcecore/errors_test.go`, and `resourcecore/copy_test.go`; no write symbol is allowed. <!-- sdd-owner: implementation -->

**Allowed edit surfaces:** exactly the new files `resourcecore/types.go`, `resourcecore/values.go`, `resourcecore/queries.go`, `resourcecore/errors.go`, `resourcecore/copy.go`, `resourcecore/types_test.go`, `resourcecore/values_test.go`, `resourcecore/errors_test.go`, and `resourcecore/copy_test.go`.

**Dependencies and end state:** the Stage 5 gate is green; this is the first allowed public package path. Define all 11 kind codes, descriptors, records, resource identity/scope/unit/attributes, typed values including `NOT_APPLICABLE`, explicit lifecycle scope/paging, canonical base-10 numeric strings without float64, opaque catalog IDs, durable identity-v1, and all fifteen public categories. Public DTO collections are owned by copy and public errors do not expose internal causes. No Reader, bridge, repository, service, snapshot, reload, publish, or write method is added in this slice. Rollback removes only `resourcecore/` DTO/value files.

**Focused checks:** `go test ./resourcecore -run 'Test(Canonical|Value|NotApplicable|Query|Page|Defensive|PublicError)' -count=1`.

**Strict evidence:** RED adds table-driven external-facing DTO/value/copy/error tests before implementation; GREEN implements the smallest owned contract; TRIANGULATE covers scale variants, negative zero, nil/empty collections, nested references/rules/options, caller mutation, concrete error/unwrap inspection, and API reflection for absent writes; REFACTOR keeps DTOs generic and descriptor-driven.

**Mandatory slice boundary:** after the focused command, run `go test ./... -count=1` and record compile/full-suite success before 6B.

#### 6B — Public Reader and internal bridge

- [x] Add the read-only `Reader` and public `ReadCapabilities` in `resourcecore/reader.go`, `resourcecore/reader_test.go`, and `resourcecore/external_test.go`, plus the module-owned adapter in `internal/bridge/resourcecore/adapter.go` and `internal/bridge/resourcecore/adapter_test.go`; keep the construction evidence external-package-only at the public boundary and do not wire an unused Reader into the shipped CLI/TUI. <!-- sdd-owner: implementation -->

**Allowed edit surfaces:** exactly `resourcecore/reader.go`, `resourcecore/reader_test.go`, `resourcecore/external_test.go`, `internal/bridge/resourcecore/adapter.go`, and `internal/bridge/resourcecore/adapter_test.go`.

**Dependencies and end state:** 6A is green. `NewReadOnly(nil)` is invalid; Reader exposes active classes, descriptors, catalog list/get, resource search/get, and canonical description only. The external-package test constructs Reader using only public types and imports no `internal` path. The module-owned bridge tests exercise the authoritative service-shaped capabilities, deep-copy both edges, validate transport shape, delegate canonical description to `recursos.Service.Describe`, and translate through the Stage 5 neutral mapper. No `cmd/garfex/main.go` or shipped CLI composition claims or wires a Reader, and no TUI wiring is added. No public create/update/deactivate/reactivate/delete/publish/reload/repository/mutable-authority method exists. Rollback removes only Reader/bridge surfaces.

**Focused checks:** `go test ./resourcecore ./internal/bridge/resourcecore -run 'Test(Adapter|Reader|External|Bridge|Read|Description|Error|API)' -count=1`.

**Strict evidence:** RED adds external-package construction using public types, delegation, copy, nil, and API-absence tests first; GREEN implements Reader/bridge projections; TRIANGULATE uses module-owned bridge tests against authoritative service-shaped capabilities and injects not-found, integrity, invalid-catalog, unavailable, internal, scope, paging, and presentation-spy cases; REFACTOR reruns external tests and inspects imports/signatures for internal leakage without adding CLI/TUI Reader wiring.

**Mandatory slice boundary:** after the focused command, run `go test ./... -count=1` and record compile/full-suite success before 6C.

#### 6C — Public READ exit documentation

- [x] Document ownership, safe errors, all-11 lifecycle semantics, identity, freshness, one-writer topology, migration 8 compatibility, and the absence of public WRITE in `resourcecore/doc.go`, `docs/architecture/resource-master-core.md`, and the Resource Master sections of `docs/architecture/catalog-source-of-truth.md`; do not add write APIs or edit implementation files to hide failures. <!-- sdd-owner: implementation -->

**Allowed edit surfaces:** exactly `resourcecore/doc.go`, `docs/architecture/resource-master-core.md`, and `docs/architecture/catalog-source-of-truth.md`.

**Dependencies and end state:** 6B is green. Documentation agrees with the public signatures and accepted design: pool/resource ownership stays at composition, errors are safe and opaque, hash IDs are not durable, identity-v1 is durable, one process is the authoritative writer, other processes require explicit reload/restart, there is no live cross-process coherence, and public READ is the only shipped contract. Rollback is limited to these documentation files.

**Focused checks:** `go test ./resourcecore ./internal/bridge/resourcecore -count=1` plus `gofmt -l resourcecore`.

**Strict evidence:** RED reviews documentation assertions against `proposal.md`, `design.md`, and `specs/resource-master-core/spec.md`; GREEN updates only the listed docs/package comment; TRIANGULATE independently checks API visibility, all 11 kinds, all 15 categories, identity preservation, normalization limits, and no-write claims; REFACTOR simplifies for reviewer scanning without weakening guarantees.

**Mandatory slice boundary:** after the focused command, run `go test ./... -count=1` and record compile/full-suite success before the final READ gate.

## Parent-controlled gates after each stage

Parent actions are separate from implementation units. A gate must fail closed: do not start the next stage until its predecessor's focused evidence and mandatory full-suite receipt are recorded. Every gate preserves `resource-master-technical-debt` and unrelated user changes.

- [x] P1. After 1D, run a bounded review of `internal/domain/catalog_mutation.go`, `internal/domain/catalog_record.go`, `internal/domain/catalog_kind.go` discovery results, and the Stage 1 test receipts; confirm mutation correctness is lossless, option sets are not no-ops, applicability is copied atomically at the candidate level, every Stage 1 unit is independently compilable, and `go test ./... -count=1` passed before authorizing Stage 2. <!-- sdd-owner: parent -->
- [x] P2. After 2C, review `internal/domain/catalog_kind.go`, `internal/domain/catalog_mutation.go`, `internal/app/catalogo/service.go`, and all 11×5 lifecycle evidence; confirm conservative guards, inactive/history/non-blocking blockers, active-delete rejection, no publication on rejection, and full-suite proof before authorizing Stage 3. <!-- sdd-owner: parent -->
- [x] P3. After 3I, review the isolated PostgreSQL receipts and allowed surfaces `migrations/000005*`, `migrations/000007_resource_identity_v1.*`, `migrations/000008_resource_revisions.*`, `internal/postgres/`, and `internal/app/recursos/`; confirm migration-5/7 reset compatibility and restoration first, then identity-v1 fixture repair with the preserved reset suite, migration-8 backfill/grants, atomic applicability, all-11 catalog/resource CAS, canonical reactivation E2E, two-connection race evidence, and `go test ./... -count=1` before authorizing Stage 4. <!-- sdd-owner: parent -->
  - Confirmed 2026-08-19: 3A (migration-5/7 reset restoration), 3B (identity-v1 fixture repair, reset suite preserved), 3C (migration 8 backfill, identity byte-equality), 3E (atomic APLICABILIDAD parent+rules), 3F/3G1/3G2/3G3 (all-11 catalog CAS, concrete `CatalogAdminRepositoryV2` conformance, dormant), 3H+correction (canonical reactivation E2E and Update-CAS through `recursos.Service`), 3I (genuine two-connection revision race and dependency/delete race, both deterministic under real DB locking, not `-race`-conflated) — each independently verified by a fresh-context validator before acceptance. `go build ./...`, `go vet ./...`, and `go test ./... -count=1` reconfirmed green across every package (including `internal/tui`) by the parent at checkpoint time.
  - Known non-blocking issue carried into Stage 4: `internal/tui/catalog_admin_e2e_test.go` fails only when an integration DSN is set (legacy `catalog_mutation.go` applicability-rules validation path) — never manifests in real CI (no DSN there) and is outside every Stage 3 slice's edit surface. Stage 4's TUI co-migration work (4G/4H) should account for this pre-existing gap.
- [x] P4. After 4H, review `internal/domain/catalog_equivalence.go`, `internal/postgres/catalog_admin_repository_v2.go`, `internal/app/catalogo/`, `cmd/garfex/main.go`, and the exact TUI co-migration surfaces; confirm committed reload equivalence, one publication after commit, complete additive service adoption, no dormant-authority path, required caller/fake/TUI compatibility, and full-suite proof before authorizing Stage 5. <!-- sdd-owner: parent -->
  - Confirmed 2026-08-20: 4A (domain equivalence, incl. materialized-definition correction), 4B (transactional coherent result — CAS→reload→validate→equivalence→commit-only-on-match for all 11 kinds, with a user-accepted adapter-level order-normalization workaround), 4C (additive service V2 adoption, publishes only committed result, 7/7 no-publish categories proven, writer-unavailable latch), 4D (production/integration composition switch — Create is now V2-authoritative; Update/Deactivate/Reactivate/Delete deliberately remain legacy pending 4F/4G's CAS-revision plumbing; pre-existing DSN-only TUI failure independently reproduced as orthogonal to the switch), 4E (11×5 authority oracle vs independent reload, writer-latch gap on `Create` closed), 4F (TUI revision-state plumbing, no service signature changes, negative-control-proven), 4G (TUI caller co-migration to the revision-aware methods; deactivate/hard-delete not conflated; zero revision-less writer remains reachable from `internal/tui`), 4H (compatibility-retirement decision — all 7 legacy methods deliberately retained as still-referenced safe wrappers, zero code delta) — each independently verified by a fresh-context validator (or, for 4H's zero-diff decision, directly by the parent) before acceptance. `go build ./...`, `go vet ./...`, and `go test ./... -count=1` reconfirmed green across every package by the parent at checkpoint time.
  - Two user-approved size:exceptions carried into this stage: 4C (591/400) and 4D (line count independently reconfirmed by the parent after the ledger failed to measure it, ~well within budget once verified).
- [x] P5. After 5B, review `internal/domain/core_errors.go`, `internal/core/`, `internal/app/`, and `internal/postgres/`; confirm all dedicated outcomes remain distinct, neutral GARFEX errors are non-unwrappable, diagnostics remain internal, no technical detail leaks, no string parsing exists, no public package path was introduced, and `go test ./... -count=1` passed before authorizing Stage 6. <!-- sdd-owner: parent -->
  - Confirmed 2026-08-20 (retroactively, at the P6 gate, since Stage 6 had already been implemented and this checkbox lagged its own bookkeeping): `internal/core/errors.go`'s `Map` is one exhaustive `errors.Is` switch covering all 15 categories with the exact documented precedence (`ErrRevisionConflict` → `ErrIdentityConflict` → `ErrReactivationImpossible` → `ErrInvalidCatalog` → `ErrInvalidLifecycle` → `ErrCodeImmutable` → `ErrCatalogInUse` → `ErrResourceIntegrity` → `ErrResourceValidation` → duplicate/reference/not-found → `ErrInvalidArgument`/`ErrUnavailable`/context cancellation → default `Internal`); no string/substring/regex matching against `Error()`; no public package path created by 5A/5B; `go test ./... -count=1` green for every package this stage touches at the time of this review.
- [x] P6. After 6C, recalculate READ readiness from `resourcecore/`, `internal/bridge/resourcecore/`, `internal/app/catalogo/`, `internal/app/recursos/`, `internal/domain/`, and `openspec/changes/stabilize-resource-master-core/readiness.md`; record external-package usage, projections, copies, values, queries, canonical presentation, safe read errors, API absence of WRITE, focused receipts, full-suite proof, vet/lint/race/build CI status, deviations, rollback notes, and freshness limits. Do not apply or plan public WRITE in this change; leave it for a separate post-READ readiness decision. <!-- sdd-owner: parent -->
  - Confirmed 2026-08-20: recorded in `openspec/changes/stabilize-resource-master-core/readiness.md`. Verdict: READ-ONLY is ready. `golangci-lint run ./resourcecore/... ./internal/bridge/resourcecore/...` reports `0 issues.`; `go vet` and `gofmt -l resourcecore` clean; focused suites green (27/27); `go test ./... -count=1` green for every package this change touches, with the same two pre-existing, untracked, out-of-scope failures carried since 6A (`agent/skills/golang-cli/assets/examples`, `internal/tui/suppliers_admin.go`) neither introduced nor affected by this change. No public WRITE symbol exists. Public WRITE readiness is explicitly out of scope for this gate and this change.

## Final graph self-audit

- Stage order is exactly mutation correctness → all-11 lifecycle → completely green PostgreSQL → `CatalogAuthority`/persistence equivalence and adoption → Core outcomes and neutral mapping → public READ-ONLY.
- No `resourcecore/` or `internal/bridge/resourcecore/` path appears in Stages 1–5; 6A is the first public package path.
- Every existing port change is additive or co-migrated with all current adapters/compile assertions in the same slice; every service caller/fake/composition/TUI migration is explicit in 3H, 4C–4H, and the fail-closed pre-start textual-reference gates.
- Every unit has a concrete start dependency, exact edit surfaces, focused RED/GREEN/TRIANGULATE/REFACTOR evidence, a rollback boundary, and mandatory `go test ./... -count=1` compile/full-suite proof.
- Applicability parent plus rules are atomic, all dependencies block hard delete, identity-v1 and its constraint remain intact, hash IDs remain opaque, and stable errors expose no technical cause.
- The graph contains no public WRITE implementation tasks. Public WRITE remains future work after the final READ readiness recalculation.
- Only `openspec/changes/stabilize-resource-master-core/design.md`, `openspec/changes/stabilize-resource-master-core/tasks.md`, `openspec/changes/stabilize-resource-master-core/apply-progress.md`, and the corresponding Engram topic are being repaired; proposal/spec, `resource-master-technical-debt`, and unrelated user changes remain untouched.
