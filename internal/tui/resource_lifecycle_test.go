package tui

import (
	"context"
	"errors"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/GARFEX33/garfex-costos-unitarios/internal/domain"
)

func TestResourcesWorkspaceLifecycleShowsStateAndPreservesCancellation(t *testing.T) {
	resource := domain.Resource{ID: 42, ClassCode: "MATERIAL", FamilyCode: "CONDUCTORES", TypeCode: "CABLE", NaturalUnit: "M", IdentityKey: "v1|resource", Active: true}
	lifecycle := &fakeResourceLifecycle{resource: resource}
	adapter := newLifecycleAdapter(lifecycle, resource)

	response, err := adapter.Respond(context.Background(), InteractionInput{Kind: InputSelection, Key: searchResultsKey, Value: "MATERIAL|v1|resource"})
	if err != nil {
		t.Fatal(err)
	}
	result := response.Messages[0].(StructuredResult)
	if !strings.Contains(result.Title, "Activo") || !hasAction(response, deactivateActionID) {
		t.Fatalf("detail = %+v, want active state and Deactivate action", response)
	}

	response, err = adapter.Respond(context.Background(), InteractionInput{Kind: InputAction, ActionID: deactivateActionID})
	if err != nil {
		t.Fatal(err)
	}
	confirmation := response.Pending.(ConfirmationRequest)
	if !strings.Contains(confirmation.Question, "Desactivar") || lifecycle.deactivateCalls != 0 {
		t.Fatalf("confirmation = %+v, calls = %d; want explicit cancellation gate", confirmation, lifecycle.deactivateCalls)
	}
	response, err = adapter.Respond(context.Background(), InteractionInput{Kind: InputSelection, Key: resourcesLifecycleConfirmKey, Value: "no"})
	if err != nil || lifecycle.deactivateCalls != 0 || response.Pending == nil {
		t.Fatalf("cancel response = %+v, %v; want same detail and no transition", response, err)
	}
}

func TestResourcesWorkspaceLifecycleDiscoversInactiveAndReactivates(t *testing.T) {
	inactive := domain.Resource{ID: 43, ClassCode: "MATERIAL", FamilyCode: "CONDUCTORES", TypeCode: "CABLE", NaturalUnit: "M", IdentityKey: "v1|inactive", Active: false, Revision: 7}
	active := inactive
	active.Active = true
	lifecycle := &fakeResourceLifecycle{resource: inactive, reactivateResult: domain.LifecycleResult{Resource: active, Changed: true}}
	searcher := &fakeResourceSearcher{results: []domain.Resource{inactive}}
	adapter := newLifecycleAdapterWithSearch(lifecycle, searcher)

	response, err := adapter.Respond(context.Background(), InteractionInput{Kind: InputAction, ActionID: inactiveDiscoveryActionID})
	if err != nil || searcher.gotCriteria.LifecycleScope != domain.LifecycleScopeInactive {
		t.Fatalf("inactive discovery = %+v, %v; want explicit inactive scope", searcher.gotCriteria, err)
	}
	question, ok := response.Pending.(QuestionRequest)
	if !ok {
		t.Fatalf("inactive discovery response = %+v", response)
	}
	if !strings.Contains(question.Options[0].Label, "Inactivo") {
		t.Fatalf("inactive option = %+v, want visible state", response.Pending)
	}
	_, err = adapter.Respond(context.Background(), InteractionInput{Kind: InputSelection, Key: searchResultsKey, Value: "MATERIAL|v1|inactive"})
	if err != nil {
		t.Fatal(err)
	}
	response, err = adapter.Respond(context.Background(), InteractionInput{Kind: InputAction, ActionID: reactivateActionID})
	if err != nil || !strings.Contains(response.Pending.(ConfirmationRequest).Question, "Reactivar") {
		t.Fatalf("reactivation confirmation = %+v, %v", response.Pending, err)
	}
	response, err = adapter.Respond(context.Background(), InteractionInput{Kind: InputSelection, Key: resourcesLifecycleConfirmKey, Value: "yes"})
	if err != nil || lifecycle.reactivateCalls != 1 {
		t.Fatalf("reactivation = %+v, %v; calls = %d", response, err, lifecycle.reactivateCalls)
	}
	if len(lifecycle.reactivateRevisions) != 1 || lifecycle.reactivateRevisions[0] != 7 {
		t.Fatalf("reactivateRevisions = %v, want [7] (ReactivateRevision's captured CAS revision)", lifecycle.reactivateRevisions)
	}
	if !strings.Contains(response.Messages[0].(StructuredResult).Title, "Activo") {
		t.Fatalf("reactivated detail = %+v, want active state", response.Messages[0])
	}
}

func TestResourcesWorkspaceReactivateFailureKeepsInactiveDetail(t *testing.T) {
	inactive := domain.Resource{ID: 44, ClassCode: "MATERIAL", FamilyCode: "CONDUCTORES", TypeCode: "CABLE", NaturalUnit: "M", IdentityKey: "v1|conflict", Active: false}
	lifecycle := &fakeResourceLifecycle{resource: inactive, reactivateErr: errors.New("identity conflict")}
	adapter := newLifecycleAdapter(lifecycle, inactive)
	_, _ = adapter.Respond(context.Background(), InteractionInput{Kind: InputSelection, Key: searchResultsKey, Value: "MATERIAL|v1|conflict"})
	_, _ = adapter.Respond(context.Background(), InteractionInput{Kind: InputAction, ActionID: reactivateActionID})
	response, err := adapter.Respond(context.Background(), InteractionInput{Kind: InputSelection, Key: resourcesLifecycleConfirmKey, Value: "yes"})
	if err != nil || lifecycle.reactivateCalls != 1 {
		t.Fatalf("failed reactivation = %+v, %v", response, err)
	}
	message := response.Messages[0].(ErrorMessage)
	if strings.Contains(message.Text, "identity conflict") || adapter.lastDetail.Active {
		t.Fatalf("failure leaked or changed state: %q, %+v", message.Text, adapter.lastDetail)
	}
}

func TestModelUpdateRoutesInactiveDiscoveryFromPalette(t *testing.T) {
	probe := &lifecycleModelProbe{}
	model := NewWithAgent(Handlers{}, probe)
	model.screen, model.interactionMode = screenWorkspace, interactionModeMenu
	model.paletteActions = []assistantAction{{id: inactiveDiscoveryActionID, label: "Buscar recursos inactivos"}}
	next, _ := model.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter}))
	if probe.actionID != inactiveDiscoveryActionID || next.(Model).interactionMode != interactionModeChoice {
		t.Fatalf("Update action=%q mode=%v, want inactive discovery and choice", probe.actionID, next.(Model).interactionMode)
	}
}

type lifecycleModelProbe struct{ actionID string }

func (p *lifecycleModelProbe) Respond(_ context.Context, input InteractionInput) (InteractionResponse, error) {
	p.actionID = input.ActionID
	return InteractionResponse{Pending: QuestionRequest{Key: searchResultsKey}}, nil
}

type fakeResourceLifecycle struct {
	resource            domain.Resource
	deactivateResult    domain.LifecycleResult
	deactivateErr       error
	deactivateCalls     int
	deactivateRevisions []uint64
	reactivateResult    domain.LifecycleResult
	reactivateErr       error
	reactivateCalls     int
	reactivateRevisions []uint64
}

func (f *fakeResourceLifecycle) DeactivateRevision(_ context.Context, _ int64, expectedRevision uint64) (domain.LifecycleResult, error) {
	f.deactivateCalls++
	f.deactivateRevisions = append(f.deactivateRevisions, expectedRevision)
	result := f.deactivateResult
	if result.Resource.ID == 0 {
		result.Resource = f.resource
		result.Resource.Active = false
	}
	return result, f.deactivateErr
}

func (f *fakeResourceLifecycle) ReactivateRevision(_ context.Context, _ int64, expectedRevision uint64) (domain.LifecycleResult, error) {
	f.reactivateCalls++
	f.reactivateRevisions = append(f.reactivateRevisions, expectedRevision)
	result := f.reactivateResult
	if result.Resource.ID == 0 {
		result.Resource = f.resource
		result.Resource.Active = true
	}
	return result, f.reactivateErr
}

func newLifecycleAdapter(lifecycle resourceLifecycle, resource domain.Resource) *ResourcesWorkspaceAdapter {
	return newLifecycleAdapterWithSearch(lifecycle, &fakeResourceSearcher{results: []domain.Resource{resource}})
}

func newLifecycleAdapterWithSearch(lifecycle resourceLifecycle, searcher *fakeResourceSearcher) *ResourcesWorkspaceAdapter {
	return NewResourcesWorkspaceAdapter(searcher, &fakeResourceGetter{resource: lifecycleResource(lifecycle)}, &fakeResourceDescriber{}, &fakeResourceCreator{}, &fakeResourceUpdater{}, lifecycle, domain.SeedResourceCatalog(), "")
}

func lifecycleResource(resource resourceLifecycle) domain.Resource {
	if fake, ok := resource.(*fakeResourceLifecycle); ok {
		return fake.resource
	}
	return domain.Resource{}
}

func hasAction(response InteractionResponse, id string) bool {
	actions, ok := response.Pending.(ActionRequest)
	if !ok {
		return false
	}
	for _, action := range actions.Actions {
		if action.ID == id {
			return true
		}
	}
	return false
}
