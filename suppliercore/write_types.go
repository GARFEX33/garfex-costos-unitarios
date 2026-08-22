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

// SupplierUpdateRequest replaces one existing Supplier's content, identified
// by ID. Actor and a positive ID are required for shape validation; the five
// content fields are optional at this layer — their combined-content rule is
// domain.NewSupplier's sole authority. Update is a full replace, not a
// patch: every content field is written exactly as supplied, so a field left
// empty clears the stored value. There is no Active field: Update never
// changes lifecycle state.
type SupplierUpdateRequest struct {
	Actor         string
	ID            int64
	TradeName     string
	LegalName     string
	TaxIdentifier string
	Website       string
	Notes         string
}
