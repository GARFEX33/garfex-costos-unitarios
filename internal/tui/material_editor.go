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
	// editorStepAttributePicker, editorStepAttributeEdit, and
	// editorStepConfirm are edit-only steps (see startEditEditor): the picker
	// shows a single-select menu of the material's currently-set fields (plus
	// a "Terminar edición" sentinel), attributeEdit asks about the ONE picked
	// field, and confirm shows a summary of every field actually edited this
	// session before anything is persisted. Picking a field from the picker
	// loops back to the SAME picker (now showing that field's updated value)
	// so the user can keep editing more fields, or re-edit one already
	// touched, until they pick "Terminar edición". CREATE never reaches any
	// of these — it still walks editorStepFamily -> editorStepProductType ->
	// editorStepAttribute -> editorStepUnit exactly as before.
	editorStepAttributePicker
	editorStepAttributeEdit
	editorStepConfirm
)

// editNaturalUnitFieldCode is the attributePickerQuestion Option.Value used
// for the "Unidad natural" menu entry — a TUI-layer-only routing sentinel,
// never a real catalog attribute code and never passed to the domain layer.
const editNaturalUnitFieldCode = "__natural_unit__"

// editFinishFieldCode is the attributePickerQuestion Option.Value used for
// the "Terminar edición" menu entry — a TUI-layer-only routing sentinel that
// ends the edit-field loop, either cancelling cleanly (nothing was changed)
// or moving on to the confirmation summary (one or more fields were
// changed). Never a real catalog attribute code and never passed to the
// domain layer.
const editFinishFieldCode = "__finish_edit__"

// materialEditorMode distinguishes the "nuevo material" create flow from the
// "Editar" flow — both drive the exact same state machine below, the mode
// only decides (a) which step the flow starts at (create asks Family/
// ProductType first; edit, per approved decision D3, jumps straight to
// editorStepAttribute since ProductType is fixed during edit) and (b) which
// operation finishEditor calls to persist the result.
type materialEditorMode int

const (
	editorModeCreate materialEditorMode = iota
	editorModeEdit
)

// materialEditorState is the in-progress "nuevo material"/"Editar" flow's
// cross-turn state (see MaterialsWorkspaceAdapter.editor). It is entirely
// catalog-driven: nothing here branches on a specific Family/ProductType.
type materialEditorState struct {
	mode        materialEditorMode
	step        materialEditorStep
	family      string
	productType string
	// attributes are resolved once family+productType are both known, in
	// catalog declaration order.
	attributes []domain.FamilyAttribute
	// nextIndex is the index into attributes for the next question to ask.
	// Used only by the CREATE/legacy-sequential walkthrough (advanceEditor);
	// the loop-edit flow uses editingCode instead (see below).
	nextIndex int
	// editingCode is the FamilyAttribute.Definition.Code currently being
	// asked about during editorStepAttributeEdit — the loop-edit flow's
	// counterpart to nextIndex, set by answerAttributePicker and read by
	// answerSingleAttributeEdit.
	editingCode string
	// editedCodes accumulates the attribute codes (and
	// editNaturalUnitFieldCode, if the unit was changed) the user actually
	// edited this session, in the order first touched — used only to build
	// the final confirmation summary. Re-editing the same field later does
	// not add a second entry; the summary always looks up the CURRENT value
	// from state.values/currentUnit at confirm time regardless, so this is
	// purely about "what to list", not "what value to show".
	editedCodes []string
	values      []domain.MaterialAttributeValue
	// originalID/originalValues/originalUnit are populated only in
	// editorModeEdit: originalID is the stable Material.ID Update() targets
	// (independent of IdentityKey, which editing may change);
	// originalValues/originalUnit are read-only references used solely to
	// default each question's cursor to the material's current value — they
	// are never written back directly, `values` is still built the exact
	// same way editorModeCreate already builds it (appended as each
	// question is answered), so there is no separate "replace if already
	// present" logic to get wrong.
	originalID     int64
	originalValues []domain.MaterialAttributeValue
	originalUnit   string
	// currentUnit is editorModeEdit's mutable counterpart to originalUnit: it
	// starts as a copy of the material's NaturalUnit and gets overwritten if
	// the user actually edits it during this session. finishEditor must be
	// called with currentUnit (possibly changed), not necessarily the unit
	// from the most-recently-answered question, since NaturalUnit might not
	// be the last field the user selected.
	currentUnit string
}

// startCreateEditor begins the "nuevo material" flow with the first
// question: which Family to create.
func (a *MaterialsWorkspaceAdapter) startCreateEditor() (InteractionResponse, error) {
	a.editor = &materialEditorState{mode: editorModeCreate, step: editorStepFamily}
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

// startEditEditor begins the "Editar" flow for the material currently shown
// in the detail view (a.lastDetail). Per the project owner's redesigned
// flow, Family and ProductType are fixed during edit and there is no
// sequential walkthrough of every attribute: the wizard opens directly on a
// single-select menu of the material's currently-set fields
// (attributePickerQuestion) so the user can pick one to change, then loops
// back to the same menu to pick another (or re-pick one already changed)
// until they choose "Terminar edición". state.values is seeded as a copy of
// the material's FULL existing Attributes (unlike CREATE, which starts
// values nil/empty) — this is what lets a partial (one-or-more-field)
// replace still produce a complete, valid Material without re-asking about
// every field.
func (a *MaterialsWorkspaceAdapter) startEditEditor() (InteractionResponse, error) {
	material := a.lastDetail
	catalog := domain.NewMaterialsCatalog()
	a.editor = &materialEditorState{
		mode:           editorModeEdit,
		step:           editorStepAttributePicker,
		family:         material.FamilyCode,
		productType:    material.ProductTypeCode,
		attributes:     catalog.AttributesFor(material.FamilyCode, material.ProductTypeCode),
		values:         append([]domain.MaterialAttributeValue(nil), material.Attributes...),
		originalID:     material.ID,
		originalValues: material.Attributes,
		originalUnit:   material.NaturalUnit,
		currentUnit:    material.NaturalUnit,
	}
	return a.attributePickerQuestion(), nil
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
	case editorStepAttributePicker:
		return a.answerAttributePicker(catalog, input.Value), true
	case editorStepAttributeEdit:
		return a.answerSingleAttributeEdit(catalog, input.Value), true
	case editorStepUnit:
		if state.mode == editorModeEdit {
			state.currentUnit = input.Value
			state.editedCodes = appendUnique(state.editedCodes, editNaturalUnitFieldCode)
			return a.attributePickerQuestion(), true
		}
		return a.finishEditor(ctx, catalog, input.Value), true
	case editorStepConfirm:
		return a.answerEditConfirmation(ctx, catalog, input.Value), true
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

// attributePickerQuestion builds the edit-only "¿Qué campo querés editar?"
// menu: one Option per currently-set attribute (labeled with its current
// value via formatAttributeValue, the same renderer the detail view already
// uses), plus one explicit "Unidad natural" entry — NaturalUnit is a real,
// editable Material property (see domain.NewMaterial) even though it is not
// a FamilyAttribute, so it needs its own labeled entry rather than being
// silently omitted. Attributes not currently set (e.g. skipped by a prior
// DESNUDO-style rule) or explicitly NOT_APPLICABLE are not offered — there
// is nothing meaningful to "change" about a field that isn't there.
//
// It is a plain single-select (SelectionSingle): the user picks exactly one
// field to change, answers just that one (see answerAttributePicker), and
// lands back on this same menu — now showing that field's updated value,
// since the menu always builds its labels from the live state.values — to
// either pick another field or pick "Terminar edición".
func (a *MaterialsWorkspaceAdapter) attributePickerQuestion() InteractionResponse {
	state := a.editor
	state.step = editorStepAttributePicker
	byCode := map[string]domain.MaterialAttributeValue{}
	for _, value := range state.values {
		byCode[value.AttributeCode] = value
	}
	var options []Option
	for _, attribute := range state.attributes {
		value, present := byCode[attribute.Definition.Code]
		if !present || value.Text == notApplicableAttributeText {
			continue
		}
		label := attribute.Definition.Name + ": " + formatAttributeValue(value)
		options = append(options, Option{ID: attribute.Definition.Code, Label: label, Value: attribute.Definition.Code})
	}
	options = append(options, Option{ID: editNaturalUnitFieldCode, Label: "Unidad natural: " + state.currentUnit, Value: editNaturalUnitFieldCode})
	options = append(options, Option{ID: editFinishFieldCode, Label: "Terminar edición", Value: editFinishFieldCode})
	prompt := "¿Qué campo querés editar?"
	return InteractionResponse{Pending: QuestionRequest{
		ID:            materialEditorKey,
		Key:           materialEditorKey,
		Prompt:        prompt,
		Question:      prompt,
		SelectionMode: SelectionSingle,
		Options:       options,
	}}
}

// answerAttributePicker handles the (single-select) answer to
// attributePickerQuestion, routing three ways: "Terminar edición" ends the
// loop (a plain cancel message if nothing was changed, or the confirmation
// summary otherwise); the NaturalUnit sentinel routes to unitQuestion (the
// exact same question CREATE's last step asks, reused unchanged); any other
// code must match one of state.attributes and routes to that attribute's
// question via the untouched attributeQuestion — answering it (see
// answerSingleAttributeEdit) loops back to this same picker rather than
// advancing through a queue.
func (a *MaterialsWorkspaceAdapter) answerAttributePicker(catalog domain.MaterialsCatalog, code string) InteractionResponse {
	state := a.editor
	if code == editFinishFieldCode {
		if len(state.editedCodes) == 0 {
			a.editor = nil
			return InteractionResponse{Messages: []InteractionMessage{
				TextMessage{Text: "No hiciste ningún cambio."},
			}}
		}
		return a.editConfirmationQuestion()
	}
	if code == editNaturalUnitFieldCode {
		state.step = editorStepUnit
		return a.unitQuestion(catalog)
	}
	for _, attribute := range state.attributes {
		if attribute.Definition.Code != code {
			continue
		}
		state.editingCode = code
		state.step = editorStepAttributeEdit
		response, err := a.attributeQuestion(catalog, attribute)
		if err != nil {
			a.editor = nil
			return InteractionResponse{Messages: []InteractionMessage{
				ErrorMessage{Text: "No pude continuar con la edición."},
			}}
		}
		return response
	}
	// Defensive: an unrecognized code — return to the picker rather than
	// getting stuck (should not happen, the picker only ever offers real
	// codes and the two sentinels).
	return a.attributePickerQuestion()
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
		options = defaultOptionFirst(options, a.originalOptionCode(attribute.Definition.Code))
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

// answerAttribute handles the answer for state.attributes[state.nextIndex]
// during CREATE's/EDIT's legacy sequential walkthrough (advanceEditor).
func (a *MaterialsWorkspaceAdapter) answerAttribute(catalog domain.MaterialsCatalog, value string) InteractionResponse {
	state := a.editor
	attribute := state.attributes[state.nextIndex]
	newValue, ok := buildAttributeValue(attribute, value)
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
	state.values = append(state.values, newValue)
	state.nextIndex++
	return a.advanceEditor(catalog)
}

// answerSingleAttributeEdit handles the answer to the ONE attribute question
// answerAttributePicker routed to (state.editingCode) — the loop-edit flow's
// counterpart to answerAttribute. It reuses buildAttributeValue for the
// exact same CONTROLLED_OPTION/QUANTITY parsing answerAttribute uses (so the
// two can never diverge), replaces (or appends, defensively) the matching
// value in state.values, records the field as edited, then loops back to
// the picker so the user can pick another field (or re-pick this one) —
// nothing is persisted here.
func (a *MaterialsWorkspaceAdapter) answerSingleAttributeEdit(catalog domain.MaterialsCatalog, value string) InteractionResponse {
	state := a.editor
	var attribute domain.FamilyAttribute
	for _, candidate := range state.attributes {
		if candidate.Definition.Code == state.editingCode {
			attribute = candidate
			break
		}
	}
	newValue, ok := buildAttributeValue(attribute, value)
	if !ok {
		response, err := a.attributeQuestion(catalog, attribute)
		if err != nil {
			a.editor = nil
			return InteractionResponse{Messages: []InteractionMessage{
				ErrorMessage{Text: "No pude continuar con la edición."},
			}}
		}
		response.Messages = []InteractionMessage{
			ErrorMessage{Text: fmt.Sprintf("No entendí %q. Escribí un valor y una unidad, por ejemplo \"600 V\".", value)},
		}
		return response
	}
	state.values = replaceOrAppendValue(state.values, newValue)
	state.editedCodes = appendUnique(state.editedCodes, state.editingCode)
	return a.attributePickerQuestion()
}

// appendUnique returns codes with code appended, unless code is already
// present — used by answerAttributePicker/answerSingleAttributeEdit to keep
// materialEditorState.editedCodes free of duplicates when the user
// re-edits the same field more than once in a session.
func appendUnique(codes []string, code string) []string {
	for _, existing := range codes {
		if existing == code {
			return codes
		}
	}
	return append(codes, code)
}

// buildAttributeValue parses raw into a MaterialAttributeValue for
// attribute's ValueType — the single CONTROLLED_OPTION/QUANTITY parsing
// logic shared by answerAttribute and answerSingleAttributeEdit, so the two
// flows can never quietly diverge. ok is false when raw fails to parse as a
// QUANTITY (no unit) or when attribute's ValueType is anything other than
// the two attributeQuestion ever asks about — attributeQuestion already
// refuses to build a question for any other ValueType, so that branch is
// unreachable via a real catalog, not a distinct user-facing case.
func buildAttributeValue(attribute domain.FamilyAttribute, raw string) (domain.MaterialAttributeValue, bool) {
	switch attribute.Definition.ValueType {
	case domain.ValueTypeControlledOption:
		return domain.OptionValue(attribute.Definition.Code, raw), true
	case domain.ValueTypeQuantity:
		numeric, unit, ok := splitQuantityInput(raw)
		if !ok {
			return domain.MaterialAttributeValue{}, false
		}
		return domain.QuantityValue(attribute.Definition.Code, numeric, unit), true
	default:
		return domain.MaterialAttributeValue{}, false
	}
}

// replaceOrAppendValue returns values with newValue substituted in place of
// any existing entry sharing its AttributeCode, or appended if none matched
// (the append branch is a defensive fallback — startEditEditor seeds
// state.values from the material's full Attributes, so the attribute being
// edited is always already present in practice).
func replaceOrAppendValue(values []domain.MaterialAttributeValue, newValue domain.MaterialAttributeValue) []domain.MaterialAttributeValue {
	for i, value := range values {
		if value.AttributeCode == newValue.AttributeCode {
			values[i] = newValue
			return values
		}
	}
	return append(values, newValue)
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
	units = defaultUnitFirst(units, a.editor.originalUnit)
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

// defaultOptionFirst reorders options so the one matching defaultCode (if
// any) is first — used only to pre-select an editing material's current
// value as the wizard's starting cursor position (see materialEditorState's
// doc comment). A no-op when defaultCode is empty or not found (e.g.
// editorModeCreate, where callers pass "").
func defaultOptionFirst(options []domain.AttributeOption, defaultCode string) []domain.AttributeOption {
	if defaultCode == "" {
		return options
	}
	for i, option := range options {
		if option.Code == defaultCode {
			if i == 0 {
				return options
			}
			reordered := make([]domain.AttributeOption, 0, len(options))
			reordered = append(reordered, option)
			reordered = append(reordered, options[:i]...)
			reordered = append(reordered, options[i+1:]...)
			return reordered
		}
	}
	return options
}

// defaultUnitFirst is defaultOptionFirst's counterpart for the NaturalUnit
// question's []domain.UnitDefinition options.
func defaultUnitFirst(units []domain.UnitDefinition, defaultCode string) []domain.UnitDefinition {
	if defaultCode == "" {
		return units
	}
	for i, unit := range units {
		if unit.Code == defaultCode {
			if i == 0 {
				return units
			}
			reordered := make([]domain.UnitDefinition, 0, len(units))
			reordered = append(reordered, unit)
			reordered = append(reordered, units[:i]...)
			reordered = append(reordered, units[i+1:]...)
			return reordered
		}
	}
	return units
}

// originalOptionCode looks up material's current option code for
// attributeCode from a.editor.originalValues — "" (no default) if this is
// a create flow (originalValues is nil) or the attribute wasn't set.
func (a *MaterialsWorkspaceAdapter) originalOptionCode(attributeCode string) string {
	for _, value := range a.editor.originalValues {
		if value.AttributeCode == attributeCode {
			return value.OptionCode
		}
	}
	return ""
}

// editConfirmationQuestion builds the loop-edit flow's final step: a summary
// of every field the user actually edited this session, showing its CURRENT
// value (reusing formatAttributeValue, the same renderer the picker menu and
// detail view already use) and a ConfirmationRequest — nothing has been
// persisted yet at this point.
func (a *MaterialsWorkspaceAdapter) editConfirmationQuestion() InteractionResponse {
	state := a.editor
	state.step = editorStepConfirm
	byCode := map[string]domain.MaterialAttributeValue{}
	for _, value := range state.values {
		byCode[value.AttributeCode] = value
	}
	lines := []string{"Vas a guardar estos cambios:"}
	for _, code := range state.editedCodes {
		if code == editNaturalUnitFieldCode {
			lines = append(lines, "Unidad natural: "+state.currentUnit)
			continue
		}
		for _, attribute := range state.attributes {
			if attribute.Definition.Code != code {
				continue
			}
			if value, ok := byCode[code]; ok {
				lines = append(lines, attribute.Definition.Name+": "+formatAttributeValue(value))
			}
			break
		}
	}
	question := strings.Join(lines, "\n")
	return InteractionResponse{Pending: ConfirmationRequest{
		ID:           materialEditorKey,
		Key:          materialEditorKey,
		Question:     question,
		ConfirmLabel: "Sí, guardar",
		CancelLabel:  "No, cancelar",
	}}
}

// answerEditConfirmation handles the answer to editConfirmationQuestion:
// "yes" calls finishEditor with the CURRENT unit (possibly changed during
// this session, see materialEditorState.currentUnit) — the exact same
// finishEditor/Update/duplicate-collision path Edit already had. Any other
// answer ("no") cancels the whole edit — resets a.editor, nothing saved,
// the same "cancelado" spirit as InputCancel.
func (a *MaterialsWorkspaceAdapter) answerEditConfirmation(ctx context.Context, catalog domain.MaterialsCatalog, value string) InteractionResponse {
	state := a.editor
	if value != "yes" {
		a.editor = nil
		return InteractionResponse{Messages: []InteractionMessage{
			TextMessage{Text: "Se canceló la edición."},
		}}
	}
	return a.finishEditor(ctx, catalog, state.currentUnit)
}

// filterApplicableValues drops any value whose attribute no longer resolves
// to applicable (per FamilyAttribute.Effective given the FULL final values)
// before a candidate is built. This is a real no-op for CREATE/the legacy
// sequential walkthrough: advanceEditor already skips ModeForbidden/
// notApplicable attributes, so state.values never accumulates one of those
// in the first place there. It is NOT a no-op for the loop-edit flow:
// state.values starts as a copy of the material's full pre-existing
// Attributes, so replacing just ONE (e.g. insulation THW -> DESNUDO) can
// leave an now-forbidden OTHER value (color/voltage) still sitting in the
// slice — passing that straight into domain.NewMaterial would reject the
// whole edit ("attribute %q is forbidden"). Applied unconditionally so
// there is exactly one final-validation code path for both flows.
func filterApplicableValues(attributes []domain.FamilyAttribute, values []domain.MaterialAttributeValue) []domain.MaterialAttributeValue {
	applicable := map[string]bool{}
	for _, attribute := range attributes {
		mode, _, notApplicable := attribute.Effective(values)
		if mode != domain.ModeForbidden && !notApplicable {
			applicable[attribute.Definition.Code] = true
		}
	}
	var filtered []domain.MaterialAttributeValue
	for _, value := range values {
		if applicable[value.AttributeCode] {
			filtered = append(filtered, value)
		}
	}
	return filtered
}

// finishEditor builds the candidate Material from the fully-answered editor
// state and persists it (Create for editorModeCreate, Update targeting the
// original Material.ID for editorModeEdit). It always resets a.editor to
// nil — success, validation failure, and every Create/Update error all end
// the flow; simplicity over cleverness, an edge case a fresh "nuevo
// material"/"Editar" can retry.
func (a *MaterialsWorkspaceAdapter) finishEditor(ctx context.Context, catalog domain.MaterialsCatalog, unit string) InteractionResponse {
	state := a.editor
	values := filterApplicableValues(state.attributes, state.values)
	material, err := domain.NewMaterial(catalog, state.family, state.productType, unit, values)
	if err != nil {
		a.editor = nil
		errorText := "No pude crear el material con esos datos."
		if state.mode == editorModeEdit {
			errorText = "No pude guardar los cambios con esos datos."
		}
		return InteractionResponse{Messages: []InteractionMessage{ErrorMessage{Text: errorText}}}
	}

	var opErr error
	if state.mode == editorModeEdit {
		material.ID = state.originalID
		opErr = a.updater.Update(ctx, material)
	} else {
		opErr = a.creator.Create(ctx, material)
	}
	if opErr != nil {
		a.editor = nil
		if errors.Is(opErr, domain.ErrDuplicateMaterial) {
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
		errorText := "No pude crear el material. Probá de nuevo en un momento."
		if state.mode == editorModeEdit {
			errorText = "No pude guardar los cambios. Probá de nuevo en un momento."
		}
		return InteractionResponse{Messages: []InteractionMessage{ErrorMessage{Text: errorText}}}
	}

	a.editor = nil
	title, fields := a.materialPresentation(material)
	return InteractionResponse{Messages: []InteractionMessage{StructuredResult{Title: title, Fields: fields}}}
}
