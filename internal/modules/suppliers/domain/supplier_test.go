package domain

import (
	"errors"
	"testing"
)

func TestNewSupplierSupportsProgressiveEnrichment(t *testing.T) {
	tests := []struct {
		name    string
		details SupplierDetails
		wantErr bool
	}{
		{name: "trade name only", details: SupplierDetails{TradeName: "  Acme  "}},
		{name: "legal name only", details: SupplierDetails{LegalName: "Acme SA"}},
		{name: "tax identifier only", details: SupplierDetails{TaxIdentifier: "ABC123"}},
		{name: "no business identifier", details: SupplierDetails{Notes: "later"}, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NewSupplier(tt.details)
			if tt.wantErr {
				if !errors.Is(err, ErrValidation) {
					t.Fatalf("NewSupplier() error = %v, want ErrValidation", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("NewSupplier() error = %v", err)
			}
			if !got.Active {
				t.Fatal("new supplier must be active")
			}
			if got.TradeName == "  Acme  " {
				t.Fatal("trade name was not canonicalized")
			}
		})
	}
}
