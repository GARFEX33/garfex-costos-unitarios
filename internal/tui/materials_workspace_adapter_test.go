package tui

import (
	"context"
	"strings"
	"testing"

	"github.com/GARFEX33/garfex-costos-unitarios/internal/domain"
)

func TestMaterialsWorkspaceAdapterGreeting(t *testing.T) {
	adapter := NewMaterialsWorkspaceAdapter(&fakeMaterialGetter{})
	greeting, ok := adapter.Greeting().(TextMessage)
	if !ok {
		t.Fatalf("Greeting() = %T, want TextMessage", adapter.Greeting())
	}
	if !containsAll(greeting.Text, "conectado", "catálogo real") {
		t.Fatalf("Greeting() text = %q, want it to mention the real catalog connection", greeting.Text)
	}
	if !containsAll(greeting.Text, "búsqueda", "no está implementada") {
		t.Fatalf("Greeting() text = %q, want it to mention search is not implemented yet", greeting.Text)
	}
}

func TestMaterialsWorkspaceAdapterRespondNeverFabricatesQuestionsOrResults(t *testing.T) {
	adapter := NewMaterialsWorkspaceAdapter(&fakeMaterialGetter{})
	inputs := []InteractionInput{
		{Kind: InputText, Value: "cemento"},
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
		if !containsAll(message.Text, "búsqueda", "no está implementada") {
			t.Fatalf("Respond(%+v) text = %q, want the honest status message", input, message.Text)
		}
	}
}

func TestMaterialsWorkspaceAdapterAcceptsMaterialGetter(t *testing.T) {
	var getter materialGetter = &fakeMaterialGetter{material: domain.Material{FamilyCode: "CEMENT"}}
	adapter := NewMaterialsWorkspaceAdapter(getter)
	if adapter == nil {
		t.Fatal("NewMaterialsWorkspaceAdapter() = nil")
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
