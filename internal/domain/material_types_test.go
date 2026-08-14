package domain

import (
	"errors"
	"strings"
	"testing"
)

func TestMaterialValidationErrorsAreActionable(t *testing.T) {
	_, err := NewMaterial(NewMaterialsCatalog(), "CONDUCTORES", "CABLE", "M", nil)
	if err == nil || !strings.Contains(err.Error(), "required attribute") {
		t.Fatalf("error = %v, want missing attribute", err)
	}
	if !errors.Is(err, ErrMaterialValidation) {
		t.Errorf("error = %v, want ErrMaterialValidation", err)
	}
}

func without(values []MaterialAttributeValue, code string) []MaterialAttributeValue {
	out := make([]MaterialAttributeValue, 0, len(values))
	for _, value := range values {
		if value.AttributeCode != code {
			out = append(out, value)
		}
	}
	return out
}

func replace(values []MaterialAttributeValue, value MaterialAttributeValue) []MaterialAttributeValue {
	out := append([]MaterialAttributeValue(nil), values...)
	for i := range out {
		if out[i].AttributeCode == value.AttributeCode {
			out[i] = value
			return out
		}
	}
	return append(out, value)
}
