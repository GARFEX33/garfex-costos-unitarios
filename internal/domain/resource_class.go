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
// This is an additive, PR1-scoped type: it is not yet seeded into
// MaterialsCatalog or wired into any production code path. Later phases of
// recursos-maestro seed MATERIAL/MANO_DE_OBRA/EQUIPO_HERRAMIENTA and build
// the catalog-driven menu from it.
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
