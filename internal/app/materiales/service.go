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
	repo domain.MaterialRepository
}

// NewService returns a Service backed by repo.
func NewService(repo domain.MaterialRepository) *Service {
	return &Service{repo: repo}
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
