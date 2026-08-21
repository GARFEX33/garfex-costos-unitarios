# Stabilize the Resource Master Core for consumer-neutral use

## Intent

Provide a stable, importable Go contract for the GARFEX Resource Master Core while preserving the existing application services as the only business-operation authorities. The first usable workflow is read-only: consumers can traverse active catalog classes and structure, list/search/filter/page resources, obtain resource details, and render canonical presentation without depending on terminal, Web, mobile, agent, or infrastructure technology.

The change then stabilizes the existing catalog mutation path so all 11 registered catalog kinds have complete, lossless lifecycle behavior before WRITE is exposed. Persistence and the published in-process catalog snapshot must remain equivalent after every successful mutation, and neither may change after a rejected operation.

## User outcome

A consumer can use one neutral public package to:

1. discover active catalog classes and catalog structure;
2. list and search resources with explicit filters and pagination;
3. inspect complete resource details; and
4. obtain the canonical GARFEX presentation produced by the authoritative resource service.

After the later WRITE slices are completed, authorized application integrations can create, update, deactivate, reactivate, and conservatively hard-delete every supported catalog kind with stable errors, atomic validation, and optimistic concurrency protection.

## Binding decisions

| Area | Decision |
| --- | --- |
| Lifecycle | Support create, update, deactivate, reactivate, and guarded hard delete for all 11 registered catalog kinds. Hard delete requires the record to be inactive and dependency-free. |
| `APLICABILIDAD` | The record and all nested attribute rules are one atomic functional mutation. Incomplete, invalid, lost, catalog-invalid, or snapshot-invalid rules reject the entire create/update. Successful persistence and publication contain complete equivalent rules. |
| Delete policy | Every existing relationship blocks physical deletion by default, including active, inactive, historical, and relationships currently marked non-blocking. Inactive means retained history, not absence. Prefer deactivation whenever history or dependency exists. Any future exception requires an explicit per-relation rule. |
| Delivery order | Deliver READ-ONLY before WRITE. Evaluate and expose WRITE separately per operation only after lifecycle parity and authority equivalence are proven. |
| Public boundary | Add a consumer-neutral public Go contract backed by an internal bridge to the authoritative application services. Do not expose internal services, repositories, domain aliases, snapshots, or publication controls. |
| DTOs | Use owned, defensively copied, generic descriptor-driven DTOs. Represent numeric values as canonical strings and retain lossless typed value semantics. |
| Presentation | Delegate canonical resource presentation to `internal/app/recursos.Service`; consumers must not reconstruct it. |
| Errors | Expose only stable GARFEX error categories. Technical causes remain internal to diagnostics, logging, observability, and debugging and cannot be publicly unwrapped or inspected. Public strings and error chains must not leak pgx types, `PgError`, SQLSTATE, constraints, tables, columns, PostgreSQL messages, or infrastructure wrapping. |
| Concurrency | WRITE uses a persisted, monotonic revision and compare-and-swap semantics. Stale mutation, lifecycle, and delete requests return a stable conflict without persistence or publication. |
| Writer topology | This version supports one authoritative writer process. It does not implement cross-process refresh or claim live coherence for other processes. |
| Identity | Resource `identity-v1` is canonical and durable. Hash-derived catalog IDs are opaque snapshot/implementation references; they may change after natural-code changes and are not durable business identity. |
| Quality | Use strict test-driven development and preserve red-green-refactor evidence for every slice where tests exist. |

## Scope

### 1. Consumer-neutral READ-ONLY contract

- Add a top-level importable Go package with public, package-owned DTOs and stable error categories.
- Provide a usable module-owned construction/bridge path without requiring consumers to name or implement `internal` types or repository ports.
- Project descriptors and list/get records for all 11 catalog kinds, including kind code, active state, public values, references, and paging/filter inputs.
- Support resource paged search and get with explicit active/inactive scope.
- Return stable resource ID, durable `identity-v1`, scope, natural unit, active state, lossless typed attributes, and canonical numeric strings.
- Return canonical presentation by delegating to `internal/app/recursos.Service.Describe`.
- Keep all DTO collections immutable-by-copy at the boundary.

### 2. Complete catalog lifecycle and snapshot mutation

- Correct mutation metadata, comments, builders, and setters so create, update, deactivate, reactivate, and guarded hard delete work for every registered kind:
  `CLASE`, `FAMILIA`, `TIPO`, `CARACTERISTICA`, `CONJUNTO_OPCIONES`, `OPCION`, `RELACION_OPCIONES`, `UNIDAD`, `POLITICA_UNIDAD`, `APLICABILIDAD`, and `PRESENTACION`.
- Preserve active state, option sets, references, maps/slices, and nested rules without aliasing or silent field loss.
- Treat `APLICABILIDAD` and its nested rules as one atomic aggregate for validation, persistence, snapshot mutation, and publication.
- Validate a private candidate snapshot before persistence/publication as required by the authoritative service flow.
- After commit, publish exactly once; the resulting `CatalogAuthority` snapshot must be equivalent to an independent coherent reload from PostgreSQL.

### 3. Authoritative guards and stable WRITE semantics

- Keep `internal/app/catalogo.Service` as the catalog administration authority and `internal/app/recursos.Service` as the resource authority.
- Enforce the inactive prerequisite and conservative dependency checks inside the application boundary for hard delete; consumer preflight is not authoritative.
- Count all known dependency/history relationships as delete blockers regardless of active or current `Blocking` metadata, with database constraints retained as race backstops.
- Add persisted monotonic revisions and conditional compare-and-swap behavior for WRITE operations before publishing their public commands.
- Return conflict for a stale expected revision without changing persistence or the authority snapshot.
- Translate internal outcomes into safe public GARFEX errors such as invalid argument, not found, duplicate, invalid reference, validation, in use, immutable code, conflict, unavailable, and internal.
- Retain technical diagnostic context only behind non-public seams.

### 4. Evidence and documentation

- Add external-package compile/usage tests proving consumers need no `internal` imports.
- Cover all DTO value types, `NOT_APPLICABLE`, defensive copying, lifecycle scopes, filtering, pagination, and canonical presentation delegation.
- Cover safe error identity and messages, including assertions that public values and unwrap chains expose no PostgreSQL or infrastructure detail.
- Cover all 11 kinds across all five lifecycle operations at domain and application boundaries.
- Compare each successful service mutation's published authority with a fresh coherent database load after normalization.
- Cover rule-aggregate rejection, repository failures, stale revisions, dependency blockers, and no-persistence/no-publication guarantees.
- Document pool/resource ownership, error semantics, identity semantics, writer topology, and process freshness limits.

## Scope boundaries and non-goals

- No HTTP, MCP, UI/TUI behavior, new service, new repository abstraction, or duplicate domain/business-rule implementation.
- No direct consumer access to repositories, PostgreSQL adapters, mutable domain snapshots, `Publish`, or compatibility methods whose names obscure deactivation versus hard deletion.
- No consumer-specific commands, rendering, transport schemas, or assumptions about terminal, Web, mobile, external processes, agents, or interface technology.
- No cross-process notification, listener, polling, or automatic refresh implementation. Other processes require an explicit reload/restart to observe writes.
- No multi-writer or live multi-process coherence guarantee in this version.
- No promise that hash-derived catalog IDs survive natural-code changes or provide durable business identity.
- No twelfth independently administrable kind for nested applicability rules.
- No change to resource `identity-v1`, authoritative canonical presentation rules, or the established application-service dependency direction.
- No WRITE exposure merely because its internal operation exists; each public operation follows proven lifecycle parity, atomicity, revision, error, and authority-equivalence behavior.
- The unrelated `resource-master-technical-debt` change remains untouched.

## Affected areas

| Area | Expected impact |
| --- | --- |
| New public package | Consumer-neutral façade, DTOs, filters/paging, safe errors, construction bridge, and external-consumer tests. |
| `internal/domain` | Lossless catalog mutation for all kinds, nested-rule atomicity, lifecycle registry corrections, copying, and invariants. |
| `internal/app/recursos` | Read façade delegation and projection seams; existing service remains authoritative. |
| `internal/app/catalogo` | Atomic guards, complete lifecycle orchestration, revision conflict handling, and publish-after-commit equivalence. |
| `internal/postgres` | Complete projections/mutations, conservative dependency checks, monotonic revision persistence/CAS, safe internal error classification, and integration tests. |
| Migrations | Additive schema support for persisted monotonic revisions, backfill, update behavior, grants, and reversible migration mechanics. |
| Documentation | Public ownership, errors, lifecycle, identities, construction, concurrency, and process-freshness guarantees. |

## Architecture impact

The dependency direction remains application services -> domain -> ports -> PostgreSQL adapters. The public package is a translation boundary, not a new authority: it owns safe public types and delegates through an internal bridge to the existing application services. It neither exports nor re-declares repositories.

The in-process `CatalogAuthority` remains a published read snapshot. A successful catalog mutation commits complete state and then publishes one equivalent snapshot. Failed validation, guard checks, stale revisions, or repository operations publish nothing. The supported topology is one writer process; read-only processes receive coherent boot/reload snapshots but no automatic post-write refresh.

## Risks and mitigations

| Risk | Mitigation |
| --- | --- |
| Database commits while the in-memory authority loses fields or rules | Lossless copy-on-write tests for every kind plus fresh-loader equivalence after each successful mutation. |
| Partial or destructive `APLICABILIDAD` updates | Treat the record and all nested rules as one aggregate; reject the whole operation before publication when any part is missing or invalid. |
| Hard delete removes retained history | Require inactive state and block on every known active, inactive, historical, blocking, or non-blocking relationship by default. |
| Public API leaks PostgreSQL details | Central safe-error translation, opaque unexpected failures, no public diagnostic cause, and leakage-focused table tests. |
| Concurrent edits overwrite newer data | Persist monotonic revisions and require CAS for WRITE; stale requests fail without persistence/publication. |
| Consumers mistake hash IDs for durable identity | Document and type them as opaque references; keep `identity-v1` as the durable resource identity. |
| Other processes serve stale catalogs after a write | State the one-writer/no-live-refresh guarantee explicitly and require reload/restart outside the writer process. |
| Public boundary duplicates business rules | Keep validation, presentation, lifecycle, and dependency policy in authoritative application/domain layers; façade only validates transport shape and translates. |
| Large change becomes unreviewable | Deliver an ordered auto-chain with each review unit targeted below 400 changed lines. |

## Rollback boundaries

- Each auto-chain unit must remain independently revertible while preserving the previously accepted units.
- READ-ONLY façade slices can remain available if later lifecycle or WRITE slices are rolled back.
- WRITE commands must not be enabled until their revision, atomicity, error, and equivalence requirements are complete; an affected WRITE operation can be withdrawn without bypassing the application services.
- The monotonic revision migration is additive. During code rollback, the column may remain inert and backward-compatible; destructive down migration is permitted only after confirming no deployed writer or stored contract depends on it.
- If publication equivalence fails after persistence, stop/disable the writer path and reload authority from the database; never compensate by publishing a known-invalid partial snapshot.
- Rollback must not weaken hard-delete guards, reinterpret inactive dependencies as absent, expose technical errors, or change `identity-v1`.

## Success criteria

- [ ] An external Go package test constructs and uses the READ-ONLY API without importing any `internal` package.
- [ ] The read workflow supports active classes -> catalog structure -> resource list/search -> filter/page -> detail -> canonical presentation.
- [ ] Public contracts and documentation are independent of terminal, Web, mobile, agent, external-process, and transport technology.
- [ ] Generic descriptor-driven DTOs use canonical numeric strings, preserve typed values, and defensively copy all mutable collections.
- [ ] All 11 catalog kinds pass create, update, deactivate, reactivate, and guarded hard-delete tests.
- [ ] `APLICABILIDAD` and nested rules are persisted and published atomically and losslessly; every invalid or incomplete aggregate is rejected without side effects.
- [ ] Hard delete is allowed only for inactive, dependency-free records, and every existing relationship blocks by default.
- [ ] Every successful catalog mutation publishes exactly once after commit and matches a fresh coherent database load after normalization.
- [ ] Validation, dependency, revision conflict, and repository failures cause neither persistence nor publication.
- [ ] Public errors have stable GARFEX identity and expose no technical cause or PostgreSQL/infrastructure detail through strings, types, or unwrapping.
- [ ] WRITE mutations use persisted monotonic revisions and stale CAS requests return conflict.
- [ ] Documentation clearly states one authoritative writer process and the absence of cross-process refresh/live-coherence guarantees.
- [ ] Resource `identity-v1` remains canonical and durable; hash-derived catalog IDs are documented as opaque and changeable.
- [ ] Focused and full verification passes under the project policy, with CI-only race/build checks reported separately when not run locally.

## Delivery plan

This initiative is expected to exceed 400 changed lines and must use an ordered auto-chain. Each unit targets no more than 400 changed lines and follows strict red-green-refactor:

1. Public READ-ONLY contracts, safe error model, and projection unit tests.
2. Public construction/internal bridge and read integration tests.
3. Lossless all-11 domain lifecycle, including option sets and atomic applicability rules.
4. Application guards, conservative hard delete, and authority/database equivalence.
5. Additive monotonic revision migration, repository CAS, and stale-write integration tests.
6. Per-operation WRITE façade, safe error mapping, guarantee documentation, and final verification.

WRITE remains unavailable until the prerequisite slices for that operation are green. No slice may bypass the authoritative application services to accelerate delivery.
