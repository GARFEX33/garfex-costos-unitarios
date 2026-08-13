// Package materiales provides application use cases for the materials read model.
package materiales

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/GARFEX33/garfex-costos-unitarios/internal/domain"
)

// ErrInvalidArgument is returned when a material lookup key is incomplete.
var ErrInvalidArgument = errors.New("material lookup argument is required")

// Service implements read-only material use cases.
type Service struct {
	repo    domain.MaterialRepository
	catalog domain.MaterialsCatalog
}

// NewService returns a Service backed by repo, using catalog to resolve
// catalog-controlled concerns such as canonical presentation.
func NewService(repo domain.MaterialRepository, catalog domain.MaterialsCatalog) *Service {
	return &Service{repo: repo, catalog: catalog}
}

// Get returns a material by its family code and deterministic identity key.
func (s *Service) Get(ctx context.Context, familyCode, identityKey string) (domain.Material, error) {
	if strings.TrimSpace(familyCode) == "" || strings.TrimSpace(identityKey) == "" {
		return domain.Material{}, ErrInvalidArgument
	}
	m, err := s.repo.Get(ctx, familyCode, identityKey)
	if err != nil {
		if errors.Is(err, domain.ErrMaterialNotFound) {
			return domain.Material{}, domain.ErrMaterialNotFound
		}
		return domain.Material{}, fmt.Errorf("get material %s/%s: %w", familyCode, identityKey, err)
	}
	return m, nil
}

// Search returns materials matching criteria. Empty Text/FamilyCode/Filters
// are valid "match everything" inputs, so no argument validation is needed
// here — Limit/Offset clamping is a SQL-correctness concern handled by the
// repository.
func (s *Service) Search(ctx context.Context, criteria domain.SearchCriteria) ([]domain.Material, error) {
	materials, err := s.repo.Search(ctx, criteria)
	if err != nil {
		return nil, fmt.Errorf("search materials: %w", err)
	}
	return materials, nil
}

// Describe resolves the canonical presentation of material using the
// catalog-controlled configuration owned by its ProductType.
func (s *Service) Describe(material domain.Material) string {
	return s.catalog.Describe(material)
}
