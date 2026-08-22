package suppliercore

func copyBranchID(id *int64) *int64 {
	if id == nil {
		return nil
	}
	v := *id
	return &v
}

// CloneSupplier returns a copy of s. Supplier is a flat value type, so a
// plain copy is already a full clone.
func CloneSupplier(s Supplier) Supplier { return s }

// CloneBranch returns a copy of b. Branch is a flat value type, so a plain
// copy is already a full clone.
func CloneBranch(b Branch) Branch { return b }

// CloneContact returns a copy of c with an independent BranchID pointer.
func CloneContact(c Contact) Contact {
	out := c
	out.BranchID = copyBranchID(c.BranchID)
	return out
}

func cloneSupplierSlice(s []Supplier) []Supplier {
	if s == nil {
		return nil
	}
	out := make([]Supplier, len(s))
	copy(out, s)
	return out
}

func cloneBranchSlice(b []Branch) []Branch {
	if b == nil {
		return nil
	}
	out := make([]Branch, len(b))
	copy(out, b)
	return out
}

func cloneContactSlice(c []Contact) []Contact {
	if c == nil {
		return nil
	}
	out := make([]Contact, len(c))
	for i := range c {
		out[i] = CloneContact(c[i])
	}
	return out
}
