package postgres

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

func TestResourceIntegrityMigrationAuditRelation(t *testing.T) {
	sql, err := os.ReadFile(filepath.Join("..", "..", "migrations", "000005_resource_integrity.up.sql"))
	if err != nil {
		t.Fatalf("read resource integrity migration: %v", err)
	}
	migration := string(sql)
	relation := `resource_integrity_identity_audit`

	t.Run("does not require temporary table privilege", func(t *testing.T) {
		if regexp.MustCompile(`(?i)\bTEMP(?:ORARY)?\b`).MatchString(migration) {
			t.Fatal("migration requests TEMP or TEMPORARY privilege")
		}
	})

	t.Run("fully qualifies every audit relation reference", func(t *testing.T) {
		references := regexp.MustCompile(`(?i)([a-z_][a-z0-9_]*\s*\.\s*)?`+relation).FindAllStringSubmatch(migration, -1)
		whitespace := regexp.MustCompile(`\s+`)
		if len(references) == 0 {
			t.Fatal("migration does not reference the audit relation")
		}
		for _, reference := range references {
			qualifier := whitespace.ReplaceAllString(reference[1], "")
			if !strings.EqualFold(qualifier, "public.") {
				t.Errorf("audit relation reference %q is not qualified with public", reference[0])
			}
		}
	})

	t.Run("explicitly drops audit relation", func(t *testing.T) {
		drop := regexp.MustCompile(`(?i)\bDROP\s+TABLE\s+(?:IF\s+EXISTS\s+)?public\s*\.\s*` + relation + `\b`)
		if !drop.MatchString(migration) {
			t.Fatal("migration does not explicitly drop the public audit relation")
		}
	})
}
