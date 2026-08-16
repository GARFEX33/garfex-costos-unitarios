package tui

import (
	"context"
	"strings"
	"testing"

	"github.com/GARFEX33/garfex-costos-unitarios/internal/domain"
)

func TestU3CatalogStatusDiscovery(t *testing.T) {
	ctx := context.Background()
	inactive := classRecord(7, "INACTIVA", "Clase inactiva")
	inactive.Active = false
	tests := []struct {
		name, selection string
		status          domain.CatalogStatus
		records         []domain.CatalogRecord
		wantText        string
	}{
		{"active", catalogStatusActiveID, domain.CatalogStatusActive, []domain.CatalogRecord{classRecord(6, "ACTIVA", "Clase activa")}, "Clase activa"},
		{"inactive", catalogStatusInactiveID, domain.CatalogStatusInactive, []domain.CatalogRecord{inactive}, "Clase inactiva"},
		{"all", catalogStatusAllID, domain.CatalogStatusAll, []domain.CatalogRecord{classRecord(6, "ACTIVA", "Clase activa"), inactive}, "Clase inactiva"},
		{"empty", catalogStatusInactiveID, domain.CatalogStatusInactive, nil, "No hay registros para este estado."},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lister := &fakeCatalogLister{records: map[domain.CatalogKindCode][]domain.CatalogRecord{domain.KindClass: tt.records}}
			a := newCatalogAdminAdapter(lister, &fakeCatalogGetter{}, &fakeCatalogCreator{}, &fakeCatalogUpdater{})
			status, err := a.Respond(ctx, InteractionInput{Kind: InputAction, ActionID: catalogOpenActionID(domain.KindClass)})
			if err != nil {
				t.Fatal(err)
			}
			q, ok := status.Pending.(QuestionRequest)
			if !ok || len(q.Options) != 3 || q.Options[0].Label != "Activos" || q.Options[1].Label != "Inactivos" || q.Options[2].Label != "Todos" {
				t.Fatalf("status request = %#v", status.Pending)
			}
			list, err := a.Respond(ctx, InteractionInput{Kind: InputSelection, Key: catalogStatusMenuKey, Value: tt.selection})
			if err != nil {
				t.Fatal(err)
			}
			if lister.lastFilter.Status != tt.status || !responseContains(list, tt.wantText) {
				t.Fatalf("status/filter response = %v/%#v, want %v/%q", lister.lastFilter.Status, list, tt.status, tt.wantText)
			}
		})
	}
}

func TestU3DetailLifecycleIsTruthful(t *testing.T) {
	inactive := classRecord(7, "INACTIVA", "Clase inactiva")
	inactive.Active = false
	response := (&CatalogAdminAdapter{registry: domain.NewCatalogRegistry(), activeKind: domain.KindClass, activeRecord: inactive}).recordDetailResponse()
	result := response.Messages[0].(StructuredResult)
	if !fieldsContain(result.Fields, "Estado", "Inactivo") || !actionsContain(response.Pending.(ActionRequest).Actions, "Reactivar") {
		t.Fatalf("supported inactive detail = %#v", response)
	}

	unsupported := definitionRecord(8, "ATTR", "Característica")
	unsupported.Active = false
	response = (&CatalogAdminAdapter{registry: domain.NewCatalogRegistry(), activeKind: domain.KindAttributeDefinition, activeRecord: unsupported}).recordDetailResponse()
	if !responseContains(response, "Reactivación no disponible para este tipo de catálogo") || actionsContain(response.Pending.(ActionRequest).Actions, "Reactivar") {
		t.Fatalf("unsupported inactive detail = %#v", response)
	}
}

func TestU3ReferenceOptionsAlwaysRequestActive(t *testing.T) {
	lister := &fakeCatalogLister{records: map[domain.CatalogKindCode][]domain.CatalogRecord{domain.KindClass: {classRecord(1, "ACTIVA", "Activa")}}}
	a := newCatalogAdminAdapter(lister, &fakeCatalogGetter{}, &fakeCatalogCreator{}, &fakeCatalogUpdater{})
	_, err := a.refOptions(context.Background(), domain.FieldDescriptor{RefKind: domain.KindClass}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if lister.lastFilter.Status != domain.CatalogStatusActive {
		t.Fatalf("refOptions status = %v, want Active", lister.lastFilter.Status)
	}
}

func responseContains(response InteractionResponse, want string) bool {
	for _, message := range response.Messages {
		switch value := message.(type) {
		case TextMessage:
			if strings.Contains(value.Text, want) {
				return true
			}
		case StructuredResult:
			if strings.Contains(value.Title, want) {
				return true
			}
		}
	}
	if question, ok := response.Pending.(QuestionRequest); ok {
		for _, option := range question.Options {
			if strings.Contains(option.Label, want) {
				return true
			}
		}
	}
	return false
}

func fieldsContain(fields []Field, label, value string) bool {
	for _, field := range fields {
		if field.Label == label && field.Value == value {
			return true
		}
	}
	return false
}

func actionsContain(actions []Action, label string) bool {
	for _, action := range actions {
		if action.Label == label {
			return true
		}
	}
	return false
}
