// Package resourcecore is the internal bridge between the public resourcecore
// read contract and the authoritative application services. It translates
// public requests to domain calls, maps neutral errors to public errors, and
// defensively copies data crossing the boundary.
package resourcecore

import (
	"context"
	"fmt"
	"strconv"

	"github.com/GARFEX33/garfex-costos-unitarios/internal/core"
	"github.com/GARFEX33/garfex-costos-unitarios/internal/domain"
	public "github.com/GARFEX33/garfex-costos-unitarios/resourcecore"
)

// catalogReader is the narrow catalog read seam the bridge consumes.
type catalogReader interface {
	Kinds() []domain.CatalogKind
	List(ctx context.Context, kind domain.CatalogKindCode, filter domain.CatalogFilter) ([]domain.CatalogRecord, error)
	Get(ctx context.Context, kind domain.CatalogKindCode, id int64) (domain.CatalogRecord, error)
}

// resourceReader is the narrow resource read seam the bridge consumes.
type resourceReader interface {
	Get(ctx context.Context, classCode, identityKey string) (domain.Resource, error)
	SearchPage(ctx context.Context, criteria domain.SearchCriteria) (domain.ResourcePage, error)
	Describe(resource domain.Resource) string
}

// Adapter implements public.ReadCapabilities by delegating to the internal
// catalog and resource application services.
type Adapter struct {
	catalog   catalogReader
	resources resourceReader
	kinds     map[domain.CatalogKindCode]domain.CatalogKind
}

// NewAdapter returns a ReadCapabilities implementation backed by catalog and
// resources. Neither dependency may be nil.
func NewAdapter(catalog catalogReader, resources resourceReader) *Adapter {
	kinds := make(map[domain.CatalogKindCode]domain.CatalogKind, len(catalog.Kinds()))
	for _, kind := range catalog.Kinds() {
		kinds[kind.Code] = kind
	}
	return &Adapter{catalog: catalog, resources: resources, kinds: kinds}
}

func (a *Adapter) fieldKind(kind domain.CatalogKindCode, name string) domain.FieldKind {
	k, ok := a.kinds[kind]
	if !ok {
		return domain.FieldText
	}
	for _, f := range k.Fields {
		if f.Name == name {
			return f.Kind
		}
	}
	return domain.FieldText
}

// ActiveClasses returns active CLASE records.
func (a *Adapter) ActiveClasses(ctx context.Context) ([]public.CatalogRecord, error) {
	recs, err := a.catalog.List(ctx, domain.KindClass, domain.CatalogFilter{Status: domain.CatalogStatusActive})
	if err != nil {
		return nil, mapError(err)
	}
	return a.mapCatalogRecordSlice(domain.KindClass, recs), nil
}

// CatalogDescriptors returns a public descriptor for every registered kind.
func (a *Adapter) CatalogDescriptors(ctx context.Context) ([]public.CatalogDescriptor, error) {
	_ = ctx
	descs := make([]public.CatalogDescriptor, 0, len(a.catalog.Kinds()))
	for _, kind := range a.catalog.Kinds() {
		descs = append(descs, mapCatalogDescriptor(kind))
	}
	return descs, nil
}

// ListCatalog returns a page of catalog records for the requested kind.
func (a *Adapter) ListCatalog(ctx context.Context, q public.CatalogQuery) (public.CatalogPage, error) {
	filter := domain.CatalogFilter{
		Status: catalogStatusFromPublic(q.Scope),
		Text:   q.Text,
		Limit:  q.Limit + 1,
		Offset: q.Offset,
	}
	recs, err := a.catalog.List(ctx, domain.CatalogKindCode(q.Kind), filter)
	if err != nil {
		return public.CatalogPage{}, mapError(err)
	}
	return a.buildCatalogPage(q, recs), nil
}

// GetCatalog returns one catalog record by kind and id.
func (a *Adapter) GetCatalog(ctx context.Context, key public.CatalogKey) (public.CatalogRecord, error) {
	rec, err := a.catalog.Get(ctx, domain.CatalogKindCode(key.Kind), key.ID)
	if err != nil {
		return public.CatalogRecord{}, mapError(err)
	}
	return a.mapCatalogRecord(domain.CatalogKindCode(key.Kind), rec), nil
}

// SearchResources returns a page of resources matching the query.
func (a *Adapter) SearchResources(ctx context.Context, q public.ResourceQuery) (public.ResourcePage, error) {
	if q.Scope == public.ScopeAll {
		return public.ResourcePage{}, public.NewError(public.InvalidArgument, "ScopeAll is not supported for resource search")
	}
	if q.TypeCode != "" {
		return public.ResourcePage{}, public.NewError(public.InvalidArgument, "TypeCode filtering is not supported for resource search")
	}
	criteria := domain.SearchCriteria{
		LifecycleScope: resourceLifecycleFromPublic(q.Scope),
		ClassCode:      q.ClassCode,
		FamilyCode:     q.FamilyCode,
		Text:           q.Text,
		Limit:          q.Limit,
		Offset:         q.Offset,
	}
	page, err := a.resources.SearchPage(ctx, criteria)
	if err != nil {
		return public.ResourcePage{}, mapError(err)
	}
	return public.ResourcePage{
		Query:       mapResourceQuery(page.Criteria),
		Resources:   mapResourceSlice(page.Resources),
		HasPrevious: page.HasPrevious,
		HasNext:     page.HasNext,
	}, nil
}

// GetResource returns one resource by class code and durable identity-v1.
func (a *Adapter) GetResource(ctx context.Context, key public.ResourceKey) (public.Resource, error) {
	res, err := a.resources.Get(ctx, key.ClassCode, key.IdentityV1)
	if err != nil {
		return public.Resource{}, mapError(err)
	}
	return mapResource(res), nil
}

// DescribeResource returns the canonical presentation of the identified
// resource by delegating to the resource service's Describe method.
func (a *Adapter) DescribeResource(ctx context.Context, key public.ResourceKey) (string, error) {
	res, err := a.resources.Get(ctx, key.ClassCode, key.IdentityV1)
	if err != nil {
		return "", mapError(err)
	}
	return a.resources.Describe(res), nil
}

func mapError(err error) error {
	if err == nil {
		return nil
	}
	neut := core.Map(err)
	return public.NewError(public.ErrorCode(neut.Code()), neut.Error())
}

func catalogStatusFromPublic(s public.LifecycleScope) domain.CatalogStatus {
	switch s {
	case public.ScopeInactive:
		return domain.CatalogStatusInactive
	case public.ScopeAll:
		return domain.CatalogStatusAll
	}
	return domain.CatalogStatusActive
}

func resourceLifecycleFromPublic(s public.LifecycleScope) domain.ResourceLifecycleScope {
	if s == public.ScopeInactive {
		return domain.LifecycleScopeInactive
	}
	return domain.LifecycleScopeActive
}

func (a *Adapter) buildCatalogPage(q public.CatalogQuery, recs []domain.CatalogRecord) public.CatalogPage {
	hasNext := len(recs) > q.Limit && q.Limit > 0
	end := len(recs)
	if hasNext {
		end = q.Limit
	}
	if q.Limit == 0 {
		hasNext = false
	}
	return public.CatalogPage{
		Query: public.CatalogQuery{
			Kind:   q.Kind,
			Scope:  q.Scope,
			Text:   q.Text,
			Limit:  q.Limit,
			Offset: q.Offset,
		},
		Records:     a.mapCatalogRecordSlice(domain.CatalogKindCode(q.Kind), recs[:end]),
		HasPrevious: q.Offset > 0,
		HasNext:     hasNext,
	}
}

func mapCatalogDescriptor(kind domain.CatalogKind) public.CatalogDescriptor {
	fields := make([]public.FieldDescriptor, len(kind.Fields))
	for i, f := range kind.Fields {
		fields[i] = public.FieldDescriptor{
			Name:        f.Name,
			Label:       f.Label,
			Kind:        valueKindFromFieldKind(f.Kind),
			Required:    f.Required,
			RefKind:     public.KindCode(f.RefKind),
			RefScopedBy: append([]string(nil), f.RefScopedBy...),
			EnumValues:  mapEnumValues(f.EnumValues),
		}
	}
	identity := make([]string, len(kind.IdentityFields))
	copy(identity, kind.IdentityFields)
	return public.CatalogDescriptor{
		Kind:           public.KindCode(kind.Code),
		Singular:       kind.Singular,
		Plural:         kind.Plural,
		Fields:         fields,
		IdentityFields: identity,
		ParentKind:     public.KindCode(kind.ParentKind),
		ParentField:    kind.ParentField,
	}
}

func mapEnumValues(values []domain.EnumValue) []public.EnumValue {
	if values == nil {
		return nil
	}
	out := make([]public.EnumValue, len(values))
	for i, v := range values {
		out[i] = public.EnumValue{Value: v.Value, Label: v.Label}
	}
	return out
}

func valueKindFromFieldKind(k domain.FieldKind) public.ValueKind {
	switch k {
	case domain.FieldText:
		return public.ValueText
	case domain.FieldCode:
		return public.ValueCode
	case domain.FieldBool:
		return public.ValueBool
	case domain.FieldInt:
		return public.ValueInteger
	case domain.FieldRef:
		return public.ValueReference
	case domain.FieldEnum:
		return public.ValueEnum
	case domain.FieldStringList:
		return public.ValueStringList
	}
	return public.ValueText
}

func (a *Adapter) mapCatalogRecordSlice(kind domain.CatalogKindCode, recs []domain.CatalogRecord) []public.CatalogRecord {
	out := make([]public.CatalogRecord, len(recs))
	for i, rec := range recs {
		out[i] = a.mapCatalogRecord(kind, rec)
	}
	return out
}

func (a *Adapter) mapCatalogRecord(kind domain.CatalogKindCode, rec domain.CatalogRecord) public.CatalogRecord {
	values := make(map[string]public.Value, len(rec.Values))
	for name, value := range rec.Values {
		values[name] = a.mapCatalogValue(kind, name, value)
	}
	return public.CatalogRecord{
		Kind:     public.KindCode(rec.Kind),
		ID:       rec.ID,
		Revision: rec.Revision,
		Active:   rec.Active,
		Values:   values,
		Rules:    mapCatalogRules(rec.Rules),
	}
}

func mapCatalogRules(rules []domain.CatalogRuleRecord) []public.ApplicabilityRule {
	if rules == nil {
		return nil
	}
	out := make([]public.ApplicabilityRule, len(rules))
	for i, r := range rules {
		out[i] = public.ApplicabilityRule{
			AttributeCode:        r.When.AttributeCode,
			Equals:               public.Value{Kind: public.ValueText, Text: r.When.Equals},
			Mode:                 string(r.Mode),
			IdentityParticipates: r.IdentityParticipates,
			NotApplicable:        r.NotApplicable,
			Active:               r.Active,
		}
	}
	return out
}

func (a *Adapter) mapCatalogValue(kind domain.CatalogKindCode, name string, v domain.CatalogValue) public.Value {
	fieldKind := a.fieldKind(kind, name)
	switch fieldKind {
	case domain.FieldCode:
		return public.Value{Kind: public.ValueCode, Text: v.Text}
	case domain.FieldBool:
		return public.Value{Kind: public.ValueBool, Bool: v.Bool}
	case domain.FieldInt:
		return public.Value{Kind: public.ValueInteger, Text: strconv.FormatInt(int64(v.Int), 10)}
	case domain.FieldStringList:
		return public.Value{Kind: public.ValueStringList, Strings: append([]string(nil), v.List...)}
	case domain.FieldRef:
		if v.Ref.Code == "" && v.Ref.Kind == "" {
			return public.Value{Kind: public.ValueReference}
		}
		return public.Value{Kind: public.ValueReference, Reference: &public.Reference{Kind: public.KindCode(v.Ref.Kind), ID: 0, Code: v.Ref.Code}}
	case domain.FieldEnum:
		return public.Value{Kind: public.ValueEnum, Text: v.Text}
	}
	return public.Value{Kind: public.ValueText, Text: v.Text}
}

func mapResourceQuery(criteria domain.SearchCriteria) public.ResourceQuery {
	return public.ResourceQuery{
		Scope:      publicScopeFromResourceLifecycle(criteria.LifecycleScope),
		ClassCode:  criteria.ClassCode,
		FamilyCode: criteria.FamilyCode,
		Text:       criteria.Text,
		Limit:      criteria.Limit,
		Offset:     criteria.Offset,
	}
}

func publicScopeFromResourceLifecycle(s domain.ResourceLifecycleScope) public.LifecycleScope {
	if s == domain.LifecycleScopeInactive {
		return public.ScopeInactive
	}
	return public.ScopeActive
}

func mapResourceSlice(resources []domain.Resource) []public.Resource {
	out := make([]public.Resource, len(resources))
	for i, res := range resources {
		out[i] = mapResource(res)
	}
	return out
}

func mapResource(res domain.Resource) public.Resource {
	attrs := make([]public.AttributeValue, len(res.Attributes))
	for i, av := range res.Attributes {
		attrs[i] = public.AttributeValue{
			Code:     av.AttributeCode,
			UnitCode: resourceUnitCode(av),
			Value:    mapResourceAttributeValue(av),
		}
	}
	return public.Resource{
		ID:          res.ID,
		IdentityV1:  res.IdentityKey,
		Scope:       public.ResourceScope{ClassCode: res.ClassCode, FamilyCode: res.FamilyCode, TypeCode: res.TypeCode},
		NaturalUnit: res.NaturalUnit,
		Active:      res.Active,
		Revision:    res.Revision,
		Attributes:  attrs,
	}
}

func resourceUnitCode(av domain.ResourceAttributeValue) string {
	if av.Quantity != nil {
		return av.Quantity.UnitCode
	}
	return ""
}

func mapResourceAttributeValue(av domain.ResourceAttributeValue) public.Value {
	switch av.Type {
	case domain.ValueTypeControlledOption:
		return public.Value{Kind: public.ValueControlledOption, Text: av.OptionCode}
	case domain.ValueTypeInteger:
		if av.Integer == nil {
			return public.Value{Kind: public.ValueInteger}
		}
		return public.Value{Kind: public.ValueInteger, Text: strconv.FormatInt(*av.Integer, 10)}
	case domain.ValueTypeDecimal:
		if av.Decimal == nil {
			return public.Value{Kind: public.ValueDecimal}
		}
		return public.Value{Kind: public.ValueDecimal, Text: av.Decimal.String()}
	case domain.ValueTypeQuantity:
		if av.Quantity == nil {
			return public.Value{Kind: public.ValueQuantity}
		}
		return public.Value{Kind: public.ValueQuantity, Text: av.Quantity.Value.String(), UnitCode: av.Quantity.UnitCode}
	case domain.ValueTypeBoolean:
		if av.Boolean == nil {
			return public.Value{Kind: public.ValueBool}
		}
		return public.Value{Kind: public.ValueBool, Bool: *av.Boolean}
	case domain.ValueTypeControlledText:
		if av.Text == domain.NotApplicableText {
			return public.Value{Kind: public.ValueNotApplicable}
		}
		return public.Value{Kind: public.ValueText, Text: av.Text}
	}
	return public.Value{Kind: public.ValueText, Text: av.Text}
}

func (a *Adapter) String() string {
	return fmt.Sprintf("resourcecore.Adapter(%p)", a)
}
