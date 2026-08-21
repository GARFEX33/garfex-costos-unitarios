# Design: Stabilize the Resource Master Core

## Decision summary

Core stabilization precedes every public contract. Implementation and rollout follow one strict dependency chain: **(1)** catalog mutation correctness, **(2)** lifecycle alignment for all 11 kinds, **(3)** a completely green PostgreSQL layer including migration-5/7 reset compatibility, identity-v1 fixture repair, canonical reactivation E2E, integration, and race evidence, **(4)** `CatalogAuthority` semantic correctness with persistence/publication equivalence, **(5)** neutral GARFEX error outcomes, then **(6)** the public **READ-ONLY** Go contract. Reset restoration is a prerequisite to honest fixture execution on a migration-7 database. READ-ONLY is still the first public contract; no public package or symbol lands before Core stabilization, and public WRITE remains a later operation-gated concern.

The final topology adds a top-level `resourcecore` package as an owned translation boundary. It accepts narrow, service-shaped read capabilities; a module-owned adapter under `internal/bridge/resourcecore` implements them with `internal/app/recursos.Service` and `internal/app/catalogo.Service`. No public signature mentions an internal type, repository, pool, DSN, PostgreSQL, snapshot, or publication control.

Cross-layer contracts migrate additively. Existing ports, service methods, constructors, callers, test doubles, and TUI actions continue to compile until a complete replacement path exists. New coherent write results, applicability aggregate methods, and revision/CAS operations are added beside legacy surfaces. The complete V2 domain contract may exist before its PostgreSQL implementation, but no PostgreSQL V2 constructor or concrete conformance assertion exists until every operation family is implemented in stage 3G; services adopt only that complete path, all production write composition and the exact TUI callers switch, and only then may compatibility surfaces be retired in a later independently green slice. Catalog persistence returns a coherent catalog loaded in the mutation transaction, and the service publishes that committed result exactly once. Resources and each of the 11 catalog parent tables receive an additive `BIGINT revision`; applicability rules share their parent applicability revision.

## Goals and non-goals

### Goals

- Correct lossless catalog mutation before changing persistence or public boundaries.
- Align create, update, deactivate, reactivate, and hard delete for all 11 kinds.
- Make PostgreSQL, migrations, fixtures, integration, canonical reactivation, and races completely green.
- Publish only coherent committed catalog snapshots and prove them against an independent reload.
- Establish neutral safe errors only after authoritative Core outcomes are stable.
- Ship an externally importable, consumer-neutral read contract as the first public contract and expose no writes.
- Preserve resource `identity-v1`, opaque catalog IDs, durable monotonic revisions/CAS, and a single-authoritative-writer topology.

### Non-goals

- No HTTP, MCP, TUI-specific public API, public repository port, public database factory, listener, poller, notification bus, or multi-writer protocol.
- No public write method in the first read-only slice.
- No durable identity promise for hash-derived catalog IDs.
- No separately administrable kind or revision for applicability rules.

## Architecture and dependency direction

The diagram is the **final stage-6 topology**, not the delivery order. Stages 1–5 stabilize the existing internal dependency direction before `resourcecore` exists.

```text
external consumer
    |
    v
resourcecore.Reader (public DTO validation/copy/error translation only)
    |
    v
resourcecore.ReadCapabilities (public, narrow service-shaped seam)
    ^
    |
internal/bridge/resourcecore.Adapter
    |                         |
    v                         v
internal/app/catalogo     internal/app/recursos
    |                         |
    +------> internal/domain <+
                 |
                 v
          internal/postgres
```

The public package does not import PostgreSQL packages and does not own business validation, presentation, lifecycle, dependency, or repository rules. The internal adapter may import both the public package and internal services. Stage 6 proves the production-grade bridge by integrating the real authoritative service interfaces under module-owned tests; it does not wire an otherwise unused `Reader` into `cmd/garfex/main.go`, the TUI, or the shipped CLI. A future composition root may wrap those services with the adapter and call the public constructor when a concrete Reader consumer is introduced.

### External-package construction and the internal bridge

The public construction contract is:

```go
package resourcecore

type ReadCapabilities interface {
    ActiveClasses(context.Context) ([]CatalogRecord, error)
    CatalogDescriptors(context.Context) ([]CatalogDescriptor, error)
    ListCatalog(context.Context, CatalogQuery) (CatalogPage, error)
    GetCatalog(context.Context, CatalogKey) (CatalogRecord, error)
    SearchResources(context.Context, ResourceQuery) (ResourcePage, error)
    GetResource(context.Context, ResourceKey) (Resource, error)
    DescribeResource(context.Context, ResourceKey) (string, error)
}

type Reader struct { /* unexported capability and copy policy */ }

func NewReadOnly(cap ReadCapabilities) (*Reader, error)
```

`Reader` exposes corresponding read methods only. `NewReadOnly(nil)` returns `INVALID_ARGUMENT`. There is no `Create`, `Update`, `Deactivate`, `Reactivate`, `Delete`, `Publish`, `Reload`, repository accessor, or mutable authority accessor on `Reader`.

`ReadCapabilities` is intentionally not sealed: an unexported sealing method would make the specification's external-package construction scenario impossible. It is also not a repository port. Its methods are coarse application reads, use public DTOs, and contain no persistence CRUD or transaction controls. The supported production implementation is the module-owned `internal/bridge/resourcecore.Adapter`; external implementations are expected only at application-composition or test boundaries and must already provide authoritative results. Implementing this seam does not confer write authority and must not duplicate GARFEX rules. Production-grade evidence consists of module-owned bridge tests integrating the real authoritative `catalogo` and `recursos` service interfaces, plus an external-package test that constructs a `Reader` using only public types and a service-shaped fake. Because the public READ contract has no concrete binary consumer yet, `cmd/garfex/main.go` is intentionally outside stage 6 and no unused Reader is wired into the TUI or CLI merely as proof. Future composition roots may instantiate the bridge when a concrete consumer exists.

A public PostgreSQL factory was rejected because accepting a pool, DSN, driver option, or ownership policy would violate the approved consumer-neutral boundary. Reading hidden environment state in `NewReadOnly` was rejected as implicit infrastructure coupling. Re-declaring public repositories was rejected because it would duplicate ports and encourage application-service bypass.

## Public read contract

### DTO model

All public DTOs are package-owned concrete values. Internal domain values are projected, never aliased.

```go
type KindCode string
type LifecycleScope string // ACTIVE, INACTIVE, ALL

type CatalogDescriptor struct {
    Kind KindCode
    Singular, Plural string
    Fields []FieldDescriptor
    IdentityFields []string
    Parent *ParentDescriptor
}

type CatalogRecord struct {
    Kind KindCode
    ID int64                 // opaque record reference, not business identity
    Revision uint64
    Active bool
    Values map[string]Value
    Rules []ApplicabilityRule // non-nil/meaningful only for APLICABILIDAD
}

type ValueKind string
const (
    ValueText ValueKind = "TEXT"
    ValueCode ValueKind = "CODE"
    ValueBool ValueKind = "BOOLEAN"
    ValueInteger ValueKind = "INTEGER"
    ValueDecimal ValueKind = "DECIMAL"
    ValueQuantity ValueKind = "QUANTITY"
    ValueReference ValueKind = "REFERENCE"
    ValueEnum ValueKind = "ENUM"
    ValueStringList ValueKind = "STRING_LIST"
    ValueControlledOption ValueKind = "CONTROLLED_OPTION"
    ValueNotApplicable ValueKind = "NOT_APPLICABLE"
)

type Value struct {
    Kind ValueKind
    Text string              // text/code/enum/option or canonical integer/decimal
    Bool bool
    UnitCode string          // only QUANTITY; Text is its canonical magnitude
    Reference *Reference
    Strings []string
}
```

The tagged union has exactly one meaningful payload for each `Kind`; projection rejects impossible internal shapes instead of guessing. `NOT_APPLICABLE` is its own kind with no payload and is distinct from absent, empty text, and zero. Catalog descriptors remain generic and descriptor-driven rather than introducing eleven public record structs.

`Resource` contains `ID`, `IdentityV1`, `Scope{ClassCode,FamilyCode,TypeCode}`, `NaturalUnit`, `Active`, `Revision`, and ordered `[]AttributeValue`. `IdentityV1` is documented as durable business identity. Catalog `ID` is documented as opaque; natural codes are projected but hash IDs can change after a permitted natural-key change.

`CatalogQuery` and `ResourceQuery` contain explicit lifecycle scope, text/filter fields, `Limit`, and `Offset`. Zero lifecycle scope is `ACTIVE`; `ALL` must be requested explicitly. Pages return normalized `Limit`, `Offset`, `HasPrevious`, `HasNext`, and items in repository-defined deterministic order. The bridge delegates resource presentation to `recursos.Service.Describe`; it never renders attributes itself.

### Canonical numeric strings

A single projector produces canonical base-10 strings:

- integer: `strconv.FormatInt(v, 10)`;
- decimal and quantity magnitude: `decimal.String()` after normalizing negative zero to zero;
- no exponent, leading plus, redundant integer leading zero, or redundant fractional trailing zero;
- zero is always `"0"`.

The public API does not expose `shopspring/decimal`. Parsing/normalization tests cover positive and negative values, scale variants, very small/large values, and negative zero. Decimal data is never converted through `float64`.

### Defensive-copy invariant

The façade copies at both edges:

1. Copy every query slice/map before calling a capability.
2. Copy every capability result before returning it.
3. Deep-copy descriptor fields, enum values, identity fields, references, string lists, resource attributes, option collections, and applicability rule collections.
4. Never cache caller-owned DTO storage.

Nil and empty collections are normalized only where contractually equivalent. For applicability mutation input later, nil and empty are intentionally distinct as described below.

## Public errors and internal diagnostics

### Complete public error model

```go
type ErrorCode string
const (
    InvalidArgument       ErrorCode = "INVALID_ARGUMENT"
    NotFound              ErrorCode = "NOT_FOUND"
    Duplicate             ErrorCode = "DUPLICATE"
    InvalidReference      ErrorCode = "INVALID_REFERENCE"
    Validation            ErrorCode = "VALIDATION"
    Integrity             ErrorCode = "INTEGRITY"
    IdentityConflict      ErrorCode = "IDENTITY_CONFLICT"
    InvalidLifecycle      ErrorCode = "INVALID_LIFECYCLE"
    ReactivationImpossible ErrorCode = "REACTIVATION_IMPOSSIBLE"
    InvalidCatalog        ErrorCode = "INVALID_CATALOG"
    InUse                 ErrorCode = "IN_USE"
    ImmutableCode         ErrorCode = "IMMUTABLE_CODE"
    Conflict              ErrorCode = "CONFLICT"
    Unavailable           ErrorCode = "UNAVAILABLE"
    Internal              ErrorCode = "INTERNAL"
)

func Code(error) ErrorCode
func IsCode(error, ErrorCode) bool
```

The categories are non-overlapping public outcomes. `DUPLICATE` is an ordinary uniqueness collision such as creating an already-existing record. `INTEGRITY` means persisted structure/cardinality is inconsistent. `IDENTITY_CONFLICT` means a durable resource identity disagrees with the Core's canonical identity; it is never downgraded to duplicate or integrity. `CONFLICT` is reserved for stale expected revision/CAS. `VALIDATION` is invalid business input against a valid authority, while `INVALID_CATALOG` means the catalog/candidate authority itself is structurally or semantically unusable. `INVALID_LIFECYCLE` is a transition/precondition violation; `REACTIVATION_IMPOSSIBLE` means an inactive retained record cannot be made valid under the current authority.

The concrete public error has private fields for code and a stable, operation-neutral safe message. It does **not** implement `Unwrap`, expose a `Cause`, retain the original error, or format arbitrary details. `Code` and `IsCode` inspect only this public value; callers cannot recover an internal chain. Context cancellation/deadline and the writer-unavailable latch map to stable `UNAVAILABLE`. Unexpected failures map to `INTERNAL`, or to `UNAVAILABLE` only when a typed internal availability classification says retry/service recovery is required.

### Internal Core outcomes and mapping

Existing errors currently collapse four required semantics, so the Core adds dedicated internal sentinels in `internal/domain` (or application-owned typed errors where operation context is required):

- `ErrIdentityConflict`: replaces `ErrResourceIntegrity` when rehydrated/stored `identity-v1` or the reactivation expected identity differs from the canonical identity. Real persistence/cardinality corruption continues to use `ErrResourceIntegrity`.
- `ErrInvalidLifecycle`: identifies an operation forbidden by current state, especially hard delete of an active record and any non-idempotent unsupported transition. The temporary `ErrSoftDeleteUnsupported` gap is eliminated by all-11 lifecycle parity; if encountered during rollout it is adapted to `ErrInvalidLifecycle`, not validation.
- `ErrReactivationImpossible`: an application-level reactivation outcome that wraps the internal reason when a retained inactive resource/catalog record cannot validate under the current active catalog or cannot restore required references. Canonical/stored identity disagreement is excluded and returns `ErrIdentityConflict` instead. The wrapper remains internal and supports `errors.Is`; the public mapper gives this sentinel priority over wrapped validation/reference causes.
- `ErrInvalidCatalog`: wraps failures proving the current, candidate, transaction-reloaded, or independently loaded catalog is invalid or lossy. Request-level bad combinations continue to use `ErrResourceValidation`; a named but absent reference continues to use `ErrResourceReference`/`ErrCatalogReference`.

`ErrRevisionConflict` is the dedicated stale-CAS sentinel. The writer latch returns a typed/sentinel unavailable outcome. These internal errors may wrap diagnostic causes and retain operation metadata because they never cross the public boundary.

| Authoritative internal outcome | Public code |
| --- | --- |
| malformed DTO, missing key, zero/invalid revision, unsupported enum/paging shape | `INVALID_ARGUMENT` |
| resource/catalog record absent | `NOT_FOUND` |
| ordinary resource/catalog uniqueness collision | `DUPLICATE` |
| request names an absent/incompatible reference | `INVALID_REFERENCE` |
| business-value/rule combination invalid against a valid catalog | `VALIDATION` |
| persisted cardinality, aggregate count, or storage invariant is inconsistent | `INTEGRITY` |
| `ErrIdentityConflict` | `IDENTITY_CONFLICT` |
| `ErrInvalidLifecycle` | `INVALID_LIFECYCLE` |
| `ErrReactivationImpossible` (before inspecting its wrapped reason) | `REACTIVATION_IMPOSSIBLE` |
| `ErrInvalidCatalog` | `INVALID_CATALOG` |
| dependency/history/FK race backstop | `IN_USE` |
| referenced natural code cannot change | `IMMUTABLE_CODE` |
| `ErrRevisionConflict` only | `CONFLICT` |
| cancellation/deadline, indeterminate commit latch, classified temporary authority/storage outage | `UNAVAILABLE` |
| unclassified unexpected failure | `INTERNAL` |

Translation is one centralized, exhaustive `errors.Is`/typed-classification switch with explicit precedence for `ErrRevisionConflict`, `ErrIdentityConflict`, `ErrReactivationImpossible`, and `ErrInvalidCatalog` before broader wrapped sentinels. The public façade and bridge MUST NOT infer a category from `Error()`, substring matching, regular expressions, PostgreSQL messages, or constraint names. PostgreSQL adapters may classify `pgconn.PgError` by typed fields such as SQLSTATE and known constraint identity, then emit a Core sentinel; the public translator never sees or inspects the driver error.

The internal bridge records the original error before translation through a non-public diagnostic seam containing operation, kind/resource key, and cause. Public tests inspect `Error()`, `%v`, `%+v`, concrete type, `errors.Unwrap`, recursive chains, and category identity using injected pgx-like messages; no SQLSTATE, constraint, table, column, driver type, server message, internal sentinel, or diagnostic cause may escape.

## Complete catalog mutation model

### Record and metadata changes

`domain.CatalogRecord` gains `Revision uint64` and `Rules []CatalogRuleRecord`. `CatalogRuleRecord` contains ordered `When{AttributeCode,Equals}`, `Mode`, `IdentityParticipates`, `NotApplicable`, and `Active`. Every descriptor's `SoftDelete` becomes true because every one of the 11 domain structures has an `Active` field. Comments and tests that describe five-kind support are replaced.

Mutation inputs are deep-cloned before use. `CatalogValue.List`, maps, aliases/keywords, and rules never share backing storage with the caller or prior snapshot.

### Exact all-11 `ApplyCatalogMutation` strategy

| Kind | Snapshot collection | Builder/lifecycle behavior |
| --- | --- | --- |
| `CLASE` | `Classes` | Preserve `Active`, aliases, keywords; copy lists; use `Active` setter. |
| `FAMILIA` | `Families` | Preserve `Active` and class reference; use `Active` setter. |
| `TIPO` | `Types` | Preserve `Active`, class/family references; use `Active` setter. |
| `CARACTERISTICA` | `Definitions` | Preserve `Active`; use `Active` setter; after update rebuild every embedded `ResourceAttribute.Definition` from the new canonical definition. |
| `CONJUNTO_OPCIONES` | `OptionSets` | Perform real insert/replace/set-active/remove by canonical code; never return the unchanged snapshot. |
| `OPCION` | `Options` | Preserve option set, definition reference, label, code, and `Active`; use `Active` setter. |
| `RELACION_OPCIONES` | `Relations` | Preserve option set, both definition/option endpoints, and `Active`; use `Active` setter. |
| `UNIDAD` | `Units` | Preserve code/name/symbol/dimension and `Active`; use `Active` setter. |
| `POLITICA_UNIDAD` | `UnitPolicies` | Preserve class/family/unit refs, allowed/suggested, and `Active`; use `Active` setter. |
| `APLICABILIDAD` | `Attributes` | Preserve all scope refs, option set, materialized definition, mode, identity flag, `Active`, and complete ordered `Rules`; use `Active` setter. |
| `PRESENTACION` | `PresentationFields` | Preserve scope, definition ref, position, and `Active`; use `Active` setter. |

For a definition update, the candidate first replaces the item in `Definitions`, then rebuilds all matching embedded definitions in `Attributes`, including active and inactive bindings. This prevents stale name, value type, dimension, default identity, or active data. Natural code changes are rejected as `ErrCodeImmutable` whenever any catalog dependency or resource relationship exists; therefore the candidate does not silently cascade business identities. An unreferenced definition rename has no embedded consumers. The same dependency-aware immutability policy applies to every `ImmutableOnceReferenced` field.

Deactivate/reactivate only toggles `Active` on a copied item and preserves every other field and nested rule. Delete removes only the selected parent object; application guards ensure this cannot orphan another collection.

### Applicability aggregate

For `APLICABILIDAD` create/update, `Rules == nil` means “aggregate omitted” and is invalid. A non-nil empty slice means an explicitly rule-free aggregate and is allowed only when domain validation permits it. The complete slice is replacement semantics, not patch semantics.

The PostgreSQL adapter performs parent and rules in one transaction:

- create parent with revision 1, then insert every ordered rule;
- update parent with CAS, delete/replace all child rules, verify persisted count and order;
- parent or any child error rolls back the whole transaction;
- lifecycle changes only parent `active` and revision; nested rule `Active` values remain unchanged;
- hard delete removes parent and rules through existing `ON DELETE CASCADE` only after guards;
- child rules have no independent public ID/revision contract; their concurrency token is the parent applicability revision.

The private candidate is fully built and `Validate()`d before persistence. A malformed requested rule/value maps to validation or invalid reference, while proof that candidate materialization, transaction reload, or snapshot semantics are themselves invalid wraps `ErrInvalidCatalog`. The transaction then reloads and compares the complete aggregate, so omission or alteration of a definition relationship or rule rolls back before publication.

## Lifecycle, revision, and transaction contracts

### Definitive application-service APIs

The internal application boundary evolves toward these explicit contracts only through additive, compile-safe transitions. These signatures are an end state, not permission to break a port or caller in an earlier PR:

```go
func (s *Service) Create(ctx context.Context, kind domain.CatalogKindCode, rec domain.CatalogRecord) (domain.CatalogRecord, error)
func (s *Service) Update(ctx context.Context, kind domain.CatalogKindCode, rec domain.CatalogRecord, expected uint64) (domain.CatalogRecord, error)
func (s *Service) Deactivate(ctx context.Context, kind domain.CatalogKindCode, id int64, expected uint64) (domain.CatalogRecord, error)
func (s *Service) Reactivate(ctx context.Context, kind domain.CatalogKindCode, id int64, expected uint64) (domain.CatalogRecord, error)
func (s *Service) HardDelete(ctx context.Context, kind domain.CatalogKindCode, id int64, expected uint64) error
```

The ambiguous catalog `Delete` name is retired after internal callers migrate; no compatibility alias is exposed publicly. Resource write contracts similarly gain `ExpectedRevision uint64` and return the resulting revision. Creation starts at revision 1 and has no expected revision.

Expected revision zero is invalid. Update and lifecycle compare against persistence, not only the in-memory snapshot. A stale expected revision maps to `ErrRevisionConflict`. For an already-requested lifecycle state, the expected revision is still checked; when current, the operation is an idempotent no-op returning the unchanged revision and causing no publication. Every actual update or active-state change sets `revision = revision + 1` exactly once. A stale expected revision always returns `ErrRevisionConflict`; state/lifecycle checks run only after the authoritative CAS disambiguation so stale requests cannot be misreported as `INVALID_LIFECYCLE`.

Reactivation validates the retained record under the current catalog. A canonical identity mismatch returns `ErrIdentityConflict`; inability to rebuild a valid active record for any other known lifecycle reason returns `ErrReactivationImpossible` with the validation/reference cause retained internally. These remain distinct from duplicate uniqueness, storage integrity, invalid lifecycle, and stale revision outcomes.

### Schema and migration

Migration `000008_resource_revisions` adds:

```sql
revision BIGINT NOT NULL DEFAULT 1 CHECK (revision > 0)
```

to `recursos` and these 11 parent tables:

1. `resource_classes`
2. `resource_families`
3. `resource_types`
4. `attribute_definitions`
5. `resource_option_sets`
6. `attribute_options`
7. `attribute_option_relations`
8. `unit_definitions`
9. `resource_unit_policies`
10. `resource_attributes`
11. `resource_type_presentation_fields`

Existing rows backfill to 1. `resource_attribute_rules` deliberately receives no revision. Application-role SELECT/INSERT/UPDATE behavior and any column-level grants are verified after adding the column. The down migration drops only these revision constraints/columns; it never rewrites identity keys or sequences.

No trigger increments revision. Every authoritative SQL mutation explicitly increments it in the same statement as the data change, making accidental non-authoritative writes detectable in review and avoiding double increments. `updated_at` remains diagnostic metadata, not CAS. `xmin` is rejected because it is PostgreSQL-specific and not durable across maintenance. Hash-derived IDs are rejected as revisions and business identities because they can collide/change with natural keys.

Internal deployment order is: apply migration 8; deploy code that reads revisions while the additive V2 write path remains dormant; prove PostgreSQL integrations/races; complete stage-4 service adoption; stop the old writer; then enable the one revision-aware internal writer. Old read code ignores the additive column. Old write code could bypass CAS, so there must be no mixed-version writer deployment. No public package is released until stage 6 READ-ONLY, and public WRITE remains a future operation-gated change. The column may remain during code rollback.

### SQL CAS classification

Update/lifecycle SQL follows:

```sql
UPDATE <table>
SET ..., revision = revision + 1, updated_at = NOW()
WHERE <physical key predicate> AND revision = $expected
RETURNING revision;
```

For tables addressed publicly by hash-derived IDs, the physical predicate resolves the hash to the current natural-key row, but revision remains a real stored column. The returned record may have a changed opaque ID after a permitted unreferenced natural-key update.

If `RETURNING` has no row, the adapter performs a same-transaction existence/revision lookup under row lock: absent means not found; present with another revision means conflict. It does not infer conflict from a constraint or error string. Delete uses `DELETE ... WHERE key AND revision=$expected RETURNING ...`, with the same disambiguation. Resource update, deactivate, reactivate, and any future resource hard delete use identical CAS principles; replacement of resource attribute values occurs in the same resource transaction after the parent CAS and rolls back with it.

### Catalog transaction and coherent publication

The end-state catalog repository port returns a `CatalogWriteResult` containing the resulting record (when any) and a complete coherent `ResourceCatalog`. It is **not** changed in place across PRs. Stage 3D may define the complete additive domain `CatalogAdminRepositoryV2` contract (name may be refined locally) and `CatalogWriteResult`, with domain tests and fakes, while leaving the old interface unchanged. Stage 3D MUST NOT add a PostgreSQL constructor returning V2 or a concrete compile-time assertion. Stages 3E and 3F build dormant internal transactional helpers and kind/method families without claiming that the PostgreSQL concrete satisfies the complete interface. After every V2 operation family exists, stage 3G adds the PostgreSQL concrete interface methods, `NewCatalogAdminRepositoryV2`, and `var _ CatalogAdminRepositoryV2 = ...` assertion together as one independently green slice; the existing constructor and port remain valid. A compile-safe stub MUST NOT silently return unsupported for a method that later becomes authority. A later service-adoption slice adds a V2 service constructor/path and adapts every fake before compiling against it. Only after all methods, production write composition roots, and exact TUI callers use the complete path does the old constructor become a delegating compatibility wrapper; removal is a separate green slice.

Each V2 adapter mutation transaction does this in order:

1. perform CAS/insert/delete, including applicability children;
2. load all 11 catalog collections and all nested rules through `loadResourceCatalogTx` in that same transaction;
3. validate the loaded catalog;
4. compare it with the service-supplied private candidate using domain-owned normalization/equivalence;
5. roll back on mismatch;
6. commit;
7. return the loaded catalog.

`catalogo.Service`, while holding its process-local writer mutex, publishes the returned catalog exactly once only after the repository reports a successful commit. It never publishes the hand-built candidate. Validation, dependency, stale CAS, SQL, reload, equivalence, and commit failures publish nothing. A commit whose outcome is technically indeterminate latches the writer unavailable and publishes nothing; an operator must perform an explicit coherent reload/restart before writes resume. This avoids publishing an unconfirmed state.

An independent post-operation `LoadResourceCatalog` remains the integration-test oracle. Normalization is limited to representational equivalence (default option-set spelling and nil/empty collections), never field/rule omission, active state, revision, or identity. All 11 kinds and all five operations compare authority with this oracle.

## Conservative hard delete

`catalogo.Service.HardDelete` owns the policy and executes:

1. reject invalid kind/id/revision;
2. load current persisted record and classify stale revision before any candidate work;
3. reject an active target with `ErrInvalidLifecycle` (public `INVALID_LIFECYCLE`), never generic validation;
4. call `Dependents` and reject if **any** count is non-zero, ignoring historical `Blocking` metadata;
5. reject if `ReferencedByResources` reports any active or inactive/history relationship;
6. apply delete to a private snapshot and validate it;
7. invoke CAS hard delete; database foreign keys are the race backstop;
8. receive the transaction's coherent reload and publish once.

Dependency probes must count rows without `active` filters. `Blocking` remains useful for deactivation/user guidance but cannot relax hard delete. PostgreSQL FKs ensure a dependency committed before or racing with the delete prevents one side from violating referential integrity; FK violation maps internally to in-use. If a known logical relationship lacks an FK, migration 8 or a preceding compatibility slice adds the constraint before hard delete readiness rather than relying on a racy preflight.

No consumer preflight authorizes deletion. Deactivation is preferred whenever history exists. Active targets, dependencies, stale revisions, and transaction failures leave authority unpublished.

## Resource revisions and identity

`domain.Resource`, snapshots, pages, loaders, and persistence projections gain `Revision uint64`. Resource create stores revision 1. Update replaces parent and attributes atomically only after CAS; deactivate/reactivate increment revision only on an actual transition. `identity_key` is neither regenerated by the revision migration nor used as the revision. Existing `identity-v1` values are read byte-for-byte before and after migration.

Canonical reactivation integration tests create/deactivate/reactivate through `recursos.Service`, pass the current revision, and assert unchanged `identity-v1`, incremented revision, complete attributes, and authoritative validation. Direct repository reactivation alone is insufficient exit evidence.

## Writer topology and freshness

Exactly one process is designated as the authoritative writer, and that process owns one `catalogo.Service`/`CatalogAuthority` composition. Its mutex serializes catalog writes locally; persisted CAS additionally protects stale callers. CAS is not presented as permission for multiple writer processes because two processes could publish different in-process authorities and there is no refresh protocol.

A read-only process loads one coherent snapshot at construction. After another process writes, it must explicitly reconstruct/reload its bridge or restart to obtain freshness. `Reader` intentionally exposes no `Reload`, because publication control is not a consumer capability. No LISTEN/NOTIFY, polling, shared cache, leader election, or event stream is implemented now: these add operational delivery and failure semantics that are unnecessary for the approved one-writer topology and would broaden the contract prematurely.

## Files and change surfaces

| Surface | Planned change |
| --- | --- |
| `resourcecore/*.go`, `resourcecore/*_test.go` | Public read-only DTOs, reader, canonical values, copies, safe errors, external-package tests. |
| `internal/bridge/resourcecore/*.go` | Module-owned adapter to authoritative services and non-public diagnostic seam. |
| `internal/domain/catalog_record.go` | Revision and applicability aggregate records; revised CAS port results. |
| `internal/domain/catalog_kind.go` | Correct all-11 lifecycle metadata/comments and defensive descriptor copies. |
| `internal/domain/catalog_mutation.go` | Exact all-11 lossless copy-on-write, OptionSets, active setters, rule preservation, definition rebuild. |
| `internal/domain/resource_types.go`, `catalog_repository_errors.go`, and a focused Core error file if needed | Resource revisions, command expected revisions, and dedicated `ErrIdentityConflict`, `ErrInvalidLifecycle`, `ErrReactivationImpossible`, `ErrInvalidCatalog`, and `ErrRevisionConflict` identities without public exposure. |
| `internal/domain/catalog_authority.go` | Continue deep copies; include every nested collection; no public publication access. |
| `internal/app/catalogo/service.go` | Revision checks, aggregate mutation, hard-delete policy, coherent-result publication. |
| `internal/app/recursos/service.go` | Revision-bearing read/write projections and CAS delegation; presentation unchanged. |
| `internal/postgres/catalog_admin_repository.go`, `catalog_admin_kinds.go` | Revision selects, CAS, atomic applicability, guarded delete, transactional reload/result. |
| `internal/postgres/catalog_loader.go` | Reusable transaction loader and complete aggregate hydration. |
| `internal/postgres/resource_repository*.go` | Resource revision select/CAS and atomic lifecycle behavior. |
| `migrations/000008_resource_revisions.*.sql` | Additive revision schema/backfill/down path. |
| Integration and app tests | All-11 lifecycle, stale races, identity preservation, coherent reload, fixtures. |
| Exact TUI compatibility surfaces | Stage 4F is limited to the eight enumerated state/test files below. Stage 4G is limited to its thirteen enumerated caller/interface/fake files, including `internal/tui/resource_editor.go` and `internal/tui/catalogo_recursos_u2a_test.go`. No new TUI behavior. |

## PostgreSQL fixtures and evidence

Migration-5/7 reset restoration comes first because fixture-only work cannot honestly preserve or execute the legacy migration-5/7 integration scenarios on a migration-7 database. Reset tests are serialized with an integration advisory lock, are not `t.Parallel`, capture the original migration-5 identity map and migration-7 constraint state, and restore both in cleanup even when the test body fails. Starting from migration 7, they run down 7, reverse only owned/mapped v1 rows to legacy through the identity map, run down 5, execute every historical collision/applicability/mapping/up-down scenario without skips, early returns, or weakened assertions, remove only owned rows, then restore migration 5 and migration 7 when initially present. The lock is released only after restoration, and every other schema-destructive integration test uses the same coordination. Migration 5/7 SQL remains unchanged.

Only after that reset suite is green may identity-v1 fixtures be repaired. Existing direct inserts such as `TEST_REPO_IDENTITY_KEY`, `TEST_U2A_RESTART_KEY`, and `TEST_UNIT_NAMES_RESOURCE` derive valid unique `v1|...` identities from their actual fixture scope and persisted identity-participating values; no synthetic unpersisted suffix participates. Fixtures retain returned IDs, clean ID-owned children before parents, never blanket-delete seeded business rows, and preserve the reset scenarios unchanged. Migration 8 tests then snapshot existing identity keys, apply/backfill revision, and assert exact identity equality. A migration test must not leave the shared integration database at version 5 or 7 for another test.

The all-11 fixture creates isolated parent chains, options, relations, applicability rules, presentation, and resources. After stage 3G establishes complete PostgreSQL V2 conformance, catalog persistence operations exercise that complete additive adapter directly while it remains non-authoritative; canonical resource reactivation is additionally proven end-to-end through `recursos.Service`. For race proof, two independent database connections issue writes with the same expected revision. Exactly one succeeds, one receives conflict, and persisted revisions advance once. Hard-delete/dependency races use separate connections and verify the dependency or delete wins consistently with FK integrity, never both. In stage 4, the same all-11 scenarios are rerun through `catalogo.Service` and additionally assert exactly one authority publication equivalent to an independent reload.

Integration tests skip under `testing.Short()` or absent integration DSN. Race evidence is `go test ./... -race -count=1` in CI plus isolated PostgreSQL integration execution. No test weakens identity-v1, FK constraints, CAS, or guards.

After PostgreSQL and authority stages are green, stage-5 focused table-driven error tests cover all fifteen public categories and assert exact code identity. Additional scenarios prove: canonical/stored identity mismatch is `IDENTITY_CONFLICT` rather than `INTEGRITY`; ordinary uniqueness remains `DUPLICATE`; active hard delete is `INVALID_LIFECYCLE`; stale hard delete is `CONFLICT` even when the current state would also violate lifecycle; failed reactivation under a changed catalog is `REACTIVATION_IMPOSSIBLE`; an invalid/lossy candidate or coherent reload is `INVALID_CATALOG`; cardinality corruption remains `INTEGRITY`; and dependency races remain `IN_USE`. Mapper unit tests inject wrapped sentinels to verify precedence and determinism without strings. Leakage tests inject driver errors containing SQLSTATE, constraints, tables, columns, and server text, verify the internal diagnostic sink received the original cause, and verify the returned public error has no `Unwrap` path or retained cause.

## Compatibility invariant and compile-safe transitions

> **Compatibility invariant:** after every slice, `go test ./... -count=1` builds every current adapter, service, composition root, TUI call site, test double, and external test. A slice never changes a domain port unless all current implementations and applicable compile-time assertions are adapted in that slice; never adds a PostgreSQL constructor returning a complete interface or a concrete conformance assertion before every method is real; never uses an unsupported-returning stub for a method that later becomes authority; never changes a service signature unless every current caller and fake is adapted in that slice; never routes production authority through a partially implemented V2 path; and never leaves Core correctness dependent on a later PR. The exact TUI surface enumerations are authoritative. A final textual reference scan is a gate, and any additional current caller requires design and tasks correction before the affected slice starts; no dynamically discovered caller may be deferred. When work cannot fit within 400 changed lines, it is split into another additive, dormant slice rather than weakening these rules.

### Additive transition mechanics

1. **Coherent write results and complete domain contract:** stage 3D adds `CatalogWriteResult` and the complete `CatalogAdminRepositoryV2` domain interface without editing the old interface. Domain tests/fakes prove defensive result copies and the complete method set. No PostgreSQL constructor returning V2 and no PostgreSQL concrete conformance assertion exist yet.
2. **Dormant PostgreSQL families:** stages 3E and 3F add transactional applicability helpers and narrow CAS/kind handlers beside current parent-row handlers. They remain internal and dormant, do not claim complete interface conformance, and do not use unsupported-returning compile stubs. The legacy methods remain untouched.
3. **Complete PostgreSQL V2 conformance:** stage 3G finishes every remaining V2 operation family, then adds the concrete interface methods, `NewCatalogAdminRepositoryV2`, and the concrete `var _ CatalogAdminRepositoryV2 = ...` assertion in that same independently green slice. Zero-row disambiguation and SQL tests land with each real method family. No existing interface gains a method mid-chain.
4. **Service adoption:** add revision-aware methods and a V2 constructor beside current APIs, adapting service fakes in the same slice. After the whole V2 path is proven, migrate every production composition root together. Legacy service methods then delegate to the complete revision-aware path using a freshly read revision; they never retain a revision-less SQL path.
5. **TUI compatibility:** stage 4F carries revision state through exactly these eight files: `internal/tui/catalog_admin.go`, `internal/tui/catalog_admin_dispatch.go`, `internal/tui/catalog_wizard.go`, `internal/tui/resource_editor_state.go`, `internal/tui/resource_editor_persistence.go`, `internal/tui/catalog_admin_test.go`, `internal/tui/resource_editor_test.go`, and `internal/tui/resources_workspace_adapter_test.go`. Stage 4G co-migrates interfaces, callers, and fakes through exactly these thirteen files: `internal/tui/catalog_admin.go`, `internal/tui/catalog_admin_dispatch.go`, `internal/tui/catalog_wizard.go`, `internal/tui/resource_editor.go`, `internal/tui/resource_editor_persistence.go`, `internal/tui/resources_workspace_dispatch.go`, `internal/tui/catalog_admin_test.go`, `internal/tui/resource_editor_test.go`, `internal/tui/resource_lifecycle_test.go`, `internal/tui/resources_workspace_adapter_test.go`, `internal/tui/catalogo_recursos_u2a_test.go`, `internal/app/catalogo/service_test.go`, and `internal/app/recursos/service_test.go`. These enumerations are authoritative. Before either slice starts, a final textual scan for catalog/resource `Create`, `Update`, `Deactivate`, `Reactivate`, `Delete`, `SetActive`, and V2 constructor references must confirm the list; any additional current caller requires design/tasks correction first and cannot be deferred. Do not remove old service methods until the scan and compile assertions prove no current caller remains.
6. **Authority switch:** keep V2 dormant while incomplete. Switch `CatalogAuthority` publication only when all V2 writes return validated committed reloads. In the switch slice, every successful path publishes the returned catalog once and every old entry point delegates to that same path; there is no second writer flow.

### Slice-transition proof

| Stage / independently green slice | Start state | End state and compatibility proof | Required focused evidence | Forecast |
| --- | --- | --- | --- | --- |
| 1A mutation copy primitives | Current domain API | Add deep-copy helpers used by existing mutation code; no signatures change | domain aliasing/value tables; full suite | ≤250 lines |
| 1B applicability value model | 1A | Add `CatalogRuleRecord`/rules to records and copying while old callers omit them safely | nil/empty/reorder/caller-mutation tests | ≤300 lines |
| 1C exact kind materializers I | 1B | Correct `CLASE` through `CONJUNTO_OPCIONES`; current `ApplyCatalogMutation` signature remains | create/update losslessness and OptionSet non-no-op | ≤400 lines |
| 1D exact kind materializers II | 1C | Correct `OPCION` through `PRESENTACION`, definition rebuild, and aggregate candidate validation | remaining kind tables and invalid aggregate rejection | ≤400 lines |
| 2A active-state domain types | Stage 1 green | Add/preserve `Active` and setters for kinds currently lacking them; all builders/loaders compile in this slice | inactive round trips, descriptor-copy tests | ≤400 lines |
| 2B lifecycle registry + operations | 2A | Mark all 11 lifecycle-capable and remove no-op/unsupported semantics only after all materializers support them | 11×5 domain matrix | ≤400 lines |
| 2C conservative service guards | 2B | Existing signatures retained; every dependency blocks hard delete and active delete fails | app fake tables, no repo call/publication | ≤350 lines |
| 3A migration-5/7 reset compatibility | Stage 2 green | Restore every historical reset scenario; advisory-lock schema changes; capture and restore migration-5 map and migration-7 constraint state even on body failure; migration SQL stays read-only | injected failure/restoration, seeded identity unchanged, focused reset suite, full suite | ≤350 lines |
| 3B identity-v1 fixture repair | 3A | Repair fixture identities from actual scope plus persisted identity-participating values; retain IDs and owned child-before-parent cleanup; preserve the 3A reset suite | focused PostgreSQL fixtures and canonical identity assertions | ≤300 lines |
| 3C revision schema/loaders | 3B, including preserved 3A reset suite | Add migration 8, `Revision` fields, loader/backfill/grant compatibility; old write APIs still compile | identity byte equality, migration up/down, loader tests | ≤400 lines |
| 3D additive V2 domain contract | 3C | Define complete `CatalogAdminRepositoryV2` and `CatalogWriteResult` with domain tests/fakes; legacy port unchanged; no PostgreSQL V2 constructor or concrete assertion | domain contract/fake compile tests and result-copy tests | ≤250 lines |
| 3E dormant applicability internals | 3D | Build atomic parent+ordered-rule transactional helpers/reload internally; no complete interface conformance claim | commit/rollback/count/order integration | ≤400 lines |
| 3F dormant catalog CAS handlers | 3E | Build real create/update CAS handlers for narrow kind groups; no unsupported stubs, constructor, or concrete assertion | success/stale/not-found SQL tables | ≤400 lines |
| 3G complete PostgreSQL V2 conformance | 3F | Finish remaining create/update/lifecycle/delete families, then add concrete interface methods, `NewCatalogAdminRepositoryV2`, and `var _ CatalogAdminRepositoryV2` together | all-11 SQL matrix, FK backstop, constructor and compile assertion | ≤400 lines |
| 3H resource CAS + canonical reactivation | 3G | Add compatible repository V2 methods; `recursos.Service` E2E proves deactivate/reactivate identity-v1 and attributes | canonical service E2E, stale cases | ≤400 lines |
| 3I PostgreSQL race exit | 3H | PostgreSQL is completely green before authority work begins | two-connection stale race, delete/dependency race, integration suite, `-race` CI | ≤350 lines |
| 4A domain equivalence | Stage 3 green | Add normalization/equivalence without port or service changes | field/rule omission is unequal; representational normalization only | ≤250 lines |
| 4B transactional coherent result | 4A | V2 writes load, validate, compare, then commit complete catalog | rollback on reload/equivalence/commit failure | ≤400 lines |
| 4C additive service V2 | 4B | Add V2 constructor/methods and adapt all service fakes in the same slice; production still legacy | no-publication failures and returned-result tests | ≤400 lines |
| 4D write composition + authority switch | 4C | Migrate all production write composition roots; old entry points delegate to V2; publish committed result once | authority versus independent reload for focused kind group | ≤400 lines |
| 4E all-11 authority oracle | 4D | Complete 11×5 service oracle and writer-unavailable latch; no temporary candidate publication remains | exact publication count, integration and full suite | ≤400 lines |
| 4F TUI revision state | 4E | Carry captured revisions through the exact eight enumerated state/test files; service calls unchanged | direct `Model.Update` state tests plus pre-slice textual reference gate | ≤300 lines |
| 4G TUI revision calls | 4F | Co-migrate the exact thirteen enumerated caller/interface/fake files, including `resource_editor.go` and `catalogo_recursos_u2a_test.go`, to revision-aware methods | catalog/resource TUI focused, final textual reference scan, and full suite | ≤400 lines |
| 4H compatibility retirement | 4G | Optional later removal only when grep/compile proves no caller; otherwise keep safe delegators | compile assertions and full suite | ≤250 lines |
| 5A Core semantic outcomes | Stage 4 green | Add dedicated internal outcomes and emit them at authoritative app/adapter sites | non-collapse app/PostgreSQL tables | ≤350 lines |
| 5B neutral mapper + diagnostics | 5A | Add internal neutral translation and opaque diagnostic seam, still no public package | fifteen-category precedence and leakage tests | ≤400 lines |
| 6A public read DTOs | Stage 5 green | Add owned DTOs, canonical values, copies, filters/pages; no constructor or writes | external-package DTO/copy tests | ≤400 lines |
| 6B public Reader + bridge | 6A | Add the read-only constructor and module-owned internal bridge; prove real authoritative service-interface integration under tests without wiring the unused Reader into CLI/TUI | external-package construction using only public types, bridge integration, presentation delegation, safe errors | ≤400 lines |
| 6C public read exit/docs | 6B | Document identity, ownership, freshness, one writer, and no public WRITE | focused/full tests, vet/lint/race report | ≤300 lines |

Every row starts only after the preceding row's focused tests and full suite are green. If measured additions plus deletions exceed the forecast, split by kind group, method family, or test fixture; do not request a broad cross-layer exception.

## TDD and ordered auto-chain

Every review unit in the transition table targets at most 400 changed lines and executes strictly in stage order 1→6. Each unit records four evidence points: **RED** (new focused behavior test fails for the intended reason), **GREEN** (smallest implementation passes focused tests), **TRIANGULATE** (at least one additional edge/failure case prevents a trivial implementation), and **REFACTOR** (cleanup occurs only while focused tests remain green). A commit/PR note records commands and observed failure/pass summaries; a test added after implementation is not accepted as RED evidence.

The chain is: mutation correctness → all-11 lifecycle → completely green PostgreSQL → authority equivalence → neutral errors → public READ-ONLY. TUI compatibility belongs inside authority/service adoption, not after a breaking signature. Public WRITE is not part of this stabilization chain; a future operation-gated chain may begin only after READ-ONLY ships and must preserve READ as the first public contract.

Focused package tests run first, then `gofmt -l .`, `go vet ./...`, `golangci-lint run ./...`, and `go test ./... -count=1`. CI-only build and race checks are reported separately under project policy.

## READ and per-operation WRITE readiness gates

READ-ONLY readiness is evaluated independently from future public WRITE readiness, but only **after stages 1–5 are fully green**. It graduates when every reachable read/construction failure maps deterministically into the complete public model, with focused evidence for `INVALID_ARGUMENT`, `NOT_FOUND`, `INTEGRITY`, `INVALID_CATALOG`, `UNAVAILABLE`, and `INTERNAL`, plus negative assertions that no read path mislabels an outcome as another of the fifteen categories. The stage-5 neutral mapper is reused; the public enum defines all fifteen stable identities at read release so future operation-gated writes do not change error identity. Read graduation also requires no-public-`Unwrap`, diagnostic-cause retention only behind the internal seam, and proof that no translation parses error strings.

Each of create, update, deactivate, reactivate, and hard delete has a separate WRITE checklist. An operation remains absent/disabled until tests cover every category reachable by that operation and explicit non-collapse cases across the complete set: `INVALID_ARGUMENT`, `NOT_FOUND`, `DUPLICATE`, `INVALID_REFERENCE`, `VALIDATION`, `INTEGRITY`, `IDENTITY_CONFLICT`, `INVALID_LIFECYCLE`, `REACTIVATION_IMPOSSIBLE`, `INVALID_CATALOG`, `IN_USE`, `IMMUTABLE_CODE`, stale-revision `CONFLICT`, `UNAVAILABLE`, and `INTERNAL`. A category may be marked not reachable only with a reviewed operation-specific reason; it must not silently fall through to `INTERNAL`. Reactivate cannot graduate without identity-conflict and impossible-reactivation scenarios. Hard delete cannot graduate without invalid-lifecycle, in-use, stale-conflict precedence, and FK-race scenarios. Update cannot graduate without immutable-code, identity/duplicate distinctions where applicable, and stale conflict.

The gate evidence is a table-driven mapper matrix, application boundary scenarios proving the correct dedicated Core sentinel is emitted, PostgreSQL classification/integration scenarios, leakage assertions, and the existing CAS/atomicity/publication/equivalence/race evidence. Any regression in distinction, no-public-`Unwrap`, diagnostic handling, or no-string-parsing withdraws only the affected write operation (or READ if a reachable read mapping regresses).

## Compatibility rollout and rollback

### Rollout

1. Merge catalog mutation correctness slices; deploy no public API and change no production authority route.
2. Merge all-11 lifecycle alignment and conservative guards; every slice keeps current adapters, services, TUI, fakes, and full tests green.
3. Restore migration-5/7 reset compatibility first, because reset restoration is required for honest fixture execution on a migration-7 database; then repair identity-v1 fixtures while preserving that reset suite. Apply additive migration 8, define the complete V2 domain contract without premature PostgreSQL conformance, build dormant applicability/revision/CAS families, and only after all methods exist add the PostgreSQL V2 constructor/assertion in stage 3G; then complete canonical reactivation E2E, integrations, and two-connection race evidence. PostgreSQL must be completely green before proceeding.
4. Complete coherent write results and service adoption additively, migrate production write composition and the exact authoritative TUI caller/interface/fake surfaces, stop the old writer, deploy exactly one revision-aware writer, then switch publication to committed transaction reloads. The pre-slice and final textual scans must find no unplanned caller; any additional current caller requires design/tasks correction before implementation. Prove all-11 persistence/publication equivalence before proceeding.
5. Stabilize authoritative Core outcomes and neutral safe translation, including no-unwrapping leakage evidence. No public package exists yet.
6. Add and release the public READ-ONLY DTOs, `Reader`, and module-owned bridge as the first public contract. Prove the bridge against real authoritative service interfaces under tests and prove external construction using only public types; do not modify `cmd/garfex/main.go` or wire an unused Reader into the CLI/TUI without a concrete consumer. WRITE symbols do not exist.
7. Consider public WRITE later, per operation, only after a separate readiness review proves every reachable stable outcome and retains all Core, CAS, atomicity, and equivalence evidence.

### Rollback

- Before stage 6 there is no public contract to preserve; revert only the latest independently green additive slice and keep earlier stabilization stages.
- After stage 6 the read-only package may remain while any future public WRITE slice is disabled or reverted; never route around application services.
- Revision columns may remain inert during code rollback. Do not run migration 8 down while any revision-aware process or stored contract exists.
- If a commit outcome/publication is uncertain, disable the sole writer and perform a coherent reload/restart; never publish the candidate speculatively.
- Rollback cannot restore revision-less mixed writers, weaken hard-delete policy, alter identity-v1, expose technical causes, or introduce cross-process refresh claims.

## Invariants and failure modes

| Invariant/failure | Required behavior |
| --- | --- |
| Caller mutates DTO storage | No accepted or returned state changes. |
| Invalid typed union/canonical number | `INVALID_ARGUMENT` or `VALIDATION`; no authority call when shape-invalid. |
| Invalid request against valid domain rules | `VALIDATION` or `INVALID_REFERENCE`; no persistence/publication. |
| Candidate/current/reloaded catalog invalid or lossy | `INVALID_CATALOG`; no publication and rollback when pre-commit. |
| Stored/canonical identity mismatch | `IDENTITY_CONFLICT`, never `DUPLICATE` or `INTEGRITY`. |
| Retained record cannot reactivate under current authority | `REACTIVATION_IMPOSSIBLE`; internal reason retained only diagnostically. |
| Lifecycle precondition is invalid | `INVALID_LIFECYCLE`; no persistence/publication. |
| Applicability child fails | Whole transaction rolls back; no publication. |
| Stale expected revision | `CONFLICT` only; no data/revision/publication change. |
| Active or referenced hard-delete target | Reject; no delete/publication. |
| SQL/reload/equivalence failure before commit | Rollback and no publication. |
| Indeterminate commit | Return unavailable, publish nothing, latch writer until reload/restart. |
| Successful catalog mutation | Commit complete state, publish exactly once, match independent coherent reload. |
| Another process remains stale | Explicit reload/restart required; no live-coherence claim. |

## Alternatives rejected

- **Eleven public catalog DTO types:** duplicates descriptor knowledge and makes registry evolution expensive; generic tagged DTOs preserve type safety with less rule duplication.
- **Public decimal dependency:** leaks an implementation dependency; canonical strings are lossless and transport-neutral.
- **`updated_at`, `xmin`, or hash ID as revision:** timestamps are not a strict monotonic contract, `xmin` is not durable/public, and hashes are mutable opaque references.
- **Publish the hand-built candidate after independent commit:** can diverge from persistence or lose rules; same-transaction coherent load is the publication source.
- **Reload only after commit:** a reload failure would leave persistence changed without a publishable result; loading/comparing before commit makes ordinary failure atomic.
- **Trust `RelationDescriptor.Blocking` for hard delete:** inactive/history/non-blocking relationships still retain meaning and all block physical deletion.
- **Expose public WRITE with the initial `Reader`:** violates independent read readiness and makes incomplete operations discoverable.
- **Cross-process notification now:** introduces a separate distributed freshness protocol outside the one-writer requirement.
