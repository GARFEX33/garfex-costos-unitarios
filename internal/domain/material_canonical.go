package domain

import (
	"fmt"
	"strings"
)

func (c MaterialsCatalog) canonicalValue(definition AttributeDefinition, value MaterialAttributeValue) (MaterialAttributeValue, error) {
	value.AttributeCode = canonicalAttribute(value.AttributeCode)
	if value.Type != definition.ValueType {
		return MaterialAttributeValue{}, validation("attribute %q has type %q, want %q", value.AttributeCode, value.Type, definition.ValueType)
	}
	switch value.Type {
	case ValueTypeControlledOption:
		option, ok := c.option(value.AttributeCode, value.OptionCode)
		if !ok {
			return MaterialAttributeValue{}, validation("option %q is invalid for attribute %q", value.OptionCode, value.AttributeCode)
		}
		value.OptionCode = option
	case ValueTypeQuantity:
		if value.Quantity == nil || value.Quantity.Value.IsNegative() || value.Quantity.Value.IsZero() {
			return MaterialAttributeValue{}, validation("quantity %q must be positive", value.AttributeCode)
		}
		value.Quantity.UnitCode = canonical(value.Quantity.UnitCode)
	case ValueTypeInteger:
		if value.Integer == nil {
			return MaterialAttributeValue{}, validation("integer %q is missing", value.AttributeCode)
		}
	case ValueTypeDecimal:
		if value.Decimal == nil {
			return MaterialAttributeValue{}, validation("decimal %q is missing", value.AttributeCode)
		}
	case ValueTypeBoolean:
		if value.Boolean == nil {
			return MaterialAttributeValue{}, validation("boolean %q is missing", value.AttributeCode)
		}
	case ValueTypeControlledText:
		if strings.TrimSpace(value.Text) == "" {
			return MaterialAttributeValue{}, validation("controlled text %q is empty", value.AttributeCode)
		}
		value.Text = strings.Join(strings.Fields(strings.TrimSpace(value.Text)), " ")
	default:
		return MaterialAttributeValue{}, validation("value type %q is unknown", value.Type)
	}
	return value, nil
}

func (c MaterialsCatalog) option(attribute, raw string) (string, bool) {
	attribute = canonicalAttribute(attribute)
	for _, option := range c.Options {
		if canonicalAttribute(option.AttributeCode) == attribute && option.Code == raw {
			return option.Code, true
		}
	}
	return "", false
}

func (v MaterialAttributeValue) canonical(definition AttributeDefinition) string {
	switch v.Type {
	case ValueTypeControlledOption:
		return v.OptionCode
	case ValueTypeQuantity:
		return canonicalQuantity(definition.Dimension, *v.Quantity)
	case ValueTypeInteger:
		return fmt.Sprintf("%d", *v.Integer)
	case ValueTypeDecimal:
		return v.Decimal.String()
	case ValueTypeBoolean:
		return fmt.Sprintf("%t", *v.Boolean)
	default:
		return canonical(v.Text)
	}
}

func canonicalQuantity(dimension string, quantity Quantity) string {
	unit := canonical(quantity.UnitCode)
	return quantity.Value.String() + " " + unit
}

func ExactDuplicate(a, b Material) bool { return a.IdentityKey != "" && a.IdentityKey == b.IdentityKey }

func canonical(value string) string {
	return strings.ToUpper(strings.Join(strings.Fields(strings.TrimSpace(value)), " "))
}

func canonicalAttribute(value string) string {
	return strings.ToLower(strings.Join(strings.Fields(strings.TrimSpace(value)), " "))
}
