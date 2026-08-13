package tui

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/GARFEX33/garfex-costos-unitarios/internal/domain"
	"github.com/shopspring/decimal"
)

func TestMaterialsHandler(t *testing.T) {
	value := int64(25)
	material := domain.Material{
		FamilyCode: "CEMENT", NaturalUnit: "kg", IdentityKey: "CEMENT|kg|25",
		Attributes: []domain.MaterialAttributeValue{
			{AttributeCode: "strength", Type: domain.ValueTypeInteger, Integer: &value},
			domain.DecimalValue("density", "2.40"),
			domain.QuantityValue("length", "3.5", "m"),
		},
	}
	repositoryError := errors.New("database unavailable")
	cases := []struct {
		name     string
		material domain.Material
		err      error
		want     string
		wantErr  string
	}{
		{name: "renders technical detail", material: material, want: "Material\nUnidad natural: kg\nAtributos técnicos:\n- density: 2.4\n- length: 3.5 m\n- strength: 25\n"},
		{name: "returns repository error", err: repositoryError, wantErr: "materiales: database unavailable"},
		{name: "returns not found", err: domain.ErrMaterialNotFound, wantErr: "materiales: material not found"},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			getter := &fakeMaterialGetter{material: tt.material, err: tt.err}
			msg := Materials(getter, "CEMENT", "CEMENT|kg|25")()().(resultMsg)
			if tt.wantErr != "" {
				if msg.err == nil || msg.err.Error() != tt.wantErr {
					t.Fatalf("error = %v, want %q", msg.err, tt.wantErr)
				}
				return
			}
			if msg.err != nil || msg.text != tt.want {
				t.Fatalf("result = (%q, %v), want (%q, nil)", msg.text, msg.err, tt.want)
			}
			for _, forbidden := range []string{"UnitCost", "Unit cost", "100.00", "Supplier", "Provider", "precio", "CEMENT|kg|25", "Identity key", "familyCode", "identityKey"} {
				if strings.Contains(msg.text, forbidden) {
					t.Fatalf("commercial field %q leaked in %q", forbidden, msg.text)
				}
			}
		})
	}
}

// fakeMaterialGetter is the shared materialGetter test double for this
// package (tui) — reused as-is by materials_workspace_adapter_test.go
// rather than duplicated, per this project's established convention of not
// duplicating test fakes across files in the same package. It records the
// most recent call's arguments and a call count so callers can assert on
// exactly what/how many times Get was invoked.
type fakeMaterialGetter struct {
	material domain.Material
	err      error

	callCount      int
	gotFamilyCode  string
	gotIdentityKey string
}

func (f *fakeMaterialGetter) Get(_ context.Context, familyCode, identityKey string) (domain.Material, error) {
	f.callCount++
	f.gotFamilyCode = familyCode
	f.gotIdentityKey = identityKey
	return f.material, f.err
}

func TestRenderMaterialDetailIsDeterministic(t *testing.T) {
	material := domain.Material{FamilyCode: "F", NaturalUnit: "u", IdentityKey: "k", Attributes: []domain.MaterialAttributeValue{
		domain.DecimalValue("z", "1.0"), domain.OptionValue("a", "X"),
	}}
	if first, second := renderMaterialDetail(material), renderMaterialDetail(material); first != second {
		t.Fatalf("render differs: %q != %q", first, second)
	}
	if !strings.Contains(renderMaterialDetail(material), decimal.RequireFromString("1.0").String()) {
		t.Fatal("decimal attribute missing")
	}
}
