package materiales

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/GARFEX33/garfex-costos-unitarios/internal/domain"
)

type fakeRepo struct {
	gotFamily, gotIdentity string
	material               domain.Material
	err                    error
}

func (f *fakeRepo) Create(context.Context, domain.Material) error { return nil }

func (f *fakeRepo) Get(_ context.Context, familyCode, identityKey string) (domain.Material, error) {
	f.gotFamily, f.gotIdentity = familyCode, identityKey
	return f.material, f.err
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
