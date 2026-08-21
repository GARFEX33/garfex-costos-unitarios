package resourcecore

import (
	"reflect"
	"testing"
)

func TestNoWriteMethods(t *testing.T) {
	for _, typ := range []any{CatalogRecord{}, CatalogDescriptor{}, Value{}, Resource{}, CatalogPage{}, ResourcePage{}} {
		if reflect.TypeOf(typ).NumMethod() != 0 {
			t.Fatalf("unexpected methods on %s", reflect.TypeOf(typ).Name())
		}
	}
}

func TestQueryAndPage(t *testing.T) {
	cq := CatalogQuery{Scope: ScopeActive, Limit: 25}
	if cq.Scope != ScopeActive || cq.Limit != 25 {
		t.Fatalf("catalog query mismatch")
	}
	rq := ResourceQuery{Scope: ScopeAll, ClassCode: "MAT"}
	if rq.Scope != ScopeAll || rq.ClassCode != "MAT" {
		t.Fatalf("resource query mismatch")
	}
	cp := CatalogPage{Records: []CatalogRecord{{ID: 1}}, HasPrevious: true}
	if len(cp.Records) != 1 || !cp.HasPrevious {
		t.Fatalf("catalog page mismatch")
	}
	rp := ResourcePage{Resources: []Resource{{ID: 1}}, HasNext: true}
	if len(rp.Resources) != 1 || !rp.HasNext {
		t.Fatalf("resource page mismatch")
	}
}
