package domain

import (
	"fmt"
	"strings"
)

// canonicalValue validates and canonicalizes value against attribute's
// definition. It takes the full ResourceAttribute (not just its
// AttributeDefinition) so a ValueTypeControlledOption value can be checked
// against attribute's own named OptionSet (design D3) rather than any
// option sharing the attribute code across every class.
func (c ResourceCatalog) canonicalValue(attribute ResourceAttribute, value ResourceAttributeValue) (ResourceAttributeValue, error) {
	definition := attribute.Definition
	value.AttributeCode = canonicalAttribute(value.AttributeCode)
	if value.Type != definition.ValueType {
		return ResourceAttributeValue{}, validation("attribute %q has type %q, want %q", value.AttributeCode, value.Type, definition.ValueType)
	}
	switch value.Type {
	case ValueTypeControlledOption:
		option, ok := c.option(attribute, value.OptionCode)
		if !ok {
			return ResourceAttributeValue{}, validation("option %q is invalid for attribute %q", value.OptionCode, value.AttributeCode)
		}
		value.OptionCode = option
	case ValueTypeQuantity:
		if value.Quantity == nil || value.Quantity.Value.IsNegative() || value.Quantity.Value.IsZero() {
			return ResourceAttributeValue{}, validation("quantity %q must be positive", value.AttributeCode)
		}
		value.Quantity.UnitCode = canonical(value.Quantity.UnitCode)
	case ValueTypeInteger:
		if value.Integer == nil {
			return ResourceAttributeValue{}, validation("integer %q is missing", value.AttributeCode)
		}
	case ValueTypeDecimal:
		if value.Decimal == nil {
			return ResourceAttributeValue{}, validation("decimal %q is missing", value.AttributeCode)
		}
	case ValueTypeBoolean:
		if value.Boolean == nil {
			return ResourceAttributeValue{}, validation("boolean %q is missing", value.AttributeCode)
		}
	case ValueTypeControlledText:
		if strings.TrimSpace(value.Text) == "" {
			return ResourceAttributeValue{}, validation("controlled text %q is empty", value.AttributeCode)
		}
		value.Text = strings.Join(strings.Fields(strings.TrimSpace(value.Text)), " ")
	default:
		return ResourceAttributeValue{}, validation("value type %q is unknown", value.Type)
	}
	return value, nil
}

// option resolves raw against attribute's own OptionSet (design D3) — an
// option sharing the attribute code but bound to a different OptionSet
// never matches.
func (c ResourceCatalog) option(attribute ResourceAttribute, raw string) (string, bool) {
	code := canonicalAttribute(attribute.Definition.Code)
	set := attribute.setKey()
	for _, option := range c.Options {
		if canonicalAttribute(option.AttributeCode) == code && canonicalOptionSet(option.OptionSet) == set && option.Code == raw {
			return option.Code, true
		}
	}
	return "", false
}

func (v ResourceAttributeValue) canonical(definition AttributeDefinition) string {
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

func ExactDuplicate(a, b Resource) bool { return a.IdentityKey != "" && a.IdentityKey == b.IdentityKey }

func canonical(value string) string {
	return strings.ToUpper(strings.Join(strings.Fields(strings.TrimSpace(value)), " "))
}

func canonicalAttribute(value string) string {
	return strings.ToLower(strings.Join(strings.Fields(strings.TrimSpace(value)), " "))
}
