package domain

import (
	"sort"
	"strings"
)

// Describe resolves the canonical, human-readable presentation of material:
// its ProductType's name followed by its catalog-configured
// PresentationFields, in order. It never falls back to listing every
// attribute — a ProductType with no PresentationFields configured degrades
// to just its own name (an intentionally incomplete-but-safe result, not an
// improvised technical dump), and a Material whose ProductType cannot be
// resolved in the catalog returns "".
func (c MaterialsCatalog) Describe(material Material) string {
	family := canonical(material.FamilyCode)
	productTypeCode := canonical(material.ProductTypeCode)
	productType, ok := c.productType(family, productTypeCode)
	if !ok {
		return ""
	}
	fields := c.presentationFields(productTypeCode)
	if len(fields) == 0 {
		return productType.Name
	}
	byCode := map[string]MaterialAttributeValue{}
	for _, value := range material.Attributes {
		byCode[canonicalAttribute(value.AttributeCode)] = value
	}
	parts := []string{productType.Name}
	for _, field := range fields {
		value, present := byCode[canonicalAttribute(field.AttributeCode)]
		if !present || value.Text == NotApplicableText {
			continue
		}
		parts = append(parts, value.canonical(c.definitionFor(field.AttributeCode)))
	}
	return strings.Join(parts, " ")
}

func (c MaterialsCatalog) productType(family, code string) (ProductType, bool) {
	for _, productType := range c.ProductTypes {
		if canonical(productType.FamilyCode) == family && canonical(productType.Code) == code {
			return productType, true
		}
	}
	return ProductType{}, false
}

func (c MaterialsCatalog) presentationFields(productType string) []PresentationField {
	var fields []PresentationField
	for _, field := range c.PresentationFields {
		if canonical(field.ProductTypeCode) == productType {
			fields = append(fields, field)
		}
	}
	sort.Slice(fields, func(i, j int) bool { return fields[i].Position < fields[j].Position })
	return fields
}

func (c MaterialsCatalog) definitionFor(code string) AttributeDefinition {
	code = canonicalAttribute(code)
	for _, definition := range c.Definitions {
		if canonicalAttribute(definition.Code) == code {
			return definition
		}
	}
	return AttributeDefinition{}
}
