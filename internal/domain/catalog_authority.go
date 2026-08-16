package domain

import "sync"

// CatalogAuthority owns the immutable catalog version shared by one process.
type CatalogAuthority struct {
	mu      sync.RWMutex
	catalog ResourceCatalog
	version uint64
}

func NewCatalogAuthority(catalog ResourceCatalog) *CatalogAuthority {
	return &CatalogAuthority{catalog: cloneResourceCatalog(catalog), version: 1}
}

// Current returns one coherent catalog/version pair.
func (a *CatalogAuthority) Current() (ResourceCatalog, uint64) {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return cloneResourceCatalog(a.catalog), a.version
}

// Publish atomically replaces the catalog after its persistence transaction
// has succeeded.
func (a *CatalogAuthority) Publish(catalog ResourceCatalog) uint64 {
	catalog = cloneResourceCatalog(catalog)
	a.mu.Lock()
	defer a.mu.Unlock()
	a.catalog = catalog
	a.version++
	return a.version
}

func cloneResourceCatalog(c ResourceCatalog) ResourceCatalog {
	next := c
	next.Classes = append([]ResourceClass(nil), c.Classes...)
	for i := range next.Classes {
		next.Classes[i].Aliases = append([]string(nil), c.Classes[i].Aliases...)
		next.Classes[i].Keywords = append([]string(nil), c.Classes[i].Keywords...)
	}
	next.Families, next.Types = append([]ResourceFamily(nil), c.Families...), append([]ResourceType(nil), c.Types...)
	next.PresentationFields, next.Units = append([]PresentationField(nil), c.PresentationFields...), append([]UnitDefinition(nil), c.Units...)
	next.UnitPolicies, next.Definitions = append([]ResourceUnitPolicy(nil), c.UnitPolicies...), append([]AttributeDefinition(nil), c.Definitions...)
	next.Attributes = append([]ResourceAttribute(nil), c.Attributes...)
	for i := range next.Attributes {
		next.Attributes[i].Rules = append([]AttributeRule(nil), c.Attributes[i].Rules...)
	}
	next.Options, next.Relations = append([]AttributeOption(nil), c.Options...), append([]AttributeOptionRelation(nil), c.Relations...)
	return next
}
