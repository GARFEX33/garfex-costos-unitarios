# Proposal: Expose public WRITE HardDeleteCatalog on the Resource Master Core

## Intent

Fourth and final per-operation WRITE graduation on `resourcecore`, after
`Create` (archived `2026-08-21-resource-master-core-write`), `Update`
(archived `2026-08-20-resource-master-core-write-update`), and
`Deactivate`/`Reactivate` (archived
`2026-08-21-resource-master-core-write-lifecycle`). External consumers (PI)
can currently create, edit, and toggle catalog records, but cannot remove a
mistaken or obsolete catalog entry — the only remedy is deactivation, which
leaves permanent clutter in every catalog projection. The destructive
operation was deliberately sequenced last.

**Scope boundary, stated first-class**: this change graduates
`HardDeleteCatalog` **only**. `HardDeleteResource` is explicitly out of scope
and is a deliberate, documented gap — not an oversight. The original
stabilization design named resource hard delete as future work
(`openspec/changes/archive/2026-08-20-stabilize-resource-master-core/design.md:334`,
"any future resource hard delete"), and today no domain interface method, no
repository implementation, and no guard-chain design exists for it
(`internal/domain/resource_repository_v2.go:25-44` declares only
Deactivate/Reactivate/Update; `internal/app/recursos/service.go:153-161`
documents `Delete` as a compatibility alias that never physically removes).
Completing this change finishes the **catalog** write contract — not the full
`resourcecore` write contract.

Internal authority already exists and is production-wired
(`internal/app/catalogo/service.go:607-630`, CAS-aware, reached from
`cmd/garfex/main.go:66`). This change is the public mirror plus its sole
bridge translation. No internal behavior changes.

## Scope

### In Scope
- `resourcecore.Writer.HardDeleteCatalog` + `WriteCapabilities` grows from 8
  to 9 methods.
- Reuse `CatalogLifecycleRequest` (`resourcecore/write_types.go:47-52`;
  Actor/Kind/ID/ExpectedRevision) **as-is** — no new request type. Strict CAS:
  `validateCatalogLifecycleRequest` already requires a non-zero
  `ExpectedRevision`, and deletion correctly has no idempotent-no-op bypass.
- Bridge: extend the `catalogWriter` interface
  (`internal/bridge/resourcecore/adapter.go:29-34`) with a
  `HardDeleteRevision`-shaped method and add `Adapter.HardDeleteCatalog`.
  This is the single missing link on the catalog side.
- Update the exhaustive reflection-based compiled-surface assertion
  `TestWriter_NoUngraduatedMethodExported`
  (`resourcecore/writer_test.go:211-233`), which today forbids any
  `HardDelete*` symbol and will fail-fast the moment the method lands.
- First "reachable error category coverage" proof for `INVALID_LIFECYCLE`
  (active target) and `IN_USE` (non-zero `Dependents` /
  `ReferencedByResources`), following the exact scenario pattern of the three
  prior graduations.

### Out of Scope
- **`HardDeleteResource`** — deliberate, evidenced deferral (see Intent).
  Needs its own exploration: "in use" has no defined meaning for a resource
  today.
- `internal/app/catalogo/service.go` and
  `internal/postgres/catalog_admin_repository_v2.go` — already complete.
- Any `cmd/garfex/` or `internal/tui/` change (zero changed lines).
- Any read-contract change, including post-delete visibility semantics.
- New `ErrorCode` values — all 15 already exist and both categories are
  already mapped in `internal/core/errors.go:80-85`.

## Capabilities

### New Capabilities
None.

### Modified Capabilities
- `resource-master-core`: **MODIFY** "Compiled surface extended to Create,
  Update, Deactivate, and Reactivate" (`spec.md:581-589`), which currently
  states "No method for `HardDelete` MUST be exported, stubbed, or otherwise
  discoverable" — this becomes a 9-method surface with `HardDeleteCatalog`
  present and `HardDeleteResource` still forbidden. **ADD** requirements
  mirroring the Deactivate/Reactivate section shape: consumer-neutral public
  `HardDeleteCatalog`, field completeness through the sole bridge, actor
  attribution without persistence, and HardDelete-reachable error category
  coverage.

## Approach

Mirror the three prior graduations: package-owned DTO reused unchanged,
`Writer` does shape validation only (no business rules), sole-bridge
translation, `Actor` carried via `core.WithActor`/`ActorFrom` — never
persisted, never validated. Every guardrail already lives at the service
layer (`buildV2DeleteCandidate`, `internal/app/catalogo/service.go:446-483`):
reject active target → `ErrInvalidLifecycle`; reject non-zero `Dependents` or
any `ReferencedByResources` hit → `ErrCatalogInUse`. The bridge translates,
it does not re-implement.

**Open design decision, deferred to `sdd-design` (do NOT resolve here)**:
`HardDeleteRevision` and `domain.CatalogAdminRepositoryV2.Delete` return
**only `error`** — the interface doc states the record is always nil — while
all 7 existing `Writer` methods return `(Record, error)`. The public shape of
`HardDeleteCatalog` (error-only, breaking the pattern; or a record-returning
shape requiring a pre-delete read) is a genuine design tradeoff, not a
mechanical copy. The lifecycle change's catalog confirm-read is **not** a
usable precedent here: after a delete there is nothing left to read back.

## Affected Areas

| Area | Impact | Description |
|------|--------|-------------|
| `resourcecore/writer.go` | Modified | `HardDeleteCatalog`; `WriteCapabilities` 8 → 9 methods |
| `resourcecore/write_types.go` | Unchanged | `CatalogLifecycleRequest` reused as-is |
| `internal/bridge/resourcecore/adapter.go` | Modified | `catalogWriter` gains a HardDelete method; `Adapter.HardDeleteCatalog` |
| `resourcecore/writer_test.go` | Modified | Exhaustive method-list assertion updated (currently rejects `HardDelete*`) |
| `openspec/specs/resource-master-core/spec.md` | Modified | 1 MODIFIED + ~4 ADDED requirements |
| `internal/app/`, `internal/postgres/`, `cmd/garfex/`, `internal/tui/` | Unchanged | Zero changed lines |
| Tests | New | First `INVALID_LIFECYCLE` / `IN_USE` reachability suite |

## Risks

| Risk | Likelihood | Mitigation |
|------|------------|------------|
| Return-shape decision (error-only vs. `(Record, error)`) is made mechanically and breaks consumer expectations either way | High | Explicitly flagged as an unresolved `sdd-design` decision; not pre-committed in this proposal |
| Change name `...-write-harddelete` reads as full catalog+resource parity | High | Scope boundary stated first-class in Intent, Scope, and carried into the spec delta |
| `TestWriter_NoUngraduatedMethodExported` breaks the build the moment the method lands | Certain | Explicitly in scope; updated in the same PR slice as the method |
| Consumers read "write contract complete" as including resource hard delete | Med | Proposal and spec both state catalog-only completion with the deferral cited |
| Irreversible operation reaches production with untested guards | Low | First-ever reachability proof for `INVALID_LIFECYCLE`/`IN_USE` is a required deliverable, not optional |
| Slice exceeds the 400-line review budget | Low | Smallest of the four graduations: one method, one bridge method, one reused DTO; `sdd-tasks` forecasts explicitly |

## Rollback Plan

Revert `HardDeleteCatalog`, its bridge method, the `catalogWriter` interface
line, and the `writer_test.go` assertion update; restore the spec requirement
to its 8-method form. Create, Update, Deactivate, Reactivate, and Read stay
unaffected. No persisted data, migration, or `internal/app`/`internal/domain`
behavior is touched by either the change or its rollback — the underlying
`HardDeleteRevision` remains reachable internally exactly as it is today.

## Dependencies

- `resource-master-core-write` (Create), archived `2026-08-21`.
- `resource-master-core-write-update` (Update), archived `2026-08-20`.
- `resource-master-core-write-lifecycle` (Deactivate/Reactivate), archived
  `2026-08-21`.

## Success Criteria

- [ ] An external consumer hard-deletes a catalog revision via `resourcecore`
      with `ExpectedRevision` CAS and no `internal` import.
- [ ] `WriteCapabilities` compiles exactly 9 methods: the prior 8 plus
      `HardDeleteCatalog` — and **no** `HardDeleteResource` stub or export.
- [ ] The return-shape decision is recorded with rationale in `design.md`.
- [ ] `INVALID_LIFECYCLE` and `IN_USE` are each proven reachable through the
      public surface for the first time.
- [ ] Deleting an active catalog revision fails; deleting one with dependents
      or resource references fails; neither leaves partial state.
- [ ] A stale `ExpectedRevision` yields `CONFLICT`, and no PostgreSQL detail
      leaks through any error string or type.
- [ ] The spec states the catalog-only boundary and names
      `HardDeleteResource` as a deliberate gap.
- [ ] `cmd/garfex/`, `internal/tui/`, `internal/app/`, `internal/postgres/`
      have zero changed lines.

## Proposal question round

These are product decisions this proposal had to assume to stay unblocked.
Please confirm, correct, or ask for a second round.

1. **Is deletion the right product remedy at all?** Deactivation already
   removes a catalog kind from active use. What operational pain justifies
   physical deletion — audit noise, uniqueness collisions on a name being
   reused, or regulatory removal? **Assumption used**: cleanup of
   mistakenly-created, never-used entries; the existing guard chain (inactive
   + zero dependents + zero resource references) already encodes exactly that
   narrow intent and is reused unchanged.
2. **Who may hard-delete?** `Actor` is currently optional, unauthenticated,
   and never persisted. For an irreversible operation, should this
   graduation require a non-empty `Actor`, or does the existing precedent
   hold? **Assumption used**: unchanged precedent — optional and
   diagnostics-only. Introducing an authorization concept here would exceed
   a bridge-only graduation's boundary.
3. **Should a successful delete return anything?** The internal path returns
   only `error` and cannot produce a post-delete record. Do consumers need a
   snapshot of what was deleted (for their own audit trail), or is a bare
   success sufficient? **Assumption used**: flagged as an unresolved
   `sdd-design` decision, deliberately not pre-committed here.
4. **Is the asymmetric public surface acceptable to ship?** After this
   change, catalog supports 5 write operations and resource supports 4.
   **Assumption used**: yes — accepted and documented as a deliberate gap,
   because no product need for physical resource deletion has been stated
   and no guard semantics exist for it.
5. **Audit expectations.** Should a hard delete leave any durable trace
   (event, tombstone, log record) outside the deleted row? **Assumption
   used**: no new persistence — out of scope; the operation is a public
   mirror of already-shipped internal behavior, which writes no tombstone
   today.

If these assumptions hold, no changes to this proposal are needed and it is
ready for `sdd-spec`/`sdd-design`.

## User confirmation (2026-08-21)

All five assumptions above are confirmed as stated, with these clarifications:

1. Physical delete stays scoped to cleanup of mistakenly-created, never-used
   entries — not an audit mechanism, not a workaround for name/code reuse.
2. No new authorization model. `Actor` keeps its exact existing semantics
   (optional, unauthenticated, never persisted).
3. `sdd-design` decides whether the public contract mirrors the internal
   error-only return directly or needs a consistent public shape — without
   inventing data the Core does not produce.
4. Asymmetry is explicit and accepted: Catalog reaches 5 write operations,
   Resource stays at 4. No symmetry requirement exists between the two
   contracts. `HardDeleteResource` is confirmed out of scope.
5. No new audit trail, tombstone, or log record. This change graduates an
   existing capability; it does not redefine its semantics.

**Sixth guardrail added by the user, to carry into the spec delta**:

> The public exposure of `HardDeleteCatalog` MUST NOT weaken, duplicate, or
> bypass the internal service's existing guard chain. The bridge MUST
> delegate to the Core's current authority rather than re-implementing any
> hard-delete rule.

Rationale: prevents a second, drifting implementation of hard-delete rules
from accidentally appearing once the capability goes public. `sdd-spec` must
express this as an explicit requirement/scenario, not leave it implicit in
the Approach section's "the bridge translates, it does not re-implement"
language.
