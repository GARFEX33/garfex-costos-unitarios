package resourcecore

// CatalogWriteRequest creates one record of any registered catalog kind.
type CatalogWriteRequest struct {
	Actor  string
	Kind   KindCode
	Active bool
	Values map[string]Value
	Rules  []ApplicabilityRule // APLICABILIDAD aggregate; nil and empty differ
}

// ResourceWriteRequest creates one resource.
type ResourceWriteRequest struct {
	Actor       string
	Scope       ResourceScope
	NaturalUnit string
	Attributes  []AttributeValue
}
