package domain

import "testing"

func TestLifecycleTransitionsAreIdempotent(t *testing.T) {
	tests := []struct {
		name       string
		deactivate func() bool
		activate   func() bool
	}{
		{name: "supplier", deactivate: (&Supplier{Active: true}).Deactivate, activate: (&Supplier{}).Activate},
		{name: "branch", deactivate: (&Branch{Active: true}).Deactivate, activate: (&Branch{}).Activate},
		{name: "contact", deactivate: (&Contact{Active: true}).Deactivate, activate: (&Contact{}).Activate},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if !tt.deactivate() || tt.deactivate() {
				t.Fatal("Deactivate() must change once and then be idempotent")
			}
			if !tt.activate() || tt.activate() {
				t.Fatal("Activate() must change once and then be idempotent")
			}
		})
	}
}
