package materiales

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/GARFEX33/garfex-costos-unitarios/internal/domain"
)

type fakeRepo struct {
	gotFamily, gotIdentity string
	material               domain.Material
	err                    error

	gotCriteria domain.SearchCriteria
	materials   []domain.Material
	searchErr   error
}

func (f *fakeRepo) Create(context.Context, domain.Material) error { return nil }

func (f *fakeRepo) Get(_ context.Context, familyCode, identityKey string) (domain.Material, error) {
	f.gotFamily, f.gotIdentity = familyCode, identityKey
	return f.material, f.err
}

func (f *fakeRepo) Search(_ context.Context, criteria domain.SearchCriteria) ([]domain.Material, error) {
	f.gotCriteria = criteria
	return f.materials, f.searchErr
}

func TestServiceGet(t *testing.T) {
	repositoryError := errors.New("connection lost")
	cases := []struct {
		name       string
		family     string
		identity   string
		material   domain.Material
		repoErr    error
		wantErr    error
		wantCalled bool
	}{
		{name: "returns technical material", family: "CEMENT", identity: "CEMENT|kg", material: domain.Material{FamilyCode: "CEMENT", NaturalUnit: "kg", IdentityKey: "CEMENT|kg"}, wantCalled: true},
		{name: "rejects missing family", identity: "CEMENT|kg", wantErr: ErrInvalidArgument},
		{name: "rejects missing identity", family: "CEMENT", wantErr: ErrInvalidArgument},
		{name: "propagates not found", family: "CEMENT", identity: "missing", repoErr: domain.ErrMaterialNotFound, wantErr: domain.ErrMaterialNotFound, wantCalled: true},
		{name: "wraps repository error", family: "CEMENT", identity: "CEMENT|kg", repoErr: repositoryError, wantErr: repositoryError, wantCalled: true},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			repo := &fakeRepo{material: tt.material, err: tt.repoErr}
			got, err := NewService(repo).Get(context.Background(), tt.family, tt.identity)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("Get() error = %v, want %v", err, tt.wantErr)
			}
			if !tt.wantCalled && (repo.gotFamily != "" || repo.gotIdentity != "") {
				t.Fatalf("repository called with %q/%q", repo.gotFamily, repo.gotIdentity)
			}
			if tt.wantErr == nil && !reflect.DeepEqual(got, tt.material) {
				t.Fatalf("Get() = %+v, want %+v", got, tt.material)
			}
		})
	}
}

func TestServiceSearch(t *testing.T) {
	repositoryError := errors.New("connection lost")
	cases := []struct {
		name      string
		criteria  domain.SearchCriteria
		materials []domain.Material
		repoErr   error
		wantErr   error
	}{
		{name: "passes results through unchanged", criteria: domain.SearchCriteria{Text: "THW"}, materials: []domain.Material{{FamilyCode: "CONDUCTORES", NaturalUnit: "M", IdentityKey: "CONDUCTORES|a"}, {FamilyCode: "CONDUCTORES", NaturalUnit: "M", IdentityKey: "CONDUCTORES|b"}}},
		{name: "passes empty criteria through unchanged", criteria: domain.SearchCriteria{}, materials: nil},
		{name: "wraps repository error", criteria: domain.SearchCriteria{Text: "THW"}, repoErr: repositoryError, wantErr: repositoryError},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			repo := &fakeRepo{materials: tt.materials, searchErr: tt.repoErr}
			got, err := NewService(repo).Search(context.Background(), tt.criteria)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("Search() error = %v, want %v", err, tt.wantErr)
			}
			if tt.wantErr != nil && !strings.Contains(err.Error(), "search materials:") {
				t.Fatalf("Search() error = %v, want prefix %q", err, "search materials:")
			}
			if !reflect.DeepEqual(repo.gotCriteria, tt.criteria) {
				t.Fatalf("repository called with %+v, want %+v", repo.gotCriteria, tt.criteria)
			}
			if tt.wantErr == nil && !reflect.DeepEqual(got, tt.materials) {
				t.Fatalf("Search() = %+v, want %+v", got, tt.materials)
			}
		})
	}
}
