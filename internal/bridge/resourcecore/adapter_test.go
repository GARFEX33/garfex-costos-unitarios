package resourcecore

import (
	"context"
	"errors"
	"testing"

	"github.com/GARFEX33/garfex-costos-unitarios/internal/core"
	"github.com/GARFEX33/garfex-costos-unitarios/internal/domain"
	public "github.com/GARFEX33/garfex-costos-unitarios/resourcecore"
)

type fakeCatalogReader struct {
	kinds []domain.CatalogKind
	list  func(ctx context.Context, kind domain.CatalogKindCode, filter domain.CatalogFilter) ([]domain.CatalogRecord, error)
	get   func(ctx context.Context, kind domain.CatalogKindCode, id int64) (domain.CatalogRecord, error)
}

func (f *fakeCatalogReader) Kinds() []domain.CatalogKind { return f.kinds }
func (f *fakeCatalogReader) List(ctx context.Context, kind domain.CatalogKindCode, filter domain.CatalogFilter) ([]domain.CatalogRecord, error) {
	return f.list(ctx, kind, filter)
}
func (f *fakeCatalogReader) Get(ctx context.Context, kind domain.CatalogKindCode, id int64) (domain.CatalogRecord, error) {
	return f.get(ctx, kind, id)
}

type fakeResourceReader struct {
	get           func(ctx context.Context, classCode, identityKey string) (domain.Resource, error)
	search        func(ctx context.Context, criteria domain.SearchCriteria) (domain.ResourcePage, error)
	describe      func(resource domain.Resource) string
	lastDescribed domain.Resource
}

func (f *fakeResourceReader) Get(ctx context.Context, classCode, identityKey string) (domain.Resource, error) {
	return f.get(ctx, classCode, identityKey)
}
func (f *fakeResourceReader) SearchPage(ctx context.Context, criteria domain.SearchCriteria) (domain.ResourcePage, error) {
	return f.search(ctx, criteria)
}
func (f *fakeResourceReader) Describe(resource domain.Resource) string {
	f.lastDescribed = resource
	return f.describe(resource)
}

func classKind() domain.CatalogKind {
	return domain.CatalogKind{
		Code: domain.KindClass, Singular: "Clase", Plural: "Clases",
		Fields: []domain.FieldDescriptor{
			{Name: "code", Label: "Código", Kind: domain.FieldCode, Required: true},
			{Name: "name", Label: "Nombre", Kind: domain.FieldText},
			{Name: "active", Label: "Activo", Kind: domain.FieldBool},
		},
		IdentityFields: []string{"code"},
	}
}

func familyKind() domain.CatalogKind {
	return domain.CatalogKind{
		Code: domain.KindFamily, Singular: "Familia", Plural: "Familias",
		ParentKind: domain.KindClass, ParentField: "class",
		Fields: []domain.FieldDescriptor{
			{Name: "class", Label: "Clase", Kind: domain.FieldRef, RefKind: domain.KindClass},
			{Name: "code", Label: "Código", Kind: domain.FieldCode},
		},
		IdentityFields: []string{"class", "code"},
	}
}

func newTestAdapter(catalog *fakeCatalogReader, resources *fakeResourceReader) *Adapter {
	if catalog == nil {
		catalog = &fakeCatalogReader{kinds: []domain.CatalogKind{classKind(), familyKind()}}
	}
	if resources == nil {
		resources = &fakeResourceReader{}
	}
	return NewAdapter(catalog, resources)
}

func TestAdapter_ActiveClasses(t *testing.T) {
	catalog := &fakeCatalogReader{
		kinds: []domain.CatalogKind{classKind()},
		list: func(ctx context.Context, kind domain.CatalogKindCode, filter domain.CatalogFilter) ([]domain.CatalogRecord, error) {
			if kind != domain.KindClass {
				t.Fatalf("expected KindClass, got %q", kind)
			}
			if filter.Status != domain.CatalogStatusActive {
				t.Fatalf("expected active status, got %v", filter.Status)
			}
			return []domain.CatalogRecord{{Kind: domain.KindClass, ID: 1, Active: true, Values: map[string]domain.CatalogValue{"code": {Text: "MAT"}}}}, nil
		},
	}
	adapter := newTestAdapter(catalog, nil)
	recs, err := adapter.ActiveClasses(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(recs) != 1 || recs[0].Kind != public.KindClass || recs[0].Values["code"].Text != "MAT" {
		t.Fatalf("unexpected records: %+v", recs)
	}
}

func TestAdapter_CatalogDescriptors(t *testing.T) {
	catalog := &fakeCatalogReader{kinds: []domain.CatalogKind{classKind()}}
	adapter := newTestAdapter(catalog, nil)
	descs, err := adapter.CatalogDescriptors(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(descs) != 1 || descs[0].Kind != public.KindClass || descs[0].Fields[0].Kind != public.ValueCode {
		t.Fatalf("unexpected descriptors: %+v", descs)
	}
}

func TestAdapter_ListCatalogPagination(t *testing.T) {
	catalog := &fakeCatalogReader{
		kinds: []domain.CatalogKind{classKind()},
		list: func(ctx context.Context, kind domain.CatalogKindCode, filter domain.CatalogFilter) ([]domain.CatalogRecord, error) {
			if filter.Limit != 3 {
				t.Fatalf("expected limit 3 (page+1), got %d", filter.Limit)
			}
			if filter.Offset != 2 {
				t.Fatalf("expected offset 2, got %d", filter.Offset)
			}
			return []domain.CatalogRecord{
				{Kind: domain.KindClass, ID: 1},
				{Kind: domain.KindClass, ID: 2},
				{Kind: domain.KindClass, ID: 3},
			}, nil
		},
	}
	adapter := newTestAdapter(catalog, nil)
	page, err := adapter.ListCatalog(context.Background(), public.CatalogQuery{Kind: public.KindClass, Limit: 2, Offset: 2})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(page.Records) != 2 {
		t.Fatalf("expected 2 records, got %d", len(page.Records))
	}
	if !page.HasPrevious || !page.HasNext {
		t.Fatalf("expected previous and next, got previous=%v next=%v", page.HasPrevious, page.HasNext)
	}
	if page.Query.Limit != 2 {
		t.Fatalf("expected normalized limit 2, got %d", page.Query.Limit)
	}
}

func TestAdapter_GetCatalog(t *testing.T) {
	catalog := &fakeCatalogReader{
		kinds: []domain.CatalogKind{classKind()},
		get: func(ctx context.Context, kind domain.CatalogKindCode, id int64) (domain.CatalogRecord, error) {
			if kind != domain.KindClass || id != 5 {
				t.Fatalf("unexpected get: %q %d", kind, id)
			}
			return domain.CatalogRecord{Kind: domain.KindClass, ID: 5, Active: true, Values: map[string]domain.CatalogValue{"code": {Text: "C"}, "active": {Bool: true}}}, nil
		},
	}
	adapter := newTestAdapter(catalog, nil)
	rec, err := adapter.GetCatalog(context.Background(), public.CatalogKey{Kind: public.KindClass, ID: 5})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rec.ID != 5 || rec.Values["code"].Kind != public.ValueCode || !rec.Values["active"].Bool {
		t.Fatalf("unexpected record: %+v", rec)
	}
}

func TestAdapter_GetResource(t *testing.T) {
	resources := &fakeResourceReader{
		get: func(ctx context.Context, classCode, identityKey string) (domain.Resource, error) {
			if classCode != "MAT" || identityKey != "v1|x" {
				t.Fatalf("unexpected get: %q %q", classCode, identityKey)
			}
			return domain.Resource{ID: 9, ClassCode: "MAT", FamilyCode: "F", TypeCode: "T", NaturalUnit: "U", IdentityKey: "v1|x", Active: true, Revision: 2}, nil
		},
	}
	adapter := newTestAdapter(nil, resources)
	res, err := adapter.GetResource(context.Background(), public.ResourceKey{ClassCode: "MAT", IdentityV1: "v1|x"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.ID != 9 || res.IdentityV1 != "v1|x" || res.Revision != 2 {
		t.Fatalf("unexpected resource: %+v", res)
	}
}

func TestAdapter_DescribeResourceDelegates(t *testing.T) {
	resources := &fakeResourceReader{
		get: func(ctx context.Context, classCode, identityKey string) (domain.Resource, error) {
			return domain.Resource{ID: 1, ClassCode: "MAT", IdentityKey: "v1|x"}, nil
		},
		describe: func(resource domain.Resource) string {
			return "canonical " + resource.IdentityKey
		},
	}
	adapter := newTestAdapter(nil, resources)
	desc, err := adapter.DescribeResource(context.Background(), public.ResourceKey{ClassCode: "MAT", IdentityV1: "v1|x"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if desc != "canonical v1|x" {
		t.Fatalf("unexpected description: %q", desc)
	}
	if resources.lastDescribed.IdentityKey != "v1|x" {
		t.Fatalf("describe did not receive the fetched resource")
	}
}

func TestAdapter_SearchResources(t *testing.T) {
	resources := &fakeResourceReader{
		search: func(ctx context.Context, criteria domain.SearchCriteria) (domain.ResourcePage, error) {
			if criteria.LifecycleScope != domain.LifecycleScopeInactive {
				t.Fatalf("expected inactive scope, got %v", criteria.LifecycleScope)
			}
			if criteria.ClassCode != "MAT" || criteria.FamilyCode != "F" || criteria.Text != "cable" {
				t.Fatalf("unexpected criteria: %+v", criteria)
			}
			return domain.ResourcePage{
				Criteria:    criteria,
				Resources:   []domain.Resource{{ID: 1, Active: false}},
				HasPrevious: true,
				HasNext:     true,
			}, nil
		},
	}
	adapter := newTestAdapter(nil, resources)
	page, err := adapter.SearchResources(context.Background(), public.ResourceQuery{
		Scope: public.ScopeInactive, ClassCode: "MAT", FamilyCode: "F", Text: "cable", Limit: 10, Offset: 5,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(page.Resources) != 1 || page.Resources[0].ID != 1 {
		t.Fatalf("unexpected resources: %+v", page)
	}
	if !page.HasPrevious || !page.HasNext {
		t.Fatalf("unexpected pagination: %+v", page)
	}
}

func TestAdapter_SearchResourcesAllScopeNotSupported(t *testing.T) {
	adapter := newTestAdapter(nil, nil)
	_, err := adapter.SearchResources(context.Background(), public.ResourceQuery{Scope: public.ScopeAll})
	if !public.IsCode(err, public.InvalidArgument) {
		t.Fatalf("expected INVALID_ARGUMENT for ALL scope, got %v", err)
	}
}

func TestAdapter_SearchResourcesTypeCodeNotSupported(t *testing.T) {
	adapter := newTestAdapter(nil, nil)
	_, err := adapter.SearchResources(context.Background(), public.ResourceQuery{TypeCode: "CABLE"})
	if !public.IsCode(err, public.InvalidArgument) {
		t.Fatalf("expected INVALID_ARGUMENT for unsupported TypeCode filter, got %v", err)
	}
}

func TestAdapter_CatalogDescriptorsIncludesRefScopedByAndEnumValues(t *testing.T) {
	kind := domain.CatalogKind{
		Code: domain.KindFamily, Singular: "Familia", Plural: "Familias",
		Fields: []domain.FieldDescriptor{
			{Name: "class", Label: "Clase", Kind: domain.FieldRef, RefKind: domain.KindClass, RefScopedBy: []string{"class"}},
			{Name: "valueType", Label: "Tipo de valor", Kind: domain.FieldEnum, EnumValues: []domain.EnumValue{{Value: "TEXT", Label: "Texto"}, {Value: "BOOL", Label: "Booleano"}}},
		},
	}
	catalog := &fakeCatalogReader{kinds: []domain.CatalogKind{kind}}
	adapter := newTestAdapter(catalog, nil)
	descs, err := adapter.CatalogDescriptors(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(descs) != 1 {
		t.Fatalf("expected 1 descriptor, got %d", len(descs))
	}
	fields := descs[0].Fields
	if len(fields) != 2 {
		t.Fatalf("expected 2 fields, got %d", len(fields))
	}
	if len(fields[0].RefScopedBy) != 1 || fields[0].RefScopedBy[0] != "class" {
		t.Fatalf("expected RefScopedBy [class], got %+v", fields[0].RefScopedBy)
	}
	if len(fields[1].EnumValues) != 2 || fields[1].EnumValues[0].Value != "TEXT" || fields[1].EnumValues[1].Label != "Booleano" {
		t.Fatalf("expected mapped EnumValues, got %+v", fields[1].EnumValues)
	}
}

func TestAdapter_CatalogRecordIncludesRules(t *testing.T) {
	catalog := &fakeCatalogReader{
		kinds: []domain.CatalogKind{classKind()},
		get: func(ctx context.Context, kind domain.CatalogKindCode, id int64) (domain.CatalogRecord, error) {
			return domain.CatalogRecord{
				Kind: domain.KindClass, ID: 1, Active: true,
				Values: map[string]domain.CatalogValue{"code": {Text: "C"}},
				Rules: []domain.CatalogRuleRecord{
					{
						When:                 domain.AttributeCondition{AttributeCode: "insulation", Equals: "DESNUDO"},
						Mode:                 domain.ModeForbidden,
						IdentityParticipates: true,
						NotApplicable:        true,
						Active:               true,
					},
				},
			}, nil
		},
	}
	adapter := newTestAdapter(catalog, nil)
	rec, err := adapter.GetCatalog(context.Background(), public.CatalogKey{Kind: public.KindClass, ID: 1})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(rec.Rules) != 1 {
		t.Fatalf("expected 1 rule, got %d: %+v", len(rec.Rules), rec.Rules)
	}
	rule := rec.Rules[0]
	if rule.AttributeCode != "insulation" || rule.Equals.Text != "DESNUDO" {
		t.Fatalf("unexpected rule condition: %+v", rule)
	}
	if rule.Mode != string(domain.ModeForbidden) || !rule.IdentityParticipates || !rule.NotApplicable || !rule.Active {
		t.Fatalf("unexpected rule flags: %+v", rule)
	}
}

func TestAdapter_MapsErrorsThroughNeutralBoundary(t *testing.T) {
	tests := []struct {
		name     string
		setup    func(*fakeCatalogReader, *fakeResourceReader)
		call     func(*Adapter, context.Context) error
		wantCode public.ErrorCode
	}{
		{
			name: "not found",
			setup: func(c *fakeCatalogReader, _ *fakeResourceReader) {
				c.kinds = []domain.CatalogKind{classKind()}
				c.get = func(ctx context.Context, kind domain.CatalogKindCode, id int64) (domain.CatalogRecord, error) {
					return domain.CatalogRecord{}, domain.ErrCatalogRecordNotFound
				}
			},
			call: func(a *Adapter, ctx context.Context) error {
				_, err := a.GetCatalog(ctx, public.CatalogKey{Kind: public.KindClass, ID: 1})
				return err
			},
			wantCode: public.NotFound,
		},
		{
			name: "integrity",
			setup: func(c *fakeCatalogReader, _ *fakeResourceReader) {
				c.kinds = []domain.CatalogKind{classKind()}
				c.list = func(ctx context.Context, kind domain.CatalogKindCode, filter domain.CatalogFilter) ([]domain.CatalogRecord, error) {
					return nil, domain.ErrResourceIntegrity
				}
			},
			call: func(a *Adapter, ctx context.Context) error {
				_, err := a.ActiveClasses(ctx)
				return err
			},
			wantCode: public.Integrity,
		},
		{
			name: "invalid catalog",
			setup: func(c *fakeCatalogReader, _ *fakeResourceReader) {
				c.kinds = []domain.CatalogKind{classKind()}
				c.list = func(ctx context.Context, kind domain.CatalogKindCode, filter domain.CatalogFilter) ([]domain.CatalogRecord, error) {
					return nil, domain.ErrInvalidCatalog
				}
			},
			call: func(a *Adapter, ctx context.Context) error {
				_, err := a.ListCatalog(ctx, public.CatalogQuery{Kind: public.KindClass})
				return err
			},
			wantCode: public.InvalidCatalog,
		},
		{
			name: "unavailable",
			setup: func(_ *fakeCatalogReader, r *fakeResourceReader) {
				r.get = func(ctx context.Context, classCode, identityKey string) (domain.Resource, error) {
					return domain.Resource{}, core.ErrUnavailable
				}
			},
			call: func(a *Adapter, ctx context.Context) error {
				_, err := a.GetResource(ctx, public.ResourceKey{ClassCode: "MAT", IdentityV1: "v1|x"})
				return err
			},
			wantCode: public.Unavailable,
		},
		{
			name: "internal",
			setup: func(_ *fakeCatalogReader, r *fakeResourceReader) {
				r.search = func(ctx context.Context, criteria domain.SearchCriteria) (domain.ResourcePage, error) {
					return domain.ResourcePage{}, errors.New("unexpected failure")
				}
			},
			call: func(a *Adapter, ctx context.Context) error {
				_, err := a.SearchResources(ctx, public.ResourceQuery{})
				return err
			},
			wantCode: public.Internal,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			catalog := &fakeCatalogReader{kinds: []domain.CatalogKind{classKind()}}
			resources := &fakeResourceReader{}
			tt.setup(catalog, resources)
			adapter := newTestAdapter(catalog, resources)
			err := tt.call(adapter, context.Background())
			if !public.IsCode(err, tt.wantCode) {
				t.Fatalf("expected %v, got %v", tt.wantCode, err)
			}
			var publicErr public.Error
			if !errors.As(err, &publicErr) {
				t.Fatalf("expected public.Error, got %T", err)
			}
			if errors.Unwrap(publicErr) != nil {
				t.Fatalf("public error must not unwrap")
			}
		})
	}
}

func TestAdapter_DeepCopyIncomingQuery(t *testing.T) {
	catalog := &fakeCatalogReader{
		kinds: []domain.CatalogKind{classKind()},
		list: func(ctx context.Context, kind domain.CatalogKindCode, filter domain.CatalogFilter) ([]domain.CatalogRecord, error) {
			return []domain.CatalogRecord{{Kind: domain.KindClass, ID: 1, Values: map[string]domain.CatalogValue{"code": {Text: "C"}}}}, nil
		},
	}
	adapter := newTestAdapter(catalog, nil)
	q := public.CatalogQuery{Kind: public.KindClass}
	page, err := adapter.ListCatalog(context.Background(), q)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	page.Records[0].Values["code"] = public.Value{Kind: public.ValueCode, Text: "Z"}
	if q.Kind != public.KindClass {
		t.Fatalf("input query mutated")
	}
}

func TestAdapter_ResourceValueProjection(t *testing.T) {
	i := int64(42)
	b := true
	resources := &fakeResourceReader{
		get: func(ctx context.Context, classCode, identityKey string) (domain.Resource, error) {
			return domain.Resource{
				ID: 1, ClassCode: "MAT", IdentityKey: "v1|x",
				Attributes: []domain.ResourceAttributeValue{
					{AttributeCode: "opt", Type: domain.ValueTypeControlledOption, OptionCode: "RED"},
					{AttributeCode: "int", Type: domain.ValueTypeInteger, Integer: &i},
					{AttributeCode: "bool", Type: domain.ValueTypeBoolean, Boolean: &b},
					{AttributeCode: "text", Type: domain.ValueTypeControlledText, Text: domain.NotApplicableText},
				},
			}, nil
		},
	}
	adapter := newTestAdapter(nil, resources)
	res, err := adapter.GetResource(context.Background(), public.ResourceKey{ClassCode: "MAT", IdentityV1: "v1|x"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	byCode := map[string]public.AttributeValue{}
	for _, av := range res.Attributes {
		byCode[av.Code] = av
	}
	if byCode["opt"].Value.Kind != public.ValueControlledOption || byCode["opt"].Value.Text != "RED" {
		t.Fatalf("option projection mismatch: %+v", byCode["opt"])
	}
	if byCode["int"].Value.Kind != public.ValueInteger || byCode["int"].Value.Text != "42" {
		t.Fatalf("integer projection mismatch: %+v", byCode["int"])
	}
	if byCode["bool"].Value.Kind != public.ValueBool || !byCode["bool"].Value.Bool {
		t.Fatalf("boolean projection mismatch: %+v", byCode["bool"])
	}
	if byCode["text"].Value.Kind != public.ValueNotApplicable {
		t.Fatalf("not-applicable projection mismatch: %+v", byCode["text"])
	}
}
