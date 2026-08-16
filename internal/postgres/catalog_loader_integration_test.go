package postgres

import (
	"context"
	"os"
	"reflect"
	"testing"

	"github.com/GARFEX33/garfex-costos-unitarios/internal/domain"
	"github.com/jackc/pgx/v5/pgxpool"
)

// TestLoadResourceCatalogIntegration proves LoadResourceCatalog fully
// hydrates the real, seeded catalog (migrations 000002+000003) — including
// nested Rules from the new resource_attribute_rules table — and that its
// declaration order matches domain.SeedResourceCatalog()'s Go literal via
// the new display_order columns (design §1d/§2). Skips per this project's
// existing GARFEX_TEST_DSN convention.
func TestLoadResourceCatalogIntegration(t *testing.T) {
	dsn := os.Getenv("GARFEX_TEST_DSN")
	if dsn == "" {
		t.Skip("GARFEX_TEST_DSN not set")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("connect to PostgreSQL: %v", err)
	}
	t.Cleanup(pool.Close)

	catalog, err := LoadResourceCatalog(ctx, pool)
	if err != nil {
		t.Fatalf("LoadResourceCatalog() error = %v, want nil", err)
	}

	t.Run("classes hydrated in declaration order", func(t *testing.T) {
		var codes []string
		for _, class := range catalog.Classes {
			codes = append(codes, class.Code)
		}
		want := []string{"MATERIAL", "MANO_DE_OBRA", "EQUIPO_HERRAMIENTA"}
		if !reflect.DeepEqual(codes, want) {
			t.Fatalf("class codes = %v, want %v", codes, want)
		}
	})

	t.Run("gauge options preserve business order, not alphabetical", func(t *testing.T) {
		var found []string
		for _, option := range catalog.Options {
			if option.AttributeCode == "gauge" {
				found = append(found, option.Code)
			}
		}
		want := []string{"14 AWG", "12 AWG", "10 AWG", "8 AWG", "6 AWG", "4 AWG", "2 AWG", "1 AWG", "1/0 AWG", "2/0 AWG", "3/0 AWG", "4/0 AWG"}
		if !reflect.DeepEqual(found, want) {
			t.Fatalf("gauge option order = %v, want %v (must be business order 14->12->10 AWG, not alphabetical)", found, want)
		}
	})

	t.Run("conditional rules hydrate nested from resource_attribute_rules", func(t *testing.T) {
		var color, voltage *domain.ResourceAttribute
		for i := range catalog.Attributes {
			switch catalog.Attributes[i].Definition.Code {
			case "color":
				color = &catalog.Attributes[i]
			case "voltage":
				voltage = &catalog.Attributes[i]
			}
		}
		if color == nil || voltage == nil {
			t.Fatalf("color/voltage attributes not found in hydrated catalog")
		}
		for _, attribute := range []*domain.ResourceAttribute{color, voltage} {
			if attribute.Mode != domain.ModeConditional {
				t.Fatalf("%s Mode = %v, want ModeConditional", attribute.Definition.Code, attribute.Mode)
			}
			if len(attribute.Rules) != 1 {
				t.Fatalf("%s Rules = %+v, want exactly 1 rule", attribute.Definition.Code, attribute.Rules)
			}
			rule := attribute.Rules[0]
			if rule.When.AttributeCode != "insulation" || rule.When.Equals != "DESNUDO" {
				t.Fatalf("%s Rules[0].When = %+v, want insulation=DESNUDO", attribute.Definition.Code, rule.When)
			}
			if rule.Mode != domain.ModeForbidden || !rule.NotApplicable {
				t.Fatalf("%s Rules[0] = %+v, want Mode=FORBIDDEN NotApplicable=true", attribute.Definition.Code, rule)
			}
		}
	})

	t.Run("unit names hydrate without changing stable fields", func(t *testing.T) {
		want := map[string]string{"M": "Metro", "PZA": "Pieza"}
		seen := 0
		for _, unit := range catalog.Units {
			if expected, ok := want[unit.Code]; ok && (unit.Name != expected || unit.Symbol != unit.Code) {
				t.Errorf("unit %q = %+v, want name %q and stable symbol", unit.Code, unit, expected)
			} else if ok {
				seen++
			}
		}
		if seen != len(want) {
			t.Fatalf("hydrated unit count = %d, want %d approved units", seen, len(want))
		}
	})

	t.Run("parity with SeedResourceCatalog once normalized", func(t *testing.T) {
		got := normalizeCatalogForParity(catalog)
		want := normalizeCatalogForParity(domain.SeedResourceCatalog())
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("LoadResourceCatalog() does not match SeedResourceCatalog() after normalization\n got  = %+v\n want = %+v", got, want)
		}
	})
}
