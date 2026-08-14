package domain

// ResourceClass is the top-level "Clase de recurso" catalog entry
// (recursos-maestro design §2) — e.g. MATERIAL, MANO_DE_OBRA,
// EQUIPO_HERRAMIENTA. Code is the internal join key and is never rendered;
// Name/Plural/Slug are the Spanish, user-visible copy a later TUI phase
// derives its menu/palette/workspace surfaces from. Aliases/Keywords feed
// palette fuzzy matching; Order is the deterministic menu sort key;
// Active == false hides the class from menu/palette and from create-flow
// class selection while leaving resources already created under it
// readable (owner-confirmed behavior, recursos-maestro proposal id 1005).
//
// Seeded by NewResourceCatalog() (resource_catalog.go) with the MATERIAL
// class as of PR2a; MANO_DE_OBRA/EQUIPO_HERRAMIENTA and the catalog-driven
// TUI menu are later recursos-maestro phases.
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
