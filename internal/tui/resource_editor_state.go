package tui

import "github.com/GARFEX33/garfex-costos-unitarios/internal/domain"

// resourceEditorState contains the cross-turn state for a resource editor.
type resourceEditorState struct {
	mode resourceEditorMode
	step resourceEditorStep

	class    string
	family   string
	itemType string

	attributes  []domain.ResourceAttribute
	nextIndex   int
	editingCode string
	editedCodes []string
	values      []domain.ResourceAttributeValue

	originalID     int64
	originalValues []domain.ResourceAttributeValue
	originalUnit   string
	currentUnit    string

	// originalRevision is the migration-8 CAS domain.Resource.Revision
	// captured from the existing resource — the resource-editor counterpart
	// of catalog_admin.go's catalogEditorState.revision. 0 for
	// editorModeCreate/editorModeDuplicate (no meaningful prior revision),
	// the existing resource's own Revision for editorModeEdit.
	// resource_editor.go's own startEditEditor call site (where originalID is
	// populated from resource.ID) is outside this slice's edit surface, so no
	// current caller populates this yet — 4G wires it, alongside switching to
	// the revision-aware call that reads it via persistenceRevision() below.
	originalRevision uint64
}
