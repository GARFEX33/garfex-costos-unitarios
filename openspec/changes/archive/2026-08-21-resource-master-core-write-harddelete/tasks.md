# Implementation Tasks: Expose Public WRITE HardDeleteCatalog on the Resource Master Core

This is the strictly ordered auto-chain for the fourth and final per-operation
public WRITE graduation on `resourcecore`, after Create (archived
`2026-08-21-resource-master-core-write`), Update (archived
`2026-08-20-resource-master-core-write-update`), and Deactivate/Reactivate
(archived `2026-08-21-resource-master-core-write-lifecycle`). Per-operation
graduation is inherited and binding from the archived spec's "Separate read
and per-operation write readiness" requirement and is not reopened here.
`HardDeleteCatalog` is the only operation in scope; `HardDeleteResource`
remains a deliberate, documented gap — no unit in this plan compiles,
exports, or stubs it. This graduation is smaller than the lifecycle change:
one method, one bridge method, one reused, unchanged DTO — two source units
plus readiness, not four.

## Review Workload Forecast

| Field | Value |
| ------- | ------- |
| Estimated changed lines | ~380–560 total (1 `Writer` method + 1 `WriteCapabilities` line + doc comment in U1; 1 seam method + 1 `Adapter` method + an 8-category injected table + a non-duplication proof + a stale-CAS proof in U2; a documentation-only readiness record in U3) — the smallest of the four graduations, per proposal.md's Risks table |
| 400-line budget risk | Medium (per-unit stays well under budget; the whole-change total may approach it) |
| Chained PRs recommended | Yes |
| Suggested split | U1 → U2 → U3 |
| Delivery strategy | auto-chain |
| Chain strategy | stacked-to-main |

Decision needed before apply: No
Chained PRs recommended: Yes
Chain strategy: stacked-to-main
400-line budget risk: Medium

Each unit is one stacked-to-main review boundary with its tests, focused
receipt, rollback boundary, and full-suite proof. No unit may exceed its
forecast below. If measured work exceeds its forecast, split that unit along
design.md's own file/table granularity; do not create a broad exception and
do not improvise a new boundary.

### Per-slice forecast

| Slice | Forecast |
| ------- | ---------- |
| U1 | ≤200 changed lines |
| U2 | ≤300 changed lines |
| U3 | ≤120 changed lines |

Every unchecked implementation task below maps one-to-one to this forecast
table.

## Global execution and compatibility contract

- Use strict RED → GREEN → TRIANGULATE → REFACTOR evidence for every unit
  that touches a package with tests. RED must be captured before the
  production behavior is implemented. U3 adds no new Go test names; its
  evidence is documentation-review plus full-suite/lint/vet re-verification.
- For every slice, run the unit's exact focused `-run` command first, then
  the mandatory boundary command `go test ./... -count=1`; the latter is
  both the compile-safety proof for every current adapter, service,
  composition root, TUI call site, test double, and external test, and the
  full-suite proof. Record the exact command and result in the unit receipt.
- Per the `GARFEX_STRICT` focused-test discoverability gate
  (`.agents/skills/golang-hexagonal/references/garfex-strict-profile.md`):
  run the exact stated `-run` command, confirm it selects at least one test
  in every package the unit touched, and name every new test function so at
  least one literal alternative from that unit's `-run` pattern appears as a
  substring of the function name after `Test`.
- `resourcecore.NewWriter` has no production call site today — no
  composition root exists that satisfies `WriteCapabilities` with the
  bridge `Adapter`. Widening the interface in U1 therefore does not require
  U2 to land in the same unit for the codebase to keep compiling; the two
  units are genuinely separable, confirmed by direct grep, mirroring the
  lifecycle precedent's identical U1/U2 independence.
- Per the field-completeness gate: diff `CatalogLifecycleRequest`'s `Kind`,
  `ID`, `ExpectedRevision`, `Actor` against `HardDeleteRevision`'s
  destination arguments; an omission needs either a mapping or a one-line
  rationale comment at the omission site, never silence.
- Never widen `catalogWriter` without adapting every in-repo fake in the
  same slice.
- Never change `core.Record`'s, `core.WithActor`'s, or `DiagnosticSink`'s
  existing signature. `Actor` travels only via `core.WithActor(ctx, ...)` /
  `core.ActorFrom(ctx)`, reusing the mechanism built for Create and extended
  for Update, Deactivate, and Reactivate; never persisted; never given a
  new column or migration; never exposed on a public DTO.
- `CatalogLifecycleRequest` is reused with **zero struct change** — only its
  doc comment widens. No new request type, no `Values`/`Rules`/`Scope`/
  `Attributes` field is added.
- `ExpectedRevision == 0` and `ID == 0` are both `INVALID_ARGUMENT` at the
  `Writer` shape gate — CAS is mandatory. Unlike Deactivate/Reactivate,
  `HardDeleteCatalog` MUST NOT apply any idempotent no-op bypass: a stale
  `ExpectedRevision` always yields `CONFLICT`, proven by U2's dedicated test.
  There is no "already deleted" target to short-circuit to.
- The bridge issues **no confirm-read** after a successful delete — the
  deleted record no longer exists to read back. This is the operative
  contrast with the catalog side of the lifecycle precedent.
- The bridge MUST NOT re-implement any guard the internal service already
  owns (`buildV2DeleteCandidate`: active-target rejection, dependents
  rejection, resource-reference rejection). This is proven structurally by
  capability starvation (`catalogReader` exposes no `Dependents`/
  `ReferencedByResources`) and falsifiably by U2's non-duplication test.
- `internal/app/*`, `internal/domain/*`, `internal/postgres/*`,
  `cmd/garfex/`, and `internal/tui/` stay at zero changed lines for the
  entire chain. `internal/core/errors.go`'s `Map` is unchanged —
  `InvalidLifecycle`/`InUse` already exist and are already correctly
  ordered.
- No unit compiles, exports, or stubs `HardDeleteResource` as a symbol, an
  "unimplemented" branch, an unused request field, or any other
  runtime-discoverable artifact.
- The READ contract and the compiled Create/Update/Deactivate/Reactivate
  WRITE surface stay byte-compatible: no read DTO, method, or error
  identity changes in any unit; no prior method, field, or error identity
  changes in any unit.
- Inspect `git diff --name-only` and `git diff --stat` before and after
  every unit. Preserve unrelated user changes and preserve every other
  in-flight `openspec/changes/*` directory without edits.
- Do not run local `go build`, `docker build`, or blanket database/volume
  cleanup. U1–U2 use in-repo fakes for `catalogWriter`, not a live
  database; live-PostgreSQL integration evidence is a disclosed gap in a
  sandboxed apply session, matching the three prior graduations' precedent.
- When work cannot fit in a unit's forecast, split into another additive
  unit along design.md's own file/table granularity rather than weakening
  these rules.

## Ordered implementation units

### U1 — Public HardDeleteCatalog contract

- [x] Add `Writer.HardDeleteCatalog(ctx, CatalogLifecycleRequest) error` and
  the `WriteCapabilities` 9th method to `resourcecore/writer.go`, reusing
  `validateCatalogLifecycleRequest` unchanged; widen the doc comment on
  `CatalogLifecycleRequest` in `resourcecore/write_types.go` from
  "deactivates or reactivates" to also cover hard delete (struct body
  unchanged); extend `resourcecore/writer_test.go`
  (`TestWriter_NoUngraduatedMethodExported`: `NumMethod() != 8` → `!= 9`,
  `want` += `"HardDeleteCatalog"`) and `resourcecore/external_test.go`.
  Satisfies spec requirements "Consumer-neutral public HardDeleteCatalog"
  and the compile-time half of "Compiled surface extended to Create,
  Update, Deactivate, Reactivate, and HardDeleteCatalog." <!-- sdd-owner: implementation -->

**Allowed edit surfaces:** exactly `resourcecore/writer.go`,
`resourcecore/write_types.go`, `resourcecore/writer_test.go`, and
`resourcecore/external_test.go`. `resourcecore/copy.go` is out of scope —
the request is scalar-only, no `Clone*` function is needed.

**Dependencies and end state:** start from Deactivate/Reactivate-WRITE
shipped and archived; `Reader`/`ReadCapabilities` and every prior write
method remain untouched byte-for-byte. `Writer.HardDeleteCatalog` validates
request shape only, via the already-existing `validateCatalogLifecycleRequest`:
`ID == 0` → `INVALID_ARGUMENT`; `ExpectedRevision == 0` → `INVALID_ARGUMENT`;
blank `Actor` after `strings.TrimSpace` → `INVALID_ARGUMENT`. No unknown-`Kind`
check — an invalid `Kind` surfaces as `NOT_FOUND` from the internal service's
own `Get`, matching every prior graduation's precedent. `WriteCapabilities`
compiles exactly 9 methods (the prior 8 plus `HardDeleteCatalog`); no
`HardDeleteResource` stub, symbol, or discoverable artifact exists. The
request passes by value (no `Clone*` needed — scalar-only fields, Go value
semantics already guarantee no caller-mutation-after-call effect). Nothing
implements `WriteCapabilities` in production yet with the widened 9-method
shape — no bridge or composition wiring is affected by this unit;
`internal/bridge/resourcecore.Adapter` is untouched until U2. Rollback
removes only these four `resourcecore` files' `HardDeleteCatalog` additions.

**Focused checks:** `go test ./resourcecore -run 'TestWriter|TestExternalWrite' -count=1`.

**Strict evidence:** RED adds, before implementation:
  - `TestWriter_HardDeleteCatalog_ShapeValidation` (`ID == 0`,
    `ExpectedRevision == 0`, blank `Actor`, each → `INVALID_ARGUMENT`, no
    capability call).
  - `TestWriter_NoUngraduatedMethodExported` (extended): reflection over
    `WriteCapabilities`/`Writer` asserting exactly 9 methods, still no
    `HardDeleteResource` stub.
  - `TestExternalWrite_ConsumerHardDeletesInactiveUnreferencedCatalog` — a
    package-external fake `WriteCapabilities` implementing all 9 methods,
    proving `HardDeleteCatalog` compiles and succeeds using only public
    types (spec scenario "External consumer hard-deletes an inactive,
    unreferenced catalog record").

  GREEN implements the smallest `Writer` method that satisfies shape
  validation only, with no business decision. TRIANGULATE covers `ID == 0`
  and `ExpectedRevision == 0` independently, and blank `Actor` after
  `strings.TrimSpace`. REFACTOR keeps `Writer` free of any reference-existence,
  guard-chain, or revision-currency decision — those stay internal.

**Mandatory slice boundary:** after the focused command, run
`go test ./... -count=1` and record compile/full-suite success before U2.

### U2 — Bridge translation, no confirm-read, 8-category table, non-duplication proof

- [x] Widen `catalogWriter` in `internal/bridge/resourcecore/adapter.go`
  with `HardDeleteRevision(ctx, kind, id, expectedRevision) error`, bound to
  the already-production `catalogo.Service.HardDeleteRevision`
  (`service.go:608`); add `Adapter.HardDeleteCatalog` — translate `Kind`,
  attach `Actor` via `core.WithActor`, delegate, `mapError`, return, with
  **no** confirm-read — reusing `mapError`/`core.WithActor` unmodified, zero
  new mapping functions; adapt every in-repo bridge fake in the same slice.
  Add coverage in `internal/bridge/resourcecore/adapter_test.go`. Satisfies
  "HardDelete field completeness through the sole bridge, no confirm-read,"
  "Bridge delegates to Core authority — no guard-chain re-implementation,"
  "Strict CAS on HardDeleteCatalog — no idempotent no-op bypass,"
  "HardDelete-reachable error category coverage," and "Actor attribution
  without persistence on HardDeleteCatalog." <!-- sdd-owner: implementation -->

**Allowed edit surfaces:** exactly `internal/bridge/resourcecore/adapter.go`
and `internal/bridge/resourcecore/adapter_test.go`.

**Dependencies and end state:** U1 is green. `catalogWriter` = existing
`Create`/`UpdateRevision`/`DeactivateRevision`/`ReactivateRevision` + new
`HardDeleteRevision`. `*catalogo.Service` already satisfies the widened
port in production — only in-repo test fakes gain the method;
`internal/tui/catalog_admin.go:83-85`'s existing `catalogDeleter` interface
already proves this exact call shape is a real, already-consumed production
seam. `Adapter.HardDeleteCatalog` calls
`a.catalog.HardDeleteRevision(core.WithActor(ctx, req.Actor), kind, req.ID, req.ExpectedRevision)`
and returns `mapError(err)` directly — no success-path record, no
confirm-read, because `HardDeleteRevision` returns only `error` and the
record no longer exists. Field-completeness: `.Kind`→`kind` argument
(Mapped), `.ID`→`id` argument (Mapped), `.ExpectedRevision`→`expectedRevision`
argument (Mapped), `.Actor`→`core.WithActor` (Mapped, diagnostics-only).
`internal/app/*`, `internal/domain/*`, `internal/postgres/*` remain
unchanged. Rollback removes only these two files' `HardDeleteCatalog`
additions.

**Focused checks:** `go test ./internal/bridge/resourcecore -run 'TestHardDeleteCatalog|TestWriteBridge_CatalogWriter_HardDeleteRevisionFakeAdapted' -count=1`.

**Strict evidence:** RED adds, before implementation, in
`internal/bridge/resourcecore/adapter_test.go`:
  - `TestWriteBridge_CatalogWriter_HardDeleteRevisionFakeAdapted` — every
    in-repo fake compiles against the widened port.
  - `TestHardDeleteCatalog_FieldCompleteness_KindIDExpectedRevision` — the
    three completeness rows against the `kind`/`id`/`expectedRevision`
    arguments.
  - `TestHardDeleteCatalog_EightCategoryTable_DistinctAndNoLeakage` —
    injects each of the 8 reachable sentinel families
    (`INVALID_ARGUMENT`, `NOT_FOUND`, `INVALID_CATALOG`,
    `INVALID_LIFECYCLE`, `IN_USE`, `CONFLICT`, `UNAVAILABLE`, `INTERNAL`)
    into the widened `catalogWriter` fake and asserts exact public code,
    distinctness, and no leakage (`Error()`, `%v`, `%+v`, concrete type,
    `errors.Unwrap`, recursive chain) of pgx/`PgError`/SQLSTATE/constraint/
    table/column/server text. `INVALID_LIFECYCLE` and `IN_USE` are the
    first-ever reachable proof from the public surface (spec: "Active-target
    and in-use rejections are the first-ever reachable proof, within the
    declared set of 8").
  - `TestHardDeleteCatalog_EightCategoryTable_ExcludesUnreachable` —
    explicit assertion that `DUPLICATE`, `INVALID_REFERENCE`, `VALIDATION`,
    `INTEGRITY`, `IDENTITY_CONFLICT`, `REACTIVATION_IMPOSSIBLE`, and
    `IMMUTABLE_CODE` are never claimed reachable from `HardDeleteCatalog`.
  - `TestHardDeleteCatalog_NoGuardChainDuplication_OneSeamCallZeroReads` —
    **mechanism 3, the falsifiable non-duplication proof.** A fake
    `catalogWriter` returns `nil` from `HardDeleteRevision` while its read
    projection reports `Active: true`; asserts the bridge still succeeds,
    with exactly one seam call and zero `Get`/`List` calls (spec: "A
    guard-chain rejection surfaces unchanged through the bridge, with no
    second check").
  - `TestHardDeleteCatalog_StaleExpectedRevision_AlwaysConflict_NoBypass` —
    **the CAS contrast with the lifecycle precedent.** Injects
    `domain.ErrRevisionConflict` for a stale `ExpectedRevision`; asserts
    `CONFLICT` regardless of the target's current state, with no idempotent
    no-op bypass (spec scenario "A stale ExpectedRevision yields CONFLICT").
  - `TestHardDeleteCatalog_NoConfirmReadAfterSuccess` — after a successful
    fake `HardDeleteRevision` call, asserts zero `Get`/`List` calls follow
    (spec: "the bridge issues no follow-up read of the now-deleted record").
  - `TestHardDeleteCatalog_ActorReachesDiagnosticSeam` — `Actor` reaches
    `internal/core/diagnostics.go`'s `core.WithActor`/`ActorFrom` seam on
    both the success and failure path; asserts `Actor` appears on no
    persisted field and no public DTO.

  GREEN implements the seam widening and `Adapter.HardDeleteCatalog`, and
  adapts every in-repo fake. TRIANGULATE covers the guard-chain-duplication
  boundary independently of the CAS boundary, and every reachable/
  unreachable category from the 8-category table. REFACTOR confirms zero
  new mapping functions were introduced (only the one `Adapter` method and
  the one seam method are new) and that no reference-existence, dependency,
  or business decision entered the bridge.

**Mandatory slice boundary:** after the focused command, run
`go test ./... -count=1` and record compile/full-suite success before U3.

### U3 — Readiness record and full-suite verification

- [x] Create `openspec/changes/resource-master-core-write-harddelete/readiness.md`
  recording the verdict, the 8-category reachability table (with evidence
  pointers into design.md's and U2's line-numbered tables), the
  non-duplication and no-confirm-read proofs, and the strict-CAS contrast
  with the lifecycle precedent. No source change in this unit beyond the
  readiness document. Satisfies the zero-touch half of "Compiled surface
  extended to Create, Update, Deactivate, Reactivate, and HardDeleteCatalog"
  and closes the readiness success criteria from proposal.md. <!-- sdd-owner: implementation -->

**Allowed edit surfaces:** exactly
`openspec/changes/resource-master-core-write-harddelete/readiness.md`. No
`.go` file is touched in this unit.

**Dependencies and end state:** U2 is green. The readiness record states:
the verdict; the 8 proven-reachable categories with evidence pointers,
including the first-ever `INVALID_LIFECYCLE`/`IN_USE` proof; the explicit
exclusion list (`DUPLICATE`, `INVALID_REFERENCE`, `VALIDATION`, `INTEGRITY`,
`IDENTITY_CONFLICT`, `REACTIVATION_IMPOSSIBLE`, `IMMUTABLE_CODE`); the
guard-chain non-duplication proof (`TestHardDeleteCatalog_NoGuardChainDuplication_OneSeamCallZeroReads`);
the no-confirm-read proof (`TestHardDeleteCatalog_NoConfirmReadAfterSuccess`);
the strict-CAS-no-bypass contrast with Deactivate/Reactivate's documented
asymmetry (`TestHardDeleteCatalog_StaleExpectedRevision_AlwaysConflict_NoBypass`);
the disclosed live-PostgreSQL integration-evidence gap in a sandboxed apply
session, matching the three prior graduations' precedent; and zero-touch
confirmation for `cmd/garfex`/`internal/tui`.

**Focused checks:** `go test ./... -count=1`; `rg -l resourcecore cmd/garfex internal/tui`
and `git diff --stat -- cmd/garfex internal/tui` both empty; `gofmt -l .`,
`go vet ./...`, `golangci-lint run ./...` all clean.

**Strict evidence:** RED confirms, before writing the readiness record,
that the 8-category table, the non-duplication proof, and the no-confirm-read
proof are each backed by a named, already-green test from U2 (re-grepped by
test name, not repeated from memory). GREEN writes the readiness record
citing those exact test names and design.md's line-numbered evidence.
TRIANGULATE independently re-runs the full suite and the zero-touch scans
rather than trusting U1–U2's prior receipts alone. REFACTOR simplifies the
readiness record for reviewer scanning without weakening any claim.

**Mandatory slice boundary:** after the focused command, run
`go test ./... -count=1` one final time and record compile/full-suite
success before the parent-controlled gate.

## Parent-controlled gate after the final unit

- [x] G1 — HardDeleteCatalog-WRITE readiness. After U3, recalculate
  HardDeleteCatalog-WRITE readiness from `resourcecore/`,
  `internal/bridge/resourcecore/`, and
  `openspec/changes/resource-master-core-write-harddelete/readiness.md`;
  confirm all of the following before treating HardDeleteCatalog-WRITE as
  ready:
  - (a) an external Go package constructs a `Writer` and calls
    `HardDeleteCatalog` successfully under CAS with no `internal` import,
    against an inactive, unreferenced record, using only the reused
    `CatalogLifecycleRequest` fields (spec: "Consumer-neutral public
    HardDeleteCatalog");
  - (b) every public `CatalogLifecycleRequest` field reaches
    `HardDeleteRevision` unchanged or carries a one-line omission
    rationale, and no confirm-read follows success (spec: "HardDelete field
    completeness through the sole bridge, no confirm-read");
  - (c) the bridge never re-implements the guard chain — proven by U2's
    call-count assertion, not asserted only in prose (spec: "Bridge
    delegates to Core authority — no guard-chain re-implementation");
  - (d) a stale `ExpectedRevision` always yields `CONFLICT`, with no
    idempotent no-op bypass (spec: "Strict CAS on HardDeleteCatalog — no
    idempotent no-op bypass");
  - (e) exactly 8 reachable and distinct categories are proven, including
    the first-ever reach of `INVALID_LIFECYCLE`/`IN_USE`, with every
    excluded category confirmed never claimed reachable (spec:
    "HardDelete-reachable error category coverage");
  - (f) `Actor` reaches the internal diagnostic seam via
    `core.WithActor`/`core.ActorFrom` only, is never persisted, introduces
    no new column or migration, and appears on no public DTO, on both the
    success and failure path (spec: "Actor attribution without persistence
    on HardDeleteCatalog");
  - (g) exactly `CreateCatalog`, `CreateResource`, `UpdateCatalog`,
    `UpdateResource`, `DeactivateCatalog`, `ReactivateCatalog`,
    `DeactivateResource`, `ReactivateResource`, and `HardDeleteCatalog` —
    9 methods — exist on `Writer`/`WriteCapabilities`, with no
    `HardDeleteResource` stub (spec: "Compiled surface extended to Create,
    Update, Deactivate, Reactivate, and HardDeleteCatalog");
  - (h) `cmd/garfex/` and `internal/tui/` show zero changed lines and zero
    new `resourcecore` references, confirmed by U3's textual-reference
    scan;
  - (i) the request is defensively copied at the boundary; a caller
    mutating it after the call changes nothing in flight or persisted
    (spec: "Consumer-neutral public HardDeleteCatalog");
  - (j) `go test ./... -count=1`, `gofmt -l .`, `go vet ./...`, and
    `golangci-lint run ./...` all pass; CI-only race/build results and the
    disclosed live-PostgreSQL integration-evidence gap are reported
    separately, as receipts, not as a gate substitute.

  `HardDeleteResource` is not applied or planned in this change; it remains
  a separate, later, per-operation-gated change with its own readiness
  evidence, per the spec's inherited "Failing evidence withdraws only
  affected write readiness" requirement and this change's own non-goals.
  The archived spec's prior 8-method "Compiled surface" requirement is
  superseded and removed from the merged spec at archive time, per this
  change's spec delta. <!-- sdd-owner: parent -->

## Final graph self-audit

- Unit order is exactly public HardDeleteCatalog contract (U1) → bridge
  translation, no confirm-read, 8-category table, non-duplication proof
  (U2) → readiness + full-suite verification (U3), matching design.md's
  file-change table and testing strategy exactly.
- No unit edits `internal/app/*`, `internal/domain/*`, or
  `internal/postgres/*` — no internal-layer fix is needed for this
  graduation; `internal/core/errors.go`'s `Map` is never edited, since
  `InvalidLifecycle`/`InUse` already exist and are already correctly
  ordered.
- The one port widening (`catalogWriter` in U2) adapts every in-repo fake
  in the same slice; no port is edited without every current implementation
  and test double adapting together.
- `core.Record`'s, `core.WithActor`'s, and `DiagnosticSink`'s signatures
  never change across the chain — the mechanism built for Create and
  extended for Update, Deactivate, and Reactivate is reused as-is.
- Every unit has a concrete start dependency, exact edit surfaces, focused
  RED/GREEN/TRIANGULATE/REFACTOR evidence, a rollback boundary, and
  mandatory `go test ./... -count=1` compile/full-suite proof.
- The design's structural novelty — no confirm-read, no idempotent-bypass,
  and the guard-chain non-duplication proof — is resolved by named,
  concrete tests in U2 (`TestHardDeleteCatalog_NoConfirmReadAfterSuccess`,
  `TestHardDeleteCatalog_StaleExpectedRevision_AlwaysConflict_NoBypass`,
  `TestHardDeleteCatalog_NoGuardChainDuplication_OneSeamCallZeroReads`), not
  left as prose-only claims.
- The graph contains no `HardDeleteResource` implementation task; it
  remains future work after its own later, separate, per-operation
  readiness decision, exactly as proposal.md's non-goals require.
- U1 and U2 are confirmed independently compilable: `resourcecore.NewWriter`
  has no production call site today, so widening `WriteCapabilities` in U1
  cannot break `internal/bridge/resourcecore` compilation before U2 lands.
- Only `openspec/changes/resource-master-core-write-harddelete/tasks.md`,
  the four `resourcecore`/`internal/bridge/resourcecore` source-and-test
  surfaces named above,
  `openspec/changes/resource-master-core-write-harddelete/readiness.md`, and
  the corresponding Engram topic are affected by this plan; the archived
  Create, Update, and Deactivate/Reactivate artifacts and any unrelated
  in-flight change remain untouched. The `openspec/specs/resource-master-core/spec.md`
  merge is handled by `sdd-archive`, not a task unit here.

## Key Learnings

1. `resourcecore.NewWriter` has no production call site, so `WriteCapabilities` interface widening never breaks bridge compilation before the bridge implements it.
2. `HardDeleteCatalog` is the smallest of the four write graduations because it reuses `CatalogLifecycleRequest` unchanged and adds exactly one method per layer.
3. The guard-chain non-duplication claim is proven falsifiably by a fake returning success while its read projection looks active, asserting one seam call and zero reads.
4. Strict CAS with no idempotent bypass is the operative contrast between hard delete and the lifecycle toggle precedent, since a deleted record has no "already deleted" state to short-circuit to.
5. All three prior graduations close with a dedicated, source-free readiness unit, and this change mirrors that same three-unit shape at a smaller scale.
