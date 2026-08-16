package tui

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/GARFEX33/garfex-costos-unitarios/internal/domain"
)

// resourceEditorKey identifies every QuestionRequest the create/edit editor
// emits (class, family, type, each attribute, the natural unit). Every step
// shares the same Key so respondToEditor can claim any answer directed at
// the in-progress editor without tracking a separate key per step.
const resourceEditorKey = "resource-editor"

// searchableOptionThreshold is the option-count above which an attribute's
// question uses SelectionSearchable instead of SelectionSingle — e.g. gauge
// (12 options) is searchable, color (5 options) is not.
const searchableOptionThreshold = 6

type resourceEditorStep int

const (
	// editorStepClase is the create flow's first question — "¿Qué clase de
	// recurso querés crear?" — asked only when the workspace is unfiltered
	// (a.classFilter == ""). A filtered workspace (e.g. /materiales) already
	// knows its class and pre-seeds it, starting at editorStepFamily
	// instead — see startCreateEditor.
	editorStepClase resourceEditorStep = iota
	editorStepFamily
	editorStepType
	editorStepAttribute
	editorStepUnit
	// editorStepAttributePicker, editorStepAttributeEdit, and
	// editorStepConfirm are the "Editar"/"Duplicar" steps (see
	// startEditEditor/startDuplicateEditor): the picker shows a single-select
	// menu of the resource's currently-set fields (plus a "Terminar edición"
	// sentinel), attributeEdit asks about the ONE picked field, and confirm
	// shows a summary of every field actually edited this session before
	// anything is persisted. Picking a field from the picker loops back to
	// the SAME picker (now showing that field's updated value) so the user
	// can keep editing more fields, or re-edit one already touched, until
	// they pick "Terminar edición". CREATE never reaches any of these — it
	// still walks editorStepClase (unfiltered only) -> editorStepFamily ->
	// editorStepType -> editorStepAttribute -> editorStepUnit exactly as
	// before. EDIT/DUPLICATE never ask editorStepClase either: Clase (like
	// Familia/Tipo) is already fixed by the resource being edited/duplicated
	// — the same precedent this project already established for Tipo.
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

// resourceEditorMode distinguishes the "nuevo recurso" create flow from the
// "Editar" and "Duplicar" flows — all three drive the exact same state
// machine below, the mode only decides (a) which step the flow starts at
// (create asks Clase/Familia/Tipo first, per D1's classFilter pre-seed rule;
// edit and duplicate jump straight to editorStepAttributePicker since
// Clase/Familia/Tipo are fixed for both) and (b) which operation
// finishEditor calls to persist the result: Update (targeting the source
// resource's original ID) for editorModeEdit, Create (never carrying the
// source's ID) for both editorModeCreate and editorModeDuplicate.
// editorModeDuplicate also differs from editorModeEdit in one respect:
// "Terminar edición" with zero changes must NOT silently cancel (see
// answerAttributePicker) — an unmodified duplicate is still a meaningful
// attempt at an exact copy, and Create's existing duplicate-collision
// handling is what correctly detects it.
type resourceEditorMode int

const (
	editorModeCreate resourceEditorMode = iota
	editorModeEdit
	editorModeDuplicate
)

// resourceEditorState is the in-progress "nuevo recurso"/"Editar" flow's
// cross-turn state (see ResourcesWorkspaceAdapter.editor). It is entirely
// catalog-driven: nothing here branches on a specific Clase/Familia/Tipo.
type resourceEditorState struct {
	mode resourceEditorMode
	step resourceEditorStep
	// class is the resource's ResourceScope.ClassCode — set from
	// a.classFilter (filtered workspaces) or answerClass (unfiltered
	// workspaces), or copied from the source resource for Edit/Duplicate.
	class    string
	family   string
	itemType string
	// attributes are resolved once class+family+itemType are all known, in
	// catalog declaration order.
	attributes []domain.ResourceAttribute
	// nextIndex is the index into attributes for the next question to ask.
	// Used only by the CREATE/legacy-sequential walkthrough (advanceEditor);
	// the loop-edit flow uses editingCode instead (see below).
	nextIndex int
	// editingCode is the ResourceAttribute.Definition.Code currently being
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
	values      []domain.ResourceAttributeValue
	// originalID/originalValues/originalUnit are populated only in
	// editorModeEdit: originalID is the stable Resource.ID Update() targets
	// (independent of IdentityKey, which editing may change);
	// originalValues/originalUnit are read-only references used solely to
	// default each question's cursor to the resource's current value — they
	// are never written back directly, `values` is still built the exact
	// same way editorModeCreate already builds it (appended as each
	// question is answered), so there is no separate "replace if already
	// present" logic to get wrong.
	originalID     int64
	originalValues []domain.ResourceAttributeValue
	originalUnit   string
	// currentUnit is editorModeEdit's mutable counterpart to originalUnit: it
	// starts as a copy of the resource's NaturalUnit and gets overwritten if
	// the user actually edits it during this session. finishEditor must be
	// called with currentUnit (possibly changed), not necessarily the unit
	// from the most-recently-answered question, since NaturalUnit might not
	// be the last field the user selected.
	currentUnit string
}

// resourceSearcher is the minimal surface a resources workspace needs from
// the application service to search the real catalog.
type resourceSearcher interface {
	Search(ctx context.Context, criteria domain.SearchCriteria) ([]domain.Resource, error)
}

// resourceGetter is the minimal surface a resources workspace needs to
// resolve one resource by its owning class code and identity key (design
// R1: Get's first argument is a CLASS code, never a family code).
type resourceGetter interface {
	Get(ctx context.Context, classCode, identityKey string) (domain.Resource, error)
}

// resourceDescriber is the minimal surface a resources workspace needs to
// resolve a resource's canonical presentation — deliberately kept separate
// from resourceSearcher/resourceGetter so the adapter never needs to know
// about domain.ResourceCatalog.Describe's implementation directly.
type resourceDescriber interface {
	Describe(resource domain.Resource) string
}

// resourceCreator is the minimal surface the generic resource editor needs
// to persist a newly-built candidate Resource.
type resourceCreator interface {
	Create(ctx context.Context, resource domain.Resource) error
}

// resourceUpdater is the minimal surface the generic resource editor needs
// to persist changes to an existing Resource (identified by its stable ID,
// independent of IdentityKey, which editing can change — see Resource.ID).
type resourceUpdater interface {
	Update(ctx context.Context, resource domain.Resource) error
}

// resourceDeleter is the minimal surface the "Eliminar" flow needs to soft-
// delete a resource by its stable ID (never a hard delete).
type resourceDeleter interface {
	Delete(ctx context.Context, id int64) error
}

// ResourcesWorkspaceAdapter is the production InteractionAgent shape for a
// Recursos Maestros workspace — either the unfiltered "/recursos" workspace
// (classFilter == "") or a class-scoped workspace like "/materiales"
// (classFilter == "MATERIAL"). catalog is injected rather than resolved via
// a package-level constructor call, which is what lets a synthetic catalog
// (e.g. a 4th class added purely as data) drive this exact adapter/editor
// code in a test without any code change (recursos-maestro design §5).
//
// This PR (recursos-maestro PR6) implements only the create/edit/duplicate
// editor state machine below (respondToEditor and everything it calls). The
// full workspace behavior (Greeting/Respond dispatch, text search, detail
// view, delete confirmation — currently still on the not-yet-renamed
// MaterialsWorkspaceAdapter in materials_workspace_adapter.go) is added to
// this same struct by a later PR once that file is retired; resources/
// deleter/lastQuery already have their final field shape reserved here so
// that PR only adds methods, it does not change this struct.
type ResourcesWorkspaceAdapter struct {
	resources       resourceSearcher
	resourcesGetter resourceGetter
	describer       resourceDescriber
	creator         resourceCreator
	updater         resourceUpdater
	deleter         resourceDeleter
	// catalog is the injected domain.ResourceCatalog this adapter/editor
	// reads every Clase/Familia/Tipo/Atributo/Unidad option from — see the
	// struct doc comment above.
	catalog domain.ResourceCatalog
	// classFilter narrows this workspace to one ResourceClass.Code (e.g.
	// "MATERIAL" for "/materiales"); "" means the unfiltered "/recursos"
	// workspace, which asks editorStepClase in the create flow.
	classFilter string
	// lastQuery remembers the text of the most recent successful search so
	// the "volver a los resultados" action can reproduce the identical
	// result list deterministically, without a separate cache of results.
	lastQuery string
	// lastDetail remembers the resource currently shown in the detail view
	// so a later "Editar"/"Duplicar" selection knows which one to act on.
	lastDetail domain.Resource
	// editor holds the in-progress "nuevo recurso" create/edit flow, if any;
	// nil means no create/edit flow is in progress. See this file.
	editor *resourceEditorState
}

// NewResourcesWorkspaceAdapter returns a ResourcesWorkspaceAdapter for one
// workspace slot. searcher, getter, describer, creator, updater and deleter
// are satisfied structurally by *recursos.Service — composed for real in
// cmd/garfex/main.go, once per active ResourceClass plus once for the
// unfiltered "/recursos" workspace (classFilter == "" there).
func NewResourcesWorkspaceAdapter(searcher resourceSearcher, getter resourceGetter, describer resourceDescriber,
	creator resourceCreator, updater resourceUpdater, deleter resourceDeleter,
	catalog domain.ResourceCatalog, classFilter string) *ResourcesWorkspaceAdapter {
	return &ResourcesWorkspaceAdapter{
		resources: searcher, resourcesGetter: getter, describer: describer,
		creator: creator, updater: updater, deleter: deleter,
		catalog: catalog, classFilter: classFilter,
	}
}

// startCreateEditor begins the "nuevo recurso" flow. A filtered workspace
// (a.classFilter != "") already knows its class, so it pre-seeds it and
// starts one step later, at editorStepFamily — exactly the same
// pre-seed-and-skip trick startEditEditor/startDuplicateEditor already use
// to skip Clase/Familia/Tipo entirely for an existing resource. An
// unfiltered workspace starts at editorStepClase instead.
func (a *ResourcesWorkspaceAdapter) startCreateEditor() (InteractionResponse, error) {
	// task 7.3: a filtered workspace already knows its class and never asks
	// classQuestion (which is what filters inactive classes for the
	// unfiltered path below) — so it must check activeness itself. This
	// mostly guards against a workspace that stayed alive after its class
	// was deactivated mid-session (buildWorkspaceDescriptors only excludes
	// it at boot-time build, not on every Crear).
	if a.classFilter != "" && !a.classIsActive(a.classFilter) {
		return InteractionResponse{Messages: []InteractionMessage{
			ErrorMessage{Text: "No se puede crear un recurso: la Clase está inactiva."},
		}}, nil
	}
	a.editor = &resourceEditorState{mode: editorModeCreate}
	if a.classFilter != "" {
		a.editor.class = a.classFilter
		return a.familyQuestion(), nil
	}
	a.editor.step = editorStepClase
	return a.classQuestion(), nil
}

// classIsActive reports whether code is a KNOWN, currently-active
// ResourceClass in a.catalog (task 7.3). An unrecognized code returns true
// (fail open) — this check exists solely to block Crear/Editar/Duplicar for
// a Clase the catalog explicitly marks inactive, not to police malformed
// data that a.catalog.Validate() would already have rejected at boot.
func (a *ResourcesWorkspaceAdapter) classIsActive(code string) bool {
	for _, class := range a.catalog.Classes {
		if class.Code == code {
			return class.Active
		}
	}
	return true
}

// startEditEditor begins the "Editar" flow for the resource currently shown
// in the detail view (a.lastDetail). Per the project owner's redesigned
// flow, Clase/Familia/Tipo are fixed during edit and there is no sequential
// walkthrough of every attribute: the wizard opens directly on a
// single-select menu of the resource's currently-set fields
// (attributePickerQuestion) so the user can pick one to change, then loops
// back to the same menu to pick another (or re-pick one already changed)
// until they choose "Terminar edición". state.values is seeded as a copy of
// the resource's FULL existing Attributes (unlike CREATE, which starts
// values nil/empty) — this is what lets a partial (one-or-more-field)
// replace still produce a complete, valid Resource without re-asking about
// every field.
func (a *ResourcesWorkspaceAdapter) startEditEditor() (InteractionResponse, error) {
	resource := a.lastDetail
	// task 7.3: an existing resource under an inactive Clase stays readable/
	// searchable (spec), but editing it is blocked — same rule as Crear.
	if !a.classIsActive(resource.ClassCode) {
		return InteractionResponse{
			Messages: []InteractionMessage{ErrorMessage{Text: "No se puede editar este recurso: su Clase está inactiva."}},
			Pending:  detailActionsRequest(),
		}, nil
	}
	scope := domain.ResourceScope{ClassCode: resource.ClassCode, FamilyCode: resource.FamilyCode, TypeCode: resource.TypeCode}
	a.editor = &resourceEditorState{
		mode:           editorModeEdit,
		step:           editorStepAttributePicker,
		class:          resource.ClassCode,
		family:         resource.FamilyCode,
		itemType:       resource.TypeCode,
		attributes:     a.catalog.AttributesFor(scope),
		values:         append([]domain.ResourceAttributeValue(nil), resource.Attributes...),
		originalID:     resource.ID,
		originalValues: resource.Attributes,
		originalUnit:   resource.NaturalUnit,
		currentUnit:    resource.NaturalUnit,
	}
	return a.attributePickerQuestion(), nil
}

// startDuplicateEditor begins the "Duplicar" flow for the resource currently
// shown in the detail view (a.lastDetail): the same field-picker loop
// Editar uses, pre-loaded with the source resource's current values, but
// building a brand-new Resource (Create, never Update) rather than
// modifying the one it started from — originalID is deliberately left zero.
// Clase/Familia/Tipo stay fixed to the source's, same restriction as Editar
// — reclassifying via Duplicar is out of scope here.
func (a *ResourcesWorkspaceAdapter) startDuplicateEditor() (InteractionResponse, error) {
	resource := a.lastDetail
	// task 7.3: Duplicar functionally creates a new resource, so it is
	// blocked by the same inactive-Clase rule as Crear/Editar.
	if !a.classIsActive(resource.ClassCode) {
		return InteractionResponse{
			Messages: []InteractionMessage{ErrorMessage{Text: "No se puede duplicar este recurso: su Clase está inactiva."}},
			Pending:  detailActionsRequest(),
		}, nil
	}
	scope := domain.ResourceScope{ClassCode: resource.ClassCode, FamilyCode: resource.FamilyCode, TypeCode: resource.TypeCode}
	a.editor = &resourceEditorState{
		mode:           editorModeDuplicate,
		step:           editorStepAttributePicker,
		class:          resource.ClassCode,
		family:         resource.FamilyCode,
		itemType:       resource.TypeCode,
		attributes:     a.catalog.AttributesFor(scope),
		values:         append([]domain.ResourceAttributeValue(nil), resource.Attributes...),
		originalValues: resource.Attributes,
		originalUnit:   resource.NaturalUnit,
		currentUnit:    resource.NaturalUnit,
	}
	return a.attributePickerQuestion(), nil
}

// respondToEditor handles one InteractionInput while a.editor is
// in-progress. The returned bool reports whether input actually belonged to
// the editor (its own Key, or a cancellation) — when false, the caller
// (Respond) must fall through to its normal handling instead of this
// method silently swallowing unrelated input.
func (a *ResourcesWorkspaceAdapter) respondToEditor(ctx context.Context, input InteractionInput) (InteractionResponse, bool) {
	if input.Kind == InputCancel {
		a.editor = nil
		return InteractionResponse{Messages: []InteractionMessage{
			TextMessage{Text: "Se canceló la operación."},
		}}, true
	}
	if input.Key != resourceEditorKey {
		return InteractionResponse{}, false
	}

	state := a.editor
	switch state.step {
	case editorStepClase:
		return a.answerClass(input.Value), true
	case editorStepFamily:
		state.family = input.Value
		state.step = editorStepType
		return a.typeQuestion(), true
	case editorStepType:
		state.itemType = input.Value
		state.attributes = a.catalog.AttributesFor(domain.ResourceScope{ClassCode: state.class, FamilyCode: state.family, TypeCode: state.itemType})
		state.nextIndex = 0
		state.step = editorStepAttribute
		return a.advanceEditor(), true
	case editorStepAttribute:
		return a.answerAttribute(input.Value), true
	case editorStepAttributePicker:
		return a.answerAttributePicker(input.Value), true
	case editorStepAttributeEdit:
		return a.answerSingleAttributeEdit(input.Value), true
	case editorStepUnit:
		if state.mode == editorModeEdit || state.mode == editorModeDuplicate {
			state.currentUnit = input.Value
			state.editedCodes = appendUnique(state.editedCodes, editNaturalUnitFieldCode)
			return a.attributePickerQuestion(), true
		}
		return a.finishEditor(ctx, input.Value), true
	case editorStepConfirm:
		return a.answerEditConfirmation(ctx, input.Value), true
	}
	return InteractionResponse{}, true
}

// classQuestion builds the create flow's first wizard step — asked only for
// an unfiltered workspace (see startCreateEditor) — offering every ACTIVE
// ResourceClass (design §1's ActiveClasses, sorted Order then Name); an
// inactive class never appears here even though its existing resources stay
// readable elsewhere.
func (a *ResourcesWorkspaceAdapter) classQuestion() InteractionResponse {
	classes := a.catalog.ActiveClasses()
	options := make([]Option, len(classes))
	for i, class := range classes {
		options[i] = Option{ID: class.Code, Label: class.Name, Value: class.Code}
	}
	prompt := "¿Qué clase de recurso querés crear?"
	return InteractionResponse{Pending: QuestionRequest{
		ID:            resourceEditorKey,
		Key:           resourceEditorKey,
		Prompt:        prompt,
		Question:      prompt,
		SelectionMode: SelectionSingle,
		Options:       options,
	}}
}

// answerClass handles editorStepClase's answer: it records the chosen class
// and continues into familyQuestion — the exact same body a pre-seeded
// filtered workspace's startCreateEditor also calls, so both entry paths
// share one implementation.
func (a *ResourcesWorkspaceAdapter) answerClass(value string) InteractionResponse {
	a.editor.class = value
	return a.familyQuestion()
}

// familyQuestion builds the "¿Qué familia querés crear?" step, shared by
// both create-flow entry paths: an unfiltered workspace reaches it via
// answerClass, a filtered workspace reaches it directly from
// startCreateEditor (class already pre-seeded in both cases by the time
// this runs).
func (a *ResourcesWorkspaceAdapter) familyQuestion() InteractionResponse {
	a.editor.step = editorStepFamily
	families := a.catalog.FamiliesFor(a.editor.class)
	options := make([]Option, len(families))
	for i, family := range families {
		options[i] = Option{ID: family.Code, Label: resourceFamilyPresentation(a.catalog, family), Value: family.Code}
	}
	prompt := "¿Qué familia querés crear?"
	return InteractionResponse{Pending: QuestionRequest{
		ID:            resourceEditorKey,
		Key:           resourceEditorKey,
		Prompt:        prompt,
		Question:      prompt,
		SelectionMode: SelectionSingle,
		Options:       options,
	}}
}

// typeQuestion builds the third wizard step: which Tipo of the
// already-chosen Clase/Familia to create.
func (a *ResourcesWorkspaceAdapter) typeQuestion() InteractionResponse {
	state := a.editor
	types := a.catalog.TypesFor(domain.ResourceScope{ClassCode: state.class, FamilyCode: state.family})
	options := make([]Option, len(types))
	for i, t := range types {
		options[i] = Option{ID: t.Code, Label: resourceTypePresentation(a.catalog, t), Value: t.Code}
	}
	prompt := "¿Qué tipo querés crear?"
	return InteractionResponse{Pending: QuestionRequest{
		ID:            resourceEditorKey,
		Key:           resourceEditorKey,
		Prompt:        prompt,
		Question:      prompt,
		SelectionMode: SelectionSingle,
		Options:       options,
	}}
}

// attributePickerQuestion builds the edit-only "¿Qué campo querés editar?"
// menu: one Option per currently-set attribute (labeled with its current
// value via formatResourceAttributeValue, the same renderer the detail view
// uses), plus one explicit "Unidad natural" entry — NaturalUnit is a real,
// editable Resource property (see domain.NewResource) even though it is not
// a ResourceAttribute, so it needs its own labeled entry rather than being
// silently omitted. Attributes not currently set (e.g. skipped by a prior
// DESNUDO-style rule) or explicitly NOT_APPLICABLE are not offered — there
// is nothing meaningful to "change" about a field that isn't there.
//
// It is a plain single-select (SelectionSingle): the user picks exactly one
// field to change, answers just that one (see answerAttributePicker), and
// lands back on this same menu — now showing that field's updated value,
// since the menu always builds its labels from the live state.values — to
// either pick another field or pick "Terminar edición".
func (a *ResourcesWorkspaceAdapter) attributePickerQuestion() InteractionResponse {
	state := a.editor
	state.step = editorStepAttributePicker
	byCode := map[string]domain.ResourceAttributeValue{}
	for _, value := range state.values {
		byCode[value.AttributeCode] = value
	}
	var options []Option
	for _, attribute := range state.attributes {
		value, present := byCode[attribute.Definition.Code]
		if !present || value.Text == notApplicableAttributeText {
			continue
		}
		label := attribute.Definition.Name + ": " + formatResourceAttributeValue(value)
		options = append(options, Option{ID: attribute.Definition.Code, Label: label, Value: attribute.Definition.Code})
	}
	options = append(options, Option{ID: editNaturalUnitFieldCode, Label: "Unidad natural: " + state.currentUnit, Value: editNaturalUnitFieldCode})
	options = append(options, Option{ID: editFinishFieldCode, Label: "Terminar edición", Value: editFinishFieldCode})
	prompt := "¿Qué campo querés editar?"
	if state.mode == editorModeDuplicate {
		prompt = "¿Qué campo querés modificar en la copia?"
	}
	return InteractionResponse{Pending: QuestionRequest{
		ID:            resourceEditorKey,
		Key:           resourceEditorKey,
		Prompt:        prompt,
		Question:      prompt,
		SelectionMode: SelectionSingle,
		Options:       options,
	}}
}

// answerAttributePicker handles the (single-select) answer to
// attributePickerQuestion, routing three ways: "Terminar edición" ends the
// loop — for editorModeEdit, a plain cancel message if nothing was changed
// (there's nothing to save), or the confirmation summary otherwise; for
// editorModeDuplicate, ALWAYS the confirmation summary, even with zero
// changes, since an unmodified duplicate is still a meaningful attempt at an
// exact copy (finishEditor's existing ErrDuplicateResource handling is what
// then correctly detects that exact-copy case); the NaturalUnit sentinel
// routes to unitQuestion (the exact same question CREATE's last step asks,
// reused unchanged); any other code must match one of state.attributes and
// routes to that attribute's question via the untouched attributeQuestion —
// answering it (see answerSingleAttributeEdit) loops back to this same
// picker rather than advancing through a queue.
func (a *ResourcesWorkspaceAdapter) answerAttributePicker(code string) InteractionResponse {
	state := a.editor
	if code == editFinishFieldCode {
		if len(state.editedCodes) == 0 && state.mode == editorModeEdit {
			a.editor = nil
			return InteractionResponse{Messages: []InteractionMessage{
				TextMessage{Text: "No hiciste ningún cambio."},
			}}
		}
		return a.editConfirmationQuestion()
	}
	if code == editNaturalUnitFieldCode {
		state.step = editorStepUnit
		return a.unitQuestion()
	}
	for _, attribute := range state.attributes {
		if attribute.Definition.Code != code {
			continue
		}
		state.editingCode = code
		state.step = editorStepAttributeEdit
		response, err := a.attributeQuestion(attribute)
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
// attribute whose ResourceAttribute.Effective resolves to ModeForbidden or
// notApplicable — this is exactly how e.g. insulation=DESNUDO skips
// color/voltage without any Familia/Tipo-specific code here. It stops at the
// first attribute that must be asked about, or moves on to the NaturalUnit
// question once every attribute has been resolved.
func (a *ResourcesWorkspaceAdapter) advanceEditor() InteractionResponse {
	state := a.editor
	for state.nextIndex < len(state.attributes) {
		attribute := state.attributes[state.nextIndex]
		mode, _, notApplicable := attribute.Effective(state.values)
		if mode == domain.ModeForbidden || notApplicable {
			state.nextIndex++
			continue
		}
		response, err := a.attributeQuestion(attribute)
		if err != nil {
			a.editor = nil
			return InteractionResponse{Messages: []InteractionMessage{
				ErrorMessage{Text: "No pude continuar con la creación del recurso."},
			}}
		}
		return response
	}
	state.step = editorStepUnit
	return a.unitQuestion()
}

// attributeQuestion builds the QuestionRequest for one ResourceAttribute.
// Only CONTROLLED_OPTION and QUANTITY are implemented (the only two
// AttributeValueTypes any real catalog attribute uses today) — any other
// value type is a deliberate, explicit error rather than a guessed-at UI.
func (a *ResourcesWorkspaceAdapter) attributeQuestion(attribute domain.ResourceAttribute) (InteractionResponse, error) {
	prompt := attribute.Definition.Name
	switch attribute.Definition.ValueType {
	case domain.ValueTypeControlledOption:
		options := a.catalog.ValidOptions(attribute, a.narrowingContext())
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
			ID:            resourceEditorKey,
			Key:           resourceEditorKey,
			Prompt:        prompt,
			Question:      prompt,
			SelectionMode: selectionMode,
			Options:       selectOptions,
		}}, nil
	case domain.ValueTypeQuantity:
		hintedPrompt := prompt + ` (ej. 600 V, 1 kV)`
		return InteractionResponse{Pending: QuestionRequest{
			ID:            resourceEditorKey,
			Key:           resourceEditorKey,
			Prompt:        hintedPrompt,
			Question:      hintedPrompt,
			SelectionMode: SelectionSingle,
			AllowCustom:   true,
		}}, nil
	default:
		return InteractionResponse{}, fmt.Errorf("resource editor: unsupported attribute value type %q for attribute %q", attribute.Definition.ValueType, attribute.Definition.Code)
	}
}

// answerAttribute handles the answer for state.attributes[state.nextIndex]
// during CREATE's/EDIT's legacy sequential walkthrough (advanceEditor).
func (a *ResourcesWorkspaceAdapter) answerAttribute(value string) InteractionResponse {
	state := a.editor
	attribute := state.attributes[state.nextIndex]
	newValue, ok := buildAttributeValue(attribute, value)
	if !ok {
		response, err := a.attributeQuestion(attribute)
		if err != nil {
			a.editor = nil
			return InteractionResponse{Messages: []InteractionMessage{
				ErrorMessage{Text: "No pude continuar con la creación del recurso."},
			}}
		}
		response.Messages = []InteractionMessage{
			ErrorMessage{Text: fmt.Sprintf("No entendí %q. Escribí un valor y una unidad, por ejemplo \"600 V\".", value)},
		}
		return response
	}
	state.values = append(state.values, newValue)
	state.nextIndex++
	return a.advanceEditor()
}

// answerSingleAttributeEdit handles the answer to the ONE attribute question
// answerAttributePicker routed to (state.editingCode) — the loop-edit flow's
// counterpart to answerAttribute. It reuses buildAttributeValue for the
// exact same CONTROLLED_OPTION/QUANTITY parsing answerAttribute uses (so the
// two can never diverge), replaces (or appends, defensively) the matching
// value in state.values, records the field as edited, then loops back to
// the picker so the user can pick another field (or re-pick this one) —
// nothing is persisted here.
func (a *ResourcesWorkspaceAdapter) answerSingleAttributeEdit(value string) InteractionResponse {
	state := a.editor
	var attribute domain.ResourceAttribute
	for _, candidate := range state.attributes {
		if candidate.Definition.Code == state.editingCode {
			attribute = candidate
			break
		}
	}
	newValue, ok := buildAttributeValue(attribute, value)
	if !ok {
		response, err := a.attributeQuestion(attribute)
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
// resourceEditorState.editedCodes free of duplicates when the user re-edits
// the same field more than once in a session.
func appendUnique(codes []string, code string) []string {
	for _, existing := range codes {
		if existing == code {
			return codes
		}
	}
	return append(codes, code)
}

// buildAttributeValue parses raw into a ResourceAttributeValue for
// attribute's ValueType — the single CONTROLLED_OPTION/QUANTITY parsing
// logic shared by answerAttribute and answerSingleAttributeEdit, so the two
// flows can never quietly diverge. ok is false when raw fails to parse as a
// QUANTITY (no unit) or when attribute's ValueType is anything other than
// the two attributeQuestion ever asks about — attributeQuestion already
// refuses to build a question for any other ValueType, so that branch is
// unreachable via a real catalog, not a distinct user-facing case.
func buildAttributeValue(attribute domain.ResourceAttribute, raw string) (domain.ResourceAttributeValue, bool) {
	switch attribute.Definition.ValueType {
	case domain.ValueTypeControlledOption:
		return domain.OptionValue(attribute.Definition.Code, raw), true
	case domain.ValueTypeQuantity:
		numeric, unit, ok := splitQuantityInput(raw)
		if !ok {
			return domain.ResourceAttributeValue{}, false
		}
		return domain.QuantityValue(attribute.Definition.Code, numeric, unit), true
	default:
		return domain.ResourceAttributeValue{}, false
	}
}

// replaceOrAppendValue returns values with newValue substituted in place of
// any existing entry sharing its AttributeCode, or appended if none matched
// (the append branch is a defensive fallback — startEditEditor seeds
// state.values from the resource's full Attributes, so the attribute being
// edited is always already present in practice).
func replaceOrAppendValue(values []domain.ResourceAttributeValue, newValue domain.ResourceAttributeValue) []domain.ResourceAttributeValue {
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
// resource under.
func (a *ResourcesWorkspaceAdapter) unitQuestion() InteractionResponse {
	state := a.editor
	units := a.catalog.NaturalUnitsFor(domain.ResourceScope{ClassCode: state.class, FamilyCode: state.family})
	units = defaultUnitFirst(units, state.originalUnit)
	options := make([]Option, len(units))
	for i, unit := range units {
		options[i] = Option{ID: unit.Code, Label: catalogUnitPresentation(unit.Name, unit.Symbol, unit.Dimension), Value: unit.Code}
	}
	prompt := "¿Cuál es la unidad natural?"
	return InteractionResponse{Pending: QuestionRequest{
		ID:            resourceEditorKey,
		Key:           resourceEditorKey,
		Prompt:        prompt,
		Question:      prompt,
		SelectionMode: SelectionSingle,
		Options:       options,
	}}
}

func resourceFamilyPresentation(catalog domain.ResourceCatalog, family domain.ResourceFamily) string {
	className := family.ClassCode
	for _, class := range catalog.Classes {
		if class.Code == family.ClassCode {
			className = class.Name
			break
		}
	}
	return catalogRecordPresentation(domain.CatalogKind{Code: domain.KindFamily}, domain.CatalogRecord{Values: map[string]domain.CatalogValue{
		"name": {Text: family.Name}, "class": {Ref: domain.CatalogRef{Label: className}},
	}})
}

func resourceTypePresentation(catalog domain.ResourceCatalog, itemType domain.ResourceType) string {
	className, familyName := itemType.ClassCode, itemType.FamilyCode
	for _, class := range catalog.Classes {
		if class.Code == itemType.ClassCode {
			className = class.Name
			break
		}
	}
	for _, family := range catalog.Families {
		if family.ClassCode == itemType.ClassCode && family.Code == itemType.FamilyCode {
			familyName = family.Name
			break
		}
	}
	return catalogRecordPresentation(domain.CatalogKind{Code: domain.KindType}, domain.CatalogRecord{Values: map[string]domain.CatalogValue{
		"name": {Text: itemType.Name}, "class": {Ref: domain.CatalogRef{Label: className}}, "family": {Ref: domain.CatalogRef{Label: familyName}},
	}})
}

// defaultOptionFirst reorders options so the one matching defaultCode (if
// any) is first — used only to pre-select an editing resource's current
// value as the wizard's starting cursor position (see resourceEditorState's
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

// narrowingContext returns the attribute values that should narrow the
// CURRENTLY-asked attribute's options via ValidOptions. For CREATE, that's
// simply the full accumulated state.values (attributes are asked once each,
// in order, so nothing about the attribute currently being asked is already
// in there — only genuinely-already-chosen siblings are, which is exactly
// what should narrow it). For the EDIT loop, state.values starts as a full
// copy of the resource's PRE-EXISTING attributes (so a partial edit can
// still produce a complete Resource) — passing that as-is would let an
// untouched, stale sibling value (e.g. an old diameter_mm) wrongly
// constrain a field the user is actively trying to change (e.g.
// diameter_inch) down to essentially its own current value. So during edit,
// only attributes genuinely re-chosen THIS session (state.editedCodes)
// narrow the one currently being asked about — the first field edited in a
// session always sees the full option list, and editing a related field
// earlier in the same loop correctly narrows a later one, matching what a
// user actually expects.
func (a *ResourcesWorkspaceAdapter) narrowingContext() []domain.ResourceAttributeValue {
	state := a.editor
	if state.mode != editorModeEdit {
		return state.values
	}
	edited := map[string]bool{}
	for _, code := range state.editedCodes {
		edited[code] = true
	}
	var context []domain.ResourceAttributeValue
	for _, value := range state.values {
		if edited[value.AttributeCode] {
			context = append(context, value)
		}
	}
	return context
}

// originalOptionCode looks up resource's current option code for
// attributeCode from a.editor.originalValues — "" (no default) if this is
// a create flow (originalValues is nil) or the attribute wasn't set.
func (a *ResourcesWorkspaceAdapter) originalOptionCode(attributeCode string) string {
	for _, value := range a.editor.originalValues {
		if value.AttributeCode == attributeCode {
			return value.OptionCode
		}
	}
	return ""
}

// editConfirmationQuestion builds the loop-edit flow's final step: a summary
// of every field the user actually edited this session, showing its CURRENT
// value (reusing formatResourceAttributeValue, the same renderer the picker
// menu and detail view already use) and a ConfirmationRequest — nothing has
// been persisted yet at this point. The header differs by mode:
// editorModeEdit says "guardar estos cambios" (an existing resource is being
// modified); editorModeDuplicate says "crear un nuevo recurso" (a brand-new
// Resource is about to be built, possibly with zero edited fields).
func (a *ResourcesWorkspaceAdapter) editConfirmationQuestion() InteractionResponse {
	state := a.editor
	state.step = editorStepConfirm
	byCode := map[string]domain.ResourceAttributeValue{}
	for _, value := range state.values {
		byCode[value.AttributeCode] = value
	}
	header := "Vas a guardar estos cambios:"
	if state.mode == editorModeDuplicate {
		header = "Vas a crear un nuevo recurso con estos valores:"
	}
	lines := []string{header}
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
				lines = append(lines, attribute.Definition.Name+": "+formatResourceAttributeValue(value))
			}
			break
		}
	}
	question := strings.Join(lines, "\n")
	return InteractionResponse{Pending: ConfirmationRequest{
		ID:           resourceEditorKey,
		Key:          resourceEditorKey,
		Question:     question,
		ConfirmLabel: "Sí, guardar",
		CancelLabel:  "No, cancelar",
	}}
}

// answerEditConfirmation handles the answer to editConfirmationQuestion:
// "yes" calls finishEditor with the CURRENT unit (possibly changed during
// this session, see resourceEditorState.currentUnit) — the exact same
// finishEditor/Update/duplicate-collision path Edit already had. Any other
// answer ("no") cancels the whole edit — resets a.editor, nothing saved,
// the same "cancelado" spirit as InputCancel.
func (a *ResourcesWorkspaceAdapter) answerEditConfirmation(ctx context.Context, value string) InteractionResponse {
	state := a.editor
	if value != "yes" {
		a.editor = nil
		return InteractionResponse{Messages: []InteractionMessage{
			TextMessage{Text: "Se canceló la edición."},
		}}
	}
	return a.finishEditor(ctx, state.currentUnit)
}

// filterApplicableValues drops any value whose attribute no longer resolves
// to applicable (per ResourceAttribute.Effective given the FULL final
// values) before a candidate is built. This is a real no-op for CREATE/the
// legacy sequential walkthrough: advanceEditor already skips
// ModeForbidden/notApplicable attributes, so state.values never accumulates
// one of those in the first place there. It is NOT a no-op for the
// loop-edit flow: state.values starts as a copy of the resource's full
// pre-existing Attributes, so replacing just ONE (e.g. insulation THW ->
// DESNUDO) can leave a now-forbidden OTHER value (color/voltage) still
// sitting in the slice — passing that straight into domain.NewResource would
// reject the whole edit ("attribute %q is forbidden"). Applied
// unconditionally so there is exactly one final-validation code path for
// both flows.
func filterApplicableValues(attributes []domain.ResourceAttribute, values []domain.ResourceAttributeValue) []domain.ResourceAttributeValue {
	applicable := map[string]bool{}
	for _, attribute := range attributes {
		mode, _, notApplicable := attribute.Effective(values)
		if mode != domain.ModeForbidden && !notApplicable {
			applicable[attribute.Definition.Code] = true
		}
	}
	var filtered []domain.ResourceAttributeValue
	for _, value := range values {
		if applicable[value.AttributeCode] {
			filtered = append(filtered, value)
		}
	}
	return filtered
}

// finishEditor builds the candidate Resource from the fully-answered editor
// state and persists it (Update targeting the original Resource.ID for
// editorModeEdit; Create — never carrying an ID — for both editorModeCreate
// and editorModeDuplicate). It always resets a.editor to nil — success,
// validation failure, and every Create/Update error all end the flow;
// simplicity over cleverness, an edge case a fresh "nuevo recurso"/
// "Editar"/"Duplicar" can retry.
func (a *ResourcesWorkspaceAdapter) finishEditor(ctx context.Context, unit string) InteractionResponse {
	state := a.editor
	values := filterApplicableValues(state.attributes, state.values)
	scope := domain.ResourceScope{ClassCode: state.class, FamilyCode: state.family, TypeCode: state.itemType}
	resource, err := domain.NewResource(a.catalog, scope, unit, values)
	if err != nil {
		a.editor = nil
		// NewResource's validation errors are deliberately human-decipherable
		// domain messages (e.g. "incoherent relation between \"diameter_inch\"
		// and \"diameter_mm\""), not raw infrastructure failures — unlike a
		// Postgres/network error, surfacing this one is the whole point: the
		// user needs to know WHY the combination they just picked is invalid,
		// not just that it is. Strip the generic ErrResourceValidation prefix
		// so only the specific reason shows.
		reason := strings.TrimPrefix(err.Error(), domain.ErrResourceValidation.Error()+": ")
		errorText := fmt.Sprintf("No pude crear el recurso: %s.", reason)
		if state.mode == editorModeEdit {
			errorText = fmt.Sprintf("No pude guardar los cambios: %s.", reason)
		}
		return InteractionResponse{Messages: []InteractionMessage{ErrorMessage{Text: errorText}}}
	}

	var opErr error
	switch state.mode {
	case editorModeEdit:
		resource.ID = state.originalID
		opErr = a.updater.Update(ctx, resource)
	default: // editorModeCreate, editorModeDuplicate
		opErr = a.creator.Create(ctx, resource)
	}
	if opErr != nil {
		a.editor = nil
		if errors.Is(opErr, domain.ErrDuplicateResource) {
			// resource.ClassCode (never resource.FamilyCode) is the
			// repository's real scoping key (design R1) — the family code
			// alone would resolve to ErrResourceNotFound.
			existing, getErr := a.resourcesGetter.Get(ctx, resource.ClassCode, resource.IdentityKey)
			if getErr != nil {
				return InteractionResponse{Messages: []InteractionMessage{
					ErrorMessage{Text: "Ya existe un recurso con esa identidad, pero no pude abrir su detalle."},
				}}
			}
			title, fields := a.resourcePresentation(existing)
			return InteractionResponse{Messages: []InteractionMessage{
				TextMessage{Text: "Ya existe un recurso con esa identidad. Este es el recurso existente:"},
				StructuredResult{Title: title, Fields: fields},
			}}
		}
		errorText := "No pude crear el recurso. Probá de nuevo en un momento."
		if state.mode == editorModeEdit {
			errorText = "No pude guardar los cambios. Probá de nuevo en un momento."
		}
		return InteractionResponse{Messages: []InteractionMessage{ErrorMessage{Text: errorText}}}
	}

	a.editor = nil
	title, fields := a.resourcePresentation(resource)
	return InteractionResponse{Messages: []InteractionMessage{StructuredResult{Title: title, Fields: fields}}}
}

// resourcePresentation builds the shared "one resource" presentation used
// both as a compact selectable-list Option.Label and as the detail view's
// own Title/Fields — built once, reused twice by the full workspace
// dispatch (not yet added to this struct, see the type doc comment) and by
// finishEditor's own success/duplicate-collision paths. IdentityKey (a
// technical composite key) is deliberately never surfaced. NOT_APPLICABLE
// attributes are omitted: a blank field is noise, not information.
//
// The title is the catalog-controlled canonical presentation resolved via
// describer (see resourceDescriber): it is never an automatic dump of every
// attribute, and it never falls back to one when a Tipo has no presentation
// configured (a.describer.Describe returning "" simply leaves the title as
// the bare FamilyCode).
func (a *ResourcesWorkspaceAdapter) resourcePresentation(resource domain.Resource) (title string, fields []Field) {
	attributes := append([]domain.ResourceAttributeValue(nil), resource.Attributes...)
	sort.SliceStable(attributes, func(i, j int) bool { return attributes[i].AttributeCode < attributes[j].AttributeCode })

	fields = []Field{{Label: "Unidad natural", Value: resource.NaturalUnit}}
	for _, attribute := range attributes {
		if attribute.Text == notApplicableAttributeText {
			continue
		}
		fields = append(fields, Field{Label: attribute.AttributeCode, Value: formatResourceAttributeValue(attribute)})
	}

	title = resourceCatalogIdentity(a.catalog, resource)
	if presentation := a.describer.Describe(resource); presentation != "" {
		title += " — " + presentation
	}
	// task 7.3: mark a resource whose Clase is inactive wherever this shared
	// title renders — search results (searchResponse's Option.Label) and the
	// detail view (detailResponse's StructuredResult.Title) both go through
	// this one function, so marking it here covers both surfaces at once.
	if !a.classIsActive(resource.ClassCode) {
		title += " (Clase inactiva)"
	}
	return title, fields
}

func resourceCatalogIdentity(catalog domain.ResourceCatalog, resource domain.Resource) string {
	for _, itemType := range catalog.Types {
		if itemType.ClassCode == resource.ClassCode && itemType.FamilyCode == resource.FamilyCode && itemType.Code == resource.TypeCode {
			return resourceTypePresentation(catalog, itemType)
		}
	}
	for _, family := range catalog.Families {
		if family.ClassCode == resource.ClassCode && family.Code == resource.FamilyCode {
			return resourceFamilyPresentation(catalog, family)
		}
	}
	return resource.FamilyCode
}

// formatResourceAttributeValue renders one ResourceAttributeValue for
// display — the resource-typed counterpart of handlers.go's
// formatAttributeValue (kept separate rather than shared, since the two
// operate on distinct value types until a later PR retires the Materiales
// Maestros-specific handler).
func formatResourceAttributeValue(attribute domain.ResourceAttributeValue) string {
	switch attribute.Type {
	case domain.ValueTypeControlledOption:
		return attribute.OptionCode
	case domain.ValueTypeInteger:
		if attribute.Integer != nil {
			return fmt.Sprintf("%d", *attribute.Integer)
		}
	case domain.ValueTypeDecimal:
		if attribute.Decimal != nil {
			return attribute.Decimal.String()
		}
	case domain.ValueTypeQuantity:
		if attribute.Quantity != nil {
			return attribute.Quantity.Value.String() + " " + attribute.Quantity.UnitCode
		}
	case domain.ValueTypeBoolean:
		if attribute.Boolean != nil {
			return fmt.Sprintf("%t", *attribute.Boolean)
		}
	case domain.ValueTypeControlledText:
		return attribute.Text
	}
	return ""
}
