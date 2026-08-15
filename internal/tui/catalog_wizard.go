package tui

import (
	"context"
	"fmt"

	"github.com/GARFEX33/garfex-costos-unitarios/internal/domain"
)

// catalogWizardActionID is the "Crear estructura de recursos" palette leaf's
// action id (commands.go's buildCatalogAdminActions) — the guided wizard's
// entry point into the Configuración workspace (design §9, business ask's
// proposed "Configuración → Catálogo de recursos → Crear estructura" shape).
const catalogWizardActionID = "catalog-wizard-start"

// catalogWizardFinishedMessage is shown once every step has been resolved.
const catalogWizardFinishedMessage = "Estructura creada correctamente."

// catalogWizardState is the guided wizard's own in-progress state (design
// §9/D14): NOT a bespoke 10-step state machine — a thin sequencer over the
// SAME generic per-kind create/reuse flow catalog_admin.go already provides
// for every registered CatalogKind (spec: Generic Descriptor-Driven Engine).
// steps is the fixed Clase→Familia→Tipo→Característica→ConjuntoOpciones→
// Opción→Unidad→Aplicabilidad(+Identidad)→Presentación order — Identidad is
// a field of Aplicabilidad, not its own kind, per design's reconciliation.
// context accumulates each completed step's own record reference, keyed by
// the SAME field name a later step's own FieldDescriptor would use to
// reference it (e.g. "class", "family") — generalizing startCreateEditor's
// classFilter pre-seed/skip trick (resource_editor.go:250-256) from one
// field to a map (design D14).
type catalogWizardState struct {
	steps   []domain.CatalogKindCode
	index   int
	context map[string]domain.CatalogValue
}

// catalogWizardSteps returns the wizard's fixed step order (design §9).
func catalogWizardSteps() []domain.CatalogKindCode {
	return []domain.CatalogKindCode{
		domain.KindClass, domain.KindFamily, domain.KindType, domain.KindAttributeDefinition,
		domain.KindOptionSet, domain.KindOption, domain.KindUnit, domain.KindAttributeBinding,
		domain.KindPresentationField,
	}
}

// catalogWizardContextKey returns the field name a LATER step's own
// FieldDescriptor uses to reference kind's record, once created — the key
// catalogWizardState.context stores that record's CatalogRef under. Kinds
// never referenced by a later step in this fixed order (Opción, Unidad,
// Aplicabilidad, Presentación — the last four steps) return "": nothing
// downstream ever needs to pre-seed/skip a question about them, though the
// step itself is still walked through the normal create/reuse mechanics.
func catalogWizardContextKey(kind domain.CatalogKindCode) string {
	switch kind {
	case domain.KindClass:
		return "class"
	case domain.KindFamily:
		return "family"
	case domain.KindType:
		return "type"
	case domain.KindAttributeDefinition:
		return "characteristic"
	case domain.KindOptionSet:
		return "optionSet"
	default:
		return ""
	}
}

// catalogWizardStepFieldName is the Name given to a step's own synthetic
// FieldDescriptor (catalogWizardStepField): catalogWizardContextKey when the
// step's kind is referenced later, otherwise a stable placeholder — the step
// is still walked through the same create/reuse mechanics either way, its
// result is simply never consumed by a later step's pre-seed.
func catalogWizardStepFieldName(kind domain.CatalogKindCode) string {
	if key := catalogWizardContextKey(kind); key != "" {
		return key
	}
	return "record"
}

// catalogWizardStepField builds the ONE synthetic FieldRef FieldDescriptor a
// wizard step's own "elegir existente o crear nueva/o" question renders
// from — reusing fieldQuestion's/refOptions' own FieldRef+AllowCreate
// mechanics unchanged (design §9: "the same AllowCreate branch the
// reuse-before-create field already uses"). RefScopedBy lists every ref
// field def itself declares, so refOptions narrows the listing by whatever
// of those the wizard's inherited context already answered — the same
// FieldDescriptor.RefScopedBy convention every other registered kind's own
// fields already use.
func catalogWizardStepField(def domain.CatalogKind) domain.FieldDescriptor {
	return domain.FieldDescriptor{
		Name: catalogWizardStepFieldName(def.Code), Label: def.Singular,
		Kind: domain.FieldRef, RefKind: def.Code,
		RefScopedBy: catalogWizardRefScopedBy(def), AllowCreate: true,
	}
}

// catalogWizardRefScopedBy lists def's own FieldRef field names — see
// catalogWizardStepField.
func catalogWizardRefScopedBy(def domain.CatalogKind) []string {
	var names []string
	for _, field := range def.Fields {
		if field.Kind == domain.FieldRef {
			names = append(names, field.Name)
		}
	}
	return names
}

// startWizard begins the guided wizard (design §9): seed pre-populates
// catalogWizardState.context BEFORE the first step runs — an already-known
// context (e.g. the wizard launched from within a specific Clase's
// workspace) skips every step whose kind that context already answers,
// exactly generalizing startCreateEditor's own classFilter pre-seed/skip
// trick from one field to this whole map (design D14). seed may be nil.
func (a *CatalogAdminAdapter) startWizard(ctx context.Context, seed map[string]domain.CatalogValue) (InteractionResponse, error) {
	a.wizard = &catalogWizardState{steps: catalogWizardSteps(), context: cloneCatalogEditorValues(seed)}
	return a.advanceWizard(ctx)
}

// advanceWizard walks a.wizard.steps from its current index forward: any
// step whose own context key is already known is skipped entirely (design
// §9's "skips the redundant Clase step" scenario); the first step that is
// NOT already known starts (startWizardStep). Once every step is resolved,
// the wizard ends with a Spanish confirmation and no Pending question.
func (a *CatalogAdminAdapter) advanceWizard(ctx context.Context) (InteractionResponse, error) {
	wizard := a.wizard
	for wizard.index < len(wizard.steps) {
		kind := wizard.steps[wizard.index]
		if key := catalogWizardContextKey(kind); key != "" {
			if _, known := wizard.context[key]; known {
				wizard.index++
				continue
			}
		}
		return a.startWizardStep(ctx, kind)
	}
	a.wizard = nil
	a.editor = nil
	return InteractionResponse{Messages: []InteractionMessage{TextMessage{Text: catalogWizardFinishedMessage}}}, nil
}

// startWizardStep opens ONE step's own "elegir existente o crear nueva/o"
// question for kind, scoped by whatever of kind's own ref fields the
// wizard's inherited context already answers. a.editor becomes a SYNTHETIC
// catalogEditorState (wizardStep: true) wrapping the one FieldRef field
// catalogWizardStepField builds — never a real per-kind create/edit flow on
// its own; finishEditor's wizardStep branch (catalog_admin.go) routes its
// completion back into finishWizardStep instead of ever calling
// creator.Create/updater.Update for it.
//
// Zero existing records for kind (in scope) skip the "elegir existente"
// question entirely and enter the create sub-flow directly (design §9: "the
// same AllowCreate branch the reuse-before-create field already uses") —
// reusing startWizardStepCreate, a thin wrapper around the SAME
// pause/pre-seed/resume mechanics startNestedCreateFlow (task 8.1) already
// established via catalogEditorState.parent/resumeField, generalized to
// also pre-seed the nested editor's own leading ref fields from the
// wizard's inherited context (catalogWizardSeedContext), not just one hint
// field.
func (a *CatalogAdminAdapter) startWizardStep(ctx context.Context, kind domain.CatalogKindCode) (InteractionResponse, error) {
	def, ok := a.registry.Kind(kind)
	if !ok {
		a.wizard = nil
		a.editor = nil
		return InteractionResponse{Messages: []InteractionMessage{
			ErrorMessage{Text: "No reconozco ese tipo de catálogo."},
		}}, nil
	}
	field := catalogWizardStepField(def)
	values := cloneCatalogEditorValues(a.wizard.context)
	a.editor = &catalogEditorState{
		mode:       catalogEditorCreate,
		def:        domain.CatalogKind{Code: def.Code, Singular: def.Singular, Plural: def.Plural, Fields: []domain.FieldDescriptor{field}},
		values:     values,
		wizardStep: true,
	}

	options, err := a.refOptions(ctx, field, values)
	if err != nil {
		return InteractionResponse{}, err
	}
	if len(options) == 0 {
		return a.startWizardStepCreate(ctx, def)
	}
	return a.fieldQuestion(ctx, field)
}

// startWizardStepCreate enters kind's create sub-flow directly from a
// wizard step with zero existing records in scope — the wizard's own
// dependency-aware branching (design §9). a.editor (the synthetic
// wizardStep editor startWizardStep just built) becomes the nested flow's
// parent, the exact task 8.1 mechanism (catalogEditorState.parent/
// resumeField), so cancelling or erroring the nested create resumes the
// SAME wizard step, never aborting the whole wizard (respondToEditor's
// existing parent!=nil branch, catalog_admin.go, needs no wizard-specific
// change at all).
func (a *CatalogAdminAdapter) startWizardStepCreate(ctx context.Context, def domain.CatalogKind) (InteractionResponse, error) {
	parent := a.editor
	nested := &catalogEditorState{
		mode: catalogEditorCreate, def: def, values: map[string]domain.CatalogValue{},
		parent: parent, resumeField: parent.def.Fields[0].Name,
	}
	catalogWizardSeedContext(nested, parent.values, def)
	a.editor = nested
	response, err := a.nextFieldQuestion(ctx)
	if err != nil {
		a.editor = parent
		return InteractionResponse{Messages: []InteractionMessage{
			ErrorMessage{Text: "No pude iniciar la creación. Probá de nuevo en un momento."},
		}}, nil
	}
	note := fmt.Sprintf("No hay %s disponibles. Creando una nueva/o.", def.Plural)
	response.Messages = append([]InteractionMessage{TextMessage{Text: note}}, response.Messages...)
	return response, nil
}

// catalogWizardSeedContext pre-seeds nested.values with every LEADING
// FieldRef field of def already answered by context, advancing nested.step
// past each one — generalizing startCreateEditor's classFilter pre-seed/skip
// trick (resource_editor.go:250-256) from one field to the whole inherited
// context map (design D14). Registered kinds always declare their
// parent-scoping ref fields first (catalog_kind.go), so a contiguous prefix
// scan is exactly right: the first field that is not a known ref stops the
// scan, matching state.step's own single-index "how far this flow has
// progressed" meaning.
func catalogWizardSeedContext(nested *catalogEditorState, context map[string]domain.CatalogValue, def domain.CatalogKind) {
	for i, field := range def.Fields {
		if field.Kind != domain.FieldRef {
			return
		}
		value, known := context[field.Name]
		if !known {
			return
		}
		nested.values[field.Name] = value
		nested.step = i + 1
	}
}

// finishWizardStep is finishEditor's wizardStep branch (catalog_admin.go):
// a.editor here is ALWAYS the synthetic wizardStep editor built by
// startWizardStep, holding exactly one answered FieldRef field — either
// because an existing record was picked (answerField's normal
// refOptionMatches path) or because startWizardStepCreate's nested create
// just resumed it (finishEditor's existing parent!=nil binding,
// catalog_admin.go). Neither case calls creator.Create/updater.Update
// here — the record either already existed or was already persisted by the
// nested flow; this only merges its reference into the wizard's inherited
// context and moves on to the next step.
func (a *CatalogAdminAdapter) finishWizardStep(ctx context.Context) (InteractionResponse, error) {
	state := a.editor
	field := state.def.Fields[0]
	a.wizard.context[field.Name] = state.values[field.Name]
	a.wizard.index++
	return a.advanceWizard(ctx)
}
