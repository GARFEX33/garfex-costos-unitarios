// Package resourcecore is the public, consumer-neutral Go contract for the
// GARFEX Resource Master Core. It is a translation boundary, not a second
// business authority: every read delegates to the existing authoritative
// application services (internal/app/catalogo.Service and
// internal/app/recursos.Service) through a module-owned internal bridge
// (internal/bridge/resourcecore.Adapter). This package owns its DTOs,
// validates request shape, and defensively copies every value crossing the
// boundary in both directions; it never aliases an internal domain value,
// exposes a repository, connection pool, DSN, or publication control, or
// duplicates business, presentation, or validation rules already owned by
// Core.
//
// # Shipped contract: READ only
//
// This version exposes read-only operations through Reader: active catalog
// classes, catalog descriptors, catalog list/get, resource search/get, and
// canonical resource presentation (delegated to recursos.Service.Describe,
// never reconstructed here). Reader has no Create, Update, Deactivate,
// Reactivate, Delete, Publish, Reload, repository accessor, or mutable
// authority accessor. Public WRITE is a distinct, later, operation-gated
// change: each write operation graduates independently only after it
// separately proves lifecycle parity, atomic applicability behavior,
// revision-based compare-and-swap, guarded-delete policy where applicable,
// and persistence/authority equivalence. The presence of this package's
// read API is never permission to infer that any write operation is
// available.
//
// # Lifecycle semantics visible through READ
//
// All 11 registered catalog kinds — CLASE, FAMILIA, TIPO, CARACTERISTICA,
// CONJUNTO_OPCIONES, OPCION, RELACION_OPCIONES, UNIDAD, POLITICA_UNIDAD,
// APLICABILIDAD, and PRESENTACION — are lifecycle-capable in Core and are
// projected uniformly: every CatalogRecord reports Active and Revision, and
// an APLICABILIDAD record additionally reports its complete ordered Rules.
// Deactivated records stay readable under an explicit inactive/all lifecycle
// scope; this package draws no distinction between "inactive" and "deleted"
// on its own — deactivation retains the record, hard delete removes it from
// every lifecycle-scoped read.
//
// # Identity
//
// A resource's IdentityV1 is its canonical, durable business identity,
// derived from class, family, type, and identity-participating attributes.
// It survives compatible persistence changes, including the migration 8
// revision column. A catalog record's ID is an opaque, hash-derived
// reference only: it is not guaranteed collision-free or stable after an
// allowed natural-code change on that record. A consumer that needs a
// durable, cross-time catalog reference keys on the record's natural
// identity fields (CatalogDescriptor.IdentityFields), never on ID.
//
// # Errors
//
// Every failure returned by this package is an Error carrying one of the
// fifteen stable ErrorCode categories (see Code and IsCode). All fifteen
// categories are fixed at this read release so a later operation-gated
// WRITE change never renumbers or redefines an existing one; today's
// read-only surface actually reaches only a subset of them —
// INVALID_ARGUMENT, NOT_FOUND, INTEGRITY, INVALID_CATALOG, UNAVAILABLE, and
// INTERNAL. The remaining categories (DUPLICATE, INVALID_REFERENCE,
// VALIDATION, IDENTITY_CONFLICT, INVALID_LIFECYCLE,
// REACTIVATION_IMPOSSIBLE, IN_USE, IMMUTABLE_CODE, and CONFLICT) describe
// write outcomes and become reachable only once the corresponding write
// operation ships. Error does not implement Unwrap, does not expose a
// Cause, and never formats or retains a PostgreSQL message, SQLSTATE,
// constraint, table, column, driver type, or other technical detail; those
// causes are recorded only behind the internal bridge's non-public
// diagnostic seam.
//
// # Writer topology and freshness
//
// Exactly one process is the authoritative writer for Resource Master
// mutations; this package does not support or claim safe independent
// multi-process writers. A Reader observes a coherent snapshot as of its
// backing capability's most recent read — there is no LISTEN/NOTIFY,
// polling, or automatic cross-process refresh. A process other than the
// authoritative writer must explicitly reconstruct its bridge or restart to
// observe a write made elsewhere; Reader intentionally has no Reload
// method, because publication control is not a consumer capability.
//
// # Migration 8 compatibility
//
// Migration 000008_resource_revisions adds an additive, backfilled
// "revision BIGINT NOT NULL DEFAULT 1 CHECK (revision > 0)" column to
// recursos and all 11 catalog parent tables. This package's DTOs project
// that column wherever it exists (CatalogRecord.Revision, Resource.Revision)
// as Revision uint64. The column is additive and backward compatible: it
// changes no identity-v1 value, rewrites no existing row, and does not
// affect a process running against a database that has not yet applied it.
// This package never derives a revision from updated_at, xmin, or a
// hash-derived catalog ID.
package resourcecore
