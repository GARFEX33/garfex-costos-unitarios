package resourcecore

func CloneStringSlice(s []string) []string {
	if s == nil {
		return nil
	}
	out := make([]string, len(s))
	copy(out, s)
	return out
}

func CloneValue(v Value) Value {
	out := v
	if v.Reference != nil {
		r := *v.Reference
		out.Reference = &r
	}
	out.Strings = CloneStringSlice(v.Strings)
	return out
}

func CloneCatalogRecord(rec CatalogRecord) CatalogRecord {
	out := rec
	out.Values = make(map[string]Value, len(rec.Values))
	for k, v := range rec.Values {
		out.Values[k] = CloneValue(v)
	}
	out.Rules = make([]ApplicabilityRule, len(rec.Rules))
	for i := range rec.Rules {
		r := rec.Rules[i]
		out.Rules[i] = ApplicabilityRule{AttributeCode: r.AttributeCode, Equals: CloneValue(r.Equals), Mode: r.Mode, IdentityParticipates: r.IdentityParticipates, NotApplicable: r.NotApplicable, Active: r.Active}
	}
	return out
}

func CloneCatalogDescriptor(desc CatalogDescriptor) CatalogDescriptor {
	out := desc
	out.Fields = make([]FieldDescriptor, len(desc.Fields))
	for i := range desc.Fields {
		f := desc.Fields[i]
		out.Fields[i] = FieldDescriptor{Name: f.Name, Label: f.Label, Kind: f.Kind, Required: f.Required, RefKind: f.RefKind, RefScopedBy: CloneStringSlice(f.RefScopedBy), EnumValues: append([]EnumValue(nil), f.EnumValues...)}
	}
	out.IdentityFields = CloneStringSlice(desc.IdentityFields)
	return out
}

func CloneResource(r Resource) Resource {
	out := r
	out.Attributes = make([]AttributeValue, len(r.Attributes))
	for i := range r.Attributes {
		a := r.Attributes[i]
		out.Attributes[i] = AttributeValue{Code: a.Code, Value: CloneValue(a.Value), UnitCode: a.UnitCode}
	}
	return out
}
