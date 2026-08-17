package tui

import (
	"context"
	"errors"
	"github.com/GARFEX33/garfex-costos-unitarios/internal/domain"
	"testing"
)

type pageSearcher struct {
	fakeResourceSearcher
	pages []domain.ResourcePage
	errs  []error
	calls int
	got   domain.SearchCriteria
}
type legacyOnlySearcher struct{}

func (s *legacyOnlySearcher) Search(context.Context, domain.SearchCriteria) ([]domain.Resource, error) {
	return []domain.Resource{{ID: 1, ClassCode: "MATERIAL", IdentityKey: "legacy"}}, nil
}
func (s *pageSearcher) SearchPage(_ context.Context, criteria domain.SearchCriteria) (domain.ResourcePage, error) {
	s.got, s.calls = criteria, s.calls+1
	if s.calls <= len(s.errs) && s.errs[s.calls-1] != nil {
		return domain.ResourcePage{}, s.errs[s.calls-1]
	}
	return s.pages[s.calls-1], nil
}
func TestResourcesWorkspacePaginationFiltersBoundariesAndFailures(t *testing.T) {
	first := domain.ResourcePage{Criteria: domain.SearchCriteria{Text: "cable", ClassCode: "MATERIAL", LifecycleScope: domain.LifecycleScopeInactive, Limit: 2}, Resources: []domain.Resource{{ID: 1}}, HasNext: true}
	second := domain.ResourcePage{Criteria: domain.SearchCriteria{Text: "cable", ClassCode: "MATERIAL", LifecycleScope: domain.LifecycleScopeInactive, Limit: 2, Offset: 2}, Resources: []domain.Resource{{ID: 2}}, HasPrevious: true, HasNext: true}
	s := &pageSearcher{pages: []domain.ResourcePage{first, second}, errs: []error{nil, nil, errors.New("database unavailable")}}
	a := newDispatchAdapter(s, &fakeResourceGetter{}, &fakeResourceDescriber{}, &fakeResourceCreator{}, &fakeResourceUpdater{}, &fakeResourceDeleter{}, "MATERIAL")
	if response, err := a.Respond(context.Background(), InteractionInput{Kind: InputText, Value: "cable"}); err != nil || response.Pending == nil {
		t.Fatalf("initial page = %#v, %v", response, err)
	}
	_, _ = a.Respond(context.Background(), InteractionInput{Kind: InputAction, ActionID: previousPageActionID})
	if s.calls != 1 {
		t.Fatalf("first-page previous issued %d queries, want 1", s.calls)
	}
	if _, err := a.Respond(context.Background(), InteractionInput{Kind: InputAction, ActionID: nextPageActionID}); err != nil || s.got.Offset != 2 || s.got.Text != "cable" || s.got.ClassCode != "MATERIAL" || s.got.LifecycleScope != domain.LifecycleScopeInactive {
		t.Fatalf("next page criteria = %+v, error = %v", s.got, err)
	}
	failure, _ := a.Respond(context.Background(), InteractionInput{Kind: InputAction, ActionID: nextPageActionID})
	if len(failure.Messages) != 1 {
		t.Fatalf("failure response = %#v, want one safe message", failure)
	}
}
func TestResourcesWorkspaceRejectsLegacySearchWithoutForgedPage(t *testing.T) {
	legacy := &legacyOnlySearcher{}
	a := newDispatchAdapter(legacy, &fakeResourceGetter{}, &fakeResourceDescriber{}, &fakeResourceCreator{}, &fakeResourceUpdater{}, &fakeResourceDeleter{}, "MATERIAL")
	response, err := a.Respond(context.Background(), InteractionInput{Kind: InputText, Value: "legacy"})
	if err != nil || response.Pending != nil || len(response.Messages) != 1 {
		t.Fatalf("legacy search response = %#v, want one message and no pending page", response)
	}
	if _, ok := response.Messages[0].(ErrorMessage); !ok {
		t.Fatalf("legacy search message = %T, want ErrorMessage", response.Messages[0])
	}
	if a.lastPage.Criteria.Limit != 0 || a.lastPage.Resources != nil || a.lastQuery != "" {
		t.Fatalf("legacy search state = page=%+v query=%q, want no forged state", a.lastPage, a.lastQuery)
	}
}
func TestModelResourcePaginationPreservesOrClearsSelection(t *testing.T) {
	selected := domain.Resource{ID: 1, ClassCode: "MATERIAL", IdentityKey: "selected"}
	s := &pageSearcher{pages: []domain.ResourcePage{
		{Criteria: domain.SearchCriteria{Limit: 1}, Resources: []domain.Resource{selected}, HasNext: true},
		{Criteria: domain.SearchCriteria{Limit: 1, Offset: 1}, Resources: []domain.Resource{{ID: 2, ClassCode: "MATERIAL", IdentityKey: "other"}, selected}, HasPrevious: true, HasNext: true},
		{Criteria: domain.SearchCriteria{Limit: 1, Offset: 2}, Resources: []domain.Resource{{ID: 3, ClassCode: "MATERIAL", IdentityKey: "third"}}, HasPrevious: true},
	}}
	a := newDispatchAdapter(s, &fakeResourceGetter{}, &fakeResourceDescriber{}, &fakeResourceCreator{}, &fakeResourceUpdater{}, &fakeResourceDeleter{}, "")
	d := WorkspaceDescriptor{Slug: "resources", SearchOnEnter: true, Agent: a}
	m := NewWithAgent(Handlers{}, a)
	m.workspaces = map[string]*workspaceSlot{"resources": {descriptor: d, agent: a}}
	m.activeWorkspace, m.screen = "resources", screenWorkspace
	m.respond(InteractionInput{Kind: InputText})
	m, _ = update(t, m, key('n'))
	if m.choiceIndex != 1 {
		t.Fatalf("selection index after retaining resource = %d, want 1", m.choiceIndex)
	}
	m, _ = update(t, m, key('n'))
	if m.choiceIndex != 0 {
		t.Fatalf("selection index after clearing resource = %d, want 0", m.choiceIndex)
	}
}
