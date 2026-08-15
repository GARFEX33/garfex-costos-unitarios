package tui

import (
	"context"
	"errors"
	"testing"

	"github.com/GARFEX33/garfex-costos-unitarios/internal/domain"
)

// fakeCatalogLister is the fake catalogLister used by the tests in this
// file — mirrors resources_workspace_adapter_test.go's fakeResourceSearcher
// precedent.
type fakeCatalogLister struct {
	records    map[domain.CatalogKindCode][]domain.CatalogRecord
	err        error
	lastKind   domain.CatalogKindCode
	lastFilter domain.CatalogFilter
	callCount  int
}

func (f *fakeCatalogLister) List(_ context.Context, kind domain.CatalogKindCode, filter domain.CatalogFilter) ([]domain.CatalogRecord, error) {
	f.lastKind, f.lastFilter = kind, filter
	f.callCount++
	if f.err != nil {
		return nil, f.err
	}
	return f.records[kind], nil
}

type fakeCatalogGetter struct {
	records map[int64]domain.CatalogRecord
	err     error
}

func (f *fakeCatalogGetter) Get(_ context.Context, _ domain.CatalogKindCode, id int64) (domain.CatalogRecord, error) {
	if f.err != nil {
		return domain.CatalogRecord{}, f.err
	}
	rec, ok := f.records[id]
	if !ok {
		return domain.CatalogRecord{}, errors.New("record not found")
	}
	return rec, nil
}

type fakeCatalogCreator struct {
	calls  []domain.CatalogRecord
	kinds  []domain.CatalogKindCode
	nextID int64
	err    error
}

func (f *fakeCatalogCreator) Create(_ context.Context, kind domain.CatalogKindCode, rec domain.CatalogRecord) (domain.CatalogRecord, error) {
	f.calls = append(f.calls, rec)
	f.kinds = append(f.kinds, kind)
	if f.err != nil {
		return domain.CatalogRecord{}, f.err
	}
	rec.ID = f.nextID
	rec.Kind = kind
	return rec, nil
}

type fakeCatalogUpdater struct {
	calls []domain.CatalogRecord
	kinds []domain.CatalogKindCode
	err   error
}

func (f *fakeCatalogUpdater) Update(_ context.Context, kind domain.CatalogKindCode, rec domain.CatalogRecord) (domain.CatalogRecord, error) {
	f.calls = append(f.calls, rec)
	f.kinds = append(f.kinds, kind)
	if f.err != nil {
		return domain.CatalogRecord{}, f.err
	}
	rec.Kind = kind
	return rec, nil
}

// newCatalogAdminAdapter builds a CatalogAdminAdapter against the real
// production registry (domain.NewCatalogRegistry()) for the tests below —
// mirrors resources_workspace_adapter_test.go's newDispatchAdapter.
func newCatalogAdminAdapter(lister catalogLister, getter catalogGetter, creator catalogRecordCreator, updater catalogRecordUpdater) *CatalogAdminAdapter {
	return NewCatalogAdminAdapter(lister, getter, creator, updater, domain.NewCatalogRegistry())
}

func classRecord(id int64, code, name string) domain.CatalogRecord {
	return domain.CatalogRecord{
		Kind: domain.KindClass, ID: id, Active: true,
		Values: map[string]domain.CatalogValue{
			"code": {Text: code}, "name": {Text: name}, "plural": {Text: name + "s"}, "slug": {Text: code},
		},
	}
}

func TestCatalogAdminAdapterGreeting(t *testing.T) {
	adapter := newCatalogAdminAdapter(&fakeCatalogLister{}, &fakeCatalogGetter{}, &fakeCatalogCreator{}, &fakeCatalogUpdater{})
	greeting, ok := adapter.Greeting().(TextMessage)
	if !ok {
		t.Fatalf("Greeting() = %T, want TextMessage", adapter.Greeting())
	}
	if !containsAll(greeting.Text, "Configuración", "catálogo") {
		t.Fatalf("Greeting() text = %q, want it to mention Configuración/catálogo", greeting.Text)
	}
}

func TestCatalogAdminRespondUnknownInputFallsBackToGreeting(t *testing.T) {
	adapter := newCatalogAdminAdapter(&fakeCatalogLister{}, &fakeCatalogGetter{}, &fakeCatalogCreator{}, &fakeCatalogUpdater{})
	response, err := adapter.Respond(context.Background(), InteractionInput{Kind: InputText, Value: "hola"})
	if err != nil {
		t.Fatalf("Respond() error = %v, want nil", err)
	}
	if len(response.Messages) != 1 {
		t.Fatalf("Respond() messages = %v, want exactly one", response.Messages)
	}
	if _, ok := response.Messages[0].(TextMessage); !ok {
		t.Fatalf("Respond() message = %T, want TextMessage", response.Messages[0])
	}
}

// TestCatalogAdminFieldQuestionRendersSpanishLabels proves fieldQuestion's
// Prompt is always the FieldDescriptor's own Spanish Label ("Código",
// "Nombre"), never the raw storage key ("code", "name").
func TestCatalogAdminFieldQuestionRendersSpanishLabels(t *testing.T) {
	adapter := newCatalogAdminAdapter(&fakeCatalogLister{}, &fakeCatalogGetter{}, &fakeCatalogCreator{}, &fakeCatalogUpdater{})
	response, err := adapter.startCreateFlow(context.Background(), domain.KindClass)
	if err != nil {
		t.Fatalf("startCreateFlow() error = %v, want nil", err)
	}
	question, ok := response.Pending.(QuestionRequest)
	if !ok {
		t.Fatalf("Pending = %T, want QuestionRequest", response.Pending)
	}
	if question.Prompt != "Código" {
		t.Fatalf("first field Prompt = %q, want %q", question.Prompt, "Código")
	}
}

// TestCatalogAdminSameDispatchServesTwoStructurallyDifferentKinds is the
// spec-required proof (Generic Descriptor-Driven Engine, "shared engine path
// across two kinds"): KindClass (a top-level kind) and KindFamily (a child
// kind scoped by a parent ref) are both driven end-to-end by the exact same
// startCreateFlow/fieldQuestion/answerField functions — never a per-kind
// branch — and each produces a Create call carrying its own kind's data.
func TestCatalogAdminSameDispatchServesTwoStructurallyDifferentKinds(t *testing.T) {
	creator := &fakeCatalogCreator{}
	lister := &fakeCatalogLister{records: map[domain.CatalogKindCode][]domain.CatalogRecord{
		domain.KindClass: {classRecord(1, "MATERIAL", "Materiales")},
	}}
	adapter := newCatalogAdminAdapter(lister, &fakeCatalogGetter{}, creator, &fakeCatalogUpdater{})
	ctx := context.Background()

	// KindClass: code, name, plural, slug, order, aliases, keywords.
	response, err := adapter.startCreateFlow(ctx, domain.KindClass)
	if err != nil {
		t.Fatalf("startCreateFlow(KindClass) error = %v", err)
	}
	for _, answer := range []string{"HERRAMIENTA", "Herramientas Manuales", "Herramientas Manuales", "herramientas", "5", "", ""} {
		response, err = adapter.answerField(ctx, answer)
		if err != nil {
			t.Fatalf("answerField(%q) error = %v", answer, err)
		}
	}
	if _, ok := response.Messages[0].(StructuredResult); !ok {
		t.Fatalf("final Class response = %#v, want a StructuredResult (create completed)", response)
	}

	// KindFamily: class (ref), code, name.
	response, err = adapter.startCreateFlow(ctx, domain.KindFamily)
	if err != nil {
		t.Fatalf("startCreateFlow(KindFamily) error = %v", err)
	}
	for _, answer := range []string{"MATERIAL", "CONDUCTORES", "Conductores"} {
		response, err = adapter.answerField(ctx, answer)
		if err != nil {
			t.Fatalf("answerField(%q) error = %v", answer, err)
		}
	}
	if _, ok := response.Messages[0].(StructuredResult); !ok {
		t.Fatalf("final Family response = %#v, want a StructuredResult (create completed)", response)
	}

	if len(creator.calls) != 2 {
		t.Fatalf("creator.calls = %d, want 2 (one per kind, same dispatch functions)", len(creator.calls))
	}
	if creator.kinds[0] != domain.KindClass || creator.calls[0].Values["code"].Text != "HERRAMIENTA" {
		t.Fatalf("first Create call = kind=%v values=%#v, want KindClass with code HERRAMIENTA", creator.kinds[0], creator.calls[0].Values)
	}
	if creator.kinds[1] != domain.KindFamily || creator.calls[1].Values["code"].Text != "CONDUCTORES" {
		t.Fatalf("second Create call = kind=%v values=%#v, want KindFamily with code CONDUCTORES", creator.kinds[1], creator.calls[1].Values)
	}
	if creator.calls[1].Values["class"].Ref != (domain.CatalogRef{Kind: domain.KindClass, Code: "MATERIAL"}) {
		t.Fatalf("Family's class ref = %#v, want CatalogRef{KindClass, MATERIAL}", creator.calls[1].Values["class"].Ref)
	}
}

// TestCatalogAdminAllowCreateFieldEmitsSearchableWithAllowCustom proves an
// AllowCreate-flagged FieldRef field (e.g. Opción's "optionSet") emits
// SelectionSearchable + AllowCustom=true (design §5/D12).
func TestCatalogAdminAllowCreateFieldEmitsSearchableWithAllowCustom(t *testing.T) {
	lister := &fakeCatalogLister{}
	adapter := newCatalogAdminAdapter(lister, &fakeCatalogGetter{}, &fakeCatalogCreator{}, &fakeCatalogUpdater{})
	response, err := adapter.startCreateFlow(context.Background(), domain.KindOption)
	if err != nil {
		t.Fatalf("startCreateFlow(KindOption) error = %v", err)
	}
	question, ok := response.Pending.(QuestionRequest)
	if !ok {
		t.Fatalf("Pending = %T, want QuestionRequest", response.Pending)
	}
	if question.SelectionMode != SelectionSearchable || !question.AllowCustom {
		t.Fatalf("optionSet field question = %#v, want SelectionSearchable+AllowCustom=true", question)
	}
}

// TestCatalogAdminNonAllowCreateRefFieldEmitsSearchableWithoutAllowCustom
// proves a non-AllowCreate FieldRef field (Familia's "class") emits
// SelectionSearchable + AllowCustom=false.
func TestCatalogAdminNonAllowCreateRefFieldEmitsSearchableWithoutAllowCustom(t *testing.T) {
	lister := &fakeCatalogLister{records: map[domain.CatalogKindCode][]domain.CatalogRecord{
		domain.KindClass: {classRecord(1, "MATERIAL", "Materiales")},
	}}
	adapter := newCatalogAdminAdapter(lister, &fakeCatalogGetter{}, &fakeCatalogCreator{}, &fakeCatalogUpdater{})
	response, err := adapter.startCreateFlow(context.Background(), domain.KindFamily)
	if err != nil {
		t.Fatalf("startCreateFlow(KindFamily) error = %v", err)
	}
	question, ok := response.Pending.(QuestionRequest)
	if !ok {
		t.Fatalf("Pending = %T, want QuestionRequest", response.Pending)
	}
	if question.SelectionMode != SelectionSearchable || question.AllowCustom {
		t.Fatalf("class field question = %#v, want SelectionSearchable+AllowCustom=false", question)
	}
	if len(question.Options) != 1 || question.Options[0].Label != "Materiales" {
		t.Fatalf("class field options = %#v, want the Materiales class labeled by its Name, not its Código", question.Options)
	}
}

// TestCatalogAdminFamiliaBlockedWithoutClase proves the spec scenario
// "Familia creation blocked without Clase": zero existing Clases cancels the
// flow with a clear Spanish message, before any submission is possible.
func TestCatalogAdminFamiliaBlockedWithoutClase(t *testing.T) {
	adapter := newCatalogAdminAdapter(&fakeCatalogLister{}, &fakeCatalogGetter{}, &fakeCatalogCreator{}, &fakeCatalogUpdater{})
	response, err := adapter.startCreateFlow(context.Background(), domain.KindFamily)
	if err != nil {
		t.Fatalf("startCreateFlow(KindFamily) error = %v", err)
	}
	if response.Pending != nil {
		t.Fatalf("Pending = %#v, want nil (the flow must be cancelled, not left waiting on an impossible answer)", response.Pending)
	}
	message, ok := response.Messages[0].(ErrorMessage)
	if !ok {
		t.Fatalf("message = %T, want ErrorMessage", response.Messages[0])
	}
	if !containsAll(message.Text, "clases") {
		t.Fatalf("message = %q, want it to explain no Clases are available", message.Text)
	}
	if adapter.editor != nil {
		t.Fatal("adapter.editor != nil, want the blocked flow to leave no in-progress editor")
	}
}

// TestCatalogAdminTipoBlockedWithoutFamilia proves the SAME generic
// zero-required-ref-options guard also blocks Tipo creation without a
// Familia (spec: "Tipo creation blocked without Familia") — the identical
// code path as the Familia/Clase case above, not a second implementation.
func TestCatalogAdminTipoBlockedWithoutFamilia(t *testing.T) {
	lister := &fakeCatalogLister{records: map[domain.CatalogKindCode][]domain.CatalogRecord{
		domain.KindClass: {classRecord(1, "MATERIAL", "Materiales")},
	}}
	adapter := newCatalogAdminAdapter(lister, &fakeCatalogGetter{}, &fakeCatalogCreator{}, &fakeCatalogUpdater{})
	ctx := context.Background()
	response, err := adapter.startCreateFlow(ctx, domain.KindType)
	if err != nil {
		t.Fatalf("startCreateFlow(KindType) error = %v", err)
	}
	// Type's first field is "class" (has options); answer it, then "family"
	// has zero options (no Familia registered under MATERIAL) and must
	// block.
	response, err = adapter.answerField(ctx, "MATERIAL")
	if err != nil {
		t.Fatalf("answerField(class) error = %v", err)
	}
	if response.Pending != nil {
		t.Fatalf("Pending = %#v, want nil (blocked: no Familias under MATERIAL)", response.Pending)
	}
	if adapter.editor != nil {
		t.Fatal("adapter.editor != nil, want the blocked flow to leave no in-progress editor")
	}
}

// TestCatalogAdminCreateRejectsMissingRequiredField proves a required field
// left empty re-asks the SAME field with a Spanish error, never silently
// advancing.
func TestCatalogAdminCreateRejectsMissingRequiredField(t *testing.T) {
	adapter := newCatalogAdminAdapter(&fakeCatalogLister{}, &fakeCatalogGetter{}, &fakeCatalogCreator{}, &fakeCatalogUpdater{})
	ctx := context.Background()
	if _, err := adapter.startCreateFlow(ctx, domain.KindClass); err != nil {
		t.Fatalf("startCreateFlow() error = %v", err)
	}
	response, err := adapter.answerField(ctx, "   ")
	if err != nil {
		t.Fatalf("answerField() error = %v", err)
	}
	message, ok := response.Messages[0].(ErrorMessage)
	if !ok {
		t.Fatalf("message = %T, want ErrorMessage", response.Messages[0])
	}
	if !containsAll(message.Text, "Código", "obligatorio") {
		t.Fatalf("message = %q, want it to name the missing required field", message.Text)
	}
	question, ok := response.Pending.(QuestionRequest)
	if !ok || question.Prompt != "Código" {
		t.Fatalf("Pending = %#v, want the SAME Código question re-asked", response.Pending)
	}
	if adapter.editor.step != 0 {
		t.Fatalf("editor.step = %d, want 0 (must not advance on a rejected answer)", adapter.editor.step)
	}
}

// TestCatalogAdminEditFlowPrefillsCurrentValueAsOption proves
// catalogEditorEdit seeds a "Mantener actual" Option so accepting the
// current value does not require retyping it.
func TestCatalogAdminEditFlowPrefillsCurrentValueAsOption(t *testing.T) {
	getter := &fakeCatalogGetter{records: map[int64]domain.CatalogRecord{
		7: classRecord(7, "MATERIAL", "Materiales"),
	}}
	adapter := newCatalogAdminAdapter(&fakeCatalogLister{}, getter, &fakeCatalogCreator{}, &fakeCatalogUpdater{})
	response, err := adapter.openRecordForEdit(context.Background(), domain.KindClass, "7")
	if err != nil {
		t.Fatalf("openRecordForEdit() error = %v", err)
	}
	question, ok := response.Pending.(QuestionRequest)
	if !ok {
		t.Fatalf("Pending = %T, want QuestionRequest", response.Pending)
	}
	if len(question.Options) != 1 || question.Options[0].Value != "MATERIAL" {
		t.Fatalf("edit code question options = %#v, want one option carrying the current value MATERIAL", question.Options)
	}
	if adapter.editor.mode != catalogEditorEdit || adapter.editor.id != 7 {
		t.Fatalf("editor = %#v, want mode=catalogEditorEdit id=7", adapter.editor)
	}
}

// TestCatalogAdminUpdateCallsUpdaterWithExistingID proves finishing an edit
// flow calls Update (not Create) carrying the original record's ID.
func TestCatalogAdminUpdateCallsUpdaterWithExistingID(t *testing.T) {
	getter := &fakeCatalogGetter{records: map[int64]domain.CatalogRecord{
		7: classRecord(7, "MATERIAL", "Materiales"),
	}}
	updater := &fakeCatalogUpdater{}
	creator := &fakeCatalogCreator{}
	adapter := newCatalogAdminAdapter(&fakeCatalogLister{}, getter, creator, updater)
	ctx := context.Background()
	if _, err := adapter.openRecordForEdit(ctx, domain.KindClass, "7"); err != nil {
		t.Fatalf("openRecordForEdit() error = %v", err)
	}
	var response InteractionResponse
	var err error
	for _, answer := range []string{"MATERIAL", "Materiales", "Materiales", "materiales", "", "", ""} {
		response, err = adapter.answerField(ctx, answer)
		if err != nil {
			t.Fatalf("answerField(%q) error = %v", answer, err)
		}
	}
	if len(updater.calls) != 1 {
		t.Fatalf("updater.calls = %d, want 1", len(updater.calls))
	}
	if updater.calls[0].ID != 7 {
		t.Fatalf("updater.calls[0].ID = %d, want 7", updater.calls[0].ID)
	}
	if len(creator.calls) != 0 {
		t.Fatalf("creator.calls = %d, want 0 (edit must never call Create)", len(creator.calls))
	}
	if _, ok := response.Messages[0].(StructuredResult); !ok {
		t.Fatalf("final response = %#v, want StructuredResult", response)
	}
}

// TestCatalogAdminCreateErrorSurfacesSpanishMessage proves a repository/
// service error is mapped to a Spanish message, never a raw Go error string.
func TestCatalogAdminCreateErrorSurfacesSpanishMessage(t *testing.T) {
	creator := &fakeCatalogCreator{err: domain.ErrCatalogDuplicate}
	adapter := newCatalogAdminAdapter(&fakeCatalogLister{}, &fakeCatalogGetter{}, creator, &fakeCatalogUpdater{})
	ctx := context.Background()
	if _, err := adapter.startCreateFlow(ctx, domain.KindClass); err != nil {
		t.Fatalf("startCreateFlow() error = %v", err)
	}
	var response InteractionResponse
	var err error
	for _, answer := range []string{"MATERIAL", "Materiales", "Materiales", "materiales", "", "", ""} {
		response, err = adapter.answerField(ctx, answer)
		if err != nil {
			t.Fatalf("answerField(%q) error = %v", answer, err)
		}
	}
	message, ok := response.Messages[0].(ErrorMessage)
	if !ok {
		t.Fatalf("message = %T, want ErrorMessage", response.Messages[0])
	}
	if !containsAll(message.Text, "Ya existe") {
		t.Fatalf("message = %q, want the Spanish duplicate-code message", message.Text)
	}
	if adapter.editor != nil {
		t.Fatal("adapter.editor != nil, want the flow to end on error")
	}
}

// TestCatalogAdminCancelClearsEditor proves InputCancel ends an in-progress
// flow with a Spanish confirmation, leaving no dangling editor state.
func TestCatalogAdminCancelClearsEditor(t *testing.T) {
	adapter := newCatalogAdminAdapter(&fakeCatalogLister{}, &fakeCatalogGetter{}, &fakeCatalogCreator{}, &fakeCatalogUpdater{})
	ctx := context.Background()
	if _, err := adapter.startCreateFlow(ctx, domain.KindClass); err != nil {
		t.Fatalf("startCreateFlow() error = %v", err)
	}
	response, err := adapter.Respond(ctx, InteractionInput{Kind: InputCancel})
	if err != nil {
		t.Fatalf("Respond(InputCancel) error = %v", err)
	}
	if !containsAll(response.Messages[0].(TextMessage).Text, "canceló") {
		t.Fatalf("message = %#v, want a cancellation message", response.Messages[0])
	}
	if adapter.editor != nil {
		t.Fatal("adapter.editor != nil after cancel, want nil")
	}
}

// TestCatalogAdminOpenKindMenuListsExistingRecordsPlusCreateOption proves
// startKindMenu lists existing records (labeled by their Name, never their
// raw Código) alongside a "crear nueva/o" option.
func TestCatalogAdminOpenKindMenuListsExistingRecordsPlusCreateOption(t *testing.T) {
	lister := &fakeCatalogLister{records: map[domain.CatalogKindCode][]domain.CatalogRecord{
		domain.KindClass: {classRecord(1, "MATERIAL", "Materiales"), classRecord(2, "MANO_DE_OBRA", "Mano de obra")},
	}}
	adapter := newCatalogAdminAdapter(lister, &fakeCatalogGetter{}, &fakeCatalogCreator{}, &fakeCatalogUpdater{})
	response, err := adapter.Respond(context.Background(), InteractionInput{Kind: InputAction, ActionID: catalogOpenActionID(domain.KindClass)})
	if err != nil {
		t.Fatalf("Respond() error = %v", err)
	}
	question, ok := response.Pending.(QuestionRequest)
	if !ok {
		t.Fatalf("Pending = %T, want QuestionRequest", response.Pending)
	}
	if len(question.Options) != 3 {
		t.Fatalf("len(Options) = %d, want 3 (create-new + 2 existing)", len(question.Options))
	}
	if question.Options[0].Value != catalogCreateNewOptionID {
		t.Fatalf("Options[0] = %#v, want the create-new option first", question.Options[0])
	}
	for _, opt := range question.Options[1:] {
		if opt.Label == "MATERIAL" || opt.Label == "MANO_DE_OBRA" {
			t.Fatalf("option label = %q, must never render the raw Código", opt.Label)
		}
	}
	if adapter.activeKind != domain.KindClass {
		t.Fatalf("adapter.activeKind = %q, want KindClass", adapter.activeKind)
	}
}

// TestCatalogAdminSelectExistingRecordStartsEditFlow proves picking an
// existing record from startKindMenu's answer opens the edit flow for it.
func TestCatalogAdminSelectExistingRecordStartsEditFlow(t *testing.T) {
	lister := &fakeCatalogLister{records: map[domain.CatalogKindCode][]domain.CatalogRecord{
		domain.KindClass: {classRecord(1, "MATERIAL", "Materiales")},
	}}
	getter := &fakeCatalogGetter{records: map[int64]domain.CatalogRecord{1: classRecord(1, "MATERIAL", "Materiales")}}
	adapter := newCatalogAdminAdapter(lister, getter, &fakeCatalogCreator{}, &fakeCatalogUpdater{})
	ctx := context.Background()
	if _, err := adapter.Respond(ctx, InteractionInput{Kind: InputAction, ActionID: catalogOpenActionID(domain.KindClass)}); err != nil {
		t.Fatalf("Respond(open) error = %v", err)
	}
	response, err := adapter.Respond(ctx, InteractionInput{Kind: InputSelection, Key: catalogKindMenuKey, Value: "1"})
	if err != nil {
		t.Fatalf("Respond(select) error = %v", err)
	}
	if response.Pending == nil {
		t.Fatal("Pending = nil, want the edit flow's first field question")
	}
	if adapter.editor == nil || adapter.editor.mode != catalogEditorEdit {
		t.Fatalf("adapter.editor = %#v, want an in-progress catalogEditorEdit", adapter.editor)
	}
}

// TestCatalogAdminIdentityLeafOpensAttributeBindingKind proves the
// TUI-layer-only "Identidad" routing sentinel opens the exact same
// KindAttributeBinding flow as the "Aplicabilidad" leaf.
func TestCatalogAdminIdentityLeafOpensAttributeBindingKind(t *testing.T) {
	adapter := newCatalogAdminAdapter(&fakeCatalogLister{records: map[domain.CatalogKindCode][]domain.CatalogRecord{
		domain.KindClass: {classRecord(1, "MATERIAL", "Materiales")},
	}}, &fakeCatalogGetter{}, &fakeCatalogCreator{}, &fakeCatalogUpdater{})
	response, err := adapter.Respond(context.Background(), InteractionInput{Kind: InputAction, ActionID: catalogIdentityActionID})
	if err != nil {
		t.Fatalf("Respond() error = %v", err)
	}
	if response.Pending == nil {
		t.Fatal("Pending = nil, want the Aplicabilidad kind menu")
	}
	if adapter.activeKind != domain.KindAttributeBinding {
		t.Fatalf("adapter.activeKind = %q, want KindAttributeBinding", adapter.activeKind)
	}
}
