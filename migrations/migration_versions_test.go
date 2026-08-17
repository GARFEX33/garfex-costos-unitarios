package migrations_test

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"testing"
)

var migrationName = regexp.MustCompile(`^([0-9]+)_.+\.(up|down)\.sql$`)

func TestMigrationVersionDirectionsAreUnique(t *testing.T) {
	if err := validateUniqueMigrationVersionDirections("."); err != nil {
		t.Fatal(err)
	}
}

func TestValidateUniqueMigrationVersionDirections(t *testing.T) {
	tests := []struct {
		name      string
		files     []string
		wantError bool
	}{
		{name: "unique pairs", files: []string{"000001_first.up.sql", "000001_first.down.sql", "000002_second.up.sql"}},
		{name: "duplicate version and direction", files: []string{"000006_supplier.up.sql", "000006_resource.up.sql"}, wantError: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			for _, name := range tt.files {
				if err := os.WriteFile(filepath.Join(dir, name), nil, 0o600); err != nil {
					t.Fatalf("write migration fixture: %v", err)
				}
			}
			err := validateUniqueMigrationVersionDirections(dir)
			if (err != nil) != tt.wantError {
				t.Fatalf("validateUniqueMigrationVersionDirections() error = %v, wantError %t", err, tt.wantError)
			}
		})
	}
}

func validateUniqueMigrationVersionDirections(dir string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("read migrations: %w", err)
	}
	type key struct {
		version   uint64
		direction string
	}
	seen := make(map[key]string)
	for _, entry := range entries {
		match := migrationName.FindStringSubmatch(entry.Name())
		if entry.IsDir() || match == nil {
			continue
		}
		version, err := strconv.ParseUint(match[1], 10, 64)
		if err != nil {
			return fmt.Errorf("parse migration version %q: %w", match[1], err)
		}
		pair := key{version: version, direction: match[2]}
		if previous, exists := seen[pair]; exists {
			return fmt.Errorf("duplicate migration version %d %s: %s and %s", version, pair.direction, previous, entry.Name())
		}
		seen[pair] = entry.Name()
	}
	return nil
}
