package tui

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/GARFEX33/garfex-costos-unitarios/internal/domain"
)

func testCableResource(t *testing.T) domain.Resource {
	t.Helper()
	resource, err := domain.NewResource(
		domain.SeedResourceCatalog(),
		domain.ResourceScope{ClassCode: "MATERIAL", FamilyCode: "CONDUCTORES", TypeCode: "CABLE"},
		"M",
		cableAttributeValues(),
	)
	if err != nil {
		t.Fatalf("NewResource() error = %v, want nil", err)
	}
	resource.ID = 42
	return resource
}

func assertResourceDetailOrigin(t *testing.T, response InteractionResponse) {
	t.Helper()
	if _, ok := response.Pending.(ActionRequest); !ok {
		t.Fatalf("Pending = %T, want ActionRequest for the original detail", response.Pending)
	}
	for _, message := range response.Messages {
		if _, ok := message.(StructuredResult); ok {
			return
		}
	}
	t.Fatalf("Messages = %#v, want the original StructuredResult", response.Messages)
}

func TestResourceEditorCancelReturnsToItsOrigin(t *testing.T) {
	resource := testCableResource(t)
	for _, mode := range []struct {
		name  string
		start func(*testing.T, *ResourcesWorkspaceAdapter, domain.Resource) InteractionResponse
	}{
		{name: "edit", start: openEditFor},
		{name: "duplicate", start: openDuplicateFor},
	} {
		t.Run(mode.name, func(t *testing.T) {
			adapter := newTestAdapter(&fakeResourceGetter{}, &fakeResourceCreator{}, &fakeResourceUpdater{}, "MATERIAL")
			_ = mode.start(t, adapter, resource)

			response, err := adapter.Respond(context.Background(), InteractionInput{Kind: InputCancel})
			if err != nil {
				t.Fatalf("Respond(cancel) error = %v, want nil", err)
			}
			if adapter.editor != nil {
				t.Fatalf("adapter.editor = %#v, want nil", adapter.editor)
			}
			assertResourceDetailOrigin(t, response)
		})
	}

	t.Run("create", func(t *testing.T) {
		adapter := newTestAdapter(&fakeResourceGetter{}, &fakeResourceCreator{}, &fakeResourceUpdater{}, "MATERIAL")
		if _, err := adapter.startCreateEditor(); err != nil {
			t.Fatalf("startCreateEditor() error = %v, want nil", err)
		}
		response, err := adapter.Respond(context.Background(), InteractionInput{Kind: InputCancel})
		if err != nil {
			t.Fatalf("Respond(cancel) error = %v, want nil", err)
		}
		if response.Pending != nil || adapter.editor != nil {
			t.Fatalf("cancelled create = (%T, %#v), want origin menu and discarded editor", response.Pending, adapter.editor)
		}
	})
}

func TestResourceEditorDeclinedConfirmationReturnsToDetail(t *testing.T) {
	resource := testCableResource(t)
	adapter := newTestAdapter(&fakeResourceGetter{}, &fakeResourceCreator{}, &fakeResourceUpdater{}, "MATERIAL")
	response := openEditFor(t, adapter, resource)
	response = answerQuestion(t, adapter, response, "color")
	response = answerQuestion(t, adapter, response, "BLANCO")
	response = answerQuestion(t, adapter, response, editFinishFieldCode)
	response = answerConfirmation(t, adapter, response, "no")
	assertResourceDetailOrigin(t, response)
}

func TestResourceEditorRecoverableFailuresPreserveDraft(t *testing.T) {
	resource := testCableResource(t)
	cases := []struct {
		name string
		err  error
	}{
		{name: "duplicate", err: domain.ErrDuplicateResource},
		{name: "not found", err: domain.ErrResourceNotFound},
		{name: "reference", err: domain.ErrResourceReference},
		{name: "generic persistence", err: errors.New("database unavailable")},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			updater := &fakeResourceUpdater{err: tt.err}
			adapter := newTestAdapter(&fakeResourceGetter{resource: resource}, &fakeResourceCreator{}, updater, "MATERIAL")
			response := openEditFor(t, adapter, resource)
			response = answerQuestion(t, adapter, response, "color")
			response = answerQuestion(t, adapter, response, "BLANCO")
			response = answerQuestion(t, adapter, response, editFinishFieldCode)
			response = answerConfirmation(t, adapter, response, "yes")
			if _, ok := response.Pending.(QuestionRequest); !ok || updater.callCount != 1 {
				t.Fatalf("Pending = %T, want actionable field picker", response.Pending)
			}
			if tt.name != "generic persistence" {
				cancelled, _ := adapter.Respond(context.Background(), InteractionInput{Kind: InputCancel})
				assertResourceDetailOrigin(t, cancelled)
				return
			}
			updater.err = nil
			response = answerQuestion(t, adapter, response, "color")
			response = answerQuestion(t, adapter, response, "ROJO")
			response = answerQuestion(t, adapter, response, editFinishFieldCode)
			if updater.callCount != 1 {
				t.Fatalf("Update calls before retry confirmation = %d, want 1", updater.callCount)
			}
			response = answerConfirmation(t, adapter, response, "yes")
			if updater.callCount != 2 || !strings.Contains(updater.gotResource.IdentityKey, "color=ROJO") || response.Pending != nil {
				t.Fatalf("successful retry = calls %d, resource %#v, pending %T", updater.callCount, updater.gotResource, response.Pending)
			}
		})
	}
}

func TestResourceEditorValidationFailurePreservesDraft(t *testing.T) {
	resource, err := domain.NewResource(
		domain.SeedResourceCatalog(),
		domain.ResourceScope{ClassCode: "MATERIAL", FamilyCode: "CANALIZACIONES", TypeCode: "TUBERIA"},
		"PZA",
		tuberiaAttributeValues(),
	)
	if err != nil {
		t.Fatalf("NewResource() error = %v, want nil", err)
	}
	resource.ID = 7
	adapter := newTestAdapter(&fakeResourceGetter{}, &fakeResourceCreator{}, &fakeResourceUpdater{}, "MATERIAL")
	response := openEditFor(t, adapter, resource)
	response = answerQuestion(t, adapter, response, "diameter_inch")
	response = answerQuestion(t, adapter, response, `3/4"`)
	response = answerQuestion(t, adapter, response, editFinishFieldCode)
	response = answerConfirmation(t, adapter, response, "yes")

	if _, ok := response.Pending.(QuestionRequest); !ok {
		t.Fatalf("Pending = %T, want actionable field picker", response.Pending)
	}
	cancelled, _ := adapter.Respond(context.Background(), InteractionInput{Kind: InputCancel})
	assertResourceDetailOrigin(t, cancelled)
}

func TestResourceCreateDeclineAfterFailureReturnsToMenu(t *testing.T) {
	creator := &fakeResourceCreator{err: domain.ErrResourceReference}
	adapter := newTestAdapter(&fakeResourceGetter{}, creator, &fakeResourceUpdater{}, "MATERIAL")
	response, _ := adapter.startCreateEditor()
	for _, value := range []string{"CONDUCTORES", "CABLE", "COBRE", "10 AWG", "THW", "NEGRO", "600 V", "M"} {
		response = answerQuestion(t, adapter, response, value)
	}
	if _, ok := response.Pending.(QuestionRequest); !ok || creator.callCount != 1 {
		t.Fatalf("recoverable create = pending %T, calls %d", response.Pending, creator.callCount)
	}
	response = answerQuestion(t, adapter, response, editFinishFieldCode)
	response = answerConfirmation(t, adapter, response, "no")
	if response.Pending != nil || creator.callCount != 1 {
		t.Fatalf("declined create = pending %T, calls %d; want menu and no retry", response.Pending, creator.callCount)
	}
}

func TestResourceDetailUsesCurrentCatalogLabels(t *testing.T) {
	catalog := domain.SeedResourceCatalog()
	for i := range catalog.Attributes {
		if catalog.Attributes[i].Definition.Code == "insulation" {
			catalog.Attributes[i].Definition.Name = "Aislamiento vigente"
		}
	}
	for i := range catalog.Options {
		if catalog.Options[i].AttributeCode == "insulation" && catalog.Options[i].Code == "THW" {
			catalog.Options[i].Label = "Termoplástico vigente"
		}
	}
	adapter := NewResourcesWorkspaceAdapter(nil, &fakeResourceGetter{}, &fakeResourceDescriber{}, &fakeResourceCreator{}, &fakeResourceUpdater{}, nil, catalog, "")
	resource := testCableResource(t)
	resource.Attributes = append(resource.Attributes, domain.OptionValue("insulation", "LEGACY"))
	_, fields := adapter.resourcePresentation(resource)

	var rendered string
	for _, field := range fields {
		rendered += field.Label + ":" + field.Value + "\n"
	}
	if !strings.Contains(rendered, "Aislamiento vigente:Termoplástico vigente") {
		t.Fatalf("fields = %q, want current attribute and option labels", rendered)
	}
	if !strings.Contains(rendered, "Aislamiento vigente:Sin etiqueta (LEGACY)") {
		t.Fatalf("fields = %q, want explicit fallback for an unlabeled option", rendered)
	}
}

func TestPaletteUpdateMatchesWithoutSpanishDiacritics(t *testing.T) {
	m := NewWithCatalog(Handlers{}, NewFakeAgent(), domain.SeedResourceCatalog(), func(string) InteractionAgent {
		return NewFakeAgent()
	}, domain.NewCatalogRegistry(), NewFakeAgent())
	for _, char := range "/configuracion" {
		m, _ = update(t, m, key(char))
	}
	options := filterOptions(actionOptions(m.paletteActions), m.paletteQuery)
	for _, option := range options {
		if option.Label == "Configuración" {
			return
		}
	}
	t.Fatalf("palette options = %#v, want visible label %q", options, "Configuración")
}
