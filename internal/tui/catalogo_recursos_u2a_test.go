package tui

import (
	"context"
	"strings"
	"testing"

	"github.com/GARFEX33/garfex-costos-unitarios/internal/domain"
	"github.com/charmbracelet/x/ansi"
)

func TestU2aBusinessIdentitiesStayDistinct(t *testing.T) {
	tests := []struct {
		name string
		kind domain.CatalogKindCode
		recs []domain.CatalogRecord
		want []string
	}{
		{"familias repetidas", domain.KindFamily, []domain.CatalogRecord{familyIdentity(1, "MATERIAL", "CONDUCTORES", "Conductores", "Material"), familyIdentity(2, "EQUIPO", "CONDUCTORES", "Conductores", "Equipo")}, []string{"Conductores · Material", "Conductores · Equipo"}},
		{"tipos repetidos", domain.KindType, []domain.CatalogRecord{typeIdentity(3, "MATERIAL", "CONDUCTORES", "CABLE", "Cable", "Material", "Conductores"), typeIdentity(4, "MATERIAL", "TUBERIAS", "CABLE", "Cable", "Material", "Tuberías")}, []string{"Cable · Material › Conductores", "Cable · Material › Tuberías"}},
		{"unidades con nombre repetido", domain.KindUnit, []domain.CatalogRecord{unitIdentity(5, "M", "Metro", "m", "Longitud"), unitIdentity(6, "KG", "Metro", "kg", "Masa")}, []string{"Metro · m · Longitud", "Metro · kg · Masa"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			q := catalogQuestion(t, newCatalogAdminAdapter(&fakeCatalogLister{records: map[domain.CatalogKindCode][]domain.CatalogRecord{tt.kind: tt.recs}}, &fakeCatalogGetter{}, &fakeCatalogCreator{}, &fakeCatalogUpdater{}), tt.kind)
			if len(q.Options) != len(tt.want)+1 {
				t.Fatalf("options = %#v, want create plus %d identities", q.Options, len(tt.want))
			}
			for i, want := range tt.want {
				if q.Options[i+1].Label != want {
					t.Fatalf("identity[%d] = %q, want %q", i, q.Options[i+1].Label, want)
				}
			}
		})
	}
}
func TestU2aNarrowViewportRendersCompleteBusinessIdentity(t *testing.T) {
	a := newTestAdapter(&fakeResourceGetter{}, &fakeResourceCreator{}, &fakeResourceUpdater{}, "MATERIAL")
	r, err := a.startCreateEditor()
	if err != nil {
		t.Fatalf("startCreateEditor() error = %v", err)
	}
	m := New(Handlers{})
	m.screen, m.width, m.height, m.pending, m.interactionMode = screenWorkspace, 40, 20, r.Pending, interactionModeChoice
	m.resizeViewport()
	plain := ansi.Strip(m.View().Content)
	for _, want := range []string{"Conductores · Material", "↑↓ seleccionar", "Enter confirmar", "Esc cancelar"} {
		if !strings.Contains(plain, want) {
			t.Fatalf("narrow TUI output = %q, missing %q", plain, want)
		}
	}
}
func TestU2aListDetailEditRefreshesSearchAndReferences(t *testing.T) {
	store := &catalogRefreshStore{record: familyIdentity(7, "MATERIAL", "CONDUCTORES", "Conductores", "Material")}
	a := newCatalogAdminAdapter(store, store, &fakeCatalogCreator{}, store)
	ctx := context.Background()
	if _, err := a.startKindMenu(ctx, domain.KindFamily); err != nil {
		t.Fatal(err)
	}
	if _, err := a.Respond(ctx, InteractionInput{Kind: InputSelection, Key: catalogKindMenuKey, Value: "7"}); err != nil {
		t.Fatal(err)
	}
	if _, err := a.Respond(ctx, InteractionInput{Kind: InputAction, ActionID: catalogRecordEditActionID}); err != nil {
		t.Fatal(err)
	}
	response := advanceCatalog(t, a, "MATERIAL", "CONDUCTORES", "Conductores actualizados")
	if _, ok := response.Messages[0].(StructuredResult); !ok || store.record.Values["code"].Text != "CONDUCTORES" {
		t.Fatalf("edit result = %#v, stored record = %#v", response, store.record)
	}
	q := catalogQuestion(t, a, domain.KindFamily)
	want := "Conductores actualizados · Material"
	listWant := want
	if q.Options[1].Label != listWant {
		t.Fatalf("refreshed list label = %q", q.Options[1].Label)
	}
	if got := filterOptions(q.Options, "actualizados"); len(got) != 1 || got[0].Label != listWant {
		t.Fatalf("refreshed searchable options = %#v", got)
	}
	refs, err := a.refOptions(ctx, domain.FieldDescriptor{RefKind: domain.KindFamily}, nil)
	if err != nil || len(refs) != 1 || refs[0].Label != want {
		t.Fatalf("refreshed reference options = %#v, error = %v", refs, err)
	}
}

func TestCatalogAdminUnitDimensionGuidesMeasurementCategory(t *testing.T) {
	a := newCatalogAdminAdapter(&fakeCatalogLister{}, &fakeCatalogGetter{}, &fakeCatalogCreator{}, &fakeCatalogUpdater{})
	_, err := a.startCreateFlow(context.Background(), domain.KindUnit)
	if err != nil {
		t.Fatal(err)
	}
	q := testQuestion(t, advanceCatalog(t, a, "TEST_UNIT", "Unidad de prueba", "u"))
	for _, want := range []string{"categoría de medición", "Longitud", "Masa", "Tiempo", "Pieza"} {
		if !strings.Contains(strings.ToLower(q.Prompt), strings.ToLower(want)) {
			t.Fatalf("Dimensión prompt = %q, missing %q", q.Prompt, want)
		}
	}
}

func TestU2aExclusionsRemainAbsentFromRuntimeConfiguration(t *testing.T) {
	root := buildCatalogAdminActions(domain.NewCatalogRegistry().Kinds())
	for _, action := range flattenLeafActions([]assistantAction{root}) {
		if strings.Contains(action.label, "Dimensiones") || strings.Contains(action.label, "Navegación contextual") || strings.Contains(action.label, "Clase → Familia → Tipo") {
			t.Fatalf("deferred runtime entry exposed: %q", action.label)
		}
	}
}

type catalogRefreshStore struct{ record domain.CatalogRecord }

func (s *catalogRefreshStore) read() domain.CatalogRecord {
	r := s.record
	v := r.Values["class"]
	v.Ref.Label = "Material"
	r.Values["class"] = v
	return r
}
func (s *catalogRefreshStore) List(context.Context, domain.CatalogKindCode, domain.CatalogFilter) ([]domain.CatalogRecord, error) {
	return []domain.CatalogRecord{s.read()}, nil
}
func (s *catalogRefreshStore) Get(context.Context, domain.CatalogKindCode, int64) (domain.CatalogRecord, error) {
	return s.read(), nil
}
func (s *catalogRefreshStore) UpdateRevision(_ context.Context, kind domain.CatalogKindCode, rec domain.CatalogRecord, _ uint64) (domain.CatalogRecord, error) {
	rec.Kind, s.record = kind, rec
	return rec, nil
}

func advanceCatalog(t *testing.T, a *CatalogAdminAdapter, answers ...string) InteractionResponse {
	var r InteractionResponse
	for _, answer := range answers {
		var err error
		r, err = a.answerField(context.Background(), answer)
		if err != nil {
			t.Fatalf("answerField(%q) error = %v", answer, err)
		}
	}
	return r
}
func testQuestion(t *testing.T, r InteractionResponse) QuestionRequest {
	q, ok := r.Pending.(QuestionRequest)
	if !ok {
		t.Fatalf("Pending = %T, want QuestionRequest", r.Pending)
	}
	return q
}
func catalogQuestion(t *testing.T, a *CatalogAdminAdapter, kind domain.CatalogKindCode) QuestionRequest {
	t.Helper()
	r, err := a.startKindMenu(context.Background(), kind)
	if err != nil {
		t.Fatal(err)
	}
	return testQuestion(t, r)
}

func familyIdentity(id int64, classCode, code, name, label string) domain.CatalogRecord {
	r := familyRecord(id, classCode, code, name)
	v := r.Values["class"]
	v.Ref.Label = label
	r.Values["class"] = v
	return r
}
func typeIdentity(id int64, classCode, familyCode, code, name, classLabel, familyLabel string) domain.CatalogRecord {
	r := typeRecord(id, classCode, familyCode, code, name)
	c, f := r.Values["class"], r.Values["family"]
	c.Ref.Label, f.Ref.Label = classLabel, familyLabel
	r.Values["class"], r.Values["family"] = c, f
	return r
}
func unitIdentity(id int64, code, name, symbol, dimension string) domain.CatalogRecord {
	return domain.CatalogRecord{Kind: domain.KindUnit, ID: id, Active: true, Values: map[string]domain.CatalogValue{"code": {Text: code}, "name": {Text: name}, "symbol": {Text: symbol}, "dimension": {Text: dimension}}}
}
