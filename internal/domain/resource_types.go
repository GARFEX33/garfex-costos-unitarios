package domain

import (
	"context"
	"errors"

	"github.com/shopspring/decimal"
)

type AttributeValueType string

const (
	ValueTypeControlledOption AttributeValueType = "CONTROLLED_OPTION"
	ValueTypeInteger          AttributeValueType = "INTEGER"
	ValueTypeDecimal          AttributeValueType = "DECIMAL"
	ValueTypeQuantity         AttributeValueType = "QUANTITY"
	ValueTypeBoolean          AttributeValueType = "BOOLEAN"
	ValueTypeControlledText   AttributeValueType = "CONTROLLED_TEXT"
)

type AttributeMode string

const (
	ModeRequired    AttributeMode = "REQUIRED"
	ModeOptional    AttributeMode = "OPTIONAL"
	ModeConditional AttributeMode = "CONDITIONAL"
	ModeForbidden   AttributeMode = "FORBIDDEN"
)

// ErrResourceValidation and ErrResourceReference are declared in
// resource_catalog_validate.go (added in PR1) — do not redeclare here.
var (
	ErrDuplicateResource = errors.New("duplicate resource")
	ErrResourceNotFound  = errors.New("resource not found")
)

// PresentationField is one entry of a ResourceType's catalog-controlled
// canonical presentation: an explicitly ordered subset of its attributes,
// used to compose a human-readable title (see ResourceCatalog.Describe).
// It is deliberately never an automatic dump of every attribute. The full
// ClassCode+FamilyCode+TypeCode triple (design R2) is required because
// class-owned families (D1) make a bare TypeCode lookup ambiguous.
type PresentationField struct {
	ClassCode     string
	FamilyCode    string
	TypeCode      string
	AttributeCode string
	Position      int
}

// NotApplicableText is the sentinel domain.ResourceAttributeValue.Text value
// for an attribute that is structurally not applicable to a resource (e.g.
// color/voltage on a DESNUDO conductor). A Resource fetched from the real
// repository carries this marker instead of omitting the attribute entirely.
const NotApplicableText = "NOT_APPLICABLE"

type UnitDefinition struct {
	Code      string
	Name      string
	Symbol    string
	Dimension string
	// Active — see ResourceFamily.Active (resource_catalog_validate.go).
	Active bool
}

type AttributeDefinition struct {
	Code                        string
	Name                        string
	ValueType                   AttributeValueType
	Dimension                   string
	DefaultIdentityParticipates bool
}

type AttributeCondition struct {
	AttributeCode string
	Equals        string
}

type AttributeRule struct {
	When                 AttributeCondition
	Mode                 AttributeMode
	IdentityParticipates bool
	NotApplicable        bool
}

// AttributeOption is one controlled value. OptionSet (design D3) names the
// set it belongs to; "" resolves to the shared default set via
// canonicalOptionSet — see resource_catalog_query.go.
type AttributeOption struct {
	OptionSet     string
	AttributeCode string
	Code          string
	Label         string
	// Active — see ResourceFamily.Active (resource_catalog_validate.go).
	Active bool
}

type Quantity struct {
	Value    decimal.Decimal
	UnitCode string
}

type ResourceAttributeValue struct {
	AttributeCode string
	Type          AttributeValueType
	OptionCode    string
	Integer       *int64
	Decimal       *decimal.Decimal
	Quantity      *Quantity
	Boolean       *bool
	Text          string
}

func OptionValue(attribute, option string) ResourceAttributeValue {
	return ResourceAttributeValue{AttributeCode: attribute, Type: ValueTypeControlledOption, OptionCode: option}
}

func QuantityValue(attribute, value, unit string) ResourceAttributeValue {
	parsed, err := decimal.NewFromString(value)
	if err != nil {
		parsed = decimal.Zero
	}
	return ResourceAttributeValue{AttributeCode: attribute, Type: ValueTypeQuantity, Quantity: &Quantity{Value: parsed, UnitCode: unit}}
}

func IntegerValue(attribute string, value int64) ResourceAttributeValue {
	return ResourceAttributeValue{AttributeCode: attribute, Type: ValueTypeInteger, Integer: &value}
}

func DecimalValue(attribute, value string) ResourceAttributeValue {
	parsed, err := decimal.NewFromString(value)
	if err != nil {
		parsed = decimal.Zero
	}
	return ResourceAttributeValue{AttributeCode: attribute, Type: ValueTypeDecimal, Decimal: &parsed}
}

func BooleanValue(attribute string, value bool) ResourceAttributeValue {
	return ResourceAttributeValue{AttributeCode: attribute, Type: ValueTypeBoolean, Boolean: &value}
}

func ControlledTextValue(attribute, value string) ResourceAttributeValue {
	return ResourceAttributeValue{AttributeCode: attribute, Type: ValueTypeControlledText, Text: value}
}

type Resource struct {
	ID          int64
	ClassCode   string
	FamilyCode  string
	TypeCode    string
	NaturalUnit string
	Attributes  []ResourceAttributeValue
	IdentityKey string
}

// SearchCriteria narrows a Search over the resource catalog. All fields
// combine with AND; a zero-value criteria matches every active resource.
type SearchCriteria struct {
	// Text, when non-empty, is matched case-insensitively as a partial
	// substring against the resource identity key or its family code/name.
	//
	// This is the initial textual implementation over the existing schema,
	// not a permanent guarantee of the public SearchMaterials contract
	// shape: a future change may swap in a dedicated search layer without
	// changing this Go signature.
	Text       string
	ClassCode  string
	FamilyCode string
	// Filters requires an exact, canonical match per attribute: each entry
	// is ANDed as its own existence check against the resource's attribute
	// values.
	Filters []ResourceAttributeValue
	Limit   int
	Offset  int
}

// ResourceRepository persists and retrieves the complete technical
// aggregate. NaturalUnit is stored metadata; IdentityKey is the
// deterministic lookup key. Get's first argument is the owning class code
// (design R1): IdentityKey already encodes class|family|type, so class+key
// is the minimal complete lookup.
type ResourceRepository interface {
	Create(context.Context, Resource) error
	Get(context.Context, string, string) (Resource, error)
	Search(context.Context, SearchCriteria) ([]Resource, error)
	Update(context.Context, Resource) error
	SetActive(context.Context, int64, bool) error
}

// AttributeOptionRelation narrows one attribute's valid options by another
// attribute's already-chosen value. OptionSet (design D3) scopes the
// relation to one named option set: a relation cannot connect an attribute
// bound to set A to one bound to set B (design R4, not solved here).
type AttributeOptionRelation struct {
	OptionSet     string
	FromAttribute string
	FromOption    string
	ToAttribute   string
	ToOption      string
}

// ResourceCatalog is the unified Clase/Familia/Tipo/Atributo catalog
// (design §2). ResourceClass, ResourceFamily, ResourceType,
// ResourceAttribute, ResourceUnitPolicy, and Validate()/its helpers are
// declared in resource_class.go / resource_scope.go / resource_catalog_validate.go
// (PR1) — extended here with the remaining design §2 fields rather than
// redeclared.
type ResourceCatalog struct {
	Classes            []ResourceClass
	Families           []ResourceFamily
	Types              []ResourceType
	PresentationFields []PresentationField
	Units              []UnitDefinition
	UnitPolicies       []ResourceUnitPolicy
	Definitions        []AttributeDefinition
	Attributes         []ResourceAttribute
	Options            []AttributeOption
	Relations          []AttributeOptionRelation
}
