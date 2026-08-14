package postgres

import (
	"context"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
)

// TestCatalogAdminGrantsIntegration proves migrations/000003_catalog_admin's
// GRANTs actually let the runtime application role write catalog structure
// (design's Risk#1: 000002 REVOKEs ALL then GRANTs SELECT only on every
// catalog-definition table, so without 000003's GRANTs every admin write
// fails at runtime with 42501 permission denied — invisible to unit tests
// and to any test connecting as garfex_admin instead of garfex_app).
//
// GARFEX_TEST_DSN for this test MUST be a garfex_app-privilege DSN, not
// garfex_admin, or it proves nothing. Skips per this project's existing
// GARFEX_TEST_DSN convention (testing-data-boundary.md: real PostgreSQL,
// fixtures created and cleaned up by the test itself, TEST_-prefixed rows).
func TestCatalogAdminGrantsIntegration(t *testing.T) {
	dsn := os.Getenv("GARFEX_TEST_DSN")
	if dsn == "" {
		t.Skip("GARFEX_TEST_DSN not set")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("connect to PostgreSQL: %v", err)
	}
	// Registered first so it runs LAST (t.Cleanup is LIFO): every other
	// Cleanup below deletes fixture rows and needs the pool still open.
	t.Cleanup(pool.Close)

	must := func(label string, err error) {
		t.Helper()
		if err != nil {
			t.Fatalf("%s: %v", label, err)
		}
	}
	exec := func(sql string, args ...any) error {
		_, err := pool.Exec(ctx, sql, args...)
		return err
	}
	insertReturningID := func(sql string, args ...any) (int64, error) {
		var id int64
		err := pool.QueryRow(ctx, sql, args...).Scan(&id)
		return id, err
	}

	// One small dependency chain of TEST_-prefixed rows exercising all 12
	// catalog-definition objects the migration grants: resource_classes,
	// resource_option_sets, resource_families, resource_types,
	// unit_definitions, resource_unit_policies, attribute_definitions,
	// attribute_options, attribute_option_relations, resource_attributes,
	// resource_type_presentation_fields, resource_attribute_rules. Every
	// BIGSERIAL insert below also exercises the matching sequence USAGE
	// grant (nextval requires it).

	classID, err := insertReturningID(
		`INSERT INTO public.resource_classes (code, name, plural, slug) VALUES ($1,$2,$3,$4) RETURNING id`,
		"TEST_CATALOG_ADMIN_CLASS", "Test Clase", "Test Clases", "test-catalog-admin-clase")
	must("insert resource_classes", err)
	t.Cleanup(func() {
		if err := exec(`DELETE FROM public.resource_classes WHERE id = $1`, classID); err != nil {
			t.Errorf("cleanup resource_classes: %v", err)
		}
	})

	familyID, err := insertReturningID(
		`INSERT INTO public.resource_families (class_id, code, name) VALUES ($1,$2,$3) RETURNING id`,
		classID, "TEST_CATALOG_ADMIN_FAMILY", "Test Familia")
	must("insert resource_families", err)
	t.Cleanup(func() {
		if err := exec(`DELETE FROM public.resource_families WHERE id = $1`, familyID); err != nil {
			t.Errorf("cleanup resource_families: %v", err)
		}
	})

	typeID, err := insertReturningID(
		`INSERT INTO public.resource_types (class_id, family_id, code, name) VALUES ($1,$2,$3,$4) RETURNING id`,
		classID, familyID, "TEST_CATALOG_ADMIN_TYPE", "Test Tipo")
	must("insert resource_types", err)
	t.Cleanup(func() {
		if err := exec(`DELETE FROM public.resource_types WHERE id = $1`, typeID); err != nil {
			t.Errorf("cleanup resource_types: %v", err)
		}
	})

	unitID, err := insertReturningID(
		`INSERT INTO public.unit_definitions (code, symbol, dimension) VALUES ($1,$2,$3) RETURNING id`,
		"TEST_CATALOG_ADMIN_UNIT", "TCU", "TEST_DIMENSION")
	must("insert unit_definitions", err)
	t.Cleanup(func() {
		if err := exec(`DELETE FROM public.unit_definitions WHERE id = $1`, unitID); err != nil {
			t.Errorf("cleanup unit_definitions: %v", err)
		}
	})

	must("insert resource_unit_policies", exec(
		`INSERT INTO public.resource_unit_policies (family_id, unit_id, allowed, suggested) VALUES ($1,$2,TRUE,FALSE)`,
		familyID, unitID))
	t.Cleanup(func() {
		if err := exec(`DELETE FROM public.resource_unit_policies WHERE family_id = $1 AND unit_id = $2`, familyID, unitID); err != nil {
			t.Errorf("cleanup resource_unit_policies: %v", err)
		}
	})

	const optionSetCode = "TEST_CATALOG_ADMIN_OPTIONSET"
	must("insert resource_option_sets", exec(
		`INSERT INTO public.resource_option_sets (code, name) VALUES ($1,$2)`,
		optionSetCode, "Test Conjunto"))
	t.Cleanup(func() {
		if err := exec(`DELETE FROM public.resource_option_sets WHERE code = $1`, optionSetCode); err != nil {
			t.Errorf("cleanup resource_option_sets: %v", err)
		}
	})

	attrDefID1, err := insertReturningID(
		`INSERT INTO public.attribute_definitions (code, name, value_type) VALUES ($1,$2,$3) RETURNING id`,
		"test_catalog_admin_attr_1", "Test Attr 1", "CONTROLLED_OPTION")
	must("insert attribute_definitions 1", err)
	t.Cleanup(func() {
		if err := exec(`DELETE FROM public.attribute_definitions WHERE id = $1`, attrDefID1); err != nil {
			t.Errorf("cleanup attribute_definitions 1: %v", err)
		}
	})

	attrDefID2, err := insertReturningID(
		`INSERT INTO public.attribute_definitions (code, name, value_type) VALUES ($1,$2,$3) RETURNING id`,
		"test_catalog_admin_attr_2", "Test Attr 2", "CONTROLLED_OPTION")
	must("insert attribute_definitions 2", err)
	t.Cleanup(func() {
		if err := exec(`DELETE FROM public.attribute_definitions WHERE id = $1`, attrDefID2); err != nil {
			t.Errorf("cleanup attribute_definitions 2: %v", err)
		}
	})

	must("insert attribute_options 1", exec(
		`INSERT INTO public.attribute_options (option_set, attribute_definition_id, code, label) VALUES ($1,$2,$3,$4)`,
		optionSetCode, attrDefID1, "A", "Opción A"))
	t.Cleanup(func() {
		if err := exec(`DELETE FROM public.attribute_options WHERE option_set = $1 AND attribute_definition_id = $2 AND code = $3`,
			optionSetCode, attrDefID1, "A"); err != nil {
			t.Errorf("cleanup attribute_options 1: %v", err)
		}
	})

	must("insert attribute_options 2", exec(
		`INSERT INTO public.attribute_options (option_set, attribute_definition_id, code, label) VALUES ($1,$2,$3,$4)`,
		optionSetCode, attrDefID2, "B", "Opción B"))
	t.Cleanup(func() {
		if err := exec(`DELETE FROM public.attribute_options WHERE option_set = $1 AND attribute_definition_id = $2 AND code = $3`,
			optionSetCode, attrDefID2, "B"); err != nil {
			t.Errorf("cleanup attribute_options 2: %v", err)
		}
	})

	relationID, err := insertReturningID(
		`INSERT INTO public.attribute_option_relations
			(option_set, from_attribute_definition_id, from_option_code, to_attribute_definition_id, to_option_code)
		 VALUES ($1,$2,$3,$4,$5) RETURNING id`,
		optionSetCode, attrDefID1, "A", attrDefID2, "B")
	must("insert attribute_option_relations", err)
	t.Cleanup(func() {
		if err := exec(`DELETE FROM public.attribute_option_relations WHERE id = $1`, relationID); err != nil {
			t.Errorf("cleanup attribute_option_relations: %v", err)
		}
	})

	resourceAttrID, err := insertReturningID(
		`INSERT INTO public.resource_attributes (class_id, family_id, type_id, definition_id, mode) VALUES ($1,$2,$3,$4,$5) RETURNING id`,
		classID, familyID, typeID, attrDefID1, "REQUIRED")
	must("insert resource_attributes", err)
	t.Cleanup(func() {
		if err := exec(`DELETE FROM public.resource_attributes WHERE id = $1`, resourceAttrID); err != nil {
			t.Errorf("cleanup resource_attributes: %v", err)
		}
	})

	must("insert resource_type_presentation_fields", exec(
		`INSERT INTO public.resource_type_presentation_fields (type_id, attribute_definition_id, position) VALUES ($1,$2,$3)`,
		typeID, attrDefID1, 1))
	t.Cleanup(func() {
		if err := exec(`DELETE FROM public.resource_type_presentation_fields WHERE type_id = $1 AND attribute_definition_id = $2`,
			typeID, attrDefID1); err != nil {
			t.Errorf("cleanup resource_type_presentation_fields: %v", err)
		}
	})

	ruleID, err := insertReturningID(
		`INSERT INTO public.resource_attribute_rules
			(resource_attribute_id, display_order, when_definition_id, when_equals, mode)
		 VALUES ($1,$2,$3,$4,$5) RETURNING id`,
		resourceAttrID, 0, attrDefID2, "B", "FORBIDDEN")
	must("insert resource_attribute_rules", err)
	t.Cleanup(func() {
		if err := exec(`DELETE FROM public.resource_attribute_rules WHERE id = $1`, ruleID); err != nil {
			t.Errorf("cleanup resource_attribute_rules: %v", err)
		}
	})
}
