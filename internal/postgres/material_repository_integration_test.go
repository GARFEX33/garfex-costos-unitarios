package postgres

import (
	"context"
	"errors"
	"os"
	"sort"
	"testing"

	"github.com/GARFEX33/garfex-costos-unitarios/internal/domain"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestMaterialRepositoryIntegration(t *testing.T) {
	dsn := os.Getenv("GARFEX_TEST_DSN")
	if dsn == "" {
		t.Skip("GARFEX_TEST_DSN not set")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("connect to PostgreSQL: %v", err)
	}
	defer pool.Close()

	repo := NewMaterialRepository(pool)
	catalog := domain.NewMaterialsCatalog()

	t.Run("insulated and desnudo round trip", func(t *testing.T) {
		cleanupMaterials(ctx, t, pool)
		defer cleanupMaterials(ctx, t, pool)
		insulated := mustCreateMaterial(t, catalog, "CONDUCTORES", "M", []domain.MaterialAttributeValue{domain.OptionValue("conductor_material", "COBRE"), domain.OptionValue("gauge", "10 AWG"), domain.OptionValue("insulation", "THW-LS"), domain.OptionValue("color", "NEGRO"), domain.QuantityValue("voltage", "600", "V")})
		if err := repo.Create(ctx, insulated); err != nil {
			t.Fatalf("Create() insulated: %v", err)
		}
		assertRoundTrip(t, ctx, repo, insulated)
		desnudo := domain.Material{FamilyCode: "CONDUCTORES", NaturalUnit: "M", IdentityKey: "CONDUCTORES|conductor_material=COBRE|gauge=12 AWG|insulation=DESNUDO", Attributes: []domain.MaterialAttributeValue{domain.OptionValue("conductor_material", "COBRE"), domain.OptionValue("gauge", "12 AWG"), domain.OptionValue("insulation", "DESNUDO"), {AttributeCode: "color", Type: domain.ValueTypeControlledOption, Text: notApplicableState}, {AttributeCode: "voltage", Type: domain.ValueTypeQuantity, Text: notApplicableState}}}
		if err := repo.Create(ctx, desnudo); err != nil {
			t.Fatalf("Create() desnudo: %v", err)
		}
		assertRoundTrip(t, ctx, repo, desnudo)
	})

	t.Run("decimal quantity round trip", func(t *testing.T) {
		cleanupMaterials(ctx, t, pool)
		defer cleanupMaterials(ctx, t, pool)
		material := mustCreateMaterial(t, catalog, "CONDUCTORES", "M", []domain.MaterialAttributeValue{domain.OptionValue("conductor_material", "COBRE"), domain.OptionValue("gauge", "8 AWG"), domain.OptionValue("insulation", "THW"), domain.OptionValue("color", "BLANCO"), domain.QuantityValue("voltage", "600.00", "V")})
		if err := repo.Create(ctx, material); err != nil {
			t.Fatalf("Create() decimal quantity: %v", err)
		}
		assertRoundTrip(t, ctx, repo, material)
	})

	t.Run("voltage identity canonicalizes duplicate", func(t *testing.T) {
		cleanupMaterials(ctx, t, pool)
		defer cleanupMaterials(ctx, t, pool)
		kv := mustCreateMaterial(t, catalog, "CONDUCTORES", "M", []domain.MaterialAttributeValue{domain.OptionValue("conductor_material", "COBRE"), domain.OptionValue("gauge", "6 AWG"), domain.OptionValue("insulation", "THHN"), domain.OptionValue("color", "ROJO"), domain.QuantityValue("voltage", "1", "kV")})
		volts := mustCreateMaterial(t, catalog, "CONDUCTORES", "M", []domain.MaterialAttributeValue{domain.OptionValue("conductor_material", "COBRE"), domain.OptionValue("gauge", "6 AWG"), domain.OptionValue("insulation", "THHN"), domain.OptionValue("color", "ROJO"), domain.QuantityValue("voltage", "1000", "V")})
		if err := repo.Create(ctx, kv); err != nil {
			t.Fatalf("Create() 1 kV: %v", err)
		}
		if err := repo.Create(ctx, volts); !errors.Is(err, domain.ErrDuplicateMaterial) {
			t.Fatalf("Create() 1000 V error = %v, want ErrDuplicateMaterial", err)
		}
		assertRoundTrip(t, ctx, repo, kv)
	})

	t.Run("natural unit persisted and excluded from identity", func(t *testing.T) {
		cleanupMaterials(ctx, t, pool)
		defer cleanupMaterials(ctx, t, pool)
		material := mustCreateMaterial(t, catalog, "CONDUCTORES", "M", []domain.MaterialAttributeValue{domain.OptionValue("conductor_material", "ALUMINIO"), domain.OptionValue("gauge", "4 AWG"), domain.OptionValue("insulation", "XHHW-2"), domain.OptionValue("color", "AZUL"), domain.QuantityValue("voltage", "600", "V")})
		if err := repo.Create(ctx, material); err != nil {
			t.Fatalf("Create() natural unit: %v", err)
		}
		got := assertRoundTrip(t, ctx, repo, material)
		if got.NaturalUnit != "M" {
			t.Fatalf("NaturalUnit = %q, want M", got.NaturalUnit)
		}
		if got.IdentityKey != material.IdentityKey {
			t.Fatalf("IdentityKey changed with NaturalUnit: %q", got.IdentityKey)
		}
	})
	t.Run("duplicate material returns domain error", func(t *testing.T) {
		cleanupMaterials(ctx, t, pool)
		defer cleanupMaterials(ctx, t, pool)
		material := mustCreateMaterial(t, catalog, "CONDUCTORES", "M", []domain.MaterialAttributeValue{domain.OptionValue("conductor_material", "COBRE"), domain.OptionValue("gauge", "2 AWG"), domain.OptionValue("insulation", "THW-LS"), domain.OptionValue("color", "VERDE"), domain.QuantityValue("voltage", "600", "V")})
		if err := repo.Create(ctx, material); err != nil {
			t.Fatalf("Create() first: %v", err)
		}
		if err := repo.Create(ctx, material); !errors.Is(err, domain.ErrDuplicateMaterial) {
			t.Fatalf("Create() duplicate error = %v, want ErrDuplicateMaterial", err)
		}
	})
	t.Run("invalid option returns reference error", func(t *testing.T) {
		cleanupMaterials(ctx, t, pool)
		defer cleanupMaterials(ctx, t, pool)
		material := mustCreateMaterial(t, catalog, "CONDUCTORES", "M", []domain.MaterialAttributeValue{domain.OptionValue("conductor_material", "COBRE"), domain.OptionValue("gauge", "2 AWG"), domain.OptionValue("insulation", "THW-LS"), domain.OptionValue("color", "VERDE"), domain.QuantityValue("voltage", "600", "V")})
		material.Attributes[1].OptionCode = "NOEXIST"
		if err := repo.Create(ctx, material); !errors.Is(err, domain.ErrMaterialReference) {
			t.Fatalf("Create() invalid option error = %v, want ErrMaterialReference", err)
		}
	})
	t.Run("set without option returns reference error", func(t *testing.T) {
		cleanupMaterials(ctx, t, pool)
		defer cleanupMaterials(ctx, t, pool)
		material := domain.Material{FamilyCode: "CONDUCTORES", NaturalUnit: "M", IdentityKey: "CONDUCTORES|conductor_material=COBRE|gauge=2 AWG|insulation=THW-LS|color=|voltage=600 V", Attributes: []domain.MaterialAttributeValue{domain.OptionValue("conductor_material", "COBRE"), {AttributeCode: "gauge", Type: domain.ValueTypeControlledOption}, domain.OptionValue("insulation", "THW-LS"), {AttributeCode: "color", Type: domain.ValueTypeControlledOption}, domain.QuantityValue("voltage", "600", "V")}}
		if err := repo.Create(ctx, material); !errors.Is(err, domain.ErrMaterialReference) {
			t.Fatalf("Create() set without option error = %v, want ErrMaterialReference", err)
		}
	})
	t.Run("not applicable with payload returns reference error", func(t *testing.T) {
		cleanupMaterials(ctx, t, pool)
		defer cleanupMaterials(ctx, t, pool)
		material := domain.Material{FamilyCode: "CONDUCTORES", NaturalUnit: "M", IdentityKey: "CONDUCTORES|conductor_material=COBRE|gauge=2 AWG|insulation=DESNUDO", Attributes: []domain.MaterialAttributeValue{domain.OptionValue("conductor_material", "COBRE"), domain.OptionValue("gauge", "2 AWG"), domain.OptionValue("insulation", "DESNUDO"), {AttributeCode: "color", Type: domain.ValueTypeControlledOption, OptionCode: "NEGRO", Text: notApplicableState}, {AttributeCode: "voltage", Type: domain.ValueTypeQuantity, Text: notApplicableState}}}
		if err := repo.Create(ctx, material); !errors.Is(err, domain.ErrMaterialReference) {
			t.Fatalf("Create() NOT_APPLICABLE payload error = %v, want ErrMaterialReference", err)
		}
	})
	t.Run("unknown family returns reference error", func(t *testing.T) {
		cleanupMaterials(ctx, t, pool)
		defer cleanupMaterials(ctx, t, pool)
		material := domain.Material{FamilyCode: "UNKNOWN", NaturalUnit: "M", IdentityKey: "UNKNOWN|"}
		if err := repo.Create(ctx, material); !errors.Is(err, domain.ErrMaterialReference) {
			t.Fatalf("Create() unknown family error = %v, want ErrMaterialReference", err)
		}
	})
	t.Run("tuberias round trip", func(t *testing.T) {
		cleanupMaterials(ctx, t, pool)
		defer cleanupMaterials(ctx, t, pool)
		material := mustCreateMaterial(t, catalog, "TUBERIAS", "PZA", []domain.MaterialAttributeValue{domain.OptionValue("tipo", "CONDUIT PARED DELGADA"), domain.OptionValue("diameter_inch", `1/2"`), domain.OptionValue("diameter_mm", "13 mm")})
		if err := repo.Create(ctx, material); err != nil {
			t.Fatalf("Create() tuberias: %v", err)
		}
		assertRoundTrip(t, ctx, repo, material)
	})
	t.Run("tuberias search by inch and mm returns same material", func(t *testing.T) {
		cleanupMaterials(ctx, t, pool)
		defer cleanupMaterials(ctx, t, pool)
		byInch := mustCreateMaterial(t, catalog, "TUBERIAS", "PZA", []domain.MaterialAttributeValue{domain.OptionValue("tipo", "CONDUIT PARED GRUESA"), domain.OptionValue("diameter_inch", `3/4"`), domain.OptionValue("diameter_mm", "19 mm")})
		if err := repo.Create(ctx, byInch); err != nil {
			t.Fatalf("Create() tuberias by inch: %v", err)
		}
		byMM := mustCreateMaterial(t, catalog, "TUBERIAS", "PZA", []domain.MaterialAttributeValue{domain.OptionValue("tipo", "CONDUIT PARED GRUESA"), domain.OptionValue("diameter_inch", `3/4"`), domain.OptionValue("diameter_mm", "19 mm")})
		if byInch.IdentityKey != byMM.IdentityKey {
			t.Fatalf("same technical identity expected: %q != %q", byInch.IdentityKey, byMM.IdentityKey)
		}
		got, err := repo.Get(ctx, byInch.FamilyCode, byInch.IdentityKey)
		if err != nil {
			t.Fatalf("Get() tuberias: %v", err)
		}
		assertMaterialEqual(t, got, byInch)
	})
}

func mustCreateMaterial(t *testing.T, catalog domain.MaterialsCatalog, family, unit string, values []domain.MaterialAttributeValue) domain.Material {
	t.Helper()
	material, err := domain.NewMaterial(catalog, family, unit, values)
	if err != nil {
		t.Fatalf("build material: %v", err)
	}
	return material
}

func cleanupMaterials(ctx context.Context, t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	if _, err := pool.Exec(ctx, "DELETE FROM public.materiales"); err != nil {
		t.Fatalf("cleanup materials: %v", err)
	}
}

func assertRoundTrip(t *testing.T, ctx context.Context, repo domain.MaterialRepository, want domain.Material) domain.Material {
	t.Helper()
	got, err := repo.Get(ctx, want.FamilyCode, want.IdentityKey)
	if err != nil {
		t.Fatalf("Get() round trip: %v", err)
	}
	assertMaterialEqual(t, got, want)
	return got
}

func assertMaterialEqual(t *testing.T, got, want domain.Material) {
	t.Helper()
	if got.FamilyCode != want.FamilyCode || got.NaturalUnit != want.NaturalUnit || got.IdentityKey != want.IdentityKey {
		t.Fatalf("material identity = %#v, want %#v", got, want)
	}
	if len(got.Attributes) != len(want.Attributes) {
		t.Fatalf("attribute count = %d, want %d", len(got.Attributes), len(want.Attributes))
	}
	gotAttrs := append([]domain.MaterialAttributeValue(nil), got.Attributes...)
	wantAttrs := append([]domain.MaterialAttributeValue(nil), want.Attributes...)
	sort.Slice(gotAttrs, func(i, j int) bool { return gotAttrs[i].AttributeCode < gotAttrs[j].AttributeCode })
	sort.Slice(wantAttrs, func(i, j int) bool { return wantAttrs[i].AttributeCode < wantAttrs[j].AttributeCode })
	for i := range wantAttrs {
		if gotAttrs[i].AttributeCode != wantAttrs[i].AttributeCode || gotAttrs[i].Type != wantAttrs[i].Type || gotAttrs[i].OptionCode != wantAttrs[i].OptionCode || gotAttrs[i].Text != wantAttrs[i].Text {
			t.Fatalf("attribute %d = %#v, want %#v", i, gotAttrs[i], wantAttrs[i])
		}
		if wantAttrs[i].Quantity != nil && (!gotAttrs[i].Quantity.Value.Equal(wantAttrs[i].Quantity.Value) || gotAttrs[i].Quantity.UnitCode != wantAttrs[i].Quantity.UnitCode) {
			t.Fatalf("quantity %d = %#v, want %#v", i, gotAttrs[i].Quantity, wantAttrs[i].Quantity)
		}
	}
}
