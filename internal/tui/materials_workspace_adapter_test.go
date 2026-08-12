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
	adapter := NewMaterialsWorkspaceAdapter(&fakeMaterialSearcher{})
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

func TestMaterialsWorkspaceAdapterRespondNonTextInputsNeverFabricateOrSearch(t *testing.T) {
	fake := &fakeMaterialSearcher{}
	adapter := NewMaterialsWorkspaceAdapter(fake)
	inputs := []InteractionInput{
		{Kind: InputSelection, Value: "some-option"},
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
		t.Fatalf("Search call count = %d, want 0 for non-InputText inputs", fake.callCount)
	}
	if fake.gotCriteria.Text != "" || fake.gotCriteria.Limit != 0 {
		t.Fatalf("gotCriteria = %+v, want zero-value (Search never called)", fake.gotCriteria)
	}
}

func TestMaterialsWorkspaceAdapterAcceptsMaterialSearcher(t *testing.T) {
	var searcher materialSearcher = &fakeMaterialSearcher{}
	adapter := NewMaterialsWorkspaceAdapter(searcher)
	if adapter == nil {
		t.Fatal("NewMaterialsWorkspaceAdapter() = nil")
	}
}

func TestMaterialsWorkspaceAdapterRespondSendsTextUnchanged(t *testing.T) {
	fake := &fakeMaterialSearcher{}
	adapter := NewMaterialsWorkspaceAdapter(fake)
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
	adapter := NewMaterialsWorkspaceAdapter(fake)
	if _, err := adapter.Respond(context.Background(), InteractionInput{Kind: InputText, Value: "cemento"}); err != nil {
		t.Fatalf("Respond() error = %v, want nil", err)
	}
	if fake.gotCriteria.Limit != 11 {
		t.Fatalf("gotCriteria.Limit = %d, want 11", fake.gotCriteria.Limit)
	}
}

func TestMaterialsWorkspaceAdapterRespondZeroResults(t *testing.T) {
	fake := &fakeMaterialSearcher{results: nil}
	adapter := NewMaterialsWorkspaceAdapter(fake)
	response, err := adapter.Respond(context.Background(), InteractionInput{Kind: InputText, Value: "inexistente"})
	if err != nil {
		t.Fatalf("Respond() error = %v, want nil", err)
	}
	if len(response.Messages) != 1 {
		t.Fatalf("Messages = %v, want exactly one message", response.Messages)
	}
	message, ok := response.Messages[0].(TextMessage)
	if !ok {
		t.Fatalf("Messages[0] = %T, want TextMessage", response.Messages[0])
	}
	if !strings.Contains(message.Text, "inexistente") {
		t.Fatalf("text = %q, want it to mention the searched text", message.Text)
	}
}

func TestMaterialsWorkspaceAdapterRespondErrorNeverLeaksRawError(t *testing.T) {
	fake := &fakeMaterialSearcher{err: errors.New("connect to database: dial tcp refused")}
	adapter := NewMaterialsWorkspaceAdapter(fake)
	response, err := adapter.Respond(context.Background(), InteractionInput{Kind: InputText, Value: "cemento"})
	if err != nil {
		t.Fatalf("Respond() error = %v, want nil (failure must already be converted to an InteractionMessage)", err)
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

func TestMaterialsWorkspaceAdapterRespondOneResultOmitsIdentityKey(t *testing.T) {
	material := domain.Material{
		FamilyCode: "CEMENT", NaturalUnit: "kg", IdentityKey: "CEMENT|kg|SECRET-42",
		Attributes: []domain.MaterialAttributeValue{domain.OptionValue("color", "GRIS")},
	}
	fake := &fakeMaterialSearcher{results: []domain.Material{material}}
	adapter := NewMaterialsWorkspaceAdapter(fake)
	response, err := adapter.Respond(context.Background(), InteractionInput{Kind: InputText, Value: "cemento"})
	if err != nil {
		t.Fatalf("Respond() error = %v, want nil", err)
	}
	if len(response.Messages) != 1 {
		t.Fatalf("Messages = %v, want exactly one message", response.Messages)
	}
	result, ok := response.Messages[0].(StructuredResult)
	if !ok {
		t.Fatalf("Messages[0] = %T, want StructuredResult", response.Messages[0])
	}
	if len(result.Sections) != 1 {
		t.Fatalf("Sections = %v, want exactly 1 section", result.Sections)
	}
	rendered := renderInteractionMessage(result)
	if strings.Contains(rendered, material.IdentityKey) {
		t.Fatalf("rendered result = %q, must never contain IdentityKey %q", rendered, material.IdentityKey)
	}
	for _, field := range result.Sections[0].Fields {
		if strings.Contains(field.Value, material.IdentityKey) {
			t.Fatalf("field %+v leaks IdentityKey", field)
		}
	}
}

func TestMaterialsWorkspaceAdapterRespondMultipleResultsPreserveOrder(t *testing.T) {
	materials := []domain.Material{
		{FamilyCode: "CEMENT", NaturalUnit: "kg", IdentityKey: "CEMENT|kg|1"},
		{FamilyCode: "ARENA", NaturalUnit: "m3", IdentityKey: "ARENA|m3|2"},
		{FamilyCode: "GRAVA", NaturalUnit: "m3", IdentityKey: "GRAVA|m3|3"},
	}
	fake := &fakeMaterialSearcher{results: materials}
	adapter := NewMaterialsWorkspaceAdapter(fake)
	response, err := adapter.Respond(context.Background(), InteractionInput{Kind: InputText, Value: "x"})
	if err != nil {
		t.Fatalf("Respond() error = %v, want nil", err)
	}
	result, ok := response.Messages[0].(StructuredResult)
	if !ok {
		t.Fatalf("Messages[0] = %T, want StructuredResult", response.Messages[0])
	}
	if len(result.Sections) != len(materials) {
		t.Fatalf("Sections = %d, want %d", len(result.Sections), len(materials))
	}
	for i, material := range materials {
		if !strings.HasPrefix(result.Sections[i].Title, material.FamilyCode) {
			t.Fatalf("Sections[%d].Title = %q, want it to start with %q (order preserved)", i, result.Sections[i].Title, material.FamilyCode)
		}
	}
}

func TestMaterialsWorkspaceAdapterRespondTenResultsNoMoreResultsHint(t *testing.T) {
	materials := make([]domain.Material, 10)
	for i := range materials {
		materials[i] = domain.Material{FamilyCode: fmt.Sprintf("FAM%d", i), NaturalUnit: "u", IdentityKey: fmt.Sprintf("FAM%d|u|%d", i, i)}
	}
	fake := &fakeMaterialSearcher{results: materials}
	adapter := NewMaterialsWorkspaceAdapter(fake)
	response, err := adapter.Respond(context.Background(), InteractionInput{Kind: InputText, Value: "x"})
	if err != nil {
		t.Fatalf("Respond() error = %v, want nil", err)
	}
	if len(response.Messages) != 1 {
		t.Fatalf("Messages = %v, want exactly one message for exactly 10 results", response.Messages)
	}
	result, ok := response.Messages[0].(StructuredResult)
	if !ok {
		t.Fatalf("Messages[0] = %T, want StructuredResult", response.Messages[0])
	}
	if len(result.Sections) != 10 {
		t.Fatalf("Sections = %d, want 10", len(result.Sections))
	}
	rendered := renderInteractionMessage(result)
	if strings.Contains(rendered, "más resultados") {
		t.Fatalf("rendered = %q, must not hint at more results for exactly 10", rendered)
	}
}

func TestMaterialsWorkspaceAdapterRespondElevenResultsShowsTenAndMoreHint(t *testing.T) {
	materials := make([]domain.Material, 11)
	for i := range materials {
		materials[i] = domain.Material{FamilyCode: fmt.Sprintf("FAM%d", i), NaturalUnit: "u", IdentityKey: fmt.Sprintf("FAM%d|u|%d", i, i)}
	}
	fake := &fakeMaterialSearcher{results: materials}
	adapter := NewMaterialsWorkspaceAdapter(fake)
	response, err := adapter.Respond(context.Background(), InteractionInput{Kind: InputText, Value: "x"})
	if err != nil {
		t.Fatalf("Respond() error = %v, want nil", err)
	}
	result, ok := response.Messages[0].(StructuredResult)
	if !ok {
		t.Fatalf("Messages[0] = %T, want StructuredResult", response.Messages[0])
	}
	if len(result.Sections) != 10 {
		t.Fatalf("Sections = %d, want exactly 10 (N+1 signal trimmed)", len(result.Sections))
	}
	var rendered strings.Builder
	for _, message := range response.Messages {
		rendered.WriteString(renderInteractionMessage(message))
	}
	if !strings.Contains(rendered.String(), "más resultados") {
		t.Fatalf("rendered = %q, want a hint that more results exist", rendered.String())
	}
}

func TestMaterialsWorkspaceAdapterRespondOmitsNotApplicableAttributes(t *testing.T) {
	material := domain.Material{
		FamilyCode: "CONDUCTORES", NaturalUnit: "m", IdentityKey: "CONDUCTORES|m|1",
		Attributes: []domain.MaterialAttributeValue{
			domain.OptionValue("insulation", "DESNUDO"),
			{AttributeCode: "color", Type: domain.ValueTypeControlledOption, Text: "NOT_APPLICABLE"},
		},
	}
	fake := &fakeMaterialSearcher{results: []domain.Material{material}}
	adapter := NewMaterialsWorkspaceAdapter(fake)
	response, err := adapter.Respond(context.Background(), InteractionInput{Kind: InputText, Value: "conductor"})
	if err != nil {
		t.Fatalf("Respond() error = %v, want nil", err)
	}
	result, ok := response.Messages[0].(StructuredResult)
	if !ok {
		t.Fatalf("Messages[0] = %T, want StructuredResult", response.Messages[0])
	}
	for _, field := range result.Sections[0].Fields {
		if field.Label == "color" {
			t.Fatalf("Fields = %+v, want NOT_APPLICABLE attribute %q omitted", result.Sections[0].Fields, "color")
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
