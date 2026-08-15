package tui

import (
	"context"
	"testing"

	"github.com/GARFEX33/garfex-costos-unitarios/internal/domain"
)

// TestCatalogAdminWizardActionStartsWizard proves the "Crear estructura de
// recursos" palette leaf (commands.go's catalogWizardActionID) actually
// starts the guided wizard via CatalogAdminAdapter.Respond — the production
// entry point, not just the lower-level startWizard helper.
func TestCatalogAdminWizardActionStartsWizard(t *testing.T) {
	adapter := newCatalogAdminAdapter(&fakeCatalogLister{}, &fakeCatalogGetter{}, &fakeCatalogCreator{}, &fakeCatalogUpdater{})
	ctx := context.Background()
	response, err := adapter.Respond(ctx, InteractionInput{Kind: InputAction, ActionID: catalogWizardActionID})
	if err != nil {
		t.Fatalf("Respond(wizard action) error = %v, want nil", err)
	}
	if adapter.wizard == nil {
		t.Fatalf("adapter.wizard = nil, want a started wizard")
	}
	question, ok := response.Pending.(QuestionRequest)
	if !ok || question.Prompt != "Código" {
		t.Fatalf("Pending = %#v, want KindClass's own first (Código) question — no existing Clases", response.Pending)
	}
}

// TestCatalogAdminWizardZeroRowsEntersCreateSubFlowDirectly proves task 9.2's
// first dependency-aware scenario: when the current step's kind has zero
// existing records in scope, the wizard skips the "elegir existente"
// question entirely and enters the create sub-flow directly (design §9),
// while still surfacing an explanatory Spanish note.
func TestCatalogAdminWizardZeroRowsEntersCreateSubFlowDirectly(t *testing.T) {
	adapter := newCatalogAdminAdapter(&fakeCatalogLister{}, &fakeCatalogGetter{}, &fakeCatalogCreator{}, &fakeCatalogUpdater{})
	ctx := context.Background()
	response, err := adapter.startWizard(ctx, nil)
	if err != nil {
		t.Fatalf("startWizard() error = %v, want nil", err)
	}
	if adapter.editor == nil || adapter.editor.def.Code != domain.KindClass {
		t.Fatalf("adapter.editor = %#v, want an in-progress nested KindClass create", adapter.editor)
	}
	if adapter.editor.parent == nil || !adapter.editor.parent.wizardStep {
		t.Fatalf("adapter.editor.parent = %#v, want the synthetic wizard-step editor as parent", adapter.editor.parent)
	}
	question, ok := response.Pending.(QuestionRequest)
	if !ok || question.Prompt != "Código" {
		t.Fatalf("Pending = %#v, want Clase's own Código question", response.Pending)
	}
	foundNote := false
	for _, msg := range response.Messages {
		if text, ok := msg.(TextMessage); ok && containsAll(text.Text, "No hay", "Clases") {
			foundNote = true
		}
	}
	if !foundNote {
		t.Fatalf("Messages = %#v, want a Spanish note explaining no existing Clases were found", response.Messages)
	}
}

// TestCatalogAdminWizardSkipsWholeStepWhenContextAlreadyKnown proves task
// 9.2's second dependency-aware scenario: entering the wizard with an
// already-known context (e.g. launched from a specific Clase) skips the
// redundant Clase step entirely and starts at Familia — the classFilter
// pre-seed/skip precedent generalized to an entire step, not just one field.
func TestCatalogAdminWizardSkipsWholeStepWhenContextAlreadyKnown(t *testing.T) {
	adapter := newCatalogAdminAdapter(&fakeCatalogLister{}, &fakeCatalogGetter{}, &fakeCatalogCreator{}, &fakeCatalogUpdater{})
	ctx := context.Background()
	seed := map[string]domain.CatalogValue{
		"class": {Ref: domain.CatalogRef{Kind: domain.KindClass, Code: "MATERIAL"}},
	}
	response, err := adapter.startWizard(ctx, seed)
	if err != nil {
		t.Fatalf("startWizard() error = %v, want nil", err)
	}
	if adapter.wizard.index != 1 {
		t.Fatalf("adapter.wizard.index = %d, want 1 (Clase step skipped)", adapter.wizard.index)
	}
	if adapter.editor == nil || adapter.editor.def.Code != domain.KindFamily {
		t.Fatalf("adapter.editor = %#v, want an in-progress nested KindFamily create", adapter.editor)
	}
	// Familia's own "class" field must ALSO be pre-seeded/skipped — its
	// first real question is "Código", never "Clase" again.
	question, ok := response.Pending.(QuestionRequest)
	if !ok || question.Prompt != "Código" {
		t.Fatalf("Pending = %#v, want Familia's Código question (class pre-seeded)", response.Pending)
	}
	if adapter.editor.values["class"].Ref != (domain.CatalogRef{Kind: domain.KindClass, Code: "MATERIAL"}) {
		t.Fatalf("nested Familia values[class] = %#v, want the seeded Clase ref", adapter.editor.values["class"])
	}
}

// TestCatalogAdminWizardOffersExistingRecordsBeforeCreatingNew proves the
// dependency-aware branching's non-zero-rows half: when the current step's
// kind DOES have existing records in scope, the wizard shows a searchable
// "elegir existente" question (reusing fieldQuestion's own FieldRef+
// AllowCreate rendering) instead of jumping straight into create, and
// picking an existing record merges its ref into context WITHOUT calling
// Create.
func TestCatalogAdminWizardOffersExistingRecordsBeforeCreatingNew(t *testing.T) {
	lister := &fakeCatalogLister{records: map[domain.CatalogKindCode][]domain.CatalogRecord{
		domain.KindFamily: {familyRecord(7, "MATERIAL", "CONDUCTORES", "Conductores")},
	}}
	creator := &fakeCatalogCreator{nextID: 1}
	adapter := newCatalogAdminAdapter(lister, &fakeCatalogGetter{}, creator, &fakeCatalogUpdater{})
	ctx := context.Background()
	seed := map[string]domain.CatalogValue{
		"class": {Ref: domain.CatalogRef{Kind: domain.KindClass, Code: "MATERIAL"}},
	}
	response, err := adapter.startWizard(ctx, seed)
	if err != nil {
		t.Fatalf("startWizard() error = %v, want nil", err)
	}
	question, ok := response.Pending.(QuestionRequest)
	if !ok || question.Prompt != "Familia" {
		t.Fatalf("Pending = %#v, want the Familia 'elegir existente' question", response.Pending)
	}
	if len(question.Options) != 1 || question.Options[0].Value != "CONDUCTORES" {
		t.Fatalf("Options = %#v, want the single existing Conductores Familia", question.Options)
	}
	if adapter.editor == nil || !adapter.editor.wizardStep {
		t.Fatalf("adapter.editor = %#v, want the synthetic wizard-step editor still pending", adapter.editor)
	}

	// Picking the existing Familia merges its ref into context and advances
	// past the Tipo step's own "class"/"family" questions without creating
	// anything.
	response, err = adapter.answerField(ctx, "CONDUCTORES")
	if err != nil {
		t.Fatalf("answerField(existing familia) error = %v, want nil", err)
	}
	if len(creator.calls) != 0 {
		t.Fatalf("creator.calls = %#v, want zero — an existing record was reused, not created", creator.calls)
	}
	if adapter.wizard.context["family"].Ref != (domain.CatalogRef{Kind: domain.KindFamily, Code: "CONDUCTORES"}) {
		t.Fatalf("wizard.context[family] = %#v, want a Ref to the reused Conductores record", adapter.wizard.context["family"])
	}
	tipoQuestion, ok := response.Pending.(QuestionRequest)
	if !ok || tipoQuestion.Prompt != "Código" {
		t.Fatalf("Pending = %#v, want Tipo's own Código question (class+family pre-seeded)", response.Pending)
	}
}

// TestCatalogAdminWizardSequencesNineKindsPreseedingInheritedContext is the
// spec-required end-to-end proof (catalog-guided-setup/Guided Wizard with
// Dependency-Aware Steps): the wizard walks Clase→Familia→Tipo→
// Característica→ConjuntoOpciones→Opción→Unidad→Aplicabilidad(+Identidad)→
// Presentación in order, and at every step after the first, every leading
// ref field already known from a PRIOR step's created record is pre-seeded
// and skipped — never re-asked — generalizing startCreateEditor's
// classFilter pre-seed/skip trick from one field to the whole inherited
// context map (design D14/§9).
func TestCatalogAdminWizardSequencesNineKindsPreseedingInheritedContext(t *testing.T) {
	creator := &fakeCatalogCreator{nextID: 1}
	adapter := newCatalogAdminAdapter(&fakeCatalogLister{}, &fakeCatalogGetter{}, creator, &fakeCatalogUpdater{})
	ctx := context.Background()

	response, err := adapter.startWizard(ctx, nil)
	if err != nil {
		t.Fatalf("startWizard() error = %v, want nil", err)
	}
	assertPrompt(t, response, "Código") // Clase.code

	answers := []struct {
		value      string
		wantPrompt string // the prompt expected AFTER sending value, "" if the wizard should be done
	}{
		// Clase: code, name, plural, slug, order, aliases, keywords.
		{"TEST_CLASE", "Nombre"},
		{"Test Clase", "Plural"},
		{"Test Clases", "Slug"},
		{"test-clase", "Orden"},
		{"", "Alias"},
		{"", "Palabras clave"},
		{"", "Código"}, // Clase created -> Familia.code (class pre-seeded/skipped)
		// Familia: code, name.
		{"TEST_FAMILIA", "Nombre"},
		{"Test Familia", "Código"}, // Familia created -> Tipo.code (class+family pre-seeded/skipped)
		// Tipo: code, name.
		{"TEST_TIPO", "Nombre"},
		{"Test Tipo", "Código"}, // Tipo created -> Característica.code (no refs to pre-seed)
		// Característica: code, name, valueType, dimension, defaultIdentityParticipates.
		{"test_nivel", "Nombre"},
		{"Nivel de Servicio", "Tipo de valor"},
		{"CONTROLLED_TEXT", "Dimensión"},
		{"", "Participa en Identidad por defecto"},
		{"false", "Código"}, // Característica created -> ConjuntoOpciones.code
		// ConjuntoOpciones: code, name.
		{"TEST_NIVELES", "Nombre"},
		{"Niveles de Servicio", "Código"}, // ConjuntoOpciones created -> Opción.code (optionSet+characteristic pre-seeded)
		// Opción: code, label.
		{"ALTO", "Etiqueta"},
		{"Alto", "Código"}, // Opción created -> Unidad.code (no refs to pre-seed)
		// Unidad: code, symbol, dimension.
		{"TEST_UNIDAD", "Símbolo"},
		{"tu", "Dimensión"},
		{"", "Modo"}, // Unidad created -> Aplicabilidad.mode (class/family/type/characteristic/optionSet ALL pre-seeded)
		// Aplicabilidad: mode, identityParticipates.
		{"REQUIRED", "Participa en Identidad"},
		{"true", "Posición"}, // Aplicabilidad created -> Presentación.position (class/family/type/characteristic pre-seeded)
		// Presentación: position.
		{"1", ""}, // Presentación created -> wizard finished
	}

	for i, step := range answers {
		response, err = adapter.answerField(ctx, step.value)
		if err != nil {
			t.Fatalf("step %d: answerField(%q) error = %v, want nil", i, step.value, err)
		}
		if step.wantPrompt == "" {
			continue
		}
		assertPrompt(t, response, step.wantPrompt)
	}

	if adapter.wizard != nil {
		t.Fatalf("adapter.wizard = %#v, want nil — the wizard must be finished", adapter.wizard)
	}
	if adapter.editor != nil {
		t.Fatalf("adapter.editor = %#v, want nil — no flow left in progress", adapter.editor)
	}
	foundFinish := false
	for _, msg := range response.Messages {
		if text, ok := msg.(TextMessage); ok && containsAll(text.Text, "Estructura", "creada") {
			foundFinish = true
		}
	}
	if !foundFinish {
		t.Fatalf("Messages = %#v, want a final confirmation that the structure was created", response.Messages)
	}

	wantKinds := []domain.CatalogKindCode{
		domain.KindClass, domain.KindFamily, domain.KindType, domain.KindAttributeDefinition,
		domain.KindOptionSet, domain.KindOption, domain.KindUnit, domain.KindAttributeBinding,
		domain.KindPresentationField,
	}
	if len(creator.kinds) != len(wantKinds) {
		t.Fatalf("creator.kinds = %#v, want exactly 9 Create calls in step order", creator.kinds)
	}
	for i, want := range wantKinds {
		if creator.kinds[i] != want {
			t.Fatalf("creator.kinds[%d] = %q, want %q", i, creator.kinds[i], want)
		}
	}
}

// TestCatalogAdminWizardCancelAtTopLevelStepAbortsWholeWizard proves
// cancelling while the synthetic wizard-step editor itself is pending (no
// nested create in progress) clears the ENTIRE wizard, not just the current
// step.
func TestCatalogAdminWizardCancelAtTopLevelStepAbortsWholeWizard(t *testing.T) {
	lister := &fakeCatalogLister{records: map[domain.CatalogKindCode][]domain.CatalogRecord{
		domain.KindClass: {classRecord(1, "MATERIAL", "Materiales")},
	}}
	adapter := newCatalogAdminAdapter(lister, &fakeCatalogGetter{}, &fakeCatalogCreator{}, &fakeCatalogUpdater{})
	ctx := context.Background()
	if _, err := adapter.startWizard(ctx, nil); err != nil {
		t.Fatalf("startWizard() error = %v, want nil", err)
	}
	if adapter.editor == nil || !adapter.editor.wizardStep {
		t.Fatalf("adapter.editor = %#v, want the synthetic wizard-step editor pending (existing Clase found)", adapter.editor)
	}
	response, err := adapter.Respond(ctx, InteractionInput{Kind: InputCancel})
	if err != nil {
		t.Fatalf("Respond(InputCancel) error = %v, want nil", err)
	}
	if adapter.wizard != nil {
		t.Fatalf("adapter.wizard = %#v, want nil — cancelling a top-level wizard step aborts the whole wizard", adapter.wizard)
	}
	if adapter.editor != nil {
		t.Fatalf("adapter.editor = %#v, want nil", adapter.editor)
	}
	if !containsAll(response.Messages[0].(TextMessage).Text, "canceló", "estructura") {
		t.Fatalf("Messages[0] = %#v, want a wizard-specific cancellation note", response.Messages[0])
	}
}

// TestCatalogAdminWizardCancelDuringNestedCreateKeepsWizardAlive proves
// cancelling a NESTED create sub-flow started from within a wizard step
// resumes the wizard step's own question (mirroring task 8.1's "cancel is
// local" precedent) instead of aborting the whole wizard.
func TestCatalogAdminWizardCancelDuringNestedCreateKeepsWizardAlive(t *testing.T) {
	adapter := newCatalogAdminAdapter(&fakeCatalogLister{}, &fakeCatalogGetter{}, &fakeCatalogCreator{}, &fakeCatalogUpdater{})
	ctx := context.Background()
	if _, err := adapter.startWizard(ctx, nil); err != nil {
		t.Fatalf("startWizard() error = %v, want nil", err)
	}
	if adapter.editor == nil || adapter.editor.def.Code != domain.KindClass {
		t.Fatalf("adapter.editor = %#v, want the nested KindClass create in progress", adapter.editor)
	}
	response, err := adapter.Respond(ctx, InteractionInput{Kind: InputCancel})
	if err != nil {
		t.Fatalf("Respond(InputCancel) error = %v, want nil", err)
	}
	if adapter.wizard == nil {
		t.Fatalf("adapter.wizard = nil, want the wizard to survive a nested-create cancel")
	}
	if adapter.editor == nil || !adapter.editor.wizardStep {
		t.Fatalf("adapter.editor = %#v, want the synthetic wizard-step editor resumed", adapter.editor)
	}
	if !containsAll(response.Messages[0].(TextMessage).Text, "canceló") {
		t.Fatalf("Messages[0] = %#v, want a cancellation note", response.Messages[0])
	}
}

func assertPrompt(t *testing.T, response InteractionResponse, want string) {
	t.Helper()
	question, ok := response.Pending.(QuestionRequest)
	if !ok || question.Prompt != want {
		t.Fatalf("Pending = %#v, want Prompt %q", response.Pending, want)
	}
}
