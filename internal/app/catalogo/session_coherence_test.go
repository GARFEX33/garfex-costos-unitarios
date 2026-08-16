package catalogo

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/GARFEX33/garfex-costos-unitarios/internal/domain"
)

func TestCatalogMutationsMatchFreshCatalogAndFailuresRemainInvisible(t *testing.T) {
	persistErr := errors.New("persist failed")
	cases := []struct {
		name    string
		initial domain.ResourceFamily
		current domain.CatalogRecord
		mutate  func(*Service) error
		want    domain.ResourceFamily
	}{
		{name: "create", initial: family("CONDUCTORES", "Conductores", true),
			mutate: func(s *Service) error {
				_, err := s.Create(context.Background(), domain.KindFamily, familyRecord(0, "CANALIZACIONES", "Canalizaciones", true))
				return err
			},
			want: family("CANALIZACIONES", "Canalizaciones", true)},
		{name: "update including allowed code rename", initial: family("CONDUCTORES", "Conductores", true), current: familyRecord(5, "CONDUCTORES", "Conductores", true),
			mutate: func(s *Service) error {
				_, err := s.Update(context.Background(), domain.KindFamily, familyRecord(5, "CABLES", "Cables", true))
				return err
			},
			want: family("CABLES", "Cables", true)},
		{name: "deactivate", initial: family("CONDUCTORES", "Conductores", true), current: familyRecord(5, "CONDUCTORES", "Conductores", true),
			mutate: func(s *Service) error { return s.Deactivate(context.Background(), domain.KindFamily, 5) }, want: family("CONDUCTORES", "Conductores", false)},
		{name: "reactivate", initial: family("CONDUCTORES", "Conductores", false), current: familyRecord(5, "CONDUCTORES", "Conductores", false),
			mutate: func(s *Service) error { return s.Reactivate(context.Background(), domain.KindFamily, 5) }, want: family("CONDUCTORES", "Conductores", true)},
	}
	for _, tt := range cases {
		t.Run(tt.name+" publishes the fresh-session shape", func(t *testing.T) {
			initial := baseSnapshot()
			initial.Families = []domain.ResourceFamily{tt.initial}
			authority := domain.NewCatalogAuthority(initial)
			repo := &fakeCatalogAdminRepository{getFn: func(context.Context, domain.CatalogKindCode, int64) (domain.CatalogRecord, error) {
				return tt.current, nil
			}}
			if err := tt.mutate(NewServiceWithCatalogAuthority(repo, domain.NewCatalogRegistry(), authority)); err != nil {
				t.Fatal(err)
			}
			got, version := authority.Current()
			fresh := initial
			if tt.name == "create" {
				fresh.Families = append(append([]domain.ResourceFamily(nil), initial.Families...), tt.want)
			} else {
				fresh.Families = []domain.ResourceFamily{tt.want}
			}
			if version != 2 || !reflect.DeepEqual(got, fresh) {
				t.Fatalf("same-session catalog = %+v at v%d, fresh catalog = %+v", got.Families, version, fresh.Families)
			}
		})
		t.Run(tt.name+" persistence failure retains the previous version", func(t *testing.T) {
			initial := baseSnapshot()
			initial.Families = []domain.ResourceFamily{tt.initial}
			authority := domain.NewCatalogAuthority(initial)
			repo := &fakeCatalogAdminRepository{getFn: func(context.Context, domain.CatalogKindCode, int64) (domain.CatalogRecord, error) {
				return tt.current, nil
			},
				insertFn: func(context.Context, domain.CatalogRecord) (int64, error) { return 0, persistErr }, updateFn: func(context.Context, domain.CatalogRecord) error { return persistErr },
				setActiveFn: func(context.Context, domain.CatalogKindCode, int64, bool) error { return persistErr }}
			if err := tt.mutate(NewServiceWithCatalogAuthority(repo, domain.NewCatalogRegistry(), authority)); !errors.Is(err, persistErr) {
				t.Fatalf("mutation error = %v, want persistence failure", err)
			}
			got, version := authority.Current()
			if version != 1 || !reflect.DeepEqual(got, initial) {
				t.Fatalf("failed mutation exposed version %d", version)
			}
		})
	}
}

func family(code, name string, active bool) domain.ResourceFamily {
	return domain.ResourceFamily{ClassCode: "MATERIAL", Code: code, Name: name, Active: active}
}

func familyRecord(id int64, code, name string, active bool) domain.CatalogRecord {
	return domain.CatalogRecord{ID: id, Kind: domain.KindFamily, Active: active, Values: map[string]domain.CatalogValue{
		"class": {Ref: domain.CatalogRef{Kind: domain.KindClass, Code: "MATERIAL"}}, "code": {Text: code}, "name": {Text: name},
	}}
}
