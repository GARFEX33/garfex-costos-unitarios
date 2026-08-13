package tui

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/GARFEX33/garfex-costos-unitarios/internal/domain"
)

// materialEditorKey identifies every QuestionRequest the create/edit editor
// emits (family, product type, each attribute, the natural unit). Every step
// shares the same Key so respondToEditor can claim any answer directed at
// the in-progress editor without tracking a separate key per step.
const materialEditorKey = "material-editor"

// searchableOptionThreshold is the option-count above which an attribute's
// question uses SelectionSearchable instead of SelectionSingle — e.g. gauge
// (12 options) is searchable, color (5 options) is not.
const searchableOptionThreshold = 6

type materialEditorStep int

const (
	editorStepFamily materialEditorStep = iota
	editorStepProductType
	editorStepAttribute
	editorStepUnit
)

// materialEditorState is the in-progress "nuevo material" create flow's
// cross-turn state (see MaterialsWorkspaceAdapter.editor). It is entirely
// catalog-driven: nothing here branches on a specific Family/ProductType.
type materialEditorState struct {
	step        materialEditorStep
	family      string
	productType string
	// attributes are resolved once family+productType are both known, in
	// catalog declaration order.
	attributes []domain.FamilyAttribute
	// nextIndex is the index into attributes for the next question to ask.
	nextIndex int
	values    []domain.MaterialAttributeValue
}

// startCreateEditor begins the "nuevo material" flow with the first
// question: which Family to create.
func (a *MaterialsWorkspaceAdapter) startCreateEditor() (InteractionResponse, error) {
	a.editor = &materialEditorState{step: editorStepFamily}
	catalog := domain.NewMaterialsCatalog()
	options := make([]Option, len(catalog.Families))
	for i, family := range catalog.Families {
		options[i] = Option{ID: family.Code, Label: family.Name, Value: family.Code}
	}
	prompt := "¿Qué familia querés crear?"
	return InteractionResponse{Pending: QuestionRequest{
		ID:            materialEditorKey,
		Key:           materialEditorKey,
		Prompt:        prompt,
		Question:      prompt,
		SelectionMode: SelectionSingle,
		Options:       options,
	}}, nil
}

// respondToEditor handles one InteractionInput while a.editor is
// in-progress. The returned bool reports whether input actually belonged to
// the editor (its own Key, or a cancellation) — when false, the caller
// (Respond) must fall through to its normal handling instead of this
// method silently swallowing unrelated input.
func (a *MaterialsWorkspaceAdapter) respondToEditor(ctx context.Context, input InteractionInput) (InteractionResponse, bool) {
	if input.Kind == InputCancel {
		a.editor = nil
		return InteractionResponse{Messages: []InteractionMessage{
			TextMessage{Text: "Se canceló la creación del material."},
		}}, true
	}
	if input.Key != materialEditorKey {
		return InteractionResponse{}, false
	}

	catalog := domain.NewMaterialsCatalog()
	state := a.editor
	switch state.step {
	case editorStepFamily:
		state.family = input.Value
		state.step = editorStepProductType
		return a.productTypeQuestion(catalog), true
	case editorStepProductType:
		state.productType = input.Value
		state.attributes = catalog.AttributesFor(state.family, state.productType)
		state.nextIndex = 0
		state.step = editorStepAttribute
		return a.advanceEditor(catalog), true
	case editorStepAttribute:
		return a.answerAttribute(catalog, input.Value), true
	case editorStepUnit:
		return a.finishEditor(ctx, catalog, input.Value), true
	}
	return InteractionResponse{}, true
}

// productTypeQuestion builds the second wizard step: which ProductType of
// the already-chosen Family to create.
func (a *MaterialsWorkspaceAdapter) productTypeQuestion(catalog domain.MaterialsCatalog) InteractionResponse {
	productTypes := catalog.ProductTypesFor(a.editor.family)
	options := make([]Option, len(productTypes))
	for i, productType := range productTypes {
		options[i] = Option{ID: productType.Code, Label: productType.Name, Value: productType.Code}
	}
	prompt := "¿Qué tipo de producto querés crear?"
	return InteractionResponse{Pending: QuestionRequest{
		ID:            materialEditorKey,
		Key:           materialEditorKey,
		Prompt:        prompt,
		Question:      prompt,
		SelectionMode: SelectionSingle,
		Options:       options,
	}}
}

// advanceEditor walks state.attributes from nextIndex, silently skipping any
// attribute whose FamilyAttribute.Effective resolves to ModeForbidden or
// notApplicable — this is exactly how e.g. insulation=DESNUDO skips
// color/voltage without any Family/ProductType-specific code here. It stops
// at the first attribute that must be asked about, or moves on to the
// NaturalUnit question once every attribute has been resolved.
func (a *MaterialsWorkspaceAdapter) advanceEditor(catalog domain.MaterialsCatalog) InteractionResponse {
	state := a.editor
	for state.nextIndex < len(state.attributes) {
		attribute := state.attributes[state.nextIndex]
		mode, _, notApplicable := attribute.Effective(state.values)
		if mode == domain.ModeForbidden || notApplicable {
			state.nextIndex++
			continue
		}
		response, err := a.attributeQuestion(catalog, attribute)
		if err != nil {
			a.editor = nil
			return InteractionResponse{Messages: []InteractionMessage{
				ErrorMessage{Text: "No pude continuar con la creación del material."},
			}}
		}
		return response
	}
	state.step = editorStepUnit
	return a.unitQuestion(catalog)
}

// attributeQuestion builds the QuestionRequest for one FamilyAttribute.
// Only CONTROLLED_OPTION and QUANTITY are implemented (the only two
// AttributeValueTypes any real catalog attribute uses today, per the
// PR-E2 scope cut) — any other value type is a deliberate, explicit error
// rather than a guessed-at UI.
func (a *MaterialsWorkspaceAdapter) attributeQuestion(catalog domain.MaterialsCatalog, attribute domain.FamilyAttribute) (InteractionResponse, error) {
	prompt := attribute.Definition.Name
	switch attribute.Definition.ValueType {
	case domain.ValueTypeControlledOption:
		options := catalog.ValidOptions(attribute.Definition.Code, a.editor.values)
		selectOptions := make([]Option, len(options))
		for i, option := range options {
			selectOptions[i] = Option{ID: option.Code, Label: option.Label, Value: option.Code}
		}
		selectionMode := SelectionSingle
		if len(selectOptions) > searchableOptionThreshold {
			selectionMode = SelectionSearchable
		}
		return InteractionResponse{Pending: QuestionRequest{
			ID:            materialEditorKey,
			Key:           materialEditorKey,
			Prompt:        prompt,
			Question:      prompt,
			SelectionMode: selectionMode,
			Options:       selectOptions,
		}}, nil
	case domain.ValueTypeQuantity:
		hintedPrompt := prompt + ` (ej. 600 V, 1 kV)`
		return InteractionResponse{Pending: QuestionRequest{
			ID:            materialEditorKey,
			Key:           materialEditorKey,
			Prompt:        hintedPrompt,
			Question:      hintedPrompt,
			SelectionMode: SelectionSingle,
			AllowCustom:   true,
		}}, nil
	default:
		return InteractionResponse{}, fmt.Errorf("material editor: unsupported attribute value type %q for attribute %q", attribute.Definition.ValueType, attribute.Definition.Code)
	}
}

// answerAttribute handles the answer for state.attributes[state.nextIndex].
func (a *MaterialsWorkspaceAdapter) answerAttribute(catalog domain.MaterialsCatalog, value string) InteractionResponse {
	state := a.editor
	attribute := state.attributes[state.nextIndex]
	switch attribute.Definition.ValueType {
	case domain.ValueTypeControlledOption:
		state.values = append(state.values, domain.OptionValue(attribute.Definition.Code, value))
		state.nextIndex++
		return a.advanceEditor(catalog)
	case domain.ValueTypeQuantity:
		numeric, unit, ok := splitQuantityInput(value)
		if !ok {
			response, err := a.attributeQuestion(catalog, attribute)
			if err != nil {
				a.editor = nil
				return InteractionResponse{Messages: []InteractionMessage{
					ErrorMessage{Text: "No pude continuar con la creación del material."},
				}}
			}
			response.Messages = []InteractionMessage{
				ErrorMessage{Text: fmt.Sprintf("No entendí %q. Escribí un valor y una unidad, por ejemplo \"600 V\".", value)},
			}
			return response
		}
		state.values = append(state.values, domain.QuantityValue(attribute.Definition.Code, numeric, unit))
		state.nextIndex++
		return a.advanceEditor(catalog)
	default:
		a.editor = nil
		return InteractionResponse{Messages: []InteractionMessage{
			ErrorMessage{Text: "No pude continuar con la creación del material."},
		}}
	}
}

// splitQuantityInput parses free text like "600 V" or "1 kV" into its
// numeric and unit parts by splitting on the last whitespace-separated
// token. Input with fewer than two whitespace-separated tokens (e.g.
// "notanumber") is reported as unparseable via ok=false.
func splitQuantityInput(value string) (numeric, unit string, ok bool) {
	fields := strings.Fields(value)
	if len(fields) < 2 {
		return "", "", false
	}
	unit = fields[len(fields)-1]
	numeric = strings.Join(fields[:len(fields)-1], " ")
	return numeric, unit, true
}

// unitQuestion builds the final wizard step: which NaturalUnit to store the
// material under.
func (a *MaterialsWorkspaceAdapter) unitQuestion(catalog domain.MaterialsCatalog) InteractionResponse {
	units := catalog.NaturalUnitsFor(a.editor.family)
	options := make([]Option, len(units))
	for i, unit := range units {
		options[i] = Option{ID: unit.Code, Label: unit.Symbol, Value: unit.Code}
	}
	prompt := "¿Cuál es la unidad natural?"
	return InteractionResponse{Pending: QuestionRequest{
		ID:            materialEditorKey,
		Key:           materialEditorKey,
		Prompt:        prompt,
		Question:      prompt,
		SelectionMode: SelectionSingle,
		Options:       options,
	}}
}

// finishEditor builds the candidate Material from the fully-answered editor
// state and persists it. It always resets a.editor to nil — success,
// validation failure, and every Create error all end the flow; simplicity
// over cleverness, an edge case a fresh "nuevo material" can retry.
func (a *MaterialsWorkspaceAdapter) finishEditor(ctx context.Context, catalog domain.MaterialsCatalog, unit string) InteractionResponse {
	state := a.editor
	material, err := domain.NewMaterial(catalog, state.family, state.productType, unit, state.values)
	if err != nil {
		a.editor = nil
		return InteractionResponse{Messages: []InteractionMessage{
			ErrorMessage{Text: "No pude crear el material con esos datos."},
		}}
	}

	if err := a.creator.Create(ctx, material); err != nil {
		a.editor = nil
		if errors.Is(err, domain.ErrDuplicateMaterial) {
			existing, getErr := a.materialsGetter.Get(ctx, material.FamilyCode, material.IdentityKey)
			if getErr != nil {
				return InteractionResponse{Messages: []InteractionMessage{
					ErrorMessage{Text: "Ya existe un material con esa identidad, pero no pude abrir su detalle."},
				}}
			}
			title, fields := a.materialPresentation(existing)
			return InteractionResponse{Messages: []InteractionMessage{
				TextMessage{Text: "Ya existe un material con esa identidad. Este es el material existente:"},
				StructuredResult{Title: title, Fields: fields},
			}}
		}
		return InteractionResponse{Messages: []InteractionMessage{
			ErrorMessage{Text: "No pude crear el material. Probá de nuevo en un momento."},
		}}
	}

	a.editor = nil
	title, fields := a.materialPresentation(material)
	return InteractionResponse{Messages: []InteractionMessage{StructuredResult{Title: title, Fields: fields}}}
}
