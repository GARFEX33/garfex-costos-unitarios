package resourcecore

import "testing"

func TestCanonicalNumeric(t *testing.T) {
	if got, err := CanonicalIntString(42); err != nil || got != "42" {
		t.Fatalf("CanonicalIntString(42) = %q, %v", got, err)
	}
	if got, err := CanonicalDecimalString("1.200"); err != nil || got != "1.2" {
		t.Fatalf("CanonicalDecimalString(\"1.200\") = %q, %v", got, err)
	}
	if _, err := CanonicalDecimalString("not-a-number"); err == nil {
		t.Fatalf("CanonicalDecimalString expected error")
	}
}

func TestValueNotApplicable(t *testing.T) {
	if v := NewIntegerValue(42); v.Kind != ValueInteger || v.Text != "42" {
		t.Fatalf("integer value mismatch")
	}
	if d, err := NewDecimalValue("1.200"); err != nil || d.Kind != ValueDecimal || d.Text != "1.2" {
		t.Fatalf("decimal value mismatch")
	}
	if v := NotApplicableValue(); v.Kind != ValueNotApplicable {
		t.Fatalf("not-applicable kind mismatch")
	}
	if NewIntegerValue(0).Kind == ValueNotApplicable {
		t.Fatalf("zero integer must not equal NOT_APPLICABLE")
	}
}
