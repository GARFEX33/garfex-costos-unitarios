package suppliercore

import "testing"

func TestCloneContact_BranchIDPointerIsIndependent(t *testing.T) {
	branchID := int64(42)
	original := Contact{ID: 1, BranchID: &branchID}

	clone := CloneContact(original)
	if clone.BranchID == original.BranchID {
		t.Fatal("CloneContact returned the same BranchID pointer as the source")
	}
	if *clone.BranchID != 42 {
		t.Fatalf("clone.BranchID = %d, want 42", *clone.BranchID)
	}

	*original.BranchID = 999
	if *clone.BranchID != 42 {
		t.Fatalf("mutating source BranchID leaked into clone: clone.BranchID = %d, want 42", *clone.BranchID)
	}
}

func TestCloneContact_NilBranchIDStaysNil(t *testing.T) {
	clone := CloneContact(Contact{ID: 1, BranchID: nil})
	if clone.BranchID != nil {
		t.Fatalf("CloneContact(nil BranchID).BranchID = %v, want nil", clone.BranchID)
	}
}

func TestCloneSupplierSlice_MutatingCloneDoesNotAffectSource(t *testing.T) {
	source := []Supplier{{ID: 1, TradeName: "Original"}}
	clone := cloneSupplierSlice(source)

	clone[0].TradeName = "Mutated"
	if source[0].TradeName != "Original" {
		t.Fatalf("mutating clone leaked into source: source[0].TradeName = %q, want %q", source[0].TradeName, "Original")
	}
}

func TestCloneSupplierSlice_NilStaysNil(t *testing.T) {
	if got := cloneSupplierSlice(nil); got != nil {
		t.Fatalf("cloneSupplierSlice(nil) = %v, want nil", got)
	}
}

func TestCloneBranchSlice_MutatingCloneDoesNotAffectSource(t *testing.T) {
	source := []Branch{{ID: 1, Name: "Original"}}
	clone := cloneBranchSlice(source)

	clone[0].Name = "Mutated"
	if source[0].Name != "Original" {
		t.Fatalf("mutating clone leaked into source: source[0].Name = %q, want %q", source[0].Name, "Original")
	}
}

func TestCloneContactSlice_MutatingClonePointerDoesNotAffectSource(t *testing.T) {
	branchID := int64(7)
	source := []Contact{{ID: 1, BranchID: &branchID}}
	clone := cloneContactSlice(source)

	*clone[0].BranchID = 999
	if *source[0].BranchID != 7 {
		t.Fatalf("mutating clone's BranchID leaked into source: *source[0].BranchID = %d, want 7", *source[0].BranchID)
	}
}
