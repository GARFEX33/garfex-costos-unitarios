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

// TestMaterialEditorFullHappyPathCreatesCableMaterial covers the full
// CONDUCTORES/CABLE create flow end to end: trigger, family, productType,
// every attribute (in catalog order), unit, and a successful Create with
// the exact expected Material.
func TestMaterialEditorFullHappyPathCreatesCableMaterial(t *testing.T) {
	creator := &fakeMaterialCreator{}
	adapter := NewMaterialsWorkspaceAdapter(&fakeMaterialSearcher{}, &fakeMaterialGetter{}, &fakeMaterialDescriber{}, creator)

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
	adapter := NewMaterialsWorkspaceAdapter(&fakeMaterialSearcher{}, &fakeMaterialGetter{}, &fakeMaterialDescriber{}, &fakeMaterialCreator{})

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
	adapter := NewMaterialsWorkspaceAdapter(&fakeMaterialSearcher{}, &fakeMaterialGetter{}, &fakeMaterialDescriber{}, &fakeMaterialCreator{})

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
	adapter := NewMaterialsWorkspaceAdapter(&fakeMaterialSearcher{}, getter, &fakeMaterialDescriber{}, creator)

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
	adapter := NewMaterialsWorkspaceAdapter(fake, &fakeMaterialGetter{}, &fakeMaterialDescriber{}, &fakeMaterialCreator{})

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
	adapter := NewMaterialsWorkspaceAdapter(fake, &fakeMaterialGetter{}, &fakeMaterialDescriber{}, &fakeMaterialCreator{})
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
