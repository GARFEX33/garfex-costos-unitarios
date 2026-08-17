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
}
