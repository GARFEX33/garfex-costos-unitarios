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

	gotUpdate domain.Material
	updateErr error

	gotSetActiveID    int64
	gotSetActiveValue bool
	setActiveErr      error
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

func (f *fakeRepo) Update(_ context.Context, material domain.Material) error {
	f.gotUpdate = material
	return f.updateErr
}

func (f *fakeRepo) SetActive(_ context.Context, id int64, active bool) error {
	f.gotSetActiveID, f.gotSetActiveValue = id, active
	return f.setActiveErr
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
			got, err := NewService(repo, domain.NewMaterialsCatalog()).Get(context.Background(), tt.family, tt.identity)
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
			got, err := NewService(repo, domain.NewMaterialsCatalog()).Search(context.Background(), tt.criteria)
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

func TestServiceUpdate(t *testing.T) {
	repositoryError := errors.New("connection lost")
	cases := []struct {
		name       string
		material   domain.Material
		repoErr    error
		wantErr    error
		wantCalled bool
	}{
		{name: "happy path calls through", material: domain.Material{ID: 1, FamilyCode: "CONDUCTORES", NaturalUnit: "M", IdentityKey: "CONDUCTORES|a"}, wantCalled: true},
		{name: "rejects zero id", material: domain.Material{FamilyCode: "CONDUCTORES", NaturalUnit: "M", IdentityKey: "CONDUCTORES|a"}, wantErr: ErrInvalidArgument},
		{name: "propagates not found", material: domain.Material{ID: 1}, repoErr: domain.ErrMaterialNotFound, wantErr: domain.ErrMaterialNotFound, wantCalled: true},
		{name: "propagates duplicate", material: domain.Material{ID: 1}, repoErr: domain.ErrDuplicateMaterial, wantErr: domain.ErrDuplicateMaterial, wantCalled: true},
		{name: "propagates reference error", material: domain.Material{ID: 1}, repoErr: domain.ErrMaterialReference, wantErr: domain.ErrMaterialReference, wantCalled: true},
		{name: "wraps repository error", material: domain.Material{ID: 1}, repoErr: repositoryError, wantErr: repositoryError, wantCalled: true},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			repo := &fakeRepo{updateErr: tt.repoErr}
			err := NewService(repo, domain.NewMaterialsCatalog()).Update(context.Background(), tt.material)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("Update() error = %v, want %v", err, tt.wantErr)
			}
			if !tt.wantCalled && repo.gotUpdate.ID != 0 {
				t.Fatalf("repository called with %+v", repo.gotUpdate)
			}
			if tt.wantCalled && !reflect.DeepEqual(repo.gotUpdate, tt.material) {
				t.Fatalf("repository called with %+v, want %+v", repo.gotUpdate, tt.material)
			}
			if tt.repoErr == repositoryError && !strings.Contains(err.Error(), "update material") {
				t.Fatalf("Update() error = %v, want prefix %q", err, "update material")
			}
		})
	}
}

func TestServiceDelete(t *testing.T) {
	repositoryError := errors.New("connection lost")
	cases := []struct {
		name       string
		id         int64
		repoErr    error
		wantErr    error
		wantCalled bool
	}{
		{name: "happy path calls through", id: 1, wantCalled: true},
		{name: "rejects zero id", id: 0, wantErr: ErrInvalidArgument},
		{name: "propagates not found", id: 1, repoErr: domain.ErrMaterialNotFound, wantErr: domain.ErrMaterialNotFound, wantCalled: true},
		{name: "wraps repository error", id: 1, repoErr: repositoryError, wantErr: repositoryError, wantCalled: true},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			repo := &fakeRepo{setActiveErr: tt.repoErr}
			err := NewService(repo, domain.NewMaterialsCatalog()).Delete(context.Background(), tt.id)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("Delete() error = %v, want %v", err, tt.wantErr)
			}
			if !tt.wantCalled && repo.gotSetActiveID != 0 {
				t.Fatalf("repository called with id %d", repo.gotSetActiveID)
			}
			if tt.wantCalled {
				if repo.gotSetActiveID != tt.id {
					t.Fatalf("repository called with id %d, want %d", repo.gotSetActiveID, tt.id)
				}
				if repo.gotSetActiveValue {
					t.Fatalf("repository called with active = %v, want false", repo.gotSetActiveValue)
				}
			}
			if tt.repoErr == repositoryError && !strings.Contains(err.Error(), "delete material") {
				t.Fatalf("Delete() error = %v, want prefix %q", err, "delete material")
			}
		})
	}
}
