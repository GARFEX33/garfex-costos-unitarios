# Design: Public WRITE HardDeleteCatalog on the Resource Master Core

## Decision summary

`resourcecore.Writer` and `WriteCapabilities` gain exactly one method,
`HardDeleteCatalog`, reaching 9 methods. `HardDeleteResource` stays absent,
unstubbed, and undiscoverable. `CatalogLifecycleRequest`
(`resourcecore/write_types.go:47-52`) is reused with **zero struct change**;
only its doc comment widens from "deactivates or reactivates" to cover hard
delete. `internal/app/*`, `internal/domain/*`, `internal/postgres/*`,
`cmd/garfex/`, and `internal/tui/` change zero lines.

## Decision 1 — public return shape (the deferred decision)

**Choice: `HardDeleteCatalog(ctx context.Context, req CatalogLifecycleRequest) error`.**

| Option | Tradeoff | Verdict |
| --- | --- | --- |
| **A. error-only** | Breaks the `(Record, error)` uniformity the other 7 methods share | **Chosen** |
| B. `(CatalogRecord, error)` returning a zero value | Uniform signature, but every field is fabricated: the Core produced no record | Rejected |
| C. `(CatalogRecord, error)` via a bridge pre-delete `Get` | Returns a pre-state snapshot the Core never produced as an outcome; TOCTOU-racy against the service's own internal read; adds a bridge-owned read to a delete path | Rejected |
| D. new `CatalogDeleteResult{Kind, ID}` type | Echoes the caller's own input back as if it were an outcome; a new public type carrying zero Core-produced data | Rejected |

**Rationale.** `catalogo.Service.HardDeleteRevision` returns only `error`
(`internal/app/catalogo/service.go:608`), and `domain.CatalogAdminRepositoryV2.Delete`
documents `CatalogWriteResult.Record` as **always nil** for delete
(`internal/domain/catalog_admin_v2.go:63-66`). A delete has no post-state to
project. The `(Record, error)` uniformity exists to carry the *persisted
projection*; where no projection exists, uniformity would be honoured by
inventing one. The user's confirmation is explicit: "sin inventar datos que
el Core no produce". Options B, C, and D each satisfy the signature at the
cost of the honesty invariant; A pays a signature-shape cost that is visible,
documented, and idiomatic Go for delete. Consequence, accepted and recorded:
`WriteCapabilities` is deliberately non-uniform, and the reflection assertion
below compiles that intent in rather than leaving it to prose.

## Decision 2 — guard-chain non-duplication as a structural mechanism

The user's sixth guardrail is enforced by **capability starvation plus a
four-line body**, not by prose.

The complete guard chain lives in `buildV2DeleteCandidate`
(`internal/app/catalogo/service.go:449-483`): reject `current.Active` →
`domain.ErrInvalidLifecycle` (456-458); reject any `Dependents` entry with
`Count != 0` → `domain.ErrCatalogInUse` (463-467); reject
`ReferencedByResources` → `domain.ErrCatalogInUse` (472-474); then
`ApplyCatalogMutation(OpDelete)` + `next.Validate()` (475-481).

**Mechanism 1 — the bridge cannot evaluate the dependency guards.** The
bridge's catalog seam is `catalogPort = catalogReader + catalogWriter`
(`adapter.go:19-41`). `catalogReader` exposes only `Kinds`, `List`, `Get`.
Neither `Dependents` nor `ReferencedByResources` is on any seam the `Adapter`
holds, so `IN_USE` is structurally un-reimplementable at the bridge. This
property is preserved by adding **one** method to `catalogWriter` and nothing
to `catalogReader`.

**Mechanism 2 — pass-through body, precedent-identical.** `Adapter.HardDeleteCatalog`
replicates `Adapter.DeactivateCatalog` (`adapter.go:206-216`) minus its
confirm-read: translate `Kind`, attach `Actor`, delegate, `mapError`, return.
No `if`, no lookup, no branch other than the error check. `Active` is
technically readable through `Get`, so the lifecycle guard is the one rule a
bridge *could* duplicate — mechanism 3 makes any such duplication fail a test.

**Mechanism 3 — falsifiable test.** A fake `catalogWriter` that returns `nil`
from `HardDeleteRevision` while its read projection reports `Active: true`
must still yield success through the bridge, and the fake must record exactly
one seam call and zero `Get`/`List` calls. A bridge that re-implemented the
lifecycle check would fail this test. Production safety is unaffected: the
real service rejects that case at `service.go:456-458`.

## Interfaces

```go
// resourcecore/writer.go — WriteCapabilities reaches 9 methods.
HardDeleteCatalog(context.Context, CatalogLifecycleRequest) error

func (w *Writer) HardDeleteCatalog(ctx context.Context, req CatalogLifecycleRequest) error {
    if err := validateCatalogLifecycleRequest(req); err != nil {
        return err
    }
    return w.cap.HardDeleteCatalog(ctx, req) // scalar-only: no Clone*, no record to clone back
}

// internal/bridge/resourcecore/adapter.go — catalogWriter gains one method.
HardDeleteRevision(ctx context.Context, kind domain.CatalogKindCode, id int64, expectedRevision uint64) error

func (a *Adapter) HardDeleteCatalog(ctx context.Context, req public.CatalogLifecycleRequest) error {
    return mapError(a.catalog.HardDeleteRevision(
        core.WithActor(ctx, req.Actor), domain.CatalogKindCode(req.Kind), req.ID, req.ExpectedRevision))
}
```

The seam signature is not invented: `internal/tui/catalog_admin.go:83-85`
already declares `catalogDeleter` with exactly this method, and
`*catalogo.Service` already satisfies it in production. Only in-repo test
fakes gain one method.

## Validation and CAS semantics

`validateCatalogLifecycleRequest` (`resourcecore/writer.go:141-152`) is reused
**unchanged**. Verified content: it rejects blank/whitespace `Actor`, zero
`ID`, and zero `ExpectedRevision`. Correction to a common shorthand: `Actor`
is *not* shape-optional — it is required non-blank, but is unauthenticated,
never persisted, and diagnostics-only via `core.WithActor`. Reusing the
validator therefore preserves confirmed assumption 2 exactly, with no new
authorization concept.

Strict CAS holds with no idempotent bypass, and this is the operative contrast
with the lifecycle precedent. `setActiveRevision` short-circuits on
`current.Active == active` **before** any CAS comparison (`service.go:572-574`),
so a stale `ExpectedRevision` can succeed silently there. `HardDeleteRevision`
has no such short-circuit: it always reaches `repoV2.Delete(ctx, kind, id,
expectedRevision)` (`service.go:623`). A stale `ExpectedRevision` therefore
always yields `CONFLICT` — never a silent no-op. No `Kind` validity check is
added, matching the lifecycle precedent: an unknown kind surfaces as
`NOT_FOUND` from the service's own `s.repo.Get` (`service.go:450`).

## Data flow

```text
consumer -> Writer.HardDeleteCatalog        (shape gate only)
         -> WriteCapabilities               (9th method, error-only)
         -> Adapter.HardDeleteCatalog       (WithActor + mapError; no rules)
         -> catalogo.Service.HardDeleteRevision
              -> prepareV2Write -> buildV2DeleteCandidate (THE guard chain)
              -> repoV2.Delete (CAS) -> classifyV2WriteError -> publishCoherent
```

## Error-category reachability — 8 of 15, two of them first-ever

| Category | Evidence |
| --- | --- |
| `INVALID_ARGUMENT` | Writer shape gate; `id == 0 \|\| expectedRevision == 0` (`service.go:609-611`) |
| `NOT_FOUND` | `s.repo.Get` in `buildV2DeleteCandidate` (`service.go:450-452`) |
| `INVALID_LIFECYCLE` | **First reachable** — `current.Active` → `ErrInvalidLifecycle` (`service.go:456-458`) |
| `IN_USE` | **First reachable** — `Dependents` `Count != 0` (463-467) or `ReferencedByResources` (472-474) |
| `INVALID_CATALOG` | `WrapInvalidCatalog(next.Validate())` (`service.go:479-481`) |
| `CONFLICT` | `classifyV2WriteError` after `repoV2.Delete` (`service.go:623-626`); always CAS-checked |
| `UNAVAILABLE` | `prepareV2Write` (`service.go:616`); ctx cancellation |
| `INTERNAL` | Any unclassified injected error; no pgx/SQLSTATE/constraint text leaks |

`internal/core/errors.go` needs **no change**, confirmed by direct read:
`domain.ErrInvalidLifecycle` → `InvalidLifecycle` at lines 80-81, and
`domain.ErrCatalogInUse` → `InUse` at lines 84-85, both already ordered
correctly within `Map`'s documented precedence.

Not reachable: `DUPLICATE`, `INVALID_REFERENCE`, `VALIDATION`, `INTEGRITY`,
`IDENTITY_CONFLICT`, `REACTIVATION_IMPOSSIBLE`, `IMMUTABLE_CODE`.

## File changes

| File | Action | Description |
| --- | --- | --- |
| `resourcecore/writer.go` | Modify | `WriteCapabilities` +1 method; `Writer.HardDeleteCatalog`; `validateCatalogLifecycleRequest` reused unchanged |
| `resourcecore/write_types.go` | Modify | Doc comment only — `CatalogLifecycleRequest` struct unchanged |
| `resourcecore/copy.go` | Unchanged | Scalar-only request; no record returned |
| `resourcecore/writer_test.go` | Modify | `NumMethod() != 8` → `!= 9`; `want` += `"HardDeleteCatalog"`; failure message updated |
| `resourcecore/external_test.go` | Modify | External consumer deletes using only public types |
| `internal/bridge/resourcecore/adapter.go` | Modify | `catalogWriter` +1 method; `Adapter.HardDeleteCatalog`; zero new mapping functions |
| `internal/bridge/resourcecore/adapter_test.go` | Modify | Fake +1 method; 8-category table; non-duplication proof; stale-CAS `CONFLICT` test |
| `internal/core/errors.go` | Unchanged | Both categories already mapped (lines 80-85) |

`TestWriter_NoUngraduatedMethodExported` (`writer_test.go:211-233`) derives its
`allowed` set from `want`, so adding only `HardDeleteCatalog` keeps
`HardDeleteResource` a compile-surface failure — the asymmetry guard survives
the update rather than being disarmed by it.

## Testing strategy

| Layer | What | How |
| --- | --- | --- |
| Unit (`resourcecore`) | Shape gate: blank `Actor`, zero `ID`, zero `ExpectedRevision` → `INVALID_ARGUMENT`, no capability call; compiled surface is exactly 9 methods with no `HardDeleteResource` | Table tests + reflection assertion |
| Unit (bridge) | 8-category injected table; `Actor` reaches `core.ActorFrom` and is never persisted | Fake `catalogWriter` with injected sentinels |
| Unit (bridge, guardrail) | Seam returns `nil` on an "active-looking" projection → bridge succeeds; exactly one seam call, zero `Get`/`List` | Call-recording fake |
| Unit (bridge, CAS) | Stale `ExpectedRevision` → `CONFLICT`, never silent success | Inject `domain.ErrRevisionConflict` |
| Integration | Active target → `INVALID_LIFECYCLE`; dependents/references → `IN_USE`; neither leaves partial state | Live PostgreSQL where the existing evidence gap permits |

## Threat matrix

N/A — no routing, shell command, subprocess, VCS/PR automation,
executable-file classification, or process-integration boundary. This change
adds one in-process Go method and one in-process translation.

## Migration / rollout

No migration, no schema change, no composition wiring, no tombstone, no audit
record (confirmed assumption 5). Rollback removes the `Writer` method, the
bridge method, the one `catalogWriter` line, and the test-assertion update;
`HardDeleteRevision` stays internally reachable exactly as today.

## Open questions

None. The deferred return-shape decision is resolved above as Option A.

## Key Learnings

1. The bridge cannot re-implement the `IN_USE` guard because `Dependents` and `ReferencedByResources` are absent from every seam the `Adapter` holds.
2. Signature uniformity across `WriteCapabilities` exists to carry a persisted projection, so a delete with no post-state honestly returns `error` only.
3. `validateCatalogLifecycleRequest` requires a non-blank `Actor`, which is a shape rule rather than the authorization concept the proposal deliberately excluded.
4. `HardDeleteRevision` has no idempotent short-circuit, so a stale `ExpectedRevision` always produces `CONFLICT` unlike catalog deactivate and reactivate.
5. The reflection assertion derives its allowed set from `want`, so `HardDeleteResource` stays a compile-surface failure after the update.
