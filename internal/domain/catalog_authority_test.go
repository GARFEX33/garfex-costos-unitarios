package domain

import (
	"sync"
	"testing"
)

func TestCatalogAuthorityDefensivelyClonesNestedCatalogData(t *testing.T) {
	input := SeedResourceCatalog()
	input.Classes[0].Aliases = []string{"original"}
	authority := NewCatalogAuthority(input)
	input.Classes[0].Aliases[0] = "mutated input"
	input.Attributes[3].Rules[0].When.Equals = "mutated input"

	current, _ := authority.Current()
	current.Classes[0].Aliases[0] = "mutated output"
	current.Attributes[3].Rules[0].When.Equals = "mutated output"
	fresh, _ := authority.Current()
	if fresh.Classes[0].Aliases[0] != "original" || fresh.Attributes[3].Rules[0].When.Equals != "DESNUDO" {
		t.Fatalf("authority leaked external mutation: class=%q rule=%q", fresh.Classes[0].Aliases[0], fresh.Attributes[3].Rules[0].When.Equals)
	}

	var wg sync.WaitGroup
	for range 8 {
		wg.Go(func() { snapshot, _ := authority.Current(); snapshot.Classes[0].Aliases[0] = "private" })
	}
	wg.Wait()
}
