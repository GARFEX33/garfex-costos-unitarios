package tui

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/GARFEX33/garfex-costos-unitarios/internal/domain"
)

// catalogEditorKey identifies every QuestionRequest the catalog-structure
// create/edit engine emits — mirrors resource_editor.go's resourceEditorKey
// (one shared Key per in-progress flow, so respondToEditor can claim any
// answer directed at it without tracking a separate key per field).
const catalogEditorKey = "catalog-editor"

type catalogEditorMode int

const (
	catalogEditorCreate catalogEditorMode = iota
	catalogEditorEdit
)

// catalogLister/catalogGetter/catalogRecordCreator/catalogRecordUpdater are
// the minimal surfaces the generic catalog-admin engine needs from the
// application service — satisfied structurally by *catalogo.Service, the
// same narrow-interface convention resource_editor.go already established
// (resourceSearcher/resourceCreator/...) so this package never imports
// internal/app/catalogo directly.
type catalogLister interface {
	List(ctx context.Context, kind domain.CatalogKindCode, filter domain.CatalogFilter) ([]domain.CatalogRecord, error)
}

type catalogGetter interface {
	Get(ctx context.Context, kind domain.CatalogKindCode, id int64) (domain.CatalogRecord, error)
}

type catalogRecordCreator interface {
	Create(ctx context.Context, kind domain.CatalogKindCode, rec domain.CatalogRecord) (domain.CatalogRecord, error)
}

// catalogRecordUpdater's UpdateRevision is Update's CAS-aware additive
// sibling (design "Catalog transaction and coherent publication", stage
// 4C/4G): expectedRevision must be the record's own captured
// domain.CatalogRecord.Revision, never zero — insertion (catalogRecordCreator
// above) needs no such CAS predicate, which is why only Update migrated here.
type catalogRecordUpdater interface {
	UpdateRevision(ctx context.Context, kind domain.CatalogKindCode, rec domain.CatalogRecord, expectedRevision uint64) (domain.CatalogRecord, error)
}

// catalogDependencyChecker is the minimal surface the guarded-delete flow
// (task 7.1) needs: which OTHER catalog-structure kinds still reference a
// record — the data a "está siendo utilizada por N ..." message renders.
type catalogDependencyChecker interface {
	Dependencies(ctx context.Context, kind domain.CatalogKindCode, id int64) ([]domain.CatalogDependency, error)
}

// catalogReferenceChecker is the minimal surface the Código-immutability UI
// (task 7.2) and the guarded-delete flow (task 7.1) need: whether a record
// is referenced by at least one REAL resource instance (as opposed to
// another catalog-structure record, which catalogDependencyChecker covers).
type catalogReferenceChecker interface {
	ReferencedByResources(ctx context.Context, kind domain.CatalogKindCode, id int64) (bool, error)
}

// catalogDeactivator/catalogReactivator/catalogDeleter are the minimal
// surfaces the Desactivar/Reactivar/Eliminar lifecycle actions (task 7.1)
// need — each a narrow, single-method interface following this file's own
// established convention (catalogLister/catalogGetter/...). All three are the
// CAS-aware additive V2 siblings (4G): expectedRevision is always
// a.activeRecord.Revision, the value captured when the record's detail was
// opened (openRecordDetail).
type catalogDeactivator interface {
	DeactivateRevision(ctx context.Context, kind domain.CatalogKindCode, id int64, expectedRevision uint64) error
}

type catalogReactivator interface {
	ReactivateRevision(ctx context.Context, kind domain.CatalogKindCode, id int64, expectedRevision uint64) error
}

type catalogDeleter interface {
	HardDeleteRevision(ctx context.Context, kind domain.CatalogKindCode, id int64, expectedRevision uint64) error
}

// catalogEditorState is the in-progress create/edit flow for ONE record of
// ONE CatalogKind — the catalog-structure-admin counterpart to
// resourceEditorState (design D15: a PARALLEL state machine sharing zero
// code with it, since catalog-structure concepts like FieldDescriptor/
// CatalogRecord have no resource-*instance* meaning). Entirely
// descriptor-driven: nothing here ever branches on a specific
// CatalogKindCode — the SAME functions below serve every registered kind,
// differing only by def's own data (spec: Generic Descriptor-Driven Engine).
type catalogEditorState struct {
	mode catalogEditorMode
	// def is the CatalogKind descriptor this flow renders from.
	def domain.CatalogKind
	// step indexes def.Fields for the field currently being asked. Fields
	// are always walked in the registry's own declared order, which already
	// places parent-scoping refs before the fields they narrow (e.g. Tipo's
	// "class" then "family" before "code") — see catalog_kind.go.
	step int
	// values accumulates one CatalogValue per answered field, keyed by
	// FieldDescriptor.Name. Seeded from the existing record's Values for
	// catalogEditorEdit, empty for catalogEditorCreate.
	values map[string]domain.CatalogValue
	// original is a frozen copy of the record's Values as they were BEFORE
	// this edit session touched anything — nil for catalogEditorCreate. Used
	// solely by finishEditor (task 7.4) to detect whether
	// identityParticipates actually changed, since `values` above is the
	// mutable, live-edited copy.
	original map[string]domain.CatalogValue
	// id is the repository-assigned CatalogRecord.ID — 0 for create, the
	// existing record's ID for edit (finishEditor's Update target).
	id int64
	// revision is the migration-8 CAS domain.CatalogRecord.Revision captured
	// from the existing record at startEditFlow time — 0 for
	// catalogEditorCreate (no meaningful prior revision), the existing
	// record's own Revision for catalogEditorEdit. finishEditor threads it
	// onto the outgoing CatalogRecord so a later slice (4G) can pass it to
	// the revision-aware V2 service methods; dormant, no current service call
	// reads or checks it (mirrors CatalogRecord.Revision's own doc comment).
	revision uint64
	// parent is the PAUSED outer flow this editor was nested from (task 8.1:
	// reuse-before-create's nested create sub-flow) — nil for every top-level
	// flow (startCreateFlow/startEditFlow). A single linked pointer supports
	// any nesting depth for free, though the business ask only ever needs one
	// level (a reference field mid-flow needing a brand-new record). While a
	// nested editor is active, a.editor points at IT, not at parent — parent
	// itself is never touched until the nested flow ends (success or
	// cancel/error), at which point it becomes a.editor again.
	parent *catalogEditorState
	// resumeField is the PARENT's own FieldDescriptor.Name that triggered
	// this nested flow (an AllowCreate FieldRef field that received a custom
	// answer) — finishEditor uses it to bind the newly-created record's Ref
	// into parent.values once the nested flow completes. Empty when parent is
	// nil.
	resumeField string
	// wizardStep marks this editor as the SYNTHETIC "elegir existente o crear
	// nueva/o" step the guided wizard (task 9.1/9.2, catalog_wizard.go) builds
	// for ONE CatalogKind in its sequence — never a real per-kind create/edit
	// flow on its own (def always holds exactly one synthetic FieldRef field,
	// see catalogWizardStepField). finishEditor and respondToEditor's
	// InputCancel branch both special-case it: finishing merges the answered
	// field into the wizard's inherited context instead of ever calling
	// creator.Create/updater.Update, and cancelling a TOP-LEVEL wizard step
	// (parent == nil) aborts the whole wizard rather than just this step.
	wizardStep bool
}

// startCreateFlow begins the "crear" flow for kind — the generic entry point
// every registered CatalogKind shares. Two structurally different kinds
// (e.g. KindClass, a top-level kind, and KindFamily, a child kind scoped by
// a parent ref) both flow through this exact function, fieldQuestion, and
// answerField below — never a per-kind branch (spec: Generic
// Descriptor-Driven Engine, "shared engine path across two kinds").
func (a *CatalogAdminAdapter) startCreateFlow(ctx context.Context, kind domain.CatalogKindCode) (InteractionResponse, error) {
	def, ok := a.registry.Kind(kind)
	if !ok {
		return InteractionResponse{Messages: []InteractionMessage{
			ErrorMessage{Text: "No reconozco ese tipo de catálogo."},
		}}, nil
	}
	a.editor = &catalogEditorState{mode: catalogEditorCreate, def: def, values: map[string]domain.CatalogValue{}}
	response, err := a.nextFieldQuestion(ctx)
	if err != nil {
		a.editor = nil
		return InteractionResponse{Messages: []InteractionMessage{
			ErrorMessage{Text: "No pude iniciar la creación. Probá de nuevo en un momento."},
		}}, nil
	}
	return response, nil
}

// startEditFlow begins the "editar" flow for an existing record — the same
// generic entry point as startCreateFlow, differing only in the seeded
// values/id and the mode finishEditor later reads to call Update instead of
// Create.
func (a *CatalogAdminAdapter) startEditFlow(ctx context.Context, kind domain.CatalogKindCode, rec domain.CatalogRecord) (InteractionResponse, error) {
	def, ok := a.registry.Kind(kind)
	if !ok {
		return InteractionResponse{Messages: []InteractionMessage{
			ErrorMessage{Text: "No reconozco ese tipo de catálogo."},
		}}, nil
	}
	if catalogRecordIsConditional(def, rec) {
		return InteractionResponse{Messages: []InteractionMessage{
			ErrorMessage{Text: conditionalCatalogReadOnlyMessage},
		}}, nil
	}
	a.editor = &catalogEditorState{
		mode: catalogEditorEdit, def: def,
		values:   cloneCatalogEditorValues(rec.Values),
		original: cloneCatalogEditorValues(rec.Values),
		id:       rec.ID,
		revision: rec.Revision,
	}
	response, err := a.nextFieldQuestion(ctx)
	if err != nil {
		a.editor = nil
		return InteractionResponse{Messages: []InteractionMessage{
			ErrorMessage{Text: "No pude iniciar la edición. Probá de nuevo en un momento."},
		}}, nil
	}
	if kind == domain.KindUnit && strings.HasPrefix(rec.Values["name"].Text, "Provisional: ") {
		response.Messages = append([]InteractionMessage{TextMessage{Text: "Este nombre es provisional. Corregilo por un nombre humano antes de guardar."}}, response.Messages...)
	}
	return response, nil
}

func catalogRecordIsConditional(def domain.CatalogKind, rec domain.CatalogRecord) bool {
	return def.Code == domain.KindAttributeBinding && rec.Values["mode"].Text == "CONDITIONAL"
}

// respondToEditor handles one InteractionInput while a.editor is
// in-progress — mirrors resource_editor.go's respondToEditor: the returned
// bool reports whether input actually belonged to the editor (its own Key,
// or a cancellation); when false, Respond falls through to its normal
// dispatch instead of this method silently swallowing unrelated input.
func (a *CatalogAdminAdapter) respondToEditor(ctx context.Context, input InteractionInput) (InteractionResponse, bool) {
	if input.Kind == InputCancel {
		// Cancelling a NESTED create sub-flow (task 8.1) returns to the
		// PARENT flow's SAME field/question — never aborts the whole parent
		// flow, mirroring this codebase's general "cancel is local, not
		// catastrophic" philosophy (e.g. answerDeleteConfirmation's "no"
		// returning to the same detail view). Only a top-level flow's own
		// cancel actually clears a.editor entirely.
		if a.editor.parent != nil {
			parent := a.editor.parent
			a.editor = parent
			response, err := a.fieldQuestion(ctx, parent.def.Fields[parent.step])
			if err != nil {
				a.editor = nil
				return InteractionResponse{Messages: []InteractionMessage{
					ErrorMessage{Text: "No pude continuar con la operación. Probá de nuevo en un momento."},
				}}, true
			}
			response.Messages = append([]InteractionMessage{
				TextMessage{Text: "Se canceló la creación del registro nuevo. Segui con la pregunta anterior."},
			}, response.Messages...)
			return response, true
		}
		// Cancelling a TOP-LEVEL wizard step (task 9.2) aborts the WHOLE
		// wizard, not just the current step — mirrors resource_editor.go's
		// own top-level cancel precedent (nothing "local" is left in
		// progress once the very first question of a flow is cancelled).
		if a.editor.wizardStep {
			a.editor = nil
			a.wizard = nil
			return InteractionResponse{Messages: []InteractionMessage{
				TextMessage{Text: "Se canceló la creación de la estructura."},
			}}, true
		}
		a.editor = nil
		return InteractionResponse{Messages: []InteractionMessage{
			TextMessage{Text: "Se canceló la operación."},
		}}, true
	}
	if input.Key != catalogEditorKey {
		return InteractionResponse{}, false
	}
	response, err := a.answerField(ctx, input.Value)
	if err != nil {
		a.editor = nil
		return InteractionResponse{Messages: []InteractionMessage{
			ErrorMessage{Text: "No pude continuar con la operación. Probá de nuevo en un momento."},
		}}, true
	}
	return response, true
}

// fieldQuestion builds the QuestionRequest for ONE FieldDescriptor — the
// single per-FieldKind rendering switch every registered kind's every field
// goes through (spec: Generic Descriptor-Driven Engine). AllowCreate-flagged
// FieldRef fields emit SelectionSearchable+AllowCustom (design §5/D12) so a
// custom typed answer can later drive a nested create sub-flow — wiring that
// nested sub-flow itself is PR8's job; this PR only emits the correct
// question shape (see buildCatalogFieldValue's FieldRef branch).
func (a *CatalogAdminAdapter) fieldQuestion(ctx context.Context, field domain.FieldDescriptor) (InteractionResponse, error) {
	state := a.editor
	prompt := field.Label
	if field.Guidance != "" {
		prompt += "\n" + field.Guidance
	}
	current, hasCurrent := state.values[field.Name]

	switch field.Kind {
	case domain.FieldRef:
		options, err := a.refOptions(ctx, field, state.values)
		if err != nil {
			return InteractionResponse{}, err
		}
		if len(options) == 0 && !field.AllowCreate {
			refDef, _ := a.registry.Kind(field.RefKind)
			a.editor = nil
			return InteractionResponse{Messages: []InteractionMessage{
				ErrorMessage{Text: fmt.Sprintf("No hay %s disponibles. Primero creá un registro de %s.", strings.ToLower(refDef.Plural), refDef.Singular)},
			}}, nil
		}
		if hasCurrent && current.Ref.Code != "" {
			options = reorderOptionsCurrentFirst(options, current.Ref.Code)
		}
		return InteractionResponse{Pending: QuestionRequest{
			ID: catalogEditorKey, Key: catalogEditorKey, Prompt: prompt, Question: prompt,
			SelectionMode: SelectionSearchable, Options: options, AllowCustom: field.AllowCreate,
		}}, nil

	case domain.FieldEnum:
		options := make([]Option, len(field.EnumValues))
		for i, ev := range field.EnumValues {
			options[i] = Option{ID: ev.Value, Label: ev.Label, Value: ev.Value}
		}
		if hasCurrent {
			options = reorderOptionsCurrentFirst(options, current.Text)
		}
		return InteractionResponse{Pending: QuestionRequest{
			ID: catalogEditorKey, Key: catalogEditorKey, Prompt: prompt, Question: prompt,
			SelectionMode: SelectionSingle, Options: options,
		}}, nil

	case domain.FieldBool:
		options := []Option{
			{ID: "si", Label: "Sí", Value: "true"},
			{ID: "no", Label: "No", Value: "false"},
		}
		if hasCurrent {
			options = reorderOptionsCurrentFirst(options, boolValueString(current.Bool))
		}
		return InteractionResponse{Pending: QuestionRequest{
			ID: catalogEditorKey, Key: catalogEditorKey, Prompt: prompt, Question: prompt,
			SelectionMode: SelectionSingle, Options: options,
		}}, nil

	case domain.FieldText, domain.FieldCode, domain.FieldInt, domain.FieldStringList:
		// Free text: no Options list by default (the user must type a
		// value, mirroring resource_editor.go's QUANTITY question
		// precedent — SelectionSingle+AllowCustom with zero Options, since
		// SelectionFreeText is not wired into model.go's rendering/input
		// handling anywhere in this codebase). Whenever a current value
		// already exists — EDIT's stored value, OR task 8.1's nested
		// create sub-flow pre-seeding a hint field from typed text — one
		// "keep it" Option is seeded so pressing Enter without typing
		// reuses/accepts it (currentFreeTextOption, keyed off hasCurrent,
		// not off mode).
		var options []Option
		if label, value, ok := currentFreeTextOption(state.mode, field, current, hasCurrent); ok {
			options = []Option{{ID: "actual", Label: label, Value: value}}
		}
		return InteractionResponse{Pending: QuestionRequest{
			ID: catalogEditorKey, Key: catalogEditorKey, Prompt: prompt, Question: prompt,
			SelectionMode: SelectionSingle, Options: options, AllowCustom: true,
		}}, nil

	default:
		return InteractionResponse{}, fmt.Errorf("catalog admin: unsupported field kind %d for field %q", field.Kind, field.Name)
	}
}

// answerField handles the answer to state.def.Fields[state.step], advancing
// to the next field or, once every field has been answered, calling
// finishEditor — the single per-kind-agnostic advance loop every registered
// kind's create/edit flow shares.
func (a *CatalogAdminAdapter) answerField(ctx context.Context, value string) (InteractionResponse, error) {
	state := a.editor
	field := state.def.Fields[state.step]
	// Reuse-before-create (task 8.1/D12): an AllowCreate FieldRef's answer
	// that does NOT match any existing field.RefKind record already offered
	// (refOptionMatches) is a genuinely custom answer — pause THIS flow and
	// start a nested create sub-flow for field.RefKind instead of silently
	// building a Ref to a code that doesn't exist (which would only surface
	// as a late ErrCatalogReference from the repository). An answer matching
	// an existing option (by Código or by its own display Label) falls
	// through to the normal buildCatalogFieldValue path below, unchanged.
	if field.Kind == domain.FieldRef && field.AllowCreate {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			matched, err := a.refOptionMatches(ctx, field, state.values, trimmed)
			if err != nil {
				return InteractionResponse{}, err
			}
			if !matched {
				return a.startNestedCreateFlow(ctx, field, trimmed)
			}
		}
	}
	newValue, ok, errMsg := buildCatalogFieldValue(field, value)
	if !ok {
		response, err := a.fieldQuestion(ctx, field)
		if err != nil {
			return InteractionResponse{}, err
		}
		response.Messages = []InteractionMessage{ErrorMessage{Text: errMsg}}
		return response, nil
	}
	state.values[field.Name] = newValue
	state.step++
	return a.nextFieldQuestion(ctx)
}

// nextFieldQuestion advances state.step past any field that is currently
// LOCKED (task 7.2: a "código" field, Immutable == ImmutableOnceReferenced,
// during catalogEditorEdit, once referenceChecker reports the record is
// already referenced by real resources) — auto-keeping each skipped field's
// existing value untouched — then returns either the next actually-askable
// field's question or, once every field has been resolved, finishEditor's
// result. Every skipped field's Label is collected into a single Spanish
// informational note prepended to whatever InteractionResponse is ultimately
// returned, so the operator understands why that question never appeared.
// catalogEditorCreate never locks anything (fieldIsLocked's own mode check),
// so this is a pure passthrough for CREATE — startCreateFlow/answerField
// share it with startEditFlow/answerField rather than duplicating the
// "advance, then ask" loop per mode.
func (a *CatalogAdminAdapter) nextFieldQuestion(ctx context.Context) (InteractionResponse, error) {
	state := a.editor
	var lockedLabels []string
	for state.step < len(state.def.Fields) {
		field := state.def.Fields[state.step]
		locked, err := a.fieldIsLocked(ctx, field)
		if err != nil {
			return InteractionResponse{}, err
		}
		if !locked {
			break
		}
		lockedLabels = append(lockedLabels, field.Label)
		state.step++
	}

	var response InteractionResponse
	var err error
	if state.step >= len(state.def.Fields) {
		response, err = a.finishEditor(ctx)
	} else {
		response, err = a.fieldQuestion(ctx, state.def.Fields[state.step])
	}
	if err != nil {
		return InteractionResponse{}, err
	}
	if len(lockedLabels) > 0 {
		note := fmt.Sprintf("%s no se puede modificar porque ya está en uso por recursos existentes.", strings.Join(lockedLabels, ", "))
		response.Messages = append([]InteractionMessage{TextMessage{Text: note}}, response.Messages...)
	}
	return response, nil
}

// fieldIsLocked reports whether field must be silently skipped for the
// in-progress editor — see nextFieldQuestion's doc comment. Only
// catalogEditorEdit ever locks anything; CREATE and every Mutable/non-código
// field return false without ever calling the repository (task 7.2/D11).
func (a *CatalogAdminAdapter) fieldIsLocked(ctx context.Context, field domain.FieldDescriptor) (bool, error) {
	state := a.editor
	if state.mode != catalogEditorEdit || field.Immutable != domain.ImmutableOnceReferenced {
		return false, nil
	}
	return a.referenceChecker.ReferencedByResources(ctx, state.def.Code, state.id)
}

// finishEditor builds the CatalogRecord from the fully-answered editor state
// and persists it: Create for catalogEditorCreate, Update (targeting the
// original record's ID) for catalogEditorEdit.
//
// A TOP-LEVEL flow (state.parent == nil) always resets a.editor to nil —
// success and failure both end the flow, mirroring resource_editor.go
// finishEditor's own "simplicity over cleverness" precedent.
//
// A NESTED flow (task 8.1: state.parent != nil, started by
// startNestedCreateFlow) never ends the whole session on its own: success
// binds the newly-created record's Código into the parent's triggering field,
// advances the parent PAST that field, and resumes it via nextFieldQuestion
// (the parent's NEXT question, not a restart); an error resumes the parent at
// the SAME field/question it was on before nesting, showing the error inline
// — mirroring cancellation's own "return to the parent, don't abort it"
// behavior (respondToEditor) rather than losing the whole in-progress parent
// flow over a nested duplicate-código or validation error.
func (a *CatalogAdminAdapter) finishEditor(ctx context.Context) (InteractionResponse, error) {
	state := a.editor
	// A wizard step (task 9.1/9.2, catalog_wizard.go) never persists anything
	// of its own here — the record it resolves either already existed (an
	// "elegir existente" answer) or was already persisted by a nested create
	// sub-flow that already ran through THIS SAME finishEditor once, as a
	// normal (non-wizardStep) editor, and resumed the wizard step via the
	// parent-binding branch below. finishWizardStep only merges the result
	// into the wizard's inherited context and advances to the next step.
	if state.wizardStep {
		return a.finishWizardStep(ctx)
	}
	rec := domain.CatalogRecord{Kind: state.def.Code, Values: state.values, Active: true}

	var result domain.CatalogRecord
	var err error
	if state.mode == catalogEditorEdit {
		rec.ID = state.id
		// Thread the captured revision onto the outgoing record AND pass it
		// as UpdateRevision's CAS expectedRevision (see
		// catalogEditorState.revision's doc comment) — 4G's revision-aware
		// call.
		rec.Revision = state.revision
		result, err = a.updater.UpdateRevision(ctx, state.def.Code, rec, state.revision)
	} else {
		result, err = a.creator.Create(ctx, state.def.Code, rec)
	}

	mode, def, original, newValues, parent, resumeField := state.mode, state.def, state.original, state.values, state.parent, state.resumeField

	if err != nil {
		errMessage := ErrorMessage{Text: catalogErrorMessage(mode, def, err)}
		if parent != nil {
			a.editor = parent
			response, ferr := a.fieldQuestion(ctx, parent.def.Fields[parent.step])
			if ferr != nil {
				a.editor = nil
				return InteractionResponse{}, ferr
			}
			response.Messages = append([]InteractionMessage{errMessage}, response.Messages...)
			return response, nil
		}
		a.editor = nil
		return InteractionResponse{Messages: []InteractionMessage{errMessage}}, nil
	}

	if parent != nil {
		parent.values[resumeField] = domain.CatalogValue{Ref: domain.CatalogRef{Kind: def.Code, Code: result.Values["code"].Text}}
		parent.step++
		a.editor = parent
		response, nerr := a.nextFieldQuestion(ctx)
		if nerr != nil {
			a.editor = nil
			return InteractionResponse{}, nerr
		}
		note := fmt.Sprintf("%s creado: %s.", def.Singular, catalogRecordDisplayLabel(def, result))
		response.Messages = append([]InteractionMessage{TextMessage{Text: note}}, response.Messages...)
		return response, nil
	}

	a.editor = nil
	verb := "creado"
	if mode == catalogEditorEdit {
		verb = "actualizado"
	}
	title := fmt.Sprintf("%s %s", def.Singular, verb)
	messages := []InteractionMessage{StructuredResult{Title: title, Fields: catalogRecordFields(def, result)}}
	if identityParticipatesChanged(mode, def, original, newValues) {
		messages = append([]InteractionMessage{TextMessage{Text: identidadChangeWarning}}, messages...)
	}
	return InteractionResponse{Messages: messages}, nil
}

// identidadChangeWarning is task 7.4's Spanish warning (spec gap: "Warning
// shown on Identidad save") — shown at save time, never blocking the save
// itself. It is purely informational: domain.NewResource computes a
// Resource's IdentityKey once, at creation, and recursos-maestro has no
// recompute/migration path for an already-persisted resource (verified
// against internal/domain/resource_canonical.go and resource_editor.go's
// finishEditor, which only ever calls NewResource for a brand-new candidate
// — Update never recomputes IdentityKey for the resource being edited's
// UNCHANGED identity fields). Without this warning an operator could
// reasonably assume changing which Características participate in identity
// retroactively "fixes" existing resources, which it deliberately does not.
const identidadChangeWarning = "Aviso: los recursos existentes conservan su Identidad actual sin cambios. Solo los recursos que se creen a partir de ahora usarán esta nueva configuración de Identidad."

// identityParticipatesChanged reports whether finishEditor just persisted an
// Aplicabilidad (KindAttributeBinding) edit that flipped
// identityParticipates — the ONLY case identidadChangeWarning applies to
// (task 7.4 is scoped to Identidad configuration specifically, not every
// Aplicabilidad field). original is nil for catalogEditorCreate (see
// catalogEditorState's doc comment), which this correctly treats as "never
// changed" via mode != catalogEditorEdit — a brand-new binding has no prior
// rule to warn about.
func identityParticipatesChanged(mode catalogEditorMode, def domain.CatalogKind, original, values map[string]domain.CatalogValue) bool {
	if mode != catalogEditorEdit || def.Code != domain.KindAttributeBinding {
		return false
	}
	return original["identityParticipates"].Bool != values["identityParticipates"].Bool
}

// refOptions lists field.RefKind's existing records, narrowed by
// field.RefScopedBy (already-answered parent field values, e.g. Tipo's
// "family" field is scoped by its own already-answered "class" value) —
// CatalogFilter.Parent's own documented convention (catalog_record.go).
// Option.Value/ID is always the referenced record's own natural CODE
// (domain D10/CatalogRef — never a numeric database ID), and Option.Label is
// always its Spanish display name (catalogRecordDisplayLabel), never its raw
// Código (spec: Spanish-Only Catalog-Admin UI).
func (a *CatalogAdminAdapter) refOptions(ctx context.Context, field domain.FieldDescriptor, values map[string]domain.CatalogValue) ([]Option, error) {
	filter := domain.CatalogFilter{Status: domain.CatalogStatusActive}
	if len(field.RefScopedBy) > 0 {
		parent := make(map[string]domain.CatalogValue, len(field.RefScopedBy))
		for _, name := range field.RefScopedBy {
			if v, ok := values[name]; ok {
				parent[name] = v
			}
		}
		filter.Parent = parent
	}
	records, err := a.lister.List(ctx, field.RefKind, filter)
	if err != nil {
		return nil, err
	}
	refDef, _ := a.registry.Kind(field.RefKind)
	options := make([]Option, 0, len(records))
	for _, rec := range records {
		code := rec.Values["code"].Text
		if code == "" {
			continue
		}
		options = append(options, Option{ID: code, Label: catalogRecordDisplayLabel(refDef, rec), Value: code})
	}
	return options, nil
}

// startNestedCreateFlow begins a nested "crear" sub-flow for field.RefKind
// (task 8.1: reuse-before-create) after answerField determined typed does not
// match any existing field.RefKind record. The in-progress (parent) editor is
// preserved via catalogEditorState.parent rather than discarded — finishEditor
// resumes it once the nested record is created (or respondToEditor resumes it
// on cancel). typed pre-seeds the nested kind's own display field (name/
// label/symbol — catalogHintFieldName's priority, the same one
// catalogRecordDisplayLabel reads when rendering an existing record) as a
// SUGGESTED default: the nested flow still ASKS that field (fieldQuestion's
// free-text branch offers it as a selectable "Usar: <typed>" option,
// generalizing the edit flow's own "Mantener actual" precedent to CREATE via
// hasCurrent rather than mode) — nothing about what the operator typed is
// silently committed without their confirmation.
func (a *CatalogAdminAdapter) startNestedCreateFlow(ctx context.Context, field domain.FieldDescriptor, typed string) (InteractionResponse, error) {
	refDef, ok := a.registry.Kind(field.RefKind)
	if !ok {
		return InteractionResponse{Messages: []InteractionMessage{
			ErrorMessage{Text: "No reconozco ese tipo de catálogo."},
		}}, nil
	}
	parent := a.editor
	nested := &catalogEditorState{
		mode: catalogEditorCreate, def: refDef, values: map[string]domain.CatalogValue{},
		parent: parent, resumeField: field.Name,
	}
	if hintField, ok := catalogHintFieldName(refDef); ok {
		nested.values[hintField] = domain.CatalogValue{Text: typed}
	}
	a.editor = nested
	response, err := a.nextFieldQuestion(ctx)
	if err != nil {
		a.editor = parent
		return InteractionResponse{Messages: []InteractionMessage{
			ErrorMessage{Text: "No pude iniciar la creación. Probá de nuevo en un momento."},
		}}, nil
	}
	note := fmt.Sprintf("No encontré %q entre %s existentes. Creando un nuevo registro de %s.",
		typed, strings.ToLower(refDef.Plural), refDef.Singular)
	response.Messages = append([]InteractionMessage{TextMessage{Text: note}}, response.Messages...)
	return response, nil
}

// catalogHintFieldName returns the first field name def declares among the
// SAME name/label/symbol display priority catalogRecordDisplayLabel uses to
// render an existing record — startNestedCreateFlow uses it to decide which
// field the operator's typed text becomes a suggested default for. ok is
// false when def declares none of the three (no AllowCreate target kind hits
// this today — Característica/Unidad/ConjuntoOpciones all declare "name" or
// "symbol" — but the fallback keeps this helper honest rather than assuming).
func catalogHintFieldName(def domain.CatalogKind) (string, bool) {
	for _, name := range []string{"name", "label", "symbol"} {
		for _, fd := range def.Fields {
			if fd.Name == name {
				return name, true
			}
		}
	}
	return "", false
}

// refOptionMatches reports whether raw exactly matches an existing
// field.RefKind record already offered as an Option for this field's current
// scope (refOptions) — either its natural Código (Option.Value) or its own
// Spanish display label (Option.Label, case-insensitive), so an operator who
// types out an existing record's full name reuses it instead of accidentally
// creating a duplicate (task 8.1/8.2: search-before-create). Any non-match is
// what answerField treats as genuinely custom, routing into
// startNestedCreateFlow.
func (a *CatalogAdminAdapter) refOptionMatches(ctx context.Context, field domain.FieldDescriptor, values map[string]domain.CatalogValue, raw string) (bool, error) {
	options, err := a.refOptions(ctx, field, values)
	if err != nil {
		return false, err
	}
	for _, opt := range options {
		if opt.Value == raw || strings.EqualFold(opt.Label, raw) {
			return true, nil
		}
	}
	return false, nil
}

// cloneCatalogEditorValues returns a shallow copy of values — startEditFlow
// must never let the in-progress editor mutate the CatalogRecord the caller
// (openRecordForEdit) read from the repository.
func cloneCatalogEditorValues(values map[string]domain.CatalogValue) map[string]domain.CatalogValue {
	out := make(map[string]domain.CatalogValue, len(values))
	for k, v := range values {
		out[k] = v
	}
	return out
}

// buildCatalogFieldValue parses raw (an Option.Value or a free-typed answer)
// into a CatalogValue for field's Kind — the single per-FieldKind parsing
// logic every registered kind's every field shares (mirrors
// resource_editor.go's buildAttributeValue precedent). ok is false when raw
// fails validation (a required empty field, an unparseable integer, an
// unrecognized enum/bool option) — errMsg is the Spanish message to show,
// re-asking the same field rather than silently advancing.
func buildCatalogFieldValue(field domain.FieldDescriptor, raw string) (domain.CatalogValue, bool, string) {
	switch field.Kind {
	case domain.FieldText, domain.FieldCode:
		trimmed := strings.TrimSpace(raw)
		if field.Required && trimmed == "" {
			return domain.CatalogValue{}, false, fmt.Sprintf("%s es obligatorio.", field.Label)
		}
		return domain.CatalogValue{Text: trimmed}, true, ""
	case domain.FieldInt:
		trimmed := strings.TrimSpace(raw)
		if trimmed == "" {
			if field.Required {
				return domain.CatalogValue{}, false, fmt.Sprintf("%s es obligatorio.", field.Label)
			}
			return domain.CatalogValue{}, true, ""
		}
		n, err := strconv.Atoi(trimmed)
		if err != nil {
			return domain.CatalogValue{}, false, fmt.Sprintf("%q no es un número entero válido para %s.", raw, field.Label)
		}
		return domain.CatalogValue{Int: n}, true, ""
	case domain.FieldBool:
		switch raw {
		case "true":
			return domain.CatalogValue{Bool: true}, true, ""
		case "false":
			return domain.CatalogValue{Bool: false}, true, ""
		default:
			return domain.CatalogValue{}, false, fmt.Sprintf("Respuesta no válida para %s.", field.Label)
		}
	case domain.FieldStringList:
		var list []string
		for _, part := range strings.Split(raw, ",") {
			trimmed := strings.TrimSpace(part)
			if trimmed != "" {
				list = append(list, trimmed)
			}
		}
		if field.Required && len(list) == 0 {
			return domain.CatalogValue{}, false, fmt.Sprintf("%s es obligatorio.", field.Label)
		}
		return domain.CatalogValue{List: list}, true, ""
	case domain.FieldEnum:
		for _, ev := range field.EnumValues {
			if ev.Value == raw {
				return domain.CatalogValue{Text: raw}, true, ""
			}
		}
		return domain.CatalogValue{}, false, fmt.Sprintf("Opción no válida para %s.", field.Label)
	case domain.FieldRef:
		trimmed := strings.TrimSpace(raw)
		if field.Required && trimmed == "" {
			return domain.CatalogValue{}, false, fmt.Sprintf("%s es obligatorio.", field.Label)
		}
		return domain.CatalogValue{Ref: domain.CatalogRef{Kind: field.RefKind, Code: trimmed}}, true, ""
	default:
		return domain.CatalogValue{}, false, "Tipo de campo no soportado."
	}
}

// currentFreeTextOption builds a pre-filled Option for a free-text field
// (FieldText/FieldCode/FieldInt/FieldStringList) whenever hasCurrent is true
// — ok is false when there is no meaningful current value to offer (e.g. an
// unset optional field), matching resource_editor.go's "never fabricate a
// default for nothing" precedent. The label prefix depends on WHY a current
// value exists: catalogEditorEdit's stored value is something to keep
// ("Mantener actual: X"); a catalogEditorCreate nested sub-flow's hint value
// (task 8.1: typed text pre-seeding the new record's display field) is only a
// suggestion to use ("Usar: X") — either way the operator must actively
// accept it (selecting the Option), nothing is silently committed.
func currentFreeTextOption(mode catalogEditorMode, field domain.FieldDescriptor, current domain.CatalogValue, hasCurrent bool) (label, value string, ok bool) {
	if !hasCurrent {
		return "", "", false
	}
	prefix := "Usar"
	if mode == catalogEditorEdit {
		prefix = "Mantener actual"
	}
	switch field.Kind {
	case domain.FieldInt:
		return fmt.Sprintf("%s: %d", prefix, current.Int), strconv.Itoa(current.Int), true
	case domain.FieldStringList:
		if len(current.List) == 0 {
			return "", "", false
		}
		joined := strings.Join(current.List, ", ")
		return prefix + ": " + joined, joined, true
	default: // FieldText, FieldCode
		if current.Text == "" {
			return "", "", false
		}
		return prefix + ": " + current.Text, current.Text, true
	}
}

// reorderOptionsCurrentFirst reorders options so the one matching
// currentValue (if any) is first — the catalog-admin engine's own analogue
// of resource_editor.go's defaultOptionFirst/defaultUnitFirst (D15: a
// separate implementation, not shared code, since it operates on []Option
// rather than a domain-specific slice type).
func reorderOptionsCurrentFirst(options []Option, currentValue string) []Option {
	if currentValue == "" {
		return options
	}
	for i, opt := range options {
		if opt.Value != currentValue {
			continue
		}
		if i == 0 {
			return options
		}
		reordered := make([]Option, 0, len(options))
		reordered = append(reordered, opt)
		reordered = append(reordered, options[:i]...)
		reordered = append(reordered, options[i+1:]...)
		return reordered
	}
	return options
}

func boolValueString(b bool) string {
	if b {
		return "true"
	}
	return "false"
}

// catalogRecordDisplayLabel renders rec's Spanish display label for use as
// an Option.Label/StructuredResult field — the first non-empty candidate
// among name/label/symbol (the Spanish-labeled fields every kind that is
// ever a FieldRef target actually declares), falling back to its Código only
// when no friendlier field exists (e.g. Unidad, which has no separate
// display name). This is the generic engine's own "never render a raw
// Código as if it were a friendly label" guard (spec: Spanish-Only
// Catalog-Admin UI) — it never special-cases a specific CatalogKindCode.
func catalogRecordDisplayLabel(def domain.CatalogKind, rec domain.CatalogRecord) string {
	return catalogRecordPresentation(def, rec)
}

// catalogRecordPresentation is the single business-first identity policy for
// catalog lists, searchable references, and detail titles. Stable Código is
// kept in detail fields and mutation values, never promoted over the human
// identity.
func catalogRecordPresentation(def domain.CatalogKind, rec domain.CatalogRecord) string {
	name := rec.Values["name"].Text
	switch def.Code {
	case domain.KindFamily:
		return joinCatalogIdentity(name, catalogRefLabel(rec, "class"))
	case domain.KindType:
		return joinCatalogIdentity(name, catalogRefLabel(rec, "class")+" › "+catalogRefLabel(rec, "family"))
	case domain.KindUnit:
		return catalogUnitPresentation(name, rec.Values["symbol"].Text, rec.Values["dimension"].Text)
	}
	for _, name := range []string{"name", "label", "symbol"} {
		if v, ok := rec.Values[name]; ok && strings.TrimSpace(v.Text) != "" {
			return v.Text
		}
	}
	if v, ok := rec.Values["code"]; ok && v.Text != "" {
		return v.Text
	}
	return fmt.Sprintf("%s #%d", def.Singular, rec.ID)
}

func catalogRefLabel(rec domain.CatalogRecord, field string) string {
	ref := rec.Values[field].Ref
	if strings.TrimSpace(ref.Label) != "" {
		return ref.Label
	}
	return ref.Code
}

func joinCatalogIdentity(primary, context string) string {
	primary = strings.TrimSpace(primary)
	context = strings.TrimSpace(context)
	if primary == "" {
		return context
	}
	if context == "" {
		return primary
	}
	return primary + " · " + context
}

func catalogUnitPresentation(name, symbol, dimension string) string {
	return strings.Join([]string{strings.TrimSpace(name), strings.TrimSpace(symbol), strings.TrimSpace(dimension)}, " · ")
}

// catalogRecordFields renders rec as the StructuredResult.Fields shown after
// a successful create/edit — one Field per def.Fields entry, Label always
// the FieldDescriptor's own Spanish Label, never a raw storage key or Código
// (spec: Spanish-Only Catalog-Admin UI).
func catalogRecordFields(def domain.CatalogKind, rec domain.CatalogRecord) []Field {
	fields := make([]Field, 0, len(def.Fields)+1)
	for _, fd := range def.Fields {
		fields = append(fields, Field{Label: fd.Label, Value: formatCatalogFieldValue(fd, rec.Values[fd.Name])})
	}
	fields = append(fields, Field{Label: "Estado", Value: catalogStatusLabel(rec.Active)})
	return fields
}

func catalogStatusLabel(active bool) string {
	if active {
		return "Activo"
	}
	return "Inactivo"
}

// formatCatalogFieldValue renders one CatalogValue for display, per its
// FieldDescriptor.Kind — the catalog-admin counterpart to
// resource_editor.go's formatResourceAttributeValue.
func formatCatalogFieldValue(field domain.FieldDescriptor, v domain.CatalogValue) string {
	switch field.Kind {
	case domain.FieldBool:
		if v.Bool {
			return "Sí"
		}
		return "No"
	case domain.FieldInt:
		return strconv.Itoa(v.Int)
	case domain.FieldStringList:
		return strings.Join(v.List, ", ")
	case domain.FieldRef:
		if strings.TrimSpace(v.Ref.Label) != "" {
			return v.Ref.Label
		}
		return v.Ref.Code
	case domain.FieldEnum:
		for _, ev := range field.EnumValues {
			if ev.Value == v.Text {
				return ev.Label
			}
		}
		return v.Text
	default:
		return v.Text
	}
}

// catalogErrorMessage maps a Create/Update error into a Spanish, kind-aware
// message — never leaking a raw Go error or a raw CatalogKindCode into the
// UI (spec: Spanish-Only Catalog-Admin UI). PR7 owns the dedicated
// dependency-guard/Desactivar messaging (design §6/§1a); this is the
// generic Create/Edit engine's own baseline error surface.
func catalogErrorMessage(mode catalogEditorMode, def domain.CatalogKind, err error) string {
	verb := "crear"
	if mode == catalogEditorEdit {
		verb = "guardar"
	}
	switch {
	case errors.Is(err, domain.ErrCodeImmutable):
		return "El código ya está en uso por recursos existentes y no puede cambiarse."
	case errors.Is(err, domain.ErrCatalogDuplicate):
		return fmt.Sprintf("Ya existe un registro de %s con ese código.", def.Singular)
	case errors.Is(err, domain.ErrCatalogReference):
		return "Una de las referencias seleccionadas no es válida."
	case errors.Is(err, domain.ErrCatalogInUse):
		return "No pude completar la operación: el registro sigue en uso. Podés desactivarlo en su lugar."
	case errors.Is(err, domain.ErrResourceValidation):
		reason := strings.TrimPrefix(err.Error(), domain.ErrResourceValidation.Error()+": ")
		return fmt.Sprintf("No pude %s %s: %s.", verb, strings.ToLower(def.Singular), reason)
	case errors.Is(err, domain.ErrResourceReference):
		reason := strings.TrimPrefix(err.Error(), domain.ErrResourceReference.Error()+": ")
		return fmt.Sprintf("No pude %s %s: %s.", verb, strings.ToLower(def.Singular), reason)
	default:
		return fmt.Sprintf("No pude %s %s. Probá de nuevo en un momento.", verb, strings.ToLower(def.Singular))
	}
}

// catalogLifecycleErrorMessage maps a Desactivar/Reactivar/Eliminar error
// (task 7.1) into a Spanish message — the lifecycle-action counterpart of
// catalogErrorMessage, kept separate since these actions have no "mode"
// (Create/Edit) of their own and their own distinct verb ("desactivar",
// "reactivar", "eliminar").
func catalogLifecycleErrorMessage(def domain.CatalogKind, verb string, err error) string {
	switch {
	case errors.Is(err, domain.ErrSoftDeleteUnsupported):
		return fmt.Sprintf("%s no admite desactivar/reactivar todavía.", def.Plural)
	case errors.Is(err, domain.ErrCatalogInUse):
		return fmt.Sprintf("No pude %s: el registro sigue en uso. Podés desactivarlo en su lugar.", verb)
	case errors.Is(err, domain.ErrCatalogRecordNotFound):
		return "No encontré ese registro. Puede que ya haya sido eliminado."
	default:
		return fmt.Sprintf("No pude %s %s. Probá de nuevo en un momento.", verb, strings.ToLower(def.Singular))
	}
}
