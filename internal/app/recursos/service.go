// Package recursos provides application use cases for the unified resource
// master read model (materials, mano de obra, equipos/herramientas — design
// §4). It renames and supersedes the retired internal/app/materiales
// package: same thin passthrough shape, class-scoped lookups (design R1).
package recursos

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/GARFEX33/garfex-costos-unitarios/internal/domain"
)

// ErrInvalidArgument is returned when a resource lookup key is incomplete.
var ErrInvalidArgument = errors.New("resource lookup argument is required")

// Service implements read-only resource use cases.
type Service struct {
	repo      domain.ResourceRepository
	authority *domain.CatalogAuthority
}

// NewService returns a Service backed by repo, using catalog to resolve
// catalog-controlled concerns such as canonical presentation.
func NewService(repo domain.ResourceRepository, catalog domain.ResourceCatalog) *Service {
	return NewServiceWithCatalogAuthority(repo, domain.NewCatalogAuthority(catalog))
}

// NewServiceWithCatalogAuthority resolves catalog-controlled behavior from the
// current committed version.
func NewServiceWithCatalogAuthority(repo domain.ResourceRepository, authority *domain.CatalogAuthority) *Service {
	return &Service{repo: repo, authority: authority}
}

// Get returns a resource by its owning class code and deterministic
// identity key. classCode is the resource's ResourceClass.Code (design R1)
// — IdentityKey already encodes class|family|type, so class+key is the
// minimal complete lookup; a family code here will not match.
func (s *Service) Get(ctx context.Context, classCode, identityKey string) (domain.Resource, error) {
	if strings.TrimSpace(classCode) == "" || strings.TrimSpace(identityKey) == "" {
		return domain.Resource{}, ErrInvalidArgument
	}
	r, err := s.repo.Get(ctx, classCode, identityKey)
	if err != nil {
		if errors.Is(err, domain.ErrResourceNotFound) {
			return domain.Resource{}, domain.ErrResourceNotFound
		}
		return domain.Resource{}, fmt.Errorf("get resource %s/%s: %w", classCode, identityKey, err)
	}
	return r, nil
}

// Search returns resources matching criteria. Empty Text/ClassCode/
// FamilyCode/Filters are valid "match everything" inputs, so no argument
// validation is needed here — Limit/Offset clamping is a SQL-correctness
// concern handled by the repository.
func (s *Service) Search(ctx context.Context, criteria domain.SearchCriteria) ([]domain.Resource, error) {
	resources, err := s.repo.Search(ctx, criteria)
	if err != nil {
		return nil, fmt.Errorf("search resources: %w", err)
	}
	return resources, nil
}

// Describe resolves the canonical presentation of resource using the
// catalog-controlled configuration owned by its ResourceType.
func (s *Service) Describe(resource domain.Resource) string {
	catalog, _ := s.authority.Current()
	return catalog.Describe(resource)
}

// Create persists a new resource built by the caller via domain.NewResource
// (this method does not itself validate — NewResource already did).
func (s *Service) Create(ctx context.Context, resource domain.Resource) error {
	if err := s.repo.Create(ctx, resource); err != nil {
		if errors.Is(err, domain.ErrDuplicateResource) {
			return domain.ErrDuplicateResource
		}
		if errors.Is(err, domain.ErrResourceReference) {
			return domain.ErrResourceReference
		}
		return fmt.Errorf("create resource: %w", err)
	}
	return nil
}

// Update persists changes to an existing resource, identified by its stable
// ID (independent of IdentityKey, which the update itself may change).
func (s *Service) Update(ctx context.Context, resource domain.Resource) error {
	if resource.ID == 0 {
		return ErrInvalidArgument
	}
	if err := s.repo.Update(ctx, resource); err != nil {
		if errors.Is(err, domain.ErrResourceNotFound) {
			return domain.ErrResourceNotFound
		}
		if errors.Is(err, domain.ErrDuplicateResource) {
			return domain.ErrDuplicateResource
		}
		if errors.Is(err, domain.ErrResourceReference) {
			return domain.ErrResourceReference
		}
		return fmt.Errorf("update resource %d: %w", resource.ID, err)
	}
	return nil
}

// Delete soft-deletes a resource by its stable ID (toggles active=false —
// never a hard delete, per project decision: preserves future referential
// integrity once other modules start referencing resources).
func (s *Service) Delete(ctx context.Context, id int64) error {
	if id == 0 {
		return ErrInvalidArgument
	}
	if err := s.repo.SetActive(ctx, id, false); err != nil {
		if errors.Is(err, domain.ErrResourceNotFound) {
			return domain.ErrResourceNotFound
		}
		return fmt.Errorf("delete resource %d: %w", id, err)
	}
	return nil
}
