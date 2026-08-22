# Archive Report: supplier-master-core-read

**Change**: supplier-master-core-read
**Archived**: 2026-08-21
**Artifact store**: OpenSpec (hybrid mode with Engram)
**Status**: ✅ CLOSED — merged to main via PR #169 (issue #168)

## Final Verdict

**PASS WITH ONE FIX APPLIED DURING VERIFICATION**

- **Gap found during `sdd-verify`**: `activeFromScope` (lifecycle-scope mapping in the bridge) was implemented correctly but had zero direct test coverage against the spec's "Lifecycle scope honestly reachable" requirement. Fixed before merge: `TestActiveFromScope_MapsEachLifecycleScope` + `TestAdapter_SearchSuppliers_PassesScopeThroughToInternalCriteria`.
- **Requirements covered**: 12/12 by passing tests (see `verify-report.md` requirement-by-requirement table).
- **Tasks complete**: 3/3 units + G1 parent gate, all `[x]`.
- **CI**: both `verify` checks green on PR #169.

## Compiled surface

New public package `suppliercore` (repo root): `Reader`/`ReadCapabilities` with exactly 6 methods — `GetSupplier`, `SearchSuppliers`, `ListBranches`, `GetBranch`, `ListContacts`, `GetContact` — proven by a reflection-based compiled-surface guard (`TestReader_NoUngraduatedMethodExported`), mirroring `resourcecore/writer_test.go`'s `TestWriter_NoUngraduatedMethodExported` pattern.

New internal bridge `internal/bridge/suppliercore`: `Adapter` implementing `public.ReadCapabilities` over a single narrow `serviceReader` seam onto `internal/modules/suppliers/app.Service` — a deliberate, documented divergence from `resourcecore`'s two-seam (`catalogReader`/`resourceReader`) bridge, justified because one internal `Service` struct (not two distinct services) backs all three Supplier Master aggregates.

## Scope boundary

**In scope**: read-only access to Supplier, Branch, and Contact.
**Out of scope, explicitly and by design**:
- Any write operation (Create/Update/Deactivate/Reactivate) — a future, separate, Supplier-first series mirroring `resourcecore`'s own Create→Update→Lifecycle sequencing.
- `Actor` attribution field — will be introduced only when the Writer series ships, since the internal service has no such concept today.
- `Revision`/CAS optimistic concurrency — settled as permanently out of scope for now; no entity in the Supplier Master carries a revision column, and this package reflects that reality rather than inventing one.
- HardDelete — no internal capability exists for any of the three entities (a *total* absence, unlike `resourcecore`'s catalog-only `HardDeleteCatalog`/missing-`HardDeleteResource` split); never proposed here.

## Real bug caught and fixed before any code was written

Naively mirroring `resourcecore`'s `ListCatalog` bridge technique (`Limit: q.Limit + 1`) for `SearchSuppliers` would have broken the default page size: a caller passing `Limit: 0` (meaning "use the default") would have caused the bridge to request `Limit: 1`, returning a 1-row page instead of the intended default-100 page — because, unlike Branch/Contact, the internal Supplier repository does not over-fetch internally. Caught during the design phase (before implementation), fixed by resolving `effectiveLimit(q.Limit)` before deciding whether to over-fetch, and covered by an explicit regression test (`TestAdapter_SearchSuppliers_DefaultLimitDoesNotUnderFetch`).

## Process note

Explore and Propose phases were delegated to `sdd-explore`/`sdd-propose` sub-agents successfully. `sdd-spec` and `sdd-design` sub-agents both failed with API session-limit errors before producing output — the orchestrator implemented spec, design, tasks, and all 3 implementation units directly instead, extending the previously-recorded direct-implementation pattern (see Engram `feedback_sdd_apply_direct_implementation`) beyond `sdd-apply` to the planning phases as well.

## Follow-on work (not started)

The umbrella exploration `openspec/changes/supplier-master-public-contract/explore.md` remains active (not archived) — it documents the full Supplier Master public-contract series this change was sliced from. The Writer series (Create → Update → Lifecycle, Supplier-first, Branch/Contact writes later) is proposed but not yet started.
