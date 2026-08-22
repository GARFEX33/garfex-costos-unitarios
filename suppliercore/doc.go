// Package suppliercore is the public, consumer-neutral Go contract for the
// GARFEX Supplier Master Core. It is a translation boundary, not a second
// business authority: every read delegates to the existing authoritative
// application service (internal/modules/suppliers/app.Service) through a
// module-owned internal bridge (internal/bridge/suppliercore.Adapter). This
// package owns its DTOs, validates request shape, and defensively copies
// every value crossing the boundary; it never aliases an internal domain
// value, exposes a repository or connection pool, or duplicates business or
// validation rules already owned by the internal service — most notably the
// branch-ownership invariant enforced when listing a supplier's contacts
// scoped to one of its branches, which this package only observes the
// result of and never re-implements.
//
// # Shipped contract: read, plus Supplier Create
//
// This version exposes all read operations through Reader, covering all
// three Supplier Master entities: Supplier (get/search), Branch (get/list,
// scoped by SupplierID), and Contact (get/list, scoped by SupplierID and
// optionally filtered by BranchID). It additionally exposes two write
// operations through Writer: CreateSupplier and UpdateSupplier. Deactivate,
// Reactivate, and Delete/HardDelete are not exposed for Supplier, and no
// operation is exposed for Branch or Contact — each is a distinct, later
// change this package does not anticipate the shape of. The presence of
// this package's API is never permission to infer that any other operation
// is available. HardDelete in particular is permanently absent: no internal
// capability exists for Supplier, Branch, or Contact, so this is not
// deferred work.
//
// # Actor
//
// SupplierWriteRequest carries a required Actor field, mirroring
// resourcecore, even though the internal Supplier Master service has no
// actor-attribution concept of its own today. Actor is diagnostic-only and
// is not persisted or returned on any DTO in this version — it is passed
// through the internal core.WithActor mechanism as a foreseeable seed for a
// future audit-data change, not authorization and not a silent surprise if
// a later change persists it.
//
// # Update semantics
//
// UpdateSupplier replaces content; it does not patch it. Every content
// field of SupplierUpdateRequest is written exactly as supplied, so a field
// left empty clears the stored value instead of preserving it. A caller
// intending a partial edit must read the supplier first and resubmit the
// fields it wants to keep — this package never merges, because choosing
// what survives a partial write is a business decision it does not own.
//
// Update is also lifecycle-neutral and lifecycle-blind: it never changes
// Active, and it does not refuse a deactivated supplier. Editing an
// inactive supplier succeeds and leaves it inactive; reactivation is a
// separate operation this version does not expose.
//
// # No optimistic concurrency
//
// Unlike resourcecore, no entity in the Supplier Master carries a Revision
// field: the internal service has none today, and this package reflects
// that reality rather than inventing one. A concurrent write between two
// reads of the same record is not detectable through this package.
// UpdateSupplier exercises that gap directly: the internal service reads
// the current supplier, rebuilds its details, and writes the result back
// with no transaction and no compare-and-set, so two concurrent updates
// resolve last-write-wins and the losing writer's fields disappear without
// any error. This package reports the risk rather than closing it —
// closing it would mean inventing a Revision the internal service does not
// have.
//
// # Errors
//
// Every failure returned by this package is an Error carrying one of five
// stable ErrorCode categories (see Code and IsCode): NOT_FOUND, VALIDATION,
// CONFLICT, INVALID_ARGUMENT, and INTERNAL — deliberately smaller than
// resourcecore's fixed fifteen, proportional to what this module's simpler
// internal error taxonomy actually distinguishes. INVALID_ARGUMENT is
// reserved for request-shape rejection inside this package, before the
// internal service is ever called; VALIDATION, NOT_FOUND, and CONFLICT
// originate from the internal service and cross the bridge unchanged in
// category. Error never formats or retains a PostgreSQL message, SQLSTATE,
// constraint, table, column, or driver type; INTERNAL always carries a
// fixed, generic message regardless of the underlying cause. NOT_FOUND is
// write-reachable for the first time in this version; UpdateSupplier
// against an unknown id returns it.
package suppliercore
