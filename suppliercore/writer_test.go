package suppliercore

import (
	"context"
	"reflect"
	"testing"
)

type fakeWriteCapabilities struct {
	createSupplier func(ctx context.Context, req SupplierWriteRequest) (Supplier, error)
	updateSupplier func(ctx context.Context, req SupplierUpdateRequest) (Supplier, error)
}

func (f *fakeWriteCapabilities) CreateSupplier(ctx context.Context, req SupplierWriteRequest) (Supplier, error) {
	return f.createSupplier(ctx, req)
}

func (f *fakeWriteCapabilities) UpdateSupplier(ctx context.Context, req SupplierUpdateRequest) (Supplier, error) {
	return f.updateSupplier(ctx, req)
}

func validSupplierWriteRequest() SupplierWriteRequest {
	return SupplierWriteRequest{Actor: "PI", TradeName: "ACME", LegalName: "ACME SA", TaxIdentifier: "TAX1"}
}

func validSupplierUpdateRequest() SupplierUpdateRequest {
	return SupplierUpdateRequest{Actor: "PI", ID: 1, TradeName: "ACME", LegalName: "ACME SA", TaxIdentifier: "TAX1"}
}

func TestNewWriter_NilCapability_ReturnsInvalidArgument(t *testing.T) {
	w, err := NewWriter(nil)
	if w != nil {
		t.Fatalf("expected nil Writer, got %+v", w)
	}
	if !IsCode(err, InvalidArgument) {
		t.Fatalf("expected INVALID_ARGUMENT, got %v", err)
	}
}

func TestWriter_CreateSupplier_BlankActor_RejectsWithoutDelegating(t *testing.T) {
	for _, actor := range []string{"", "   "} {
		called := false
		cap := &fakeWriteCapabilities{
			createSupplier: func(ctx context.Context, req SupplierWriteRequest) (Supplier, error) {
				called = true
				return Supplier{}, nil
			},
		}
		w, err := NewWriter(cap)
		if err != nil {
			t.Fatalf("unexpected NewWriter error: %v", err)
		}
		req := validSupplierWriteRequest()
		req.Actor = actor
		_, err = w.CreateSupplier(context.Background(), req)
		if !IsCode(err, InvalidArgument) {
			t.Fatalf("actor %q: expected INVALID_ARGUMENT, got %v", actor, err)
		}
		if called {
			t.Fatalf("actor %q: expected the capability to not be called for a shape-invalid request", actor)
		}
	}
}

func TestWriter_CreateSupplier_DelegatesAndClonesResult(t *testing.T) {
	want := Supplier{ID: 1, TradeName: "ACME", LegalName: "ACME SA", TaxIdentifier: "TAX1", Active: true}
	cap := &fakeWriteCapabilities{
		createSupplier: func(ctx context.Context, req SupplierWriteRequest) (Supplier, error) {
			return want, nil
		},
	}
	w, err := NewWriter(cap)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got, err := w.CreateSupplier(context.Background(), validSupplierWriteRequest())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != want {
		t.Fatalf("CreateSupplier result = %+v, want %+v", got, want)
	}
}

func TestWriter_CreateSupplier_MutatingRequestAfterCall_NoLeak(t *testing.T) {
	var captured SupplierWriteRequest
	cap := &fakeWriteCapabilities{
		createSupplier: func(ctx context.Context, req SupplierWriteRequest) (Supplier, error) {
			captured = req
			return Supplier{TradeName: req.TradeName}, nil
		},
	}
	w, err := NewWriter(cap)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	req := validSupplierWriteRequest()
	if _, err := w.CreateSupplier(context.Background(), req); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	req.TradeName = "MUTATED"

	if captured.TradeName != "ACME" {
		t.Fatalf("caller mutation after call leaked into the capability call: %+v", captured)
	}
}

func TestWriter_UpdateSupplier_RejectsBlankActorOrNonPositiveID(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*SupplierUpdateRequest)
	}{
		{"blank actor", func(r *SupplierUpdateRequest) { r.Actor = "" }},
		{"whitespace actor", func(r *SupplierUpdateRequest) { r.Actor = "   " }},
		{"zero id", func(r *SupplierUpdateRequest) { r.ID = 0 }},
		{"negative id", func(r *SupplierUpdateRequest) { r.ID = -1 }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			called := false
			cap := &fakeWriteCapabilities{
				updateSupplier: func(ctx context.Context, req SupplierUpdateRequest) (Supplier, error) {
					called = true
					return Supplier{}, nil
				},
			}
			w, err := NewWriter(cap)
			if err != nil {
				t.Fatalf("unexpected NewWriter error: %v", err)
			}
			req := validSupplierUpdateRequest()
			tt.mutate(&req)
			_, err = w.UpdateSupplier(context.Background(), req)
			if !IsCode(err, InvalidArgument) {
				t.Fatalf("expected INVALID_ARGUMENT, got %v", err)
			}
			if called {
				t.Fatalf("expected the capability to not be called for a shape-invalid request")
			}
		})
	}
}

func TestWriter_UpdateSupplier_DelegatesAndClonesResult(t *testing.T) {
	want := Supplier{ID: 1, TradeName: "ACME", LegalName: "ACME SA", TaxIdentifier: "TAX1", Active: true}
	cap := &fakeWriteCapabilities{
		updateSupplier: func(ctx context.Context, req SupplierUpdateRequest) (Supplier, error) {
			return want, nil
		},
	}
	w, err := NewWriter(cap)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got, err := w.UpdateSupplier(context.Background(), validSupplierUpdateRequest())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != want {
		t.Fatalf("UpdateSupplier result = %+v, want %+v", got, want)
	}
}

func TestWriter_UpdateSupplier_MutatingRequestAfterCall_NoLeak(t *testing.T) {
	var captured SupplierUpdateRequest
	cap := &fakeWriteCapabilities{
		updateSupplier: func(ctx context.Context, req SupplierUpdateRequest) (Supplier, error) {
			captured = req
			return Supplier{TradeName: req.TradeName}, nil
		},
	}
	w, err := NewWriter(cap)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	req := validSupplierUpdateRequest()
	if _, err := w.UpdateSupplier(context.Background(), req); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	req.TradeName = "MUTATED"

	if captured.TradeName != "ACME" {
		t.Fatalf("caller mutation after call leaked into the capability call: %+v", captured)
	}
}

func TestWriter_NoUngraduatedMethodExported(t *testing.T) {
	typ := reflect.TypeOf((*WriteCapabilities)(nil)).Elem()
	if typ.NumMethod() != 2 {
		t.Fatalf("expected exactly 2 methods on WriteCapabilities, got %d: %v", typ.NumMethod(), typ)
	}
	for _, name := range []string{"CreateSupplier", "UpdateSupplier"} {
		if _, ok := typ.MethodByName(name); !ok {
			t.Fatalf("expected WriteCapabilities to declare %s", name)
		}
	}
	writerType := reflect.TypeOf((*Writer)(nil))
	allowed := map[string]bool{"CreateSupplier": true, "UpdateSupplier": true}
	for i := 0; i < writerType.NumMethod(); i++ {
		name := writerType.Method(i).Name
		if !allowed[name] {
			t.Fatalf("unexpected exported Writer method %s; only CreateSupplier and UpdateSupplier are graduated so far", name)
		}
	}
}

func TestSupplierWriteRequest_NoReferenceTypedField(t *testing.T) {
	typ := reflect.TypeOf(SupplierWriteRequest{})
	for i := 0; i < typ.NumField(); i++ {
		f := typ.Field(i)
		switch f.Type.Kind() {
		case reflect.Pointer, reflect.Slice, reflect.Map, reflect.Chan, reflect.Func:
			t.Fatalf("SupplierWriteRequest.%s is reference-typed (%v); this would require a defensive clone helper that does not exist", f.Name, f.Type.Kind())
		}
	}
}

func TestSupplierUpdateRequest_NoReferenceTypedField(t *testing.T) {
	typ := reflect.TypeOf(SupplierUpdateRequest{})
	for i := 0; i < typ.NumField(); i++ {
		f := typ.Field(i)
		switch f.Type.Kind() {
		case reflect.Pointer, reflect.Slice, reflect.Map, reflect.Chan, reflect.Func:
			t.Fatalf("SupplierUpdateRequest.%s is reference-typed (%v); this would require a defensive clone helper that does not exist", f.Name, f.Type.Kind())
		}
	}
}

func TestWriter_NoDeleteOrHardDeleteMethodExported(t *testing.T) {
	for _, typ := range []reflect.Type{
		reflect.TypeOf((*ReadCapabilities)(nil)).Elem(),
		reflect.TypeOf((*Reader)(nil)),
		reflect.TypeOf((*WriteCapabilities)(nil)).Elem(),
		reflect.TypeOf((*Writer)(nil)),
	} {
		for i := 0; i < typ.NumMethod(); i++ {
			name := typ.Method(i).Name
			if name == "Delete" || name == "HardDelete" || (len(name) >= 6 && name[:6] == "Delete") || (len(name) >= 10 && name[:10] == "HardDelete") {
				t.Fatalf("%v exports %s; no delete or hard-delete method exists internally for this module", typ, name)
			}
		}
	}
}
