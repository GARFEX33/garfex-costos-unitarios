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

var (
	ErrMaterialValidation = errors.New("material validation failed")
	ErrDuplicateMaterial  = errors.New("duplicate material")
	ErrMaterialReference  = errors.New("material catalog reference invalid")
	ErrMaterialNotFound   = errors.New("material not found")
)

type MaterialCategory struct{ Code, Name string }

type MaterialFamily struct {
	Code     string
	Name     string
	Category string
}

type UnitDefinition struct {
	Code      string
	Symbol    string
	Dimension string
}

// FamilyUnitPolicy controls management units only. It has deliberately no
// identity flag: NaturalUnit never changes technical identity.
type FamilyUnitPolicy struct {
	FamilyCode string
	UnitCode   string
	Allowed    bool
	Suggested  bool
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

type FamilyAttribute struct {
	FamilyCode           string
	Definition           AttributeDefinition
	Mode                 AttributeMode
	IdentityParticipates bool
	Rules                []AttributeRule
}

type AttributeOption struct {
	AttributeCode string
	Code          string
	Label         string
}

type Quantity struct {
	Value    decimal.Decimal
	UnitCode string
}

type MaterialAttributeValue struct {
	AttributeCode string
	Type          AttributeValueType
	OptionCode    string
	Integer       *int64
	Decimal       *decimal.Decimal
	Quantity      *Quantity
	Boolean       *bool
	Text          string
}

func OptionValue(attribute, option string) MaterialAttributeValue {
	return MaterialAttributeValue{AttributeCode: attribute, Type: ValueTypeControlledOption, OptionCode: option}
}

func QuantityValue(attribute, value, unit string) MaterialAttributeValue {
	parsed, err := decimal.NewFromString(value)
	if err != nil {
		parsed = decimal.Zero
	}
	return MaterialAttributeValue{AttributeCode: attribute, Type: ValueTypeQuantity, Quantity: &Quantity{Value: parsed, UnitCode: unit}}
}

func IntegerValue(attribute string, value int64) MaterialAttributeValue {
	return MaterialAttributeValue{AttributeCode: attribute, Type: ValueTypeInteger, Integer: &value}
}

func DecimalValue(attribute, value string) MaterialAttributeValue {
	parsed, err := decimal.NewFromString(value)
	if err != nil {
		parsed = decimal.Zero
	}
	return MaterialAttributeValue{AttributeCode: attribute, Type: ValueTypeDecimal, Decimal: &parsed}
}

func BooleanValue(attribute string, value bool) MaterialAttributeValue {
	return MaterialAttributeValue{AttributeCode: attribute, Type: ValueTypeBoolean, Boolean: &value}
}

func ControlledTextValue(attribute, value string) MaterialAttributeValue {
	return MaterialAttributeValue{AttributeCode: attribute, Type: ValueTypeControlledText, Text: value}
}

type Material struct {
	FamilyCode  string
	NaturalUnit string
	Attributes  []MaterialAttributeValue
	IdentityKey string
}

// SearchCriteria narrows a Search over the material catalog. All fields
// combine with AND; a zero-value criteria matches every active material.
type SearchCriteria struct {
	// Text, when non-empty, is matched case-insensitively as a partial
	// substring against the material identity key or its family code/name.
	//
	// This is the initial textual implementation over the existing schema,
	// not a permanent guarantee of the public SearchMaterials contract
	// shape: a future change may swap in a dedicated search layer without
	// changing this Go signature.
	//
	// Known, accepted, documented limitation: with today's seeded catalog
	// data, Text="cab" will NOT match CONDUCTORES materials, because the
	// literal substring "cable" does not exist anywhere in the current
	// family/attribute-option data (the family is named "Conductores" /
	// "Conductors", not "cable"). This is a data/naming gap, not a bug.
	Text       string
	FamilyCode string
	// Filters requires an exact, canonical match per attribute: each entry
	// is ANDed as its own existence check against the material's attribute
	// values.
	Filters []MaterialAttributeValue
	Limit   int
	Offset  int
}

// MaterialRepository persists and retrieves the complete technical aggregate.
// NaturalUnit is stored metadata; IdentityKey is the deterministic lookup key.
type MaterialRepository interface {
	Create(context.Context, Material) error
	Get(context.Context, string, string) (Material, error)
	Search(context.Context, SearchCriteria) ([]Material, error)
}

type AttributeOptionRelation struct {
	FromAttribute string
	FromOption    string
	ToAttribute   string
	ToOption      string
}

type MaterialsCatalog struct {
	Categories       []MaterialCategory
	Families         []MaterialFamily
	Units            []UnitDefinition
	UnitPolicies     []FamilyUnitPolicy
	Definitions      []AttributeDefinition
	FamilyAttributes []FamilyAttribute
	Options          []AttributeOption
	Relations        []AttributeOptionRelation
}
