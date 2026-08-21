package resourcecore

import (
	"context"
	"reflect"
	"testing"
)

type fakeWriteCapabilities struct {
	createCatalog  func(ctx context.Context, req CatalogWriteRequest) (CatalogRecord, error)
	createResource func(ctx context.Context, req ResourceWriteRequest) (Resource, error)
}

func (f *fakeWriteCapabilities) CreateCatalog(ctx context.Context, req CatalogWriteRequest) (CatalogRecord, error) {
	return f.createCatalog(ctx, req)
}

func (f *fakeWriteCapabilities) CreateResource(ctx context.Context, req ResourceWriteRequest) (Resource, error) {
	return f.createResource(ctx, req)
}

func validCatalogWriteRequest() CatalogWriteRequest {
	return CatalogWriteRequest{
		Actor: "PI",
		Kind:  KindClass,
		Values: map[string]Value{
			"code": {Kind: ValueCode, Text: "MAT"},
		},
	}
}

func validResourceWriteRequest() ResourceWriteRequest {
	return ResourceWriteRequest{
		Actor:       "PI",
		Scope:       ResourceScope{ClassCode: "MAT", FamilyCode: "CONDUCTORES", TypeCode: "CABLE"},
		NaturalUnit: "m",
	}
}

func TestWriter_NewWriter_NilCapability_InvalidArgument(t *testing.T) {
	w, err := NewWriter(nil)
	if w != nil {
		t.Fatalf("expected nil Writer, got %+v", w)
	}
	if !IsCode(err, InvalidArgument) {
		t.Fatalf("expected INVALID_ARGUMENT, got %v", err)
	}
}

func TestWriter_CreateCatalog_ShapeValidation(t *testing.T) {
	tests := []struct {
		name    string
		mutate  func(*CatalogWriteRequest)
		wantErr bool
	}{
		{"valid", func(r *CatalogWriteRequest) {}, false},
		{"blank actor", func(r *CatalogWriteRequest) { r.Actor = "  " }, true},
		{"unknown kind", func(r *CatalogWriteRequest) { r.Kind = "NOPE" }, true},
		{"nil values", func(r *CatalogWriteRequest) { r.Values = nil }, true},
		{"empty values", func(r *CatalogWriteRequest) { r.Values = map[string]Value{} }, true},
		{"unknown value kind", func(r *CatalogWriteRequest) { r.Values["code"] = Value{Kind: "BOGUS", Text: "x"} }, true},
		{"non-zero reference id", func(r *CatalogWriteRequest) {
			r.Values["ref"] = Value{Kind: ValueReference, Reference: &Reference{Kind: KindFamily, ID: 5, Code: "F"}}
		}, true},
		{"zero reference id ok", func(r *CatalogWriteRequest) {
			r.Values["ref"] = Value{Kind: ValueReference, Reference: &Reference{Kind: KindFamily, Code: "F"}}
		}, false},
		{"unit code on non-quantity", func(r *CatalogWriteRequest) { r.Values["code"] = Value{Kind: ValueCode, Text: "MAT", UnitCode: "m"} }, true},
		{"missing unit code on quantity", func(r *CatalogWriteRequest) { r.Values["q"] = Value{Kind: ValueQuantity, Text: "1"} }, true},
		{"quantity with unit code ok", func(r *CatalogWriteRequest) { r.Values["q"] = Value{Kind: ValueQuantity, Text: "1", UnitCode: "m"} }, false},
		{"invalid integer text", func(r *CatalogWriteRequest) { r.Values["n"] = Value{Kind: ValueInteger, Text: "not-a-number"} }, true},
		{"invalid decimal text", func(r *CatalogWriteRequest) { r.Values["d"] = Value{Kind: ValueDecimal, Text: "not-a-number"} }, true},
		{"applicability equals not text", func(r *CatalogWriteRequest) {
			r.Rules = []ApplicabilityRule{{AttributeCode: "insulation", Equals: Value{Kind: ValueBool, Bool: true}, Mode: "FORBIDDEN"}}
		}, true},
		{"applicability equals text ok", func(r *CatalogWriteRequest) {
			r.Rules = []ApplicabilityRule{{AttributeCode: "insulation", Equals: Value{Kind: ValueText, Text: "DESNUDO"}, Mode: "FORBIDDEN"}}
		}, false},
		{"applicability unknown mode", func(r *CatalogWriteRequest) {
			r.Rules = []ApplicabilityRule{{AttributeCode: "insulation", Equals: Value{Kind: ValueText, Text: "DESNUDO"}, Mode: "BOGUS"}}
		}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := validCatalogWriteRequest()
			tt.mutate(&req)
			cap := &fakeWriteCapabilities{
				createCatalog: func(ctx context.Context, req CatalogWriteRequest) (CatalogRecord, error) {
					return CatalogRecord{Kind: req.Kind, ID: 1, Revision: 1}, nil
				},
			}
			w, err := NewWriter(cap)
			if err != nil {
				t.Fatalf("unexpected NewWriter error: %v", err)
			}
			_, err = w.CreateCatalog(context.Background(), req)
			if tt.wantErr {
				if !IsCode(err, InvalidArgument) {
					t.Fatalf("expected INVALID_ARGUMENT, got %v", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestWriter_CreateResource_ShapeValidation(t *testing.T) {
	tests := []struct {
		name    string
		mutate  func(*ResourceWriteRequest)
		wantErr bool
	}{
		{"valid", func(r *ResourceWriteRequest) {}, false},
		{"blank actor", func(r *ResourceWriteRequest) { r.Actor = "" }, true},
		{"blank class code", func(r *ResourceWriteRequest) { r.Scope.ClassCode = "" }, true},
		{"blank family code", func(r *ResourceWriteRequest) { r.Scope.FamilyCode = "" }, true},
		{"blank type code", func(r *ResourceWriteRequest) { r.Scope.TypeCode = "" }, true},
		{"blank natural unit", func(r *ResourceWriteRequest) { r.NaturalUnit = "" }, true},
		{"attribute value unit code on non-quantity", func(r *ResourceWriteRequest) {
			r.Attributes = []AttributeValue{{Code: "gauge", Value: Value{Kind: ValueText, Text: "x", UnitCode: "m"}}}
		}, true},
		{"attribute value quantity missing unit code", func(r *ResourceWriteRequest) {
			r.Attributes = []AttributeValue{{Code: "length", Value: Value{Kind: ValueQuantity, Text: "1"}}}
		}, true},
		{"attribute value quantity ok", func(r *ResourceWriteRequest) {
			r.Attributes = []AttributeValue{{Code: "length", Value: Value{Kind: ValueQuantity, Text: "1", UnitCode: "m"}}}
		}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := validResourceWriteRequest()
			tt.mutate(&req)
			cap := &fakeWriteCapabilities{
				createResource: func(ctx context.Context, req ResourceWriteRequest) (Resource, error) {
					return Resource{ID: 1, Scope: req.Scope, Revision: 1}, nil
				},
			}
			w, err := NewWriter(cap)
			if err != nil {
				t.Fatalf("unexpected NewWriter error: %v", err)
			}
			_, err = w.CreateResource(context.Background(), req)
			if tt.wantErr {
				if !IsCode(err, InvalidArgument) {
					t.Fatalf("expected INVALID_ARGUMENT, got %v", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestWriter_NoUngraduatedMethodExported(t *testing.T) {
	typ := reflect.TypeOf((*WriteCapabilities)(nil)).Elem()
	if typ.NumMethod() != 2 {
		t.Fatalf("expected exactly 2 methods on WriteCapabilities, got %d: %v", typ.NumMethod(), typ)
	}
	for _, name := range []string{"CreateCatalog", "CreateResource"} {
		if _, ok := typ.MethodByName(name); !ok {
			t.Fatalf("expected WriteCapabilities to declare %s", name)
		}
	}
	writerType := reflect.TypeOf((*Writer)(nil))
	for i := 0; i < writerType.NumMethod(); i++ {
		name := writerType.Method(i).Name
		if name != "CreateCatalog" && name != "CreateResource" {
			t.Fatalf("unexpected exported Writer method %s; only CreateCatalog/CreateResource may be graduated in this change", name)
		}
	}
}

func TestWriteRequestCopy_CallerMutationAfterCall_NoEffect(t *testing.T) {
	var captured CatalogWriteRequest
	cap := &fakeWriteCapabilities{
		createCatalog: func(ctx context.Context, req CatalogWriteRequest) (CatalogRecord, error) {
			captured = req
			return CatalogRecord{Kind: req.Kind, ID: 1, Revision: 1, Values: req.Values}, nil
		},
	}
	w, err := NewWriter(cap)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	req := validCatalogWriteRequest()
	rec, err := w.CreateCatalog(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	req.Values["code"] = Value{Kind: ValueCode, Text: "MUTATED"}
	rec.Values["code"] = Value{Kind: ValueCode, Text: "MUTATED"}

	if captured.Values["code"].Text != "MAT" {
		t.Fatalf("caller mutation after call leaked into the capability call: %+v", captured.Values["code"])
	}
}
