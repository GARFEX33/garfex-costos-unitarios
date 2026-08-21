package resourcecore

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/GARFEX33/garfex-costos-unitarios/internal/app/catalogo"
	"github.com/GARFEX33/garfex-costos-unitarios/internal/core"
	"github.com/GARFEX33/garfex-costos-unitarios/internal/domain"
	public "github.com/GARFEX33/garfex-costos-unitarios/resourcecore"
)

// bridgePgxLikeError simulates a PostgreSQL driver error crossing the
// bridge, proving no technical detail leaks through the public boundary.
type bridgePgxLikeError struct {
	SQLState, ConstraintName, TableName, ColumnName, ServerMessage string
}

func (e bridgePgxLikeError) Error() string { return e.ServerMessage }

type fakeCatalogReader struct {
	kinds  []domain.CatalogKind
	list   func(ctx context.Context, kind domain.CatalogKindCode, filter domain.CatalogFilter) ([]domain.CatalogRecord, error)
	get    func(ctx context.Context, kind domain.CatalogKindCode, id int64) (domain.CatalogRecord, error)
	create func(ctx context.Context, kind domain.CatalogKindCode, rec domain.CatalogRecord) (domain.CatalogRecord, error)
}

func (f *fakeCatalogReader) Kinds() []domain.CatalogKind { return f.kinds }
func (f *fakeCatalogReader) List(ctx context.Context, kind domain.CatalogKindCode, filter domain.CatalogFilter) ([]domain.CatalogRecord, error) {
	return f.list(ctx, kind, filter)
}
func (f *fakeCatalogReader) Get(ctx context.Context, kind domain.CatalogKindCode, id int64) (domain.CatalogRecord, error) {
	return f.get(ctx, kind, id)
}
func (f *fakeCatalogReader) Create(ctx context.Context, kind domain.CatalogKindCode, rec domain.CatalogRecord) (domain.CatalogRecord, error) {
	return f.create(ctx, kind, rec)
}

type fakeResourceReader struct {
	get           func(ctx context.Context, classCode, identityKey string) (domain.Resource, error)
	search        func(ctx context.Context, criteria domain.SearchCriteria) (domain.ResourcePage, error)
	describe      func(resource domain.Resource) string
	create        func(ctx context.Context, command domain.CreateCommand) (domain.Resource, error)
	lastDescribed domain.Resource
}

func (f *fakeResourceReader) Create(ctx context.Context, command domain.CreateCommand) (domain.Resource, error) {
	return f.create(ctx, command)
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

func allFieldKindsKind() domain.CatalogKind {
	return domain.CatalogKind{
		Code: domain.KindFamily, Singular: "Familia", Plural: "Familias",
		Fields: []domain.FieldDescriptor{
			{Name: "text", Label: "Texto", Kind: domain.FieldText},
			{Name: "code", Label: "Código", Kind: domain.FieldCode},
			{Name: "flag", Label: "Bandera", Kind: domain.FieldBool},
			{Name: "count", Label: "Cantidad", Kind: domain.FieldInt},
			{Name: "tags", Label: "Etiquetas", Kind: domain.FieldStringList},
			{Name: "class", Label: "Clase", Kind: domain.FieldRef, RefKind: domain.KindClass},
			{Name: "kind", Label: "Tipo de valor", Kind: domain.FieldEnum},
		},
	}
}

func TestWriteBridge_CatalogPort_FakeAdapted(t *testing.T) {
	catalog := &fakeCatalogReader{
		kinds: []domain.CatalogKind{classKind()},
		create: func(ctx context.Context, kind domain.CatalogKindCode, rec domain.CatalogRecord) (domain.CatalogRecord, error) {
			rec.ID, rec.Revision = 1, 1
			return rec, nil
		},
	}
	adapter := newTestAdapter(catalog, nil)
	rec, err := adapter.CreateCatalog(context.Background(), public.CatalogWriteRequest{
		Actor: "PI", Kind: public.KindClass,
		Values: map[string]public.Value{"code": {Kind: public.ValueCode, Text: "MAT"}},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rec.ID != 1 || rec.Revision != 1 {
		t.Fatalf("unexpected record: %+v", rec)
	}
}

func TestWriteBridge_CatalogCreate_FieldCompleteness(t *testing.T) {
	var capturedKind domain.CatalogKindCode
	var captured domain.CatalogRecord
	catalog := &fakeCatalogReader{
		kinds: []domain.CatalogKind{classKind()},
		create: func(ctx context.Context, kind domain.CatalogKindCode, rec domain.CatalogRecord) (domain.CatalogRecord, error) {
			capturedKind, captured = kind, rec
			rec.Kind, rec.ID, rec.Revision = kind, 3, 1
			return rec, nil
		},
	}
	adapter := newTestAdapter(catalog, nil)
	_, err := adapter.CreateCatalog(context.Background(), public.CatalogWriteRequest{
		Actor:  "PI",
		Kind:   public.KindClass,
		Active: true,
		Values: map[string]public.Value{"code": {Kind: public.ValueCode, Text: "MAT"}},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if capturedKind != domain.KindClass {
		t.Fatalf("Kind not mapped to the Create kind argument: got %q", capturedKind)
	}
	if !captured.Active {
		t.Fatalf("Active not mapped verbatim: %+v", captured)
	}
	if captured.Values["code"].Text != "MAT" {
		t.Fatalf("Values not mapped: %+v", captured)
	}
	if captured.Rules != nil {
		t.Fatalf("expected nil Rules when request omits the aggregate, got %+v", captured.Rules)
	}
}

func TestCatalogCreate_ValueMapping_InverseRoundTrip(t *testing.T) {
	tests := []struct {
		field   string
		value   public.Value
		wantErr bool
	}{
		{"text", public.Value{Kind: public.ValueText, Text: "hello"}, false},
		{"code", public.Value{Kind: public.ValueCode, Text: "COD"}, false},
		{"flag", public.Value{Kind: public.ValueBool, Bool: true}, false},
		{"count", public.Value{Kind: public.ValueInteger, Text: "42"}, false},
		{"tags", public.Value{Kind: public.ValueStringList, Strings: []string{"a", "b"}}, false},
		{"class", public.Value{Kind: public.ValueReference, Reference: &public.Reference{Kind: public.KindClass, Code: "MAT"}}, false},
		{"kind", public.Value{Kind: public.ValueEnum, Text: "TEXT"}, false},
	}
	for _, tt := range tests {
		t.Run(tt.field, func(t *testing.T) {
			var captured domain.CatalogRecord
			catalog := &fakeCatalogReader{
				kinds: []domain.CatalogKind{allFieldKindsKind()},
				create: func(ctx context.Context, kind domain.CatalogKindCode, rec domain.CatalogRecord) (domain.CatalogRecord, error) {
					captured = rec
					return rec, nil
				},
			}
			adapter := newTestAdapter(catalog, nil)
			_, err := adapter.CreateCatalog(context.Background(), public.CatalogWriteRequest{
				Actor:  "PI",
				Kind:   public.KindFamily,
				Values: map[string]public.Value{tt.field: tt.value},
			})
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			back := adapter.mapCatalogValue(domain.KindFamily, tt.field, captured.Values[tt.field])
			if back.Kind != tt.value.Kind {
				t.Fatalf("round trip kind mismatch for %s: got %+v, want %+v", tt.field, back, tt.value)
			}
		})
	}
}

func TestCatalogCreate_RuleMapping_AllSixFieldsMapped(t *testing.T) {
	var captured domain.CatalogRecord
	catalog := &fakeCatalogReader{
		kinds: []domain.CatalogKind{classKind()},
		create: func(ctx context.Context, kind domain.CatalogKindCode, rec domain.CatalogRecord) (domain.CatalogRecord, error) {
			captured = rec
			return rec, nil
		},
	}
	adapter := newTestAdapter(catalog, nil)
	_, err := adapter.CreateCatalog(context.Background(), public.CatalogWriteRequest{
		Actor:  "PI",
		Kind:   public.KindClass,
		Values: map[string]public.Value{"code": {Kind: public.ValueCode, Text: "MAT"}},
		Rules: []public.ApplicabilityRule{
			{AttributeCode: "insulation", Equals: public.Value{Kind: public.ValueText, Text: "DESNUDO"}, Mode: "FORBIDDEN", IdentityParticipates: true, NotApplicable: true, Active: true},
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(captured.Rules) != 1 {
		t.Fatalf("expected 1 rule, got %d", len(captured.Rules))
	}
	r := captured.Rules[0]
	if r.When.AttributeCode != "insulation" || r.When.Equals != "DESNUDO" || r.Mode != domain.ModeForbidden || !r.IdentityParticipates || !r.NotApplicable || !r.Active {
		t.Fatalf("unexpected mapped rule: %+v", r)
	}
}

func TestCatalogCreate_NineCategoryTable_DistinctAndNoLeakage(t *testing.T) {
	pgxErr := bridgePgxLikeError{"23505", "resource_classes_code_key", "resource_classes", "code", "duplicate key value violates unique constraint"}
	tests := []struct {
		name     string
		err      error
		wantCode public.ErrorCode
	}{
		{"invalid argument", catalogo.ErrInvalidArgument, public.InvalidArgument},
		{"not found", domain.ErrCatalogRecordNotFound, public.NotFound},
		{"duplicate", domain.ErrCatalogDuplicate, public.Duplicate},
		{"invalid reference", domain.ErrCatalogReference, public.InvalidReference},
		{"validation", domain.ErrResourceValidation, public.Validation},
		{"integrity", domain.ErrResourceIntegrity, public.Integrity},
		{"invalid catalog", domain.WrapInvalidCatalog(domain.ErrResourceValidation), public.InvalidCatalog},
		{"unavailable", catalogo.ErrCatalogWriterUnavailable, public.Unavailable},
		{"internal", pgxErr, public.Internal},
	}
	seen := map[public.ErrorCode]struct{}{}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			catalog := &fakeCatalogReader{
				kinds: []domain.CatalogKind{classKind()},
				create: func(ctx context.Context, kind domain.CatalogKindCode, rec domain.CatalogRecord) (domain.CatalogRecord, error) {
					return domain.CatalogRecord{}, tt.err
				},
			}
			adapter := newTestAdapter(catalog, nil)
			_, err := adapter.CreateCatalog(context.Background(), public.CatalogWriteRequest{
				Actor:  "PI",
				Kind:   public.KindClass,
				Values: map[string]public.Value{"code": {Kind: public.ValueCode, Text: "MAT"}},
			})
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
			for _, leak := range []string{pgxErr.SQLState, pgxErr.ConstraintName, pgxErr.TableName, pgxErr.ColumnName, pgxErr.ServerMessage} {
				if leak != "" && strings.Contains(fmt.Sprintf("%v %+v", err, err), leak) {
					t.Fatalf("leaked technical detail %q in %v", leak, err)
				}
			}
			seen[tt.wantCode] = struct{}{}
		})
	}
	if len(seen) != len(tests) {
		t.Fatalf("expected %d distinct categories, got %d: %+v", len(tests), len(seen), seen)
	}
}

func TestWriteBridge_ResourcePort_FakeAdapted(t *testing.T) {
	resources := &fakeResourceReader{
		create: func(ctx context.Context, command domain.CreateCommand) (domain.Resource, error) {
			return domain.Resource{ClassCode: command.Scope.ClassCode, IdentityKey: "v1|x"}, nil
		},
		get: func(ctx context.Context, classCode, identityKey string) (domain.Resource, error) {
			return domain.Resource{ID: 1, ClassCode: classCode, IdentityKey: identityKey, Revision: 1}, nil
		},
	}
	adapter := newTestAdapter(nil, resources)
	res, err := adapter.CreateResource(context.Background(), public.ResourceWriteRequest{
		Actor:       "PI",
		Scope:       public.ResourceScope{ClassCode: "MAT", FamilyCode: "CONDUCTORES", TypeCode: "CABLE"},
		NaturalUnit: "m",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.ID != 1 || res.IdentityV1 != "v1|x" {
		t.Fatalf("unexpected resource: %+v", res)
	}
}

func TestResourceCreate_FieldCompleteness_AgainstCreateCommand(t *testing.T) {
	var captured domain.CreateCommand
	resources := &fakeResourceReader{
		create: func(ctx context.Context, command domain.CreateCommand) (domain.Resource, error) {
			captured = command
			return domain.Resource{ClassCode: command.Scope.ClassCode, IdentityKey: "v1|x"}, nil
		},
		get: func(ctx context.Context, classCode, identityKey string) (domain.Resource, error) {
			return domain.Resource{ID: 9, ClassCode: classCode, IdentityKey: identityKey, Revision: 1}, nil
		},
	}
	adapter := newTestAdapter(nil, resources)
	_, err := adapter.CreateResource(context.Background(), public.ResourceWriteRequest{
		Actor:       "PI",
		Scope:       public.ResourceScope{ClassCode: "MAT", FamilyCode: "CONDUCTORES", TypeCode: "CABLE"},
		NaturalUnit: "m",
		Attributes:  []public.AttributeValue{{Code: "gauge", Value: public.Value{Kind: public.ValueText, Text: "12"}}},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if captured.Scope.ClassCode != "MAT" || captured.Scope.FamilyCode != "CONDUCTORES" || captured.Scope.TypeCode != "CABLE" {
		t.Fatalf("Scope not fully mapped: %+v", captured.Scope)
	}
	if captured.NaturalUnit != "m" {
		t.Fatalf("NaturalUnit not mapped: %+v", captured)
	}
	if len(captured.Attributes) != 1 || captured.Attributes[0].AttributeCode != "gauge" {
		t.Fatalf("Attributes not mapped: %+v", captured.Attributes)
	}
}

func TestResourceCreate_AttributeMapping_InverseRoundTripSevenKinds(t *testing.T) {
	tests := []struct {
		name  string
		value public.Value
	}{
		{"controlled_option", public.Value{Kind: public.ValueControlledOption, Text: "RED"}},
		{"integer", public.Value{Kind: public.ValueInteger, Text: "42"}},
		{"decimal", public.Value{Kind: public.ValueDecimal, Text: "1.5"}},
		{"quantity", public.Value{Kind: public.ValueQuantity, Text: "1.5", UnitCode: "m"}},
		{"boolean", public.Value{Kind: public.ValueBool, Bool: true}},
		{"not_applicable", public.Value{Kind: public.ValueNotApplicable}},
		{"text", public.Value{Kind: public.ValueText, Text: "hello"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var captured domain.CreateCommand
			resources := &fakeResourceReader{
				create: func(ctx context.Context, command domain.CreateCommand) (domain.Resource, error) {
					captured = command
					return domain.Resource{ClassCode: command.Scope.ClassCode, IdentityKey: "v1|x"}, nil
				},
				get: func(ctx context.Context, classCode, identityKey string) (domain.Resource, error) {
					return domain.Resource{ID: 1, ClassCode: classCode, IdentityKey: identityKey, Revision: 1, Attributes: captured.Attributes}, nil
				},
			}
			adapter := newTestAdapter(nil, resources)
			res, err := adapter.CreateResource(context.Background(), public.ResourceWriteRequest{
				Actor:       "PI",
				Scope:       public.ResourceScope{ClassCode: "MAT", FamilyCode: "CONDUCTORES", TypeCode: "CABLE"},
				NaturalUnit: "m",
				Attributes:  []public.AttributeValue{{Code: "attr", Value: tt.value}},
			})
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(res.Attributes) != 1 || res.Attributes[0].Value.Kind != tt.value.Kind {
				t.Fatalf("round trip kind mismatch: got %+v, want %+v", res.Attributes, tt.value)
			}
		})
	}
}

func TestResourceCreate_AttributeMapping_RejectsFourUnmappableKinds(t *testing.T) {
	for _, kind := range []public.ValueKind{public.ValueCode, public.ValueEnum, public.ValueStringList, public.ValueReference} {
		t.Run(string(kind), func(t *testing.T) {
			resources := &fakeResourceReader{
				create: func(ctx context.Context, command domain.CreateCommand) (domain.Resource, error) {
					t.Fatalf("Create must not be called for an unmappable attribute kind")
					return domain.Resource{}, nil
				},
			}
			adapter := newTestAdapter(nil, resources)
			_, err := adapter.CreateResource(context.Background(), public.ResourceWriteRequest{
				Actor:       "PI",
				Scope:       public.ResourceScope{ClassCode: "MAT", FamilyCode: "CONDUCTORES", TypeCode: "CABLE"},
				NaturalUnit: "m",
				Attributes:  []public.AttributeValue{{Code: "attr", Value: public.Value{Kind: kind, Text: "x"}}},
			})
			if !public.IsCode(err, public.InvalidArgument) {
				t.Fatalf("expected INVALID_ARGUMENT for unmappable kind %s, got %v", kind, err)
			}
		})
	}
}

func TestResourceCreate_ConfirmRead_IdentityV1NoReclassification(t *testing.T) {
	resources := &fakeResourceReader{
		create: func(ctx context.Context, command domain.CreateCommand) (domain.Resource, error) {
			return domain.Resource{ClassCode: command.Scope.ClassCode, IdentityKey: "v1|x"}, nil
		},
		get: func(ctx context.Context, classCode, identityKey string) (domain.Resource, error) {
			return domain.Resource{ID: 5, ClassCode: classCode, IdentityKey: identityKey, Revision: 1}, nil
		},
	}
	adapter := newTestAdapter(nil, resources)
	res, err := adapter.CreateResource(context.Background(), public.ResourceWriteRequest{
		Actor:       "PI",
		Scope:       public.ResourceScope{ClassCode: "MAT", FamilyCode: "CONDUCTORES", TypeCode: "CABLE"},
		NaturalUnit: "m",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.ID <= 0 || res.Revision < 1 || res.IdentityV1 != "v1|x" {
		t.Fatalf("expected persisted projection from the confirm read, got %+v", res)
	}

	resources.get = func(ctx context.Context, classCode, identityKey string) (domain.Resource, error) {
		return domain.Resource{}, domain.ErrResourceNotFound
	}
	_, err = adapter.CreateResource(context.Background(), public.ResourceWriteRequest{
		Actor:       "PI",
		Scope:       public.ResourceScope{ClassCode: "MAT", FamilyCode: "CONDUCTORES", TypeCode: "CABLE"},
		NaturalUnit: "m",
	})
	if !public.IsCode(err, public.NotFound) {
		t.Fatalf("expected an unreclassified NOT_FOUND from a failed confirm read, got %v", err)
	}
}

func TestResourceCreate_NineCategoryTable_DistinctAndNoLeakage(t *testing.T) {
	pgxErr := bridgePgxLikeError{"23505", "recursos_identity_key_v1", "recursos", "identity_key", "duplicate key value violates unique constraint"}
	tests := []struct {
		name     string
		err      error
		wantCode public.ErrorCode
	}{
		{"invalid argument", catalogo.ErrInvalidArgument, public.InvalidArgument},
		{"not found", domain.ErrResourceNotFound, public.NotFound},
		{"duplicate", domain.ErrDuplicateResource, public.Duplicate},
		{"invalid reference", domain.ErrResourceReference, public.InvalidReference},
		{"validation", domain.ErrResourceValidation, public.Validation},
		{"integrity", domain.ErrResourceIntegrity, public.Integrity},
		{"invalid catalog", domain.WrapInvalidCatalog(domain.ErrResourceValidation), public.InvalidCatalog},
		{"unavailable", core.ErrUnavailable, public.Unavailable},
		{"internal", pgxErr, public.Internal},
	}
	seen := map[public.ErrorCode]struct{}{}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resources := &fakeResourceReader{
				create: func(ctx context.Context, command domain.CreateCommand) (domain.Resource, error) {
					return domain.Resource{}, tt.err
				},
			}
			adapter := newTestAdapter(nil, resources)
			_, err := adapter.CreateResource(context.Background(), public.ResourceWriteRequest{
				Actor:       "PI",
				Scope:       public.ResourceScope{ClassCode: "MAT", FamilyCode: "CONDUCTORES", TypeCode: "CABLE"},
				NaturalUnit: "m",
			})
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
			for _, leak := range []string{pgxErr.SQLState, pgxErr.ConstraintName, pgxErr.TableName, pgxErr.ColumnName, pgxErr.ServerMessage} {
				if leak != "" && strings.Contains(fmt.Sprintf("%v %+v", err, err), leak) {
					t.Fatalf("leaked technical detail %q in %v", leak, err)
				}
			}
			seen[tt.wantCode] = struct{}{}
		})
	}
	if len(seen) != len(tests) {
		t.Fatalf("expected %d distinct categories, got %d: %+v", len(tests), len(seen), seen)
	}
}
