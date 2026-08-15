package tui

import (
	"context"
	"testing"

	"github.com/GARFEX33/garfex-costos-unitarios/internal/domain"
)

// TestCatalogAdminScreensNeverRenderRawCatalogKindCodes extends
// internal/domain's TestKnownResourceCodesAreNeverRenderedOrTranslated
// (recursos-maestro PR3 guard rail) to the new catalog-admin screens (task
// 6.3): every registered CatalogKindCode constant (e.g. "CLASE", "FAMILIA",
// "APLICABILIDAD") must never appear verbatim as a rendered palette/menu
// label — only each kind's own Spanish Singular/Plural (a FieldDescriptor's
// own Spanish Label, or a record's Name/Label/Plural field) may render.
func TestCatalogAdminScreensNeverRenderRawCatalogKindCodes(t *testing.T) {
	registry := domain.NewCatalogRegistry()
	kinds := registry.Kinds()

	rawCodes := map[string]bool{}
	for _, kind := range kinds {
		rawCodes[string(kind.Code)] = true
	}

	assertNeverRaw := func(t *testing.T, label string) {
		t.Helper()
		if rawCodes[label] {
			t.Fatalf("label %q is a raw CatalogKindCode, must render its Spanish Singular/Plural instead", label)
		}
	}

	t.Run("palette subtree labels", func(t *testing.T) {
		root := buildCatalogAdminActions(kinds)
		assertNeverRaw(t, root.label)
		for _, group := range root.children {
			assertNeverRaw(t, group.label)
			for _, leaf := range group.children {
				assertNeverRaw(t, leaf.label)
			}
		}
	})

	t.Run("assistant top-level Configuración leaf", func(t *testing.T) {
		actions := buildAssistantActions(nil, kinds)
		for _, action := range actions {
			assertNeverRaw(t, action.label)
		}
	})

	t.Run("field labels", func(t *testing.T) {
		for _, kind := range kinds {
			for _, field := range kind.Fields {
				assertNeverRaw(t, field.Label)
			}
		}
	})

	t.Run("kind menu options render display names, never raw code", func(t *testing.T) {
		lister := &fakeCatalogLister{records: map[domain.CatalogKindCode][]domain.CatalogRecord{
			domain.KindClass: {classRecord(1, "MATERIAL", "Materiales")},
		}}
		adapter := newCatalogAdminAdapter(lister, &fakeCatalogGetter{}, &fakeCatalogCreator{}, &fakeCatalogUpdater{})
		response, err := adapter.startKindMenu(context.Background(), domain.KindClass)
		if err != nil {
			t.Fatalf("startKindMenu() error = %v", err)
		}
		question := response.Pending.(QuestionRequest)
		for _, opt := range question.Options {
			if opt.Label == "MATERIAL" {
				t.Fatalf("option label = %q, must never render the raw Código — only the Name field", opt.Label)
			}
		}
	})

	t.Run("create-flow field questions never prompt with a raw code", func(t *testing.T) {
		adapter := newCatalogAdminAdapter(&fakeCatalogLister{}, &fakeCatalogGetter{}, &fakeCatalogCreator{}, &fakeCatalogUpdater{})
		for _, kind := range kinds {
			response, err := adapter.startCreateFlow(context.Background(), kind.Code)
			if err != nil {
				t.Fatalf("startCreateFlow(%v) error = %v", kind.Code, err)
			}
			// Some kinds legitimately block immediately (zero required ref
			// options, e.g. Familia with zero Clases) — that is itself
			// exercised elsewhere; here we only assert whatever WAS rendered
			// never leaks a raw CatalogKindCode as its Prompt.
			if question, ok := response.Pending.(QuestionRequest); ok {
				assertNeverRaw(t, question.Prompt)
			}
		}
	})
}
