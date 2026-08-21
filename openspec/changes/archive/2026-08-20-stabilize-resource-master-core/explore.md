# Exploration: Stabilize the Resource Master Core

## Outcome

The current application and PostgreSQL layers are suitable authorities to wrap, but they are not yet a stable external Go API. The safest initiative is an ordered auto-chain: publish a consumer-neutral read-only façade first, then complete and characterize the existing catalog write/lifecycle path before exposing writes. No HTTP, MCP, UI, repository bypass, or duplicate business rules are needed.

The largest verified WRITE gap is not PostgreSQL CRUD. The repository already implements all five lifecycle operations for all 11 registered kinds. The gap is the in-memory mutation/authority path: six kinds cannot deactivate/reactivate, one kind is a complete snapshot no-op, and several record builders discard `Active` or nested rules. Exposing the current service unchanged would publish a stale or invalid `CatalogAuthority` even when PostgreSQL committed successfully.

## Independently verified baseline

The prior audit was used only to locate likely seams. Current source confirms:

- `internal/app/recursos.Service` remains the authoritative resource instance boundary for get, paged search, canonical description, create, update, deactivate, and guarded reactivate.
- `internal/app/catalogo.Service` remains the authoritative catalog boundary. Its mutex serializes mutations only within one service instance; validation uses a private snapshot and publication uses the shared `domain.CatalogAuthority`.
- `internal/postgres` implements the domain ports, transactions, static kind dispatch, catalog loading, and SQL error classification. The TUI is not required by either service.
- All reusable packages remain under `internal/`, so an external Go module cannot import any current service, command, repository port, or projection directly.
- `LoadResourceCatalog` reads a coherent PostgreSQL snapshot in a read-only `REPEATABLE READ` transaction. `CatalogAuthority` defensively clones and versions only an in-process snapshot.
- PostgreSQL has `active`, `created_at`, and `updated_at` on all 11 catalog tables. `resource_attribute_rules` also has those columns but is nested under the administrable `APLICABILIDAD` kind rather than registered as a twelfth kind.

## Exact lifecycle and mutation gap

| Kind | Repository create/update/lifecycle/delete | Snapshot mutation today | Required stabilization |
| --- | --- | --- | --- |
| `CLASE` | Implemented | Full lifecycle works | Characterize full equivalence and guards |
| `FAMILIA` | Implemented | Full lifecycle works | Extend existing equivalence tests |
| `TIPO` | Implemented | Full lifecycle works | Extend existing equivalence tests |
| `CARACTERISTICA` | Implemented | `Active` is discarded; lifecycle setter is nil | Preserve `Active`; enable lifecycle |
| `CONJUNTO_OPCIONES` | Implemented | Every operation returns the unchanged snapshot | Mutate `ResourceCatalog.OptionSets` fully |
| `OPCION` | Implemented | Full lifecycle works | Characterize synthetic-ID and rename behavior |
| `RELACION_OPCIONES` | Implemented | `Active` is discarded; lifecycle setter is nil | Preserve `Active`; enable lifecycle |
| `UNIDAD` | Implemented | Full lifecycle works | Characterize full equivalence and guards |
| `POLITICA_UNIDAD` | Implemented | `Active` is discarded; lifecycle setter is nil | Preserve `Active`; enable lifecycle |
| `APLICABILIDAD` | Implemented | `Active` and nested `Rules` are discarded; lifecycle setter is nil | Preserve active state and rules; define rule-write scope |
| `PRESENTACION` | Implemented | `Active` is discarded; lifecycle setter is nil | Preserve `Active`; enable lifecycle |

The contradictory comments in `catalog_kind.go`, `catalog_mutation.go`, and tests predate the current domain structs: those structs now contain `Active`, while `SoftDelete` metadata and mutation setters still describe only five supported kinds. These comments and registry flags are part of the stabilization surface.

`APLICABILIDAD` needs an explicit design decision. Its public record currently exposes mode and identity participation but not `[]AttributeRule`; creating a conditional binding cannot produce a valid snapshot, and updating an existing conditional binding rebuilds it without rules. The proposal must either include rule projection/mutation atomically within this kind or explicitly prevent unsupported conditional writes. Full lifecycle does not require a twelfth kind, but full create/update for the approved 11 kinds requires a lossless nested-rule contract.

## Public Go package boundary

A new top-level package (working name `resourcecore`) can be imported outside the module's `internal` tree and delegate every operation to the existing services. It should expose owned, immutable-by-copy DTOs and stable façade errors, not aliases of internal domain types.

### READ-ONLY first

The minimal useful read surface is:

- catalog kind descriptors and list/get projections for all 11 kinds, including stable kind codes, active state, public field values, references, and pagination/filter inputs;
- resource paged search and get, with explicit active/inactive scope;
- canonical description returned by `recursos.Service.Describe` rather than recomputed presentation;
- resource projections containing stable ID, opaque identity key, scope, natural unit, active state, and lossless typed attribute values.

Projection decisions still required are whether decimal/quantity values use `shopspring/decimal` or canonical strings, and whether catalog records expose generic descriptor-driven values or eleven typed read models. Generic DTOs match the existing registry without duplicating business rules; typed scalar unions still need strict validation and defensive slice/map copying.

The package must have a usable construction story. An exported constructor cannot require callers to name `internal` service or port types. Viable options are: (1) a consumer-neutral PostgreSQL-backed factory accepting a pool and internally loading/wiring the existing services, or (2) a public façade plus an exported factory from this module while keeping its constructor unexported. Re-declaring public repositories for consumers to implement would duplicate ports and encourage bypass, so it is not preferred. Ownership and closing of any injected pool must be explicit.

### WRITE second

After lifecycle parity is green, add façade commands that translate to `recursos.Service` and `catalogo.Service`; do not export repositories, raw snapshots, `Publish`, or compatibility `recursos.Service.Delete` (which only deactivates). Catalog hard delete must be named distinctly and guarded in the service boundary rather than depending on a consumer preflight.

## Guarded hard delete and write integrity

`catalogo.Service.Delete` currently validates removal against its snapshot and relies on PostgreSQL foreign keys for the final in-use backstop. `Dependencies` and `ReferencedByResources` are separate pass-through calls mainly orchestrated by the TUI. A public hard-delete contract should make the guard authoritative and race-safe:

1. validate ID/kind and load the current row;
2. evaluate catalog dependents and resource references according to a documented policy;
3. apply and validate the candidate snapshot;
4. delete transactionally, retaining FK mapping as the race backstop;
5. publish only after commit.

The design must define whether inactive dependents block hard delete (current probes count all rows), whether non-`Blocking` dependencies are informational or blocking for delete, and whether deleting an inactive record is mandatory. These are product rules, not façade concerns.

## Error boundary and leakage

Current errors are appropriate internal diagnostics but not a stable public contract:

- services sometimes return domain sentinels directly and sometimes wrap arbitrary repository errors;
- catalog SQL mapping includes constraint names or PostgreSQL messages;
- validation joins detailed messages containing kind codes and supplied values;
- unsupported repository capabilities are ad-hoc `errors.New` values.

The public package should map known outcomes to stable public codes/sentinels such as invalid argument, not found, duplicate, invalid reference, validation, in use, immutable code, conflict, and unavailable/internal. Unexpected infrastructure details should not appear in public messages. The proposal/design must decide whether an operation error retains an unwrap-only diagnostic cause; doing so is convenient in-process but allows callers to inspect PostgreSQL details. Error-mapping tests should prove both `errors.Is` behavior and absence of SQL/constraint/message leakage from the public string.

## Concurrency and process guarantees

The current guarantees are narrower than a reusable library may imply:

- one `catalogo.Service` instance serializes its own writes with `sync.Mutex`;
- two service instances, even if they share one `CatalogAuthority`, keep independent snapshots and mutexes and can validate/publish stale state;
- multiple processes never receive another process's publication; their catalogs remain boot-time snapshots;
- repository updates are blind writes and no DTO carries a revision;
- resource updates are also blind by ID, although uniqueness/FKs prevent some invalid outcomes.

Therefore the initiative must document the supported baseline as **one catalog writer service instance in one process**. Multiple read-only processes are snapshot-consistent at load time but are not live-coherent after writes; restart/reload is required. This is not a multi-process freshness guarantee.

Optimistic concurrency is a proposal gate for the WRITE phase. It should be included now if the public writer is expected to support concurrent editors or multiple service instances; otherwise the public write API must explicitly remain single-writer and reject claims of safe multi-process writing. Deferring concurrency after publishing revision-less update commands creates a compatibility burden.

Existing data can ground an initial token: every catalog row and `recursos` row has `updated_at`, maintained by migration triggers/manual updates. The repositories currently do not select it. An opaque token based on exact `updated_at` plus stable row identity can support conditional `UPDATE/SetActive/Delete ... WHERE updated_at = expected`, but timestamp equality and hash-derived catalog IDs need careful characterization. PostgreSQL `xmin` is not a durable public token. If strict monotonic semantics are required, an additive integer `revision` column is safer; existing `updated_at` can initialize/migrate the token but is not equivalent to a monotonic revision. The proposal/design must select one guarantee.

Four catalog kinds use `hashtextextended` natural-key hashes as synthetic `int64` IDs. Those IDs have collision risk and change when a mutable natural code changes; after a successful code rename, the returned record still carries the old ID. Public identity and concurrency tokens must not silently promise these hashes are durable object identities.

## Test seams and evidence plan

### Public boundary tests

- Add an external-package compile test (`package resourcecore_test`) proving an outside consumer can construct and use the read façade without importing any `internal` package.
- Use table-driven projection tests for every resource attribute value type, `NOT_APPLICABLE`, defensive copies, lifecycle scope, paging metadata, and canonical description delegation.
- Use table-driven public error mapping tests for each known sentinel plus an opaque unexpected error; assert no PostgreSQL constraint/message is exposed.
- Add façade delegation spies at the smallest internal seam; assert no repository or rule implementation appears in the public package.

### Catalog lifecycle and authority tests

- Replace five-kind assumptions with a table covering all 11 kinds and all five operations: create, update, deactivate, reactivate, guarded hard delete.
- At the domain seam, verify `ApplyCatalogMutation` is lossless and copy-on-write for every slice, including nested rules, option sets, active flags, and caller-owned maps/lists.
- At the application seam, test no persistence/publication on validation, guard, conflict, or repository failure; publication occurs exactly once after success.
- Expand `session_coherence_test.go` from `FAMILIA` to every kind and delete. The strongest integration oracle is: mutate through `catalogo.Service`, then independently call `LoadResourceCatalog` and compare the authority snapshot with the freshly loaded catalog after normalization. This is the required `CatalogAuthority` equivalence test, not only comparison with a hand-built expected value.
- Add concurrent writer tests for the selected policy: stale revision returns conflict with no publication, or a documented second-writer rejection in single-writer mode.

### PostgreSQL and migration fixtures

Existing migration seeds provide three classes, two families/types/units, definitions/options/relations/presentation, and nested rules. Existing admin integration tests cover representative shapes, not all 11 end-to-end operations. Add TEST-prefixed, reverse-cleaned fixtures for the six currently partial kinds and conditional applicability/rules. Continue skipping DB tests when the required DSN is absent and keep app-role grant evidence separate from admin-role migration manipulation.

If no schema change is selected, characterize exact `updated_at` reads and compare-and-swap behavior for serial-ID and composite/hash-ID tables. If a revision migration is selected, test backfill, trigger/update increment, stale write/delete/lifecycle conflicts, grants, and rollback. Do not use blanket cleanup or mutate seeded business rows without restoration.

## Change surfaces

Expected production surfaces, in order:

1. New top-level public package and external-consumer tests for READ-ONLY projections, construction, and stable errors.
2. `internal/domain/catalog_mutation.go`, `catalog_kind.go`, and related domain tests to make all 11 mutations lossless.
3. `internal/app/catalogo/service.go` and tests for authoritative guards, publication parity, and selected concurrency policy.
4. `internal/postgres/catalog_admin_repository.go`, `catalog_admin_kinds.go`, loader/projection queries, and integration tests for revision/guard behavior if selected.
5. An additive migration pair only if a monotonic revision is selected; migration version must follow the current unique sequence.
6. Core package documentation stating ownership, error, lifecycle, and single-writer/multi-process guarantees.

`internal/app/recursos.Service`, repository rules, identity v1, lifecycle semantics, loaders, and migrations remain the implementation authorities. Existing TUI code and the unrelated `openspec/changes/resource-master-technical-debt` change are out of scope.

## Proposal/design decisions to resolve

1. **Construction:** public PostgreSQL-backed factory versus another usable factory that does not expose internal types or duplicate ports.
2. **Read DTOs:** canonical strings versus decimal dependency, and generic descriptor-driven catalog records versus eleven typed projections.
3. **Applicability rules:** atomic nested rule create/update contract within `APLICABILIDAD`.
4. **Delete policy:** inactive prerequisite, inactive-dependent treatment, and meaning of non-blocking dependencies.
5. **Concurrency:** include optimistic concurrency in WRITE now, or formally constrain the API to one writer instance; select `updated_at` CAS versus a monotonic revision migration.
6. **Public identity:** avoid promising hash-derived IDs are collision-free or stable across code rename.
7. **Diagnostics:** stable safe errors with or without an inspectable underlying cause.
8. **Refresh:** this initiative may document restart/reload for other processes, but a listener/poller is a separate capability unless explicitly brought into scope.

## Delivery shape and budget

The full initiative will exceed the 400 changed-line review budget and should be auto-chained. Recommended review units are: (1) READ-ONLY public contracts/projections/errors, (2) public construction and read integration, (3) all-11 lossless domain lifecycle, (4) service guards and authority equivalence, (5) concurrency/revision migration and repository CAS if approved, and (6) WRITE façade plus guarantee documentation. Each unit should follow strict red-green-refactor with focused tests before the full suite.

No production code or unrelated change was modified during this exploration. Tests were not executed because the available exploration tooling provided source inspection but no command runner; current suite status and live PostgreSQL behavior remain to be verified during proposal/apply verification.
