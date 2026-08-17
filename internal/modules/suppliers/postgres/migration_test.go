package postgres

import (
	"os"
	"strings"
	"testing"
)

func TestMigrationEnforcesOwnershipWithoutAccidentalCityIdentity(t *testing.T) {
	content, err := os.ReadFile("../../../../migrations/000006_supplier_master.up.sql")
	if err != nil {
		t.Fatalf("read migration: %v", err)
	}
	sql := string(content)
	wants := []string{
		"UNIQUE (supplier_id, id)",
		"FOREIGN KEY (supplier_id, branch_id)",
		"CONSTRAINT supplier_contacts_supplier_branch_fkey",
		"GRANT SELECT, INSERT, UPDATE ON",
	}
	for _, want := range wants {
		if !strings.Contains(sql, want) {
			t.Fatalf("migration missing %q", want)
		}
	}
	if strings.Contains(sql, "UNIQUE (supplier_id, city)") || strings.Contains(sql, "GRANT SELECT, INSERT, UPDATE, DELETE ON") {
		t.Fatal("migration introduced city identity or runtime DELETE privilege")
	}
}

func TestMigrationRevokesDeleteFromRuntimeRole(t *testing.T) {
	content, err := os.ReadFile("../../../../migrations/000006_supplier_master.up.sql")
	if err != nil {
		t.Fatalf("read migration: %v", err)
	}

	normalizedSQL := strings.Join(strings.Fields(string(content)), " ")
	want := "REVOKE DELETE ON public.suppliers, public.supplier_branches, public.supplier_contacts FROM garfex_app;"
	if !strings.Contains(normalizedSQL, want) {
		t.Fatalf("migration missing DELETE revocation for all Supplier Master tables: %s", want)
	}
}
