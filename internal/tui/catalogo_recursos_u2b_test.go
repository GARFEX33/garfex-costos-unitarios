package tui

import (
	"context"
	"strings"
	"testing"

	"github.com/GARFEX33/garfex-costos-unitarios/internal/domain"
)

func TestU2bConditionalAuthoringIsAbsentFromSharedFieldPaths(t *testing.T) {
	registry := domain.NewCatalogRegistry()
	definition, ok := registry.Kind(domain.KindAttributeBinding)
	if !ok {
		t.Fatal("AttributeBinding descriptor is not registered")
	}
	var mode domain.FieldDescriptor
	for _, field := range definition.Fields {
		if field.Name == "mode" {
			mode = field
			break
		}
	}
	if len(mode.EnumValues) == 0 {
		t.Fatal("mode descriptor has no authoring options")
	}
	adapter := newCatalogAdminAdapter(&fakeCatalogLister{}, &fakeCatalogGetter{}, &fakeCatalogCreator{}, &fakeCatalogUpdater{})
	adapter.editor = &catalogEditorState{def: definition, values: map[string]domain.CatalogValue{}}
	response, err := adapter.fieldQuestion(context.Background(), mode)
	if err != nil {
		t.Fatalf("fieldQuestion(mode) error = %v", err)
	}
	question, ok := response.Pending.(QuestionRequest)
	if !ok {
		t.Fatalf("Pending = %T, want QuestionRequest", response.Pending)
	}
	for _, option := range question.Options {
		if option.Value == "CONDITIONAL" || strings.EqualFold(option.Label, "Condicional") {
			t.Fatalf("conditional authoring option = %#v, must be hidden", option)
		}
	}
}

func TestU2bPersistedConditionalRecordIsVisibleButReadOnly(t *testing.T) {
	record := conditionalBindingRecord(41)
	lister := &fakeCatalogLister{records: map[domain.CatalogKindCode][]domain.CatalogRecord{
		domain.KindAttributeBinding: {record},
	}}
	getter := &fakeCatalogGetter{records: map[int64]domain.CatalogRecord{record.ID: record}}
	adapter := newCatalogAdminAdapter(lister, getter, &fakeCatalogCreator{}, &fakeCatalogUpdater{})
	ctx := context.Background()

	response, err := adapter.startKindMenu(ctx, domain.KindAttributeBinding)
	if err != nil {
		t.Fatalf("startKindMenu() error = %v", err)
	}
	question := testQuestion(t, response)
	if len(question.Options) != 2 || question.Options[1].Value != "41" {
		t.Fatalf("conditional options = %#v, want the persisted record visible", question.Options)
	}
	response, err = adapter.Respond(ctx, InteractionInput{Kind: InputSelection, Key: catalogKindMenuKey, Value: "41"})
	if err != nil {
		t.Fatalf("Respond(select conditional) error = %v", err)
	}
	actions, ok := response.Pending.(ActionRequest)
	if !ok {
		t.Fatalf("Pending = %T, want ActionRequest", response.Pending)
	}
	for _, action := range actions.Actions {
		if action.ID == catalogRecordEditActionID {
			t.Fatalf("conditional actions = %#v, must hide Editar", actions.Actions)
		}
	}
	response, err = adapter.Respond(ctx, InteractionInput{Kind: InputAction, ActionID: catalogRecordEditActionID})
	if err != nil {
		t.Fatalf("Respond(injected edit) error = %v", err)
	}
	if adapter.editor != nil {
		t.Fatal("conditional record accepted an injected edit action")
	}
	if len(response.Messages) == 0 {
		t.Fatal("injected edit should return an actionable read-only message")
	}
	message, ok := response.Messages[0].(ErrorMessage)
	if !ok || !strings.Contains(strings.ToLower(message.Text), "solo lectura") {
		t.Fatalf("read-only response = %#v, want Spanish solo lectura guidance", response.Messages)
	}
}

func TestU2bPersistedConditionalFixtureKeepsLoaderValidationAndEffectiveRules(t *testing.T) {
	catalog := domain.SeedResourceCatalog()
	if err := catalog.Validate(); err != nil {
		t.Fatalf("loaded conditional fixture Validate() = %v", err)
	}
	for _, attribute := range catalog.Attributes {
		if attribute.Definition.Code != "color" {
			continue
		}
		if attribute.Mode != domain.ModeConditional || len(attribute.Rules) != 1 {
			t.Fatalf("conditional fixture = %#v, want one persisted rule", attribute)
		}
		mode, _, notApplicable := attribute.Effective([]domain.ResourceAttributeValue{domain.OptionValue("insulation", "DESNUDO")})
		if mode != domain.ModeForbidden || !notApplicable {
			t.Fatalf("Effective(DESNUDO) = %v, %v, want forbidden/not-applicable", mode, notApplicable)
		}
		mode, _, notApplicable = attribute.Effective([]domain.ResourceAttributeValue{domain.OptionValue("insulation", "THW")})
		if mode != domain.ModeRequired || notApplicable {
			t.Fatalf("Effective(THW) = %v, %v, want required/applicable", mode, notApplicable)
		}
		return
	}
	t.Fatal("loaded conditional color fixture is missing")
}

func TestU2bCompleteCatalogCapabilitiesReachKeyboardAndSearchPalette(t *testing.T) {
	registry := domain.NewCatalogRegistry()
	root := buildCatalogAdminActions(registry.Kinds())
	leaves := flattenLeafActions([]assistantAction{root})
	for _, want := range []string{
		"Crear estructura de recursos", "Características", "Unidades", "Políticas de Unidad",
		"Aplicabilidades", "Identidad", "Campos de Presentación",
	} {
		found := false
		for _, leaf := range leaves {
			if leaf.label == want {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("complete capability %q is not reachable as a palette leaf", want)
		}
	}
	for _, query := range []string{"estructura", "características", "unidades", "aplicabilidad", "identidad", "presentación"} {
		if matches := filterOptions(actionOptions(leaves), query); len(matches) == 0 {
			t.Fatalf("palette search %q returned no complete capability", query)
		}
	}

	agent := &fakeCatalogAgent{}
	descriptor := buildCatalogAdminWorkspace(registry.Kinds(), agent)
	m := NewWithWorkspaces(Handlers{}, NewFakeAgent(), []WorkspaceDescriptor{descriptor})
	if !m.enterWorkspace(configuracionSlug) {
		t.Fatal("configuration workspace is not registered")
	}
	m, _ = update(t, m, key('/'))
	m, _ = update(t, m, enter())
	m, _ = update(t, m, key('j'))
	m, _ = update(t, m, enter())
	m, _ = update(t, m, key('j'))
	m, _ = update(t, m, enter())
	if agent.last.ActionID != catalogOpenActionID(domain.KindFamily) {
		t.Fatalf("keyboard palette action = %#v, mode=%v palette=%#v index=%d, want Familias", agent.last, m.interactionMode, m.paletteActions, m.paletteIndex)
	}

	m, _ = update(t, m, key('/'))
	m, _ = update(t, m, enter())
	for _, character := range "unid" {
		m, _ = update(t, m, key(character))
	}
	m, _ = update(t, m, enter())
	if agent.last.ActionID != catalogOpenActionID(domain.KindUnit) {
		t.Fatalf("search palette action = %#v, want Unidades", agent.last)
	}
}

func TestU2bCompleteCatalogCapabilitiesDispatchEveryKeyboardAndSearchPath(t *testing.T) {
	registry := domain.NewCatalogRegistry()
	tests := []struct {
		name, id, query string
	}{
		{"structure creation", catalogWizardActionID, "crear estructura"},
		{"characteristics", catalogOpenActionID(domain.KindAttributeDefinition), "características"},
		{"units", catalogOpenActionID(domain.KindUnit), "unidades"},
		{"unit policies", catalogOpenActionID(domain.KindUnitPolicy), "política"},
		{"applicability", catalogOpenActionID(domain.KindAttributeBinding), "aplicabilidad"},
		{"identity", catalogIdentityActionID, "identidad"},
		{"presentation", catalogOpenActionID(domain.KindPresentationField), "presentación"},
	}
	for _, tt := range tests {
		t.Run(tt.name+" keyboard", func(t *testing.T) {
			agent := &fakeCatalogAgent{}
			m := newU2bCatalogPaletteModel(registry, agent)
			path := paletteActionPath(buildCatalogAdminActions(registry.Kinds()).children, tt.id)
			if path == nil {
				t.Fatalf("capability %q is not in the palette tree", tt.id)
			}
			m, _ = update(t, m, key('/'))
			m, _ = update(t, m, enter())
			for _, index := range path {
				for range index {
					m, _ = update(t, m, key('j'))
				}
				m, _ = update(t, m, enter())
			}
			if agent.last.ActionID != tt.id {
				t.Fatalf("keyboard dispatch = %#v, want %q", agent.last, tt.id)
			}
		})
		t.Run(tt.name+" search", func(t *testing.T) {
			agent := &fakeCatalogAgent{}
			m := newU2bCatalogPaletteModel(registry, agent)
			m, _ = update(t, m, key('/'))
			m, _ = update(t, m, enter())
			for _, character := range tt.query {
				m, _ = update(t, m, key(character))
			}
			_, _ = update(t, m, enter())
			if agent.last.ActionID != tt.id {
				t.Fatalf("search dispatch = %#v, want %q", agent.last, tt.id)
			}
		})
	}
}

func TestU2bExclusionsStayAbsentThroughRuntimePalette(t *testing.T) {
	registry := domain.NewCatalogRegistry()
	definition, ok := registry.Kind(domain.KindAttributeBinding)
	if !ok {
		t.Fatal("AttributeBinding descriptor is not registered")
	}
	for _, field := range definition.Fields {
		if field.Name != "mode" {
			continue
		}
		for _, option := range field.EnumValues {
			if option.Value == "CONDITIONAL" {
				t.Fatalf("incomplete CONDITIONAL authoring remains in descriptor: %#v", option)
			}
		}
	}
	root := buildCatalogAdminActions(registry.Kinds())
	for _, leaf := range flattenLeafActions([]assistantAction{root}) {
		for _, forbidden := range []string{"Dimensiones", "Navegación contextual", "Clase → Familia → Tipo"} {
			if leaf.label == forbidden {
				t.Fatalf("deferred capability %q is exposed", forbidden)
			}
		}
	}
	for _, query := range []string{"condicional", "dimensiones", "navegación contextual", "crud", "migración"} {
		agent := &fakeCatalogAgent{}
		m := newU2bCatalogPaletteModel(registry, agent)
		m, _ = update(t, m, key('/'))
		m, _ = update(t, m, enter())
		for _, character := range query {
			m, _ = update(t, m, key(character))
		}
		if matches := filterOptions(actionOptions(m.paletteActions), m.paletteQuery); len(matches) != 0 {
			t.Fatalf("runtime palette query %q exposed excluded entries: %#v", query, matches)
		}
	}
}

func newU2bCatalogPaletteModel(registry domain.CatalogRegistry, agent InteractionAgent) Model {
	m := NewWithWorkspaces(Handlers{}, NewFakeAgent(), []WorkspaceDescriptor{buildCatalogAdminWorkspace(registry.Kinds(), agent)})
	m.enterWorkspace(configuracionSlug)
	return m
}

func paletteActionPath(actions []assistantAction, target string) []int {
	for index, action := range actions {
		if action.id == target {
			return []int{index}
		}
		if path := paletteActionPath(action.children, target); path != nil {
			return append([]int{index}, path...)
		}
	}
	return nil
}

func conditionalBindingRecord(id int64) domain.CatalogRecord {
	return domain.CatalogRecord{
		Kind: domain.KindAttributeBinding, ID: id, Active: true,
		Values: map[string]domain.CatalogValue{
			"class":          {Ref: domain.CatalogRef{Kind: domain.KindClass, Code: "MATERIAL"}},
			"family":         {Ref: domain.CatalogRef{Kind: domain.KindFamily, Code: "CONDUCTORES"}},
			"type":           {Ref: domain.CatalogRef{Kind: domain.KindType, Code: "CABLE"}},
			"characteristic": {Ref: domain.CatalogRef{Kind: domain.KindAttributeDefinition, Code: "COLOR"}},
			"mode":           {Text: "CONDITIONAL"},
		},
	}
}
