package recursos

import (
	"context"
	"errors"
	"testing"

	"github.com/GARFEX33/garfex-costos-unitarios/internal/domain"
)

func TestServiceReactivateRejectsUnavailableCatalogBeforeRepositoryTransition(t *testing.T) {
	resource := domain.Resource{
		ID: 8, ClassCode: "MATERIAL", FamilyCode: "CONDUCTORES", TypeCode: "CABLE", NaturalUnit: "M",
		IdentityKey: "v1|resource", Active: false,
		Attributes: []domain.ResourceAttributeValue{domain.OptionValue("conductor_material", "COBRE")},
	}
	repo := &fakeLifecycleRepo{byID: resource}
	service := NewService(repo, domain.SeedResourceCatalog())

	result, err := service.Reactivate(context.Background(), resource.ID)
	if !errors.Is(err, domain.ErrResourceValidation) {
		t.Fatalf("Reactivate() error = %v, want catalog validation error", err)
	}
	if result.Resource.Active || repo.reactivateCalls != 0 {
		t.Fatalf("failed Reactivate() = %+v with %d repository transitions, want inactive and zero", result, repo.reactivateCalls)
	}
}

type fakeLifecycleRepo struct {
	fakeRepo
	byID            domain.Resource
	reactivateCalls int
}

func (f *fakeLifecycleRepo) GetByID(context.Context, int64) (domain.Resource, error) {
	return f.byID, nil
}

func (f *fakeLifecycleRepo) Reactivate(context.Context, int64, string) (domain.LifecycleResult, error) {
	f.reactivateCalls++
	return domain.LifecycleResult{}, nil
}
