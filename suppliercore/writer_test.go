package suppliercore

import (
	"context"
	"reflect"
	"testing"
)

type fakeWriteCapabilities struct {
	createSupplier func(ctx context.Context, req SupplierWriteRequest) (Supplier, error)
}

func (f *fakeWriteCapabilities) CreateSupplier(ctx context.Context, req SupplierWriteRequest) (Supplier, error) {
	return f.createSupplier(ctx, req)
}

func validSupplierWriteRequest() SupplierWriteRequest {
	return SupplierWriteRequest{Actor: "PI", TradeName: "ACME", LegalName: "ACME SA", TaxIdentifier: "TAX1"}
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

func TestWriter_NoUngraduatedMethodExported(t *testing.T) {
	typ := reflect.TypeOf((*WriteCapabilities)(nil)).Elem()
	if typ.NumMethod() != 1 {
		t.Fatalf("expected exactly 1 method on WriteCapabilities, got %d: %v", typ.NumMethod(), typ)
	}
	if _, ok := typ.MethodByName("CreateSupplier"); !ok {
		t.Fatalf("expected WriteCapabilities to declare CreateSupplier")
	}
	writerType := reflect.TypeOf((*Writer)(nil))
	allowed := map[string]bool{"CreateSupplier": true}
	for i := 0; i < writerType.NumMethod(); i++ {
		name := writerType.Method(i).Name
		if !allowed[name] {
			t.Fatalf("unexpected exported Writer method %s; only CreateSupplier is graduated so far", name)
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
