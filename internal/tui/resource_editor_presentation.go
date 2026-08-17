package tui

import (
	"sort"

	"github.com/GARFEX33/garfex-costos-unitarios/internal/domain"
)

func (a *ResourcesWorkspaceAdapter) resourcePresentation(resource domain.Resource) (title string, fields []Field) {
	attributes := append([]domain.ResourceAttributeValue(nil), resource.Attributes...)
	sort.SliceStable(attributes, func(i, j int) bool { return attributes[i].AttributeCode < attributes[j].AttributeCode })

	fields = []Field{{Label: "Unidad natural", Value: resource.NaturalUnit}}
	for _, attribute := range attributes {
		if attribute.Text == notApplicableAttributeText {
			continue
		}
		label, value := a.resourceAttributePresentation(resource, attribute)
		fields = append(fields, Field{Label: label, Value: value})
	}

	title = resourceCatalogIdentity(a.catalog, resource)
	if presentation := a.describer.Describe(resource); presentation != "" {
		title += " — " + presentation
	}
	if !a.classIsActive(resource.ClassCode) {
		title += " (Clase inactiva)"
	}
	return title, fields
}
