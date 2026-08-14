package tui

import (
	"errors"
	"strings"
	"testing"

	"github.com/GARFEX33/garfex-costos-unitarios/internal/domain"
	"github.com/shopspring/decimal"
)

func TestResourcesHandler(t *testing.T) {
	value := int64(25)
	resource := domain.Resource{
		ClassCode: "MATERIAL", FamilyCode: "CEMENT", NaturalUnit: "kg", IdentityKey: "MATERIAL|CEMENT|kg|25",
		Attributes: []domain.ResourceAttributeValue{
			{AttributeCode: "strength", Type: domain.ValueTypeInteger, Integer: &value},
			domain.DecimalValue("density", "2.40"),
			domain.QuantityValue("length", "3.5", "m"),
		},
	}
	repositoryError := errors.New("database unavailable")
	cases := []struct {
		name     string
		resource domain.Resource
		err      error
		want     string
		wantErr  string
	}{
		{name: "renders technical detail", resource: resource, want: "Recurso\nUnidad natural: kg\nAtributos técnicos:\n- density: 2.4\n- length: 3.5 m\n- strength: 25\n"},
		{name: "returns repository error", err: repositoryError, wantErr: "recursos: database unavailable"},
		{name: "returns not found", err: domain.ErrResourceNotFound, wantErr: "recursos: resource not found"},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			// fakeResourceGetter is defined in resource_editor_test.go
			// (same package) — reused directly rather than duplicated here,
			// per this project's established convention of not duplicating
			// test fakes across files in the same package.
			getter := &fakeResourceGetter{resource: tt.resource, err: tt.err}
			msg := Resources(getter, "MATERIAL", "MATERIAL|CEMENT|kg|25")()().(resultMsg)
			if tt.wantErr != "" {
				if msg.err == nil || msg.err.Error() != tt.wantErr {
					t.Fatalf("error = %v, want %q", msg.err, tt.wantErr)
				}
				return
			}
			if msg.err != nil || msg.text != tt.want {
				t.Fatalf("result = (%q, %v), want (%q, nil)", msg.text, msg.err, tt.want)
			}
			for _, forbidden := range []string{"UnitCost", "Unit cost", "100.00", "Supplier", "Provider", "precio", "MATERIAL|CEMENT|kg|25", "Identity key", "classCode", "identityKey"} {
				if strings.Contains(msg.text, forbidden) {
					t.Fatalf("commercial field %q leaked in %q", forbidden, msg.text)
				}
			}
		})
	}
}

func TestRenderResourceDetailIsDeterministic(t *testing.T) {
	resource := domain.Resource{FamilyCode: "F", NaturalUnit: "u", IdentityKey: "k", Attributes: []domain.ResourceAttributeValue{
		domain.DecimalValue("z", "1.0"), domain.OptionValue("a", "X"),
	}}
	if first, second := renderResourceDetail(resource), renderResourceDetail(resource); first != second {
		t.Fatalf("render differs: %q != %q", first, second)
	}
	if !strings.Contains(renderResourceDetail(resource), decimal.RequireFromString("1.0").String()) {
		t.Fatal("decimal attribute missing")
	}
}
