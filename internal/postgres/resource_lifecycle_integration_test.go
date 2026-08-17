package postgres

import (
	"context"
	"errors"
	"os"
	"testing"

	"github.com/GARFEX33/garfex-costos-unitarios/internal/domain"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestResourceRepositoryLifecycleIntegration(t *testing.T) {
	dsn := os.Getenv("GARFEX_TEST_DSN")
	if dsn == "" {
		t.Skip("GARFEX_TEST_DSN not set")
	}
	ctx := context.Background()
	p, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("connect to PostgreSQL: %v", err)
	}
	defer p.Close()
	cleanupResources(ctx, t, p)
	defer cleanupResources(ctx, t, p)

	catalog := domain.SeedResourceCatalog()
	resource := mustCreateResource(t, catalog, conductoresScope, "M", []domain.ResourceAttributeValue{
		domain.OptionValue("conductor_material", "COBRE"),
		domain.OptionValue("gauge", "10 AWG"),
		domain.OptionValue("insulation", "THW"),
		domain.OptionValue("color", "NEGRO"),
		domain.OptionValue("voltage", "600 V"),
	})
	repo := NewResourceRepository(p)
	if err := repo.Create(ctx, resource); err != nil {
		t.Fatalf("Create() = %v", err)
	}
	stored, err := repo.Get(ctx, resource.ClassCode, resource.IdentityKey)
	if err != nil {
		t.Fatalf("Get() = %v", err)
	}

	deactivated, err := repo.Deactivate(ctx, stored.ID)
	if err != nil || !deactivated.Changed || deactivated.Resource.Active {
		t.Fatalf("Deactivate() = %+v, %v; want changed inactive result", deactivated, err)
	}
	repeated, err := repo.Deactivate(ctx, stored.ID)
	if err != nil || repeated.Changed || repeated.Resource.Active {
		t.Fatalf("repeated Deactivate() = %+v, %v; want inactive no-op", repeated, err)
	}
	if active, err := repo.Search(ctx, domain.SearchCriteria{FamilyCode: "CONDUCTORES"}); err != nil || len(active) != 0 {
		t.Fatalf("default Search() after Deactivate = %#v, %v; want no rows", active, err)
	}
	inactive, err := repo.Search(ctx, domain.SearchCriteria{FamilyCode: "CONDUCTORES", LifecycleScope: domain.LifecycleScopeInactive})
	if err != nil || len(inactive) != 1 || inactive[0].Active {
		t.Fatalf("inactive Search() = %#v, %v; want one inactive resource", inactive, err)
	}

	if _, err := p.Exec(ctx, `UPDATE public.resource_families SET active=FALSE WHERE code='CONDUCTORES'`); err != nil {
		t.Fatalf("deactivate catalog dependency: %v", err)
	}
	defer func() {
		if _, err := p.Exec(ctx, `UPDATE public.resource_families SET active=TRUE WHERE code='CONDUCTORES'`); err != nil {
			t.Errorf("restore catalog dependency: %v", err)
		}
	}()
	if _, err := repo.Reactivate(ctx, stored.ID, resource.IdentityKey); !errors.Is(err, domain.ErrResourceReference) {
		t.Fatalf("Reactivate() with inactive catalog = %v, want ErrResourceReference", err)
	}
	stillInactive, err := repo.Get(ctx, resource.ClassCode, resource.IdentityKey)
	if err != nil || stillInactive.Active {
		t.Fatalf("Get() after failed Reactivate = %+v, %v; want inactive", stillInactive, err)
	}
	if _, err := p.Exec(ctx, `UPDATE public.resource_families SET active=TRUE WHERE code='CONDUCTORES'`); err != nil {
		t.Fatalf("restore catalog dependency: %v", err)
	}

	reactivated, err := repo.Reactivate(ctx, stored.ID, resource.IdentityKey)
	if err != nil || !reactivated.Changed || !reactivated.Resource.Active {
		t.Fatalf("Reactivate() = %+v, %v; want changed active result", reactivated, err)
	}
	if _, err := repo.Deactivate(ctx, stored.ID); err != nil {
		t.Fatalf("Deactivate() before conflict = %v", err)
	}
	if _, err := repo.Reactivate(ctx, stored.ID, "conflicting-identity"); !errors.Is(err, domain.ErrResourceIntegrity) {
		t.Fatalf("Reactivate() identity conflict = %v, want ErrResourceIntegrity", err)
	}
	var originalActive bool
	if err := p.QueryRow(ctx, `SELECT active FROM public.recursos WHERE id=$1`, stored.ID).Scan(&originalActive); err != nil || originalActive {
		t.Fatalf("original active after conflict = %v, %v; want inactive", originalActive, err)
	}
}
