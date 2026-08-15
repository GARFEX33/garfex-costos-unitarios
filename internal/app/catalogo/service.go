// Package catalogo provides the catalog-structure administration use cases
// (design §4, tasks 5.1/5.2): the generic descriptor-driven read/write API
// consumed by the future TUI admin engine (PR6+) and the AC#21 e2e proof
// (PR10). Service holds exactly one in-memory ResourceCatalog snapshot — the
// same value main.go loads at boot via postgres.LoadResourceCatalog — used
// SOLELY to validate a pending mutation against the whole catalog's
// structural invariants before persisting it (design D9); it is never the
// read source for List/Get (see their doc comments).
package catalogo

import (
	"context"
	"errors"
	"sync"

	"github.com/GARFEX33/garfex-costos-unitarios/internal/domain"
)

// ErrInvalidArgument is returned when a caller omits a required lookup key
// (e.g. a zero record ID), mirroring internal/app/recursos.ErrInvalidArgument.
var ErrInvalidArgument = errors.New("catalog lookup argument is required")

// Service implements the catalog-structure admin use cases (design §4).
type Service struct {
	mu       sync.Mutex
	repo     domain.CatalogAdminRepository
	registry domain.CatalogRegistry
	snapshot domain.ResourceCatalog
}

// NewService returns a Service backed by repo, using registry to describe
// every administrable CatalogKind and snapshot as the initial in-memory
// catalog state (design §4) — normally the exact value postgres.
// LoadResourceCatalog returned at boot.
func NewService(repo domain.CatalogAdminRepository, registry domain.CatalogRegistry, snapshot domain.ResourceCatalog) *Service {
	return &Service{repo: repo, registry: registry, snapshot: snapshot}
}

// Kinds returns every registered CatalogKind (design §3) — the descriptor
// data the generic TUI admin engine renders from.
func (s *Service) Kinds() []domain.CatalogKind {
	return s.registry.Kinds()
}

// List returns kind's records matching filter, read directly from the
// repository. domain.ResourceCatalog carries no ID field (design D10), so
// only the repository can return the ID-bearing CatalogRecords a caller
// needs for a later Update/Deactivate/Reactivate/Delete/Dependencies call —
// the in-memory snapshot exists solely to run ApplyCatalogMutation/Validate,
// never to serve reads.
func (s *Service) List(ctx context.Context, kind domain.CatalogKindCode, filter domain.CatalogFilter) ([]domain.CatalogRecord, error) {
	return s.repo.List(ctx, kind, filter)
}

// Get returns one record by kind and repository-assigned id, read directly
// from the repository (see List's doc comment for why).
func (s *Service) Get(ctx context.Context, kind domain.CatalogKindCode, id int64) (domain.CatalogRecord, error) {
	if id == 0 {
		return domain.CatalogRecord{}, ErrInvalidArgument
	}
	return s.repo.Get(ctx, kind, id)
}

// Dependencies reports which other kinds reference id, read directly from
// the repository's dependency probes (design §6) — the data a guarded
// Desactivar/Eliminar confirmation renders.
func (s *Service) Dependencies(ctx context.Context, kind domain.CatalogKindCode, id int64) ([]domain.CatalogDependency, error) {
	return s.repo.Dependents(ctx, kind, id)
}

// ReferencedByResources reports whether id is referenced by at least one
// real resource instance, read directly from the repository's
// ReferencedByResources probe (design §6). It is the TUI-facing signal task
// 7.2's Código-immutability UI (and the guarded-delete flow, task 7.1) need
// to decide, respectively, whether to render a "código" field read-only and
// whether a hard delete must be blocked in favor of Desactivar — the same
// probe Update already consults internally, exposed here as its own method
// since a caller may need the answer BEFORE attempting a write (e.g. to
// decide how to render a question in the first place).
func (s *Service) ReferencedByResources(ctx context.Context, kind domain.CatalogKindCode, id int64) (bool, error) {
	return s.repo.ReferencedByResources(ctx, kind, id)
}

// Create validates rec as a new kind record against the whole catalog
// before persisting it (design D9): rec is applied to an in-memory COPY of
// the current snapshot via domain.ApplyCatalogMutation, the copy's
// Validate() re-checks every structural invariant, and only when that
// succeeds does Create call the repository — nothing persists on rejection,
// and the snapshot commits only once persistence itself also succeeds.
func (s *Service) Create(ctx context.Context, kind domain.CatalogKindCode, rec domain.CatalogRecord) (domain.CatalogRecord, error) {
	rec.Kind = kind

	s.mu.Lock()
	defer s.mu.Unlock()

	next, err := domain.ApplyCatalogMutation(s.snapshot, s.registry, domain.CatalogMutation{Op: domain.OpInsert, Record: rec})
	if err != nil {
		return domain.CatalogRecord{}, err
	}
	if err := next.Validate(); err != nil {
		return domain.CatalogRecord{}, err
	}

	id, err := s.repo.Insert(ctx, rec)
	if err != nil {
		return domain.CatalogRecord{}, err
	}

	s.snapshot = next
	rec.ID = id
	return rec, nil
}

// Update validates rec's changes against the whole catalog before
// persisting them (design D9), and enforces Código immutability's
// authoritative, service-level layer (design D11 layer 1): for every field
// marked domain.ImmutableOnceReferenced (every "código" field), a changed
// value is rejected with domain.ErrCodeImmutable when
// repo.ReferencedByResources reports the record is already in use by at
// least one resource — checked, and rejected, BEFORE calling repo.Update.
//
// When a código change IS allowed (not yet referenced), it is frozen back
// to its current value for the in-memory snapshot mutation only: every
// per-kind match/build closure in domain.ApplyCatalogMutation resolves an
// existing element by the mutation record's OWN identity-defining fields
// (catalog_mutation.go), so passing the already-renamed value would make
// the lookup miss the very element being renamed. The actual rename is
// still persisted for real — repo.Update always receives the caller's full,
// unfrozen rec — only the snapshot's own bookkeeping copy keeps the old
// identity, which is safe because List/Get never read the snapshot (see
// List's doc comment); it exists only to validate future mutations.
func (s *Service) Update(ctx context.Context, kind domain.CatalogKindCode, rec domain.CatalogRecord) (domain.CatalogRecord, error) {
	if rec.ID == 0 {
		return domain.CatalogRecord{}, ErrInvalidArgument
	}
	rec.Kind = kind

	s.mu.Lock()
	defer s.mu.Unlock()

	current, err := s.repo.Get(ctx, kind, rec.ID)
	if err != nil {
		return domain.CatalogRecord{}, err
	}

	def, ok := s.registry.Kind(kind)
	if !ok {
		return domain.CatalogRecord{}, domain.ErrCatalogKindUnknown
	}

	mutationRecord := rec
	mutationRecord.Values = cloneCatalogValues(rec.Values)
	for _, field := range def.Fields {
		if field.Immutable != domain.ImmutableOnceReferenced {
			continue
		}
		oldValue, hasOld := current.Values[field.Name]
		newValue, hasNew := rec.Values[field.Name]
		if !hasOld {
			continue
		}
		if hasNew && oldValue.Text != newValue.Text {
			referenced, err := s.repo.ReferencedByResources(ctx, kind, rec.ID)
			if err != nil {
				return domain.CatalogRecord{}, err
			}
			if referenced {
				return domain.CatalogRecord{}, domain.ErrCodeImmutable
			}
		}
		// Freeze to the current value for the snapshot mutation lookup — see
		// the method doc comment above.
		mutationRecord.Values[field.Name] = oldValue
	}

	next, err := domain.ApplyCatalogMutation(s.snapshot, s.registry, domain.CatalogMutation{Op: domain.OpUpdate, Record: mutationRecord})
	if err != nil {
		return domain.CatalogRecord{}, err
	}
	if err := next.Validate(); err != nil {
		return domain.CatalogRecord{}, err
	}

	if err := s.repo.Update(ctx, rec); err != nil {
		return domain.CatalogRecord{}, err
	}

	s.snapshot = next
	return rec, nil
}

// Deactivate soft-deletes kind's record id (design §1a/Risk#3): validated
// and persisted the same validate-before-persist way as Create/Update.
// Returns domain.ErrSoftDeleteUnsupported, unchanged, for a kind whose
// underlying ResourceCatalog struct has no Go Active field yet (see
// domain.CatalogKind.SoftDelete's doc comment) — nothing is persisted.
func (s *Service) Deactivate(ctx context.Context, kind domain.CatalogKindCode, id int64) error {
	return s.setActive(ctx, kind, id, false)
}

// Reactivate restores kind's record id — see Deactivate's doc comment.
func (s *Service) Reactivate(ctx context.Context, kind domain.CatalogKindCode, id int64) error {
	return s.setActive(ctx, kind, id, true)
}

func (s *Service) setActive(ctx context.Context, kind domain.CatalogKindCode, id int64, active bool) error {
	if id == 0 {
		return ErrInvalidArgument
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	current, err := s.repo.Get(ctx, kind, id)
	if err != nil {
		return err
	}

	op := domain.OpDeactivate
	if active {
		op = domain.OpReactivate
	}
	next, err := domain.ApplyCatalogMutation(s.snapshot, s.registry, domain.CatalogMutation{Op: op, Record: current})
	if err != nil {
		return err
	}
	if err := next.Validate(); err != nil {
		return err
	}

	if err := s.repo.SetActive(ctx, kind, id, active); err != nil {
		return err
	}

	s.snapshot = next
	return nil
}

// Delete permanently removes kind's record id: validated and persisted the
// same validate-before-persist way as Create/Update/Deactivate — a delete
// that would orphan a referencing catalog-structure row (design §6's
// dependency probes) is caught by Validate() before repo.Delete is ever
// called. The repository's own ErrCatalogInUse (design §6, PR7's job)
// remains the guard against a resource *instance* still referencing the
// record — a check List/Dependencies exposes for the guarded-delete UI,
// not one this generic method re-implements.
func (s *Service) Delete(ctx context.Context, kind domain.CatalogKindCode, id int64) error {
	if id == 0 {
		return ErrInvalidArgument
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	current, err := s.repo.Get(ctx, kind, id)
	if err != nil {
		return err
	}

	next, err := domain.ApplyCatalogMutation(s.snapshot, s.registry, domain.CatalogMutation{Op: domain.OpDelete, Record: current})
	if err != nil {
		return err
	}
	if err := next.Validate(); err != nil {
		return err
	}

	if err := s.repo.Delete(ctx, kind, id); err != nil {
		return err
	}

	s.snapshot = next
	return nil
}

// cloneCatalogValues returns a shallow copy of values — Update must never
// mutate the caller's own rec.Values map in place, since rec itself (with
// its true, unfrozen values) is still needed for the later repo.Update call.
func cloneCatalogValues(values map[string]domain.CatalogValue) map[string]domain.CatalogValue {
	out := make(map[string]domain.CatalogValue, len(values))
	for k, v := range values {
		out[k] = v
	}
	return out
}
