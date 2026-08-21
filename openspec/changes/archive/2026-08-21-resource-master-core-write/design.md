# Design: Public WRITE on the Resource Master Core — Create

## Decision summary

`resourcecore` gains a second public contract object, `Writer`, built exactly like `Reader`: a package-owned struct over a narrow, service-shaped capability seam, validating request *shape* only, defensively copying at both edges, and translating internal outcomes into the existing fifteen-category public error model. The complete eventual write surface — Create, Update, Deactivate, Reactivate, and catalog-only HardDelete — is fixed here on paper so later operations *extend* the shape. This change compiles exactly two methods: `CreateCatalog` and `CreateResource`. No ungraduated operation exists as a symbol, a stub, an "unimplemented" branch, an unused request field, or any other runtime-discoverable artifact.

`internal/bridge/resourcecore.Adapter` remains the sole translation site. It gains two narrow write seams (`catalogWriter`, `resourceWriter`) that parallel the existing `catalogReader`/`resourceReader`, and two delegating methods. It calls `catalogo.Service.Create` and `recursos.Service.Create` — the already-built, production-hardened internal authority — and adds no conditional business decision of its own.

Caller attribution travels as an explicit public `Actor` field on every write request and is carried into the internal diagnostic seam as **request-scoped context metadata**, never as a business-logic parameter and never as persisted data. `internal/core/diagnostics.go` gains `WithActor`/`ActorFrom`/`NewDiagnosticRecord` and an `Actor` field on `DiagnosticRecord`; `core.Record` and `core.DiagnosticSink` keep their exact current signatures.

`internal/core.Map` is unchanged. All fifteen categories already exist and the nine Create-reachable ones already have correct precedence.

## Goals and non-goals

### Goals

- Make an external Go consumer able to create catalog records (all 11 kinds) and resources with no `internal` import.
- Fix the complete `Writer`/`WriteCapabilities` shape now: naming, request-DTO conventions, CAS argument placement, and return projection.
- Prove field completeness in the write direction, field by field, under the `GARFEX_STRICT` gate.
- Make every write attributable in diagnostics without inventing durable business data.
- Keep `internal/app/*`, `internal/postgres/*`, `internal/domain/*`, `cmd/garfex/`, and `internal/tui/` at zero changed lines.

### Non-goals

- Compiling, exporting, or stubbing `Update`, `Deactivate`, `Reactivate`, or `HardDelete`.
- Any resource hard-delete capability, now or ever.
- New error codes, new services, new repositories, new composition wiring, or a second translation site.
- Persisting `Actor`, authenticating it, or authorizing on it.

## Architecture and dependency direction

```text
external consumer
    |
    v
resourcecore.Writer            resourcecore.Reader
  (shape validation,             (unchanged)
   defensive copy,
   Actor required)
    |                                |
    v                                v
resourcecore.WriteCapabilities   resourcecore.ReadCapabilities
    ^                                ^
    |                                |
    +---- internal/bridge/resourcecore.Adapter ----+
              |                          |
              |  core.WithActor(ctx)     |
              v                          v
      internal/app/catalogo      internal/app/recursos
              |                          |
              +-----> internal/domain <--+
                          |
                          v
                   internal/postgres
```

The `Writer` is a translation and shape-validation boundary, not an authority. It must not validate business rules, resolve references, decide lifecycle, or construct identity. `resourcecore` imports no `internal` package.

## Public write contract

### Request DTO conventions (fixed now, for every eventual operation)

1. Every write request is a package-owned struct whose **first field is `Actor string`**, required and non-empty after `strings.TrimSpace`.
2. Request types are **per operation family**, never one struct with fields some operations ignore. A public field a given operation cannot honor is the `ResourceQuery.TypeCode` anti-precedent and is forbidden.
3. CAS lives on the request as `ExpectedRevision uint64` — never a positional argument — placed immediately after the identity field. Create carries no identity and no `ExpectedRevision`.
4. Requests reuse the existing public `Value`, `AttributeValue`, `ResourceScope`, and `ApplicabilityRule` types. No parallel write-only value model exists.
5. Every write returns the persisted public projection plus `error` (`CatalogRecord` or `Resource`), including `Revision`. The single exception is catalog `HardDelete`, which returns only `error` because no post-delete record exists.
6. Requests are deep-copied on entry; a caller mutating a request after the call changes nothing in flight.

### Compiled types (this change)

```go
package resourcecore

// CatalogWriteRequest creates one record of any registered catalog kind.
type CatalogWriteRequest struct {
    Actor  string
    Kind   KindCode
    Active bool
    Values map[string]Value
    Rules  []ApplicabilityRule // APLICABILIDAD aggregate; nil and empty differ
}

// ResourceWriteRequest creates one resource.
type ResourceWriteRequest struct {
    Actor       string
    Scope       ResourceScope
    NaturalUnit string
    Attributes  []AttributeValue
}

// WriteCapabilities is the Core-owned, service-shaped write seam. Only
// graduated operations appear here; a later per-operation change adds one
// method, which is an accepted per-slice seam change, not a consumer break.
type WriteCapabilities interface {
    CreateCatalog(context.Context, CatalogWriteRequest) (CatalogRecord, error)
    CreateResource(context.Context, ResourceWriteRequest) (Resource, error)
}

type Writer struct{ cap WriteCapabilities }

func NewWriter(cap WriteCapabilities) (*Writer, error) // nil -> INVALID_ARGUMENT

func (w *Writer) CreateCatalog(ctx context.Context, req CatalogWriteRequest) (CatalogRecord, error)
func (w *Writer) CreateResource(ctx context.Context, req ResourceWriteRequest) (Resource, error)
```

`resourcecore/copy.go` gains `CloneCatalogWriteRequest` and `CloneResourceWriteRequest`, built on the existing `CloneValue`/`CloneStringSlice` primitives and preserving nil-versus-empty for `Values` and `Rules`.

### The complete eventual shape (design record — NOT compiled here)

Recorded so later slices add methods without reshaping DTOs. None of the following exists in shipped code after this change.

| Eventual method | Eventual request type | Return |
| --- | --- | --- |
| `CreateCatalog` | `CatalogWriteRequest{Actor, Kind, Active, Values, Rules}` | `(CatalogRecord, error)` — **graduated here** |
| `UpdateCatalog` | `CatalogUpdateRequest{Actor, Kind, ID, ExpectedRevision, Active, Values, Rules}` | `(CatalogRecord, error)` |
| `DeactivateCatalog` | `CatalogLifecycleRequest{Actor, Kind, ID, ExpectedRevision}` | `(CatalogRecord, error)` |
| `ReactivateCatalog` | `CatalogLifecycleRequest{...}` | `(CatalogRecord, error)` |
| `HardDeleteCatalog` | `CatalogDeleteRequest{Actor, Kind, ID, ExpectedRevision}` | `error` only |
| `CreateResource` | `ResourceWriteRequest{Actor, Scope, NaturalUnit, Attributes}` | `(Resource, error)` — **graduated here** |
| `UpdateResource` | `ResourceUpdateRequest{Actor, ID, ExpectedRevision, Scope, NaturalUnit, Attributes}` | `(Resource, error)` |
| `DeactivateResource` | `ResourceLifecycleRequest{Actor, ID, ExpectedRevision}` | `(Resource, error)` |
| `ReactivateResource` | `ResourceLifecycleRequest{...}` | `(Resource, error)` |

There is no `HardDeleteResource` row and there never will be: `recursos.Service.Delete` is a compatibility alias for deactivation.

### Shape validation owned by `Writer` (never business validation)

`INVALID_ARGUMENT` before any capability call for: nil capability; blank `Actor`; unknown `Kind` (reuses the existing `isKnownKind`); nil/empty `Values`; a `Value` whose `Kind` is not a member of the tagged union; a canonical numeric string that does not parse; a non-zero `Reference.ID` (references are by natural code — `ID` is a read-only opaque projection); a non-empty `UnitCode` on a non-`QUANTITY` value; an empty `UnitCode` on a `QUANTITY` value; a blank `Scope.ClassCode`/`FamilyCode`/`TypeCode` or `NaturalUnit`; an `ApplicabilityRule.Equals` whose `Kind` is not `TEXT`; a `Mode` outside `REQUIRED|OPTIONAL|CONDITIONAL|FORBIDDEN`.

Everything else — reference existence, catalog validity, uniqueness, identity — belongs to the Core and is never pre-judged at the boundary.

## Actor attribution

**Decision: both, with strictly separated roles.** `Actor` is an explicit public request field (the contract), and the bridge converts it into request-scoped context metadata (the carriage). It is never a parameter of any domain or application function, and never persisted.

### Justification against `golang-context`

| Convention | How this design satisfies it |
| --- | --- |
| #9 — value keys must be unexported types | `type actorContextKey struct{}` is unexported in `internal/core`. |
| #10 — values carry only request-scoped metadata, never function parameters | `Actor` is caller-identity metadata for diagnostics. No domain or application function accepts it, reads it, or branches on it. Threading it as a parameter is precisely what #10 forbids. |
| #1 / #3 — propagate the same ctx; never store ctx in a struct | The bridge derives one child ctx per write call and passes it down; `Adapter` stores no ctx. |

Two decisive pieces of evidence rule out an explicit positional parameter on the diagnostic seam:

1. `core.Record` has **six existing call sites** — `internal/app/catalogo/service.go:375,380,390`, `internal/postgres/catalog_admin_repository.go:227,248`, and `internal/postgres/resource_repository_codec.go:115`. Adding a positional `actor` argument forces edits in `internal/app/catalogo` and `internal/postgres`, breaking the proposal's binding "Internal authority: zero changes".
2. Context carriage attributes diagnostics emitted **deeper than the bridge**. `catalogo.Service.prepareV2Write` and `classifyV2WriteError` already call `core.Record(ctx, ...)` with the caller's ctx; because the bridge tags that ctx, those records become attributed with no change to `catalogo` at all. A bridge-local parameter could never reach them.

### Exact changes in `internal/core/diagnostics.go`

```go
// NEW — unexported key type (golang-context #9).
type actorContextKey struct{}

// NEW. WithActor returns a child ctx carrying the caller-supplied actor for
// diagnostic attribution only. It is never business data and is never
// persisted. A blank actor returns ctx unchanged.
func WithActor(ctx context.Context, actor string) context.Context

// NEW. ActorFrom returns the actor carried by ctx, or "" when absent.
func ActorFrom(ctx context.Context) string

// NEW. NewDiagnosticRecord assembles the complete diagnostic shape, including
// the ctx-carried actor, so a sink cannot forget attribution.
func NewDiagnosticRecord(ctx context.Context, op Operation, kind string, id int64, cause error) DiagnosticRecord

// CHANGED — one added field.
type DiagnosticRecord struct {
    Op    Operation
    Kind  string
    ID    int64
    Actor string // NEW: caller-supplied, ctx-carried, never persisted
    Cause error
}

// UNCHANGED, deliberately — byte-identical signatures, zero call-site churn.
func Record(ctx context.Context, op Operation, kind string, id int64, cause error)
func MapWithDiagnostic(ctx context.Context, op Operation, err error) Error
type DiagnosticSink interface {
    Record(ctx context.Context, op Operation, kind string, id int64, cause error)
}
```

A sink resolves attribution through `NewDiagnosticRecord(ctx, ...)`, or `ActorFrom(ctx)` directly. `internal/core/errors_test.go:98` already hand-builds a `DiagnosticRecord`; it switches to the constructor in the same slice.

**Attribution scope, stated honestly:** `core.Record` ignores nil causes, so a *successful* create emits no diagnostic record. The invariant this change delivers is that **every write executes under an actor-tagged ctx**, so every diagnostic emitted anywhere under that ctx is attributed. Known gap: `internal/postgres` records with `context.Background()` (three sites above) and therefore loses the actor. Fixing that requires touching `internal/postgres`, which this change forbids; it is recorded as an INFO-level follow-up, not silently ignored.

## Bridge translation

### Seam definitions

```go
// internal/bridge/resourcecore/adapter.go
type catalogWriter interface {
    Create(ctx context.Context, kind domain.CatalogKindCode, rec domain.CatalogRecord) (domain.CatalogRecord, error)
}
type resourceWriter interface {
    Create(ctx context.Context, command domain.CreateCommand) (domain.Resource, error)
}

type catalogPort  interface { catalogReader;  catalogWriter  }
type resourcePort interface { resourceReader; resourceWriter }
```

`Adapter` stores `catalog catalogPort` and `resources resourcePort`. `NewAdapter`'s arity and its production call sites are unchanged; `*catalogo.Service` and `*recursos.Service` already satisfy the widened ports. Only in-repo test fakes gain one method each, adapted in the same slice per the compatibility invariant.

**`Create`, not `CreateV2`.** `catalogo.Service.Create` already routes to `insertLocked` (the V2 coherent-publication path) whenever `repoV2` is configured, which production is. Selecting `CreateV2` at the bridge would hard-fail every non-V2 composition, making the bridge an authority over composition policy — a `golang-hexagonal` BLOCKER. Composition, not translation, decides which repository backs the service.

### Catalog field-completeness (`CatalogWriteRequest` → `catalogo.Service.Create`)

| Public field | Internal destination | Verdict |
| --- | --- | --- |
| `Actor` | `core.WithActor(ctx, req.Actor)` — ctx only | Mapped (diagnostics); intentionally not business data |
| `Kind` | `Create`'s `kind` argument; the service assigns `rec.Kind` | Mapped |
| `Active` | `domain.CatalogRecord.Active` | Mapped verbatim; the domain candidate decides validity |
| `Values` | `domain.CatalogRecord.Values map[string]CatalogValue` | Mapped per descriptor field kind (below) |
| `Rules` | `domain.CatalogRecord.Rules []CatalogRuleRecord` | Mapped; complete-replacement, nil ≠ empty |
| *(absent)* `ID` | assigned by persistence | Intentionally not on a create request |
| *(absent)* `Revision` | assigned by persistence (starts at 1) | Intentionally not on a create request |

`domain.CatalogRecord` has exactly six fields — `Kind`, `ID`, `Revision`, `Active`, `Values`, `Rules`. Every one is either mapped or listed above with a rationale. No field is silently dropped.

**Value sub-mapping** — the exact inverse of the existing `Adapter.mapCatalogValue`, directed by `a.fieldKind(kind, name)`:

| Descriptor `FieldKind` | Accepted `Value.Kind` | `domain.CatalogValue` destination |
| --- | --- | --- |
| `FieldText` | `TEXT` | `Text` |
| `FieldCode` | `CODE` | `Text` |
| `FieldBool` | `BOOLEAN` | `Bool` |
| `FieldInt` | `INTEGER` | `Int` (`strconv.Atoi` of the canonical string; overflow → `INVALID_ARGUMENT`) |
| `FieldStringList` | `STRING_LIST` | `List` (cloned; nil ≠ empty) |
| `FieldRef` | `REFERENCE` | `Ref.Kind`, `Ref.Code` |
| `FieldEnum` | `ENUM` | `Text` |

Any other pairing is `INVALID_ARGUMENT`; nothing is coerced. `domain.CatalogRef.Label` receives no public source **by design** — it is the repository-resolved presentation label, not the mutation key, and the read direction never projects it either. `Value.UnitCode`, `Value.Reference.ID`, and unused union payloads must be zero, else `INVALID_ARGUMENT`.

**Rule sub-mapping** — all six public fields mapped, none dropped:

| `ApplicabilityRule` | `domain.CatalogRuleRecord` |
| --- | --- |
| `AttributeCode` | `When.AttributeCode` |
| `Equals` (must be `TEXT`) | `When.Equals` (string) |
| `Mode` | `Mode` (`domain.AttributeMode`) |
| `IdentityParticipates` | `IdentityParticipates` |
| `NotApplicable` | `NotApplicable` |
| `Active` | `Active` |

### Resource field-completeness (`ResourceWriteRequest` → `recursos.Service.Create`)

`domain.CreateCommand` has exactly three fields; all three are mapped.

| Public field | Internal destination | Verdict |
| --- | --- | --- |
| `Actor` | `core.WithActor(ctx, req.Actor)` — ctx only | Mapped (diagnostics) |
| `Scope.ClassCode` / `FamilyCode` / `TypeCode` | `CreateCommand.Scope` (all three) | Mapped |
| `NaturalUnit` | `CreateCommand.NaturalUnit` | Mapped |
| `Attributes` | `CreateCommand.Attributes []ResourceAttributeValue` | Mapped per union kind (below) |

**Attribute sub-mapping** — the exact inverse of the existing `mapResourceAttributeValue`:

| `Value.Kind` | `UnitCode` | `domain.ResourceAttributeValue` |
| --- | --- | --- |
| `CONTROLLED_OPTION` | must be empty | `Type: ValueTypeControlledOption, OptionCode: Text` |
| `INTEGER` | must be empty | `Type: ValueTypeInteger, Integer: &parsed` |
| `DECIMAL` | must be empty | `Type: ValueTypeDecimal, Decimal: &parsed` |
| `QUANTITY` | **required** | `Type: ValueTypeQuantity, Quantity: &{Value: parsed, UnitCode}` |
| `BOOLEAN` | must be empty | `Type: ValueTypeBoolean, Boolean: &Bool` |
| `NOT_APPLICABLE` | must be empty | `Type: ValueTypeControlledText, Text: domain.NotApplicableText` |
| `TEXT` | must be empty | `Type: ValueTypeControlledText, Text: Text` |
| `CODE`, `ENUM`, `STRING_LIST`, `REFERENCE` | — | **No resource-attribute counterpart → `INVALID_ARGUMENT`**, never coerced |

Decimal text is parsed with `shopspring/decimal` inside the bridge; `resourcecore` still exposes no decimal dependency and never converts through `float64`.

**Round-trip obligation:** for every graduated `Value.Kind`, `mapResourceAttributeValue(toDomainAttribute(v)) == v`, and the catalog analogue through `mapCatalogValue`. A table-driven inverse test is the field-completeness proof, not a comment.

### The create-confirm read (resource only)

`internal/postgres.resourceRepository.Create(ctx, resource) error` (`resource_repository_crud.go:22`) returns no identity and no revision, so `recursos.Service.Create` returns a canonically-built `domain.Resource` with `ID == 0` and `Revision == 0`; only `IdentityKey` is real. Returning those zeros publicly would be accept-and-ignore on the result side.

`Adapter.CreateResource` therefore performs one **create-confirm read** through the seam it already holds: after a successful `Create`, it calls `resources.Get(ctx, req.Scope.ClassCode, created.IdentityKey)` and projects that persisted resource. `identity-v1` is canonical and unique, so the read is exact.

Confirm-read errors pass through the ordinary `mapError` unchanged — **no reclassification.** Turning a confirm-read `NOT_FOUND` into `INTEGRITY` would make the bridge a classification authority over an outcome the Core already classified, which is a `golang-hexagonal` BLOCKER. This is also the concrete, provable source of Create-reachable `NOT_FOUND` for resources. The read is non-transactional; under the approved one-writer topology no other writer can intervene, and that assumption is documented in the readiness record.

The catalog path needs no confirm read: `catalogAdminRepositoryV2.Insert` returns a `CatalogWriteResult` whose `Record` carries the real `ID` and `Revision` (`internal/postgres/catalog_admin_repository_v2.go:159-161`), and `insertLocked` returns it. Under a composition where `repoV2` is nil, `Create` falls back to the legacy path and `Revision` is 0; that is a composition precondition recorded in the readiness record, **not** a bridge conditional.

## Error mapping — `internal/core.Map` needs no change

All fifteen categories exist with correct precedence (`internal/core/errors.go:60-103`). The nine Create-reachable categories map through existing arms:

| Public code | Create-reachable internal source | Existing `Map` arm |
| --- | --- | --- |
| `INVALID_ARGUMENT` | `Writer` shape gate; `core.ErrInvalidArgument` (aliased by `catalogo.ErrInvalidArgument`) | `errors.Is(err, ErrInvalidArgument)` |
| `NOT_FOUND` | resource create-confirm read `domain.ErrResourceNotFound`; catalog `domain.ErrCatalogRecordNotFound` | `ErrCatalogRecordNotFound`, `ErrResourceNotFound` |
| `DUPLICATE` | `domain.ErrCatalogDuplicate`, `domain.ErrDuplicateResource` | dedicated arm |
| `INVALID_REFERENCE` | `domain.ErrCatalogReference`, `domain.ErrResourceReference` | dedicated arm |
| `VALIDATION` | `domain.ErrResourceValidation` from `domain.NewResource` and mutation input | dedicated arm |
| `INTEGRITY` | `domain.ErrResourceIntegrity` from `recursos.Service.Create` | dedicated arm |
| `INVALID_CATALOG` | `domain.WrapInvalidCatalog(next.Validate())` in `insertLocked` | dedicated arm, ahead of validation |
| `UNAVAILABLE` | `catalogo.ErrCatalogWriterUnavailable` and `ErrCatalogAdminRepositoryV2Unavailable` (both wrap `core.ErrUnavailable`); ctx cancel/deadline | `ErrUnavailable`, `context.Canceled`, `context.DeadlineExceeded` |
| `INTERNAL` | any unclassified failure | `default` |

The six categories unreachable from Create — `IDENTITY_CONFLICT`, `INVALID_LIFECYCLE`, `REACTIVATION_IMPOSSIBLE`, `CONFLICT`, `IN_USE`, `IMMUTABLE_CODE` — stay unproven and are named as such in the readiness record. The proposal lists five; `INVALID_LIFECYCLE` is the sixth and is likewise unreachable from Create.

Evidence shape: per-entry-point table-driven tests inject each sentinel family into the `catalogWriter`/`resourceWriter` seam and assert the exact public code, distinctness from the other eight, and no leakage (`Error()`, `%v`, `%+v`, concrete type, `errors.Unwrap`, recursive chain) of pgx, `PgError`, SQLSTATE, constraints, tables, columns, or server text. This mirrors the archived mapper-injection evidence.

## Files and change surfaces

| Surface | Action | Description |
| --- | --- | --- |
| `resourcecore/write_types.go` | Create | `CatalogWriteRequest`, `ResourceWriteRequest`. |
| `resourcecore/writer.go` | Create | `WriteCapabilities`, `Writer`, `NewWriter`, shape validation. |
| `resourcecore/copy.go` | Modify | `CloneCatalogWriteRequest`, `CloneResourceWriteRequest`. |
| `resourcecore/writer_test.go` | Create | Shape gate, nil capability, defensive copy, no ungraduated symbol. |
| `resourcecore/external_test.go` | Modify | External consumer creates both, using only public types. |
| `internal/bridge/resourcecore/adapter.go` | Modify | `catalogWriter`/`resourceWriter` seams, `CreateCatalog`, `CreateResource`, inverse value mappers, create-confirm read. |
| `internal/bridge/resourcecore/adapter_test.go` | Modify | Fakes gain `Create`; field-completeness, inverse round-trip, and 9-category tables. |
| `internal/core/diagnostics.go` | Modify | `WithActor`, `ActorFrom`, `NewDiagnosticRecord`, `DiagnosticRecord.Actor`. |
| `internal/core/errors_test.go` | Modify | Spy sink builds through `NewDiagnosticRecord`; actor attribution assertion. |
| `internal/core/errors.go` | Unchanged | All fifteen categories and their precedence are already correct. |
| `internal/app/*`, `internal/domain/*`, `internal/postgres/*` | Unchanged | Write authority already built and wired. |
| `cmd/garfex/`, `internal/tui/` | Unchanged | Zero `resourcecore` references before and after. |
| `openspec/changes/resource-master-core-write/readiness.md` | Create | Create readiness record and unavailable-operation list. |

## Compatibility invariant

> After every slice, `go test ./... -count=1` builds every current adapter, service, composition root, TUI call site, test double, and external test. A slice never adds a public method for an ungraduated operation, never adds a public request field an operation ignores, never changes `core.Record`'s or `DiagnosticSink`'s signature, never changes `internal/app`, `internal/domain`, `internal/postgres`, `cmd/garfex`, or `internal/tui`, and never widens a bridge seam without adapting every in-repo fake in that same slice. The READ contract stays byte-compatible: no read DTO, method, or error identity changes. When work cannot fit in 400 changed lines, it is split into another additive slice rather than weakening these rules.

## Slice-transition proof

| Unit | Start state | End state and compatibility proof | Required focused evidence | Forecast |
| --- | --- | --- | --- | --- |
| **W1 — public write contract** | READ-only `resourcecore` | Add write DTOs, defensive copies, `WriteCapabilities`, `Writer`, `NewWriter`, shape validation. No bridge change; nothing implements the seam in production yet, so no composition is affected. `Reader` untouched. | `go test ./resourcecore -run 'TestWriter\|TestExternalWrite\|TestWriteRequestCopy'`; nil-capability `INVALID_ARGUMENT`; caller-mutation-after-call table; assertion that no ungraduated method name is exported | ≤400 |
| **W2 — catalog Create bridge + actor seam** | W1 | Add `core.WithActor`/`ActorFrom`/`NewDiagnosticRecord`/`DiagnosticRecord.Actor` (all additive; the six existing `core.Record` call sites are untouched). Widen `catalogPort`, add `Adapter.CreateCatalog` and the inverse catalog value/rule mappers, adapt bridge fakes in-slice. | `go test ./internal/core ./internal/bridge/resourcecore -run 'TestWriteBridge\|TestActor\|TestCatalogCreate'`; field-by-field completeness table against `domain.CatalogRecord`; `mapCatalogValue` inverse round trip for all 7 field kinds; 9-category injected table for catalog; leakage assertions | ≤400 |
| **W3 — resource Create bridge + readiness** | W2 | Widen `resourcePort`, add `Adapter.CreateResource` with the create-confirm read and the inverse attribute mapper. Add the readiness record and documentation. Full verification. | `go test ./... -run 'TestWriteBridge\|TestResourceCreate\|TestExternalWrite'` plus the full suite; completeness table against `domain.CreateCommand`; inverse round trip for the 7 graduated value kinds and rejection of the 4 unmappable ones; 9-category injected table for resource; `ID>0` / `Revision>=1` / `identity-v1` assertions; zero-reference scan of `cmd/garfex` and `internal/tui` | ≤400 |

Review-guard forecast: **Decision needed before apply: No** (the project's configured `auto-chain` already covers it). **Chained PRs recommended: Yes.** **400-line budget risk: High** — the READ-ONLY precedent cost 1,348 lines for 7 methods.

Each unit follows strict red-green-refactor with RED, GREEN, TRIANGULATE, and REFACTOR evidence, and leaves the tree green. Focused tests run first, then `gofmt -l .`, `go vet ./...`, `golangci-lint run ./...`, `go test ./... -count=1`.

Per the `GARFEX_STRICT` focused-test discoverability gate, every new test function name contains a literal alternative from its unit's `-run` pattern as a substring, and each focused command must select at least one test in every package the unit touched.

## Invariants and failure modes

| Invariant / failure | Required behavior |
| --- | --- |
| Caller mutates a request after the call | Nothing in flight or persisted changes. |
| Blank or whitespace `Actor` | `INVALID_ARGUMENT`; no capability call. |
| `Actor` supplied | Every downstream diagnostic under that ctx is attributed; nothing is persisted; nothing is exposed publicly. |
| `Value.Kind` mismatches the descriptor field kind | `INVALID_ARGUMENT`; never coerced. |
| `Value.Kind` has no internal counterpart (`CODE`/`ENUM`/`STRING_LIST`/`REFERENCE` on a resource attribute) | `INVALID_ARGUMENT`; never silently dropped. |
| Public field the domain cannot honor | `MISSING DOMAIN CRITERION` BLOCKER: drop the field from the contract; never accept-and-ignore. |
| Non-zero `Reference.ID` on a write | `INVALID_ARGUMENT`; references are by natural code. |
| `Rules == nil` for `APLICABILIDAD` | Aggregate omitted; the domain rejects it. Non-nil empty means an explicitly rule-free aggregate. |
| Create succeeds | Returns the persisted projection with `ID > 0`, `Revision >= 1`, and, for resources, durable `identity-v1`. |
| Create-confirm read fails | Ordinary `mapError` category; no reclassification, no retry, no partial fabrication. |
| Any internal error | One of the 9 proven categories; no pgx/SQLSTATE/constraint/table/column/server text in string, type, or unwrap chain; no public `Unwrap`. |
| `catalogo` writer latched unavailable | `UNAVAILABLE`; nothing published; operator reload required. |
| Bridge encounters a business decision | BLOCKER — move it to the Core. |
| Ungraduated operation | Not exported, not stubbed, not discoverable at runtime. |

## Threat matrix

N/A — no routing, shell command, subprocess, VCS/PR automation, executable-file classification, or process-integration boundary. This change adds in-process Go types and one in-process translation site.

## Migration / rollout

No migration. No schema change, no new column, no composition wiring, no new binary behavior. `Actor` is never persisted. Rolling back any unit reverts Go source only; the READ contract and every database artifact are untouched. Withdrawing Create later removes only `CreateCatalog`/`CreateResource`.

## Alternatives rejected

| Alternative | Why rejected |
| --- | --- |
| Compile all five write operations, returning "unimplemented" | Makes an unsafe operation discoverable and creates an abstraction for a hypothetical future; both are `golang-hexagonal` violations and a binding proposal decision. |
| One `CatalogWriteRequest` covering create and update, with `ID`/`ExpectedRevision` that Create ignores | Reproduces the `ResourceQuery.TypeCode` accept-and-ignore precedent the strict profile forbids. |
| `Actor` as a positional argument on `core.Record` | Forces edits in `internal/app/catalogo` and `internal/postgres`, breaking the binding zero-change decision, and makes request metadata a business-function parameter (`golang-context` #10). |
| `Actor` in ctx only, with no public field | The public contract could not require attribution, and consumers would need a second exported context-helper API surface. |
| Persist `Actor` with the record | Requires a migration and turns caller identity into durable business data the domain never modeled; explicitly out of scope. |
| Bind the seam to `catalogo.Service.CreateV2` | Hard-fails every non-V2 composition, making the bridge an authority over composition policy. |
| Reclassify a failed create-confirm read as `INTEGRITY` | Makes the bridge a classification authority over an outcome the Core already classified. |
| Skip the create-confirm read and return `ID: 0`, `Revision: 0` | Accept-and-ignore on the result side; violates the proposal's persisted-revision success criterion. |
| Change `internal/postgres.resourceRepository.Create` to return the identity | Touches internal authority, which the proposal forbids; the confirm read achieves the same public contract with zero internal change. |
| A second public write package or a public repository port | A second translation site; duplicates ports and invites application-service bypass. |

## Open questions

- [ ] `CatalogWriteRequest.Active` maps verbatim to `domain.CatalogRecord.Active`. Unit W2's RED test must prove `Active: false` round-trips. If the domain forces an active create, the field is a `MISSING DOMAIN CRITERION` and must be dropped from the public request rather than silently ignored.
- [ ] Catalog-side `NOT_FOUND` reachability from Create is proven by seam injection. If no end-to-end path exists, the readiness record must name it not-reachable-for-catalog with a reviewed operation-specific reason (permitted by the archived gate) rather than letting it fall through to `INTERNAL`.
