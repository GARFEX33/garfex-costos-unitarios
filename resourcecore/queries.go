package resourcecore

type CatalogQuery struct {
	Kind          KindCode
	Scope         LifecycleScope
	Text          string
	Limit, Offset int
}
type ResourceQuery struct {
	Scope                                 LifecycleScope
	Text, ClassCode, FamilyCode, TypeCode string
	Limit, Offset                         int
}
type CatalogPage struct {
	Query                CatalogQuery
	Records              []CatalogRecord
	HasPrevious, HasNext bool
}
type ResourcePage struct {
	Query                ResourceQuery
	Resources            []Resource
	HasPrevious, HasNext bool
}
