package suppliercore

// SupplierWriteRequest creates one Supplier. Actor is required for shape
// validation; the five content fields are optional at this layer — their
// combined-content rule is domain.NewSupplier's sole authority. There is no
// Active field: created suppliers are always active.
type SupplierWriteRequest struct {
	Actor         string
	TradeName     string
	LegalName     string
	TaxIdentifier string
	Website       string
	Notes         string
}
