# Design: Public WRITE on the Supplier Master Core — Supplier Create

## Technical Approach

Mirror the archived `resourcecore` write pattern one operation at a time. `suppliercore` gains a `Writer` over a one-method `WriteCapabilities` port; the existing `internal/bridge/suppliercore.Adapter` gains a narrow `serviceWriter` seam and translates `SupplierWriteRequest` → `domain.SupplierDetails` → `app.Service.CreateSupplier`, projecting the persisted result through the existing `mapSupplier`. The Writer validates shape only (non-blank `Actor`); every content rule stays in `domain.NewSupplier`. No new error code, no CAS, no confirm-read, no composition.

## Architecture Decisions

| # | Decision | Choice | Rejected alternative | Rationale |
|---|---|---|---|---|
| 1 | Bridge seam shape | Add `serviceWriter` (one method) and a combined `supplierService interface { serviceReader; serviceWriter }`; `Adapter.service` and `NewAdapter` take the combined seam. Add `var _ public.WriteCapabilities = (*Adapter)(nil)`. | Second nullable `writer` field keeping `NewAdapter(serviceReader)` | One `*app.Service` already backs everything (same reasoning that produced a single `serviceReader`). A nullable second field would let a constructed `Adapter` satisfy `WriteCapabilities` and panic on call. Cost: read-only construction disappears and the read test fake gains one method. |
| 2 | **Open question 1** — `mapError` default message | Neutralize the literal to `"supplier master operation failed"`; keep one `mapError`. | Sibling `mapWriteError` | The proposal's non-goals forbid "a second translation site". Verified no test or spec asserts the read literal (`adapter_test.go:91` asserts only `Code() == INTERNAL`); the only other occurrences are archived docs. The spec constrains leakage and a fixed generic message, not the wording. |
| 3 | **Open question 2** — `CloneSupplierWriteRequest` | Omit it. Add `TestSupplierWriteRequest_NoReferenceTypedField` (reflect: no pointer/slice/map/chan/func field) to keep the assumption enforced. | Export a by-value no-op for symmetry with `CloneSupplier` | `CloneSupplier` defends *returned* values a consumer keeps; an all-string request is copied by Go on the call itself, so there is no aliasing to prevent — "no abstraction without a real boundary". The spec's copy scenario passes on value semantics alone. The guard converts a silent assumption into a failing test the day a Branch/Contact slice adds a reference field, which is when the helper becomes mandatory. `copy.go` therefore stays **unchanged**, diverging from the proposal's affected-areas table. |
| 4 | Create result | Return `mapSupplier(created)` directly. | resourcecore-style create-confirm read | `Service.CreateSupplier` already returns the persisted `domain.Supplier` with ID and timestamps; a confirm-read would add a second query and a new failure mode for nothing. |
| 5 | Canonicalization | Writer passes the five detail fields verbatim; no `TrimSpace` at the boundary. | Trim in the Writer | `SupplierDetails.canonical()` is the sole authority; trimming twice would let a whitespace-only identifier be rejected as `INVALID_ARGUMENT` instead of `VALIDATION`, contradicting the spec. |
| 6 | CONFLICT mapping | No new `mapError` branch. | Dedicated `ErrTaxIdentifierConflict` branch | `ErrTaxIdentifierConflict = fmt.Errorf("%w: tax identifier", ErrConflict)`, so the existing `errors.Is(err, domain.ErrConflict)` branch already yields plain `CONFLICT` with a sanitized message. |

## Data Flow

    consumer ──► suppliercore.Writer.CreateSupplier(ctx, req)
                    │ validate: TrimSpace(Actor) != ""   → INVALID_ARGUMENT
                    ▼
              WriteCapabilities.CreateSupplier
                    │  (bridge Adapter)
                    │  ctx := core.WithActor(ctx, req.Actor)   ── diagnostic only
                    │  domain.SupplierDetails{5 fields}
                    ▼
              app.Service.CreateSupplier ──► domain.NewSupplier ──► repo (PostgreSQL)
                    │                              └─ VALIDATION
                    │  err ──► mapError ──► NOT_FOUND | VALIDATION | CONFLICT | INTERNAL
                    ▼
              mapSupplier(created) ──► CloneSupplier ──► public Supplier

## File Changes

| File | Action | Description |
|------|--------|-------------|
| `suppliercore/write_types.go` | Create | `SupplierWriteRequest{Actor, TradeName, LegalName, TaxIdentifier, Website, Notes}` — no `Active`, no `Revision`. |
| `suppliercore/writer.go` | Create | `WriteCapabilities` (1 method), `Writer`, `NewWriter` nil guard, `validateSupplierWriteRequest`. |
| `suppliercore/writer_test.go` | Create | Shape/nil-guard/delegation/clone tests, reflection guard, value-typed-field guard. |
| `suppliercore/external_test.go` | Modify | Add `TestExternalConsumer_CreatesSupplier` (package `suppliercore_test`, no `internal` import). |
| `suppliercore/doc.go` | Modify | "READ only" → "read plus Supplier Create"; Actor as diagnostic-only future audit seed; inherited lost-update race; HardDelete permanently absent; `NOT_FOUND` unreachable from Create. |
| `internal/bridge/suppliercore/adapter.go` | Modify | `serviceWriter` + combined seam, `Adapter.CreateSupplier`, `WriteCapabilities` assertion, neutral `mapError` default message. |
| `internal/bridge/suppliercore/adapter_test.go` | Modify | Fake gains `CreateSupplier`; add create mapping, field-completeness, and leakage tests. |
| `suppliercore/copy.go`, `errors.go`, `internal/modules/suppliers/**` | Unchanged | See decisions 3 and 6. |

## Interfaces / Contracts

```go
type WriteCapabilities interface {
    CreateSupplier(context.Context, SupplierWriteRequest) (Supplier, error)
}

type supplierService interface { // internal/bridge/suppliercore
    serviceReader
    serviceWriter
}
```

**GARFEX_STRICT field-completeness gate** (`SupplierWriteRequest` → `domain.SupplierDetails`): `TradeName`, `LegalName`, `TaxIdentifier`, `Website`, `Notes` all mapped, 5/5. `Actor` is deliberately not a `SupplierDetails` field — it travels via `core.WithActor` and needs the one-line omission comment at the mapping site. Result direction reuses `mapSupplier` (9/9 already gated by the read slice).

## Testing Strategy

| Layer | What to test | Approach |
|-------|--------------|----------|
| Unit (`suppliercore`) | Nil-cap `INVALID_ARGUMENT`; blank/whitespace Actor rejected without touching the capability; success path clones the result; `WriteCapabilities.NumMethod() == 1`; no reference-typed request field | Fake `WriteCapabilities` + `reflect`, mirroring `resourcecore/writer_test.go` |
| Unit (bridge) | All 5 details reach the fake service; `WithActor` populated; VALIDATION/CONFLICT/INTERNAL mapping; raw `PgError`-shaped error yields INTERNAL with no SQLSTATE/constraint text | Fake `supplierService` returning seeded errors |
| External | Consumer in `suppliercore_test` builds `NewWriter` and creates a supplier with no `internal` import | Extend `external_test.go` |
| Integration/E2E | N/A — library-only, nothing composed or wired in this repo | — |

Test names must contain `Writer`, `External`, or `Adapter`-plus-`Create` so the unit's focused `-run` pattern actually selects them (focused-test-discoverability gate).

## Threat Matrix

N/A — no routing, shell, subprocess, VCS/PR automation, executable-file classification, or process-integration boundary.

## Migration / Rollout

No migration required. No schema, wiring, or persisted state changes; reverting removes only the write surface.

## Open Questions

None. Proposal questions 4 and 5 are resolved by decisions 2 and 3.

Discovered during design (not blocking): `NewAdapter`'s parameter widening is a small breaking change inside `internal/`, absorbed by updating the existing read test fake.
