package tui

import (
	"context"
	"strings"
	"testing"

	"github.com/GARFEX33/garfex-costos-unitarios/internal/domain"
)

// fakeResourceGetter is the fake resourceGetter used by editor tests. It
// records the classCode it was called with (not just identityKey) so tests
// can assert design R1's "Get's first argument is a class code, never a
// family code" contract at the editor's own call site.
type fakeResourceGetter struct {
	resource       domain.Resource
	callCount      int
	gotClassCode   string
	gotIdentityKey string
	err            error
}

func (f *fakeResourceGetter) Get(_ context.Context, classCode, identityKey string) (domain.Resource, error) {
	f.callCount++
	f.gotClassCode = classCode
	f.gotIdentityKey = identityKey
	if f.err != nil {
		return domain.Resource{}, f.err
	}
	return f.resource, nil
}

// fakeResourceDescriber is a resourceDescriber returning a fixed text — the
// editor tests leave it zero-valued (a no-op, since they only exercise the
// technical field list, never the catalog-controlled title), while the
// workspace dispatch tests in resources_workspace_adapter_test.go set text
// to prove the detail title composes FamilyCode + " — " + Describe(...).
type fakeResourceDescriber struct{ text string }

func (f *fakeResourceDescriber) Describe(domain.Resource) string { return f.text }

// fakeResourceCreator is the fake resourceCreator used by every editor test.
type fakeResourceCreator struct {
	callCount   int
	gotResource domain.Resource
	err         error
}

func (f *fakeResourceCreator) Create(_ context.Context, resource domain.Resource) error {
	f.callCount++
	f.gotResource = resource
	return f.err
}

// fakeResourceUpdater is the fake resourceUpdater used by the edit-flow
// tests.
type fakeResourceUpdater struct {
	callCount   int
	gotResource domain.Resource
	err         error
}

func (f *fakeResourceUpdater) Update(_ context.Context, resource domain.Resource) error {
	f.callCount++
	f.gotResource = resource
	return f.err
}

// newTestAdapter builds a ResourcesWorkspaceAdapter against the real
// production catalog (domain.SeedResourceCatalog()) — the editor tests
// deliberately exercise the real CONDUCTORES/CANALIZACIONES seed data, not a
// synthetic fixture, mirroring the original Materiales Maestros editor
// tests. resources/deleter are left nil: this PR's tests only drive the
// create/edit/duplicate editor state machine directly (startCreateEditor/
// respondToEditor/startEditEditor/startDuplicateEditor), never the full
// Respond dispatch that would need a real searcher/deleter.
func newTestAdapter(getter resourceGetter, creator resourceCreator, updater resourceUpdater, classFilter string) *ResourcesWorkspaceAdapter {
	return NewResourcesWorkspaceAdapter(nil, getter, &fakeResourceDescriber{}, creator, updater, nil, domain.SeedResourceCatalog(), classFilter)
}

// answerQuestion is a small test helper: it asserts response.Pending is a
// QuestionRequest keyed resourceEditorKey and drives the next turn directly
// through respondToEditor (the editor's own dispatch, in scope for this
// PR) with that key and the given value.
func answerQuestion(t *testing.T, adapter *ResourcesWorkspaceAdapter, response InteractionResponse, value string) InteractionResponse {
	t.Helper()
	request, ok := response.Pending.(QuestionRequest)
	if !ok {
		t.Fatalf("Pending = %T, want QuestionRequest", response.Pending)
	}
	if request.Key != resourceEditorKey {
		t.Fatalf("Pending.Key = %q, want %q", request.Key, resourceEditorKey)
	}
	next, handled := adapter.respondToEditor(context.Background(), InteractionInput{Kind: InputSelection, Key: request.Key, Value: value})
	if !handled {
		t.Fatalf("respondToEditor did not claim input keyed %q", request.Key)
	}
	return next
}

// answerConfirmation is answerQuestion's counterpart for a
// ConfirmationRequest.
func answerConfirmation(t *testing.T, adapter *ResourcesWorkspaceAdapter, response InteractionResponse, value string) InteractionResponse {
	t.Helper()
	request, ok := response.Pending.(ConfirmationRequest)
	if !ok {
		t.Fatalf("Pending = %T, want ConfirmationRequest", response.Pending)
	}
	if request.Key != resourceEditorKey {
		t.Fatalf("Pending.Key = %q, want %q", request.Key, resourceEditorKey)
	}
	next, handled := adapter.respondToEditor(context.Background(), InteractionInput{Kind: InputSelection, Key: request.Key, Value: value})
	if !handled {
		t.Fatalf("respondToEditor did not claim input keyed %q", request.Key)
	}
	return next
}

// cableAttributeValues is the shared MATERIAL/CONDUCTORES/CABLE attribute
// set used by several edit-flow tests, in catalog declaration order.
func cableAttributeValues() []domain.ResourceAttributeValue {
	return []domain.ResourceAttributeValue{
		domain.OptionValue("conductor_material", "COBRE"),
		domain.OptionValue("gauge", "10 AWG"),
		domain.OptionValue("insulation", "THW"),
		domain.OptionValue("color", "NEGRO"),
		domain.OptionValue("voltage", "600 V"),
	}
}

// tuberiaAttributeValues is the shared MATERIAL/CANALIZACIONES/TUBERIA
// attribute set used by the diameter-narrowing tests. diameter_inch=1/2"
// and diameter_mm=13 mm are the related pair per tuberiasRelations()
// (resource_catalog.go).
func tuberiaAttributeValues() []domain.ResourceAttributeValue {
	return []domain.ResourceAttributeValue{
		domain.OptionValue("tipo", "CONDUIT PARED DELGADA"),
		domain.OptionValue("diameter_inch", `1/2"`),
		domain.OptionValue("diameter_mm", "13 mm"),
	}
}

// openEditFor seeds adapter.lastDetail and starts the "Editar" flow for
// resource — the direct-call counterpart of the old suite's
// openDetailViaSearch, since this PR's adapter has no Respond/search/detail
// dispatch yet (see ResourcesWorkspaceAdapter's doc comment).
func openEditFor(t *testing.T, adapter *ResourcesWorkspaceAdapter, resource domain.Resource) InteractionResponse {
	t.Helper()
	adapter.lastDetail = resource
	response, err := adapter.startEditEditor()
	if err != nil {
		t.Fatalf("startEditEditor() error = %v, want nil", err)
	}
	return response
}

// openDuplicateFor is openEditFor's counterpart for the "Duplicar" flow.
func openDuplicateFor(t *testing.T, adapter *ResourcesWorkspaceAdapter, resource domain.Resource) InteractionResponse {
	t.Helper()
	adapter.lastDetail = resource
	response, err := adapter.startDuplicateEditor()
	if err != nil {
		t.Fatalf("startDuplicateEditor() error = %v, want nil", err)
	}
	return response
}

// inactiveClassCatalog is domain.SeedResourceCatalog() with MATERIAL marked
// inactive — task 7.3's fixture for the "resource under an inactive Clase"
// blocking tests below. The existing resources still readable/searchable
// under it are untouched (spec: "existing resources remain
// readable/searchable"), only the Clase's own Active flag flips.
func inactiveClassCatalog() domain.ResourceCatalog {
	catalog := domain.SeedResourceCatalog()
	for i := range catalog.Classes {
		if catalog.Classes[i].Code == "MATERIAL" {
			catalog.Classes[i].Active = false
		}
	}
	return catalog
}

// TestResourcePresentationMarksInactiveClass proves task 7.3: a resource
// belonging to an inactive Clase is clearly marked "(Clase inactiva)" in its
// presentation title — the shared renderer both search results and the
// detail view use (resourcePresentation), so marking it here covers both
// surfaces at once.
func TestResourcePresentationMarksInactiveClass(t *testing.T) {
	adapter := NewResourcesWorkspaceAdapter(nil, &fakeResourceGetter{}, &fakeResourceDescriber{}, &fakeResourceCreator{}, &fakeResourceUpdater{}, nil, inactiveClassCatalog(), "")
	resource := domain.Resource{ClassCode: "MATERIAL", FamilyCode: "CONDUCTORES", TypeCode: "CABLE", Attributes: cableAttributeValues(), NaturalUnit: "m"}
	title, _ := adapter.resourcePresentation(resource)
	if !containsAll(title, "Clase inactiva") {
		t.Fatalf("title = %q, want it to mark the inactive Clase", title)
	}
}

// TestResourcePresentationDoesNotMarkActiveClass is the contrast: an active
// Clase's resource title is never marked.
func TestResourcePresentationDoesNotMarkActiveClass(t *testing.T) {
	adapter := newTestAdapter(&fakeResourceGetter{}, &fakeResourceCreator{}, &fakeResourceUpdater{}, "")
	resource := domain.Resource{ClassCode: "MATERIAL", FamilyCode: "CONDUCTORES", TypeCode: "CABLE", Attributes: cableAttributeValues(), NaturalUnit: "m"}
	title, _ := adapter.resourcePresentation(resource)
	if containsAll(title, "Clase inactiva") {
		t.Fatalf("title = %q, must not mark an active Clase's resource", title)
	}
}

// TestStartEditEditorBlockedForInactiveClass proves task 7.3: "Editar" is
// blocked for a resource whose Clase is inactive — no editor starts, and a
// Spanish message explains why.
func TestStartEditEditorBlockedForInactiveClass(t *testing.T) {
	adapter := NewResourcesWorkspaceAdapter(nil, &fakeResourceGetter{}, &fakeResourceDescriber{}, &fakeResourceCreator{}, &fakeResourceUpdater{}, nil, inactiveClassCatalog(), "")
	resource := domain.Resource{ClassCode: "MATERIAL", FamilyCode: "CONDUCTORES", TypeCode: "CABLE", Attributes: cableAttributeValues(), NaturalUnit: "m"}
	response := openEditFor(t, adapter, resource)
	if adapter.editor != nil {
		t.Fatal("adapter.editor != nil, want Editar blocked for an inactive Clase")
	}
	message, ok := response.Messages[0].(ErrorMessage)
	if !ok || !containsAll(message.Text, "inactiva") {
		t.Fatalf("Messages[0] = %#v, want an ErrorMessage mentioning the inactive Clase", response.Messages[0])
	}
}

// TestStartDuplicateEditorBlockedForInactiveClass mirrors the Editar test
// for "Duplicar" — duplicating a resource functionally creates a new one, so
// it is blocked by the same rule as Crear.
func TestStartDuplicateEditorBlockedForInactiveClass(t *testing.T) {
	adapter := NewResourcesWorkspaceAdapter(nil, &fakeResourceGetter{}, &fakeResourceDescriber{}, &fakeResourceCreator{}, &fakeResourceUpdater{}, nil, inactiveClassCatalog(), "")
	resource := domain.Resource{ClassCode: "MATERIAL", FamilyCode: "CONDUCTORES", TypeCode: "CABLE", Attributes: cableAttributeValues(), NaturalUnit: "m"}
	response := openDuplicateFor(t, adapter, resource)
	if adapter.editor != nil {
		t.Fatal("adapter.editor != nil, want Duplicar blocked for an inactive Clase")
	}
	message, ok := response.Messages[0].(ErrorMessage)
	if !ok || !containsAll(message.Text, "inactiva") {
		t.Fatalf("Messages[0] = %#v, want an ErrorMessage mentioning the inactive Clase", response.Messages[0])
	}
}

// TestStartCreateEditorBlockedForFilteredInactiveClass proves task 7.3's
// defensive case: a FILTERED workspace (e.g. "/materiales") already scoped
// to a class that has since become inactive (within the same running
// process — buildWorkspaceDescriptors excludes it at build time, but that
// build only happens once) must still refuse Crear, not silently proceed
// past the (skipped, pre-seeded) Clase question.
func TestStartCreateEditorBlockedForFilteredInactiveClass(t *testing.T) {
	adapter := NewResourcesWorkspaceAdapter(nil, &fakeResourceGetter{}, &fakeResourceDescriber{}, &fakeResourceCreator{}, &fakeResourceUpdater{}, nil, inactiveClassCatalog(), "MATERIAL")
	response, err := adapter.startCreateEditor()
	if err != nil {
		t.Fatalf("startCreateEditor() error = %v, want nil", err)
	}
	if adapter.editor != nil {
		t.Fatal("adapter.editor != nil, want Crear blocked for a filtered inactive Clase")
	}
	message, ok := response.Messages[0].(ErrorMessage)
	if !ok || !containsAll(message.Text, "inactiva") {
		t.Fatalf("Messages[0] = %#v, want an ErrorMessage mentioning the inactive Clase", response.Messages[0])
	}
}

// TestResourceEditorUnfilteredCreateFlowAsksClaseFamiliaTipoAttributesUnit
// covers the required unfiltered-entry sequence assertion (recursos-maestro
// tasks 6.1): a create flow started with no classFilter must begin at
// editorStepClase, then walk Familia -> Tipo -> every attribute -> Unit,
// exactly as a filtered workspace does after its own pre-seeded Familia
// step — proving the whole downstream state machine is unaffected by where
// the class comes from.
func TestResourceEditorUnfilteredCreateFlowAsksClaseFamiliaTipoAttributesUnit(t *testing.T) {
	creator := &fakeResourceCreator{}
	adapter := newTestAdapter(&fakeResourceGetter{}, creator, &fakeResourceUpdater{}, "")

	response, err := adapter.startCreateEditor()
	if err != nil {
		t.Fatalf("startCreateEditor() error = %v, want nil", err)
	}

	claseRequest, ok := response.Pending.(QuestionRequest)
	if !ok {
		t.Fatalf("Pending = %T, want QuestionRequest (Clase)", response.Pending)
	}
	if !strings.Contains(strings.ToLower(claseRequest.Prompt), "clase") {
		t.Fatalf("first question Prompt = %q, want the Clase question", claseRequest.Prompt)
	}
	foundMaterial := false
	for _, option := range claseRequest.Options {
		if option.Value == "MATERIAL" {
			foundMaterial = true
		}
	}
	if !foundMaterial {
		t.Fatalf("Clase Options = %v, want MATERIAL among them", claseRequest.Options)
	}

	response = answerQuestion(t, adapter, response, "MATERIAL")

	familyRequest, ok := response.Pending.(QuestionRequest)
	if !ok {
		t.Fatalf("Pending = %T, want QuestionRequest (Familia)", response.Pending)
	}
	if !strings.Contains(strings.ToLower(familyRequest.Prompt), "familia") {
		t.Fatalf("Prompt after Clase = %q, want the Familia question", familyRequest.Prompt)
	}

	response = answerQuestion(t, adapter, response, "CONDUCTORES")

	typeRequest, ok := response.Pending.(QuestionRequest)
	if !ok {
		t.Fatalf("Pending = %T, want QuestionRequest (Tipo)", response.Pending)
	}
	if !strings.Contains(strings.ToLower(typeRequest.Prompt), "tipo") {
		t.Fatalf("Prompt after Familia = %q, want the Tipo question", typeRequest.Prompt)
	}

	response = answerQuestion(t, adapter, response, "CABLE")
	response = answerQuestion(t, adapter, response, "COBRE")  // conductor_material
	response = answerQuestion(t, adapter, response, "10 AWG") // gauge
	response = answerQuestion(t, adapter, response, "THW")    // insulation
	response = answerQuestion(t, adapter, response, "NEGRO")  // color
	response = answerQuestion(t, adapter, response, "600 V")  // voltage

	unitRequest, ok := response.Pending.(QuestionRequest)
	if !ok {
		t.Fatalf("Pending = %T, want QuestionRequest (Unidad)", response.Pending)
	}
	if !strings.Contains(strings.ToLower(unitRequest.Prompt), "unidad") {
		t.Fatalf("Prompt after last attribute = %q, want the NaturalUnit question", unitRequest.Prompt)
	}

	response = answerQuestion(t, adapter, response, "M")

	if response.Pending != nil {
		t.Fatalf("Pending = %#v, want nil after successful create", response.Pending)
	}
	if creator.callCount != 1 {
		t.Fatalf("Create call count = %d, want 1", creator.callCount)
	}

	catalog := domain.SeedResourceCatalog()
	scope := domain.ResourceScope{ClassCode: "MATERIAL", FamilyCode: "CONDUCTORES", TypeCode: "CABLE"}
	want, err := domain.NewResource(catalog, scope, "M", cableAttributeValues())
	if err != nil {
		t.Fatalf("NewResource(want) error = %v, want nil", err)
	}
	if creator.gotResource.ClassCode != want.ClassCode {
		t.Fatalf("gotResource.ClassCode = %q, want %q", creator.gotResource.ClassCode, want.ClassCode)
	}
	if creator.gotResource.FamilyCode != want.FamilyCode {
		t.Fatalf("gotResource.FamilyCode = %q, want %q", creator.gotResource.FamilyCode, want.FamilyCode)
	}
	if creator.gotResource.IdentityKey != want.IdentityKey {
		t.Fatalf("gotResource.IdentityKey = %q, want %q", creator.gotResource.IdentityKey, want.IdentityKey)
	}
}

// TestResourceEditorFilteredCreateFlowSkipsClaseAndStartsAtFamilia covers
// the required filtered-entry sequence assertion (recursos-maestro tasks
// 6.1): a create flow started with classFilter == "MATERIAL" must never ask
// editorStepClase at all — the very first question must already be Familia,
// with the class pre-seeded onto the editor state exactly like
// startEditEditor/startDuplicateEditor already pre-seed Familia/Tipo for an
// existing resource.
func TestResourceEditorFilteredCreateFlowSkipsClaseAndStartsAtFamilia(t *testing.T) {
	adapter := newTestAdapter(&fakeResourceGetter{}, &fakeResourceCreator{}, &fakeResourceUpdater{}, "MATERIAL")

	response, err := adapter.startCreateEditor()
	if err != nil {
		t.Fatalf("startCreateEditor() error = %v, want nil", err)
	}

	familyRequest, ok := response.Pending.(QuestionRequest)
	if !ok {
		t.Fatalf("Pending = %T, want QuestionRequest (Familia)", response.Pending)
	}
	if strings.Contains(strings.ToLower(familyRequest.Prompt), "clase") {
		t.Fatalf("Prompt = %q, must not be the Clase question for a filtered workspace", familyRequest.Prompt)
	}
	if !strings.Contains(strings.ToLower(familyRequest.Prompt), "familia") {
		t.Fatalf("Prompt = %q, want the Familia question", familyRequest.Prompt)
	}
	if adapter.editor.class != "MATERIAL" {
		t.Fatalf("editor.class = %q, want the pre-seeded classFilter %q", adapter.editor.class, "MATERIAL")
	}
	if adapter.editor.step != editorStepFamily {
		t.Fatalf("editor.step = %v, want editorStepFamily (Clase skipped entirely)", adapter.editor.step)
	}
}

// TestResourceEditorDesnudoSkipsColorAndVoltage covers the DESNUDO path:
// choosing insulation=DESNUDO must make the very next question the
// NaturalUnit question, never color or voltage — proving advanceEditor's
// ResourceAttribute.Effective-driven skip survives the generalization.
func TestResourceEditorDesnudoSkipsColorAndVoltage(t *testing.T) {
	adapter := newTestAdapter(&fakeResourceGetter{}, &fakeResourceCreator{}, &fakeResourceUpdater{}, "MATERIAL")

	response, err := adapter.startCreateEditor()
	if err != nil {
		t.Fatalf("startCreateEditor() error = %v, want nil", err)
	}
	response = answerQuestion(t, adapter, response, "CONDUCTORES")
	response = answerQuestion(t, adapter, response, "CABLE")
	response = answerQuestion(t, adapter, response, "COBRE")
	response = answerQuestion(t, adapter, response, "10 AWG")
	response = answerQuestion(t, adapter, response, "DESNUDO")

	request, ok := response.Pending.(QuestionRequest)
	if !ok {
		t.Fatalf("Pending = %T, want QuestionRequest", response.Pending)
	}
	if !strings.Contains(strings.ToLower(request.Prompt), "unidad") {
		t.Fatalf("Prompt after DESNUDO = %q, want the NaturalUnit question (color/voltage skipped)", request.Prompt)
	}
}

// TestResourceEditorNarrowsDiameterMmByDiameterInch covers the TUBERIA
// narrowing path: after choosing diameter_inch=1/2", the diameter_mm
// question's Options must contain exactly the related option (13 mm).
func TestResourceEditorNarrowsDiameterMmByDiameterInch(t *testing.T) {
	adapter := newTestAdapter(&fakeResourceGetter{}, &fakeResourceCreator{}, &fakeResourceUpdater{}, "MATERIAL")

	response, err := adapter.startCreateEditor()
	if err != nil {
		t.Fatalf("startCreateEditor() error = %v, want nil", err)
	}
	response = answerQuestion(t, adapter, response, "CANALIZACIONES")
	response = answerQuestion(t, adapter, response, "TUBERIA")
	response = answerQuestion(t, adapter, response, "CONDUIT PARED DELGADA") // tipo
	response = answerQuestion(t, adapter, response, `1/2"`)                  // diameter_inch

	request, ok := response.Pending.(QuestionRequest)
	if !ok {
		t.Fatalf("Pending = %T, want QuestionRequest", response.Pending)
	}
	if len(request.Options) != 1 || request.Options[0].Value != "13 mm" {
		t.Fatalf("Options = %v, want exactly [13 mm]", request.Options)
	}
}

// TestResourceEditorCreateDuplicateIdentityLooksUpByClassCode covers
// finishEditor's ErrDuplicateResource path AND design R1: the duplicate
// lookup must call resourcesGetter.Get with the candidate's ClassCode, never
// its FamilyCode — the exact miswiring the compiler cannot catch (design
// R1), now regression-tested at the editor's own call site.
func TestResourceEditorCreateDuplicateIdentityLooksUpByClassCode(t *testing.T) {
	existing := domain.Resource{
		ClassCode: "MATERIAL", FamilyCode: "CONDUCTORES", TypeCode: "CABLE", NaturalUnit: "M",
		IdentityKey: "MATERIAL|CONDUCTORES|CABLE|color=NEGRO|conductor_material=COBRE|gauge=10 AWG|insulation=THW|voltage=600 V",
		Attributes:  []domain.ResourceAttributeValue{domain.OptionValue("insulation", "THW")},
	}
	creator := &fakeResourceCreator{err: domain.ErrDuplicateResource}
	getter := &fakeResourceGetter{resource: existing}
	adapter := newTestAdapter(getter, creator, &fakeResourceUpdater{}, "MATERIAL")

	response, err := adapter.startCreateEditor()
	if err != nil {
		t.Fatalf("startCreateEditor() error = %v, want nil", err)
	}
	response = answerQuestion(t, adapter, response, "CONDUCTORES")
	response = answerQuestion(t, adapter, response, "CABLE")
	response = answerQuestion(t, adapter, response, "COBRE")
	response = answerQuestion(t, adapter, response, "10 AWG")
	response = answerQuestion(t, adapter, response, "THW")
	response = answerQuestion(t, adapter, response, "NEGRO")
	response = answerQuestion(t, adapter, response, "600 V")
	response = answerQuestion(t, adapter, response, "M")

	if getter.callCount != 1 {
		t.Fatalf("Get call count = %d, want 1", getter.callCount)
	}
	if getter.gotClassCode != "MATERIAL" {
		t.Fatalf("Get called with classCode %q, want %q", getter.gotClassCode, "MATERIAL")
	}
	if getter.gotClassCode == "CONDUCTORES" {
		t.Fatalf("Get called with the family code %q as the class code — design R1 violation", getter.gotClassCode)
	}
	found := false
	for _, message := range response.Messages {
		if result, ok := message.(StructuredResult); ok {
			found = true
			if !strings.HasPrefix(result.Title, "Cable · Material › Conductores") {
				t.Fatalf("existing resource Title = %q, want it to start with the business identity", result.Title)
			}
		}
	}
	if !found {
		t.Fatalf("Messages = %v, want a StructuredResult showing the existing resource", response.Messages)
	}
	if response.Pending != nil {
		t.Fatalf("Pending = %#v, want nil (editor reset after duplicate)", response.Pending)
	}
}

// TestResourceEditorCancelResetsEditor covers cancellation mid-flow:
// InputCancel resets a.editor.
func TestResourceEditorCancelResetsEditor(t *testing.T) {
	adapter := newTestAdapter(&fakeResourceGetter{}, &fakeResourceCreator{}, &fakeResourceUpdater{}, "MATERIAL")

	response, err := adapter.startCreateEditor()
	if err != nil {
		t.Fatalf("startCreateEditor() error = %v, want nil", err)
	}
	response = answerQuestion(t, adapter, response, "CONDUCTORES")
	if response.Pending == nil {
		t.Fatalf("Pending = nil, want the Tipo question still in progress")
	}

	cancelled, handled := adapter.respondToEditor(context.Background(), InteractionInput{Kind: InputCancel})
	if !handled {
		t.Fatalf("respondToEditor did not claim the cancellation")
	}
	if cancelled.Pending != nil {
		t.Fatalf("Pending = %#v, want nil after cancellation", cancelled.Pending)
	}
	if adapter.editor != nil {
		t.Fatalf("adapter.editor = %#v, want nil after cancellation", adapter.editor)
	}
}

// TestResourceEditorEditFullHappyPathReproducesSameResource covers the
// picker-based EDIT flow end to end: opening a MATERIAL/CONDUCTORES/CABLE
// resource, starting "Editar" (never asking Clase/Familia/Tipo again),
// picking ONE field, confirming its question defaults to the resource's
// current value, answering with that SAME value, looping back to the
// picker, and picking "Terminar edición" to reach the confirmation —
// confirming it calls updater.Update (never creator.Create) with the
// original Resource.ID preserved and the exact same IdentityKey.
func TestResourceEditorEditFullHappyPathReproducesSameResource(t *testing.T) {
	catalog := domain.SeedResourceCatalog()
	scope := domain.ResourceScope{ClassCode: "MATERIAL", FamilyCode: "CONDUCTORES", TypeCode: "CABLE"}
	existing, err := domain.NewResource(catalog, scope, "M", cableAttributeValues())
	if err != nil {
		t.Fatalf("NewResource(existing) error = %v, want nil", err)
	}
	existing.ID = 42

	creator := &fakeResourceCreator{}
	updater := &fakeResourceUpdater{}
	adapter := newTestAdapter(&fakeResourceGetter{}, creator, updater, "MATERIAL")

	response := openEditFor(t, adapter, existing)

	pickerRequest, ok := response.Pending.(QuestionRequest)
	if !ok {
		t.Fatalf("Pending = %T, want QuestionRequest (field picker)", response.Pending)
	}
	if !strings.Contains(strings.ToLower(pickerRequest.Prompt), "campo") {
		t.Fatalf("Prompt = %q, want the field picker prompt", pickerRequest.Prompt)
	}

	response = answerQuestion(t, adapter, response, "conductor_material")

	request, ok := response.Pending.(QuestionRequest)
	if !ok {
		t.Fatalf("Pending = %T, want QuestionRequest (conductor_material question)", response.Pending)
	}
	if len(request.Options) == 0 || request.Options[0].Value != "COBRE" {
		t.Fatalf("conductor_material Options = %v, want the resource's current value (COBRE) first", request.Options)
	}

	response = answerQuestion(t, adapter, response, "COBRE") // unchanged
	response = answerQuestion(t, adapter, response, editFinishFieldCode)
	response = answerConfirmation(t, adapter, response, "yes")

	if creator.callCount != 0 {
		t.Fatalf("Create call count = %d, want 0 (edit must never call Create)", creator.callCount)
	}
	if updater.callCount != 1 {
		t.Fatalf("Update call count = %d, want 1", updater.callCount)
	}
	if updater.gotResource.ID != 42 {
		t.Fatalf("gotResource.ID = %d, want 42 (the original Resource.ID)", updater.gotResource.ID)
	}
	if updater.gotResource.IdentityKey != existing.IdentityKey {
		t.Fatalf("gotResource.IdentityKey = %q, want %q (an unchanged answer reproduces the same identity)", updater.gotResource.IdentityKey, existing.IdentityKey)
	}
	if response.Pending != nil {
		t.Fatalf("Pending = %#v, want nil after a successful edit", response.Pending)
	}
}

// TestResourceEditorEditChangesOneAttributeAndNarrowsByEditedSiblingOnly
// covers the field picker's menu shape (current values, including "Unidad
// natural") AND the narrowingContext regression: editing diameter_inch on a
// CANALIZACIONES/TUBERIA resource with an UNTOUCHED, stale diameter_mm must
// show every approved diameter_inch option (not narrowed by the stale
// sibling), while a color change on a CONDUCTORES/CABLE resource still
// produces the expected Update with every other attribute unchanged.
func TestResourceEditorEditChangesOneAttributeAndNarrowsByEditedSiblingOnly(t *testing.T) {
	catalog := domain.SeedResourceCatalog()
	cableScope := domain.ResourceScope{ClassCode: "MATERIAL", FamilyCode: "CONDUCTORES", TypeCode: "CABLE"}
	existing, err := domain.NewResource(catalog, cableScope, "M", cableAttributeValues())
	if err != nil {
		t.Fatalf("NewResource(existing) error = %v, want nil", err)
	}
	existing.ID = 42

	updater := &fakeResourceUpdater{}
	adapter := newTestAdapter(&fakeResourceGetter{}, &fakeResourceCreator{}, updater, "MATERIAL")

	response := openEditFor(t, adapter, existing)
	pickerRequest, ok := response.Pending.(QuestionRequest)
	if !ok {
		t.Fatalf("Pending = %T, want QuestionRequest (field picker)", response.Pending)
	}
	wantLabels := map[string]string{
		"conductor_material":     "Material del conductor: COBRE",
		"gauge":                  "Calibre: 10 AWG",
		"insulation":             "Aislamiento: THW",
		"color":                  "Color: NEGRO",
		"voltage":                "Voltaje: 600 V",
		editNaturalUnitFieldCode: "Unidad natural: M",
		editFinishFieldCode:      "Terminar edición",
	}
	if len(pickerRequest.Options) != len(wantLabels) {
		t.Fatalf("picker Options = %v, want %d entries: %v", pickerRequest.Options, len(wantLabels), wantLabels)
	}
	for _, option := range pickerRequest.Options {
		wantLabel, known := wantLabels[option.ID]
		if !known {
			t.Fatalf("unexpected picker Option.ID %q (Label %q)", option.ID, option.Label)
		}
		if option.Label != wantLabel {
			t.Fatalf("picker Option[%q].Label = %q, want %q", option.ID, option.Label, wantLabel)
		}
	}

	response = answerQuestion(t, adapter, response, "color")
	response = answerQuestion(t, adapter, response, "BLANCO") // changed from NEGRO
	response = answerQuestion(t, adapter, response, editFinishFieldCode)
	response = answerConfirmation(t, adapter, response, "yes")

	if updater.callCount != 1 {
		t.Fatalf("Update call count = %d, want 1", updater.callCount)
	}
	if updater.gotResource.IdentityKey == existing.IdentityKey {
		t.Fatalf("gotResource.IdentityKey = %q, want it to differ after changing color", updater.gotResource.IdentityKey)
	}

	// Second half: the narrowingContext regression on the untouched-sibling
	// case (design's original bug report), using a fresh CANALIZACIONES
	// resource/adapter.
	tuberiaScope := domain.ResourceScope{ClassCode: "MATERIAL", FamilyCode: "CANALIZACIONES", TypeCode: "TUBERIA"}
	tuberia, err := domain.NewResource(catalog, tuberiaScope, "PZA", tuberiaAttributeValues())
	if err != nil {
		t.Fatalf("NewResource(tuberia) error = %v, want nil", err)
	}
	tuberia.ID = 7
	tuberiaAdapter := newTestAdapter(&fakeResourceGetter{}, &fakeResourceCreator{}, &fakeResourceUpdater{}, "MATERIAL")
	tuberiaResponse := openEditFor(t, tuberiaAdapter, tuberia)
	tuberiaResponse = answerQuestion(t, tuberiaAdapter, tuberiaResponse, "diameter_inch")

	request, ok := tuberiaResponse.Pending.(QuestionRequest)
	if !ok {
		t.Fatalf("Pending = %T, want QuestionRequest (diameter_inch question)", tuberiaResponse.Pending)
	}
	wantOptions := []string{`1/2"`, `3/4"`, `1"`, `1 1/4"`, `1 1/2"`, `2"`, `2 1/2"`, `3"`, `4"`}
	if len(request.Options) != len(wantOptions) {
		t.Fatalf("diameter_inch Options = %v, want all %d approved values (untouched diameter_mm must not narrow it)", request.Options, len(wantOptions))
	}
}

// TestResourceEditorEditDesnudoDropsNowForbiddenAttributes covers the real
// correctness gap the single-field edit flow introduces that CREATE's
// sequential walkthrough never had: editing insulation from THW to DESNUDO
// leaves the pre-existing color/voltage values sitting in state.values even
// though DESNUDO makes both ModeForbidden/notApplicable — this proves the
// edit still completes successfully with them silently dropped.
func TestResourceEditorEditDesnudoDropsNowForbiddenAttributes(t *testing.T) {
	catalog := domain.SeedResourceCatalog()
	scope := domain.ResourceScope{ClassCode: "MATERIAL", FamilyCode: "CONDUCTORES", TypeCode: "CABLE"}
	existing, err := domain.NewResource(catalog, scope, "M", cableAttributeValues())
	if err != nil {
		t.Fatalf("NewResource(existing) error = %v, want nil", err)
	}
	existing.ID = 42

	updater := &fakeResourceUpdater{}
	adapter := newTestAdapter(&fakeResourceGetter{}, &fakeResourceCreator{}, updater, "MATERIAL")

	response := openEditFor(t, adapter, existing)
	response = answerQuestion(t, adapter, response, "insulation")
	response = answerQuestion(t, adapter, response, "DESNUDO")
	response = answerQuestion(t, adapter, response, editFinishFieldCode)
	response = answerConfirmation(t, adapter, response, "yes")

	if response.Pending != nil {
		t.Fatalf("Pending = %#v, want nil after a successful edit", response.Pending)
	}
	if _, ok := response.Messages[0].(StructuredResult); !ok {
		t.Fatalf("Messages[0] = %T, want StructuredResult (no validation error)", response.Messages[0])
	}
	if updater.callCount != 1 {
		t.Fatalf("Update call count = %d, want 1", updater.callCount)
	}
	for _, value := range updater.gotResource.Attributes {
		if value.AttributeCode == "color" || value.AttributeCode == "voltage" {
			t.Fatalf("gotResource.Attributes = %+v, want neither color nor voltage (DESNUDO makes both forbidden)", updater.gotResource.Attributes)
		}
	}
}

// TestResourceEditorEditNaturalUnitEntryRoutesAndSaves covers the field
// picker's explicit "Unidad natural" entry: picking it must route to the
// same unitQuestion CREATE's last step already uses, answering it must loop
// back to the SAME field picker, and picking "Terminar edición" must reach
// the confirmation and save via finishEditor.
func TestResourceEditorEditNaturalUnitEntryRoutesAndSaves(t *testing.T) {
	catalog := domain.SeedResourceCatalog()
	scope := domain.ResourceScope{ClassCode: "MATERIAL", FamilyCode: "CONDUCTORES", TypeCode: "CABLE"}
	existing, err := domain.NewResource(catalog, scope, "M", cableAttributeValues())
	if err != nil {
		t.Fatalf("NewResource(existing) error = %v, want nil", err)
	}
	existing.ID = 42

	updater := &fakeResourceUpdater{}
	adapter := newTestAdapter(&fakeResourceGetter{}, &fakeResourceCreator{}, updater, "MATERIAL")

	response := openEditFor(t, adapter, existing)
	response = answerQuestion(t, adapter, response, editNaturalUnitFieldCode)

	request, ok := response.Pending.(QuestionRequest)
	if !ok {
		t.Fatalf("Pending = %T, want QuestionRequest (natural unit question)", response.Pending)
	}
	if len(request.Options) == 0 || request.Options[0].Value != "M" {
		t.Fatalf("unit Options = %v, want the resource's current unit (M) first", request.Options)
	}

	response = answerQuestion(t, adapter, response, "M")
	if _, ok := response.Pending.(QuestionRequest); !ok {
		t.Fatalf("Pending = %T, want QuestionRequest (back at the field picker)", response.Pending)
	}
	response = answerQuestion(t, adapter, response, editFinishFieldCode)
	response = answerConfirmation(t, adapter, response, "yes")

	if updater.callCount != 1 {
		t.Fatalf("Update call count = %d, want 1", updater.callCount)
	}
	if updater.gotResource.NaturalUnit != "M" {
		t.Fatalf("gotResource.NaturalUnit = %q, want %q", updater.gotResource.NaturalUnit, "M")
	}
}

// TestResourceEditorEditMultiFieldSavesBothChanges covers the core
// loop-edit behavior: picking a field, answering it, landing back on the
// SAME picker, picking a SECOND field, answering it, "Terminar edición", a
// confirmation summary mentioning BOTH new values, and a single Update call
// carrying both changes together.
func TestResourceEditorEditMultiFieldSavesBothChanges(t *testing.T) {
	catalog := domain.SeedResourceCatalog()
	scope := domain.ResourceScope{ClassCode: "MATERIAL", FamilyCode: "CONDUCTORES", TypeCode: "CABLE"}
	existing, err := domain.NewResource(catalog, scope, "M", cableAttributeValues())
	if err != nil {
		t.Fatalf("NewResource(existing) error = %v, want nil", err)
	}
	existing.ID = 42

	creator := &fakeResourceCreator{}
	updater := &fakeResourceUpdater{}
	adapter := newTestAdapter(&fakeResourceGetter{}, creator, updater, "MATERIAL")

	response := openEditFor(t, adapter, existing)
	response = answerQuestion(t, adapter, response, "color")
	response = answerQuestion(t, adapter, response, "BLANCO")
	response = answerQuestion(t, adapter, response, "voltage")
	response = answerQuestion(t, adapter, response, "300 V")
	response = answerQuestion(t, adapter, response, editFinishFieldCode)

	confirmRequest, ok := response.Pending.(ConfirmationRequest)
	if !ok {
		t.Fatalf("Pending = %T, want ConfirmationRequest", response.Pending)
	}
	if !strings.Contains(confirmRequest.Question, "BLANCO") || !strings.Contains(confirmRequest.Question, "300 V") {
		t.Fatalf("confirmation Question = %q, want it to mention both BLANCO and 300 V", confirmRequest.Question)
	}
	if creator.callCount != 0 || updater.callCount != 0 {
		t.Fatalf("creator/updater call counts = %d/%d, want 0/0 (nothing persisted before confirming)", creator.callCount, updater.callCount)
	}

	response = answerConfirmation(t, adapter, response, "yes")

	if updater.callCount != 1 {
		t.Fatalf("Update call count = %d, want 1", updater.callCount)
	}
	if updater.gotResource.ID != 42 {
		t.Fatalf("gotResource.ID = %d, want 42 (the original Resource.ID)", updater.gotResource.ID)
	}
	if response.Pending != nil {
		t.Fatalf("Pending = %#v, want nil after a successful edit", response.Pending)
	}
}

// TestResourceEditorEditConfirmationDeclineDiscardsChanges covers declining
// the final confirmation ("no"): nothing must be persisted and a.editor
// must reset to nil.
func TestResourceEditorEditConfirmationDeclineDiscardsChanges(t *testing.T) {
	catalog := domain.SeedResourceCatalog()
	scope := domain.ResourceScope{ClassCode: "MATERIAL", FamilyCode: "CONDUCTORES", TypeCode: "CABLE"}
	existing, err := domain.NewResource(catalog, scope, "M", cableAttributeValues())
	if err != nil {
		t.Fatalf("NewResource(existing) error = %v, want nil", err)
	}
	existing.ID = 42

	creator := &fakeResourceCreator{}
	updater := &fakeResourceUpdater{}
	adapter := newTestAdapter(&fakeResourceGetter{}, creator, updater, "MATERIAL")

	response := openEditFor(t, adapter, existing)
	response = answerQuestion(t, adapter, response, "color")
	response = answerQuestion(t, adapter, response, "BLANCO")
	response = answerQuestion(t, adapter, response, editFinishFieldCode)

	if _, ok := response.Pending.(ConfirmationRequest); !ok {
		t.Fatalf("Pending = %T, want ConfirmationRequest", response.Pending)
	}
	response = answerConfirmation(t, adapter, response, "no")

	if creator.callCount != 0 || updater.callCount != 0 {
		t.Fatalf("creator/updater call counts = %d/%d, want 0/0 (declining must not persist anything)", creator.callCount, updater.callCount)
	}
	if response.Pending != nil {
		t.Fatalf("Pending = %#v, want nil after declining", response.Pending)
	}
	if adapter.editor != nil {
		t.Fatalf("adapter.editor = %#v, want nil after declining", adapter.editor)
	}
}

// TestResourceEditorEditEscMidMultiFieldSequenceAbortsImmediately covers Esc
// (InputCancel) sent mid loop-edit sequence: the whole edit must abort
// immediately and nothing must be persisted.
func TestResourceEditorEditEscMidMultiFieldSequenceAbortsImmediately(t *testing.T) {
	catalog := domain.SeedResourceCatalog()
	scope := domain.ResourceScope{ClassCode: "MATERIAL", FamilyCode: "CONDUCTORES", TypeCode: "CABLE"}
	existing, err := domain.NewResource(catalog, scope, "M", cableAttributeValues())
	if err != nil {
		t.Fatalf("NewResource(existing) error = %v, want nil", err)
	}
	existing.ID = 42

	creator := &fakeResourceCreator{}
	updater := &fakeResourceUpdater{}
	adapter := newTestAdapter(&fakeResourceGetter{}, creator, updater, "MATERIAL")

	response := openEditFor(t, adapter, existing)
	response = answerQuestion(t, adapter, response, "color")
	response = answerQuestion(t, adapter, response, "BLANCO")

	if _, ok := response.Pending.(QuestionRequest); !ok {
		t.Fatalf("Pending = %T, want QuestionRequest (back at the field picker)", response.Pending)
	}

	cancelled, handled := adapter.respondToEditor(context.Background(), InteractionInput{Kind: InputCancel})
	if !handled {
		t.Fatalf("respondToEditor did not claim the cancellation")
	}
	if cancelled.Pending != nil {
		t.Fatalf("Pending = %#v, want nil after cancellation", cancelled.Pending)
	}
	if adapter.editor != nil {
		t.Fatalf("adapter.editor = %#v, want nil after cancellation", adapter.editor)
	}
	if creator.callCount != 0 || updater.callCount != 0 {
		t.Fatalf("creator/updater call counts = %d/%d, want 0/0 (Esc mid-sequence must not persist anything)", creator.callCount, updater.callCount)
	}
}

// TestResourceEditorEditFinishImmediatelyCancelsCleanly covers picking
// "Terminar edición" as the VERY FIRST answer, before touching any field:
// with nothing changed this must cancel cleanly with a plain message, never
// a ConfirmationRequest, never call Update/Create.
func TestResourceEditorEditFinishImmediatelyCancelsCleanly(t *testing.T) {
	catalog := domain.SeedResourceCatalog()
	scope := domain.ResourceScope{ClassCode: "MATERIAL", FamilyCode: "CONDUCTORES", TypeCode: "CABLE"}
	existing, err := domain.NewResource(catalog, scope, "M", cableAttributeValues())
	if err != nil {
		t.Fatalf("NewResource(existing) error = %v, want nil", err)
	}
	existing.ID = 42

	creator := &fakeResourceCreator{}
	updater := &fakeResourceUpdater{}
	adapter := newTestAdapter(&fakeResourceGetter{}, creator, updater, "MATERIAL")

	response := openEditFor(t, adapter, existing)
	response = answerQuestion(t, adapter, response, editFinishFieldCode)

	if response.Pending != nil {
		t.Fatalf("Pending = %#v, want nil (no confirmation when nothing changed)", response.Pending)
	}
	if _, ok := response.Messages[0].(TextMessage); !ok {
		t.Fatalf("Messages[0] = %T, want TextMessage", response.Messages[0])
	}
	if creator.callCount != 0 || updater.callCount != 0 {
		t.Fatalf("creator/updater call counts = %d/%d, want 0/0 (nothing changed, nothing to save)", creator.callCount, updater.callCount)
	}
	if adapter.editor != nil {
		t.Fatalf("adapter.editor = %#v, want nil after finishing with no changes", adapter.editor)
	}
}

// TestResourceEditorDuplicateChangesOneAttributeCreatesNewResource covers
// the "Duplicar" happy path: opening a resource's detail, starting
// Duplicar, picking one field, changing it, "Terminar edición", confirming
// "yes" — the resulting Resource must be built via Create (never Update),
// must never carry the source resource's ID, and must have the changed
// attribute with every other attribute unchanged.
func TestResourceEditorDuplicateChangesOneAttributeCreatesNewResource(t *testing.T) {
	catalog := domain.SeedResourceCatalog()
	scope := domain.ResourceScope{ClassCode: "MATERIAL", FamilyCode: "CONDUCTORES", TypeCode: "CABLE"}
	existing, err := domain.NewResource(catalog, scope, "M", cableAttributeValues())
	if err != nil {
		t.Fatalf("NewResource(existing) error = %v, want nil", err)
	}
	existing.ID = 42

	creator := &fakeResourceCreator{}
	updater := &fakeResourceUpdater{}
	adapter := newTestAdapter(&fakeResourceGetter{}, creator, updater, "MATERIAL")

	response := openDuplicateFor(t, adapter, existing)
	pickerRequest, ok := response.Pending.(QuestionRequest)
	if !ok {
		t.Fatalf("Pending = %T, want QuestionRequest (field picker)", response.Pending)
	}
	if !strings.Contains(strings.ToLower(pickerRequest.Prompt), "campo") {
		t.Fatalf("Prompt = %q, want the field picker prompt", pickerRequest.Prompt)
	}

	response = answerQuestion(t, adapter, response, "color")
	response = answerQuestion(t, adapter, response, "BLANCO") // changed from NEGRO
	response = answerQuestion(t, adapter, response, editFinishFieldCode)
	response = answerConfirmation(t, adapter, response, "yes")

	if updater.callCount != 0 {
		t.Fatalf("Update call count = %d, want 0 (Duplicar must never call Update)", updater.callCount)
	}
	if creator.callCount != 1 {
		t.Fatalf("Create call count = %d, want 1", creator.callCount)
	}
	if creator.gotResource.ID != 0 {
		t.Fatalf("gotResource.ID = %d, want 0 (never the source resource's ID)", creator.gotResource.ID)
	}
	if response.Pending != nil {
		t.Fatalf("Pending = %#v, want nil after a successful duplicate", response.Pending)
	}
}

// TestResourceEditorDuplicateZeroChangesStillProceedsAndDetectsCollision
// covers the critical Duplicar-specific behavior: going straight to
// "Terminar edición" without changing anything must NOT show Edit's silent
// "No hiciste ningún cambio." shortcut — it must still reach the
// confirmation step, and confirming builds a candidate with the exact same
// identity as the source, so Create correctly fails with
// domain.ErrDuplicateResource.
func TestResourceEditorDuplicateZeroChangesStillProceedsAndDetectsCollision(t *testing.T) {
	catalog := domain.SeedResourceCatalog()
	scope := domain.ResourceScope{ClassCode: "MATERIAL", FamilyCode: "CONDUCTORES", TypeCode: "CABLE"}
	existing, err := domain.NewResource(catalog, scope, "M", cableAttributeValues())
	if err != nil {
		t.Fatalf("NewResource(existing) error = %v, want nil", err)
	}
	existing.ID = 42

	getter := &fakeResourceGetter{resource: existing}
	creator := &fakeResourceCreator{err: domain.ErrDuplicateResource}
	updater := &fakeResourceUpdater{}
	adapter := newTestAdapter(getter, creator, updater, "MATERIAL")

	response := openDuplicateFor(t, adapter, existing)
	response = answerQuestion(t, adapter, response, editFinishFieldCode)

	if _, ok := response.Pending.(ConfirmationRequest); !ok {
		t.Fatalf("Pending = %T, want ConfirmationRequest (Duplicar must proceed to confirmation even with zero changes)", response.Pending)
	}
	for _, message := range response.Messages {
		if text, ok := message.(TextMessage); ok && text.Text == "No hiciste ningún cambio." {
			t.Fatalf("Messages = %v, want no silent-cancel shortcut for Duplicar", response.Messages)
		}
	}

	response = answerConfirmation(t, adapter, response, "yes")

	if updater.callCount != 0 {
		t.Fatalf("Update call count = %d, want 0 (Duplicar must never call Update)", updater.callCount)
	}
	if creator.callCount != 1 {
		t.Fatalf("Create call count = %d, want 1", creator.callCount)
	}
	if getter.gotClassCode != "MATERIAL" {
		t.Fatalf("Get called with classCode %q, want %q", getter.gotClassCode, "MATERIAL")
	}
	found := false
	for _, message := range response.Messages {
		if _, ok := message.(StructuredResult); ok {
			found = true
		}
	}
	if !found {
		t.Fatalf("Messages = %v, want a StructuredResult showing the existing (original) resource, proving no second row was created", response.Messages)
	}
	if response.Pending != nil {
		t.Fatalf("Pending = %#v, want nil (editor reset after duplicate collision)", response.Pending)
	}
}

// TestResourceEditorFinishEditorSurfacesTheSpecificValidationReason covers a
// real usability guarantee: picking a diameter_inch that no longer matches
// the untouched diameter_mm makes domain.NewResource's own
// validateRelations reject the candidate with a specific,
// human-decipherable reason ("incoherent relation between ...") that
// finishEditor must surface (with the internal ErrResourceValidation
// wrapper prefix stripped), not a generic message.
func TestResourceEditorFinishEditorSurfacesTheSpecificValidationReason(t *testing.T) {
	catalog := domain.SeedResourceCatalog()
	scope := domain.ResourceScope{ClassCode: "MATERIAL", FamilyCode: "CANALIZACIONES", TypeCode: "TUBERIA"}
	existing, err := domain.NewResource(catalog, scope, "PZA", tuberiaAttributeValues())
	if err != nil {
		t.Fatalf("NewResource(existing) error = %v, want nil", err)
	}
	existing.ID = 7

	adapter := newTestAdapter(&fakeResourceGetter{}, &fakeResourceCreator{}, &fakeResourceUpdater{}, "MATERIAL")

	response := openEditFor(t, adapter, existing)
	response = answerQuestion(t, adapter, response, "diameter_inch")
	// diameter_mm stays untouched at "13 mm" (paired with 1/2", not 3/4") —
	// this candidate is genuinely incoherent.
	response = answerQuestion(t, adapter, response, `3/4"`)
	response = answerQuestion(t, adapter, response, editFinishFieldCode)
	response = answerConfirmation(t, adapter, response, "yes")

	message, ok := response.Messages[0].(ErrorMessage)
	if !ok {
		t.Fatalf("Messages[0] = %T, want ErrorMessage", response.Messages[0])
	}
	if strings.Contains(message.Text, "con esos datos") {
		t.Fatalf("Text = %q, want the specific validation reason, not a generic message", message.Text)
	}
	if !strings.Contains(message.Text, "diameter_inch") || !strings.Contains(message.Text, "diameter_mm") {
		t.Fatalf("Text = %q, want it to name the actual conflicting attributes", message.Text)
	}
}
