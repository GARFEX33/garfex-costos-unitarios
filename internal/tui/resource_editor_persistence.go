package tui

import "github.com/GARFEX33/garfex-costos-unitarios/internal/domain"

func domainScope(state *resourceEditorState) domain.ResourceScope {
	return domain.ResourceScope{ClassCode: state.class, FamilyCode: state.family, TypeCode: state.itemType}
}

func (state *resourceEditorState) persistenceValues() []domain.ResourceAttributeValue {
	return filterApplicableValues(state.attributes, state.values)
}

func filterApplicableValues(attributes []domain.ResourceAttribute, values []domain.ResourceAttributeValue) []domain.ResourceAttributeValue {
	applicable := map[string]bool{}
	for _, attribute := range attributes {
		mode, _, notApplicable := attribute.Effective(values)
		if mode != domain.ModeForbidden && !notApplicable {
			applicable[attribute.Definition.Code] = true
		}
	}
	var filtered []domain.ResourceAttributeValue
	for _, value := range values {
		if applicable[value.AttributeCode] {
			filtered = append(filtered, value)
		}
	}
	return filtered
}
