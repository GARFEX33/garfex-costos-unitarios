package domain

// ResourceClass is the top-level "Clase de recurso" catalog entry. Code is
// the internal join key and is never rendered; Name/Plural/Slug provide the
// user-visible labels for TUI menus and workspaces. Aliases/Keywords feed
// palette matching, Order provides deterministic menu sorting, and an
// inactive class is excluded from new-resource selection while existing
// resources under it remain readable.
//
// SeedResourceCatalog supplies the fixture catalog used by domain tests.
type ResourceClass struct {
	Code     string
	Name     string
	Plural   string
	Slug     string
	Aliases  []string
	Keywords []string
	Order    int
	Active   bool
}
