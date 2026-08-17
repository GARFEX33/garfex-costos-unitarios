// Package domain defines the Supplier Master business model and persistence port.
package domain

import (
	"strings"
	"time"
)

// Supplier is an independent commercial party. Its optional data can be
// progressively enriched; at least one business identifier must be present.
type Supplier struct {
	ID            int64
	TradeName     string
	LegalName     string
	TaxIdentifier string
	Website       string
	Notes         string
	Active        bool
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

type SupplierDetails struct {
	TradeName     string
	LegalName     string
	TaxIdentifier string
	Website       string
	Notes         string
}

func NewSupplier(details SupplierDetails) (Supplier, error) {
	details = details.canonical()
	if details.TradeName == "" && details.LegalName == "" && details.TaxIdentifier == "" {
		return Supplier{}, NewValidationError("supplier", "trade name, legal name, or tax identifier is required")
	}
	return Supplier{TradeName: details.TradeName, LegalName: details.LegalName, TaxIdentifier: details.TaxIdentifier, Website: details.Website, Notes: details.Notes, Active: true}, nil
}

func (s Supplier) WithDetails(details SupplierDetails) (Supplier, error) {
	next, err := NewSupplier(details)
	if err != nil {
		return Supplier{}, err
	}
	s.TradeName, s.LegalName, s.TaxIdentifier = next.TradeName, next.LegalName, next.TaxIdentifier
	s.Website, s.Notes = next.Website, next.Notes
	return s, nil
}

func (d SupplierDetails) canonical() SupplierDetails {
	d.TradeName = strings.TrimSpace(d.TradeName)
	d.LegalName = strings.TrimSpace(d.LegalName)
	d.TaxIdentifier = strings.TrimSpace(d.TaxIdentifier)
	d.Website = strings.TrimSpace(d.Website)
	d.Notes = strings.TrimSpace(d.Notes)
	return d
}
