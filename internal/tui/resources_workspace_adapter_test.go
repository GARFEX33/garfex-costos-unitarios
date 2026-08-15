package tui

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/GARFEX33/garfex-costos-unitarios/internal/domain"
)

// fakeResourceSearcher is the fake resourceSearcher used by the workspace
// dispatch tests in this file — resource_editor_test.go's newTestAdapter
// leaves this nil since the editor state machine tests never search.
type fakeResourceSearcher struct {
	gotCriteria domain.SearchCriteria
	callCount   int
	results     []domain.Resource
	err         error
}

func (f *fakeResourceSearcher) Search(_ context.Context, criteria domain.SearchCriteria) ([]domain.Resource, error) {
	f.gotCriteria = criteria
	f.callCount++
	return f.results, f.err
}

// fakeResourceDeleter is the fake resourceDeleter used by the delete-flow
// tests in this file.
type fakeResourceDeleter struct {
	callCount int
	gotID     int64
	err       error
}

func (f *fakeResourceDeleter) Delete(_ context.Context, id int64) error {
	f.callCount++
	f.gotID = id
	return f.err
}

// newDispatchAdapter builds a ResourcesWorkspaceAdapter against the real
// production catalog (domain.SeedResourceCatalog()) for the workspace
// dispatch tests below — mirrors resource_editor_test.go's newTestAdapter
// but wires every dependency (search/delete included), since Respond's full
// dispatch (unlike the editor-only tests) exercises all of them.
func newDispatchAdapter(searcher resourceSearcher, getter resourceGetter, describer resourceDescriber, creator resourceCreator, updater resourceUpdater, deleter resourceDeleter, classFilter string) *ResourcesWorkspaceAdapter {
	return NewResourcesWorkspaceAdapter(searcher, getter, describer, creator, updater, deleter, domain.SeedResourceCatalog(), classFilter)
}

func TestResourcesWorkspaceAdapterGreeting(t *testing.T) {
	adapter := newDispatchAdapter(&fakeResourceSearcher{}, &fakeResourceGetter{}, &fakeResourceDescriber{}, &fakeResourceCreator{}, &fakeResourceUpdater{}, &fakeResourceDeleter{}, "")
	greeting, ok := adapter.Greeting().(TextMessage)
	if !ok {
		t.Fatalf("Greeting() = %T, want TextMessage", adapter.Greeting())
	}
	if !containsAll(greeting.Text, "conectado", "catálogo real") {
		t.Fatalf("Greeting() text = %q, want it to mention the real catalog connection", greeting.Text)
	}
}

// TestResourcesWorkspaceAdapterRespondNonTextInputsNeverFabricateOrSearch
// covers InteractionInput kinds that are none of: a text search, a
// selection of a search-result option, or the "back" action from the detail
// view — those all fall through to the unchanged status/greeting fallback,
// and neither Search nor Get is called.
func TestResourcesWorkspaceAdapterRespondNonTextInputsNeverFabricateOrSearch(t *testing.T) {
	fake := &fakeResourceSearcher{}
	getter := &fakeResourceGetter{}
	adapter := newDispatchAdapter(fake, getter, &fakeResourceDescriber{}, &fakeResourceCreator{}, &fakeResourceUpdater{}, &fakeResourceDeleter{}, "")
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

func TestResourcesWorkspaceAdapterRespondSendsTextUnchanged(t *testing.T) {
	fake := &fakeResourceSearcher{}
	adapter := newDispatchAdapter(fake, &fakeResourceGetter{}, &fakeResourceDescriber{}, &fakeResourceCreator{}, &fakeResourceUpdater{}, &fakeResourceDeleter{}, "")
	value := "  Cemento PORTLAND   Tipo I "
	if _, err := adapter.Respond(context.Background(), InteractionInput{Kind: InputText, Value: value}); err != nil {
		t.Fatalf("Respond() error = %v, want nil", err)
	}
	if fake.gotCriteria.Text != value {
		t.Fatalf("gotCriteria.Text = %q, want %q unchanged", fake.gotCriteria.Text, value)
	}
}

func TestResourcesWorkspaceAdapterRespondSendsLimitEleven(t *testing.T) {
	fake := &fakeResourceSearcher{}
	adapter := newDispatchAdapter(fake, &fakeResourceGetter{}, &fakeResourceDescriber{}, &fakeResourceCreator{}, &fakeResourceUpdater{}, &fakeResourceDeleter{}, "")
	if _, err := adapter.Respond(context.Background(), InteractionInput{Kind: InputText, Value: "cemento"}); err != nil {
		t.Fatalf("Respond() error = %v, want nil", err)
	}
	if fake.gotCriteria.Limit != 11 {
		t.Fatalf("gotCriteria.Limit = %d, want 11", fake.gotCriteria.Limit)
	}
}

// TestResourcesWorkspaceAdapterSearchResponsePassesClassFilterIntoCriteria
// is the D4 proof this PR's task explicitly calls for: the SAME adapter
// TYPE, reused per workspace slot, must pass a.classFilter straight into
// SearchCriteria.ClassCode — empty (unfiltered) for the "/recursos"
// workspace, non-empty (scoped) for a class-specific workspace like
// "/materiales".
func TestResourcesWorkspaceAdapterSearchResponsePassesClassFilterIntoCriteria(t *testing.T) {
	for _, tt := range []struct {
		name        string
		classFilter string
	}{
		{name: "unfiltered /recursos workspace", classFilter: ""},
		{name: "class-scoped /materiales workspace", classFilter: "MATERIAL"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			fake := &fakeResourceSearcher{}
			adapter := newDispatchAdapter(fake, &fakeResourceGetter{}, &fakeResourceDescriber{}, &fakeResourceCreator{}, &fakeResourceUpdater{}, &fakeResourceDeleter{}, tt.classFilter)
			if _, err := adapter.Respond(context.Background(), InteractionInput{Kind: InputText, Value: "cable"}); err != nil {
				t.Fatalf("Respond() error = %v, want nil", err)
			}
			if fake.gotCriteria.ClassCode != tt.classFilter {
				t.Fatalf("gotCriteria.ClassCode = %q, want %q", fake.gotCriteria.ClassCode, tt.classFilter)
			}
		})
	}
}

// TestResourcesWorkspaceAdapterRespondZeroResults is unchanged from the
// retired MaterialsWorkspaceAdapter suite: 0 results still produces a plain
// TextMessage, no Pending.
func TestResourcesWorkspaceAdapterRespondZeroResults(t *testing.T) {
	fake := &fakeResourceSearcher{results: nil}
	adapter := newDispatchAdapter(fake, &fakeResourceGetter{}, &fakeResourceDescriber{}, &fakeResourceCreator{}, &fakeResourceUpdater{}, &fakeResourceDeleter{}, "")
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

// TestResourcesWorkspaceAdapterRespondErrorNeverLeaksRawError is unchanged:
// a Search error still produces a plain ErrorMessage, no Pending.
func TestResourcesWorkspaceAdapterRespondErrorNeverLeaksRawError(t *testing.T) {
	fake := &fakeResourceSearcher{err: errors.New("connect to database: dial tcp refused")}
	adapter := newDispatchAdapter(fake, &fakeResourceGetter{}, &fakeResourceDescriber{}, &fakeResourceCreator{}, &fakeResourceUpdater{}, &fakeResourceDeleter{}, "")
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

// TestResourcesWorkspaceAdapterRespondOneResultReturnsSelectableQuestion
// covers the same shape as the retired suite: a successful search with
// results is a selectable QuestionRequest carried on Pending. The
// Option.Value encoding is now ClassCode + "|" + IdentityKey (design R1:
// Get's first argument is a class code, never a family code) instead of the
// old FamilyCode + "|" + IdentityKey.
func TestResourcesWorkspaceAdapterRespondOneResultReturnsSelectableQuestion(t *testing.T) {
	resource := domain.Resource{
		ClassCode: "MATERIAL", FamilyCode: "CEMENT", NaturalUnit: "kg", IdentityKey: "CEMENT|kg|SECRET-42",
		Attributes: []domain.ResourceAttributeValue{domain.OptionValue("color", "GRIS")},
	}
	fake := &fakeResourceSearcher{results: []domain.Resource{resource}}
	adapter := newDispatchAdapter(fake, &fakeResourceGetter{}, &fakeResourceDescriber{}, &fakeResourceCreator{}, &fakeResourceUpdater{}, &fakeResourceDeleter{}, "")
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
	wantValue := "MATERIAL|CEMENT|kg|SECRET-42"
	if option.Value != wantValue {
		t.Fatalf("Options[0].Value = %q, want %q", option.Value, wantValue)
	}
	if strings.Contains(option.Label, resource.IdentityKey) {
		t.Fatalf("Options[0].Label = %q, must never contain IdentityKey %q", option.Label, resource.IdentityKey)
	}
	if !strings.HasPrefix(option.Label, resource.FamilyCode) {
		t.Fatalf("Options[0].Label = %q, want it to start with %q", option.Label, resource.FamilyCode)
	}
}

// TestResourcesWorkspaceAdapterRespondMultipleResultsPreserveOrder is
// unchanged: order is asserted on Pending's Options.
func TestResourcesWorkspaceAdapterRespondMultipleResultsPreserveOrder(t *testing.T) {
	resources := []domain.Resource{
		{ClassCode: "MATERIAL", FamilyCode: "CEMENT", NaturalUnit: "kg", IdentityKey: "CEMENT|kg|1"},
		{ClassCode: "MATERIAL", FamilyCode: "ARENA", NaturalUnit: "m3", IdentityKey: "ARENA|m3|2"},
		{ClassCode: "MATERIAL", FamilyCode: "GRAVA", NaturalUnit: "m3", IdentityKey: "GRAVA|m3|3"},
	}
	fake := &fakeResourceSearcher{results: resources}
	adapter := newDispatchAdapter(fake, &fakeResourceGetter{}, &fakeResourceDescriber{}, &fakeResourceCreator{}, &fakeResourceUpdater{}, &fakeResourceDeleter{}, "")
	response, err := adapter.Respond(context.Background(), InteractionInput{Kind: InputText, Value: "x"})
	if err != nil {
		t.Fatalf("Respond() error = %v, want nil", err)
	}
	request, ok := response.Pending.(QuestionRequest)
	if !ok {
		t.Fatalf("Pending = %T, want QuestionRequest", response.Pending)
	}
	if len(request.Options) != len(resources) {
		t.Fatalf("Options = %d, want %d", len(request.Options), len(resources))
	}
	for i, resource := range resources {
		if !strings.HasPrefix(request.Options[i].Label, resource.FamilyCode) {
			t.Fatalf("Options[%d].Label = %q, want it to start with %q (order preserved)", i, request.Options[i].Label, resource.FamilyCode)
		}
		wantValue := resource.ClassCode + "|" + resource.IdentityKey
		if request.Options[i].Value != wantValue {
			t.Fatalf("Options[%d].Value = %q, want %q", i, request.Options[i].Value, wantValue)
		}
	}
}

// TestResourcesWorkspaceAdapterRespondTenResultsNoMoreResultsHint is
// unchanged: exactly 10 results shows no "more results" hint.
func TestResourcesWorkspaceAdapterRespondTenResultsNoMoreResultsHint(t *testing.T) {
	resources := make([]domain.Resource, 10)
	for i := range resources {
		resources[i] = domain.Resource{ClassCode: "MATERIAL", FamilyCode: fmt.Sprintf("FAM%d", i), NaturalUnit: "u", IdentityKey: fmt.Sprintf("FAM%d|u|%d", i, i)}
	}
	fake := &fakeResourceSearcher{results: resources}
	adapter := newDispatchAdapter(fake, &fakeResourceGetter{}, &fakeResourceDescriber{}, &fakeResourceCreator{}, &fakeResourceUpdater{}, &fakeResourceDeleter{}, "")
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

// TestResourcesWorkspaceAdapterRespondElevenResultsShowsTenAndMoreHint is
// unchanged: the N+1 "more results" hint is part of Pending's Prompt.
func TestResourcesWorkspaceAdapterRespondElevenResultsShowsTenAndMoreHint(t *testing.T) {
	resources := make([]domain.Resource, 11)
	for i := range resources {
		resources[i] = domain.Resource{ClassCode: "MATERIAL", FamilyCode: fmt.Sprintf("FAM%d", i), NaturalUnit: "u", IdentityKey: fmt.Sprintf("FAM%d|u|%d", i, i)}
	}
	fake := &fakeResourceSearcher{results: resources}
	adapter := newDispatchAdapter(fake, &fakeResourceGetter{}, &fakeResourceDescriber{}, &fakeResourceCreator{}, &fakeResourceUpdater{}, &fakeResourceDeleter{}, "")
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

// TestResourcesWorkspaceAdapterRespondSelectResultOpensDetail covers
// selecting a search result (InputSelection, Key: searchResultsKey): it
// calls resourcesGetter.Get with the decoded classCode/identityKey (design
// R1 — never familyCode) and returns a StructuredResult detail plus a
// Pending ActionRequest offering Editar/Duplicar/Eliminar/volver.
func TestResourcesWorkspaceAdapterRespondSelectResultOpensDetail(t *testing.T) {
	resource := domain.Resource{
		ClassCode: "MATERIAL", FamilyCode: "CEMENT", NaturalUnit: "kg", IdentityKey: "CEMENT|kg|SECRET-42",
		Attributes: []domain.ResourceAttributeValue{domain.OptionValue("color", "GRIS")},
	}
	getter := &fakeResourceGetter{resource: resource}
	adapter := newDispatchAdapter(&fakeResourceSearcher{}, getter, &fakeResourceDescriber{}, &fakeResourceCreator{}, &fakeResourceUpdater{}, &fakeResourceDeleter{}, "")

	response, err := adapter.Respond(context.Background(), InteractionInput{
		Kind: InputSelection, Key: searchResultsKey, Value: "MATERIAL|CEMENT|kg|SECRET-42",
	})
	if err != nil {
		t.Fatalf("Respond() error = %v, want nil", err)
	}
	if getter.gotClassCode != "MATERIAL" || getter.gotIdentityKey != "CEMENT|kg|SECRET-42" {
		t.Fatalf("Get called with (%q, %q), want (%q, %q)", getter.gotClassCode, getter.gotIdentityKey, "MATERIAL", "CEMENT|kg|SECRET-42")
	}
	if len(response.Messages) != 1 {
		t.Fatalf("Messages = %v, want exactly one message", response.Messages)
	}
	result, ok := response.Messages[0].(StructuredResult)
	if !ok {
		t.Fatalf("Messages[0] = %T, want StructuredResult", response.Messages[0])
	}
	if !strings.HasPrefix(result.Title, resource.FamilyCode) {
		t.Fatalf("Title = %q, want it to start with %q", result.Title, resource.FamilyCode)
	}
	if strings.Contains(result.Title, resource.IdentityKey) {
		t.Fatalf("Title = %q, must never contain IdentityKey %q", result.Title, resource.IdentityKey)
	}
	for _, field := range result.Fields {
		if strings.Contains(field.Value, resource.IdentityKey) {
			t.Fatalf("field %+v leaks IdentityKey", field)
		}
	}
	action, ok := response.Pending.(ActionRequest)
	if !ok {
		t.Fatalf("Pending = %T, want ActionRequest", response.Pending)
	}
	if len(action.Actions) != 4 || action.Actions[0].ID != editActionID || action.Actions[1].ID != duplicateActionID || action.Actions[2].ID != deleteActionID || action.Actions[3].ID != backActionID {
		t.Fatalf("Actions = %+v, want exactly [%q, %q, %q, %q] in that order", action.Actions, editActionID, duplicateActionID, deleteActionID, backActionID)
	}
}

// TestResourcesWorkspaceAdapterRespondGetErrorNeverLeaksRawError mirrors the
// Search failure path's discipline for the Get failure path.
func TestResourcesWorkspaceAdapterRespondGetErrorNeverLeaksRawError(t *testing.T) {
	getter := &fakeResourceGetter{err: errors.New("connect to database: dial tcp refused")}
	adapter := newDispatchAdapter(&fakeResourceSearcher{}, getter, &fakeResourceDescriber{}, &fakeResourceCreator{}, &fakeResourceUpdater{}, &fakeResourceDeleter{}, "")

	response, err := adapter.Respond(context.Background(), InteractionInput{
		Kind: InputSelection, Key: searchResultsKey, Value: "MATERIAL|CEMENT|kg|1",
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

// TestResourcesWorkspaceAdapterRespondBackReRunsSameSearch covers "volver":
// it re-runs the exact same search (same Text) and reproduces the identical
// QuestionRequest/Options.
func TestResourcesWorkspaceAdapterRespondBackReRunsSameSearch(t *testing.T) {
	resources := []domain.Resource{
		{ClassCode: "MATERIAL", FamilyCode: "CEMENT", NaturalUnit: "kg", IdentityKey: "CEMENT|kg|1"},
		{ClassCode: "MATERIAL", FamilyCode: "ARENA", NaturalUnit: "m3", IdentityKey: "ARENA|m3|2"},
	}
	fake := &fakeResourceSearcher{results: resources}
	adapter := newDispatchAdapter(fake, &fakeResourceGetter{resource: resources[0]}, &fakeResourceDescriber{}, &fakeResourceCreator{}, &fakeResourceUpdater{}, &fakeResourceDeleter{}, "")

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

	back, err := adapter.Respond(context.Background(), InteractionInput{Kind: InputAction, Key: resourcesDetailActionsKey, ActionID: backActionID, Value: backActionID})
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

// TestResourcesWorkspaceAdapterRespondDetailOmitsNotApplicableAttributes
// covers NOT_APPLICABLE-attribute omission on the detail view's
// StructuredResult.Fields.
func TestResourcesWorkspaceAdapterRespondDetailOmitsNotApplicableAttributes(t *testing.T) {
	resource := domain.Resource{
		ClassCode: "MATERIAL", FamilyCode: "CONDUCTORES", NaturalUnit: "m", IdentityKey: "CONDUCTORES|m|1",
		Attributes: []domain.ResourceAttributeValue{
			domain.OptionValue("insulation", "DESNUDO"),
			{AttributeCode: "color", Type: domain.ValueTypeControlledOption, Text: "NOT_APPLICABLE"},
		},
	}
	getter := &fakeResourceGetter{resource: resource}
	adapter := newDispatchAdapter(&fakeResourceSearcher{}, getter, &fakeResourceDescriber{}, &fakeResourceCreator{}, &fakeResourceUpdater{}, &fakeResourceDeleter{}, "")

	response, err := adapter.Respond(context.Background(), InteractionInput{
		Kind: InputSelection, Key: searchResultsKey, Value: "MATERIAL|CONDUCTORES|m|1",
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

// TestResourcesWorkspaceAdapterRespondDetailUsesDescriberForTitle proves the
// adapter composes its detail title as FamilyCode + " — " +
// describer.Describe(...) instead of any locally-built headline.
func TestResourcesWorkspaceAdapterRespondDetailUsesDescriberForTitle(t *testing.T) {
	resource := domain.Resource{
		ClassCode: "MATERIAL", FamilyCode: "CONDUCTORES", TypeCode: "CABLE", NaturalUnit: "M", IdentityKey: "CONDUCTORES|CABLE|1",
		Attributes: []domain.ResourceAttributeValue{domain.OptionValue("insulation", "THHN")},
	}
	getter := &fakeResourceGetter{resource: resource}
	adapter := newDispatchAdapter(&fakeResourceSearcher{}, getter, &fakeResourceDescriber{text: "Cable THHN 12 AWG BLANCO"}, &fakeResourceCreator{}, &fakeResourceUpdater{}, &fakeResourceDeleter{}, "")

	response, err := adapter.Respond(context.Background(), InteractionInput{
		Kind: InputSelection, Key: searchResultsKey, Value: "MATERIAL|CONDUCTORES|CABLE|1",
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

// openDeleteConfirmationDetail is a small test helper: it opens a detail
// view for resource (via the fakeResourceGetter) and returns the adapter
// plus the fakeResourceDeleter it was wired with, ready for the "Eliminar"
// action.
func openDeleteConfirmationDetail(t *testing.T, resource domain.Resource) (*ResourcesWorkspaceAdapter, *fakeResourceDeleter) {
	t.Helper()
	getter := &fakeResourceGetter{resource: resource}
	deleter := &fakeResourceDeleter{}
	adapter := newDispatchAdapter(&fakeResourceSearcher{}, getter, &fakeResourceDescriber{}, &fakeResourceCreator{}, &fakeResourceUpdater{}, deleter, "")

	response, err := adapter.Respond(context.Background(), InteractionInput{
		Kind: InputSelection, Key: searchResultsKey, Value: "MATERIAL|CONDUCTORES|M|1",
	})
	if err != nil {
		t.Fatalf("Respond(open detail) error = %v, want nil", err)
	}
	if _, ok := response.Pending.(ActionRequest); !ok {
		t.Fatalf("Pending = %T, want ActionRequest after opening detail", response.Pending)
	}
	return adapter, deleter
}

// TestResourcesWorkspaceAdapterRespondDeleteActionShowsConfirmation covers
// selecting "Eliminar" from the detail view: it shows a ConfirmationRequest
// naming the resource and does NOT call Delete yet.
func TestResourcesWorkspaceAdapterRespondDeleteActionShowsConfirmation(t *testing.T) {
	resource := domain.Resource{
		ID: 42, ClassCode: "MATERIAL", FamilyCode: "CONDUCTORES", NaturalUnit: "M", IdentityKey: "CONDUCTORES|M|1",
	}
	adapter, deleter := openDeleteConfirmationDetail(t, resource)

	response, err := adapter.Respond(context.Background(), InteractionInput{Kind: InputAction, ActionID: deleteActionID})
	if err != nil {
		t.Fatalf("Respond(delete action) error = %v, want nil", err)
	}
	confirmation, ok := response.Pending.(ConfirmationRequest)
	if !ok {
		t.Fatalf("Pending = %T, want ConfirmationRequest", response.Pending)
	}
	if !strings.Contains(confirmation.Question, "CONDUCTORES") {
		t.Fatalf("Question = %q, want it to name the resource", confirmation.Question)
	}
	if confirmation.Key != resourcesDeleteConfirmKey {
		t.Fatalf("Key = %q, want %q", confirmation.Key, resourcesDeleteConfirmKey)
	}
	if deleter.callCount != 0 {
		t.Fatalf("Delete callCount = %d, want 0 before confirming", deleter.callCount)
	}
}

// TestResourcesWorkspaceAdapterRespondDeleteConfirmYesCallsDelete covers
// confirming "yes": Delete is called exactly once with the resource's ID.
func TestResourcesWorkspaceAdapterRespondDeleteConfirmYesCallsDelete(t *testing.T) {
	resource := domain.Resource{
		ID: 42, ClassCode: "MATERIAL", FamilyCode: "CONDUCTORES", NaturalUnit: "M", IdentityKey: "CONDUCTORES|M|1",
	}
	adapter, deleter := openDeleteConfirmationDetail(t, resource)

	if _, err := adapter.Respond(context.Background(), InteractionInput{Kind: InputAction, ActionID: deleteActionID}); err != nil {
		t.Fatalf("Respond(delete action) error = %v, want nil", err)
	}

	response, err := adapter.Respond(context.Background(), InteractionInput{
		Kind: InputSelection, Key: resourcesDeleteConfirmKey, Value: "yes",
	})
	if err != nil {
		t.Fatalf("Respond(confirm yes) error = %v, want nil", err)
	}
	if deleter.callCount != 1 {
		t.Fatalf("Delete callCount = %d, want exactly 1", deleter.callCount)
	}
	if deleter.gotID != 42 {
		t.Fatalf("Delete gotID = %d, want %d", deleter.gotID, 42)
	}
	if response.Pending != nil {
		t.Fatalf("Pending = %#v, want nil after a successful delete", response.Pending)
	}
	if len(response.Messages) != 1 {
		t.Fatalf("Messages = %v, want exactly one message", response.Messages)
	}
	message, ok := response.Messages[0].(TextMessage)
	if !ok {
		t.Fatalf("Messages[0] = %T, want TextMessage", response.Messages[0])
	}
	if !strings.Contains(message.Text, "CONDUCTORES") {
		t.Fatalf("text = %q, want it to mention the deleted resource", message.Text)
	}
}

// TestResourcesWorkspaceAdapterRespondDeleteConfirmNoReturnsToSameDetail
// covers declining "no": Delete is NEVER called, and the response shows the
// SAME detail again with the full 4-action menu, not a bare cancellation.
func TestResourcesWorkspaceAdapterRespondDeleteConfirmNoReturnsToSameDetail(t *testing.T) {
	resource := domain.Resource{
		ID: 42, ClassCode: "MATERIAL", FamilyCode: "CONDUCTORES", NaturalUnit: "M", IdentityKey: "CONDUCTORES|M|1",
	}
	adapter, deleter := openDeleteConfirmationDetail(t, resource)

	if _, err := adapter.Respond(context.Background(), InteractionInput{Kind: InputAction, ActionID: deleteActionID}); err != nil {
		t.Fatalf("Respond(delete action) error = %v, want nil", err)
	}

	response, err := adapter.Respond(context.Background(), InteractionInput{
		Kind: InputSelection, Key: resourcesDeleteConfirmKey, Value: "no",
	})
	if err != nil {
		t.Fatalf("Respond(confirm no) error = %v, want nil", err)
	}
	if deleter.callCount != 0 {
		t.Fatalf("Delete callCount = %d, want 0 after declining", deleter.callCount)
	}
	if len(response.Messages) != 1 {
		t.Fatalf("Messages = %v, want exactly one message (the same detail again)", response.Messages)
	}
	result, ok := response.Messages[0].(StructuredResult)
	if !ok {
		t.Fatalf("Messages[0] = %T, want StructuredResult", response.Messages[0])
	}
	if !strings.HasPrefix(result.Title, "CONDUCTORES") {
		t.Fatalf("Title = %q, want it to start with CONDUCTORES", result.Title)
	}
	action, ok := response.Pending.(ActionRequest)
	if !ok {
		t.Fatalf("Pending = %T, want ActionRequest", response.Pending)
	}
	if len(action.Actions) != 4 || action.Actions[0].ID != editActionID || action.Actions[1].ID != duplicateActionID || action.Actions[2].ID != deleteActionID || action.Actions[3].ID != backActionID {
		t.Fatalf("Actions = %+v, want exactly [%q, %q, %q, %q] in that order", action.Actions, editActionID, duplicateActionID, deleteActionID, backActionID)
	}
}

// TestResourcesWorkspaceAdapterRespondDeleteFailureNeverLeaksRawError
// mirrors the Search/Get failure discipline for the Delete failure path.
func TestResourcesWorkspaceAdapterRespondDeleteFailureNeverLeaksRawError(t *testing.T) {
	resource := domain.Resource{
		ID: 42, ClassCode: "MATERIAL", FamilyCode: "CONDUCTORES", NaturalUnit: "M", IdentityKey: "CONDUCTORES|M|1",
	}
	getter := &fakeResourceGetter{resource: resource}
	deleter := &fakeResourceDeleter{err: errors.New("connect to database: dial tcp refused")}
	adapter := newDispatchAdapter(&fakeResourceSearcher{}, getter, &fakeResourceDescriber{}, &fakeResourceCreator{}, &fakeResourceUpdater{}, deleter, "")

	if _, err := adapter.Respond(context.Background(), InteractionInput{
		Kind: InputSelection, Key: searchResultsKey, Value: "MATERIAL|CONDUCTORES|M|1",
	}); err != nil {
		t.Fatalf("Respond(open detail) error = %v, want nil", err)
	}
	if _, err := adapter.Respond(context.Background(), InteractionInput{Kind: InputAction, ActionID: deleteActionID}); err != nil {
		t.Fatalf("Respond(delete action) error = %v, want nil", err)
	}

	response, err := adapter.Respond(context.Background(), InteractionInput{
		Kind: InputSelection, Key: resourcesDeleteConfirmKey, Value: "yes",
	})
	if err != nil {
		t.Fatalf("Respond(confirm yes) error = %v, want nil (failure must already be converted to an InteractionMessage)", err)
	}
	if response.Pending != nil {
		t.Fatalf("Pending = %#v, want nil when Delete fails", response.Pending)
	}
	if len(response.Messages) != 1 {
		t.Fatalf("Messages = %v, want exactly one message", response.Messages)
	}
	message, ok := response.Messages[0].(ErrorMessage)
	if !ok {
		t.Fatalf("Messages[0] = %T, want ErrorMessage", response.Messages[0])
	}
	if strings.Contains(message.Text, "connect to database") || strings.Contains(message.Text, "dial tcp") {
		t.Fatalf("text = %q, must not leak the raw error", message.Text)
	}
}

// TestResourcesWorkspaceAdapterRespondDeleteThenSearchStillWorks is a
// regression guard: after a successful delete, an ordinary subsequent search
// still works normally.
func TestResourcesWorkspaceAdapterRespondDeleteThenSearchStillWorks(t *testing.T) {
	resource := domain.Resource{
		ID: 42, ClassCode: "MATERIAL", FamilyCode: "CONDUCTORES", NaturalUnit: "M", IdentityKey: "CONDUCTORES|M|1",
	}
	getter := &fakeResourceGetter{resource: resource}
	deleter := &fakeResourceDeleter{}
	fake := &fakeResourceSearcher{results: []domain.Resource{
		{ClassCode: "MATERIAL", FamilyCode: "CEMENT", NaturalUnit: "kg", IdentityKey: "CEMENT|kg|1"},
	}}
	adapter := newDispatchAdapter(fake, getter, &fakeResourceDescriber{}, &fakeResourceCreator{}, &fakeResourceUpdater{}, deleter, "")

	if _, err := adapter.Respond(context.Background(), InteractionInput{
		Kind: InputSelection, Key: searchResultsKey, Value: "MATERIAL|CONDUCTORES|M|1",
	}); err != nil {
		t.Fatalf("Respond(open detail) error = %v, want nil", err)
	}
	if _, err := adapter.Respond(context.Background(), InteractionInput{Kind: InputAction, ActionID: deleteActionID}); err != nil {
		t.Fatalf("Respond(delete action) error = %v, want nil", err)
	}
	if _, err := adapter.Respond(context.Background(), InteractionInput{
		Kind: InputSelection, Key: resourcesDeleteConfirmKey, Value: "yes",
	}); err != nil {
		t.Fatalf("Respond(confirm yes) error = %v, want nil", err)
	}

	searchResponse, err := adapter.Respond(context.Background(), InteractionInput{Kind: InputText, Value: "cemento"})
	if err != nil {
		t.Fatalf("Respond(search) error = %v, want nil", err)
	}
	if fake.callCount != 1 {
		t.Fatalf("Search call count = %d, want 1 (a fresh search after a delete must work normally)", fake.callCount)
	}
	if fake.gotCriteria.Text != "cemento" {
		t.Fatalf("gotCriteria.Text = %q, want %q", fake.gotCriteria.Text, "cemento")
	}
	if _, ok := searchResponse.Pending.(QuestionRequest); !ok {
		t.Fatalf("Pending = %T, want a normal search-results QuestionRequest", searchResponse.Pending)
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
