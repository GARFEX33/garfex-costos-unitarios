package tui

import (
	"context"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/GARFEX33/garfex-costos-unitarios/internal/domain"
)

type catalogSessionAgent struct{ classes int }

func (catalogSessionAgent) Respond(context.Context, InteractionInput) (InteractionResponse, error) {
	return InteractionResponse{}, nil
}

func TestModelRefreshMatchesFreshSessionAndInvalidatesOpenResourceFlows(t *testing.T) {
	initial := domain.ResourceCatalog{Classes: []domain.ResourceClass{{Code: "MATERIAL", Name: "Material", Plural: "Materiales", Slug: "materiales", Active: true}}}
	authority := domain.NewCatalogAuthority(initial)
	agentFor := func(c domain.ResourceCatalog, _ string) InteractionAgent {
		return catalogSessionAgent{classes: len(c.Classes)}
	}
	registry := domain.NewCatalogRegistry()
	model := NewWithCatalogAuthority(Handlers{}, catalogSessionAgent{}, authority, agentFor, registry, catalogSessionAgent{})

	if !model.enterWorkspace("materiales") {
		t.Fatal("enterWorkspace(materiales) = false")
	}
	model.pending = QuestionRequest{Key: "obsolete-editor"}
	next := initial
	next.Classes = append([]domain.ResourceClass(nil), initial.Classes...)
	next.Classes[0].Active = false
	next.Classes = append(next.Classes, domain.ResourceClass{Code: "SERVICIO", Name: "Servicio", Plural: "Servicios", Slug: "servicios", Active: true})
	authority.Publish(next)

	updated, _ := model.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	model = updated.(Model)
	fresh := NewWithCatalogAuthority(Handlers{}, catalogSessionAgent{}, authority, agentFor, registry, catalogSessionAgent{})
	if _, ok := model.workspaces["materiales"]; ok {
		t.Fatal("inactive class workspace survived refresh")
	}
	if _, ok := model.workspaces["servicios"]; !ok {
		t.Fatal("new active class workspace is missing")
	}
	if model.activeWorkspace != "" {
		t.Fatalf("active workspace = %q, want explicit return to Assistant", model.activeWorkspace)
	}
	if len(model.workspaces) != len(fresh.workspaces) || len(model.assistantActions) != len(fresh.assistantActions) {
		t.Fatalf("same-session topology differs from fresh session: workspaces %d/%d, actions %d/%d", len(model.workspaces), len(fresh.workspaces), len(model.assistantActions), len(fresh.assistantActions))
	}
	if got := model.history[len(model.history)-1].text; got == "" {
		t.Fatal("catalog refresh did not explicitly report invalidated resource flows")
	}
	for slug, slot := range model.workspaces {
		if agent, ok := slot.agent.(catalogSessionAgent); ok && slug != configuracionSlug && agent.classes != 2 {
			t.Fatalf("workspace %q bound to %d classes, want captured snapshot with 2", slug, agent.classes)
		}
	}
}
