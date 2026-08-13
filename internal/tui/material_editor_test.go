package tui

import (
	"context"
	"strings"
	"testing"

	"github.com/GARFEX33/garfex-costos-unitarios/internal/domain"
)

// fakeMaterialCreator is the fake materialCreator used by every editor test
// (and the compile-time-satisfaction call sites in
// materials_workspace_adapter_test.go).
type fakeMaterialCreator struct {
	callCount   int
	gotMaterial domain.Material
	err         error
}

func (f *fakeMaterialCreator) Create(_ context.Context, material domain.Material) error {
	f.callCount++
	f.gotMaterial = material
	return f.err
}

// fakeMaterialUpdater is the fake materialUpdater used by the edit-flow
// tests (and the compile-time-satisfaction call sites in
// materials_workspace_adapter_test.go).
type fakeMaterialUpdater struct {
	callCount   int
	gotMaterial domain.Material
	err         error
}

func (f *fakeMaterialUpdater) Update(_ context.Context, material domain.Material) error {
	f.callCount++
	f.gotMaterial = material
	return f.err
}

// answerQuestion is a small test helper: it asserts response.Pending is a
// QuestionRequest keyed materialEditorKey and drives the next Respond call
// with that key and the given value.
func answerQuestion(t *testing.T, adapter *MaterialsWorkspaceAdapter, response InteractionResponse, value string) InteractionResponse {
	t.Helper()
	request, ok := response.Pending.(QuestionRequest)
	if !ok {
		t.Fatalf("Pending = %T, want QuestionRequest", response.Pending)
	}
	if request.Key != materialEditorKey {
		t.Fatalf("Pending.Key = %q, want %q", request.Key, materialEditorKey)
	}
	next, err := adapter.Respond(context.Background(), InteractionInput{Kind: InputSelection, Key: request.Key, Value: value})
	if err != nil {
		t.Fatalf("Respond(%q) error = %v, want nil", value, err)
	}
	return next
}

// answerMultiQuestion is answerQuestion's counterpart for the field picker's
// SelectionMultiple QuestionRequest: it asserts response.Pending is a
// multi-select QuestionRequest keyed materialEditorKey and drives the next
// Respond call with the given Values slice.
func answerMultiQuestion(t *testing.T, adapter *MaterialsWorkspaceAdapter, response InteractionResponse, values []string) InteractionResponse {
	t.Helper()
	request, ok := response.Pending.(QuestionRequest)
	if !ok {
		t.Fatalf("Pending = %T, want QuestionRequest", response.Pending)
	}
	if request.Key != materialEditorKey {
		t.Fatalf("Pending.Key = %q, want %q", request.Key, materialEditorKey)
	}
	if request.SelectionMode != SelectionMultiple {
		t.Fatalf("Pending.SelectionMode = %q, want %q", request.SelectionMode, SelectionMultiple)
	}
	next, err := adapter.Respond(context.Background(), InteractionInput{Kind: InputSelection, Key: request.Key, Values: values})
	if err != nil {
		t.Fatalf("Respond(%v) error = %v, want nil", values, err)
	}
	return next
}

// answerConfirmation is answerQuestion's counterpart for a ConfirmationRequest:
// it asserts response.Pending is a ConfirmationRequest keyed materialEditorKey
// and drives the next Respond call with the given value ("yes"/"no").
func answerConfirmation(t *testing.T, adapter *MaterialsWorkspaceAdapter, response InteractionResponse, value string) InteractionResponse {
	t.Helper()
	request, ok := response.Pending.(ConfirmationRequest)
	if !ok {
		t.Fatalf("Pending = %T, want ConfirmationRequest", response.Pending)
	}
	if request.Key != materialEditorKey {
		t.Fatalf("Pending.Key = %q, want %q", request.Key, materialEditorKey)
	}
	next, err := adapter.Respond(context.Background(), InteractionInput{Kind: InputSelection, Key: request.Key, Value: value})
	if err != nil {
		t.Fatalf("Respond(%q) error = %v, want nil", value, err)
	}
	return next
}

// TestMaterialEditorFullHappyPathCreatesCableMaterial covers the full
// CONDUCTORES/CABLE create flow end to end: trigger, family, productType,
// every attribute (in catalog order), unit, and a successful Create with
// the exact expected Material.
func TestMaterialEditorFullHappyPathCreatesCableMaterial(t *testing.T) {
	creator := &fakeMaterialCreator{}
	adapter := NewMaterialsWorkspaceAdapter(&fakeMaterialSearcher{}, &fakeMaterialGetter{}, &fakeMaterialDescriber{}, creator, &fakeMaterialUpdater{})

	response, err := adapter.Respond(context.Background(), InteractionInput{Kind: InputAction, ActionID: createMaterialActionID, Value: createMaterialActionID, Target: ActionTargetAgent})
	if err != nil {
		t.Fatalf("Respond(trigger) error = %v, want nil", err)
	}

	response = answerQuestion(t, adapter, response, "CONDUCTORES") // family
	response = answerQuestion(t, adapter, response, "CABLE")       // product type
	response = answerQuestion(t, adapter, response, "COBRE")       // conductor_material
	response = answerQuestion(t, adapter, response, "10 AWG")      // gauge
	response = answerQuestion(t, adapter, response, "THW")         // insulation (not DESNUDO)
	response = answerQuestion(t, adapter, response, "NEGRO")       // color

	// voltage is now a controlled option (Adjustment A): assert its
	// question offers exactly the 7 approved values, in catalog order,
	// before picking one — the same shape as any other controlled-option
	// step, not typed free text.
	voltageRequest, ok := response.Pending.(QuestionRequest)
	if !ok {
		t.Fatalf("Pending = %T, want QuestionRequest (voltage)", response.Pending)
	}
	wantVoltageOptions := []string{"300 V", "600 V", "1000 V", "5000 V", "15000 V", "25000 V", "35000 V"}
	if len(voltageRequest.Options) != len(wantVoltageOptions) {
		t.Fatalf("voltage Options = %v, want %v", voltageRequest.Options, wantVoltageOptions)
	}
	for i, want := range wantVoltageOptions {
		if voltageRequest.Options[i].Value != want {
			t.Fatalf("voltage Options[%d] = %q, want %q", i, voltageRequest.Options[i].Value, want)
		}
	}

	response = answerQuestion(t, adapter, response, "600 V") // voltage
	response = answerQuestion(t, adapter, response, "M")     // natural unit

	if len(response.Messages) != 1 {
		t.Fatalf("Messages = %v, want exactly one message", response.Messages)
	}
	if _, ok := response.Messages[0].(StructuredResult); !ok {
		t.Fatalf("Messages[0] = %T, want StructuredResult", response.Messages[0])
	}
	if response.Pending != nil {
		t.Fatalf("Pending = %#v, want nil after successful create", response.Pending)
	}
	if creator.callCount != 1 {
		t.Fatalf("Create call count = %d, want 1", creator.callCount)
	}

	catalog := domain.NewMaterialsCatalog()
	want, err := domain.NewMaterial(catalog, "CONDUCTORES", "CABLE", "M", []domain.MaterialAttributeValue{
		domain.OptionValue("conductor_material", "COBRE"),
		domain.OptionValue("gauge", "10 AWG"),
		domain.OptionValue("insulation", "THW"),
		domain.OptionValue("color", "NEGRO"),
		domain.OptionValue("voltage", "600 V"),
	})
	if err != nil {
		t.Fatalf("NewMaterial(want) error = %v, want nil", err)
	}
	if creator.gotMaterial.FamilyCode != want.FamilyCode {
		t.Fatalf("gotMaterial.FamilyCode = %q, want %q", creator.gotMaterial.FamilyCode, want.FamilyCode)
	}
	if creator.gotMaterial.ProductTypeCode != want.ProductTypeCode {
		t.Fatalf("gotMaterial.ProductTypeCode = %q, want %q", creator.gotMaterial.ProductTypeCode, want.ProductTypeCode)
	}
	if creator.gotMaterial.NaturalUnit != want.NaturalUnit {
		t.Fatalf("gotMaterial.NaturalUnit = %q, want %q", creator.gotMaterial.NaturalUnit, want.NaturalUnit)
	}
	if creator.gotMaterial.IdentityKey != want.IdentityKey {
		t.Fatalf("gotMaterial.IdentityKey = %q, want %q", creator.gotMaterial.IdentityKey, want.IdentityKey)
	}
	if len(creator.gotMaterial.Attributes) != len(want.Attributes) {
		t.Fatalf("gotMaterial.Attributes = %+v, want %+v", creator.gotMaterial.Attributes, want.Attributes)
	}
	for i := range want.Attributes {
		got, wantAttr := creator.gotMaterial.Attributes[i], want.Attributes[i]
		if got.AttributeCode != wantAttr.AttributeCode {
			t.Fatalf("Attributes[%d].AttributeCode = %q, want %q", i, got.AttributeCode, wantAttr.AttributeCode)
		}
	}
}

// TestMaterialEditorDesnudoSkipsColorAndVoltage covers the DESNUDO path:
// choosing insulation=DESNUDO must make the very next question the
// NaturalUnit question, never color or voltage — proving advanceEditor's
// FamilyAttribute.Effective-driven skip, not a CABLE-specific check.
func TestMaterialEditorDesnudoSkipsColorAndVoltage(t *testing.T) {
	adapter := NewMaterialsWorkspaceAdapter(&fakeMaterialSearcher{}, &fakeMaterialGetter{}, &fakeMaterialDescriber{}, &fakeMaterialCreator{}, &fakeMaterialUpdater{})

	response, err := adapter.Respond(context.Background(), InteractionInput{Kind: InputAction, ActionID: createMaterialActionID, Value: createMaterialActionID, Target: ActionTargetAgent})
	if err != nil {
		t.Fatalf("Respond(trigger) error = %v, want nil", err)
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
	for _, option := range request.Options {
		if option.Value == "NEGRO" || option.Value == "BLANCO" {
			t.Fatalf("Options after DESNUDO = %v, want no color options (color must be skipped)", request.Options)
		}
	}
}

// TestMaterialEditorNarrowsDiameterMmByDiameterInch covers the TUBERIA
// narrowing path: after choosing diameter_inch=1/2", the diameter_mm
// question's Options must contain exactly the related option (13 mm) — this
// proves ValidOptions integration, not a CABLE/TUBERIA-specific check.
func TestMaterialEditorNarrowsDiameterMmByDiameterInch(t *testing.T) {
	adapter := NewMaterialsWorkspaceAdapter(&fakeMaterialSearcher{}, &fakeMaterialGetter{}, &fakeMaterialDescriber{}, &fakeMaterialCreator{}, &fakeMaterialUpdater{})

	response, err := adapter.Respond(context.Background(), InteractionInput{Kind: InputAction, ActionID: createMaterialActionID, Value: createMaterialActionID, Target: ActionTargetAgent})
	if err != nil {
		t.Fatalf("Respond(trigger) error = %v, want nil", err)
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

// TestMaterialEditorDuplicateIdentityShowsExistingMaterial covers §6/finishEditor's
// ErrDuplicateMaterial path: when Create reports a collision, the response
// shows the pre-existing material's detail (via materialsGetter.Get on the
// candidate's own identity), not an opaque failure.
func TestMaterialEditorDuplicateIdentityShowsExistingMaterial(t *testing.T) {
	existing := domain.Material{
		FamilyCode: "CONDUCTORES", ProductTypeCode: "CABLE", NaturalUnit: "M",
		IdentityKey: "CONDUCTORES|CABLE|color=NEGRO|conductor_material=COBRE|gauge=10 AWG|insulation=THW|voltage=600 V",
		Attributes:  []domain.MaterialAttributeValue{domain.OptionValue("insulation", "THW")},
	}
	creator := &fakeMaterialCreator{err: domain.ErrDuplicateMaterial}
	getter := &fakeMaterialGetter{material: existing}
	adapter := NewMaterialsWorkspaceAdapter(&fakeMaterialSearcher{}, getter, &fakeMaterialDescriber{}, creator, &fakeMaterialUpdater{})

	response, err := adapter.Respond(context.Background(), InteractionInput{Kind: InputAction, ActionID: createMaterialActionID, Value: createMaterialActionID, Target: ActionTargetAgent})
	if err != nil {
		t.Fatalf("Respond(trigger) error = %v, want nil", err)
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
	if getter.gotFamilyCode != "CONDUCTORES" {
		t.Fatalf("Get called with familyCode %q, want %q", getter.gotFamilyCode, "CONDUCTORES")
	}
	found := false
	for _, message := range response.Messages {
		if result, ok := message.(StructuredResult); ok {
			found = true
			if !strings.HasPrefix(result.Title, "CONDUCTORES") {
				t.Fatalf("existing material Title = %q, want it to start with %q", result.Title, "CONDUCTORES")
			}
		}
	}
	if !found {
		t.Fatalf("Messages = %v, want a StructuredResult showing the existing material", response.Messages)
	}
	if response.Pending != nil {
		t.Fatalf("Pending = %#v, want nil (editor reset after duplicate)", response.Pending)
	}
}

// TestMaterialEditorCancelResetsAndOrdinarySearchStillWorks covers
// cancellation mid-flow: InputCancel resets a.editor, and a subsequent
// ordinary text search is handled normally afterward, not swallowed by
// stale editor state.
func TestMaterialEditorCancelResetsAndOrdinarySearchStillWorks(t *testing.T) {
	fake := &fakeMaterialSearcher{results: []domain.Material{
		{FamilyCode: "CEMENT", NaturalUnit: "kg", IdentityKey: "CEMENT|kg|1"},
	}}
	adapter := NewMaterialsWorkspaceAdapter(fake, &fakeMaterialGetter{}, &fakeMaterialDescriber{}, &fakeMaterialCreator{}, &fakeMaterialUpdater{})

	response, err := adapter.Respond(context.Background(), InteractionInput{Kind: InputAction, ActionID: createMaterialActionID, Value: createMaterialActionID, Target: ActionTargetAgent})
	if err != nil {
		t.Fatalf("Respond(trigger) error = %v, want nil", err)
	}
	response = answerQuestion(t, adapter, response, "CONDUCTORES")
	if response.Pending == nil {
		t.Fatalf("Pending = nil, want the productType question still in progress")
	}

	cancelled, err := adapter.Respond(context.Background(), InteractionInput{Kind: InputCancel})
	if err != nil {
		t.Fatalf("Respond(cancel) error = %v, want nil", err)
	}
	if cancelled.Pending != nil {
		t.Fatalf("Pending = %#v, want nil after cancellation", cancelled.Pending)
	}
	if adapter.editor != nil {
		t.Fatalf("adapter.editor = %#v, want nil after cancellation", adapter.editor)
	}

	searchResponse, err := adapter.Respond(context.Background(), InteractionInput{Kind: InputText, Value: "cemento"})
	if err != nil {
		t.Fatalf("Respond(search) error = %v, want nil", err)
	}
	if fake.callCount != 1 {
		t.Fatalf("Search call count = %d, want 1 (a fresh search after cancel must work normally)", fake.callCount)
	}
	if fake.gotCriteria.Text != "cemento" {
		t.Fatalf("gotCriteria.Text = %q, want %q", fake.gotCriteria.Text, "cemento")
	}
	if _, ok := searchResponse.Pending.(QuestionRequest); !ok {
		t.Fatalf("Pending = %T, want a normal search-results QuestionRequest", searchResponse.Pending)
	}
}

// TestMaterialEditorRegressionOrdinarySearchUnaffected is a smoke-level
// regression guard: with no editor ever started, ordinary search behavior
// (covered extensively in materials_workspace_adapter_test.go) is unaffected
// by this change.
func TestMaterialEditorRegressionOrdinarySearchUnaffected(t *testing.T) {
	fake := &fakeMaterialSearcher{results: []domain.Material{
		{FamilyCode: "CEMENT", NaturalUnit: "kg", IdentityKey: "CEMENT|kg|1"},
	}}
	adapter := NewMaterialsWorkspaceAdapter(fake, &fakeMaterialGetter{}, &fakeMaterialDescriber{}, &fakeMaterialCreator{}, &fakeMaterialUpdater{})
	response, err := adapter.Respond(context.Background(), InteractionInput{Kind: InputText, Value: "cemento"})
	if err != nil {
		t.Fatalf("Respond(search) error = %v, want nil", err)
	}
	if fake.gotCriteria.Text != "cemento" {
		t.Fatalf("gotCriteria.Text = %q, want %q", fake.gotCriteria.Text, "cemento")
	}
	if _, ok := response.Pending.(QuestionRequest); !ok {
		t.Fatalf("Pending = %T, want QuestionRequest", response.Pending)
	}
}

// openDetailViaSearch drives a text search followed by selecting its one
// result, exactly the way production code reaches a.lastDetail — used by
// the edit-flow tests below instead of reaching into the adapter's private
// field directly.
func openDetailViaSearch(t *testing.T, adapter *MaterialsWorkspaceAdapter, material domain.Material) {
	t.Helper()
	searchResponse, err := adapter.Respond(context.Background(), InteractionInput{Kind: InputText, Value: "buscar"})
	if err != nil {
		t.Fatalf("Respond(search) error = %v, want nil", err)
	}
	searchRequest, ok := searchResponse.Pending.(QuestionRequest)
	if !ok {
		t.Fatalf("Pending = %T, want QuestionRequest (search results)", searchResponse.Pending)
	}
	if len(searchRequest.Options) != 1 {
		t.Fatalf("search Options = %v, want exactly 1", searchRequest.Options)
	}
	detailResponse, err := adapter.Respond(context.Background(), InteractionInput{Kind: InputSelection, Key: searchResultsKey, Value: searchRequest.Options[0].Value})
	if err != nil {
		t.Fatalf("Respond(select) error = %v, want nil", err)
	}
	if _, ok := detailResponse.Pending.(ActionRequest); !ok {
		t.Fatalf("Pending = %T, want ActionRequest (detail actions)", detailResponse.Pending)
	}
}

// cableAttributeValues is the shared CONDUCTORES/CABLE attribute set used by
// the edit-flow tests, in catalog declaration order.
func cableAttributeValues() []domain.MaterialAttributeValue {
	return []domain.MaterialAttributeValue{
		domain.OptionValue("conductor_material", "COBRE"),
		domain.OptionValue("gauge", "10 AWG"),
		domain.OptionValue("insulation", "THW"),
		domain.OptionValue("color", "NEGRO"),
		domain.OptionValue("voltage", "600 V"),
	}
}

// TestMaterialEditorEditFullHappyPathReproducesSameMaterial covers the new
// picker-based EDIT flow end to end: opening a CONDUCTORES/CABLE material's
// detail, selecting "Editar", confirming the wizard opens on the field
// picker (never Family/ProductType), picking ONE field, confirming its
// question defaults to the material's current value, and confirming that
// answering with that SAME value calls updater.Update (never
// creator.Create) with the original Material.ID preserved and the exact
// same IdentityKey — proving a no-op single-field edit truly changes
// nothing.
func TestMaterialEditorEditFullHappyPathReproducesSameMaterial(t *testing.T) {
	catalog := domain.NewMaterialsCatalog()
	existing, err := domain.NewMaterial(catalog, "CONDUCTORES", "CABLE", "M", cableAttributeValues())
	if err != nil {
		t.Fatalf("NewMaterial(existing) error = %v, want nil", err)
	}
	existing.ID = 42

	getter := &fakeMaterialGetter{material: existing}
	creator := &fakeMaterialCreator{}
	updater := &fakeMaterialUpdater{}
	adapter := NewMaterialsWorkspaceAdapter(&fakeMaterialSearcher{results: []domain.Material{existing}}, getter, &fakeMaterialDescriber{}, creator, updater)

	openDetailViaSearch(t, adapter, existing)

	response, err := adapter.Respond(context.Background(), InteractionInput{Kind: InputAction, ActionID: editActionID, Value: editActionID, Target: ActionTargetAgent})
	if err != nil {
		t.Fatalf("Respond(edit) error = %v, want nil", err)
	}

	pickerRequest, ok := response.Pending.(QuestionRequest)
	if !ok {
		t.Fatalf("Pending = %T, want QuestionRequest (field picker)", response.Pending)
	}
	if !strings.Contains(strings.ToLower(pickerRequest.Prompt), "campo") {
		t.Fatalf("Prompt = %q, want the field picker prompt", pickerRequest.Prompt)
	}

	response = answerMultiQuestion(t, adapter, response, []string{"conductor_material"})

	request, ok := response.Pending.(QuestionRequest)
	if !ok {
		t.Fatalf("Pending = %T, want QuestionRequest (conductor_material question)", response.Pending)
	}
	if len(request.Options) == 0 || request.Options[0].Value != "COBRE" {
		t.Fatalf("conductor_material Options = %v, want the material's current value (COBRE) first", request.Options)
	}

	response = answerQuestion(t, adapter, response, "COBRE") // unchanged
	response = answerConfirmation(t, adapter, response, "yes")

	if creator.callCount != 0 {
		t.Fatalf("Create call count = %d, want 0 (edit must never call Create)", creator.callCount)
	}
	if updater.callCount != 1 {
		t.Fatalf("Update call count = %d, want 1", updater.callCount)
	}
	if updater.gotMaterial.ID != 42 {
		t.Fatalf("gotMaterial.ID = %d, want 42 (the original Material.ID)", updater.gotMaterial.ID)
	}
	if updater.gotMaterial.IdentityKey != existing.IdentityKey {
		t.Fatalf("gotMaterial.IdentityKey = %q, want %q (an unchanged answer reproduces the same identity)", updater.gotMaterial.IdentityKey, existing.IdentityKey)
	}
	if len(updater.gotMaterial.Attributes) != len(existing.Attributes) {
		t.Fatalf("gotMaterial.Attributes = %+v, want %+v (byte-identical to the original)", updater.gotMaterial.Attributes, existing.Attributes)
	}
	if response.Pending != nil {
		t.Fatalf("Pending = %#v, want nil after a successful edit", response.Pending)
	}
	if len(response.Messages) != 1 {
		t.Fatalf("Messages = %v, want exactly one message", response.Messages)
	}
	if _, ok := response.Messages[0].(StructuredResult); !ok {
		t.Fatalf("Messages[0] = %T, want StructuredResult", response.Messages[0])
	}
}

// TestMaterialEditorEditChangesOneAttribute covers the field picker's menu
// shape (it must list the material's actual fields with their CURRENT
// values, including a "Unidad natural" entry) and picking one field
// (color) with a DIFFERENT answer (NEGRO -> BLANCO): the final Update call
// must carry the new value with every OTHER attribute identical to the
// original, the original Material.ID must still be preserved, and the
// resulting IdentityKey must differ from the original.
func TestMaterialEditorEditChangesOneAttribute(t *testing.T) {
	catalog := domain.NewMaterialsCatalog()
	existing, err := domain.NewMaterial(catalog, "CONDUCTORES", "CABLE", "M", cableAttributeValues())
	if err != nil {
		t.Fatalf("NewMaterial(existing) error = %v, want nil", err)
	}
	existing.ID = 42

	getter := &fakeMaterialGetter{material: existing}
	updater := &fakeMaterialUpdater{}
	adapter := NewMaterialsWorkspaceAdapter(&fakeMaterialSearcher{results: []domain.Material{existing}}, getter, &fakeMaterialDescriber{}, &fakeMaterialCreator{}, updater)

	openDetailViaSearch(t, adapter, existing)
	response, err := adapter.Respond(context.Background(), InteractionInput{Kind: InputAction, ActionID: editActionID, Value: editActionID, Target: ActionTargetAgent})
	if err != nil {
		t.Fatalf("Respond(edit) error = %v, want nil", err)
	}

	pickerRequest, ok := response.Pending.(QuestionRequest)
	if !ok {
		t.Fatalf("Pending = %T, want QuestionRequest (field picker)", response.Pending)
	}
	wantLabels := map[string]string{
		"conductor_material":     "Conductor material: COBRE",
		"gauge":                  "Gauge: 10 AWG",
		"insulation":             "Insulation: THW",
		"color":                  "Color: NEGRO",
		"voltage":                "Voltage: 600 V",
		editNaturalUnitFieldCode: "Unidad natural: M",
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

	response = answerMultiQuestion(t, adapter, response, []string{"color"})
	response = answerQuestion(t, adapter, response, "BLANCO") // changed from NEGRO
	response = answerConfirmation(t, adapter, response, "yes")

	if updater.callCount != 1 {
		t.Fatalf("Update call count = %d, want 1", updater.callCount)
	}
	if updater.gotMaterial.ID != 42 {
		t.Fatalf("gotMaterial.ID = %d, want 42 (the original Material.ID)", updater.gotMaterial.ID)
	}
	if updater.gotMaterial.IdentityKey == existing.IdentityKey {
		t.Fatalf("gotMaterial.IdentityKey = %q, want it to differ from the original %q after changing color", updater.gotMaterial.IdentityKey, existing.IdentityKey)
	}
	if len(updater.gotMaterial.Attributes) != len(existing.Attributes) {
		t.Fatalf("gotMaterial.Attributes = %+v, want the same %d attributes as the original (only color's value should differ)", updater.gotMaterial.Attributes, len(existing.Attributes))
	}
	for _, value := range updater.gotMaterial.Attributes {
		if value.AttributeCode == "color" {
			if value.OptionCode != "BLANCO" {
				t.Fatalf("color attribute = %q, want %q", value.OptionCode, "BLANCO")
			}
			continue
		}
		var want string
		for _, original := range existing.Attributes {
			if original.AttributeCode == value.AttributeCode {
				want = original.OptionCode
			}
		}
		if value.OptionCode != want {
			t.Fatalf("%s attribute = %q, want unchanged %q", value.AttributeCode, value.OptionCode, want)
		}
	}
	if response.Pending != nil {
		t.Fatalf("Pending = %#v, want nil after a successful edit", response.Pending)
	}
}

// TestMaterialEditorEditDuplicateIdentityShowsExistingMaterial covers
// finishEditor's ErrDuplicateMaterial path for EDIT through the new 2-step
// (picker -> single answer) flow: when Update reports a collision with a
// DIFFERENT already-existing material, the response must show that other
// material's detail — never a bare failure, never a silent overwrite of
// either material.
func TestMaterialEditorEditDuplicateIdentityShowsExistingMaterial(t *testing.T) {
	catalog := domain.NewMaterialsCatalog()
	editing, err := domain.NewMaterial(catalog, "CONDUCTORES", "CABLE", "M", cableAttributeValues())
	if err != nil {
		t.Fatalf("NewMaterial(editing) error = %v, want nil", err)
	}
	editing.ID = 42

	other := domain.Material{
		FamilyCode: "CONDUCTORES", ProductTypeCode: "CABLE", NaturalUnit: "M",
		IdentityKey: "CONDUCTORES|CABLE|color=BLANCO|conductor_material=COBRE|gauge=10 AWG|insulation=THW|voltage=600 V",
		Attributes:  []domain.MaterialAttributeValue{domain.OptionValue("insulation", "THW")},
	}

	getter := &fakeMaterialGetter{material: editing}
	updater := &fakeMaterialUpdater{err: domain.ErrDuplicateMaterial}
	adapter := NewMaterialsWorkspaceAdapter(&fakeMaterialSearcher{results: []domain.Material{editing}}, getter, &fakeMaterialDescriber{}, &fakeMaterialCreator{}, updater)

	openDetailViaSearch(t, adapter, editing)
	response, err := adapter.Respond(context.Background(), InteractionInput{Kind: InputAction, ActionID: editActionID, Value: editActionID, Target: ActionTargetAgent})
	if err != nil {
		t.Fatalf("Respond(edit) error = %v, want nil", err)
	}

	// getter.material now switches to the OTHER pre-existing material that
	// the edited identity collides with, mirroring how the real repository
	// would resolve a Get on the new (colliding) identity — the original
	// material's attributes are already captured in the editor's state by
	// this point (see startEditEditor's values), so this mutation only
	// affects the later duplicate-lookup Get call in finishEditor.
	getter.material = other

	response = answerMultiQuestion(t, adapter, response, []string{"color"})
	response = answerQuestion(t, adapter, response, "BLANCO")
	response = answerConfirmation(t, adapter, response, "yes")

	if updater.callCount != 1 {
		t.Fatalf("Update call count = %d, want 1", updater.callCount)
	}
	if getter.callCount != 2 {
		t.Fatalf("Get call count = %d, want 2 (detail open + duplicate lookup)", getter.callCount)
	}
	found := false
	for _, message := range response.Messages {
		if result, ok := message.(StructuredResult); ok {
			found = true
			if !strings.HasPrefix(result.Title, "CONDUCTORES") {
				t.Fatalf("existing material Title = %q, want it to start with %q", result.Title, "CONDUCTORES")
			}
		}
	}
	if !found {
		t.Fatalf("Messages = %v, want a StructuredResult showing the other existing material", response.Messages)
	}
	if response.Pending != nil {
		t.Fatalf("Pending = %#v, want nil (editor reset after duplicate)", response.Pending)
	}
}

// TestMaterialEditorEditDesnudoDropsNowForbiddenAttributes covers the real
// correctness gap the single-field edit flow introduces that CREATE's
// sequential walkthrough never had (see finishEditor's filterApplicableValues):
// state.values is seeded from the material's FULL existing Attributes, so
// editing insulation from THW to DESNUDO leaves the pre-existing color and
// voltage values still sitting in state.values even though DESNUDO makes
// both of them ModeForbidden/notApplicable per the catalog's own rule.
// Passing them straight into domain.NewMaterial would reject the whole edit
// ("attribute %q is forbidden"). This proves the edit instead completes
// successfully with color/voltage silently dropped, and the resulting
// IdentityKey has no color=/voltage= segments.
func TestMaterialEditorEditDesnudoDropsNowForbiddenAttributes(t *testing.T) {
	catalog := domain.NewMaterialsCatalog()
	existing, err := domain.NewMaterial(catalog, "CONDUCTORES", "CABLE", "M", cableAttributeValues())
	if err != nil {
		t.Fatalf("NewMaterial(existing) error = %v, want nil", err)
	}
	existing.ID = 42

	getter := &fakeMaterialGetter{material: existing}
	updater := &fakeMaterialUpdater{}
	adapter := NewMaterialsWorkspaceAdapter(&fakeMaterialSearcher{results: []domain.Material{existing}}, getter, &fakeMaterialDescriber{}, &fakeMaterialCreator{}, updater)

	openDetailViaSearch(t, adapter, existing)
	response, err := adapter.Respond(context.Background(), InteractionInput{Kind: InputAction, ActionID: editActionID, Value: editActionID, Target: ActionTargetAgent})
	if err != nil {
		t.Fatalf("Respond(edit) error = %v, want nil", err)
	}

	response = answerMultiQuestion(t, adapter, response, []string{"insulation"})
	response = answerQuestion(t, adapter, response, "DESNUDO")
	response = answerConfirmation(t, adapter, response, "yes")

	if response.Pending != nil {
		t.Fatalf("Pending = %#v, want nil after a successful edit", response.Pending)
	}
	if len(response.Messages) != 1 {
		t.Fatalf("Messages = %v, want exactly one message (no validation error)", response.Messages)
	}
	if _, ok := response.Messages[0].(StructuredResult); !ok {
		t.Fatalf("Messages[0] = %T, want StructuredResult (no validation error)", response.Messages[0])
	}
	if updater.callCount != 1 {
		t.Fatalf("Update call count = %d, want 1", updater.callCount)
	}
	for _, value := range updater.gotMaterial.Attributes {
		if value.AttributeCode == "color" || value.AttributeCode == "voltage" {
			t.Fatalf("gotMaterial.Attributes = %+v, want neither color nor voltage (DESNUDO makes both forbidden)", updater.gotMaterial.Attributes)
		}
	}
	if strings.Contains(updater.gotMaterial.IdentityKey, "color=") || strings.Contains(updater.gotMaterial.IdentityKey, "voltage=") {
		t.Fatalf("gotMaterial.IdentityKey = %q, want no color=/voltage= segments", updater.gotMaterial.IdentityKey)
	}
	foundInsulation := false
	for _, value := range updater.gotMaterial.Attributes {
		if value.AttributeCode == "insulation" {
			foundInsulation = true
			if value.OptionCode != "DESNUDO" {
				t.Fatalf("insulation attribute = %q, want %q", value.OptionCode, "DESNUDO")
			}
		}
	}
	if !foundInsulation {
		t.Fatalf("gotMaterial.Attributes = %+v, want an insulation attribute", updater.gotMaterial.Attributes)
	}
}

// TestMaterialEditorEditNaturalUnitEntryRoutesToUnitQuestionAndSaves covers
// the field picker's explicit "Unidad natural" entry: picking it must route
// to exactly the same unitQuestion CREATE's last step already uses (reused
// unchanged, see answerAttributePicker), and answering it must save via
// finishEditor. The real catalog declares only ONE allowed NaturalUnit per
// family today (CONDUCTORES -> "M" only — see NewMaterialsCatalog's single
// UnitPolicies entry for CONDUCTORES), so there is no second value the real
// catalog would accept here; this test can only exercise "pick the current
// unit again" (a no-op save), not a genuinely different unit. It still
// proves the routing (editorStepUnit reached via the picker, not only via
// CREATE's sequential walkthrough) and that finishEditor is reached and
// succeeds.
func TestMaterialEditorEditNaturalUnitEntryRoutesToUnitQuestionAndSaves(t *testing.T) {
	catalog := domain.NewMaterialsCatalog()
	existing, err := domain.NewMaterial(catalog, "CONDUCTORES", "CABLE", "M", cableAttributeValues())
	if err != nil {
		t.Fatalf("NewMaterial(existing) error = %v, want nil", err)
	}
	existing.ID = 42

	getter := &fakeMaterialGetter{material: existing}
	updater := &fakeMaterialUpdater{}
	adapter := NewMaterialsWorkspaceAdapter(&fakeMaterialSearcher{results: []domain.Material{existing}}, getter, &fakeMaterialDescriber{}, &fakeMaterialCreator{}, updater)

	openDetailViaSearch(t, adapter, existing)
	response, err := adapter.Respond(context.Background(), InteractionInput{Kind: InputAction, ActionID: editActionID, Value: editActionID, Target: ActionTargetAgent})
	if err != nil {
		t.Fatalf("Respond(edit) error = %v, want nil", err)
	}

	response = answerMultiQuestion(t, adapter, response, []string{editNaturalUnitFieldCode})

	request, ok := response.Pending.(QuestionRequest)
	if !ok {
		t.Fatalf("Pending = %T, want QuestionRequest (natural unit question)", response.Pending)
	}
	if !strings.Contains(strings.ToLower(request.Prompt), "unidad") {
		t.Fatalf("Prompt = %q, want the NaturalUnit question", request.Prompt)
	}
	if len(request.Options) == 0 || request.Options[0].Value != "M" {
		t.Fatalf("unit Options = %v, want the material's current unit (M) first", request.Options)
	}

	response = answerQuestion(t, adapter, response, "M")
	response = answerConfirmation(t, adapter, response, "yes")

	if updater.callCount != 1 {
		t.Fatalf("Update call count = %d, want 1", updater.callCount)
	}
	if updater.gotMaterial.NaturalUnit != "M" {
		t.Fatalf("gotMaterial.NaturalUnit = %q, want %q", updater.gotMaterial.NaturalUnit, "M")
	}
	if updater.gotMaterial.IdentityKey != existing.IdentityKey {
		t.Fatalf("gotMaterial.IdentityKey = %q, want %q (natural unit is not part of identity)", updater.gotMaterial.IdentityKey, existing.IdentityKey)
	}
	if response.Pending != nil {
		t.Fatalf("Pending = %#v, want nil after a successful edit", response.Pending)
	}
}

// TestMaterialEditorEditMultiFieldHappyPathSavesBothChanges covers the core
// new behavior this task adds: picking TWO fields (color, voltage) from the
// picker's checkbox menu, answering each in turn, seeing a confirmation
// summary that mentions BOTH new values, and only then persisting — a single
// Update call carrying both changes together.
func TestMaterialEditorEditMultiFieldHappyPathSavesBothChanges(t *testing.T) {
	catalog := domain.NewMaterialsCatalog()
	existing, err := domain.NewMaterial(catalog, "CONDUCTORES", "CABLE", "M", cableAttributeValues())
	if err != nil {
		t.Fatalf("NewMaterial(existing) error = %v, want nil", err)
	}
	existing.ID = 42

	getter := &fakeMaterialGetter{material: existing}
	creator := &fakeMaterialCreator{}
	updater := &fakeMaterialUpdater{}
	adapter := NewMaterialsWorkspaceAdapter(&fakeMaterialSearcher{results: []domain.Material{existing}}, getter, &fakeMaterialDescriber{}, creator, updater)

	openDetailViaSearch(t, adapter, existing)
	response, err := adapter.Respond(context.Background(), InteractionInput{Kind: InputAction, ActionID: editActionID, Value: editActionID, Target: ActionTargetAgent})
	if err != nil {
		t.Fatalf("Respond(edit) error = %v, want nil", err)
	}

	response = answerMultiQuestion(t, adapter, response, []string{"color", "voltage"})
	response = answerQuestion(t, adapter, response, "BLANCO") // color, changed from NEGRO
	response = answerQuestion(t, adapter, response, "300 V")  // voltage, changed from 600 V

	confirmRequest, ok := response.Pending.(ConfirmationRequest)
	if !ok {
		t.Fatalf("Pending = %T, want ConfirmationRequest", response.Pending)
	}
	if !strings.Contains(confirmRequest.Question, "BLANCO") {
		t.Fatalf("confirmation Question = %q, want it to mention BLANCO", confirmRequest.Question)
	}
	if !strings.Contains(confirmRequest.Question, "300 V") {
		t.Fatalf("confirmation Question = %q, want it to mention 300 V", confirmRequest.Question)
	}
	if creator.callCount != 0 || updater.callCount != 0 {
		t.Fatalf("creator/updater call counts = %d/%d, want 0/0 (nothing persisted before confirming)", creator.callCount, updater.callCount)
	}

	response = answerConfirmation(t, adapter, response, "yes")

	if creator.callCount != 0 {
		t.Fatalf("Create call count = %d, want 0 (edit must never call Create)", creator.callCount)
	}
	if updater.callCount != 1 {
		t.Fatalf("Update call count = %d, want 1", updater.callCount)
	}
	if updater.gotMaterial.ID != 42 {
		t.Fatalf("gotMaterial.ID = %d, want 42 (the original Material.ID)", updater.gotMaterial.ID)
	}
	for _, value := range updater.gotMaterial.Attributes {
		switch value.AttributeCode {
		case "color":
			if value.OptionCode != "BLANCO" {
				t.Fatalf("color attribute = %q, want %q", value.OptionCode, "BLANCO")
			}
		case "voltage":
			if value.OptionCode != "300 V" {
				t.Fatalf("voltage attribute = %q, want %q", value.OptionCode, "300 V")
			}
		case "conductor_material", "gauge", "insulation":
			var want string
			for _, original := range existing.Attributes {
				if original.AttributeCode == value.AttributeCode {
					want = original.OptionCode
				}
			}
			if value.OptionCode != want {
				t.Fatalf("%s attribute = %q, want unchanged %q", value.AttributeCode, value.OptionCode, want)
			}
		default:
			t.Fatalf("unexpected attribute %q in gotMaterial.Attributes", value.AttributeCode)
		}
	}
	if response.Pending != nil {
		t.Fatalf("Pending = %#v, want nil after a successful edit", response.Pending)
	}
}

// TestMaterialEditorEditConfirmationDeclineDiscardsChanges covers declining
// the final confirmation ("no"): nothing must be persisted, a.editor must
// reset to nil, and a subsequent ordinary search must still work normally —
// mirroring TestMaterialEditorCancelResetsAndOrdinarySearchStillWorks's
// pattern for InputCancel.
func TestMaterialEditorEditConfirmationDeclineDiscardsChanges(t *testing.T) {
	catalog := domain.NewMaterialsCatalog()
	existing, err := domain.NewMaterial(catalog, "CONDUCTORES", "CABLE", "M", cableAttributeValues())
	if err != nil {
		t.Fatalf("NewMaterial(existing) error = %v, want nil", err)
	}
	existing.ID = 42

	getter := &fakeMaterialGetter{material: existing}
	creator := &fakeMaterialCreator{}
	updater := &fakeMaterialUpdater{}
	searcher := &fakeMaterialSearcher{results: []domain.Material{existing}}
	adapter := NewMaterialsWorkspaceAdapter(searcher, getter, &fakeMaterialDescriber{}, creator, updater)

	openDetailViaSearch(t, adapter, existing)
	response, err := adapter.Respond(context.Background(), InteractionInput{Kind: InputAction, ActionID: editActionID, Value: editActionID, Target: ActionTargetAgent})
	if err != nil {
		t.Fatalf("Respond(edit) error = %v, want nil", err)
	}

	response = answerMultiQuestion(t, adapter, response, []string{"color", "voltage"})
	response = answerQuestion(t, adapter, response, "BLANCO")
	response = answerQuestion(t, adapter, response, "300 V")

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

	searcher.results = []domain.Material{
		{FamilyCode: "CEMENT", NaturalUnit: "kg", IdentityKey: "CEMENT|kg|1"},
	}
	searchResponse, err := adapter.Respond(context.Background(), InteractionInput{Kind: InputText, Value: "cemento"})
	if err != nil {
		t.Fatalf("Respond(search) error = %v, want nil", err)
	}
	if searcher.callCount != 2 {
		t.Fatalf("Search call count = %d, want 2 (openDetailViaSearch's search + a fresh search after decline)", searcher.callCount)
	}
	if _, ok := searchResponse.Pending.(QuestionRequest); !ok {
		t.Fatalf("Pending = %T, want a normal search-results QuestionRequest", searchResponse.Pending)
	}
}

// TestMaterialEditorEditMultiFieldIncludingNaturalUnitThreadsCurrentUnit
// covers picking an attribute AND the "Unidad natural" entry together: both
// must be asked about, and the (possibly unchanged) unit answer must still
// land correctly in the final persisted material via
// materialEditorState.currentUnit — proving the unit is threaded through to
// finishEditor regardless of exactly where NaturalUnit falls in the
// answered sequence, not just when it happens to be CREATE's fixed last
// step.
func TestMaterialEditorEditMultiFieldIncludingNaturalUnitThreadsCurrentUnit(t *testing.T) {
	catalog := domain.NewMaterialsCatalog()
	existing, err := domain.NewMaterial(catalog, "CONDUCTORES", "CABLE", "M", cableAttributeValues())
	if err != nil {
		t.Fatalf("NewMaterial(existing) error = %v, want nil", err)
	}
	existing.ID = 42

	getter := &fakeMaterialGetter{material: existing}
	updater := &fakeMaterialUpdater{}
	adapter := NewMaterialsWorkspaceAdapter(&fakeMaterialSearcher{results: []domain.Material{existing}}, getter, &fakeMaterialDescriber{}, &fakeMaterialCreator{}, updater)

	openDetailViaSearch(t, adapter, existing)
	response, err := adapter.Respond(context.Background(), InteractionInput{Kind: InputAction, ActionID: editActionID, Value: editActionID, Target: ActionTargetAgent})
	if err != nil {
		t.Fatalf("Respond(edit) error = %v, want nil", err)
	}

	response = answerMultiQuestion(t, adapter, response, []string{"color", editNaturalUnitFieldCode})
	response = answerQuestion(t, adapter, response, "BLANCO") // color

	unitRequest, ok := response.Pending.(QuestionRequest)
	if !ok {
		t.Fatalf("Pending = %T, want QuestionRequest (natural unit question)", response.Pending)
	}
	if !strings.Contains(strings.ToLower(unitRequest.Prompt), "unidad") {
		t.Fatalf("Prompt = %q, want the NaturalUnit question", unitRequest.Prompt)
	}

	response = answerQuestion(t, adapter, response, "M") // the catalog's only allowed unit

	if _, ok := response.Pending.(ConfirmationRequest); !ok {
		t.Fatalf("Pending = %T, want ConfirmationRequest", response.Pending)
	}

	response = answerConfirmation(t, adapter, response, "yes")

	if updater.callCount != 1 {
		t.Fatalf("Update call count = %d, want 1", updater.callCount)
	}
	if updater.gotMaterial.NaturalUnit != "M" {
		t.Fatalf("gotMaterial.NaturalUnit = %q, want %q", updater.gotMaterial.NaturalUnit, "M")
	}
	foundColor := false
	for _, value := range updater.gotMaterial.Attributes {
		if value.AttributeCode == "color" {
			foundColor = true
			if value.OptionCode != "BLANCO" {
				t.Fatalf("color attribute = %q, want %q", value.OptionCode, "BLANCO")
			}
		}
	}
	if !foundColor {
		t.Fatalf("gotMaterial.Attributes = %+v, want a color attribute", updater.gotMaterial.Attributes)
	}
	if response.Pending != nil {
		t.Fatalf("Pending = %#v, want nil after a successful edit", response.Pending)
	}
}

// TestMaterialEditorEditEscMidMultiFieldSequenceAbortsImmediately covers Esc
// (InputCancel) sent mid multi-field sequence, after the first selected
// field has already been answered but before the second: the whole edit
// must abort immediately, exactly like it does at any other step, and
// nothing must be persisted — proving respondToEditor's top-of-function
// InputCancel handling is unweakened by this restructuring.
func TestMaterialEditorEditEscMidMultiFieldSequenceAbortsImmediately(t *testing.T) {
	catalog := domain.NewMaterialsCatalog()
	existing, err := domain.NewMaterial(catalog, "CONDUCTORES", "CABLE", "M", cableAttributeValues())
	if err != nil {
		t.Fatalf("NewMaterial(existing) error = %v, want nil", err)
	}
	existing.ID = 42

	getter := &fakeMaterialGetter{material: existing}
	creator := &fakeMaterialCreator{}
	updater := &fakeMaterialUpdater{}
	adapter := NewMaterialsWorkspaceAdapter(&fakeMaterialSearcher{results: []domain.Material{existing}}, getter, &fakeMaterialDescriber{}, creator, updater)

	openDetailViaSearch(t, adapter, existing)
	response, err := adapter.Respond(context.Background(), InteractionInput{Kind: InputAction, ActionID: editActionID, Value: editActionID, Target: ActionTargetAgent})
	if err != nil {
		t.Fatalf("Respond(edit) error = %v, want nil", err)
	}

	response = answerMultiQuestion(t, adapter, response, []string{"color", "voltage"})
	response = answerQuestion(t, adapter, response, "BLANCO") // color answered

	if _, ok := response.Pending.(QuestionRequest); !ok {
		t.Fatalf("Pending = %T, want QuestionRequest (voltage question still pending)", response.Pending)
	}

	cancelled, err := adapter.Respond(context.Background(), InteractionInput{Kind: InputCancel})
	if err != nil {
		t.Fatalf("Respond(cancel) error = %v, want nil", err)
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
