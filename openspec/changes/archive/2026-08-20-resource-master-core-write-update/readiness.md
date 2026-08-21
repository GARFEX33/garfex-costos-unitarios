# Readiness: resource-master-core-write-update (Update graduation)

## Verdict

**Update-WRITE is ready**, with one honestly documented evidence gap: the live-PostgreSQL integration suite for `internal/postgres/catalog_admin_repository_v2.go` could not be executed in this sandboxed session (no `GARFEX_TEST_DSN`-exposed database available; `TestApplicabilityAggregateV2Integration` reports `SKIP: GARFEX_TEST_DSN not set` and no other test in that file requires a live database). Per this repository's own documented convention (`docs/architecture/catalog-source-of-truth.md`), PostgreSQL evidence for isolated-DB-only suites comes from the recorded isolated CI run, not a local sandbox without an exposed DSN. Everything gated by unit tests, `gofmt`, `go vet`, and the full non-DB test suite is proven and green.

## What shipped

- `resourcecore.Writer.UpdateCatalog`/`UpdateResource` (+`WriteCapabilities` +2 methods) — public Update contract under optimistic concurrency (`ID`/`Kind` + `ExpectedRevision uint64` + `Actor`), mirroring Create's mechanism exactly.
- `internal/bridge/resourcecore.Adapter.UpdateCatalog`/`UpdateResource` — sole-bridge translation, reusing every inverse mapper Create built, zero new mapping functions.
- The CONFLICT-wiring fix: `internal/postgres/catalog_admin_repository_v2.go`'s `errApplicabilityStaleRevision` (line 20) and `errCatalogStaleRevisionV2` (line 198) reassigned directly to `domain.ErrRevisionConflict`.

## Update-reachable error category coverage

### Catalog Update — 10 of 15 reachable

| Category | Reachable | Evidence |
| --- | --- | --- |
| `INVALID_ARGUMENT` | Yes | `Writer` shape gate; `catalogo.ErrInvalidArgument` on `ID == 0 \|\| expectedRevision == 0` |
| `NOT_FOUND` | **Yes — reverses Create's finding** | `buildV2UpdateCandidate`'s `s.repo.Get` and `ApplyCatalogMutation(OpUpdate)`'s `mutateSlice` `idx < 0` branch |
| `DUPLICATE` | Yes | `mapCatalogWriteError`'s unique-constraint (`23505`) classification |
| `INVALID_REFERENCE` | Yes | Same FK-violation classification; also `validateApplicabilityRecord`'s reference checks for APLICABILIDAD |
| `VALIDATION` | Yes | `ApplyCatalogMutation`'s own error, returned unwrapped in `buildV2UpdateCandidate` before `WrapInvalidCatalog(next.Validate())` — same mechanism Create's catalog VALIDATION already used |
| `INTEGRITY` | **No** | `domain.ErrResourceIntegrity` has zero references anywhere in `internal/postgres/catalog_admin_*.go` (grep-verified) — resource-repository-only sentinel. Narrows the explore.md-estimated 11 to 10; see `design.md`'s "Discrepancy with explore.md, stated honestly" |
| `INVALID_CATALOG` | Yes | `domain.WrapInvalidCatalog(next.Validate())` at the end of `buildV2UpdateCandidate` |
| `UNAVAILABLE` | Yes | `catalogo.ErrCatalogWriterUnavailable`/ctx cancellation |
| `INTERNAL` | Yes | Unclassified/driver errors |
| `CONFLICT` | **Yes — this change's fix** | `casUpdateRevision`'s present-but-different-revision branch and `updateApplicabilityAggregateV2`'s APLICABILIDAD path, both now aliased to `domain.ErrRevisionConflict` |
| `IMMUTABLE_CODE` | **Yes — new** | `buildV2UpdateCandidate`'s referenced-code-rename guard (`domain.ErrCodeImmutable`), proven via kind-registry lookup (`TestCatalogUpdate_ImmutableCode_ViaKindRegistryLookup`), not a hardcoded kind |

Excluded, never claimed reachable: `IDENTITY_CONFLICT`, `INVALID_LIFECYCLE`, `REACTIVATION_IMPOSSIBLE`, `IN_USE` (all Reactivate/Delete-only sentinels).

### Resource Update — 9 of 15 reachable (matches explore.md exactly)

| Category | Reachable | Evidence |
| --- | --- | --- |
| `INVALID_ARGUMENT` | Yes | `Writer` shape gate; `command.ID <= 0 \|\| expectedRevision == 0` guard |
| `NOT_FOUND` | Yes | `domain.ErrResourceNotFound`, passed through unwrapped |
| `DUPLICATE` | Yes | `domain.ErrDuplicateResource`, passed through |
| `INVALID_REFERENCE` | Yes | `domain.ErrResourceReference`, passed through |
| `VALIDATION` | Yes | `domain.NewResource`'s validation failure, returned before any repository call |
| `INTEGRITY` | Yes | `domain.ErrResourceIntegrity`, passed through |
| `UNAVAILABLE` | Yes | ctx `Canceled`/`DeadlineExceeded` |
| `INTERNAL` | Yes | Fallback `fmt.Errorf` wrap for any error outside the explicitly classified five |
| `CONFLICT` | Yes — **already correct before this change** | `domain.ErrResourceRevisionConflict`, returned directly by `resource_repository_crud.go` on stale CAS. No postgres fix needed on the resource side |

Excluded, never claimed reachable: `IDENTITY_CONFLICT`, `INVALID_LIFECYCLE`, `REACTIVATION_IMPOSSIBLE`, `IN_USE` (Reactivate/Delete-only), `INVALID_CATALOG` (no resource-Update call site — `recursos.Service.UpdateRevision` never calls `domain.WrapInvalidCatalog`), `IMMUTABLE_CODE` (catalog-only concept).

## CONFLICT-wiring fix — verification

- **Diff scope**: exactly the two sentinel value reassignments plus one updated doc comment in `internal/postgres/catalog_admin_repository_v2.go` (7 insertions, 5 deletions — `git diff --stat`). No other line in the file changed.
- **Direct reassignment, not wrapping**: `errApplicabilityStaleRevision = domain.ErrRevisionConflict` and `errCatalogStaleRevisionV2 = domain.ErrRevisionConflict` — both identifiers now the identical value as `domain.ErrRevisionConflict`.
- **14 existing `errors.Is` call sites re-grepped and confirmed as the complete set** of references to either sentinel (`catalog_admin_repository_v2_integration_test.go`, lines 111, 407, 414, 603, 746, 753, 1027, 1037, 1047 for `errCatalogStaleRevisionV2`; line 111 area for `errApplicabilityStaleRevision`). None compares via `==` or `.Error()` string content — every site uses `errors.Is`, which trivially still succeeds since the compared value is now the exact same identity.
- **Compile-level proof**: `go build ./internal/postgres/...` and `go vet ./internal/postgres/...` both clean.
- **Live-DB proof — gap, honestly documented**: this sandbox has no `GARFEX_TEST_DSN`-exposed PostgreSQL instance. `go test ./internal/postgres -run 'TestCatalogAdminRepositoryV2|TestApplicability' -count=1` ran; the one test requiring a live DB (`TestApplicabilityAggregateV2Integration`) reported `SKIP: GARFEX_TEST_DSN not set`; the one non-DB test in scope (`TestCatalogAdminRepositoryV2ConstructorConformance`) passed. The zero-breakage claim for the 14 `errors.Is` sites is **structurally guaranteed by the reassignment itself** (identity comparison, not behavioral change) and grep-verified as the complete reference set, but was not exercised against a live database in this session. Recommend running `GARFEX_TEST_DSN=<isolated-app-dsn> GARFEX_ADMIN_TEST_DSN=<isolated-admin-dsn> go test ./internal/postgres -run 'TestCatalogAdminRepositoryV2|TestApplicability' -count=1` against an isolated database (never the `garfex_pgdata` volume) before merge, per this repository's own established convention for PostgreSQL evidence.

## Open questions from design.md — resolved

1. **`Adapter.UpdateResource` performs no create-confirm read.** Proven by `TestResourceUpdate_NoConfirmRead_CallCountAssertion`: asserts `UpdateRevision` is called exactly once and `Get` exactly zero times, across the success path and 5 injected-failure paths.
2. **`IMMUTABLE_CODE` evidence is registry-driven, not hardcoded.** Proven by `TestCatalogUpdate_ImmutableCode_ViaKindRegistryLookup`: iterates `domain.NewCatalogRegistry().Kind(...)`'s `Fields`, locates the field with `Immutable == domain.ImmutableOnceReferenced`, and drives the injected case from that lookup.

## Actor attribution

`Actor` reaches `internal/core/diagnostics.go`'s `core.WithActor`/`ActorFrom` seam on both the success and failure path for both `UpdateCatalog` and `UpdateResource` (`TestCatalogUpdate_ActorReachesDiagnosticSeam`, `TestResourceUpdate_ActorReachesDiagnosticSeam`) — reusing Create's mechanism verbatim, no redesign. Never persisted, no new column or migration, never appears on a public DTO.

## Zero-touch confirmation

- `git diff --stat -- cmd/garfex internal/tui` — empty.
- `rg -l resourcecore cmd/garfex internal/tui` — no matches.
- `internal/app/*`, `internal/domain/*` — zero changed lines (confirmed via `git diff --stat`, not shown above since untouched).

## Compiled surface

`WriteCapabilities`/`Writer` compile exactly 4 methods: `CreateCatalog`, `CreateResource`, `UpdateCatalog`, `UpdateResource` (`TestWriter_NoUngraduatedMethodExported`, reflection-based). No `Deactivate`/`Reactivate`/`HardDelete` symbol, stub, or discoverable artifact exists.

## Full-suite proof

`go build ./...`, `gofmt -l .`, `go vet ./...`, and `go test ./... -count=1` all pass for every real project package. The only failure in the full-suite run is `agent/skills/golang-cli/assets/examples` — a pre-existing, untracked example asset package with unrelated missing third-party dependencies (`github.com/fatih/color`, `fsnotify`, `cobra`, `viper`, `github.com/you/myapp/cmd`), unconnected to this change and already broken before it (present in `git status` as untracked prior to this change).

## Changed-line summary

870 insertions, 19 deletions across `resourcecore/{write_types,writer,copy,writer_test,external_test}.go`, `internal/bridge/resourcecore/{adapter,adapter_test}.go`, `internal/postgres/catalog_admin_repository_v2.go` — 8 files, ~889 total changed lines, split across the U1/U2/U3 stacked chain per the review-budget forecast.
