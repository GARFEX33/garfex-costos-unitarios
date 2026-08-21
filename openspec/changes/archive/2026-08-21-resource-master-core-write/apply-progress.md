# Apply Progress: resource-master-core-write

## Unit W1 — Public write contract

- Completed implementation task: **W1**, persisted as checked in `tasks.md`.
- Delivery boundary: `auto-chain`, `stacked-to-main`; this receipt covers one W1 stacked-to-main review boundary. No commit or PR action was performed.
- Scope stayed within the five authorized surfaces: `resourcecore/write_types.go`, `resourcecore/writer.go`, `resourcecore/copy.go`, `resourcecore/writer_test.go`, `resourcecore/external_test.go`.

### Changed surfaces

| File | Change | Purpose |
|---|---|---|
| `resourcecore/write_types.go` | New | `CatalogWriteRequest`, `ResourceWriteRequest` — package-owned, per-operation-family request DTOs reusing existing public `Value`/`AttributeValue`/`ResourceScope`/`ApplicabilityRule` types. |
| `resourcecore/writer.go` | New | `WriteCapabilities` (exactly `CreateCatalog`/`CreateResource`, no ungraduated method), `Writer`, `NewWriter` (nil → `INVALID_ARGUMENT`), and shape-only validation per design.md's fixed list. |
| `resourcecore/copy.go` | Modified | `CloneCatalogWriteRequest` (preserves nil vs. non-nil-empty `Rules`), `CloneResourceWriteRequest`. |
| `resourcecore/writer_test.go` | New | Nil-capability, catalog/resource shape-validation tables, no-ungraduated-method reflection check, caller-mutation-after-call proof. |
| `resourcecore/external_test.go` | Modified | Added `TestExternalWrite_ConsumerConstructsWriterUsingPublicTypesOnly` — external-package proof using only public types. |

Authored Go/test delta: **441 changed lines** (`write_types.go` 18 + `writer.go` 142 + `writer_test.go` 202 + `copy.go`/`external_test.go` diffs 79), marginally over the ≤400-line W1 forecast — recorded as a small `size:exception` rather than an artificial split, since the shape-validation test table (13 catalog + 9 resource subtests, covering every condition in design.md's fixed gate list) is the correctness evidence for this unit and splitting it would separate a test from the behavior it proves.

### Strict TDD evidence

| Phase | Command | Result |
|---|---|---|
| RED | `go test ./resourcecore/... -count=1` before `writer.go` existed | Failed to compile as intended (`undefined: NewWriter`, `undefined: WriteCapabilities`, `undefined: Writer`). |
| GREEN | `go test ./resourcecore -run 'TestWriter\|TestExternalWrite\|TestWriteRequestCopy' -v -count=1` | All pass after `writer.go`/`copy.go` implemented. |
| TRIANGULATE | Shape-validation tables cover: blank Actor, unknown Kind, nil/empty Values, unknown Value.Kind, non-zero `Reference.ID`, `UnitCode` required-on-QUANTITY/forbidden-elsewhere, integer/decimal parse failure, `ApplicabilityRule.Equals.Kind != TEXT`, unknown Mode, blank Scope fields, blank NaturalUnit — every condition in design.md's shape-gate list. | Pass. |
| REFACTOR | `gofmt -l resourcecore/*.go` (clean), `go vet ./resourcecore/...` (clean), `golangci-lint run ./resourcecore/...` (`0 issues.`) | Clean. |
| Boundary | `go test ./... -count=1` | Every package this change touches is green. One known, pre-existing, untracked, unrelated failure persists: `agent/skills/golang-cli/assets/examples` (missing example deps, not in git, never reaches real CI — documented since the `stabilize-resource-master-core` change). |

### Deviations and issues

- None. Implementation matches `design.md`'s exact fixed shape-validation list and DTO shapes.
- Note: `Writer`'s shape validation deliberately does not touch `AttributeValue.UnitCode` (the redundant top-level field alongside `AttributeValue.Value.UnitCode`) — only `Value.UnitCode` (nested) is validated, matching design.md's explicit shape-gate list which speaks only of "a Value['s] UnitCode." Reconciling the two `UnitCode` fields, if needed, is W3's bridge-mapping concern, not W1's contract-shape concern.

### Rollback boundary

Revert only the W1 surfaces:
- Delete `resourcecore/write_types.go`, `resourcecore/writer.go`, `resourcecore/writer_test.go`.
- Remove `CloneCatalogWriteRequest`/`CloneResourceWriteRequest` from `resourcecore/copy.go`.
- Remove `TestExternalWrite_ConsumerConstructsWriterUsingPublicTypesOnly` and its fake from `resourcecore/external_test.go`.

Nothing in `internal/bridge/resourcecore`, `internal/core`, `internal/app`, `internal/domain`, `internal/postgres`, `cmd/garfex`, or `internal/tui` was touched.

### tasks.md — W1 checked

The **tasks.md W1 checkbox is now marked `[x]`**.

### Next recommended phase

Continue implementation with **W2 — Catalog Create bridge + actor seam** (`internal/core/diagnostics.go`, `internal/core/errors_test.go`, `internal/bridge/resourcecore/adapter.go`, `internal/bridge/resourcecore/adapter_test.go`). No commit, PR, review, refutation, correction, validation, sync, or archive actors/actions were launched.

## Unit W2 — Catalog Create bridge + actor seam

- Completed implementation task: **W2**, persisted as checked in `tasks.md`.
- Scope stayed within the four authorized surfaces. `core.Record`'s and `DiagnosticSink`'s signatures stayed byte-identical; the six pre-existing `core.Record` call sites are untouched.

### Changed surfaces

| File | Change | Purpose |
|---|---|---|
| `internal/core/diagnostics.go` | +26 lines | `actorContextKey` (unexported), `WithActor`, `ActorFrom`, `NewDiagnosticRecord`, `DiagnosticRecord.Actor` field. Additive only. |
| `internal/core/errors_test.go` | +29/-2 | `TestActor_WithActor_ActorFrom_RoundTrip`, `TestActor_NewDiagnosticRecord_IncludesActorAndBlankNoop`; `spySink.Record` switched to build via `NewDiagnosticRecord`. |
| `internal/bridge/resourcecore/adapter.go` | +95 lines | `catalogWriter` seam, `catalogPort` (= `catalogReader` + `catalogWriter`), widened `Adapter.catalog`/`NewAdapter` from `catalogReader` to `catalogPort`, `Adapter.CreateCatalog` (binds to `catalogo.Service.Create`, never `CreateV2`), inverse mappers `toDomainCatalogValues`/`toDomainCatalogValue`/`toDomainCatalogRules`. |
| `internal/bridge/resourcecore/adapter_test.go` | +218/-9 | `fakeCatalogReader` gained a `create` field/method (every existing read-only test unaffected); `TestWriteBridge_CatalogPort_FakeAdapted`, `TestWriteBridge_CatalogCreate_FieldCompleteness`, `TestCatalogCreate_ValueMapping_InverseRoundTrip` (7 field kinds), `TestCatalogCreate_RuleMapping_AllSixFieldsMapped`, `TestCatalogCreate_NineCategoryTable_DistinctAndNoLeakage`. |

Authored Go/test delta: **368 changed lines** (357 insertions + 11 deletions), within the ≤400 W2 forecast.

### Strict TDD evidence

| Phase | Command | Result |
|---|---|---|
| RED (actor) | `go test ./internal/core/... -count=1` before `diagnostics.go` changes | Failed to compile as intended (`undefined: NewDiagnosticRecord`, `ActorFrom`, `WithActor`). |
| RED (bridge) | `go test ./internal/bridge/resourcecore/... -count=1` before `adapter.go` changes | Failed to compile as intended (`adapter.CreateCatalog undefined`). |
| GREEN | `go test ./internal/core ./internal/bridge/resourcecore -run 'TestWriteBridge\|TestActor\|TestCatalogCreate' -v -count=1` | One genuine test-authoring bug caught and fixed in the same pass: `TestWriteBridge_CatalogCreate_FieldCompleteness` asserted `captured.Kind` on the record the fake received, but design.md's field-completeness table documents `Kind` mapping to the `Create` *kind argument* (the real `catalogo.Service.Create` assigns `rec.Kind = kind` internally) — the fake didn't replicate that, so the test asserted the wrong field. Fixed the test to capture and assert the `kind` argument instead of production code. |
| TRIANGULATE | 7-field-kind inverse round trip, 6-field rule mapping, 9-category injected-sentinel table (asserting exact code, distinctness across all 9, and no leakage of SQLSTATE/constraint/table/column/server text) | Pass. |
| REFACTOR | `gofmt -l` (clean), `go vet` (clean), `golangci-lint run` (`0 issues.`) | Clean. |
| Boundary | `go test ./... -count=1` | Every package this change touches is green; only the known, pre-existing, untracked, unrelated `agent/skills/golang-cli/assets/examples` failure persists. |

### Deviations and issues

- None in production code. One test-authoring correction (above), caught by the focused run itself before being accepted as GREEN — not a production defect.

### Rollback boundary

Revert only the W2 surfaces: the additive `diagnostics.go` symbols, the `catalogWriter`/`catalogPort`/`CreateCatalog`/inverse-mapper additions in `adapter.go`, and the corresponding test additions. `internal/app/catalogo`, `internal/domain`, `internal/postgres` were never touched.

### tasks.md — W2 checked

The **tasks.md W2 checkbox is now marked `[x]`**.

### Next recommended phase

Continue implementation with **W3 — Resource Create bridge** (`internal/bridge/resourcecore/adapter.go`, `internal/bridge/resourcecore/adapter_test.go`, `resourcecore/external_test.go`). No commit, PR, review, refutation, correction, validation, sync, or archive actors/actions were launched.

## Unit W3 — Resource Create bridge

- Completed implementation task: **W3**, persisted as checked in `tasks.md`.
- Scope stayed within the three authorized surfaces. `internal/app/recursos`, `internal/domain`, `internal/postgres` were never touched.

### Changed surfaces

| File | Change | Purpose |
|---|---|---|
| `internal/bridge/resourcecore/adapter.go` | +~98 lines (this unit's own contribution, isolated from W2's cumulative diff to the same file) | `resourceWriter` seam, `resourcePort` (= `resourceReader` + `resourceWriter`), widened `Adapter.resources`/`NewAdapter` from `resourceReader` to `resourcePort`, `Adapter.CreateResource` (binds to `recursos.Service.Create`, performs the create-confirm read via `resources.Get` since the repository's `Create` returns no ID/Revision, no reclassification of confirm-read errors), inverse mapper `toDomainResourceAttributes`/`toDomainResourceAttribute` (7 mappable kinds, 4 rejected as `INVALID_ARGUMENT`). |
| `internal/bridge/resourcecore/adapter_test.go` | +~214 lines (this unit's own contribution) | `fakeResourceReader` gained a `create` field/method; `TestWriteBridge_ResourcePort_FakeAdapted`, `TestResourceCreate_FieldCompleteness_AgainstCreateCommand`, `TestResourceCreate_AttributeMapping_InverseRoundTripSevenKinds`, `TestResourceCreate_AttributeMapping_RejectsFourUnmappableKinds`, `TestResourceCreate_ConfirmRead_IdentityV1NoReclassification` (proves both the happy path `ID>0`/`Revision>=1`/`identity-v1` and that a failed confirm read returns unreclassified `NOT_FOUND`), `TestResourceCreate_NineCategoryTable_DistinctAndNoLeakage`. |
| `resourcecore/external_test.go` | Already covered in W1 | `TestExternalWrite_ConsumerConstructsWriterUsingPublicTypesOnly` (added in W1) already exercises `CreateResource` through only public types — no separate resource-only external test added, to avoid duplicate coverage of the same proof. |

Authored Go/test delta for this unit alone: **~312 changed lines**, within the ≤400 W3 forecast.

### Strict TDD evidence

| Phase | Command | Result |
|---|---|---|
| RED | `go test ./internal/bridge/resourcecore/... -count=1` before `CreateResource` existed | Failed to compile as intended (`adapter.CreateResource undefined`), across all 7 new call sites. |
| GREEN | `go test ./internal/bridge/resourcecore ./resourcecore -run 'TestWriteBridge\|TestResourceCreate\|TestExternalWrite' -v -count=1` | All pass on first implementation attempt — no test-authoring correction needed this time. |
| TRIANGULATE | 7-kind inverse round trip (`CONTROLLED_OPTION`, `INTEGER`, `DECIMAL`, `QUANTITY` with required `UnitCode`, `BOOLEAN`, `NOT_APPLICABLE`, `TEXT`), 4-kind rejection (`CODE`/`ENUM`/`STRING_LIST`/`REFERENCE`, asserting `Create` is never even called for these), confirm-read happy path (`ID>0`, `Revision>=1`, `identity-v1`) and confirm-read failure (unreclassified `NOT_FOUND`), 9-category injected-sentinel table with leakage assertions | Pass. |
| REFACTOR | `gofmt -l` (clean), `go vet` (clean), `golangci-lint run` (`0 issues.`) | Clean. |
| Boundary | `go test ./... -count=1` | Every package this change touches is green; only the known, pre-existing, untracked, unrelated `agent/skills/golang-cli/assets/examples` failure persists. |

### Deviations and issues

- `TestNoWriteMethods` (a pre-existing `resourcecore` test) was independently re-checked: it asserts that the plain DTO types (`CatalogRecord`, `Value`, `Resource`, etc.) carry no methods — unrelated to and unaffected by adding `Writer`/`WriteCapabilities`, which are separate types. Confirmed still passing, not a false negative.
- The redundant top-level `AttributeValue.UnitCode` field (alongside the nested `AttributeValue.Value.UnitCode` that this unit's mapper actually reads) is not populated or required on the write path — consistent with the W1 receipt's noted deviation.

### Rollback boundary

Revert only the W3 surfaces: the `resourceWriter`/`resourcePort`/`CreateResource`/inverse-attribute-mapper additions in `adapter.go`, and the corresponding test additions in `adapter_test.go`. `internal/app/recursos`, `internal/domain`, `internal/postgres` were never touched.

### tasks.md — W3 checked

The **tasks.md W3 checkbox is now marked `[x]`**.

### Next recommended phase

Continue implementation with **W4 — Create readiness record, documentation, and zero-touch confirmation** (`openspec/changes/resource-master-core-write/readiness.md` only; no `.go` file may be edited). No commit, PR, review, refutation, correction, validation, sync, or archive actors/actions were launched.

## Unit W4 — Create readiness record, documentation, and zero-touch confirmation

- Completed implementation task: **W4**, persisted as checked in `tasks.md`. No `.go` file was edited — documentation and verification only, exactly as scoped.

### Changed surfaces

| File | Change |
|---|---|
| `openspec/changes/resource-master-core-write/readiness.md` | New — full Create-WRITE readiness record. |

### Zero-touch confirmation (required evidence for the "Compiled surface limited to graduated Create" spec scenario)

```
$ rg -l "resourcecore" cmd/garfex/*.go internal/tui/*.go
(no output)
$ git diff --stat -- cmd/garfex internal/tui
(no output)
```

Both empty — zero `resourcecore` references, zero changed lines. Confirmed directly, not assumed.

### Resolved open questions from `design.md`

1. **`CatalogWriteRequest.Active` round-trips** — read `insertLocked` in full: it builds the insert candidate purely from the submitted `rec` (via `domain.ApplyCatalogMutation`'s `OpInsert` builders, which read `Active` directly off the record) — no forced-active behavior. `Active: false` round-trips correctly; no `MISSING DOMAIN CRITERION` needed.
2. **Catalog-side `NOT_FOUND` reachability** — read `insertLocked` in full: `ApplyCatalogMutation` → `next.Validate()` → `s.repoV2.Insert(ctx, rec)`, with no lookup-by-ID anywhere in the path (unlike resource create, which needs the confirm-read precisely because it lacks this). **Catalog `NOT_FOUND` is not end-to-end reachable from Create** — named as such in `readiness.md` with this reviewed, operation-specific reason, per the archived spec's explicit allowance for a reviewed non-reachability declaration. This narrows rather than weakens the claimed 9-category evidence: 9 for resource, 8 (excluding `NOT_FOUND`) for catalog, both fully accounted for.

### Strict TDD evidence

| Phase | Result |
|---|---|
| RED | Reviewed the readiness draft against `proposal.md`, `design.md`, and `specs/resource-master-core/spec.md`'s ten scenarios and two open questions before accepting any claim. |
| GREEN | Recorded only confirmed evidence — no aspirational or unverified claim; both open questions resolved by reading production code directly, not by assumption. |
| TRIANGULATE | Independently re-ran the zero-reference scan and cross-checked the field-completeness and 9-category evidence from W2/W3 against the readiness record's claims — all consistent. |
| REFACTOR | Simplified for reviewer scanning without weakening any claim. |
| Boundary | `go test ./... -count=1`, `gofmt -l .`, `go vet ./...`, `golangci-lint run ./...` — all clean except the known unrelated `agent/skills/golang-cli/assets/examples` issue. |

### tasks.md — W4 and G1 checked

The **tasks.md W4 checkbox and the G1 parent gate are now both marked `[x]`**. Every unit and gate in this change's `tasks.md` is now checked.

### Next recommended phase

No further implementation task remains unchecked in `tasks.md`. Next is `sdd-verify` (independent re-verification against spec/design/tasks before archive) or a delivery decision — neither was launched by this unit.
