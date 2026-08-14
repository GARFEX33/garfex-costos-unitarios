package domain

import (
	"errors"
	"strings"
	"testing"
)

// Task 2b.3: adapted from the pre-rename material_types_test.go.

func TestResourceValidationErrorsAreActionable(t *testing.T) {
	_, err := NewResource(SeedResourceCatalog(), conductoresScope, "M", nil)
	if err == nil || !strings.Contains(err.Error(), "required attribute") {
		t.Fatalf("error = %v, want missing attribute", err)
	}
	if !errors.Is(err, ErrResourceValidation) {
		t.Errorf("error = %v, want ErrResourceValidation", err)
	}
}

func without(values []ResourceAttributeValue, code string) []ResourceAttributeValue {
	out := make([]ResourceAttributeValue, 0, len(values))
	for _, value := range values {
		if value.AttributeCode != code {
			out = append(out, value)
		}
	}
	return out
}

func replace(values []ResourceAttributeValue, value ResourceAttributeValue) []ResourceAttributeValue {
	out := append([]ResourceAttributeValue(nil), values...)
	for i := range out {
		if out[i].AttributeCode == value.AttributeCode {
			out[i] = value
			return out
		}
	}
	return append(out, value)
}
