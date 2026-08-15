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

// fakeCatalogDependencyChecker is the fake catalogDependencyChecker used by
// the guarded-delete tests (task 7.1) — returns zero dependents by default,
// so the default newCatalogAdminAdapter helper never blocks a delete unless
// a test explicitly overrides it.
type fakeCatalogDependencyChecker struct {
	deps []domain.CatalogDependency
	err  error
}

func (f *fakeCatalogDependencyChecker) Dependencies(_ context.Context, _ domain.CatalogKindCode, _ int64) ([]domain.CatalogDependency, error) {
	return f.deps, f.err
}

// fakeCatalogReferenceChecker is the fake catalogReferenceChecker used by
// the Código-immutability-UI tests (task 7.2) and the guarded-delete tests
// (task 7.1) — reports "not referenced" by default.
type fakeCatalogReferenceChecker struct {
	referenced bool
	err        error
}

func (f *fakeCatalogReferenceChecker) ReferencedByResources(_ context.Context, _ domain.CatalogKindCode, _ int64) (bool, error) {
	return f.referenced, f.err
}

// fakeCatalogDeactivator/fakeCatalogReactivator/fakeCatalogDeleter are the
// fake catalogDeactivator/catalogReactivator/catalogDeleter used by the
// lifecycle-action tests (task 7.1) — succeed by default, recording every
// call's id.
type fakeCatalogDeactivator struct {
	calls []int64
	err   error
}

func (f *fakeCatalogDeactivator) Deactivate(_ context.Context, _ domain.CatalogKindCode, id int64) error {
	f.calls = append(f.calls, id)
	return f.err
}

type fakeCatalogReactivator struct {
	calls []int64
	err   error
}

func (f *fakeCatalogReactivator) Reactivate(_ context.Context, _ domain.CatalogKindCode, id int64) error {
	f.calls = append(f.calls, id)
	return f.err
}

type fakeCatalogDeleter struct {
	calls []int64
	err   error
}

func (f *fakeCatalogDeleter) Delete(_ context.Context, _ domain.CatalogKindCode, id int64) error {
	f.calls = append(f.calls, id)
	return f.err
}

// newCatalogAdminAdapter builds a CatalogAdminAdapter against the real
// production registry (domain.NewCatalogRegistry()) for the tests below —
// mirrors resources_workspace_adapter_test.go's newDispatchAdapter. The five
// lifecycle dependencies (task 7.1/7.2) default to permissive fakes (zero
// dependents, not referenced, deactivate/reactivate/delete all succeed) so
// every pre-existing test in this file keeps compiling and passing unchanged
// — tests that need to exercise a specific lifecycle behavior construct
// NewCatalogAdminAdapter directly instead of using this helper.
func newCatalogAdminAdapter(lister catalogLister, getter catalogGetter, creator catalogRecordCreator, updater catalogRecordUpdater) *CatalogAdminAdapter {
	return NewCatalogAdminAdapter(lister, getter, creator, updater,
		&fakeCatalogDependencyChecker{}, &fakeCatalogReferenceChecker{}, &fakeCatalogDeactivator{}, &fakeCatalogReactivator{}, &fakeCatalogDeleter{},
		domain.NewCatalogRegistry())
}

func classRecord(id int64, code, name string) domain.CatalogRecord {
	return domain.CatalogRecord{
		Kind: domain.KindClass, ID: id, Active: true,
		Values: map[string]domain.CatalogValue{
			"code": {Text: code}, "name": {Text: name}, "plural": {Text: name + "s"}, "slug": {Text: code},
		},
	}
}

func familyRecord(id int64, classCode, code, name string) domain.CatalogRecord {
	return domain.CatalogRecord{
		Kind: domain.KindFamily, ID: id, Active: true,
		Values: map[string]domain.CatalogValue{
			"class": {Ref: domain.CatalogRef{Kind: domain.KindClass, Code: classCode}},
			"code":  {Text: code}, "name": {Text: name},
		},
	}
}

func typeRecord(id int64, classCode, familyCode, code, name string) domain.CatalogRecord {
	return domain.CatalogRecord{
		Kind: domain.KindType, ID: id, Active: true,
		Values: map[string]domain.CatalogValue{
			"class":  {Ref: domain.CatalogRef{Kind: domain.KindClass, Code: classCode}},
			"family": {Ref: domain.CatalogRef{Kind: domain.KindFamily, Code: familyCode}},
			"code":   {Text: code}, "name": {Text: name},
		},
	}
}

func definitionRecord(id int64, code, name string) domain.CatalogRecord {
	return domain.CatalogRecord{
		Kind: domain.KindAttributeDefinition, ID: id, Active: true,
		Values: map[string]domain.CatalogValue{
			"code": {Text: code}, "name": {Text: name}, "valueType": {Text: "CONTROLLED_TEXT"},
		},
	}
}

func attributeBindingRecord(id int64, classCode, familyCode, typeCode, characteristicCode string, identityParticipates bool) domain.CatalogRecord {
	return domain.CatalogRecord{
		Kind: domain.KindAttributeBinding, ID: id, Active: true,
		Values: map[string]domain.CatalogValue{
			"class":                {Ref: domain.CatalogRef{Kind: domain.KindClass, Code: classCode}},
			"family":               {Ref: domain.CatalogRef{Kind: domain.KindFamily, Code: familyCode}},
			"type":                 {Ref: domain.CatalogRef{Kind: domain.KindType, Code: typeCode}},
			"characteristic":       {Ref: domain.CatalogRef{Kind: domain.KindAttributeDefinition, Code: characteristicCode}},
			"mode":                 {Text: "REQUIRED"},
			"identityParticipates": {Bool: identityParticipates},
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

// TestCatalogAdminSelectExistingRecordOpensDetailWithActions proves picking
// an existing record from startKindMenu's answer opens a detail view (task
// 7.1: mirrors resources_workspace_dispatch.go's detailResponse/
// detailActionsRequest precedent) offering lifecycle actions, rather than
// jumping straight into the edit flow — the edit flow now starts only once
// "Editar" is explicitly picked from that action menu (see the next test).
func TestCatalogAdminSelectExistingRecordOpensDetailWithActions(t *testing.T) {
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
	if adapter.editor != nil {
		t.Fatal("adapter.editor != nil, want no in-progress editor before Editar is explicitly chosen")
	}
	if _, ok := response.Messages[0].(StructuredResult); !ok {
		t.Fatalf("Messages[0] = %T, want a StructuredResult showing the record's detail", response.Messages[0])
	}
	actions, ok := response.Pending.(ActionRequest)
	if !ok {
		t.Fatalf("Pending = %T, want ActionRequest", response.Pending)
	}
	var gotEdit, gotDeactivate, gotDelete bool
	for _, action := range actions.Actions {
		switch action.ID {
		case catalogRecordEditActionID:
			gotEdit = true
		case catalogRecordDeactivateActionID:
			gotDeactivate = true
		case catalogRecordDeleteActionID:
			gotDelete = true
		}
	}
	if !gotEdit || !gotDeactivate || !gotDelete {
		t.Fatalf("actions = %#v, want Editar+Desactivar+Eliminar (Clase is SoftDelete)", actions.Actions)
	}
	if adapter.activeRecord.ID != 1 {
		t.Fatalf("adapter.activeRecord = %#v, want the opened record", adapter.activeRecord)
	}
}

// TestCatalogAdminDetailEditActionStartsEditFlow proves choosing "Editar"
// from the detail action menu opens the edit flow for the record currently
// shown.
func TestCatalogAdminDetailEditActionStartsEditFlow(t *testing.T) {
	lister := &fakeCatalogLister{records: map[domain.CatalogKindCode][]domain.CatalogRecord{
		domain.KindClass: {classRecord(1, "MATERIAL", "Materiales")},
	}}
	getter := &fakeCatalogGetter{records: map[int64]domain.CatalogRecord{1: classRecord(1, "MATERIAL", "Materiales")}}
	adapter := newCatalogAdminAdapter(lister, getter, &fakeCatalogCreator{}, &fakeCatalogUpdater{})
	ctx := context.Background()
	if _, err := adapter.Respond(ctx, InteractionInput{Kind: InputAction, ActionID: catalogOpenActionID(domain.KindClass)}); err != nil {
		t.Fatalf("Respond(open) error = %v", err)
	}
	if _, err := adapter.Respond(ctx, InteractionInput{Kind: InputSelection, Key: catalogKindMenuKey, Value: "1"}); err != nil {
		t.Fatalf("Respond(select) error = %v", err)
	}
	response, err := adapter.Respond(ctx, InteractionInput{Kind: InputAction, ActionID: catalogRecordEditActionID})
	if err != nil {
		t.Fatalf("Respond(edit) error = %v", err)
	}
	if response.Pending == nil {
		t.Fatal("Pending = nil, want the edit flow's first field question")
	}
	if adapter.editor == nil || adapter.editor.mode != catalogEditorEdit {
		t.Fatalf("adapter.editor = %#v, want an in-progress catalogEditorEdit", adapter.editor)
	}
}

// TestCatalogAdminDeleteBlockedByBlockingDependencyOffersDeactivate proves
// task 7.1: a delete blocked by a blocking CatalogDependency is rejected
// with a Spanish "está siendo utilizado por N ..." message, WITHOUT calling
// the deleter, and the same action menu (including Desactivar) is offered
// again.
func TestCatalogAdminDeleteBlockedByBlockingDependencyOffersDeactivate(t *testing.T) {
	lister := &fakeCatalogLister{}
	getter := &fakeCatalogGetter{records: map[int64]domain.CatalogRecord{1: classRecord(1, "MATERIAL", "Materiales")}}
	deps := &fakeCatalogDependencyChecker{deps: []domain.CatalogDependency{{Kind: domain.KindFamily, Count: 3, Blocking: true}}}
	deleter := &fakeCatalogDeleter{}
	adapter := NewCatalogAdminAdapter(lister, getter, &fakeCatalogCreator{}, &fakeCatalogUpdater{},
		deps, &fakeCatalogReferenceChecker{}, &fakeCatalogDeactivator{}, &fakeCatalogReactivator{}, deleter,
		domain.NewCatalogRegistry())
	ctx := context.Background()
	if _, err := adapter.openRecordDetail(ctx, domain.KindClass, "1"); err != nil {
		t.Fatalf("openRecordDetail() error = %v", err)
	}
	response, err := adapter.Respond(ctx, InteractionInput{Kind: InputAction, ActionID: catalogRecordDeleteActionID})
	if err != nil {
		t.Fatalf("Respond(delete) error = %v", err)
	}
	if len(deleter.calls) != 0 {
		t.Fatalf("deleter.calls = %v, want none — delete must be blocked before ever calling the deleter", deleter.calls)
	}
	message, ok := response.Messages[0].(ErrorMessage)
	if !ok {
		t.Fatalf("Messages[0] = %T, want ErrorMessage", response.Messages[0])
	}
	if !containsAll(message.Text, "3", "familias") {
		t.Fatalf("message = %q, want it to name the blocking count and kind", message.Text)
	}
	actions, ok := response.Pending.(ActionRequest)
	if !ok {
		t.Fatalf("Pending = %T, want the SAME action menu re-offered (Desactivar available)", response.Pending)
	}
	found := false
	for _, action := range actions.Actions {
		if action.ID == catalogRecordDeactivateActionID {
			found = true
		}
	}
	if !found {
		t.Fatalf("actions = %#v, want Desactivar offered as the alternative", actions.Actions)
	}
}

// TestCatalogAdminDeleteAllowedWhenNoDependentsAsksConfirmation proves a
// record with zero blocking dependents and no real-resource reference is NOT
// rejected outright — it shows a real delete confirmation instead (mirrors
// resources_workspace_dispatch.go's startDeleteConfirmation precedent).
func TestCatalogAdminDeleteAllowedWhenNoDependentsAsksConfirmation(t *testing.T) {
	getter := &fakeCatalogGetter{records: map[int64]domain.CatalogRecord{1: classRecord(1, "MATERIAL", "Materiales")}}
	adapter := NewCatalogAdminAdapter(&fakeCatalogLister{}, getter, &fakeCatalogCreator{}, &fakeCatalogUpdater{},
		&fakeCatalogDependencyChecker{}, &fakeCatalogReferenceChecker{}, &fakeCatalogDeactivator{}, &fakeCatalogReactivator{}, &fakeCatalogDeleter{},
		domain.NewCatalogRegistry())
	ctx := context.Background()
	if _, err := adapter.openRecordDetail(ctx, domain.KindClass, "1"); err != nil {
		t.Fatalf("openRecordDetail() error = %v", err)
	}
	response, err := adapter.Respond(ctx, InteractionInput{Kind: InputAction, ActionID: catalogRecordDeleteActionID})
	if err != nil {
		t.Fatalf("Respond(delete) error = %v", err)
	}
	confirmation, ok := response.Pending.(ConfirmationRequest)
	if !ok {
		t.Fatalf("Pending = %T, want ConfirmationRequest", response.Pending)
	}
	if confirmation.Key != catalogDeleteConfirmKey {
		t.Fatalf("confirmation.Key = %q, want %q", confirmation.Key, catalogDeleteConfirmKey)
	}
}

// TestCatalogAdminDeleteConfirmedCallsDeleter proves confirming a real
// delete calls the deleter with the record's kind+ID.
func TestCatalogAdminDeleteConfirmedCallsDeleter(t *testing.T) {
	getter := &fakeCatalogGetter{records: map[int64]domain.CatalogRecord{1: classRecord(1, "MATERIAL", "Materiales")}}
	deleter := &fakeCatalogDeleter{}
	adapter := NewCatalogAdminAdapter(&fakeCatalogLister{}, getter, &fakeCatalogCreator{}, &fakeCatalogUpdater{},
		&fakeCatalogDependencyChecker{}, &fakeCatalogReferenceChecker{}, &fakeCatalogDeactivator{}, &fakeCatalogReactivator{}, deleter,
		domain.NewCatalogRegistry())
	ctx := context.Background()
	if _, err := adapter.openRecordDetail(ctx, domain.KindClass, "1"); err != nil {
		t.Fatalf("openRecordDetail() error = %v", err)
	}
	if _, err := adapter.Respond(ctx, InteractionInput{Kind: InputAction, ActionID: catalogRecordDeleteActionID}); err != nil {
		t.Fatalf("Respond(delete) error = %v", err)
	}
	response, err := adapter.Respond(ctx, InteractionInput{Kind: InputSelection, Key: catalogDeleteConfirmKey, Value: "yes"})
	if err != nil {
		t.Fatalf("Respond(confirm) error = %v", err)
	}
	if len(deleter.calls) != 1 || deleter.calls[0] != 1 {
		t.Fatalf("deleter.calls = %v, want [1]", deleter.calls)
	}
	if _, ok := response.Messages[0].(TextMessage); !ok {
		t.Fatalf("Messages[0] = %T, want a confirmation TextMessage", response.Messages[0])
	}
}

// TestCatalogAdminDeactivateCallsDeactivator proves the Desactivar action
// calls the deactivator and refreshes the detail view.
func TestCatalogAdminDeactivateCallsDeactivator(t *testing.T) {
	getter := &fakeCatalogGetter{records: map[int64]domain.CatalogRecord{1: classRecord(1, "MATERIAL", "Materiales")}}
	deactivator := &fakeCatalogDeactivator{}
	adapter := NewCatalogAdminAdapter(&fakeCatalogLister{}, getter, &fakeCatalogCreator{}, &fakeCatalogUpdater{},
		&fakeCatalogDependencyChecker{}, &fakeCatalogReferenceChecker{}, deactivator, &fakeCatalogReactivator{}, &fakeCatalogDeleter{},
		domain.NewCatalogRegistry())
	ctx := context.Background()
	if _, err := adapter.openRecordDetail(ctx, domain.KindClass, "1"); err != nil {
		t.Fatalf("openRecordDetail() error = %v", err)
	}
	response, err := adapter.Respond(ctx, InteractionInput{Kind: InputAction, ActionID: catalogRecordDeactivateActionID})
	if err != nil {
		t.Fatalf("Respond(deactivate) error = %v", err)
	}
	if len(deactivator.calls) != 1 || deactivator.calls[0] != 1 {
		t.Fatalf("deactivator.calls = %v, want [1]", deactivator.calls)
	}
	if adapter.activeRecord.Active {
		t.Fatal("adapter.activeRecord.Active = true, want false after Desactivar")
	}
	actions, ok := response.Pending.(ActionRequest)
	if !ok {
		t.Fatalf("Pending = %T, want the refreshed action menu", response.Pending)
	}
	var gotReactivate bool
	for _, action := range actions.Actions {
		if action.ID == catalogRecordReactivateActionID {
			gotReactivate = true
		}
	}
	if !gotReactivate {
		t.Fatalf("actions = %#v, want Reactivar offered after Desactivar", actions.Actions)
	}
}

// TestCatalogAdminReactivateCallsReactivator mirrors the Deactivate test for
// Reactivar.
func TestCatalogAdminReactivateCallsReactivator(t *testing.T) {
	rec := classRecord(1, "MATERIAL", "Materiales")
	rec.Active = false
	getter := &fakeCatalogGetter{records: map[int64]domain.CatalogRecord{1: rec}}
	reactivator := &fakeCatalogReactivator{}
	adapter := NewCatalogAdminAdapter(&fakeCatalogLister{}, getter, &fakeCatalogCreator{}, &fakeCatalogUpdater{},
		&fakeCatalogDependencyChecker{}, &fakeCatalogReferenceChecker{}, &fakeCatalogDeactivator{}, reactivator, &fakeCatalogDeleter{},
		domain.NewCatalogRegistry())
	ctx := context.Background()
	if _, err := adapter.openRecordDetail(ctx, domain.KindClass, "1"); err != nil {
		t.Fatalf("openRecordDetail() error = %v", err)
	}
	if _, err := adapter.Respond(ctx, InteractionInput{Kind: InputAction, ActionID: catalogRecordReactivateActionID}); err != nil {
		t.Fatalf("Respond(reactivate) error = %v", err)
	}
	if len(reactivator.calls) != 1 || reactivator.calls[0] != 1 {
		t.Fatalf("reactivator.calls = %v, want [1]", reactivator.calls)
	}
	if !adapter.activeRecord.Active {
		t.Fatal("adapter.activeRecord.Active = false, want true after Reactivar")
	}
}

// TestCatalogAdminDeactivateNotOfferedForNonSoftDeleteKind proves the
// action menu never offers Desactivar/Reactivar for a kind whose
// CatalogKind.SoftDelete is false (e.g. Aplicabilidad) — Service.Deactivate
// would just fail with ErrSoftDeleteUnsupported for those, so the UI never
// exposes an action guaranteed to fail (Generic Descriptor-Driven Engine).
func TestCatalogAdminDeactivateNotOfferedForNonSoftDeleteKind(t *testing.T) {
	rec := attributeBindingRecord(1, "MATERIAL", "CONDUCTORES", "CABLE", "COLOR", false)
	getter := &fakeCatalogGetter{records: map[int64]domain.CatalogRecord{1: rec}}
	adapter := newCatalogAdminAdapter(&fakeCatalogLister{}, getter, &fakeCatalogCreator{}, &fakeCatalogUpdater{})
	ctx := context.Background()
	if _, err := adapter.openRecordDetail(ctx, domain.KindAttributeBinding, "1"); err != nil {
		t.Fatalf("openRecordDetail() error = %v", err)
	}
	actions := adapter.recordDetailResponse().Pending.(ActionRequest)
	for _, action := range actions.Actions {
		if action.ID == catalogRecordDeactivateActionID || action.ID == catalogRecordReactivateActionID {
			t.Fatalf("actions = %#v, must not offer Desactivar/Reactivar for a non-SoftDelete kind", actions.Actions)
		}
	}
}

// TestCatalogAdminCodeFieldSkippedWhenReferenced proves task 7.2: once a
// record's Código is referenced by real resources, editing it never asks the
// Código question — it is kept at its current value automatically and the
// flow advances straight past it, with an informational note explaining why.
// Table-driven per kind that carries a real "código" field
// (Clase/Familia/Tipo/Característica, the task's own scoping).
func TestCatalogAdminCodeFieldSkippedWhenReferenced(t *testing.T) {
	tests := []struct {
		name       string
		kind       domain.CatalogKindCode
		rec        domain.CatalogRecord
		refAnswers []string
		nextPrompt string
	}{
		{"Clase", domain.KindClass, classRecord(1, "MATERIAL", "Materiales"), nil, "Nombre"},
		{"Familia", domain.KindFamily, familyRecord(2, "MATERIAL", "CONDUCTORES", "Conductores"), []string{"MATERIAL"}, "Nombre"},
		{"Tipo", domain.KindType, typeRecord(3, "MATERIAL", "CONDUCTORES", "CABLE", "Cable"), []string{"MATERIAL", "CONDUCTORES"}, "Nombre"},
		{"Característica", domain.KindAttributeDefinition, definitionRecord(4, "COLOR", "Color"), nil, "Nombre"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			lister := &fakeCatalogLister{records: map[domain.CatalogKindCode][]domain.CatalogRecord{
				domain.KindClass:  {classRecord(1, "MATERIAL", "Materiales")},
				domain.KindFamily: {familyRecord(2, "MATERIAL", "CONDUCTORES", "Conductores")},
			}}
			getter := &fakeCatalogGetter{records: map[int64]domain.CatalogRecord{tc.rec.ID: tc.rec}}
			adapter := NewCatalogAdminAdapter(lister, getter, &fakeCatalogCreator{}, &fakeCatalogUpdater{},
				&fakeCatalogDependencyChecker{}, &fakeCatalogReferenceChecker{referenced: true},
				&fakeCatalogDeactivator{}, &fakeCatalogReactivator{}, &fakeCatalogDeleter{}, domain.NewCatalogRegistry())
			ctx := context.Background()
			response, err := adapter.startEditFlow(ctx, tc.kind, tc.rec)
			if err != nil {
				t.Fatalf("startEditFlow() error = %v", err)
			}
			for _, answer := range tc.refAnswers {
				response, err = adapter.answerField(ctx, answer)
				if err != nil {
					t.Fatalf("answerField(%q) error = %v", answer, err)
				}
			}
			question, ok := response.Pending.(QuestionRequest)
			if !ok {
				t.Fatalf("Pending = %T, want QuestionRequest", response.Pending)
			}
			if question.Prompt != tc.nextPrompt {
				t.Fatalf("Prompt = %q, want %q (Código must be skipped once referenced)", question.Prompt, tc.nextPrompt)
			}
			foundNote := false
			for _, msg := range response.Messages {
				if text, ok := msg.(TextMessage); ok && containsAll(text.Text, "Código") {
					foundNote = true
				}
			}
			if !foundNote {
				t.Fatalf("Messages = %#v, want a note mentioning Código is locked", response.Messages)
			}
		})
	}
}

// TestCatalogAdminCodeFieldAskedWhenNotReferenced is
// TestCatalogAdminCodeFieldSkippedWhenReferenced's contrast: an
// unreferenced record's Código question is asked normally.
func TestCatalogAdminCodeFieldAskedWhenNotReferenced(t *testing.T) {
	getter := &fakeCatalogGetter{records: map[int64]domain.CatalogRecord{1: classRecord(1, "MATERIAL", "Materiales")}}
	adapter := newCatalogAdminAdapter(&fakeCatalogLister{}, getter, &fakeCatalogCreator{}, &fakeCatalogUpdater{})
	response, err := adapter.startEditFlow(context.Background(), domain.KindClass, classRecord(1, "MATERIAL", "Materiales"))
	if err != nil {
		t.Fatalf("startEditFlow() error = %v", err)
	}
	question, ok := response.Pending.(QuestionRequest)
	if !ok || question.Prompt != "Código" {
		t.Fatalf("Pending = %#v, want the Código question asked normally", response.Pending)
	}
}

// TestCatalogAdminIdentidadChangeShowsWarning proves task 7.4 (the spec gap
// task): saving an Aplicabilidad edit that changes identityParticipates
// shows a Spanish warning that existing resources keep their current
// IdentityKey, and only new resources use the updated rule.
func TestCatalogAdminIdentidadChangeShowsWarning(t *testing.T) {
	rec := attributeBindingRecord(9, "MATERIAL", "CONDUCTORES", "CABLE", "COLOR", false)
	lister := &fakeCatalogLister{records: map[domain.CatalogKindCode][]domain.CatalogRecord{
		domain.KindClass:               {classRecord(1, "MATERIAL", "Materiales")},
		domain.KindFamily:              {familyRecord(2, "MATERIAL", "CONDUCTORES", "Conductores")},
		domain.KindType:                {typeRecord(3, "MATERIAL", "CONDUCTORES", "CABLE", "Cable")},
		domain.KindAttributeDefinition: {definitionRecord(4, "COLOR", "Color")},
	}}
	getter := &fakeCatalogGetter{records: map[int64]domain.CatalogRecord{9: rec}}
	adapter := newCatalogAdminAdapter(lister, getter, &fakeCatalogCreator{}, &fakeCatalogUpdater{})
	ctx := context.Background()
	response, err := adapter.startEditFlow(ctx, domain.KindAttributeBinding, rec)
	if err != nil {
		t.Fatalf("startEditFlow() error = %v", err)
	}
	// Field order: class, family, type, characteristic, optionSet, mode,
	// identityParticipates (catalog_kind.go). Flip identityParticipates
	// false -> true.
	answers := []string{"MATERIAL", "CONDUCTORES", "CABLE", "COLOR", "", "REQUIRED", "true"}
	for _, answer := range answers {
		response, err = adapter.answerField(ctx, answer)
		if err != nil {
			t.Fatalf("answerField(%q) error = %v", answer, err)
		}
	}
	found := false
	for _, msg := range response.Messages {
		if text, ok := msg.(TextMessage); ok && containsAll(text.Text, "Identidad") {
			found = true
		}
	}
	if !found {
		t.Fatalf("Messages = %#v, want the Identidad-change warning", response.Messages)
	}
}

// TestCatalogAdminIdentidadUnchangedShowsNoWarning is the contrast: saving
// an Aplicabilidad edit that does NOT change identityParticipates shows no
// warning.
func TestCatalogAdminIdentidadUnchangedShowsNoWarning(t *testing.T) {
	rec := attributeBindingRecord(9, "MATERIAL", "CONDUCTORES", "CABLE", "COLOR", true)
	lister := &fakeCatalogLister{records: map[domain.CatalogKindCode][]domain.CatalogRecord{
		domain.KindClass:               {classRecord(1, "MATERIAL", "Materiales")},
		domain.KindFamily:              {familyRecord(2, "MATERIAL", "CONDUCTORES", "Conductores")},
		domain.KindType:                {typeRecord(3, "MATERIAL", "CONDUCTORES", "CABLE", "Cable")},
		domain.KindAttributeDefinition: {definitionRecord(4, "COLOR", "Color")},
	}}
	getter := &fakeCatalogGetter{records: map[int64]domain.CatalogRecord{9: rec}}
	adapter := newCatalogAdminAdapter(lister, getter, &fakeCatalogCreator{}, &fakeCatalogUpdater{})
	ctx := context.Background()
	response, err := adapter.startEditFlow(ctx, domain.KindAttributeBinding, rec)
	if err != nil {
		t.Fatalf("startEditFlow() error = %v", err)
	}
	answers := []string{"MATERIAL", "CONDUCTORES", "CABLE", "COLOR", "", "REQUIRED", "true"}
	for _, answer := range answers {
		response, err = adapter.answerField(ctx, answer)
		if err != nil {
			t.Fatalf("answerField(%q) error = %v", answer, err)
		}
	}
	for _, msg := range response.Messages {
		if text, ok := msg.(TextMessage); ok && containsAll(text.Text, "Identidad") {
			t.Fatalf("Messages = %#v, want no Identidad warning (identityParticipates unchanged)", response.Messages)
		}
	}
}

// TestCatalogAdminCustomAnswerOnAllowCreateFieldStartsNestedCreateFlow proves
// task 8.1: answering an AllowCreate FieldRef field with text that matches NO
// existing field.RefKind record pushes a NESTED create sub-flow for that
// RefKind, pausing (not discarding) the parent flow. KindOption's "optionSet"
// field (AllowCreate=true, RefKind=KindOptionSet) is the target — no
// Conjuntos de Opciones exist yet, so any typed text is genuinely custom.
func TestCatalogAdminCustomAnswerOnAllowCreateFieldStartsNestedCreateFlow(t *testing.T) {
	adapter := newCatalogAdminAdapter(&fakeCatalogLister{}, &fakeCatalogGetter{}, &fakeCatalogCreator{}, &fakeCatalogUpdater{})
	ctx := context.Background()
	if _, err := adapter.startCreateFlow(ctx, domain.KindOption); err != nil {
		t.Fatalf("startCreateFlow(KindOption) error = %v", err)
	}
	response, err := adapter.answerField(ctx, "Calibre AWG")
	if err != nil {
		t.Fatalf("answerField(%q) error = %v", "Calibre AWG", err)
	}
	if adapter.editor == nil || adapter.editor.def.Code != domain.KindOptionSet {
		t.Fatalf("adapter.editor = %#v, want an in-progress KindOptionSet nested flow", adapter.editor)
	}
	if adapter.editor.parent == nil || adapter.editor.parent.def.Code != domain.KindOption {
		t.Fatalf("adapter.editor.parent = %#v, want the paused KindOption parent flow", adapter.editor.parent)
	}
	if adapter.editor.resumeField != "optionSet" {
		t.Fatalf("adapter.editor.resumeField = %q, want %q", adapter.editor.resumeField, "optionSet")
	}
	question, ok := response.Pending.(QuestionRequest)
	if !ok || question.Prompt != "Código" {
		t.Fatalf("Pending = %#v, want KindOptionSet's own first (Código) question", response.Pending)
	}
	foundNote := false
	for _, msg := range response.Messages {
		if text, ok := msg.(TextMessage); ok && containsAll(text.Text, "Calibre AWG") {
			foundNote = true
		}
	}
	if !foundNote {
		t.Fatalf("Messages = %#v, want a note explaining the nested creation started", response.Messages)
	}
}

// TestCatalogAdminNestedCreateHintsDisplayFieldAsSuggestedOption proves the
// typed text that started the nested flow pre-seeds the nested kind's own
// display field (name/label/symbol priority) as a suggested — not forced —
// default: KindOptionSet's "name" field must offer it as a selectable Option,
// not skip straight past it.
func TestCatalogAdminNestedCreateHintsDisplayFieldAsSuggestedOption(t *testing.T) {
	adapter := newCatalogAdminAdapter(&fakeCatalogLister{}, &fakeCatalogGetter{}, &fakeCatalogCreator{}, &fakeCatalogUpdater{})
	ctx := context.Background()
	if _, err := adapter.startCreateFlow(ctx, domain.KindOption); err != nil {
		t.Fatalf("startCreateFlow(KindOption) error = %v", err)
	}
	if _, err := adapter.answerField(ctx, "Calibre AWG"); err != nil {
		t.Fatalf("answerField(%q) error = %v", "Calibre AWG", err)
	}
	// Nested KindOptionSet's first field is Código — answer it to reach the
	// hinted "name" field.
	response, err := adapter.answerField(ctx, "CALIBRE_AWG")
	if err != nil {
		t.Fatalf("answerField(código) error = %v", err)
	}
	question, ok := response.Pending.(QuestionRequest)
	if !ok || question.Prompt != "Nombre" {
		t.Fatalf("Pending = %#v, want the Nombre question next", response.Pending)
	}
	if len(question.Options) != 1 || question.Options[0].Value != "Calibre AWG" {
		t.Fatalf("Nombre question options = %#v, want one suggested option carrying %q", question.Options, "Calibre AWG")
	}
}

// TestCatalogAdminNestedCreateSuccessResumesParentWithNewRef proves task 8.1's
// core contract end-to-end: completing the nested create sub-flow calls
// Create for the nested kind, binds the new record's Código into the PARENT
// flow's triggering field, and resumes the parent's NEXT question — never
// restarting the parent flow.
func TestCatalogAdminNestedCreateSuccessResumesParentWithNewRef(t *testing.T) {
	creator := &fakeCatalogCreator{nextID: 50}
	adapter := newCatalogAdminAdapter(&fakeCatalogLister{}, &fakeCatalogGetter{}, creator, &fakeCatalogUpdater{})
	ctx := context.Background()
	if _, err := adapter.startCreateFlow(ctx, domain.KindOption); err != nil {
		t.Fatalf("startCreateFlow(KindOption) error = %v", err)
	}
	// optionSet (nested trigger) -> nested: code, name.
	response, err := adapter.answerField(ctx, "Calibre AWG")
	if err != nil {
		t.Fatalf("answerField(optionSet) error = %v", err)
	}
	response, err = adapter.answerField(ctx, "CALIBRE_AWG")
	if err != nil {
		t.Fatalf("answerField(código) error = %v", err)
	}
	response, err = adapter.answerField(ctx, "Calibre AWG")
	if err != nil {
		t.Fatalf("answerField(nombre) error = %v", err)
	}
	// The nested KindOptionSet create must have run exactly once, and the
	// parent editor must be resumed (not nil, not still the nested KindOptionSet).
	if len(creator.calls) != 1 || creator.kinds[0] != domain.KindOptionSet {
		t.Fatalf("creator.calls = %#v kinds=%v, want exactly one KindOptionSet Create", creator.calls, creator.kinds)
	}
	if adapter.editor == nil || adapter.editor.def.Code != domain.KindOption {
		t.Fatalf("adapter.editor = %#v, want the resumed KindOption parent flow", adapter.editor)
	}
	if adapter.editor.parent != nil {
		t.Fatalf("adapter.editor.parent = %#v, want nil (parent itself has no parent)", adapter.editor.parent)
	}
	if adapter.editor.values["optionSet"].Ref != (domain.CatalogRef{Kind: domain.KindOptionSet, Code: "CALIBRE_AWG"}) {
		t.Fatalf("parent's optionSet value = %#v, want a Ref to the newly-created CALIBRE_AWG record", adapter.editor.values["optionSet"])
	}
	// Parent resumes at its NEXT question: KindOption's second field is
	// "characteristic" (also AllowCreate — a custom answer would nest again,
	// but here we only assert the parent moved on to it).
	question, ok := response.Pending.(QuestionRequest)
	if !ok || question.Prompt != "Característica" {
		t.Fatalf("Pending = %#v, want the parent's NEXT question (Característica)", response.Pending)
	}
	foundNote := false
	for _, msg := range response.Messages {
		if text, ok := msg.(TextMessage); ok && containsAll(text.Text, "creado") {
			foundNote = true
		}
	}
	if !foundNote {
		t.Fatalf("Messages = %#v, want a confirmation note that the nested record was created", response.Messages)
	}
}

// TestCatalogAdminNestedCreateCancelReturnsToParentSameField proves
// cancelling the nested sub-flow does NOT abort the parent flow — it returns
// to the parent's SAME field/question, unchanged.
func TestCatalogAdminNestedCreateCancelReturnsToParentSameField(t *testing.T) {
	adapter := newCatalogAdminAdapter(&fakeCatalogLister{}, &fakeCatalogGetter{}, &fakeCatalogCreator{}, &fakeCatalogUpdater{})
	ctx := context.Background()
	if _, err := adapter.startCreateFlow(ctx, domain.KindOption); err != nil {
		t.Fatalf("startCreateFlow(KindOption) error = %v", err)
	}
	if _, err := adapter.answerField(ctx, "Calibre AWG"); err != nil {
		t.Fatalf("answerField(optionSet) error = %v", err)
	}
	if adapter.editor.def.Code != domain.KindOptionSet {
		t.Fatalf("adapter.editor.def.Code = %v, want KindOptionSet before cancelling", adapter.editor.def.Code)
	}
	response, err := adapter.Respond(ctx, InteractionInput{Kind: InputCancel})
	if err != nil {
		t.Fatalf("Respond(InputCancel) error = %v", err)
	}
	if adapter.editor == nil || adapter.editor.def.Code != domain.KindOption {
		t.Fatalf("adapter.editor = %#v, want the parent KindOption flow resumed, not aborted", adapter.editor)
	}
	question, ok := response.Pending.(QuestionRequest)
	if !ok || question.Prompt != "Conjunto de Opciones" {
		t.Fatalf("Pending = %#v, want the parent's SAME optionSet question re-asked", response.Pending)
	}
	if !containsAll(response.Messages[0].(TextMessage).Text, "canceló") {
		t.Fatalf("Messages[0] = %#v, want a cancellation note", response.Messages[0])
	}
}

// TestCatalogAdminAnswerMatchingExistingRefOptionNeverNests proves an
// AllowCreate field answer that matches an existing record's Código is
// treated as ordinary reuse — never starts a nested create sub-flow (task
// 8.2's own flip side: search-before-create only creates for a GENUINE
// non-match).
func TestCatalogAdminAnswerMatchingExistingRefOptionNeverNests(t *testing.T) {
	lister := &fakeCatalogLister{records: map[domain.CatalogKindCode][]domain.CatalogRecord{
		domain.KindOptionSet: {{Kind: domain.KindOptionSet, ID: 1, Active: true, Values: map[string]domain.CatalogValue{
			"code": {Text: "CALIBRE_AWG"}, "name": {Text: "Calibre AWG"},
		}}},
	}}
	creator := &fakeCatalogCreator{}
	adapter := newCatalogAdminAdapter(lister, &fakeCatalogGetter{}, creator, &fakeCatalogUpdater{})
	ctx := context.Background()
	if _, err := adapter.startCreateFlow(ctx, domain.KindOption); err != nil {
		t.Fatalf("startCreateFlow(KindOption) error = %v", err)
	}
	response, err := adapter.answerField(ctx, "CALIBRE_AWG")
	if err != nil {
		t.Fatalf("answerField(optionSet) error = %v", err)
	}
	if adapter.editor == nil || adapter.editor.def.Code != domain.KindOption || adapter.editor.parent != nil {
		t.Fatalf("adapter.editor = %#v, want the SAME top-level KindOption flow, no nesting", adapter.editor)
	}
	if adapter.editor.values["optionSet"].Ref != (domain.CatalogRef{Kind: domain.KindOptionSet, Code: "CALIBRE_AWG"}) {
		t.Fatalf("optionSet value = %#v, want a Ref to the existing CALIBRE_AWG record", adapter.editor.values["optionSet"])
	}
	if len(creator.calls) != 0 {
		t.Fatalf("creator.calls = %#v, want none — matching an existing record must never call Create", creator.calls)
	}
	question, ok := response.Pending.(QuestionRequest)
	if !ok || question.Prompt != "Característica" {
		t.Fatalf("Pending = %#v, want the parent's NEXT question, advanced normally", response.Pending)
	}
}

// TestCatalogAdminAllowCreateFieldListsExistingBeforeCustomCreate proves task
// 8.2: existing matching records for an AllowCreate field are offered in the
// question's Options list, and NO separate "create new" Option is mixed into
// that list — custom creation is reachable only by typing a genuinely
// non-matching value (answerField's own branch), never as a selectable Option
// alongside the real records.
func TestCatalogAdminAllowCreateFieldListsExistingBeforeCustomCreate(t *testing.T) {
	lister := &fakeCatalogLister{records: map[domain.CatalogKindCode][]domain.CatalogRecord{
		domain.KindAttributeDefinition: {
			definitionRecord(1, "COLOR", "Color"),
			definitionRecord(2, "CALIBRE", "Calibre"),
		},
	}}
	adapter := newCatalogAdminAdapter(lister, &fakeCatalogGetter{}, &fakeCatalogCreator{}, &fakeCatalogUpdater{})
	ctx := context.Background()
	if _, err := adapter.startCreateFlow(ctx, domain.KindOption); err != nil {
		t.Fatalf("startCreateFlow(KindOption) error = %v", err)
	}
	// KindOption's second field, "characteristic", is the AllowCreate field
	// under test here (RefKind KindAttributeDefinition) — fieldQuestion
	// renders any field's question directly off the descriptor, independent
	// of state.step, so it can be inspected in isolation without first
	// resolving "optionSet".
	characteristicField := adapter.editor.def.Fields[1]
	if characteristicField.Name != "characteristic" {
		t.Fatalf("Fields[1].Name = %q, want characteristic", characteristicField.Name)
	}
	question, err := adapter.fieldQuestion(ctx, characteristicField)
	if err != nil {
		t.Fatalf("fieldQuestion(characteristic) error = %v", err)
	}
	pending, ok := question.Pending.(QuestionRequest)
	if !ok {
		t.Fatalf("Pending = %T, want QuestionRequest", question.Pending)
	}
	if len(pending.Options) != 2 {
		t.Fatalf("Options = %#v, want exactly the 2 existing Características, no extra create-affordance entry", pending.Options)
	}
	gotCodes := map[string]bool{}
	for _, opt := range pending.Options {
		gotCodes[opt.Value] = true
		if opt.Value == catalogCreateNewOptionID {
			t.Fatalf("Options contains the kind-menu create-new sentinel %q — custom creation must only be reachable by typing, not as a menu Option", catalogCreateNewOptionID)
		}
	}
	if !gotCodes["COLOR"] || !gotCodes["CALIBRE"] {
		t.Fatalf("Options = %#v, want both existing Características COLOR and CALIBRE listed", pending.Options)
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
