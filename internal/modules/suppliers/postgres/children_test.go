package postgres

import (
	"strings"
	"testing"
)

func TestQueriesAlwaysScopeChildrenBySupplier(t *testing.T) {
	for name, query := range map[string]string{"get branch": getBranchSQL, "list branches": listBranchesSQL, "get contact": getContactSQL, "list contacts": listContactsSQL} {
		t.Run(name, func(t *testing.T) {
			if !strings.Contains(query, "supplier_id = $1") {
				t.Fatalf("query is not supplier scoped: %s", query)
			}
		})
	}
}
