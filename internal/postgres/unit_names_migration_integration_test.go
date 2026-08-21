package postgres

import (
	"context"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
)

func TestUnitNamesMigrationIntegration(t *testing.T) {
	adminDSN, appDSN := os.Getenv("GARFEX_ADMIN_TEST_DSN"), os.Getenv("GARFEX_TEST_DSN")
	if adminDSN == "" || appDSN == "" {
		t.Skip("GARFEX_ADMIN_TEST_DSN and GARFEX_TEST_DSN are required")
	}
	ctx, admin, app := context.Background(), openUnitTestPool(t, adminDSN), openUnitTestPool(t, appDSN)
	window := beginMigrationTestWindow(t, admin)
	must := func(label string, err error) {
		if err != nil {
			t.Fatalf("%s: %v", label, err)
		}
	}
	var hasName bool
	must("inspect schema", admin.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_schema='public' AND table_name='unit_definitions' AND column_name='name')`).Scan(&hasName))
	window.cleanups = append(window.cleanups, func(ctx context.Context) error {
		var current bool
		if err := admin.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_schema='public' AND table_name='unit_definitions' AND column_name='name')`).Scan(&current); err != nil {
			return err
		}
		if current == hasName {
			return nil
		}
		name := "000004_unit_names.down.sql"
		if hasName {
			name = "000004_unit_names.up.sql"
		}
		return applySchemaMigration(admin, name)
	})
	if hasName {
		applyUnitNamesMigration(t, admin, "000004_unit_names.down.sql")
	}
	units := []struct{ code, symbol, want string }{
		{"M", "M", "Metro"}, {"PZA", "PZA", "Pieza"}, {"KG", "KG", "Provisional: Código KG"}, {"TEST_CUSTOM", "TCU", "Provisional: Código TEST_CUSTOM"},
	}
	type stable struct {
		id                int64
		symbol, dimension string
	}
	before := map[string]stable{}
	for _, unit := range units[2:] {
		var id int64
		must("insert legacy unit", admin.QueryRow(ctx, `INSERT INTO public.unit_definitions (code, symbol, dimension) VALUES ($1,$2,'MASS') RETURNING id`, unit.code, unit.symbol).Scan(&id))
		before[unit.code] = stable{id: id, symbol: unit.symbol, dimension: "MASS"}
		window.TrackUnit(id)
	}
	var classID, familyID, typeID int64
	must("resolve references", admin.QueryRow(ctx, `SELECT cl.id, f.id, ty.id FROM public.resource_classes cl JOIN public.resource_families f ON f.class_id=cl.id AND f.code='CONDUCTORES' JOIN public.resource_types ty ON ty.family_id=f.id AND ty.code='CABLE' WHERE cl.code='MATERIAL'`).Scan(&classID, &familyID, &typeID))
	_, err := admin.Exec(ctx, `INSERT INTO public.resource_unit_policies (family_id, unit_id, allowed, suggested) VALUES ($1,$2,TRUE,FALSE)`, familyID, before["KG"].id)
	must("insert preserved policy", err)
	var resourceID int64
	must("insert preserved resource", admin.QueryRow(ctx, `INSERT INTO public.recursos (class_id,family_id,type_id,natural_unit_id,display_name,identity_key) VALUES ($1,$2,$3,$4,'Migration resource','TEST_UNIT_NAMES_RESOURCE') RETURNING id`, classID, familyID, typeID, before["KG"].id).Scan(&resourceID))
	window.TrackResource(resourceID)

	applyUnitNamesMigration(t, admin, "000004_unit_names.up.sql")
	for _, unit := range units {
		var id int64
		var code, name, symbol, dimension string
		must("read migrated unit", admin.QueryRow(ctx, `SELECT id,code,name,symbol,dimension FROM public.unit_definitions WHERE code=$1`, unit.code).Scan(&id, &code, &name, &symbol, &dimension))
		if name != unit.want || code != unit.code || symbol != unit.symbol {
			t.Errorf("unit %q = %q/%q/%q, want %q/%q/%q", unit.code, name, symbol, code, unit.want, unit.symbol, unit.code)
		}
		if old, ok := before[unit.code]; ok && (id != old.id || symbol != old.symbol || dimension != old.dimension) {
			t.Errorf("unit %q stable fields changed: id=%d symbol=%q dimension=%q", unit.code, id, symbol, dimension)
		}
	}
	var policyCount, preservedID int64
	must("check policy", admin.QueryRow(ctx, `SELECT count(*) FROM public.resource_unit_policies WHERE unit_id=$1`, before["KG"].id).Scan(&policyCount))
	must("check resource", admin.QueryRow(ctx, `SELECT natural_unit_id FROM public.recursos WHERE id=$1`, resourceID).Scan(&preservedID))
	if policyCount != 1 || preservedID != before["KG"].id {
		t.Fatalf("references changed: policy_count=%d natural_unit_id=%d", policyCount, preservedID)
	}
	var notNull, check, noUnique, grants bool
	must("inspect constraints and grants", admin.QueryRow(ctx, `SELECT a.attnotnull, EXISTS (SELECT 1 FROM pg_constraint WHERE conrelid='public.unit_definitions'::regclass AND conname='unit_definitions_name_nonblank'), NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conrelid='public.unit_definitions'::regclass AND contype='u' AND pg_get_constraintdef(oid) ILIKE '%name%'), has_table_privilege('garfex_app','public.unit_definitions','SELECT') AND has_table_privilege('garfex_app','public.unit_definitions','INSERT') AND has_table_privilege('garfex_app','public.unit_definitions','UPDATE') AND has_table_privilege('garfex_app','public.unit_definitions','DELETE') FROM pg_attribute a WHERE a.attrelid='public.unit_definitions'::regclass AND a.attname='name'`).Scan(&notNull, &check, &noUnique, &grants))
	if !notNull || !check || !noUnique || !grants {
		t.Fatalf("name schema = not_null:%v check:%v no_unique:%v grants:%v", notNull, check, noUnique, grants)
	}
	must("insert duplicate names", func() error {
		_, err := app.Exec(ctx, `INSERT INTO public.unit_definitions (code,name,symbol,dimension) VALUES ('TEST_DUPLICATE_A','Metro','TDA','LENGTH'),('TEST_DUPLICATE_B','Metro','TDB','LENGTH')`)
		return err
	}())
	for _, code := range []string{"TEST_DUPLICATE_A", "TEST_DUPLICATE_B"} {
		var id int64
		must("record duplicate-name unit", admin.QueryRow(ctx, `SELECT id FROM public.unit_definitions WHERE code=$1`, code).Scan(&id))
		window.TrackUnit(id)
	}
	if _, err := app.Exec(ctx, `INSERT INTO public.unit_definitions (code,name,symbol,dimension) VALUES ('TEST_BLANK','   ','TBL','LENGTH')`); err == nil {
		t.Fatal("blank unit name was accepted")
	}
	applyUnitNamesMigration(t, admin, "000004_unit_names.down.sql")
	var name string
	must("read retained name", admin.QueryRow(ctx, `SELECT name FROM public.unit_definitions WHERE code='M'`).Scan(&name))
	if name != "Metro" {
		t.Fatalf("down lost Metro name %q", name)
	}
	_, err = admin.Exec(ctx, `UPDATE public.unit_definitions SET name=NULL WHERE code='TEST_CUSTOM'`)
	must("prepare re-up", err)
	applyUnitNamesMigration(t, admin, "000004_unit_names.up.sql")
	must("read re-up fallback", admin.QueryRow(ctx, `SELECT name FROM public.unit_definitions WHERE code='TEST_CUSTOM'`).Scan(&name))
	if name != "Provisional: Código TEST_CUSTOM" {
		t.Fatalf("re-up fallback = %q", name)
	}
}

func openUnitTestPool(t *testing.T, dsn string) *pgxpool.Pool {
	p, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		t.Fatalf("connect test PostgreSQL: %v", err)
	}
	t.Cleanup(p.Close)
	return p
}

func applyUnitNamesMigration(t *testing.T, pool *pgxpool.Pool, name string) {
	if err := applySchemaMigration(pool, name); err != nil {
		t.Fatalf("apply migration %s: %v", name, err)
	}
}
