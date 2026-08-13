package tui

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/GARFEX33/garfex-costos-unitarios/internal/domain"
)

func TestMaterialsWorkspaceAdapterGreeting(t *testing.T) {
	adapter := NewMaterialsWorkspaceAdapter(&fakeMaterialSearcher{}, &fakeMaterialGetter{}, &fakeMaterialDescriber{}, &fakeMaterialCreator{})
	greeting, ok := adapter.Greeting().(TextMessage)
	if !ok {
		t.Fatalf("Greeting() = %T, want TextMessage", adapter.Greeting())
	}
	if !containsAll(greeting.Text, "conectado", "catálogo real") {
		t.Fatalf("Greeting() text = %q, want it to mention the real catalog connection", greeting.Text)
	}
	for _, stale := range []string{"no está implementada", "todavía no"} {
		if strings.Contains(greeting.Text, stale) {
			t.Fatalf("Greeting() text = %q, must not claim search is unavailable anymore", greeting.Text)
		}
	}
}

// TestMaterialsWorkspaceAdapterRespondNonTextInputsNeverFabricateOrSearch
// covers InteractionInput kinds that are none of: a text search, a
// selection of a search-result option, or the "back" action from the
// detail view — those all fall through to the unchanged status/greeting
// fallback, and neither Search nor Get is called.
func TestMaterialsWorkspaceAdapterRespondNonTextInputsNeverFabricateOrSearch(t *testing.T) {
	fake := &fakeMaterialSearcher{}
	getter := &fakeMaterialGetter{}
	adapter := NewMaterialsWorkspaceAdapter(fake, getter, &fakeMaterialDescriber{}, &fakeMaterialCreator{})
	inputs := []InteractionInput{
		{Kind: InputSelection, Key: "some-other-key", Value: "some-option"},
		{Kind: InputAction, ActionID: "whatever"},
		{Kind: InputCancel},
	}
	for _, input := range inputs {
		response, err := adapter.Respond(context.Background(), input)
		if err != nil {
			t.Fatalf("Respond(%+v) error = %v, want nil", input, err)
		}
		if len(response.Messages) != 1 {
			t.Fatalf("Respond(%+v) messages = %v, want exactly one message", input, response.Messages)
		}
		if response.Pending != nil {
			t.Fatalf("Respond(%+v) pending = %#v, want nil (must never fabricate a pending question)", input, response.Pending)
		}
		message, ok := response.Messages[0].(TextMessage)
		if !ok {
			t.Fatalf("Respond(%+v) message = %T, want TextMessage (never QuestionRequest/ConfirmationRequest/ActionRequest/StructuredResult)", input, response.Messages[0])
		}
		if !containsAll(message.Text, "catálogo real") {
			t.Fatalf("Respond(%+v) text = %q, want the status message", input, message.Text)
		}
	}
	if fake.callCount != 0 {
		t.Fatalf("Search call count = %d, want 0 for non-matching inputs", fake.callCount)
	}
	if fake.gotCriteria.Text != "" || fake.gotCriteria.Limit != 0 {
		t.Fatalf("gotCriteria = %+v, want zero-value (Search never called)", fake.gotCriteria)
	}
	if getter.callCount != 0 {
		t.Fatalf("Get call count = %d, want 0 for non-matching inputs", getter.callCount)
	}
}

// TestMaterialsWorkspaceAdapterAcceptsMaterialSearcherAndGetter is a
// compile-time interface-satisfaction check: NewMaterialsWorkspaceAdapter
// now takes both a materialSearcher and a materialGetter, and a fake for
// each must be accepted structurally.
func TestMaterialsWorkspaceAdapterAcceptsMaterialSearcherAndGetter(t *testing.T) {
	var searcher materialSearcher = &fakeMaterialSearcher{}
	var getter materialGetter = &fakeMaterialGetter{}
	adapter := NewMaterialsWorkspaceAdapter(searcher, getter, &fakeMaterialDescriber{}, &fakeMaterialCreator{})
	if adapter == nil {
		t.Fatal("NewMaterialsWorkspaceAdapter() = nil")
	}
}

func TestMaterialsWorkspaceAdapterRespondSendsTextUnchanged(t *testing.T) {
	fake := &fakeMaterialSearcher{}
	adapter := NewMaterialsWorkspaceAdapter(fake, &fakeMaterialGetter{}, &fakeMaterialDescriber{}, &fakeMaterialCreator{})
	value := "  Cemento PORTLAND   Tipo I "
	if _, err := adapter.Respond(context.Background(), InteractionInput{Kind: InputText, Value: value}); err != nil {
		t.Fatalf("Respond() error = %v, want nil", err)
	}
	if fake.gotCriteria.Text != value {
		t.Fatalf("gotCriteria.Text = %q, want %q unchanged", fake.gotCriteria.Text, value)
	}
}

func TestMaterialsWorkspaceAdapterRespondSendsLimitEleven(t *testing.T) {
	fake := &fakeMaterialSearcher{}
	adapter := NewMaterialsWorkspaceAdapter(fake, &fakeMaterialGetter{}, &fakeMaterialDescriber{}, &fakeMaterialCreator{})
	if _, err := adapter.Respond(context.Background(), InteractionInput{Kind: InputText, Value: "cemento"}); err != nil {
		t.Fatalf("Respond() error = %v, want nil", err)
	}
	if fake.gotCriteria.Limit != 11 {
		t.Fatalf("gotCriteria.Limit = %d, want 11", fake.gotCriteria.Limit)
	}
}

// TestMaterialsWorkspaceAdapterRespondZeroResults is unchanged from before
// this PR: 0 results still produces a plain TextMessage, no Pending.
func TestMaterialsWorkspaceAdapterRespondZeroResults(t *testing.T) {
	fake := &fakeMaterialSearcher{results: nil}
	adapter := NewMaterialsWorkspaceAdapter(fake, &fakeMaterialGetter{}, &fakeMaterialDescriber{}, &fakeMaterialCreator{})
	response, err := adapter.Respond(context.Background(), InteractionInput{Kind: InputText, Value: "inexistente"})
	if err != nil {
		t.Fatalf("Respond() error = %v, want nil", err)
	}
	if len(response.Messages) != 1 {
		t.Fatalf("Messages = %v, want exactly one message", response.Messages)
	}
	if response.Pending != nil {
		t.Fatalf("Pending = %#v, want nil for zero results", response.Pending)
	}
	message, ok := response.Messages[0].(TextMessage)
	if !ok {
		t.Fatalf("Messages[0] = %T, want TextMessage", response.Messages[0])
	}
	if !strings.Contains(message.Text, "inexistente") {
		t.Fatalf("text = %q, want it to mention the searched text", message.Text)
	}
}

// TestMaterialsWorkspaceAdapterRespondErrorNeverLeaksRawError is unchanged
// from before this PR: a Search error still produces a plain ErrorMessage,
// no Pending.
func TestMaterialsWorkspaceAdapterRespondErrorNeverLeaksRawError(t *testing.T) {
	fake := &fakeMaterialSearcher{err: errors.New("connect to database: dial tcp refused")}
	adapter := NewMaterialsWorkspaceAdapter(fake, &fakeMaterialGetter{}, &fakeMaterialDescriber{}, &fakeMaterialCreator{})
	response, err := adapter.Respond(context.Background(), InteractionInput{Kind: InputText, Value: "cemento"})
	if err != nil {
		t.Fatalf("Respond() error = %v, want nil (failure must already be converted to an InteractionMessage)", err)
	}
	if len(response.Messages) != 1 {
		t.Fatalf("Messages = %v, want exactly one message", response.Messages)
	}
	if response.Pending != nil {
		t.Fatalf("Pending = %#v, want nil for a search error", response.Pending)
	}
	message, ok := response.Messages[0].(ErrorMessage)
	if !ok {
		t.Fatalf("Messages[0] = %T, want ErrorMessage", response.Messages[0])
	}
	if strings.Contains(message.Text, "connect to database") || strings.Contains(message.Text, "dial tcp") {
		t.Fatalf("text = %q, must not leak the raw error", message.Text)
	}
}

// TestMaterialsWorkspaceAdapterRespondOneResultReturnsSelectableQuestion
// replaces the PR6-era TestMaterialsWorkspaceAdapterRespondOneResultOmitsIdentityKey:
// a successful search with results is no longer a flat StructuredResult
// message — it's a selectable QuestionRequest carried on Pending, with no
// separate Messages entry (the QuestionRequest's own Prompt/Options already
// render the full "N found: [list]" experience). IdentityKey must still
// never be surfaced raw, this time via Option.Value's encoded form check
// plus a substring check on the Option.Label.
func TestMaterialsWorkspaceAdapterRespondOneResultReturnsSelectableQuestion(t *testing.T) {
	material := domain.Material{
		FamilyCode: "CEMENT", NaturalUnit: "kg", IdentityKey: "CEMENT|kg|SECRET-42",
		Attributes: []domain.MaterialAttributeValue{domain.OptionValue("color", "GRIS")},
	}
	fake := &fakeMaterialSearcher{results: []domain.Material{material}}
	adapter := NewMaterialsWorkspaceAdapter(fake, &fakeMaterialGetter{}, &fakeMaterialDescriber{}, &fakeMaterialCreator{})
	response, err := adapter.Respond(context.Background(), InteractionInput{Kind: InputText, Value: "cemento"})
	if err != nil {
		t.Fatalf("Respond() error = %v, want nil", err)
	}
	if len(response.Messages) != 0 {
		t.Fatalf("Messages = %v, want none (the Pending QuestionRequest carries the summary)", response.Messages)
	}
	request, ok := response.Pending.(QuestionRequest)
	if !ok {
		t.Fatalf("Pending = %T, want QuestionRequest", response.Pending)
	}
	if request.Key != searchResultsKey {
		t.Fatalf("Pending.Key = %q, want %q", request.Key, searchResultsKey)
	}
	if request.SelectionMode != SelectionSingle {
		t.Fatalf("Pending.SelectionMode = %q, want %q", request.SelectionMode, SelectionSingle)
	}
	if len(request.Options) != 1 {
		t.Fatalf("Pending.Options = %v, want exactly 1 option", request.Options)
	}
	option := request.Options[0]
	wantValue := "CEMENT|CEMENT|kg|SECRET-42"
	if option.Value != wantValue {
		t.Fatalf("Options[0].Value = %q, want %q", option.Value, wantValue)
	}
	if strings.Contains(option.Label, material.IdentityKey) {
		t.Fatalf("Options[0].Label = %q, must never contain IdentityKey %q", option.Label, material.IdentityKey)
	}
	if !strings.HasPrefix(option.Label, material.FamilyCode) {
		t.Fatalf("Options[0].Label = %q, want it to start with %q", option.Label, material.FamilyCode)
	}
}

// TestMaterialsWorkspaceAdapterRespondMultipleResultsPreserveOrder replaces
// the PR6-era test of the same name: order is now asserted on Pending's
// Options instead of a StructuredResult's Sections.
func TestMaterialsWorkspaceAdapterRespondMultipleResultsPreserveOrder(t *testing.T) {
	materials := []domain.Material{
		{FamilyCode: "CEMENT", NaturalUnit: "kg", IdentityKey: "CEMENT|kg|1"},
		{FamilyCode: "ARENA", NaturalUnit: "m3", IdentityKey: "ARENA|m3|2"},
		{FamilyCode: "GRAVA", NaturalUnit: "m3", IdentityKey: "GRAVA|m3|3"},
	}
	fake := &fakeMaterialSearcher{results: materials}
	adapter := NewMaterialsWorkspaceAdapter(fake, &fakeMaterialGetter{}, &fakeMaterialDescriber{}, &fakeMaterialCreator{})
	response, err := adapter.Respond(context.Background(), InteractionInput{Kind: InputText, Value: "x"})
	if err != nil {
		t.Fatalf("Respond() error = %v, want nil", err)
	}
	request, ok := response.Pending.(QuestionRequest)
	if !ok {
		t.Fatalf("Pending = %T, want QuestionRequest", response.Pending)
	}
	if len(request.Options) != len(materials) {
		t.Fatalf("Options = %d, want %d", len(request.Options), len(materials))
	}
	for i, material := range materials {
		if !strings.HasPrefix(request.Options[i].Label, material.FamilyCode) {
			t.Fatalf("Options[%d].Label = %q, want it to start with %q (order preserved)", i, request.Options[i].Label, material.FamilyCode)
		}
		wantValue := material.FamilyCode + "|" + material.IdentityKey
		if request.Options[i].Value != wantValue {
			t.Fatalf("Options[%d].Value = %q, want %q", i, request.Options[i].Value, wantValue)
		}
	}
}

// TestMaterialsWorkspaceAdapterRespondTenResultsNoMoreResultsHint replaces
// the PR6-era test of the same name: the "no more results" case is now
// asserted against Pending's Prompt instead of a rendered StructuredResult.
func TestMaterialsWorkspaceAdapterRespondTenResultsNoMoreResultsHint(t *testing.T) {
	materials := make([]domain.Material, 10)
	for i := range materials {
		materials[i] = domain.Material{FamilyCode: fmt.Sprintf("FAM%d", i), NaturalUnit: "u", IdentityKey: fmt.Sprintf("FAM%d|u|%d", i, i)}
	}
	fake := &fakeMaterialSearcher{results: materials}
	adapter := NewMaterialsWorkspaceAdapter(fake, &fakeMaterialGetter{}, &fakeMaterialDescriber{}, &fakeMaterialCreator{})
	response, err := adapter.Respond(context.Background(), InteractionInput{Kind: InputText, Value: "x"})
	if err != nil {
		t.Fatalf("Respond() error = %v, want nil", err)
	}
	if len(response.Messages) != 0 {
		t.Fatalf("Messages = %v, want none for exactly 10 results", response.Messages)
	}
	request, ok := response.Pending.(QuestionRequest)
	if !ok {
		t.Fatalf("Pending = %T, want QuestionRequest", response.Pending)
	}
	if len(request.Options) != 10 {
		t.Fatalf("Options = %d, want 10", len(request.Options))
	}
	if strings.Contains(request.Prompt, "más") {
		t.Fatalf("Prompt = %q, must not hint at more results for exactly 10", request.Prompt)
	}
}

// TestMaterialsWorkspaceAdapterRespondElevenResultsShowsTenAndMoreHint
// replaces the PR6-era test of the same name: the N+1 "more results" hint
// is now part of Pending's Prompt instead of a StructuredResult.Subtitle.
func TestMaterialsWorkspaceAdapterRespondElevenResultsShowsTenAndMoreHint(t *testing.T) {
	materials := make([]domain.Material, 11)
	for i := range materials {
		materials[i] = domain.Material{FamilyCode: fmt.Sprintf("FAM%d", i), NaturalUnit: "u", IdentityKey: fmt.Sprintf("FAM%d|u|%d", i, i)}
	}
	fake := &fakeMaterialSearcher{results: materials}
	adapter := NewMaterialsWorkspaceAdapter(fake, &fakeMaterialGetter{}, &fakeMaterialDescriber{}, &fakeMaterialCreator{})
	response, err := adapter.Respond(context.Background(), InteractionInput{Kind: InputText, Value: "x"})
	if err != nil {
		t.Fatalf("Respond() error = %v, want nil", err)
	}
	request, ok := response.Pending.(QuestionRequest)
	if !ok {
		t.Fatalf("Pending = %T, want QuestionRequest", response.Pending)
	}
	if len(request.Options) != 10 {
		t.Fatalf("Options = %d, want exactly 10 (N+1 signal trimmed)", len(request.Options))
	}
	if !strings.Contains(request.Prompt, "más") {
		t.Fatalf("Prompt = %q, want a hint that more results exist", request.Prompt)
	}
}

// TestMaterialsWorkspaceAdapterRespondSelectResultOpensDetail covers §6:
// selecting a search result (InputSelection, Key: searchResultsKey) calls
// materialsGetter.Get with the decoded familyCode/identityKey and returns a
// StructuredResult detail plus a Pending ActionRequest offering "volver".
func TestMaterialsWorkspaceAdapterRespondSelectResultOpensDetail(t *testing.T) {
	material := domain.Material{
		FamilyCode: "CEMENT", NaturalUnit: "kg", IdentityKey: "CEMENT|kg|SECRET-42",
		Attributes: []domain.MaterialAttributeValue{domain.OptionValue("color", "GRIS")},
	}
	getter := &fakeMaterialGetter{material: material}
	adapter := NewMaterialsWorkspaceAdapter(&fakeMaterialSearcher{}, getter, &fakeMaterialDescriber{}, &fakeMaterialCreator{})

	response, err := adapter.Respond(context.Background(), InteractionInput{
		Kind: InputSelection, Key: searchResultsKey, Value: "CEMENT|CEMENT|kg|SECRET-42",
	})
	if err != nil {
		t.Fatalf("Respond() error = %v, want nil", err)
	}
	if getter.gotFamilyCode != "CEMENT" || getter.gotIdentityKey != "CEMENT|kg|SECRET-42" {
		t.Fatalf("Get called with (%q, %q), want (%q, %q)", getter.gotFamilyCode, getter.gotIdentityKey, "CEMENT", "CEMENT|kg|SECRET-42")
	}
	if len(response.Messages) != 1 {
		t.Fatalf("Messages = %v, want exactly one message", response.Messages)
	}
	result, ok := response.Messages[0].(StructuredResult)
	if !ok {
		t.Fatalf("Messages[0] = %T, want StructuredResult", response.Messages[0])
	}
	if !strings.HasPrefix(result.Title, material.FamilyCode) {
		t.Fatalf("Title = %q, want it to start with %q", result.Title, material.FamilyCode)
	}
	if strings.Contains(result.Title, material.IdentityKey) {
		t.Fatalf("Title = %q, must never contain IdentityKey %q", result.Title, material.IdentityKey)
	}
	for _, field := range result.Fields {
		if strings.Contains(field.Value, material.IdentityKey) {
			t.Fatalf("field %+v leaks IdentityKey", field)
		}
	}
	action, ok := response.Pending.(ActionRequest)
	if !ok {
		t.Fatalf("Pending = %T, want ActionRequest", response.Pending)
	}
	if len(action.Actions) != 1 || action.Actions[0].ID != backActionID {
		t.Fatalf("Actions = %+v, want exactly one action with ID %q", action.Actions, backActionID)
	}
}

// TestMaterialsWorkspaceAdapterRespondGetErrorNeverLeaksRawError covers the
// Get failure path of §6, mirroring TestMaterialsWorkspaceAdapterRespondErrorNeverLeaksRawError's
// discipline for the Search failure path.
func TestMaterialsWorkspaceAdapterRespondGetErrorNeverLeaksRawError(t *testing.T) {
	getter := &fakeMaterialGetter{err: errors.New("connect to database: dial tcp refused")}
	adapter := NewMaterialsWorkspaceAdapter(&fakeMaterialSearcher{}, getter, &fakeMaterialDescriber{}, &fakeMaterialCreator{})

	response, err := adapter.Respond(context.Background(), InteractionInput{
		Kind: InputSelection, Key: searchResultsKey, Value: "CEMENT|CEMENT|kg|1",
	})
	if err != nil {
		t.Fatalf("Respond() error = %v, want nil (failure must already be converted to an InteractionMessage)", err)
	}
	if len(response.Messages) != 1 {
		t.Fatalf("Messages = %v, want exactly one message", response.Messages)
	}
	if response.Pending != nil {
		t.Fatalf("Pending = %#v, want nil when Get fails", response.Pending)
	}
	message, ok := response.Messages[0].(ErrorMessage)
	if !ok {
		t.Fatalf("Messages[0] = %T, want ErrorMessage", response.Messages[0])
	}
	if strings.Contains(message.Text, "connect to database") || strings.Contains(message.Text, "dial tcp") {
		t.Fatalf("text = %q, must not leak the raw error", message.Text)
	}
}

// TestMaterialsWorkspaceAdapterRespondBackReRunsSameSearch covers §7:
// selecting "volver" from the detail view re-runs the exact same search
// (same Text sent to the fake searcher as the original query) and
// reproduces the identical QuestionRequest/Options.
func TestMaterialsWorkspaceAdapterRespondBackReRunsSameSearch(t *testing.T) {
	materials := []domain.Material{
		{FamilyCode: "CEMENT", NaturalUnit: "kg", IdentityKey: "CEMENT|kg|1"},
		{FamilyCode: "ARENA", NaturalUnit: "m3", IdentityKey: "ARENA|m3|2"},
	}
	fake := &fakeMaterialSearcher{results: materials}
	adapter := NewMaterialsWorkspaceAdapter(fake, &fakeMaterialGetter{material: materials[0]}, &fakeMaterialDescriber{}, &fakeMaterialCreator{})

	original, err := adapter.Respond(context.Background(), InteractionInput{Kind: InputText, Value: "cemento arena"})
	if err != nil {
		t.Fatalf("Respond(search) error = %v, want nil", err)
	}
	originalRequest, ok := original.Pending.(QuestionRequest)
	if !ok {
		t.Fatalf("Pending = %T, want QuestionRequest", original.Pending)
	}

	if _, err := adapter.Respond(context.Background(), InteractionInput{
		Kind: InputSelection, Key: searchResultsKey, Value: originalRequest.Options[0].Value,
	}); err != nil {
		t.Fatalf("Respond(select) error = %v, want nil", err)
	}

	back, err := adapter.Respond(context.Background(), InteractionInput{Kind: InputAction, Key: materialsDetailActionsKey, ActionID: backActionID, Value: backActionID})
	if err != nil {
		t.Fatalf("Respond(back) error = %v, want nil", err)
	}
	if fake.callCount != 2 {
		t.Fatalf("Search call count = %d, want 2 (original + back)", fake.callCount)
	}
	if fake.gotCriteria.Text != "cemento arena" {
		t.Fatalf("second Search criteria.Text = %q, want %q (same as original)", fake.gotCriteria.Text, "cemento arena")
	}
	backRequest, ok := back.Pending.(QuestionRequest)
	if !ok {
		t.Fatalf("Pending = %T, want QuestionRequest", back.Pending)
	}
	if len(backRequest.Options) != len(originalRequest.Options) {
		t.Fatalf("Options = %d, want %d (identical to original search)", len(backRequest.Options), len(originalRequest.Options))
	}
	for i := range originalRequest.Options {
		got, want := backRequest.Options[i], originalRequest.Options[i]
		if got.ID != want.ID || got.Label != want.Label || got.Value != want.Value {
			t.Fatalf("Options[%d] = %+v, want identical to original %+v", i, got, want)
		}
	}
}

// TestMaterialsWorkspaceAdapterRespondDetailOmitsNotApplicableAttributes
// replaces the PR6-era TestMaterialsWorkspaceAdapterRespondOmitsNotApplicableAttributes:
// NOT_APPLICABLE-attribute omission is now asserted on the detail view's
// StructuredResult.Fields (its own distinct response step) instead of a
// search result's Section.Fields.
func TestMaterialsWorkspaceAdapterRespondDetailOmitsNotApplicableAttributes(t *testing.T) {
	material := domain.Material{
		FamilyCode: "CONDUCTORES", NaturalUnit: "m", IdentityKey: "CONDUCTORES|m|1",
		Attributes: []domain.MaterialAttributeValue{
			domain.OptionValue("insulation", "DESNUDO"),
			{AttributeCode: "color", Type: domain.ValueTypeControlledOption, Text: "NOT_APPLICABLE"},
		},
	}
	getter := &fakeMaterialGetter{material: material}
	adapter := NewMaterialsWorkspaceAdapter(&fakeMaterialSearcher{}, getter, &fakeMaterialDescriber{}, &fakeMaterialCreator{})

	response, err := adapter.Respond(context.Background(), InteractionInput{
		Kind: InputSelection, Key: searchResultsKey, Value: "CONDUCTORES|CONDUCTORES|m|1",
	})
	if err != nil {
		t.Fatalf("Respond() error = %v, want nil", err)
	}
	result, ok := response.Messages[0].(StructuredResult)
	if !ok {
		t.Fatalf("Messages[0] = %T, want StructuredResult", response.Messages[0])
	}
	for _, field := range result.Fields {
		if field.Label == "color" {
			t.Fatalf("Fields = %+v, want NOT_APPLICABLE attribute %q omitted", result.Fields, "color")
		}
	}
}

func containsAll(value string, substrings ...string) bool {
	for _, substring := range substrings {
		if !strings.Contains(value, substring) {
			return false
		}
	}
	return true
}

type fakeMaterialSearcher struct {
	gotCriteria domain.SearchCriteria
	callCount   int
	results     []domain.Material
	err         error
}

func (f *fakeMaterialSearcher) Search(_ context.Context, criteria domain.SearchCriteria) ([]domain.Material, error) {
	f.gotCriteria = criteria
	f.callCount++
	return f.results, f.err
}

type fakeMaterialDescriber struct{ text string }

func (f *fakeMaterialDescriber) Describe(domain.Material) string { return f.text }

// TestMaterialsWorkspaceAdapterRespondDetailUsesDescriberForTitle proves the
// adapter composes its detail title as FamilyCode + " — " +
// describer.Describe(...) instead of any locally-built headline — the
// canonical presentation is entirely delegated to materialDescriber.
func TestMaterialsWorkspaceAdapterRespondDetailUsesDescriberForTitle(t *testing.T) {
	material := domain.Material{
		FamilyCode: "CONDUCTORES", ProductTypeCode: "CABLE", NaturalUnit: "M", IdentityKey: "CONDUCTORES|CABLE|1",
		Attributes: []domain.MaterialAttributeValue{domain.OptionValue("insulation", "THHN")},
	}
	getter := &fakeMaterialGetter{material: material}
	adapter := NewMaterialsWorkspaceAdapter(&fakeMaterialSearcher{}, getter, &fakeMaterialDescriber{text: "Cable THHN 12 AWG BLANCO"}, &fakeMaterialCreator{})

	response, err := adapter.Respond(context.Background(), InteractionInput{
		Kind: InputSelection, Key: searchResultsKey, Value: "CONDUCTORES|CONDUCTORES|CABLE|1",
	})
	if err != nil {
		t.Fatalf("Respond() error = %v, want nil", err)
	}
	result, ok := response.Messages[0].(StructuredResult)
	if !ok {
		t.Fatalf("Messages[0] = %T, want StructuredResult", response.Messages[0])
	}
	want := "CONDUCTORES — Cable THHN 12 AWG BLANCO"
	if result.Title != want {
		t.Fatalf("Title = %q, want %q", result.Title, want)
	}
}
